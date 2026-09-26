package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

// browseModel returns a model past a completed search: browse phase,
// dimensions set, and the load command for the first file in hand.
func browseModel(t *testing.T, workdir string, w, h int, recs ...string) (Model, tea.Cmd) {
	t.Helper()
	m := newModel(nil, nil)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: w, Height: h})
	m, cmd := update(t, m, searchDoneMsg{index: fixtureIndex(t, workdir, recs...)})
	return m, cmd
}

// writeWorkFile creates a file inside the search working directory.
func writeWorkFile(t *testing.T, dir, name, data string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
}

// A completed search presents the two-pane browse view: the file list on
// the left in raw-path order, the filename rule, and "Loading…" until
// the current file's load completes.
func TestCompletionTransitionsToBrowse(t *testing.T) {
	dir := t.TempDir()
	writeWorkFile(t, dir, "a.txt", "alpha\nbeta\n")
	writeWorkFile(t, dir, "b.txt", "hit\n")
	m, _ := browseModel(t, dir, 80, 24,
		`{"type":"match","data":{"path":{"text":"b.txt"},"lines":{"text":"hit\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}`,
		`{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"beta\n"},"line_number":2,"submatches":[{"match":{"text":"beta"},"start":0,"end":4}]}}`,
		`{"type":"summary","data":{}}`,
	)
	v := m.View().Content
	if !strings.Contains(v, "a.txt") || !strings.Contains(v, "b.txt") {
		t.Fatalf("browse view lacks the file list: %q", v)
	}
	if strings.Index(v, "a.txt") > strings.Index(v, "b.txt") {
		t.Fatalf("file list out of raw-path order: %q", v)
	}
	if !strings.Contains(v, "Loading…") {
		t.Fatalf("browse view lacks the loading placeholder: %q", v)
	}
	if strings.Contains(v, "Searching…") || strings.Contains(v, "matched line") {
		t.Fatalf("browse view still shows a prior screen: %q", v)
	}
}

// The load command delivered by the search-done transition carries a
// prepared buffer; after its completion message the panel renders the
// file's escaped content with the gutter and the filename rule.
func TestLoadCompletionRendersContent(t *testing.T) {
	dir := t.TempDir()
	writeWorkFile(t, dir, "a.txt", "alpha\nhit beta\n")
	m, cmd := browseModel(t, dir, 80, 24,
		`{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"hit beta\n"},"line_number":2,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}`,
		`{"type":"summary","data":{}}`,
	)
	if cmd == nil {
		t.Fatal("search completion returned no load command")
	}
	msg := cmd()
	if _, ok := msg.(loadDoneMsg); !ok {
		t.Fatalf("load command message = %T, want loadDoneMsg", msg)
	}
	m2, _ := update(t, m, msg)
	v := m2.View().Content
	if strings.Contains(v, "Loading…") {
		t.Fatalf("loaded view still shows the placeholder: %q", v)
	}
	if !strings.Contains(v, "alpha") || !strings.Contains(v, "hit") || !strings.Contains(v, " beta") {
		t.Fatalf("loaded view lacks content: %q", v)
	}
	if !strings.Contains(v, "1  ") || !strings.Contains(v, "2  ") {
		t.Fatalf("loaded view lacks the gutter: %q", v)
	}
	// The filename rule embeds the current path in a horizontal rule.
	if !strings.Contains(v, "─") || !strings.Contains(v, "a.txt") {
		t.Fatalf("loaded view lacks the filename rule: %q", v)
	}
}

// Matched spans render in inverse video over the escaped display text.
func TestMatchRendersInverse(t *testing.T) {
	dir := t.TempDir()
	writeWorkFile(t, dir, "a.txt", "a hit\n")
	m, cmd := browseModel(t, dir, 80, 24,
		`{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"a hit\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":2,"end":5}]}}`,
		`{"type":"summary","data":{}}`,
	)
	m2, _ := update(t, m, cmd())
	v := m2.View().Content
	if !strings.Contains(v, "\x1b[7mhit\x1b[27m") {
		t.Fatalf("match not rendered in inverse video: %q", v)
	}
}

// While the load worker is gate-held — read and decode/map together —
// the model still answers keys and resizes; releasing the gate delivers
// the prepared buffer and content appears.
func TestGatedLoadKeepsResponsive(t *testing.T) {
	dir := t.TempDir()
	writeWorkFile(t, dir, "a.txt", "payload\n")
	gate := make(chan struct{})
	m := newModel(nil, nil)
	m.loadGate = gate
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := update(t, m, searchDoneMsg{index: fixtureIndex(t, dir,
		`{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"payload\n"},"line_number":1,"submatches":[{"match":{"text":"payload"},"start":0,"end":7}]}}`,
		`{"type":"summary","data":{}}`,
	)})
	if cmd == nil {
		t.Fatal("search completion returned no load command")
	}

	// The worker holds read and decode/map behind the gate.
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	select {
	case msg := <-done:
		t.Fatalf("load completed while the gate was held: %v", msg)
	case <-time.After(50 * time.Millisecond):
	}

	// Resize and keys are handled while the load is parked; the view
	// still shows the placeholder.
	m2, _ := update(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	if m2.width != 100 || m2.height != 30 {
		t.Fatalf("resize during gated load lost: %dx%d", m2.width, m2.height)
	}
	m3, c3 := update(t, m2, keyPress("x"))
	if c3 != nil {
		t.Fatalf("ordinary key during load produced a command %T", c3)
	}
	if v := m3.View().Content; !strings.Contains(v, "Loading…") {
		t.Fatalf("gated-load view lacks placeholder: %q", v)
	}

	close(gate)
	select {
	case msg := <-done:
		m4, _ := update(t, m3, msg)
		if v := m4.View().Content; !strings.Contains(v, "payload") {
			t.Fatalf("view after gate release lacks content: %q", v)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("gated load did not complete after release")
	}
}

// ctrl+c while the load worker is gate-held exits 130 through the Issue
// 4 cancellation path without waiting on the gate.
func TestCtrlCWhileLoadGateHeld(t *testing.T) {
	dir := t.TempDir()
	writeWorkFile(t, dir, "a.txt", "x\n")
	gate := make(chan struct{})
	cancelled := false
	m := newModel(nil, func() { cancelled = true })
	m.loadGate = gate
	m, cmd := update(t, m, searchDoneMsg{index: fixtureIndex(t, dir,
		`{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"x\n"},"line_number":1,"submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}`,
		`{"type":"summary","data":{}}`,
	)})
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	select {
	case <-done:
		t.Fatal("load completed while the gate was held")
	case <-time.After(50 * time.Millisecond):
	}
	m2, qc := update(t, m, ctrlCPress())
	if _, ok := qc().(tea.QuitMsg); !ok {
		t.Fatalf("ctrl+c command = %T, want tea.QuitMsg", qc())
	}
	if m2.ExitCode() != 130 || !cancelled {
		t.Fatalf("ctrl+c during gated load: code=%d cancelled=%v", m2.ExitCode(), cancelled)
	}
	// A load finishing after cancellation cannot revive the UI.
	close(gate)
	m3, c3 := update(t, m2, <-done)
	if c3 != nil || !m3.quit || m3.ExitCode() != 130 {
		t.Fatalf("late load revived a cancelled UI: quit=%v code=%d", m3.quit, m3.ExitCode())
	}
}

// q in the browse view quits with exit 0 through the ordinary cleanup
// path.
func TestQOnBrowseExitsZero(t *testing.T) {
	dir := t.TempDir()
	writeWorkFile(t, dir, "a.txt", "x\n")
	m, _ := browseModel(t, dir, 80, 24,
		`{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"x\n"},"line_number":1,"submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}`,
		`{"type":"summary","data":{}}`,
	)
	m2, cmd := update(t, m, keyPress("q"))
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("q on browse command = %T, want tea.QuitMsg", cmd())
	}
	if m2.ExitCode() != 0 {
		t.Fatalf("ExitCode = %d, want 0", m2.ExitCode())
	}
}

// Esc in the base browse state is a no-op.
func TestEscOnBrowseIsNoOp(t *testing.T) {
	dir := t.TempDir()
	writeWorkFile(t, dir, "a.txt", "x\n")
	m, _ := browseModel(t, dir, 80, 24,
		`{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"x\n"},"line_number":1,"submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}`,
		`{"type":"summary","data":{}}`,
	)
	m2, cmd := update(t, m, escPress())
	if cmd != nil {
		t.Fatalf("Esc on browse produced a command %T, want none", cmd)
	}
	if m2.quit || m2.phase != phaseBrowse {
		t.Fatalf("Esc on browse changed state: quit=%v phase=%d", m2.quit, m2.phase)
	}
}

// A failed load leaves the current file's panel showing the unreadable
// placeholder.
func TestLoadFailureShowsUnreadable(t *testing.T) {
	dir := t.TempDir()
	m, _ := browseModel(t, dir, 80, 24,
		`{"type":"match","data":{"path":{"text":"gone.txt"},"lines":{"text":"x\n"},"line_number":1,"submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}`,
		`{"type":"summary","data":{}}`,
	)
	m2, _ := update(t, m, loadDoneMsg{path: []byte("gone.txt"), err: os.ErrNotExist})
	if v := m2.View().Content; !strings.Contains(v, "(unreadable)") {
		t.Fatalf("failed load view lacks the unreadable placeholder: %q", v)
	}
}

// The composed view renders the file list in raw-path order with the
// current entry underlined, the filename rule embedding the escaped
// path, and the right-justified gutter followed by two spaces.
func TestBrowseRendering(t *testing.T) {
	dir := t.TempDir()
	writeWorkFile(t, dir, "a.txt", "one\ntwo\nthree\n")
	writeWorkFile(t, dir, "b.txt", "x\n")
	m, cmd := browseModel(t, dir, 80, 24,
		`{"type":"match","data":{"path":{"text":"b.txt"},"lines":{"text":"x\n"},"line_number":1,"submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}`,
		`{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"two\n"},"line_number":2,"submatches":[{"match":{"text":"two"},"start":0,"end":3}]}}`,
		`{"type":"summary","data":{}}`,
	)
	m2, _ := update(t, m, cmd())
	v := m2.View().Content

	// Current file underlined in the list.
	if !strings.Contains(v, "\x1b[4ma.txt") {
		t.Fatalf("current file not underlined in the list: %q", v)
	}
	// Filename rule: dashes around the escaped current path.
	if !strings.Contains(v, "── a.txt ") {
		t.Fatalf("filename rule missing or malformed: %q", v)
	}
	// Gutter: right-justified numbers followed by two spaces (the
	// matched line's span sits inside the SGR run).
	for _, want := range []string{"1  one", "2  \x1b[7mtwo", "3  three"} {
		if !strings.Contains(v, want) {
			t.Fatalf("gutter row %q missing: %q", want, v)
		}
	}
	// The matched line renders its span in inverse video.
	if !strings.Contains(v, "\x1b[7mtwo\x1b[27m") {
		t.Fatalf("match not inverse: %q", v)
	}
}
