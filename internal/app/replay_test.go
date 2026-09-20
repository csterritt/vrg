package app

import (
	"bytes"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

// feedStderr delivers raw child stderr bytes to the model the way the
// collection path does: one stderrMsg per drained line, in order,
// through Update. A diagnostic is collected when the model processes
// the message carrying it.
func feedStderr(t *testing.T, m Model, stderr string) Model {
	t.Helper()
	for _, line := range strings.SplitAfter(stderr, "\n") {
		if line == "" {
			continue
		}
		m, _ = update(t, m, stderrMsg{raw: line})
	}
	return m
}

// Three collected diagnostics — one stderr line shown in the warning
// overlay and two file-load failures never displayed — reach the
// post-restoration replay writer exactly once each, in collection
// order, on a normal exit. The collection is independent of what was
// displayed.
func TestReplayCollectsDisplayedAndUndisplayedOnceInOrder(t *testing.T) {
	child := &fakeChild{
		stdout:     strings.NewReader(""),
		stderr:     strings.NewReader(""),
		waited:     make(chan struct{}),
		terminated: make(chan struct{}),
	}
	var stderr bytes.Buffer
	var acked []string
	code := Run([]string{"--json", "--no-config", "--", "hit", "."}, Env{
		Start:  func(argv []string, workdir string) (Child, error) { return child, nil },
		Stderr: &stderr,
		OnCollect: func(line string) {
			acked = append(acked, line)
		},
		Program: func(m Model) (tea.Model, error) {
			m = feedStderr(t, m, "warn one\n")
			m, _ = update(t, m, searchResult{index: fixtureIndex(t, 2, 1), integrity: completeStream})
			if m.overlay == nil {
				t.Fatal("no warning overlay for the stderr diagnostic")
			}
			if got := m.View().Content; !strings.Contains(got, "warn one") {
				t.Fatalf("overlay does not display the stderr diagnostic:\n%s", got)
			}
			// Dismiss the warning, then record two load failures — the
			// diagnostics are collected but never displayed.
			m, _ = update(t, m, keyPress("q"))
			m, _ = update(t, m, loadResult{path: []byte("a.txt"), err: errors.New("denied")})
			m, _ = update(t, m, loadResult{path: []byte("b.txt"), err: errors.New("gone")})
			if got := m.View().Content; strings.Contains(got, "denied") || strings.Contains(got, "gone") {
				t.Fatalf("a never-displayed diagnostic reached the screen:\n%s", got)
			}
			m, cmd := update(t, m, keyPress("q"))
			requireQuit(t, cmd, "q in the browse view")
			return m, nil
		},
	})
	if code != 0 {
		t.Fatalf("exit status = %d, want 0", code)
	}
	want := []string{"warn one", "cannot read a.txt: denied", "cannot read b.txt: gone"}
	if got := strings.Split(strings.TrimSuffix(stderr.String(), "\n"), "\n"); !slices.Equal(got, want) {
		t.Fatalf("replayed stderr = %q, want %q — each diagnostic once, in collection order", got, want)
	}
	if !slices.Equal(acked, want) {
		t.Fatalf("collected diagnostics = %q, want %q", acked, want)
	}
}

// The shutdown boundary is Update processing the message carrying a
// diagnostic: a stderr diagnostic delivered and processed before the
// ctrl+c update is replayed at exit 130; one still in flight at the
// exit decision is never waited for and never replayed.
func TestDiagnosticProcessedBeforeCtrlCReplayed(t *testing.T) {
	child := &fakeChild{
		stdout:     strings.NewReader(""),
		stderr:     strings.NewReader(""),
		waited:     make(chan struct{}),
		terminated: make(chan struct{}),
	}
	var stderr bytes.Buffer
	var acked []string
	code := Run([]string{"--json", "--no-config", "--", "hit", "."}, Env{
		Start:  func(argv []string, workdir string) (Child, error) { return child, nil },
		Stderr: &stderr,
		OnCollect: func(line string) {
			acked = append(acked, line)
		},
		Program: func(m Model) (tea.Model, error) {
			m = feedStderr(t, m, "early warn\n")
			m, cmd := update(t, m, keyPress("ctrl+c"))
			requireQuit(t, cmd, "ctrl+c")
			// In flight at the exit decision: a diagnostic arriving now
			// is discarded by the cancelled model, not collected.
			m, _ = update(t, m, stderrMsg{raw: "too late\n"})
			return m, nil
		},
	})
	if code != 130 {
		t.Fatalf("exit status = %d, want 130", code)
	}
	requireClosed(t, child.terminated, "child termination")
	requireClosed(t, child.waited, "child reap")
	if got := stderr.String(); got != "early warn\n" {
		t.Fatalf("replayed stderr = %q, want only the collected diagnostic %q", got, "early warn\n")
	}
	if !slices.Equal(acked, []string{"early warn"}) {
		t.Fatalf("collected diagnostics = %q, want %q", acked, []string{"early warn"})
	}
}

// A diagnostic whose carrying message is never delivered — a load held
// behind the worker gate at cancellation — is not waited for and not
// replayed.
func TestGatedUndeliveredDiagnosticNotReplayed(t *testing.T) {
	child := &fakeChild{
		stdout:     strings.NewReader(""),
		stderr:     strings.NewReader(""),
		waited:     make(chan struct{}),
		terminated: make(chan struct{}),
	}
	gate := make(chan struct{}) // never closed; cancellation releases the worker
	late := make(chan tea.Msg, 1)
	var stderr bytes.Buffer
	var acked []string
	code := Run([]string{"--json", "--no-config", "--", "hit", "."}, Env{
		Start:  func(argv []string, workdir string) (Child, error) { return child, nil },
		Stderr: &stderr,
		OnCollect: func(line string) {
			acked = append(acked, line)
		},
		Program: func(m Model) (tea.Model, error) {
			m.loadGate = gate
			m, cmd := update(t, m, searchResult{index: fixtureIndex(t, 1, 1), integrity: completeStream})
			if cmd == nil {
				t.Fatal("browse entry started no file load")
			}
			go func() { late <- cmd() }()
			m, quit := update(t, m, keyPress("ctrl+c"))
			requireQuit(t, quit, "ctrl+c during a gate-held load")
			return m, nil
		},
	})
	if code != 130 {
		t.Fatalf("exit status = %d, want 130", code)
	}
	// Cancellation released the gate-held worker; its load result — a
	// read failure for a file that does not exist — was never delivered
	// and is never collected.
	select {
	case <-late:
	case <-time.After(10 * time.Second):
		t.Fatal("the gate-held load did not complete after cancellation")
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("stderr = %q, want empty: an undelivered diagnostic must not be replayed", got)
	}
	if len(acked) != 0 {
		t.Fatalf("collected diagnostics = %q, want none", acked)
	}
}

// q while result preparation is gate-held is cancellation: a diagnostic
// acknowledged as collected before q is replayed at exit 130, and the
// gated search result — carrying a generated integrity diagnostic — is
// never waited on and never replayed.
func TestQWhileGateHeldPreparationReplaysCollected(t *testing.T) {
	child := &fakeChild{
		stdout: strings.NewReader(`{"type":"begin","data":{"path":{"text":"a.txt"}}}` + "\n" +
			`{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"hit\n"},` +
			`"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}` + "\n" +
			`{"type":"end","data":{"path":{"text":"a.txt"},"binary_offset":null}}` + "\n"),
		stderr:     strings.NewReader("warn gate\n"),
		waited:     make(chan struct{}),
		terminated: make(chan struct{}),
	}
	gate := make(chan struct{}) // never created: preparation stays held
	var stderr bytes.Buffer
	var acked []string
	code := Run([]string{"--json", "--no-config", "--", "hit", "."}, Env{
		Start:  func(argv []string, workdir string) (Child, error) { return child, nil },
		Stderr: &stderr,
		Gate:   gate,
		OnCollect: func(line string) {
			acked = append(acked, line)
		},
		Program: func(m Model) (tea.Model, error) {
			inbox := make(chan tea.Msg, 8)
			m.diags.emit = func(msg tea.Msg) { inbox <- msg }
			go func() { inbox <- m.Init()() }()
			// The drained stderr diagnostic is delivered while searching;
			// collecting it is acknowledged by the OnCollect seam.
			msg := <-inbox
			if _, ok := msg.(stderrMsg); !ok {
				t.Fatalf("first collection message = %T, want stderrMsg", msg)
			}
			m, _ = update(t, m, msg)
			if len(acked) != 1 {
				t.Fatalf("acked = %q after the stderr message, want one line", acked)
			}
			m, quit := update(t, m, keyPress("q"))
			requireQuit(t, quit, "q during gate-held preparation")
			return m, nil
		},
	})
	if code != 130 {
		t.Fatalf("exit status = %d, want 130", code)
	}
	requireClosed(t, child.terminated, "child termination")
	requireClosed(t, child.waited, "child reap")
	// Only the collected stderr diagnostic is replayed; the gated result's
	// integrity diagnostic was never delivered and is not waited on.
	if got := stderr.String(); got != "warn gate\n" {
		t.Fatalf("replayed stderr = %q, want only %q", got, "warn gate\n")
	}
	if strings.Contains(stderr.String(), "incomplete") {
		t.Fatalf("gated result diagnostics reached the replay: %q", stderr.String())
	}
	if !slices.Equal(acked, []string{"warn gate"}) {
		t.Fatalf("collected diagnostics = %q, want %q", acked, []string{"warn gate"})
	}
}

// The controlled-failure diagnostic enters the session collection and
// is replayed by the common post-restoration writer — never written
// directly by the failure path — so it lands on stderr exactly once,
// after any earlier diagnostics, in collection order.
func TestControlledFailureReplaysThroughCollectionOnce(t *testing.T) {
	child := &fakeChild{
		stdout:     strings.NewReader(""),
		stderr:     strings.NewReader(""),
		waited:     make(chan struct{}),
		terminated: make(chan struct{}),
	}
	var stderr bytes.Buffer
	code := Run([]string{"--json", "--no-config", "--", "foo", "."}, Env{
		Start:  func(argv []string, workdir string) (Child, error) { return child, nil },
		Stderr: &stderr,
		Program: func(m Model) (tea.Model, error) {
			m = feedStderr(t, m, "early warn\n")
			return m, errors.New("boom \x1b[31m\nsecond line")
		},
	})
	if code != 2 {
		t.Fatalf("exit status = %d, want 2", code)
	}
	requireClosed(t, child.terminated, "child termination")
	requireClosed(t, child.waited, "child reap")
	got := stderr.String()
	want := "early warn\n" + `vrg: boom ^[[31m\nsecond line` + "\n"
	if got != want {
		t.Fatalf("stderr = %q, want %q: the failure diagnostic replays after earlier diagnostics, exactly once", got, want)
	}
	if n := strings.Count(got, "boom"); n != 1 {
		t.Fatalf("failure diagnostic count = %d, want exactly once across both mechanisms: %q", n, got)
	}
}

// A diagnostic embedding a filename with a newline and an ESC byte is
// escaped and single-lined through the safe-presentation utility in the
// replayed output: filename newlines cannot forge diagnostic line
// boundaries.
func TestReplayEscapesEmbeddedFilename(t *testing.T) {
	child := &fakeChild{
		stdout:     strings.NewReader(""),
		stderr:     strings.NewReader(""),
		waited:     make(chan struct{}),
		terminated: make(chan struct{}),
	}
	var stderr bytes.Buffer
	code := Run([]string{"--json", "--no-config", "--", "hit", "."}, Env{
		Start:  func(argv []string, workdir string) (Child, error) { return child, nil },
		Stderr: &stderr,
		Program: func(m Model) (tea.Model, error) {
			m, _ = update(t, m, searchResult{index: fixtureIndex(t, 1, 1), integrity: completeStream})
			m, _ = update(t, m, loadResult{
				path: []byte("evil\nname\x1b.txt"),
				err:  errors.New("denied"),
			})
			m, cmd := update(t, m, keyPress("q"))
			requireQuit(t, cmd, "q in the browse view")
			return m, nil
		},
	})
	if code != 0 {
		t.Fatalf("exit status = %d, want 0", code)
	}
	got := stderr.String()
	if !strings.Contains(got, `evil\nname^[.txt`) {
		t.Fatalf("replayed diagnostic lacks the escaped single-lined filename: %q", got)
	}
	if strings.ContainsAny(got, "\x1b\x9b") || strings.Count(strings.TrimSuffix(got, "\n"), "\n") != 0 {
		t.Fatalf("replayed diagnostic is not a sanitized single line: %q", got)
	}
}
