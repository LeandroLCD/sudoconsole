package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestNewCommand(t *testing.T) {
	c, err := NewCommand("apt", []string{"update"})
	if err != nil {
		t.Fatal(err)
	}
	if c.Path != "apt" || len(c.Args) != 1 || c.Args[0] != "update" {
		t.Fatalf("unexpected command: %+v", c)
	}
	if c.Raw != "apt update" {
		t.Fatalf("Raw = %q, want %q", c.Raw, "apt update")
	}
}

func TestNewCommand_EmptyPath(t *testing.T) {
	_, err := NewCommand("", nil)
	if !errors.Is(err, ErrInvalidCommand) {
		t.Fatalf("err = %v, want ErrInvalidCommand", err)
	}
}

func TestNewCommand_DefensiveCopy(t *testing.T) {
	args := []string{"update"}
	c, _ := NewCommand("apt", args)
	args[0] = "MUTATED"
	if c.Args[0] != "update" {
		t.Fatal("Args should be a defensive copy")
	}
}

func TestNewCommandFromRaw(t *testing.T) {
	c, err := NewCommandFromRaw("ssh user@host")
	if err != nil {
		t.Fatal(err)
	}
	if c.Path != "ssh" || c.Args[0] != "user@host" {
		t.Fatalf("unexpected: %+v", c)
	}
	if c.Raw != "ssh user@host" {
		t.Fatalf("Raw = %q", c.Raw)
	}
}

func TestNewCommandFromRaw_Empty(t *testing.T) {
	_, err := NewCommandFromRaw("   ")
	if !errors.Is(err, ErrInvalidCommand) {
		t.Fatalf("err = %v", err)
	}
	_, err = NewCommandFromRaw("")
	if !errors.Is(err, ErrInvalidCommand) {
		t.Fatalf("empty err = %v", err)
	}
}

func TestNewCommandFromRaw_PreservesDangerousPatterns(t *testing.T) {
	// The whole point: raw form is preserved for policy matching.
	c, err := NewCommandFromRaw("ssh -R 8080:localhost:80 user@host")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(c.Raw, "-R 8080") {
		t.Fatalf("Raw lost the reverse-tunnel flag: %q", c.Raw)
	}
}

func TestCommand_Basename(t *testing.T) {
	c := Command{Path: "/usr/bin/ssh"}
	if c.Basename() != "ssh" {
		t.Fatalf("Basename = %q", c.Basename())
	}
	c = Command{Path: "apt"}
	if c.Basename() != "apt" {
		t.Fatalf("Basename = %q", c.Basename())
	}
}

func TestCommand_Is(t *testing.T) {
	c, _ := NewCommand("/usr/bin/ssh", nil)
	if !c.Is("ssh") {
		t.Fatal("Is should match by basename")
	}
	if c.Is("scp") {
		t.Fatal("Is should not match other binaries")
	}
}

func TestCommand_Equals(t *testing.T) {
	a, _ := NewCommand("apt", []string{"update"})
	b, _ := NewCommand("apt", []string{"update"})
	if !a.Equals(b) {
		t.Fatal("identical commands should be equal")
	}
	c, _ := NewCommand("apt", []string{"upgrade"})
	if a.Equals(c) {
		t.Fatal("different args should not be equal")
	}
	d, _ := NewCommand("apt", []string{"update", "extra"})
	if a.Equals(d) {
		t.Fatal("different arg count should not be equal")
	}
	e, _ := NewCommand("dnf", []string{"update"})
	if a.Equals(e) {
		t.Fatal("different path should not be equal")
	}
}

func TestCommand_ContainsAny(t *testing.T) {
	c, _ := NewCommand("/usr/bin/ssh", []string{"-R", "8080:localhost:80", "user@host"})
	if !c.ContainsAny([]string{"-R", "-L"}) {
		t.Fatal("ContainsAny should find -R")
	}
	if c.ContainsAny([]string{"-X", "-Y"}) {
		t.Fatal("ContainsAny should not find -X/-Y")
	}
	if !c.ContainsPath([]string{"/usr/bin/ssh"}) {
		t.Fatal("ContainsPath should find path")
	}
	// Match in path even when no args match.
	if !c.ContainsAny([]string{"/usr/bin"}) {
		t.Fatal("ContainsAny should fall through to path check")
	}
	if c.ContainsPath([]string{"nothing"}) {
		t.Fatal("ContainsPath should not match unrelated substring")
	}
}

func TestCommand_Validate(t *testing.T) {
	c, _ := NewCommand("apt", nil)
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	bad := Command{}
	if err := bad.Validate(); !errors.Is(err, ErrInvalidCommand) {
		t.Fatalf("Validate empty = %v, want ErrInvalidCommand", err)
	}
}

func TestCommand_String(t *testing.T) {
	c, _ := NewCommand("ssh", []string{"-R", "8080 user@host"})
	if !strings.Contains(c.String(), "user@host") {
		t.Fatalf("String should quote args with spaces: %q", c.String())
	}
	c = Command{Path: "ls"}
	if !strings.Contains(c.String(), "ls") {
		t.Fatalf("String should include path even without args: %q", c.String())
	}
}

func TestCommand_CommandLine(t *testing.T) {
	c, _ := NewCommand("echo", []string{"hi"})
	if c.CommandLine() != "echo hi" {
		t.Fatalf("CommandLine = %q", c.CommandLine())
	}
	c.Raw = "echo hi  "
	if c.CommandLine() != "echo hi  " {
		t.Fatalf("CommandLine should preserve Raw verbatim: %q", c.CommandLine())
	}
}
