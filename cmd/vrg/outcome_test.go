package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// keyStep is one scripted pty interaction in an outcome test: send key
// ("" sends nothing), then wait until expect appears in output written
// since the step began — proving a fresh repaint of whatever the
// overlay covered rather than matching an old frame's bytes. ack, when
// set, is the additional application-side acknowledgement the step
// waits on after its key is processed — e.g. overlay:open or
// overlay:dismissed — before the expected repaint.
type keyStep struct {
	key    string
	expect string
	ack    string
}

// waitForFrom polls until needle appears in output written at or after
// off — a repaint of previously covered content — bounded like waitFor.
func (s *ptySession) waitForFrom(off int, needle string) bool {
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		out := s.output()
		if off > len(out) {
			off = len(out)
		}
		if strings.Contains(out[off:], needle) {
			return true
		}
		select {
		case code := <-s.done:
			s.done <- code // leave it for waitExit
			return false
		default:
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

// runSteps performs the scripted interactions against a started
// session: each step sends its key, waits on the application-side
// acknowledgement that the key was processed — and on the step's named
// acknowledgement when it has one — then waits for its marker in the
// output the key provoked. Every acknowledgement wait is correlated
// per occurrence against the baseline snapshotted before the send.
func runSteps(t *testing.T, s *ptySession, steps []keyStep) {
	t.Helper()
	if s.ackPath == "" {
		t.Fatal("runSteps requires the event-acknowledgement log (VRG_TEST_EVENT_ACK)")
	}
	for i, st := range steps {
		off := len(s.output())
		keyBase, ackBase := 0, 0
		if st.key != "" {
			keyBase = s.ackCount(keyEvent(st.key))
		}
		if st.ack != "" {
			ackBase = s.ackCount(st.ack)
		}
		if st.key != "" {
			s.send(st.key)
			s.waitAck(t, keyEvent(st.key), keyBase)
		}
		if st.ack != "" {
			s.waitAck(t, st.ack, ackBase)
		}
		if st.expect == "" {
			continue
		}
		if !s.waitForFrom(off, st.expect) {
			s.cmd.Process.Kill()
			<-s.done
			t.Fatalf("step %d: %q never appeared; output: %q",
				i, st.expect, s.output())
		}
	}
}

// runVrgWithKeys starts vrg on a pty against the fake rg in env with a
// fresh event-acknowledgement log, drives the outcome steps, and
// returns the captured output and exit code.
func runVrgWithKeys(t *testing.T, dir string, env []string, steps []keyStep, args ...string) (string, int) {
	t.Helper()
	env = append(append([]string(nil), env...), ackEnv(t))
	s := startVrgPTY(t, dir, env, args...)
	runSteps(t, s, steps)
	return s.output(), s.waitExit()
}

// runVrgKillChild starts vrg against a fake rg that blocks mid-stream,
// SIGKILLs the child once its ready file lands, then drives the steps —
// the process dies by signal while its stream stays incomplete.
func runVrgKillChild(t *testing.T, dir string, env []string, pidFile, ready string, steps []keyStep, args ...string) (string, int) {
	t.Helper()
	env = append(append([]string(nil), env...), ackEnv(t))
	s := startVrgPTY(t, dir, env, args...)
	waitForFile(t, ready)
	data, err := os.ReadFile(pidFile)
	if err != nil {
		s.cmd.Process.Kill()
		<-s.done
		t.Fatalf("rg pid file: %v", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		s.cmd.Process.Kill()
		<-s.done
		t.Fatalf("rg pid %q: %v", data, err)
	}
	if err := syscall.Kill(pid, syscall.SIGKILL); err != nil {
		s.cmd.Process.Kill()
		<-s.done
		t.Fatalf("kill rg %d: %v", pid, err)
	}
	runSteps(t, s, steps)
	return s.output(), s.waitExit()
}

// writeOutcomeFile writes the 20-line fixture file the outcome PTY
// tests search: real content on disk so the browse panel under the
// overlay repaints a recognizable line when the overlay dismisses.
func writeOutcomeFile(t *testing.T, dir string) {
	t.Helper()
	var sb strings.Builder
	for i := 1; i <= 20; i++ {
		fmt.Fprintf(&sb, "alpha line %d\n", i)
	}
	if err := os.WriteFile(filepath.Join(dir, "f"), []byte(sb.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

// fatalExitRG emits two valid matches for ./f, writes a stderr
// diagnostic, marks the handshake after both pipes have drained, then
// exits non-zero.
const fatalExitRG = `
cat <<'EOF'
{"type":"begin","data":{"path":{"text":"./f"}}}
{"type":"match","data":{"path":{"text":"./f"},"lines":{"text":"alpha line 2\n"},"line_number":2,"absolute_offset":0,"submatches":[{"match":{"text":"alpha"},"start":0,"end":5}]}}
{"type":"match","data":{"path":{"text":"./f"},"lines":{"text":"alpha line 5\n"},"line_number":5,"absolute_offset":0,"submatches":[{"match":{"text":"alpha"},"start":0,"end":5}]}}
{"type":"end","data":{"path":{"text":"./f"},"binary_offset":null,"stats":{}}}
{"type":"summary","data":{"stats":{}}}
EOF
printf 'boom\n' >&2
[ -n "$FAKE_RG_HANDSHAKE_FILE" ] && : > "$FAKE_RG_HANDSHAKE_FILE"
exit 3
`

// killedRG emits an unclosed stream — a begin and a match with no end
// or summary — records its pid, then blocks: the process dies mid-
// stream by whatever signal the test delivers.
const killedRG = `
printf '%s\n' '{"type":"begin","data":{"path":{"text":"./f"}}}'
printf '%s\n' '{"type":"match","data":{"path":{"text":"./f"},"lines":{"text":"alpha line 2\n"},"line_number":2,"absolute_offset":0,"submatches":[{"match":{"text":"alpha"},"start":0,"end":5}]}}'
echo $$ > "$FAKE_RG_PID_FILE"
: > "$FAKE_RG_READY_FILE"
exec sleep 600
`

// warnSummaryRG writes a nonfatal stderr diagnostic and a complete
// summary-only stream, exiting 1 like a matchless rg.
const warnSummaryRG = `
printf 'warn\n' >&2
printf '%s\n' '{"type":"summary","data":{"stats":{}}}'
exit 1
`

// A fake rg exiting non-zero after its handshake — two retained matches
// plus stderr "boom" — opens the error overlay over the browse view;
// Esc dismisses to the browse frame, and q then exits 2.
func TestFatalExitWithResultsShowsOverlay(t *testing.T) {
	dir := t.TempDir()
	writeOutcomeFile(t, dir)
	handshake := filepath.Join(dir, "done")
	fakebin := fakeRG(t, fatalExitRG)
	env := testEnv(fakebin, "FAKE_RG_HANDSHAKE_FILE="+handshake)

	out, code := runVrgWithKeys(t, dir, env, []keyStep{
		{expect: "boom", ack: "overlay:open"},                            // overlay opens with the stderr diagnostic
		{key: "\x1b", expect: "alpha line 11", ack: "overlay:dismissed"}, // Esc dismisses; covered content repaints
		{key: "q"},
	}, "foo")
	if code != 2 {
		t.Fatalf("exit = %d, want 2; output: %q", code, out)
	}
	if !strings.Contains(out, "./f") {
		t.Fatalf("browse frame missing the filename rule: %q", out)
	}
	if _, err := os.Stat(handshake); err != nil {
		t.Fatalf("handshake missing: rg did not exit 3 only after finishing both pipes: %v", err)
	}
}

// A failed process with no stderr gets a generated diagnostic naming
// the exit code; with no usable results the fatal overlay-only
// presentation exits on q.
func TestFatalExitNoOutputNamesExitCode(t *testing.T) {
	dir := t.TempDir()
	fakebin := fakeRG(t, `exit 2`)
	out, code := runVrgWithKeys(t, dir, testEnv(fakebin), []keyStep{
		{expect: "code 2", ack: "overlay:open"},
		{key: "q"},
	}, "foo")
	if code != 2 {
		t.Fatalf("exit = %d, want 2; output: %q", code, out)
	}
}

// The same failed process's overlay-only presentation exits on Esc too
// — the one state where Esc terminates, because there is no underlying
// screen to reveal.
func TestFatalExitNoOutputEscExits2(t *testing.T) {
	dir := t.TempDir()
	fakebin := fakeRG(t, `exit 2`)
	out, code := runVrgWithKeys(t, dir, testEnv(fakebin), []keyStep{
		{expect: "code 2", ack: "overlay:open"},
		{key: "\x1b"},
	}, "foo")
	if code != 2 {
		t.Fatalf("exit = %d, want 2; output: %q", code, out)
	}
}

// SIGKILL mid-stream: the child's signal death and the incomplete
// stream are both fatal; the overlay names the signal, Esc reveals the
// retained match in browse, and q exits 2.
func TestSignalDeathNamesSignal(t *testing.T) {
	dir := t.TempDir()
	writeOutcomeFile(t, dir)
	pidFile := filepath.Join(dir, "rg.pid")
	ready := filepath.Join(dir, "rg.ready")
	fakebin := fakeRG(t, killedRG)
	env := testEnv(fakebin, "FAKE_RG_PID_FILE="+pidFile, "FAKE_RG_READY_FILE="+ready)

	out, code := runVrgKillChild(t, dir, env, pidFile, ready, []keyStep{
		{expect: "killed", ack: "overlay:open"},
		{key: "\x1b", expect: "alpha line 11", ack: "overlay:dismissed"},
		{key: "q"},
	}, "foo")
	if code != 2 {
		t.Fatalf("exit = %d, want 2; output: %q", code, out)
	}
}

// A warning on stderr with a complete summary-only stream: the warning
// overlay dismisses to the no-results screen and q exits 1.
func TestStderrWarningWithSummaryShowsWarningOverlay(t *testing.T) {
	dir := t.TempDir()
	fakebin := fakeRG(t, warnSummaryRG)
	out, code := runVrgWithKeys(t, dir, testEnv(fakebin), []keyStep{
		{expect: "warn", ack: "overlay:open"},
		{key: "\x1b", expect: "No results found", ack: "overlay:dismissed"},
		{key: "q"},
	}, "foo")
	if code != 1 {
		t.Fatalf("exit = %d, want 1; output: %q", code, out)
	}
}

// The stderr-content fixture: more than 1 MiB of stderr interleaved
// with a valid stdout stream. The stream survives complete — all 18
// recorded matches reach the browse frame — and the captured stderr
// heads the scrollable diagnostic the overlay opens with. (The model
// test asserts the diagnostic's tail is in the scrollable set; Issue
// #41 supersedes showing both ends in one frame.)
func TestStderrContentFixture(t *testing.T) {
	dir := t.TempDir()
	var content strings.Builder
	for i := 0; i < 18; i++ {
		content.WriteString("x f\n")
	}
	if err := os.WriteFile(filepath.Join(dir, "f"), []byte(content.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	handshake := filepath.Join(dir, "done")
	fakebin := fakeRG(t, `
printf 'STDERR-HEAD\n' >&2
printf '%s\n' '{"type":"begin","data":{"path":{"text":"./f"}}}'
i=0
while [ $i -lt 20 ]; do
    dd if=/dev/zero bs=65536 count=1 2>/dev/null | tr '\000' 'e' >&2
    printf '%s\n' '{"type":"match","data":{"path":{"text":"./f"},"lines":{"text":"x f\n"},"line_number":'$((i + 1))',"absolute_offset":0,"submatches":[{"match":{"text":"f"},"start":2,"end":3}]}}'
    i=$((i + 1))
done
printf '%s\n' '{"type":"end","data":{"path":{"text":"./f"},"binary_offset":null,"stats":{}}}'
printf '%s\n' '{"type":"summary","data":{"stats":{}}}'
printf 'STDERR-TAIL\n' >&2
[ -n "$FAKE_RG_HANDSHAKE_FILE" ] && : > "$FAKE_RG_HANDSHAKE_FILE"
`)
	env := testEnv(fakebin, "FAKE_RG_HANDSHAKE_FILE="+handshake, ackEnv(t))

	s := startVrgPTY(t, dir, env, "foo")
	s.waitAck(t, "overlay:open", 0)
	if !s.waitFor("STDERR-HEAD") {
		s.cmd.Process.Kill()
		<-s.done
		t.Fatalf("captured stderr never reached the overlay; output: %q", s.output())
	}
	off := len(s.output())
	s.send("\x1b") // dismiss the warning overlay to the browse view
	s.waitAck(t, "overlay:dismissed", 0)
	if !s.waitForFrom(off, "18  x ") {
		s.cmd.Process.Kill()
		<-s.done
		t.Fatalf("browse frame never repainted after dismissal; output: %q", s.output())
	}
	s.send("q")
	s.waitAck(t, keyEvent("q"), 0)
	out, code := s.output(), s.waitExit()
	if code != 0 {
		t.Fatalf("exit = %d, want 0; output: %q", code, out)
	}
	// Every recorded match survives as an inverse-video span in the
	// final content frame — the stdout stream is complete despite the
	// stderr flood beside it.
	last := strings.LastIndex(out, " 1  x ")
	if last < 0 {
		t.Fatalf("content frame missing: %q", out)
	}
	if n := strings.Count(out[last:], "30;47"); n != 18 {
		t.Fatalf("inverse-video matches = %d, want 18 — records lost", n)
	}
	if _, err := os.Stat(handshake); err != nil {
		t.Fatalf("handshake missing: child could not finish writing both pipes: %v", err)
	}
}
