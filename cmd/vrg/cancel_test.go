//go:build unix

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/charmbracelet/x/term"
	"github.com/creack/pty"
)

// termSession drives vrg on a pty whose slave termios was captured
// before launch, so a test can prove the PTY input modes were restored
// after exit. The test keeps its own slave fd open so the terminal state
// remains queryable once the child process is gone.
type termSession struct {
	*ptySession
	tty    *os.File
	before *term.State
}

func startVrgTermPTY(t *testing.T, dir string, env []string, args ...string) *termSession {
	t.Helper()
	pt, tty, err := pty.Open()
	if err != nil {
		t.Fatalf("pty open: %v", err)
	}
	t.Cleanup(func() { tty.Close() })
	if err := pty.Setsize(pt, &pty.Winsize{Rows: 24, Cols: 80}); err != nil {
		t.Fatalf("pty size: %v", err)
	}
	before, err := term.GetState(tty.Fd())
	if err != nil {
		t.Fatalf("termios before launch: %v", err)
	}
	cmd := exec.Command(binPath, args...)
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdin, cmd.Stdout, cmd.Stderr = tty, tty, tty
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	s := &ptySession{t: t, cmd: cmd, pt: pt, done: make(chan int, 1)}
	s.watch()
	return &termSession{ptySession: s, tty: tty, before: before}
}

// assertTermiosRestored compares the pty's termios after vrg exits with
// the state captured before launch.
func (s *termSession) assertTermiosRestored() {
	s.t.Helper()
	after, err := term.GetState(s.tty.Fd())
	if err != nil {
		s.t.Fatalf("termios after exit: %v", err)
	}
	if !reflect.DeepEqual(*s.before, *after) {
		s.t.Fatalf("pty input modes not restored\nbefore: %+v\nafter:  %+v", *s.before, *after)
	}
}

// The display-restoration sequences every controlled exit must emit:
// leave the alternate screen and make the cursor visible.
const (
	exitAltScreen = "\x1b[?1049l"
	showCursor    = "\x1b[?25h"
)

func assertDisplayRestored(t *testing.T, out string) {
	t.Helper()
	for _, seq := range []string{exitAltScreen, showCursor} {
		if !strings.Contains(out, seq) {
			t.Fatalf("display restoration missing %q; output: %q", seq, out)
		}
	}
}

// assertChildGone proves the pid the fake rg recorded no longer exists.
func assertChildGone(t *testing.T, pidFile string) {
	t.Helper()
	b, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("fake rg pid record: %v", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		t.Fatalf("fake rg pid record %q: %v", b, err)
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	if err := p.Signal(syscall.Signal(0)); err == nil {
		t.Fatalf("child pid %d still exists after vrg exited", pid)
	}
}

// reapEvidence returns the side-channel line vrg writes after its own
// wait/reap path ran — proof of reaping, not an inferred missing pid.
func reapEvidence(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reap evidence missing; vrg's wait/reap path did not report: %v", err)
	}
	return string(b)
}

// blockedRG is the controllable fake rg: it signals readiness, records
// its pid, then replaces itself with sleep so the test controls whether
// it ever finishes — termination must come from vrg.
const blockedRG = `
: > "$VRG_TEST_RG_READY"
echo $$ > "$VRG_TEST_RG_PID"
exec sleep 600
`

// q and ctrl+c against a blocked fake rg are cancellation: exit 130 with
// no further screen, the child terminated and reaped, the display
// restoration sequence emitted, and the pty termios back to its
// pre-launch state.
func TestCancelWhileSearchingKillsChild(t *testing.T) {
	for _, tc := range []struct{ name, key string }{
		{"q", "q"},
		{"ctrl+c", "\x03"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			ready := filepath.Join(dir, "ready")
			pidFile := filepath.Join(dir, "pid")
			reapFile := filepath.Join(dir, "reap")
			fakebin := fakeRG(t, blockedRG)
			env := testEnv(fakebin,
				"VRG_TEST_RG_READY="+ready,
				"VRG_TEST_RG_PID="+pidFile,
				"VRG_TEST_REAP="+reapFile)

			s := startVrgTermPTY(t, dir, env, "foo")
			waitForFile(t, ready)
			if !s.waitFor("Searching") {
				s.cmd.Process.Kill()
				<-s.done
				t.Fatalf("searching screen never appeared; output: %q", s.output())
			}
			s.send(tc.key)
			code := s.waitExit()
			out := s.output()
			if code != 130 {
				t.Fatalf("exit = %d, want 130; output: %q", code, out)
			}
			if strings.Contains(out, "matched lines") {
				t.Fatalf("cancellation rendered a further screen: %q", out)
			}
			assertDisplayRestored(t, out)
			s.assertTermiosRestored()
			assertChildGone(t, pidFile)
			if ev := reapEvidence(t, reapFile); !strings.Contains(ev, "code=-1") {
				t.Fatalf("reap evidence = %q, want the killed child's wait status", ev)
			}
		})
	}
}

// q while index preparation is gate-held — the fake rg already exited
// and its stream is fully collected — is cancellation (130), not the
// browse quit; the released completion must not appear.
func TestQDuringGateHeldPreparationCancels(t *testing.T) {
	dir := t.TempDir()
	gate := filepath.Join(dir, "gate")
	ack := filepath.Join(dir, "ack")
	reapFile := filepath.Join(dir, "reap")
	if err := os.WriteFile(gate, []byte("held"), 0o644); err != nil {
		t.Fatal(err)
	}
	fakebin := fakeRG(t, happyStreamRG)
	env := testEnv(fakebin,
		"VRG_TEST_GATE="+gate,
		"VRG_TEST_COLLECT_ACK="+ack,
		"VRG_TEST_REAP="+reapFile)

	s := startVrgTermPTY(t, dir, env, "foo")
	waitForFile(t, ack) // rg exited and its stream is fully collected
	if !s.waitFor("Searching") {
		s.cmd.Process.Kill()
		<-s.done
		t.Fatalf("searching screen never appeared; output: %q", s.output())
	}
	s.send("q")
	code := s.waitExit()
	out := s.output()
	if code != 130 {
		t.Fatalf("exit = %d, want 130 (cancellation, not browse quit); output: %q", code, out)
	}
	if strings.Contains(out, "matched lines") {
		t.Fatalf("late completion revived the summary after cancellation: %q", out)
	}
	assertDisplayRestored(t, out)
	s.assertTermiosRestored()
	if ev := reapEvidence(t, reapFile); !strings.Contains(ev, "code=0") {
		t.Fatalf("reap evidence = %q, want the exited child's wait status", ev)
	}
}

// An ordinary exit reaps the child through the same cleanup: the summary
// quit leaves vrg's wait/reap evidence behind and no live rg process.
func TestOrdinaryQuitLeavesNoChild(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "pid")
	reapFile := filepath.Join(dir, "reap")
	fakebin := fakeRG(t, `echo $$ > "$VRG_TEST_RG_PID"`+happyStreamRG)
	env := testEnv(fakebin,
		"VRG_TEST_RG_PID="+pidFile,
		"VRG_TEST_REAP="+reapFile)

	s := startVrgTermPTY(t, dir, env, "foo")
	if !s.waitFor("matched lines") {
		s.cmd.Process.Kill()
		<-s.done
		t.Fatalf("summary never appeared; output: %q", s.output())
	}
	s.send("q")
	code := s.waitExit()
	out := s.output()
	if code != 0 {
		t.Fatalf("exit = %d, want 0; output: %q", code, out)
	}
	assertDisplayRestored(t, out)
	s.assertTermiosRestored()
	assertChildGone(t, pidFile)
	if ev := reapEvidence(t, reapFile); !strings.Contains(ev, "code=0") {
		t.Fatalf("reap evidence = %q, want the child's wait status", ev)
	}
}

// An injected controlled failure after the child signalled ready
// terminates and reaps the child, restores the display and the pty input
// modes, and writes the sanitized diagnostic exactly once — after the
// restoration sequence — exiting 2.
func TestControlledFailureCleanupExit2(t *testing.T) {
	dir := t.TempDir()
	ready := filepath.Join(dir, "ready")
	pidFile := filepath.Join(dir, "pid")
	reapFile := filepath.Join(dir, "reap")
	trigger := filepath.Join(dir, "fail")
	fakebin := fakeRG(t, blockedRG)
	env := testEnv(fakebin,
		"VRG_TEST_RG_READY="+ready,
		"VRG_TEST_RG_PID="+pidFile,
		"VRG_TEST_REAP="+reapFile,
		"VRG_TEST_FAIL="+trigger)

	s := startVrgTermPTY(t, dir, env, "foo")
	waitForFile(t, ready)
	if !s.waitFor("Searching") {
		s.cmd.Process.Kill()
		<-s.done
		t.Fatalf("searching screen never appeared; output: %q", s.output())
	}
	if err := os.WriteFile(trigger, []byte("go"), 0o644); err != nil {
		t.Fatal(err)
	}
	code := s.waitExit()
	out := s.output()
	if code != 2 {
		t.Fatalf("exit = %d, want 2; output: %q", code, out)
	}
	const diag = "vrg: injected test failure"
	if n := strings.Count(out, diag); n != 1 {
		t.Fatalf("controlled-failure diagnostic appears %d times, want exactly once: %q", n, out)
	}
	restored := strings.Index(out, exitAltScreen)
	if restored < 0 || strings.Index(out, diag) < restored {
		t.Fatalf("diagnostic must follow the display-restoration sequence: %q", out)
	}
	if !strings.Contains(out, "^[[7m") {
		t.Fatalf("diagnostic was not sanitized, want the escaped form ^[[7m: %q", out)
	}
	s.assertTermiosRestored()
	assertChildGone(t, pidFile)
	reapEvidence(t, reapFile)
}
