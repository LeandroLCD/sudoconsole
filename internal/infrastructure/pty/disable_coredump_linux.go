//go:build linux

package pty

import (
	"os/exec"
	"syscall"
)

// disableCoreDumpSyscall on Linux is a no-op for now. Setting RLIMIT_CORE
// on the child after fork but before exec requires either a fork+exec
// helper or the SysProcAttr.Cmd field (Go 1.25+). M12 will revisit.
func disableCoreDumpSyscall(cmd *exec.Cmd) {
	_ = cmd
	_ = syscall.SysProcAttr{}
}
