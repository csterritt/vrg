//go:build linux

package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// The unified shutdown contract for every program.Run() return shape
// (Issue #46): the VRG_TEST_RUN_* runner seam substitutes the
// (final model, error) tuple at the real Run() boundary after a true
// PTY lifecycle in which diagnostics were already collected — the
// collect-ack handshake proves collection, then q ends the real
// program and the overrides take effect. Every failing shape exits 2
// through the one shutdown sequence: terminal restoration, child
// termination and reap, then the ordered replay — session diagnostics
// in collection order, a diagnostic naming an absent or wrong-type
// final model when applicable, and the runtime error exactly once.
// Because the real model's searching-quit would exit 130, the observed
// 2 and the injected texts pin the substituted tuple, not a controlled
// model quit, as what reached the post-Run() branches.
func TestPTYRunReturnShapes(t *testing.T) {
	cases := []struct {
		name string
		env  []string
		want []string // the post-restoration replay tail, in order
	}{
		{
			name: "valid model with run error",
			env: []string{
				"VRG_TEST_RUN_FINAL_MODEL=valid",
				"VRG_TEST_RUN_ERROR=boom-runtime",
			},
			want: []string{"warn one", "vrg: boom-runtime"},
		},
		{
			name: "nil model with run error",
			env: []string{
				"VRG_TEST_RUN_FINAL_MODEL=nil",
				"VRG_TEST_RUN_ERROR=boom-runtime",
			},
			want: []string{
				"warn one",
				"vrg: program returned a nil final model",
				"vrg: boom-runtime",
			},
		},
		{
			name: "invalid model with run error",
			env: []string{
				"VRG_TEST_RUN_FINAL_MODEL=invalid",
				"VRG_TEST_RUN_ERROR=boom-runtime",
			},
			want: []string{
				"warn one",
				"vrg: program returned an unexpected final model",
				"vrg: boom-runtime",
			},
		},
		{
			name: "nil model without error",
			env:  []string{"VRG_TEST_RUN_FINAL_MODEL=nil"},
			want: []string{
				"warn one",
				"vrg: program returned a nil final model",
			},
		},
		{
			name: "invalid model without error",
			env:  []string{"VRG_TEST_RUN_FINAL_MODEL=invalid"},
			want: []string{
				"warn one",
				"vrg: program returned an unexpected final model",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fakeDir, capDir := writeFakeRg(t, fakeRgWarnBlockScript)
			ack := filepath.Join(capDir, "ack")
			reap := filepath.Join(capDir, "reap")
			events := filepath.Join(capDir, "events")
			env := append([]string{
				"VRG_TEST_COLLECT_ACK=" + ack,
				"VRG_TEST_EVENT_ACK=" + events,
				"VRG_TEST_REAP=" + reap,
			}, tc.env...)
			r := startVrgPTY(t, ptyEnv(fakeDir, capDir, env...), "foo")
			pid := awaitReadyPID(t, capDir)
			awaitAck(t, events, 1, "phase", "searching")
			r.waitFor(t, "Searching…")
			// The application-side acknowledgement proves the
			// diagnostic reached the session collection before the
			// keypress ends the real program.
			awaitAck(t, events, 1, "collected", "")
			awaitFileContent(t, ack)
			r.send(t, "q")
			// The real model's own quit commit lands before the seam
			// substitutes the Run() tuple — the observed 2 below can
			// only come from the override.
			awaitAck(t, events, 1, "quitting", "130")
			code, out := r.waitExit(t)
			if code != 2 {
				t.Fatalf("exit = %d, want 2; output: %q", code, out)
			}
			assertDisplayRestored(t, out)
			// Each expected line appears exactly once in the whole
			// stream — never both in a direct write and the replay —
			// and the post-restoration tail carries them in the
			// contract's order.
			tail := replayTail(t, out)
			prev := -1
			for _, w := range tc.want {
				if n := strings.Count(out, w); n != 1 {
					t.Fatalf("%q emitted %d times, want once: %q", w, n, out)
				}
				idx := strings.Index(tail, w)
				if idx < 0 {
					t.Fatalf("replay tail lacks %q: %q", w, tail)
				}
				if idx < prev {
					t.Fatalf("replay out of order — %q at %d follows %d: %q", w, idx, prev, tail)
				}
				prev = idx
			}
			// The child was terminated and reaped exactly as on a
			// normal exit: the side channel records the killed wait
			// status and the pid is fully gone.
			if got := readFile(t, reap); !strings.Contains(got, "killed") {
				t.Fatalf("reap side channel = %q, want the killed wait status", got)
			}
			pidIsGone(t, pid)
		})
	}
}
