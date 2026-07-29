//go:build darwin

package pty

import (
	"os/exec"
)

// disableCoreDumpSyscall on macOS is a no-op for now. M12 will revisit.
func disableCoreDumpSyscall(cmd *exec.Cmd) {
	_ = cmd
}
