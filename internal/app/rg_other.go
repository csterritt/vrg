//go:build !unix

package app

import (
	"errors"
	"os"
	"syscall"
)

// errProcGroupGone stands in for ESRCH on platforms without unix process
// groups: a finished direct child is the only "group already gone"
// condition the fallback can observe.
var errProcGroupGone = os.ErrProcessDone

// procGroupAttr is nil off unix: there is no portable process-group
// separation to request.
func procGroupAttr() *syscall.SysProcAttr {
	return nil
}

// killProcGroup falls back to terminating the direct child — the same
// scope Terminate had before group termination existed.
func killProcGroup(p *os.Process) error {
	if err := p.Kill(); err != nil {
		if errors.Is(err, os.ErrProcessDone) {
			return errProcGroupGone
		}
		return err
	}
	return nil
}
