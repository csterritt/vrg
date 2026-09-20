package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/creack/pty"
	"golang.org/x/sys/unix"
)

// This file is the subprocess boundary harness for Issue #4: a
// controllable fake rg with explicit readiness handshakes run under a
// controlled terminal (a real PTY), so tests assert actual child
// termination, vrg's wait/reap path, display restoration, and restored
// PTY input modes rather than inferring cleanup from a missing PID.
//
// Fixture-owned variables (read by the fake rg scripts, never by vrg):
// FAKE_RG_READY_FILE is a file the fake rg touches once it has started, and
// FAKE_RG_PID_FILE is a file it writes its process ID to. The vrg-consumed
// seams are VRG_TEST_GATE (hold index preparation until the named file
// exists), VRG_TEST_REAP (vrg writes the reaped wait status there),
// VRG_TEST_COLLECT_ACK (vrg appends each collected diagnostic line
// there), VRG_TEST_FAIL_TRIGGER (inject a controlled application failure
// when the named file appears), VRG_TEST_FAIL_DIAGNOSTIC (its
// text), and VRG_TEST_ACK (vrg appends one acknowledgement record per
// Update-processed message and committed transition — the Issue 48
// deterministic-handshake stream).

// lockedBuffer is a bytes.Buffer safe for concurrent writers and
// readers; PTY and stderr copier goroutines write while the test polls.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// ptyRun drives one vrg subprocess under a controlled terminal.
type ptyRun struct {
	cmd    *exec.Cmd
	master *os.File
	slave  *os.File // the parent's slave end; kept open until after Wait
	out    lockedBuffer
	stderr lockedBuffer
	done   chan error
	// copyDone closes when the PTY copier reaches master EOF — after
	// every slave end is closed and its buffered bytes are drained.
	copyDone chan struct{}
	// ackPath is the run's VRG_TEST_ACK acknowledgement stream — the
	// per-process event log the handshake helpers wait on — or empty
	// when the run was launched without one armed.
	ackPath string
	before  unix.Termios
	after   unix.Termios
}

// pollBounded polls cond until it holds, returning a descriptive error
// when the deadline passes. Its sleep only paces re-checks of an
// explicit condition — it is the package's single pacing site.
func pollBounded(what string, timeout time.Duration, cond func() bool) error {
	deadline := time.Now().Add(timeout)
	for !cond() {
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
	return nil
}

// waitBounded polls cond until it holds or the deadline fails the test.
func waitBounded(t *testing.T, what string, timeout time.Duration, cond func() bool) {
	t.Helper()
	if err := pollBounded(what, timeout, cond); err != nil {
		t.Fatal(err)
	}
}

// waitFile waits until the named file exists — the fixture's readiness
// handshake.
func waitFile(t *testing.T, path string) {
	t.Helper()
	waitBounded(t, "file "+path, 15*time.Second, func() bool {
		_, err := os.Stat(path)
		return err == nil
	})
}

func (r *ptyRun) output() string { return r.out.String() }

// waitOutput waits until the PTY byte stream contains marker — the
// observable post-state a following key send relies on.
func (r *ptyRun) waitOutput(t *testing.T, marker string) {
	t.Helper()
	waitBounded(t, "PTY output "+strconv.Quote(marker), 15*time.Second,
		func() bool { return strings.Contains(r.output(), marker) })
}

// send writes key bytes to the terminal input.
func (r *ptyRun) send(t *testing.T, keys string) {
	t.Helper()
	if _, err := r.master.WriteString(keys); err != nil {
		t.Fatalf("PTY write %q: %v", keys, err)
	}
}

// waitExit waits for process exit, snapshots the post-exit termios, and
// returns the exit status.
func (r *ptyRun) waitExit(t *testing.T) int {
	t.Helper()
	select {
	case err := <-r.done:
		if err == nil {
			return 0
		}
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode()
		}
		t.Fatalf("waiting for vrg: %v", err)
	case <-time.After(15 * time.Second):
		t.Fatal("vrg did not exit")
	}
	return -1
}

// finish captures the post-exit termios and releases the PTY pair. The
// parent's slave closes first so the copier drains the master's buffered
// bytes — including anything written just before process exit — and
// reaches EOF before the master itself is closed.
func (r *ptyRun) finish(t *testing.T) {
	t.Helper()
	if after, err := unix.IoctlGetTermios(int(r.master.Fd()), unix.TCGETS); err == nil {
		r.after = *after
	}
	_ = r.slave.Close()
	select {
	case <-r.copyDone:
	case <-time.After(5 * time.Second):
	}
	_ = r.master.Close()
}

// startVrgPTY launches the built vrg binary on a fresh 80x24 PTY. stdin
// and stdout are the slave; stderr goes to a separate pipe unless merged
// is set, in which case it is additionally forwarded into the PTY stream
// so terminal output and stderr share one ordered byte stream.
func startVrgPTY(t *testing.T, dir string, env []string, merged bool, args ...string) *ptyRun {
	t.Helper()
	return startBinaryPTY(t, binPath, dir, env, merged, args...)
}

// startBinaryPTY launches bin — rather than TestMain's binary — under
// the same PTY harness, so tests can exercise a differently built
// artifact such as the untagged production binary.
func startBinaryPTY(t *testing.T, bin, dir string, env []string, merged bool, args ...string) *ptyRun {
	t.Helper()
	master, slave, err := pty.Open()
	if err != nil {
		t.Fatalf("pty.Open: %v", err)
	}
	if err := pty.Setsize(master, &pty.Winsize{Rows: 24, Cols: 80}); err != nil {
		t.Fatalf("pty.Setsize: %v", err)
	}
	before, err := unix.IoctlGetTermios(int(master.Fd()), unix.TCGETS)
	if err != nil {
		t.Fatalf("termios before launch: %v", err)
	}
	r := &ptyRun{master: master, slave: slave, before: *before}
	for _, kv := range env {
		if v, ok := strings.CutPrefix(kv, "VRG_TEST_ACK="); ok {
			r.ackPath = v
		}
	}
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdin = slave
	cmd.Stdout = slave
	if merged {
		cmd.Stderr = io.MultiWriter(&r.stderr, slave)
	} else {
		cmd.Stderr = &r.stderr
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start vrg: %v", err)
	}
	r.cmd = cmd
	r.done = make(chan error, 1)
	r.copyDone = make(chan struct{})
	go func() { r.done <- cmd.Wait() }()
	go func() { _, _ = io.Copy(&r.out, master); close(r.copyDone) }()
	t.Cleanup(func() {
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
			select {
			case <-r.done:
			case <-time.After(5 * time.Second):
			}
		}
		_ = slave.Close()
		_ = master.Close()
	})
	return r
}

// killPidOnCleanup kills the process whose pid the fixture recorded, so
// a failed run cannot leak a blocked fake rg or detached sleeper.
func killPidOnCleanup(t *testing.T, pidFile string) {
	t.Helper()
	t.Cleanup(func() {
		b, err := os.ReadFile(pidFile)
		if err != nil {
			return
		}
		if pid, err := strconv.Atoi(strings.TrimSpace(string(b))); err == nil {
			_ = syscall.Kill(-pid, syscall.SIGKILL) // the child's process group
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	})
}

// childEnv returns the process environment with each override applied by
// name rather than appended, so overridden variables win.
func childEnv(overrides map[string]string) []string {
	env := make([]string, 0, len(overrides)+16)
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		if _, ok := overrides[k]; !ok {
			env = append(env, kv)
		}
	}
	for k, v := range overrides {
		env = append(env, k+"="+v)
	}
	return env
}

// writeFakeRg installs an executable fake rg script on a private PATH
// directory and returns the environment prefix that resolves it.
func fakeRgPath(t *testing.T, script string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "rg"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func pidAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}

// waitPidGone polls until the pid no longer exists; a killed but
// unreaped zombie still exists, so this distinguishes reaping.
func waitPidGone(t *testing.T, pid int) {
	t.Helper()
	waitBounded(t, "pid gone", 10*time.Second, func() bool { return !pidAlive(pid) })
}

func readPid(t *testing.T, path string) int {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read pid file: %v", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		t.Fatalf("pid file %q: %v", string(b), err)
	}
	return pid
}

// reapStatus returns the contents of the VRG_TEST_REAP side channel —
// the wait status vrg recorded when its reap path ran — failing if vrg
// never reaped the child.
func reapStatus(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("no reap evidence at %s: vrg's wait/reap path did not run: %v", path, err)
	}
	return strings.TrimSpace(string(b))
}

// assertTerminalRestored requires the display-restoration sequence (exit
// alternate screen, cursor visible) in the PTY stream and the PTY's
// termios after exit to equal the termios captured before launch.
func assertTerminalRestored(t *testing.T, r *ptyRun) {
	t.Helper()
	out := r.output()
	if !strings.Contains(out, "\x1b[?1049l") {
		t.Fatalf("PTY output lacks the leave-alternate-screen sequence: %q", out)
	}
	if !strings.Contains(out, "\x1b[?25h") {
		t.Fatalf("PTY output lacks the show-cursor sequence: %q", out)
	}
	if r.before != r.after {
		t.Fatalf("PTY termios changed: before %+v, after %+v", r.before, r.after)
	}
}

// TestQAgainstBlockedFakeRGExits130: fake rg signals readiness then
// blocks; q while searching terminates and reaps the child, restores
// display and input modes, and exits 130.
func TestQAgainstBlockedFakeRGExits130(t *testing.T) {
	dir := t.TempDir()
	ready := filepath.Join(dir, "ready")
	pidFile := filepath.Join(dir, "pid")
	reap := filepath.Join(dir, "reap")
	events := filepath.Join(dir, "events")
	rgDir := fakeRgPath(t, `#!/bin/sh
echo $$ > "$FAKE_RG_PID_FILE"
: > "$FAKE_RG_READY_FILE"
exec sleep 100000
`)
	r := startVrgPTY(t, dir, childEnv(map[string]string{
		"PATH":               rgDir + ":" + os.Getenv("PATH"),
		"TERM":               "xterm-256color",
		"FAKE_RG_READY_FILE": ready,
		"FAKE_RG_PID_FILE":   pidFile,
		"VRG_TEST_REAP":      reap,
		"VRG_TEST_ACK":       events,
	}), false, "foo", ".")
	waitFile(t, ready)
	killPidOnCleanup(t, pidFile)
	r.waitOutput(t, "Searching")
	r.sendAcked(t, "q", "q")
	code := r.waitExit(t)
	r.finish(t)

	if code != 130 {
		t.Fatalf("exit status = %d, want 130", code)
	}
	waitPidGone(t, readPid(t, pidFile))
	if got := reapStatus(t, reap); got != "signal: killed" {
		t.Fatalf("reaped wait status = %q, want %q", got, "signal: killed")
	}
	assertTerminalRestored(t, r)
	if s := r.stderr.String(); s != "" {
		t.Fatalf("cancellation wrote to stderr: %q", s)
	}
}

// TestCtrlCAgainstBlockedFakeRGExits130: the same cancellation contract
// through ctrl+c, delivered as a key press while the terminal is raw.
func TestCtrlCAgainstBlockedFakeRGExits130(t *testing.T) {
	dir := t.TempDir()
	ready := filepath.Join(dir, "ready")
	pidFile := filepath.Join(dir, "pid")
	reap := filepath.Join(dir, "reap")
	events := filepath.Join(dir, "events")
	rgDir := fakeRgPath(t, `#!/bin/sh
echo $$ > "$FAKE_RG_PID_FILE"
: > "$FAKE_RG_READY_FILE"
exec sleep 100000
`)
	r := startVrgPTY(t, dir, childEnv(map[string]string{
		"PATH":               rgDir + ":" + os.Getenv("PATH"),
		"TERM":               "xterm-256color",
		"FAKE_RG_READY_FILE": ready,
		"FAKE_RG_PID_FILE":   pidFile,
		"VRG_TEST_REAP":      reap,
		"VRG_TEST_ACK":       events,
	}), false, "foo", ".")
	waitFile(t, ready)
	killPidOnCleanup(t, pidFile)
	// The rendered searching screen proves raw mode is active, so the
	// ^C byte arrives as a key press rather than a signal.
	r.waitOutput(t, "Searching")
	r.sendAcked(t, "\x03", "ctrl+c")
	code := r.waitExit(t)
	r.finish(t)

	if code != 130 {
		t.Fatalf("exit status = %d, want 130", code)
	}
	waitPidGone(t, readPid(t, pidFile))
	if got := reapStatus(t, reap); got != "signal: killed" {
		t.Fatalf("reaped wait status = %q, want %q", got, "signal: killed")
	}
	assertTerminalRestored(t, r)
	if s := r.stderr.String(); s != "" {
		t.Fatalf("cancellation wrote to stderr: %q", s)
	}
}

// TestQDuringGateHeldPreparationExits130: rg has exited and been reaped,
// but index preparation is held by VRG_TEST_GATE; q in that window is
// cancellation (130), not a browse quit.
func TestQDuringGateHeldPreparationExits130(t *testing.T) {
	dir := t.TempDir()
	ready := filepath.Join(dir, "ready")
	pidFile := filepath.Join(dir, "pid")
	reap := filepath.Join(dir, "reap")
	events := filepath.Join(dir, "events")
	gate := filepath.Join(dir, "gate") // never created: preparation stays held
	rgDir := fakeRgPath(t, `#!/bin/sh
echo $$ > "$FAKE_RG_PID_FILE"
printf '%s\n' '{"type":"summary","data":{}}'
: > "$FAKE_RG_READY_FILE"
exit 0
`)
	r := startVrgPTY(t, dir, childEnv(map[string]string{
		"PATH":               rgDir + ":" + os.Getenv("PATH"),
		"TERM":               "xterm-256color",
		"FAKE_RG_READY_FILE": ready,
		"FAKE_RG_PID_FILE":   pidFile,
		"VRG_TEST_REAP":      reap,
		"VRG_TEST_GATE":      gate,
		"VRG_TEST_ACK":       events,
	}), false, "foo", ".")
	waitFile(t, ready)
	killPidOnCleanup(t, pidFile)
	// The reap record proves rg exited and vrg's wait path ran; the gate
	// still holds index preparation, so the app remains in searching.
	waitBounded(t, "reap record", 15*time.Second, func() bool {
		_, err := os.Stat(reap)
		return err == nil
	})
	r.waitOutput(t, "Searching")
	if strings.Contains(r.output(), "──") {
		t.Fatalf("gate-held run reached the browse screen: %q", r.output())
	}
	r.sendAcked(t, "q", "q")
	code := r.waitExit(t)
	r.finish(t)

	if code != 130 {
		t.Fatalf("exit status = %d, want 130", code)
	}
	if got := reapStatus(t, reap); got != "exit status 0" {
		t.Fatalf("reaped wait status = %q, want %q", got, "exit status 0")
	}
	assertTerminalRestored(t, r)
}

// TestNormalExitReapsChild: an ordinary quit while a process spawned by
// rg still runs leaves nothing behind — vrg's wait path ran (the reap
// record) and cleanup terminates the child's process group.
func TestNormalExitReapsChild(t *testing.T) {
	dir := t.TempDir()
	ready := filepath.Join(dir, "ready")
	pidFile := filepath.Join(dir, "pid")
	reap := filepath.Join(dir, "reap")
	events := filepath.Join(dir, "events")
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("foo bar\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// rg emits a complete stream with one match and exits, leaving a
	// detached-output sleeper in its process group.
	rgDir := fakeRgPath(t, `#!/bin/sh
sleep 100000 </dev/null >/dev/null 2>&1 &
echo $! > "$FAKE_RG_PID_FILE"
printf '%s\n' '{"type":"begin","data":{"path":{"text":"file.txt"}}}'
printf '%s\n' '{"type":"match","data":{"path":{"text":"file.txt"},"lines":{"text":"foo bar\n"},"line_number":1,"submatches":[{"match":{"text":"foo"},"start":0,"end":3}]}}'
printf '%s\n' '{"type":"end","data":{"path":{"text":"file.txt"},"binary_offset":null}}'
printf '%s\n' '{"type":"summary","data":{}}'
: > "$FAKE_RG_READY_FILE"
exit 0
`)
	r := startVrgPTY(t, dir, childEnv(map[string]string{
		"PATH":               rgDir + ":" + os.Getenv("PATH"),
		"TERM":               "xterm-256color",
		"FAKE_RG_READY_FILE": ready,
		"FAKE_RG_PID_FILE":   pidFile,
		"VRG_TEST_REAP":      reap,
		"VRG_TEST_ACK":       events,
	}), false, "foo", ".")
	waitFile(t, ready)
	killPidOnCleanup(t, pidFile)
	r.waitOutput(t, "file.txt")
	r.sendAcked(t, "q", "q")
	code := r.waitExit(t)
	r.finish(t)

	if code != 0 {
		t.Fatalf("exit status = %d, want 0", code)
	}
	if got := reapStatus(t, reap); got != "exit status 0" {
		t.Fatalf("reaped wait status = %q, want %q", got, "exit status 0")
	}
	waitPidGone(t, readPid(t, pidFile))
	assertTerminalRestored(t, r)
}

// TestInjectedControlledFailure: an injected controlled application
// failure after the child signalled ready terminates and reaps the
// child, restores the terminal, writes a sanitized diagnostic to stderr
// exactly once and after the display-restoration sequence, and exits 2.
func TestInjectedControlledFailure(t *testing.T) {
	dir := t.TempDir()
	ready := filepath.Join(dir, "ready")
	pidFile := filepath.Join(dir, "pid")
	reap := filepath.Join(dir, "reap")
	trigger := filepath.Join(dir, "trigger")
	diag := "injected failure \x1b[31m\nsecond line"
	rgDir := fakeRgPath(t, `#!/bin/sh
echo $$ > "$FAKE_RG_PID_FILE"
: > "$FAKE_RG_READY_FILE"
exec sleep 100000
`)
	r := startVrgPTY(t, dir, childEnv(map[string]string{
		"PATH":                     rgDir + ":" + os.Getenv("PATH"),
		"TERM":                     "xterm-256color",
		"FAKE_RG_READY_FILE":       ready,
		"FAKE_RG_PID_FILE":         pidFile,
		"VRG_TEST_REAP":            reap,
		"VRG_TEST_FAIL_TRIGGER":    trigger,
		"VRG_TEST_FAIL_DIAGNOSTIC": diag,
	}), true, "foo", ".")
	waitFile(t, ready)
	killPidOnCleanup(t, pidFile)
	r.waitOutput(t, "Searching")
	if err := os.WriteFile(trigger, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	code := r.waitExit(t)
	r.finish(t)

	if code != 2 {
		t.Fatalf("exit status = %d, want 2", code)
	}
	waitPidGone(t, readPid(t, pidFile))
	if got := reapStatus(t, reap); got != "signal: killed" {
		t.Fatalf("reaped wait status = %q, want %q", got, "signal: killed")
	}
	assertTerminalRestored(t, r)

	// The diagnostic is sanitized to a single line and written exactly
	// once — never both directly and through a replay.
	errOut := r.stderr.String()
	if strings.Count(errOut, "injected failure") != 1 {
		t.Fatalf("diagnostic count = %d, want exactly once: %q",
			strings.Count(errOut, "injected failure"), errOut)
	}
	if !strings.HasPrefix(strings.TrimSpace(errOut), "vrg: ") {
		t.Fatalf("stderr = %q, want a vrg: diagnostic", errOut)
	}
	if strings.Contains(errOut, "\x1b") || strings.Count(strings.TrimSuffix(errOut, "\n"), "\n") != 0 {
		t.Fatalf("diagnostic is not sanitized to a single line: %q", errOut)
	}
	if !strings.Contains(errOut, "^[") {
		t.Fatalf("escaped control byte missing from diagnostic: %q", errOut)
	}

	// stderr was merged into the PTY stream, so ordering is observable:
	// the diagnostic appears after the display-restoration sequence.
	out := r.output()
	restore := strings.Index(out, "\x1b[?1049l")
	diagAt := strings.Index(out, "injected failure")
	if restore < 0 || diagAt < 0 || diagAt < restore {
		t.Fatalf("diagnostic not after display restoration (restore@%d, diag@%d): %q",
			restore, diagAt, out)
	}
}
