package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// The Issue #48 acknowledgement seam's harness side: the vrg_testhooks
// binary writes one "<seq> <kind> [<detail>]" record per awaited model
// transition to the file VRG_TEST_EVENT_ACK names, with a per-session
// monotonic sequence, and every covered helper waits on the n-th
// matching record — an earlier same-kind record can never satisfy a
// later wait. The helpers here are portable so the untagged boundary
// tests share them; the PTY-specific contract tests live in
// handshake_test.go.

// ackRec is one acknowledgement record read from the event file.
type ackRec struct {
	seq    int
	kind   string
	detail string
}

// ackMatches reports whether rec satisfies the wanted kind and detail.
// An empty detail matches any; otherwise the record's detail must equal
// the wanted text or carry it as a prefix word. "|" separates
// alternatives, so the matrix can name a set (e.g. "browse|fatal").
func ackMatches(rec ackRec, kind, detail string) bool {
	if rec.kind != kind {
		return false
	}
	for _, alt := range strings.Split(detail, "|") {
		if alt == "" || rec.detail == alt || strings.HasPrefix(rec.detail, alt+" ") {
			return true
		}
	}
	return false
}

// readAcks parses the acknowledgement file at path. A missing file
// reads as zero records — the seam may simply not have produced one
// yet. A trailing line without a newline is an in-flight write and is
// dropped; a malformed complete line is a real protocol error.
func readAcks(path string) ([]ackRec, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	s := string(b)
	if s != "" && !strings.HasSuffix(s, "\n") {
		s = s[:strings.LastIndexByte(s, '\n')+1]
	}
	var recs []ackRec
	for _, line := range strings.Split(strings.TrimSuffix(s, "\n"), "\n") {
		if line == "" {
			continue
		}
		fields := strings.SplitN(line, " ", 3)
		seq, err := strconv.Atoi(fields[0])
		if err != nil || len(fields) < 2 {
			return nil, fmt.Errorf("malformed acknowledgement record %q", line)
		}
		rec := ackRec{seq: seq, kind: fields[1]}
		if len(fields) == 3 {
			rec.detail = fields[2]
		}
		recs = append(recs, rec)
	}
	return recs, nil
}

// countAcks returns how many records satisfy kind and detail.
func countAcks(recs []ackRec, kind, detail string) int {
	n := 0
	for _, r := range recs {
		if ackMatches(r, kind, detail) {
			n++
		}
	}
	return n
}

// ackSeen renders the observed records compactly for failure messages.
func ackSeen(recs []ackRec) string {
	var sb strings.Builder
	for _, r := range recs {
		fmt.Fprintf(&sb, "\n  %d %s %s", r.seq, r.kind, r.detail)
	}
	return sb.String()
}

// pollAck is the bounded condition poll behind awaitAck: it re-reads
// the acknowledgement file until the n-th record matching kind and
// detail has landed, returning it. On timeout the error names the
// wanted record and lists everything the file did carry — a bounded
// failure, never a hang.
func pollAck(path string, n int, kind, detail string, timeout time.Duration) (ackRec, error) {
	deadline := time.Now().Add(timeout)
	var recs []ackRec
	for time.Now().Before(deadline) {
		var err error
		recs, err = readAcks(path)
		if err != nil {
			return ackRec{}, err
		}
		if m := matchingAcks(recs, kind, detail); len(m) >= n {
			return m[n-1], nil
		}
		time.Sleep(5 * time.Millisecond)
	}
	return ackRec{}, fmt.Errorf("timed out after %s waiting for acknowledgement #%d %q %q; observed:%s",
		timeout, n, kind, detail, ackSeen(recs))
}

func matchingAcks(recs []ackRec, kind, detail string) []ackRec {
	var out []ackRec
	for _, r := range recs {
		if ackMatches(r, kind, detail) {
			out = append(out, r)
		}
	}
	return out
}

// assertAcksMonotonic requires strictly increasing sequence numbers —
// the per-occurrence correlation guarantee that keeps an earlier
// same-kind record from ever satisfying a later wait.
func assertAcksMonotonic(t *testing.T, recs []ackRec) {
	t.Helper()
	for i := 1; i < len(recs); i++ {
		if recs[i].seq <= recs[i-1].seq {
			t.Fatalf("acknowledgement sequence is not monotonic: %v then %v", recs[i-1], recs[i])
		}
	}
}

// awaitAck blocks until the n-th acknowledgement record matching kind
// and detail has landed in the session's event file — the handshake a
// key send or assumed transition must wait on.
func awaitAck(t *testing.T, path string, n int, kind, detail string) ackRec {
	t.Helper()
	rec, err := pollAck(path, n, kind, detail, 15*time.Second)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return rec
}

// ackStep is one scripted interaction with the running TUI, gated on
// an acknowledgement: the keys are written only once the n-th event
// record matching kind and detail has landed — and, when marker is
// non-empty, once that marker has also been observed in the captured
// stdout, so output-content assertions stay exact.
type ackStep struct {
	n      int
	kind   string
	detail string
	marker string
	keys   string
}

// runAckSteps runs the binary at bin driving ack-gated steps: each
// step waits for its acknowledgement record (the application-side
// handshake) and its optional stdout marker before writing its keys.
// The context deadline bounds the run; the pending step is reported on
// timeout so a wedged session fails rather than hangs.
func runAckSteps(t *testing.T, bin, workdir string, env []string, events string, steps []ackStep, args ...string) runResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = workdir
	if env != nil {
		cmd.Env = env
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	// The reader drains stdout continuously into a shared buffer so the
	// child never blocks on a full pipe while the stepper waits on
	// acknowledgements — and so the stepper can observe stdout markers.
	var mu sync.Mutex
	var buf bytes.Buffer
	outCh := make(chan string, 1)
	go func() {
		tmp := make([]byte, 8192)
		for {
			n, rerr := stdout.Read(tmp)
			if n > 0 {
				mu.Lock()
				buf.Write(tmp[:n])
				mu.Unlock()
			}
			if rerr != nil {
				break
			}
		}
		mu.Lock()
		out := buf.String()
		mu.Unlock()
		outCh <- out
	}()
	stdoutHas := func(marker string) bool {
		mu.Lock()
		defer mu.Unlock()
		return strings.Contains(buf.String(), marker)
	}

	// The stepper writes keys only after each step's acknowledgement
	// (and optional marker) is observed. A step that never arrives
	// leaves the run to the context deadline, which reports it.
	var pending atomic.Int64
	go func() {
		defer stdin.Close()
		for i, st := range steps {
			pending.Store(int64(i))
			deadline := time.Now().Add(18 * time.Second)
			for {
				recs, _ := readAcks(events)
				acked := len(matchingAcks(recs, st.kind, st.detail)) >= st.n
				marked := st.marker == "" || stdoutHas(st.marker)
				if acked && marked {
					break
				}
				if ctx.Err() != nil || time.Now().After(deadline) {
					return
				}
				time.Sleep(5 * time.Millisecond)
			}
			if _, err := stdin.Write([]byte(st.keys)); err != nil {
				return
			}
		}
		pending.Store(int64(len(steps)))
	}()

	werr := cmd.Wait()
	out := <-outCh
	if ctx.Err() == context.DeadlineExceeded {
		var st any = "startup"
		if i := pending.Load(); i < int64(len(steps)) {
			st = steps[i]
		}
		recs, _ := readAcks(events)
		t.Fatalf("vrg %v timed out awaiting step %v; acknowledgements:%s (stdout %q, stderr %q)",
			args, st, ackSeen(recs), out, stderr.String())
	}
	res := runResult{stdout: out, stderr: stderr.String()}
	if werr == nil {
		res.code = 0
		return res
	}
	if ee, ok := werr.(*exec.ExitError); ok {
		res.code = ee.ExitCode()
		return res
	}
	t.Fatalf("failed to run %v: %v", args, werr)
	return res
}
