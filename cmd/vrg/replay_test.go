package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vrg/internal/app"
)

// waitForAckLines polls ackFile until it has at least n lines, up to
// timeout. It calls t.Fatal if the file does not have enough lines in
// time. This waits for the application-side collection acknowledgement
// (Issue #11), not a child-side write handshake.
func waitForAckLines(t *testing.T, ackFile string, n int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(ackFile)
		if err == nil {
			lines := strings.Count(string(data), "\n")
			if lines >= n {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("ack file %s did not reach %d lines within %s", ackFile, n, timeout)
}

// replayResult holds the result of a PTY replay test run.
type replayResult struct {
	rawOutput string
	stderr    string
	exitCode  int
}

// runVrgReplay starts vrg under a PTY and runs the trigger callback
// through the shared Issue #48 driver: every wait is a bounded
// condition poll (fixture files, collect-ack line counts) or an
// application-side acknowledgement (update-ack events), every key
// send is acknowledged by the Update-processed event for that key,
// and termios restoration is checked on exit. It returns the raw PTY
// output, stderr, and exit code.
func runVrgReplay(t *testing.T, cmd *exec.Cmd, ackFile string, trigger func(d *ptyDriver)) replayResult {
	t.Helper()
	res := runVrgPTY(t, cmd, ackFile, trigger)
	return replayResult{rawOutput: res.raw, stderr: res.stderr, exitCode: res.exitCode}
}

// assertReplayAfterRestoration requires the raw PTY output to contain
// display-restoration sequences (cursor show, alt-screen exit if
// entered) and the stderr to contain the diagnostic exactly once. The
// ordering (replay after restoration) is guaranteed by the
// implementation: replay runs after program.Run() returns, which is
// after Bubble Tea restores the terminal.
func assertReplayAfterRestoration(t *testing.T, res replayResult, diag string) {
	t.Helper()
	assertDisplayRestoration(t, res.rawOutput)
	if !strings.Contains(res.stderr, diag) {
		t.Fatalf("stderr does not contain diagnostic %q: %q", diag, res.stderr)
	}
	if count := strings.Count(res.stderr, diag); count != 1 {
		t.Fatalf("diagnostic %q appears %d times in stderr, want 1: %q", diag, count, res.stderr)
	}
	if strings.ContainsAny(res.stderr, "\x1b\x9b") {
		t.Fatalf("stderr contains raw control bytes: %q", res.stderr)
	}
}

// writeStderrFakeRG writes a fake rg that writes stderrText to stderr,
// outputs a valid ripgrep JSON stream, touches the handshake file, and
// exits with exitCode. If block is true, the script blocks indefinitely
// after writing stderr (for testing cancellation while rg is still
// running).
func writeStderrFakeRG(t *testing.T, dir, stderrText string, exitCode int, block bool, extraEnv string) string {
	t.Helper()
	rgPath := filepath.Join(dir, "rg")
	script := "#!/bin/sh\n"
	if stderrText != "" {
		script += "printf '%s\\n' " + shellQuote(stderrText) + " >&2\n"
	}
	if block {
		// Write ready/pid if requested, then block.
		script += "if [ -n \"$FAKE_RG_READY_FILE\" ]; then touch \"$FAKE_RG_READY_FILE\"; fi\n"
		script += "if [ -n \"$FAKE_RG_PID_FILE\" ]; then echo $$ > \"$FAKE_RG_PID_FILE\"; fi\n"
		script += "sleep 100000\n"
	} else {
		// Output a valid stream.
		script += "echo '{\"type\":\"begin\",\"data\":{\"path\":{\"text\":\"test.txt\"}}}'\n"
		script += "printf '%s\\n' '{\"type\":\"match\",\"data\":{\"path\":{\"text\":\"test.txt\"},\"lines\":{\"text\":\"hello world\\n\"},\"line_number\":1,\"submatches\":[{\"match\":{\"text\":\"hello\"},\"start\":0,\"end\":5}]}}'\n"
		script += "echo '{\"type\":\"end\",\"data\":{\"path\":{\"text\":\"test.txt\"},\"binary_offset\":null}}'\n"
		script += "echo '{\"type\":\"summary\",\"data\":{}}'\n"
		script += "if [ -n \"$FAKE_RG_HANDSHAKE_FILE\" ]; then touch \"$FAKE_RG_HANDSHAKE_FILE\"; fi\n"
		script += "exit " + itoa(exitCode) + "\n"
	}
	if err := os.WriteFile(rgPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return rgPath
}

// TestReplayCtrlCAfterStderrDiagnostic verifies that ctrl+c sent after a
// stderr diagnostic has been collected (acknowledged) exits 130 with
// termios restored and the diagnostic on vrg's stderr exactly once
// after the display-restoration sequence (Issue #11, AC3).
func TestReplayCtrlCAfterStderrDiagnostic(t *testing.T) {
	fakeDir := t.TempDir()
	collectAck := filepath.Join(t.TempDir(), "ack")
	updateAck := filepath.Join(t.TempDir(), "update-ack")
	writeStderrFakeRG(t, fakeDir, "ctrl+c test warning", 1, false, "")

	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "test.txt"), []byte("hello world\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(binPath, "hello", ".")
	cmd.Dir = repo
	cmd.Env = []string{
		"PATH=" + fakeDir + ":" + os.Getenv("PATH"),
		"VRG_TEST_COLLECT_ACK=" + collectAck,
		"VRG_TEST_UPDATE_ACK=" + updateAck,
	}

	res := runVrgReplay(t, cmd, updateAck, func(d *ptyDriver) {
		// Wait for the diagnostic to be collected (ack fires after
		// SearchCompleteMsg is processed).
		waitForAckLines(t, collectAck, 1, 15*time.Second)
		// Send ctrl+c; the send blocks on the acknowledgement that
		// Update processed the key.
		d.sendKey(t, "\x03")
	})

	if res.exitCode != 130 {
		t.Fatalf("vrg exited %d, want 130 (ctrl+c)", res.exitCode)
	}
	assertReplayAfterRestoration(t, res, "ctrl+c test warning")
}

// TestReplayQWhileSearchingAfterDiagnostic verifies that q sent after
// the collection acknowledgement while the fake rg is still blocked
// (searching incomplete) exits 130 with termios restored and the
// collected diagnostic replayed exactly once after the
// display-restoration sequence (Issue #11, AC3).
func TestReplayQWhileSearchingAfterDiagnostic(t *testing.T) {
	fakeDir := t.TempDir()
	readyFile := filepath.Join(t.TempDir(), "ready")
	pidFile := filepath.Join(t.TempDir(), "pid")
	collectAck := filepath.Join(t.TempDir(), "ack")
	updateAck := filepath.Join(t.TempDir(), "update-ack")
	diagTrigger := filepath.Join(t.TempDir(), "diagtrigger")
	writeStderrFakeRG(t, fakeDir, "", 0, true, "")

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
		"VRG_TEST_COLLECT_ACK=" + collectAck,
		"VRG_TEST_UPDATE_ACK=" + updateAck,
		"VRG_TEST_DIAGNOSTIC_TRIGGER=" + diagTrigger,
		"VRG_TEST_DIAGNOSTIC_TEXT=searching diag",
	}

	res := runVrgReplay(t, cmd, updateAck, func(d *ptyDriver) {
		// Wait for the fake rg to signal ready.
		waitForFile(t, readyFile, 15*time.Second)
		// Trigger the diagnostic emission.
		if err := os.WriteFile(diagTrigger, []byte("trigger"), 0o644); err != nil {
			t.Errorf("cannot write diag trigger: %v", err)
		}
		// Wait for the diagnostic to be collected (ack fires).
		waitForAckLines(t, collectAck, 1, 15*time.Second)
		// Send q while searching (rg still blocked); the send blocks
		// on the acknowledgement that Update processed the key.
		d.sendKey(t, "q")
	})

	if res.exitCode != 130 {
		t.Fatalf("vrg exited %d, want 130 (q while searching)", res.exitCode)
	}
	assertReplayAfterRestoration(t, res, "searching diag")
}

// TestReplayQWhileGateHeldAfterDiagnostic verifies that q sent after
// the collection acknowledgement while the preparation gate is still
// held (result preparation incomplete) exits 130 with termios restored
// and the collected diagnostic replayed exactly once after the
// display-restoration sequence (Issue #11, AC3).
func TestReplayQWhileGateHeldAfterDiagnostic(t *testing.T) {
	fakeDir := t.TempDir()
	handshakeFile := filepath.Join(t.TempDir(), "handshake")
	collectAck := filepath.Join(t.TempDir(), "ack")
	updateAck := filepath.Join(t.TempDir(), "update-ack")
	diagTrigger := filepath.Join(t.TempDir(), "diagtrigger")
	gateFile := filepath.Join(t.TempDir(), "gate")
	writeStderrFakeRG(t, fakeDir, "", 0, false, "")

	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "test.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(binPath, "hello", ".")
	cmd.Dir = repo
	cmd.Env = []string{
		"PATH=" + fakeDir + ":" + os.Getenv("PATH"),
		"FAKE_RG_HANDSHAKE_FILE=" + handshakeFile,
		"VRG_TEST_COLLECT_ACK=" + collectAck,
		"VRG_TEST_UPDATE_ACK=" + updateAck,
		"VRG_TEST_DIAGNOSTIC_TRIGGER=" + diagTrigger,
		"VRG_TEST_DIAGNOSTIC_TEXT=gate diag",
		"VRG_TEST_GATE=" + gateFile,
	}

	res := runVrgReplay(t, cmd, updateAck, func(d *ptyDriver) {
		// Wait for the fake rg to finish (handshake).
		waitForFile(t, handshakeFile, 15*time.Second)
		// Trigger the diagnostic emission while the gate is held.
		if err := os.WriteFile(diagTrigger, []byte("trigger"), 0o644); err != nil {
			t.Errorf("cannot write diag trigger: %v", err)
		}
		// Wait for the diagnostic to be collected (ack fires).
		waitForAckLines(t, collectAck, 1, 15*time.Second)
		// Send q while gate-held (preparation incomplete); the send
		// blocks on the acknowledgement that Update processed the key.
		d.sendKey(t, "q")
	})

	if res.exitCode != 130 {
		t.Fatalf("vrg exited %d, want 130 (q while gate held)", res.exitCode)
	}
	assertReplayAfterRestoration(t, res, "gate diag")
}

// TestReplayNormalQAfterCompletedStreamWithWarning verifies that a
// normal q after a completed stream with a stderr warning replays the
// warning to stderr exactly once after the display-restoration
// sequence (Issue #11, AC2).
func TestReplayNormalQAfterCompletedStreamWithWarning(t *testing.T) {
	fakeDir := t.TempDir()
	collectAck := filepath.Join(t.TempDir(), "ack")
	updateAck := filepath.Join(t.TempDir(), "update-ack")
	writeStderrFakeRG(t, fakeDir, "warn one", 0, false, "")

	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "test.txt"), []byte("hello world\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(binPath, "hello", ".")
	cmd.Dir = repo
	cmd.Env = []string{
		"PATH=" + fakeDir + ":" + os.Getenv("PATH"),
		"VRG_TEST_COLLECT_ACK=" + collectAck,
		"VRG_TEST_UPDATE_ACK=" + updateAck,
	}

	res := runVrgReplay(t, cmd, updateAck, func(d *ptyDriver) {
		// Wait for the diagnostic to be collected (ack fires after
		// SearchCompleteMsg is processed — the warning overlay text
		// is collected).
		waitForAckLines(t, collectAck, 1, 15*time.Second)
		// Dismiss the warning overlay — the esc event must
		// acknowledge the dismissal before q is sent (Issue #48
		// overlay-dismissal-before-quit row) — then quit from
		// browse.
		esc := d.sendKey(t, "\x1b")
		if !esc.dismissed {
			t.Errorf("esc did not dismiss the warning overlay: %+v", esc)
		}
		d.sendKey(t, "q")
	})

	if res.exitCode != 0 {
		t.Fatalf("vrg exited %d, want 0 (normal quit from browse)", res.exitCode)
	}
	assertReplayAfterRestoration(t, res, "warn one")
}

// TestReplayControlledFailureWithEarlierDiagnostic verifies that an
// injected controlled failure's diagnostic appears exactly once
// alongside an earlier diagnostic in collection order, counted across
// both the former direct-write and replay mechanisms with no duplicate
// (Issue #11, AC4).
func TestReplayControlledFailureWithEarlierDiagnostic(t *testing.T) {
	fakeDir := t.TempDir()
	readyFile := filepath.Join(t.TempDir(), "ready")
	pidFile := filepath.Join(t.TempDir(), "pid")
	reapFile := filepath.Join(t.TempDir(), "reap")
	collectAck := filepath.Join(t.TempDir(), "ack")
	updateAck := filepath.Join(t.TempDir(), "update-ack")
	diagTrigger := filepath.Join(t.TempDir(), "diagtrigger")
	failTrigger := filepath.Join(t.TempDir(), "failtrigger")
	writeStderrFakeRG(t, fakeDir, "", 0, true, "")

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
		"VRG_TEST_COLLECT_ACK=" + collectAck,
		"VRG_TEST_UPDATE_ACK=" + updateAck,
		"VRG_TEST_DIAGNOSTIC_TRIGGER=" + diagTrigger,
		"VRG_TEST_DIAGNOSTIC_TEXT=earlier diag",
		"VRG_TEST_FAIL_TRIGGER=" + failTrigger,
		"VRG_TEST_FAIL_DIAGNOSTIC=controlled failure",
	}

	res := runVrgReplay(t, cmd, updateAck, func(d *ptyDriver) {
		// Wait for the fake rg to signal ready.
		waitForFile(t, readyFile, 15*time.Second)
		// Trigger the earlier diagnostic emission.
		if err := os.WriteFile(diagTrigger, []byte("trigger"), 0o644); err != nil {
			t.Errorf("cannot write diag trigger: %v", err)
		}
		// Wait for the earlier diagnostic to be collected.
		waitForAckLines(t, collectAck, 1, 15*time.Second)
		// Trigger the controlled failure.
		if err := os.WriteFile(failTrigger, []byte("trigger"), 0o644); err != nil {
			t.Errorf("cannot write fail trigger: %v", err)
		}
		// vrg exits on its own after the controlled failure.
	})

	if res.exitCode != 2 {
		t.Fatalf("vrg exited %d, want 2 (controlled failure)", res.exitCode)
	}
	assertReapEvidence(t, reapFile)
	assertDisplayRestoration(t, res.rawOutput)

	// Both diagnostics must appear exactly once in stderr.
	if count := strings.Count(res.stderr, "earlier diag"); count != 1 {
		t.Fatalf("'earlier diag' appears %d times in stderr, want 1: %q", count, res.stderr)
	}
	if count := strings.Count(res.stderr, "controlled failure"); count != 1 {
		t.Fatalf("'controlled failure' appears %d times in stderr, want 1: %q", count, res.stderr)
	}
	if strings.ContainsAny(res.stderr, "\x1b\x9b") {
		t.Fatalf("stderr contains raw control bytes: %q", res.stderr)
	}

	// Collection order: "earlier diag" must appear before "controlled
	// failure" in stderr.
	earlierIdx := strings.Index(res.stderr, "earlier diag")
	failIdx := strings.Index(res.stderr, "controlled failure")
	if earlierIdx < 0 || failIdx < 0 {
		t.Fatalf("missing diagnostic in stderr: %q", res.stderr)
	}
	if earlierIdx > failIdx {
		t.Fatalf("collection order wrong: 'earlier diag' at %d after 'controlled failure' at %d: %q", earlierIdx, failIdx, res.stderr)
	}
}

// TestReplayFilenameWithNewlineAndESC verifies that a diagnostic
// embedding a filename with \n and ESC is escaped and single-lined in
// the replayed stderr text through the Issue #6 utility (Issue #11,
// AC5).
func TestReplayFilenameWithNewlineAndESC(t *testing.T) {
	fakeDir := t.TempDir()
	readyFile := filepath.Join(t.TempDir(), "ready")
	pidFile := filepath.Join(t.TempDir(), "pid")
	collectAck := filepath.Join(t.TempDir(), "ack")
	updateAck := filepath.Join(t.TempDir(), "update-ack")
	diagTrigger := filepath.Join(t.TempDir(), "diagtrigger")
	writeStderrFakeRG(t, fakeDir, "", 0, true, "")

	// Build the diagnostic with the filename escaped through
	// EscapePathForDiagnostic (Issue #6 utility) so filename newlines
	// become literal \n sequences and cannot become diagnostic line
	// breaks.
	rawFilename := "file\x1bname\nwith\nnewline"
	escapedName := app.EscapePathForDiagnostic(rawFilename)
	diagText := "oversized record skipped for " + escapedName

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
		"VRG_TEST_COLLECT_ACK=" + collectAck,
		"VRG_TEST_UPDATE_ACK=" + updateAck,
		"VRG_TEST_DIAGNOSTIC_TRIGGER=" + diagTrigger,
		"VRG_TEST_DIAGNOSTIC_TEXT=" + diagText,
	}

	res := runVrgReplay(t, cmd, updateAck, func(d *ptyDriver) {
		// Wait for the fake rg to signal ready.
		waitForFile(t, readyFile, 15*time.Second)
		// Trigger the diagnostic emission.
		if err := os.WriteFile(diagTrigger, []byte("trigger"), 0o644); err != nil {
			t.Errorf("cannot write diag trigger: %v", err)
		}
		// Wait for the diagnostic to be collected.
		waitForAckLines(t, collectAck, 1, 15*time.Second)
		// Send ctrl+c; the send blocks on the acknowledgement that
		// Update processed the key.
		d.sendKey(t, "\x03")
	})

	if res.exitCode != 130 {
		t.Fatalf("vrg exited %d, want 130 (ctrl+c)", res.exitCode)
	}
	assertDisplayRestoration(t, res.rawOutput)

	diag := res.stderr
	if diag == "" {
		t.Fatal("stderr is empty, want a diagnostic")
	}
	// The replayed diagnostic must not contain a real newline (the
	// filename's \n must be escaped to literal \n).
	if strings.Contains(strings.TrimRight(diag, "\n"), "\n") {
		t.Fatalf("replayed diagnostic contains a real newline (filename not single-lined): %q", diag)
	}
	// The replayed diagnostic must not contain raw ESC.
	if strings.ContainsAny(diag, "\x1b\x9b") {
		t.Fatalf("replayed diagnostic contains raw ESC: %q", diag)
	}
	// The replayed diagnostic must contain the escaped filename form.
	// EscapePath converts \n to \n (literal backslash-n) and ESC to ^[.
	if !strings.Contains(diag, `\n`) {
		t.Fatalf("replayed diagnostic does not contain escaped \\n: %q", diag)
	}
	// The diagnostic must appear exactly once.
	// The stderr has one line (the diagnostic + newline). Check that
	// the diagnostic text (minus the trailing newline) appears once.
	diagNoTrailing := strings.TrimRight(diag, "\n")
	if diagNoTrailing != diagText {
		// The sanitized diagnostic should match the input (already
		// escaped through EscapePath, then EscapeDiagnostic preserves
		// backslashes and printable text).
		t.Fatalf("replayed diagnostic = %q, want %q", diagNoTrailing, diagText)
	}
}
