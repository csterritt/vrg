package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeRunShapeFakeRG writes a fake rg that records its PID, writes
// stderrText to stderr, emits a complete valid stream for test.txt,
// touches the handshake file, and exits 0. The session collects the
// stderr text as a diagnostic when SearchCompleteMsg is processed and
// ends in browse with a warning overlay, leaving a clean model quit as
// the underlying Run() result for the runner seam to override.
func writeRunShapeFakeRG(t *testing.T, dir, stderrText string) string {
	t.Helper()
	rgPath := filepath.Join(dir, "rg")
	script := "#!/bin/sh\n"
	script += "if [ -n \"$VRG_TEST_PID\" ]; then echo $$ > \"$VRG_TEST_PID\"; fi\n"
	if stderrText != "" {
		script += "printf '%s\\n' " + shellQuote(stderrText) + " >&2\n"
	}
	script += "echo '{\"type\":\"begin\",\"data\":{\"path\":{\"text\":\"test.txt\"}}}'\n"
	script += "printf '%s\\n' '{\"type\":\"match\",\"data\":{\"path\":{\"text\":\"test.txt\"},\"lines\":{\"text\":\"hello world\\n\"},\"line_number\":1,\"submatches\":[{\"match\":{\"text\":\"hello\"},\"start\":0,\"end\":5}]}}'\n"
	script += "echo '{\"type\":\"end\",\"data\":{\"path\":{\"text\":\"test.txt\"},\"binary_offset\":null}}'\n"
	script += "echo '{\"type\":\"summary\",\"data\":{}}'\n"
	script += "if [ -n \"$VRG_TEST_HANDSHAKE\" ]; then touch \"$VRG_TEST_HANDSHAKE\"; fi\n"
	script += "exit 0\n"
	if err := os.WriteFile(rgPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return rgPath
}

// assertStderrOrderedOnce requires each part to appear in stderr
// exactly once, in the given order. Exactly-once across the whole
// stream proves no diagnostic was emitted both by a direct write and
// by the replay.
func assertStderrOrderedOnce(t *testing.T, stderr string, parts []string) {
	t.Helper()
	prev := -1
	for _, p := range parts {
		if count := strings.Count(stderr, p); count != 1 {
			t.Fatalf("%q appears %d times in stderr, want 1: %q", p, count, stderr)
		}
		idx := strings.Index(stderr, p)
		if idx < prev {
			t.Fatalf("%q is out of order in stderr: %q", p, stderr)
		}
		prev = idx
	}
}

// TestRunReturnShapeUnifiedShutdown drives the Issue #46 return-shape
// matrix through the Issue #45 tagged program-runner seam at the real
// program.Run() boundary. Every case runs a full PTY lifecycle in
// which two diagnostics are collected in a deterministic order before
// the model quits cleanly from browse; the seam then substitutes the
// selected final-model/error tuple at the actual Run() return site.
// The unified shutdown contract requires: terminal restoration and
// child termination/reap exactly as on normal exits, then the ordered
// replay — session diagnostics in collection order, then a diagnostic
// naming the invalid-final-model condition when applicable, then the
// Run() runtime error exactly once — and exit 2 on every failing
// shape, never a silent or zero exit. The injected markers and exit 2
// (where the real model's clean browse quit would have exited 0) prove
// each tuple reached the executable's actual post-Run() type/error
// branches rather than a controlled model quit.
func TestRunReturnShapeUnifiedShutdown(t *testing.T) {
	const runErrMarker = "runerr-injected-046"
	// invalidModelDiag names the invalid-final-model condition the
	// unified shutdown emits when Run()'s final model is absent or
	// has the wrong type.
	const invalidModelDiag = "final model"

	cases := []struct {
		name string
		// finalModel is the VRG_TEST_RUN_FINAL_MODEL value; ""
		// leaves the control unset so the real app.Model survives.
		finalModel string
		// runError is the VRG_TEST_RUN_ERROR value; always set.
		runError string
		// wantInvalidDiag is true when the shape must emit the
		// invalid-final-model diagnostic after the session
		// diagnostics.
		wantInvalidDiag bool
		// wantRunErr is true when the shape must append the
		// injected runtime error last.
		wantRunErr bool
	}{
		{"valid_model_run_error", "", runErrMarker, false, true},
		{"nil_model_run_error", "nil", runErrMarker, true, true},
		{"invalid_model_run_error", "invalid", runErrMarker, true, true},
		{"nil_model_nil_error", "nil", "nil", true, false},
		{"invalid_model_nil_error", "invalid", "nil", true, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fakeDir := t.TempDir()
			writeRunShapeFakeRG(t, fakeDir, "session diag one")

			repo := t.TempDir()
			if err := os.WriteFile(filepath.Join(repo, "test.txt"), []byte("hello world\n"), 0o644); err != nil {
				t.Fatal(err)
			}

			pidFile := filepath.Join(t.TempDir(), "pid")
			handshakeFile := filepath.Join(t.TempDir(), "handshake")
			reapFile := filepath.Join(t.TempDir(), "reap")
			collectAck := filepath.Join(t.TempDir(), "ack")
			updateAck := filepath.Join(t.TempDir(), "update-ack")
			diagTrigger := filepath.Join(t.TempDir(), "diagtrigger")

			env := []string{
				"PATH=" + fakeDir + ":" + os.Getenv("PATH"),
				"VRG_TEST_PID=" + pidFile,
				"VRG_TEST_HANDSHAKE=" + handshakeFile,
				"VRG_TEST_REAP=" + reapFile,
				"VRG_TEST_COLLECT_ACK=" + collectAck,
				"VRG_TEST_UPDATE_ACK=" + updateAck,
				"VRG_TEST_DIAGNOSTIC_TRIGGER=" + diagTrigger,
				"VRG_TEST_DIAGNOSTIC_TEXT=session diag two",
				"VRG_TEST_RUN_ERROR=" + tc.runError,
			}
			if tc.finalModel != "" {
				env = append(env, "VRG_TEST_RUN_FINAL_MODEL="+tc.finalModel)
			}

			cmd := exec.Command(binPath, "hello", ".")
			cmd.Dir = repo
			cmd.Env = env

			res := runVrgReplay(t, cmd, updateAck, func(d *ptyDriver) {
				// Wait for the fake rg to finish, then for the
				// SearchComplete-collected diagnostic (the fake
				// rg's stderr warning) to be acknowledged. The
				// triggered diagnostic is collected second, so
				// the session collection order is deterministic.
				waitForFile(t, handshakeFile, 15*time.Second)
				waitForAckLines(t, collectAck, 1, 15*time.Second)
				if err := os.WriteFile(diagTrigger, []byte("trigger"), 0o644); err != nil {
					t.Errorf("cannot write diag trigger: %v", err)
				}
				waitForAckLines(t, collectAck, 2, 15*time.Second)
				// Esc dismisses the warning overlay — the esc
				// event must acknowledge the dismissal before q
				// is sent (Issue #48 overlay-dismissal-before-quit
				// row) — then q quits browse, so the real Run()
				// result is a clean (app.Model, nil) tuple for
				// the seam to override.
				esc := d.sendKey(t, "\x1b")
				if !esc.dismissed {
					t.Errorf("esc did not dismiss the warning overlay: %+v", esc)
				}
				d.sendKey(t, "q")
			})

			// Every failing Run() return shape is a controlled
			// application failure: exit 2, matching the
			// startup-failure convention. The real model quit
			// from browse would have exited 0, so 2 proves the
			// injected tuple reached the post-Run() branches.
			if res.exitCode != 2 {
				t.Fatalf("vrg exited %d, want 2 (stderr %q)", res.exitCode, res.stderr)
			}
			// Terminal restoration and child termination/reap are
			// exactly as on normal exits; termios equality is
			// asserted inside runVrgReplay.
			assertReapEvidence(t, reapFile)
			assertChildGone(t, pidFile)
			assertDisplayRestoration(t, res.rawOutput)

			if strings.ContainsAny(res.stderr, "\x1b\x9b") {
				t.Fatalf("stderr contains raw control bytes: %q", res.stderr)
			}

			// Ordered replay: session diagnostics in collection
			// order, then the invalid-final-model diagnostic when
			// applicable, then the runtime error — each exactly
			// once, with no duplicate between a direct write and
			// the replay.
			want := []string{"session diag one", "session diag two"}
			if tc.wantInvalidDiag {
				want = append(want, invalidModelDiag)
			} else if strings.Contains(res.stderr, invalidModelDiag) {
				t.Fatalf("invalid-final-model diagnostic emitted for the valid-model shape: %q", res.stderr)
			}
			if tc.wantRunErr {
				want = append(want, runErrMarker)
			}
			assertStderrOrderedOnce(t, res.stderr, want)
		})
	}
}
