package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/safepresentation"
	"vrg/internal/theme"
)

var (
	keyH        = tea.KeyPressMsg{Text: "h", Code: 'h'}
	keyQuestion = tea.KeyPressMsg{Text: "?", Code: '?'}
)

// helpBrowseModel returns a model in the browse state — the happy
// stream's two files — sized 80x24 with nothing modal open.
func helpBrowseModel(t *testing.T) *model {
	t.Helper()
	m := newTestModel(fakeChild{res: Result{Stdout: []byte(happyStream), Code: 0}}, options{})
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m.Update(runCollectCmd(t, m))
	if m.state != stateBrowse {
		t.Fatalf("state = %v, want browse", m.state)
	}
	return m
}

// h and ? open the modal help over ordinary browsing — the bordered
// binding table floats over the file view — and each of q, Esc, h, ?
// closes it back to browsing with no exit.
func TestHelpOpensFromBrowseAndCloses(t *testing.T) {
	for _, open := range []tea.KeyPressMsg{keyH, keyQuestion} {
		for _, close := range []tea.KeyPressMsg{keyQ, keyEsc, keyH, keyQuestion} {
			m := helpBrowseModel(t)
			m.theme = theme.Plain()
			if _, cmd := m.Update(open); cmd != nil {
				t.Fatalf("opening help with %v returned a command", open)
			}
			if !m.helpOpen {
				t.Fatalf("%v did not open help from browse", open)
			}
			v := viewText(m)
			if !strings.Contains(v, "┌") || !strings.Contains(v, "Key bindings") {
				t.Fatalf("help view = %q, want the bordered binding table", v)
			}
			_, cmd := m.Update(close)
			if cmd != nil {
				t.Fatalf("%v under help returned a command", close)
			}
			if m.helpOpen || m.quitting {
				t.Fatalf("%v: helpOpen=%v quitting=%v, want closed and running",
					close, m.helpOpen, m.quitting)
			}
			if m.state != stateBrowse {
				t.Fatalf("%v close left state = %v, want browse", close, m.state)
			}
			if v := viewText(m); strings.Contains(v, "Key bindings") {
				t.Fatalf("%v: dismissed help still renders: %q", close, v)
			}
		}
	}
}

// h and ? open help over the no-results screen too; each close key
// returns to that screen — other keys stay ignored while it is up —
// and a subsequent q still exits 1.
func TestHelpOpensFromNoResults(t *testing.T) {
	for _, open := range []tea.KeyPressMsg{keyH, keyQuestion} {
		for _, close := range []tea.KeyPressMsg{keyQ, keyEsc, keyH, keyQuestion} {
			m := newTestModel(fakeChild{res: Result{Stdout: []byte(emptyStream), Code: 1}}, options{})
			m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
			m.Update(runCollectCmd(t, m))
			if m.state != stateNoResults {
				t.Fatalf("state = %v, want no-results", m.state)
			}
			m.Update(open)
			if !m.helpOpen {
				t.Fatalf("%v did not open help from no-results", open)
			}
			// Keys outside the modal set stay ignored while it is up.
			if _, cmd := m.Update(keyN); cmd != nil || !m.helpOpen || m.quitting {
				t.Fatalf("n under help: cmd=%v helpOpen=%v quitting=%v, want ignored",
					cmd != nil, m.helpOpen, m.quitting)
			}
			if _, cmd := m.Update(close); cmd != nil {
				t.Fatalf("%v under help returned a command", close)
			}
			if m.helpOpen || m.quitting {
				t.Fatalf("%v: helpOpen=%v quitting=%v, want closed and running",
					close, m.helpOpen, m.quitting)
			}
			if v := viewText(m); !strings.Contains(v, "No results found") {
				t.Fatalf("%v: view = %q, want the no-results screen back", close, v)
			}
			_, cmd := m.Update(keyQ)
			if !m.quitting {
				t.Fatal("q on the restored no-results screen did not begin a controlled exit")
			}
			runQuittingCmd(t, cmd)
			if m.status != 1 {
				t.Fatalf("status = %d, want the empty search's exit 1", m.status)
			}
		}
	}
}

// While help is open every key other than up/down, the close keys, and
// ctrl+c is ignored: the file behind is untouched — the cursor, the
// scroll position, the wrap and colour settings, the file-list
// preference, and the load state are all what they were — and no
// command issues.
func TestHelpIgnoresBaseKeys(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	idx := navIndex(t, []navFile{
		{name: "a.txt", content: numberedContent("a", 60), stops: []navStop{{line: 5, start: 0, end: 1}}},
		{name: "b.txt", content: numberedContent("b", 40), stops: []navStop{{line: 2, start: 0, end: 1}}},
	})
	finishLoad(t, m, startBrowse(t, m, idx))
	m.Update(keyDown)
	m.Update(keyDown)
	key := string(idx.Files[0].Path)
	top0 := m.vps[key].Top()
	if top0 == 0 {
		t.Fatal("browse scrolling did not move the viewport before help opened")
	}
	cur0, _ := m.idx.Cursor()
	m.Update(keyH)
	if !m.helpOpen {
		t.Fatal("h did not open help")
	}
	for _, k := range []tea.KeyPressMsg{
		keyN, keyP, keyW, keyC, keyR,
		keyU, keyD, keyPgUp, keyPgDn,
		keyLeft, keyRight, keyTab, keyShiftTab,
		{Text: ",", Code: ','}, {Text: ".", Code: '.'},
		{Text: "<", Code: '<'}, {Text: ">", Code: '>'},
		{Text: "[", Code: '['}, {Text: "]", Code: ']'},
		{Code: tea.KeyEnter}, {Code: tea.KeyHome}, {Code: tea.KeyEnd},
		{Text: "x", Code: 'x'},
	} {
		if _, cmd := m.Update(k); cmd != nil {
			t.Fatalf("key %v under help returned a command", k)
		}
		if !m.helpOpen || m.help.scroll != 0 || m.quitting {
			t.Fatalf("key %v disturbed help: open=%v scroll=%d quitting=%v",
				k, m.helpOpen, m.help.scroll, m.quitting)
		}
	}
	// The state behind is exactly what it was.
	if cur, _ := m.idx.Cursor(); cur != cur0 {
		t.Fatalf("cursor moved to %v under help", cur)
	}
	if got := m.vps[key].Top(); got != top0 {
		t.Fatalf("file viewport scrolled to %d under help, want %d", got, top0)
	}
	if !m.wrap {
		t.Fatal("w toggled wrap under help")
	}
	if m.theme != theme.Dark() {
		t.Fatal("c toggled the theme under help")
	}
	if len(m.loading) != 0 {
		t.Fatalf("r minted a load under help: %v", m.loading)
	}
	if !m.listVisible {
		t.Fatal("tab/left toggled the file list under help")
	}
}

// up and down scroll help's complete wrapped row set, clamped to
// [0, rows-visible]; the file behind never moves.
func TestHelpScrollsRenderedRows(t *testing.T) {
	m := helpBrowseModel(t)
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 8})
	m.Update(keyH)
	if !m.helpOpen {
		t.Fatal("h did not open help")
	}
	maxScroll := len(m.helpRows()) - m.overlayVisible()
	if maxScroll < 1 {
		t.Fatalf("help did not overflow the frame: rows=%d visible=%d",
			len(m.helpRows()), m.overlayVisible())
	}

	// up at the top is a clamp, not an error; down scrolls one row.
	if _, cmd := m.Update(keyUp); cmd != nil || m.help.scroll != 0 {
		t.Fatalf("up at scroll 0: scroll=%d cmd=%v", m.help.scroll, cmd)
	}
	m.Update(keyDown)
	if m.help.scroll != 1 {
		t.Fatalf("scroll after down = %d, want 1", m.help.scroll)
	}
	m.Update(keyUp)
	if m.help.scroll != 0 {
		t.Fatalf("scroll after up = %d, want 0", m.help.scroll)
	}

	// The scrollable set is the complete wrapped table: scroll past
	// the bottom and the view clamps on the last rows — the title row
	// has left the frame, the final wrapped row is in it.
	for i := 0; i < maxScroll+5; i++ {
		m.Update(keyDown)
	}
	if m.help.scroll != maxScroll {
		t.Fatalf("scroll clamped at %d, want %d", m.help.scroll, maxScroll)
	}
	v := viewText(m)
	last := m.helpRows()[len(m.helpRows())-1]
	if !strings.Contains(v, strings.TrimSpace(last)) {
		t.Fatalf("bottom of the scroll range missing the tail row %q: %q", last, v)
	}
	if strings.Contains(v, "Key bindings") {
		t.Fatalf("scrolled-to-bottom frame still shows the title row: %q", v)
	}
}

// ctrl+c inside help is the global override: cleanup and exit 130
// whatever the fixed search-derived status was.
func TestHelpCtrlCExits130(t *testing.T) {
	m := helpBrowseModel(t)
	m.Update(keyH)
	if !m.helpOpen {
		t.Fatal("h did not open help")
	}
	_, cmd := m.Update(keyCtrlC)
	if !m.quitting {
		t.Fatal("ctrl+c inside help did not begin a controlled exit")
	}
	runQuittingCmd(t, cmd)
	if m.status != 130 {
		t.Fatalf("status = %d, want ctrl+c's 130 override", m.status)
	}
}

// A long unbroken substituted string — the footer slot — wraps within
// the border: every rendered row fits the terminal width and the whole
// text survives across the wrapped rows.
func TestHelpWrapsUnbrokenSubstitution(t *testing.T) {
	prev := helpFooter
	helpFooter = strings.Repeat("Z", 200)
	t.Cleanup(func() { helpFooter = prev })
	m := helpBrowseModel(t)
	m.theme = theme.Plain()
	m.Update(keyH)
	if !m.helpOpen {
		t.Fatal("h did not open help")
	}
	v := viewText(m)
	if !strings.Contains(v, "┌") || !strings.Contains(v, "└") {
		t.Fatalf("help border missing: %q", v)
	}
	for i, row := range strings.Split(v, "\n") {
		if w := safepresentation.CellWidth(row); w > 80 {
			t.Fatalf("row %d is %d cells wide, over the 80-cell frame: %q", i, w, row)
		}
	}
	if n := strings.Count(v, "Z"); n != 200 {
		t.Fatalf("wrapped help lost substitution text: %d Z's, want 200", n)
	}
}

// At a tiny size above the 20x3 minimum the help is clipped to the
// terminal — there is no borderless mode, the border still draws —
// and growing the terminal restores the full table.
func TestHelpClippedAtTinySize(t *testing.T) {
	m := helpBrowseModel(t)
	m.theme = theme.Plain()
	m.Update(tea.WindowSizeMsg{Width: 25, Height: 8})
	m.Update(keyQuestion)
	if !m.helpOpen {
		t.Fatal("? did not open help at 25x8")
	}
	v := viewText(m) // must render without panic
	rows := strings.Split(v, "\n")
	if len(rows) != 8 {
		t.Fatalf("view rows = %d, want the 8-row frame", len(rows))
	}
	for i, row := range rows {
		if w := safepresentation.CellWidth(row); w > 25 {
			t.Fatalf("row %d is %d cells at a 25-cell frame: %q", i, w, row)
		}
	}
	if !strings.Contains(v, "┌") || !strings.Contains(v, "└") {
		t.Fatalf("clipped help lost its border: %q", v)
	}

	// Growth restores the whole table, head to tail.
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	v = viewText(m)
	if !strings.Contains(v, "Key bindings") || !strings.Contains(v, "exit immediately") {
		t.Fatalf("grown view = %q, want the whole binding table", v)
	}
}

// The help body is rendered from the single binding-table data source:
// every table row's key set and description appear in the composed
// text and in the frame — the same table Issue #34's documentation
// test will iterate.
func TestHelpRendersBindingTable(t *testing.T) {
	m := helpBrowseModel(t)
	m.Update(keyH)
	if !m.helpOpen {
		t.Fatal("h did not open help")
	}
	text := strings.Join(m.helpRows(), "\n")
	v := viewText(m)
	for _, b := range helpBindings {
		if !strings.Contains(text, b.keys) || !strings.Contains(text, b.desc) {
			t.Fatalf("help text missing binding %q — %q: %q", b.keys, b.desc, text)
		}
		if !strings.Contains(v, b.keys) || !strings.Contains(v, b.desc) {
			t.Fatalf("help view missing binding %q — %q: %q", b.keys, b.desc, v)
		}
	}
}

// Opening help cancels a live file-change pop-up; the pop-up does not
// return after help closes.
func TestHelpCancelsPopup(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, popupStubTicks)
	idx := navIndex(t, navFiles)
	finishLoad(t, m, startBrowse(t, m, idx))
	navLeafMsgs(t, m, keyN)
	navLeafMsgs(t, m, keyN) // b.txt — pop-up up
	if m.popupID == 0 {
		t.Fatal("no live pop-up instance before help")
	}

	m.Update(keyQuestion)
	if !m.helpOpen {
		t.Fatal("? did not open help under the pop-up")
	}
	if m.popupID != 0 {
		t.Fatal("opening help did not cancel the pop-up")
	}

	m.Update(keyEsc)
	if m.helpOpen {
		t.Fatal("Esc did not close help")
	}
	if m.popupID != 0 {
		t.Fatal("the pop-up returned after help closed")
	}
}
