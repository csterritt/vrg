package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/clipperhouse/displaywidth"

	"vrg/internal/searchindex"
	"vrg/internal/theme"
)

// helpModelAt drives a model into the browse state over one loaded file
// at w×h and opens the help overlay with key, failing unless it opened.
func helpModelAt(t *testing.T, key string, w, h int) Model {
	t.Helper()
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", "hit a\n")
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "hit a\n", 1, 0, 3, "hit"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, w, h)
	m = applyLoad(t, m, cmd())
	m, _ = update(t, m, keyPress(key))
	if m.help == nil {
		t.Fatalf("%s did not open the help overlay", key)
	}
	return m
}

// helpModel is helpModelAt at the canonical 80x24 size.
func helpModel(t *testing.T, key string) Model {
	t.Helper()
	return helpModelAt(t, key, 80, 24)
}

// noResultsModel drives a model to the plain no-results screen — a
// complete rg-1 search that retained nothing — at 80x24.
func noResultsModel(t *testing.T) Model {
	t.Helper()
	idx := searchindex.New("/w")
	idx.Finish()
	m := New(&fakeChild{stdout: strings.NewReader(""), stderr: strings.NewReader("")}, "/w")
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = update(t, m, searchResult{index: idx, integrity: completeStream, err: exitError(1)})
	if m.state != stateNoResults || m.overlay != nil {
		t.Fatalf("setup: state %d, overlay %v, want bare no-results", m.state, m.overlay != nil)
	}
	return m
}

// h and ? open the modal help overlay over ordinary browsing: the
// bordered binding list replaces the centre rows while the browse state
// stays underneath.
func TestHelpOpensFromBrowse(t *testing.T) {
	for _, key := range []string{"h", "?"} {
		t.Run(key, func(t *testing.T) {
			m := helpModel(t, key)
			if m.state != stateBrowse {
				t.Fatalf("state under help = %d, want stateBrowse", m.state)
			}
			v := m.View().Content
			if !strings.Contains(v, "┌") || !strings.Contains(v, "└") {
				t.Fatalf("help view lacks the bordered box:\n%s", v)
			}
			if !strings.Contains(v, "matched line") {
				t.Fatalf("help view lacks the binding table:\n%s", v)
			}
		})
	}
}

// h and ? open help over the no-results screen; the screen underneath
// is still the no-results state.
func TestHelpOpensFromNoResults(t *testing.T) {
	for _, key := range []string{"h", "?"} {
		t.Run(key, func(t *testing.T) {
			m := noResultsModel(t)
			m, cmd := update(t, m, keyPress(key))
			if cmd != nil {
				t.Fatalf("%s on no-results returned a command: %v", key, cmd)
			}
			if m.help == nil || m.state != stateNoResults {
				t.Fatalf("%s did not open help over no-results: help %v, state %d",
					key, m.help != nil, m.state)
			}
			if v := m.View().Content; !strings.Contains(v, "┌") {
				t.Fatalf("help over no-results lacks the box:\n%s", v)
			}
		})
	}
}

// h while searching is unbound: the searching screen and state are
// unchanged.
func TestHelpDoesNotOpenWhileSearching(t *testing.T) {
	for _, key := range []string{"h", "?"} {
		m := New(&fakeChild{stdout: strings.NewReader(""), stderr: strings.NewReader("")}, "/w")
		m, cmd := update(t, m, keyPress(key))
		if cmd != nil {
			t.Fatalf("%s while searching returned a command: %v", key, cmd)
		}
		if m.help != nil || m.state != stateSearching {
			t.Fatalf("%s while searching opened help or changed state: help %v, state %d",
				key, m.help != nil, m.state)
		}
	}
}

// q, Esc, h, and ? each close help: control returns to the underlying
// base state — browse or no-results — and no command runs. On
// no-results a subsequent q still exits 1.
func TestHelpCloseKeys(t *testing.T) {
	for _, key := range []string{"q", "esc", "h", "?"} {
		t.Run(key+" over browse", func(t *testing.T) {
			m := helpModel(t, "h")
			m, cmd := update(t, m, keyPress(key))
			if cmd != nil {
				t.Fatalf("%s closing help returned a command: %v", key, cmd)
			}
			if m.help != nil || m.state != stateBrowse {
				t.Fatalf("%s did not close help to browse: help %v, state %d",
					key, m.help != nil, m.state)
			}
			if v := m.View().Content; strings.Contains(v, "┌") {
				t.Fatalf("closed help still renders its box:\n%s", v)
			}
		})
		t.Run(key+" over no-results", func(t *testing.T) {
			m := noResultsModel(t)
			m, _ = update(t, m, keyMsg("h"))
			if m.help == nil {
				t.Fatal("h did not open help over no-results")
			}
			m, cmd := update(t, m, keyPress(key))
			if cmd != nil {
				t.Fatalf("%s closing help returned a command: %v", key, cmd)
			}
			if m.help != nil || m.state != stateNoResults {
				t.Fatalf("%s did not close help to no-results: help %v, state %d",
					key, m.help != nil, m.state)
			}
			if v := m.View().Content; !strings.Contains(v, "No results found") {
				t.Fatalf("closed help lost the no-results screen:\n%s", v)
			}
			m, cmd = update(t, m, keyPress("q"))
			requireQuit(t, cmd, "q on no-results after help closed")
			if m.status != 1 {
				t.Fatalf("exit status = %d, want 1", m.status)
			}
		})
	}
}

// While help is open every key other than up, down, q, Esc, h, ?, and
// ctrl+c is ignored: nothing reaches the content behind — no
// navigation, wrap or colour toggle, list change, reload, or page
// scroll.
func TestHelpIgnoresOtherKeys(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", "hit a\n")
	writeMatchFile(t, dir, "b.txt", "hit b\n")
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "hit a\n", 1, 0, 3, "hit"))
	addRec(t, idx, matchRec("b.txt", "hit b\n", 1, 0, 3, "hit"))
	idx.Finish()

	// A short terminal leaves rows below the fold to scroll to.
	m, cmd := startBrowse(t, dir, idx, 80, 10)
	m = applyLoad(t, m, cmd())
	m, _ = update(t, m, keyMsg("h"))
	if m.help == nil {
		t.Fatal("h did not open help")
	}
	m, _ = update(t, m, keyPress("down"))
	if m.help == nil || m.help.scroll != 1 {
		t.Fatalf("setup: help scroll = %v/%d", m.help != nil, m.help.scroll)
	}

	for _, key := range []string{"x", "n", "p", "w", "c", "r", "left", "right", "u", "d", "pgup", "pgdown", ",", ".", "enter", " "} {
		m2, cmd := update(t, m, keyPress(key))
		if cmd != nil {
			t.Fatalf("%s with help open returned a command: %v", key, cmd)
		}
		m = m2
		if m.help == nil {
			t.Fatalf("%s closed the help overlay", key)
		}
		if m.help.scroll != 1 {
			t.Fatalf("%s moved the help scroll to %d", key, m.help.scroll)
		}
		if m.state != stateBrowse {
			t.Fatalf("%s changed state to %d", key, m.state)
		}
	}

	// Nothing behind the modal moved: same cursor stop, wrap on,
	// original scheme, list still visible, no reload minted.
	stop, _ := m.index.Current()
	if string(stop.Path) != "a.txt" || stop.Line != 1 {
		t.Fatalf("cursor moved under help to %s:%d", stop.Path, stop.Line)
	}
	if !m.wrap || !m.listVisible {
		t.Fatalf("behind-state changed under help: wrap %v, list %v", m.wrap, m.listVisible)
	}
	if len(m.reloading) != 0 || len(m.loading) != 0 {
		t.Fatalf("a load was minted under help: reloading %v, loading %v", m.reloading, m.loading)
	}
	if v := m.View().Content; !strings.HasPrefix(v, "\x1b[37;40m") {
		t.Fatalf("c toggled the scheme under help:\n%q", v)
	}
}

// ctrl+c exits 130 even with help open.
func TestHelpCtrlCExits130(t *testing.T) {
	child := &fakeChild{
		stdout:     strings.NewReader(""),
		stderr:     strings.NewReader(""),
		terminated: make(chan struct{}),
	}
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", "hit a\n")
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "hit a\n", 1, 0, 3, "hit"))
	idx.Finish()

	m := New(child, dir)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = update(t, m, searchResult{index: idx, integrity: completeStream})
	m, _ = update(t, m, keyMsg("h"))
	if m.help == nil {
		t.Fatal("h did not open help")
	}

	m, cmd := update(t, m, keyPress("ctrl+c"))
	requireQuit(t, cmd, "ctrl+c with help open")
	if m.status != 130 {
		t.Fatalf("exit status = %d, want 130", m.status)
	}
	requireClosed(t, child.terminated, "child termination")
}

// up and down scroll the help overlay's wrapped rows, clamped to the
// complete row set so every row is reachable: each row in turn is the
// top interior row of some scroll position.
func TestHelpScrollsUpDown(t *testing.T) {
	// 40x8 gives a 38-cell interior six rows tall: the binding table
	// overflows it, so scrolling has somewhere to go.
	m := helpModelAt(t, "h", 40, 8)
	rows := m.help.rows(38)
	if len(rows) <= 6 {
		t.Fatalf("help wraps to %d rows at interior 38, want more than 6", len(rows))
	}
	max := len(rows) - 6
	for s := 1; s <= max; s++ {
		m, _ = update(t, m, keyPress("down"))
		if m.help.scroll != s {
			t.Fatalf("down #%d: scroll = %d, want %d", s, m.help.scroll, s)
		}
		if top := strings.TrimRight(rows[s], " "); !strings.Contains(m.View().Content, top) {
			t.Fatalf("at scroll %d the view lacks its top row %q:\n%s", s, top, m.View().Content)
		}
	}
	for i := 0; i < 20; i++ {
		m, _ = update(t, m, keyPress("down"))
	}
	if m.help.scroll != max {
		t.Fatalf("scroll past the bottom = %d, want %d", m.help.scroll, max)
	}
	if v := m.View().Content; strings.Contains(v, strings.TrimRight(rows[0], " ")) {
		t.Fatalf("scrolled-to-bottom help still shows the first row:\n%s", v)
	}

	for s := max - 1; s >= 0; s-- {
		m, _ = update(t, m, keyPress("up"))
		if m.help.scroll != s {
			t.Fatalf("up: scroll = %d, want %d", m.help.scroll, s)
		}
	}
	for i := 0; i < 20; i++ {
		m, _ = update(t, m, keyPress("up"))
	}
	if m.help.scroll != 0 {
		t.Fatalf("scroll past the top = %d, want 0", m.help.scroll)
	}
	if top := strings.TrimRight(rows[0], " "); !strings.Contains(m.View().Content, top) {
		t.Fatalf("scrolled-to-top help lacks the first row %q:\n%s", top, m.View().Content)
	}
}

// The help overlay renders every row of the shared binding table — each
// entry's keys and description — plus the footer slot's content after
// the table.
func TestHelpRendersBindingTable(t *testing.T) {
	helpFooter = "scale note fixture"
	defer func() { helpFooter = "" }()

	m := helpModel(t, "h")
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 40})
	v := m.View().Content
	for _, b := range helpBindings {
		if !strings.Contains(v, b.keys) {
			t.Fatalf("help lacks binding %q:\n%s", b.keys, v)
		}
		if !strings.Contains(v, b.desc) {
			t.Fatalf("help lacks description %q:\n%s", b.desc, v)
		}
	}
	if !strings.Contains(v, "scale note fixture") {
		t.Fatalf("help lacks the footer slot's content:\n%s", v)
	}
	// The footer follows the table: it renders below the last binding.
	if keys, foot := strings.Index(v, "ctrl+c"), strings.Index(v, "scale note fixture"); foot < keys {
		t.Fatalf("footer rendered above the binding table (%d < %d):\n%s", foot, keys, v)
	}
}

// A long unbroken footer line wraps at the interior width inside the
// border: no rendered row exceeds the terminal width.
func TestHelpWrapsUnbrokenText(t *testing.T) {
	helpFooter = strings.Repeat("z", 200)
	defer func() { helpFooter = "" }()

	m := helpModel(t, "?")
	m.theme = theme.Plain()
	v := m.View().Content

	full := "│" + strings.Repeat("z", 78) + "│"
	if n := strings.Count(v, full); n != 2 {
		t.Fatalf("wrapped unbroken footer rows = %d, want 2 full-width rows:\n%s", n, v)
	}
	partial := "│" + strings.Repeat("z", 44) + strings.Repeat(" ", 34) + "│"
	if !strings.Contains(v, partial) {
		t.Fatalf("wrapped help lacks the partial third footer row:\n%s", v)
	}
	for i, line := range strings.Split(v, "\n") {
		if w := displaywidth.String(line); w > 80 {
			t.Fatalf("rendered line %d is %d cells wide, over the 80-cell terminal:\n%q", i, w, line)
		}
	}
}

// At a tiny size the help overlay is clipped to the terminal without a
// borderless mode — the bordered box still renders — and growth
// restores the normal layout.
func TestHelpClipsAtTinySize(t *testing.T) {
	m := helpModel(t, "h")
	m.theme = theme.Plain()

	m, _ = update(t, m, tea.WindowSizeMsg{Width: 25, Height: 8})
	v := m.View().Content
	for i, r := range strings.Split(v, "\n") {
		if w := displaywidth.String(r); w > 25 {
			t.Fatalf("row %d is %d cells wide at width 25:\n%q", i, w, r)
		}
	}
	if !strings.Contains(v, "┌") || !strings.Contains(v, "┘") {
		t.Fatalf("clipped help lost its border — no borderless mode:\n%s", v)
	}

	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	v = m.View().Content
	if !strings.Contains(v, "┌"+strings.Repeat("─", 78)+"┐") {
		t.Fatalf("help did not restore its full-width box on growth:\n%s", v)
	}
	if !strings.Contains(v, "matched line") {
		t.Fatalf("grown help lost the binding table:\n%s", v)
	}
}

// Opening help cancels an active file-change pop-up; it does not return
// when help closes.
func TestHelpCancelsPopup(t *testing.T) {
	m := twoFileBrowse(t, 80, 24)
	m, _ = update(t, m, keyMsg("n"))
	if m.popup == nil {
		t.Fatal("cross-file n opened no pop-up")
	}

	m, _ = update(t, m, keyMsg("?"))
	if m.help == nil {
		t.Fatal("? did not open help over the pop-up")
	}
	if m.popup != nil {
		t.Fatal("opening help did not cancel the pop-up")
	}

	m, _ = update(t, m, keyPress("esc"))
	if m.help != nil {
		t.Fatal("Esc did not close help")
	}
	if m.popup != nil {
		t.Fatal("the pop-up returned after help closed")
	}
	if v := m.View().Content; strings.Contains(v, "│b.txt│") {
		t.Fatalf("cancelled pop-up rendered after help closed:\n%s", v)
	}
}
