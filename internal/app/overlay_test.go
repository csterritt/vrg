package app

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"vrg/internal/theme"
)

// overlayModel returns a model whose completed search opened the
// diagnostics overlay: a valid one-match stream, the given child stderr
// and wait error. It fails the test when no overlay opened.
func overlayModel(t *testing.T, stderr string, waitErr error) Model {
	t.Helper()
	dir := t.TempDir()
	writeWorkFile(t, dir, "f.txt", "hit\n")
	m := newModel(nil, nil)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = update(t, m, searchDoneMsg{
		index:   fixtureIndex(t, dir, recsOneMatch...),
		stderr:  []byte(stderr),
		waitErr: waitErr,
	})
	if m.overlay == nil {
		t.Fatalf("no overlay opened (phase %d)", m.phase)
	}
	return m
}

// overlayLines builds a numbered stderr block: "err-00" … "err-(n-1)".
func overlayLines(n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "err-%02d\n", i)
	}
	return b.String()
}

// floodStderr builds the ≥ 1 MiB stderr shape the PTY flood fixture
// emits: a head marker, 525 numbered lines padded to 1999 cells, and a
// tail marker — orders of magnitude more wrapped rows than any usable
// frame can show at once.
func floodStderr() string {
	var b strings.Builder
	b.WriteString("ERRHEAD-MARKER\n")
	for i := 0; i < 525; i++ {
		fmt.Fprintf(&b, "chunk%04d%s\n", i, strings.Repeat("e", 1990))
	}
	b.WriteString("ERRTAIL-MARKER\n")
	return b.String()
}

// The overlay scrolls its diagnostic lines with up/down: the head is
// visible first, the tail only after scrolling, and scrolling clamps at
// both ends instead of drifting past the content.
func TestOverlayScrollsWithUpDown(t *testing.T) {
	m := overlayModel(t, overlayLines(40), nil)

	v := m.View().Content
	if !strings.Contains(v, "err-00") {
		t.Fatalf("overlay head not visible initially: %q", v)
	}
	if strings.Contains(v, "err-39") {
		t.Fatalf("overlay tail visible before scrolling: %q", v)
	}

	// Up at the top is a clamped no-op.
	m, _ = pressKey(t, m, "up")
	if !strings.Contains(m.View().Content, "err-00") {
		t.Fatal("up at the top scrolled past the head")
	}

	// Enough downs reach the tail; extra downs clamp at the bottom.
	for i := 0; i < 40; i++ {
		m, _ = pressKey(t, m, "down")
	}
	v = m.View().Content
	if !strings.Contains(v, "err-39") {
		t.Fatalf("overlay tail not visible after scrolling: %q", v)
	}
	if strings.Contains(v, "err-00") {
		t.Fatalf("overlay head still visible at the bottom: %q", v)
	}

	// Back up returns the head.
	for i := 0; i < 40; i++ {
		m, _ = pressKey(t, m, "up")
	}
	if !strings.Contains(m.View().Content, "err-00") {
		t.Fatal("up did not scroll back to the head")
	}
}

// The overlay's scrollable row set is the complete wrapped diagnostic:
// joining the rows layout derives reproduces every diagnostic line in
// order — nothing elided from the middle, no ellipsis row injected —
// and the set carries the first and last markers of a diagnostic far
// larger than the frame. maxScroll is exactly rows−interior height, so
// every row sits inside some clamped window.
func TestOverlayScrollableSetIsCompleteDiagnostic(t *testing.T) {
	m := overlayModel(t, floodStderr(), nil)
	lines, interiorH, scroll := m.overlay.layout(m.width, m.height)

	if scroll != 0 {
		t.Fatalf("initial scroll = %d, want 0", scroll)
	}
	if len(lines) <= interiorH {
		t.Fatalf("flood diagnostic produced %d rows for interior %d, want a scrollable set",
			len(lines), interiorH)
	}
	if lines[0] != "ERRHEAD-MARKER" {
		t.Fatalf("first scrollable row = %q, want the head marker", lines[0])
	}
	if lines[len(lines)-1] != "ERRTAIL-MARKER" {
		t.Fatalf("last scrollable row = %q, want the tail marker", lines[len(lines)-1])
	}
	var want strings.Builder
	for _, l := range m.overlay.lines {
		want.WriteString(l)
	}
	if got := strings.Join(lines, ""); got != want.String() {
		t.Fatalf("scrollable rows do not reproduce the complete diagnostic: %d rows, %d cells joined, want %d",
			len(lines), len(got), want.Len())
	}
	for i, row := range lines {
		if strings.Contains(row, "…") {
			t.Fatalf("scrollable row %d is an elision row, want complete content: %q", i, row)
		}
	}
	if got, want := m.overlay.maxScroll(m.width, m.height), max(0, len(lines)-interiorH); got != want {
		t.Fatalf("maxScroll = %d, want rows−interior = %d", got, want)
	}
}

// The scroll clamp is [0, rows−interior] in both places it is enforced:
// the key handler stops down exactly at maxScroll — never short, never
// past — and the render path clamps a stored offset outside the range
// to the same bound, so the tail row of an arbitrarily long diagnostic
// stays reachable and no row is ever compressed away. Directly stored
// offsets stand in for the thousands of key presses a PTY traversal of
// a ≥ 1 MiB diagnostic would need.
func TestOverlayScrollClampsToCompleteSet(t *testing.T) {
	m := overlayModel(t, floodStderr(), nil)
	lines, _, _ := m.overlay.layout(m.width, m.height)
	maxScroll := m.overlay.maxScroll(m.width, m.height)

	// Key-handler clamp: the last step still moves, the next is a
	// no-op — the bound is exactly rows−interior, not smaller.
	m.overlay.scroll = maxScroll - 1
	m, _ = pressKey(t, m, "down")
	if m.overlay.scroll != maxScroll {
		t.Fatalf("down at maxScroll−1 left scroll %d, want %d", m.overlay.scroll, maxScroll)
	}
	m, _ = pressKey(t, m, "down")
	if m.overlay.scroll != maxScroll {
		t.Fatalf("down at maxScroll moved scroll to %d, want clamped at %d", m.overlay.scroll, maxScroll)
	}

	// The clamped bottom renders the diagnostic's final row.
	if v := m.View().Content; !strings.Contains(v, "ERRTAIL-MARKER") {
		t.Fatalf("tail marker not rendered at the clamped bottom: %q", v)
	}

	// A middle row is reachable at its own clamped position: no row
	// between head and tail is missing from the set.
	mid := -1
	for i, row := range lines {
		if strings.Contains(row, "chunk0300") {
			mid = i
			break
		}
	}
	if mid < 0 {
		t.Fatal("a middle diagnostic line is absent from the scrollable set")
	}
	m.overlay.scroll = mid
	if v := m.View().Content; !strings.Contains(v, "chunk0300") {
		t.Fatalf("middle row %d not rendered at its scroll position: %q", mid, v)
	}

	// Render-path clamp: a stored offset past either end clamps to the
	// same range — the tail beyond the top, the head below zero.
	m.overlay.scroll = maxScroll + 5000
	if v := m.View().Content; !strings.Contains(v, "ERRTAIL-MARKER") {
		t.Fatalf("an over-large stored offset did not clamp to the tail: %q", v)
	}
	m.overlay.scroll = -5
	if v := m.View().Content; !strings.Contains(v, "ERRHEAD-MARKER") {
		t.Fatalf("a negative stored offset did not clamp to the head: %q", v)
	}
}

// A diagnostic whose wrapped rows only slightly exceed the interior
// height traverses row-by-row: exactly maxScroll downs reach the final
// line and the same number of ups returns to the first, clamping at
// both ends.
func TestOverlayTraversalReachesBothEnds(t *testing.T) {
	m := overlayModel(t, overlayLines(25), nil)
	lines, interiorH, _ := m.overlay.layout(m.width, m.height)
	maxScroll := max(0, len(lines)-interiorH)
	if maxScroll == 0 || maxScroll > 5 {
		t.Fatalf("fixture rows = %d, interior = %d: want a set only slightly taller than the frame",
			len(lines), interiorH)
	}

	for i := 0; i < maxScroll; i++ {
		m, _ = pressKey(t, m, "down")
	}
	if m.overlay.scroll != maxScroll {
		t.Fatalf("bounded traversal stopped at %d, want %d", m.overlay.scroll, maxScroll)
	}
	v := m.View().Content
	if !strings.Contains(v, "err-24") {
		t.Fatalf("final line unreachable after %d downs: %q", maxScroll, v)
	}
	if strings.Contains(v, "err-00") {
		t.Fatalf("first line still visible at the bottom: %q", v)
	}

	for i := 0; i < maxScroll; i++ {
		m, _ = pressKey(t, m, "up")
	}
	if m.overlay.scroll != 0 {
		t.Fatalf("up traversal stopped at %d, want 0", m.overlay.scroll)
	}
	if v := m.View().Content; !strings.Contains(v, "err-00") {
		t.Fatalf("first line unreachable after returning to the top: %q", v)
	}
}

// An error appended while the overlay is open extends the scrollable
// set — its rows join the tail and stay renderable at the clamped
// bottom — without moving the reader's position.
func TestAppendedErrorExtendsScrollableSet(t *testing.T) {
	m := overlayModel(t, overlayLines(30), nil)
	for i := 0; i < 4; i++ {
		m, _ = pressKey(t, m, "down")
	}
	before, _, _ := m.overlay.layout(m.width, m.height)

	m.openOverlay([]string{"late failure row"})

	after, _, scroll := m.overlay.layout(m.width, m.height)
	if scroll != 4 {
		t.Fatalf("the append moved the reader's scroll to %d, want 4", scroll)
	}
	if len(after) != len(before)+1 {
		t.Fatalf("scrollable set grew %d → %d rows, want exactly the appended row",
			len(before), len(after))
	}
	if after[len(after)-1] != "late failure row" {
		t.Fatalf("appended row landed at %q, want the tail of the set", after[len(after)-1])
	}
	m.overlay.scroll = m.overlay.maxScroll(m.width, m.height)
	if v := m.View().Content; !strings.Contains(v, "late failure row") {
		t.Fatalf("the appended row is not renderable at the clamped bottom: %q", v)
	}
}

// q and Esc each dismiss a non-fatal overlay to the underlying screen.
func TestOverlayDismissKeys(t *testing.T) {
	for _, key := range []string{"q", "esc"} {
		t.Run(key, func(t *testing.T) {
			m := overlayModel(t, "rg: warn\n", nil)
			m2, cmd := pressKey(t, m, key)
			if cmd != nil {
				t.Fatalf("%s on a warning overlay produced a command %T, want none", key, cmd)
			}
			if m2.overlay != nil || m2.phase != phaseBrowse || m2.quit {
				t.Fatalf("%s dismissal: overlay=%v phase=%d quit=%v, want browse revealed",
					key, m2.overlay != nil, m2.phase, m2.quit)
			}
		})
	}
}

// ctrl+c inside the overlay takes the cancellation path: quit at 130
// whatever the fixed status would have been.
func TestOverlayCtrlCExits130(t *testing.T) {
	m := overlayModel(t, "rg: warn\n", exitErr(t, 3))
	m2, cmd := pressKey(t, m, "ctrl+c")
	if cmd == nil {
		t.Fatal("ctrl+c on the overlay returned no command, want tea.Quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("ctrl+c command = %T, want tea.QuitMsg", cmd())
	}
	if m2.ExitCode() != 130 {
		t.Fatalf("ExitCode = %d, want 130", m2.ExitCode())
	}
}

// Every other key is ignored while the overlay is open: no dismissal,
// no quit, no theme toggle, no state change of any kind. The browse
// scroll keys u, d, page up, and page down are covered explicitly —
// only up and down act on the modal.
func TestOverlayIgnoresOtherKeys(t *testing.T) {
	m := overlayModel(t, overlayLines(40), nil)
	for _, key := range []string{"x", "n", "c", "0", "left", "right", "tab", "enter", "u", "d", "pgup", "pgdown"} {
		m2, cmd := pressKey(t, m, key)
		if cmd != nil {
			t.Fatalf("%q on the overlay produced a command %T, want none", key, cmd)
		}
		scroll := -1
		if m2.overlay != nil {
			scroll = m2.overlay.scroll
		}
		if m2.overlay == nil || scroll != 0 || m2.quit || m2.phase != phaseBrowse {
			t.Fatalf("%q changed state: overlay=%v scroll=%d quit=%v phase=%d",
				key, m2.overlay != nil, scroll, m2.quit, m2.phase)
		}
	}
	if m.theme.Light() {
		t.Fatal("c reached the colour toggle while the overlay was open")
	}
}

// A long unbroken diagnostic wraps inside the border rather than
// overflowing it: every frame row stays within the terminal width and
// the text occupies several interior rows.
func TestOverlayWrapsUnbrokenDiagnostic(t *testing.T) {
	m := overlayModel(t, strings.Repeat("x", 300)+"\n", exitErr(t, 3))
	m.theme = theme.Plain()
	v := m.View().Content
	for i, row := range strings.Split(v, "\n") {
		if w := ansi.StringWidth(row); w > 80 {
			t.Fatalf("frame row %d is %d cells wide, want ≤80: %q", i, w, row)
		}
	}
	interior := 0
	for _, row := range strings.Split(v, "\n") {
		if strings.HasPrefix(row, "│") {
			interior++
		}
	}
	if interior < 4 {
		t.Fatalf("300-cell diagnostic occupied %d interior rows, want wrapping", interior)
	}
}

// A failed process that wrote no stderr still gets a diagnostic naming
// its exit code or its signal — the overlay never opens empty.
func TestFailedProcessWithoutStderrGetsGeneratedDiagnostic(t *testing.T) {
	m := overlayModel(t, "", exitErr(t, 3))
	if v := m.View().Content; !strings.Contains(v, "exit status 3") {
		t.Fatalf("overlay lacks the generated exit-code diagnostic: %q", v)
	}
	m = overlayModel(t, "", sigErr(t))
	if v := m.View().Content; !strings.Contains(v, "signal: killed") {
		t.Fatalf("overlay lacks the generated signal diagnostic: %q", v)
	}
}
