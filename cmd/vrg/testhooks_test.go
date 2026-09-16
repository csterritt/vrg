package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// vrgConsumedHookManifest is the explicit list of vrg-consumed test
// hook environment variables (Issue #45). The option-related hooks
// wire app.Option and proc.OnReap seams; the runner controls select
// the program.Run() return shapes required by Issue #46. Issue #48
// may extend this list with acknowledgement hooks. Fake-rg fixture
// variables (VRG_TEST_ARGV, VRG_TEST_CWD, VRG_TEST_HANDSHAKE,
// VRG_TEST_READY, VRG_TEST_PID) are deliberately absent: they are
// consumed by the test's fake-rg scripts, not by the vrg binary, and
// are renamed FAKE_RG_* by Issue #50.
var vrgConsumedHookManifest = []string{
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

// buildVrgVariant compiles cmd/vrg into a fresh temp dir and returns
// the binary path. tagged selects the vrg_testhooks build variant.
func buildVrgVariant(t *testing.T, tagged bool) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "vrg")
	args := []string{"build"}
	if tagged {
		args = append(args, "-tags", "vrg_testhooks")
	}
	args = append(args, "-o", bin, ".")
	if out, err := exec.Command("go", args...).CombinedOutput(); err != nil {
		t.Fatalf("go %v failed: %v\n%s", args, err, out)
	}
	return bin
}

// hookProbeEnv builds the environment for the hook-manifest probe:
// every name in vrgConsumedHookManifest set to a provocative value —
// trigger paths that never appear, side-effect files inside dir, and
// marker text that must never surface in output.
func hookProbeEnv(t *testing.T, dir, fakeDir, handshake string) []string {
	t.Helper()
	return []string{
		"PATH=" + fakeDir + ":" + os.Getenv("PATH"),
		"VRG_TEST_HANDSHAKE=" + handshake,
		"VRG_TEST_REAP=" + filepath.Join(dir, "reap"),
		"VRG_TEST_GATE=" + filepath.Join(dir, "gate"),
		"VRG_TEST_FAIL_TRIGGER=" + filepath.Join(dir, "failtrigger"),
		"VRG_TEST_FAIL_DIAGNOSTIC=hook must not fire: controlled failure",
		"VRG_TEST_DIAGNOSTIC_TRIGGER=" + filepath.Join(dir, "diagtrigger"),
		"VRG_TEST_DIAGNOSTIC_TEXT=hook must not fire: diagnostic",
		"VRG_TEST_COLLECT_ACK=" + filepath.Join(dir, "ack"),
		"VRG_TEST_RUN_FINAL_MODEL=nil",
		"VRG_TEST_RUN_ERROR=hook must not fire: run error",
	}
}

// TestUntaggedBinaryIgnoresHookManifest is the clean-artifact boundary
// test (Issue #45). It builds the production binary without build
// tags, drives an ordinary search with every name in the explicit
// vrg-consumed hook manifest set, and asserts no behavioural change:
// normal exit 0, no injected diagnostics or failures, no side-effect
// files, and none of the hook names present in the artifact. It fails
// while the env-var seams are compiled into the production binary —
// proving the released artifact itself is clean, not merely that a
// tag defaults off.
func TestUntaggedBinaryIgnoresHookManifest(t *testing.T) {
	prodBin := buildVrgVariant(t, false)

	fakeDir := t.TempDir()
	writeFakeRG(t, fakeDir, "")

	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "test.txt"), []byte("hello world\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	hookDir := t.TempDir()
	handshakeFile := filepath.Join(t.TempDir(), "handshake")
	cmd := exec.Command(prodBin, "hello", ".")
	cmd.Dir = repo
	cmd.Env = hookProbeEnv(t, hookDir, fakeDir, handshakeFile)

	_, stderr, exitCode := runVrgWithKeys(t, cmd, handshakeFile, "q")

	if exitCode != 0 {
		t.Fatalf("production binary exited %d with every hook set, want 0 (stderr %q)", exitCode, stderr)
	}
	if strings.Contains(stderr, "hook must not fire") {
		t.Fatalf("hook marker text reached stderr in the production binary: %q", stderr)
	}
	for _, name := range []string{"reap", "ack"} {
		if _, err := os.Stat(filepath.Join(hookDir, name)); err == nil {
			t.Fatalf("production binary created hook side-effect file %s", name)
		}
	}

	data, err := os.ReadFile(prodBin)
	if err != nil {
		t.Fatalf("cannot read production binary: %v", err)
	}
	for _, name := range vrgConsumedHookManifest {
		if bytes.Contains(data, []byte(name)) {
			t.Fatalf("production binary contains hook name %s", name)
		}
	}
}

// runTaggedWithHooks runs the tagged binary under a PTY with the fake
// rg, sends q after the handshake, and returns stderr and the exit
// code. The final-model and run-error controls select the tuple the
// tagged runner seam returns at the program.Run() call site; empty
// means the control is left unset.
func runTaggedWithHooks(t *testing.T, bin, finalModel, runError string) (stderr string, exitCode int) {
	t.Helper()

	fakeDir := t.TempDir()
	writeFakeRG(t, fakeDir, "")

	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "test.txt"), []byte("hello world\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	handshakeFile := filepath.Join(t.TempDir(), "handshake")
	env := []string{
		"PATH=" + fakeDir + ":" + os.Getenv("PATH"),
		"VRG_TEST_HANDSHAKE=" + handshakeFile,
	}
	if finalModel != "" {
		env = append(env, "VRG_TEST_RUN_FINAL_MODEL="+finalModel)
	}
	if runError != "" {
		env = append(env, "VRG_TEST_RUN_ERROR="+runError)
	}

	cmd := exec.Command(bin, "hello", ".")
	cmd.Dir = repo
	cmd.Env = env
	_, stderr, exitCode = runVrgWithKeys(t, cmd, handshakeFile, "q")
	return stderr, exitCode
}

// TestTaggedRunnerSeamReturnShapes proves the vrg_testhooks runner
// seam can inject every final-model/error tuple Issue #46 needs at
// the executable's actual program.Run() return site (Issue #45). The
// real program still runs — q quits browse — then the seam's tuple
// reaches the post-Run() branches unchanged: a non-nil error surfaces
// at the runtime-error branch with its exact text, and a nil or
// wrong-type final model reaches the invalid-final-model branch
// rather than the ordinary app.Model path. These tests specify only
// seam reachability and tuple fidelity; Issue #46 owns the shutdown,
// replay, and exit-status contract behind each branch.
func TestTaggedRunnerSeamReturnShapes(t *testing.T) {
	taggedBin := buildVrgVariant(t, true)

	const marker = "tagged-runner-injected-error-045"

	// A non-nil injected error reaches the Run() error branch with
	// its text unchanged, whatever the final-model shape.
	for _, finalModel := range []string{"valid", "nil", "invalid"} {
		t.Run("error/"+finalModel+"_model", func(t *testing.T) {
			stderr, exitCode := runTaggedWithHooks(t, taggedBin, finalModel, marker)
			if exitCode != 2 {
				t.Fatalf("tagged binary exited %d, want 2 (stderr %q)", exitCode, stderr)
			}
			if !strings.Contains(stderr, marker) {
				t.Fatalf("injected run error %q did not reach the Run() error branch: %q", marker, stderr)
			}
		})
	}

	// A nil or wrong-type final model with a nil error reaches the
	// invalid-final-model branch (non-zero exit), not the ordinary
	// app.Model exit path.
	for _, finalModel := range []string{"nil", "invalid"} {
		t.Run("no_error/"+finalModel+"_model", func(t *testing.T) {
			stderr, exitCode := runTaggedWithHooks(t, taggedBin, finalModel, "nil")
			if exitCode != 2 {
				t.Fatalf("tagged binary exited %d, want 2 for the %s final-model shape (stderr %q)", exitCode, finalModel, stderr)
			}
		})
	}

	// With the runner controls unset the seam delegates: q from
	// browse after a clean fake-rg stream exits 0.
	t.Run("no_controls_delegates", func(t *testing.T) {
		stderr, exitCode := runTaggedWithHooks(t, taggedBin, "", "")
		if exitCode != 0 {
			t.Fatalf("tagged binary with no runner controls exited %d, want 0 (stderr %q)", exitCode, stderr)
		}
	})
}
