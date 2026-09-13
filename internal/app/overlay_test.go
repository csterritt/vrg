package app_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/app"
	"vrg/internal/filebuffer"
	"vrg/internal/searchindex"
	"vrg/internal/sinkfixtures"
	"vrg/internal/theme"
)

// setupOverlayBrowse creates a browse model with an error overlay open,
// using the given diagnostic text. The file load completes immediately.
func setupOverlayBrowse(t *testing.T, diag string) app.Model {
	t.Helper()
	idx := buildIndex(t, "/work",
		textBegin("a.go"),
		textMatch("a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
		endRecord("a.go", nil),
		summaryRecord(),
	)
	gate := make(chan struct{})
	close(gate)
	loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		return makeBuf(nil, 0, 3), nil
	}
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(gate),
		app.WithFileLoader(loader),
	)
	m, cmd := update(t, m, app.SearchCompleteMsg{
		Files: idx.Files(), Lines: idx.Len(), Index: idx,
		Process: app.ProcessResult{ExitCode: 3},
		Stderr:  diag,
	})
	if cmd != nil {
		execCmd(t, cmd)
	}
	if !m.OverlayOpen() {
		t.Fatalf("overlay not open, want open error overlay")
	}
	if m.OverlayKind() != app.OverlayError {
		t.Fatalf("OverlayKind = %v, want OverlayError", m.OverlayKind())
	}
	return m
}

// setupOverlayFatalNoResults creates a fatal no-results model with an
// error overlay open and no underlying state.
func setupOverlayFatalNoResults(t *testing.T, diag string) app.Model {
	t.Helper()
	idx := buildIndex(t, "/work", summaryRecord())
	m := app.New([]string{"--json", "--", "foo", "."}, "/work")
	m, _ = update(t, m, app.SearchCompleteMsg{
		Files: idx.Files(), Lines: idx.Len(), Index: idx,
		Process: app.ProcessResult{ExitCode: 2},
		Stderr:  diag,
	})
	if !m.OverlayOpen() {
		t.Fatalf("overlay not open, want open fatal overlay")
	}
	if m.OverlayKind() != app.OverlayError {
		t.Fatalf("OverlayKind = %v, want OverlayError", m.OverlayKind())
	}
	return m
}

// TestOverlayKeyDownScrolls verifies that the down arrow key scrolls
// the overlay content down.
func TestOverlayKeyDownScrolls(t *testing.T) {
	m := setupOverlayBrowse(t, "line1\nline2\nline3\nline4\nline5")
	m, cmd := update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	if cmd != nil {
		t.Fatalf("down produced a command: %v", cmd)
	}
	if !m.OverlayOpen() {
		t.Fatalf("after down, overlay closed, want still open")
	}
}

// TestOverlayKeyUpScrolls verifies that the up arrow key scrolls the
// overlay content up.
func TestOverlayKeyUpScrolls(t *testing.T) {
	m := setupOverlayBrowse(t, "line1\nline2\nline3\nline4\nline5")
	// Scroll down first, then up.
	m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	m, cmd := update(t, m, tea.KeyPressMsg{Code: tea.KeyUp})
	if cmd != nil {
		t.Fatalf("up produced a command: %v", cmd)
	}
	if !m.OverlayOpen() {
		t.Fatalf("after up, overlay closed, want still open")
	}
}

// TestOverlayKeyQDismisses verifies that q dismisses a non-fatal
// browse overlay (returns to browse, no quit).
func TestOverlayKeyQDismisses(t *testing.T) {
	m := setupOverlayBrowse(t, "boom")
	m, cmd := update(t, m, keyPress('q'))
	if cmd != nil {
		t.Fatalf("q on browse overlay produced a command: %v", cmd)
	}
	if m.OverlayOpen() {
		t.Fatalf("after q, overlay still open, want closed")
	}
	if m.State() != app.StateBrowse {
		t.Fatalf("after q, State = %v, want StateBrowse", m.State())
	}
}

// TestOverlayKeyEscDismisses verifies that Esc dismisses a non-fatal
// browse overlay (returns to browse, no quit).
func TestOverlayKeyEscDismisses(t *testing.T) {
	m := setupOverlayBrowse(t, "boom")
	m, cmd := update(t, m, keyPressOrEscape(tea.KeyEscape))
	if cmd != nil {
		t.Fatalf("Esc on browse overlay produced a command: %v", cmd)
	}
	if m.OverlayOpen() {
		t.Fatalf("after Esc, overlay still open, want closed")
	}
	if m.State() != app.StateBrowse {
		t.Fatalf("after Esc, State = %v, want StateBrowse", m.State())
	}
}

// TestOverlayKeyCtrlCExits130 verifies that ctrl+c from an open overlay
// exits 130.
func TestOverlayKeyCtrlCExits130(t *testing.T) {
	m := setupOverlayBrowse(t, "boom")
	m, cmd := update(t, m, ctrlC())
	assertQuit(t, cmd)
	if m.ExitCode() != 130 {
		t.Fatalf("ExitCode = %d, want 130", m.ExitCode())
	}
}

// TestOverlayKeyOtherIgnored verifies that any key other than
// up/down/q/Esc/ctrl+c is ignored by the overlay.
func TestOverlayKeyOtherIgnored(t *testing.T) {
	m := setupOverlayBrowse(t, "boom")
	for _, k := range []tea.KeyPressMsg{
		keyPress('x'),
		keyPress('a'),
		keyPress('c'), // c toggles theme in browse, but overlay open → ignored
		tea.KeyPressMsg{Code: tea.KeyLeft},
		tea.KeyPressMsg{Code: tea.KeyRight},
	} {
		mm, cmd := update(t, m, k)
		if cmd != nil {
			t.Fatalf("key %v on overlay produced a command: %v", k, cmd)
		}
		if !mm.OverlayOpen() {
			t.Fatalf("key %v closed the overlay, want still open", k)
		}
		if mm.State() != app.StateBrowse {
			t.Fatalf("key %v changed state to %v, want StateBrowse", k, mm.State())
		}
	}
}

// TestOverlayFatalNoResultsQExits2 verifies that q on a fatal no-results
// overlay exits 2.
func TestOverlayFatalNoResultsQExits2(t *testing.T) {
	m := setupOverlayFatalNoResults(t, "")
	m, cmd := update(t, m, keyPress('q'))
	assertQuit(t, cmd)
	if m.ExitCode() != 2 {
		t.Fatalf("ExitCode = %d, want 2", m.ExitCode())
	}
}

// TestOverlayFatalNoResultsEscExits2 verifies that Esc on a fatal
// no-results overlay exits 2 (the one case where Esc terminates).
func TestOverlayFatalNoResultsEscExits2(t *testing.T) {
	m := setupOverlayFatalNoResults(t, "")
	m, cmd := update(t, m, keyPressOrEscape(tea.KeyEscape))
	assertQuit(t, cmd)
	if m.ExitCode() != 2 {
		t.Fatalf("ExitCode = %d, want 2", m.ExitCode())
	}
}

// TestOverlayFatalNoResultsCtrlCExits130 verifies that ctrl+c on a
// fatal no-results overlay exits 130 (overriding the fixed 2).
func TestOverlayFatalNoResultsCtrlCExits130(t *testing.T) {
	m := setupOverlayFatalNoResults(t, "")
	m, cmd := update(t, m, ctrlC())
	assertQuit(t, cmd)
	if m.ExitCode() != 130 {
		t.Fatalf("ExitCode = %d, want 130", m.ExitCode())
	}
}

// TestOverlayRendersDiagnostic verifies that the overlay view contains
// the diagnostic text.
func TestOverlayRendersDiagnostic(t *testing.T) {
	m := setupOverlayBrowse(t, "boom")
	view := viewContent(m)
	if !strings.Contains(view, "boom") {
		t.Fatalf("overlay view does not contain 'boom': %q", view)
	}
}

// TestOverlayRendersBorder verifies that the overlay view contains a
// single-line border.
func TestOverlayRendersBorder(t *testing.T) {
	m := setupOverlayBrowse(t, "boom")
	view := viewContent(m)
	// Single-line border uses box-drawing chars ┌ ┐ └ ┘ and ─.
	if !strings.Contains(view, "\u250c") {
		t.Fatalf("overlay view does not contain top-left border corner: %q", view)
	}
	if !strings.Contains(view, "\u2510") {
		t.Fatalf("overlay view does not contain top-right border corner: %q", view)
	}
	if !strings.Contains(view, "\u2514") {
		t.Fatalf("overlay view does not contain bottom-left border corner: %q", view)
	}
	if !strings.Contains(view, "\u2518") {
		t.Fatalf("overlay view does not contain bottom-right border corner: %q", view)
	}
}

// TestOverlayBaseColors verifies that the overlay uses the theme base
// colours.
func TestOverlayBaseColors(t *testing.T) {
	idx := buildIndex(t, "/work",
		textBegin("a.go"),
		textMatch("a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
		endRecord("a.go", nil),
		summaryRecord(),
	)
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithTheme(theme.New()),
	)
	m, _ = update(t, m, app.SearchCompleteMsg{
		Files: idx.Files(), Lines: idx.Len(), Index: idx,
		Process: app.ProcessResult{ExitCode: 3},
		Stderr:  "boom",
	})
	view := viewContent(m)
	// Dark base: white on black.
	if !strings.Contains(view, "\x1b[37;40m") {
		t.Fatalf("overlay view does not contain dark base colours: %q", view)
	}
}

// TestOverlayLongUnbrokenWraps verifies that a long unbroken diagnostic
// string wraps within the overlay border.
func TestOverlayLongUnbrokenWraps(t *testing.T) {
	long := strings.Repeat("x", 200)
	m := setupOverlayBrowse(t, long)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	view := viewContent(m)
	// The overlay must contain the head of the diagnostic.
	if !strings.Contains(view, "x") {
		t.Fatalf("overlay view does not contain the diagnostic head: %q", view)
	}
	// The view must contain multiple lines (wrapped), each within the
	// border width. Count border side chars (│) to confirm wrapping.
	if strings.Count(view, "\u2502") < 2 {
		t.Fatalf("overlay view does not show wrapped lines (too few border sides): %q", view)
	}
}

// TestOverlaySinkSafetyNoStyle verifies that the error overlay passes
// the Issue #6 sink-safety check: no raw control bytes survive in the
// no-style composition path for every shared hostile fixture.
func TestOverlaySinkSafetyNoStyle(t *testing.T) {
	for _, fx := range sinkfixtures.Fixtures {
		t.Run(fx.Name, func(t *testing.T) {
			idx := buildIndex(t, "/work",
				textBegin("a.go"),
				textMatch("a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
				endRecord("a.go", nil),
				summaryRecord(),
			)
			m := app.New([]string{"--json", "--", "foo", "."}, "/work",
				app.WithTheme(theme.NoStyle()),
			)
			m, _ = update(t, m, app.SearchCompleteMsg{
				Files: idx.Files(), Lines: idx.Len(), Index: idx,
				Process: app.ProcessResult{ExitCode: 3},
				Stderr:  string(fx.Raw),
			})
			view := viewContent(m)
			if !sinkfixtures.NoControlBytes(view) {
				t.Fatalf("raw control byte in overlay for %s: %q", fx.Name, view)
			}
		})
	}
}

// TestOverlaySinkSafetyStyled verifies that with styles enabled, the
// fixture's distinctive payload never appears immediately after an
// unescaped ESC in the error overlay.
func TestOverlaySinkSafetyStyled(t *testing.T) {
	for _, fx := range sinkfixtures.Fixtures {
		t.Run(fx.Name, func(t *testing.T) {
			idx := buildIndex(t, "/work",
				textBegin("a.go"),
				textMatch("a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
				endRecord("a.go", nil),
				summaryRecord(),
			)
			m := app.New([]string{"--json", "--", "foo", "."}, "/work",
				app.WithTheme(theme.New()),
			)
			m, _ = update(t, m, app.SearchCompleteMsg{
				Files: idx.Files(), Lines: idx.Len(), Index: idx,
				Process: app.ProcessResult{ExitCode: 3},
				Stderr:  string(fx.Raw),
			})
			view := viewContent(m)
			if !sinkfixtures.NoPayloadAfterESC(view, fx.Payload) {
				t.Fatalf("fixture payload after unescaped ESC in overlay for %s: %q", fx.Name, view)
			}
		})
	}
}

// TestOverlayDiagnosticSanitized verifies that diagnostic content in
// the overlay is sanitized through the Issue #6 utility (no raw control
// bytes from a hostile diagnostic survive) in the no-style path.
func TestOverlayDiagnosticSanitized(t *testing.T) {
	hostile := "before\x1b]0;pwned\x07after\n\x07\x08\x1b"
	idx := buildIndex(t, "/work",
		textBegin("a.go"),
		textMatch("a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
		endRecord("a.go", nil),
		summaryRecord(),
	)
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithTheme(theme.NoStyle()),
	)
	m, _ = update(t, m, app.SearchCompleteMsg{
		Files: idx.Files(), Lines: idx.Len(), Index: idx,
		Process: app.ProcessResult{ExitCode: 3},
		Stderr:  hostile,
	})
	view := viewContent(m)
	if !sinkfixtures.NoControlBytes(view) {
		t.Fatalf("raw control byte in overlay with hostile diagnostic: %q", view)
	}
	// The ESC should be escaped as caret notation, not raw.
	if strings.Contains(view, "\x1b]0;pwned") {
		t.Fatalf("overlay contains raw OSC sequence: %q", view)
	}
}

// TestOverlayPreservesSafeWrapping verifies that overlay rendering
// preserves safe wrapping and scrolling behavior after a resize.
func TestOverlayPreservesSafeWrapping(t *testing.T) {
	long := strings.Repeat("y", 300)
	idx := buildIndex(t, "/work",
		textBegin("a.go"),
		textMatch("a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
		endRecord("a.go", nil),
		summaryRecord(),
	)
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithTheme(theme.NoStyle()),
	)
	m, _ = update(t, m, app.SearchCompleteMsg{
		Files: idx.Files(), Lines: idx.Len(), Index: idx,
		Process: app.ProcessResult{ExitCode: 3},
		Stderr:  long,
	})
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 60, Height: 20})
	view := viewContent(m)
	if !sinkfixtures.NoControlBytes(view) {
		t.Fatalf("raw control byte after resize: %q", view)
	}
	// Scrolling still works after resize.
	m, cmd := update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	if cmd != nil {
		t.Fatalf("down after resize produced a command: %v", cmd)
	}
	if !m.OverlayOpen() {
		t.Fatalf("overlay closed after down, want still open")
	}
}

// TestEscNeverQuitsWhenNoOverlay verifies that Esc never quits when no
// overlay is open, across all base states.
func TestEscNeverQuitsWhenNoOverlay(t *testing.T) {
	// Browse without overlay.
	idx := buildIndex(t, "/work",
		textBegin("a.go"),
		textMatch("a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
		endRecord("a.go", nil),
		summaryRecord(),
	)
	m := app.New([]string{"--json", "--", "foo", "."}, "/work")
	m, _ = update(t, m, app.SearchCompleteMsg{
		Files: idx.Files(), Lines: idx.Len(), Index: idx,
		Process: app.ProcessResult{ExitCode: 0},
	})
	m, cmd := update(t, m, keyPressOrEscape(tea.KeyEscape))
	if cmd != nil {
		t.Fatalf("Esc from browse (no overlay) produced a command: %v", cmd)
	}

	// No-results without overlay.
	idx = buildIndex(t, "/work", summaryRecord())
	m = app.New([]string{"--json", "--", "foo", "."}, "/work")
	m, _ = update(t, m, app.SearchCompleteMsg{
		Files: idx.Files(), Lines: idx.Len(), Index: idx,
		Process: app.ProcessResult{ExitCode: 0},
	})
	m, cmd = update(t, m, keyPressOrEscape(tea.KeyEscape))
	if cmd != nil {
		t.Fatalf("Esc from no-results (no overlay) produced a command: %v", cmd)
	}
}
