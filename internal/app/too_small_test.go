package app_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/app"
	"vrg/internal/filebuffer"
	"vrg/internal/theme"
	"vrg/internal/viewport"
)

// --- Issue #33: "Terminal too small" screen with full state recovery ---

// tooSmallView asserts that the model's view shows the too-small screen.
func tooSmallView(t *testing.T, m app.Model) {
	t.Helper()
	if !m.TooSmall() {
		t.Fatalf("TooSmall = false, want true")
	}
	view := viewContent(m)
	if !strings.Contains(view, "Terminal too small") {
		t.Fatalf("view does not contain 'Terminal too small':\n%s", view)
	}
}

// notTooSmallView asserts that the model is not in the too-small state.
func notTooSmallView(t *testing.T, m app.Model) {
	t.Helper()
	if m.TooSmall() {
		t.Fatalf("TooSmall = true, want false")
	}
}

// resizeTooSmall sends a WindowSizeMsg below the 20x3 minimum. The
// returned command is delivered (it should be nil once the too-small
// gate is implemented, but deliverLayout handles nil safely).
func resizeTooSmall(t *testing.T, m app.Model, width, height int) app.Model {
	t.Helper()
	m, cmd := update(t, m, tea.WindowSizeMsg{Width: width, Height: height})
	return deliverLayout(t, m, cmd)
}

// resizeRecover sends a WindowSizeMsg at or above the 20x3 minimum and
// delivers any resulting layout preparation command so the viewport is
// rebuilt at the recovered dimensions.
func resizeRecover(t *testing.T, m app.Model, width, height int) app.Model {
	t.Helper()
	m, cmd := update(t, m, tea.WindowSizeMsg{Width: width, Height: height})
	return deliverLayout(t, m, cmd)
}

// setupTooSmallBrowse creates a browse model at 80x24 with a single
// loaded file, then shrinks to a too-small size. The pre-shrink state
// (cursor, viewport, etc.) is available for recovery assertions.
func setupTooSmallBrowse(t *testing.T) app.Model {
	t.Helper()
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
		textMatch("src/b.go", "world\n", 1, subSpec{"world", 0, 5}),
	)
	buf := makeBuf([]filebuffer.Line{ml(1, "hello world")}, 1, 3)
	m := setupBrowseWithSize(t, idx, buf, 80, 24)
	m = resizeTooSmall(t, m, 19, 10)
	return m
}

// --- Threshold and display ---

// TestTooSmallWidthBelow20 verifies that a terminal width below 20
// columns triggers the too-small screen with the centred message.
func TestTooSmallWidthBelow20(t *testing.T) {
	m := setupTooSmallBrowse(t)
	tooSmallView(t, m)
}

// TestTooSmallHeightBelow3 verifies that a terminal height below 3 rows
// triggers the too-small screen with the centred message.
func TestTooSmallHeightBelow3(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
	)
	buf := makeBuf([]filebuffer.Line{ml(1, "hello world")}, 1, 3)
	m := setupBrowseWithSize(t, idx, buf, 80, 24)
	m = resizeTooSmall(t, m, 40, 2)
	tooSmallView(t, m)
}

// TestTooSmallBoundary20x3NotTooSmall verifies that exactly 20x3 is the
// minimum usable size — the too-small screen is NOT shown.
func TestTooSmallBoundary20x3NotTooSmall(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
	)
	buf := makeBuf([]filebuffer.Line{ml(1, "hello world")}, 1, 3)
	m := setupBrowseWithSize(t, idx, buf, 80, 24)
	m = resizeRecover(t, m, 20, 3)
	notTooSmallView(t, m)
}

// TestTooSmallRecoveryRestoresView verifies that growing back from a
// too-small size restores the normal browse view (no "Terminal too
// small" text).
func TestTooSmallRecoveryRestoresView(t *testing.T) {
	m := setupTooSmallBrowse(t)
	tooSmallView(t, m)
	m = resizeRecover(t, m, 80, 24)
	notTooSmallView(t, m)
	view := viewContent(m)
	if strings.Contains(view, "Terminal too small") {
		t.Fatalf("view still contains 'Terminal too small' after recovery:\n%s", view)
	}
}

// --- Exit semantics: q on the too-small screen ---

// TestTooSmallQDuringSearchingExits130 verifies that q on the too-small
// screen while searching exits with code 130 (cancellation).
func TestTooSmallQDuringSearchingExits130(t *testing.T) {
	m := app.New([]string{"--json", "--", "foo", "."}, "/work")
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m = resizeTooSmall(t, m, 19, 10)
	tooSmallView(t, m)
	m, cmd := update(t, m, keyPress('q'))
	assertQuit(t, cmd)
	if m.ExitCode() != 130 {
		t.Fatalf("ExitCode = %d, want 130", m.ExitCode())
	}
}

// TestTooSmallQWhileBrowsingExitsFixedStatus verifies that q on the
// too-small screen while browsing exits with the fixed search-derived
// status (0 for a clean search with results).
func TestTooSmallQWhileBrowsingExitsFixedStatus(t *testing.T) {
	m := setupTooSmallBrowse(t)
	m, cmd := update(t, m, keyPress('q'))
	assertQuit(t, cmd)
	if m.ExitCode() != 0 {
		t.Fatalf("ExitCode = %d, want 0 (fixed browse status)", m.ExitCode())
	}
}

// TestTooSmallQOnNoResultsExits1 verifies that q on the too-small screen
// on the no-results screen exits with code 1.
func TestTooSmallQOnNoResultsExits1(t *testing.T) {
	idx := buildIndex(t, "/work", summaryRecord())
	m := app.New([]string{"--json", "--", "foo", "."}, "/work")
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	if m.State() != app.StateNoResults {
		t.Fatalf("State = %v, want StateNoResults", m.State())
	}
	m = resizeTooSmall(t, m, 19, 10)
	tooSmallView(t, m)
	m, cmd := update(t, m, keyPress('q'))
	assertQuit(t, cmd)
	if m.ExitCode() != 1 {
		t.Fatalf("ExitCode = %d, want 1 (no-results)", m.ExitCode())
	}
}

// TestTooSmallQFatalNoResultsOverlayExits2 verifies that q on the
// too-small screen with a fatal no-results overlay logically open exits
// with code 2 (not merely dismissing the overlay).
func TestTooSmallQFatalNoResultsOverlayExits2(t *testing.T) {
	m := setupFatalNoResultsOverlay(t)
	m = resizeTooSmall(t, m, 19, 10)
	tooSmallView(t, m)
	// The overlay is logically open but not displayed.
	if !m.OverlayOpen() {
		t.Fatalf("overlay should be logically open during too-small")
	}
	m, cmd := update(t, m, keyPress('q'))
	assertQuit(t, cmd)
	if m.ExitCode() != 2 {
		t.Fatalf("ExitCode = %d, want 2 (fatal no-results)", m.ExitCode())
	}
}

// TestTooSmallQBrowseErrorOverlayExitsProgram verifies that q on the
// too-small screen with a browse error overlay logically open exits the
// program with the fixed status (2) rather than merely dismissing the
// overlay.
func TestTooSmallQBrowseErrorOverlayExitsProgram(t *testing.T) {
	m := setupBrowseErrorOverlay(t)
	m = resizeTooSmall(t, m, 19, 10)
	tooSmallView(t, m)
	if !m.OverlayOpen() {
		t.Fatalf("overlay should be logically open during too-small")
	}
	m, cmd := update(t, m, keyPress('q'))
	assertQuit(t, cmd)
	if m.ExitCode() != 2 {
		t.Fatalf("ExitCode = %d, want 2 (browse error fixed status)", m.ExitCode())
	}
}

// TestTooSmallCtrlCExits130 verifies that ctrl+c on the too-small
// screen exits with code 130.
func TestTooSmallCtrlCExits130(t *testing.T) {
	m := setupTooSmallBrowse(t)
	m, cmd := update(t, m, ctrlC())
	assertQuit(t, cmd)
	if m.ExitCode() != 130 {
		t.Fatalf("ExitCode = %d, want 130", m.ExitCode())
	}
}

// --- Esc and other-key no-ops ---

// TestTooSmallEscWithOverlayIsNoOp verifies that Esc on the too-small
// screen with an overlay logically open is a no-op: the overlay is
// still open and the program does not exit. After recovery the overlay
// is still open.
func TestTooSmallEscWithOverlayIsNoOp(t *testing.T) {
	m := setupBrowseHelpOverlay(t)
	helpScroll := m.OverlayScroll()
	m = resizeTooSmall(t, m, 19, 10)
	tooSmallView(t, m)
	if !m.OverlayOpen() {
		t.Fatalf("overlay should be logically open during too-small")
	}
	m, cmd := update(t, m, keyPressOrEscape(tea.KeyEscape))
	if cmd != nil {
		t.Fatalf("Esc on too-small produced a command: %v", cmd)
	}
	if !m.OverlayOpen() {
		t.Fatalf("overlay closed after Esc on too-small, want still open")
	}
	if m.OverlayKind() != app.OverlayHelp {
		t.Fatalf("OverlayKind = %v, want OverlayHelp (preserved)", m.OverlayKind())
	}
	if m.OverlayScroll() != helpScroll {
		t.Fatalf("overlay scroll = %d after Esc, want %d (preserved)", m.OverlayScroll(), helpScroll)
	}
	// Recover: the overlay should still be open at the same scroll.
	m = resizeRecover(t, m, 80, 24)
	notTooSmallView(t, m)
	if !m.OverlayOpen() {
		t.Fatalf("overlay not open after recovery, want still open")
	}
	if m.OverlayKind() != app.OverlayHelp {
		t.Fatalf("OverlayKind = %v after recovery, want OverlayHelp", m.OverlayKind())
	}
	if m.OverlayScroll() != helpScroll {
		t.Fatalf("overlay scroll = %d after recovery, want %d", m.OverlayScroll(), helpScroll)
	}
}

// TestTooSmallOtherKeysAreNoOps verifies that n, w, and other keys are
// no-ops on the too-small screen: the state is unchanged and no command
// is produced.
func TestTooSmallOtherKeysAreNoOps(t *testing.T) {
	m := setupTooSmallBrowse(t)
	cursorBefore := m.CursorPosition()
	overlayBefore := m.OverlayOpen()
	for _, k := range []tea.KeyPressMsg{
		keyPress('n'),
		keyPress('p'),
		keyPress('w'),
		keyPress('c'),
		keyPress('r'),
		keyPress('h'),
		keyPress('?'),
		tea.KeyPressMsg{Code: tea.KeyDown},
		tea.KeyPressMsg{Code: tea.KeyUp},
		tea.KeyPressMsg{Code: tea.KeyPgDown},
	} {
		m2, cmd := update(t, m, k)
		if cmd != nil {
			t.Fatalf("key %v on too-small produced a command: %v", k, cmd)
		}
		if m2.CursorPosition() != cursorBefore {
			t.Fatalf("key %v moved cursor on too-small", k)
		}
		if m2.OverlayOpen() != overlayBefore {
			t.Fatalf("key %v changed overlay state on too-small", k)
		}
		m = m2
	}
}

// --- Round-trip state restoration ---

// TestTooSmallRoundTripPreservesCursor verifies that a too-small round
// trip preserves the cursor selection (position and current file).
func TestTooSmallRoundTripPreservesCursor(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
		textMatch("src/b.go", "world\n", 1, subSpec{"world", 0, 5}),
		textMatch("src/c.go", "foo\n", 1, subSpec{"foo", 0, 3}),
	)
	buf := makeBuf([]filebuffer.Line{ml(1, "hello world")}, 1, 3)
	m := setupBrowseWithSize(t, idx, buf, 80, 24)
	// Navigate to the third stop (file c.go).
	m, _ = update(t, m, keyPress('n'))
	m, _ = update(t, m, keyPress('n'))
	if m.CursorPosition() != 2 {
		t.Fatalf("CursorPosition = %d, want 2", m.CursorPosition())
	}
	// Too-small round trip.
	m = resizeTooSmall(t, m, 19, 10)
	m = resizeRecover(t, m, 80, 24)
	if m.CursorPosition() != 2 {
		t.Fatalf("after round trip, CursorPosition = %d, want 2 (preserved)", m.CursorPosition())
	}
	if p := m.CurrentPath(); p == nil || string(p) != "src/c.go" {
		t.Fatalf("after round trip, CurrentPath = %q, want src/c.go", p)
	}
}

// TestTooSmallRoundTripPreservesViewportAndAnchor verifies that a
// too-small round trip preserves the per-file viewport offset and the
// logical reading anchor.
func TestTooSmallRoundTripPreservesViewportAndAnchor(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", strings.Repeat("x", 500)+"\n", 1, subSpec{"x", 0, 1}),
	)
	display := strings.Repeat("x", 500)
	buf := makeBuf([]filebuffer.Line{ml(1, display)}, 1, 4)
	m := setupBrowseWithSize(t, idx, buf, 50, 10)
	// Scroll down 5 rows. At width 35, row 5 covers cells 75-89.
	for i := 0; i < 5; i++ {
		m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}
	if m.ViewportOffset() != 5 {
		t.Fatalf("ViewportOffset = %d, want 5", m.ViewportOffset())
	}
	anchor := m.ViewportAnchor()
	// Too-small round trip.
	m = resizeTooSmall(t, m, 19, 10)
	m = resizeRecover(t, m, 50, 10)
	if m.ViewportOffset() != 5 {
		t.Fatalf("after round trip, ViewportOffset = %d, want 5 (preserved)", m.ViewportOffset())
	}
	anchorAfter := m.ViewportAnchor()
	if anchorAfter != anchor {
		t.Fatalf("after round trip, Anchor = %+v, want %+v (preserved)", anchorAfter, anchor)
	}
}

// TestTooSmallRoundTripPreservesHorizontalOffset verifies that a
// too-small round trip preserves the horizontal pan offset in
// run-off-edge mode.
func TestTooSmallRoundTripPreservesHorizontalOffset(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", strings.Repeat("x", 200)+"\n", 1, subSpec{"x", 0, 1}),
	)
	display := strings.Repeat("x", 200)
	buf := makeBuf([]filebuffer.Line{ml(1, display)}, 1, 3)
	m := setupBrowseWithSize(t, idx, buf, 80, 24)
	// Toggle to run-off-edge mode and pan right.
	m, cmd := update(t, m, keyPress('w'))
	m = deliverLayout(t, m, cmd)
	m, _ = update(t, m, keyPress('>'))
	hOffset := m.ViewportHOffset()
	if hOffset == 0 {
		t.Fatalf("ViewportHOffset = 0, want nonzero after pan")
	}
	// Too-small round trip.
	m = resizeTooSmall(t, m, 19, 10)
	m = resizeRecover(t, m, 80, 24)
	if m.ViewportHOffset() != hOffset {
		t.Fatalf("after round trip, ViewportHOffset = %d, want %d (preserved)", m.ViewportHOffset(), hOffset)
	}
}

// TestTooSmallRoundTripPreservesListVisibility verifies that a too-small
// round trip preserves the file-list visibility preference.
func TestTooSmallRoundTripPreservesListVisibility(t *testing.T) {
	m := setupTooSmallBrowse(t)
	// Hide the list.
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = update(t, m, tabKey())
	if m.ListVisible() {
		t.Fatalf("ListVisible = true, want false after tab")
	}
	// Too-small round trip.
	m = resizeTooSmall(t, m, 19, 10)
	m = resizeRecover(t, m, 80, 24)
	if m.ListVisible() {
		t.Fatalf("after round trip, ListVisible = true, want false (preserved)")
	}
}

// TestTooSmallRoundTripPreservesWrapMode verifies that a too-small round
// trip preserves the wrap mode setting.
func TestTooSmallRoundTripPreservesWrapMode(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
	)
	buf := makeBuf([]filebuffer.Line{ml(1, "hello world")}, 1, 3)
	m := setupBrowseWithSize(t, idx, buf, 80, 24)
	// Toggle to run-off-edge mode.
	m, _ = update(t, m, keyPress('w'))
	if m.WrapMode() != viewport.WrapOff {
		t.Fatalf("WrapMode = %v, want WrapOff", m.WrapMode())
	}
	// Too-small round trip.
	m = resizeTooSmall(t, m, 19, 10)
	m = resizeRecover(t, m, 80, 24)
	if m.WrapMode() != viewport.WrapOff {
		t.Fatalf("after round trip, WrapMode = %v, want WrapOff (preserved)", m.WrapMode())
	}
}

// TestTooSmallRoundTripPreservesColourSetting verifies that a too-small
// round trip preserves the colour scheme setting.
func TestTooSmallRoundTripPreservesColourSetting(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
	)
	buf := makeBuf([]filebuffer.Line{ml(1, "hello world")}, 1, 3)
	m := setupBrowseWithSize(t, idx, buf, 80, 24)
	// Toggle to light scheme.
	m, _ = update(t, m, keyPress('c'))
	if m.Theme().Scheme() != theme.SchemeLight {
		t.Fatalf("Scheme = %v, want SchemeLight", m.Theme().Scheme())
	}
	// Too-small round trip.
	m = resizeTooSmall(t, m, 19, 10)
	m = resizeRecover(t, m, 80, 24)
	if m.Theme().Scheme() != theme.SchemeLight {
		t.Fatalf("after round trip, Scheme = %v, want SchemeLight (preserved)", m.Theme().Scheme())
	}
}

// TestTooSmallRoundTripPreservesScrolledHelpOverlay verifies that a
// too-small round trip preserves a scrolled help overlay at its prior
// scroll position.
func TestTooSmallRoundTripPreservesScrolledHelpOverlay(t *testing.T) {
	m := setupBrowseHelpOverlay(t)
	// Scroll help to position S.
	const scrollS = 4
	for i := 0; i < scrollS; i++ {
		m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}
	if m.OverlayScroll() != scrollS {
		t.Fatalf("help scroll = %d, want %d", m.OverlayScroll(), scrollS)
	}
	// Too-small round trip.
	m = resizeTooSmall(t, m, 19, 10)
	tooSmallView(t, m)
	if !m.OverlayOpen() {
		t.Fatalf("overlay not open during too-small, want logically open")
	}
	if m.OverlayKind() != app.OverlayHelp {
		t.Fatalf("OverlayKind = %v, want OverlayHelp", m.OverlayKind())
	}
	if m.OverlayScroll() != scrollS {
		t.Fatalf("during too-small, overlay scroll = %d, want %d (preserved)", m.OverlayScroll(), scrollS)
	}
	m = resizeRecover(t, m, 80, 24)
	notTooSmallView(t, m)
	if !m.OverlayOpen() {
		t.Fatalf("overlay not open after recovery, want open")
	}
	if m.OverlayKind() != app.OverlayHelp {
		t.Fatalf("OverlayKind = %v after recovery, want OverlayHelp", m.OverlayKind())
	}
	if m.OverlayScroll() != scrollS {
		t.Fatalf("after round trip, overlay scroll = %d, want %d (preserved)", m.OverlayScroll(), scrollS)
	}
}

// TestTooSmallRoundTripPreservesScrolledErrorOverlay verifies that a
// too-small round trip preserves a scrolled error overlay at its prior
// scroll position.
func TestTooSmallRoundTripPreservesScrolledErrorOverlay(t *testing.T) {
	m := setupBrowseErrorOverlay(t)
	// Scroll the error overlay to position P.
	const scrollP = 2
	for i := 0; i < scrollP; i++ {
		m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}
	if m.OverlayScroll() != scrollP {
		t.Fatalf("error scroll = %d, want %d", m.OverlayScroll(), scrollP)
	}
	// Too-small round trip.
	m = resizeTooSmall(t, m, 19, 10)
	tooSmallView(t, m)
	if !m.OverlayOpen() {
		t.Fatalf("overlay not open during too-small, want logically open")
	}
	if m.OverlayKind() != app.OverlayError {
		t.Fatalf("OverlayKind = %v, want OverlayError", m.OverlayKind())
	}
	if m.OverlayScroll() != scrollP {
		t.Fatalf("during too-small, overlay scroll = %d, want %d (preserved)", m.OverlayScroll(), scrollP)
	}
	m = resizeRecover(t, m, 80, 24)
	notTooSmallView(t, m)
	if !m.OverlayOpen() {
		t.Fatalf("overlay not open after recovery, want open")
	}
	if m.OverlayKind() != app.OverlayError {
		t.Fatalf("OverlayKind = %v after recovery, want OverlayError", m.OverlayKind())
	}
	if m.OverlayScroll() != scrollP {
		t.Fatalf("after round trip, overlay scroll = %d, want %d (preserved)", m.OverlayScroll(), scrollP)
	}
}

// TestTooSmallRoundTripPreservesErrorOverHelpStack verifies that a
// too-small round trip preserves an error-over-help stack: the error
// overlay is open, help is suspended, and the help scroll position is
// retained. After recovery, dismissing the error restores help at the
// saved scroll.
func TestTooSmallRoundTripPreservesErrorOverHelpStack(t *testing.T) {
	m := setupBrowseErrorOverHelp(t)
	// The error overlay is open over suspended help. The help scroll
	// was saved at the position help was scrolled to before the error.
	helpScroll := m.SuspendedHelpScroll()
	if !m.HelpSuspended() {
		t.Fatalf("HelpSuspended = false, want true")
	}
	// Too-small round trip.
	m = resizeTooSmall(t, m, 19, 10)
	tooSmallView(t, m)
	if !m.OverlayOpen() {
		t.Fatalf("overlay not open during too-small, want logically open")
	}
	if m.OverlayKind() != app.OverlayError {
		t.Fatalf("OverlayKind = %v, want OverlayError", m.OverlayKind())
	}
	if !m.HelpSuspended() {
		t.Fatalf("HelpSuspended = false during too-small, want true (preserved)")
	}
	if m.SuspendedHelpScroll() != helpScroll {
		t.Fatalf("SuspendedHelpScroll = %d during too-small, want %d (preserved)", m.SuspendedHelpScroll(), helpScroll)
	}
	m = resizeRecover(t, m, 80, 24)
	notTooSmallView(t, m)
	if !m.OverlayOpen() {
		t.Fatalf("overlay not open after recovery, want open")
	}
	if m.OverlayKind() != app.OverlayError {
		t.Fatalf("OverlayKind = %v after recovery, want OverlayError", m.OverlayKind())
	}
	if !m.HelpSuspended() {
		t.Fatalf("HelpSuspended = false after recovery, want true (preserved)")
	}
	// Dismiss the error; help should be restored at the saved scroll.
	m, cmd := update(t, m, keyPressOrEscape(tea.KeyEscape))
	if cmd != nil {
		t.Fatalf("Esc produced a command: %v", cmd)
	}
	if !m.OverlayOpen() {
		t.Fatalf("overlay closed after Esc, want help restored")
	}
	if m.OverlayKind() != app.OverlayHelp {
		t.Fatalf("OverlayKind = %v, want OverlayHelp (restored)", m.OverlayKind())
	}
	if m.OverlayScroll() != helpScroll {
		t.Fatalf("restored help scroll = %d, want %d", m.OverlayScroll(), helpScroll)
	}
}

// --- Resize wholly within the too-small state ---

// TestTooSmallResizeWhollyWithinPreservesState verifies that a sequence
// of resizes wholly within the too-small state (19x2 → 10x1 → 25x8)
// installs no ordinary layout at the pathological dimensions, mutates
// no anchors, and performs no partial modal restoration. Recovery uses
// the final 25x8 dimensions while a scrolled help overlay, an
// error-over-help stack, and a nontrivial viewport state are preserved
// without loss.
func TestTooSmallResizeWhollyWithinPreservesState(t *testing.T) {
	// Build a browse model with a nontrivial viewport state (scrolled
	// into a long wrapped line) and a scrolled help overlay open.
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", strings.Repeat("x", 500)+"\n", 1, subSpec{"x", 0, 1}),
	)
	display := strings.Repeat("x", 500)
	buf := makeBuf([]filebuffer.Line{ml(1, display)}, 1, 4)
	m := setupBrowseWithSize(t, idx, buf, 50, 10)
	// Scroll down 5 rows for a nontrivial viewport state.
	for i := 0; i < 5; i++ {
		m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}
	viewportOffset := m.ViewportOffset()
	anchor := m.ViewportAnchor()
	if viewportOffset == 0 {
		t.Fatalf("ViewportOffset = 0, want nonzero (nontrivial state)")
	}
	// Open help and scroll it.
	m, _ = update(t, m, keyPress('h'))
	const helpScroll = 3
	for i := 0; i < helpScroll; i++ {
		m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}
	if m.OverlayScroll() != helpScroll {
		t.Fatalf("help scroll = %d, want %d", m.OverlayScroll(), helpScroll)
	}

	// Resize wholly within the too-small state: 19x2 → 10x1 → 25x8.
	// 19x2 is too-small (height < 3).
	m = resizeTooSmall(t, m, 19, 2)
	tooSmallView(t, m)
	// At 19x2 the overlay is logically open and the viewport state is
	// preserved.
	if !m.OverlayOpen() || m.OverlayKind() != app.OverlayHelp {
		t.Fatalf("at 19x2, overlay not help, want logically open help")
	}
	if m.OverlayScroll() != helpScroll {
		t.Fatalf("at 19x2, help scroll = %d, want %d (preserved)", m.OverlayScroll(), helpScroll)
	}

	// 10x1 is still too-small (both dimensions below minimum).
	m = resizeTooSmall(t, m, 10, 1)
	tooSmallView(t, m)
	// No ordinary layout should be installed at 10x1; the viewport
	// offset and anchor should not have changed.
	if m.ViewportOffset() != viewportOffset {
		t.Fatalf("at 10x1, ViewportOffset = %d, want %d (no mutation)", m.ViewportOffset(), viewportOffset)
	}
	anchorAt10x1 := m.ViewportAnchor()
	if anchorAt10x1 != anchor {
		t.Fatalf("at 10x1, Anchor = %+v, want %+v (no mutation)", anchorAt10x1, anchor)
	}
	// No partial modal restoration: the help overlay is still open at
	// the same scroll.
	if !m.OverlayOpen() || m.OverlayKind() != app.OverlayHelp {
		t.Fatalf("at 10x1, overlay not help, want logically open help")
	}
	if m.OverlayScroll() != helpScroll {
		t.Fatalf("at 10x1, help scroll = %d, want %d (no partial restoration)", m.OverlayScroll(), helpScroll)
	}

	// Recover to 25x8 (above minimum). Recovery uses the final
	// dimensions.
	m = resizeRecover(t, m, 25, 8)
	notTooSmallView(t, m)
	// The help overlay should still be open at the same scroll.
	if !m.OverlayOpen() || m.OverlayKind() != app.OverlayHelp {
		t.Fatalf("after recovery, overlay not help, want open help")
	}
	if m.OverlayScroll() != helpScroll {
		t.Fatalf("after recovery, help scroll = %d, want %d (preserved)", m.OverlayScroll(), helpScroll)
	}
	// The viewport state should be preserved (the anchor is preserved
	// through the rebuild at the new dimensions).
	anchorAfter := m.ViewportAnchor()
	if anchorAfter != anchor {
		t.Fatalf("after recovery, Anchor = %+v, want %+v (preserved)", anchorAfter, anchor)
	}
}

// TestTooSmallResizeWhollyWithinErrorOverHelpStack verifies that a
// resize wholly within the too-small state preserves an error-over-help
// stack without loss. The error overlay, the suspended-help
// relationship, and the help scroll position all survive.
func TestTooSmallResizeWhollyWithinErrorOverHelpStack(t *testing.T) {
	m := setupBrowseErrorOverHelp(t)
	helpScroll := m.SuspendedHelpScroll()
	if !m.HelpSuspended() {
		t.Fatalf("HelpSuspended = false, want true")
	}

	// Resize wholly within too-small: 19x2 → 10x1 → 25x8.
	m = resizeTooSmall(t, m, 19, 2)
	tooSmallView(t, m)
	if !m.HelpSuspended() {
		t.Fatalf("at 19x2, HelpSuspended = false, want true (preserved)")
	}
	if m.SuspendedHelpScroll() != helpScroll {
		t.Fatalf("at 19x2, SuspendedHelpScroll = %d, want %d (preserved)", m.SuspendedHelpScroll(), helpScroll)
	}

	m = resizeTooSmall(t, m, 10, 1)
	tooSmallView(t, m)
	if !m.HelpSuspended() {
		t.Fatalf("at 10x1, HelpSuspended = false, want true (preserved)")
	}
	if m.SuspendedHelpScroll() != helpScroll {
		t.Fatalf("at 10x1, SuspendedHelpScroll = %d, want %d (preserved)", m.SuspendedHelpScroll(), helpScroll)
	}

	// Recover to 25x8.
	m = resizeRecover(t, m, 25, 8)
	notTooSmallView(t, m)
	if !m.OverlayOpen() || m.OverlayKind() != app.OverlayError {
		t.Fatalf("after recovery, overlay not error, want error")
	}
	if !m.HelpSuspended() {
		t.Fatalf("after recovery, HelpSuspended = false, want true (preserved)")
	}
	if m.SuspendedHelpScroll() != helpScroll {
		t.Fatalf("after recovery, SuspendedHelpScroll = %d, want %d (preserved)", m.SuspendedHelpScroll(), helpScroll)
	}
	// Dismiss the error; help should be restored at the saved scroll.
	m, _ = update(t, m, keyPressOrEscape(tea.KeyEscape))
	if m.OverlayKind() != app.OverlayHelp {
		t.Fatalf("after dismiss, OverlayKind = %v, want OverlayHelp (restored)", m.OverlayKind())
	}
	if m.OverlayScroll() != helpScroll {
		t.Fatalf("after dismiss, help scroll = %d, want %d (restored)", m.OverlayScroll(), helpScroll)
	}
}

// --- Pop-up timer continuation ---

// TestTooSmallPopupTimerContinuesAndExpiryDismisses verifies that an
// active pop-up's timer continues during too-small, an expiry during
// too-small dismisses the pop-up so it is absent after recovery, and
// the pop-up is never displayed on the too-small screen.
func TestTooSmallPopupTimerContinuesAndExpiryDismisses(t *testing.T) {
	m := setupBrowsePopup(t)
	// Navigate cross-file to open the pop-up.
	m, _ = update(t, m, keyPress('n'))
	if !m.PopupOpen() {
		t.Fatalf("pop-up not open after cross-file n")
	}
	popupInstance := m.PopupInstance()
	// The pop-up timer command was returned; capture it by re-running
	// the navigation. Instead, inject the expiry directly during
	// too-small.

	// Resize to too-small. The pop-up is not displayed.
	m = resizeTooSmall(t, m, 19, 10)
	tooSmallView(t, m)
	// The pop-up should not be shown on the too-small screen.
	view := viewContent(m)
	if m.PopupOpen() {
		// The pop-up is logically open (timer still running) but not
		// displayed on the too-small screen.
		// The view should not contain the pop-up path text.
		if p := m.PopupPath(); p != nil {
			if strings.Contains(view, string(p)) {
				t.Fatalf("too-small view contains pop-up path %q, want not displayed", string(p))
			}
		}
	}

	// Deliver the pop-up expiry during too-small. This dismisses the
	// pop-up.
	m, _ = update(t, m, app.FileChangePopupExpiryMsg{Instance: popupInstance})
	if m.PopupOpen() {
		t.Fatalf("pop-up still open after expiry during too-small, want dismissed")
	}

	// Recover. The pop-up should be absent.
	m = resizeRecover(t, m, 80, 24)
	notTooSmallView(t, m)
	if m.PopupOpen() {
		t.Fatalf("pop-up open after recovery, want absent (timer expired during too-small)")
	}
}

// TestTooSmallPopupNotDisplayed verifies that the pop-up is never
// displayed on the too-small screen, even while the timer is still
// running.
func TestTooSmallPopupNotDisplayed(t *testing.T) {
	m := setupBrowsePopup(t)
	// Navigate cross-file to open the pop-up.
	m, _ = update(t, m, keyPress('n'))
	if !m.PopupOpen() {
		t.Fatalf("pop-up not open after cross-file n")
	}
	// Resize to too-small while the pop-up timer is still running.
	m = resizeTooSmall(t, m, 19, 10)
	tooSmallView(t, m)
	// The pop-up timer is still running (not expired), so the pop-up
	// is logically open but not displayed.
	if !m.PopupOpen() {
		t.Fatalf("pop-up closed during too-small, want timer still running")
	}
	// The view must not show the pop-up path.
	view := viewContent(m)
	if p := m.PopupPath(); p != nil && strings.Contains(view, string(p)) {
		t.Fatalf("too-small view contains pop-up path %q, want not displayed", string(p))
	}
	// The view should show "Terminal too small", not the browse content
	// with the pop-up.
	if !strings.Contains(view, "Terminal too small") {
		t.Fatalf("view does not show 'Terminal too small' with active pop-up:\n%s", view)
	}
}
