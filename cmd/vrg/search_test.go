package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/creack/pty"
)

// ptySession drives the real vrg binary on a pseudo-terminal so the TUI
// runs exactly as it does for a user.
type ptySession struct {
	t    *testing.T
	cmd  *exec.Cmd
	pt   *os.File
	done chan int // receives the exit code exactly once

	mu  sync.Mutex
	out bytes.Buffer
}

// testEnv builds the child environment: the fake rg directory first on
// PATH (so it wins), a known TERM, and caller-supplied overrides.
func testEnv(fakebin string, extra ...string) []string {
	env := []string{
		"PATH=" + fakebin + string(os.PathListSeparator) + os.Getenv("PATH"),
		"TERM=xterm",
	}
	return append(env, extra...)
}

// fakeRG writes script as an executable "rg" in a fresh directory and
// returns the directory for testEnv.
func fakeRG(t *testing.T, script string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "rg"), []byte("#!/bin/sh\n"+script), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func startVrgPTY(t *testing.T, dir string, env []string, args ...string) *ptySession {
	t.Helper()
	cmd := exec.Command(binPath, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = env
	pt, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 24, Cols: 80})
	if err != nil {
		t.Fatalf("pty start: %v", err)
	}
	s := &ptySession{t: t, cmd: cmd, pt: pt, done: make(chan int, 1)}
	s.watch()
	return s
}

// watch starts the exit-code and output-drain goroutines for a session
// whose command has already been started on its pty.
func (s *ptySession) watch() {
	go func() {
		err := s.cmd.Wait()
		if err == nil {
			s.done <- 0
			return
		}
		if ee, ok := err.(*exec.ExitError); ok {
			s.done <- ee.ExitCode()
			return
		}
		s.done <- -1
	}()
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := s.pt.Read(buf)
			if n > 0 {
				s.mu.Lock()
				s.out.Write(buf[:n])
				s.mu.Unlock()
			}
			if err != nil {
				return
			}
		}
	}()
}

func (s *ptySession) output() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.out.String()
}

// waitFor polls until the needle appears in pty output or the bound
// expires; it checks an explicit condition each iteration rather than
// settling on a fixed delay.
func (s *ptySession) waitFor(needle string) bool {
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(s.output(), needle) {
			return true
		}
		select {
		case code := <-s.done:
			s.done <- code // leave it for waitExit
			return false
		default:
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

func (s *ptySession) send(keys string) {
	s.t.Helper()
	if _, err := s.pt.WriteString(keys); err != nil {
		s.t.Fatalf("pty write: %v", err)
	}
}

func (s *ptySession) waitExit() int {
	s.t.Helper()
	select {
	case code := <-s.done:
		s.pt.Close()
		return code
	case <-time.After(20 * time.Second):
		s.cmd.Process.Kill()
		s.t.Fatalf("vrg did not exit; captured output: %q", s.output())
		return -1
	}
}

// runVrgWithQuit runs vrg on a pty, waits for the browse view — marked
// by the filename rule, present while loading and loaded alike — sends
// q, and returns the captured output and exit code.
func runVrgWithQuit(t *testing.T, dir string, env []string, args ...string) (string, int) {
	t.Helper()
	s := startVrgPTY(t, dir, env, args...)
	if !s.waitFor("─ ") {
		s.cmd.Process.Kill()
		<-s.done
		t.Fatalf("browse view never appeared; output: %q", s.output())
	}
	s.send("q")
	return s.output(), s.waitExit()
}

// waitForFile polls for a path to appear, on an explicit condition.
func waitForFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("file never appeared: %s", path)
}

// happyStreamRG is the fake rg used for successful searches: two files,
// three matched lines, full ripgrep 15.x record shapes.
const happyStreamRG = `
cat <<'EOF'
{"type":"begin","data":{"path":{"text":"./a.go"}}}
{"type":"match","data":{"path":{"text":"./a.go"},"lines":{"text":"alpha\n"},"line_number":2,"absolute_offset":0,"submatches":[{"match":{"text":"alpha"},"start":0,"end":5}]}}
{"type":"match","data":{"path":{"text":"./a.go"},"lines":{"text":"alpha again\n"},"line_number":5,"absolute_offset":20,"submatches":[{"match":{"text":"alpha"},"start":0,"end":5}]}}
{"type":"end","data":{"path":{"text":"./a.go"},"binary_offset":null,"stats":{"elapsed":{"secs":0,"nanos":1,"human":"0s"},"searches":1,"searches_with_match":1,"bytes_searched":12,"bytes_printed":0,"matched_lines":1,"matches":1}}}
{"type":"begin","data":{"path":{"text":"./b.go"}}}
{"type":"match","data":{"path":{"text":"./b.go"},"lines":{"text":"beta alpha\n"},"line_number":1,"absolute_offset":0,"submatches":[{"match":{"text":"alpha"},"start":5,"end":10}]}}
{"type":"end","data":{"path":{"text":"./b.go"},"binary_offset":null,"stats":{"elapsed":{"secs":0,"nanos":1,"human":"0s"},"searches":1,"searches_with_match":1,"bytes_searched":12,"bytes_printed":0,"matched_lines":1,"matches":1}}}
{"type":"summary","data":{"elapsed_total":{"human":"0.005s","nanos":5000000,"secs":0},"stats":{"bytes_printed":482,"bytes_searched":34,"elapsed":{"human":"0s","nanos":1,"secs":0},"matched_lines":3,"matches":3,"searches":2,"searches_with_match":2}}}
EOF
`

// The child argv at the process boundary: vrg runs "rg" with the exact
// protected vector from the invocation working directory.
func TestChildArgvAndWorkdir(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		wantArgv string
	}{
		{"no flags", []string{"foo"}, "--json\n--no-config\n--\nfoo\n.\n"},
		{"flags and root", []string{"-iw", "foo", "sub"}, "--json\n--no-config\n-i\n-w\n--\nfoo\nsub\n"},
		{"protected pattern", []string{"--", "-pat"}, "--json\n--no-config\n--\n-pat\n.\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if tc.name == "flags and root" {
				if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			argvFile := filepath.Join(dir, "argv")
			cwdFile := filepath.Join(dir, "cwd")
			fakebin := fakeRG(t, `
[ -n "$VRG_TEST_ARGV" ] && { : > "$VRG_TEST_ARGV"; for a in "$@"; do printf '%s\n' "$a" >> "$VRG_TEST_ARGV"; done; }
[ -n "$VRG_TEST_CWD" ] && pwd -P > "$VRG_TEST_CWD"
`+happyStreamRG)
			env := testEnv(fakebin, "VRG_TEST_ARGV="+argvFile, "VRG_TEST_CWD="+cwdFile)

			out, code := runVrgWithQuit(t, dir, env, tc.args...)
			if code != 0 {
				t.Fatalf("exit = %d, want 0; output: %q", code, out)
			}
			argv, err := os.ReadFile(argvFile)
			if err != nil {
				t.Fatalf("fake rg argv record: %v", err)
			}
			if string(argv) != tc.wantArgv {
				t.Fatalf("child argv = %q, want %q", argv, tc.wantArgv)
			}
			cwd, err := os.ReadFile(cwdFile)
			if err != nil {
				t.Fatalf("fake rg cwd record: %v", err)
			}
			wantDir, _ := filepath.EvalSymlinks(dir)
			if strings.TrimSpace(string(cwd)) != wantDir {
				t.Fatalf("child cwd = %q, want %q", cwd, wantDir)
			}
		})
	}
}

// rg absent from PATH: a sanitized stderr diagnostic and exit 2, without
// entering the TUI.
func TestStartFailureExit2(t *testing.T) {
	empty := t.TempDir()
	res := runVrgFull(t, "", []string{"PATH=" + empty}, "foo")
	if res.code != 2 {
		t.Fatalf("exit = %d, want 2 (stdout %q)", res.code, res.stdout)
	}
	if res.stdout != "" {
		t.Fatalf("stdout not empty on start failure: %q", res.stdout)
	}
	diag := strings.TrimSpace(res.stderr)
	if !strings.HasPrefix(diag, "vrg:") || !strings.Contains(diag, "rg") {
		t.Fatalf("diagnostic = %q, want a vrg diagnostic naming the start failure", diag)
	}
	if strings.ContainsAny(diag, "\x1b\x9b") {
		t.Fatalf("diagnostic contains raw control bytes: %q", diag)
	}
	if strings.Contains(diag, "\n") {
		t.Fatalf("diagnostic spans multiple lines: %q", diag)
	}
}

// A fake rg writing well over pipe capacity to stderr — interleaved with
// a valid stdout stream — must neither deadlock vrg nor lose records:
// all 18 recorded matches surface as inverse-video spans once the file
// loads. The handshake file is written only after both writes complete.
func TestDualPipeBackpressure(t *testing.T) {
	dir := t.TempDir()
	var content strings.Builder
	for i := 0; i < 18; i++ {
		content.WriteString("x f\n")
	}
	if err := os.WriteFile(filepath.Join(dir, "f"), []byte(content.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	handshake := filepath.Join(dir, "done")
	fakebin := fakeRG(t, `
printf '%s\n' '{"type":"begin","data":{"path":{"text":"./f"}}}'
i=0
while [ $i -lt 18 ]; do
    dd if=/dev/zero bs=65536 count=1 2>/dev/null | tr '\000' 'e' >&2
    printf '%s\n' '{"type":"match","data":{"path":{"text":"./f"},"lines":{"text":"x f\n"},"line_number":'$((i + 1))',"absolute_offset":0,"submatches":[{"match":{"text":"f"},"start":2,"end":3}]}}'
    i=$((i + 1))
done
printf '%s\n' '{"type":"end","data":{"path":{"text":"./f"},"binary_offset":null,"stats":{}}}'
printf '%s\n' '{"type":"summary","data":{"stats":{}}}'
[ -n "$VRG_TEST_HANDSHAKE" ] && : > "$VRG_TEST_HANDSHAKE"
`)
	env := testEnv(fakebin, "VRG_TEST_HANDSHAKE="+handshake)

	s := startVrgPTY(t, dir, env, "foo")
	if !s.waitFor("18  x ") {
		s.cmd.Process.Kill()
		<-s.done
		t.Fatalf("loaded content never appeared; output: %q", s.output())
	}
	s.send("q")
	out, code := s.output(), s.waitExit()
	if code != 0 {
		t.Fatalf("exit = %d, want 0; output: %q", code, out)
	}
	// The final content frame begins at the last line-1 gutter; every
	// recorded match survives as an inverse-video span there. The
	// renderer normalizes the reset sequence, so count the inverse
	// colour pair before each matched f rather than a full styled run —
	// the current matched line's span additionally carries ;4.
	last := strings.LastIndex(out, " 1  x ")
	if last < 0 {
		t.Fatalf("content frame missing: %q", out)
	}
	if n := strings.Count(out[last:], "30;47"); n != 18 {
		t.Fatalf("inverse-video matches = %d, want 18 — records lost", n)
	}
	if _, err := os.Stat(handshake); err != nil {
		t.Fatalf("handshake missing: child could not finish writing both pipes: %v", err)
	}
}

// Diagnostics on stderr while rg still exits 0 with a well-formed stream
// are captured without blocking the child.
func TestStderrCapturedWithoutBlocking(t *testing.T) {
	dir := t.TempDir()
	fakebin := fakeRG(t, `
printf '%s\n' 'rg: warning: a made-up diagnostic' >&2
printf '%s\n' 'rg: another warning line' >&2
`+happyStreamRG)
	out, code := runVrgWithQuit(t, dir, testEnv(fakebin), "foo")
	if code != 0 {
		t.Fatalf("exit = %d, want 0; output: %q", code, out)
	}
	if !strings.Contains(out, "a.go") || !strings.Contains(out, "b.go") {
		t.Fatalf("browse file list missing from output: %q", out)
	}
}

// While VRG_TEST_GATE holds index preparation after the child has exited
// (collection acknowledged via VRG_TEST_COLLECT_ACK), the app stays on
// the searching screen until the gate releases.
func TestGateHeldPreparationKeepsSearching(t *testing.T) {
	dir := t.TempDir()
	gate := filepath.Join(dir, "gate")
	ack := filepath.Join(dir, "ack")
	if err := os.WriteFile(gate, []byte("held"), 0o644); err != nil {
		t.Fatal(err)
	}
	fakebin := fakeRG(t, happyStreamRG)
	env := testEnv(fakebin, "VRG_TEST_GATE="+gate, "VRG_TEST_COLLECT_ACK="+ack)

	s := startVrgPTY(t, dir, env, "foo")
	waitForFile(t, ack) // rg has exited and its stream is fully collected
	// While the gate holds, the searching screen is the only possible
	// state: the browse view cannot render until the file is removed.
	if !s.waitFor("Searching") {
		s.cmd.Process.Kill()
		<-s.done
		t.Fatalf("searching screen never appeared; output: %q", s.output())
	}
	if out := s.output(); strings.Contains(out, "─") {
		s.cmd.Process.Kill()
		<-s.done
		t.Fatalf("gate-held view wrong: %q", out)
	}
	if err := os.Remove(gate); err != nil {
		t.Fatal(err)
	}
	if !s.waitFor("─ ") {
		s.cmd.Process.Kill()
		<-s.done
		t.Fatalf("browse view never appeared after gate release; output: %q", s.output())
	}
	s.send("q")
	if code := s.waitExit(); code != 0 {
		t.Fatalf("exit = %d, want 0; output: %q", code, s.output())
	}
}
