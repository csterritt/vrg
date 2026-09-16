package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/creack/pty"
)

// Issue #48 handshake matrix — every PTY helper action, the
// application-side postcondition it requires, the acknowledgement
// that proves it, and the action the acknowledgement unlocks.
// UPDATE_ACK rows are Update events on the VRG_TEST_UPDATE_ACK seam
// (one record per processed message, per-process monotonic seq).
// COLLECT_ACK rows are the WithOnCollect file (Issues #11/#45).
// FIXTURE rows are child-side files the fake rg writes (bounded
// waitForFile poll — fixture readiness, never a model-transition
// assumption). OUTPUT is a bounded poll of the rendered PTY stream
// for an explicit marker. Every row blocks on its acknowledgement
// with a bounded timeout; none may wait on elapsed time.
//
// | helper / test group           | action                  | postcondition                                   | acknowledgement                     | unlocks                    |
// |-------------------------------|-------------------------|-------------------------------------------------|-------------------------------------|----------------------------|
// | runVrgWithKeys                | start under PTY         | SearchCompleteMsg processed (state+overlay set) | UPDATE_ACK msg=search-complete      | first key send             |
// | runVrgWithKeys                | send key K              | KeyPressMsg(K) processed by Update              | UPDATE_ACK msg=key key=K            | next key send              |
// | runVrgWithKeys (esc before q) | send Esc                | open overlay dismissed                          | UPDATE_ACK key=esc dismissed=true   | q send                     |
// | runVrgKillChild               | start under PTY         | fake rg running and blocked                     | FIXTURE readyFile                   | SIGKILL child group        |
// | runVrgKillChild               | SIGKILL child group     | child death processed into outcome              | UPDATE_ACK msg=search-complete      | key sends                  |
// | runVrgKillChild               | send key K              | KeyPressMsg(K) processed                        | UPDATE_ACK msg=key key=K            | next key send              |
// | painted-post-state assertions | transient state entered | view/overlay frame actually rendered            | OUTPUT substring (bounded poll)   | dismissal/quit keys        |
// | runVrgWithQuit                | start under PTY         | SearchCompleteMsg processed                     | UPDATE_ACK msg=search-complete      | first q                    |
// | runVrgWithQuit                | send q                  | key processed; dismissed iff overlay was open   | UPDATE_ACK msg=key key=q            | second q iff dismissed     |
// | runVrgReplay triggers         | emit diagnostic         | diagnostic collected into session collection    | COLLECT_ACK line count              | trigger/exit key           |
// | runVrgReplay triggers         | send Esc/q/ctrl+c       | KeyPressMsg processed; esc dismissal acked      | UPDATE_ACK msg=key key=K            | next key                   |
// | runVrgCancel (blocked rg)     | start under PTY         | fake rg running and blocked                     | FIXTURE readyFile                   | cancel key (still searching)|
// | runVrgCancel (gate held)      | rg exits, gate held     | model still searching (gate never releases)     | FIXTURE handshakeFile               | q (cancels)                |
// | runVrgCancel (reaps child)    | rg exits                | SearchCompleteMsg processed → browse            | UPDATE_ACK msg=search-complete      | q (quits)                  |
// | runshape replay triggers      | emit second diagnostic  | both diagnostics collected in order             | COLLECT_ACK line counts 1 then 2    | esc dismissal, then q      |
// | TestInjectedControlledFailure | start under PTY         | fake rg running and blocked                     | FIXTURE readyFile                   | fail-trigger file write    |
// | untagged hook-manifest probe  | production binary run   | browse view rendered (fixture file visible)     | OUTPUT substring                    | q                          |
//
// Correlation is per process (each run writes its own acknowledgement
// file) and per occurrence (records carry a monotonic seq; the ackLog
// cursor consumes them in order so an earlier same-kind event can
// never satisfy a later wait).

// ackTimeout bounds every acknowledgement wait: a handshake that
// never arrives fails the test with a descriptive message instead of
// hanging.
const ackTimeout = 20 * time.Second

// ackEvent is one parsed acknowledgement record from the
// VRG_TEST_UPDATE_ACK log written by the vrg_testhooks seam: one line
// per Update-processed message, carrying the per-process sequence
// number, the message kind, the key label for key events, and the
// model state/overlay/dismissal post-state.
type ackEvent struct {
	seq       int
	msg       string
	key       string
	state     string
	overlay   string
	dismissed bool
}

// readAckEvents parses every complete acknowledgement line in file.
// A partially written final line (a read racing an in-flight append)
// is skipped and picked up by the next poll.
func readAckEvents(file string) []ackEvent {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	var events []ackEvent
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" {
			continue
		}
		ev, ok := parseAckLine(line)
		if ok {
			events = append(events, ev)
		}
	}
	return events
}

// parseAckLine parses one "<seq> msg=<kind> key=<label> state=<name>
// overlay=<name> dismissed=<bool>" record.
func parseAckLine(line string) (ackEvent, bool) {
	fields := strings.Fields(line)
	if len(fields) != 6 {
		return ackEvent{}, false
	}
	seq, err := strconv.Atoi(fields[0])
	if err != nil || seq < 1 {
		return ackEvent{}, false
	}
	kv := map[string]string{}
	for _, f := range fields[1:] {
		k, v, found := strings.Cut(f, "=")
		if !found {
			return ackEvent{}, false
		}
		kv[k] = v
	}
	return ackEvent{
		seq:       seq,
		msg:       kv["msg"],
		key:       kv["key"],
		state:     kv["state"],
		overlay:   kv["overlay"],
		dismissed: kv["dismissed"] == "true",
	}, true
}

// ackLog correlates waits with per-occurrence acknowledgement
// records. last is the sequence of the most recently consumed event:
// every wait only considers events after it, so an earlier same-kind
// event can never satisfy a later wait (Issue #48).
type ackLog struct {
	file     string
	procDone <-chan struct{} // closed when the vrg process exits; nil allowed
	last     int
}

// scan returns the first recorded event after the cursor that
// satisfies match, without consuming it.
func (l *ackLog) scan(match func(ackEvent) bool) (ackEvent, bool) {
	for _, ev := range readAckEvents(l.file) {
		if ev.seq > l.last && match(ev) {
			return ev, true
		}
	}
	return ackEvent{}, false
}

// waitEvent polls the acknowledgement log until an event after the
// cursor satisfies match, the vrg process exits, or timeout elapses.
// The 10 ms sleep paces re-checks of the explicit condition; it is a
// bounded condition poll, not a settling delay. A process exit aborts
// the wait after one final scan — acknowledgement records are
// appended inside Update, so every genuine event precedes exit.
func (l *ackLog) waitEvent(desc string, match func(ackEvent) bool, timeout time.Duration) (ackEvent, error) {
	deadline := time.Now().Add(timeout)
	for {
		if ev, ok := l.scan(match); ok {
			l.last = ev.seq
			return ev, nil
		}
		if l.procDone != nil {
			select {
			case <-l.procDone:
				if ev, ok := l.scan(match); ok {
					l.last = ev.seq
					return ev, nil
				}
				return ackEvent{}, fmt.Errorf("vrg exited while waiting for %s (ack log %s, last consumed seq %d)", desc, l.file, l.last)
			default:
			}
		}
		if time.Now().After(deadline) {
			return ackEvent{}, fmt.Errorf("timed out after %s waiting for %s (ack log %s, last consumed seq %d)", timeout, desc, l.file, l.last)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// mustWait is waitEvent with the standard bounded timeout; it fails
// the test when the acknowledgement never arrives.
func (l *ackLog) mustWait(t *testing.T, desc string, match func(ackEvent) bool) ackEvent {
	t.Helper()
	ev, err := l.waitEvent(desc, match, ackTimeout)
	if err != nil {
		t.Fatal(err)
	}
	return ev
}

// ptyResult is the captured result of one vrg run under a PTY.
type ptyResult struct {
	raw      string // raw PTY master output, escape sequences intact
	stdout   string // raw output with ANSI sequences stripped
	stderr   string
	exitCode int
}

// syncBuffer is the PTY output accumulator shared between the reader
// goroutine and the driver's bounded output waits.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// ptyDriver is the handle a drive/trigger callback uses to act on the
// running vrg: acknowledged key sends and bounded waits on the
// VRG_TEST_UPDATE_ACK event log and on rendered PTY output. ack is
// nil when the run wires no acknowledgement file (the untagged
// production probe), in which case synchronization rides on
// externally observable output instead of the test seam.
type ptyDriver struct {
	ptmx     *os.File
	out      *syncBuffer
	ack      *ackLog
	procDone <-chan struct{} // closed when the vrg process exits
	readDone <-chan struct{} // closed when the PTY reader drains
}

// waitMsg blocks until the next Update-processed message of the
// given kind is acknowledged.
func (d *ptyDriver) waitMsg(t *testing.T, msg string) ackEvent {
	t.Helper()
	if d.ack == nil {
		t.Fatalf("waitMsg(%q): run has no acknowledgement log", msg)
	}
	return d.ack.mustWait(t, "msg="+msg+" processed",
		func(ev ackEvent) bool { return ev.msg == msg })
}

// waitAck blocks until an acknowledgement event satisfies match.
func (d *ptyDriver) waitAck(t *testing.T, desc string, match func(ackEvent) bool) ackEvent {
	t.Helper()
	if d.ack == nil {
		t.Fatalf("waitAck(%q): run has no acknowledgement log", desc)
	}
	return d.ack.mustWait(t, desc, match)
}

// sendKey writes one key to the PTY and — when the run wires an
// acknowledgement log — blocks until the key-press event for that
// key is consumed, proving Update processed it before the next
// action. The returned event carries the post-state fields
// (dismissed, state, overlay) the caller's postcondition needs.
// Waiting for each key's own event before sending the next also
// prevents the input reader from coalescing consecutive sends (the
// Esc+q → alt+q hazard noted in TestStderrContentFixture).
func (d *ptyDriver) sendKey(t *testing.T, key string) ackEvent {
	t.Helper()
	label := sentKeyLabel(t, key)
	if _, err := io.WriteString(d.ptmx, key); err != nil {
		t.Fatalf("cannot send key %q to PTY: %v", key, err)
	}
	if d.ack == nil {
		return ackEvent{}
	}
	return d.waitAck(t, fmt.Sprintf("key %q processed", label),
		func(ev ackEvent) bool { return ev.msg == "key" && ev.key == label })
}

// sentKeyLabel maps a raw PTY byte string to the keystroke label the
// model reports for it (tea.KeyPressMsg.String(): the textual key
// for printable keys, the keystroke name for special keys).
func sentKeyLabel(t *testing.T, key string) string {
	t.Helper()
	switch key {
	case "\x1b":
		return "esc"
	case "\x03":
		return "ctrl+c"
	case "\x1b[A":
		return "up"
	case "\x1b[B":
		return "down"
	case "\x1b[C":
		return "right"
	case "\x1b[D":
		return "left"
	case "\t":
		return "tab"
	case "\r":
		return "enter"
	}
	if len(key) == 1 && key[0] >= 0x20 && key[0] < 0x7f {
		return key
	}
	t.Fatalf("sentKeyLabel: no acknowledgement label for key %q", key)
	return ""
}

// waitForOutput polls the stripped PTY output for want until it
// appears, the process exits and output has fully drained, or the
// bounded timeout elapses. It is a bounded condition poll on an
// explicit, externally observable marker — used where no test seam
// exists (the untagged production probe).
func (d *ptyDriver) waitForOutput(t *testing.T, want string) {
	t.Helper()
	deadline := time.Now().Add(ackTimeout)
	for {
		if strings.Contains(stripAnsi(d.out.String()), want) {
			return
		}
		if d.procDone != nil {
			select {
			case <-d.procDone:
				// PTY bytes may still be draining; wait for the
				// reader (bounded) so the final check is decisive.
				select {
				case <-d.readDone:
				case <-time.After(5 * time.Second):
				}
				if strings.Contains(stripAnsi(d.out.String()), want) {
					return
				}
				t.Fatalf("vrg exited and output drained; %q never appeared: %q", want, stripAnsi(d.out.String()))
			default:
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out after %s waiting for output %q", ackTimeout, want)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// runVrgPTY is the shared PTY scaffold for every vrg-under-PTY test
// helper (Issue #48): it starts cmd under a PTY, captures termios,
// runs the drive callback — which performs waits, trigger writes,
// and acknowledged key sends — waits for exit on a bounded timeout,
// verifies termios restoration, and returns the captured output.
// Every synchronization inside drive goes through the driver's
// acknowledgement waits or the named bounded condition polls; no
// fixed settling or inter-key delay exists anywhere in the harness.
func runVrgPTY(t *testing.T, cmd *exec.Cmd, ackFile string, drive func(d *ptyDriver)) ptyResult {
	t.Helper()
	var se bytes.Buffer
	cmd.Stderr = &se

	ptmx, ptmxErr := pty.Start(cmd)
	if ptmxErr != nil {
		t.Fatalf("failed to start vrg with PTY: %v", ptmxErr)
	}
	defer func() { _ = ptmx.Close() }()

	if err := pty.Setsize(ptmx, &pty.Winsize{Rows: 24, Cols: 80}); err != nil {
		t.Fatalf("failed to set PTY window size: %v", err)
	}

	beforeTermios := getTermios(t, int(ptmx.Fd()))

	out := &syncBuffer{}
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		io.Copy(out, ptmx)
	}()

	procDone := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
		close(procDone)
	}()

	d := &ptyDriver{ptmx: ptmx, out: out, procDone: procDone, readDone: readDone}
	if ackFile != "" {
		d.ack = &ackLog{file: ackFile, procDone: procDone}
	}
	driveDone := make(chan struct{})
	go func() {
		defer close(driveDone)
		drive(d)
	}()

	select {
	case err := <-done:
		<-driveDone
		afterTermios := getTermios(t, int(ptmx.Fd()))
		_ = ptmx.Close()
		<-readDone
		if !termiosEqual(beforeTermios, afterTermios) {
			t.Errorf("termios not restored after exit\nbefore: %+v\nafter:  %+v", beforeTermios, afterTermios)
		}
		res := ptyResult{raw: out.String(), stdout: stripAnsi(out.String()), stderr: se.String()}
		if err == nil {
			return res
		}
		if ee, ok := err.(*exec.ExitError); ok {
			res.exitCode = ee.ExitCode()
			return res
		}
		t.Fatalf("vrg failed: %v (stderr %q)", err, se.String())
	case <-time.After(30 * time.Second):
		cmd.Process.Kill()
		t.Fatal("vrg did not complete within 30 seconds")
	}
	return ptyResult{}
}

// --- Issue #48 handshake-contract tests ---

// TestUpdateAckSeamContract proves the tagged binary exposes the
// application-side acknowledgement seam (matrix rows "search
// completion" and "key processed"): with VRG_TEST_UPDATE_ACK set, a
// completed fake-rg search followed by q produces a search-complete
// event and a causally later key=q event with a strictly increasing
// per-process sequence number.
func TestUpdateAckSeamContract(t *testing.T) {
	fakeDir := t.TempDir()
	writeFakeRG(t, fakeDir, "")

	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "test.txt"), []byte("hello world\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ackFile := filepath.Join(t.TempDir(), "update-ack")
	cmd := exec.Command(binPath, "hello", ".")
	cmd.Dir = repo
	cmd.Env = []string{
		"PATH=" + fakeDir + ":" + os.Getenv("PATH"),
		"VRG_TEST_UPDATE_ACK=" + ackFile,
	}

	var scEv, keyEv ackEvent
	res := runVrgPTY(t, cmd, ackFile, func(d *ptyDriver) {
		scEv = d.waitMsg(t, "search-complete")
		keyEv = d.sendKey(t, "q")
	})

	if res.exitCode != 0 {
		t.Fatalf("vrg exited %d, want 0 (stderr %q)", res.exitCode, res.stderr)
	}
	if scEv.state != "browse" {
		t.Fatalf("search-complete event state = %q, want browse: %+v", scEv.state, scEv)
	}
	if keyEv.msg != "key" || keyEv.key != "q" {
		t.Fatalf("key acknowledgement = %+v, want msg=key key=q", keyEv)
	}
	if keyEv.seq <= scEv.seq {
		t.Fatalf("acknowledgements are not per-occurrence correlated: key event seq %d not after search-complete seq %d", keyEv.seq, scEv.seq)
	}
}

// TestUpdateAckPerOccurrenceCorrelation proves an earlier same-kind
// event cannot satisfy a later wait. The fixture part pins the
// cursor semantics directly; the PTY part drives two identical key
// sends (repeated same-kind events) through the real seam and
// requires two distinct, increasing sequence numbers.
func TestUpdateAckPerOccurrenceCorrelation(t *testing.T) {
	// Cursor semantics on a fixture log: the second wait for the
	// same kind must return the second occurrence, never the first.
	fixture := filepath.Join(t.TempDir(), "ack")
	content := "1 msg=key key=q state=browse overlay=none dismissed=false\n" +
		"2 msg=key key=q state=browse overlay=none dismissed=false\n"
	if err := os.WriteFile(fixture, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	log := &ackLog{file: fixture}
	isQ := func(ev ackEvent) bool { return ev.msg == "key" && ev.key == "q" }
	first, err := log.waitEvent("first q key event", isQ, time.Second)
	if err != nil {
		t.Fatalf("first occurrence not found: %v", err)
	}
	second, err := log.waitEvent("second q key event", isQ, time.Second)
	if err != nil {
		t.Fatalf("second occurrence not found: %v", err)
	}
	if first.seq != 1 || second.seq != 2 {
		t.Fatalf("per-occurrence correlation broken: waits consumed seqs %d then %d, want 1 then 2", first.seq, second.seq)
	}

	// PTY part: repeated same-kind key events through the real seam.
	// A fatal-with-results stream opens the error overlay over
	// browse; two overlay scroll keys are the repeated same-kind
	// events, then Esc dismisses and q exits 2.
	fakeDir := t.TempDir()
	records := []string{
		`{"type":"begin","data":{"path":{"text":"test.txt"}}}`,
		`{"type":"match","data":{"path":{"text":"test.txt"},"lines":{"text":"hello world\n"},"line_number":1,"submatches":[{"match":{"text":"hello"},"start":0,"end":5}]}}`,
		`{"type":"end","data":{"path":{"text":"test.txt"},"binary_offset":null}}`,
		`{"type":"summary","data":{}}`,
	}
	writeFatalFakeRG(t, fakeDir, records, "boom", 3)

	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "test.txt"), []byte("hello world\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ackFile := filepath.Join(t.TempDir(), "update-ack")
	cmd := exec.Command(binPath, "hello", ".")
	cmd.Dir = repo
	cmd.Env = []string{
		"PATH=" + fakeDir + ":" + os.Getenv("PATH"),
		"VRG_TEST_UPDATE_ACK=" + ackFile,
	}

	var down1, down2, escEv ackEvent
	res := runVrgPTY(t, cmd, ackFile, func(d *ptyDriver) {
		d.waitMsg(t, "search-complete")
		down1 = d.sendKey(t, "\x1b[B")
		down2 = d.sendKey(t, "\x1b[B")
		escEv = d.sendKey(t, "\x1b")
		if !escEv.dismissed {
			t.Fatalf("esc event did not acknowledge overlay dismissal: %+v", escEv)
		}
		d.sendKey(t, "q")
	})

	if res.exitCode != 2 {
		t.Fatalf("vrg exited %d, want 2 (fatal with results)", res.exitCode)
	}
	if down1.msg != "key" || down1.key != "down" || down2.msg != "key" || down2.key != "down" {
		t.Fatalf("repeated key events = %+v, %+v; want two key=down events", down1, down2)
	}
	if down2.seq <= down1.seq {
		t.Fatalf("second same-kind wait satisfied by earlier event: seqs %d then %d", down1.seq, down2.seq)
	}
}

// TestUpdateAckOverlayDismissalBeforeQuit proves the
// overlay-dismissal acknowledgement is causally ordered before a
// following quit (matrix row "esc before q"): Esc on the warning
// overlay produces a key=esc event carrying dismissed=true and
// overlay=none, and the q key event comes strictly after it.
func TestUpdateAckOverlayDismissalBeforeQuit(t *testing.T) {
	fakeDir := t.TempDir()
	writeFatalFakeRG(t, fakeDir, []string{`{"type":"summary","data":{}}`}, "warn", 1)

	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "test.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ackFile := filepath.Join(t.TempDir(), "update-ack")
	cmd := exec.Command(binPath, "hello", ".")
	cmd.Dir = repo
	cmd.Env = []string{
		"PATH=" + fakeDir + ":" + os.Getenv("PATH"),
		"VRG_TEST_UPDATE_ACK=" + ackFile,
	}

	var escEv, qEv ackEvent
	res := runVrgPTY(t, cmd, ackFile, func(d *ptyDriver) {
		d.waitMsg(t, "search-complete")
		escEv = d.sendKey(t, "\x1b")
		if !escEv.dismissed || escEv.overlay != "none" {
			t.Fatalf("esc event did not acknowledge overlay dismissal: %+v", escEv)
		}
		qEv = d.sendKey(t, "q")
	})

	if res.exitCode != 1 {
		t.Fatalf("vrg exited %d, want 1 (warning dismissed to no-results)", res.exitCode)
	}
	if qEv.seq <= escEv.seq {
		t.Fatalf("quit key event seq %d not after dismissal seq %d — dismissal was not acknowledged before quit", qEv.seq, escEv.seq)
	}
}

// TestAckWaitBoundedTimeout proves a wait for an acknowledgement
// that never arrives fails on a bounded timeout with a useful
// message rather than hanging (Issue #48).
func TestAckWaitBoundedTimeout(t *testing.T) {
	log := &ackLog{file: filepath.Join(t.TempDir(), "missing")}
	start := time.Now()
	_, err := log.waitEvent("a handshake that never arrives",
		func(ackEvent) bool { return false }, 300*time.Millisecond)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("wait for a never-arriving acknowledgement succeeded, want bounded-timeout failure")
	}
	if elapsed > 5*time.Second {
		t.Fatalf("bounded wait took %s, want prompt failure near the 300ms timeout", elapsed)
	}
	if !strings.Contains(err.Error(), "a handshake that never arrives") {
		t.Fatalf("timeout error does not name the awaited condition: %v", err)
	}
}

// boundedPollSleepFuncs names the bounded condition-poll helpers
// whose short sleeps pace re-checks of an explicit condition (file
// exists, line count reached, event matched, output substring
// rendered). Every other time.Sleep in a cmd/vrg test file is a
// fixed settling or inter-key delay and fails the Issue #48 static
// check.
var boundedPollSleepFuncs = map[string]bool{
	"waitForFile":     true, // bounded poll: fixture file exists
	"waitForAckLines": true, // bounded poll: collect-ack line count
	"waitEvent":       true, // bounded poll: update-ack event match
	"waitForOutput":   true, // bounded poll: rendered output marker
}

// TestNoFixedSleepsInPtyHelpers is the static Issue #48 check: no
// cmd/vrg test helper or trigger callback may synchronize on a fixed
// settling or inter-key time.Sleep. It scans every *_test.go file in
// this package for time.Sleep calls and requires each to live inside
// one of the named bounded condition polls in boundedPollSleepFuncs.
func TestNoFixedSleepsInPtyHelpers(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("cannot list package directory: %v", err)
	}
	fset := token.NewFileSet()
	var violations []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("cannot parse %s: %v", name, err)
		}
		for _, decl := range f.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			fnName := fd.Name.Name
			ast.Inspect(fd, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "Sleep" {
					return true
				}
				pkg, ok := sel.X.(*ast.Ident)
				if !ok || pkg.Name != "time" {
					return true
				}
				if !boundedPollSleepFuncs[fnName] {
					violations = append(violations,
						fmt.Sprintf("%s:%d: time.Sleep inside %s is a fixed delay, not a bounded condition poll",
							name, fset.Position(call.Pos()).Line, fnName))
				}
				return true
			})
		}
	}
	if len(violations) > 0 {
		t.Fatalf("fixed settling/inter-key sleeps found in PTY helpers (Issue #48):\n%s",
			strings.Join(violations, "\n"))
	}
}
