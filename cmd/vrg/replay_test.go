//go:build unix

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// warnThenBlockRG emits one stderr diagnostic, records its pid, then
// blocks: the diagnostic is delivered while the child still runs, so it
// is the model's collection — not the child's write — the test proves.
const warnThenBlockRG = `
printf 'warn one\n' >&2
echo $$ > "$VRG_TEST_RG_PID"
exec sleep 600
`

// warnStreamRG emits two stderr diagnostics ahead of a complete valid
// stream and exits 0.
const warnStreamRG = `
printf 'warn one\nwarn two\n' >&2
` + happyStreamRG

// hostileNameRG reports one match for a path whose bytes carry ESC and
// LF; no such file exists on disk, so the browse load fails and the
// diagnostic embeds the hostile name.
const hostileNameRG = `
cat <<'EOF'
{"type":"begin","data":{"path":{"text":"a\u001bb\nc.txt"}}}
{"type":"match","data":{"path":{"text":"a\u001bb\nc.txt"},"lines":{"text":"x f\n"},"line_number":1,"absolute_offset":0,"submatches":[{"match":{"text":"f"},"start":2,"end":3}]}}
{"type":"end","data":{"path":{"text":"a\u001bb\nc.txt"},"binary_offset":null,"stats":{}}}
{"type":"summary","data":{"stats":{}}}
EOF
`

// waitForAcks polls until the diag-acknowledgement file holds n lines —
// proof that n diagnostics were processed into the session collection,
// not merely written by the child. Same mechanism family as the reap
// evidence: a file the tested process writes.
func waitForAcks(t *testing.T, path string, n int) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if b, err := os.ReadFile(path); err == nil && strings.Count(string(b), "\n") >= n {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	b, _ := os.ReadFile(path)
	t.Fatalf("diag acknowledgement never reached %d lines at %s: %q", n, path, b)
}

// assertReplayedOnce asserts a never-displayed diagnostic: exactly one
// occurrence in the whole capture, strictly after the display
// restoration — never emitted by the live TUI, never duplicated.
func assertReplayedOnce(t *testing.T, out, diag string) {
	t.Helper()
	i := strings.Index(out, exitAltScreen)
	if i < 0 {
		t.Fatalf("display restoration missing; output: %q", out)
	}
	if strings.Contains(out[:i], diag) {
		t.Fatalf("diagnostic appeared before display restoration: %q", out)
	}
	if n := strings.Count(out[i:], diag); n != 1 {
		t.Fatalf("diagnostic replayed %d times after restoration, want exactly once: %q", n, out)
	}
}

// assertReplayOrder asserts each diagnostic appears exactly once after
// the restoration sequence, in collection order — for diagnostics the
// TUI may also have shown, where pre-restoration occurrences are the
// overlay's own frames, not the replay.
func assertReplayOrder(t *testing.T, out string, diags []string) {
	t.Helper()
	i := strings.Index(out, exitAltScreen)
	if i < 0 {
		t.Fatalf("display restoration missing; output: %q", out)
	}
	post := out[i:]
	last := -1
	for _, d := range diags {
		if n := strings.Count(post, d); n != 1 {
			t.Fatalf("diagnostic %q replayed %d times after restoration, want exactly once: %q", d, n, out)
		}
		j := strings.Index(post, d)
		if j < last {
			t.Fatalf("replayed diagnostics out of collection order: %q", out)
		}
		last = j
	}
}

// q and ctrl+c after a processed diagnostic — while the child still
// runs — are cancellation: exit 130, termios restored, and the
// diagnostic the model acknowledged collecting replays to stderr
// exactly once, after the display-restoration sequence. The wait is on
// the application-side acknowledgement, not a child-side handshake.
func TestCancelReplaysProcessedDiagnostic(t *testing.T) {
	for _, tc := range []struct{ name, key string }{
		{"q", "q"},
		{"ctrl+c", "\x03"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			pidFile := filepath.Join(dir, "pid")
			ack := filepath.Join(dir, "diag.ack")
			reapFile := filepath.Join(dir, "reap")
			fakebin := fakeRG(t, warnThenBlockRG)
			env := testEnv(fakebin,
				"VRG_TEST_RG_PID="+pidFile,
				"VRG_TEST_DIAG_ACK="+ack,
				"VRG_TEST_REAP="+reapFile)

			s := startVrgTermPTY(t, dir, env, "foo")
			waitForAcks(t, ack, 1) // processed into the collection
			s.send(tc.key)
			code := s.waitExit()
			out := s.output()
			if code != 130 {
				t.Fatalf("exit = %d, want 130; output: %q", code, out)
			}
			assertDisplayRestored(t, out)
			assertReplayedOnce(t, out, "warn one")
			s.assertTermiosRestored()
			assertChildGone(t, pidFile)
			if ev := reapEvidence(t, reapFile); !strings.Contains(ev, "code=-1") {
				t.Fatalf("reap evidence = %q, want the killed child's wait status", ev)
			}
		})
	}
}

// q while index preparation is gate-held — the fake rg already exited
// after emitting a stderr diagnostic — is the same cancellation
// boundary: the acknowledged diagnostic replays exactly once after
// restoration, exit 130, and the exit waits on no undelivered work.
func TestQDuringGateHeldPreparationReplaysDiagnostic(t *testing.T) {
	dir := t.TempDir()
	gate := filepath.Join(dir, "gate")
	ack := filepath.Join(dir, "diag.ack")
	reapFile := filepath.Join(dir, "reap")
	if err := os.WriteFile(gate, []byte("held"), 0o644); err != nil {
		t.Fatal(err)
	}
	fakebin := fakeRG(t, `printf 'warn one\n' >&2`+happyStreamRG)
	env := testEnv(fakebin,
		"VRG_TEST_GATE="+gate,
		"VRG_TEST_DIAG_ACK="+ack,
		"VRG_TEST_REAP="+reapFile)

	s := startVrgTermPTY(t, dir, env, "foo")
	waitForAcks(t, ack, 1) // collected while the gate still holds
	s.send("q")
	code := s.waitExit()
	out := s.output()
	if code != 130 {
		t.Fatalf("exit = %d, want 130 (cancellation, not browse quit); output: %q", code, out)
	}
	assertDisplayRestored(t, out)
	assertReplayedOnce(t, out, "warn one")
	s.assertTermiosRestored()
	if ev := reapEvidence(t, reapFile); !strings.Contains(ev, "code=0") {
		t.Fatalf("reap evidence = %q, want the exited child's wait status", ev)
	}
}

// A normal quit after a completed stream replays the collected
// diagnostics — the same lines the warning overlay displayed — exactly
// once each, in collection order, after the restoration sequence.
func TestNormalQuitReplaysDiagnosticsInOrder(t *testing.T) {
	dir := t.TempDir()
	ack := filepath.Join(dir, "diag.ack")
	// The stream's files exist on disk so their loads add no
	// diagnostics of their own.
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("alpha\nalpha again\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.go"), []byte("beta alpha\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fakebin := fakeRG(t, warnStreamRG)
	env := testEnv(fakebin, "VRG_TEST_DIAG_ACK="+ack)

	s := startVrgTermPTY(t, dir, env, "foo")
	waitForAcks(t, ack, 2)
	if !s.waitFor("warn one") {
		s.cmd.Process.Kill()
		<-s.done
		t.Fatalf("warning overlay never appeared; output: %q", s.output())
	}
	s.send("q") // dismiss the warning overlay
	s.send("q") // ordinary browse quit — each keypress is a discrete message
	code := s.waitExit()
	out := s.output()
	if code != 0 {
		t.Fatalf("exit = %d, want 0; output: %q", code, out)
	}
	assertDisplayRestored(t, out)
	assertReplayOrder(t, out, []string{"warn one", "warn two"})
	s.assertTermiosRestored()
}

// The injected controlled failure enters the session collection: the
// already-acknowledged diagnostic replays first, the failure line
// second, each exactly once across the whole capture — the common
// post-restoration writer replaced the separate direct write.
func TestControlledFailureReplaysViaCollection(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "pid")
	ack := filepath.Join(dir, "diag.ack")
	reapFile := filepath.Join(dir, "reap")
	trigger := filepath.Join(dir, "fail")
	fakebin := fakeRG(t, warnThenBlockRG)
	env := testEnv(fakebin,
		"VRG_TEST_RG_PID="+pidFile,
		"VRG_TEST_DIAG_ACK="+ack,
		"VRG_TEST_REAP="+reapFile,
		"VRG_TEST_FAIL="+trigger)

	s := startVrgTermPTY(t, dir, env, "foo")
	waitForAcks(t, ack, 1) // "warn one" is collected
	if err := os.WriteFile(trigger, []byte("go"), 0o644); err != nil {
		t.Fatal(err)
	}
	code := s.waitExit()
	out := s.output()
	if code != 2 {
		t.Fatalf("exit = %d, want 2; output: %q", code, out)
	}
	assertDisplayRestored(t, out)
	assertReplayOrder(t, out, []string{"warn one", "vrg: injected test failure ^[[7m"})
	if n := strings.Count(out, "injected test failure"); n != 1 {
		t.Fatalf("failure diagnostic appears %d times across all mechanisms, want exactly once: %q", n, out)
	}
	s.assertTermiosRestored()
	assertChildGone(t, pidFile)
	reapEvidence(t, reapFile)
}

// A filename carrying ESC and LF bytes is embedded in a vrg diagnostic
// already escaped and single-lined: the failed browse load replays
// "cannot read <escaped>: <reason>" as one line after restoration, and
// the raw hostile bytes never reach the terminal.
func TestReplayEscapesHostileFilename(t *testing.T) {
	dir := t.TempDir()
	ack := filepath.Join(dir, "diag.ack")
	fakebin := fakeRG(t, hostileNameRG)
	env := testEnv(fakebin, "VRG_TEST_DIAG_ACK="+ack)

	s := startVrgTermPTY(t, dir, env, "foo")
	waitForAcks(t, ack, 1) // the load failure is collected
	if !s.waitFor("─ ") {
		s.cmd.Process.Kill()
		<-s.done
		t.Fatalf("browse view never appeared; output: %q", s.output())
	}
	s.send("q")
	code := s.waitExit()
	out := s.output()
	if code != 0 {
		t.Fatalf("exit = %d, want 0; output: %q", code, out)
	}
	assertDisplayRestored(t, out)
	post := out[strings.Index(out, exitAltScreen):]
	want := `a^[b\nc.txt: no such file or directory`
	if n := strings.Count(post, want); n != 1 {
		t.Fatalf("escaped single-line diagnostic replayed %d times, want exactly once: %q", n, out)
	}
	if strings.Contains(post, "b\nc.txt") {
		t.Fatalf("the filename's raw newline forged a diagnostic line boundary: %q", out)
	}
	if i := strings.Index(post, want); strings.ContainsAny(post[i:i+len(want)], "\x1b") {
		t.Fatalf("replayed diagnostic carries a raw ESC: %q", out)
	}
	s.assertTermiosRestored()
}
