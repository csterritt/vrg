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
)

// runVrgWithKeys starts vrg under a PTY, waits for handshakeFile to
// appear, then sends the given keys in sequence (with a small delay
// between each so the TUI processes them), and returns the stripped
// stdout, stderr, and exit code.
func runVrgWithKeys(t *testing.T, cmd *exec.Cmd, handshake string, keys ...string) (stdout, stderr string, exitCode int) {
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

	var so bytes.Buffer
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		io.Copy(&so, ptmx)
	}()

	// Wait for the handshake, then send keys in sequence.
	go func() {
		if handshake != "" {
			for i := 0; i < 1000; i++ {
				if _, err := os.Stat(handshake); err == nil {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
		}
		for _, k := range keys {
			time.Sleep(100 * time.Millisecond)
			io.WriteString(ptmx, k)
		}
	}()

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		_ = ptmx.Close()
		<-readDone
		if err == nil {
			return stripAnsi(so.String()), se.String(), 0
		}
		if ee, ok := err.(*exec.ExitError); ok {
			return stripAnsi(so.String()), se.String(), ee.ExitCode()
		}
		t.Fatalf("vrg failed: %v (stderr %q)", err, se.String())
	case <-time.After(30 * time.Second):
		cmd.Process.Kill()
		t.Fatal("vrg did not complete within 30 seconds")
	}
	return "", se.String(), -1
}

// runVrgKillChild starts vrg under a PTY, waits for readyFile, then
// sends SIGKILL to the fake rg (whose PID is in pidFile), waits for the
// child to die, then sends the given keys and returns the result.
func runVrgKillChild(t *testing.T, cmd *exec.Cmd, readyFile, pidFile string, keys ...string) (stdout, stderr string, exitCode int) {
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

	var so bytes.Buffer
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		io.Copy(&so, ptmx)
	}()

	// Wait for ready, then kill the fake rg, then send keys.
	go func() {
		waitForFile(t, readyFile, 15*time.Second)
		// Read the fake rg PID and kill its entire process group
		// (the shell script's sleep child inherits the stdout pipe
		// and would keep it open if only the shell is killed).
		data, err := os.ReadFile(pidFile)
		if err != nil {
			t.Errorf("cannot read pid file: %v", err)
			return
		}
		var pid int
		fmt.Sscanf(strings.TrimSpace(string(data)), "%d", &pid)
		// SIGKILL the entire process group.
		_ = syscall.Kill(-pid, syscall.SIGKILL)
		for _, k := range keys {
			time.Sleep(500 * time.Millisecond)
			io.WriteString(ptmx, k)
		}
	}()

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		_ = ptmx.Close()
		<-readDone
		if err == nil {
			return stripAnsi(so.String()), se.String(), 0
		}
		if ee, ok := err.(*exec.ExitError); ok {
			return stripAnsi(so.String()), se.String(), ee.ExitCode()
		}
		t.Fatalf("vrg failed: %v (stderr %q)", err, se.String())
	case <-time.After(30 * time.Second):
		cmd.Process.Kill()
		t.Fatal("vrg did not complete within 30 seconds")
	}
	return "", se.String(), -1
}

// writeFatalFakeRG writes a fake rg that emits the given stdout records
// (joined by newlines), writes stderrText to stderr, then exits with
// exitCode. A handshake file is touched after both pipes are done.
func writeFatalFakeRG(t *testing.T, dir string, stdoutRecords []string, stderrText string, exitCode int) string {
	t.Helper()
	rgPath := filepath.Join(dir, "rg")
	stdoutPart := strings.Join(stdoutRecords, "\n")
	// Build the script. Use printf for the stdout to avoid echo
	// interpretation, and write stderr before exit.
	script := "#!/bin/sh\n"
	if stderrText != "" {
		// Write stderr text using printf to avoid shell interpretation.
		script += "printf '%s\\n' " + shellQuote(stderrText) + " >&2\n"
	}
	if stdoutPart != "" {
		script += "printf '%s\\n' " + shellQuote(stdoutPart) + "\n"
	}
	script += "if [ -n \"$VRG_TEST_HANDSHAKE\" ]; then touch \"$VRG_TEST_HANDSHAKE\"; fi\n"
	script += "exit " + itoa(exitCode) + "\n"
	if err := os.WriteFile(rgPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return rgPath
}

// shellQuote single-quotes a string for safe shell embedding.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// itoa returns the decimal string of n.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// TestFatalExitWithResultsShowsOverlay verifies that a fake rg that
// emits two valid matches then exits non-zero with stderr shows the
// browse view with an error overlay containing the stderr text.
// Dismissal with Esc returns to browse; q exits 2.
func TestFatalExitWithResultsShowsOverlay(t *testing.T) {
	fakeDir := t.TempDir()
	records := []string{
		`{"type":"begin","data":{"path":{"text":"test.txt"}}}`,
		`{"type":"match","data":{"path":{"text":"test.txt"},"lines":{"text":"hello world\n"},"line_number":1,"submatches":[{"match":{"text":"hello"},"start":0,"end":5}]}}`,
		`{"type":"end","data":{"path":{"text":"test.txt"},"binary_offset":null}}`,
		`{"type":"summary","data":{}}`,
	}
	writeFatalFakeRG(t, fakeDir, records, "boom", 3)

	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "test.txt"), []byte("hello world\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	handshakeFile := filepath.Join(t.TempDir(), "handshake")
	cmd := exec.Command(binPath, "hello", ".")
	cmd.Dir = repo
	cmd.Env = []string{
		"PATH=" + fakeDir + ":" + os.Getenv("PATH"),
		"VRG_TEST_HANDSHAKE=" + handshakeFile,
	}
	// Send Esc to dismiss the overlay, then q to quit.
	stdout, _, exitCode := runVrgWithKeys(t, cmd, handshakeFile, "\x1b", "q")

	if exitCode != 2 {
		t.Fatalf("vrg exited %d, want 2 (fatal)", exitCode)
	}
	// The overlay must contain the stderr text "boom".
	if !strings.Contains(stdout, "boom") {
		t.Fatalf("vrg stdout does not contain the stderr diagnostic 'boom': %q", stdout)
	}
	// The browse view must be present (the matched file appears).
	if !strings.Contains(stdout, "test.txt") {
		t.Fatalf("vrg stdout does not contain the browse view: %q", stdout)
	}
}

// TestFatalExitNoOutputNamesExitCode verifies that a fake rg that exits
// non-zero with no output produces an overlay naming the exit code.
// q exits 2; Esc also exits 2.
func TestFatalExitNoOutputNamesExitCode(t *testing.T) {
	fakeDir := t.TempDir()
	writeFatalFakeRG(t, fakeDir, nil, "", 2)

	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "test.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	handshakeFile := filepath.Join(t.TempDir(), "handshake")
	cmd := exec.Command(binPath, "hello", ".")
	cmd.Dir = repo
	cmd.Env = []string{
		"PATH=" + fakeDir + ":" + os.Getenv("PATH"),
		"VRG_TEST_HANDSHAKE=" + handshakeFile,
	}
	stdout, _, exitCode := runVrgWithKeys(t, cmd, handshakeFile, "q")

	if exitCode != 2 {
		t.Fatalf("vrg exited %d, want 2 (fatal no output)", exitCode)
	}
	// The overlay must name the exit code 2.
	if !strings.Contains(stdout, "2") {
		t.Fatalf("vrg stdout does not name exit code 2: %q", stdout)
	}
}

// TestFatalExitNoOutputEscExits2 verifies that Esc on the fatal
// no-results overlay also exits 2.
func TestFatalExitNoOutputEscExits2(t *testing.T) {
	fakeDir := t.TempDir()
	writeFatalFakeRG(t, fakeDir, nil, "", 2)

	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "test.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	handshakeFile := filepath.Join(t.TempDir(), "handshake")
	cmd := exec.Command(binPath, "hello", ".")
	cmd.Dir = repo
	cmd.Env = []string{
		"PATH=" + fakeDir + ":" + os.Getenv("PATH"),
		"VRG_TEST_HANDSHAKE=" + handshakeFile,
	}
	_, _, exitCode := runVrgWithKeys(t, cmd, handshakeFile, "\x1b")

	if exitCode != 2 {
		t.Fatalf("vrg exited %d, want 2 (Esc on fatal no-results overlay)", exitCode)
	}
}

// TestSignalDeathNamesSignal verifies that a fake rg killed by SIGKILL
// mid-stream produces an overlay naming the signal.
func TestSignalDeathNamesSignal(t *testing.T) {
	fakeDir := t.TempDir()
	rgPath := filepath.Join(fakeDir, "rg")
	// The fake rg writes a partial stream (no summary) then blocks,
	// waiting to be killed. vrg should detect the signal death.
	script := `#!/bin/sh
if [ -n "$VRG_TEST_PID" ]; then echo $$ > "$VRG_TEST_PID"; fi
echo '{"type":"begin","data":{"path":{"text":"test.txt"}}}'
printf '%s\n' '{"type":"match","data":{"path":{"text":"test.txt"},"lines":{"text":"hello\n"},"line_number":1,"submatches":[{"match":{"text":"hello"},"start":0,"end":5}]}}'
if [ -n "$VRG_TEST_READY" ]; then touch "$VRG_TEST_READY"; fi
# Block until killed.
sleep 100000
`
	if err := os.WriteFile(rgPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "test.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	readyFile := filepath.Join(t.TempDir(), "ready")
	pidFile := filepath.Join(t.TempDir(), "pid")
	cmd := exec.Command(binPath, "hello", ".")
	cmd.Dir = repo
	cmd.Env = []string{
		"PATH=" + fakeDir + ":" + os.Getenv("PATH"),
		"VRG_TEST_READY=" + readyFile,
		"VRG_TEST_PID=" + pidFile,
	}

	// Start vrg under a PTY, wait for ready, then SIGKILL the fake rg.
	stdout, _, exitCode := runVrgKillChild(t, cmd, readyFile, pidFile, "q", "q")

	if exitCode != 2 {
		t.Fatalf("vrg exited %d, want 2 (signal death)", exitCode)
	}
	// The overlay must name the signal (SIGKILL or signal 9).
	lowered := strings.ToLower(stdout)
	if !strings.Contains(lowered, "signal") && !strings.Contains(stdout, "9") {
		t.Fatalf("vrg stdout does not name the signal: %q", stdout)
	}
}

// TestStderrWarningWithSummaryShowsWarningOverlay verifies that a fake
// rg that writes "warn" to stderr and a summary-only stream, exit 1,
// shows a warning overlay then the no-results screen, and q exits 1.
func TestStderrWarningWithSummaryShowsWarningOverlay(t *testing.T) {
	fakeDir := t.TempDir()
	records := []string{`{"type":"summary","data":{}}`}
	writeFatalFakeRG(t, fakeDir, records, "warn", 1)

	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "test.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	handshakeFile := filepath.Join(t.TempDir(), "handshake")
	cmd := exec.Command(binPath, "hello", ".")
	cmd.Dir = repo
	cmd.Env = []string{
		"PATH=" + fakeDir + ":" + os.Getenv("PATH"),
		"VRG_TEST_HANDSHAKE=" + handshakeFile,
	}
	// Esc dismisses the warning overlay → no-results; q exits 1.
	stdout, _, exitCode := runVrgWithKeys(t, cmd, handshakeFile, "\x1b", "q")

	if exitCode != 1 {
		t.Fatalf("vrg exited %d, want 1 (warning → no-results)", exitCode)
	}
	// The warning overlay must contain "warn".
	if !strings.Contains(stdout, "warn") {
		t.Fatalf("vrg stdout does not contain the warning 'warn': %q", stdout)
	}
	// After dismissal, the no-results screen must appear.
	if !strings.Contains(stdout, "No results found") {
		t.Fatalf("vrg stdout does not contain 'No results found' after dismissal: %q", stdout)
	}
}

// TestStderrContentFixture verifies that a fake rg writing ≥ 1 MiB to
// stderr interleaved with a valid stdout stream produces a complete
// stdout stream, includes the captured stderr in diagnostics, and the
// overlay contains both the head and tail of the stderr text.
func TestStderrContentFixture(t *testing.T) {
	fakeDir := t.TempDir()
	rgPath := filepath.Join(fakeDir, "rg")
	handshakeFile := filepath.Join(t.TempDir(), "handshake")
	// Write a distinctive head and tail to stderr, with ≥ 1 MiB total.
	// The head is "HEADMARKER" and the tail is "TAILMARKER".
	script := `#!/bin/sh
printf 'HEADMARKER\n' >&2
i=0
while [ $i -lt 100 ]; do
	head -c 10240 /dev/zero | tr '\0' 'E' >&2
	echo '{"type":"begin","data":{"path":{"text":"test.txt"}}}'
	printf '%s\n' '{"type":"match","data":{"path":{"text":"test.txt"},"lines":{"text":"hello\n"},"line_number":1,"submatches":[{"match":{"text":"hello"},"start":0,"end":5}]}}'
	echo '{"type":"end","data":{"path":{"text":"test.txt"},"binary_offset":null}}'
	i=$((i + 1))
done
printf 'TAILMARKER\n' >&2
echo '{"type":"summary","data":{}}'
touch "$VRG_TEST_HANDSHAKE"
exit 3
`
	if err := os.WriteFile(rgPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "test.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(binPath, "hello", ".")
	cmd.Dir = repo
	cmd.Env = []string{
		"PATH=" + fakeDir + ":" + os.Getenv("PATH"),
		"VRG_TEST_HANDSHAKE=" + handshakeFile,
	}
	// Esc dismisses the overlay, then q exits 2.
	stdout, _, exitCode := runVrgWithKeys(t, cmd, handshakeFile, "\x1b", "q")

	if exitCode != 2 {
		t.Fatalf("vrg exited %d, want 2 (fatal with stderr)", exitCode)
	}
	// The stdout stream must be complete: the browse view appears.
	if !strings.Contains(stdout, "test.txt") {
		t.Fatalf("vrg stdout does not contain the browse view (stdout stream lost): %q", stdout)
	}
	// The overlay must contain the head of the stderr text.
	if !strings.Contains(stdout, "HEADMARKER") {
		t.Fatalf("vrg stdout does not contain the stderr head 'HEADMARKER': %q", stdout)
	}
	// The overlay must contain the tail of the stderr text.
	if !strings.Contains(stdout, "TAILMARKER") {
		t.Fatalf("vrg stdout does not contain the stderr tail 'TAILMARKER': %q", stdout)
	}
}
