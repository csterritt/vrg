package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// This file drives the Issue 46 unified-shutdown contract at the real
// subprocess boundary: the tagged runner seam injects each
// final-model/error tuple at the executable's actual program.Run()
// return site after a real PTY lifecycle in which a diagnostic was
// collected. Every shape shares one ordered shutdown: the program
// returns after Bubble Tea restores the terminal, the child is
// terminated and reaped exactly as on normal exits, and only then does
// the replay emit — session diagnostics in collection order, then a
// diagnostic naming an absent or wrong-type final model when
// applicable, then the runtime error exactly once. Every failing shape
// is a controlled application failure: exit 2, never silent or zero.
//
// The collection acknowledgement (VRG_TEST_COLLECT_ACK) — not the
// child's write — is what each test waits on before quitting: it proves
// the diagnostic was processed into the session collection before the
// injected return.

// invalidModelDiagnostic is the diagnostic naming the
// invalid-final-model condition: emitted once, after the session
// diagnostics and before any runtime error, whenever the returned final
// model is absent or the wrong type.
const invalidModelDiagnostic = "vrg: program ended without a usable final model"

// runReturnShape drives the shared lifecycle of the return-shape
// matrix: fake rg emits one stderr diagnostic and a complete stream,
// the warning overlay opens over browse, and q then q quits — the real
// program's ordinary-exit tuple (a status-0 model, nil error) reaches
// the runner seam, which rewrites it to env's selected shape. It
// returns the finished run and the reaped wait status.
func runReturnShape(t *testing.T, env map[string]string) (*ptyRun, string) {
	t.Helper()
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
printf '%s\n' 'warn one' >&2
printf '%s\n' '{"type":"begin","data":{"path":{"text":"file.txt"}}}'
printf '%s\n' '{"type":"match","data":{"path":{"text":"file.txt"},"lines":{"text":"hit 01\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}'
printf '%s\n' '{"type":"end","data":{"path":{"text":"file.txt"},"binary_offset":null}}'
printf '%s\n' '{"type":"summary","data":{}}'
: > "$VRG_TEST_READY"
exit 0
`)
	env["PATH"] = rgDir + ":" + os.Getenv("PATH")
	env["TERM"] = "xterm-256color"
	env["VRG_TEST_READY"] = ready
	env["VRG_TEST_PID"] = pidFile
	env["VRG_TEST_REAP"] = reap
	env["VRG_TEST_COLLECT_ACK"] = ack
	r := startVrgPTY(t, dir, childEnv(env), true, "hit", ".")
	waitFile(t, ready)
	killPidOnCleanup(t, pidFile)

	waitCollectAck(t, ack, "warn one") // collected before the injected return
	r.waitOutput(t, "warn one")        // the warning overlay is open over browse
	r.waitOutput(t, "hit 09")          // the row above the box shows
	if strings.Contains(r.output(), "hit 10") {
		t.Fatal("a row under the overlay box was visible before dismissal")
	}
	r.send(t, "q") // dismisses the overlay to browse
	r.waitOutput(t, "hit 10")
	r.send(t, "q") // quits browse: Run() returns (status-0 model, nil)
	waitReplayedInputRestored(t, r, "warn one")
	code := r.waitExit(t)
	r.finish(t)
	if code != 2 {
		t.Fatalf("exit status = %d, want 2", code)
	}
	if got := reapStatus(t, reap); got != "exit status 0" {
		t.Fatalf("reaped wait status = %q, want %q", got, "exit status 0")
	}
	waitPidGone(t, readPid(t, pidFile))
	assertTerminalRestored(t, r)
	return r, reap
}

// Valid final model + Run() error: the collected diagnostic replays in
// collection order after terminal restoration and the injected runtime
// error is appended exactly once — the post-Run() error branch, not the
// real model's status-0 quit, produced exit 2. The absence of the
// invalid-final-model diagnostic proves the type-assertion branch saw
// the real model.
func TestRuntimeErrorReplaysCollectedDiagnostics(t *testing.T) {
	r, _ := runReturnShape(t, map[string]string{
		"VRG_TEST_RUN_FINAL_MODEL": "valid",
		"VRG_TEST_RUN_ERROR":       "VRG-INJECTED-RUN-ERROR",
	})

	errOut := r.stderr.String()
	want := "warn one\nvrg: VRG-INJECTED-RUN-ERROR\n"
	if errOut != want {
		t.Fatalf("stderr = %q, want %q: session diagnostics in order, then the runtime error once", errOut, want)
	}
	if strings.Contains(errOut, "final model") {
		t.Fatalf("a valid final model produced the invalid-model diagnostic: %q", errOut)
	}
	assertReplayedAfterRestore(t, r, "warn one")
	assertReplayedAfterRestore(t, r, "VRG-INJECTED-RUN-ERROR")
}

// Nil or wrong-type final model + Run() error: the retained session
// diagnostic replays first, then the diagnostic naming the
// invalid-final-model condition — the proof the injected tuple reached
// the post-Run() type branch rather than a controlled model quit —
// then the runtime error exactly once, and the run exits 2.
func TestRuntimeErrorWithInvalidFinalModelReplays(t *testing.T) {
	for _, fm := range []string{"nil", "invalid"} {
		t.Run(fm+" final model", func(t *testing.T) {
			r, _ := runReturnShape(t, map[string]string{
				"VRG_TEST_RUN_FINAL_MODEL": fm,
				"VRG_TEST_RUN_ERROR":       "VRG-INJECTED-RUN-ERROR",
			})

			errOut := r.stderr.String()
			want := "warn one\n" + invalidModelDiagnostic + "\nvrg: VRG-INJECTED-RUN-ERROR\n"
			if errOut != want {
				t.Fatalf("stderr = %q, want %q: session diagnostics, the invalid-model diagnostic, then the error once", errOut, want)
			}
			assertReplayedAfterRestore(t, r, "warn one")
			assertReplayedAfterRestore(t, r, "VRG-INJECTED-RUN-ERROR")
		})
	}
}

// Nil or wrong-type final model + nil Run() error: the retained session
// diagnostic replays, then the invalid-final-model diagnostic — a
// controlled application failure exiting 2, never a silent or zero exit.
func TestInvalidFinalModelWithoutErrorExitsTwo(t *testing.T) {
	for _, fm := range []string{"nil", "invalid"} {
		t.Run(fm+" final model", func(t *testing.T) {
			r, _ := runReturnShape(t, map[string]string{
				"VRG_TEST_RUN_FINAL_MODEL": fm,
				"VRG_TEST_RUN_ERROR":       "nil",
			})

			errOut := r.stderr.String()
			want := "warn one\n" + invalidModelDiagnostic + "\n"
			if errOut != want {
				t.Fatalf("stderr = %q, want %q: session diagnostics then the invalid-model diagnostic", errOut, want)
			}
			assertReplayedAfterRestore(t, r, "warn one")
		})
	}
}
