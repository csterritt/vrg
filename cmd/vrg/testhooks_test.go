package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// testHookManifest is the explicit list of vrg-consumed test-hook
// environment variables: the names the vrg_testhooks build reads and
// the untagged production binary must ignore — and must not even
// contain. The list is Issue 45's manifest, never derived by scanning
// for VRG_TEST_* occurrences, because fixture-owned fake-rg variables
// (VRG_TEST_READY, VRG_TEST_PID — renamed FAKE_RG_* by Issue 50) are
// not vrg behaviour. Issue 48 may extend it with acknowledgement
// hooks.
var testHookManifest = []string{
	"VRG_TEST_REAP",
	"VRG_TEST_GATE",
	"VRG_TEST_FAIL_TRIGGER",
	"VRG_TEST_FAIL_DIAGNOSTIC",
	"VRG_TEST_DIAGNOSTIC_TRIGGER",
	"VRG_TEST_DIAGNOSTIC_TEXT",
	"VRG_TEST_COLLECT_ACK",
	"VRG_TEST_RUN_FINAL_MODEL",
	"VRG_TEST_RUN_ERROR",
}

// buildVrgVariant compiles cmd/vrg into a private temp dir — untagged
// for the production artifact, tagged for the seam-equipped variant —
// so each test exercises the exact artifact under scrutiny rather than
// TestMain's build.
func buildVrgVariant(t *testing.T, tagged bool) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "vrg")
	args := []string{"build"}
	if tagged {
		args = append(args, "-tags", "vrg_testhooks")
	}
	args = append(args, "-o", bin, ".")
	if out, err := exec.Command("go", args...).CombinedOutput(); err != nil {
		t.Fatalf("go %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return bin
}

// streamFixture is a fake rg emitting one complete match for file.txt
// then exiting 0; it touches the ready file embedded in the script.
const streamFixture = `#!/bin/sh
printf '%%s\n' '{"type":"begin","data":{"path":{"text":"file.txt"}}}'
printf '%%s\n' '{"type":"match","data":{"path":{"text":"file.txt"},"lines":{"text":"hit\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}'
printf '%%s\n' '{"type":"end","data":{"path":{"text":"file.txt"},"binary_offset":null}}'
printf '%%s\n' '{"type":"summary","data":{}}'
: > %q
exit 0
`

// exitTwoFixture is a fake rg emitting nothing and exiting 2, so the
// real run produces a fatal overlay and a status-2 final model.
const exitTwoFixture = `#!/bin/sh
: > %q
exit 2
`

// TestUntaggedBinaryIgnoresHookManifest runs the untagged production
// binary under a PTY with every name in the hook manifest armed: a
// never-created gate, already-created triggers, side-channel paths,
// injected marker texts, and both runner controls. A clean binary
// ignores all of them — the search reaches browse, q exits 0, no side
// channel is written, and no marker reaches either stream.
func TestUntaggedBinaryIgnoresHookManifest(t *testing.T) {
	bin := buildVrgVariant(t, false)
	dir := t.TempDir()
	ready := filepath.Join(dir, "ready")
	reap := filepath.Join(dir, "reap")
	ack := filepath.Join(dir, "ack")
	gate := filepath.Join(dir, "gate") // never created: a live seam would hold preparation
	trigger := filepath.Join(dir, "trigger")
	diagTrigger := filepath.Join(dir, "diag-trigger")
	for _, p := range []string{trigger, diagTrigger} {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("hit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rgDir := fakeRgPath(t, fmt.Sprintf(streamFixture, ready))
	env := childEnv(map[string]string{
		"PATH": rgDir + ":" + os.Getenv("PATH"),
		"TERM": "xterm-256color",
		// Every vrg-consumed hook armed at once.
		"VRG_TEST_REAP":               reap,
		"VRG_TEST_GATE":               gate,
		"VRG_TEST_FAIL_TRIGGER":       trigger,
		"VRG_TEST_FAIL_DIAGNOSTIC":    "VRG-INJECTED-FAILURE",
		"VRG_TEST_DIAGNOSTIC_TRIGGER": diagTrigger,
		"VRG_TEST_DIAGNOSTIC_TEXT":    "VRG-INJECTED-DIAGNOSTIC",
		"VRG_TEST_COLLECT_ACK":        ack,
		"VRG_TEST_RUN_FINAL_MODEL":    "nil",
		"VRG_TEST_RUN_ERROR":          "VRG-INJECTED-RUN-ERROR",
	})
	r := startBinaryPTY(t, bin, dir, env, false, "hit", ".")
	waitFile(t, ready)
	r.waitOutput(t, "file.txt") // reaching browse proves the gate did not hold
	r.send(t, "q")
	code := r.waitExit(t)
	r.finish(t)

	if code != 0 {
		t.Fatalf("exit status = %d, want 0 — a manifest hook changed the run", code)
	}
	for _, p := range []string{reap, ack} {
		if _, err := os.Stat(p); err == nil {
			t.Fatalf("the untagged binary wrote hook side channel %s", p)
		}
	}
	if s := r.stderr.String(); s != "" {
		t.Fatalf("stderr = %q, want empty — no injected diagnostic or failure", s)
	}
	for _, marker := range []string{"VRG-INJECTED-FAILURE", "VRG-INJECTED-DIAGNOSTIC", "VRG-INJECTED-RUN-ERROR"} {
		if strings.Contains(r.output(), marker) {
			t.Fatalf("hook marker %q reached the PTY stream", marker)
		}
	}
	assertTerminalRestored(t, r)
}

// TestUntaggedBinaryContainsNoHookStrings inspects the released
// artifact itself: none of the manifest names may be present, so the
// proof is the clean binary, not merely a tag that defaults off.
func TestUntaggedBinaryContainsNoHookStrings(t *testing.T) {
	bin := buildVrgVariant(t, false)
	blob, err := os.ReadFile(bin)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range testHookManifest {
		if bytes.Contains(blob, []byte(name)) {
			t.Fatalf("untagged binary contains hook name %q", name)
		}
	}
}

// The vrg_testhooks runner seam: each test proves the tuple selected
// through VRG_TEST_RUN_FINAL_MODEL/VRG_TEST_RUN_ERROR reaches the
// executable's real program-Run return site — the injected values ride
// the production post-Run() branches after a real PTY lifecycle, they
// never substitute a fake program. Issue 46 owns the shutdown, replay,
// and exit-status semantics of each shape; these tests pin only seam
// reachability and tuple fidelity.
func TestTaggedRunnerInjectsErrorOverValidModel(t *testing.T) {
	bin := buildVrgVariant(t, true)
	dir := t.TempDir()
	ready := filepath.Join(dir, "ready")
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("hit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rgDir := fakeRgPath(t, fmt.Sprintf(streamFixture, ready))
	r := startBinaryPTY(t, bin, dir, childEnv(map[string]string{
		"PATH":               rgDir + ":" + os.Getenv("PATH"),
		"TERM":               "xterm-256color",
		"VRG_TEST_RUN_ERROR": "VRG-INJECTED-RUN-ERROR",
	}), false, "hit", ".")
	waitFile(t, ready)
	r.waitOutput(t, "file.txt") // the real program ran to browse
	r.send(t, "q")
	code := r.waitExit(t)
	r.finish(t)

	// The real run ended cleanly (status 0, nil error); the injected
	// error surfaces through the same post-Run() error branch a real
	// runtime error would take.
	if code != 2 {
		t.Fatalf("exit status = %d, want 2", code)
	}
	errOut := r.stderr.String()
	if n := strings.Count(errOut, "VRG-INJECTED-RUN-ERROR"); n != 1 {
		t.Fatalf("injected error count = %d, want exactly once: %q", n, errOut)
	}
	if !strings.HasPrefix(strings.TrimSpace(errOut), "vrg: ") {
		t.Fatalf("stderr = %q, want a vrg: diagnostic", errOut)
	}
	assertTerminalRestored(t, r)
}

// A nil or wrong-type final model replaces the real one at the return
// site: with the error left nil, the run's exit reflects the injected
// model, not the real model's status-2 — Issue 46 makes the unusable
// model itself a controlled failure (exit 2 with a diagnostic; the
// ordered-replay contract lives in runerror_test.go). The seam
// assertion is that the injected tuple, not the real model, reaches
// the post-Run() branch.
func TestTaggedRunnerInjectsFinalModel(t *testing.T) {
	bin := buildVrgVariant(t, true)
	for _, tc := range []struct {
		name     string
		finalMod string
		wantCode int
	}{
		{"valid final model keeps the real status", "valid", 2},
		{"nil final model", "nil", 2},
		{"invalid final model", "invalid", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			ready := filepath.Join(dir, "ready")
			rgDir := fakeRgPath(t, fmt.Sprintf(exitTwoFixture, ready))
			r := startBinaryPTY(t, bin, dir, childEnv(map[string]string{
				"PATH":                     rgDir + ":" + os.Getenv("PATH"),
				"TERM":                     "xterm-256color",
				"VRG_TEST_RUN_FINAL_MODEL": tc.finalMod,
				"VRG_TEST_RUN_ERROR":       "nil",
			}), false, "hit", ".")
			waitFile(t, ready)
			r.waitOutput(t, "code 2") // the fatal overlay proves the real program ran
			r.send(t, "q")
			code := r.waitExit(t)
			r.finish(t)

			if code != tc.wantCode {
				t.Fatalf("exit status = %d, want %d", code, tc.wantCode)
			}
			assertTerminalRestored(t, r)
		})
	}
}

// A nil or wrong-type final model paired with an injected error takes
// the post-Run() error branch: the run exits 2 and the injected error
// text reaches stderr through the ordinary diagnostic path exactly
// once.
func TestTaggedRunnerInjectsModelAndError(t *testing.T) {
	bin := buildVrgVariant(t, true)
	for _, fm := range []string{"nil", "invalid"} {
		t.Run(fm+" final model with error", func(t *testing.T) {
			dir := t.TempDir()
			ready := filepath.Join(dir, "ready")
			rgDir := fakeRgPath(t, fmt.Sprintf(exitTwoFixture, ready))
			r := startBinaryPTY(t, bin, dir, childEnv(map[string]string{
				"PATH":                     rgDir + ":" + os.Getenv("PATH"),
				"TERM":                     "xterm-256color",
				"VRG_TEST_RUN_FINAL_MODEL": fm,
				"VRG_TEST_RUN_ERROR":       "VRG-INJECTED-RUN-ERROR",
			}), false, "hit", ".")
			waitFile(t, ready)
			r.waitOutput(t, "code 2") // the real program ran
			r.send(t, "q")
			code := r.waitExit(t)
			r.finish(t)

			if code != 2 {
				t.Fatalf("exit status = %d, want 2", code)
			}
			errOut := r.stderr.String()
			if n := strings.Count(errOut, "VRG-INJECTED-RUN-ERROR"); n != 1 {
				t.Fatalf("injected error count = %d, want exactly once: %q", n, errOut)
			}
			assertTerminalRestored(t, r)
		})
	}
}
