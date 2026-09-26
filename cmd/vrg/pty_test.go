//go:build linux

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// The PTY harness: the built vrg binary runs with a pseudo-terminal
// slave as stdin, stdout, and stderr — the arrangement a real terminal
// session has — while a controllable fake rg on PATH exposes explicit
// readiness (a "ready" file carrying its pid) and completion (the exit
// status vrg reports through its VRG_TEST_REAP side channel)
// handshakes. Asserting the slave's termios after exit equals its
// pre-launch value proves vrg restored the input modes; asserting the
// captured stream carries the alt-screen-exit and cursor-show sequences
// proves display restoration.

// pty is an open pseudo-terminal master/slave pair plus the slave's
// termios captured before the child launched.
type pty struct {
	master *os.File
	slave  *os.File
	before unix.Termios
}

func openPTY(t *testing.T) *pty {
	t.Helper()
	return openPTYSize(t, 80, 24)
}

// openPTYSize opens a pseudo-terminal pair at the given dimensions —
// the sized variant behind openPTY for tests needing a nonstandard
// frame.
func openPTYSize(t *testing.T, cols, rows uint16) *pty {
	t.Helper()
	mfd, err := unix.Open("/dev/ptmx", unix.O_RDWR|unix.O_NOCTTY|unix.O_CLOEXEC, 0)
	if err != nil {
		t.Skipf("pseudo-terminal unavailable: %v", err)
	}
	master := os.NewFile(uintptr(mfd), "ptmx")
	t.Cleanup(func() { master.Close() })
	if err := unix.IoctlSetPointerInt(mfd, unix.TIOCSPTLCK, 0); err != nil {
		t.Fatalf("unlockpt: %v", err)
	}
	n, err := unix.IoctlGetInt(mfd, unix.TIOCGPTN)
	if err != nil {
		t.Fatalf("TIOCGPTN: %v", err)
	}
	slave, err := os.OpenFile(fmt.Sprintf("/dev/pts/%d", n), os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		t.Fatalf("open slave: %v", err)
	}
	t.Cleanup(func() { slave.Close() })
	if err := unix.IoctlSetWinsize(int(slave.Fd()), unix.TIOCSWINSZ,
		&unix.Winsize{Row: rows, Col: cols}); err != nil {
		t.Fatalf("set winsize: %v", err)
	}
	return &pty{master: master, slave: slave, before: termiosOf(t, slave)}
}

func termiosOf(t *testing.T, f *os.File) unix.Termios {
	t.Helper()
	tio, err := unix.IoctlGetTermios(int(f.Fd()), unix.TCGETS)
	if err != nil {
		t.Fatalf("TCGETS on %s: %v", f.Name(), err)
	}
	return *tio
}

// ptyRun is a vrg process attached to the PTY slave with its output
// continuously captured from the master.
type ptyRun struct {
	cmd      *exec.Cmd
	pty      *pty
	mu       sync.Mutex
	out      bytes.Buffer
	readDone chan struct{}
}

func startVrgPTY(t *testing.T, env []string, args ...string) *ptyRun {
	t.Helper()
	return startVrgPTYSize(t, env, 80, 24, args...)
}

// startVrgPTYSize is startVrgPTY on a PTY of the given dimensions.
func startVrgPTYSize(t *testing.T, env []string, cols, rows uint16, args ...string) *ptyRun {
	t.Helper()
	p := openPTYSize(t, cols, rows)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, binPath, args...)
	cmd.Env = env
	cmd.Dir = t.TempDir()
	cmd.Stdin = p.slave
	cmd.Stdout = p.slave
	cmd.Stderr = p.slave
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start vrg: %v", err)
	}
	r := &ptyRun{cmd: cmd, pty: p, readDone: make(chan struct{})}
	go func() {
		defer close(r.readDone)
		buf := make([]byte, 8192)
		for {
			n, err := p.master.Read(buf)
			if n > 0 {
				r.mu.Lock()
				r.out.Write(buf[:n])
				r.mu.Unlock()
			}
			if err != nil {
				return // EIO once every slave fd is closed
			}
		}
	}()
	return r
}

func (r *ptyRun) output() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.out.String()
}

// waitFor blocks until marker appears in the captured stream or the
// process ends the session first.
func (r *ptyRun) waitFor(t *testing.T, marker string) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(r.output(), marker) {
			return
		}
		select {
		case <-r.readDone:
			t.Fatalf("session ended before %q appeared; output: %q", marker, r.output())
		case <-time.After(5 * time.Millisecond):
		}
	}
	t.Fatalf("timed out waiting for %q; output: %q", marker, r.output())
}

// send writes raw key bytes to the master, exactly what a terminal
// delivers to the child once it has entered raw mode.
func (r *ptyRun) send(t *testing.T, keys string) {
	t.Helper()
	if _, err := r.pty.master.WriteString(keys); err != nil {
		t.Fatalf("writing %q to PTY: %v", keys, err)
	}
}

// waitForGrowth blocks until the captured stream has grown past since —
// evidence the model handled the sent key and emitted a fresh frame,
// used where dismissal replaces marker text the buffer already holds.
func (r *ptyRun) waitForGrowth(t *testing.T, since int) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if len(r.output()) > since {
			return
		}
		select {
		case <-r.readDone:
			t.Fatalf("session ended before a new frame; output: %q", r.output())
		case <-time.After(5 * time.Millisecond):
		}
	}
	t.Fatalf("timed out waiting for a new frame; output: %q", r.output())
}

// waitExit waits for the process to exit and returns its code and the
// complete captured stream. It asserts the slave termios came back to
// its pre-launch state — the input-mode half of terminal restoration.
func (r *ptyRun) waitExit(t *testing.T) (int, string) {
	t.Helper()
	err := r.cmd.Wait()
	if after := termiosOf(t, r.pty.slave); after != r.pty.before {
		t.Fatalf("slave termios not restored:\nbefore %+v\nafter  %+v", r.pty.before, after)
	}
	r.pty.slave.Close()
	<-r.readDone
	r.pty.master.Close()
	out := r.output()
	if err == nil {
		return 0, out
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode(), out
	}
	t.Fatalf("wait: %v (output %q)", err, out)
	return 0, out
}

// assertDisplayRestored requires the leave-alternate-screen and
// cursor-show sequences in the captured stream — the display half of
// terminal restoration.
func assertDisplayRestored(t *testing.T, out string) {
	t.Helper()
	for _, seq := range []string{"\x1b[?1049l", "\x1b[?25h"} {
		if !strings.Contains(out, seq) {
			t.Fatalf("output lacks display-restoration sequence %q; full output: %q", seq, out)
		}
	}
}

// awaitReadyPID returns the pid the fake rg wrote into its readiness
// handshake file.
func awaitReadyPID(t *testing.T, capDir string) int {
	t.Helper()
	pid, err := strconv.Atoi(strings.TrimSpace(awaitFileContent(t, filepath.Join(capDir, "ready"))))
	if err != nil {
		t.Fatalf("ready file does not carry a pid: %v", err)
	}
	return pid
}

// awaitFileContent polls until the file exists with nonempty content.
func awaitFileContent(t *testing.T, path string) string {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if b, err := os.ReadFile(path); err == nil && len(b) > 0 {
			return string(b)
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s to have content", path)
	return ""
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// pidIsGone requires the pid to be fully gone — ESRCH — so a terminated
// child is proven reaped rather than a lingering zombie.
func pidIsGone(t *testing.T, pid int) {
	t.Helper()
	if err := syscall.Kill(pid, 0); err != syscall.ESRCH {
		t.Fatalf("pid %d still exists (err=%v), want it gone", pid, err)
	}
}

// ptyEnv builds the child environment for a PTY run: the fake rg first
// on PATH, the capture directory, a real TERM, and the VRG_TEST_* seams
// a test opts into.
func ptyEnv(fakeDir, capDir string, extra ...string) []string {
	return append([]string{
		"PATH=" + fakeDir + string(os.PathListSeparator) + os.Getenv("PATH"),
		"VRG_CAPTURE_DIR=" + capDir,
		"TERM=xterm-256color",
	}, extra...)
}

// fakeRgBlockScript signals readiness with its pid then blocks forever:
// the run can only end through cancellation, a signal, or failure.
const fakeRgBlockScript = `#!/bin/sh
echo $$ > "$VRG_CAPTURE_DIR/ready"
exec sleep 3600
`

// fakeRgStreamScript signals readiness, emits one complete result
// stream, and exits 0 — the post-exit preparation window is where the
// gate test holds the app.
const fakeRgStreamScript = `#!/bin/sh
echo $$ > "$VRG_CAPTURE_DIR/ready"
printf '%s\n' \
'{"type":"begin","data":{"path":{"text":"f.txt"}}}' \
'{"type":"match","data":{"path":{"text":"f.txt"},"lines":{"text":"hello\n"},"line_number":1,"submatches":[{"match":{"text":"hello"},"start":0,"end":5}]}}' \
'{"type":"end","data":{"path":{"text":"f.txt"},"binary_offset":null}}' \
'{"type":"summary","data":{}}'
exit 0
`

// q while searching cancels: exit 130, the blocked child terminated and
// reaped — proven by the reap side channel and the vanished pid — and
// both halves of terminal restoration hold.
func TestPTYQWhileSearchingExits130(t *testing.T) {
	fakeDir, capDir := writeFakeRg(t, fakeRgBlockScript)
	reap := filepath.Join(capDir, "reap")
	r := startVrgPTY(t, ptyEnv(fakeDir, capDir, "VRG_TEST_REAP="+reap), "foo")
	pid := awaitReadyPID(t, capDir)
	r.waitFor(t, "Searching…")
	r.send(t, "q")
	code, out := r.waitExit(t)
	if code != 130 {
		t.Fatalf("exit = %d, want 130; output: %q", code, out)
	}
	assertDisplayRestored(t, out)
	if got := readFile(t, reap); !strings.Contains(got, "killed") {
		t.Fatalf("reap side channel = %q, want the killed wait status", got)
	}
	pidIsGone(t, pid)
}

// ctrl+c under a raw terminal arrives as byte 0x03; the same contract
// as q applies: exit 130 after full cleanup.
func TestPTYCtrlCWhileSearchingExits130(t *testing.T) {
	fakeDir, capDir := writeFakeRg(t, fakeRgBlockScript)
	reap := filepath.Join(capDir, "reap")
	r := startVrgPTY(t, ptyEnv(fakeDir, capDir, "VRG_TEST_REAP="+reap), "foo")
	pid := awaitReadyPID(t, capDir)
	r.waitFor(t, "Searching…")
	r.send(t, "\x03")
	code, out := r.waitExit(t)
	if code != 130 {
		t.Fatalf("exit = %d, want 130; output: %q", code, out)
	}
	assertDisplayRestored(t, out)
	if got := readFile(t, reap); !strings.Contains(got, "killed") {
		t.Fatalf("reap side channel = %q, want the killed wait status", got)
	}
	pidIsGone(t, pid)
}

// A real SIGINT — the signal, not the keystroke — takes the same
// cancellation path through the boundary.
func TestPTYSIGINTWhileSearchingExits130(t *testing.T) {
	fakeDir, capDir := writeFakeRg(t, fakeRgBlockScript)
	reap := filepath.Join(capDir, "reap")
	r := startVrgPTY(t, ptyEnv(fakeDir, capDir, "VRG_TEST_REAP="+reap), "foo")
	pid := awaitReadyPID(t, capDir)
	r.waitFor(t, "Searching…")
	if err := r.cmd.Process.Signal(syscall.SIGINT); err != nil {
		t.Fatalf("SIGINT: %v", err)
	}
	code, out := r.waitExit(t)
	if code != 130 {
		t.Fatalf("exit = %d, want 130; output: %q", code, out)
	}
	assertDisplayRestored(t, out)
	if got := readFile(t, reap); !strings.Contains(got, "killed") {
		t.Fatalf("reap side channel = %q, want the killed wait status", got)
	}
	pidIsGone(t, pid)
}

// q while index preparation is gate-held — rg has already exited and
// been reaped, which the side channel proves — is cancellation: exit
// 130, no browse view, ordinary cleanup.
func TestPTYQDuringGateHeldPreparationExits130(t *testing.T) {
	fakeDir, capDir := writeFakeRg(t, fakeRgStreamScript)
	reap := filepath.Join(capDir, "reap")
	gate := filepath.Join(capDir, "gate-fifo")
	if err := syscall.Mkfifo(gate, 0o600); err != nil {
		t.Fatalf("mkfifo: %v", err)
	}
	r := startVrgPTY(t, ptyEnv(fakeDir, capDir,
		"VRG_TEST_REAP="+reap,
		"VRG_TEST_GATE="+gate,
	), "foo")
	pid := awaitReadyPID(t, capDir)
	r.waitFor(t, "Searching…")
	// The child exited and vrg already waited on it while preparation is
	// still gate-held: this is the post-exit preparation window.
	if got := awaitFileContent(t, reap); !strings.Contains(got, "exit status 0") {
		t.Fatalf("reap side channel = %q, want the child's clean exit", got)
	}
	r.send(t, "q")
	code, out := r.waitExit(t)
	if code != 130 {
		t.Fatalf("exit = %d, want 130; output: %q", code, out)
	}
	if strings.Contains(out, "f.txt") {
		t.Fatalf("gate-held search showed the browse view: %q", out)
	}
	assertDisplayRestored(t, out)
	pidIsGone(t, pid)
}

// An ordinary exit while rg still runs — here SIGTERM ends the program
// without a cancellation key — still terminates and reaps the child and
// restores the terminal.
func TestPTYOrdinaryExitReapsChild(t *testing.T) {
	fakeDir, capDir := writeFakeRg(t, fakeRgBlockScript)
	reap := filepath.Join(capDir, "reap")
	r := startVrgPTY(t, ptyEnv(fakeDir, capDir, "VRG_TEST_REAP="+reap), "foo")
	pid := awaitReadyPID(t, capDir)
	r.waitFor(t, "Searching…")
	if err := r.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("SIGTERM: %v", err)
	}
	code, out := r.waitExit(t)
	if code != 0 {
		t.Fatalf("exit = %d, want 0 for the ordinary quit path; output: %q", code, out)
	}
	assertDisplayRestored(t, out)
	if got := readFile(t, reap); !strings.Contains(got, "killed") {
		t.Fatalf("reap side channel = %q, want the killed wait status", got)
	}
	pidIsGone(t, pid)
}

// An injected controlled failure after the child has signalled ready:
// the child is terminated and reaped, the terminal is restored, and a
// sanitized diagnostic is written exactly once, after the
// display-restoration sequence. Exit status 2.
func TestPTYControlledFailureExits2(t *testing.T) {
	fakeDir, capDir := writeFakeRg(t, fakeRgBlockScript)
	reap := filepath.Join(capDir, "reap")
	fail := filepath.Join(capDir, "fail-fifo")
	if err := syscall.Mkfifo(fail, 0o600); err != nil {
		t.Fatalf("mkfifo: %v", err)
	}
	r := startVrgPTY(t, ptyEnv(fakeDir, capDir,
		"VRG_TEST_REAP="+reap,
		"VRG_TEST_FAIL_TRIGGER="+fail,
	), "foo")
	pid := awaitReadyPID(t, capDir)
	r.waitFor(t, "Searching…")
	// The fifo handshake: the writer's open pairs with vrg's reader, and
	// its close delivers EOF — the injected failure fires. It runs off
	// the test goroutine so a wedged vrg cannot deadlock the test.
	go func() {
		if w, err := os.OpenFile(fail, os.O_WRONLY, 0); err == nil {
			_, _ = w.WriteString("fail\n")
			_ = w.Close()
		}
	}()
	code, out := r.waitExit(t)
	if code != 2 {
		t.Fatalf("exit = %d, want 2; output: %q", code, out)
	}
	restore := strings.Index(out, "\x1b[?1049l")
	diag := strings.Index(out, "vrg:")
	if restore < 0 {
		t.Fatalf("output lacks the display-restoration sequence: %q", out)
	}
	if diag < 0 || diag < restore {
		t.Fatalf("diagnostic missing or written before terminal restoration: %q", out)
	}
	if n := strings.Count(out, "vrg:"); n != 1 {
		t.Fatalf("controlled-failure diagnostic written %d times, want exactly once: %q", n, out)
	}
	assertDisplayRestored(t, out)
	if got := readFile(t, reap); !strings.Contains(got, "killed") {
		t.Fatalf("reap side channel = %q, want the killed wait status", got)
	}
	pidIsGone(t, pid)
}

// The controlled-failure diagnostic belongs to stderr alone: with piped
// streams the injected failure writes it there exactly once and never on
// stdout.
func TestControlledFailureDiagnosticOnStderr(t *testing.T) {
	fakeDir, capDir := writeFakeRg(t, fakeRgBlockScript)
	reap := filepath.Join(capDir, "reap")
	fail := filepath.Join(capDir, "fail-fifo")
	if err := syscall.Mkfifo(fail, 0o600); err != nil {
		t.Fatalf("mkfifo: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binPath, "foo")
	cmd.Env = ptyEnv(fakeDir, capDir,
		"VRG_TEST_REAP="+reap,
		"VRG_TEST_FAIL_TRIGGER="+fail,
	)
	cmd.Dir = t.TempDir()
	// A held-open pipe for stdin: a pollable input that never delivers
	// EOF, so the program simply runs until the failure fires.
	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer stdinW.Close()
	defer stdinR.Close()
	cmd.Stdin = stdinR
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	pid := awaitReadyPID(t, capDir)
	// The fifo writer pairs with vrg's reader and its close delivers the
	// EOF that fires the injected failure. It runs off the test goroutine
	// so a vrg that died early cannot wedge the test.
	go func() {
		if w, err := os.OpenFile(fail, os.O_WRONLY, 0); err == nil {
			_ = w.Close()
		}
	}()
	werr := cmd.Wait()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("vrg timed out (stdout %q, stderr %q)", stdout.String(), stderr.String())
	}
	ee, ok := werr.(*exec.ExitError)
	if !ok || ee.ExitCode() != 2 {
		t.Fatalf("exit = %v, want 2 (stderr %q)", werr, stderr.String())
	}
	if n := strings.Count(stderr.String(), "vrg:"); n != 1 {
		t.Fatalf("stderr diagnostic written %d times, want exactly once: %q", n, stderr.String())
	}
	if strings.Contains(stdout.String(), "vrg:") {
		t.Fatalf("diagnostic leaked to stdout: %q", stdout.String())
	}
	if got := readFile(t, reap); !strings.Contains(got, "killed") {
		t.Fatalf("reap side channel = %q, want the killed wait status", got)
	}
	pidIsGone(t, pid)
}

// fakeRgExit3Script signals readiness, emits one complete valid result
// stream, then exits 3 — a fatal process outcome over usable results.
const fakeRgExit3Script = `#!/bin/sh
echo $$ > "$VRG_CAPTURE_DIR/ready"
printf '%s\n' \
'{"type":"begin","data":{"path":{"text":"f.txt"}}}' \
'{"type":"match","data":{"path":{"text":"f.txt"},"lines":{"text":"hello\n"},"line_number":1,"submatches":[{"match":{"text":"hello"},"start":0,"end":5}]}}' \
'{"type":"end","data":{"path":{"text":"f.txt"},"binary_offset":null}}' \
'{"type":"summary","data":{}}'
exit 3
`

// A fake rg exiting non-zero after its handshake with usable results in
// the stream: the error overlay opens over browse naming the exit
// status, the first q dismisses to browse, the second quits with the
// fixed fatal status 2.
func TestPTYNonZeroExitBrowseOverlayExits2(t *testing.T) {
	fakeDir, capDir := writeFakeRg(t, fakeRgExit3Script)
	r := startVrgPTY(t, ptyEnv(fakeDir, capDir), "foo")
	pid := awaitReadyPID(t, capDir) // the handshake: the child ran
	r.waitFor(t, "exit status 3")   // the generated diagnostic in the overlay
	before := len(r.output())
	r.send(t, "q")             // dismiss the overlay, revealing browse
	r.waitForGrowth(t, before) // the dismissal re-rendered the frame
	r.send(t, "q")             // quit from browse with the fixed status
	code, out := r.waitExit(t)
	if code != 2 {
		t.Fatalf("exit = %d, want 2; output: %q", code, out)
	}
	if !strings.Contains(out, "f.txt") {
		t.Fatalf("browse view never appeared: %q", out)
	}
	assertDisplayRestored(t, out)
	pidIsGone(t, pid)
}

// fakeRgFloodScript signals readiness, writes over 1 MiB to stderr
// interleaved with a valid stdout stream — a head marker, ~525 padded
// lines, and a tail marker — then exits 0. The stderr text is a warning
// diagnostic: vrg shows it in the overlay.
const fakeRgFloodScript = `#!/bin/sh
echo $$ > "$VRG_CAPTURE_DIR/ready"
chunk=$(printf '%1999s' '' | tr ' ' 'e')
echo "ERRHEAD-MARKER" >&2
printf '%s\n' '{"type":"begin","data":{"path":{"text":"f0.txt"}}}'
i=0
while [ "$i" -lt 175 ]; do printf '%s\n' "$chunk" >&2; i=$((i+1)); done
printf '%s\n' '{"type":"match","data":{"path":{"text":"f0.txt"},"lines":{"text":"hit\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}'
i=0
while [ "$i" -lt 175 ]; do printf '%s\n' "$chunk" >&2; i=$((i+1)); done
printf '%s\n' '{"type":"end","data":{"path":{"text":"f0.txt"},"binary_offset":null}}'
i=0
while [ "$i" -lt 175 ]; do printf '%s\n' "$chunk" >&2; i=$((i+1)); done
printf '%s\n' '{"type":"summary","data":{}}'
echo "ERRTAIL-MARKER" >&2
: > "$VRG_CAPTURE_DIR/writes-done"
exit 0
`

// The stderr-content fixture: over 1 MiB of stderr interleaved with a
// valid stdout stream at an ordinary terminal size. The captured
// stderr opens the warning overlay with its head in view — the tail's
// reachability is proven by the model-level complete-row and clamp
// tests rather than by a simultaneous head/tail frame — and dismissal
// reveals the browse view built from the complete stdout stream. Exit
// status stays 0.
func TestPTYStderrContentFixture(t *testing.T) {
	fakeDir, capDir := writeFakeRg(t, fakeRgFloodScript)
	r := startVrgPTY(t, ptyEnv(fakeDir, capDir), "foo")
	awaitReadyPID(t, capDir)
	r.waitFor(t, "ERRHEAD-MARKER") // the captured stderr in the overlay
	before := len(r.output())
	r.send(t, "q")             // dismiss the warning overlay
	r.waitForGrowth(t, before) // the dismissal re-rendered the frame
	r.send(t, "q")             // quit from browse
	code, out := r.waitExit(t)
	if code != 0 {
		t.Fatalf("exit = %d, want 0 for a warning over clean results; output: %q", code, out)
	}
	if !strings.Contains(out, "f0.txt") {
		t.Fatalf("browse view lacks the streamed file — stdout incomplete: %q", out)
	}
	if _, err := os.Stat(filepath.Join(capDir, "writes-done")); err != nil {
		t.Fatalf("child handshake missing: %v", err)
	}
	assertDisplayRestored(t, out)
}
