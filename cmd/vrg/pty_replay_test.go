//go:build linux

package main

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// The stderr-replay PTY contract: every diagnostic processed into the
// session collection is written to vrg's own stderr exactly once, in
// collection order, strictly after the display-restoration sequence —
// with the slave termios already back to its pre-launch state. The
// VRG_TEST_DIAG_ACK_FILE side channel carries one line per collected
// diagnostic: the application-side acknowledgement the tests wait for
// before sending an exit key, proving the diagnostic was processed —
// where a child-side write handshake would prove only that bytes
// reached the pipe.

// assertReplayedOnce requires text to appear exactly once in the
// captured stream after the display-restoration sequence — the
// post-restoration stderr replay. Diagnostics that were also displayed
// (an open overlay) legitimately appear earlier in the stream; only
// the tail is counted.
func assertReplayedOnce(t *testing.T, out, text string) {
	t.Helper()
	restore := strings.Index(out, "\x1b[?1049l")
	if restore < 0 {
		t.Fatalf("output lacks the display-restoration sequence: %q", out)
	}
	if n := strings.Count(out[restore:], text); n != 1 {
		t.Fatalf("%q appears %d times after display restoration, want once: %q", text, n, out)
	}
}

// replayTail returns the captured stream from the display-restoration
// sequence onward — where the replayed diagnostics must appear.
func replayTail(t *testing.T, out string) string {
	t.Helper()
	restore := strings.Index(out, "\x1b[?1049l")
	if restore < 0 {
		t.Fatalf("output lacks the display-restoration sequence: %q", out)
	}
	return out[restore:]
}

// fakeRgWarnBlockScript signals readiness, writes one stderr
// diagnostic, then blocks forever — the run can only end through
// cancellation, with the diagnostic already collected.
const fakeRgWarnBlockScript = `#!/bin/sh
echo $$ > "$VRG_CAPTURE_DIR/ready"
echo "warn one" >&2
exec sleep 3600
`

// fakeRgWarnStreamScript signals readiness, writes one stderr
// diagnostic and a complete valid result stream, and exits 0.
const fakeRgWarnStreamScript = `#!/bin/sh
echo $$ > "$VRG_CAPTURE_DIR/ready"
echo "warn one" >&2
printf '%s\n' \
'{"type":"begin","data":{"path":{"text":"f.txt"}}}' \
'{"type":"match","data":{"path":{"text":"f.txt"},"lines":{"text":"hello\n"},"line_number":1,"submatches":[{"match":{"text":"hello"},"start":0,"end":5}]}}' \
'{"type":"end","data":{"path":{"text":"f.txt"},"binary_offset":null}}' \
'{"type":"summary","data":{}}'
exit 0
`

// fakeRgWarnNoSummaryScript is the same but withholds the summary
// record: its "missing summary" diagnostic exists only inside the
// completion message, which the preparation gate can hold back.
const fakeRgWarnNoSummaryScript = `#!/bin/sh
echo $$ > "$VRG_CAPTURE_DIR/ready"
echo "warn one" >&2
printf '%s\n' \
'{"type":"begin","data":{"path":{"text":"f.txt"}}}' \
'{"type":"match","data":{"path":{"text":"f.txt"},"lines":{"text":"hello\n"},"line_number":1,"submatches":[{"match":{"text":"hello"},"start":0,"end":5}]}}' \
'{"type":"end","data":{"path":{"text":"f.txt"},"binary_offset":null}}'
exit 0
`

// ctrl+c after a collected stderr diagnostic, with rg still running:
// exit 130, termios restored, and the diagnostic on vrg's stderr after
// the display-restoration sequence exactly once.
func TestPTYCtrlCAfterDiagnosticReplaysOnce(t *testing.T) {
	fakeDir, capDir := writeFakeRg(t, fakeRgWarnBlockScript)
	ack := filepath.Join(capDir, "diag-ack")
	reap := filepath.Join(capDir, "reap")
	r := startVrgPTY(t, ptyEnv(fakeDir, capDir,
		"VRG_TEST_DIAG_ACK_FILE="+ack,
		"VRG_TEST_REAP_FILE="+reap), "foo")
	pid := awaitReadyPID(t, capDir)
	r.waitFor(t, "Searching…")
	// Wait for the application-side acknowledgement — the diagnostic
	// is in the session collection — before sending the exit key.
	awaitFileContent(t, ack)
	r.send(t, "\x03")
	code, out := r.waitExit(t)
	if code != 130 {
		t.Fatalf("exit = %d, want 130; output: %q", code, out)
	}
	assertDisplayRestored(t, out)
	assertReplayedOnce(t, out, "warn one")
	if got := readFile(t, reap); !strings.Contains(got, "killed") {
		t.Fatalf("reap side channel = %q, want the killed wait status", got)
	}
	pidIsGone(t, pid)
}

// The same boundary for q: sent while rg is still running — a
// cancellation while searching, not a quit after a completed stream —
// exit 130 with the collected diagnostic replayed exactly once.
func TestPTYQWhileSearchingReplaysDiagnostic(t *testing.T) {
	fakeDir, capDir := writeFakeRg(t, fakeRgWarnBlockScript)
	ack := filepath.Join(capDir, "diag-ack")
	reap := filepath.Join(capDir, "reap")
	r := startVrgPTY(t, ptyEnv(fakeDir, capDir,
		"VRG_TEST_DIAG_ACK_FILE="+ack,
		"VRG_TEST_REAP_FILE="+reap), "foo")
	pid := awaitReadyPID(t, capDir)
	r.waitFor(t, "Searching…")
	awaitFileContent(t, ack)
	r.send(t, "q")
	code, out := r.waitExit(t)
	if code != 130 {
		t.Fatalf("exit = %d, want 130; output: %q", code, out)
	}
	assertDisplayRestored(t, out)
	assertReplayedOnce(t, out, "warn one")
	if got := readFile(t, reap); !strings.Contains(got, "killed") {
		t.Fatalf("reap side channel = %q, want the killed wait status", got)
	}
	pidIsGone(t, pid)
}

// q while gate-held result preparation keeps the app searching: rg has
// exited, the stderr diagnostic is collected and acknowledged, and the
// completion's "missing summary" diagnostic is never delivered —
// cancellation replays only what was collected, without waiting on the
// gate. Exit 130.
func TestPTYQDuringGateHeldPreparationReplaysDiagnostic(t *testing.T) {
	fakeDir, capDir := writeFakeRg(t, fakeRgWarnNoSummaryScript)
	ack := filepath.Join(capDir, "diag-ack")
	reap := filepath.Join(capDir, "reap")
	gate := filepath.Join(capDir, "gate-fifo")
	if err := syscall.Mkfifo(gate, 0o600); err != nil {
		t.Fatalf("mkfifo: %v", err)
	}
	r := startVrgPTY(t, ptyEnv(fakeDir, capDir,
		"VRG_TEST_DIAG_ACK_FILE="+ack,
		"VRG_TEST_REAP_FILE="+reap,
		"VRG_TEST_GATE_FIFO="+gate), "foo")
	pid := awaitReadyPID(t, capDir)
	r.waitFor(t, "Searching…")
	// The child has exited and been reaped while preparation stays
	// gate-held; the stderr diagnostic is already collected.
	if got := awaitFileContent(t, reap); !strings.Contains(got, "exit status 0") {
		t.Fatalf("reap side channel = %q, want the child's clean exit", got)
	}
	awaitFileContent(t, ack)
	r.send(t, "q")
	code, out := r.waitExit(t)
	if code != 130 {
		t.Fatalf("exit = %d, want 130; output: %q", code, out)
	}
	assertDisplayRestored(t, out)
	assertReplayedOnce(t, out, "warn one")
	tail := replayTail(t, out)
	if strings.Contains(tail, "missing summary") {
		t.Fatalf("gate-held completion diagnostic was replayed: %q", tail)
	}
	pidIsGone(t, pid)
}

// A normal q after a completed stream carrying a stderr warning: the
// acknowledgement precedes the keypress, the warning overlay dismisses
// to browse, and quitting replays the diagnostic exactly once after
// restoration. Exit 0 — the warning alone never changes the status.
func TestPTYQuitAfterCompletedStreamReplaysWarning(t *testing.T) {
	fakeDir, capDir := writeFakeRg(t, fakeRgWarnStreamScript)
	ack := filepath.Join(capDir, "diag-ack")
	r := startVrgPTY(t, ptyEnv(fakeDir, capDir,
		"VRG_TEST_DIAG_ACK_FILE="+ack), "foo")
	awaitReadyPID(t, capDir)
	awaitFileContent(t, ack) // collected before the keypress
	r.waitFor(t, "warn one") // the warning overlay over browse
	before := len(r.output())
	r.send(t, "q") // dismiss the overlay
	r.waitForGrowth(t, before)
	r.send(t, "q") // quit from browse
	code, out := r.waitExit(t)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; output: %q", code, out)
	}
	assertDisplayRestored(t, out)
	assertReplayedOnce(t, out, "warn one")
}

// An injected controlled failure after a diagnostic was collected: the
// failure's own diagnostic enters the session collection and the common
// post-restoration writer replays both — earlier diagnostics first —
// exactly once each across the direct-write and replay mechanisms.
func TestPTYControlledFailureReplaysAlongsideEarlierDiagnostics(t *testing.T) {
	fakeDir, capDir := writeFakeRg(t, fakeRgWarnBlockScript)
	ack := filepath.Join(capDir, "diag-ack")
	reap := filepath.Join(capDir, "reap")
	fail := filepath.Join(capDir, "fail-fifo")
	if err := syscall.Mkfifo(fail, 0o600); err != nil {
		t.Fatalf("mkfifo: %v", err)
	}
	r := startVrgPTY(t, ptyEnv(fakeDir, capDir,
		"VRG_TEST_DIAG_ACK_FILE="+ack,
		"VRG_TEST_REAP_FILE="+reap,
		"VRG_TEST_FAIL_FIFO="+fail), "foo")
	pid := awaitReadyPID(t, capDir)
	r.waitFor(t, "Searching…")
	awaitFileContent(t, ack)
	// The fifo handshake fires the injected failure: the writer's open
	// pairs with vrg's reader and its close delivers the EOF.
	go func() {
		if w, err := os.OpenFile(fail, os.O_WRONLY, 0); err == nil {
			_, _ = w.WriteString("fail\n")
			_ = w.Close()
		}
	}()
	code, out := r.waitExit(t)
	if code != 2 {
		t.Fatalf("exit = %d, want 2; output: %q", code, out)
	}
	assertDisplayRestored(t, out)
	tail := replayTail(t, out)
	if n := strings.Count(tail, "warn one"); n != 1 {
		t.Fatalf("collected diagnostic replayed %d times, want once: %q", n, tail)
	}
	if n := strings.Count(out, "vrg:"); n != 1 {
		t.Fatalf("failure diagnostic written %d times across both mechanisms, want once: %q", n, out)
	}
	if strings.Index(tail, "warn one") > strings.Index(tail, "vrg:") {
		t.Fatalf("replayed diagnostics out of collection order: %q", tail)
	}
	if got := readFile(t, reap); !strings.Contains(got, "killed") {
		t.Fatalf("reap side channel = %q, want the killed wait status", got)
	}
	pidIsGone(t, pid)
}

// fakeRgBadPathScript emits a complete stream whose one match names a
// file with an embedded newline and ESC — a path that cannot load in
// the empty working directory — then exits 0. The load failure is a
// collected diagnostic that no screen displays.
const fakeRgBadPathScript = `#!/bin/sh
echo $$ > "$VRG_CAPTURE_DIR/ready"
printf '%s\n' \
'{"type":"begin","data":{"path":{"text":"we\nir\u001bd.txt"}}}' \
'{"type":"match","data":{"path":{"text":"we\nir\u001bd.txt"},"lines":{"text":"x\n"},"line_number":1,"submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}' \
'{"type":"end","data":{"path":{"text":"we\nir\u001bd.txt"},"binary_offset":null}}' \
'{"type":"summary","data":{}}'
exit 0
`

// A diagnostic embedding a filename with a newline and ESC is escaped
// and single-lined in the replayed text: the filename's newline cannot
// become a diagnostic line break and no raw ESC reaches the terminal.
func TestPTYReplayEscapesEmbeddedFilename(t *testing.T) {
	fakeDir, capDir := writeFakeRg(t, fakeRgBadPathScript)
	ack := filepath.Join(capDir, "diag-ack")
	r := startVrgPTY(t, ptyEnv(fakeDir, capDir,
		"VRG_TEST_DIAG_ACK_FILE="+ack), "foo")
	awaitReadyPID(t, capDir)
	r.waitFor(t, "(unreadable)") // the failed load's placeholder
	awaitFileContent(t, ack)     // the failure diagnostic is collected
	r.send(t, "q")
	code, out := r.waitExit(t)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; output: %q", code, out)
	}
	assertDisplayRestored(t, out)
	tail := replayTail(t, out)
	idx := strings.Index(tail, "cannot read")
	if idx < 0 {
		t.Fatalf("replay lacks the load-failure diagnostic: %q", out)
	}
	diagLine := tail[idx:]
	if nl := strings.IndexByte(diagLine, '\n'); nl >= 0 {
		diagLine = diagLine[:nl]
	}
	diagLine = strings.TrimSuffix(diagLine, "\r")
	if !strings.Contains(diagLine, `we\nir^[d.txt`) {
		t.Fatalf("diagnostic lacks the single-lined escaped filename: %q", diagLine)
	}
	if strings.ContainsAny(diagLine, "\x1b\r") {
		t.Fatalf("replayed diagnostic carries a raw control byte: %q", diagLine)
	}
}
