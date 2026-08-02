//go:build linux || darwin

package pty

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/LeandroLCD/sudoconsole/internal/domain"
)

func TestGateway_Allocate_SpawnsEcho(t *testing.T) {
	g := NewGateway()
	ctx := context.Background()
	sess, err := g.Allocate(ctx, []string{"/bin/sh", "-c", "echo HELLO"}, nil)
	if err != nil {
		// Skip if there's no PTY available (e.g. some sandboxes).
		if strings.Contains(err.Error(), "operation not permitted") {
			t.Skip("PTY not available in this environment")
		}
		t.Fatal(err)
	}
	defer sess.Close()

	buf := make([]byte, 1024)
	got := make([]byte, 0)
	done := make(chan struct{})
	go func() {
		for {
			n, err := sess.Read(buf)
			if n > 0 {
				got = append(got, buf[:n]...)
			}
			if err != nil {
				close(done)
				return
			}
		}
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for echo")
	}
	if !strings.Contains(string(got), "HELLO") {
		t.Fatalf("expected HELLO in output, got %q", string(got))
	}
}

func TestGateway_Allocate_EmptyArgv(t *testing.T) {
	g := NewGateway()
	_, err := g.Allocate(context.Background(), nil, nil)
	if err == nil {
		t.Fatal("empty argv should error")
	}
}

func TestGateway_Allocate_NonExistentCommand(t *testing.T) {
	g := NewGateway()
	_, err := g.Allocate(context.Background(), []string{"/nonexistent/binary-xyz"}, nil)
	if err == nil {
		t.Fatal("non-existent command should error")
	}
}

func TestGateway_ReadSecret_NoTTY(t *testing.T) {
	g := NewGateway()
	// When /dev/tty is not available (e.g. under CI), ReadSecret should
	// return ErrPTYFailed rather than silently capturing the wrong thing.
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	_, err := g.ReadSecret(ctx, "Password: ")
	if err == nil {
		t.Skip("test only meaningful when /dev/tty is unavailable")
	}
	if !errIsPTY(err) {
		t.Fatalf("expected ErrPTYFailed, got %v", err)
	}
}

func TestSession_Resize(t *testing.T) {
	g := NewGateway()
	sess, err := g.Allocate(context.Background(), []string{"/bin/sleep", "1"}, nil)
	if err != nil {
		t.Skip("PTY not available")
	}
	defer sess.Close()

	if err := sess.Resize(40, 100); err != nil {
		t.Fatalf("Resize failed: %v", err)
	}
}

func TestSession_Pid(t *testing.T) {
	g := NewGateway()
	sess, err := g.Allocate(context.Background(), []string{"/bin/sleep", "1"}, nil)
	if err != nil {
		t.Skip("PTY not available")
	}
	defer sess.Close()

	pid := sess.Pid()
	if pid <= 0 {
		t.Fatalf("Pid should be positive, got %d", pid)
	}
}

func TestSession_WriteRead(t *testing.T) {
	g := NewGateway()
	// Use a command that exits after reading one line.
	sess, err := g.Allocate(context.Background(), []string{"/bin/sh", "-c", "read line; echo got:$line"}, nil)
	if err != nil {
		t.Skip("PTY not available")
	}
	defer sess.Close()

	// Write input then close stdin so read() returns.
	go func() {
		time.Sleep(50 * time.Millisecond)
		_, _ = sess.Write([]byte("hello\n"))
	}()

	buf := make([]byte, 64)
	got := make([]byte, 0)
	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatalf("timeout waiting for output, got %q", string(got))
		default:
		}
		n, err := sess.Read(buf)
		if n > 0 {
			got = append(got, buf[:n]...)
		}
		if err != nil {
			break
		}
		if strings.Contains(string(got), "got:hello") {
			return
		}
	}
	if !strings.Contains(string(got), "got:hello") {
		t.Fatalf("expected 'got:hello' in output, got %q", string(got))
	}
}

// errIsPTY reports whether err wraps domain.ErrPTYFailed.
func errIsPTY(err error) bool {
	for err != nil {
		if errors.Is(err, domain.ErrPTYFailed) {
			return true
		}
		type unwrap interface{ Unwrap() error }
		if u, ok := err.(unwrap); ok {
			err = u.Unwrap()
			continue
		}
		break
	}
	return false
}

// Ensure the package compiles cleanly when /dev/tty is writable but no
// terminal is attached.
func TestReadPasswordFrom_NoFD(t *testing.T) {
	f, err := os.OpenFile("/dev/null", os.O_RDONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	// This will fail because /dev/null isn't a terminal, but it
	// exercises the function.
	_, _ = readPasswordFrom(f)
}
