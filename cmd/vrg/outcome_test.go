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
)

// runVrgWithKeys starts vrg under a PTY, waits for the
// search-complete acknowledgement — proving the model processed
// SearchCompleteMsg into its post-search state — then sends the
// given keys in sequence. Each send blocks on the acknowledgement
// that Update processed that key press before the next is sent
// (Issue #48 handshake matrix), so no fixed inter-key delay remains.
// It returns the stripped stdout, stderr, and exit code.
func runVrgWithKeys(t *testing.T, cmd *exec.Cmd, ackFile string, keys ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	res := runVrgPTY(t, cmd, ackFile, func(d *ptyDriver) {
		d.waitMsg(t, "search-complete")
		for _, k := range keys {
			d.sendKey(t, k)
		}
	})
	return res.stdout, res.stderr, res.exitCode
}

// runVrgKillChild starts vrg under a PTY, waits for the fake rg's
// ready file (a fixture-side bounded poll proving the child is
// running and blocked), sends SIGKILL to the child's process group,
// waits for the search-complete acknowledgement proving the model
// processed the child's death into its outcome, waits for expect —
// when non-empty — to appear in the rendered output (a bounded poll
// proving the post-search view/overlay painted; the Update
// acknowledgement alone cannot prove a paint because the renderer
// coalesces frames), then sends the given keys — each acknowledged
// as in runVrgWithKeys — and returns the result.
func runVrgKillChild(t *testing.T, cmd *exec.Cmd, readyFile, pidFile, ackFile, expect string, keys ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	res := runVrgPTY(t, cmd, ackFile, func(d *ptyDriver) {
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
		d.waitMsg(t, "search-complete")
		if expect != "" {
			d.waitForOutput(t, expect)
		}
		for _, k := range keys {
			d.sendKey(t, k)
		}
	})
	return res.stdout, res.stderr, res.exitCode
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
	script += "if [ -n \"$FAKE_RG_HANDSHAKE_FILE\" ]; then touch \"$FAKE_RG_HANDSHAKE_FILE\"; fi\n"
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

	ackFile := filepath.Join(t.TempDir(), "update-ack")
	cmd := exec.Command(binPath, "hello", ".")
	cmd.Dir = repo
	cmd.Env = []string{
		"PATH=" + fakeDir + ":" + os.Getenv("PATH"),
		"VRG_TEST_UPDATE_ACK=" + ackFile,
	}
	// Wait for the error overlay to paint (the Update
	// acknowledgement proves the model state, not the frame — the
	// renderer coalesces frames, so the overlay text must be
	// observed in the output before dismissal), then send Esc to
	// dismiss and q to quit.
	res := runVrgPTY(t, cmd, ackFile, func(d *ptyDriver) {
		d.waitMsg(t, "search-complete")
		d.waitForOutput(t, "boom")
		if esc := d.sendKey(t, "\x1b"); !esc.dismissed {
			t.Errorf("esc did not dismiss the error overlay: %+v", esc)
		}
		d.sendKey(t, "q")
	})
	stdout, _, exitCode := res.stdout, res.stderr, res.exitCode

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

	ackFile := filepath.Join(t.TempDir(), "update-ack")
	cmd := exec.Command(binPath, "hello", ".")
	cmd.Dir = repo
	cmd.Env = []string{
		"PATH=" + fakeDir + ":" + os.Getenv("PATH"),
		"VRG_TEST_UPDATE_ACK=" + ackFile,
	}
	// Wait for the generated fatal diagnostic to paint before
	// sending q — the Update acknowledgement alone cannot prove the
	// overlay frame was rendered.
	res := runVrgPTY(t, cmd, ackFile, func(d *ptyDriver) {
		d.waitMsg(t, "search-complete")
		d.waitForOutput(t, "code 2")
		d.sendKey(t, "q")
	})
	stdout, _, exitCode := res.stdout, res.stderr, res.exitCode

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

	ackFile := filepath.Join(t.TempDir(), "update-ack")
	cmd := exec.Command(binPath, "hello", ".")
	cmd.Dir = repo
	cmd.Env = []string{
		"PATH=" + fakeDir + ":" + os.Getenv("PATH"),
		"VRG_TEST_UPDATE_ACK=" + ackFile,
	}
	_, _, exitCode := runVrgWithKeys(t, cmd, ackFile, "\x1b")

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
if [ -n "$FAKE_RG_PID_FILE" ]; then echo $$ > "$FAKE_RG_PID_FILE"; fi
echo '{"type":"begin","data":{"path":{"text":"test.txt"}}}'
printf '%s\n' '{"type":"match","data":{"path":{"text":"test.txt"},"lines":{"text":"hello\n"},"line_number":1,"submatches":[{"match":{"text":"hello"},"start":0,"end":5}]}}'
if [ -n "$FAKE_RG_READY_FILE" ]; then touch "$FAKE_RG_READY_FILE"; fi
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
	ackFile := filepath.Join(t.TempDir(), "update-ack")
	cmd := exec.Command(binPath, "hello", ".")
	cmd.Dir = repo
	cmd.Env = []string{
		"PATH=" + fakeDir + ":" + os.Getenv("PATH"),
		"FAKE_RG_READY_FILE=" + readyFile,
		"FAKE_RG_PID_FILE=" + pidFile,
		"VRG_TEST_UPDATE_ACK=" + ackFile,
	}

	// Start vrg under a PTY, wait for ready, then SIGKILL the fake rg.
	// The run waits for the "signal" diagnostic to paint before the
	// dismissal keys — the Update acknowledgement proves the model
	// state, not the frame.
	stdout, _, exitCode := runVrgKillChild(t, cmd, readyFile, pidFile, ackFile, "signal", "q", "q")

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

	ackFile := filepath.Join(t.TempDir(), "update-ack")
	cmd := exec.Command(binPath, "hello", ".")
	cmd.Dir = repo
	cmd.Env = []string{
		"PATH=" + fakeDir + ":" + os.Getenv("PATH"),
		"VRG_TEST_UPDATE_ACK=" + ackFile,
	}
	// Esc dismisses the warning overlay → no-results; q exits 1.
	// Both rendered post-states are waited on in the output: the
	// Update acknowledgement proves the model transition, not the
	// paint, and each view is transient (the overlay yields to
	// no-results, which yields to exit).
	res := runVrgPTY(t, cmd, ackFile, func(d *ptyDriver) {
		d.waitMsg(t, "search-complete")
		d.waitForOutput(t, "warn")
		if esc := d.sendKey(t, "\x1b"); !esc.dismissed {
			t.Errorf("esc did not dismiss the warning overlay: %+v", esc)
		}
		d.waitForOutput(t, "No results found")
		d.sendKey(t, "q")
	})
	stdout, _, exitCode := res.stdout, res.stderr, res.exitCode

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
// stdout stream, includes the captured stderr in diagnostics, and
// completes. Issue #41 revises this contract: the overlay's
// scrollable row set is the complete wrapped diagnostic, so head and
// tail are no longer required to render simultaneously — the head is
// visible when the overlay opens, and tail reachability is proven by
// the model-level complete-rows and bounded-traversal tests rather
// than by sending thousands of PTY keys.
func TestStderrContentFixture(t *testing.T) {
	fakeDir := t.TempDir()
	rgPath := filepath.Join(fakeDir, "rg")
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
exit 3
`
	if err := os.WriteFile(rgPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "test.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ackFile := filepath.Join(t.TempDir(), "update-ack")
	cmd := exec.Command(binPath, "hello", ".")
	cmd.Dir = repo
	cmd.Env = []string{
		"PATH=" + fakeDir + ":" + os.Getenv("PATH"),
		"VRG_TEST_UPDATE_ACK=" + ackFile,
	}
	// The head of the diagnostic must paint at scroll position 0
	// before any scroll key — the Update acknowledgement proves the
	// model state, not the frame. A few downs then exercise scrolling
	// over the large diagnostic; the first q dismisses the overlay
	// and the second exits 2. A bare Esc is avoided here: after an
	// expensive Update on the 1 MiB diagnostic the input reader can
	// coalesce "\x1b"+"q" into a single Alt+q press, which the modal
	// overlay ignores.
	res := runVrgPTY(t, cmd, ackFile, func(d *ptyDriver) {
		d.waitMsg(t, "search-complete")
		d.waitForOutput(t, "HEADMARKER")
		for range 3 {
			d.sendKey(t, "\x1b[B")
		}
		if q := d.sendKey(t, "q"); !q.dismissed {
			t.Errorf("first q did not dismiss the error overlay: %+v", q)
		}
		d.sendKey(t, "q")
	})
	stdout, _, exitCode := res.stdout, res.stderr, res.exitCode

	if exitCode != 2 {
		t.Fatalf("vrg exited %d, want 2 (fatal with stderr)", exitCode)
	}
	// The stdout stream must be complete: the browse view appears.
	if !strings.Contains(stdout, "test.txt") {
		t.Fatalf("vrg stdout does not contain the browse view (stdout stream lost): %q", stdout)
	}
	// The captured stderr must reach the diagnostic overlay: the head
	// is visible at scroll position 0. Issue #41: tail reachability
	// is proven by the model-level complete-rows and traversal tests,
	// not by simultaneous head/tail rendering.
	if !strings.Contains(stdout, "HEADMARKER") {
		t.Fatalf("vrg stdout does not contain the stderr head 'HEADMARKER': %q", stdout)
	}
}
