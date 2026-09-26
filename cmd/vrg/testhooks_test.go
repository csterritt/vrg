//go:build linux

package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// hookManifest is the explicit list of VRG_TEST_* environment names vrg
// consumes — every one lives behind the vrg_testhooks build tag and
// reaches the production binary only as an absent string. The probed
// list comes only from this manifest, never from grepping VRG_TEST_*:
// fake-rg fixture variables such as VRG_CAPTURE_DIR are harness
// behaviour, not vrg seams. Issue #48 appends its acknowledgement
// hooks here.
var hookManifest = []string{
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
// the binary path; tags names an extra -tags build constraint.
func buildVrgVariant(t *testing.T, tags string) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "vrg")
	args := []string{"build", "-o", bin}
	if tags != "" {
		args = append(args, "-tags", tags)
	}
	args = append(args, ".")
	if out, err := exec.Command("go", args...).CombinedOutput(); err != nil {
		t.Fatalf("go %v failed: %v\n%s", args, err, out)
	}
	return bin
}

// runBin executes the binary at bin once with piped output streams.
func runBin(t *testing.T, bin string, env []string, args ...string) runResult {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = t.TempDir()
	if env != nil {
		cmd.Env = env
	}
	var so, se bytes.Buffer
	cmd.Stdout = &so
	cmd.Stderr = &se
	err := cmd.Run()
	res := runResult{stdout: so.String(), stderr: se.String()}
	if err == nil {
		res.code = 0
		return res
	}
	if ee, ok := err.(*exec.ExitError); ok {
		res.code = ee.ExitCode()
		return res
	}
	t.Fatalf("failed to run %v: %v", args, err)
	return res
}

// pipeRun is a vrg process on plain pipes: stdin stays open for
// scripted keys and both output streams are captured.
type pipeRun struct {
	cmd    *exec.Cmd
	ctx    context.Context
	stdin  io.WriteCloser
	stdout bytes.Buffer
	stderr bytes.Buffer
}

func startPiped(t *testing.T, bin string, env []string, args ...string) *pipeRun {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = t.TempDir()
	if env != nil {
		cmd.Env = env
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	r := &pipeRun{cmd: cmd, ctx: ctx, stdin: stdin}
	cmd.Stdout = &r.stdout
	cmd.Stderr = &r.stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	return r
}

func (r *pipeRun) send(t *testing.T, keys string) {
	t.Helper()
	if _, err := r.stdin.Write([]byte(keys)); err != nil {
		t.Fatalf("writing %q to stdin: %v", keys, err)
	}
}

// finish waits for the process and reports the outcome; the context
// deadline turns a wedged session into a test failure rather than a
// hang.
func (r *pipeRun) finish(t *testing.T, what string) runResult {
	t.Helper()
	werr := r.cmd.Wait()
	if r.ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("%s timed out (stdout %q, stderr %q)", what, r.stdout.String(), r.stderr.String())
	}
	res := runResult{stdout: r.stdout.String(), stderr: r.stderr.String()}
	if werr == nil {
		res.code = 0
		return res
	}
	if ee, ok := werr.(*exec.ExitError); ok {
		res.code = ee.ExitCode()
		return res
	}
	t.Fatalf("%s: wait: %v", what, werr)
	return res
}

// fireFifo opens path for writing and immediately closes it — the
// open/close handshake the trigger fifos key on. The open pairs only
// with a reader, so it runs off the test goroutine: against a binary
// without the seam it simply never pairs.
func fireFifo(path string) {
	go func() {
		w, err := os.OpenFile(path, os.O_WRONLY, 0)
		if err == nil {
			_ = w.Close()
		}
	}()
}

// holdFifo pairs a fifo writer for holdFor then closes it: a hooked
// binary's gate stays held until the close, while an unhooked binary
// never opens the fifo and ignores it entirely.
func holdFifo(path string, holdFor time.Duration) {
	go func() {
		w, err := os.OpenFile(path, os.O_WRONLY, 0)
		if err != nil {
			return
		}
		time.Sleep(holdFor)
		_ = w.Close()
	}()
}

// The production-artifact boundary: an untagged build ignores every
// name in the hook manifest and carries none of them in the artifact.
// Help output is identical under the full environment, and a complete
// search run with every seam armed — trigger fifos fired, gate held
// then released, runner overrides set, injection text supplied — still
// exits 0 with clean stderr and leaves no reap or acknowledgement
// files. Then the binary itself is probed: no manifest name appears in
// it at all, which is stronger than merely proving the tag defaults
// off.
func TestProductionBinaryHasNoTestHooks(t *testing.T) {
	bin := buildVrgVariant(t, "")

	dir := t.TempDir()
	hookEnv := func(base []string) []string {
		mkv := func(name, val string) string { return name + "=" + val }
		return append(base,
			mkv("VRG_TEST_REAP", filepath.Join(dir, "reap")),
			mkv("VRG_TEST_COLLECT_ACK", filepath.Join(dir, "ack")),
			mkv("VRG_TEST_GATE", filepath.Join(dir, "gate-fifo")),
			mkv("VRG_TEST_FAIL_TRIGGER", filepath.Join(dir, "fail-fifo")),
			mkv("VRG_TEST_FAIL_DIAGNOSTIC", "injected failure diagnostic"),
			mkv("VRG_TEST_DIAGNOSTIC_TRIGGER", filepath.Join(dir, "diag-fifo")),
			mkv("VRG_TEST_DIAGNOSTIC_TEXT", "injected session diagnostic"),
			mkv("VRG_TEST_RUN_FINAL_MODEL", "nil"),
			mkv("VRG_TEST_RUN_ERROR", "injected runtime error"),
		)
	}

	// Help under the full hook environment is byte-identical to a
	// plain help run: exit 0, one usage copy on stdout, empty stderr.
	assertHelpRun(t, runBin(t, bin, hookEnv(nil), "--help"), []string{"--help"})

	for _, f := range []string{"gate-fifo", "fail-fifo", "diag-fifo"} {
		if err := syscall.Mkfifo(filepath.Join(dir, f), 0o600); err != nil {
			t.Fatalf("mkfifo %s: %v", f, err)
		}
	}
	fireFifo(filepath.Join(dir, "fail-fifo"))
	fireFifo(filepath.Join(dir, "diag-fifo"))
	holdFifo(filepath.Join(dir, "gate-fifo"), 2*time.Second)

	// A complete search: every manifest name set, trigger fifos
	// already fired, the gate held by a paired writer. An unhooked
	// binary ignores all of it.
	work := t.TempDir()
	if err := os.WriteFile(filepath.Join(work, "f.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fakeDir, capDir := writeFakeRg(t, fakeRgScript)
	res := runStepsBin(t, bin, work, hookEnv(searchEnv(fakeDir, capDir)),
		[]tuiStep{{marker: "f.txt", keys: "q"}}, "foo")
	if res.code != 0 {
		t.Fatalf("hooked-environment search exited %d, want 0 (stderr %q)", res.code, res.stderr)
	}
	if !strings.Contains(res.stdout, "f.txt") {
		t.Fatalf("search run lacks the browse view: %q", res.stdout)
	}
	if res.stderr != "" {
		t.Fatalf("hooked environment changed stderr: %q", res.stderr)
	}
	for _, f := range []string{"reap", "ack"} {
		if _, err := os.Stat(filepath.Join(dir, f)); !os.IsNotExist(err) {
			t.Fatalf("%s side channel was written by the production binary", f)
		}
	}

	// The artifact probe: the released binary contains none of the
	// manifest names — the seams are compiled out, not just inert.
	blob, err := os.ReadFile(bin)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range hookManifest {
		if bytes.Contains(blob, []byte(name)) {
			t.Errorf("production binary contains hook name %s", name)
		}
	}
}

// The complementary half: a vrg_testhooks build carries every manifest
// name and answers each runner tuple. VRG_TEST_RUN_FINAL_MODEL and
// VRG_TEST_RUN_ERROR select the (final model, error) shape the real
// program.Run() boundary returns; the matrix proves both halves reach
// the executable's actual post-Run() site unchanged — the model half by
// the invalid-final-model diagnostic only a substituted model produces,
// the error half by the "vrg: <text>" diagnostic and exit 2. The
// baseline run shows the real tuple: q while searching exits 130 with
// the collected warning replayed.
func TestTaggedRunnerSeamSelectsReturnShape(t *testing.T) {
	bin := buildVrgVariant(t, "vrg_testhooks")

	// Sanity: the tagged artifact names every manifest hook — the
	// seams are compiled into this variant, by construction.
	blob, err := os.ReadFile(bin)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range hookManifest {
		if !bytes.Contains(blob, []byte(name)) {
			t.Fatalf("tagged binary lacks hook name %s", name)
		}
	}

	cases := []struct {
		name     string
		env      []string
		code     int
		want     []string // stderr must contain each, once where noted
		unwanted []string
	}{
		{
			name: "baseline real tuple",
			code: 130,
			want: []string{"warn one"},
		},
		{
			name: "valid model with error",
			env:  []string{"VRG_TEST_RUN_ERROR=boom"},
			code: 2,
			want: []string{"warn one", "vrg: boom"},
		},
		{
			name: "explicit valid model with error",
			env:  []string{"VRG_TEST_RUN_FINAL_MODEL=valid", "VRG_TEST_RUN_ERROR=boom"},
			code: 2,
			want: []string{"warn one", "vrg: boom"},
		},
		{
			// The session snapshot survives the substituted model:
			// the collected diagnostic still replays, then the
			// invalid-model diagnostic, then the runtime error.
			name: "nil model with error",
			env:  []string{"VRG_TEST_RUN_FINAL_MODEL=nil", "VRG_TEST_RUN_ERROR=boom"},
			code: 2,
			want: []string{"warn one", "vrg: program returned a nil final model", "vrg: boom"},
		},
		{
			name: "invalid model with error",
			env:  []string{"VRG_TEST_RUN_FINAL_MODEL=invalid", "VRG_TEST_RUN_ERROR=boom"},
			code: 2,
			want: []string{"warn one", "vrg: program returned an unexpected final model", "vrg: boom"},
		},
		{
			// The real tuple here exits 130; the injected nil model
			// takes the unified failure path — exit 2 with the
			// retained diagnostic and the invalid-model diagnostic —
			// the difference pins the nil model, not a quit, as what
			// Run() returned.
			name: "nil model without error",
			env:  []string{"VRG_TEST_RUN_FINAL_MODEL=nil"},
			code: 2,
			want: []string{"warn one", "vrg: program returned a nil final model"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fakeDir, capDir := writeFakeRg(t, fakeRgWarnBlockScript)
			ack := filepath.Join(capDir, "ack")
			env := append(ptyEnv(fakeDir, capDir,
				"VRG_TEST_COLLECT_ACK="+ack), tc.env...)
			r := startPiped(t, bin, env, "foo")
			awaitFileContent(t, ack) // the warning is collected
			r.send(t, "q")           // q while searching cancels
			res := r.finish(t, "vrg foo")
			if res.code != tc.code {
				t.Fatalf("exit = %d, want %d (stderr %q)", res.code, tc.code, res.stderr)
			}
			for _, w := range tc.want {
				if n := strings.Count(res.stderr, w); n != 1 {
					t.Fatalf("stderr carries %q %d times, want once: %q", w, n, res.stderr)
				}
			}
			for _, u := range tc.unwanted {
				if strings.Contains(res.stderr, u) {
					t.Fatalf("stderr carries unwanted %q: %q", u, res.stderr)
				}
			}
		})
	}
}

// The remaining manifest seams on the tagged binary: the diagnostic
// trigger injects VRG_TEST_DIAGNOSTIC_TEXT into the session collection
// (acknowledged like any collected diagnostic and replayed at exit),
// and the failure trigger injects a controlled failure whose diagnostic
// is VRG_TEST_FAIL_DIAGNOSTIC — with the child terminated and reaped,
// which the VRG_TEST_REAP side channel records.
func TestTaggedInjectionSeams(t *testing.T) {
	bin := buildVrgVariant(t, "vrg_testhooks")

	// Diagnostic injection: fifo close lands the text in the session
	// collection while the search still runs.
	t.Run("diagnostic trigger", func(t *testing.T) {
		fakeDir, capDir := writeFakeRg(t, fakeRgBlockScript)
		ack := filepath.Join(capDir, "ack")
		trig := filepath.Join(capDir, "diag-fifo")
		if err := syscall.Mkfifo(trig, 0o600); err != nil {
			t.Fatalf("mkfifo: %v", err)
		}
		r := startPiped(t, bin, ptyEnv(fakeDir, capDir,
			"VRG_TEST_COLLECT_ACK="+ack,
			"VRG_TEST_DIAGNOSTIC_TRIGGER="+trig,
			"VRG_TEST_DIAGNOSTIC_TEXT=injected line one"), "foo")
		fireFifo(trig)
		awaitFileContent(t, ack) // the injected diagnostic was collected
		r.send(t, "q")
		res := r.finish(t, "vrg foo")
		if res.code != 130 {
			t.Fatalf("exit = %d, want 130 (stderr %q)", res.code, res.stderr)
		}
		if n := strings.Count(res.stderr, "injected line one"); n != 1 {
			t.Fatalf("injected diagnostic replayed %d times, want once: %q", n, res.stderr)
		}
	})

	// Failure injection: the fifo close ends the run as a controlled
	// failure carrying the requested diagnostic — exit 2, the child
	// reaped, the diagnostic emitted exactly once.
	t.Run("failure trigger", func(t *testing.T) {
		fakeDir, capDir := writeFakeRg(t, fakeRgBlockScript)
		reap := filepath.Join(capDir, "reap")
		trig := filepath.Join(capDir, "fail-fifo")
		if err := syscall.Mkfifo(trig, 0o600); err != nil {
			t.Fatalf("mkfifo: %v", err)
		}
		r := startPiped(t, bin, ptyEnv(fakeDir, capDir,
			"VRG_TEST_REAP="+reap,
			"VRG_TEST_FAIL_TRIGGER="+trig,
			"VRG_TEST_FAIL_DIAGNOSTIC=chosen failure"), "foo")
		fireFifo(trig)
		res := r.finish(t, "vrg foo")
		if res.code != 2 {
			t.Fatalf("exit = %d, want 2 (stderr %q)", res.code, res.stderr)
		}
		if n := strings.Count(res.stderr, "vrg: chosen failure"); n != 1 {
			t.Fatalf("failure diagnostic emitted %d times, want once: %q", n, res.stderr)
		}
		if got := readFile(t, reap); !strings.Contains(got, "killed") {
			t.Fatalf("reap side channel = %q, want the killed wait status", got)
		}
	})
}
