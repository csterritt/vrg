package app

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/theme"
	"vrg/internal/viewport"
)

// resizeTo sends a terminal size through Update and returns the
// command it issued — nil while the too-small gate is up.
func resizeTo(t *testing.T, m *model, w, h int) tea.Cmd {
	t.Helper()
	_, cmd := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return cmd
}

// updCmd sends a message through Update and returns its command,
// failing when none was issued.
func updCmd(t *testing.T, m *model, msg tea.Msg) tea.Cmd {
	t.Helper()
	_, cmd := m.Update(msg)
	if cmd == nil {
		t.Fatalf("%T issued no command, want one", msg)
	}
	return cmd
}

// Below the fixed minimum — under 20 columns or under 3 rows — the
// whole frame is the centred "Terminal too small" note, clipped to
// what fits; no state, overlay, or pop-up presentation composites over
// it. At exactly 20x3 the ordinary layout renders.
func TestTooSmallThresholdAndDisplay(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Stdout: []byte(happyStream)}}, options{})
	m.theme = theme.Plain()

	// 19 columns is under the width minimum: the note centres on row
	// (10-1)/2 = 4 and nothing else renders.
	if cmd := resizeTo(t, m, 19, 10); cmd != nil {
		t.Fatal("the too-small resize issued a command, want none")
	}
	v := viewText(m)
	rows := strings.Split(v, "\n")
	if len(rows) != 5 || rows[4] != "Terminal too small" {
		t.Fatalf("19x10 view = %q, want the note alone on row 4", v)
	}
	for i := 0; i < 4; i++ {
		if strings.TrimSpace(rows[i]) != "" {
			t.Fatalf("19x10 row %d = %q, want blank — nothing composites over the gate",
				i, rows[i])
		}
	}
	if strings.Contains(v, "Searching") {
		t.Fatalf("19x10 view = %q, the searching screen composited over the gate", v)
	}

	// 2 rows is under the height minimum even with width to spare:
	// the note horizontally centres at column (40-18)/2 = 11.
	resizeTo(t, m, 40, 2)
	if v := viewText(m); strings.Index(v, "Terminal too small") != 11 {
		t.Fatalf("40x2 view = %q, want the note centred at column 11", v)
	}

	// As space permits: a 10-column frame clips the note rather than
	// overflowing the terminal.
	resizeTo(t, m, 10, 5)
	if v := viewText(m); strings.Split(v, "\n")[2] != "Terminal t" {
		t.Fatalf("10x5 view = %q, want the note clipped to ten cells on row 2", v)
	}

	// The boundary itself is ordinary: at 20x3 the searching screen
	// renders, not the gate.
	resizeTo(t, m, 20, 3)
	if v := viewText(m); !strings.Contains(v, "Searching") || strings.Contains(v, "Terminal too small") {
		t.Fatalf("20x3 view = %q, want the ordinary searching screen", v)
	}
}

// q on the too-small screen exits with the state-applicable outcome —
// 130 while searching, the fixed search-derived status while browsing,
// 1 on the no-results screen, 2 with the fatal no-results overlay
// logically open — and it exits past a logically open overlay rather
// than merely dismissing it: that precedence beats Issue #32's
// dismissal semantics so the screen never traps the user behind an
// invisible modal.
func TestTooSmallQExitsPerState(t *testing.T) {
	// Still searching — including collection — is cancellation.
	t.Run("searching exits 130", func(t *testing.T) {
		m := newTestModel(newKillChild(Result{Code: -1}), options{})
		resizeTo(t, m, 19, 10)
		_, cmd := m.Update(keyQ)
		if !m.quitting {
			t.Fatal("q on the too-small screen did not begin a controlled exit")
		}
		runQuittingCmd(t, cmd)
		if m.status != 130 {
			t.Fatalf("status = %d, want the searching cancellation's 130", m.status)
		}
	})

	// Ordinary browsing exits the fixed search-derived status — here
	// the clean run's 0.
	t.Run("browse exits the fixed status", func(t *testing.T) {
		m := newTestModel(newKillChild(Result{Code: 0}), options{})
		m.Update(doneMsg())
		resizeTo(t, m, 19, 10)
		_, cmd := m.Update(keyQ)
		if !m.quitting {
			t.Fatal("q on the too-small screen did not begin a controlled exit")
		}
		runQuittingCmd(t, cmd)
		if m.status != 0 {
			t.Fatalf("status = %d, want the fixed 0", m.status)
		}
	})

	// A browse error overlay logically open does not demote q to a
	// dismissal: the program exits with the fixed fatal status while
	// the overlay stays logically open.
	t.Run("browse error overlay exits not dismisses", func(t *testing.T) {
		m := overlayModel(t, "boom\n") // fatal code 3 over browse: fixed 2
		resizeTo(t, m, 19, 10)
		_, cmd := m.Update(keyQ)
		if !m.quitting {
			t.Fatal("q under a logically open overlay did not exit")
		}
		runQuittingCmd(t, cmd)
		if m.status != 2 {
			t.Fatalf("status = %d, want the fixed 2", m.status)
		}
		if !m.overlayOpen {
			t.Fatal("q dismissed the logically open overlay instead of exiting past it")
		}
	})

	// The same fixed status applies once the overlay is dismissed
	// before shrinking — the outcome is state-derived, not
	// overlay-derived.
	t.Run("browse after dismissal exits the fixed status", func(t *testing.T) {
		m := overlayModel(t, "boom\n")
		m.Update(keyEsc)
		if m.overlayOpen {
			t.Fatal("Esc did not dismiss the overlay at ordinary size")
		}
		resizeTo(t, m, 19, 10)
		_, cmd := m.Update(keyQ)
		runQuittingCmd(t, cmd)
		if m.status != 2 {
			t.Fatalf("status = %d, want the fixed 2", m.status)
		}
	})

	// The no-results screen exits 1.
	t.Run("no-results exits 1", func(t *testing.T) {
		m := newTestModel(fakeChild{res: Result{Stdout: []byte(emptyStream), Code: 1}}, options{})
		m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
		m.Update(runCollectCmd(t, m))
		if m.state != stateNoResults {
			t.Fatalf("state = %v, want no-results", m.state)
		}
		resizeTo(t, m, 19, 10)
		_, cmd := m.Update(keyQ)
		if !m.quitting {
			t.Fatal("q on the too-small screen did not begin a controlled exit")
		}
		runQuittingCmd(t, cmd)
		if m.status != 1 {
			t.Fatalf("status = %d, want the empty search's exit 1", m.status)
		}
	})

	// A warning overlay logically open over no-results does not demote
	// q either: exit 1, overlay still logically open.
	t.Run("no-results warning overlay exits 1", func(t *testing.T) {
		m := newTestModel(fakeChild{res: Result{
			Stdout: []byte(emptyStream), Stderr: []byte("warn\n"), Code: 1,
		}}, options{})
		m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
		m.Update(runCollectCmd(t, m))
		if m.state != stateNoResults || !m.overlayOpen {
			t.Fatalf("state=%v overlay=%v, want the warning overlay over no-results",
				m.state, m.overlayOpen)
		}
		resizeTo(t, m, 40, 2)
		_, cmd := m.Update(keyQ)
		if !m.quitting {
			t.Fatal("q under a logically open warning did not exit")
		}
		runQuittingCmd(t, cmd)
		if m.status != 1 {
			t.Fatalf("status = %d, want the empty search's exit 1", m.status)
		}
		if !m.overlayOpen {
			t.Fatal("q dismissed the logically open overlay instead of exiting past it")
		}
	})

	// The fatal no-results overlay is logically open while too small:
	// q exits 2 — the same status its dismissal would have given, now
	// reached without any dismissal.
	t.Run("fatal overlay exits 2", func(t *testing.T) {
		m := newTestModel(fakeChild{res: Result{
			Stdout: []byte(emptyStream), Stderr: []byte("boom\n"), Code: 3,
		}}, options{})
		m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
		m.Update(runCollectCmd(t, m))
		if m.state != stateOverlayOnly || !m.overlayOpen {
			t.Fatalf("state=%v overlay=%v, want the fatal overlay", m.state, m.overlayOpen)
		}
		resizeTo(t, m, 40, 2)
		_, cmd := m.Update(keyQ)
		if !m.quitting {
			t.Fatal("q under the fatal overlay did not exit")
		}
		runQuittingCmd(t, cmd)
		if m.status != 2 {
			t.Fatalf("status = %d, want the fatal outcome's 2", m.status)
		}
	})
}

// ctrl+c keeps its global precedence on the too-small screen: the
// fixed status is overridden by 130.
func TestTooSmallCtrlCExits130(t *testing.T) {
	m := overlayModel(t, "boom\n") // fixed status 2
	resizeTo(t, m, 19, 10)
	_, cmd := m.Update(keyCtrlC)
	if !m.quitting {
		t.Fatal("ctrl+c on the too-small screen did not begin a controlled exit")
	}
	runQuittingCmd(t, cmd)
	if m.status != 130 {
		t.Fatalf("status = %d, want ctrl+c's 130 override", m.status)
	}
}

// Every key other than q and ctrl+c is a strict no-op on the too-small
// screen — Esc included, which does not dismiss the logically open
// overlay — and nothing behind the gate changes: no scroll, no
// dismissal, no toggle, no load, no cursor movement, no pop-up
// dismissal. Growth restores the logically open overlay exactly as it
// was.
func TestTooSmallKeysAreNoOps(t *testing.T) {
	m := overlayModel(t, "boom\n") // error overlay logically open over browse
	resizeTo(t, m, 19, 10)
	scroll0 := m.overlay.scroll
	loads0 := len(m.loading)
	popup0 := m.popupID
	cur0, _ := m.idx.Cursor()

	for _, k := range []tea.KeyPressMsg{
		keyEsc, keyN, keyP, keyW, keyC, keyR, keyH, keyQuestion,
		keyUp, keyDown, keyU, keyD, keyPgUp, keyPgDn,
		keyLeft, keyRight, keyTab, keyShiftTab,
		{Text: ",", Code: ','}, {Text: ".", Code: '.'},
		{Text: "<", Code: '<'}, {Text: ">", Code: '>'},
		{Text: "[", Code: '['}, {Text: "]", Code: ']'},
		{Code: tea.KeyEnter}, {Code: tea.KeyHome}, {Code: tea.KeyEnd},
		{Text: "x", Code: 'x'},
	} {
		if _, cmd := m.Update(k); cmd != nil {
			t.Fatalf("key %v on the too-small screen returned a command", k)
		}
		if !m.overlayOpen || m.overlay.scroll != scroll0 || m.quitting ||
			m.state != stateBrowse || m.popupID != popup0 {
			t.Fatalf("key %v disturbed the too-small model: overlay=%v scroll=%d "+
				"quitting=%v state=%v popup=%d",
				k, m.overlayOpen, m.overlay.scroll, m.quitting, m.state, m.popupID)
		}
	}
	if cur, _ := m.idx.Cursor(); cur != cur0 {
		t.Fatalf("a swept key moved the cursor to %v behind the gate", cur)
	}
	if len(m.loading) != loads0 {
		t.Fatalf("a swept key changed the load set to %v behind the gate", m.loading)
	}
	if m.theme != theme.Dark() {
		t.Fatal("c toggled the theme behind the gate")
	}
	if !m.wrap {
		t.Fatal("w toggled wrap behind the gate")
	}
	if !m.listVisible {
		t.Fatal("a list key toggled the file list behind the gate")
	}
	if v := viewText(m); !strings.Contains(v, "Terminal too small") ||
		strings.Contains(v, "┌") || strings.Contains(v, "boom") {
		t.Fatalf("too-small view = %q, want only the note — the overlay must not render", v)
	}

	// Recovery reveals the logically open overlay exactly as it was.
	resizeTo(t, m, 80, 24)
	if !m.overlayOpen || m.overlay.scroll != scroll0 {
		t.Fatalf("overlay after recovery: open=%v scroll=%d, want open at %d",
			m.overlayOpen, m.overlay.scroll, scroll0)
	}
	if v := viewText(m); !strings.Contains(v, "boom") {
		t.Fatalf("recovered view = %q, want the overlay's diagnostic back", v)
	}
}

// The modal stack survives a too-small round trip at its prior
// positions: a scrolled help overlay, a scrolled error overlay, and an
// error-over-help stack — help suspended under the error — all reopen
// on growth exactly where they were.
func TestTooSmallRoundTripPreservesModalStack(t *testing.T) {
	t.Run("scrolled help", func(t *testing.T) {
		m := helpBrowseModel(t)
		m.Update(tea.WindowSizeMsg{Width: 80, Height: 8})
		m.Update(keyH)
		for i := 0; i < 5; i++ {
			m.Update(keyDown)
		}
		if m.help.scroll != 5 {
			t.Fatalf("help scroll = %d, want 5 — help must be scrollable", m.help.scroll)
		}
		resizeTo(t, m, 19, 10)
		if v := viewText(m); strings.Contains(v, "Key bindings") || !strings.Contains(v, "Terminal too small") {
			t.Fatalf("too-small view = %q, want only the note — help must not render", v)
		}
		resizeTo(t, m, 80, 8)
		if !m.helpOpen || m.help.scroll != 5 {
			t.Fatalf("help after the round trip: open=%v scroll=%d, want open at 5",
				m.helpOpen, m.help.scroll)
		}
	})

	t.Run("scrolled error", func(t *testing.T) {
		var sb strings.Builder
		for i := 0; i < 30; i++ {
			fmt.Fprintf(&sb, "diag-%03d\n", i)
		}
		m := overlayModel(t, sb.String())
		for i := 0; i < 7; i++ {
			m.Update(keyDown)
		}
		if m.overlay.scroll != 7 {
			t.Fatalf("overlay scroll = %d, want 7 — the diagnostic must be scrollable",
				m.overlay.scroll)
		}
		resizeTo(t, m, 40, 2)
		resizeTo(t, m, 80, 24)
		if !m.overlayOpen || m.overlay.scroll != 7 {
			t.Fatalf("error overlay after the round trip: open=%v scroll=%d, want open at 7",
				m.overlayOpen, m.overlay.scroll)
		}
		if v := viewText(m); !strings.Contains(v, "diag-007") {
			t.Fatalf("recovered view = %q, want the scrolled-to diagnostic row", v)
		}
	})

	t.Run("error over help", func(t *testing.T) {
		rig := newSuspendRig(t)
		rig.fail(t, errors.New("denied: "+strings.Repeat("again ", 120)))
		m := rig.m
		for i := 0; i < 3; i++ {
			m.Update(keyDown)
		}
		if m.overlay.scroll != 3 {
			t.Fatalf("error scroll = %d, want 3 — the diagnostic must be scrollable",
				m.overlay.scroll)
		}
		resizeTo(t, m, 19, 10)
		if !m.overlayOpen || !m.helpOpen ||
			m.overlay.scroll != 3 || m.help.scroll != suspendedHelpScroll {
			t.Fatalf("the too-small resize disturbed the stack: overlay=%v help=%v "+
				"oscroll=%d hscroll=%d",
				m.overlayOpen, m.helpOpen, m.overlay.scroll, m.help.scroll)
		}
		resizeTo(t, m, 80, 8)
		if !m.overlayOpen || !m.helpOpen ||
			m.overlay.scroll != 3 || m.help.scroll != suspendedHelpScroll {
			t.Fatalf("stack after the round trip: overlay=%v help=%v oscroll=%d "+
				"hscroll=%d, want error open at 3 over help at %d",
				m.overlayOpen, m.helpOpen, m.overlay.scroll, m.help.scroll,
				suspendedHelpScroll)
		}
		if v := viewText(m); !strings.Contains(v, "again") {
			t.Fatalf("recovered view = %q, want the scrolled error over the suspended help", v)
		}
	})
}

// A too-small round trip loses nothing of the browse state: the cursor
// selection, each file's saved viewport — top, logical anchor, and
// horizontal offset — the file-list visibility preference, and the
// wrap and colour settings all come back on growth.
func TestTooSmallRoundTripPreservesBrowseState(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, popupStubTicks)
	idx := navIndex(t, []navFile{
		{name: "a.txt",
			content: strings.Repeat("x", 500) + "\n" + numberedContent("pad", 40),
			stops:   []navStop{{line: 1, start: 0, end: 1}, {line: 15, start: 0, end: 3}}},
		{name: "b.txt", content: numberedContent("b", 60), stops: []navStop{{line: 30, start: 0, end: 1}}},
	})
	finishLoad(t, m, startBrowse(t, m, idx))
	keyA := string(idx.Files[0].Path)
	keyB := string(idx.Files[1].Path)

	// A nontrivial state: cursor on a.txt's second stop, a mid-line
	// anchor in a.txt's viewport, run-off-edge mode with a pan
	// offset, the light scheme, and the file list hidden.
	m.Update(keyN)
	for i := 0; i < 10; i++ {
		m.Update(keyUp)
	}
	if a := m.vps[keyA].Anchor(); a.Line != 1 || a.Cell == 0 {
		t.Fatalf("anchor = %+v, want a mid-line anchor on line 1", a)
	}
	deliverLayout(t, m, updCmd(t, m, keyW)) // run-off-edge mode
	for i := 0; i < 2; i++ {
		m.Update(tea.KeyPressMsg{Text: ">", Code: '>'}) // a.txt's pan offset
	}
	m.Update(keyC)                            // light scheme
	deliverLayout(t, m, updCmd(t, m, keyTab)) // file list hidden
	finishLoad(t, m, updCmd(t, m, keyN))      // cross to b.txt — cursor {1,0}
	for i := 0; i < 3; i++ {
		m.Update(keyDown) // b.txt's saved top
	}

	type vpState struct {
		top, off int
		anchor   viewport.Anchor
	}
	snap := func(key string) vpState {
		vp := m.vps[key]
		return vpState{vp.Top(), vp.Off(), vp.Anchor()}
	}
	cur0, _ := m.idx.Cursor()
	a0, b0 := snap(keyA), snap(keyB)
	theme0, wrap0, list0 := m.theme, m.wrap, m.listVisible
	if b0.top == 0 || a0.off == 0 || wrap0 || list0 || theme0 == theme.Dark() {
		t.Fatalf("fixture not nontrivial: a=%+v b=%+v wrap=%v list=%v theme=%v",
			a0, b0, wrap0, list0, theme0)
	}

	resizeTo(t, m, 19, 10)
	resizeTo(t, m, 80, 24)

	if cur, _ := m.idx.Cursor(); cur != cur0 {
		t.Fatalf("cursor = %v after the round trip, want %v", cur, cur0)
	}
	if got := snap(keyA); got != a0 {
		t.Fatalf("a.txt viewport = %+v after the round trip, want %+v", got, a0)
	}
	if got := snap(keyB); got != b0 {
		t.Fatalf("b.txt viewport = %+v after the round trip, want %+v", got, b0)
	}
	if m.theme != theme0 {
		t.Fatal("the colour scheme changed across the gate")
	}
	if m.wrap != wrap0 {
		t.Fatal("the wrap setting changed across the gate")
	}
	if m.listVisible != list0 {
		t.Fatal("the file-list preference changed across the gate")
	}
	if v := viewText(m); strings.Contains(v, "Terminal too small") ||
		!strings.Contains(v, "b line") {
		t.Fatalf("recovered view = %q, want the restored browse frame", v)
	}
}

// Resizes wholly inside the too-small state keep the gate installed:
// no ordinary layout is requested or installed at the pathological
// dimensions, no anchor or modal state is touched, and recovery runs
// at the final size. A scrolled help overlay, an error-over-help
// stack, and a nontrivial viewport all survive 19x2 -> 10x1 -> 25x8
// without loss.
func TestTooSmallResizeWithinGateDefersRecovery(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, popupStubTicks)
	m.theme = theme.Plain()
	idx := navIndex(t, []navFile{{
		name:    "a.txt",
		content: strings.Repeat("x", 500) + "\n" + numberedContent("pad", 20),
		stops:   []navStop{{line: 1, start: 0, end: 1}, {line: 15, start: 0, end: 3}},
	}})
	finishLoad(t, m, startBrowse(t, m, idx))
	key := string(idx.Files[0].Path)

	// A nontrivial viewport: cursor on the second stop and a mid-line
	// anchor scrolled into the wrapped first line.
	m.Update(keyN)
	for i := 0; i < 3; i++ {
		m.Update(keyUp)
	}
	anchor0 := m.vps[key].Anchor()
	if anchor0.Line != 1 || anchor0.Cell == 0 {
		t.Fatalf("anchor = %+v, want a mid-line anchor on line 1", anchor0)
	}
	top0 := m.vps[key].Top()
	cur0, _ := m.idx.Cursor()
	rows0 := m.rows[key]

	// The error-over-help stack, both scrolled.
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 8})
	m.Update(keyH)
	for i := 0; i < 5; i++ {
		m.Update(keyDown)
	}
	if m.help.scroll != 5 {
		t.Fatalf("help scroll = %d, want 5", m.help.scroll)
	}
	m.openOverlay("boom: "+strings.Repeat("again ", 200), false)
	for i := 0; i < 3; i++ {
		m.Update(keyDown)
	}
	if m.overlay.scroll != 3 {
		t.Fatalf("error scroll = %d, want 3", m.overlay.scroll)
	}

	// 19x2 — under both minimums: no layout is requested.
	if cmd := resizeTo(t, m, 19, 2); cmd != nil {
		t.Fatal("the resize into too-small issued a layout command")
	}
	if v := viewText(m); !strings.Contains(v, "Terminal too small") || strings.Contains(v, "┌") {
		t.Fatalf("19x2 view = %q, want only the note", v)
	}

	// 10x1 — deeper into the pathological range: still no ordinary
	// layout, and no anchor or modal mutation.
	if cmd := resizeTo(t, m, 10, 1); cmd != nil {
		t.Fatal("the 10x1 resize issued a layout command")
	}
	if len(m.layoutReqs) != 0 || m.rows[key] != rows0 {
		t.Fatal("an ordinary layout installed inside the gate")
	}
	if got := m.vps[key]; got.Top() != top0 || got.Anchor() != anchor0 {
		t.Fatalf("the viewport mutated inside the gate: %+v", got)
	}
	if !m.overlayOpen || !m.helpOpen || m.overlay.scroll != 3 || m.help.scroll != 5 {
		t.Fatalf("partial modal restoration inside the gate: overlay=%v help=%v "+
			"oscroll=%d hscroll=%d", m.overlayOpen, m.helpOpen, m.overlay.scroll, m.help.scroll)
	}
	if cur, _ := m.idx.Cursor(); cur != cur0 {
		t.Fatal("the cursor moved inside the gate")
	}
	if v := viewText(m); strings.Split(v, "\n")[0] != "Terminal t" {
		t.Fatalf("10x1 view = %q, want the clipped note", v)
	}

	// 25x8 — above the minimum: recovery runs the ordinary path at
	// the final dimensions.
	_, cmd := m.Update(tea.WindowSizeMsg{Width: 25, Height: 8})
	if cmd == nil {
		t.Fatal("recovery issued no layout request at 25x8")
	}
	deliverLayout(t, m, cmd)
	if m.currentRows(key) == nil {
		t.Fatal("no layout installed at the recovery size")
	}
	if !m.overlayOpen || !m.helpOpen || m.overlay.scroll != 3 || m.help.scroll != 5 {
		t.Fatalf("modal stack lost: overlay=%v help=%v oscroll=%d hscroll=%d",
			m.overlayOpen, m.helpOpen, m.overlay.scroll, m.help.scroll)
	}
	vp := m.vps[key]
	if vp.Anchor() != anchor0 {
		t.Fatalf("anchor after recovery = %+v, want %+v", vp.Anchor(), anchor0)
	}
	// The effective top is the row holding the retained anchor under
	// the recovery layout — not the former ordinal.
	rows := m.rows[key].(*viewport.Rows)
	if vp.Top() != rows.RowOf(anchor0) {
		t.Fatalf("top after recovery = %d, want %d — the row containing the anchor",
			vp.Top(), rows.RowOf(anchor0))
	}
	if cur, _ := m.idx.Cursor(); cur != cur0 {
		t.Fatal("the cursor moved across the gate")
	}
	if v := viewText(m); strings.Contains(v, "Terminal too small") {
		t.Fatalf("recovered view = %q, want the restored presentation", v)
	}
}

// A live file-change pop-up is not displayed on the too-small screen
// but its timer keeps running: an expiry landing behind the gate
// dismisses it so it is absent after recovery, while one that never
// expires reappears on growth.
func TestTooSmallPopupContinuesWithoutDisplay(t *testing.T) {
	for _, tc := range []struct {
		name   string
		expire bool
	}{
		{"expiry during too-small dismisses", true},
		{"live pop-up returns on recovery", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestModel(fakeChild{res: Result{Code: 0}}, popupStubTicks)
			m.theme = theme.Plain()
			idx := navIndex(t, navFiles)
			finishLoad(t, m, startBrowse(t, m, idx))
			m.Update(keyN) // same-file step
			m.Update(keyN) // cross to b.txt — the pop-up opens
			id := m.popupID
			if id == 0 {
				t.Fatal("the file crossing opened no pop-up")
			}

			resizeTo(t, m, 19, 10)
			v := viewText(m)
			if !strings.Contains(v, "Terminal too small") || strings.Contains(v, "│") {
				t.Fatalf("too-small view = %q, want the note without the pop-up box", v)
			}
			if tc.expire {
				m.Update(popupExpireMsg{id: id})
				if m.popupID != 0 {
					t.Fatal("the live instance's expiry did not land behind the gate")
				}
			}

			resizeTo(t, m, 80, 24)
			if tc.expire {
				if m.popupID != 0 || strings.Contains(viewText(m), "│") {
					t.Fatal("the expired pop-up reappeared after recovery")
				}
			} else {
				if m.popupID != id {
					t.Fatal("the live pop-up did not survive the round trip")
				}
				// The box is back; the path is longer than the
				// interior so it appears left-truncated — an
				// ellipsis and the path's tail.
				path := escapedPath(idx.Files[1])
				tail := path[len(path)-30:]
				if v := viewText(m); !strings.Contains(v, "│") || !strings.Contains(v, "…") || !strings.Contains(v, tail) {
					t.Fatalf("view lacks the recovered pop-up box:\n%s", v)
				}
			}
		})
	}
}
