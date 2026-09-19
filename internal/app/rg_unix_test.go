//go:build unix

package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// waitFile polls for a fixture file on a bounded explicit condition.
func waitFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("fixture file never appeared: %s", path)
}

// readPID reads a one-pid-per-file fixture record.
func readPID(t *testing.T, path string) int {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("pid record %s: %v", path, err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		t.Fatalf("pid record %s = %q: %v", path, b, err)
	}
	return pid
}

// signal0 probes process or process-group existence without delivering
// a signal: nil means alive/present, ESRCH means gone.
func signal0(t *testing.T, pid int) error {
	t.Helper()
	return syscall.Kill(pid, 0)
}

// waitGone polls the probe until ESRCH — process death after SIGKILL is
// not atomic with the kill returning, so a member can still appear as a
// zombie for a scheduling quantum after the group signal lands.
func waitGone(t *testing.T, probe int) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		err := signal0(t, probe)
		if errors.Is(err, syscall.ESRCH) {
			return
		}
		if err != nil || time.Now().After(deadline) {
			t.Fatalf("pid/group %d still exists after Terminate: %v", probe, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// The spawned child heads its own process group, and Terminate reaches
// every member — a fake rg that leaves a subprocess holding the output
// pipes (a backgrounded sleep under its shell) still terminates
// completely: without the group kill the orphaned pipe holder keeps
// the pipes open and Wait would never return. The FAKE_RG_* variables
// are fixture-owned; the tested binary never reads them.
func TestTerminateKillsChildProcessGroup(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "pid")
	subFile := filepath.Join(dir, "subpid")
	ready := filepath.Join(dir, "ready")
	t.Setenv("FAKE_RG_PID_FILE", pidFile)
	t.Setenv("FAKE_RG_SUBPID_FILE", subFile)
	t.Setenv("FAKE_RG_READY_FILE", ready)
	writeFakeRG(t, `
echo $$ > "$FAKE_RG_PID_FILE"
sleep 600 &
echo $! > "$FAKE_RG_SUBPID_FILE"
: > "$FAKE_RG_READY_FILE"
wait
`)

	child, err := spawn(context.Background(), nil, t.TempDir())
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	waitFile(t, ready)
	pid := readPID(t, pidFile)
	sub := readPID(t, subFile)
	// The child leads its own group: the group and both members are
	// alive, so the post-termination ESRCH is proof, not vacuity.
	if err := signal0(t, pid); err != nil {
		t.Fatalf("child pid %d not alive before Terminate: %v", pid, err)
	}
	if err := signal0(t, -pid); err != nil {
		t.Fatalf("child is not a process-group leader (pgid %d): %v", pid, err)
	}
	if err := signal0(t, sub); err != nil {
		t.Fatalf("grandchild pid %d not alive before Terminate: %v", sub, err)
	}

	child.Terminate()
	res := waitResult(t, child)
	if res.Code != -1 {
		t.Fatalf("terminated child result = code %d, want -1 (signal death)", res.Code)
	}
	for _, probe := range []int{pid, sub, -pid} {
		waitGone(t, probe)
	}
}
