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
	if !strings.Contains(res.stdout, `root="."`) && !strings.Contains(res.stdout, "root=.") {
		t.Fatalf("default root missing from stub output: %q", res.stdout)
	}

	errorCases := [][]string{
		{"--"},            // missing pattern
		{"a", "b", "c"},   // excess operands
		{"--unsupported"}, // unsupported option
		{"foo", "-x"},     // unsupported short option
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

// Help assignment spellings that disable help are not help requests: no
// help reaches stdout. The eventual status is Issue 2's and is not pinned.
func TestHelpAssignmentSpellingsAreNotHelp(t *testing.T) {
	for _, args := range [][]string{
		{"foo", "--help=false"},
		{"foo", "-h=false"},
	} {
		res := runVrg(t, args...)
		if strings.Contains(res.stdout, "Usage:") {
			t.Fatalf("vrg %v emitted help for a help-disabling spelling: %q", args, res.stdout)
		}
	}
}
