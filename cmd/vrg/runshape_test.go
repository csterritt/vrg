//go:build unix

package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// invalidFinalModelDiag is the generated diagnostic naming the
// absent/wrong-type final-model condition: the single line the unified
// shutdown sequence emits between the retained session diagnostics and
// the runtime error, so no Run() return shape exits silently.
const invalidFinalModelDiag = "vrg: program ended without a valid final model"

// warnTwiceThenBlockRG emits two stderr diagnostics ahead of its pid
// record, then blocks: both diagnostics are collected while the child
// still runs, so the replay proves collection order, not just presence.
const warnTwiceThenBlockRG = `
printf 'warn one\nwarn two\n' >&2
echo $$ > "$FAKE_RG_PID_FILE"
exec sleep 600
`

// Issue #46's return-shape matrix: the tagged runner substitutes each
// (final model, error) tuple at the executable's real program.Run()
// boundary — after a real PTY lifecycle in which two diagnostics were
// acknowledged as collected — so the assertions below can only be met
// by the post-Run() type/error branches, never by a controlled model
// quit (which would exit 130 here). Every failing shape exits 2 through
// the single ordered shutdown sequence: session diagnostics in
// collection order, then the invalid-final-model diagnostic when the
// model is absent or the wrong type, then the runtime error exactly
// once — all strictly after terminal restoration, with the child
// terminated and reaped exactly as on normal exits.
func TestRunReturnShapesShutdownReplay(t *testing.T) {
	const runErr = "vrg-injected-run-error"
	cases := []struct {
		name       string
		finalModel string // VRG_TEST_RUN_FINAL_MODEL
		runErr     string // VRG_TEST_RUN_ERROR
		wantCode   int
		wantReplay []string // the ordered post-restoration replay
	}{
		// The control: no effective override, so the real quit's
		// cancellation survives — exit 130. The failing cells below
		// diverge from this in ways only the post-Run() branches can
		// produce, proving the injected tuple reached them.
		{"valid final model, nil error", "valid", "", 130,
			[]string{"warn one", "warn two"}},
		{"valid final model, run error", "valid", runErr, 2,
			[]string{"warn one", "warn two", "vrg: " + runErr}},
		{"nil final model, run error", "nil", runErr, 2,
			[]string{"warn one", "warn two", invalidFinalModelDiag, "vrg: " + runErr}},
		{"invalid final model, run error", "invalid", runErr, 2,
			[]string{"warn one", "warn two", invalidFinalModelDiag, "vrg: " + runErr}},
		{"nil final model, nil error", "nil", "", 2,
			[]string{"warn one", "warn two", invalidFinalModelDiag}},
		{"invalid final model, nil error", "invalid", "", 2,
			[]string{"warn one", "warn two", invalidFinalModelDiag}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			pidFile := filepath.Join(dir, "pid")
			ack := filepath.Join(dir, "diag.ack")
			reapFile := filepath.Join(dir, "reap")
			fakebin := fakeRG(t, warnTwiceThenBlockRG)
			env := testEnv(fakebin,
				"FAKE_RG_PID_FILE="+pidFile,
				"VRG_TEST_DIAGNOSTIC_TRIGGER="+ack,
				"VRG_TEST_REAP="+reapFile,
				"VRG_TEST_RUN_FINAL_MODEL="+tc.finalModel,
				"VRG_TEST_RUN_ERROR="+tc.runErr,
				ackEnv(t))

			s := startVrgTermPTY(t, dir, env, "foo")
			// Both diagnostics are processed into the session
			// collection before the exit key — the injected return
			// shape replaces the result of a session that collected
			// them, not an empty one.
			waitForAcks(t, ack, 2)
			s.send("q") // a real quit: Run() returns, then the runner substitutes the tuple
			s.waitAck(t, keyEvent("q"), 0)
			code := s.waitExit()
			out := s.output()
			if code != tc.wantCode {
				t.Fatalf("exit = %d, want %d; output: %q", code, tc.wantCode, out)
			}
			// The TUI genuinely ran, and cleanup matched a normal
			// exit: display restoration emitted, pty input modes back
			// to their pre-launch state, the blocked child terminated
			// and reaped through vrg's own wait path.
			if !strings.Contains(out, "Searching") {
				t.Fatalf("the searching lifecycle never ran; output: %q", out)
			}
			assertDisplayRestored(t, out)
			s.assertTermiosRestored()
			assertChildGone(t, pidFile)
			if ev := reapEvidence(t, reapFile); !strings.Contains(ev, "code=-1") {
				t.Fatalf("reap evidence = %q, want the killed child's wait status", ev)
			}
			// One deterministic replay sequence strictly after the
			// restoration sequence, each line exactly once — a direct
			// write plus the replay would double-count here.
			assertReplayOrder(t, out, tc.wantReplay)
			for _, line := range tc.wantReplay {
				if n := strings.Count(out, line); n != 1 {
					t.Fatalf("replay line %q appears %d times across the whole capture, want exactly once: %q",
						line, n, out)
				}
			}
		})
	}
}
