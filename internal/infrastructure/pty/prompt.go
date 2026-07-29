//go:build linux || darwin

package pty

import (
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/term"
)

// readPasswordFrom reads one line from f with echo disabled and returns
// the bytes (without trailing newline).
//
// Uses golang.org/x/term.ReadPassword which handles all termios dance.
func readPasswordFrom(f *os.File) ([]byte, error) {
	return term.ReadPassword(int(f.Fd()))
}

// disableCoreDump sets RLIMIT_CORE=0 on the child process. Called before
// exec so credentials cannot leak via coredumps.
func disableCoreDump(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	disableCoreDumpSyscall(cmd)
}
