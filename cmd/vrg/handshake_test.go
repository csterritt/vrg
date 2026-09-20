package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// This file is Issue 48's deterministic-handshake contract for the
// cmd/vrg PTY harness: every key send and assumed application
// transition waits on a causally correlated application-side
// acknowledgement — never on elapsed time as a proxy for progress.
// Issue 48's text names helpers (runVrgWithKeys, runVrgKillChild, the
// runVrgReplay trigger callbacks, runVrgWithQuit) that the Issue 45/46
// harness rewrite replaced with the ptyRun family; the matrix below
// covers the successor helpers every named test now uses.
//
// The acknowledgement stream is VRG_TEST_ACK: the vrg_testhooks seam
// appends one record per Update-processed message plus one record per
// committed transition, each carrying a per-process monotonic sequence
// number — "seq kind detail". Because a wait names a kind and a
// high-water mark, an earlier same-kind record can never satisfy a
// later wait: correlation is per process (one stream per run) and per
// occurrence (seq > mark).
//
// Handshake matrix — helper/test group, triggering action,
// application-side postcondition, acknowledgement, and the next action
// the acknowledgement unlocks:
//
//	all PTY tests          fake rg started                  fixture ready file exists          waitFile: fixture-owned file        any vrg-side wait
//	replay tests           child stderr line drained        line processed into collection     waitCollectAck[N]: COLLECT_ACK line   exit key, trigger write
//	sendAcked              PTY key write                    Update processed the key press     key <name> record, seq > send mark    transition waits, next key
//	overlay dismissal      q/esc over an open overlay       m.overlay cleared                  overlay dismissed record              the following quit key
//	state entry            searchResult processed           lifecycle state entered            state <name> record                   state-valid keys
//	load applied           loadResult admitted              source/failure recorded            load ok|fail <path> record            reveal/content waits
//	layout installed       layoutResult admitted            row model installed                layout <path> record                  reveal waits
//	message processed      any Update message               Update returned                    msg <type> record                     message-boundary waits
//	reap                   child wait status observed       wait status recorded               VRG_TEST_REAP file                    status assertion
//	quit                   tea.Quit command returned        process exited                     waitExit status                       post-exit assertions
//	rendered content       frame flushed to the PTY         marker bytes in the stream         waitOutput condition poll             content assertions
//
// Bounded condition polls (waitBounded/pollBounded, waitFile,
// waitOutput, waitCollectAck, waitPidGone, waitReplayedInputRestored)
// are the permitted wait mechanism: each iteration re-checks an
// explicit condition and every wait carries a failure timeout. The
// only pacing sleep in the package lives inside pollBounded.

// ackRecord is one parsed line of the VRG_TEST_ACK stream: the
// per-process monotonic sequence number, the event kind, and the
// kind's single-line detail.
type ackRecord struct {
	seq    uint64
	kind   string
	detail string
}

// readAcks parses the acknowledgement stream at path; a missing or
// empty file yields no records. A malformed line is an emitter bug, so
// it is an error rather than a skipped record.
func readAcks(path string) ([]ackRecord, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, nil
	}
	var out []ackRecord
	for _, line := range strings.Split(string(b), "\n") {
		if line == "" {
			continue
		}
		seqText, rest, ok := strings.Cut(line, " ")
		if !ok {
			return nil, fmt.Errorf("malformed acknowledgement record %q", line)
		}
		seq, err := strconv.ParseUint(seqText, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("malformed acknowledgement record %q: %v", line, err)
		}
		kind, detail, _ := strings.Cut(rest, " ")
		out = append(out, ackRecord{seq: seq, kind: kind, detail: detail})
	}
	return out, nil
}

// ackTail renders the newest records for failure messages, so a missed
// handshake reports what the application actually acknowledged.
func ackTail(recs []ackRecord) string {
	if len(recs) == 0 {
		return "(no records)"
	}
	if len(recs) > 6 {
		recs = recs[len(recs)-6:]
	}
	var b strings.Builder
	for _, r := range recs {
		fmt.Fprintf(&b, " %d:%s:%s", r.seq, r.kind, r.detail)
	}
	return b.String()
}

// ackMark returns the stream's current high-water sequence number — 0
// when nothing has been acknowledged yet. A later waitAck against the
// mark can only be satisfied by records the following action caused.
func (r *ptyRun) ackMark(t *testing.T) uint64 {
	t.Helper()
	if r.ackPath == "" {
		t.Fatal("ackMark: no VRG_TEST_ACK stream armed for this run")
	}
	recs, err := readAcks(r.ackPath)
	if err != nil {
		t.Fatalf("ackMark: %v", err)
	}
	var max uint64
	for _, rec := range recs {
		if rec.seq > max {
			max = rec.seq
		}
	}
	return max
}

// pollAck waits until the acknowledgement stream holds a record of the
// given kind — and a detail with the given prefix when detail is
// non-empty — with a sequence number beyond since. It returns a
// descriptive timeout error rather than hanging; waitAck turns it into
// a test failure.
func (r *ptyRun) pollAck(since uint64, kind, detail string, timeout time.Duration) error {
	var recs []ackRecord
	var readErr error
	what := "ack " + kind
	if detail != "" {
		what += " " + strconv.Quote(detail)
	}
	what += " beyond seq " + strconv.FormatUint(since, 10)
	err := pollBounded(what, timeout, func() bool {
		recs, readErr = readAcks(r.ackPath)
		if readErr != nil {
			return true
		}
		for _, rec := range recs {
			if rec.seq > since && rec.kind == kind && strings.HasPrefix(rec.detail, detail) {
				return true
			}
		}
		return false
	})
	if readErr != nil {
		return fmt.Errorf("acknowledgement stream %s: %v", r.ackPath, readErr)
	}
	if err != nil {
		return fmt.Errorf("%w; stream tail:%s", err, ackTail(recs))
	}
	return nil
}

// waitAck blocks until the application acknowledges the named event —
// detail matched as a prefix — beyond the since mark, failing on a
// bounded timeout with the stream's tail for diagnosis.
func (r *ptyRun) waitAck(t *testing.T, since uint64, kind, detail string) {
	t.Helper()
	if r.ackPath == "" {
		t.Fatal("waitAck: no VRG_TEST_ACK stream armed for this run")
	}
	if err := r.pollAck(since, kind, detail, 15*time.Second); err != nil {
		t.Fatal(err)
	}
}

// sendAcked writes keys to the terminal input and blocks until the
// application acknowledges processing the key press — the key record
// must postdate the pre-send mark, so a stale same-key record cannot
// satisfy the wait. An empty want accepts any key name. It returns the
// pre-send mark so the caller can wait on the transition records the
// key committed — an overlay dismissed, a state entered.
func (r *ptyRun) sendAcked(t *testing.T, keys, want string) uint64 {
	t.Helper()
	mark := r.ackMark(t)
	r.send(t, keys)
	r.waitAck(t, mark, "key", want)
	return mark
}

// waitCollectAckN waits until the VRG_TEST_COLLECT_ACK side channel
// holds at least n occurrences of marker — the nth occurrence
// acknowledges the nth same-kind collection, so a duplicated
// diagnostic's earlier occurrence cannot satisfy a later wait.
func waitCollectAckN(t *testing.T, path, marker string, n int) {
	t.Helper()
	waitBounded(t, fmt.Sprintf("collect ack %s x%d", strconv.Quote(marker), n),
		15*time.Second, func() bool {
			b, err := os.ReadFile(path)
			return err == nil && strings.Count(string(b), marker) >= n
		})
}

// TestHandshakeAcknowledgementStream is the matrix's live proof: one
// PTY run whose every waited transition is matched by an application
// acknowledgement — state entered, load applied, layout installed,
// overlay opened, each key processed, overlay dismissed — with the
// dismissal record causally ordered between the two q key records.
func TestHandshakeAcknowledgementStream(t *testing.T) {
	dir := t.TempDir()
	ready := filepath.Join(dir, "ready")
	pidFile := filepath.Join(dir, "pid")
	events := filepath.Join(dir, "events")
	var content strings.Builder
	for i := 1; i <= 20; i++ {
		fmt.Fprintf(&content, "hit %02d\n", i)
	}
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte(content.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	rgDir := fakeRgPath(t, `#!/bin/sh
echo $$ > "$VRG_TEST_PID"
printf '%s\n' '{"type":"begin","data":{"path":{"text":"file.txt"}}}'
printf '%s\n' '{"type":"match","data":{"path":{"text":"file.txt"},"lines":{"text":"hit 01\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}'
printf '%s\n' '{"type":"end","data":{"path":{"text":"file.txt"},"binary_offset":null}}'
printf '%s\n' '{"type":"summary","data":{}}'
printf '%s\n' 'warn one' >&2
: > "$VRG_TEST_READY"
exit 0
`)
	r := startVrgPTY(t, dir, childEnv(map[string]string{
		"PATH":           rgDir + ":" + os.Getenv("PATH"),
		"TERM":           "xterm-256color",
		"VRG_TEST_READY": ready,
		"VRG_TEST_PID":   pidFile,
		"VRG_TEST_ACK":   events,
	}), true, "hit", ".")
	waitFile(t, ready)
	killPidOnCleanup(t, pidFile)

	// State entry, the first file's load, its layout install, and the
	// warning overlay's open are each acknowledged — occurrence 1, so
	// the stream start is the mark.
	r.waitAck(t, 0, "state", "browse")
	r.waitAck(t, 0, "load", "ok file.txt")
	r.waitAck(t, 0, "layout", "file.txt")
	r.waitAck(t, 0, "overlay", "open")
	r.waitOutput(t, "warn one") // the warning overlay is rendered

	// The dismissal must be acknowledged before the following quit
	// key — the matrix's overlay-dismissal-before-quit row.
	mark := r.sendAcked(t, "q", "q")
	r.waitAck(t, mark, "overlay", "dismissed")
	r.waitOutput(t, "hit 10") // the covered row is revealed
	r.sendAcked(t, "q", "q")  // quits browse
	code := r.waitExit(t)
	r.finish(t)

	if code != 0 {
		t.Fatalf("exit status = %d, want 0", code)
	}
	recs, err := readAcks(events)
	if err != nil {
		t.Fatal(err)
	}
	// Sequence numbers are strictly increasing — per-process,
	// per-occurrence correlation — and every processed message left a
	// msg record.
	msgs := 0
	var keyQ, dismissed []uint64
	for i, rec := range recs {
		if i > 0 && rec.seq <= recs[i-1].seq {
			t.Fatalf("acknowledgement sequences not monotonic: %+v around %+v", recs[i-1], rec)
		}
		switch {
		case rec.kind == "msg":
			msgs++
		case rec.kind == "key" && rec.detail == "q":
			keyQ = append(keyQ, rec.seq)
		case rec.kind == "overlay" && rec.detail == "dismissed":
			dismissed = append(dismissed, rec.seq)
		}
	}
	if msgs == 0 {
		t.Fatal("no per-message acknowledgement records in the stream")
	}
	if len(keyQ) != 2 || len(dismissed) != 1 || !(keyQ[0] < dismissed[0] && dismissed[0] < keyQ[1]) {
		t.Fatalf("want seq(key q) < seq(overlay dismissed) < seq(key q), got keys %v dismissed %v; tail:%s",
			keyQ, dismissed, ackTail(recs))
	}
	assertTerminalRestored(t, r)
}

// TestOverlayDismissalAckedBeforeQuitKey is the dedicated regression
// for the matrix's dismissal row: over the fatal outcome's error
// overlay dismissal is the exit itself, but over a warning overlay a q
// only dismisses — the run must not end until the dismissal is
// acknowledged and a second q is processed.
func TestOverlayDismissalAckedBeforeQuitKey(t *testing.T) {
	dir := t.TempDir()
	ready := filepath.Join(dir, "ready")
	pidFile := filepath.Join(dir, "pid")
	events := filepath.Join(dir, "events")
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("hit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rgDir := fakeRgPath(t, `#!/bin/sh
echo $$ > "$VRG_TEST_PID"
printf '%s\n' '{"type":"begin","data":{"path":{"text":"file.txt"}}}'
printf '%s\n' '{"type":"match","data":{"path":{"text":"file.txt"},"lines":{"text":"hit\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}'
printf '%s\n' '{"type":"end","data":{"path":{"text":"file.txt"},"binary_offset":null}}'
printf '%s\n' '{"type":"summary","data":{}}'
printf '%s\n' 'warn one' >&2
: > "$VRG_TEST_READY"
exit 0
`)
	r := startVrgPTY(t, dir, childEnv(map[string]string{
		"PATH":           rgDir + ":" + os.Getenv("PATH"),
		"TERM":           "xterm-256color",
		"VRG_TEST_READY": ready,
		"VRG_TEST_PID":   pidFile,
		"VRG_TEST_ACK":   events,
	}), false, "hit", ".")
	waitFile(t, ready)
	killPidOnCleanup(t, pidFile)

	r.waitAck(t, 0, "overlay", "open")
	mark := r.sendAcked(t, "q", "q")
	r.waitAck(t, mark, "overlay", "dismissed")
	select {
	case err := <-r.done:
		t.Fatalf("vrg exited (%v) after the overlay-dismissing q — the quit ran before dismissal was acknowledged", err)
	default:
	}
	r.sendAcked(t, "q", "q")
	code := r.waitExit(t)
	r.finish(t)

	if code != 0 {
		t.Fatalf("exit status = %d, want 0", code)
	}
	assertTerminalRestored(t, r)
}

// TestRepeatedSameKindAcksAreDistinctOccurrences is the per-occurrence
// correlation regression: two identical key sends produce two distinct
// key records, and the wait for the second cannot be satisfied by the
// first because it names a mark beyond it.
func TestRepeatedSameKindAcksAreDistinctOccurrences(t *testing.T) {
	dir := t.TempDir()
	ready := filepath.Join(dir, "ready")
	pidFile := filepath.Join(dir, "pid")
	events := filepath.Join(dir, "events")
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("hit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rgDir := fakeRgPath(t, `#!/bin/sh
echo $$ > "$VRG_TEST_PID"
printf '%s\n' '{"type":"begin","data":{"path":{"text":"file.txt"}}}'
printf '%s\n' '{"type":"match","data":{"path":{"text":"file.txt"},"lines":{"text":"hit\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}'
printf '%s\n' '{"type":"end","data":{"path":{"text":"file.txt"},"binary_offset":null}}'
printf '%s\n' '{"type":"summary","data":{}}'
: > "$VRG_TEST_READY"
exit 0
`)
	r := startVrgPTY(t, dir, childEnv(map[string]string{
		"PATH":           rgDir + ":" + os.Getenv("PATH"),
		"TERM":           "xterm-256color",
		"VRG_TEST_READY": ready,
		"VRG_TEST_PID":   pidFile,
		"VRG_TEST_ACK":   events,
	}), false, "hit", ".")
	waitFile(t, ready)
	killPidOnCleanup(t, pidFile)
	r.waitAck(t, 0, "state", "browse")

	// w toggles wrap mode — an observable, repeatable key. The second
	// sendAcked marks the stream after the first key's record, so its
	// wait can only be satisfied by the second occurrence.
	r.sendAcked(t, "w", "w")
	var first uint64
	for _, rec := range readAcksMust(t, events) {
		if rec.kind == "key" && rec.detail == "w" {
			first = rec.seq
		}
	}
	mark := r.ackMark(t)
	if mark <= first {
		t.Fatalf("mark %d does not exceed the first key w record %d", mark, first)
	}
	r.sendAcked(t, "w", "w")

	var seqs []uint64
	for _, rec := range readAcksMust(t, events) {
		if rec.kind == "key" && rec.detail == "w" {
			seqs = append(seqs, rec.seq)
		}
	}
	if len(seqs) != 2 || seqs[0] >= seqs[1] {
		t.Fatalf("want two increasing key w records, got %v", seqs)
	}
	r.sendAcked(t, "q", "q")
	if code := r.waitExit(t); code != 0 {
		t.Fatalf("exit status = %d, want 0", code)
	}
	r.finish(t)
	assertTerminalRestored(t, r)
}

// readAcksMust parses the acknowledgement stream, failing on malformed
// records.
func readAcksMust(t *testing.T, path string) []ackRecord {
	t.Helper()
	recs, err := readAcks(path)
	if err != nil {
		t.Fatal(err)
	}
	return recs
}

// TestCollectAckPerOccurrence waits on the second occurrence of a
// duplicated diagnostic — the collection acknowledgement is
// occurrence-correlated, so the first identical line cannot satisfy
// the wait for the second.
func TestCollectAckPerOccurrence(t *testing.T) {
	dir := t.TempDir()
	ready := filepath.Join(dir, "ready")
	pidFile := filepath.Join(dir, "pid")
	ack := filepath.Join(dir, "collect-ack")
	events := filepath.Join(dir, "events")
	rgDir := fakeRgPath(t, `#!/bin/sh
echo $$ > "$VRG_TEST_PID"
printf '%s\n' 'warn dup' >&2
printf '%s\n' 'warn dup' >&2
: > "$VRG_TEST_READY"
exec sleep 100000
`)
	r := startVrgPTY(t, dir, childEnv(map[string]string{
		"PATH":                 rgDir + ":" + os.Getenv("PATH"),
		"TERM":                 "xterm-256color",
		"VRG_TEST_READY":       ready,
		"VRG_TEST_PID":         pidFile,
		"VRG_TEST_COLLECT_ACK": ack,
		"VRG_TEST_ACK":         events,
	}), true, "foo", ".")
	waitFile(t, ready)
	killPidOnCleanup(t, pidFile)

	// The second collection is acknowledged only when both identical
	// lines were processed into the session collection.
	waitCollectAckN(t, ack, "warn dup", 2)
	r.sendAcked(t, "\x03", "ctrl+c")
	code := r.waitExit(t)
	r.finish(t)

	if code != 130 {
		t.Fatalf("exit status = %d, want 130", code)
	}
	if got := r.stderr.String(); got != "warn dup\nwarn dup\n" {
		t.Fatalf("stderr = %q, want both occurrences replayed in order", got)
	}
	assertTerminalRestored(t, r)
}

// TestAckWaitFailsOnBoundedTimeout pins the failure contract of a
// handshake that never arrives: the wait returns on its bound with an
// error naming the kind, detail, and mark — never a hang — and a stale
// same-kind record at or below the mark cannot satisfy it.
func TestAckWaitFailsOnBoundedTimeout(t *testing.T) {
	dir := t.TempDir()
	events := filepath.Join(dir, "events")
	if err := os.WriteFile(events, []byte("1 state searching\n2 key q\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := &ptyRun{ackPath: events}

	// The existing record satisfies a wait from the stream start.
	if err := r.pollAck(0, "key", "q", 2*time.Second); err != nil {
		t.Fatalf("pollAck rejected an existing record: %v", err)
	}
	// Beyond its own sequence it does not: the same-kind record is
	// stale relative to the mark, and no new record ever arrives.
	start := time.Now()
	err := r.pollAck(2, "key", "q", 300*time.Millisecond)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("pollAck satisfied a wait with a stale same-kind record")
	}
	if elapsed > 5*time.Second {
		t.Fatalf("pollAck exceeded its bound: %v", elapsed)
	}
	for _, want := range []string{"key", `"q"`, "beyond seq 2", "2:key:q"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("timeout error %q lacks %q — the failure must name the awaited ack", err, want)
		}
	}
	// An absent stream fails the same bounded way.
	r2 := &ptyRun{ackPath: filepath.Join(dir, "absent")}
	if err := r2.pollAck(0, "state", "browse", 300*time.Millisecond); err == nil {
		t.Fatal("pollAck on a missing stream returned nil error")
	}
}

// TestPtyHelpersUseNoSettlingSleeps is the static half of the
// contract: the only time.Sleep in cmd/vrg test helpers is the
// condition-poll pacer inside pollBounded. Every other synchronization
// is a handshake wait, a bounded channel wait, or a condition poll —
// never a fixed settling or inter-key delay.
func TestPtyHelpersUseNoSettlingSleeps(t *testing.T) {
	needle := "time." + "Sleep("
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		b, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		fn := ""
		for i, line := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(line, "func ") {
				fn = line
			}
			if !strings.Contains(line, needle) {
				continue
			}
			if !strings.HasPrefix(fn, "func pollBounded(") {
				t.Fatalf("%s:%d: time.Sleep inside %s — only pollBounded may pace a bounded condition check", file, i+1, fn)
			}
		}
	}
}

// TestKeySendsAreAcknowledged is the matrix's usage check: every PTY
// key send goes through sendAcked — the raw send remains only inside
// sendAcked itself and in the untagged-artifact boundary test, whose
// binary deliberately ignores every seam.
func TestKeySendsAreAcknowledged(t *testing.T) {
	needle := ".send(t" + ","
	files, err := filepath.Glob("*_test.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		b, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		fn := ""
		for i, line := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(line, "func ") {
				fn = line
			}
			if !strings.Contains(line, needle) {
				continue
			}
			switch {
			case file == "handshake_test.go" && strings.HasPrefix(fn, "func (r *ptyRun) sendAcked("):
			case file == "testhooks_test.go" && strings.HasPrefix(fn, "func TestUntaggedBinaryIgnoresHookManifest("):
			default:
				t.Fatalf("%s:%d: unacknowledged key send inside %s — use sendAcked", file, i+1, fn)
			}
		}
	}
}
