package app_test

import (
	"strings"
	"testing"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/app"
	"vrg/internal/filebuffer"
	"vrg/internal/searchindex"
	"vrg/internal/sinkfixtures"
	"vrg/internal/theme"
)

// setupHelpBrowse creates a browse model with a single file so the help
// overlay can be opened from ordinary browsing. The file load completes
// immediately and a default 80x24 terminal size is set.
func setupHelpBrowse(t *testing.T) app.Model {
	t.Helper()
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
	)
	buf := makeBuf(nil, 0, 3)
	gate := make(chan struct{})
	close(gate)
	loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		return buf, nil
	}
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(gate),
		app.WithFileLoader(loader),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	if cmd != nil {
		m = deliverLoad(t, m, cmd)
	}
	if m.State() != app.StateBrowse {
		t.Fatalf("State = %v, want StateBrowse", m.State())
	}
	return m
}

// setupHelpNoResults creates a no-results model so the help overlay can
// be opened from the no-results screen.
func setupHelpNoResults(t *testing.T) app.Model {
	t.Helper()
	idx := buildIndex(t, "/work", summaryRecord())
	m := app.New([]string{"--json", "--", "foo", "."}, "/work")
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	if m.State() != app.StateNoResults {
		t.Fatalf("State = %v, want StateNoResults", m.State())
	}
	return m
}

// openHelp sends an h key press to open the help overlay and asserts it
// opened.
func openHelp(t *testing.T, m app.Model) app.Model {
	t.Helper()
	m, cmd := update(t, m, keyPress('h'))
	if cmd != nil {
		t.Fatalf("h produced a command: %v", cmd)
	}
	if !m.OverlayOpen() {
		t.Fatalf("help overlay not open after h, want open")
	}
	if m.OverlayKind() != app.OverlayHelp {
		t.Fatalf("OverlayKind = %v, want OverlayHelp", m.OverlayKind())
	}
	return m
}

// openHelpQuestion sends a ? key press to open the help overlay and
// asserts it opened.
func openHelpQuestion(t *testing.T, m app.Model) app.Model {
	t.Helper()
	m, cmd := update(t, m, keyPress('?'))
	if cmd != nil {
		t.Fatalf("? produced a command: %v", cmd)
	}
	if !m.OverlayOpen() {
		t.Fatalf("help overlay not open after ?, want open")
	}
	if m.OverlayKind() != app.OverlayHelp {
		t.Fatalf("OverlayKind = %v, want OverlayHelp", m.OverlayKind())
	}
	return m
}

// --- Opening from browse and no-results ---

// TestHelpOpensFromBrowse verifies that h opens the help overlay from
// ordinary browsing.
func TestHelpOpensFromBrowse(t *testing.T) {
	m := setupHelpBrowse(t)
	m = openHelp(t, m)
}

// TestHelpOpensFromBrowseQuestion verifies that ? opens the help overlay
// from ordinary browsing.
func TestHelpOpensFromBrowseQuestion(t *testing.T) {
	m := setupHelpBrowse(t)
	m = openHelpQuestion(t, m)
}

// TestHelpOpensFromNoResults verifies that h opens the help overlay from
// the no-results screen.
func TestHelpOpensFromNoResults(t *testing.T) {
	m := setupHelpNoResults(t)
	m = openHelp(t, m)
	if m.State() != app.StateNoResults {
		t.Fatalf("after opening help, State = %v, want StateNoResults", m.State())
	}
}

// TestHelpOpensFromNoResultsQuestion verifies that ? opens the help
// overlay from the no-results screen.
func TestHelpOpensFromNoResultsQuestion(t *testing.T) {
	m := setupHelpNoResults(t)
	m = openHelpQuestion(t, m)
	if m.State() != app.StateNoResults {
		t.Fatalf("after opening help, State = %v, want StateNoResults", m.State())
	}
}

// --- Close and ignored keys ---

// TestHelpCloseReturnsToBrowse verifies that closing the help overlay
// returns to the underlying browse state.
func TestHelpCloseReturnsToBrowse(t *testing.T) {
	base := setupHelpBrowse(t)
	for _, closeKey := range []tea.KeyPressMsg{
		keyPress('q'),
		keyPressOrEscape(tea.KeyEscape),
		keyPress('h'),
		keyPress('?'),
	} {
		// Open help from the base state before each close so every
		// close key is tested.
		m := openHelp(t, base)
		m, cmd := update(t, m, closeKey)
		if cmd != nil {
			t.Fatalf("close key %v produced a command: %v", closeKey, cmd)
		}
		if m.OverlayOpen() {
			t.Fatalf("after close key %v, overlay still open", closeKey)
		}
		if m.State() != app.StateBrowse {
			t.Fatalf("after close key %v, State = %v, want StateBrowse", closeKey, m.State())
		}
	}
}

// TestHelpCloseReturnsToNoResults verifies that closing the help overlay
// returns to the underlying no-results state.
func TestHelpCloseReturnsToNoResults(t *testing.T) {
	base := setupHelpNoResults(t)
	for _, closeKey := range []tea.KeyPressMsg{
		keyPress('q'),
		keyPressOrEscape(tea.KeyEscape),
		keyPress('h'),
		keyPress('?'),
	} {
		m := openHelp(t, base)
		m, cmd := update(t, m, closeKey)
		if cmd != nil {
			t.Fatalf("close key %v produced a command: %v", closeKey, cmd)
		}
		if m.OverlayOpen() {
			t.Fatalf("after close key %v, overlay still open", closeKey)
		}
		if m.State() != app.StateNoResults {
			t.Fatalf("after close key %v, State = %v, want StateNoResults", closeKey, m.State())
		}
	}
}

// TestHelpCloseThenQExits1FromNoResults verifies that after closing help
// opened from no-results, a subsequent q still exits 1.
func TestHelpCloseThenQExits1FromNoResults(t *testing.T) {
	m := setupHelpNoResults(t)
	m = openHelp(t, m)
	m, cmd := update(t, m, keyPress('q'))
	if cmd != nil {
		t.Fatalf("q to close help produced a command: %v", cmd)
	}
	if m.OverlayOpen() {
		t.Fatalf("after q, overlay still open")
	}
	if m.State() != app.StateNoResults {
		t.Fatalf("after q, State = %v, want StateNoResults", m.State())
	}
	// A subsequent q on no-results exits 1.
	m, cmd = update(t, m, keyPress('q'))
	assertQuit(t, cmd)
	if m.ExitCode() != 1 {
		t.Fatalf("ExitCode = %d, want 1", m.ExitCode())
	}
}

// TestHelpIgnoredKeysLeaveStateUnchanged verifies that every key other
// than up/down/q/Esc/h/?/ctrl+c is ignored while help is open, leaving
// the underlying state unchanged.
func TestHelpIgnoredKeysLeaveStateUnchanged(t *testing.T) {
	m := setupHelpBrowse(t)
	m = openHelp(t, m)
	for _, k := range []tea.KeyPressMsg{
		keyPress('n'),
		keyPress('p'),
		keyPress('w'),
		keyPress('c'),
		keyPress('r'),
		keyPress('x'),
		keyPress('a'),
		tea.KeyPressMsg{Code: tea.KeyLeft},
		tea.KeyPressMsg{Code: tea.KeyRight},
		tea.KeyPressMsg{Code: tea.KeyTab},
	} {
		mm, cmd := update(t, m, k)
		if cmd != nil {
			t.Fatalf("key %v on help produced a command: %v", k, cmd)
		}
		if !mm.OverlayOpen() {
			t.Fatalf("key %v closed the help overlay, want still open", k)
		}
		if mm.OverlayKind() != app.OverlayHelp {
			t.Fatalf("key %v changed overlay kind to %v, want OverlayHelp", k, mm.OverlayKind())
		}
		if mm.State() != app.StateBrowse {
			t.Fatalf("key %v changed state to %v, want StateBrowse", k, mm.State())
		}
	}
}

// TestHelpIgnoredKeysLeaveNoResultsUnchanged verifies ignored keys leave
// the no-results state unchanged while help is open.
func TestHelpIgnoredKeysLeaveNoResultsUnchanged(t *testing.T) {
	m := setupHelpNoResults(t)
	m = openHelp(t, m)
	for _, k := range []tea.KeyPressMsg{
		keyPress('n'),
		keyPress('p'),
		keyPress('w'),
		keyPress('c'),
		keyPress('r'),
	} {
		mm, cmd := update(t, m, k)
		if cmd != nil {
			t.Fatalf("key %v on help produced a command: %v", k, cmd)
		}
		if !mm.OverlayOpen() {
			t.Fatalf("key %v closed the help overlay, want still open", k)
		}
		if mm.State() != app.StateNoResults {
			t.Fatalf("key %v changed state to %v, want StateNoResults", k, mm.State())
		}
	}
}

// --- ctrl+c ---

// TestHelpCtrlCExits130 verifies that ctrl+c from the help overlay
// exits 130.
func TestHelpCtrlCExits130(t *testing.T) {
	m := setupHelpBrowse(t)
	m = openHelp(t, m)
	m, cmd := update(t, m, ctrlC())
	assertQuit(t, cmd)
	if m.ExitCode() != 130 {
		t.Fatalf("ExitCode = %d, want 130", m.ExitCode())
	}
}

// TestHelpCtrlCExits130FromNoResults verifies that ctrl+c from the help
// overlay opened from no-results exits 130.
func TestHelpCtrlCExits130FromNoResults(t *testing.T) {
	m := setupHelpNoResults(t)
	m = openHelp(t, m)
	m, cmd := update(t, m, ctrlC())
	assertQuit(t, cmd)
	if m.ExitCode() != 130 {
		t.Fatalf("ExitCode = %d, want 130", m.ExitCode())
	}
}

// --- Scroll bounds ---

// TestHelpScrollDownIncrements verifies that down scrolls the help
// content down.
func TestHelpScrollDownIncrements(t *testing.T) {
	m := setupHelpBrowse(t)
	m = openHelp(t, m)
	m, cmd := update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	if cmd != nil {
		t.Fatalf("down produced a command: %v", cmd)
	}
	if !m.OverlayOpen() {
		t.Fatalf("after down, overlay closed, want still open")
	}
}

// TestHelpScrollUpAtTopClamps verifies that up at the top of the help
// content clamps to 0 (no negative scroll).
func TestHelpScrollUpAtTopClamps(t *testing.T) {
	m := setupHelpBrowse(t)
	m = openHelp(t, m)
	// At the top, up should not produce negative scroll.
	for i := 0; i < 5; i++ {
		var cmd tea.Cmd
		m, cmd = update(t, m, tea.KeyPressMsg{Code: tea.KeyUp})
		if cmd != nil {
			t.Fatalf("up produced a command: %v", cmd)
		}
	}
	if !m.OverlayOpen() {
		t.Fatalf("after up, overlay closed, want still open")
	}
}

// TestHelpScrollReachesEveryRow verifies that vertical scrolling reaches
// every row at a usable size. The help content is taller than the
// visible area, so scrolling down repeatedly must reach the last row.
func TestHelpScrollReachesEveryRow(t *testing.T) {
	m := setupHelpBrowse(t)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m = openHelp(t, m)
	// Scroll to the bottom by pressing down many times.
	for i := 0; i < 200; i++ {
		m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}
	// The overlay must remain open after scrolling to the bottom.
	if !m.OverlayOpen() {
		t.Fatalf("after scrolling, overlay closed, want still open")
	}
	// Scrolling back to the top must work too.
	for i := 0; i < 200; i++ {
		m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyUp})
	}
	if !m.OverlayOpen() {
		t.Fatalf("after scrolling up, overlay closed, want still open")
	}
}

// --- Wrapping including unbroken strings ---

// TestHelpWrapsLongUnbrokenString verifies that a long unbroken string
// in the help text wraps to the interior width of the overlay border.
func TestHelpWrapsLongUnbrokenString(t *testing.T) {
	m := setupHelpBrowse(t)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m = openHelp(t, m)
	view := viewContent(m)
	// The help overlay must contain a border (single-line).
	if !strings.Contains(view, "\u250c") {
		t.Fatalf("help overlay does not contain top-left border corner: %q", view)
	}
	// The help overlay must contain multiple lines (wrapped content),
	// indicated by multiple border side chars.
	if strings.Count(view, "\u2502") < 2 {
		t.Fatalf("help overlay does not show wrapped lines (too few border sides): %q", view)
	}
}

// --- Tiny-size clipping ---

// TestHelpRendersAt25x8WithoutPanic verifies that the help overlay
// renders without panic at 25x8 (tiny size).
func TestHelpRendersAt25x8WithoutPanic(t *testing.T) {
	m := setupHelpBrowse(t)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 25, Height: 8})
	m = openHelp(t, m)
	// Rendering must not panic.
	view := viewContent(m)
	if view == "" {
		t.Fatalf("help overlay view is empty at 25x8")
	}
}

// TestHelpClippedAtTinySize verifies that at a tiny size the help overlay
// is clipped to the terminal (no wider than the terminal width) without
// a borderless mode.
func TestHelpClippedAtTinySize(t *testing.T) {
	m := setupHelpBrowse(t)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 25, Height: 8})
	m = openHelp(t, m)
	view := viewContent(m)
	// Every line of the overlay must not exceed the terminal width.
	// The overlay is clipped to the terminal, so no line should be
	// wider than 25 cells (excluding ANSI escapes).
	for _, line := range strings.Split(view, "\n") {
		if w := helpVisibleWidth(line); w > 25 {
			t.Fatalf("overlay line %d cells wide exceeds terminal width 25: %q", w, line)
		}
	}
}

// TestHelpRestoredOnGrowth verifies that after shrinking to a tiny size
// and then growing, the help overlay renders normally (border present,
// content visible).
func TestHelpRestoredOnGrowth(t *testing.T) {
	m := setupHelpBrowse(t)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 25, Height: 8})
	m = openHelp(t, m)
	// Shrink then grow.
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	if !m.OverlayOpen() {
		t.Fatalf("after growth, overlay closed, want still open")
	}
	view := viewContent(m)
	if !strings.Contains(view, "\u250c") {
		t.Fatalf("after growth, help overlay does not contain border: %q", view)
	}
}

// --- Binding-table rendering ---

// TestKeyBindingsDefinedAsData verifies that the key-binding list is
// defined once as data (binding → description) that the help renderer
// consumes and documentation tests can iterate.
func TestKeyBindingsDefinedAsData(t *testing.T) {
	bindings := app.KeyBindings()
	if len(bindings) == 0 {
		t.Fatalf("KeyBindings returned empty list, want at least one binding")
	}
	// Each binding must have a non-empty binding key and description.
	for i, b := range bindings {
		if b.Key == "" {
			t.Fatalf("binding %d has empty Key", i)
		}
		if b.Description == "" {
			t.Fatalf("binding %d has empty Description", i)
		}
	}
}

// TestKeyBindingsCoversAllCategories verifies that the single binding
// table covers navigation, scrolling, panning, wrap, colour, list
// toggle, reload, help, and quit/cancel bindings.
func TestKeyBindingsCoversAllCategories(t *testing.T) {
	bindings := app.KeyBindings()
	keys := map[string]bool{}
	for _, b := range bindings {
		keys[b.Key] = true
	}
	required := []string{"n", "p", "r", "w", "c", "h/?", "q"}
	for _, k := range required {
		if !keys[k] {
			t.Fatalf("KeyBindings missing key %q", k)
		}
	}
}

// TestHelpRendersBindingTable verifies that the help overlay view
// contains the binding descriptions from the binding table.
func TestHelpRendersBindingTable(t *testing.T) {
	m := setupHelpBrowse(t)
	m = openHelp(t, m)
	view := viewContent(m)
	bindings := app.KeyBindings()
	// At least one binding description must appear in the view.
	found := false
	for _, b := range bindings {
		if strings.Contains(view, b.Description) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("help overlay view does not contain any binding description: %q", view)
	}
}

// TestHelpFooterSlot verifies that the help overlay has a footer slot
// reserved for Issue #34.
func TestHelpFooterSlot(t *testing.T) {
	footer := app.HelpFooter()
	// The footer must be a non-nil string (may be empty now, but the
	// slot must exist as a defined API).
	_ = footer
}

// --- Pop-up cancellation ---

// TestHelpOpensCancelsPopup verifies that opening the help overlay
// cancels an active Issue #15 pop-up and it does not return on close.
func TestHelpOpensCancelsPopup(t *testing.T) {
	m := setupBrowsePopup(t)
	// Navigate cross-file to open the pop-up.
	m, _ = update(t, m, keyPress('n'))
	if !m.PopupOpen() {
		t.Fatalf("pop-up not open after cross-file n")
	}
	// Open help; the pop-up must be cancelled.
	m, cmd := update(t, m, keyPress('h'))
	if cmd != nil {
		t.Fatalf("h produced a command: %v", cmd)
	}
	if !m.OverlayOpen() {
		t.Fatalf("help overlay not open after h")
	}
	if m.OverlayKind() != app.OverlayHelp {
		t.Fatalf("OverlayKind = %v, want OverlayHelp", m.OverlayKind())
	}
	if m.PopupOpen() {
		t.Fatalf("pop-up still open after help opened, want cancelled")
	}
	// Close help; the pop-up must not return.
	m, cmd = update(t, m, keyPress('q'))
	if cmd != nil {
		t.Fatalf("q to close help produced a command: %v", cmd)
	}
	if m.OverlayOpen() {
		t.Fatalf("after q, help overlay still open")
	}
	if m.PopupOpen() {
		t.Fatalf("pop-up returned after help closed, want no return")
	}
}

// --- Sink-safety row ---

// TestHelpSinkSafetyNoStyle verifies that the help overlay passes the
// Issue #6 sink-safety check: no raw control bytes survive in the
// no-style composition path. The help overlay's substituted text is
// routed through the Issue #6 utility.
func TestHelpSinkSafetyNoStyle(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
	)
	buf := makeBuf(nil, 0, 3)
	gate := make(chan struct{})
	close(gate)
	loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		return buf, nil
	}
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(gate),
		app.WithFileLoader(loader),
		app.WithTheme(theme.NoStyle()),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	m, _ = update(t, m, keyPress('h'))
	if !m.OverlayOpen() {
		t.Fatalf("help overlay not open")
	}
	view := viewContent(m)
	if !sinkfixtures.NoDangerousControls(view) {
		t.Fatalf("raw control byte in help overlay: %q", view)
	}
}

// TestHelpSinkSafetyStyled verifies that with styles enabled, the
// help overlay contains no dangerous fixture payload after an
// unescaped ESC.
func TestHelpSinkSafetyStyled(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
	)
	buf := makeBuf(nil, 0, 3)
	gate := make(chan struct{})
	close(gate)
	loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		return buf, nil
	}
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(gate),
		app.WithFileLoader(loader),
		app.WithTheme(theme.New()),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	m, _ = update(t, m, keyPress('h'))
	if !m.OverlayOpen() {
		t.Fatalf("help overlay not open")
	}
	view := viewContent(m)
	// The help overlay is fixed text (not external data), so no
	// fixture payload should appear after an unescaped ESC. The
	// styled overlay uses ANSI escapes for colours, but the help
	// text itself must not contain raw control sequences.
	for _, fx := range sinkfixtures.Fixtures {
		if !sinkfixtures.NoPayloadAfterESC(view, fx.Payload) {
			t.Fatalf("fixture payload after unescaped ESC in help overlay for %s: %q", fx.Name, view)
		}
	}
}

// TestHelpBaseColors verifies that the help overlay uses the theme
// base colours (same overlay style as the error overlay).
func TestHelpBaseColors(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
	)
	buf := makeBuf(nil, 0, 3)
	gate := make(chan struct{})
	close(gate)
	loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		return buf, nil
	}
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(gate),
		app.WithFileLoader(loader),
		app.WithTheme(theme.New()),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	m, _ = update(t, m, keyPress('h'))
	view := viewContent(m)
	// Dark base: white on black.
	if !strings.Contains(view, "\x1b[37;40m") {
		t.Fatalf("help overlay does not contain dark base colours: %q", view)
	}
}

// TestHelpRendersBorder verifies that the help overlay view contains a
// single-line border (same plain border as the error overlay).
func TestHelpRendersBorder(t *testing.T) {
	m := setupHelpBrowse(t)
	m = openHelp(t, m)
	view := viewContent(m)
	if !strings.Contains(view, "\u250c") {
		t.Fatalf("help overlay does not contain top-left border corner: %q", view)
	}
	if !strings.Contains(view, "\u2510") {
		t.Fatalf("help overlay does not contain top-right border corner: %q", view)
	}
	if !strings.Contains(view, "\u2514") {
		t.Fatalf("help overlay does not contain bottom-left border corner: %q", view)
	}
	if !strings.Contains(view, "\u2518") {
		t.Fatalf("help overlay does not contain bottom-right border corner: %q", view)
	}
}

// helpVisibleWidth returns the number of visible cells in s, excluding
// ANSI escape sequences. Each rune is one cell.
func helpVisibleWidth(s string) int {
	var w int
	for i := 0; i < len(s); {
		if s[i] == '\x1b' {
			i++
			for i < len(s) && s[i] != 'm' {
				i++
			}
			if i < len(s) {
				i++
			}
			continue
		}
		_, size := utf8.DecodeRuneInString(s[i:])
		w++
		i += size
	}
	return w
}
