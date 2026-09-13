package app_test

import (
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/app"
	"vrg/internal/filebuffer"
	"vrg/internal/searchindex"
)

// --- Issue #30: Unsupported-encoding App integration tests ---
//
// These tests cover the App-level contracts for UTF-16/UTF-32 BOM
// detection: the current/non-current notification distinction (current
// → overlay + "(unsupported encoding)" placeholder; non-current →
// diagnostic-only), reloadability through r (reissuing the load and
// preserving the placeholder when unchanged), the absence of stale
// validation against raw encoded bytes, the all-unsupported
// outcome-matrix row that leaves the fixed exit status unchanged, and
// composed-view robustness at ordinary and constrained widths (truncated
// safe path, nothing overflowing, nonnegative dimensions).

// unsupportedEnc is the exact placeholder text the panel must show for
// an unsupported-encoding file (Issue #30).
const unsupportedEnc = "(unsupported encoding)"

// makeUnsupportedBuf creates a filebuffer.Buffer that signals an
// unsupported encoding with the given diagnostic. The buffer has no
// lines, no highlights, and Stale=false (stale validation is excluded
// for unsupported encodings).
func makeUnsupportedBuf(diag string) *filebuffer.Buffer {
	return &filebuffer.Buffer{
		UnsupportedEncoding: true,
		EncodingDiagnostic:  diag,
		GutterWidth:         3,
	}
}

// setupBrowseUnsupported creates a browse model where the given paths
// return unsupported-encoding buffers from the loader. The gate is
// released immediately so loads complete. Returns the model and the
// async load channel for the startup file.
func setupBrowseUnsupported(t *testing.T, idx *searchindex.Index, unsupported map[string]string, bufs map[string]*filebuffer.Buffer) (app.Model, <-chan app.FileLoadCompleteMsg) {
	t.Helper()
	gate := make(chan struct{})
	close(gate)
	loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		if diag, ok := unsupported[string(path)]; ok {
			return makeUnsupportedBuf(diag), nil
		}
		if buf, ok := bufs[string(path)]; ok {
			return buf, nil
		}
		return &filebuffer.Buffer{}, nil
	}
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(gate),
		app.WithFileLoader(loader),
		app.WithPopupDuration(0),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	if m.State() != app.StateBrowse {
		t.Fatalf("State = %v, want StateBrowse", m.State())
	}
	loadCh := startLoadAsync(cmd)
	return m, loadCh
}

// --- PRD #47: Current-file unsupported encoding ---

// TestUnsupportedCurrentFileOverlayPlaceholder verifies that a
// current-file unsupported encoding shows the Issue #9 error overlay
// with the encoding diagnostic and the "(unsupported encoding)"
// placeholder in the panel, while the file's cursor stops are retained
// and the filename row still identifies the path.
func TestUnsupportedCurrentFileOverlayPlaceholder(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/a.go", "y\n", 3, subSpec{"y", 0, 1}),
		textMatch("src/b.go", "z\n", 1, subSpec{"z", 0, 1}),
	)
	unsupported := map[string]string{
		"src/a.go": "unsupported encoding: UTF-16 LE",
	}
	bufs := map[string]*filebuffer.Buffer{
		"src/b.go": makeBuf([]filebuffer.Line{ml(1, "content-b")}, 1, 3),
	}
	m, loadCh := setupBrowseUnsupported(t, idx, unsupported, bufs)

	// Startup file A is unsupported.
	lc := <-loadCh
	m = deliverCompletion(t, m, lc)

	// Error overlay should be open with the encoding diagnostic.
	if !m.OverlayOpen() {
		t.Fatalf("overlay should be open after current-file unsupported encoding")
	}
	if m.OverlayKind() != app.OverlayError {
		t.Fatalf("OverlayKind = %v, want OverlayError", m.OverlayKind())
	}
	if m.OverlayFatal() {
		t.Fatalf("unsupported-encoding overlay should be non-fatal (dismissal returns to browse)")
	}

	// Dismiss the overlay to see the panel content.
	m, _ = update(t, m, keyPressOrEscape(tea.KeyEscape))
	if m.OverlayOpen() {
		t.Fatalf("overlay should be closed after Esc")
	}

	// Panel should show "(unsupported encoding)".
	view := viewContent(m)
	if !strings.Contains(view, unsupportedEnc) {
		t.Fatalf("view should contain %q:\n%s", unsupportedEnc, view)
	}
	if strings.Contains(view, "Loading") {
		t.Fatalf("view should not contain 'Loading' after unsupported detection:\n%s", view)
	}

	// Cursor stops should be retained: n should advance within the
	// same file (A has two stops).
	cursorBefore := m.CursorPosition()
	m, _ = update(t, m, keyPress('n'))
	if m.CursorPosition() == cursorBefore {
		t.Fatalf("cursor did not advance after n (stops not retained)")
	}
	assertCurrentPath(t, m, "src/a.go")

	// Filename row should still identify the path.
	view = viewContent(m)
	if !strings.Contains(view, "a.go") {
		t.Fatalf("view should contain 'a.go' (filename row identifies path):\n%s", view)
	}

	// The encoding diagnostic should be collected for replay.
	diags := m.Diagnostics()
	if len(diags) == 0 {
		t.Fatalf("no diagnostics collected after current-file unsupported encoding")
	}
	found := false
	for _, d := range diags {
		if strings.Contains(d, "UTF-16") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("diagnostics %q do not contain 'UTF-16'", diags)
	}
}

// TestUnsupportedCurrentFileDismissReturnsToBrowse verifies that
// dismissing the unsupported-encoding overlay (q or Esc) returns to
// the browse state with the "(unsupported encoding)" placeholder still
// shown, not exiting.
func TestUnsupportedCurrentFileDismissReturnsToBrowse(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "z\n", 1, subSpec{"z", 0, 1}),
	)
	unsupported := map[string]string{
		"src/a.go": "unsupported encoding: UTF-16 LE",
	}
	bufs := map[string]*filebuffer.Buffer{
		"src/b.go": makeBuf([]filebuffer.Line{ml(1, "content-b")}, 1, 3),
	}
	m, loadCh := setupBrowseUnsupported(t, idx, unsupported, bufs)

	lc := <-loadCh
	m = deliverCompletion(t, m, lc)
	if !m.OverlayOpen() {
		t.Fatalf("overlay should be open")
	}

	// Esc dismisses the overlay (non-fatal).
	m, cmd := update(t, m, keyPressOrEscape(tea.KeyEscape))
	if m.OverlayOpen() {
		t.Fatalf("overlay should be closed after Esc")
	}
	if cmd != nil {
		msg := execCmd(t, cmd)
		if _, ok := msg.(tea.QuitMsg); ok {
			t.Fatalf("Esc on non-fatal unsupported-encoding overlay should not quit")
		}
	}
	if m.State() != app.StateBrowse {
		t.Fatalf("State = %v, want StateBrowse after Esc dismiss", m.State())
	}

	// Panel should still show "(unsupported encoding)".
	view := viewContent(m)
	if !strings.Contains(view, unsupportedEnc) {
		t.Fatalf("view should still contain %q after dismiss:\n%s", unsupportedEnc, view)
	}

	// q from browse should quit with the fixed exit status (0 for
	// clean rg 0).
	m, cmd = update(t, m, keyPress('q'))
	assertQuit(t, cmd)
	if m.ExitCode() != 0 {
		t.Fatalf("ExitCode = %d, want 0 (fixed status unchanged)", m.ExitCode())
	}
}

// --- PRD #37 analog: Non-current unsupported encoding is diagnostic-only ---

// TestUnsupportedNonCurrentDiagnosticOnly verifies that a non-current
// file with an unsupported encoding is collected as a diagnostic only
// — no overlay, no in-UI indicator — and is discovered by visiting
// that file (which then shows its overlay and placeholder).
func TestUnsupportedNonCurrentDiagnosticOnly(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "y\n", 1, subSpec{"y", 0, 1}),
	)
	// Use a gated loader so B's load can complete while B is
	// non-current (the user navigated away before it finished).
	loader := newGatedLoader()
	loader.set("src/a.go", makeBuf([]filebuffer.Line{ml(1, "content-a")}, 1, 3))
	loader.set("src/b.go", makeUnsupportedBuf("unsupported encoding: UTF-32 BE"))
	m, loadA := setupBrowseGated(t, idx, loader)

	// Release A's gate so A loads successfully.
	loader.release("src/a.go")
	lc := <-loadA
	m = deliverCompletion(t, m, lc)

	// No overlay should be open (A loaded fine).
	if m.OverlayOpen() {
		t.Fatalf("overlay should not be open after A loads")
	}

	// Navigate to B (cross-file), triggering a load for B. Then
	// immediately navigate back to A before B's load completes.
	m, cmd := update(t, m, keyPress('n'))
	assertCurrentPath(t, m, "src/b.go")
	// Start B's load asynchronously so we can wait for it to reach
	// the gated loader, then navigate back before it completes.
	loadBCh := startLoadAsync(cmd)
	loader.waitStarted("src/b.go")
	// B's load has started but is gated. Navigate back to A.
	m, cmd2 := update(t, m, keyPress('p'))
	assertCurrentPath(t, m, "src/a.go")
	// Deliver any popup/layout commands from the navigation back.
	m = deliverLayout(t, m, cmd2)

	// Now release B's gate. B's load completes while B is
	// non-current. The diagnostic should be collected without
	// opening an overlay.
	loader.release("src/b.go")
	// Wait for B's load to complete and deliver it.
	lcB := <-loadBCh
	m = deliverCompletion(t, m, lcB)

	// No overlay should be open (B is non-current).
	if m.OverlayOpen() {
		t.Fatalf("overlay should not be open for non-current unsupported encoding")
	}

	// The non-current diagnostic should be collected for replay.
	diags := m.Diagnostics()
	found := false
	for _, d := range diags {
		if strings.Contains(d, "UTF-32") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("non-current unsupported-encoding diagnostic not in replay: %q", diags)
	}

	// Navigate to B (the unsupported file, now cached). The overlay
	// should open and the panel should show "(unsupported encoding)".
	m, navCmd := update(t, m, keyPress('n'))
	assertCurrentPath(t, m, "src/b.go")
	m = deliverLayout(t, m, navCmd)

	if !m.OverlayOpen() {
		t.Fatalf("overlay should be open after navigating to unsupported file")
	}
	// Dismiss the overlay to see the panel content.
	m, _ = update(t, m, keyPressOrEscape(tea.KeyEscape))
	view := viewContent(m)
	if !strings.Contains(view, unsupportedEnc) {
		t.Fatalf("view should contain %q after visiting unsupported file:\n%s", unsupportedEnc, view)
	}
}

// --- PRD #40/#47: Reloadability through r ---

// TestUnsupportedReloadViaR verifies that pressing r on an
// unsupported-encoding file shows "Loading…" and then re-detects the
// BOM, showing the placeholder again with a fresh overlay. The
// placeholder is preserved when the content is unchanged.
func TestUnsupportedReloadViaR(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "z\n", 1, subSpec{"z", 0, 1}),
	)
	unsupported := map[string]string{
		"src/a.go": "unsupported encoding: UTF-16 LE",
	}
	bufs := map[string]*filebuffer.Buffer{
		"src/b.go": makeBuf([]filebuffer.Line{ml(1, "content-b")}, 1, 3),
	}
	m, loadCh := setupBrowseUnsupported(t, idx, unsupported, bufs)

	// Startup file A is unsupported.
	lc := <-loadCh
	m = deliverCompletion(t, m, lc)
	if !m.OverlayOpen() {
		t.Fatalf("overlay should be open after unsupported detection")
	}

	// Dismiss the overlay.
	m, _ = update(t, m, keyPressOrEscape(tea.KeyEscape))
	if m.OverlayOpen() {
		t.Fatalf("overlay should be closed after Esc")
	}

	// Panel shows "(unsupported encoding)".
	view := viewContent(m)
	if !strings.Contains(view, unsupportedEnc) {
		t.Fatalf("view should contain %q before reload:\n%s", unsupportedEnc, view)
	}

	// Press r to reload. The panel should show "Loading…" and a new
	// load should be issued.
	m, cmd := update(t, m, keyPress('r'))
	view = viewContent(m)
	if !strings.Contains(view, "Loading") {
		t.Fatalf("view should show 'Loading…' during reload:\n%s", view)
	}
	if strings.Contains(view, unsupportedEnc) {
		t.Fatalf("view should not show %q during reload (should show Loading):\n%s", unsupportedEnc, view)
	}

	// Deliver the reload completion. The panel should show
	// "(unsupported encoding)" again with a fresh overlay.
	loadCh2 := startLoadAsync(cmd)
	lc2 := <-loadCh2
	m = deliverCompletion(t, m, lc2)

	if !m.OverlayOpen() {
		t.Fatalf("overlay should be open after reload of unsupported file")
	}

	// Dismiss the overlay to see the panel content.
	m, _ = update(t, m, keyPressOrEscape(tea.KeyEscape))
	view = viewContent(m)
	if !strings.Contains(view, unsupportedEnc) {
		t.Fatalf("view should contain %q after reload:\n%s", unsupportedEnc, view)
	}

	// Quit with fixed status 0.
	m, cmd = update(t, m, keyPress('q'))
	assertQuit(t, cmd)
	if m.ExitCode() != 0 {
		t.Fatalf("ExitCode = %d, want 0 (fixed status unchanged)", m.ExitCode())
	}
}

// --- PRD #30: No stale validation for unsupported encodings ---

// TestUnsupportedNoStaleNote verifies that an unsupported-encoding file
// does not show the "file changed since search" stale note, even when
// the recorded submatch bytes would fail validation against any text.
// The stale-match guard is excluded for these raw encoded bytes.
func TestUnsupportedNoStaleNote(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hi\n", 1, subSpec{"hi", 0, 2}),
		textMatch("src/b.go", "z\n", 1, subSpec{"z", 0, 1}),
	)
	unsupported := map[string]string{
		"src/a.go": "unsupported encoding: UTF-16 LE",
	}
	bufs := map[string]*filebuffer.Buffer{
		"src/b.go": makeBuf([]filebuffer.Line{ml(1, "content-b")}, 1, 3),
	}
	m, loadCh := setupBrowseUnsupported(t, idx, unsupported, bufs)

	// Startup file A is unsupported.
	lc := <-loadCh
	m = deliverCompletion(t, m, lc)

	// Dismiss the overlay.
	m, _ = update(t, m, keyPressOrEscape(tea.KeyEscape))

	// The view should NOT contain the stale note.
	view := viewContent(m)
	if strings.Contains(view, "file changed since search") {
		t.Fatalf("view should not contain stale note for unsupported encoding:\n%s", view)
	}
	// The view should contain "(unsupported encoding)".
	if !strings.Contains(view, unsupportedEnc) {
		t.Fatalf("view should contain %q:\n%s", unsupportedEnc, view)
	}
}

// --- Outcome-matrix row: all-unsupported leaves the fixed exit status ---

// TestUnsupportedOutcomeAllUnsupportedFixed0 verifies that every
// retained file having an unsupported encoding still exits with the
// fixed status (0 for clean rg 0). Unsupported encodings affect only
// file presentation and diagnostics, not the exit status.
func TestUnsupportedOutcomeAllUnsupportedFixed0(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "y\n", 1, subSpec{"y", 0, 1}),
	)
	unsupported := map[string]string{
		"src/a.go": "unsupported encoding: UTF-16 LE",
		"src/b.go": "unsupported encoding: UTF-32 BE",
	}
	m, loadCh := setupBrowseUnsupported(t, idx, unsupported, nil)

	// A is unsupported (startup file).
	lc := <-loadCh
	m = deliverCompletion(t, m, lc)

	// The overlay should be open with the encoding diagnostic.
	if !m.OverlayOpen() {
		t.Fatalf("overlay should be open for unsupported file A")
	}

	// Dismiss the overlay, navigate to B (also unsupported).
	m, _ = update(t, m, keyPressOrEscape(tea.KeyEscape))
	m, cmd := update(t, m, keyPress('n'))
	assertCurrentPath(t, m, "src/b.go")
	// B is uncached, so a load is started. Deliver the load
	// completion (which includes the layout preparation).
	m = deliverLoad(t, m, cmd)

	// B should also show the overlay.
	if !m.OverlayOpen() {
		t.Fatalf("overlay should be open for unsupported file B")
	}

	// Both files are unsupported. Exit status should still be 0.
	if m.ExitCode() != 0 {
		t.Fatalf("ExitCode = %d, want 0 (all files unsupported, fixed status 0)", m.ExitCode())
	}

	// Dismiss the overlay and quit.
	m, _ = update(t, m, keyPressOrEscape(tea.KeyEscape))
	m, cmd = update(t, m, keyPress('q'))
	assertQuit(t, cmd)
	if m.ExitCode() != 0 {
		t.Fatalf("ExitCode = %d, want 0 after q (all files unsupported, fixed status 0)", m.ExitCode())
	}
}

// --- Composed-view robustness ---

// TestUnsupportedComposedViewRobustness verifies that the
// unsupported-encoding state with a long escaped path at ordinary and
// constrained widths keeps the composed view well-formed: the filename
// row shows the truncated safe path per Issue #24's slot rules, the
// panel shows "(unsupported encoding)", nothing overflows the
// terminal, and layout dimensions stay nonnegative.
func TestUnsupportedComposedViewRobustness(t *testing.T) {
	longPath := "src/a" + strings.Repeat("x", 60) + ".go"
	idx := buildIndex(t, "/work",
		textMatch(longPath, "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "y\n", 1, subSpec{"y", 0, 1}),
	)
	unsupported := map[string]string{
		longPath: "unsupported encoding: UTF-16 LE",
	}
	bufs := map[string]*filebuffer.Buffer{
		"src/b.go": makeBuf([]filebuffer.Line{ml(1, "content-b")}, 1, 3),
	}

	for _, width := range []int{80, 40, 20} {
		t.Run("width-"+strconv.Itoa(width), func(t *testing.T) {
			gate := make(chan struct{})
			close(gate)
			loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
				if diag, ok := unsupported[string(path)]; ok {
					return makeUnsupportedBuf(diag), nil
				}
				if buf, ok := bufs[string(path)]; ok {
					return buf, nil
				}
				return &filebuffer.Buffer{}, nil
			}
			m := app.New([]string{"--json", "--", "foo", "."}, "/work",
				app.WithFileLoadGate(gate),
				app.WithFileLoader(loader),
				app.WithPopupDuration(0),
			)
			m, _ = update(t, m, tea.WindowSizeMsg{Width: width, Height: 24})
			m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
			loadCh := startLoadAsync(cmd)
			lc := <-loadCh
			m = deliverCompletion(t, m, lc)

			// Overlay should be open (current file is unsupported).
			if !m.OverlayOpen() {
				t.Fatalf("overlay should be open after unsupported encoding")
			}

			// Dismiss the overlay to see the composed view.
			m, _ = update(t, m, keyPressOrEscape(tea.KeyEscape))

			view := viewContent(m)

			// Panel should show "(unsupported encoding)" (or a
			// truncation of it at very constrained widths).
			if !strings.Contains(view, "(unsupporte") {
				t.Fatalf("view should contain '(unsupporte...' at width %d:\n%s", width, view)
			}

			// Filename row should still identify the path (at least
			// the ".go" suffix).
			if !strings.Contains(view, ".go") {
				t.Fatalf("view should contain '.go' (filename row identifies path) at width %d:\n%s", width, view)
			}

			// Nothing should overflow the terminal width.
			if !noLineExceedsWidth(view, width) {
				t.Fatalf("view has lines exceeding width %d:\n%s", width, view)
			}

			// Layout dimensions should be nonnegative.
			if m.ListWidth() < 0 {
				t.Fatalf("ListWidth = %d, want nonnegative", m.ListWidth())
			}
			panelWidth := width - m.ListWidth() - 1
			if panelWidth < 0 {
				t.Fatalf("panelWidth = %d, want nonnegative", panelWidth)
			}
		})
	}
}

// --- Real file integration: filebuffer.Load detects BOM ---

// TestUnsupportedRealFileUTF16LE verifies that a real UTF-16 LE file on
// disk is detected by filebuffer.Load and presented through the App
// with the "(unsupported encoding)" placeholder and overlay.
func TestUnsupportedRealFileUTF16LE(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/b.go", "z\n", 1, subSpec{"z", 0, 1}),
		textMatch("utf16.txt", "hi\n", 1, subSpec{"hi", 0, 2}),
	)
	dir := t.TempDir()
	utf16Path := writeFileToDir(t, dir, "utf16.txt",
		[]byte{0xFF, 0xFE, 'h', 0x00, 'i', 0x00, 0x0A, 0x00})
	bPath := writeFileToDir(t, dir, "src/b.go", []byte("content-b\n"))
	pathMap := map[string]string{
		"utf16.txt": utf16Path,
		"src/b.go":  bPath,
	}
	loader := func(path []byte, stops []searchindex.Stop) (*filebuffer.Buffer, error) {
		if full, ok := pathMap[string(path)]; ok {
			return filebuffer.Load([]byte(full), stops)
		}
		return filebuffer.Load(path, stops)
	}
	gate := make(chan struct{})
	close(gate)
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(gate),
		app.WithFileLoader(loader),
		app.WithPopupDuration(0),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	loadCh := startLoadAsync(cmd)
	lc := <-loadCh
	m = deliverCompletion(t, m, lc)

	// Startup file B loads successfully.
	if m.OverlayOpen() {
		t.Fatalf("overlay should not be open for normal startup file")
	}

	// Navigate to the UTF-16 file (n). It should be detected and
	// the overlay opened.
	m, navCmd := update(t, m, keyPress('n'))
	assertCurrentPath(t, m, "utf16.txt")
	m = deliverLoad(t, m, navCmd)

	if !m.OverlayOpen() {
		t.Fatalf("overlay should be open for UTF-16 LE file")
	}

	// Dismiss and check the placeholder.
	m, _ = update(t, m, keyPressOrEscape(tea.KeyEscape))
	view := viewContent(m)
	if !strings.Contains(view, unsupportedEnc) {
		t.Fatalf("view should contain %q for real UTF-16 LE file:\n%s", unsupportedEnc, view)
	}

	// The diagnostic should mention UTF-16.
	diags := m.Diagnostics()
	found := false
	for _, d := range diags {
		if strings.Contains(d, "UTF-16") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("diagnostics should mention UTF-16: %q", diags)
	}
}
