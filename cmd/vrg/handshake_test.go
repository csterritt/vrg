//go:build linux

package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// The Issue #48 handshake contract: every PTY/subprocess helper waits on
// a causally correlated application-side acknowledgement for the model
// transition or key-processing boundary it cares about — never on
// elapsed time as a proxy for progress. The vrg_testhooks binary writes
// one record per observed event to the file VRG_TEST_EVENT_ACK names:
//
//	<seq> <kind> [<detail>]
//
// with a per-session monotonic sequence, so an earlier same-kind record
// can never satisfy a later wait. Records are per process (each run
// writes its own file) and per occurrence (waiting on the n-th matching
// record). Bounded polls pace checks of an explicit condition; no
// helper synchronizes on a fixed settling or inter-key delay. The
// acknowledgement readers and the ack-gated stepper live in
// acksteps_test.go, untagged so the portable boundary tests share them.

// handshakeRow is one line of the finite acknowledgement matrix: the
// covered helpers, the triggering action, the exact application-side
// postcondition, the acknowledgement channel that proves it, and the
// next action that acknowledgement unlocks. Event-log rows name the
// record kind and detail the harness waits for; the other channels are
// the pre-existing application-side handshakes (the reap side channel,
// the collect acknowledgement, the fifo open/close pairing, the
// child's own ready file, process exit) plus the stdout-marker
// condition poll reserved for binaries that cannot carry the seam.
type handshakeRow struct {
	helpers []string
	action  string
	post    string
	via     string
	kind    string
	detail  string
	next    string
}

// handshakeMatrix is the complete helper/action/postcondition/
// acknowledgement matrix every key send and assumed transition in the
// PTY and subprocess harness rides on.
var handshakeMatrix = []handshakeRow{
	{
		helpers: []string{"startVrgPTY", "startVrgPTYSize", "startPiped"},
		action:  "session start",
		post:    "the model entered the searching phase",
		via:     "event-log", kind: "phase", detail: "searching",
		next: "the first key send or signal",
	},
	{
		helpers: []string{"awaitReadyPID"},
		action:  "fake rg exec",
		post:    "the child ran and wrote its pid",
		via:     "ready-file",
		next:    "cancelling or signalling a live child",
	},
	{
		helpers: []string{"awaitFileContent"},
		action:  "child exit (optionally under the held gate)",
		post:    "the collector reaped the child",
		via:     "reap-file",
		next:    "a cancel key during gate-held preparation",
	},
	{
		helpers: []string{"awaitFileContent", "awaitAck"},
		action:  "a child stderr line or injected diagnostic is processed",
		post:    "the diagnostic entered the session collection",
		via:     "collect-file|event-log", kind: "collected",
		next: "the exit key that must replay it",
	},
	{
		helpers: []string{"awaitAck", "pollAck"},
		action:  "the search completion is processed",
		post:    "the post-search phase was entered",
		via:     "event-log", kind: "phase", detail: "browse|noresults|fatal",
		next: "phase-specific keys",
	},
	{
		helpers: []string{"awaitAck"},
		action:  "a completion or current-file failure carries diagnostics",
		post:    "the diagnostics overlay owns the keyboard",
		via:     "event-log", kind: "overlay", detail: "open",
		next: "the dismissal key",
	},
	{
		helpers: []string{"awaitAck", "send"},
		action:  "q/esc is sent with the diagnostics overlay open",
		post:    "the overlay was dismissed",
		via:     "event-log", kind: "overlay", detail: "closed",
		next: "the underlying screen's quit key",
	},
	{
		helpers: []string{"awaitAck"},
		action:  "h/? is sent over browse or no-results",
		post:    "the help overlay owns the keyboard",
		via:     "event-log", kind: "help", detail: "open",
		next: "the help close key",
	},
	{
		helpers: []string{"awaitAck"},
		action:  "q/esc/h/? is sent with help open",
		post:    "help was dismissed",
		via:     "event-log", kind: "help", detail: "closed",
		next: "the underlying screen's keys",
	},
	{
		helpers: []string{"send"},
		action:  "any key is written to the PTY or pipe",
		post:    "the event loop consumed the key press",
		via:     "event-log", kind: "key",
		next: "the transition acknowledgement for that key",
	},
	{
		helpers: []string{"awaitAck"},
		action:  "a file-load worker completion is processed",
		post:    "the buffer installed or its failure collected",
		via:     "event-log", kind: "load", detail: "ok|fail",
		next: "keys depending on loaded content",
	},
	{
		helpers: []string{"send", "awaitAck"},
		action:  "q/ctrl+c is sent while searching or quitting a completed state",
		post:    "the model committed to quit with its settled exit code",
		via:     "event-log", kind: "quitting", detail: "0|1|2|130",
		next: "waitExit / finish",
	},
	{
		helpers: []string{"fireFifo", "holdFifo"},
		action:  "a trigger fifo's writer opens and closes, or stays paired",
		post:    "vrg's reader consumed the release; the effect lands",
		via:     "fifo-pair",
		next:    "the effect's acknowledgement (a collected record or exit)",
	},
	{
		helpers: []string{"waitExit", "finish"},
		action:  "the process exits",
		post:    "the exit status, restored termios, and full stream",
		via:     "process-exit",
		next:    "post-exit assertions",
	},
	{
		helpers: []string{"waitFor"},
		action:  "a rendered frame carrying the marker is emitted",
		post:    "the marker text reached the terminal stream",
		via:     "stdout-marker",
		next:    "content assertions — never the sole gate on a key send",
	},
	{
		helpers: []string{"runStepsBin"},
		action:  "stdout marker observed under an untagged binary",
		post:    "the frame was emitted by a binary that cannot carry the seam",
		via:     "stdout-marker",
		next:    "the step's keys — only for the production-boundary run",
	},
	{
		helpers: []string{"runVrgTUI", "runAckSteps"},
		action:  "scripted key steps against a hooked binary",
		post:    "each step's acknowledgement record lands before its keys",
		via:     "event-log", kind: "*",
		next: "the step's keys, then the next step",
	},
}

// eventAckKinds is the finite set of record kinds the vrg_testhooks
// binary emits through the VRG_TEST_EVENT_ACK seam.
var eventAckKinds = map[string]bool{
	"phase":     true, // a phase was entered — detail: searching|browse|noresults|fatal
	"key":       true, // a key press was consumed — detail: its key name
	"collected": true, // a diagnostic entered the session collection
	"overlay":   true, // the diagnostics overlay changed — detail: open|closed
	"help":      true, // the help overlay changed — detail: open|closed
	"load":      true, // a file-load completion was processed — detail: ok|fail <path>
	"quitting":  true, // the model committed to quit — detail: settled exit code
}

// nonEventAcks are the acknowledgement channels that exist outside the
// event log: the collector's reap side channel, the collect
// acknowledgement file, the fifo open/close pairing, the fake child's
// ready file, process exit, and the stdout-marker condition poll.
var nonEventAcks = map[string]bool{
	"reap-file":     true,
	"collect-file":  true,
	"fifo-pair":     true,
	"ready-file":    true,
	"process-exit":  true,
	"stdout-marker": true,
}

// coveredHelpers is every harness function whose waits or sends the
// matrix governs.
var coveredHelpers = []string{
	"startVrgPTY", "startVrgPTYSize", "startPiped",
	"awaitReadyPID", "awaitFileContent", "awaitAck", "pollAck",
	"send", "waitExit", "finish", "waitFor",
	"fireFifo", "holdFifo",
	"runStepsBin", "runVrgTUI", "runAckSteps",
}

// testFuncDecls parses the package's test files and returns the set of
// declared function and method names plus each name's source body.
func testFuncDecls(t *testing.T) (names map[string]bool, bodies map[string]string) {
	t.Helper()
	names = map[string]bool{}
	bodies = map[string]string{}
	matches, err := filepath.Glob("*_test.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, file := range matches {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		f, err := parser.ParseFile(fset, file, src, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			names[fn.Name.Name] = true
			start := fset.Position(fn.Body.Pos()).Offset
			end := fset.Position(fn.Body.End()).Offset
			bodies[fn.Name.Name] += string(src[start:end])
		}
	}
	return names, bodies
}

// The matrix is finite and every row is checkable: each names the
// helpers it governs (which must exist), a real acknowledgement
// channel, and — for event-log rows — a kind the seam emits. Every
// covered helper is governed by at least one row.
func TestHandshakeMatrixWellFormed(t *testing.T) {
	names, _ := testFuncDecls(t)
	governed := map[string]bool{}
	for i, row := range handshakeMatrix {
		if len(row.helpers) == 0 || row.action == "" || row.post == "" || row.next == "" {
			t.Errorf("matrix row %d is incomplete: %+v", i, row)
		}
		vias := strings.Split(row.via, "|")
		for _, v := range vias {
			if !nonEventAcks[v] && v != "event-log" {
				t.Errorf("matrix row %d uses unknown acknowledgement channel %q", i, v)
			}
		}
		if row.kind != "" && row.kind != "*" && !eventAckKinds[row.kind] {
			t.Errorf("matrix row %d names un-emitted event kind %q", i, row.kind)
		}
		if row.kind == "*" && row.detail != "" {
			t.Errorf("matrix row %d uses the generic kind but pins a detail", i)
		}
		for _, h := range row.helpers {
			if !names[h] {
				t.Errorf("matrix row %d governs unknown helper %q", i, h)
			}
			governed[h] = true
		}
	}
	for _, h := range coveredHelpers {
		if !names[h] {
			t.Errorf("covered helper %q does not exist", h)
		}
		if !governed[h] {
			t.Errorf("covered helper %q has no handshake matrix row", h)
		}
	}
}

// The acknowledgement seam itself: the tag must join the explicit
// vrg-consumed hook manifest so the untagged-artifact probe covers it.
func TestEventAckHookInManifest(t *testing.T) {
	found := false
	for _, name := range hookManifest {
		if name == "VRG_TEST_EVENT_ACK" {
			found = true
		}
	}
	if !found {
		t.Fatalf("hook manifest lacks VRG_TEST_EVENT_ACK: %v", hookManifest)
	}
}

// fakeRgFatalScript signals readiness then dies with exit status 3 and
// no usable results — a fatal outcome whose overlay stands alone.
const fakeRgFatalScript = `#!/bin/sh
echo $$ > "$FAKE_RG_CAPTURE_DIR/ready"
exit 3
`

// Every event-log row of the matrix is observed, correlated per
// process (each session writes its own file) and per occurrence (the
// sequence is strictly increasing), across the sessions that produce
// its postcondition: a gated browse session exercising help and wrap
// keys, a cancellation, an overlay-bearing load failure, and a fatal
// outcome. The non-event channels — ready file, reap side channel,
// collect acknowledgement, fifo pairing, process exit, stdout marker —
// are exercised in the same runs.
func TestHandshakeMatrixRowsAcknowledged(t *testing.T) {
	var all []ackRec
	observed := map[string]bool{}

	// Session A: a completed search held at the preparation gate until
	// the fixture file exists — phase, load, key, and help records.
	t.Run("browse session", func(t *testing.T) {
		fakeDir, capDir := writeFakeRg(t, fakeRgStreamScript)
		events := filepath.Join(capDir, "events")
		reap := filepath.Join(capDir, "reap")
		gate := filepath.Join(capDir, "gate-fifo")
		if err := syscall.Mkfifo(gate, 0o600); err != nil {
			t.Fatalf("mkfifo: %v", err)
		}
		r := startVrgPTY(t, ptyEnv(fakeDir, capDir,
			"VRG_TEST_EVENT_ACK="+events,
			"VRG_TEST_REAP="+reap,
			"VRG_TEST_GATE="+gate), "foo")
		_ = awaitReadyPID(t, capDir)
		observed["ready-file"] = true
		awaitAck(t, events, 1, "phase", "searching")
		// The child exited and was reaped while preparation is still
		// gate-held — the reap side channel proves the boundary.
		if got := awaitFileContent(t, reap); !strings.Contains(got, "exit status 0") {
			t.Fatalf("reap side channel = %q, want the child's clean exit", got)
		}
		observed["reap-file"] = true
		// The load target must exist before the gate releases.
		if err := os.WriteFile(filepath.Join(r.cmd.Dir, "f.txt"), []byte("hello\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		fireFifo(gate) // writer open/close pairs with vrg's reader: released
		observed["fifo-pair"] = true
		awaitAck(t, events, 1, "phase", "browse")
		awaitAck(t, events, 1, "load", "ok")
		r.send(t, "h")
		awaitAck(t, events, 1, "key", "h")
		awaitAck(t, events, 1, "help", "open")
		r.waitFor(t, "Key bindings") // the rendered help frame
		observed["stdout-marker"] = true
		r.send(t, "\x1b") // a raw ESC byte: the terminal's esc key
		awaitAck(t, events, 1, "key", "esc")
		awaitAck(t, events, 1, "help", "closed")
		r.send(t, "w")
		first := awaitAck(t, events, 1, "key", "w")
		r.send(t, "w")
		second := awaitAck(t, events, 2, "key", "w")
		if second.seq <= first.seq {
			t.Fatalf("second w acknowledgement seq %d did not follow first %d", second.seq, first.seq)
		}
		r.send(t, "q")
		awaitAck(t, events, 1, "quitting", "0")
		code, _ := r.waitExit(t)
		if code != 0 {
			t.Fatalf("exit = %d, want 0", code)
		}
		observed["process-exit"] = true
		recs, err := readAcks(events)
		if err != nil {
			t.Fatal(err)
		}
		assertAcksMonotonic(t, recs)
		all = append(all, recs...)
	})

	// Session B: cancellation while searching — the quitting record and
	// the reap side channel both fire.
	t.Run("cancel session", func(t *testing.T) {
		fakeDir, capDir := writeFakeRg(t, fakeRgBlockScript)
		events := filepath.Join(capDir, "events")
		reap := filepath.Join(capDir, "reap")
		r := startVrgPTY(t, ptyEnv(fakeDir, capDir,
			"VRG_TEST_EVENT_ACK="+events,
			"VRG_TEST_REAP="+reap), "foo")
		_ = awaitReadyPID(t, capDir)
		awaitAck(t, events, 1, "phase", "searching")
		r.send(t, "q")
		awaitAck(t, events, 1, "key", "q")
		awaitAck(t, events, 1, "quitting", "130")
		code, _ := r.waitExit(t)
		if code != 130 {
			t.Fatalf("exit = %d, want 130", code)
		}
		if got := readFile(t, reap); !strings.Contains(got, "killed") {
			t.Fatalf("reap side channel = %q, want the killed wait status", got)
		}
		recs, err := readAcks(events)
		if err != nil {
			t.Fatal(err)
		}
		assertAcksMonotonic(t, recs)
		all = append(all, recs...)
	})

	// Session C: a current-file load failure — the collected record,
	// the load failure, and the overlay open/close pair, with the
	// standalone collect acknowledgement armed for the same events.
	t.Run("overlay session", func(t *testing.T) {
		fakeDir, capDir := writeFakeRg(t, fakeRgBadPathScript)
		events := filepath.Join(capDir, "events")
		ack := filepath.Join(capDir, "ack")
		r := startVrgPTY(t, ptyEnv(fakeDir, capDir,
			"VRG_TEST_EVENT_ACK="+events,
			"VRG_TEST_COLLECT_ACK="+ack), "foo")
		_ = awaitReadyPID(t, capDir)
		awaitAck(t, events, 1, "phase", "browse")
		awaitAck(t, events, 1, "collected", "")
		awaitFileContent(t, ack)
		observed["collect-file"] = true
		awaitAck(t, events, 1, "load", "fail")
		awaitAck(t, events, 1, "overlay", "open")
		r.send(t, "q")
		awaitAck(t, events, 1, "overlay", "closed")
		r.send(t, "q")
		awaitAck(t, events, 1, "quitting", "0")
		code, _ := r.waitExit(t)
		if code != 0 {
			t.Fatalf("exit = %d, want 0", code)
		}
		recs, err := readAcks(events)
		if err != nil {
			t.Fatal(err)
		}
		assertAcksMonotonic(t, recs)
		all = append(all, recs...)
	})

	// Session D: a fatal outcome with no usable results — the fatal
	// phase, a lone overlay, and dismissal quitting with status 2.
	t.Run("fatal session", func(t *testing.T) {
		fakeDir, capDir := writeFakeRg(t, fakeRgFatalScript)
		events := filepath.Join(capDir, "events")
		r := startVrgPTY(t, ptyEnv(fakeDir, capDir,
			"VRG_TEST_EVENT_ACK="+events), "foo")
		_ = awaitReadyPID(t, capDir)
		awaitAck(t, events, 1, "phase", "fatal")
		awaitAck(t, events, 1, "overlay", "open")
		r.send(t, "q")
		awaitAck(t, events, 1, "quitting", "2")
		code, out := r.waitExit(t)
		if code != 2 {
			t.Fatalf("exit = %d, want 2; output: %q", code, out)
		}
		recs, err := readAcks(events)
		if err != nil {
			t.Fatal(err)
		}
		assertAcksMonotonic(t, recs)
		all = append(all, recs...)
	})

	// Session E: the ack-gated stepper itself — a pipe run whose one
	// step sends q only after the browse phase is acknowledged and its
	// marker has rendered.
	t.Run("stepped session", func(t *testing.T) {
		fakeDir, capDir := writeFakeRg(t, fakeRgStreamScript)
		events := filepath.Join(capDir, "events")
		work := t.TempDir()
		if err := os.WriteFile(filepath.Join(work, "f.txt"), []byte("hello\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		res := runAckSteps(t, binPath, work, append(searchEnv(fakeDir, capDir),
			"VRG_TEST_EVENT_ACK="+events), events, []ackStep{
			{n: 1, kind: "phase", detail: "browse", marker: "f.txt", keys: "q"},
		}, "foo")
		if res.code != 0 {
			t.Fatalf("exit = %d, want 0 (stderr %q)", res.code, res.stderr)
		}
		if !strings.Contains(res.stdout, "f.txt") {
			t.Fatalf("stdout lacks the browse view: %q", res.stdout)
		}
		recs, err := readAcks(events)
		if err != nil {
			t.Fatal(err)
		}
		assertAcksMonotonic(t, recs)
		all = append(all, recs...)
	})

	// Every event-log matrix row's acknowledgement was observed; every
	// declared non-event channel was exercised.
	for i, row := range handshakeMatrix {
		for _, v := range strings.Split(row.via, "|") {
			switch v {
			case "event-log":
				if row.kind == "*" {
					continue // the stepper gates on whatever record its step names
				}
				if countAcks(all, row.kind, row.detail) == 0 {
					t.Errorf("matrix row %d (%s): no %q %q acknowledgement observed across the sessions", i, row.post, row.kind, row.detail)
				}
			default:
				if !observed[v] {
					t.Errorf("matrix row %d (%s): acknowledgement channel %q never exercised", i, row.post, v)
				}
			}
		}
	}
}

// Repeated same-kind events correlate per occurrence: the second w's
// acknowledgement is a distinct record after the first's, and while
// only one w has been sent the second-w wait cannot be satisfied —
// the wait fails on a bounded timeout rather than consuming the stale
// first record.
func TestAckRecordsCorrelatePerOccurrence(t *testing.T) {
	fakeDir, capDir := writeFakeRg(t, fakeRgStreamScript)
	events := filepath.Join(capDir, "events")
	r := startVrgPTY(t, ptyEnv(fakeDir, capDir,
		"VRG_TEST_EVENT_ACK="+events), "foo")
	_ = awaitReadyPID(t, capDir)
	awaitAck(t, events, 1, "phase", "browse")

	r.send(t, "w")
	first := awaitAck(t, events, 1, "key", "w")
	if _, err := pollAck(events, 2, "key", "w", 400*time.Millisecond); err == nil {
		t.Fatal("the first w's record satisfied the second w's wait — occurrences are not correlated")
	}
	r.send(t, "w")
	second := awaitAck(t, events, 2, "key", "w")
	if second.seq <= first.seq {
		t.Fatalf("second w acknowledgement seq %d did not follow first %d", second.seq, first.seq)
	}
	// The current file is absent from the run's empty workdir, so its
	// load fails and the error overlay owns the keyboard: the first q
	// is acknowledged as a dismissal and the second as the quit.
	awaitAck(t, events, 1, "overlay", "open")
	r.send(t, "q")
	awaitAck(t, events, 1, "overlay", "closed")
	r.send(t, "q")
	awaitAck(t, events, 1, "quitting", "")
	r.waitExit(t)
}

// Overlay dismissal is itself acknowledged before a following q: the
// q that dismisses the warning overlay produces an overlay-closed
// record causally after the open, and only that record — not frame
// growth — unlocks the quit key.
func TestOverlayDismissalAcknowledgedBeforeQuit(t *testing.T) {
	fakeDir, capDir := writeFakeRg(t, fakeRgWarnStreamScript)
	events := filepath.Join(capDir, "events")
	r := startVrgPTY(t, ptyEnv(fakeDir, capDir,
		"VRG_TEST_EVENT_ACK="+events), "foo")
	_ = awaitReadyPID(t, capDir)
	awaitAck(t, events, 1, "collected", "")
	open := awaitAck(t, events, 1, "overlay", "open")
	r.send(t, "q")
	closed := awaitAck(t, events, 1, "overlay", "closed")
	if closed.seq <= open.seq {
		t.Fatalf("dismissal record seq %d did not follow open seq %d", closed.seq, open.seq)
	}
	r.send(t, "q")
	awaitAck(t, events, 1, "quitting", "0")
	code, out := r.waitExit(t)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; output: %q", code, out)
	}
	assertReplayedOnce(t, out, "warn one")
}

// A wait for an acknowledgement that never arrives fails on a bounded
// timeout with a useful message — it names the wanted record and lists
// what the session did acknowledge — rather than hanging.
func TestMissingAcknowledgementFailsBounded(t *testing.T) {
	fakeDir, capDir := writeFakeRg(t, fakeRgBlockScript)
	events := filepath.Join(capDir, "events")
	r := startVrgPTY(t, ptyEnv(fakeDir, capDir,
		"VRG_TEST_EVENT_ACK="+events), "foo")
	_ = awaitReadyPID(t, capDir)
	awaitAck(t, events, 1, "phase", "searching")

	// The overlay is never opened in this session, so no dismissal can
	// ever be acknowledged.
	start := time.Now()
	_, err := pollAck(events, 1, "overlay", "closed", 500*time.Millisecond)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("an acknowledgement that never arrived satisfied its wait")
	}
	if elapsed > 5*time.Second {
		t.Fatalf("a missing acknowledgement took %s to fail — not bounded", elapsed)
	}
	if !strings.Contains(err.Error(), "overlay") || !strings.Contains(err.Error(), "closed") {
		t.Fatalf("timeout error does not name the wanted acknowledgement: %v", err)
	}
	if !strings.Contains(err.Error(), "searching") {
		t.Fatalf("timeout error does not list the observed records: %v", err)
	}

	r.send(t, "q")
	awaitAck(t, events, 1, "quitting", "130")
	r.waitExit(t)
}

// boundedPollHelpers are the only helpers allowed to pace a loop with
// a short sleep or After: each iteration re-checks an explicit
// condition under a bounded deadline, so the sleep paces polling,
// never stands in for progress. Everything else in the harness —
// especially key sends and transition waits — must not contain a
// fixed delay at all.
var boundedPollHelpers = map[string]bool{
	"awaitFileContent": true,
	"pollAck":          true,
	"runAckSteps":      true,
	"waitFor":          true,
}

// No PTY/subprocess helper synchronizes on a fixed settling or
// inter-key delay: a helper may only call time.Sleep/time.After inside
// a bounded condition poll, and that poll must visibly loop on a
// deadline while checking a condition each iteration.
func TestHarnessHasNoFixedDelays(t *testing.T) {
	_, bodies := testFuncDecls(t)
	// The split spellings keep this checker's own literals from matching.
	delay, after := "time."+"Sleep", "time."+"After"
	for name, body := range bodies {
		if !strings.Contains(body, delay) && !strings.Contains(body, after) {
			continue
		}
		if !boundedPollHelpers[name] {
			t.Errorf("helper %s contains a fixed delay outside the bounded-poll allowlist", name)
			continue
		}
		if !strings.Contains(body, "deadline") && !strings.Contains(body, "Deadline") {
			t.Errorf("poll helper %s lacks a bounded deadline", name)
		}
		if !strings.Contains(body, "for") {
			t.Errorf("poll helper %s does not loop on its condition", name)
		}
	}
}
