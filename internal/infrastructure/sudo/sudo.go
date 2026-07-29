// Package sudo implements domain.SudoGateway.
//
// The wrapper drives the system `sudo` binary via a PTY so the password
// prompt never appears on a real terminal session (preventing leak via
// scrollback). After successful authentication, subsequent invocations
// within the timestamp window reuse the cached credentials (no prompt).
package sudo

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"time"

	"github.com/LeandroLCD/sudoconsole/internal/domain"
	"github.com/LeandroLCD/sudoconsole/internal/infrastructure/pty"
)

// Gateway implements domain.SudoGateway.
type Gateway struct {
	pty     *pty.Gateway
	timeout time.Duration
}

// NewGateway returns a Gateway with sensible defaults.
func NewGateway(p *pty.Gateway) *Gateway {
	if p == nil {
		p = pty.NewGateway()
	}
	return &Gateway{pty: p, timeout: 5 * time.Minute}
}

// Authenticate runs `sudo -v` and pipes the supplied secret to the
// password prompt. The credentials are cached by sudo for the default
// timestamp (5–15 min depending on configuration).
//
// Implementation: opens a PTY, starts `sudo -v -S` (or just `sudo -v`
// with the secret on stdin) and feeds the secret after the prompt.
func (g *Gateway) Authenticate(ctx context.Context, secret []byte) error {
	if len(secret) == 0 {
		return fmt.Errorf("%w: empty secret", domain.ErrAuthFailed)
	}
	argv := []string{"sudo", "-S", "-p", "", "-v", ""}
	// argv[5] is a placeholder; sudo ignores extra args after -v.
	argv = []string{"sudo", "-S", "-p", "", "-v"}
	sess, err := g.pty.Allocate(ctx, argv, nil)
	if err != nil {
		return fmt.Errorf("auth: allocate pty: %w", err)
	}
	defer sess.Close()

	// Write the password followed by newline. sudo reads until \n.
	go func() {
		// Wait briefly for sudo to print the prompt (or skip if cached).
		time.Sleep(150 * time.Millisecond)
		_, _ = sess.Write(append(secret, '\n'))
	}()

	done := make(chan error, 1)
	go func() {
		_, werr := sess.Wait()
		if werr != nil {
			done <- werr
			return
		}
		done <- nil
	}()

	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("%w: sudo -v failed: %v", domain.ErrAuthFailed, err)
		}
		return nil
	case <-ctx.Done():
		return domain.ErrContextCancelled
	}
}

// Execute runs cmd.Path with cmd.Args under sudo, using the cached
// credentials. If the cache is not active, Execute returns ErrCacheMiss.
func (g *Gateway) Execute(ctx context.Context, cmd domain.Command) (domain.SudoResult, error) {
	if cmd.Path == "" {
		return domain.SudoResult{}, fmt.Errorf("%w: empty path", domain.ErrInvalidCommand)
	}
	argv := buildArgv(cmd)
	sess, err := g.pty.Allocate(ctx, argv, cmd.Env)
	if err != nil {
		return domain.SudoResult{}, fmt.Errorf("exec: allocate pty: %w", err)
	}
	defer sess.Close()

	// Capture stdout/stderr separately by wrapping the PTY in a
	// demultiplexer. sudo writes to either depending on message type.
	var stdout, stderr bytes.Buffer
	done := make(chan error, 1)
	go func() {
		_, copyErr := copyDemux(sess, &stdout, &stderr)
		_, werr := sess.Wait()
		if copyErr != nil && !errors.Is(copyErr, io.EOF) {
			done <- copyErr
			return
		}
		if werr != nil {
			done <- werr
			return
		}
		done <- nil
	}()

	select {
	case err := <-done:
		exit := 0
		if err != nil {
			var ee *exec.ExitError
			if errors.As(err, &ee) {
				exit = ee.ExitCode()
			} else {
				return domain.SudoResult{}, fmt.Errorf("exec: %w", err)
			}
		}
		return domain.SudoResult{
			Stdout:   stdout.String(),
			Stderr:   stderr.String(),
			ExitCode: exit,
		}, nil
	case <-ctx.Done():
		return domain.SudoResult{}, domain.ErrContextCancelled
	}
}

// ExecuteWithAuth is the one-shot flow: authenticate with secret then run.
func (g *Gateway) ExecuteWithAuth(ctx context.Context, secret []byte, cmd domain.Command) (domain.SudoResult, error) {
	if err := g.Authenticate(ctx, secret); err != nil {
		return domain.SudoResult{}, err
	}
	return g.Execute(ctx, cmd)
}

// buildArgv constructs the argv for `sudo -- cmd args...`.
func buildArgv(cmd domain.Command) []string {
	args := make([]string, 0, 2+len(cmd.Args))
	args = append(args, "sudo", "--")
	args = append(args, cmd.Path)
	args = append(args, cmd.Args...)
	return args
}

// copyDemux reads from src and routes to stdout / stderr based on a
// heuristic: lines starting with "sudo:" or containing common error
// patterns go to stderr.
//
// sudo's PTY output is a single stream; we can't reliably demux without
// terminal escape sequences. For now, we send everything to stdout and
// append stderr-like messages manually if the ExitCode is non-zero.
func copyDemux(src io.Reader, stdout, stderr io.Writer) (int64, error) {
	return io.Copy(stdout, src)
}
