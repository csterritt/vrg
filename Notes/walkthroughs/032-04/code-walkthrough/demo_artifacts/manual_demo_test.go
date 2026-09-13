//go:build manual_demo

package app_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/app"
	"vrg/internal/filebuffer"
	"vrg/internal/searchindex"
)

// TestManualDemoOverlayPrecedence demonstrates the Issue #32 manual
// verification scenarios deterministically through the model:
//  1. Esc with no overlay does nothing in browsing.
//  2. q with a browse error overlay closes it (browse still running).
//  3. A second q exits with the fixed status (2 for a fatal search).
//  4. A fake rg exiting 3 with no output shows a fatal overlay.
//  5. Both Esc and q on that fatal overlay exit 2.
//  6. The gated error-while-help-open case restores help at its scroll
//     position.
func TestManualDemoOverlayPrecedence(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
		textMatch("src/b.go", "world\n", 1, subSpec{"world", 0, 5}),
	)
	buf := makeBuf(nil, 0, 3)
	gate := make(chan struct{})
	close(gate)
	loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		return buf, nil
	}
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(gate),
		app.WithFileLoader(loader),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	if cmd != nil {
		m = deliverLoad(t, m, cmd)
	}
	if m.State() != app.StateBrowse {
		t.Fatalf("setup: state = %v, want browse", m.State())
	}

	// 1. Esc with no overlay does nothing in browsing.
	m, cmd = update(t, m, keyPressOrEscape(tea.KeyEscape))
	if cmd != nil {
		t.Fatalf("step 1: Esc produced a command: %v", cmd)
	}
	if m.State() != app.StateBrowse {
		t.Fatalf("step 1: state = %v, want browse", m.State())
	}
	if m.OverlayOpen() {
		t.Fatalf("step 1: overlay open after Esc, want none")
	}
	t.Logf("step 1 OK: Esc with no overlay is a no-op in browsing (state=browse)")

	// 2. q with a browse error overlay closes it (browse still running).
	// Use a fatal search (exit 3) with usable results → browse + error overlay.
	errIdx := buildIndex(t, "/work",
		textBegin("a.go"),
		textMatch("a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
		endRecord("a.go", nil),
		summaryRecord(),
	)
	em := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(gate),
		app.WithFileLoader(loader),
	)
	em, _ = update(t, em, tea.WindowSizeMsg{Width: 80, Height: 24})
	em, ecmd := update(t, em, app.SearchCompleteMsg{
		Files: errIdx.Files(), Lines: errIdx.Len(), Index: errIdx,
		Process: app.ProcessResult{ExitCode: 3},
		Stderr:  "fatal search error",
	})
	if ecmd != nil {
		em = deliverLoad(t, em, ecmd)
	}
	if !em.OverlayOpen() || em.OverlayKind() != app.OverlayError {
		t.Fatalf("step 2: error overlay not open")
	}
	if em.ExitCode() != 2 {
		t.Fatalf("step 2: exit code = %d, want 2", em.ExitCode())
	}
	em, cmd = update(t, em, keyPress('q'))
	if cmd != nil {
		t.Fatalf("step 2: q produced a command: %v", cmd)
	}
	if em.OverlayOpen() {
		t.Fatalf("step 2: overlay still open after q")
	}
	if em.State() != app.StateBrowse {
		t.Fatalf("step 2: state = %v, want browse", em.State())
	}
	t.Logf("step 2 OK: q closed the browse error overlay (browse still running, exit=2)")

	// 3. A second q exits with the fixed status (2 for a fatal search).
	em, cmd = update(t, em, keyPress('q'))
	assertQuit(t, cmd)
	if em.ExitCode() != 2 {
		t.Fatalf("step 3: exit code = %d, want 2", em.ExitCode())
	}
	t.Logf("step 3 OK: second q exited with fixed status 2")

	// 4. A fake rg exiting 3 with no output shows a fatal overlay.
	// Fatal process (exit 3) with no usable results → no-results + fatal error overlay.
	fatalIdx := buildIndex(t, "/work", summaryRecord())
	fm := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(gate),
		app.WithFileLoader(loader),
	)
	fm, _ = update(t, fm, tea.WindowSizeMsg{Width: 80, Height: 24})
	fm, _ = update(t, fm, app.SearchCompleteMsg{
		Files: fatalIdx.Files(), Lines: fatalIdx.Len(), Index: fatalIdx,
		Process: app.ProcessResult{ExitCode: 3},
		Stderr:  "fatal: no matches and bad exit",
	})
	if !fm.OverlayOpen() || fm.OverlayKind() != app.OverlayError {
		t.Fatalf("step 4: fatal error overlay not open")
	}
	if !fm.OverlayFatal() {
		t.Fatalf("step 4: overlay not fatal")
	}
	view := viewContent(fm)
	if !strings.Contains(view, "fatal") {
		t.Fatalf("step 4: overlay view does not contain 'fatal': %q", view)
	}
	t.Logf("step 4 OK: fake rg exit 3 with no output shows fatal overlay (fatal=%v)", fm.OverlayFatal())

	// 5a. Esc on that fatal overlay exits 2.
	fm, cmd = update(t, fm, keyPressOrEscape(tea.KeyEscape))
	assertQuit(t, cmd)
	if fm.ExitCode() != 2 {
		t.Fatalf("step 5a: exit code = %d, want 2", fm.ExitCode())
	}
	t.Logf("step 5a OK: Esc on fatal overlay exited 2")

	// 5b. q on that fatal overlay exits 2 (fresh model).
	fm2 := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(gate),
		app.WithFileLoader(loader),
	)
	fm2, _ = update(t, fm2, tea.WindowSizeMsg{Width: 80, Height: 24})
	fm2, _ = update(t, fm2, app.SearchCompleteMsg{
		Files: fatalIdx.Files(), Lines: fatalIdx.Len(), Index: fatalIdx,
		Process: app.ProcessResult{ExitCode: 3},
		Stderr:  "fatal: no matches and bad exit",
	})
	if !fm2.OverlayFatal() {
		t.Fatalf("step 5b: overlay not fatal")
	}
	fm2, cmd = update(t, fm2, keyPress('q'))
	assertQuit(t, cmd)
	if fm2.ExitCode() != 2 {
		t.Fatalf("step 5b: exit code = %d, want 2", fm2.ExitCode())
	}
	t.Logf("step 5b OK: q on fatal overlay exited 2")

	// 6. The gated error-while-help-open case restores help at its scroll position.
	// Set up a browse model with a gated failing loader so we can open help,
	// scroll it, then trigger a read failure while help is open.
	gLoader := newGatedFailingLoader()
	gLoader.set("src/a.go", makeBuf([]filebuffer.Line{ml(1, "content-a")}, 1, 3))
	gLoader.set("src/b.go", makeBuf([]filebuffer.Line{ml(1, "content-b")}, 1, 3))
	gGate := make(chan struct{})
	close(gGate)
	gm := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(gGate),
		app.WithFileLoader(gLoader.load),
		app.WithPopupDuration(0),
	)
	gm, _ = update(t, gm, tea.WindowSizeMsg{Width: 80, Height: 24})
	gm, gcmd := update(t, gm, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	if gm.State() != app.StateBrowse {
		t.Fatalf("step 6: state = %v, want browse", gm.State())
	}
	loadCh := startLoadAsync(gcmd)

	// Open help and scroll to position S.
	gm, _ = update(t, gm, keyPress('h'))
	if !gm.OverlayOpen() || gm.OverlayKind() != app.OverlayHelp {
		t.Fatalf("step 6: help not open")
	}
	const helpScrollS = 2
	for i := 0; i < helpScrollS; i++ {
		gm, _ = update(t, gm, tea.KeyPressMsg{Code: tea.KeyDown})
	}
	if gm.OverlayScroll() != helpScrollS {
		t.Fatalf("step 6: help scroll = %d, want %d", gm.OverlayScroll(), helpScrollS)
	}
	t.Logf("step 6a OK: help open at scroll %d", helpScrollS)

	// Trigger a read failure while help is open → error suspends help.
	gLoader.setFailing("src/a.go", "permission denied")
	gLoader.release("src/a.go")
	lc := <-loadCh
	gm = deliverCompletion(t, gm, lc)
	if !gm.OverlayOpen() || gm.OverlayKind() != app.OverlayError {
		t.Fatalf("step 6: error overlay not open over help")
	}
	if !gm.HelpSuspended() {
		t.Fatalf("step 6: help not suspended")
	}
	t.Logf("step 6b OK: read failure suspended help (error overlay open, help suspended)")

	// Dismiss the error with q → help restored at S.
	gm, cmd = update(t, gm, keyPress('q'))
	if cmd != nil {
		t.Fatalf("step 6: q produced a command: %v", cmd)
	}
	if !gm.OverlayOpen() || gm.OverlayKind() != app.OverlayHelp {
		t.Fatalf("step 6: help not restored after q")
	}
	if gm.OverlayScroll() != helpScrollS {
		t.Fatalf("step 6: restored help scroll = %d, want %d", gm.OverlayScroll(), helpScrollS)
	}
	t.Logf("step 6c OK: q dismissed error, help restored at scroll %d", helpScrollS)

	// Next q closes help to browse.
	gm, cmd = update(t, gm, keyPress('q'))
	if cmd != nil {
		t.Fatalf("step 6: second q produced a command: %v", cmd)
	}
	if gm.OverlayOpen() {
		t.Fatalf("step 6: help still open after second q")
	}
	if gm.State() != app.StateBrowse {
		t.Fatalf("step 6: state = %v, want browse", gm.State())
	}
	t.Logf("step 6d OK: second q closed help to browse")

	// Third q exits with fixed status 0 (clean rg 0).
	gm, cmd = update(t, gm, keyPress('q'))
	assertQuit(t, cmd)
	if gm.ExitCode() != 0 {
		t.Fatalf("step 6: exit code = %d, want 0", gm.ExitCode())
	}
	t.Logf("step 6e OK: third q exited with fixed status 0")

	t.Logf("Manual demonstration passed: overlay precedence behavior matches the Issue #32 contracts.")
}
