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

// --- Issue #34 help footer sink-safety row ---

// TestHelpFooterNonEmpty verifies that the help overlay footer is
// non-empty — Issue #34 fills the footer slot reserved by Issue #31
// with the scale, record-limit, and memory statements.
func TestHelpFooterNonEmpty(t *testing.T) {
	footer := app.HelpFooter()
	if footer == "" {
		t.Fatalf("HelpFooter is empty, want Issue #34 scale/record-limit/memory statements")
	}
}

// TestHelpFooterNoDangerousControls verifies that the help footer text
// itself contains no dangerous control bytes. The footer is fixed
// app-authored text, but this guards against future runtime-string
// substitutions.
func TestHelpFooterNoDangerousControls(t *testing.T) {
	footer := app.HelpFooter()
	if footer == "" {
		t.Fatalf("HelpFooter is empty")
	}
	if !sinkfixtures.NoDangerousControls(footer) {
		t.Fatalf("dangerous control byte in help footer: %q", footer)
	}
}

// TestHelpFooterSinkSafetyNoStyle verifies that the rendered help
// overlay with the Issue #34 footer passes the Issue #6 sink-safety
// check through the no-style composition path: no fixture control
// byte survives in raw output before any ANSI stripping. The footer
// is routed through the Issue #6 utility so any runtime-string
// substitution point stays safe.
func TestHelpFooterSinkSafetyNoStyle(t *testing.T) {
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
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 50})
	m, _ = update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	m, _ = update(t, m, keyPress('h'))
	if !m.OverlayOpen() {
		t.Fatalf("help overlay not open")
	}
	view := viewContent(m)
	// The footer must be non-empty and rendered in the overlay.
	footer := app.HelpFooter()
	if footer == "" {
		t.Fatalf("HelpFooter is empty")
	}
	// Assert no dangerous control bytes in the raw no-style output.
	if !sinkfixtures.NoDangerousControls(view) {
		t.Fatalf("dangerous control byte in help overlay with footer (no-style): %q", view)
	}
}

// TestHelpFooterSinkSafetyStyled verifies that with styles enabled,
// the help overlay with footer contains no fixture payload after an
// unescaped ESC. The footer is fixed text, but this guards the
// sink-safety row against future runtime-string substitutions.
func TestHelpFooterSinkSafetyStyled(t *testing.T) {
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
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 50})
	m, _ = update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	m, _ = update(t, m, keyPress('h'))
	if !m.OverlayOpen() {
		t.Fatalf("help overlay not open")
	}
	view := viewContent(m)
	for _, fx := range sinkfixtures.Fixtures {
		if !sinkfixtures.NoPayloadAfterESC(view, fx.Payload) {
			t.Fatalf("fixture payload after unescaped ESC in help overlay with footer for %s: %q", fx.Name, view)
		}
	}
}

// TestHelpFooterRenderedInOverlay verifies that the rendered help
// overlay actually contains the footer text, so the footer is visible
// to the user and not just returned by the API.
func TestHelpFooterRenderedInOverlay(t *testing.T) {
	footer := app.HelpFooter()
	if footer == "" {
		t.Fatalf("HelpFooter is empty")
	}
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
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 50})
	m, _ = update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	m, _ = update(t, m, keyPress('h'))
	if !m.OverlayOpen() {
		t.Fatalf("help overlay not open")
	}
	view := viewContent(m)
	// At least one distinctive token from the footer must appear in
	// the rendered overlay. The footer may be wrapped across lines, so
	// check for a short distinctive substring.
	found := false
	for _, tk := range []string{"64 MiB", "10,000", "50 MB", "OOM"} {
		if strings.Contains(view, tk) && strings.Contains(footer, tk) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("help overlay view does not contain any footer token (64 MiB, 10,000, 50 MB, OOM): %q", view)
	}
}
