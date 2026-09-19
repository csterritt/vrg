package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeFakeRG installs an executable shell script as "rg" in dir and
// points PATH at it (prepended so the fake wins).
func writeFakeRG(t *testing.T, script string) {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "rg")
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// waitResult blocks on child.Wait with a failure-only bound: the tests it
// serves must never deadlock, so exceeding the bound is a test failure,
// not a skip.
func waitResult(t *testing.T, c Child) Result {
	t.Helper()
	done := make(chan Result, 1)
	go func() { done <- c.Wait() }()
	select {
	case r := <-done:
		return r
	case <-time.After(30 * time.Second):
		t.Fatal("child.Wait did not return; the child or its pipe drainage is stuck")
		return Result{}
	}
}

// The spawn seam must run "rg" with the exact supplied argv from the
// given working directory. The fake records both through files named by
// its environment.
func TestSpawnArgvAndWorkdir(t *testing.T) {
	dir := t.TempDir()
	argvFile := filepath.Join(dir, "argv")
	cwdFile := filepath.Join(dir, "cwd")
	t.Setenv("FAKE_RG_ARGV_FILE", argvFile)
	t.Setenv("FAKE_RG_CWD_FILE", cwdFile)
	writeFakeRG(t, `
[ -n "$FAKE_RG_ARGV_FILE" ] && { : > "$FAKE_RG_ARGV_FILE"; for a in "$@"; do printf '%s\n' "$a" >> "$FAKE_RG_ARGV_FILE"; done; }
[ -n "$FAKE_RG_CWD_FILE" ] && pwd -P > "$FAKE_RG_CWD_FILE"
printf '%s\n' '{"type":"summary","data":{"stats":{}}}'
`)

	workdir := t.TempDir()
	child, err := spawn(context.Background(), []string{"--json", "--no-config", "-i", "-w", "--", "patt ern", "root dir"}, workdir)
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	res := waitResult(t, child)
	if res.Err != nil || res.Code != 0 {
		t.Fatalf("child result = code %d err %v, want clean exit 0", res.Code, res.Err)
	}
	if !strings.Contains(string(res.Stdout), `"type":"summary"`) {
		t.Fatalf("stdout not collected: %q", res.Stdout)
	}

	argv, err := os.ReadFile(argvFile)
	if err != nil {
		t.Fatalf("fake rg did not record argv: %v", err)
	}
	want := "--json\n--no-config\n-i\n-w\n--\npatt ern\nroot dir\n"
	if string(argv) != want {
		t.Fatalf("child argv = %q, want %q", argv, want)
	}
	cwd, err := os.ReadFile(cwdFile)
	if err != nil {
		t.Fatalf("fake rg did not record cwd: %v", err)
	}
	wantDir, _ := filepath.EvalSymlinks(workdir)
	if strings.TrimSpace(string(cwd)) != wantDir {
		t.Fatalf("child cwd = %q, want %q", cwd, wantDir)
	}
}

// Dual-pipe backpressure: a fake rg that writes well over pipe capacity
// to stderr while streaming valid JSON on stdout must neither deadlock
// nor lose the stdout stream. The handshake file proves the child
// finished writing both before exit.
func TestDualPipeDrainage(t *testing.T) {
	dir := t.TempDir()
	handshake := filepath.Join(dir, "done")
	t.Setenv("FAKE_RG_HANDSHAKE_FILE", handshake)
	writeFakeRG(t, `
printf '%s\n' '{"type":"begin","data":{"path":{"text":"./f"}}}'
i=0
while [ $i -lt 18 ]; do
    dd if=/dev/zero bs=65536 count=1 2>/dev/null | tr '\000' 'e' >&2
    printf '%s\n' '{"type":"match","data":{"path":{"text":"./f"},"lines":{"text":"x f\n"},"line_number":'$((i + 1))',"absolute_offset":0,"submatches":[{"match":{"text":"f"},"start":2,"end":3}]}}'
    i=$((i + 1))
done
printf '%s\n' '{"type":"end","data":{"path":{"text":"./f"},"binary_offset":null,"stats":{}}}'
printf '%s\n' '{"type":"summary","data":{"stats":{}}}'
[ -n "$FAKE_RG_HANDSHAKE_FILE" ] && : > "$FAKE_RG_HANDSHAKE_FILE"
`)

	child, err := spawn(context.Background(), nil, t.TempDir())
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	res := waitResult(t, child)
	if res.Err != nil || res.Code != 0 {
		t.Fatalf("child result = code %d err %v, want clean exit 0", res.Code, res.Err)
	}
	if _, err := os.Stat(handshake); err != nil {
		t.Fatalf("handshake missing: child did not finish writing both pipes: %v", err)
	}
	if len(res.Stderr) < 1<<20 {
		t.Fatalf("stderr collected = %d bytes, want at least 1 MiB", len(res.Stderr))
	}
	if got := strings.Count(string(res.Stdout), `"type":"match"`); got != 18 {
		t.Fatalf("stdout lost records: %d match records, want 18", got)
	}
	if !strings.Contains(string(res.Stdout), `"type":"summary"`) {
		t.Fatal("stdout stream missing its summary record")
	}
}

// A missing rg is a start error, reported before any collection.
func TestSpawnMissingBinary(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	child, err := spawn(context.Background(), []string{"--json"}, t.TempDir())
	if err == nil {
		t.Fatalf("spawn without rg on PATH = child %v, want error", child)
	}
	if !strings.Contains(err.Error(), "rg") {
		t.Fatalf("start error does not name rg: %v", err)
	}
}
