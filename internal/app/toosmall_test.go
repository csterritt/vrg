package app

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/searchindex"
	"vrg/internal/theme"
)

// Below the fixed 20-column/3-row minimum the whole screen is the
// centred "Terminal too small" message — clipped when even it does not
// fit — and the resize mints no layout work. At exactly 20x3 the
// ordinary screen renders.
func TestTooSmallThresholdAndMessage(t *testing.T) {
	for _, tc := range []struct {
		name string
		w, h int
		row  int    // screen row carrying the message
		text string // that row's exact content
	}{
		{"narrow", 19, 10, 5, "Terminal too small"},
		{"short", 40, 2, 1, strings.Repeat(" ", 11) + "Terminal too small"},
		{"both below", 19, 2, 1, "Terminal too small"},
		{"clips the message", 10, 1, 0, "Terminal t"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := browseLoaded(t, "a.txt", numberedContent(30), 80, 24)
			m, cmd := update(t, m, tea.WindowSizeMsg{Width: tc.w, Height: tc.h})
			if cmd != nil {
				t.Fatalf("resize into %dx%d minted a command: %v", tc.w, tc.h, cmd)
			}
			rows := strings.Split(m.View().Content, "\n")
			if len(rows) != tc.h {
				t.Fatalf("too-small view has %d rows, want %d:\n%s", len(rows), tc.h, m.View().Content)
			}
			if rows[tc.row] != tc.text {
				t.Fatalf("too-small row %d = %q, want %q:\n%s", tc.row, rows[tc.row], tc.text, m.View().Content)
			}
			for i, r := range rows {
				if i != tc.row && r != "" {
					t.Fatalf("too-small row %d = %q, want blank:\n%s", i, r, m.View().Content)
				}
			}
		})
	}

	// Exactly 20x3 is adequate: the ordinary browse screen renders and
	// the resize mints the normal layout request.
	m := browseLoaded(t, "a.txt", numberedContent(30), 80, 24)
	m, cmd := update(t, m, tea.WindowSizeMsg{Width: 20, Height: 3})
	if cmd == nil {
		t.Fatal("resize to 20x3 requested no layout preparation")
	}
	m = deliverCmd(t, m, cmd)
	if v := m.View().Content; strings.Contains(v, "Terminal too small") || !strings.Contains(v, "a.txt") {
		t.Fatalf("20x3 view = %q, want the ordinary browse screen", v)
	}
}

// The gate applies in every state: while searching, the too-small
// screen replaces "Searching…", and growing back restores it.
func TestTooSmallWhileSearching(t *testing.T) {
	m := New(&fakeChild{stdout: strings.NewReader(""), stderr: strings.NewReader("")}, "/w")
	m.theme = theme.Plain()
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 40, Height: 2})
	if v := m.View().Content; !strings.Contains(v, "Terminal too small") || strings.Contains(v, "Searching") {
		t.Fatalf("searching at 40x2 = %q, want the too-small screen", v)
	}
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 40, Height: 24})
	if v := m.View().Content; !strings.Contains(v, "Searching…") {
		t.Fatalf("grown view = %q, want the searching screen back", v)
	}
}

// q under the gate exits with the state-applicable outcome — 130 while
// searching, the fixed status while browsing, 1 on no-results, 2 under
// a logically open fatal overlay — and under a logically open browse
// error overlay or help it still exits rather than merely dismissing.
func TestTooSmallQExitsPerState(t *testing.T) {
	shrink := func(t *testing.T, m Model) Model {
		t.Helper()
		m, _ = update(t, m, tea.WindowSizeMsg{Width: 19, Height: 2})
		if v := m.View().Content; !strings.Contains(v, "Terminal too small") {
			t.Fatalf("setup: 19x2 did not show the too-small screen:\n%s", v)
		}
		return m
	}

	t.Run("searching exits 130", func(t *testing.T) {
		child := &fakeChild{
			stdout:     strings.NewReader(""),
			stderr:     strings.NewReader(""),
			terminated: make(chan struct{}),
		}
		m := New(child, "/w")
		m = shrink(t, m)
		m, cmd := update(t, m, keyMsg("q"))
		requireQuit(t, cmd, "q while searching under the gate")
		if m.status != 130 {
			t.Fatalf("exit status = %d, want 130", m.status)
		}
		requireClosed(t, child.terminated, "child termination")
	})

	t.Run("browse exits the fixed status", func(t *testing.T) {
		m := shrink(t, twoFileBrowse(t, 80, 24))
		m, cmd := update(t, m, keyMsg("q"))
		requireQuit(t, cmd, "q while browsing under the gate")
		if m.status != 0 {
			t.Fatalf("exit status = %d, want the fixed 0", m.status)
		}
	})

	t.Run("no-results exits 1", func(t *testing.T) {
		m := shrink(t, noResultsModel(t))
		m, cmd := update(t, m, keyMsg("q"))
		requireQuit(t, cmd, "q on no-results under the gate")
		if m.status != 1 {
			t.Fatalf("exit status = %d, want 1", m.status)
		}
	})

	t.Run("fatal overlay exits 2", func(t *testing.T) {
		m := overlayModel(t, []string{summaryRec()}, exitError(2), "boom\n")
		if m.overlay == nil || m.state != stateFatal {
			t.Fatalf("setup: state %d overlay %v, want the fatal overlay", m.state, m.overlay != nil)
		}
		m = shrink(t, m)
		m, cmd := update(t, m, keyMsg("q"))
		requireQuit(t, cmd, "q under a logically open fatal overlay")
		if m.status != 2 {
			t.Fatalf("exit status = %d, want 2", m.status)
		}
	})

	t.Run("browse error overlay exits not dismisses", func(t *testing.T) {
		m := overlayModel(t, validRecords(), exitError(2), "boom\n")
		if m.overlay == nil || m.state != stateBrowse {
			t.Fatalf("setup: state %d overlay %v, want browse under the error overlay",
				m.state, m.overlay != nil)
		}
		m = shrink(t, m)
		m, cmd := update(t, m, keyMsg("q"))
		requireQuit(t, cmd, "q under a logically open browse error overlay")
		if m.status != 2 {
			t.Fatalf("exit status = %d, want the fixed 2", m.status)
		}
		if m.overlay == nil {
			t.Fatal("q under the gate dismissed the overlay instead of exiting")
		}
	})

	t.Run("help logically open still exits", func(t *testing.T) {
		m := shrink(t, helpModel(t, "h"))
		m, cmd := update(t, m, keyMsg("q"))
		requireQuit(t, cmd, "q under logically open help")
		if m.status != 0 {
			t.Fatalf("exit status = %d, want the fixed 0", m.status)
		}
		if m.help == nil {
			t.Fatal("q under the gate closed help instead of exiting")
		}
	})
}

// ctrl+c under the gate is cancellation — exit 130 with the child
// terminated — in every state.
func TestTooSmallCtrlCExits130(t *testing.T) {
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
			m, _ = update(t, m, tea.WindowSizeMsg{Width: 19, Height: 2})
			if tc.browse {
				// A search completing behind the gate still transitions
				// state; the gate stays on the shrunken dimensions.
				m, _ = update(t, m, searchResult{index: fixtureIndex(t, 1, 1), integrity: completeStream})
				if m.state != stateBrowse {
					t.Fatalf("setup: completion behind the gate left state %d", m.state)
				}
			}
			m, cmd := update(t, m, keyPress("ctrl+c"))
			requireQuit(t, cmd, "ctrl+c under the gate")
			if m.status != 130 {
				t.Fatalf("exit status = %d, want 130", m.status)
			}
			requireClosed(t, child.terminated, "child termination")
		})
	}
}

// Every key but q and ctrl+c is a strict no-op under the gate: no
// overlay opens or closes, the cursor does not move, no setting
// toggles, no load is minted, and the screen stays the too-small
// message.
func TestTooSmallKeysAreNoops(t *testing.T) {
	m := browseLoaded(t, "a.txt", numberedContent(100), 80, 24)
	for i := 0; i < 10; i++ {
		m, _ = update(t, m, keyPress("down"))
	}
	anchor := m.vps["a.txt"].Anchor()
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 19, Height: 2})

	for _, key := range []string{
		"esc", "n", "p", "w", "c", "h", "?", "r", "x",
		"left", "right", "tab", "up", "down", "u", "d",
		"pgup", "pgdown", ",", ".", "<", ">", "[", "]", "enter", " ",
	} {
		m2, cmd := update(t, m, keyPress(key))
		if cmd != nil {
			t.Fatalf("%s under the gate returned a command: %v", key, cmd)
		}
		m = m2
		if m.state != stateBrowse || m.status != 0 {
			t.Fatalf("%s under the gate changed state %d status %d", key, m.state, m.status)
		}
		if v := m.View().Content; !strings.Contains(v, "Terminal too small") {
			t.Fatalf("%s under the gate changed the screen:\n%s", key, v)
		}
	}

	if stop, _ := m.index.Current(); stop.Line != 1 {
		t.Fatalf("keys under the gate moved the cursor to %+v", stop)
	}
	if got := m.vps["a.txt"].Anchor(); got != anchor {
		t.Fatalf("keys under the gate moved the anchor to %+v, want %+v", got, anchor)
	}
	if m.overlay != nil || m.help != nil {
		t.Fatal("keys under the gate opened an overlay")
	}
	if !m.wrap || !m.listVisible {
		t.Fatalf("keys under the gate changed settings: wrap %v, list %v", m.wrap, m.listVisible)
	}
	if len(m.loading) != 0 || len(m.reloading) != 0 {
		t.Fatalf("keys under the gate minted loads: %v / %v", m.loading, m.reloading)
	}
}

// Esc under the gate does not dismiss a logically open overlay: the
// scrolled error stays open — at its position — across recovery, and
// the hidden overlay never renders on the too-small screen.
func TestTooSmallEscKeepsOverlayOpen(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 40; i++ {
		fmt.Fprintf(&sb, "diag %02d\n", i)
	}
	m := overlayModel(t, validRecords(), nil, sb.String())
	for i := 0; i < 2; i++ {
		m, _ = update(t, m, keyPress("down"))
	}
	if m.overlay == nil || m.overlay.scroll != 2 {
		t.Fatalf("setup: overlay %v scroll %d", m.overlay != nil, m.overlay.scroll)
	}

	m, _ = update(t, m, tea.WindowSizeMsg{Width: 15, Height: 2})
	if v := m.View().Content; strings.Contains(v, "┌") || !strings.Contains(v, "Terminal") {
		t.Fatalf("the logically open overlay renders under the gate:\n%s", v)
	}
	m, cmd := update(t, m, keyPress("esc"))
	if cmd != nil {
		t.Fatalf("Esc under the gate returned a command: %v", cmd)
	}
	if m.overlay == nil || m.overlay.scroll != 2 {
		t.Fatalf("Esc under the gate disturbed the overlay: open %v scroll %d",
			m.overlay != nil, m.overlay.scroll)
	}

	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	if m.overlay == nil || m.overlay.scroll != 2 {
		t.Fatalf("recovery lost the overlay or its scroll: open %v scroll %d",
			m.overlay != nil, m.overlay.scroll)
	}
	if v := m.View().Content; !strings.Contains(v, "diag 02") {
		t.Fatalf("recovered overlay lacks its scroll-2 window:\n%s", v)
	}
}

// An error-over-help stack survives the gate untouched: Esc inside it
// dismisses nothing, and recovery shows the error at its scroll with
// help still suspended underneath at its own — the recovered stack
// then dismisses like ordinary error-over-help.
func TestTooSmallPreservesErrorOverHelp(t *testing.T) {
	m, hs := errorOverHelp(t, 40, 8, 3)
	m, _ = update(t, m, keyPress("down")) // scroll the error to 1
	if m.overlay == nil || m.overlay.scroll != 1 {
		t.Fatalf("setup: error %v scroll %d, want open at 1", m.overlay != nil, m.overlay.scroll)
	}

	m, cmd := update(t, m, tea.WindowSizeMsg{Width: 19, Height: 2})
	if cmd != nil {
		t.Fatalf("resize into the gate returned a command: %v", cmd)
	}
	m, cmd = update(t, m, keyPress("esc"))
	if cmd != nil {
		t.Fatalf("Esc under the gate returned a command: %v", cmd)
	}
	if m.overlay == nil || m.help == nil {
		t.Fatal("Esc under the gate disturbed the error-over-help stack")
	}

	m, _ = update(t, m, tea.WindowSizeMsg{Width: 40, Height: 8})
	if m.overlay == nil || m.overlay.scroll != 1 || m.help == nil || m.help.scroll != hs {
		t.Fatalf("recovery lost the stack: error %v/%d, help %v/%d",
			m.overlay != nil, m.overlay.scroll, m.help != nil, m.help.scroll)
	}
	m, _ = update(t, m, keyPress("esc"))
	if m.overlay != nil || m.help == nil || m.help.scroll != hs {
		t.Fatal("the recovered stack does not dismiss like error-over-help")
	}
}

// The gate is transparent to saved browse state: cursor selection,
// per-file viewports and their logical anchors, horizontal offset, the
// list-visibility preference, wrap mode, and the colour scheme all
// come back — the recovered frame is identical.
func TestTooSmallRoundTripRestoresBrowseState(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", numberedContent(100))
	var b strings.Builder
	for i := 1; i <= 60; i++ {
		fmt.Fprintf(&b, "broad-%02d-%s\n", i, strings.Repeat("x", 120))
	}
	writeMatchFile(t, dir, "b.txt", b.String())
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "line-10\n", 10, 0, 4, "line"))
	addRec(t, idx, matchRec("b.txt", "broad-01-"+strings.Repeat("x", 120)+"\n", 1, 0, 5, "broad"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	m = applyLoad(t, m, cmd())

	// a.txt scrolled state, then b.txt selected and loaded.
	for i := 0; i < 10; i++ {
		m, _ = update(t, m, keyPress("down"))
	}
	m, nav := update(t, m, keyMsg("n"))
	m = applyLoad(t, m, deliverNavLoad(t, nav))

	// b.txt: wrap off and panned, list hidden, light scheme, scrolled.
	m, lay := update(t, m, keyMsg("w"))
	m = deliverCmd(t, m, lay)
	m, lay = update(t, m, keyPress("left"))
	m = deliverCmd(t, m, lay)
	for i := 0; i < 5; i++ {
		m, _ = update(t, m, keyMsg("."))
	}
	for i := 0; i < 6; i++ {
		m, _ = update(t, m, keyPress("down"))
	}
	m, _ = update(t, m, keyMsg("c"))

	stop0, _ := m.index.Current()
	aAnchor, aTop := m.vps["a.txt"].Anchor(), m.vps["a.txt"].Top()
	bAnchor, bTop, bOff := m.vps["b.txt"].Anchor(), m.vps["b.txt"].Top(), m.vps["b.txt"].Offset()
	if aTop == 0 || bTop == 0 || bOff == 0 {
		t.Fatalf("setup: nontrivial state missing: aTop %d, bTop %d, bOff %d", aTop, bTop, bOff)
	}
	wrap0, list0, theme0 := m.wrap, m.listVisible, m.theme
	before := m.View().Content

	m, cmd = update(t, m, tea.WindowSizeMsg{Width: 19, Height: 10})
	if cmd != nil {
		t.Fatalf("resize into the gate minted a command: %v", cmd)
	}
	if v := m.View().Content; !strings.Contains(v, "Terminal too small") || strings.Contains(v, "broad") {
		t.Fatalf("the gate screen leaks the underlying state:\n%s", v)
	}
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})

	if stop, _ := m.index.Current(); stop.Line != stop0.Line || string(stop.Path) != string(stop0.Path) {
		t.Fatalf("round trip moved the cursor to %+v, want %+v", stop, stop0)
	}
	if got := m.vps["a.txt"].Anchor(); got != aAnchor || m.vps["a.txt"].Top() != aTop {
		t.Fatalf("a.txt viewport lost state: %+v top %d, want %+v top %d",
			got, m.vps["a.txt"].Top(), aAnchor, aTop)
	}
	vp := m.vps["b.txt"]
	if vp.Anchor() != bAnchor || vp.Top() != bTop || vp.Offset() != bOff {
		t.Fatalf("b.txt viewport lost state: %+v top %d off %d, want %+v top %d off %d",
			vp.Anchor(), vp.Top(), vp.Offset(), bAnchor, bTop, bOff)
	}
	if m.wrap != wrap0 || m.listVisible != list0 || m.theme != theme0 {
		t.Fatalf("round trip changed settings: wrap %v, list %v, theme %+v",
			m.wrap, m.listVisible, m.theme)
	}
	if got := m.View().Content; got != before {
		t.Fatalf("round trip changed the frame:\nbefore:\n%s\nafter:\n%s", before, got)
	}
}

// Resizes wholly inside the gate install no ordinary layout, move no
// anchor, and restore no modal state early: recovery runs once, at the
// final adequate size.
func TestTooSmallInternalResizeDefersRecovery(t *testing.T) {
	t.Run("scrolled help and viewport", func(t *testing.T) {
		dir := t.TempDir()
		writeMatchFile(t, dir, "a.txt", numberedContent(100))
		idx := searchindex.New(dir)
		addRec(t, idx, matchRec("a.txt", "line-01\n", 1, 0, 4, "line"))
		idx.Finish()
		m, cmd := startBrowse(t, dir, idx, 40, 8)
		m = applyLoad(t, m, cmd())
		m.theme = theme.Plain()

		for i := 0; i < 30; i++ {
			m, _ = update(t, m, keyPress("down"))
		}
		anchor0 := m.vps["a.txt"].Anchor()
		key0 := m.buffers["a.txt"].Key()
		m, _ = update(t, m, keyMsg("h"))
		for i := 0; i < 3; i++ {
			m, _ = update(t, m, keyPress("down"))
		}
		if m.help == nil || m.help.scroll != 3 {
			t.Fatalf("setup: help %v scroll %d", m.help != nil, m.help.scroll)
		}

		for _, size := range []tea.WindowSizeMsg{{Width: 19, Height: 2}, {Width: 10, Height: 1}} {
			m, cmd = update(t, m, size)
			if cmd != nil {
				t.Fatalf("resize to %dx%d inside the gate minted a command: %v",
					size.Width, size.Height, cmd)
			}
			if got := m.buffers["a.txt"].Key(); got != key0 {
				t.Fatalf("resize to %dx%d installed a layout: %+v", size.Width, size.Height, got)
			}
			if got := m.vps["a.txt"].Anchor(); got != anchor0 {
				t.Fatalf("resize to %dx%d moved the anchor to %+v", size.Width, size.Height, got)
			}
			if m.help == nil || m.help.scroll != 3 {
				t.Fatalf("resize to %dx%d disturbed the suspended scroll", size.Width, size.Height)
			}
			if v := m.View().Content; !strings.Contains(v, "Terminal") || strings.Contains(v, "┌") {
				t.Fatalf("help renders under the gate at %dx%d:\n%s", size.Width, size.Height, v)
			}
		}

		// Recovery at the final size requests the fresh layout; its
		// install restores the anchor-derived position, and help is
		// still scrolled to 3.
		m, cmd = update(t, m, tea.WindowSizeMsg{Width: 25, Height: 8})
		if cmd == nil {
			t.Fatal("recovery at 25x8 requested no layout preparation")
		}
		m = deliverCmd(t, m, cmd)
		if got := m.vps["a.txt"].Anchor(); got != anchor0 {
			t.Fatalf("recovery moved the anchor to %+v, want %+v", got, anchor0)
		}
		if got := m.vps["a.txt"].Top(); got != 30 {
			t.Fatalf("recovery top = %d, want the anchor-derived 30", got)
		}
		if m.help == nil || m.help.scroll != 3 {
			t.Fatal("recovery lost the scrolled help overlay")
		}
		top := strings.TrimRight(m.help.rows(23)[3], " ")
		if v := m.View().Content; !strings.Contains(v, top) {
			t.Fatalf("recovered help lacks its scroll-3 top row %q:\n%s", top, v)
		}
	})

	t.Run("error over help", func(t *testing.T) {
		m, hs := errorOverHelp(t, 40, 8, 3)
		m, _ = update(t, m, keyPress("down")) // error scroll 1

		for _, size := range []tea.WindowSizeMsg{{Width: 19, Height: 2}, {Width: 10, Height: 1}} {
			var cmd tea.Cmd
			m, cmd = update(t, m, size)
			if cmd != nil {
				t.Fatalf("resize to %dx%d inside the gate minted a command: %v",
					size.Width, size.Height, cmd)
			}
			if m.overlay == nil || m.overlay.scroll != 1 || m.help == nil || m.help.scroll != hs {
				t.Fatalf("resize to %dx%d disturbed the stack", size.Width, size.Height)
			}
			if v := m.View().Content; strings.Contains(v, "┌") {
				t.Fatalf("the stack renders under the gate at %dx%d:\n%s", size.Width, size.Height, v)
			}
		}

		m, _ = update(t, m, tea.WindowSizeMsg{Width: 25, Height: 8})
		if m.overlay == nil || m.overlay.scroll != 1 || m.help == nil || m.help.scroll != hs {
			t.Fatalf("recovery at 25x8 lost the stack: error %v help %v",
				m.overlay != nil, m.help != nil)
		}
	})
}

// A live pop-up keeps its timer behind the gate and is never painted
// on the too-small screen: an expiry arriving during it dismisses the
// pop-up, which stays gone after recovery — while one that never
// expires reappears with the restored screen.
func TestTooSmallPopupTimerContinuesUnseen(t *testing.T) {
	t.Run("expiry during too-small", func(t *testing.T) {
		m := twoFileBrowse(t, 80, 24)
		m, _ = update(t, m, keyMsg("n")) // → b.txt pop-up
		if m.popup == nil {
			t.Fatal("cross-file n opened no pop-up")
		}
		id := m.popup.id

		m, _ = update(t, m, tea.WindowSizeMsg{Width: 19, Height: 2})
		if v := m.View().Content; strings.Contains(v, "b.txt") || !strings.Contains(v, "Terminal too small") {
			t.Fatalf("the pop-up or the underlying screen renders under the gate:\n%s", v)
		}
		if m.popup == nil {
			t.Fatal("the gate dismissed the pop-up")
		}

		// A no-op key does not dismiss the hidden pop-up either.
		m, cmd := update(t, m, keyMsg("x"))
		if cmd != nil {
			t.Fatalf("x under the gate returned a command: %v", cmd)
		}
		if m.popup == nil {
			t.Fatal("a no-op key dismissed the hidden pop-up")
		}

		m, _ = update(t, m, popupExpiredMsg{id: id})
		if m.popup != nil {
			t.Fatal("the pop-up's own expiry did not dismiss it under the gate")
		}

		m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
		m.theme = theme.Plain()
		if v := m.View().Content; strings.Contains(v, "│b.txt│") {
			t.Fatalf("the expired pop-up rendered after recovery:\n%s", v)
		}
	})

	t.Run("unexpired pop-up reappears", func(t *testing.T) {
		m := twoFileBrowse(t, 80, 24)
		m, _ = update(t, m, keyMsg("n")) // → b.txt pop-up
		id := m.popup.id

		m, _ = update(t, m, tea.WindowSizeMsg{Width: 19, Height: 2})
		m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
		if m.popup == nil || m.popup.id != id {
			t.Fatal("the gate cancelled or restarted the live pop-up")
		}
		m.theme = theme.Plain()
		popupBox(t, m.View().Content, "b.txt")
	})
}
