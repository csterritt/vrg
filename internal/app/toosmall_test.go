package app

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"vrg/internal/theme"
)

// wideFileWithStops is fileWithStops with every line padded out to a
// wide display width, so run-off-edge panning has somewhere to go. The
// recorded lines still carry just the stop text: stale validation
// checks the submatch bytes at their recorded position, which the
// leading "hit" satisfies.
func wideFileWithStops(t *testing.T, dir, name string, lines, pad int, stops ...int) []string {
	t.Helper()
	var b strings.Builder
	for i := 1; i <= lines; i++ {
		if slices.Contains(stops, i) {
			fmt.Fprintf(&b, "hit%05d%s\n", i, strings.Repeat("w", pad))
		} else {
			fmt.Fprintf(&b, "x%06d%s\n", i, strings.Repeat("w", pad))
		}
	}
	writeWorkFile(t, dir, name, b.String())
	recs := []string{fmt.Sprintf(`{"type":"begin","data":{"path":{"text":"%s"}}}`, name)}
	for _, ln := range stops {
		recs = append(recs, fmt.Sprintf(
			`{"type":"match","data":{"path":{"text":"%s"},"lines":{"text":"hit%05d\n"},"line_number":%d,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}`,
			name, ln, ln))
	}
	return append(recs, fmt.Sprintf(`{"type":"end","data":{"path":{"text":"%s"},"binary_offset":null}}`, name))
}

// assertTooSmallView asserts the gate screen is up: the message (or its
// clipped head at pathological widths) and nothing from the screen or
// modals logically beneath it.
func assertTooSmallView(t *testing.T, m Model) {
	t.Helper()
	v := m.View().Content
	if !strings.Contains(v, "Terminal") {
		t.Fatalf("view at %dx%d = %q, want the too-small screen", m.width, m.height, v)
	}
	for _, leak := range []string{"Searching…", "Loading…", "No results found", "┌"} {
		if strings.Contains(v, leak) {
			t.Fatalf("too-small view leaks %q from beneath the gate: %q", leak, v)
		}
	}
}

// shrinkTo resizes the model into the too-small state and asserts the
// gate is showing.
func shrinkTo(t *testing.T, m Model, w, h int) Model {
	t.Helper()
	m, _ = update(t, m, tea.WindowSizeMsg{Width: w, Height: h})
	assertTooSmallView(t, m)
	return m
}

// Below 20 columns or 3 rows the gate replaces the whole screen; at
// exactly 20x3 the ordinary screen shows.
func TestTooSmallThreshold(t *testing.T) {
	for _, tc := range []struct {
		w, h  int
		small bool
	}{
		{19, 24, true},
		{19, 3, true},
		{40, 2, true},
		{20, 2, true},
		{10, 1, true},
		{20, 3, false},
		{21, 4, false},
		{80, 24, false},
	} {
		t.Run(fmt.Sprintf("%dx%d", tc.w, tc.h), func(t *testing.T) {
			m := completedModel(t, 1)
			m, _ = update(t, m, tea.WindowSizeMsg{Width: tc.w, Height: tc.h})
			v := m.View().Content
			if small := strings.Contains(v, "Terminal"); small != tc.small {
				t.Fatalf("at %dx%d too-small shown = %v, want %v: %q",
					tc.w, tc.h, small, tc.small, v)
			}
		})
	}
}

// The message centres itself in whatever space exists — vertically on
// the middle row, horizontally padded — and clips to the frame width
// when the text itself is too wide.
func TestTooSmallMessageCentredAsSpacePermits(t *testing.T) {
	m := completedModel(t, 1)
	m.theme = theme.Plain()
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 19, Height: 10})
	rows := strings.Split(m.View().Content, "\n")
	if len(rows) != 10 {
		t.Fatalf("19x10 frame = %d rows, want 10: %q", len(rows), m.View().Content)
	}
	if got := strings.TrimRight(rows[5], " "); got != "Terminal too small" {
		t.Fatalf("middle row = %q, want the message", got)
	}
	for i, r := range rows {
		if i != 5 && strings.TrimSpace(r) != "" {
			t.Fatalf("row %d = %q, want blank", i, r)
		}
	}

	// Wider than the text: equal padding on both sides.
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 40, Height: 2})
	rows = strings.Split(m.View().Content, "\n")
	if len(rows) != 2 || !strings.HasPrefix(rows[1], strings.Repeat(" ", 11)+"Terminal too small") {
		t.Fatalf("40x2 view = %q, want the message centred on row 1", rows)
	}

	// Narrower than the text: clipped, still alone on the frame.
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 10, Height: 1})
	if v := m.View().Content; v != "Terminal t" {
		t.Fatalf("10x1 view = %q, want the clipped message", v)
	}
}

// q on the too-small screen exits with the state-applicable outcome
// whatever modal is logically open beneath the gate: the program ends,
// it does not merely dismiss.
func TestTooSmallQExitByState(t *testing.T) {
	rows := []struct {
		name  string
		setup func(t *testing.T) Model
		code  int
	}{
		{
			name: "browsing exits the fixed status 0",
			setup: func(t *testing.T) Model {
				return completedModel(t, 1)
			},
			code: 0,
		},
		{
			name: "browsing under a fatal search exits the fixed status 2",
			setup: func(t *testing.T) Model {
				m := newModel(nil, nil)
				m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
				m, _ = update(t, m, searchDoneMsg{
					index:   fixtureIndex(t, "/w", recsOneMatch...),
					waitErr: exitErr(t, 3),
				})
				if m.overlay == nil || m.phase != phaseBrowse {
					t.Fatal("setup: want the fatal-search overlay over browse")
				}
				return m
			},
			code: 2,
		},
		{
			name: "no-results exits 1",
			setup: func(t *testing.T) Model {
				return noResultsModel(t, exitErr(t, 1), recSummary)
			},
			code: 1,
		},
		{
			name: "fatal no-results overlay logically open exits 2",
			setup: func(t *testing.T) Model {
				m := newModel(nil, nil)
				m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
				m, _ = update(t, m, searchDoneMsg{
					index:   fixtureIndex(t, "/w", recSummary),
					waitErr: exitErr(t, 3),
				})
				if m.overlay == nil || m.phase != phaseFatal {
					t.Fatal("setup: want the fatal overlay with no underlying state")
				}
				return m
			},
			code: 2,
		},
		{
			name: "browse error overlay logically open exits rather than dismissing",
			setup: func(t *testing.T) Model {
				m := completedModel(t, 1)
				m, _ = update(t, m, loadDoneMsg{
					path: []byte("fa"),
					req:  mintLoad(&m, "fa"),
					err:  errors.New("denied"),
				})
				if m.overlay == nil {
					t.Fatal("setup: the load failure did not open the overlay")
				}
				return m
			},
			code: 0,
		},
		{
			name: "help logically open exits rather than closing",
			setup: func(t *testing.T) Model {
				m := completedModel(t, 1)
				m, _ = pressKey(t, m, "?")
				if m.help == nil {
					t.Fatal("setup: ? did not open help")
				}
				return m
			},
			code: 0,
		},
		{
			name: "error-over-help stack exits rather than dismissing",
			setup: func(t *testing.T) Model {
				m := completedModel(t, 1)
				m, _ = pressKey(t, m, "?")
				m, _ = update(t, m, loadDoneMsg{
					path: []byte("fa"),
					req:  mintLoad(&m, "fa"),
					err:  errors.New("denied"),
				})
				if m.overlay == nil || m.help == nil {
					t.Fatal("setup: want the error over suspended help")
				}
				return m
			},
			code: 0,
		},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			m := row.setup(t)
			m = shrinkTo(t, m, 19, 4)
			m2, cmd := pressKey(t, m, "q")
			if cmd == nil {
				t.Fatal("q on the too-small screen returned no command, want tea.Quit")
			}
			if _, ok := cmd().(tea.QuitMsg); !ok {
				t.Fatalf("q command = %T, want tea.QuitMsg", cmd())
			}
			if !m2.quit || m2.ExitCode() != row.code {
				t.Fatalf("q: quit=%v code=%d, want quit at %d", m2.quit, m2.ExitCode(), row.code)
			}
		})
	}
}

// q while searching and too small is cancellation — exit 130, the
// child's termination signalled — not the zero-value fixed status.
func TestTooSmallQWhileSearchingCancels(t *testing.T) {
	cancelled := false
	m := newModel(make(chan searchDoneMsg), func() { cancelled = true })
	m = shrinkTo(t, m, 19, 2)
	m2, cmd := pressKey(t, m, "q")
	if cmd == nil {
		t.Fatal("q while searching and too small returned no command, want tea.Quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("q command = %T, want tea.QuitMsg", cmd())
	}
	if m2.ExitCode() != 130 {
		t.Fatalf("ExitCode = %d, want 130", m2.ExitCode())
	}
	if !cancelled {
		t.Fatal("q while searching and too small did not signal child termination")
	}
}

// ctrl+c on the too-small screen takes the cancellation path in every
// state — exit 130 whatever the fixed status would have been.
func TestTooSmallCtrlCExits130(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(t *testing.T) Model
	}{
		{"searching", func(t *testing.T) Model {
			return newModel(make(chan searchDoneMsg), nil)
		}},
		{"browsing", func(t *testing.T) Model {
			return completedModel(t, 1)
		}},
		{"no-results", func(t *testing.T) Model {
			return noResultsModel(t, exitErr(t, 1), recSummary)
		}},
		{"fatal overlay logically open", func(t *testing.T) Model {
			m := newModel(nil, nil)
			m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
			m, _ = update(t, m, searchDoneMsg{
				index:   fixtureIndex(t, "/w", recSummary),
				waitErr: exitErr(t, 3),
			})
			return m
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := tc.setup(t)
			m = shrinkTo(t, m, 15, 2)
			m2, cmd := pressKey(t, m, "ctrl+c")
			if cmd == nil {
				t.Fatal("ctrl+c on the too-small screen returned no command, want tea.Quit")
			}
			if _, ok := cmd().(tea.QuitMsg); !ok {
				t.Fatalf("ctrl+c command = %T, want tea.QuitMsg", cmd())
			}
			if m2.ExitCode() != 130 {
				t.Fatalf("ExitCode = %d, want 130", m2.ExitCode())
			}
		})
	}
}

// Behind the gate every key other than q and ctrl+c is a no-op —
// including Esc, which must not dismiss the logically open modal stack
// — and a logically open overlay is still open after recovery.
func TestTooSmallKeysAreNoOps(t *testing.T) {
	m := completedModel(t, 1)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 10})
	m, _ = pressKey(t, m, "?")
	for i := 0; i < 4; i++ {
		m, _ = pressKey(t, m, "down")
	}
	if m.help == nil || m.help.scroll != 4 {
		t.Fatalf("setup: help=%v, want open at scroll 4", m.help)
	}
	m, _ = update(t, m, loadDoneMsg{
		path: []byte("fa"),
		req:  mintLoad(&m, "fa"),
		err:  tallFailure(),
	})
	if m.overlay == nil || m.help == nil {
		t.Fatal("setup: want the error over suspended help")
	}
	m, _ = pressKey(t, m, "down")
	m, _ = pressKey(t, m, "down")
	if m.overlay.scroll != 2 {
		t.Fatalf("setup: error scroll = %d, want 2", m.overlay.scroll)
	}
	stop, _ := m.currentStop()

	m = shrinkTo(t, m, 15, 2)
	for _, key := range []string{
		"esc", "n", "p", "w", "c", "h", "?", "r", "x", "enter",
		"up", "down", "u", "d", "pgup", "pgdown",
		"left", "right", "tab", "shift+tab", ",", ".", "<", ">", "[", "]",
	} {
		m2, cmd := pressKey(t, m, key)
		if cmd != nil {
			t.Fatalf("%q on the too-small screen produced a command %T, want none", key, cmd)
		}
		if m2.quit || m2.overlay == nil || m2.help == nil ||
			m2.overlay.scroll != 2 || m2.help.scroll != 4 || m2.phase != phaseBrowse {
			t.Fatalf("%q changed state under the gate: quit=%v overlay=%v help=%v phase=%d",
				key, m2.quit, m2.overlay != nil, m2.help != nil, m2.phase)
		}
		m = m2
	}
	assertTooSmallView(t, m)

	// Recovery finds the logically open stack untouched — the hidden
	// error still owns the keyboard and help is still suspended at 4.
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 10})
	if m.overlay == nil || m.overlay.scroll != 2 {
		t.Fatalf("recovered overlay = %v, want open at scroll 2", m.overlay)
	}
	if m.help == nil || m.help.scroll != 4 {
		t.Fatalf("recovered help = %v, want suspended at scroll 4", m.help)
	}
	if s, _ := m.currentStop(); s.Line != stop.Line {
		t.Fatalf("recovery moved the cursor to %+v, want %+v", s, stop)
	}
	m, _ = pressKey(t, m, "esc")
	if m.overlay != nil || m.help == nil || m.help.scroll != 4 {
		t.Fatalf("post-recovery Esc: overlay=%v help=%v, want the error dismissed to help at 4",
			m.overlay != nil, m.help)
	}
}

// A full browse-state round trip: cursor selection, per-file viewport
// state and logical anchors, the list visibility preference, wrap and
// colour settings, and the horizontal offset all survive the gate.
func TestTooSmallRoundTripRestoresBrowseState(t *testing.T) {
	dir := t.TempDir()
	var recs []string
	recs = append(recs, wideFileWithStops(t, dir, "a.txt", 60, 140, 1, 40)...)
	recs = append(recs, wideFileWithStops(t, dir, "b.txt", 30, 140, 10)...)
	recs = append(recs, recSummary)
	m, cmd := browseModel(t, dir, 80, 24, recs...)
	m = settle(t, m, cmd)

	// Build a nontrivial state: scrolled, run-off-edge, panned, the
	// list hidden, the light scheme, and both files holding saved
	// viewport state.
	for i := 0; i < 10; i++ {
		m, _ = update(t, m, codePress(tea.KeyDown))
	}
	m, cmd = pressKey(t, m, "w")
	m = settle(t, m, cmd)
	m, _ = pressKey(t, m, ">")
	m, _ = pressKey(t, m, ">")
	if m.vp.Offset() != 20 {
		t.Fatalf("setup: offset = %d, want 20", m.vp.Offset())
	}
	m, cmd = pressKey(t, m, "tab")
	m = settle(t, m, cmd)
	m, _ = pressKey(t, m, "c")
	m, cmd = pressKey(t, m, "n") // a.txt:1 → a.txt:40, same file
	m = settle(t, m, cmd)
	m, cmd = pressKey(t, m, "n") // a.txt:40 → b.txt:10, crossing
	m = settle(t, m, cmd)
	for i := 0; i < 5; i++ {
		m, _ = update(t, m, codePress(tea.KeyDown))
	}
	m, _ = pressKey(t, m, ">")
	if m.vp.Offset() != 10 {
		t.Fatalf("setup: b.txt offset = %d, want 10", m.vp.Offset())
	}

	stop, _ := m.currentStop()
	anchor := m.vp.Anchor()
	top := m.vp.Top()
	off := m.vp.Offset()
	saved := maps.Clone(m.saved)
	listW, textW := m.listW, m.textW
	listTop := m.listTop
	if len(saved) != 2 {
		t.Fatalf("setup: saved = %v, want per-file state for both files", saved)
	}

	m = shrinkTo(t, m, 15, 2)
	// A no-op key sample under the gate; the state must not budge.
	for _, key := range []string{"n", "down", "w", "esc", "tab"} {
		m, _ = pressKey(t, m, key)
	}
	m, cmd = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m = settle(t, m, cmd)

	if s, _ := m.currentStop(); s.Line != stop.Line || !slices.Equal(s.Path, stop.Path) {
		t.Fatalf("cursor = %+v, want %+v", s, stop)
	}
	if m.vp.Anchor() != anchor {
		t.Fatalf("anchor = %v, want %v", m.vp.Anchor(), anchor)
	}
	if m.vp.Top() != top {
		t.Fatalf("top = %d, want %d", m.vp.Top(), top)
	}
	if m.vp.Offset() != off {
		t.Fatalf("horizontal offset = %d, want %d", m.vp.Offset(), off)
	}
	if !maps.Equal(m.saved, saved) {
		t.Fatalf("saved viewport state = %v, want %v", m.saved, saved)
	}
	if m.listShow {
		t.Fatal("the list visibility preference did not survive the round trip")
	}
	if m.wrap {
		t.Fatal("the wrap-mode setting did not survive the round trip")
	}
	if !m.theme.Light() {
		t.Fatal("the colour scheme did not survive the round trip")
	}
	if m.listW != listW || m.textW != textW || m.listTop != listTop {
		t.Fatalf("cached geometry = list %d text %d top %d, want %d %d %d",
			m.listW, m.textW, m.listTop, listW, textW, listTop)
	}
	if v := m.View().Content; strings.Contains(v, "Terminal too small") {
		t.Fatalf("view after recovery = %q, want the browse screen", v)
	}
}

// The modal stack round trips through the gate at its prior positions:
// scrolled help, a scrolled error overlay, and an error-over-help stack
// all reappear unchanged.
func TestTooSmallRoundTripRestoresModalState(t *testing.T) {
	t.Run("scrolled help", func(t *testing.T) {
		m := completedModel(t, 1)
		m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 10})
		m, _ = pressKey(t, m, "?")
		for i := 0; i < 5; i++ {
			m, _ = pressKey(t, m, "down")
		}
		if m.help == nil || m.help.scroll != 5 {
			t.Fatalf("setup: help=%v, want open at scroll 5", m.help)
		}
		m = shrinkTo(t, m, 19, 2)
		m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 10})
		if m.help == nil || m.help.scroll != 5 {
			t.Fatalf("recovered help = %v, want open at scroll 5", m.help)
		}
		// The title has scrolled off at position 5; the mid-list
		// bindings prove the restored overlay renders at its position.
		if v := m.View().Content; !strings.Contains(v, "scroll one page") {
			t.Fatalf("recovered view lacks help at its scroll: %q", v)
		}
	})

	t.Run("scrolled error overlay", func(t *testing.T) {
		dir := t.TempDir()
		recs := append(fileWithStops(t, dir, "a.txt", 10, 1), recSummary)
		m, cmd := loaderModel(t, dir, 80, 24, failLoaderFor("a.txt", tallFailure()), recs...)
		m = settle(t, m, cmd)
		if m.overlay == nil {
			t.Fatal("setup: the failure did not open the overlay")
		}
		for i := 0; i < 3; i++ {
			m, _ = pressKey(t, m, "down")
		}
		if m.overlay.scroll != 3 {
			t.Fatalf("setup: overlay scroll = %d, want 3", m.overlay.scroll)
		}
		m = shrinkTo(t, m, 40, 2)
		m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
		if m.overlay == nil || m.overlay.scroll != 3 {
			t.Fatalf("recovered overlay = %v, want open at scroll 3", m.overlay)
		}
		// The head line has scrolled off at position 3; the diagnostic
		// body visible mid-overlay proves the restored position.
		if v := m.View().Content; !strings.Contains(v, "context 0") {
			t.Fatalf("recovered view lacks the overlay at its scroll: %q", v)
		}
	})

	t.Run("error over help stack", func(t *testing.T) {
		m := completedModel(t, 1)
		m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 10})
		m, _ = pressKey(t, m, "?")
		for i := 0; i < 4; i++ {
			m, _ = pressKey(t, m, "down")
		}
		m, _ = update(t, m, loadDoneMsg{
			path: []byte("fa"),
			req:  mintLoad(&m, "fa"),
			err:  tallFailure(),
		})
		m, _ = pressKey(t, m, "down")
		m, _ = pressKey(t, m, "down")
		if m.overlay == nil || m.overlay.scroll != 2 || m.help == nil || m.help.scroll != 4 {
			t.Fatalf("setup: overlay=%v help=%v, want error at 2 over help at 4",
				m.overlay, m.help)
		}
		m = shrinkTo(t, m, 15, 2)
		m, _ = pressKey(t, m, "esc") // a no-op — it must not dismiss the hidden stack
		m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 10})
		if m.overlay == nil || m.overlay.scroll != 2 {
			t.Fatalf("recovered overlay = %v, want open at scroll 2", m.overlay)
		}
		if m.help == nil || m.help.scroll != 4 {
			t.Fatalf("recovered help = %v, want suspended at scroll 4", m.help)
		}
		// The restored stack still behaves: the error dismisses to
		// help at 4, which dismisses to browsing.
		m, _ = pressKey(t, m, "esc")
		if m.overlay != nil || m.help == nil || m.help.scroll != 4 {
			t.Fatalf("post-recovery Esc: overlay=%v help=%v, want help restored at 4",
				m.overlay != nil, m.help)
		}
		m, _ = pressKey(t, m, "esc")
		if m.help != nil || m.phase != phaseBrowse || m.quit {
			t.Fatalf("second Esc: help=%v phase=%d quit=%v, want browsing",
				m.help != nil, m.phase, m.quit)
		}
	})
}

// Resizes wholly inside the too-small state keep the gate installed:
// no ordinary layout is minted at the pathological size, no anchor or
// saved state moves, no modal partially restores — and recovery uses
// the final dimensions.
func TestTooSmallInteriorResizeDefersRecovery(t *testing.T) {
	run := func(t *testing.T, withError bool) {
		dir := t.TempDir()
		recs := append(fileWithStops(t, dir, "a.txt", 60, 30), recSummary)
		m, cmd := browseModel(t, dir, 80, 24, recs...)
		m = settle(t, m, cmd)
		// Nontrivial viewport state: scrolled off the top of file.
		for i := 0; i < 10; i++ {
			m, _ = update(t, m, codePress(tea.KeyDown))
		}
		// A scrollable modal: shrink to 80x10 so help scrolls, open
		// it to row 5, optionally park an error over it at row 2.
		m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 10})
		m, _ = pressKey(t, m, "?")
		for i := 0; i < 5; i++ {
			m, _ = pressKey(t, m, "down")
		}
		if withError {
			m, _ = update(t, m, loadDoneMsg{
				path: []byte("a.txt"),
				req:  mintLoad(&m, "a.txt"),
				err:  tallFailure(),
			})
			m, _ = pressKey(t, m, "down")
			m, _ = pressKey(t, m, "down")
			if m.overlay == nil || m.overlay.scroll != 2 {
				t.Fatalf("setup: overlay=%v, want the error at scroll 2", m.overlay)
			}
		}
		if m.help == nil || m.help.scroll != 5 {
			t.Fatalf("setup: help=%v, want open at scroll 5", m.help)
		}
		anchor := m.vp.Anchor()
		saved := maps.Clone(m.saved)
		vpH := m.vp.Height()
		textW := m.textW

		// Enter the gate, then a wholly interior resize.
		m, cmd = update(t, m, tea.WindowSizeMsg{Width: 19, Height: 2})
		if cmd != nil {
			t.Fatalf("the 19x2 resize produced a command %T, want none under the gate", cmd)
		}
		assertTooSmallView(t, m)
		m, cmd = update(t, m, tea.WindowSizeMsg{Width: 10, Height: 1})
		if cmd != nil {
			t.Fatalf("the 10x1 resize produced a command %T — no layout may be minted", cmd)
		}
		if len(m.reqKey) != 0 {
			t.Fatalf("the 10x1 resize minted layout requests: %v", m.reqKey)
		}
		if m.vp.Anchor() != anchor || !maps.Equal(m.saved, saved) {
			t.Fatalf("the 10x1 resize moved state: anchor=%v saved=%v, want %v %v",
				m.vp.Anchor(), m.saved, anchor, saved)
		}
		if m.vp.Height() != vpH || m.textW != textW {
			t.Fatalf("the 10x1 resize installed geometry: vp height=%d textW=%d, want %d %d",
				m.vp.Height(), m.textW, vpH, textW)
		}
		if m.help.scroll != 5 || (withError && m.overlay.scroll != 2) {
			t.Fatalf("the 10x1 resize touched modal state: help=%v overlay=%v",
				m.help, m.overlay)
		}
		assertTooSmallView(t, m)

		// Recovery happens at the final dimensions: the layout
		// request is keyed to 25x8 parameters, not the interior ones.
		m, cmd = update(t, m, tea.WindowSizeMsg{Width: 25, Height: 8})
		m = settle(t, m, cmd)
		if m.width != 25 || m.height != 8 {
			t.Fatalf("recovery dimensions = %dx%d, want 25x8", m.width, m.height)
		}
		if m.vp.Anchor() != anchor || !maps.Equal(m.saved, saved) {
			t.Fatalf("recovery lost state: anchor=%v saved=%v, want %v %v",
				m.vp.Anchor(), m.saved, anchor, saved)
		}
		if m.help == nil || m.help.scroll != 5 {
			t.Fatalf("recovered help = %v, want open at scroll 5", m.help)
		}
		if withError && (m.overlay == nil || m.overlay.scroll != 2) {
			t.Fatalf("recovered overlay = %v, want open at scroll 2", m.overlay)
		}
		v := m.View().Content
		if strings.Contains(v, "Terminal too small") {
			t.Fatalf("view after recovery = %q, want the modal stack restored", v)
		}
		if !strings.Contains(v, "┌") {
			t.Fatalf("recovered view lacks the modal box: %q", v)
		}
		if n := len(strings.Split(v, "\n")); n != 8 {
			t.Fatalf("recovered frame = %d rows, want the 25x8 layout's 8", n)
		}
	}
	t.Run("scrolled help and viewport", func(t *testing.T) { run(t, false) })
	t.Run("error over help and viewport", func(t *testing.T) { run(t, true) })
}

// A search completing behind the gate still resolves its outcome — the
// phase and any overlay settle underneath — while the screen stays on
// the gate until growth restores the presentation.
func TestTooSmallCompletionBehindGate(t *testing.T) {
	dir := t.TempDir()
	writeWorkFile(t, dir, "f.txt", "hit\n")
	m := newModel(make(chan searchDoneMsg), nil)
	m = shrinkTo(t, m, 19, 2)

	var cmd tea.Cmd
	m, cmd = update(t, m, searchDoneMsg{index: fixtureIndex(t, dir, recsOneMatch...)})
	if m.phase != phaseBrowse {
		t.Fatalf("phase behind the gate = %d, want browse", m.phase)
	}
	assertTooSmallView(t, m)
	// The startup load still runs under the gate; its completion
	// stores the buffer without installing pathological geometry.
	m = settle(t, m, cmd)
	assertTooSmallView(t, m)
	if m.bufs["f.txt"] == nil {
		t.Fatal("the load completing behind the gate did not store the buffer")
	}

	m, cmd = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m = settle(t, m, cmd)
	if v := m.View().Content; !strings.Contains(v, "hit") || strings.Contains(v, "Terminal") {
		t.Fatalf("recovered view = %q, want the loaded browse screen", v)
	}
}

// An active pop-up's timer keeps running behind the gate: the pop-up
// is never displayed on the too-small screen, an expiry arriving there
// dismisses it so it stays gone after recovery, and the resize itself
// does not restart the instance's timer.
func TestTooSmallPopupTimerContinues(t *testing.T) {
	m, _ := popupModel(t, 8, 4)
	id := m.popupID
	m, cmd := update(t, m, tea.WindowSizeMsg{Width: 15, Height: 2})
	for _, msg := range cmdMsgs(cmd) {
		if _, ok := msg.(popupExpiredMsg); ok {
			t.Fatal("the resize restarted the pop-up timer")
		}
	}
	if m.popupID != id {
		t.Fatal("entering the gate dismissed the pop-up instance")
	}
	assertTooSmallView(t, m)
	if v := m.View().Content; strings.Contains(v, "│b.txt│") {
		t.Fatalf("the pop-up rendered on the too-small screen: %q", v)
	}

	m, _ = update(t, m, popupExpiredMsg{id: id})
	if m.popupID != 0 {
		t.Fatal("the expiry arriving during too-small did not dismiss the pop-up")
	}
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	if m.popupID != 0 || strings.Contains(m.View().Content, "│b.txt│") {
		t.Fatal("the expired pop-up returned after recovery")
	}
}

// A pop-up that has not expired is only hidden by the gate — never
// cancelled: after recovery the same instance renders again.
func TestTooSmallPopupHiddenNotCancelled(t *testing.T) {
	m, _ := popupModel(t, 8, 4)
	id := m.popupID
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 15, Height: 2})
	// Inert keys while gated are complete no-ops — they cannot
	// dismiss a pop-up the user cannot see.
	m, _ = pressKey(t, m, "esc")
	m, _ = pressKey(t, m, "n")
	m, _ = pressKey(t, m, "w")
	if m.popupID != id {
		t.Fatal("a no-op key under the gate dismissed the hidden pop-up")
	}
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	if m.popupID != id {
		t.Fatalf("pop-up instance = %d, want the surviving %d", m.popupID, id)
	}
	if v := m.View().Content; !strings.Contains(v, "│b.txt│") {
		t.Fatalf("the live pop-up did not reappear after recovery: %q", v)
	}
}
