package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// waitForFile polls for file to appear up to timeout. It calls t.Fatal
// if the file does not appear in time.
func waitForFile(t *testing.T, file string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(file); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("file %s did not appear within %s", file, timeout)
}

// getTermios returns the termios of the given file descriptor.
func getTermios(t *testing.T, fd int) unix.Termios {
	t.Helper()
	tio, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		t.Fatalf("IoctlGetTermios: %v", err)
	}
	return *tio
}

// termiosEqual reports whether two termios structs are equal.
func termiosEqual(a, b unix.Termios) bool {
	return a.Iflag == b.Iflag &&
		a.Oflag == b.Oflag &&
		a.Cflag == b.Cflag &&
		a.Lflag == b.Lflag &&
		a.Line == b.Line &&
		a.Cc == b.Cc &&
		a.Ispeed == b.Ispeed &&
		a.Ospeed == b.Ospeed
}

// pidAlive reports whether a process with the given PID exists (signal 0
// succeeds). A reaped or never-started process returns false.
func pidAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

// ptyCancelResult holds the result of a PTY cancellation test run.
type ptyCancelResult struct {
	rawOutput string
	stderr    string
	exitCode  int
}

// runVrgCancel starts vrg under a PTY through the shared Issue #48
// driver, waits for readyFile to appear (a fixture-side bounded poll
// proving the child reached the awaited point), then sends key. The
// send needs no intra-run acknowledgement — nothing follows it and
// the asserted exit code proves the key was processed in the awaited
// state — so the run wires no acknowledgement log. It returns the
// raw PTY output, stderr, and exit code; termios restoration is
// asserted inside runVrgPTY.
func runVrgCancel(t *testing.T, cmd *exec.Cmd, readyFile string, key string) ptyCancelResult {
	t.Helper()
	res := runVrgPTY(t, cmd, "", func(d *ptyDriver) {
		if readyFile != "" {
			waitForFile(t, readyFile, 15*time.Second)
		}
		d.sendKey(t, key)
	})
	return ptyCancelResult{rawOutput: res.raw, stderr: res.stderr, exitCode: res.exitCode}
}

// writeBlockedFakeRG writes a fake rg that touches readyFile and pidFile,
// then blocks indefinitely. When killed, it exits.
func writeBlockedFakeRG(t *testing.T, dir, readyFile, pidFile string) string {
	t.Helper()
	rgPath := filepath.Join(dir, "rg")
	script := `#!/bin/sh
# Write argv and cwd for test verification
if [ -n "$FAKE_RG_ARGV_FILE" ]; then
	printf '%s\n' "$*" > "$FAKE_RG_ARGV_FILE"
fi
if [ -n "$FAKE_RG_CWD_FILE" ]; then
	pwd > "$FAKE_RG_CWD_FILE"
fi
if [ -n "$FAKE_RG_PID_FILE" ]; then
	echo $$ > "$FAKE_RG_PID_FILE"
fi
if [ -n "$FAKE_RG_READY_FILE" ]; then
	touch "$FAKE_RG_READY_FILE"
fi
# Block indefinitely until killed.
sleep 100000
`
	if err := os.WriteFile(rgPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return rgPath
}

// writeCompleteFakeRG writes a fake rg that outputs a complete valid
// ripgrep JSON stream, touches readyFile and pidFile, then exits 0.
// The completion handshake file is touched after the stream is done.
func writeCompleteFakeRG(t *testing.T, dir, readyFile, pidFile, completionFile string) string {
	t.Helper()
	rgPath := filepath.Join(dir, "rg")
	script := `#!/bin/sh
if [ -n "$FAKE_RG_PID_FILE" ]; then
	echo $$ > "$FAKE_RG_PID_FILE"
fi
if [ -n "$FAKE_RG_READY_FILE" ]; then
	touch "$FAKE_RG_READY_FILE"
fi
echo '{"type":"begin","data":{"path":{"text":"test.txt"}}}'
printf '%s\n' '{"type":"match","data":{"path":{"text":"test.txt"},"lines":{"text":"hello world\n"},"line_number":1,"submatches":[{"match":{"text":"hello"},"start":0,"end":5}]}}'
echo '{"type":"end","data":{"path":{"text":"test.txt"},"binary_offset":null}}'
echo '{"type":"summary","data":{}}'
if [ -n "$FAKE_RG_HANDSHAKE_FILE" ]; then
	touch "$FAKE_RG_HANDSHAKE_FILE"
fi
exit 0
`
	if err := os.WriteFile(rgPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return rgPath
}

// assertReapEvidence requires the reap evidence file to exist and be
// non-empty, proving vrg's Wait/reap path ran.
func assertReapEvidence(t *testing.T, reapFile string) {
	t.Helper()
	data, err := os.ReadFile(reapFile)
	if err != nil {
		t.Fatalf("reap evidence file %s missing: %v (vrg did not Wait on the child)", reapFile, err)
	}
	if len(data) == 0 {
		t.Fatalf("reap evidence file %s is empty", reapFile)
	}
}

// assertChildGone requires the child PID to be no longer alive.
func assertChildGone(t *testing.T, pidFile string) {
	t.Helper()
	data, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("pid file %s missing: %v", pidFile, err)
	}
	var pid int
	if _, err := fmt.Sscanf(strings.TrimSpace(string(data)), "%d", &pid); err != nil {
		t.Fatalf("cannot parse pid from %q: %v", data, err)
	}
	if pidAlive(pid) {
		t.Fatalf("child process %d is still alive (orphaned)", pid)
	}
}

// assertDisplayRestoration requires the raw PTY output to contain the
// cursor-show sequence and, if alt screen was entered, the alt-screen
// exit sequence.
func assertDisplayRestoration(t *testing.T, raw string) {
	t.Helper()
	if !strings.Contains(raw, "\x1b[?25h") {
		t.Errorf("raw output does not contain cursor-show sequence (\\x1b[?25h): %q", raw)
	}
	if strings.Contains(raw, "\x1b[?1049h") {
		if !strings.Contains(raw, "\x1b[?1049l") {
			t.Errorf("alt screen was entered but not exited: %q", raw)
		}
	}
}

// TestQAgainstBlockedFakeRGExits130 verifies that pressing q while the
// fake rg is blocked (still running) exits 130, terminates and reaps
// the child, restores the display, and restores PTY termios.
func TestQAgainstBlockedFakeRGExits130(t *testing.T) {
	fakeDir := t.TempDir()
	readyFile := filepath.Join(t.TempDir(), "ready")
	pidFile := filepath.Join(t.TempDir(), "pid")
	reapFile := filepath.Join(t.TempDir(), "reap")
	writeBlockedFakeRG(t, fakeDir, readyFile, pidFile)

	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "test.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(binPath, "hello", ".")
	cmd.Dir = repo
	cmd.Env = []string{
		"PATH=" + fakeDir + ":" + os.Getenv("PATH"),
		"FAKE_RG_READY_FILE=" + readyFile,
		"FAKE_RG_PID_FILE=" + pidFile,
		"VRG_TEST_REAP=" + reapFile,
	}
	res := runVrgCancel(t, cmd, readyFile, "q")

	if res.exitCode != 130 {
		t.Fatalf("vrg exited %d, want 130", res.exitCode)
	}
	assertChildGone(t, pidFile)
	assertReapEvidence(t, reapFile)
	assertDisplayRestoration(t, res.rawOutput)
}

// TestCtrlCAgainstBlockedFakeRGExits130 verifies that ctrl+c while the
// fake rg is blocked exits 130, terminates and reaps the child,
// restores the display, and restores PTY termios.
func TestCtrlCAgainstBlockedFakeRGExits130(t *testing.T) {
	fakeDir := t.TempDir()
	readyFile := filepath.Join(t.TempDir(), "ready")
	pidFile := filepath.Join(t.TempDir(), "pid")
	reapFile := filepath.Join(t.TempDir(), "reap")
	writeBlockedFakeRG(t, fakeDir, readyFile, pidFile)

	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "test.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(binPath, "hello", ".")
	cmd.Dir = repo
	cmd.Env = []string{
		"PATH=" + fakeDir + ":" + os.Getenv("PATH"),
		"FAKE_RG_READY_FILE=" + readyFile,
		"FAKE_RG_PID_FILE=" + pidFile,
		"VRG_TEST_REAP=" + reapFile,
	}
	// ctrl+c is byte 0x03.
	res := runVrgCancel(t, cmd, readyFile, "\x03")

	if res.exitCode != 130 {
		t.Fatalf("vrg exited %d, want 130", res.exitCode)
	}
	assertChildGone(t, pidFile)
	assertReapEvidence(t, reapFile)
	assertDisplayRestoration(t, res.rawOutput)
}

// TestNormalExitReapsChild verifies that a normal exit while rg is still
// running leaves no orphaned or unreaped child. The fake rg completes
// its stream and exits; vrg shows the summary; q exits 0. The child
// must be reaped (reap evidence present) and gone.
func TestNormalExitReapsChild(t *testing.T) {
	fakeDir := t.TempDir()
	readyFile := filepath.Join(t.TempDir(), "ready")
	pidFile := filepath.Join(t.TempDir(), "pid")
	reapFile := filepath.Join(t.TempDir(), "reap")
	writeCompleteFakeRG(t, fakeDir, readyFile, pidFile, "")

	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "test.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ackFile := filepath.Join(t.TempDir(), "update-ack")
	cmd := exec.Command(binPath, "hello", ".")
	cmd.Dir = repo
	cmd.Env = []string{
		"PATH=" + fakeDir + ":" + os.Getenv("PATH"),
		"FAKE_RG_READY_FILE=" + readyFile,
		"FAKE_RG_PID_FILE=" + pidFile,
		"VRG_TEST_REAP=" + reapFile,
		"VRG_TEST_UPDATE_ACK=" + ackFile,
	}
	// Wait for the search-complete acknowledgement — proving the
	// model processed SearchCompleteMsg out of searching — then send
	// q. Waiting on the fixture handshake alone would race the model
	// transition (q during searching exits 130, not 0).
	_, _, exitCode := runVrgWithQuit(t, cmd, ackFile)

	if exitCode != 0 {
		t.Fatalf("vrg exited %d, want 0 (normal quit from summary)", exitCode)
	}
	assertChildGone(t, pidFile)
	assertReapEvidence(t, reapFile)
}

// TestQDuringGateHeldPreparationExits130 verifies that q pressed after
// rg has exited but while index preparation is gate-held exits 130
// (cancellation), not a browse quit. The fake rg completes its stream
// and exits; the gate holds index preparation; q exits 130.
func TestQDuringGateHeldPreparationExits130(t *testing.T) {
	fakeDir := t.TempDir()
	readyFile := filepath.Join(t.TempDir(), "ready")
	pidFile := filepath.Join(t.TempDir(), "pid")
	reapFile := filepath.Join(t.TempDir(), "reap")
	handshakeFile := filepath.Join(t.TempDir(), "handshake")
	gateFile := filepath.Join(t.TempDir(), "gate")
	writeCompleteFakeRG(t, fakeDir, readyFile, pidFile, handshakeFile)

	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "test.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(binPath, "hello", ".")
	cmd.Dir = repo
	cmd.Env = []string{
		"PATH=" + fakeDir + ":" + os.Getenv("PATH"),
		"FAKE_RG_READY_FILE=" + readyFile,
		"FAKE_RG_PID_FILE=" + pidFile,
		"VRG_TEST_REAP=" + reapFile,
		"FAKE_RG_HANDSHAKE_FILE=" + handshakeFile,
		"VRG_TEST_GATE=" + gateFile,
	}
	// Wait for the completion handshake (rg done, gate held), then send q.
	res := runVrgCancel(t, cmd, handshakeFile, "q")

	if res.exitCode != 130 {
		t.Fatalf("vrg exited %d, want 130 (cancellation during gate-held preparation)", res.exitCode)
	}
	assertChildGone(t, pidFile)
	assertReapEvidence(t, reapFile)
}

// TestInjectedControlledFailure verifies that an injected controlled
// failure after the child has signalled ready terminates and reaps the
// child, restores the terminal, writes a sanitized diagnostic to stderr
// exactly once after terminal restoration, and exits 2.
func TestInjectedControlledFailure(t *testing.T) {
	fakeDir := t.TempDir()
	readyFile := filepath.Join(t.TempDir(), "ready")
	pidFile := filepath.Join(t.TempDir(), "pid")
	reapFile := filepath.Join(t.TempDir(), "reap")
	failTrigger := filepath.Join(t.TempDir(), "trigger")
	writeBlockedFakeRG(t, fakeDir, readyFile, pidFile)

	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "test.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(binPath, "hello", ".")
	cmd.Dir = repo
	cmd.Env = []string{
		"PATH=" + fakeDir + ":" + os.Getenv("PATH"),
		"FAKE_RG_READY_FILE=" + readyFile,
		"FAKE_RG_PID_FILE=" + pidFile,
		"VRG_TEST_REAP=" + reapFile,
		"VRG_TEST_FAIL_TRIGGER=" + failTrigger,
		"VRG_TEST_FAIL_DIAGNOSTIC=controlled failure for test",
	}

	// Start vrg under a PTY through the shared Issue #48 driver and
	// trigger the failure after the child signals ready (a
	// fixture-side bounded poll; no model transition is assumed).
	res := runVrgPTY(t, cmd, "", func(d *ptyDriver) {
		waitForFile(t, readyFile, 15*time.Second)
		if err := os.WriteFile(failTrigger, []byte("trigger"), 0o644); err != nil {
			t.Errorf("cannot write fail trigger: %v", err)
		}
	})

	if res.exitCode != 2 {
		t.Fatalf("vrg exited %d, want 2 (controlled failure)", res.exitCode)
	}

	assertChildGone(t, pidFile)
	assertReapEvidence(t, reapFile)
	assertDisplayRestoration(t, res.raw)

	diag := res.stderr
	if diag == "" {
		t.Fatal("stderr is empty, want a diagnostic")
	}
	if strings.ContainsAny(diag, "\x1b\x9b") {
		t.Fatalf("stderr contains raw control bytes: %q", diag)
	}
	if !strings.Contains(diag, "controlled failure for test") {
		t.Fatalf("stderr does not contain the expected diagnostic: %q", diag)
	}
	// The diagnostic must appear exactly once.
	if count := strings.Count(diag, "controlled failure for test"); count != 1 {
		t.Fatalf("diagnostic appears %d times in stderr, want 1: %q", count, diag)
	}
}
