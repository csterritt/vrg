package app

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/safepresentation"
	"vrg/internal/theme"
)

// popupStubTicks substitutes an instantly resolving expiry command for
// the pop-up's real one-second timer, so tests running navigation
// batches never wait on real time. The stub only proves a fresh
// command was issued; tests drive expiry by injecting popupExpireMsg
// with an explicit instance ID.
var popupStubTicks = options{
	popupTimer: func(id int) tea.Cmd {
		return func() tea.Msg { return popupExpireMsg{id: id} }
	},
}

// leafMsgs runs a command — unwrapping batches — and returns every
// leaf message it produced.
func leafMsgs(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range batch {
			out = append(out, leafMsgs(c)...)
		}
		return out
	}
	return []tea.Msg{msg}
}

// navLeafMsgs sends a key through Update and returns the leaf messages
// its returned command produced: the destination's fileLoadedMsg when
// a load was needed and the new instance's popupExpireMsg when a
// file-change pop-up started.
func navLeafMsgs(t *testing.T, m *model, key tea.KeyPressMsg) []tea.Msg {
	t.Helper()
	_, cmd := m.Update(key)
	return leafMsgs(cmd)
}

// deliverLoad feeds the first fileLoadedMsg in msgs back through
// Update — plus the layout completion its preparation request
// produced — and reports whether one was present. Expiry messages are
// never delivered — tests inject them explicitly by instance ID.
func deliverLoad(t *testing.T, m *model, msgs []tea.Msg) bool {
	t.Helper()
	for _, msg := range msgs {
		if lm, ok := msg.(fileLoadedMsg); ok {
			_, lc := m.Update(lm)
			deliverLayout(t, m, lc)
			return true
		}
	}
	return false
}

// navSendsNoLoad asserts a navigation key issued no load command — the
// destination was cached, in flight, or failed. Any pop-up expiry
// command the crossing also issued resolves under the stub and is
// discarded.
func navSendsNoLoad(t *testing.T, m *model, key tea.KeyPressMsg) {
	t.Helper()
	for _, msg := range navLeafMsgs(t, m, key) {
		if _, ok := msg.(fileLoadedMsg); ok {
			t.Fatalf("navigation issued a load command: %#v", msg)
		}
	}
}

// wantPopup asserts a pop-up instance is live and the frame shows a
// bordered row carrying text.
func wantPopup(t *testing.T, m *model, text string) {
	t.Helper()
	if m.popupID == 0 {
		t.Fatal("no live pop-up instance")
	}
	v := viewText(m)
	if !strings.Contains(v, "│ "+text) {
		t.Fatalf("view = %q, want a bordered pop-up row carrying %q", v, text)
	}
}

// popupBox asserts the pop-up's top border sits at the centred
// position for a box of boxW cells at the model's current size and
// returns the frame rows for further assertions.
func popupBox(t *testing.T, m *model, boxW int) []string {
	t.Helper()
	rows := strings.Split(viewText(m), "\n")
	top := (m.height - 3) / 2
	left := (m.width - boxW) / 2
	if left < 0 {
		left = 0
	}
	if i := strings.Index(rows[top], "┌"); i != left {
		t.Fatalf("pop-up border at row %d column %d, want column %d: %q", top, i, left, rows[top])
	}
	if !strings.Contains(rows[top+2], "└") {
		t.Fatalf("pop-up bottom border missing: %q", rows[top+2])
	}
	return rows
}

// Crossing a file boundary starts the pop-up at selection — the box
// shows while the destination is still loading — with a fresh
// instance ID and its own expiry command. Startup and same-file steps
// open none, and the destination's load completion neither dismisses
// the pop-up nor restarts its timer.
func TestPopupStartsAtSelectionNotLoad(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, popupStubTicks)
	idx := navIndex(t, navFiles)
	finishLoad(t, m, startBrowse(t, m, idx))
	path := escapedPath(idx.Files[1])

	// No pop-up at startup: the first selection is not a file change.
	if m.popupID != 0 {
		t.Fatalf("startup opened pop-up instance %d", m.popupID)
	}

	// A same-file step is not a file change: no pop-up, no command.
	if msgs := navLeafMsgs(t, m, keyN); len(msgs) != 0 {
		t.Fatalf("same-file n produced %v, want no commands", msgs)
	}
	if m.popupID != 0 {
		t.Fatalf("same-file n opened pop-up instance %d", m.popupID)
	}

	// Crossing into b.txt starts the first instance: the box shows the
	// escaped path over the still-loading panel, and the transition's
	// command carries the destination's load plus this instance's
	// expiry command.
	msgs := navLeafMsgs(t, m, keyN)
	if m.popupID != 1 {
		t.Fatalf("pop-up instance = %d, want 1", m.popupID)
	}
	var sawLoad, sawExpiry bool
	for _, msg := range msgs {
		switch msg := msg.(type) {
		case fileLoadedMsg:
			sawLoad = true
		case popupExpireMsg:
			sawExpiry = msg.id == m.popupID
		}
	}
	if !sawLoad || !sawExpiry {
		t.Fatalf("nav produced %v: want the load and instance %d's expiry", msgs, m.popupID)
	}
	wantPopup(t, m, path)
	if v := viewText(m); !strings.Contains(v, "Loading…") {
		t.Fatalf("view = %q, want the pop-up shown over the loading panel", v)
	}

	// Load completion neither dismisses nor restarts it: the live
	// instance is unchanged.
	deliverLoad(t, m, msgs)
	if m.popupID != 1 {
		t.Fatalf("load completion changed the live instance to %d", m.popupID)
	}
	wantPopup(t, m, path)
}

// The pop-up box is centred on the current terminal size at every
// render: the single-line bordered path lands mid-frame. The styled
// run pins centring on a styled box — SGR bytes around the border must
// not be mistaken for content cells when the box's width is measured.
func TestPopupCentredOnFrame(t *testing.T) {
	for _, styled := range []bool{false, true} {
		m := newTestModel(fakeChild{res: Result{Code: 0}}, popupStubTicks)
		m.theme = theme.Plain()
		if styled {
			m.theme = theme.Dark()
		}
		idx := navIndex(t, navFiles)
		finishLoad(t, m, startBrowse(t, m, idx)) // 80x24
		navLeafMsgs(t, m, keyN)
		navLeafMsgs(t, m, keyN) // b.txt — pop-up up

		path := escapedPath(idx.Files[1])
		top := (m.height - 3) / 2
		left := (m.width - safepresentation.CellWidth(path) - 4) / 2
		rows := strings.Split(viewText(m), "\n")
		// The top border begins at the centred column — under the
		// styled theme it follows the box's own SGR prefix.
		needle := "┌"
		if styled {
			needle = "\x1b[37;40m" + needle
		}
		if i := strings.Index(rows[top], needle); i != left {
			t.Fatalf("styled=%v: pop-up border at column %d, want %d: %q",
				styled, i, left, rows[top])
		}
		if !strings.Contains(rows[top+1], "│ "+path+" │") {
			t.Fatalf("styled=%v: pop-up content row = %q, want the centred path",
				styled, rows[top+1])
		}
	}
}

// Each file-change pop-up is a fresh instance: an expiry minted for an
// older instance is stale and cannot dismiss the newer pop-up — only
// its own expiry does.
func TestPopupStaleExpiryCannotDismissNewer(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, popupStubTicks)
	idx := navIndex(t, navFiles)
	finishLoad(t, m, startBrowse(t, m, idx))

	navLeafMsgs(t, m, keyN)         // same-file step to a.txt:8
	msgs := navLeafMsgs(t, m, keyN) // b.txt — instance 1
	deliverLoad(t, m, msgs)
	first := m.popupID

	// n on b.txt's last stop wraps to a.txt — a second crossing opens
	// a second, distinct instance even though a.txt is cached and
	// needs no load.
	msgs = navLeafMsgs(t, m, keyN)
	if deliverLoad(t, m, msgs) {
		t.Fatal("wrap to cached a.txt issued a load command")
	}
	second := m.popupID
	if second == first {
		t.Fatalf("second pop-up reused instance %d, want a fresh one", first)
	}
	wantPopup(t, m, escapedPath(idx.Files[0]))

	// The stale expiry for instance 1 is discarded.
	m.Update(popupExpireMsg{id: first})
	if m.popupID != second {
		t.Fatalf("stale expiry dismissed live instance %d", second)
	}
	wantPopup(t, m, escapedPath(idx.Files[0]))

	// Instance 2's own expiry dismisses it.
	m.Update(popupExpireMsg{id: second})
	if m.popupID != 0 {
		t.Fatalf("own expiry left live instance %d", m.popupID)
	}
	if v := viewText(m); strings.Contains(v, "│ "+escapedPath(idx.Files[0])) {
		t.Fatalf("view = %q, dismissed pop-up still renders", v)
	}
}

// Any key press dismisses the pop-up and still performs its normal
// action in the same update — a scroll key scrolls and q quits; the
// pop-up never delays navigation or quitting.
func TestPopupKeyDismissesAndActs(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, popupStubTicks)
	m.theme = theme.Plain()
	idx := navIndex(t, []navFile{
		{name: "a.txt", content: "a1\na2 hit\n", stops: []navStop{{line: 2, start: 3, end: 6}}},
		{name: "b.txt", content: numberedContent("b", 60), stops: []navStop{{line: 5, start: 0, end: 1}}},
	})
	finishLoad(t, m, startBrowse(t, m, idx))

	msgs := navLeafMsgs(t, m, keyN) // to b.txt — pop-up up
	deliverLoad(t, m, msgs)
	wantPopup(t, m, escapedPath(idx.Files[1]))

	// down dismisses the pop-up and scrolls b.txt's viewport in the
	// same update.
	m.Update(keyDown)
	if m.popupID != 0 {
		t.Fatal("down did not dismiss the pop-up")
	}
	if got := m.vps[string(idx.Files[1].Path)].Top(); got != 1 {
		t.Fatalf("down under the pop-up did not scroll: top = %d, want 1", got)
	}

	// n wraps back into a.txt — a fresh pop-up — and q dismisses it
	// while still quitting.
	msgs = navLeafMsgs(t, m, keyN)
	deliverLoad(t, m, msgs)
	wantPopup(t, m, escapedPath(idx.Files[0]))
	_, cmd := m.Update(keyQ)
	if m.popupID != 0 {
		t.Fatal("q did not dismiss the pop-up")
	}
	if !m.quitting {
		t.Fatal("q under the pop-up did not begin the controlled exit")
	}
	runQuittingCmd(t, cmd)
	if m.status != 0 {
		t.Fatalf("status = %d, want the ordinary exit's 0", m.status)
	}
}

// Esc on a pop-up dismisses it and — being a no-op in ordinary
// browsing — does nothing else: no exit, no cursor movement.
func TestPopupEscDismissesOnly(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, popupStubTicks)
	m.theme = theme.Plain()
	idx := navIndex(t, navFiles)
	finishLoad(t, m, startBrowse(t, m, idx))
	navLeafMsgs(t, m, keyN)
	navLeafMsgs(t, m, keyN) // b.txt — pop-up up
	wantPopup(t, m, escapedPath(idx.Files[1]))

	m.Update(keyEsc)
	if m.popupID != 0 {
		t.Fatal("esc did not dismiss the pop-up")
	}
	if m.quitting {
		t.Fatal("esc under the pop-up began an exit; Esc never quits from a base state")
	}
	cur, ok := m.idx.Cursor()
	if !ok || cur.File != 1 || cur.Stop != 0 {
		t.Fatalf("esc moved the cursor to %v ok=%v", cur, ok)
	}
}

// A resize recentres — and re-truncates — the pop-up against the new
// terminal size without dismissing it or restarting its timer: the
// same instance stays live and still answers its own expiry.
func TestPopupRecentresOnResizeWithoutRestart(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, popupStubTicks)
	m.theme = theme.Plain()
	idx := navIndex(t, navFiles)
	finishLoad(t, m, startBrowse(t, m, idx)) // 80x24
	navLeafMsgs(t, m, keyN)
	navLeafMsgs(t, m, keyN) // b.txt — pop-up instance 1
	id := m.popupID

	// The same box recentres on 100x30.
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	if m.popupID != id {
		t.Fatal("resize disturbed the pop-up instance")
	}
	path := escapedPath(idx.Files[1])
	popupBox(t, m, safepresentation.CellWidth(path)+4)

	// Shrinking re-truncates: at 12 columns the interior is 8 cells —
	// a leading … plus the tail of the path — on the box's content row
	// (top = (5-3)/2 = 1, so the path sits on row 2).
	m.Update(tea.WindowSizeMsg{Width: 12, Height: 5})
	rows := strings.Split(viewText(m), "\n")
	if !strings.Contains(rows[2], "…") {
		t.Fatalf("narrowed pop-up row = %q, want a truncated …-led path", rows[2])
	}
	for i, row := range rows {
		if w := safepresentation.CellWidth(row); w > 12 {
			t.Fatalf("row %d is %d cells at a 12-cell frame: %q", i, w, row)
		}
	}

	// The original instance still answers its expiry — the timer was
	// never restarted.
	m.Update(popupExpireMsg{id: id})
	if m.popupID != 0 {
		t.Fatal("the live instance's expiry did not dismiss it after resizes")
	}
}

// An error overlay arriving while the pop-up is up cancels it, and the
// pop-up does not return after the overlay is dismissed. Issue #26
// owns which diagnostics open the overlay; here the arrival itself is
// exercised through openOverlay over a failed destination load.
func TestPopupCancelledByErrorOverlay(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, popupStubTicks)
	m.theme = theme.Plain()
	idx := navIndex(t, navFiles)
	finishLoad(t, m, startBrowse(t, m, idx))
	navLeafMsgs(t, m, keyN)
	navLeafMsgs(t, m, keyN) // b.txt — pop-up up, load still in flight
	wantPopup(t, m, escapedPath(idx.Files[1]))

	// The destination's read fails — its placeholder is diagnostic
	// state only — then an error overlay arrives over it.
	m.Update(fileLoadedMsg{path: idx.Files[1].Path, req: reqOf(m, idx.Files[1].Path), err: errors.New("denied")})
	m.openOverlay(loadDiag(idx.Files[1].Path, errors.New("denied")), false)
	if !m.overlayOpen {
		t.Fatal("the error overlay did not open")
	}
	if m.popupID != 0 {
		t.Fatal("the overlay did not cancel the pop-up")
	}
	if v := viewText(m); !strings.Contains(v, "cannot read") {
		t.Fatalf("view = %q, want the failure diagnostic in the overlay", v)
	}

	// Dismissing the overlay resumes browsing; the pop-up does not
	// return and the failed file shows its placeholder.
	m.Update(keyEsc)
	if m.overlayOpen {
		t.Fatal("esc did not dismiss the overlay")
	}
	if m.popupID != 0 {
		t.Fatal("the pop-up returned after the overlay closed")
	}
	if v := viewText(m); strings.Contains(v, "│ "+escapedPath(idx.Files[1])) || !strings.Contains(v, "(unreadable)") {
		t.Fatalf("view = %q, want (unreadable) without the pop-up", v)
	}
}

// A path too wide for the frame is left-truncated with a leading …,
// keeping the basename end visible; the box never exceeds the
// terminal width.
func TestPopupLeftTruncatesLongPath(t *testing.T) {
	long := "z" + strings.Repeat("e", 57) + ".txt"
	m := newTestModel(fakeChild{res: Result{Code: 0}}, popupStubTicks)
	m.theme = theme.Plain()
	idx := navIndex(t, []navFile{
		{name: "a.txt", content: "a1\na2 hit\n", stops: []navStop{{line: 2, start: 3, end: 6}}},
		{name: long, content: "bee\n", stops: []navStop{{line: 1, start: 0, end: 3}}},
	})
	finishLoad(t, m, startBrowse(t, m, idx))
	navLeafMsgs(t, m, keyN) // into the long-named file — pop-up up

	// The resolved path is longer than the frame: at 20 columns the
	// interior is 16 cells — a leading … plus the path's tail ending
	// in ".txt".
	m.Update(tea.WindowSizeMsg{Width: 20, Height: 7})
	v := viewText(m)
	boxRow := ""
	for _, row := range strings.Split(v, "\n") {
		if strings.Contains(row, "│") {
			boxRow = row
			break
		}
	}
	if boxRow == "" {
		t.Fatalf("view = %q, no pop-up row", v)
	}
	if !strings.HasPrefix(boxRow, "│ …") {
		t.Fatalf("pop-up row = %q, want a leading …", boxRow)
	}
	if !strings.HasSuffix(boxRow, ".txt │") {
		t.Fatalf("pop-up row = %q, want the basename tail visible", boxRow)
	}
	if w := safepresentation.CellWidth(boxRow); w > 20 {
		t.Fatalf("pop-up row is %d cells at a 20-cell frame: %q", w, boxRow)
	}
}

// The pop-up path is the single-line escaped form: control bytes in
// the raw path can never reach the terminal, and an embedded newline
// cannot add rows.
func TestPopupEscapesHostilePath(t *testing.T) {
	name := "bad\nname\x1b\xff.txt"
	m := newTestModel(fakeChild{res: Result{Code: 0}}, popupStubTicks)
	m.theme = theme.Plain()
	idx := navIndex(t, []navFile{
		{name: "a.txt", content: "a1\na2 hit\n", stops: []navStop{{line: 2, start: 3, end: 6}}},
		{name: name, content: "bee\n", stops: []navStop{{line: 1, start: 0, end: 3}}},
	})
	finishLoad(t, m, startBrowse(t, m, idx))
	navLeafMsgs(t, m, keyN)

	want := escapedPath(idx.Files[1])
	v := viewText(m)
	if !strings.Contains(v, "│ "+want+" │") {
		t.Fatalf("view = %q, want the pop-up showing %q", v, want)
	}
	if n := strings.Count(v, "\n") + 1; n != 24 {
		t.Fatalf("view has %d rows, want 24 — a raw newline would add rows", n)
	}
}
