package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// This file drives the Issue 11 stderr-replay contract at the real
// subprocess boundary: diagnostics collected while the TUI owns the
// terminal are replayed to stderr after the display-restoration
// sequence, exactly once, in collection order — on cancellation,
// ordinary quits, and controlled failures alike. Tests wait on the
// application-side collection acknowledgement (VRG_TEST_COLLECT_ACK)
// before sending an exit key: a child-side write only proves bytes
// reached the pipe, not that the model processed the diagnostic into
// the session collection.

// waitCollectAck waits until the VRG_TEST_COLLECT_ACK side channel —
// one line per diagnostic processed into the session collection —
// contains marker.
func waitCollectAck(t *testing.T, path, marker string) {
	t.Helper()
	waitBounded(t, "collect ack "+strconv.Quote(marker), 15*time.Second, func() bool {
		b, err := os.ReadFile(path)
		return err == nil && strings.Contains(string(b), marker)
	})
}

// assertReplayedAfterRestore requires the diagnostic to appear in vrg's
// stderr exactly once, after the display-restoration sequence in the
// merged PTY stream — replay ordering is observable because stderr is
// forwarded into the PTY — and with the input modes already restored.
func assertReplayedAfterRestore(t *testing.T, r *ptyRun, marker string) {
	t.Helper()
	errOut := r.stderr.String()
	if n := strings.Count(errOut, marker); n != 1 {
		t.Fatalf("stderr contains %q %d times, want exactly once: %q", marker, n, errOut)
	}
	out := r.output()
	restore := strings.LastIndex(out, "\x1b[?1049l")
	diagAt := strings.LastIndex(out, marker)
	if restore < 0 {
		t.Fatalf("PTY output lacks the leave-alternate-screen sequence: %q", out)
	}
	if diagAt < restore {
		t.Fatalf("replayed %q not after display restoration (restore@%d, diag@%d): %q",
			marker, restore, diagAt, out)
	}
}

// waitReplayedInputRestored samples the PTY termios at the moment the
// replayed bytes land on the PTY: they must arrive only after the input
// modes were restored. A violation this sample misses is still caught
// by the ordering assertion — this is the live check, not a wait.
func waitReplayedInputRestored(t *testing.T, r *ptyRun, marker string) {
	t.Helper()
	waitBounded(t, "replayed "+strconv.Quote(marker), 15*time.Second, func() bool {
		return strings.Contains(r.stderr.String(), marker)
	})
	if got, err := unix.IoctlGetTermios(int(r.master.Fd()), unix.TCGETS); err == nil && *got != r.before {
		t.Fatalf("replayed %q arrived while the PTY input modes were still raw: %+v", marker, got)
	}
}

// TestCtrlCAfterCollectedDiagnosticReplays: fake rg emits a stderr
// diagnostic then blocks; the collection acknowledgement — not the
// child's write — is what the test waits on before ctrl+c. The run
// exits 130, the terminal is restored, and the diagnostic appears on
// vrg's stderr after the display-restoration sequence, exactly once.
func TestCtrlCAfterCollectedDiagnosticReplays(t *testing.T) {
	dir := t.TempDir()
	ready := filepath.Join(dir, "ready")
	pidFile := filepath.Join(dir, "pid")
	reap := filepath.Join(dir, "reap")
	ack := filepath.Join(dir, "collect-ack")
	rgDir := fakeRgPath(t, `#!/bin/sh
echo $$ > "$VRG_TEST_PID"
printf '%s\n' 'warn one' >&2
: > "$VRG_TEST_READY"
exec sleep 100000
`)
	r := startVrgPTY(t, dir, childEnv(map[string]string{
		"PATH":                 rgDir + ":" + os.Getenv("PATH"),
		"TERM":                 "xterm-256color",
		"VRG_TEST_READY":       ready,
		"VRG_TEST_PID":         pidFile,
		"VRG_TEST_REAP":        reap,
		"VRG_TEST_COLLECT_ACK": ack,
	}), true, "foo", ".")
	waitFile(t, ready)
	killPidOnCleanup(t, pidFile)
	r.waitOutput(t, "Searching")
	waitCollectAck(t, ack, "warn one")
	r.send(t, "\x03")
	waitReplayedInputRestored(t, r, "warn one")
	code := r.waitExit(t)
	r.finish(t)

	if code != 130 {
		t.Fatalf("exit status = %d, want 130", code)
	}
	waitPidGone(t, readPid(t, pidFile))
	if got := reapStatus(t, reap); got != "signal: killed" {
		t.Fatalf("reaped wait status = %q, want %q", got, "signal: killed")
	}
	assertTerminalRestored(t, r)
	assertReplayedAfterRestore(t, r, "warn one")
	if got := r.stderr.String(); got != "warn one\n" {
		t.Fatalf("stderr = %q, want only the replayed diagnostic", got)
	}
}

// TestQWhileBlockedFakeRGReplaysCollected: the same cancellation
// contract through q while searching — a diagnostic acknowledged as
// collected before q is replayed at exit 130 after restoration, and the
// blocked child is terminated and reaped.
func TestQWhileBlockedFakeRGReplaysCollected(t *testing.T) {
	dir := t.TempDir()
	ready := filepath.Join(dir, "ready")
	pidFile := filepath.Join(dir, "pid")
	reap := filepath.Join(dir, "reap")
	ack := filepath.Join(dir, "collect-ack")
	rgDir := fakeRgPath(t, `#!/bin/sh
echo $$ > "$VRG_TEST_PID"
printf '%s\n' 'warn one' >&2
: > "$VRG_TEST_READY"
exec sleep 100000
`)
	r := startVrgPTY(t, dir, childEnv(map[string]string{
		"PATH":                 rgDir + ":" + os.Getenv("PATH"),
		"TERM":                 "xterm-256color",
		"VRG_TEST_READY":       ready,
		"VRG_TEST_PID":         pidFile,
		"VRG_TEST_REAP":        reap,
		"VRG_TEST_COLLECT_ACK": ack,
	}), true, "foo", ".")
	waitFile(t, ready)
	killPidOnCleanup(t, pidFile)
	r.waitOutput(t, "Searching")
	waitCollectAck(t, ack, "warn one")
	r.send(t, "q")
	waitReplayedInputRestored(t, r, "warn one")
	code := r.waitExit(t)
	r.finish(t)

	if code != 130 {
		t.Fatalf("exit status = %d, want 130", code)
	}
	waitPidGone(t, readPid(t, pidFile))
	if got := reapStatus(t, reap); got != "signal: killed" {
		t.Fatalf("reaped wait status = %q, want %q", got, "signal: killed")
	}
	assertTerminalRestored(t, r)
	assertReplayedAfterRestore(t, r, "warn one")
	if got := r.stderr.String(); got != "warn one\n" {
		t.Fatalf("stderr = %q, want only the replayed diagnostic", got)
	}
}

// TestQDuringGateHeldPreparationReplaysCollected: rg has exited and been
// reaped but index preparation is gate-held; a diagnostic acknowledged
// as collected before q is replayed at exit 130 — cancellation, not a
// browse quit — and the gated result's diagnostics never arrive.
func TestQDuringGateHeldPreparationReplaysCollected(t *testing.T) {
	dir := t.TempDir()
	ready := filepath.Join(dir, "ready")
	pidFile := filepath.Join(dir, "pid")
	reap := filepath.Join(dir, "reap")
	ack := filepath.Join(dir, "collect-ack")
	gate := filepath.Join(dir, "gate") // never created: preparation stays held
	rgDir := fakeRgPath(t, `#!/bin/sh
echo $$ > "$VRG_TEST_PID"
printf '%s\n' '{"type":"begin","data":{"path":{"text":"a.txt"}}}'
printf '%s\n' '{"type":"end","data":{"path":{"text":"a.txt"},"binary_offset":null}}'
printf '%s\n' 'warn gate' >&2
: > "$VRG_TEST_READY"
exit 0
`)
	r := startVrgPTY(t, dir, childEnv(map[string]string{
		"PATH":                 rgDir + ":" + os.Getenv("PATH"),
		"TERM":                 "xterm-256color",
		"VRG_TEST_READY":       ready,
		"VRG_TEST_PID":         pidFile,
		"VRG_TEST_REAP":        reap,
		"VRG_TEST_GATE":        gate,
		"VRG_TEST_COLLECT_ACK": ack,
	}), true, "foo", ".")
	waitFile(t, ready)
	killPidOnCleanup(t, pidFile)
	// The reap record proves rg exited and vrg's wait path ran; the gate
	// still holds preparation, so the run stays in searching.
	waitBounded(t, "reap record", 15*time.Second, func() bool {
		_, err := os.Stat(reap)
		return err == nil
	})
	r.waitOutput(t, "Searching")
	waitCollectAck(t, ack, "warn gate")
	r.send(t, "q")
	waitReplayedInputRestored(t, r, "warn gate")
	code := r.waitExit(t)
	r.finish(t)

	if code != 130 {
		t.Fatalf("exit status = %d, want 130", code)
	}
	if got := reapStatus(t, reap); got != "exit status 0" {
		t.Fatalf("reaped wait status = %q, want %q", got, "exit status 0")
	}
	assertTerminalRestored(t, r)
	assertReplayedAfterRestore(t, r, "warn gate")
	// Only the collected stderr diagnostic replays: the gated search
	// result's integrity diagnostic was never delivered and is not
	// waited on.
	if got := r.stderr.String(); got != "warn gate\n" {
		t.Fatalf("stderr = %q, want only the collected diagnostic", got)
	}
}

// TestNormalQuitReplaysCollectedWarning: a completed stream with a
// stderr warning — collected and shown in the warning overlay — replays
// the warning after restoration on an ordinary q exit, exactly once.
func TestNormalQuitReplaysCollectedWarning(t *testing.T) {
	dir := t.TempDir()
	ready := filepath.Join(dir, "ready")
	pidFile := filepath.Join(dir, "pid")
	reap := filepath.Join(dir, "reap")
	ack := filepath.Join(dir, "collect-ack")
	var content strings.Builder
	for i := 1; i <= 20; i++ {
		fmt.Fprintf(&content, "hit %02d\n", i)
	}
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte(content.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	rgDir := fakeRgPath(t, `#!/bin/sh
echo $$ > "$VRG_TEST_PID"
printf '%s\n' '{"type":"begin","data":{"path":{"text":"file.txt"}}}'
printf '%s\n' '{"type":"match","data":{"path":{"text":"file.txt"},"lines":{"text":"hit 01\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}'
printf '%s\n' '{"type":"end","data":{"path":{"text":"file.txt"},"binary_offset":null}}'
printf '%s\n' '{"type":"summary","data":{}}'
printf '%s\n' 'warn one' >&2
: > "$VRG_TEST_READY"
exit 0
`)
	r := startVrgPTY(t, dir, childEnv(map[string]string{
		"PATH":                 rgDir + ":" + os.Getenv("PATH"),
		"TERM":                 "xterm-256color",
		"VRG_TEST_READY":       ready,
		"VRG_TEST_PID":         pidFile,
		"VRG_TEST_REAP":        reap,
		"VRG_TEST_COLLECT_ACK": ack,
	}), true, "foo", ".")
	waitFile(t, ready)
	killPidOnCleanup(t, pidFile)

	r.waitOutput(t, "warn one") // the warning overlay is open
	waitCollectAck(t, ack, "warn one")
	r.waitOutput(t, "hit 09") // the file loaded; the row above the box shows
	if strings.Contains(r.output(), "hit 10") {
		t.Fatal("a row under the overlay box was visible before dismissal")
	}
	r.send(t, "q") // dismisses the overlay to browse
	r.waitOutput(t, "hit 10")
	r.send(t, "q")
	waitReplayedInputRestored(t, r, "warn one")
	code := r.waitExit(t)
	r.finish(t)

	if code != 0 {
		t.Fatalf("exit status = %d, want 0", code)
	}
	if got := reapStatus(t, reap); got != "exit status 0" {
		t.Fatalf("reaped wait status = %q, want %q", got, "exit status 0")
	}
	assertTerminalRestored(t, r)
	// The overlay rendered "warn one" while browsing, so the merged PTY
	// stream contains it more than once; stderr itself holds it exactly
	// once, and the last occurrence is the post-restoration replay.
	assertReplayedAfterRestore(t, r, "warn one")
	if got := r.stderr.String(); got != "warn one\n" {
		t.Fatalf("stderr = %q, want exactly one replayed line", got)
	}
}

// TestControlledFailureReplaysAfterEarlierDiagnostic: a collected stderr
// diagnostic precedes an injected controlled failure; the failure's own
// diagnostic enters the same collection and both replay in order after
// restoration — the failure exactly once, never written directly.
func TestControlledFailureReplaysAfterEarlierDiagnostic(t *testing.T) {
	dir := t.TempDir()
	ready := filepath.Join(dir, "ready")
	pidFile := filepath.Join(dir, "pid")
	reap := filepath.Join(dir, "reap")
	ack := filepath.Join(dir, "collect-ack")
	trigger := filepath.Join(dir, "trigger")
	diag := "injected failure \x1b[31m\nsecond line"
	rgDir := fakeRgPath(t, `#!/bin/sh
echo $$ > "$VRG_TEST_PID"
printf '%s\n' 'early warn' >&2
: > "$VRG_TEST_READY"
exec sleep 100000
`)
	r := startVrgPTY(t, dir, childEnv(map[string]string{
		"PATH":                     rgDir + ":" + os.Getenv("PATH"),
		"TERM":                     "xterm-256color",
		"VRG_TEST_READY":           ready,
		"VRG_TEST_PID":             pidFile,
		"VRG_TEST_REAP":            reap,
		"VRG_TEST_COLLECT_ACK":     ack,
		"VRG_TEST_FAIL_TRIGGER":    trigger,
		"VRG_TEST_FAIL_DIAGNOSTIC": diag,
	}), true, "foo", ".")
	waitFile(t, ready)
	killPidOnCleanup(t, pidFile)
	r.waitOutput(t, "Searching")
	waitCollectAck(t, ack, "early warn")
	if err := os.WriteFile(trigger, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	waitReplayedInputRestored(t, r, "injected failure")
	code := r.waitExit(t)
	r.finish(t)

	if code != 2 {
		t.Fatalf("exit status = %d, want 2", code)
	}
	waitPidGone(t, readPid(t, pidFile))
	if got := reapStatus(t, reap); got != "signal: killed" {
		t.Fatalf("reaped wait status = %q, want %q", got, "signal: killed")
	}
	assertTerminalRestored(t, r)

	errOut := r.stderr.String()
	if n := strings.Count(errOut, "early warn"); n != 1 {
		t.Fatalf("stderr contains %q %d times, want exactly once: %q", "early warn", n, errOut)
	}
	if n := strings.Count(errOut, "injected failure"); n != 1 {
		t.Fatalf("failure diagnostic count = %d, want exactly once across both mechanisms: %q", n, errOut)
	}
	// Collection order is preserved: the earlier diagnostic precedes the
	// controlled failure's.
	if strings.Index(errOut, "early warn") > strings.Index(errOut, "injected failure") {
		t.Fatalf("collection order violated in replayed stderr: %q", errOut)
	}
	if !strings.Contains(errOut, "vrg: ") || strings.Contains(errOut, "\x1b") {
		t.Fatalf("failure diagnostic is not a sanitized vrg: line: %q", errOut)
	}
	assertReplayedAfterRestore(t, r, "early warn")
	assertReplayedAfterRestore(t, r, "injected failure")
}

// TestReplayedDiagnosticEscapesFilename: a diagnostic embedding a
// filename containing a newline and an ESC byte — a load failure for a
// hostile matched path — replays single-lined and escaped, after
// restoration.
func TestReplayedDiagnosticEscapesFilename(t *testing.T) {
	dir := t.TempDir()
	ready := filepath.Join(dir, "ready")
	pidFile := filepath.Join(dir, "pid")
	reap := filepath.Join(dir, "reap")
	ack := filepath.Join(dir, "collect-ack")
	hostile := "evil\nname\x1b.txt"
	b64 := base64.StdEncoding.EncodeToString([]byte(hostile))
	rgDir := fakeRgPath(t, fmt.Sprintf(`#!/bin/sh
echo $$ > "$VRG_TEST_PID"
printf '%%s\n' '{"type":"begin","data":{"path":{"bytes":"%s"}}}'
printf '%%s\n' '{"type":"match","data":{"path":{"bytes":"%s"},"lines":{"text":"hit\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}'
printf '%%s\n' '{"type":"end","data":{"path":{"bytes":"%s"},"binary_offset":null}}'
printf '%%s\n' '{"type":"summary","data":{}}'
: > "$VRG_TEST_READY"
exit 0
`, b64, b64, b64))
	r := startVrgPTY(t, dir, childEnv(map[string]string{
		"PATH":                 rgDir + ":" + os.Getenv("PATH"),
		"TERM":                 "xterm-256color",
		"VRG_TEST_READY":       ready,
		"VRG_TEST_PID":         pidFile,
		"VRG_TEST_REAP":        reap,
		"VRG_TEST_COLLECT_ACK": ack,
	}), true, "foo", ".")
	waitFile(t, ready)
	killPidOnCleanup(t, pidFile)
	// The hostile path does not exist on disk, so its load fails and the
	// diagnostic is collected — the ack, not the render, is the signal.
	waitCollectAck(t, ack, "cannot read")
	r.send(t, "q")
	waitReplayedInputRestored(t, r, "cannot read")
	code := r.waitExit(t)
	r.finish(t)

	if code != 0 {
		t.Fatalf("exit status = %d, want 0", code)
	}
	assertTerminalRestored(t, r)
	assertReplayedAfterRestore(t, r, "cannot read")
	errOut := r.stderr.String()
	if !strings.Contains(errOut, `evil\nname^[.txt`) {
		t.Fatalf("replayed diagnostic lacks the escaped single-lined filename: %q", errOut)
	}
	if strings.ContainsAny(errOut, "\x1b\x9b") || strings.Count(strings.TrimSuffix(errOut, "\n"), "\n") != 0 {
		t.Fatalf("replayed diagnostic is not a sanitized single line: %q", errOut)
	}
}
