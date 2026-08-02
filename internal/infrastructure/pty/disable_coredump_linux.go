//go:build linux

package pty

import (
	"os/exec"

	"golang.org/x/sys/unix"
)

// disableCoreDumpSyscall on Linux sets RLIMIT_CORE=0 on the calling
// process via prlimit(2). Because Unix resource limits are inherited
// by child processes across execve, sudo (and any other subprocess
// spawned via cmd.Start) will also have RLIMIT_CORE=0 — preventing
// any password bytes that might be sitting in process memory from
// being written to disk via a coredump.
//
// We mutate the parent rather than the child because Go's
// os/exec.Cmd provides no portable way to invoke prlimit on the
// post-fork / pre-exec child on Linux.
//
// PR_SET_DUMPABLE is set to 0 in addition as defense-in-depth: even
// if some descendant tries to re-enable coredumps, the kernel will
// refuse because the process is no longer "dumpable".
func disableCoreDumpSyscall(_ *exec.Cmd) {
	zero := unix.Rlimit{Cur: 0, Max: 0}
	// Ignore errors: the call may legitimately fail in containers
	// without CAP_SYS_RESOURCE. The parent process will simply keep
	// the existing limit, which is at least better than crashing
	// the whole auth flow.
	_ = unix.Prlimit(0, unix.RLIMIT_CORE, &zero, nil)
	_ = unix.Prctl(unix.PR_SET_DUMPABLE, 0, 0, 0, 0)
}
