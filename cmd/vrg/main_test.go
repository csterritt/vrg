package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// binPath is the real cmd/vrg binary built once per test run; subprocess
// assertions cover stream and exit-status ownership at the true process
// boundary.
var binPath string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "vrg-bin")
	if err != nil {
		panic(err)
	}
	binPath = filepath.Join(dir, "vrg")
	build := exec.Command("go", "build", "-o", binPath, ".")
	if out, err := build.CombinedOutput(); err != nil {
		panic("go build ./cmd/vrg failed: " + err.Error() + "\n" + string(out))
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

type runResult struct {
	stdout string
	stderr string
	code   int
}

func runVrg(t *testing.T, args ...string) runResult {
	t.Helper()
	return runVrgFull(t, "", nil, args...)
}

// runVrgFull invokes the built binary. argv0 replaces the process name
// when non-empty; env replaces the environment when non-nil.
func runVrgFull(t *testing.T, argv0 string, env []string, args ...string) runResult {
	t.Helper()
	cmd := exec.Command(binPath, args...)
	if argv0 != "" {
		cmd.Args[0] = argv0
	}
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

func assertHelpRun(t *testing.T, res runResult, args []string) {
	t.Helper()
	if res.code != 0 {
		t.Fatalf("vrg %v exited %d, want 0 (stderr %q)", args, res.code, res.stderr)
	}
	if !strings.HasPrefix(res.stdout, "Usage:") || strings.Count(res.stdout, "Usage:") != 1 {
		t.Fatalf("vrg %v: stdout does not carry exactly one help copy: %q", args, res.stdout)
	}
	if res.stderr != "" {
		t.Fatalf("vrg %v: stderr not empty: %q", args, res.stderr)
	}
	if strings.Contains(res.stdout, "Searching") {
		t.Fatalf("vrg %v: help path produced TUI output: %q", args, res.stdout)
	}
	for _, s := range []string{res.stdout, res.stderr} {
		if strings.Contains(s, "\x1b[?1049") || strings.Contains(s, "\x9b") {
			t.Fatalf("vrg %v: output contains terminal control sequences: %q", args, s)
		}
	}
}

func assertUsageError(t *testing.T, res runResult, args []string) {
	t.Helper()
	if res.code != 2 {
		t.Fatalf("vrg %v exited %d, want 2 (stdout %q)", args, res.code, res.stdout)
	}
	if res.stdout != "" {
		t.Fatalf("vrg %v: stdout not empty on usage error: %q", args, res.stdout)
	}
	// stderr is the module's sanitized single-line diagnostic followed by
	// the generated usage block; never the library's Error:/incorrect
	// usage text.
	diag, rest, found := strings.Cut(res.stderr, "\n")
	if !found || diag == "" || !strings.HasPrefix(diag, "vrg:") {
		t.Fatalf("vrg %v: stderr does not start with a diagnostic line: %q", args, res.stderr)
	}
	if strings.ContainsAny(diag, "\x1b\x9b") {
		t.Fatalf("vrg %v: diagnostic line contains raw control bytes: %q", args, diag)
	}
	if n := strings.Count(rest, "Usage:"); n != 1 {
		t.Fatalf("vrg %v: usage block after diagnostic has %d Usage lines, want 1: %q", args, n, res.stderr)
	}
	if strings.Contains(res.stderr, "incorrect usage") || strings.Contains(res.stderr, "Error:") {
		t.Fatalf("vrg %v: stderr carries library-native output: %q", args, res.stderr)
	}
}

// TestGeneratedHelpStdout is the named group Issue #6 reruns: every
// help-only spelling prints exactly one generated help copy to stdout and
// exits 0 with empty stderr and no search side effects.
func TestGeneratedHelpStdout(t *testing.T) {
	cases := [][]string{
		{},
		{"-h"},
		{"--help"},
		{"foo", "--help", "/nonexistent"},
		{"-h", "foo", "bar", "baz"},
		{"foo", "bar", "baz", "--help"},
		{"-i", "--help"},
		{"-ih"},
		{"--unsupported", "--help"},
		{"foo", "/nonexistent", "--help"},
	}
	for _, args := range cases {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			assertHelpRun(t, runVrg(t, args...), args)
		})
	}
}

// Help paths run without ripgrep on PATH and never exec a fake rg.
func TestHelpWithoutRipgrep(t *testing.T) {
	empty := t.TempDir()
	for _, args := range [][]string{{}, {"-h"}, {"--help"}, {"-ih"}} {
		res := runVrgFull(t, "", []string{"PATH=" + empty}, args...)
		assertHelpRun(t, res, args)
	}
}

func TestHelpDoesNotInvokeRipgrep(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "rg-ran")
	fake := filepath.Join(dir, "rg")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\n: > \""+marker+"\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	res := runVrgFull(t, "", []string{"PATH=" + dir, "VRG_RG_MARKER=" + marker}, "--help")
	assertHelpRun(t, res, []string{"--help"})
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("help-only invocation executed the fake rg on PATH")
	}
}

// Help requested under a hostile executable name must not leak the name
// into output: the application uses the fixed name "vrg".
func TestHelpIgnoresExecutableName(t *testing.T) {
	hostile := "ev\x1b]8;;http://x\x07il"
	res := runVrgFull(t, hostile, nil, "--help")
	assertHelpRun(t, res, []string{"--help"})
	if strings.Contains(res.stdout, "ev") || strings.Contains(res.stderr, "ev") {
		t.Fatalf("hostile argv0 reached output: %q %q", res.stdout, res.stderr)
	}
}

// TestCLIOutputSafety is the named group Issue #6 reruns: no invocation
// may emit native library bytes, duplicate diagnostics, or raw control
// data on either stream.
func TestCLIOutputSafety(t *testing.T) {
	// Hostile operand bytes must be escaped, not emitted raw.
	res := runVrg(t, "foo", "/no\x1b[31msuch")
	assertUsageError(t, res, []string{"foo", "/no\x1b[31msuch"})
	if strings.Contains(res.stderr, "\x1b") {
		t.Fatalf("raw escape byte reached stderr: %q", res.stderr)
	}
	if !strings.Contains(res.stderr, "^[") {
		t.Fatalf("escaped form missing from diagnostic: %q", res.stderr)
	}

	res = runVrg(t, "foo", "bad\xffroot")
	assertUsageError(t, res, []string{"foo", "bad\xffroot"})
	if strings.Contains(res.stderr, "\xff") {
		t.Fatalf("invalid byte reached stderr raw: %q", res.stderr)
	}
	if !strings.Contains(res.stderr, `\xff`) {
		t.Fatalf("escaped invalid byte missing: %q", res.stderr)
	}

	res = runVrg(t, "--bad\x1b[7mopt")
	assertUsageError(t, res, []string{"--bad\x1b[7mopt"})
	if strings.Contains(res.stderr, "\x1b") {
		t.Fatalf("raw escape byte reached stderr: %q", res.stderr)
	}
}

// fakeRgScript is an rg stand-in for boundary tests: it records its
// argv and working directory into $VRG_CAPTURE_DIR, emits one complete
// single-match JSON stream, and exits 0.
const fakeRgScript = `#!/bin/sh
printf '%s\n' "$@" > "$VRG_CAPTURE_DIR/argv"
pwd > "$VRG_CAPTURE_DIR/cwd"
# Hold the stream open briefly so the harness observes the searching
# screen before the browse view replaces it.
sleep 0.3
printf '%s\n' \
'{"type":"begin","data":{"path":{"text":"f.txt"}}}' \
'{"type":"match","data":{"path":{"text":"f.txt"},"lines":{"text":"hello\n"},"line_number":1,"submatches":[{"match":{"text":"hello"},"start":0,"end":5}]}}' \
'{"type":"end","data":{"path":{"text":"f.txt"},"binary_offset":null}}' \
'{"type":"summary","data":{}}'
exit 0
`

// writeFakeRg installs script as the rg executable in a fresh directory
// and returns it along with a fresh capture directory for its artifacts.
func writeFakeRg(t *testing.T, script string) (fakeDir, capDir string) {
	t.Helper()
	fakeDir = t.TempDir()
	capDir = t.TempDir()
	if err := os.WriteFile(filepath.Join(fakeDir, "rg"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return fakeDir, capDir
}

// searchEnv builds the child environment for a boundary search: the fake
// rg dir on PATH, the capture directory, and a TERM for the TUI.
func searchEnv(fakeDir, capDir string) []string {
	return []string{
		"PATH=" + fakeDir + string(os.PathListSeparator) + os.Getenv("PATH"),
		"VRG_CAPTURE_DIR=" + capDir,
		"TERM=xterm-256color",
	}
}

// tuiStep is one scripted interaction with the running TUI: wait until
// marker has appeared in the captured stdout, then write keys to stdin.
type tuiStep struct{ marker, keys string }

// runVrgTUI runs the built binary with a controlled stdin pipe: it
// watches stdout for the browse view's filename marker, sends q once it
// appears, and returns after the process exits.
func runVrgTUI(t *testing.T, workdir string, env []string, args ...string) runResult {
	t.Helper()
	return runVrgTUISteps(t, workdir, env, []tuiStep{{marker: "f.txt", keys: "q"}}, args...)
}

// runVrgTUISteps runs the built binary driving a sequence of marker/key
// steps: each step's keys are written once its marker has appeared in
// the captured stdout, letting a test dismiss an overlay before the
// underlying screen's marker can appear. The context deadline bounds
// the run so a wedged search fails rather than hangs.
func runVrgTUISteps(t *testing.T, workdir string, env []string, steps []tuiStep, args ...string) runResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binPath, args...)
	cmd.Dir = workdir
	if env != nil {
		cmd.Env = env
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	outCh := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		tmp := make([]byte, 8192)
		step := 0
		for {
			n, rerr := stdout.Read(tmp)
			if n > 0 {
				buf.Write(tmp[:n])
				if step < len(steps) && bytes.Contains(buf.Bytes(), []byte(steps[step].marker)) {
					_, _ = stdin.Write([]byte(steps[step].keys))
					step++
				}
			}
			if rerr != nil {
				break
			}
		}
		stdin.Close()
		outCh <- buf.String()
	}()

	werr := cmd.Wait()
	out := <-outCh
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("vrg %v timed out (stdout %q, stderr %q)", args, out, stderr.String())
	}
	res := runResult{stdout: out, stderr: stderr.String()}
	if werr == nil {
		res.code = 0
		return res
	}
	if ee, ok := werr.(*exec.ExitError); ok {
		res.code = ee.ExitCode()
		return res
	}
	t.Fatalf("failed to run %v: %v", args, werr)
	return res
}

// The executable boundary: cmd/vrg alone chooses statuses and streams.
func TestExecutableBoundary(t *testing.T) {
	errorCases := [][]string{
		{"--"},            // missing pattern
		{"a", "b", "c"},   // excess operands
		{"--unsupported"}, // unsupported option
		{"foo", "-z"},     // unsupported short option
		{"foo", "/nonexistent-vrg-root"},
		{"foo", "-"}, // stdin root
		{"foo", "/dev/null"},
		{"-i"}, // flags only, no pattern
	}
	for _, args := range errorCases {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			assertUsageError(t, runVrg(t, args...), args)
		})
	}

	// stdin rejection must name stdin.
	res := runVrg(t, "foo", "-")
	if !strings.Contains(res.stderr, "stdin") && !strings.Contains(res.stderr, "standard input") {
		t.Fatalf("stdin root diagnostic does not name stdin: %q", res.stderr)
	}
}

// A regular file literally named "-" validates when addressed as "./-".
func TestDashFileRootAtProcessBoundary(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "-"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The fake rg reports f.txt relative to the working directory;
	// create it so its load succeeds and no diagnostic is collected.
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fakeDir, capDir := writeFakeRg(t, fakeRgScript)
	res := runVrgTUI(t, dir, searchEnv(fakeDir, capDir), "foo", "./-")
	if res.code != 0 {
		t.Fatalf(`vrg foo ./- exited %d, want 0 (stderr %q)`, res.code, res.stderr)
	}
	if !strings.Contains(res.stdout, "f.txt") {
		t.Fatalf("missing browse view: %q", res.stdout)
	}
}

// The child receives exactly the protected argv — rg, the mandatory
// internal flags, the user's flags in encounter order with supplied
// spellings, --, pattern, root — and runs in the invocation working
// directory. Each run shows "Searching…" then the browse view, and
// q exits 0 with empty stderr.
func TestSearchLifecycleAtBoundary(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	subdir := filepath.Join(dir, "sub")
	if err := os.Mkdir(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	linkDir := filepath.Join(dir, "linkdir")
	if err := os.Symlink(subdir, linkDir); err != nil {
		t.Fatal(err)
	}
	work := t.TempDir()
	// The fake rg reports f.txt relative to the working directory;
	// create it so its load succeeds and no diagnostic is collected.
	if err := os.WriteFile(filepath.Join(work, "f.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		args []string
		argv []string
	}{
		{"default root", []string{"foo"}, []string{"--json", "--no-config", "--", "foo", "."}},
		{"explicit dir", []string{"foo", subdir}, []string{"--json", "--no-config", "--", "foo", subdir}},
		{"regular file", []string{"foo", file}, []string{"--json", "--no-config", "--", "foo", file}},
		{"symlink root", []string{"foo", linkDir}, []string{"--json", "--no-config", "--", "foo", linkDir}},
		{"empty pattern", []string{"", "."}, []string{"--json", "--no-config", "--", "", "."}},
		{"literal dash pattern", []string{"-", "."}, []string{"--json", "--no-config", "--", "-", "."}},
		{"help-like operand", []string{"--", "--help"}, []string{"--json", "--no-config", "--", "--help", "."}},
		{"-h operand", []string{"--", "-h"}, []string{"--json", "--no-config", "--", "-h", "."}},
		{"literal -- operand", []string{"--", "--"}, []string{"--json", "--no-config", "--", "--", "."}},
		{"combined shorts expand", []string{"-iw", "foo", subdir}, []string{"--json", "--no-config", "-i", "-w", "--", "foo", subdir}},
		{"encounter order", []string{"-i", "-s", "-i", "foo"}, []string{"--json", "--no-config", "-i", "-s", "-i", "--", "foo", "."}},
		{"mixed alias order", []string{"--ignore-case", "-s", "-i", "foo"}, []string{"--json", "--no-config", "--ignore-case", "-s", "-i", "--", "foo", "."}},
		{"options interleaved", []string{"foo", "-i", subdir, "-s"}, []string{"--json", "--no-config", "-i", "-s", "--", "foo", subdir}},
		{"unrestricted pair", []string{"-u", "--unrestricted", "foo"}, []string{"--json", "--no-config", "-u", "--unrestricted", "--", "foo", "."}},
		{"dash-leading pattern", []string{"--", "-foo"}, []string{"--json", "--no-config", "--", "-foo", "."}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fakeDir, capDir := writeFakeRg(t, fakeRgScript)
			res := runVrgTUI(t, work, searchEnv(fakeDir, capDir), tc.args...)
			if res.code != 0 {
				t.Fatalf("vrg %v exited %d, want 0 (stderr %q)", tc.args, res.code, res.stderr)
			}
			if res.stderr != "" {
				t.Fatalf("vrg %v: stderr not empty: %q", tc.args, res.stderr)
			}
			if !strings.Contains(res.stdout, "Searching…") {
				t.Fatalf("vrg %v: stdout lacks the searching screen: %q", tc.args, res.stdout)
			}
			if !strings.Contains(res.stdout, "f.txt") {
				t.Fatalf("vrg %v: stdout lacks the browse view: %q", tc.args, res.stdout)
			}
			gotArgv, err := os.ReadFile(filepath.Join(capDir, "argv"))
			if err != nil {
				t.Fatalf("fake rg did not capture argv: %v", err)
			}
			if got := strings.Split(strings.TrimSuffix(string(gotArgv), "\n"), "\n"); !slices.Equal(got, tc.argv) {
				t.Fatalf("vrg %v: child argv = %q, want %q", tc.args, got, tc.argv)
			}
			gotCwd, err := os.ReadFile(filepath.Join(capDir, "cwd"))
			if err != nil {
				t.Fatalf("fake rg did not capture cwd: %v", err)
			}
			wantCwd, _ := filepath.EvalSymlinks(work)
			gotCwdEval, _ := filepath.EvalSymlinks(strings.TrimSpace(string(gotCwd)))
			if gotCwdEval != wantCwd {
				t.Fatalf("vrg %v: child cwd = %q, want %q", tc.args, gotCwd, wantCwd)
			}
		})
	}
}

// A fake rg writing well over pipe capacity to stderr while streaming a
// valid stdout stream neither deadlocks vrg nor loses the stream: the
// post-write handshake file proves the child finished both pipes before
// exiting, the captured stderr opens the warning overlay (its escaped
// NUL bytes read as ^@), and dismissing it reveals the browse view —
// every match landed in the index. Exit status is still 0: rg exited
// cleanly and the stream was intact. The collected diagnostics are then
// replayed to vrg's own stderr once the TUI closes.
func TestDualPipeDrainageAtBoundary(t *testing.T) {
	flood := `#!/bin/sh
printf '%s\n' '{"type":"begin","data":{"path":{"text":"f.txt"}}}'
i=1
while [ "$i" -le 16 ]; do
  head -c 65536 /dev/zero >&2
  printf '%s\n' '{"type":"match","data":{"path":{"text":"f.txt"},"lines":{"text":"hit\n"},"line_number":'"$i"',"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}'
  i=$((i+1))
done
printf '%s\n' \
'{"type":"end","data":{"path":{"text":"f.txt"},"binary_offset":null}}' \
'{"type":"summary","data":{}}'
: > "$VRG_CAPTURE_DIR/writes-done"
exit 0
`
	fakeDir, capDir := writeFakeRg(t, flood)
	res := runVrgTUISteps(t, t.TempDir(), searchEnv(fakeDir, capDir), []tuiStep{
		{marker: "^@", keys: "q"},    // the warning overlay over browse
		{marker: "f.txt", keys: "q"}, // the revealed browse view
	}, "hit")
	if res.code != 0 {
		t.Fatalf("vrg exited %d under stderr flood, want 0 (stderr %q)", res.code, res.stderr)
	}
	if !strings.Contains(res.stdout, "f.txt") {
		t.Fatalf("stdout stream lost under backpressure: %q", res.stdout)
	}
	if _, err := os.Stat(filepath.Join(capDir, "writes-done")); err != nil {
		t.Fatalf("child handshake missing — it did not finish both pipes: %v", err)
	}
	// The collected flood is replayed to vrg's stderr after exit —
	// escaped, so the NUL bytes surface as ^@.
	if !strings.Contains(res.stderr, "^@") {
		t.Fatalf("stderr replay lacks the collected diagnostics: %.200q", res.stderr)
	}
}

// rg that cannot be started produces a sanitized stderr diagnostic and
// exit 2 without entering the TUI.
func TestStartFailureNoRipgrep(t *testing.T) {
	empty := t.TempDir()
	res := runVrgFull(t, "", []string{"PATH=" + empty}, "foo")
	if res.code != 2 {
		t.Fatalf("vrg foo with no rg exited %d, want 2", res.code)
	}
	if res.stdout != "" {
		t.Fatalf("start failure wrote to stdout or entered TUI: %q", res.stdout)
	}
	diag := strings.TrimRight(res.stderr, "\n")
	if !strings.HasPrefix(diag, "vrg:") || !strings.Contains(diag, "rg") {
		t.Fatalf("start failure diagnostic does not name the failure: %q", res.stderr)
	}
	for i := 0; i < len(res.stderr); i++ {
		if c := res.stderr[i]; c != '\n' && (c < 0x20 || c == 0x7f) {
			t.Fatalf("start failure diagnostic carries raw control byte 0x%02x: %q", c, res.stderr)
		}
	}
}

// Help assignment spellings are rejected like every other "=" form: no
// help reaches stdout and the process exits 2 with a usage diagnostic.
func TestHelpAssignmentSpellingsAreUsageErrorsAtBoundary(t *testing.T) {
	for _, args := range [][]string{
		{"foo", "--help=false"},
		{"foo", "-h=false"},
		{"foo", "--help=true"},
	} {
		res := runVrg(t, args...)
		if strings.Contains(res.stdout, "Usage:") {
			t.Fatalf("vrg %v emitted help for an assignment spelling: %q", args, res.stdout)
		}
		assertUsageError(t, res, args)
	}
}

// Issue 2 usage-error classes at the process boundary: exit 2, empty
// stdout, and one sanitized diagnostic line plus the generated usage
// block on stderr.
func TestFlagAndArgvUsageErrors(t *testing.T) {
	for _, args := range [][]string{
		{"-e", "foo"},
		{"foo", "-e"},
		{"--type", "go", "foo"},
		{"--type=go", "foo"},
		{"-uuu", "foo"},
		{"-u", "-uu", "foo"},
		{"-iuuu", "foo"},
		{"--unrestricted", "-uu", "foo"},
		{"--ignore-case=false", "foo"},
		{"-i=false", "foo"},
		{"--unrestricted=false", "foo"},
		{"-foo"},
		{"foo", "-i", "--", "--help"}, // post-terminator --help is a (nonexistent) root, not help
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			assertUsageError(t, runVrg(t, args...), args)
		})
	}
}

// Help precedence at the process boundary with flags present: one help
// copy on stdout, empty stderr, exit 0, no child argv, no TUI.
func TestHelpWithSearchFlags(t *testing.T) {
	for _, args := range [][]string{
		{"-i", "--help"},
		{"-ih", "foo"},
		{"-i", "-s", "--help"},
		{"-uuu", "--help"},
		{"-e", "--help"},
		{"foo", ".", "--help"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			assertHelpRun(t, runVrg(t, args...), args)
		})
	}
}
