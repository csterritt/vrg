package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/creack/pty"
)

// This file drives the Issue 9 outcome rows at the real subprocess
// boundary: fake rg scripts that exit nonzero after a handshake or emit
// a stderr flood interleaved with a valid stdout stream, asserted
// through the PTY's observable frames and the process exit status.

// TestFakeRgNonZeroExitOverResults: fake rg emits a complete valid
// stream, writes a stderr line, signals ready, and exits 3. The error
// overlay opens over the browse view; Esc dismisses to browse — the row
// the box covered becomes visible — and q exits 2.
func TestFakeRgNonZeroExitOverResults(t *testing.T) {
	dir := t.TempDir()
	ready := filepath.Join(dir, "ready")
	pidFile := filepath.Join(dir, "pid")
	reap := filepath.Join(dir, "reap")
	events := filepath.Join(dir, "events")
	var content strings.Builder
	for i := 1; i <= 20; i++ {
		fmt.Fprintf(&content, "hit %02d\n", i)
	}
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte(content.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	rgDir := fakeRgPath(t, `#!/bin/sh
echo $$ > "$FAKE_RG_PID_FILE"
printf '%s\n' '{"type":"begin","data":{"path":{"text":"file.txt"}}}'
printf '%s\n' '{"type":"match","data":{"path":{"text":"file.txt"},"lines":{"text":"hit 01\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}'
printf '%s\n' '{"type":"end","data":{"path":{"text":"file.txt"},"binary_offset":null}}'
printf '%s\n' '{"type":"summary","data":{}}'
printf '%s\n' 'boom' >&2
: > "$FAKE_RG_READY_FILE"
exit 3
`)
	r := startVrgPTY(t, dir, childEnv(map[string]string{
		"PATH":               rgDir + ":" + os.Getenv("PATH"),
		"TERM":               "xterm-256color",
		"FAKE_RG_READY_FILE": ready,
		"FAKE_RG_PID_FILE":   pidFile,
		"VRG_TEST_REAP":      reap,
		"VRG_TEST_ACK":       events,
	}), false, "hit", ".")
	waitFile(t, ready)
	killPidOnCleanup(t, pidFile)

	r.waitAck(t, 0, "state", "browse")
	r.waitAck(t, 0, "overlay", "open")
	r.waitOutput(t, "boom")   // the error overlay is open
	r.waitOutput(t, "hit 09") // the file loaded; the row above the box shows
	if strings.Contains(r.output(), "hit 10") {
		t.Fatal("a row under the overlay box was visible before dismissal")
	}
	mark := r.sendAcked(t, "\x1b", "esc") // Esc dismisses the overlay to browse
	r.waitAck(t, mark, "overlay", "dismissed")
	r.waitOutput(t, "hit 10") // the covered row is revealed
	r.sendAcked(t, "q", "q")
	code := r.waitExit(t)
	r.finish(t)

	if code != 2 {
		t.Fatalf("exit status = %d, want 2", code)
	}
	if got := reapStatus(t, reap); got != "exit status 3" {
		t.Fatalf("reaped wait status = %q, want %q", got, "exit status 3")
	}
	assertTerminalRestored(t, r)
}

// TestStderrContentFixture: fake rg emits at least 1 MiB of stderr — a
// marked head line, 64 blocks of 16385 bytes, then a marked tail line —
// interleaved with a complete valid stdout stream, and exits 0. The
// warning overlay shows the head; growing the terminal far past the
// diagnostic's wrapped row count reveals the tail, proving the complete
// captured diagnostic is in the overlay's scrollable model (a single
// 80x24 frame need not show both ends — Issue 41). Dismissal returns to
// browse and q exits 0, because the stdout stream was complete.
func TestStderrContentFixture(t *testing.T) {
	dir := t.TempDir()
	ready := filepath.Join(dir, "ready")
	pidFile := filepath.Join(dir, "pid")
	reap := filepath.Join(dir, "reap")
	events := filepath.Join(dir, "events")
	var content strings.Builder
	for i := 1; i <= 64; i++ {
		fmt.Fprintf(&content, "hit %02d\n", i)
	}
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte(content.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	rgDir := fakeRgPath(t, `#!/bin/sh
echo $$ > "$FAKE_RG_PID_FILE"
printf '%s\n' 'VRG-STDERR-HEAD' >&2
block=$(printf '%016385d' 0)
printf '%s\n' '{"type":"begin","data":{"path":{"text":"file.txt"}}}'
i=1
while [ "$i" -le 64 ]; do
	printf '%s\n' "$block" >&2
	printf '%s\n' '{"type":"match","data":{"path":{"text":"file.txt"},"lines":{"text":"hit\n"},"line_number":'"$i"',"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}'
	i=$((i+1))
done
printf '%s\n' 'VRG-STDERR-TAIL' >&2
printf '%s\n' '{"type":"end","data":{"path":{"text":"file.txt"},"binary_offset":null}}'
printf '%s\n' '{"type":"summary","data":{}}'
: > "$FAKE_RG_READY_FILE"
exit 0
`)
	r := startVrgPTY(t, dir, childEnv(map[string]string{
		"PATH":               rgDir + ":" + os.Getenv("PATH"),
		"TERM":               "xterm-256color",
		"FAKE_RG_READY_FILE": ready,
		"FAKE_RG_PID_FILE":   pidFile,
		"VRG_TEST_REAP":      reap,
		"VRG_TEST_ACK":       events,
	}), false, "hit", ".")
	waitFile(t, ready)
	killPidOnCleanup(t, pidFile)

	r.waitOutput(t, "VRG-STDERR-HEAD") // the warning overlay is open at its head
	// The 64 wrapped 16 KiB blocks need ~13k interior rows; enlarge the
	// terminal well past that so the tail row is rendered too. The
	// resize is acknowledged by the model's msg record before the
	// rendered tail is awaited.
	mark := r.ackMark(t)
	if err := pty.Setsize(r.master, &pty.Winsize{Rows: 20000, Cols: 80}); err != nil {
		t.Fatalf("pty.Setsize: %v", err)
	}
	r.waitAck(t, mark, "msg", "tea.WindowSizeMsg")
	r.waitOutput(t, "VRG-STDERR-TAIL")
	// Two q presses: the first dismisses the warning overlay — its
	// dismissal acknowledgement is awaited before the second — and the
	// second quits browse.
	mark = r.sendAcked(t, "q", "q")
	r.waitAck(t, mark, "overlay", "dismissed")
	r.sendAcked(t, "q", "q")
	code := r.waitExit(t)
	r.finish(t)

	if code != 0 {
		t.Fatalf("exit status = %d, want 0: a clean exit over a complete stream", code)
	}
	if got := reapStatus(t, reap); got != "exit status 0" {
		t.Fatalf("reaped wait status = %q, want %q", got, "exit status 0")
	}
	assertTerminalRestored(t, r)
}

// TestFakeRgSilentExitGeneratesDiagnostic: fake rg emits nothing and
// exits 2 after the handshake; with no stderr to show, the error overlay
// carries a generated diagnostic naming the exit code, and q exits 2.
func TestFakeRgSilentExitGeneratesDiagnostic(t *testing.T) {
	dir := t.TempDir()
	ready := filepath.Join(dir, "ready")
	pidFile := filepath.Join(dir, "pid")
	reap := filepath.Join(dir, "reap")
	events := filepath.Join(dir, "events")
	rgDir := fakeRgPath(t, `#!/bin/sh
echo $$ > "$FAKE_RG_PID_FILE"
: > "$FAKE_RG_READY_FILE"
exit 2
`)
	r := startVrgPTY(t, dir, childEnv(map[string]string{
		"PATH":               rgDir + ":" + os.Getenv("PATH"),
		"TERM":               "xterm-256color",
		"FAKE_RG_READY_FILE": ready,
		"FAKE_RG_PID_FILE":   pidFile,
		"VRG_TEST_REAP":      reap,
		"VRG_TEST_ACK":       events,
	}), false, "hit", ".")
	waitFile(t, ready)
	killPidOnCleanup(t, pidFile)

	r.waitAck(t, 0, "state", "fatal")
	r.waitOutput(t, "code 2") // the generated diagnostic names the exit code
	r.sendAcked(t, "q", "q")
	code := r.waitExit(t)
	r.finish(t)

	if code != 2 {
		t.Fatalf("exit status = %d, want 2", code)
	}
	if got := reapStatus(t, reap); got != "exit status 2" {
		t.Fatalf("reaped wait status = %q, want %q", got, "exit status 2")
	}
	assertTerminalRestored(t, r)
}
