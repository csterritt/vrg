package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Issue #48 handshake matrix: every PTY helper action, the
// application-side postcondition it awaits, the acknowledgement
// record that proves it, and the action the acknowledgement unlocks.
// The vrg_testhooks binary appends one line "<seq> <event>" to the
// VRG_TEST_EVENT_ACK file per Update-processed message and per awaited
// transition — seq is monotonic per process, and helpers snapshot the
// per-event count before acting and wait for it to grow, so an earlier
// same-kind record can never satisfy a later wait. Records:
//
//	state:searching|noresults|browse|overlayonly — a state was entered
//	key:<keystroke>   — a key press was processed by Update
//	overlay:open|append|dismissed — the modal overlay changed
//	load:ok|load:fail|load:stale — a fileLoadedMsg settled
//	layout|layout:stale — a layoutReadyMsg installed or was discarded
//	diag              — a diagnostic line was collected
//	collected         — the child's output finished collecting
//	stderrline|fileloaded|layoutready|searchdone-equiv — per-message
//	size|popup|fail|fail:nil|other — the remaining processed messages
//
// | helper / test group            | action                 | postcondition awaited            | acknowledgement             | unlocks          |
// |--------------------------------|------------------------|----------------------------------|-----------------------------|------------------|
// | runVrgWithQuit(AckBin)         | process start          | searchDone processed, browse     | state:browse                | send q           |
// |                                | send q                 | key processed                    | key:q                       | waitExit         |
// | runVrgWithQuitBin (prod probe) | process start          | browse frame rendered            | output marker "─ "          | send q           |
// | runSteps (per step)            | send key               | key processed                    | key:<keystroke>             | step ack/expect  |
// |                                | step ack field         | named transition                 | e.g. overlay:dismissed      | expect wait      |
// |                                | expect field           | rendered postcondition           | output since step start     | next step        |
// | outcome overlay steps          | stream completes       | fatal/warning overlay open       | overlay:open + marker       | dismissal key    |
// | dismissal steps                | Esc/q on the overlay   | overlay dismissed                | overlay:dismissed + repaint | following q      |
// | runVrgKillChild                | ready file + SIGKILL   | child death processed to overlay | overlay:open                | the steps        |
// | replay/cancel diag waits       | child writes stderr    | diagnostic collected             | diag (per line)             | send key         |
// | gate-held tests                | child exits            | stream fully collected           | collected                   | release the gate |
// | controlled-failure tests       | trigger file appears   | failMsg processed                | fail                        | exit             |
// | load-dependent content         | browse entered         | current file's load settled      | load:ok / layout            | content repaint  |
//
// A handshake that never arrives fails on the bounded poll with the
// event name, the awaited occurrence, and the log — never a hang.

// ackRecord is one parsed acknowledgement line: a per-process
// monotonic sequence number and the event it acknowledges.
type ackRecord struct {
	seq   int
	event string
}

// ackEnv provisions a fresh per-test event log and returns the
// VRG_TEST_EVENT_ACK environment entry installing it in the tagged
// binary.
func ackEnv(t *testing.T) string {
	t.Helper()
	return "VRG_TEST_EVENT_ACK=" + filepath.Join(t.TempDir(), "events")
}

// ackPathFromEnv returns the event-log path an environment requests,
// or "" when the run installs no acknowledgement seam.
func ackPathFromEnv(env []string) string {
	p := ""
	for _, e := range env {
		if v, ok := strings.CutPrefix(e, "VRG_TEST_EVENT_ACK="); ok {
			p = v
		}
	}
	return p
}

// keyEvent is the acknowledgement record one processed key press
// produces: "key:<keystroke>" in bubbletea's KeyPressMsg spelling.
func keyEvent(key string) string {
	name := key
	switch key {
	case "\x1b":
		name = "esc"
	case "\x03":
		name = "ctrl+c"
	}
	return "key:" + name
}

// parseAckRecords decodes the event log's "<seq> <event>" lines; a
// partially written line — a read racing an append — is skipped and
// resolves on the next poll.
func parseAckRecords(data string) []ackRecord {
	var recs []ackRecord
	for _, line := range strings.Split(data, "\n") {
		f := strings.Fields(line)
		if len(f) != 2 {
			continue
		}
		seq, err := strconv.Atoi(f[0])
		if err != nil {
			continue
		}
		recs = append(recs, ackRecord{seq: seq, event: f[1]})
	}
	return recs
}

// ackRecords is the session's acknowledgement log as it currently
// stands; a missing or unreadable file reads as no records.
func (s *ptySession) ackRecords() []ackRecord {
	if s.ackPath == "" {
		return nil
	}
	b, err := os.ReadFile(s.ackPath)
	if err != nil {
		return nil
	}
	return parseAckRecords(string(b))
}

// ackCount is the number of acknowledgements of event the log holds —
// the per-occurrence counter helpers baseline against before acting.
func (s *ptySession) ackCount(event string) int {
	n := 0
	for _, r := range s.ackRecords() {
		if r.event == event {
			n++
		}
	}
	return n
}

// awaitAck polls until the log holds more than have acknowledgements
// of event or the bound expires — the caller snapshots have before the
// triggering action so only the postcondition the action caused can
// satisfy the wait. A missing acknowledgement is a bounded failure
// naming the event and the awaited occurrence, never a hang.
func (s *ptySession) awaitAck(event string, have int, bound time.Duration) error {
	if s.ackPath == "" {
		return fmt.Errorf("session has no acknowledgement log (VRG_TEST_EVENT_ACK unset)")
	}
	deadline := time.Now().Add(bound)
	for time.Now().Before(deadline) {
		if s.ackCount(event) > have {
			return nil
		}
		select {
		case code := <-s.done:
			s.done <- code // leave it for waitExit
			return fmt.Errorf("vrg exited before ack %q occurrence %d; log: %v; output: %q",
				event, have+1, s.ackRecords(), s.output())
		default:
		}
		time.Sleep(10 * time.Millisecond)
	}
	return fmt.Errorf("ack %q occurrence %d never arrived within %s; log: %v; output: %q",
		event, have+1, bound, s.ackRecords(), s.output())
}

// waitAck blocks on the next acknowledgement of event past the
// have baseline, failing the test on the standard bounded timeout.
func (s *ptySession) waitAck(t *testing.T, event string, have int) {
	t.Helper()
	if err := s.awaitAck(event, have, 20*time.Second); err != nil {
		s.cmd.Process.Kill()
		<-s.done
		t.Fatal(err)
	}
}

// The seam's lifecycle proof: one acknowledgement record per
// Update-processed message and awaited transition, in causal order
// with strictly increasing per-process sequence numbers — the initial
// searching state, the completed collection, the browse transition,
// the settled load, and the processed quit key.
func TestEventAckSeamRecordsLifecycle(t *testing.T) {
	dir := t.TempDir()
	writeHappyFiles(t, dir)
	fakebin := fakeRG(t, happyStreamRG)
	env := testEnv(fakebin, ackEnv(t))

	s := startVrgPTY(t, dir, env, "foo")
	s.waitAck(t, "state:browse", 0) // searchDone processed; browse entered
	s.waitAck(t, "load:ok", 0)      // the current file's load settled
	s.send("q")
	s.waitAck(t, keyEvent("q"), 0) // the key was processed
	code := s.waitExit()
	if code != 0 {
		t.Fatalf("exit = %d, want 0; output: %q", code, s.output())
	}

	recs := s.ackRecords()
	if len(recs) == 0 {
		t.Fatal("event log is empty")
	}
	for i, r := range recs {
		if r.seq != i+1 {
			t.Fatalf("record %d has seq %d, want %d: %v", i, r.seq, i+1, recs)
		}
	}
	// Causal order: searching precedes collection, which precedes the
	// browse transition, which precedes the processed key.
	want := []string{"state:searching", "collected", "state:browse", "key:q"}
	next := 0
	for _, r := range recs {
		if next < len(want) && r.event == want[next] {
			next++
		}
	}
	if next != len(want) {
		t.Fatalf("event order %v lacks the causal chain %v", recs, want)
	}
}

// Acknowledgements are correlated per occurrence: two same-kind keys
// produce two distinct records, and waiting for the second occurrence
// can never be satisfied by the first's record — each send snapshots
// the count before writing and waits for it to grow.
func TestAckCorrelationSameKindOccurrences(t *testing.T) {
	dir := t.TempDir()
	writeHappyFiles(t, dir)
	fakebin := fakeRG(t, happyStreamRG)
	env := testEnv(fakebin, ackEnv(t))

	s := startVrgPTY(t, dir, env, "foo")
	s.waitAck(t, "state:browse", 0)
	// n navigates the matched-line cursor — a repeatable same-kind
	// key; each send waits on the occurrence after the baseline.
	s.send("n")
	s.waitAck(t, keyEvent("n"), 0)
	s.send("n")
	s.waitAck(t, keyEvent("n"), 1)
	s.send("q")
	s.waitAck(t, keyEvent("q"), 0)
	if code := s.waitExit(); code != 0 {
		t.Fatalf("exit = %d, want 0; output: %q", code, s.output())
	}
	var seqs []int
	for _, r := range s.ackRecords() {
		if r.event == keyEvent("n") {
			seqs = append(seqs, r.seq)
		}
	}
	if len(seqs) != 2 || seqs[0] >= seqs[1] {
		t.Fatalf("key:n records = %v, want two with increasing seqs", seqs)
	}
}

// The overlay-dismissal regression: q on the warning overlay dismisses
// it — acknowledged application-side before the following quit key is
// sent — and the second q then leaves the browse view.
func TestOverlayDismissalAcknowledgedBeforeQuit(t *testing.T) {
	dir := t.TempDir()
	writeHappyFiles(t, dir)
	fakebin := fakeRG(t, warnStreamRG)
	env := testEnv(fakebin, ackEnv(t))

	s := startVrgPTY(t, dir, env, "foo")
	s.waitAck(t, "overlay:open", 0)
	s.send("q")
	s.waitAck(t, "overlay:dismissed", 0) // acknowledged before the quit key
	s.send("q")
	s.waitAck(t, keyEvent("q"), 1)
	code := s.waitExit()
	if code != 0 {
		t.Fatalf("exit = %d, want 0; output: %q", code, s.output())
	}
	// The log proves the ordering: the dismissal is recorded inside
	// the first key's update, ahead of its key record and the second
	// key's.
	var events []string
	for _, r := range s.ackRecords() {
		events = append(events, r.event)
	}
	want := []string{"overlay:open", "overlay:dismissed", "key:q", "key:q"}
	next := 0
	for _, e := range events {
		if next < len(want) && e == want[next] {
			next++
		}
	}
	if next != len(want) {
		t.Fatalf("event order %v lacks %v", events, want)
	}
}

// A handshake that never arrives fails on the bound — with the event
// name, the awaited occurrence, and the bound in the message — rather
// than hanging.
func TestAckWaitFailsOnBoundedTimeout(t *testing.T) {
	dir := t.TempDir()
	ready := filepath.Join(dir, "ready")
	pidFile := filepath.Join(dir, "pid")
	fakebin := fakeRG(t, blockedRG)
	env := testEnv(fakebin,
		"VRG_TEST_RG_READY="+ready,
		"VRG_TEST_RG_PID="+pidFile,
		ackEnv(t))

	s := startVrgPTY(t, dir, env, "foo")
	s.waitAck(t, "state:searching", 0) // the seam is live
	start := time.Now()
	err := s.awaitAck("state:browse", 0, 300*time.Millisecond)
	if err == nil {
		s.cmd.Process.Kill()
		<-s.done
		t.Fatal("awaitAck for a never-arriving event returned nil error")
	}
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		s.cmd.Process.Kill()
		<-s.done
		t.Fatalf("awaitAck exceeded its bound: %s", elapsed)
	}
	for _, want := range []string{"state:browse", "occurrence 1", "300ms"} {
		if !strings.Contains(err.Error(), want) {
			s.cmd.Process.Kill()
			<-s.done
			t.Fatalf("timeout error lacks %q: %v", want, err)
		}
	}
	s.cmd.Process.Kill()
	<-s.done
}

// The static half of the handshake contract: no PTY helper may
// synchronize on a fixed settling or inter-key delay. The only
// permitted time.Sleep call sites are the short paces inside the
// allowlisted bounded condition polls — each iteration re-checks an
// explicit condition (output marker, fixture file, acknowledgement
// count) against a deadline — and the tagged seam's file watchers,
// whose unbounded holds are the gate mechanism itself, not
// test-side synchronization.
func TestNoFixedSleepsInPTYHelpers(t *testing.T) {
	allowed := map[string]bool{
		"waitFor":        true,
		"waitForFrom":    true,
		"waitForFile":    true,
		"waitForAcks":    true,
		"awaitAck":       true,
		"waitFileGone":   true,
		"waitFileExists": true,
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		isTest := strings.HasSuffix(name, "_test.go")
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			hasSleep := false
			ast.Inspect(fn, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				id, ok := sel.X.(*ast.Ident)
				if ok && id.Name == "time" && sel.Sel.Name == "Sleep" {
					hasSleep = true
					if !allowed[fn.Name.Name] {
						pos := fset.Position(call.Pos())
						t.Errorf("%s:%d: fixed time.Sleep in %s — helpers wait on acknowledgements or bounded condition polls",
							name, pos.Line, fn.Name.Name)
					}
				}
				return true
			})
			// A permitted poll in test code must still be bounded:
			// its body must name the deadline or bound it enforces.
			if hasSleep && isTest && allowed[fn.Name.Name] {
				bounded := false
				ast.Inspect(fn, func(n ast.Node) bool {
					if id, ok := n.(*ast.Ident); ok && (id.Name == "deadline" || id.Name == "bound") {
						bounded = true
					}
					return !bounded
				})
				if !bounded {
					t.Errorf("%s: %s paces with time.Sleep but names no deadline or bound", name, fn.Name.Name)
				}
			}
		}
	}
}
