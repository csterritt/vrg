package app

import (
	"bytes"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"vrg/internal/theme"
)

// helpModel returns a browse-phase model — the ordinary browsing state
// h and ? open the modal help overlay from.
func helpModel(t *testing.T) Model {
	t.Helper()
	m := completedModel(t, 1)
	if m.phase != phaseBrowse {
		t.Fatalf("phase = %d, want browse", m.phase)
	}
	return m
}

// h and ? each open the modal help overlay over ordinary browsing: the
// bordered box lists the key bindings while the browse state
// underneath is untouched.
func TestHelpOpensFromBrowse(t *testing.T) {
	for _, key := range []string{"h", "?"} {
		t.Run(key, func(t *testing.T) {
			m := helpModel(t)
			m2, cmd := pressKey(t, m, key)
			if cmd != nil {
				t.Fatalf("%s on browse returned a command %T, want none", key, cmd)
			}
			if m2.help == nil {
				t.Fatalf("%s did not open the help overlay", key)
			}
			if m2.overlay != nil {
				t.Fatalf("%s opened the error overlay instead of help", key)
			}
			v := m2.View().Content
			if !strings.Contains(v, "┌") || !strings.Contains(v, "└") {
				t.Fatalf("help view lacks the single-line border: %q", v)
			}
			if !strings.Contains(v, "Key bindings") {
				t.Fatalf("help view lacks the binding list: %q", v)
			}
			if m2.phase != phaseBrowse || m2.quit {
				t.Fatalf("%s changed the base state: phase=%d quit=%v", key, m2.phase, m2.quit)
			}
		})
	}
}

// h and ? each open the modal help overlay over the no-results screen;
// closing it returns to no-results and a subsequent q still exits 1.
func TestHelpOpensFromNoResults(t *testing.T) {
	for _, key := range []string{"h", "?"} {
		t.Run(key, func(t *testing.T) {
			m := noResultsModel(t, exitErr(t, 1), recSummary)
			m2, _ := pressKey(t, m, key)
			if m2.help == nil {
				t.Fatalf("%s did not open help on the no-results screen", key)
			}
			m2, _ = pressKey(t, m2, "esc")
			if m2.help != nil {
				t.Fatal("esc did not close help over no-results")
			}
			if m2.phase != phaseNoResults {
				t.Fatalf("post-close phase = %d, want no-results", m2.phase)
			}
			if v := m2.View().Content; !strings.Contains(v, "No results found") {
				t.Fatalf("view after closing help = %q, want the no-results screen", v)
			}
			m2, cmd := pressKey(t, m2, "q")
			if cmd == nil {
				t.Fatal("q on no-results returned no command, want tea.Quit")
			}
			if _, ok := cmd().(tea.QuitMsg); !ok {
				t.Fatalf("q on no-results command = %T, want tea.QuitMsg", cmd())
			}
			if m2.ExitCode() != 1 {
				t.Fatalf("ExitCode = %d, want 1", m2.ExitCode())
			}
		})
	}
}

// q, Esc, h, and ? each close the help overlay and return to the
// underlying browsing state.
func TestHelpCloseKeys(t *testing.T) {
	for _, key := range []string{"q", "esc", "h", "?"} {
		t.Run(key, func(t *testing.T) {
			m := helpModel(t)
			m, _ = pressKey(t, m, "h")
			if m.help == nil {
				t.Fatal("h did not open help")
			}
			m2, cmd := pressKey(t, m, key)
			if cmd != nil {
				t.Fatalf("%s on help produced a command %T, want none", key, cmd)
			}
			if m2.help != nil {
				t.Fatalf("%s did not close the help overlay", key)
			}
			if m2.phase != phaseBrowse || m2.quit {
				t.Fatalf("%s close: phase=%d quit=%v, want browsing resumed",
					key, m2.phase, m2.quit)
			}
		})
	}
}

// While help is open every other key is ignored — the cursor, the wrap
// and colour settings, the load state, and the underlying screen stay
// exactly as they were, and no key produces a command.
func TestHelpIgnoresOtherKeys(t *testing.T) {
	m := helpModel(t)
	m, _ = pressKey(t, m, "?")
	if m.help == nil {
		t.Fatal("? did not open help")
	}
	stop, _ := m.currentStop()
	for _, key := range []string{"n", "p", "w", "c", "r", "x", "left", "right", "tab", "enter", "u", "d", "pgup", "pgdown", ",", "."} {
		m2, cmd := pressKey(t, m, key)
		if cmd != nil {
			t.Fatalf("%q on help produced a command %T, want none", key, cmd)
		}
		if m2.help == nil || m2.quit || m2.phase != phaseBrowse {
			t.Fatalf("%q changed state: help=%v quit=%v phase=%d",
				key, m2.help != nil, m2.quit, m2.phase)
		}
		m = m2
	}
	if s, _ := m.currentStop(); !bytes.Equal(s.Path, stop.Path) || s.Line != stop.Line {
		t.Fatalf("a key moved the cursor under help: %+v → %+v", stop, s)
	}
	if !m.wrap {
		t.Fatal("w reached the wrap toggle while help was open")
	}
	if m.theme.Light() {
		t.Fatal("c reached the colour toggle while help was open")
	}
}

// ctrl+c inside the help overlay takes the cancellation path: quit at
// 130 whatever the fixed status would have been.
func TestHelpCtrlCExits130(t *testing.T) {
	m := helpModel(t)
	m, _ = pressKey(t, m, "h")
	m2, cmd := pressKey(t, m, "ctrl+c")
	if cmd == nil {
		t.Fatal("ctrl+c on help returned no command, want tea.Quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("ctrl+c command = %T, want tea.QuitMsg", cmd())
	}
	if m2.ExitCode() != 130 {
		t.Fatalf("ExitCode = %d, want 130", m2.ExitCode())
	}
}

// h and ? while searching are inert like every other ordinary key: the
// help overlay never opens over the searching screen.
func TestHelpKeysInertWhileSearching(t *testing.T) {
	for _, key := range []string{"h", "?"} {
		m := newModel(make(chan searchDoneMsg), nil)
		m2, cmd := update(t, m, keyPress(key))
		if cmd != nil {
			t.Fatalf("%s while searching produced a command %T, want none", key, cmd)
		}
		if m2.help != nil || m2.phase != phaseSearching || m2.quit {
			t.Fatalf("%s while searching changed state: help=%v phase=%d quit=%v",
				key, m2.help != nil, m2.phase, m2.quit)
		}
	}
}

// up and down scroll the help's rendered rows and clamp at both ends:
// shrinking the frame makes the binding list scrollable, enough downs
// reach the last row, extra downs stay put, and ups return to the head.
func TestHelpScrollsWithUpDown(t *testing.T) {
	m := helpModel(t)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 10})
	m, _ = pressKey(t, m, "h")
	m.theme = theme.Plain()

	v := m.View().Content
	if !strings.Contains(v, "Key bindings") {
		t.Fatalf("help head not visible initially: %q", v)
	}
	if strings.Contains(v, "ctrl+c") {
		t.Fatalf("help tail visible before scrolling: %q", v)
	}

	// Up at the top is a clamped no-op.
	m, _ = pressKey(t, m, "up")
	if !strings.Contains(m.View().Content, "Key bindings") {
		t.Fatal("up at the top scrolled past the head")
	}

	// Enough downs reach the tail — the Issue #34 footer note — passing
	// the binding table's last row on the way; extra downs clamp at the
	// bottom.
	sawBindingsTail := false
	for i := 0; i < 40; i++ {
		m, _ = pressKey(t, m, "down")
		if strings.Contains(m.View().Content, "ctrl+c") {
			sawBindingsTail = true
		}
	}
	v = m.View().Content
	if !sawBindingsTail {
		t.Fatalf("the ctrl+c binding row never scrolled into view")
	}
	if !strings.Contains(v, "terminal cleanup") {
		t.Fatalf("help tail not visible after scrolling: %q", v)
	}
	if strings.Contains(v, "Key bindings") {
		t.Fatalf("help head still visible at the bottom: %q", v)
	}

	// Back up returns the head.
	for i := 0; i < 40; i++ {
		m, _ = pressKey(t, m, "up")
	}
	if !strings.Contains(m.View().Content, "Key bindings") {
		t.Fatal("up did not scroll back to the head")
	}
}

// Substituted text — the footer slot — wraps to the interior width
// even when it carries no break points: a 300-cell unbroken string
// occupies several interior rows and no frame row overflows the
// terminal.
func TestHelpWrapsUnbrokenSubstitution(t *testing.T) {
	saved := helpFooter
	helpFooter = []string{strings.Repeat("x", 300)}
	defer func() { helpFooter = saved }()

	m := helpModel(t)
	m, _ = pressKey(t, m, "?")
	m.theme = theme.Plain()
	for i := 0; i < 40; i++ {
		m, _ = pressKey(t, m, "down") // bottom: the footer's rows
	}
	v := m.View().Content
	for i, row := range strings.Split(v, "\n") {
		if w := ansi.StringWidth(row); w > 80 {
			t.Fatalf("frame row %d is %d cells wide, want ≤80: %q", i, w, row)
		}
	}
	xrows := 0
	for _, row := range strings.Split(v, "\n") {
		if strings.Contains(row, "xxx") {
			xrows++
		}
	}
	if xrows < 3 {
		t.Fatalf("300-cell substitution occupied %d rows, want wrapping: %q", xrows, v)
	}
}

// Above the 20x3 minimum the overlay has no borderless mode: at 25x8
// it is clipped to the terminal — every row within the frame, the box
// still present — and growing the terminal restores the normal layout.
func TestHelpClippedAtTinySize(t *testing.T) {
	m := helpModel(t)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 25, Height: 8})
	m, _ = pressKey(t, m, "h")
	m.theme = theme.Plain()

	v := m.View().Content
	rows := strings.Split(v, "\n")
	if len(rows) != 8 {
		t.Fatalf("25x8 frame = %d rows, want 8: %q", len(rows), v)
	}
	for i, row := range rows {
		if w := ansi.StringWidth(row); w > 25 {
			t.Fatalf("frame row %d is %d cells wide, want ≤25: %q", i, w, row)
		}
	}
	if !strings.Contains(v, "┌") || !strings.Contains(v, "│") {
		t.Fatalf("25x8 help lacks the clipped box: %q", v)
	}
	if !strings.Contains(v, "Key bindings") {
		t.Fatalf("25x8 help lacks its content: %q", v)
	}

	// Growth restores the normal layout: the full bordered box with
	// the binding list visible inside it.
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	v = m.View().Content
	if !strings.Contains(v, "┌") || !strings.Contains(v, "└") ||
		!strings.Contains(v, "Key bindings") {
		t.Fatalf("restored help lacks the normal layout: %q", v)
	}
}

// The binding table is the single source the help renderer consumes:
// every row's key spelling and description appears in the overlay.
func TestHelpRendersBindingTable(t *testing.T) {
	if len(helpBindings) == 0 {
		t.Fatal("helpBindings is empty — the overlay needs the binding table")
	}
	m := helpModel(t)
	m, _ = pressKey(t, m, "h")
	// A tall frame shows the whole table without scrolling.
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 40})
	v := m.View().Content
	for _, b := range helpBindings {
		if !strings.Contains(v, b.keys) {
			t.Fatalf("help lacks binding %q: %q", b.keys, v)
		}
		if !strings.Contains(v, b.desc) {
			t.Fatalf("help lacks description %q: %q", b.desc, v)
		}
	}
}

// The footer slot is part of the rendered content: installed footer
// text appears inside the overlay.
func TestHelpRendersFooterSlot(t *testing.T) {
	saved := helpFooter
	helpFooter = []string{"scale note placeholder"}
	defer func() { helpFooter = saved }()

	m := helpModel(t)
	m, _ = pressKey(t, m, "h")
	m.theme = theme.Plain()
	for i := 0; i < 40; i++ {
		m, _ = pressKey(t, m, "down")
	}
	if v := m.View().Content; !strings.Contains(v, "scale note placeholder") {
		t.Fatalf("help lacks the footer slot's text: %q", v)
	}
}

// Opening help cancels an active file-change pop-up with no return:
// the pop-up is gone while help is up and does not come back when help
// closes; its stale expiry stays inert.
func TestHelpCancelsPopup(t *testing.T) {
	m, _ := popupModel(t, 8, 4)
	stale := m.popupID
	if stale == 0 {
		t.Fatal("the file crossing did not open a pop-up")
	}
	m, _ = pressKey(t, m, "?")
	if m.help == nil {
		t.Fatal("? did not open help")
	}
	if m.popupID != 0 {
		t.Fatal("help did not cancel the pop-up")
	}
	if v := m.View().Content; strings.Contains(v, "│b.txt│") {
		t.Fatalf("cancelled pop-up still rendered: %q", v)
	}
	m, _ = pressKey(t, m, "esc")
	if m.help != nil {
		t.Fatal("esc did not close help")
	}
	if m.popupID != 0 || strings.Contains(m.View().Content, "│b.txt│") {
		t.Fatal("the cancelled pop-up returned after help closed")
	}
	m, _ = update(t, m, popupExpiredMsg{id: stale})
	if m.popupID != 0 || m.help != nil {
		t.Fatal("the stale expiry changed state")
	}
}
