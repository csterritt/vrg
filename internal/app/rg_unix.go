//go:build unix

package app

import (
	"os"
	"syscall"
)

// errProcGroupGone marks "no such process group" — the group the child
// headed is already empty, so there is nothing left to terminate. The
// CommandContext Cancel adapter maps it to os.ErrProcessDone.
var errProcGroupGone = syscall.ESRCH

// procGroupAttr makes the spawned child a process-group leader: its own
// pid is the group id every subprocess it spawns inherits, so the whole
// tree is addressable by one negative pid. The group must be separate —
// the child shares vrg's process group otherwise, and a group signal
// would reach vrg itself.
func procGroupAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}

// killProcGroup SIGKILLs the process group the child heads — the child
// and every member still holding the output pipes — so neither can
// evade termination. ESRCH (the group already empty) is the no-op.
func killProcGroup(p *os.Process) error {
	return syscall.Kill(-p.Pid, syscall.SIGKILL)
}
