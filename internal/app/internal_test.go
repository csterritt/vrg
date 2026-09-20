package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/searchindex"
)

// fakeChild is a Child whose piped streams are preloaded; Wait records
// that the child was reaped and Terminate records that it was killed, so
// tests can prove the exit and cleanup paths ran.
type fakeChild struct {
	stdout     io.Reader
	stderr     io.Reader
	waitErr    error
	waited     chan struct{}
	terminated chan struct{}
	waitOnce   sync.Once
	termOnce   sync.Once
}

func (f *fakeChild) Stdout() io.Reader { return f.stdout }
func (f *fakeChild) Stderr() io.Reader { return f.stderr }
func (f *fakeChild) Wait() error {
	f.waitOnce.Do(func() {
		if f.waited != nil {
			close(f.waited)
		}
	})
	return f.waitErr
}

func (f *fakeChild) Terminate() {
	f.termOnce.Do(func() {
		if f.terminated != nil {
			close(f.terminated)
		}
	})
}

// requireClosed fails unless ch is already closed.
func requireClosed(t *testing.T, ch chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	default:
		t.Fatalf("%s did not happen", what)
	}
}

const validStream = `{"type":"begin","data":{"path":{"text":"a.txt"}}}` + "\n" +
	`{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"hit\n"},` +
	`"line_number":1,"absolute_offset":0,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}` + "\n" +
	`{"type":"end","data":{"path":{"text":"a.txt"},"binary_offset":null}}` + "\n" +
	`{"type":"summary","data":{"stats":{}}}` + "\n"

// fixtureIndex builds a finished index with the given number of files,
// each with the given number of matched lines.
func fixtureIndex(t *testing.T, files, linesPerFile int) *searchindex.Index {
	t.Helper()
	idx := searchindex.New("/w")
	for f := 0; f < files; f++ {
		path := fmt.Sprintf("%c.txt", 'a'+f)
		for l := 1; l <= linesPerFile; l++ {
			rec := fmt.Sprintf(`{"type":"match","data":{"path":{"text":%q},`+
				`"lines":{"text":"hit\n"},"line_number":%d,`+
				`"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}`, path, l)
			if err := idx.Add([]byte(rec)); err != nil {
				t.Fatalf("fixture Add: %v", err)
			}
		}
	}
	idx.Finish()
	return idx
}

func keyMsg(s string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: rune(s[0]), Text: s}
}

// The searching screen is shown while results are collected.
func TestSearchingScreenShown(t *testing.T) {
	m := New(&fakeChild{stdout: strings.NewReader(""), stderr: strings.NewReader("")}, "/w")
	if got := m.View().Content; !strings.Contains(got, "Searching…") {
		t.Fatalf("searching View = %q, want it to contain %q", got, "Searching…")
	}
}

// "Searching…" covers collection and post-exit processing: with index
// preparation held by a gate, the app stays in the searching state even
// after rg has exited.
func TestSearchingCoversPostExitPreparation(t *testing.T) {
	child := &fakeChild{
		stdout: strings.NewReader(validStream),
		stderr: strings.NewReader(""),
		waited: make(chan struct{}),
	}
	m := New(child, "/w")
	gate := make(chan struct{})
	m.gate = gate

	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init() returned no collection command")
	}
	msgs := make(chan tea.Msg, 1)
	go func() { msgs <- cmd() }()

	<-child.waited // the child has exited; preparation is still gated.

	select {
	case <-msgs:
		t.Fatal("collection delivered its result while index preparation was gated")
	default:
	}
	if got := m.View().Content; !strings.Contains(got, "Searching…") {
		t.Fatalf("View after child exit = %q, want it to still show %q", got, "Searching…")
	}

	close(gate)
	var msg tea.Msg
	select {
	case msg = <-msgs:
	case <-time.After(10 * time.Second):
		t.Fatal("collection did not deliver after the gate was released")
	}
	m2, _ := m.Update(msg)
	got := m2.(Model).View().Content
	if !strings.Contains(got, "a.txt") || !strings.Contains(got, "Loading…") {
		t.Fatalf("browse View = %q, want it to list a.txt with a loading placeholder", got)
	}
}

// An injected search-completion message transitions the model to the
// browse view: the file list appears and the current file loads.
func TestCompletionTransitionsToBrowse(t *testing.T) {
	m := New(&fakeChild{stdout: strings.NewReader(""), stderr: strings.NewReader("")}, "/w")
	m2, _ := m.Update(searchResult{index: fixtureIndex(t, 2, 3), integrity: completeStream})
	got := m2.(Model).View().Content
	if !strings.Contains(got, "a.txt") || !strings.Contains(got, "b.txt") {
		t.Fatalf("browse View = %q, want it to list a.txt and b.txt", got)
	}
	if strings.Contains(got, "Searching") {
		t.Fatalf("browse View still shows searching: %q", got)
	}
}

// q in the browse view quits with exit status 0.
func TestBrowseQuitExitsZero(t *testing.T) {
	m := New(&fakeChild{stdout: strings.NewReader(""), stderr: strings.NewReader("")}, "/w")
	m2, _ := m.Update(searchResult{index: fixtureIndex(t, 1, 1), integrity: completeStream})
	m3, cmd := m2.(Model).Update(keyMsg("q"))
	if cmd == nil {
		t.Fatal("q in the browse view returned no command, want tea.Quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("q in the browse view returned %T, want tea.QuitMsg", cmd())
	}
	if s := m3.(Model).status; s != 0 {
		t.Fatalf("exit status = %d, want 0", s)
	}
}

// A resize while searching is handled without blocking on collection.
func TestResizeWhileSearching(t *testing.T) {
	m := New(&fakeChild{stdout: strings.NewReader(""), stderr: strings.NewReader("")}, "/w")
	m2, cmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	if cmd != nil {
		t.Fatalf("resize returned a command: %v", cmd)
	}
	mm := m2.(Model)
	if mm.width != 100 || mm.height != 40 {
		t.Fatalf("size = %dx%d, want 100x40", mm.width, mm.height)
	}
	if got := mm.View().Content; !strings.Contains(got, "Searching…") {
		t.Fatalf("View after resize = %q, want it to still show %q", got, "Searching…")
	}
}

// A child that cannot start produces a sanitized stderr diagnostic and
// exit 2 without the TUI ever running.
func TestStartFailureExitsTwoWithoutTUI(t *testing.T) {
	var stderr bytes.Buffer
	programRan := false
	code := Run([]string{"--json", "--no-config", "--", "foo", "."}, Env{
		Start: func(argv []string, workdir string) (Child, error) {
			return nil, errors.New(`exec: "rg": executable file not found in $PATH`)
		},
		Stderr: &stderr,
		Program: func(Model) (tea.Model, error) {
			programRan = true
			return nil, nil
		},
	})
	if code != 2 {
		t.Fatalf("Run exit = %d, want 2", code)
	}
	if programRan {
		t.Fatal("the TUI program ran after a start failure")
	}
	diag := strings.TrimSuffix(stderr.String(), "\n")
	if !strings.HasPrefix(diag, "vrg: ") || !strings.Contains(diag, "cannot start rg") {
		t.Fatalf("stderr = %q, want a vrg: diagnostic naming the rg start failure", stderr.String())
	}
	if strings.Contains(stderr.String(), "Usage:") {
		t.Fatalf("start failure emitted usage help: %q", stderr.String())
	}
	if strings.ContainsAny(diag, "\x1b\x9b") || strings.Contains(diag, "\n") {
		t.Fatalf("diagnostic is not a sanitized single line: %q", stderr.String())
	}
}

// Hostile bytes in the start error are escaped, never emitted raw.
func TestStartFailureDiagnosticIsSanitized(t *testing.T) {
	var stderr bytes.Buffer
	code := Run([]string{"--json", "--no-config", "--", "foo", "."}, Env{
		Start: func(argv []string, workdir string) (Child, error) {
			return nil, errors.New("bad \x1b[31m error\nhere")
		},
		Stderr: &stderr,
	})
	if code != 2 {
		t.Fatalf("Run exit = %d, want 2", code)
	}
	if strings.Contains(stderr.String(), "\x1b") || strings.Count(stderr.String(), "\n") != 1 {
		t.Fatalf("hostile error reached stderr unsanitized: %q", stderr.String())
	}
}

// The production path through collect: a real started child's stream is
// indexed and both pipes are drained.
func TestCollectIndexesStream(t *testing.T) {
	child := &fakeChild{
		stdout: strings.NewReader(validStream),
		stderr: strings.NewReader("warning text\n"),
	}
	res := collect(context.Background(), child, "/w", nil)
	if res.err != nil {
		t.Fatalf("collect err = %v", res.err)
	}
	stops := res.index.Stops()
	if len(stops) != 1 || string(stops[0].Path) != "a.txt" || stops[0].Line != 1 {
		t.Fatalf("index stops = %+v, want a.txt:1", stops)
	}
	if string(res.stderr) != "warning text\n" {
		t.Fatalf("stderr = %q, want %q", res.stderr, "warning text\n")
	}
}

// q while searching is cancellation: the child is terminated, the exit
// status is 130, and the program quits without reaching the summary.
func TestQWhileSearchingCancels(t *testing.T) {
	child := &fakeChild{
		stdout:     strings.NewReader(""),
		stderr:     strings.NewReader(""),
		terminated: make(chan struct{}),
	}
	m := New(child, "/w")
	m2, cmd := m.Update(keyMsg("q"))
	if cmd == nil {
		t.Fatal("q while searching returned no command, want tea.Quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("q while searching returned %T, want tea.QuitMsg", cmd())
	}
	mm := m2.(Model)
	if mm.status != 130 {
		t.Fatalf("exit status = %d, want 130", mm.status)
	}
	requireClosed(t, child.terminated, "child termination")
	if got := mm.View().Content; strings.Contains(got, "a.txt") {
		t.Fatalf("cancellation produced a browse screen: %q", got)
	}
}

// ctrl+c is cancellation in every state: while searching and while
// browsing it terminates the child and exits 130.
func TestCtrlCCancelsFromAnyState(t *testing.T) {
	for _, tc := range []struct {
		name   string
		browse bool
	}{
		{"searching", false},
		{"browsing", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			child := &fakeChild{
				stdout:     strings.NewReader(""),
				stderr:     strings.NewReader(""),
				terminated: make(chan struct{}),
			}
			m := New(child, "/w")
			if tc.browse {
				mi, _ := m.Update(searchResult{index: fixtureIndex(t, 1, 1), integrity: completeStream})
				m = mi.(Model)
			}
			m2, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
			if cmd == nil {
				t.Fatal("ctrl+c returned no command, want tea.Quit")
			}
			if _, ok := cmd().(tea.QuitMsg); !ok {
				t.Fatalf("ctrl+c returned %T, want tea.QuitMsg", cmd())
			}
			if s := m2.(Model).status; s != 130 {
				t.Fatalf("exit status = %d, want 130", s)
			}
			requireClosed(t, child.terminated, "child termination")
		})
	}
}

// q while rg has exited but index preparation is still gate-held is
// cancellation (130), not a browse quit; cancelling releases the held
// collection promptly and a late completion must not revive the UI.
func TestQDuringGateHeldPreparationCancels(t *testing.T) {
	child := &fakeChild{
		stdout:     strings.NewReader(validStream),
		stderr:     strings.NewReader(""),
		waited:     make(chan struct{}),
		terminated: make(chan struct{}),
	}
	m := New(child, "/w")
	m.gate = make(chan struct{}) // never released

	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init() returned no collection command")
	}
	msgs := make(chan tea.Msg, 1)
	go func() { msgs <- cmd() }()

	<-child.waited // rg has exited; index preparation is still held.

	m2, quit := m.Update(keyMsg("q"))
	mm := m2.(Model)
	if quit == nil {
		t.Fatal("q during gate-held preparation returned no command, want tea.Quit")
	}
	if _, ok := quit().(tea.QuitMsg); !ok {
		t.Fatalf("q during gate-held preparation returned %T, want tea.QuitMsg", quit())
	}
	if mm.status != 130 {
		t.Fatalf("exit status = %d, want 130", mm.status)
	}
	requireClosed(t, child.terminated, "child termination")

	// Drainage and gated preparation end promptly on cancellation.
	var late tea.Msg
	select {
	case late = <-msgs:
	case <-time.After(10 * time.Second):
		t.Fatal("collection did not return promptly after cancellation")
	}

	// The late search-completion message must not revive the UI.
	m3, cmd := mm.Update(late)
	if cmd != nil {
		t.Fatalf("late completion after cancellation returned a command: %v", cmd)
	}
	mm3 := m3.(Model)
	if mm3.state != stateCancelled {
		t.Fatalf("late completion revived the UI into state %d", mm3.state)
	}
	if mm3.status != 130 {
		t.Fatalf("exit status after late completion = %d, want 130", mm3.status)
	}
}

// Esc while searching is a no-op: no state change, no command, and the
// searching screen remains.
func TestEscWhileSearchingIsNoop(t *testing.T) {
	child := &fakeChild{
		stdout:     strings.NewReader(""),
		stderr:     strings.NewReader(""),
		terminated: make(chan struct{}),
	}
	m := New(child, "/w")
	m2, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd != nil {
		t.Fatalf("Esc while searching returned a command: %v", cmd)
	}
	mm := m2.(Model)
	if mm.state != stateSearching {
		t.Fatalf("Esc while searching changed state to %d", mm.state)
	}
	if mm.status != 0 {
		t.Fatalf("Esc while searching changed status to %d", mm.status)
	}
	select {
	case <-child.terminated:
		t.Fatal("Esc while searching terminated the child")
	default:
	}
	if got := mm.View().Content; !strings.Contains(got, "Searching…") {
		t.Fatalf("View after Esc = %q, want it to still show %q", got, "Searching…")
	}
}

// A late search-completion message after cancellation is discarded: it
// produces no further screen, no status change, and no command.
func TestLateCompletionAfterCancelDiscarded(t *testing.T) {
	child := &fakeChild{
		stdout:     strings.NewReader(""),
		stderr:     strings.NewReader(""),
		terminated: make(chan struct{}),
	}
	m := New(child, "/w")
	m2, _ := m.Update(keyMsg("q"))
	mm := m2.(Model)
	m3, cmd := mm.Update(searchResult{index: fixtureIndex(t, 2, 2), integrity: completeStream})
	if cmd != nil {
		t.Fatalf("late completion after cancellation returned a command: %v", cmd)
	}
	mm3 := m3.(Model)
	if mm3.state == stateBrowse {
		t.Fatal("late completion after cancellation reached the browse state")
	}
	if mm3.status != 130 {
		t.Fatalf("exit status after late completion = %d, want 130", mm3.status)
	}
	if got := mm3.View().Content; strings.Contains(got, "a.txt") {
		t.Fatalf("late completion revived the UI: %q", got)
	}
}

// A controlled application failure after the child started terminates
// and reaps it, writes a sanitized diagnostic to stderr exactly once,
// and exits 2.
func TestControlledFailureCleansUp(t *testing.T) {
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
		Program: func(Model) (tea.Model, error) {
			return nil, errors.New("boom \x1b[31m\nsecond line")
		},
	})
	if code != 2 {
		t.Fatalf("Run exit = %d, want 2", code)
	}
	requireClosed(t, child.terminated, "child termination")
	requireClosed(t, child.waited, "child reap")
	diag := strings.TrimSuffix(stderr.String(), "\n")
	if !strings.HasPrefix(diag, "vrg: ") {
		t.Fatalf("stderr = %q, want a vrg: diagnostic", stderr.String())
	}
	if strings.Count(diag, "boom") != 1 || strings.Contains(diag, "\n") {
		t.Fatalf("diagnostic not written exactly once as a single line: %q", stderr.String())
	}
	if strings.ContainsAny(diag, "\x1b\x9b") || !strings.Contains(diag, "^[") {
		t.Fatalf("diagnostic is not sanitized: %q", stderr.String())
	}
}

// An ordinary exit while the child is still running terminates and
// reaps it; the run does not leave a live or unreaped child behind.
func TestOrdinaryExitReapsRunningChild(t *testing.T) {
	child := &fakeChild{
		stdout:     strings.NewReader(""),
		stderr:     strings.NewReader(""),
		waited:     make(chan struct{}),
		terminated: make(chan struct{}),
	}
	code := Run([]string{"--json", "--no-config", "--", "foo", "."}, Env{
		Start:   func(argv []string, workdir string) (Child, error) { return child, nil },
		Stderr:  io.Discard,
		Program: func(m Model) (tea.Model, error) { return m, nil },
	})
	if code != 0 {
		t.Fatalf("Run exit = %d, want 0", code)
	}
	requireClosed(t, child.terminated, "child termination")
	requireClosed(t, child.waited, "child reap")
}

// A program interrupted by SIGINT exits 130 and still terminates and
// reaps the child.
func TestInterruptCleansUp(t *testing.T) {
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
		Program: func(Model) (tea.Model, error) {
			return nil, tea.ErrInterrupted
		},
	})
	if code != 130 {
		t.Fatalf("Run exit = %d, want 130", code)
	}
	requireClosed(t, child.terminated, "child termination")
	requireClosed(t, child.waited, "child reap")
	if stderr.Len() != 0 {
		t.Fatalf("interrupted run wrote a diagnostic: %q", stderr.String())
	}
}
