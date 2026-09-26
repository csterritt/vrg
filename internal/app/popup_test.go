package app

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"vrg/internal/theme"
)

// instantPopupTimer substitutes a synchronous expiry command for the
// pop-up's one-second tea.Tick: each instance's timer resolves to its
// own popupExpiredMsg immediately, so tests drive the clock — and
// inspect batch contents — without sleeping.
func instantPopupTimer(m *Model) {
	m.popupTimer = func(id int) tea.Cmd {
		return func() tea.Msg { return popupExpiredMsg{id: id} }
	}
}

// cmdMsgs invokes a command for the messages it yields, flattening
// batches the way the program's event loop would. Instant pop-up
// timers keep every command synchronous.
func cmdMsgs(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	switch msg := cmd().(type) {
	case tea.BatchMsg:
		var out []tea.Msg
		for _, c := range msg {
			out = append(out, cmdMsgs(c)...)
		}
		return out
	default:
		return []tea.Msg{msg}
	}
}

// popupModel drives a two-file browse model into showing the pop-up:
// n steps a.txt:1 → a.txt:3 → b.txt:2, the second step crossing files.
// It returns the crossing's command — the destination's load, since
// the shared fixture suppresses the pop-up timer.
func popupModel(t *testing.T, aLines, bLines int) (Model, tea.Cmd) {
	t.Helper()
	m := twoFileModel(t, t.TempDir(), 80, 24, aLines, bLines)
	m, _ = update(t, m, keyPress("n"))
	if m.popupID != 0 {
		t.Fatal("a same-file step opened a pop-up")
	}
	m, cmd := update(t, m, keyPress("n"))
	if m.popupID == 0 {
		t.Fatal("crossing into b.txt did not open a pop-up")
	}
	return m, cmd
}

// popupBox locates the pop-up's single-line box in a plain-rendered
// frame: the top border's column and row, the box width, and the
// interior text.
func popupBox(t *testing.T, frame string) (x, y, w int, inner string) {
	t.Helper()
	rows := strings.Split(frame, "\n")
	for i, row := range rows {
		if j := strings.Index(row, "┌"); j >= 0 {
			k := strings.Index(row[j:], "┐")
			x = ansi.StringWidth(row[:j])
			w = ansi.StringWidth(row[j : j+k+len("┐")])
			y = i
			mid := rows[i+1]
			c1 := strings.Index(mid, "│")
			c2 := strings.LastIndex(mid, "│")
			if c1 >= 0 && c2 > c1 {
				inner = mid[c1+len("│") : c2]
			}
			return x, y, w, inner
		}
	}
	t.Fatalf("no pop-up box in frame: %q", frame)
	return 0, 0, 0, ""
}

// The pop-up opens at selection — while the destination is still
// loading — centred over the frame and showing the destination's
// single-line safe path. The crossing's command carries both the
// destination's load and a fresh instance's expiry.
func TestPopupOpensCentredAtSelection(t *testing.T) {
	m := twoFileModel(t, t.TempDir(), 80, 24, 8, 4)
	m.theme = theme.Plain()
	instantPopupTimer(&m)
	m, _ = update(t, m, keyPress("n")) // a.txt:1 → a.txt:3, same file
	m, cmd := update(t, m, keyPress("n"))
	v := m.View().Content
	if !strings.Contains(v, "Loading…") {
		t.Fatalf("destination not still loading when the pop-up is up: %q", v)
	}
	x, y, w, inner := popupBox(t, v)
	if inner != "b.txt" || w != 7 {
		t.Fatalf("pop-up interior = %q width %d, want \"b.txt\" in a 7-cell box", inner, w)
	}
	if x != (80-w)/2 || y != (24-3)/2 {
		t.Fatalf("pop-up at (%d,%d), want centred (%d,%d) on 80x24", x, y, (80-w)/2, (24-3)/2)
	}
	var sawLoad, sawExpiry bool
	for _, msg := range cmdMsgs(cmd) {
		switch msg := msg.(type) {
		case loadDoneMsg:
			sawLoad = string(msg.path) == "b.txt"
		case popupExpiredMsg:
			sawExpiry = msg.id == m.popupID
		}
	}
	if !sawLoad || !sawExpiry {
		t.Fatalf("crossing command carried load:%v expiry:%v, want both keyed to instance %d",
			sawLoad, sawExpiry, m.popupID)
	}
}

// Load completion never restarts the pop-up: the destination's
// loadDoneMsg leaves the same instance up and returns no command.
func TestLoadCompletionDoesNotRestartPopup(t *testing.T) {
	m, load := popupModel(t, 8, 4)
	id := m.popupID
	m, cmd := update(t, m, load())
	for _, msg := range cmdMsgs(cmd) {
		if _, ok := msg.(popupExpiredMsg); ok {
			t.Fatal("load completion restarted the pop-up timer")
		}
	}
	if m.popupID != id {
		t.Fatalf("load completion changed the pop-up instance %d → %d", id, m.popupID)
	}
	if v := m.View().Content; !strings.Contains(v, "│b.txt│") {
		t.Fatalf("pop-up gone after load completion: %q", v)
	}
}

// Each pop-up mints a fresh instance: a stale instance's expiry cannot
// dismiss a newer pop-up, while the pop-up's own expiry dismisses it.
func TestPopupInstanceKeyedExpiry(t *testing.T) {
	m, _ := popupModel(t, 8, 4)
	first := m.popupID
	m, _ = update(t, m, keyPress("n")) // b.txt:2 → wraps to a.txt:1
	second := m.popupID
	if second == 0 || second == first {
		t.Fatalf("second crossing kept instance %d (first was %d), want a fresh id", second, first)
	}
	m, _ = update(t, m, popupExpiredMsg{id: first})
	if m.popupID != second {
		t.Fatalf("stale expiry changed the instance %d → %d", second, m.popupID)
	}
	if v := m.View().Content; !strings.Contains(v, "│a.txt│") {
		t.Fatalf("stale expiry dismissed the newer pop-up: %q", v)
	}
	m, _ = update(t, m, popupExpiredMsg{id: second})
	if m.popupID != 0 {
		t.Fatal("the pop-up's own expiry did not dismiss it")
	}
	if v := m.View().Content; strings.Contains(v, "│a.txt│") {
		t.Fatalf("pop-up still rendered after its expiry: %q", v)
	}
}

// Any key press dismisses the pop-up and performs its normal action in
// the same update: down dismisses and scrolls.
func TestPopupKeyDismissalStillActs(t *testing.T) {
	m, load := popupModel(t, 8, 30)
	m = pump(t, m, load()) // b.txt arrives; the pop-up stays up
	if m.popupID == 0 {
		t.Fatal("load completion dismissed the pop-up")
	}
	m, _ = update(t, m, codePress(tea.KeyDown))
	if m.popupID != 0 {
		t.Fatal("the key press did not dismiss the pop-up")
	}
	if m.vp.Top() != 1 {
		t.Fatalf("down did not scroll while dismissing: top=%d, want 1", m.vp.Top())
	}
}

// The quick second n: the key dismisses b.txt's pop-up and navigates
// in the same update, so the cursor lands on a.txt:1 and the
// destination's own fresh pop-up opens.
func TestPopupDismissKeyStillNavigates(t *testing.T) {
	m, _ := popupModel(t, 8, 4)
	m, _ = update(t, m, keyPress("n"))
	if s, _ := m.currentStop(); string(s.Path) != "a.txt" || s.Line != 1 {
		t.Fatalf("n under the pop-up selected %+v, want a.txt:1", s)
	}
	if m.popupID == 0 {
		t.Fatal("the crossing did not open a fresh pop-up")
	}
	if v := m.View().Content; !strings.Contains(v, "│a.txt│") {
		t.Fatalf("the fresh pop-up is not showing a.txt: %q", v)
	}
}

// q dismisses the pop-up and quits in the same update; Esc dismisses
// and does nothing else — dismissal is its only effect on a base
// state.
func TestPopupDismissalKeys(t *testing.T) {
	m, _ := popupModel(t, 8, 4)
	m2, cmd := update(t, m, keyPress("q"))
	if cmd == nil {
		t.Fatal("q over a pop-up returned no command, want tea.Quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("q over a pop-up command = %T, want tea.QuitMsg", cmd())
	}
	if !m2.quit || m2.ExitCode() != 0 {
		t.Fatalf("q over a pop-up: quit=%v code=%d, want quit at 0", m2.quit, m2.ExitCode())
	}

	m, _ = popupModel(t, 8, 4)
	m2, cmd = update(t, m, escPress())
	if cmd != nil {
		t.Fatalf("Esc over a pop-up produced a command %T, want none", cmd)
	}
	if m2.popupID != 0 || m2.quit || m2.phase != phaseBrowse {
		t.Fatalf("Esc over a pop-up: popup=%d quit=%v phase=%d, want dismissal only",
			m2.popupID, m2.quit, m2.phase)
	}
}

// A resize neither dismisses nor restarts the pop-up: centring
// recomputes from the current terminal size at every render, so the
// box recentres on the same instance and no command reissues the
// timer.
func TestPopupRecentresOnResize(t *testing.T) {
	m, _ := popupModel(t, 8, 4)
	m.theme = theme.Plain()
	id := m.popupID
	m, cmd := update(t, m, tea.WindowSizeMsg{Width: 40, Height: 10})
	if cmd != nil {
		t.Fatalf("resize produced a command %T — the timer must not restart", cmd)
	}
	if m.popupID != id {
		t.Fatalf("resize changed the pop-up instance %d → %d", id, m.popupID)
	}
	x, y, w, inner := popupBox(t, m.View().Content)
	if inner != "b.txt" || x != (40-w)/2 || y != (10-3)/2 {
		t.Fatalf("pop-up at (%d,%d) interior %q on 40x10, want centred at (%d,%d)",
			x, y, inner, (40-w)/2, (10-3)/2)
	}
}

// A path too wide for the frame is left-truncated with a leading … —
// the basename tail stays visible — and the truncation recomputes on
// every render size without touching the instance.
func TestPopupLeftTruncatesLongPath(t *testing.T) {
	dir := t.TempDir()
	name := strings.Repeat("d", 100) + ".txt" // 104 cells
	recs := fileWithStops(t, dir, "a.txt", 4, 1)
	recs = append(recs, fileWithStops(t, dir, name, 4, 1)...)
	recs = append(recs, `{"type":"summary","data":{}}`)
	m, cmd := browseModel(t, dir, 80, 24, recs...)
	m = settle(t, m, cmd)
	m.theme = theme.Plain()
	m, _ = update(t, m, keyPress("n"))
	id := m.popupID

	_, _, w, inner := popupBox(t, m.View().Content)
	if w != 80 {
		t.Fatalf("pop-up width = %d, want the full 80-cell frame", w)
	}
	if !strings.HasPrefix(inner, "…") || !strings.HasSuffix(inner, ".txt") ||
		ansi.StringWidth(inner) != 78 {
		t.Fatalf("truncated pop-up interior = %q (width %d), want a leading … and the tail",
			inner, ansi.StringWidth(inner))
	}

	m = pump(t, m, tea.WindowSizeMsg{Width: 120, Height: 24})
	if m.popupID != id {
		t.Fatalf("resize changed the pop-up instance %d → %d", id, m.popupID)
	}
	_, _, w, inner = popupBox(t, m.View().Content)
	if inner != name || w != 106 {
		t.Fatalf("pop-up interior at 120 wide = %q (box %d), want the untruncated path", inner, w)
	}
}

// An error overlay cancels the pop-up with no return: the current
// file's load failure opens the overlay, dismissing it never brings
// the pop-up back, and the cancelled instance's late expiry is inert.
func TestErrorOverlayCancelsPopup(t *testing.T) {
	m, _ := popupModel(t, 8, 4)
	stale := m.popupID
	m, _ = update(t, m, loadDoneMsg{path: []byte("b.txt"), err: errors.New("denied")})
	if m.overlay == nil {
		t.Fatal("the current-file load failure did not open the error overlay")
	}
	if m.popupID != 0 {
		t.Fatal("the error overlay did not cancel the pop-up")
	}
	if v := m.View().Content; !strings.Contains(v, "cannot read b.txt: denied") {
		t.Fatalf("overlay lacks the failure diagnostic: %q", v)
	}
	m, _ = update(t, m, escPress())
	if m.overlay != nil {
		t.Fatal("Esc did not dismiss the overlay")
	}
	if m.popupID != 0 || strings.Contains(m.View().Content, "│b.txt│") {
		t.Fatal("the cancelled pop-up returned after the overlay dismissal")
	}
	m, _ = update(t, m, popupExpiredMsg{id: stale})
	if m.popupID != 0 || m.overlay != nil {
		t.Fatal("the stale expiry changed state")
	}
}

// No pop-up without a file-changing selection: the startup stop and
// its completed load open nothing.
func TestNoPopupWithoutCrossing(t *testing.T) {
	m := twoFileModel(t, t.TempDir(), 80, 24, 8, 4)
	if m.popupID != 0 {
		t.Fatal("startup or its load completion opened a pop-up")
	}
	if v := m.View().Content; strings.Contains(v, "┌") {
		t.Fatalf("startup frame contains a pop-up box: %q", v)
	}
}
