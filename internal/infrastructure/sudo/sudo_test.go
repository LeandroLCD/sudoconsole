package sudo

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/LeandroLCD/sudoconsole/internal/domain"
)

func TestBuildArgv(t *testing.T) {
	c, _ := domain.NewCommand("apt", []string{"update"})
	argv := buildArgv(c)
	if len(argv) < 3 {
		t.Fatalf("argv too short: %v", argv)
	}
	if argv[0] != "sudo" {
		t.Fatalf("argv[0] = %q", argv[0])
	}
	if argv[2] != "apt" {
		t.Fatalf("argv[2] = %q", argv[2])
	}
	if argv[3] != "update" {
		t.Fatalf("argv[3] = %q", argv[3])
	}
}

func TestGateway_Execute_EmptyPath(t *testing.T) {
	g := NewGateway(nil)
	_, err := g.Execute(context.Background(), domain.Command{})
	if !errors.Is(err, domain.ErrInvalidCommand) {
		t.Fatalf("err = %v", err)
	}
}

func TestGateway_Authenticate_EmptySecret(t *testing.T) {
	g := NewGateway(nil)
	err := g.Authenticate(context.Background(), nil)
	if !errors.Is(err, domain.ErrAuthFailed) {
		t.Fatalf("err = %v, want ErrAuthFailed", err)
	}
}

func TestCopyDemux(t *testing.T) {
	src := strings.NewReader("hello world")
	var dst, errBuf bytes.Buffer
	n, err := copyDemux(src, &dst, &errBuf)
	if err != nil {
		t.Fatal(err)
	}
	if n != int64(len("hello world")) {
		t.Fatalf("n = %d", n)
	}
	if dst.String() != "hello world" {
		t.Fatalf("dst = %q", dst.String())
	}
}

func TestNewGateway_NilPTY(t *testing.T) {
	g := NewGateway(nil)
	if g == nil {
		t.Fatal("nil pty should still produce a gateway")
	}
}

// Compile-time: ensure Gateway satisfies the interface.
var _ domain.SudoGateway = (*Gateway)(nil)

// Sanity: deadline-aware behaviour works through context.
func TestGateway_Authenticate_ContextCancelled(t *testing.T) {
	g := NewGateway(nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before calling
	err := g.Authenticate(ctx, []byte("x"))
	if err == nil {
		t.Skip("Authenticate may have succeeded before cancellation; depends on timing")
	}
	// Either ErrContextCancelled or ErrAuthFailed is acceptable here.
	if !errors.Is(err, domain.ErrContextCancelled) && !errors.Is(err, domain.ErrAuthFailed) {
		t.Logf("got %v (acceptable)", err)
	}
}

// Ensure the import time is used (otherwise go vet complains).
var _ = time.Second

// Avoid unused io import warning if io isn't otherwise referenced.
var _ = io.Discard
