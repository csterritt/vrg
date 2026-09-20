package app

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vrg/internal/cli"
)

// writeFakeRg installs an executable named rg on a test PATH that runs
// the given shell script.
func writeFakeRg(t *testing.T, script string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "rg")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	return dir
}

// A fake rg records the exact child argv and its working directory, then
// emits a valid empty stream. The argv is the protected vector the CLI
// produced: "--json --no-config <user flags> -- pattern root", and the
// child runs from the invocation working directory, not the search root.
func TestChildArgvAndWorkdir(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	argvFile := filepath.Join(dir, "argv")
	pwdFile := filepath.Join(dir, "pwd")
	t.Setenv("VRG_ARGV_FILE", argvFile)
	t.Setenv("VRG_PWD_FILE", pwdFile)
	writeFakeRg(t, `#!/bin/sh
printf '%s\n' "$@" > "$VRG_ARGV_FILE"
pwd -P > "$VRG_PWD_FILE"
printf '%s\n' '{"type":"summary","data":{}}'
`)

	t.Chdir(dir)
	res := cli.Parse([]string{"-iw", "foo", "sub"}, io.Discard, cli.Env{})
	if res.Kind != cli.KindSearch {
		t.Fatalf("cli.Parse kind = %v, want KindSearch (%q)", res.Kind, res.Diagnostic)
	}
	child, err := execStarter(res.Args, dir)
	if err != nil {
		t.Fatalf("execStarter: %v", err)
	}
	out := collect(context.Background(), child, dir, nil)
	if out.err != nil {
		t.Fatalf("collect err = %v", out.err)
	}

	argvBytes, err := os.ReadFile(argvFile)
	if err != nil {
		t.Fatalf("fake rg did not record argv: %v", err)
	}
	gotArgv := strings.Split(strings.TrimSuffix(string(argvBytes), "\n"), "\n")
	wantArgv := []string{"--json", "--no-config", "-i", "-w", "--", "foo", "sub"}
	if strings.Join(gotArgv, "\x00") != strings.Join(wantArgv, "\x00") {
		t.Fatalf("child argv = %q, want %q", gotArgv, wantArgv)
	}

	pwdBytes, err := os.ReadFile(pwdFile)
	if err != nil {
		t.Fatalf("fake rg did not record its working directory: %v", err)
	}
	wantDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(pwdBytes)); got != wantDir {
		t.Fatalf("child working directory = %q, want %q (the invocation directory, not the root)", got, wantDir)
	}
}

// The dual-pipe backpressure fixture: a fake rg writes well over pipe
// capacity to stderr interleaved with a valid stdout stream, with a
// handshake confirming it finished writing both before exit. vrg must
// drain both pipes concurrently so rg never blocks and the stdout stream
// is collected completely.
func TestDualPipeBackpressure(t *testing.T) {
	dir := t.TempDir()
	doneFile := filepath.Join(dir, "done")
	t.Setenv("VRG_DONE_FILE", doneFile)
	// 64 blocks x 16385 bytes ≈ 1 MiB of stderr, far over pipe capacity,
	// interleaved with 64 stdout records.
	writeFakeRg(t, `#!/bin/sh
block=$(printf '%016385d' 0)
printf '%s\n' '{"type":"begin","data":{"path":{"text":"a.txt"}}}'
i=1
while [ "$i" -le 64 ]; do
	printf '%s\n' "$block" >&2
	printf '%s\n' '{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"hit\n"},"line_number":'"$i"',"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}'
	i=$((i+1))
done
printf '%s\n' '{"type":"end","data":{"path":{"text":"a.txt"},"binary_offset":null}}'
printf '%s\n' '{"type":"summary","data":{}}'
: > "$VRG_DONE_FILE"
`)

	child, err := execStarter([]string{"--json", "--no-config", "--", "hit", "."}, dir)
	if err != nil {
		t.Fatalf("execStarter: %v", err)
	}
	done := make(chan searchResult, 1)
	go func() { done <- collect(context.Background(), child, dir, nil) }()

	var res searchResult
	select {
	case res = <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("collect deadlocked: stderr pipe filled and blocked rg")
	}
	if res.err != nil {
		t.Fatalf("collect err = %v", res.err)
	}
	if _, err := os.Stat(doneFile); err != nil {
		t.Fatalf("fake rg never finished writing both streams: %v", err)
	}
	if n := len(res.stderr); n < 1<<20 {
		t.Fatalf("captured stderr = %d bytes, want at least 1 MiB", n)
	}
	if n := len(res.index.Stops()); n != 64 {
		t.Fatalf("index stops = %d, want 64: stdout records were lost", n)
	}
}
