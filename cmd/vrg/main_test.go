package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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

func runVrgIn(t *testing.T, dir string, args ...string) runResult {
	t.Helper()
	cmd := exec.Command(binPath, args...)
	cmd.Dir = dir
	var so, se bytes.Buffer
	cmd.Stdout = &so
	cmd.Stderr = &se
	err := cmd.Run()
	res := runResult{stdout: so.String(), stderr: se.String()}
	if err == nil {
		return res
	}
	if ee, ok := err.(*exec.ExitError); ok {
		res.code = ee.ExitCode()
		return res
	}
	t.Fatalf("failed to run %v in %s: %v", args, dir, err)
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
	if strings.Contains(res.stdout, "search stub:") {
		t.Fatalf("vrg %v: help path produced stub output: %q", args, res.stdout)
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
		{"-ih", "foo"},
		{"--ignore-case", "--help"},
		{"foo", "src", "--help"},
		{"-uuu", "--help"},
		{"--ignore-case=false", "--help"},
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

	// The success stub escapes its substitutions too.
	res = runVrg(t, "pa\x1b[31mt", ".")
	if res.code != 0 {
		t.Fatalf("stub run exited %d: %q", res.code, res.stderr)
	}
	if strings.Contains(res.stdout, "\x1b") {
		t.Fatalf("raw escape byte reached stdout stub: %q", res.stdout)
	}
	if !strings.Contains(res.stdout, "^[") {
		t.Fatalf("stub did not show the escaped pattern: %q", res.stdout)
	}
}

// The executable boundary: cmd/vrg alone chooses statuses and streams.
func TestExecutableBoundary(t *testing.T) {
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

	searchCases := []struct {
		name string
		args []string
	}{
		{"default root", []string{"foo"}},
		{"explicit dir", []string{"foo", subdir}},
		{"regular file", []string{"foo", file}},
		{"symlink root", []string{"foo", linkDir}},
		{"empty pattern", []string{"", "."}},
		{"literal dash pattern", []string{"-", "."}},
		{"help-like operand", []string{"--", "--help"}},
		{"-h operand", []string{"--", "-h"}},
		{"literal -- operand", []string{"--", "--"}},
	}
	for _, tc := range searchCases {
		t.Run(tc.name, func(t *testing.T) {
			res := runVrg(t, tc.args...)
			if res.code != 0 {
				t.Fatalf("vrg %v exited %d, want 0 (stderr %q)", tc.args, res.code, res.stderr)
			}
			if res.stderr != "" {
				t.Fatalf("vrg %v: stderr not empty: %q", tc.args, res.stderr)
			}
			if !strings.Contains(res.stdout, "search stub:") {
				t.Fatalf("vrg %v: missing stub output: %q", tc.args, res.stdout)
			}
		})
	}

	res := runVrg(t, "foo")
	if want := "search stub: argv=rg --json --no-config -- foo .\n"; res.stdout != want {
		t.Fatalf("stub did not print the child argv: got %q, want %q", res.stdout, want)
	}

	errorCases := [][]string{
		{"--"},            // missing pattern
		{"a", "b", "c"},   // excess operands
		{"--unsupported"}, // unsupported option
		{"foo", "-e"},     // unsupported short option
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
	res = runVrg(t, "foo", "-")
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
	res := runVrgIn(t, dir, "foo", "./-")
	if res.code != 0 {
		t.Fatalf(`vrg foo ./- exited %d, want 0 (stderr %q)`, res.code, res.stderr)
	}
	if !strings.Contains(res.stdout, "search stub:") {
		t.Fatalf("missing stub output: %q", res.stdout)
	}
}

// Help assignment spellings that disable help are not help requests:
// Issue 2 rejects them lexically as usage errors with nothing on stdout.
func TestHelpAssignmentSpellingsAreNotHelp(t *testing.T) {
	for _, args := range [][]string{
		{"foo", "--help=false"},
		{"foo", "-h=false"},
	} {
		assertUsageError(t, runVrg(t, args...), args)
	}
}

// The success stub prints the exact protected child argv: the mandatory
// internal flags, the user's spellings in encounter order with combined
// shorts expanded, the terminator, then pattern and root.
func TestChildArgv(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"plain pattern", []string{"foo"}, "search stub: argv=rg --json --no-config -- foo .\n"},
		{"combined shorts with root", []string{"-iw", "foo", dir}, "search stub: argv=rg --json --no-config -i -w -- foo " + dir + "\n"},
		{"option after pattern", []string{"foo", "-i", dir}, "search stub: argv=rg --json --no-config -i -- foo " + dir + "\n"},
		{"repeated and mixed options", []string{"-i", "-s", "-i", "foo"}, "search stub: argv=rg --json --no-config -i -s -i -- foo .\n"},
		{"combined repetition", []string{"-isi", "foo"}, "search stub: argv=rg --json --no-config -i -s -i -- foo .\n"},
		{"combined three kinds", []string{"-iwF", "foo"}, "search stub: argv=rg --json --no-config -i -w -F -- foo .\n"},
		{"long and short aliases", []string{"--ignore-case", "-s", "-i", "foo"}, "search stub: argv=rg --json --no-config --ignore-case -s -i -- foo .\n"},
		{"options interleaved with both operands", []string{"foo", "-i", dir, "-s"}, "search stub: argv=rg --json --no-config -i -s -- foo " + dir + "\n"},
		{"unrestricted mixed aliases", []string{"-u", "--unrestricted", "foo"}, "search stub: argv=rg --json --no-config -u --unrestricted -- foo .\n"},
		{"unrestricted pair in one token", []string{"-iuu", "foo"}, "search stub: argv=rg --json --no-config -i -u -u -- foo .\n"},
		{"empty pattern verbatim", []string{"", "."}, "search stub: argv=rg --json --no-config --  .\n"},
		{"literal dash pattern", []string{"-", "."}, "search stub: argv=rg --json --no-config -- - .\n"},
		{"dash-leading pattern", []string{"--", "-foo"}, "search stub: argv=rg --json --no-config -- -foo .\n"},
		{"literal -- pattern", []string{"--", "--"}, "search stub: argv=rg --json --no-config -- -- .\n"},
		{"literal -- pattern explicit root", []string{"--", "--", dir}, "search stub: argv=rg --json --no-config -- -- " + dir + "\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := runVrg(t, tc.args...)
			if res.code != 0 {
				t.Fatalf("vrg %v exited %d, want 0 (stderr %q)", tc.args, res.code, res.stderr)
			}
			if res.stderr != "" {
				t.Fatalf("vrg %v: stderr not empty: %q", tc.args, res.stderr)
			}
			if res.stdout != tc.want {
				t.Fatalf("vrg %v: stdout = %q, want %q", tc.args, res.stdout, tc.want)
			}
		})
	}
}

// Flag-contract usage failures at the process boundary: unsupported and
// argument-taking options, the third cumulative unrestricted occurrence,
// and boolean assignment spellings all exit 2 with a sanitized diagnostic
// and the generated usage block on stderr.
func TestFlagContractUsageErrors(t *testing.T) {
	for _, args := range [][]string{
		{"-e", "foo"},
		{"--type", "go", "foo"},
		{"--max-count=3", "foo"},
		{"-uuu", "foo"},
		{"-u", "-uu", "foo"},
		{"-iuuu", "foo"},
		{"-u", "--unrestricted", "-u", "foo"},
		{"--unrestricted", "-uu", "foo"},
		{"--ignore-case=false", "foo"},
		{"-i=false", "foo"},
		{"--unrestricted=false", "foo"},
		{"--help=false", "foo"},
		{"-h=false", "foo"},
		{"-foo"},
		{"-i"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			assertUsageError(t, runVrg(t, args...), args)
		})
	}
}
