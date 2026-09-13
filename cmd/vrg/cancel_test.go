package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/creack/pty"
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

// runVrgCancel starts vrg under a PTY, waits for readyFile to appear,
// sends key (a string written to the PTY), and returns the raw PTY
// output, stderr, and exit code. It also captures termios before and
// after the run and returns them for comparison.
func runVrgCancel(t *testing.T, cmd *exec.Cmd, readyFile string, key string) ptyCancelResult {
	t.Helper()
	var se bytes.Buffer
	cmd.Stderr = &se

	ptmx, ptmxErr := pty.Start(cmd)
	if ptmxErr != nil {
		t.Fatalf("failed to start vrg with PTY: %v", ptmxErr)
	}
	defer func() { _ = ptmx.Close() }()

	if err := pty.Setsize(ptmx, &pty.Winsize{Rows: 24, Cols: 80}); err != nil {
		t.Fatalf("failed to set PTY window size: %v", err)
	}

	// Capture termios before vrg starts its TUI.
	beforeTermios := getTermios(t, int(ptmx.Fd()))

	// Read PTY output concurrently to prevent buffer deadlocks.
	var so bytes.Buffer
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		io.Copy(&so, ptmx)
	}()

	// Wait for the fake rg to signal ready, then send the key.
	go func() {
		if readyFile != "" {
			waitForFile(t, readyFile, 15*time.Second)
		}
		io.WriteString(ptmx, key)
	}()

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		afterTermios := getTermios(t, int(ptmx.Fd()))
		_ = ptmx.Close()
		<-readDone
		if !termiosEqual(beforeTermios, afterTermios) {
			t.Errorf("termios not restored after exit\nbefore: %+v\nafter:  %+v", beforeTermios, afterTermios)
		}
		if err == nil {
			return ptyCancelResult{rawOutput: so.String(), stderr: se.String(), exitCode: 0}
		}
		if ee, ok := err.(*exec.ExitError); ok {
			return ptyCancelResult{rawOutput: so.String(), stderr: se.String(), exitCode: ee.ExitCode()}
		}
		t.Fatalf("vrg failed: %v (stderr %q)", err, se.String())
	case <-time.After(30 * time.Second):
		cmd.Process.Kill()
		t.Fatal("vrg did not complete within 30 seconds")
	}
	return ptyCancelResult{}
}

// writeBlockedFakeRG writes a fake rg that touches readyFile and pidFile,
// then blocks indefinitely. When killed, it exits.
func writeBlockedFakeRG(t *testing.T, dir, readyFile, pidFile string) string {
	t.Helper()
	rgPath := filepath.Join(dir, "rg")
	script := `#!/bin/sh
# Write argv and cwd for test verification
if [ -n "$VRG_TEST_ARGV" ]; then
	printf '%s\n' "$*" > "$VRG_TEST_ARGV"
fi
if [ -n "$VRG_TEST_CWD" ]; then
	pwd > "$VRG_TEST_CWD"
fi
if [ -n "$VRG_TEST_PID" ]; then
	echo $$ > "$VRG_TEST_PID"
fi
if [ -n "$VRG_TEST_READY" ]; then
	touch "$VRG_TEST_READY"
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
if [ -n "$VRG_TEST_PID" ]; then
	echo $$ > "$VRG_TEST_PID"
fi
if [ -n "$VRG_TEST_READY" ]; then
	touch "$VRG_TEST_READY"
fi
echo '{"type":"begin","data":{"path":{"text":"test.txt"}}}'
printf '%s\n' '{"type":"match","data":{"path":{"text":"test.txt"},"lines":{"text":"hello world\n"},"line_number":1,"submatches":[{"match":{"text":"hello"},"start":0,"end":5}]}}'
echo '{"type":"end","data":{"path":{"text":"test.txt"},"binary_offset":null}}'
echo '{"type":"summary","data":{}}'
if [ -n "$VRG_TEST_HANDSHAKE" ]; then
	touch "$VRG_TEST_HANDSHAKE"
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
		"VRG_TEST_READY=" + readyFile,
		"VRG_TEST_PID=" + pidFile,
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
		"VRG_TEST_READY=" + readyFile,
		"VRG_TEST_PID=" + pidFile,
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
	handshakeFile := filepath.Join(t.TempDir(), "handshake")
	writeCompleteFakeRG(t, fakeDir, readyFile, pidFile, handshakeFile)

	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "test.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(binPath, "hello", ".")
	cmd.Dir = repo
	cmd.Env = []string{
		"PATH=" + fakeDir + ":" + os.Getenv("PATH"),
		"VRG_TEST_READY=" + readyFile,
		"VRG_TEST_PID=" + pidFile,
		"VRG_TEST_REAP=" + reapFile,
		"VRG_TEST_HANDSHAKE=" + handshakeFile,
	}
	// Wait for the completion handshake (rg done), then send q from summary.
	res := runVrgCancel(t, cmd, handshakeFile, "q")

	if res.exitCode != 0 {
		t.Fatalf("vrg exited %d, want 0 (normal quit from summary)", res.exitCode)
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
		"VRG_TEST_READY=" + readyFile,
		"VRG_TEST_PID=" + pidFile,
		"VRG_TEST_REAP=" + reapFile,
		"VRG_TEST_HANDSHAKE=" + handshakeFile,
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
		"VRG_TEST_READY=" + readyFile,
		"VRG_TEST_PID=" + pidFile,
		"VRG_TEST_REAP=" + reapFile,
		"VRG_TEST_FAIL_TRIGGER=" + failTrigger,
		"VRG_TEST_FAIL_DIAGNOSTIC=controlled failure for test",
	}

	// Start vrg under a PTY and trigger the failure after the child
	// signals ready.
	var se bytes.Buffer
	cmd.Stderr = &se

	ptmx, ptmxErr := pty.Start(cmd)
	if ptmxErr != nil {
		t.Fatalf("failed to start vrg with PTY: %v", ptmxErr)
	}
	defer func() { _ = ptmx.Close() }()

	if err := pty.Setsize(ptmx, &pty.Winsize{Rows: 24, Cols: 80}); err != nil {
		t.Fatalf("failed to set PTY window size: %v", err)
	}

	beforeTermios := getTermios(t, int(ptmx.Fd()))

	var so bytes.Buffer
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		io.Copy(&so, ptmx)
	}()

	// Wait for the child to signal ready, then trigger the failure.
	go func() {
		waitForFile(t, readyFile, 15*time.Second)
		if err := os.WriteFile(failTrigger, []byte("trigger"), 0o644); err != nil {
			t.Errorf("cannot write fail trigger: %v", err)
		}
	}()

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		afterTermios := getTermios(t, int(ptmx.Fd()))
		_ = ptmx.Close()
		<-readDone
		if !termiosEqual(beforeTermios, afterTermios) {
			t.Errorf("termios not restored after controlled failure\nbefore: %+v\nafter:  %+v", beforeTermios, afterTermios)
		}

		var exitCode int
		if err == nil {
			exitCode = 0
		} else if ee, ok := err.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
		} else {
			t.Fatalf("vrg failed: %v", err)
		}

		if exitCode != 2 {
			t.Fatalf("vrg exited %d, want 2 (controlled failure)", exitCode)
		}

		assertChildGone(t, pidFile)
		assertReapEvidence(t, reapFile)
		assertDisplayRestoration(t, so.String())

		diag := se.String()
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
	case <-time.After(30 * time.Second):
		cmd.Process.Kill()
		t.Fatal("vrg did not complete within 30 seconds")
	}
}
