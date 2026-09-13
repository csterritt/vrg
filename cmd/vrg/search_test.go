package main

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/creack/pty"
)

// writeFakeRG writes a fake rg script to dir that outputs valid ripgrep
// JSON to stdout. The script writes its argv to the file named by the
// VRG_TEST_ARGV environment variable and its cwd to VRG_TEST_CWD. After
// writing all output, it touches the file named by VRG_TEST_HANDSHAKE to
// signal completion.
func writeFakeRG(t *testing.T, dir string, extraScript string) string {
	t.Helper()
	rgPath := filepath.Join(dir, "rg")
	script := `#!/bin/sh
# Write argv and cwd for test verification
if [ -n "$VRG_TEST_ARGV" ]; then
	printf '%s\n' "$*" > "$VRG_TEST_ARGV"
fi
if [ -n "$VRG_TEST_CWD" ]; then
	pwd > "$VRG_TEST_CWD"
fi
` + extraScript + `
# Output a valid ripgrep JSON stream
echo '{"type":"begin","data":{"path":{"text":"test.txt"}}}'
printf '%s\n' '{"type":"match","data":{"path":{"text":"test.txt"},"lines":{"text":"hello world\n"},"line_number":1,"submatches":[{"match":{"text":"hello"},"start":0,"end":5}]}}'
echo '{"type":"end","data":{"path":{"text":"test.txt"},"binary_offset":null}}'
echo '{"type":"summary","data":{}}'
# Signal completion so the test can send q after the stream is done.
if [ -n "$VRG_TEST_HANDSHAKE" ]; then
	touch "$VRG_TEST_HANDSHAKE"
fi
exit 0
`
	if err := os.WriteFile(rgPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return rgPath
}

// runVrgWithQuit runs vrg with a PTY for stdin/stdout (necessary because
// Bubble Tea's cancelable reader uses epoll, which requires a terminal
// file descriptor, and the renderer needs a terminal for raw mode). It
// reads PTY output concurrently to prevent buffer deadlocks. It sends
// "q" after the handshake file appears (or immediately if handshake is
// empty).
func runVrgWithQuit(t *testing.T, cmd *exec.Cmd, handshake string) (stdout, stderr string, exitCode int) {
	t.Helper()
	var se bytes.Buffer
	cmd.Stderr = &se

	ptmx, ptmxErr := pty.Start(cmd)
	if ptmxErr != nil {
		t.Fatalf("failed to start vrg with PTY: %v", ptmxErr)
	}
	defer func() { _ = ptmx.Close() }()

	// Set a window size so Bubble Tea's renderer has a non-zero frame.
	if err := pty.Setsize(ptmx, &pty.Winsize{Rows: 24, Cols: 80}); err != nil {
		t.Fatalf("failed to set PTY window size: %v", err)
	}

	// Read PTY output concurrently to prevent the PTY buffer from
	// filling up and deadlocking Bubble Tea's renderer.
	var so bytes.Buffer
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		io.Copy(&so, ptmx)
	}()

	// Send q to the PTY after the handshake file appears (or immediately).
	go func() {
		if handshake != "" {
			for i := 0; i < 1000; i++ {
				if _, err := os.Stat(handshake); err == nil {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
		}
		io.WriteString(ptmx, "q")
	}()

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		_ = ptmx.Close()
		<-readDone
		if err == nil {
			return stripAnsi(so.String()), se.String(), 0
		}
		if ee, ok := err.(*exec.ExitError); ok {
			return stripAnsi(so.String()), se.String(), ee.ExitCode()
		}
		t.Fatalf("vrg failed: %v (stderr %q)", err, se.String())
	case <-time.After(30 * time.Second):
		cmd.Process.Kill()
		t.Fatal("vrg did not complete within 30 seconds")
	}
	return "", se.String(), -1
}

// stripAnsi removes ANSI escape sequences from s so the visible text
// can be checked without TUI rendering noise.
func stripAnsi(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b {
			// Skip CSI sequence: ESC [ ... (end at 0x40-0x7e)
			if i+1 < len(s) && s[i+1] == '[' {
				i += 2
				for i < len(s) && (s[i] < 0x40 || s[i] > 0x7e) {
					i++
				}
				continue
			}
			// Skip other escape sequences (ESC + one char)
			i++
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// TestChildArgvAndWorkdir verifies that vrg starts rg with the exact
// protected child argv from the invocation working directory.
func TestChildArgvAndWorkdir(t *testing.T) {
	fakeDir := t.TempDir()
	writeFakeRG(t, fakeDir, "")

	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "test.txt"), []byte("hello world\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// The fake rg records its cwd via the shell `pwd` builtin, which
	// resolves symlinks. On macOS /var is a symlink to /private/var, so
	// resolve symlinks on the expected path for a stable comparison.
	wantCwd, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatal(err)
	}

	argvFile := filepath.Join(t.TempDir(), "argv.txt")
	cwdFile := filepath.Join(t.TempDir(), "cwd.txt")
	handshakeFile := filepath.Join(t.TempDir(), "handshake")

	cmd := exec.Command(binPath, "hello", ".")
	cmd.Dir = repo
	cmd.Env = []string{
		"PATH=" + fakeDir + ":" + os.Getenv("PATH"),
		"VRG_TEST_ARGV=" + argvFile,
		"VRG_TEST_CWD=" + cwdFile,
		"VRG_TEST_HANDSHAKE=" + handshakeFile,
	}
	_, _, exitCode := runVrgWithQuit(t, cmd, handshakeFile)

	// vrg should exit 0 (happy path: q from summary).
	if exitCode != 0 {
		t.Fatalf("vrg exited %d, want 0", exitCode)
	}

	// Verify the fake rg was executed.
	argvBytes, err := os.ReadFile(argvFile)
	if err != nil {
		t.Fatalf("fake rg was not executed (argv file missing): %v", err)
	}
	argv := strings.TrimSpace(string(argvBytes))
	wantArgv := "--json --no-config -- hello ."
	if argv != wantArgv {
		t.Fatalf("child argv = %q, want %q", argv, wantArgv)
	}

	// Verify the working directory is the invocation directory.
	cwdBytes, err := os.ReadFile(cwdFile)
	if err != nil {
		t.Fatalf("fake rg was not executed (cwd file missing): %v", err)
	}
	cwd := strings.TrimSpace(string(cwdBytes))
	if cwd != wantCwd {
		t.Fatalf("child cwd = %q, want %q", cwd, wantCwd)
	}
}

// TestChildArgvWithFlags verifies that user flags are forwarded in order
// to the child argv.
func TestChildArgvWithFlags(t *testing.T) {
	fakeDir := t.TempDir()
	writeFakeRG(t, fakeDir, "")

	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "test.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	argvFile := filepath.Join(t.TempDir(), "argv.txt")
	handshakeFile := filepath.Join(t.TempDir(), "handshake")
	cmd := exec.Command(binPath, "-i", "-S", "hello", ".")
	cmd.Dir = repo
	cmd.Env = []string{
		"PATH=" + fakeDir + ":" + os.Getenv("PATH"),
		"VRG_TEST_ARGV=" + argvFile,
		"VRG_TEST_HANDSHAKE=" + handshakeFile,
	}
	runVrgWithQuit(t, cmd, handshakeFile)

	argvBytes, _ := os.ReadFile(argvFile)
	argv := strings.TrimSpace(string(argvBytes))
	wantArgv := "--json --no-config -i -S -- hello ."
	if argv != wantArgv {
		t.Fatalf("child argv = %q, want %q", argv, wantArgv)
	}
}

// TestStartFailureExit2 verifies that when rg cannot be started (not on
// PATH), vrg prints a sanitized stderr diagnostic and exits 2 without
// entering the TUI.
func TestStartFailureExit2(t *testing.T) {
	emptyDir := t.TempDir()
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "test.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(binPath, "hello", ".")
	cmd.Dir = repo
	cmd.Env = []string{"PATH=" + emptyDir}
	var so, se bytes.Buffer
	cmd.Stdout = &so
	cmd.Stderr = &se
	err := cmd.Run()

	ee, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("vrg did not exit with an error (stdout %q, stderr %q)", so.String(), se.String())
	}
	if ee.ExitCode() != 2 {
		t.Fatalf("vrg exited %d, want 2 (stderr %q)", ee.ExitCode(), se.String())
	}

	// stdout must be empty (no TUI output).
	if so.String() != "" {
		t.Fatalf("stdout not empty on start failure: %q", so.String())
	}

	// stderr must contain a sanitized diagnostic (no raw control bytes).
	diag := se.String()
	if diag == "" {
		t.Fatal("stderr is empty, want a diagnostic")
	}
	if strings.ContainsAny(diag, "\x1b\x9b") {
		t.Fatalf("stderr contains raw control bytes: %q", diag)
	}
	// Must not contain alternate-screen or TUI escape sequences.
	if strings.Contains(diag, "Searching") {
		t.Fatalf("stderr contains TUI output (Searching): %q", diag)
	}
}

// TestStartFailureExplicitPath verifies that invoking vrg by explicit
// binary path with an rg-free PATH produces the start-failure diagnostic
// and exit 2.
func TestStartFailureExplicitPath(t *testing.T) {
	emptyDir := t.TempDir()
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "test.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(binPath, "hello", ".")
	cmd.Dir = repo
	cmd.Env = []string{"PATH=" + emptyDir}
	var so, se bytes.Buffer
	cmd.Stdout = &so
	cmd.Stderr = &se
	err := cmd.Run()

	ee, ok := err.(*exec.ExitError)
	if !ok || ee.ExitCode() != 2 {
		t.Fatalf("vrg exited %v, want exit 2 (stdout %q, stderr %q)", err, so.String(), se.String())
	}
	if so.String() != "" {
		t.Fatalf("stdout not empty: %q", so.String())
	}
}

// TestDualPipeBackpressure verifies that vrg neither deadlocks nor loses
// the stdout stream when rg writes well over pipe capacity to stderr
// interleaved with a valid stdout stream.
func TestDualPipeBackpressure(t *testing.T) {
	fakeDir := t.TempDir()
	rgPath := filepath.Join(fakeDir, "rg")

	// The fake rg writes at least 1 MiB to stderr (well over typical pipe
	// capacity of 64 KiB), interleaved with valid stdout records. A
	// handshake file confirms it finished writing both before exit.
	handshakeFile := filepath.Join(t.TempDir(), "handshake")
	script := `#!/bin/sh
# Write at least 1 MiB to stderr in 10 KiB chunks, interleaved with
# valid stdout records. This exceeds typical pipe capacity and would
# deadlock if vrg does not drain both pipes concurrently.
i=0
while [ $i -lt 100 ]; do
	# 10 KiB to stderr
	head -c 10240 /dev/zero | tr '\0' 'E' >&2
	# Valid JSON record to stdout
	echo '{"type":"begin","data":{"path":{"text":"test.txt"}}}'
	printf '%s\n' '{"type":"match","data":{"path":{"text":"test.txt"},"lines":{"text":"hello\n"},"line_number":1,"submatches":[{"match":{"text":"hello"},"start":0,"end":5}]}}'
	echo '{"type":"end","data":{"path":{"text":"test.txt"},"binary_offset":null}}'
	i=$((i + 1))
done
echo '{"type":"summary","data":{}}'
# Handshake: signal both pipes are done
touch "$VRG_TEST_HANDSHAKE"
exit 0
`
	if err := os.WriteFile(rgPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "test.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(binPath, "hello", ".")
	cmd.Dir = repo
	cmd.Env = []string{
		"PATH=" + fakeDir + ":" + os.Getenv("PATH"),
		"VRG_TEST_HANDSHAKE=" + handshakeFile,
	}
	stdout, _, exitCode := runVrgWithQuit(t, cmd, handshakeFile)

	// vrg should exit 0 (q from summary after collection completed).
	if exitCode != 0 {
		t.Fatalf("vrg exited %d, want 0", exitCode)
	}

	// Verify the handshake file exists (fake rg finished writing both
	// pipes before exit).
	if _, err := os.Stat(handshakeFile); err != nil {
		t.Fatalf("handshake file missing: fake rg did not finish writing both pipes: %v", err)
	}

	// Verify the stdout stream was collected completely: the browse
	// view must be present in vrg's output (the matched file appears).
	if !strings.Contains(stdout, "test.txt") {
		t.Fatalf("vrg stdout does not contain the browse view (stdout stream lost): %q", stdout)
	}
}

// TestStderrCapturedWithoutBlocking verifies that rg writing diagnostics
// to stderr while still exiting 0 with a well-formed stdout stream does
// not block the child.
func TestStderrCapturedWithoutBlocking(t *testing.T) {
	fakeDir := t.TempDir()
	rgPath := filepath.Join(fakeDir, "rg")
	handshakeFile := filepath.Join(t.TempDir(), "handshake")
	script := `#!/bin/sh
# Write some diagnostic text to stderr
echo "warning: some diagnostic" >&2
echo "another warning line" >&2
# Output valid ripgrep JSON to stdout
echo '{"type":"begin","data":{"path":{"text":"test.txt"}}}'
printf '%s\n' '{"type":"match","data":{"path":{"text":"test.txt"},"lines":{"text":"hello\n"},"line_number":1,"submatches":[{"match":{"text":"hello"},"start":0,"end":5}]}}'
echo '{"type":"end","data":{"path":{"text":"test.txt"},"binary_offset":null}}'
echo '{"type":"summary","data":{}}'
touch "$VRG_TEST_HANDSHAKE"
exit 0
`
	if err := os.WriteFile(rgPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "test.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(binPath, "hello", ".")
	cmd.Dir = repo
	cmd.Env = []string{
		"PATH=" + fakeDir + ":" + os.Getenv("PATH"),
		"VRG_TEST_HANDSHAKE=" + handshakeFile,
	}
	stdout, _, exitCode := runVrgWithQuit(t, cmd, handshakeFile)

	if exitCode != 0 {
		t.Fatalf("vrg exited %d, want 0", exitCode)
	}

	// vrg should have completed successfully and show the browse view.
	if !strings.Contains(stdout, "test.txt") {
		t.Fatalf("vrg stdout does not contain the browse view: %q", stdout)
	}
}
