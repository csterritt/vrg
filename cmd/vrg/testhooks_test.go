package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// hookManifest is the explicit list of environment-variable names a
// vrg_testhooks-tagged binary may consume — the only VRG_TEST_* names
// that are vrg behaviour. Fixture-owned fake-rg variables
// (VRG_TEST_ARGV, VRG_TEST_CWD, VRG_TEST_HANDSHAKE, VRG_TEST_RG_PID,
// VRG_TEST_RG_READY) are deliberately absent: vrg never reads them, and
// Issue #50 renames them FAKE_RG_*. Issues #46 and #48 extend this list
// through the same mechanism rather than adding production hooks.
var hookManifest = []string{
	"VRG_TEST_REAP",
	"VRG_TEST_GATE",
	"VRG_TEST_LOAD_GATE",
	"VRG_TEST_COLLECT_ACK",
	"VRG_TEST_FAIL_TRIGGER",
	"VRG_TEST_FAIL_DIAGNOSTIC",
	"VRG_TEST_DIAGNOSTIC_TRIGGER",
	"VRG_TEST_DIAGNOSTIC_TEXT",
	"VRG_TEST_RUN_FINAL_MODEL",
	"VRG_TEST_RUN_ERROR",
}

// buildVrg compiles cmd/vrg into a fresh temp dir with the given extra
// go-build flags and returns the binary path.
func buildVrg(t *testing.T, extra ...string) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "vrg")
	args := append([]string{"build", "-o", bin}, extra...)
	cmd := exec.Command("go", append(args, ".")...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build %v failed: %v\n%s", args, err, out)
	}
	return bin
}

// The released artifact is clean: an untagged build ignores every name
// in the explicit vrg-consumed hook manifest — even with the files the
// hooks would consume already in place — and the binary contains none
// of their strings. This proves the artifact itself carries no seams,
// not merely that a tag defaults off.
func TestProductionBinaryIgnoresHookManifest(t *testing.T) {
	prodBin := buildVrg(t) // untagged: the released topology

	// Artifact half: none of the manifest names survive compilation.
	bin, err := os.ReadFile(prodBin)
	if err != nil {
		t.Fatalf("read built binary: %v", err)
	}
	for _, name := range hookManifest {
		if bytes.Contains(bin, []byte(name)) {
			t.Fatalf("production binary contains test-hook name %q", name)
		}
	}

	// Behavioural half: every manifest name is set to a live trigger —
	// gate files that would hang the run and trigger files that would
	// inject failure — plus writer paths that would gain lines. A clean
	// binary ignores all of them: browse, quit, exit 0, nothing written.
	dir := t.TempDir()
	writeHappyFiles(t, dir)
	probe := t.TempDir()
	gate := filepath.Join(probe, "gate")
	loadGate := filepath.Join(probe, "loadgate")
	failTrig := filepath.Join(probe, "fail")
	for _, f := range []string{gate, loadGate, failTrig} {
		if err := os.WriteFile(f, []byte("held"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	env := testEnv(fakeRG(t, happyStreamRG),
		"VRG_TEST_REAP="+filepath.Join(probe, "reap"),
		"VRG_TEST_GATE="+gate,
		"VRG_TEST_LOAD_GATE="+loadGate,
		"VRG_TEST_COLLECT_ACK="+filepath.Join(probe, "collect"),
		"VRG_TEST_FAIL_TRIGGER="+failTrig,
		"VRG_TEST_FAIL_DIAGNOSTIC=probe-failure",
		"VRG_TEST_DIAGNOSTIC_TRIGGER="+filepath.Join(probe, "diag"),
		"VRG_TEST_DIAGNOSTIC_TEXT=probe-diag",
		"VRG_TEST_RUN_FINAL_MODEL=nil",
		"VRG_TEST_RUN_ERROR=probe-run-error")

	out, code := runVrgWithQuitBin(t, prodBin, dir, env, "foo")
	if code != 0 {
		t.Fatalf("hook-manifest run exited %d, want 0; output: %q", code, out)
	}
	for _, marker := range []string{"probe-failure", "probe-diag", "probe-run-error"} {
		if strings.Contains(out, marker) {
			t.Fatalf("hook value %q reached the output of a production binary: %q", marker, out)
		}
	}
	entries, err := os.ReadDir(probe)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("hook side effects appeared in probe dir: %v", names)
	}
}

// The tagged half of the boundary: a vrg_testhooks build injects every
// final-model/error tuple Issue #46 needs through the executable's real
// program-runner call site — the values surface in the post-Run
// branches unchanged, so an injected error is replayed verbatim after
// restoration and a nil/invalid final model takes the absent-model
// exit-2 branch.
func TestRunnerSeamInjectsRunReturnShapes(t *testing.T) {
	hookBin := buildVrg(t, "-tags", "vrg_testhooks")
	const runErr = "vrg-injected-run-error"
	cases := []struct {
		name     string
		fm       string
		runErr   string
		wantCode int
		wantDiag string
	}{
		{"delegated valid model nil error", "", "", 0, ""},
		{"explicit valid model nil error", "valid", "", 0, ""},
		{"valid model error", "valid", runErr, 2, "vrg: " + runErr},
		{"nil model nil error", "nil", "", 2, ""},
		{"nil model error", "nil", runErr, 2, "vrg: " + runErr},
		{"invalid model nil error", "invalid", "", 2, ""},
		{"invalid model error", "invalid", runErr, 2, "vrg: " + runErr},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeHappyFiles(t, dir)
			env := testEnv(fakeRG(t, happyStreamRG),
				"VRG_TEST_RUN_FINAL_MODEL="+tc.fm,
				"VRG_TEST_RUN_ERROR="+tc.runErr)
			// The program still runs for real — the browse view must
			// appear before q — then the injected tuple replaces the
			// Run() result at the return site.
			out, code := runVrgWithQuitBin(t, hookBin, dir, env, "foo")
			if code != tc.wantCode {
				t.Fatalf("exit = %d, want %d; output: %q", code, tc.wantCode, out)
			}
			if tc.wantDiag != "" && strings.Count(out, tc.wantDiag) != 1 {
				t.Fatalf("injected error %q not replayed exactly once: %q", tc.wantDiag, out)
			}
		})
	}
}
