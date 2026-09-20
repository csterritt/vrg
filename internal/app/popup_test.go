package app

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/clipperhouse/displaywidth"

	"vrg/internal/searchindex"
	"vrg/internal/theme"
)

// instantPopupTimer substitutes the model's pop-up timer seam: every
// instance's expiry command produces its instance-keyed message
// immediately, so tests drive the timer without touching the clock.
func instantPopupTimer(id int) tea.Cmd {
	return func() tea.Msg { return popupExpiredMsg{id: id} }
}

// navLoadMsg returns the file-load result a navigation command carries.
// A file-change navigation batches the destination's load with the new
// pop-up instance's expiry timer; the load's message is returned and
// the timer is left to the test's injected expiry messages. A nil
// command or a cached destination — no load — yields nil.
func navLoadMsg(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	if cmd == nil {
		return nil
	}
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		if _, isLoad := msg.(loadResult); isLoad {
			return msg
		}
		return nil
	}
	out := make(chan tea.Msg, len(batch))
	for _, c := range batch {
		go func(c tea.Cmd) { out <- c() }(c)
	}
	deadline := time.After(10 * time.Second)
	for range batch {
		select {
		case got := <-out:
			if _, isLoad := got.(loadResult); isLoad {
				return got
			}
		case <-deadline:
			t.Fatal("a navigation command produced no message")
		}
	}
	return nil
}

// deliverNavLoad is navLoadMsg's required form: it fails when the
// navigation carried no load.
func deliverNavLoad(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	msg := navLoadMsg(t, cmd)
	if msg == nil {
		t.Fatal("navigation carried no load")
	}
	return msg
}

// popupBox locates the pop-up's interior row — "│text│" — in the
// composed view and returns its 0-based row and column.
func popupBox(t *testing.T, view, text string) (row, col int) {
	t.Helper()
	want := "│" + text + "│"
	for i, r := range strings.Split(view, "\n") {
		if c := strings.Index(r, want); c >= 0 {
			return i, c
		}
	}
	t.Fatalf("no pop-up row %q in view:\n%s", want, view)
	return -1, -1
}

// twoFileBrowse enters the browse state over a.txt and b.txt at w×h
// with a.txt loaded and the instant pop-up timer installed.
func twoFileBrowse(t *testing.T, w, h int) Model {
	t.Helper()
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", "hit a\n")
	writeMatchFile(t, dir, "b.txt", "hit b\n")
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "hit a\n", 1, 0, 3, "hit"))
	addRec(t, idx, matchRec("b.txt", "hit b\n", 1, 0, 3, "hit"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, w, h)
	m.popupTimer = instantPopupTimer
	m = applyLoad(t, m, cmd())
	return m
}

// Startup shows no pop-up, and neither does a same-file step: the
// pop-up marks file changes only.
func TestPopupOnlyOnFileChange(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", "hit one\nplain\nhit two\n")
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "hit one\n", 1, 0, 3, "hit"))
	addRec(t, idx, matchRec("a.txt", "hit two\n", 3, 0, 3, "hit"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	m = applyLoad(t, m, cmd())
	if m.popup != nil {
		t.Fatal("startup selection opened a pop-up")
	}
	m, nav := update(t, m, keyMsg("n"))
	if m.popup != nil {
		t.Fatal("same-file n opened a pop-up")
	}
	if nav != nil {
		t.Fatalf("same-file n returned a command: %v", nav)
	}
}

// Navigating onto a different file shows the pop-up at selection —
// before the destination finishes loading: a centred bordered box
// naming the escaped path, minting a fresh instance whose expiry
// command rides the navigation's returned batch. Load completion
// neither restarts nor removes the still-running pop-up.
func TestPopupShowsAtSelectionBeforeLoad(t *testing.T) {
	m := twoFileBrowse(t, 80, 24)

	m, cmd := update(t, m, keyMsg("n"))
	if m.popup == nil {
		t.Fatal("cross-file n opened no pop-up")
	}
	id := m.popup.id

	m.theme = theme.Plain()
	v := m.View().Content
	row, col := popupBox(t, v, "b.txt")
	if row != 11 || col != 36 {
		t.Fatalf("pop-up interior at (%d,%d), want the centred (11,36) at 80x24:\n%s", row, col, v)
	}
	if !strings.Contains(v, "Loading…") {
		t.Fatalf("panel lost its loading placeholder under the pop-up:\n%s", v)
	}

	// The returned batch carries the load and this instance's expiry.
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok || len(batch) != 2 {
		t.Fatalf("navigation command produced %#v, want a two-command batch", msg)
	}
	timerSeen := false
	for _, c := range batch {
		if em, ok := c().(popupExpiredMsg); ok {
			timerSeen = true
			if em.id != id {
				t.Fatalf("expiry names instance %d, want the pop-up's %d", em.id, id)
			}
		}
	}
	if !timerSeen {
		t.Fatal("navigation batch carried no expiry command for the pop-up instance")
	}

	// Load completion leaves the running pop-up untouched.
	m = applyLoad(t, m, deliverNavLoad(t, cmd))
	if m.popup == nil || m.popup.id != id {
		t.Fatal("load completion restarted or dismissed the pop-up")
	}
	popupBox(t, m.View().Content, "b.txt")
}

// Each file change mints a fresh instance; an expiry naming a stale
// instance cannot dismiss the newer pop-up, while the pop-up's own
// expiry dismisses it.
func TestPopupExpiryIsInstanceKeyed(t *testing.T) {
	m := twoFileBrowse(t, 80, 24)
	m, _ = update(t, m, keyMsg("n")) // → b.txt
	if m.popup == nil {
		t.Fatal("cross-file n opened no pop-up")
	}
	first := m.popup.id
	m, _ = update(t, m, keyMsg("n")) // wraps → a.txt
	if m.popup == nil || m.popup.id == first {
		t.Fatal("the second file change did not mint a fresh pop-up instance")
	}
	second := m.popup.id

	m, _ = update(t, m, popupExpiredMsg{id: first})
	if m.popup == nil || m.popup.id != second {
		t.Fatal("a stale instance's expiry dismissed the newer pop-up")
	}
	m.theme = theme.Plain()
	popupBox(t, m.View().Content, "a.txt")

	m, _ = update(t, m, popupExpiredMsg{id: second})
	if m.popup != nil {
		t.Fatal("the pop-up's own expiry did not dismiss it")
	}
	if v := m.View().Content; strings.Contains(v, "│a.txt│") {
		t.Fatalf("expired pop-up still renders:\n%s", v)
	}
}

// Any key press dismisses the pop-up and still performs its normal
// action in the same update: an unbound key only dismisses, c toggles
// the scheme, n navigates — minting the next pop-up — Esc is a plain
// dismissal, and q quits.
func TestPopupKeyDismissesAndActs(t *testing.T) {
	m := twoFileBrowse(t, 80, 24)

	m, _ = update(t, m, keyMsg("n")) // → b.txt pop-up
	m, _ = update(t, m, keyMsg("x")) // unbound: dismissal only
	if m.popup != nil {
		t.Fatal("a key press did not dismiss the pop-up")
	}

	m, _ = update(t, m, keyMsg("n")) // wraps → a.txt pop-up
	m, _ = update(t, m, keyMsg("c")) // dismiss + toggle scheme
	if m.popup != nil {
		t.Fatal("c did not dismiss the pop-up")
	}
	if v := m.View().Content; !strings.HasPrefix(v, "\x1b[30;47m") {
		t.Fatalf("c did not toggle the scheme under the pop-up:\n%q", v)
	}

	m, _ = update(t, m, keyMsg("n")) // → b.txt pop-up
	if m.popup == nil {
		t.Fatal("cross-file n opened no pop-up")
	}
	prev := m.popup.id
	m, _ = update(t, m, keyMsg("n")) // dismiss + navigate → a.txt
	if m.popup == nil || m.popup.id == prev {
		t.Fatal("n under a pop-up did not dismiss-and-navigate to a fresh pop-up")
	}
	m.theme = theme.Plain()
	popupBox(t, m.View().Content, "a.txt")

	m, _ = update(t, m, keyMsg("n")) // → b.txt pop-up
	m, cmd := update(t, m, keyPress("esc"))
	if cmd != nil {
		t.Fatalf("Esc under a pop-up returned a command: %v", cmd)
	}
	if m.popup != nil {
		t.Fatal("Esc did not dismiss the pop-up")
	}
	if m.state != stateBrowse {
		t.Fatalf("Esc left the browse state: %d", m.state)
	}

	m, _ = update(t, m, keyMsg("n")) // → a.txt pop-up
	m, cmd = update(t, m, keyMsg("q"))
	requireQuit(t, cmd, "q under a pop-up")
	if m.status != 0 {
		t.Fatalf("q under a pop-up exited %d, want 0", m.status)
	}
}

// Resize neither dismisses the pop-up nor restarts its timer: centring
// and truncation recompute from the current terminal size at every
// render.
func TestPopupRecentresOnResize(t *testing.T) {
	m := twoFileBrowse(t, 80, 24)
	m, _ = update(t, m, keyMsg("n"))
	if m.popup == nil {
		t.Fatal("cross-file n opened no pop-up")
	}
	id := m.popup.id
	m.theme = theme.Plain()

	m, cmd := update(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	if cmd != nil {
		t.Fatalf("resize returned a command — the timer must not restart: %v", cmd)
	}
	if m.popup == nil || m.popup.id != id {
		t.Fatal("resize dismissed or restarted the pop-up")
	}
	row, col := popupBox(t, m.View().Content, "b.txt")
	if row != 14 || col != 46 {
		t.Fatalf("pop-up interior at (%d,%d), want recentred (14,46) at 100x30", row, col)
	}

	m, _ = update(t, m, tea.WindowSizeMsg{Width: 6, Height: 5})
	v := m.View().Content
	row, col = popupBox(t, v, "…txt")
	if row != 2 || col != 0 {
		t.Fatalf("truncated pop-up interior at (%d,%d), want (2,0) at 6x5:\n%s", row, col, v)
	}
	for i, r := range strings.Split(v, "\n") {
		if w := displaywidth.String(r); w > 6 {
			t.Fatalf("row %d is %d cells wide at width 6:\n%q", i, w, r)
		}
	}

	// The original instance's expiry still dismisses after resizes.
	m, _ = update(t, m, popupExpiredMsg{id: id})
	if m.popup != nil {
		t.Fatal("the pop-up's own expiry did not dismiss after resizes")
	}
}

// An error overlay arriving while the pop-up is visible cancels it —
// the pop-up does not return when the overlay is dismissed.
func TestPopupCancelledByErrorOverlay(t *testing.T) {
	m := twoFileBrowse(t, 80, 24)
	m, _ = update(t, m, keyMsg("n")) // current file: b.txt
	if m.popup == nil {
		t.Fatal("no pop-up to cancel")
	}

	m, _ = update(t, m, loadResult{path: []byte("b.txt"), err: errors.New("denied")})
	if m.overlay == nil {
		t.Fatal("the current-file failure opened no error overlay")
	}
	if m.popup != nil {
		t.Fatal("the opening overlay did not cancel the pop-up")
	}
	v := m.View().Content
	if !strings.Contains(v, "cannot read b.txt: denied") {
		t.Fatalf("overlay lacks the load-failure diagnostic:\n%s", v)
	}
	if strings.Contains(v, "│b.txt│") {
		t.Fatalf("cancelled pop-up still renders:\n%s", v)
	}

	m, _ = update(t, m, keyPress("esc"))
	if m.overlay != nil {
		t.Fatal("Esc did not dismiss the overlay")
	}
	if m.popup != nil {
		t.Fatal("the pop-up returned after overlay dismissal")
	}
	if v := m.View().Content; strings.Contains(v, "│b.txt│") {
		t.Fatalf("pop-up rendered after dismissal:\n%s", v)
	}
}

// A path wider than the terminal is left-truncated with a leading …,
// the box spanning the full width without overflowing.
func TestPopupTruncatesLongPath(t *testing.T) {
	dir := t.TempDir()
	long := strings.Repeat("very-long-name-", 4) + "end.txt" // 67 cells
	writeMatchFile(t, dir, "a.txt", "hit a\n")
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "hit a\n", 1, 0, 3, "hit"))
	addRec(t, idx, matchRec(long, "hit z\n", 1, 0, 3, "hit"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 30, 10)
	m = applyLoad(t, m, cmd())
	m, _ = update(t, m, keyMsg("n"))
	m.theme = theme.Plain()

	v := m.View().Content
	want := "…" + long[len(long)-27:] // the 28-cell interior of a 30-wide box
	row, col := popupBox(t, v, want)
	if row != 4 || col != 0 {
		t.Fatalf("pop-up interior at (%d,%d), want (4,0) at 30x10:\n%s", row, col, v)
	}
	for i, r := range strings.Split(v, "\n") {
		if w := displaywidth.String(r); w > 30 {
			t.Fatalf("row %d is %d cells wide at width 30:\n%q", i, w, r)
		}
	}
}
