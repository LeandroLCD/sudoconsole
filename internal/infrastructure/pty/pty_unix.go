//go:build linux || darwin

// Package pty wraps creack/pty to provide domain.PtyGateway.
//
// Two distinct concerns:
//
//  1. Allocating a PTY for a child process (used by the sudo wrapper to
//     drive sudo's password prompt without needing a real terminal).
//  2. Reading a secret from the user's controlling terminal with echo
//     disabled (used by sudo/auth.go to capture the password).
//
// Concern 1 uses creack/pty. Concern 2 uses golang.org/x/term directly
// against /dev/tty (the user's terminal) so we don't have to spawn a
// helper child.
package pty

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/LeandroLCD/sudoconsole/internal/domain"

	"github.com/creack/pty"
)

// Gateway implements domain.PtyGateway.
type Gateway struct {
	// DisableCoreDumps (when true) sets RLIMIT_CORE=0 on spawned
	// children via prlimit(2)/setrlimit(2) to prevent credential leaks
	// via coredumps.
	DisableCoreDumps bool
}

// NewGateway constructs a Gateway with safe defaults.
func NewGateway() *Gateway {
	return &Gateway{DisableCoreDumps: true}
}

// Allocate starts argv[0] with args argv[1:] attached to a freshly
// allocated PTY. The returned session exposes the PTY master fd for
// reading and writing.
func (g *Gateway) Allocate(ctx context.Context, argv []string, env []string) (domain.PtySession, error) {
	if len(argv) == 0 {
		return nil, errors.New("pty: empty argv")
	}
	cmd := exec.Command(argv[0], argv[1:]...) // #nosec G204 -- argv is validated by caller (usecase or sudo wrapper)
	cmd.Env = append(os.Environ(), env...)

	if g.DisableCoreDumps {
		disableCoreDump(cmd)
	}

	ptmx, tty, err := pty.Open()
	if err != nil {
		return nil, fmt.Errorf("%w: pty.Open: %w", domain.ErrPTYFailed, err)
	}

	// Initial window size.
	_ = pty.Setsize(ptmx, &pty.Winsize{Rows: 24, Cols: 80})

	cmd.Stdout = tty
	cmd.Stderr = tty
	cmd.Stdin = tty

	if err := cmd.Start(); err != nil {
		_ = ptmx.Close()
		_ = tty.Close()
		return nil, fmt.Errorf("pty: cmd.Start: %w", err)
	}
	_ = tty.Close() // child now owns its end

	s := &session{
		ptmx:  ptmx,
		cmd:   cmd,
		ctx:   ctx,
		donec: make(chan struct{}),
	}
	go s.reaper()
	return s, nil
}

// ReadSecret prints prompt to stderr (where sudo and friends expect it)
// and reads one line of input from /dev/tty with echo disabled.
//
// Returns the secret bytes (without the trailing newline). The caller
// MUST zeroize the returned slice.
//
// If /dev/tty is not available (e.g. running under CI without a tty),
// ReadSecret returns ErrPTYFailed.
func (g *Gateway) ReadSecret(ctx context.Context, prompt string) ([]byte, error) {
	// Use the controlling terminal directly.
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("%w: open /dev/tty: %w", domain.ErrPTYFailed, err)
	}
	defer func() { _ = tty.Close() }()

	// Print prompt to stderr.
	if _, err := os.Stderr.WriteString(prompt); err != nil {
		return nil, err
	}

	// Channel to enforce context cancellation.
	type result struct {
		line []byte
		err  error
	}
	done := make(chan result, 1)
	go func() {
		// term.ReadPassword handles stty -echo/raw/cbreak internally.
		line, err := readPasswordFrom(tty)
		done <- result{line: line, err: err}
	}()

	select {
	case r := <-done:
		if r.err != nil {
			return nil, r.err
		}
		return r.line, nil
	case <-ctx.Done():
		return nil, domain.ErrContextCancelled
	}
}

// session wraps a child process attached to a PTY.
type session struct {
	ptmx  *os.File
	cmd   *exec.Cmd
	ctx   context.Context
	donec chan struct{}
}

func (s *session) Read(p []byte) (int, error) {
	return s.ptmx.Read(p)
}

func (s *session) Write(p []byte) (int, error) {
	return s.ptmx.Write(p)
}

func (s *session) Close() error {
	_ = s.ptmx.Close()
	if s.cmd.Process != nil {
		// Best-effort kill if the child is still running.
		_ = s.cmd.Process.Signal(syscall.SIGHUP)
	}
	return nil
}

// Resize updates the PTY window size.
func (s *session) Resize(rows, cols uint16) error {
	return pty.Setsize(s.ptmx, &pty.Winsize{Rows: rows, Cols: cols})
}

// Pid returns the child process ID, or 0 if not yet started.
func (s *session) Pid() int {
	if s.cmd.Process == nil {
		return 0
	}
	return s.cmd.Process.Pid
}

// Wait blocks until the child exits and returns the exit code.
func (s *session) Wait() (int, error) {
	<-s.donec
	if s.cmd.ProcessState == nil {
		return -1, errors.New("pty: no process state")
	}
	return s.cmd.ProcessState.ExitCode(), nil
}

// reaper waits for the child to exit and closes donec.
func (s *session) reaper() {
	_ = s.cmd.Wait()
	close(s.donec)
}
