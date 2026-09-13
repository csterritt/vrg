package app_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/app"
	"vrg/internal/filebuffer"
	"vrg/internal/searchindex"
)

// --- Issue #32: Overlay precedence and dismissal semantics ---

// setupPrecedenceBrowse creates a browse model with a single file whose
// load is gated by a gatedFailingLoader. The startup file's load is held
// until the test releases the gate, so the test can open help (or
// scroll an error overlay) before the load completes and fails.
func setupPrecedenceBrowse(t *testing.T) (app.Model, *gatedFailingLoader, <-chan app.FileLoadCompleteMsg) {
	t.Helper()
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
		textMatch("src/b.go", "world\n", 1, subSpec{"world", 0, 1}),
	)
	loader := newGatedFailingLoader()
	loader.set("src/a.go", makeBuf([]filebuffer.Line{ml(1, "content-a")}, 1, 3))
	loader.set("src/b.go", makeBuf([]filebuffer.Line{ml(1, "content-b")}, 1, 3))
	gate := make(chan struct{})
	close(gate)
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(gate),
		app.WithFileLoader(loader.load),
		app.WithPopupDuration(0),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	if m.State() != app.StateBrowse {
		t.Fatalf("State = %v, want StateBrowse", m.State())
	}
	loadCh := startLoadAsync(cmd)
	return m, loader, loadCh
}

// --- Error suspends help with scroll restoration ---

// TestErrorSuspendsHelpScrollRestoredQ verifies that a new error while
// help is open at scroll position S suspends help, and dismissing the
// error with q restores help at S.
func TestErrorSuspendsHelpScrollRestoredQ(t *testing.T) {
	m, loader, loadCh := setupPrecedenceBrowse(t)

	// Open help.
	m, _ = update(t, m, keyPress('h'))
	if !m.OverlayOpen() || m.OverlayKind() != app.OverlayHelp {
		t.Fatalf("help overlay not open")
	}

	// Scroll help to position S.
	const scrollS = 3
	for i := 0; i < scrollS; i++ {
		m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}
	if m.OverlayScroll() != scrollS {
		t.Fatalf("help scroll = %d, want %d", m.OverlayScroll(), scrollS)
	}

	// Make the startup file fail, then release the gate so the load
	// completes with an error while help is open.
	loader.setFailing("src/a.go", "permission denied")
	loader.release("src/a.go")
	lc := <-loadCh
	m = deliverCompletion(t, m, lc)

	// The error overlay should be open (help suspended).
	if !m.OverlayOpen() {
		t.Fatalf("overlay not open after error, want error overlay")
	}
	if m.OverlayKind() != app.OverlayError {
		t.Fatalf("OverlayKind = %v, want OverlayError", m.OverlayKind())
	}
	if !m.HelpSuspended() {
		t.Fatalf("HelpSuspended = false, want true (help should be suspended)")
	}

	// Dismiss the error with q; help should be restored at S.
	m, cmd := update(t, m, keyPress('q'))
	if cmd != nil {
		t.Fatalf("q on error-over-help produced a command: %v", cmd)
	}
	if !m.OverlayOpen() {
		t.Fatalf("overlay closed after q, want help restored")
	}
	if m.OverlayKind() != app.OverlayHelp {
		t.Fatalf("OverlayKind = %v, want OverlayHelp (restored)", m.OverlayKind())
	}
	if m.OverlayScroll() != scrollS {
		t.Fatalf("restored help scroll = %d, want %d", m.OverlayScroll(), scrollS)
	}
}

// TestErrorSuspendsHelpScrollRestoredEsc verifies that a new error while
// help is open at scroll position S suspends help, and dismissing the
// error with Esc restores help at S.
func TestErrorSuspendsHelpScrollRestoredEsc(t *testing.T) {
	m, loader, loadCh := setupPrecedenceBrowse(t)

	// Open help.
	m, _ = update(t, m, keyPress('h'))
	if !m.OverlayOpen() || m.OverlayKind() != app.OverlayHelp {
		t.Fatalf("help overlay not open")
	}

	// Scroll help to position S.
	const scrollS = 2
	for i := 0; i < scrollS; i++ {
		m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}
	if m.OverlayScroll() != scrollS {
		t.Fatalf("help scroll = %d, want %d", m.OverlayScroll(), scrollS)
	}

	// Trigger a read failure while help is open.
	loader.setFailing("src/a.go", "permission denied")
	loader.release("src/a.go")
	lc := <-loadCh
	m = deliverCompletion(t, m, lc)

	// The error overlay should be open (help suspended).
	if m.OverlayKind() != app.OverlayError {
		t.Fatalf("OverlayKind = %v, want OverlayError", m.OverlayKind())
	}
	if !m.HelpSuspended() {
		t.Fatalf("HelpSuspended = false, want true")
	}

	// Dismiss the error with Esc; help should be restored at S.
	m, cmd := update(t, m, keyPressOrEscape(tea.KeyEscape))
	if cmd != nil {
		t.Fatalf("Esc on error-over-help produced a command: %v", cmd)
	}
	if !m.OverlayOpen() {
		t.Fatalf("overlay closed after Esc, want help restored")
	}
	if m.OverlayKind() != app.OverlayHelp {
		t.Fatalf("OverlayKind = %v, want OverlayHelp (restored)", m.OverlayKind())
	}
	if m.OverlayScroll() != scrollS {
		t.Fatalf("restored help scroll = %d, want %d", m.OverlayScroll(), scrollS)
	}
}

// --- Append preserves scroll for all appended errors ---

// TestAppendErrorPreservesScroll verifies that a second error appended
// to an overlay the reader has scrolled to position P keeps the reader
// at P with the new text reachable. This generalizes the Issue #26
// append-preserving-scroll primitive to all appended errors. The
// second error is triggered by pressing r (reload) through the open
// read-failure overlay (Issue #27), which fails again.
func TestAppendErrorPreservesScroll(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/a.go", "y\n", 3, subSpec{"y", 0, 1}),
		textMatch("src/b.go", "z\n", 1, subSpec{"z", 0, 1}),
	)
	loader := newGatedFailingLoader()
	loader.set("src/a.go", makeBuf([]filebuffer.Line{ml(1, "content-a")}, 1, 3))
	loader.set("src/b.go", makeBuf([]filebuffer.Line{ml(1, "content-b")}, 1, 3))
	loader.setFailing("src/a.go", "first error")
	gate := make(chan struct{})
	close(gate)
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(gate),
		app.WithFileLoader(loader.load),
		app.WithPopupDuration(0),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	if m.State() != app.StateBrowse {
		t.Fatalf("State = %v, want StateBrowse", m.State())
	}

	// Startup file A fails to load → read-failure overlay opens.
	loader.release("src/a.go")
	lc := <-startLoadAsync(cmd)
	m = deliverCompletion(t, m, lc)
	if !m.OverlayOpen() || m.OverlayKind() != app.OverlayError {
		t.Fatalf("error overlay not open after first failure")
	}

	// Scroll the overlay to position P.
	const scrollP = 2
	for i := 0; i < scrollP; i++ {
		m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}
	if m.OverlayScroll() != scrollP {
		t.Fatalf("overlay scroll = %d, want %d", m.OverlayScroll(), scrollP)
	}

	// Press r to retry through the open read-failure overlay (Issue
	// #27). The overlay stays open; the reload starts.
	m, reloadCmd := update(t, m, keyPress('r'))
	// Rearm the gate so the retry blocks until we release it.
	loader.rearm("src/a.go")
	loadRetry := startLoadAsync(reloadCmd)
	// The retry fails again → second error appended.
	loader.release("src/a.go")
	lc2 := <-loadRetry
	m = deliverCompletion(t, m, lc2)

	// The overlay should still be open with the appended error.
	if !m.OverlayOpen() {
		t.Fatalf("overlay closed after append, want still open")
	}
	if m.OverlayKind() != app.OverlayError {
		t.Fatalf("OverlayKind = %v, want OverlayError", m.OverlayKind())
	}
	// Scroll should be preserved at P (not reset to 0).
	if m.OverlayScroll() != scrollP {
		t.Fatalf("overlay scroll = %d after append, want %d (preserved)", m.OverlayScroll(), scrollP)
	}
	// The new text should be reachable by scrolling down.
	overlayText := m.OverlayText()
	if !strings.Contains(overlayText, "first error") {
		t.Fatalf("overlay text does not contain 'first error': %q", overlayText)
	}
}

// --- Pop-up cancellation by error ---

// TestErrorCancelsPopup verifies that opening an error overlay cancels
// an active pop-up with no return after dismissal. The pop-up opens on
// cross-file navigation; the destination file's load failure opens the
// error overlay, which cancels the pop-up.
func TestErrorCancelsPopup(t *testing.T) {
	idx := buildIndex(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "z\n", 1, subSpec{"z", 0, 1}),
	)
	loader := newGatedFailingLoader()
	loader.set("src/a.go", makeBuf([]filebuffer.Line{ml(1, "content-a")}, 1, 3))
	loader.set("src/b.go", nil)
	loader.setFailing("src/b.go", "permission denied")
	gate := make(chan struct{})
	close(gate)
	m := app.New([]string{"--json", "--", "foo", "."}, "/work",
		app.WithFileLoadGate(gate),
		app.WithFileLoader(loader.load),
		app.WithPopupDuration(0),
	)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := update(t, m, app.SearchCompleteMsg{Files: idx.Files(), Lines: idx.Len(), Index: idx})
	if m.State() != app.StateBrowse {
		t.Fatalf("State = %v, want StateBrowse", m.State())
	}
	// A loads successfully.
	loader.release("src/a.go")
	lc := <-startLoadAsync(cmd)
	m = deliverCompletion(t, m, lc)

	// Navigate cross-file to B → pop-up opens, B starts loading.
	m, navCmd := update(t, m, keyPress('n'))
	assertCurrentPath(t, m, "src/b.go")
	if !m.PopupOpen() {
		t.Fatalf("pop-up not open after cross-file n")
	}
	loadB := startLoadAsync(navCmd)

	// B fails to load → error overlay opens, pop-up cancelled.
	loader.release("src/b.go")
	lc2 := <-loadB
	m = deliverCompletion(t, m, lc2)
	if !m.OverlayOpen() {
		t.Fatalf("error overlay not open after read failure")
	}
	if m.PopupOpen() {
		t.Fatalf("pop-up still open after error, want cancelled")
	}

	// Dismiss the error; pop-up must not return.
	m, _ = update(t, m, keyPressOrEscape(tea.KeyEscape))
	if m.OverlayOpen() {
		t.Fatalf("overlay still open after Esc")
	}
	if m.PopupOpen() {
		t.Fatalf("pop-up returned after error dismissed, want no return")
	}
}

// --- Esc no-op with pop-up dismissal ---

// TestEscNoOverlayDismissesPopup verifies that Esc with no overlay open
// in browsing dismisses a pop-up (its only effect) and otherwise does
// nothing.
func TestEscNoOverlayDismissesPopup(t *testing.T) {
	m := setupBrowsePopup(t)
	// Navigate cross-file to open the pop-up.
	m, _ = update(t, m, keyPress('n'))
	if !m.PopupOpen() {
		t.Fatalf("pop-up not open after cross-file n")
	}
	// Esc with no overlay should dismiss the pop-up and do nothing else.
	m, cmd := update(t, m, keyPressOrEscape(tea.KeyEscape))
	if cmd != nil {
		t.Fatalf("Esc produced a command: %v", cmd)
	}
	if m.PopupOpen() {
		t.Fatalf("pop-up still open after Esc, want dismissed")
	}
	if m.State() != app.StateBrowse {
		t.Fatalf("State = %v, want StateBrowse", m.State())
	}
}

// --- Dismissal-outcome table for q and Esc ---

// dismissalOutcomeCase describes one row of the dismissal-outcome table
// for Issue #32. Each row is run for both q and Esc as the dismissal
// key.
type dismissalOutcomeCase struct {
	name string
	// setup returns a model with the overlay open and the base state
	// established.
	setup func(t *testing.T) app.Model
	// wantOverlay is the expected overlay kind before dismissal.
	wantOverlay app.OverlayKind
	// wantStateBefore is the base state under the overlay.
	wantStateBefore app.State
	// fatal is true when the overlay is fatal (dismissal exits 2).
	fatal bool
	// wantStateAfter is the state after dismissing the overlay (for
	// non-fatal rows). For the error-over-help row, this is the state
	// with help restored.
	wantStateAfter app.State
	// helpRestored is true when dismissing the error restores help
	// (error-over-help row).
	helpRestored bool
	// fixedExit is the fixed exit status for the base state (used for
	// the second-q assertion).
	fixedExit int
}

// runDismissalOutcome runs one dismissal-outcome row for a given
// dismissal key (q or Esc). It asserts the overlay is open, dismisses
// it, and then runs the state-specific follow-up assertions.
func runDismissalOutcome(t *testing.T, tc dismissalOutcomeCase, dismissKey rune) {
	t.Helper()
	m := tc.setup(t)
	if !m.OverlayOpen() {
		t.Fatalf("overlay not open")
	}
	if m.OverlayKind() != tc.wantOverlay {
		t.Fatalf("OverlayKind = %v, want %v", m.OverlayKind(), tc.wantOverlay)
	}

	// Dismiss the overlay.
	m, cmd := update(t, m, keyPressOrEscape(dismissKey))
	if tc.fatal {
		// Fatal no-results overlay: dismissal exits 2.
		assertQuit(t, cmd)
		if m.ExitCode() != 2 {
			t.Fatalf("after dismiss, ExitCode = %d, want 2", m.ExitCode())
		}
		return
	}
	// Non-fatal: dismissal closes the overlay (or restores help).
	if cmd != nil {
		t.Fatalf("dismiss produced a command: %v", cmd)
	}
	if tc.helpRestored {
		// Error-over-help: dismissing the error restores help.
		if !m.OverlayOpen() {
			t.Fatalf("overlay closed after dismiss, want help restored")
		}
		if m.OverlayKind() != app.OverlayHelp {
			t.Fatalf("OverlayKind = %v, want OverlayHelp (restored)", m.OverlayKind())
		}
		// Next q or Esc closes the restored help.
		m, cmd = update(t, m, keyPressOrEscape(dismissKey))
		if cmd != nil {
			t.Fatalf("second dismiss produced a command: %v", cmd)
		}
		if m.OverlayOpen() {
			t.Fatalf("overlay still open after second dismiss, want closed")
		}
		if m.State() != tc.wantStateAfter {
			t.Fatalf("after second dismiss, State = %v, want %v", m.State(), tc.wantStateAfter)
		}
	} else {
		if m.OverlayOpen() {
			t.Fatalf("overlay still open after dismiss")
		}
		if m.State() != tc.wantStateAfter {
			t.Fatalf("after dismiss, State = %v, want %v", m.State(), tc.wantStateAfter)
		}
	}

	// State-specific follow-up: a second q exits the fixed status; a
	// second Esc leaves it running.
	if dismissKey == 'q' {
		m, cmd = update(t, m, keyPress('q'))
		assertQuit(t, cmd)
		if m.ExitCode() != tc.fixedExit {
			t.Fatalf("after second q, ExitCode = %d, want %d", m.ExitCode(), tc.fixedExit)
		}
	} else {
		// Esc: leaves the base state running (no quit).
		m, cmd = update(t, m, keyPressOrEscape(tea.KeyEscape))
		if cmd != nil {
			msg := execCmd(t, cmd)
			if _, ok := msg.(tea.QuitMsg); ok {
				t.Fatalf("second Esc quit from base state, want no-op")
			}
		}
		if m.State() != tc.wantStateAfter {
			t.Fatalf("after second Esc, State = %v, want %v", m.State(), tc.wantStateAfter)
		}
	}
}

// setupBrowseErrorOverlay creates a browse model with a non-fatal error
// overlay open (fatal search with usable results, exit 2).
func setupBrowseErrorOverlay(t *testing.T) app.Model {
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
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := update(t, m, app.SearchCompleteMsg{
		Files: idx.Files(), Lines: idx.Len(), Index: idx,
		Process: app.ProcessResult{ExitCode: 3},
		Stderr:  "fatal search error",
	})
	if cmd != nil {
		m = deliverLoad(t, m, cmd)
	}
	if !m.OverlayOpen() || m.OverlayKind() != app.OverlayError {
		t.Fatalf("error overlay not open")
	}
	return m
}

// setupBrowseHelpOverlay creates a browse model with help open.
func setupBrowseHelpOverlay(t *testing.T) app.Model {
	t.Helper()
	m := setupHelpBrowse(t)
	m = openHelp(t, m)
	return m
}

// setupBrowseErrorOverHelp creates a browse model with help open, then
// triggers a read failure so the error overlay suspends help.
func setupBrowseErrorOverHelp(t *testing.T) app.Model {
	t.Helper()
	m, loader, loadCh := setupPrecedenceBrowse(t)
	// Open help.
	m, _ = update(t, m, keyPress('h'))
	if !m.OverlayOpen() || m.OverlayKind() != app.OverlayHelp {
		t.Fatalf("help overlay not open")
	}
	// Trigger a read failure while help is open.
	loader.setFailing("src/a.go", "permission denied")
	loader.release("src/a.go")
	lc := <-loadCh
	m = deliverCompletion(t, m, lc)
	if !m.OverlayOpen() || m.OverlayKind() != app.OverlayError {
		t.Fatalf("error overlay not open over suspended help")
	}
	return m
}

// setupNoResultsWarningOverlay creates a no-results model with a
// warning overlay open (stderr on rg 0/1, no usable results).
func setupNoResultsWarningOverlay(t *testing.T) app.Model {
	t.Helper()
	idx := buildIndex(t, "/work", summaryRecord())
	m := app.New([]string{"--json", "--", "foo", "."}, "/work")
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = update(t, m, app.SearchCompleteMsg{
		Files: idx.Files(), Lines: idx.Len(), Index: idx,
		Process: app.ProcessResult{ExitCode: 0},
		Stderr:  "warning text",
	})
	if !m.OverlayOpen() || m.OverlayKind() != app.OverlayWarning {
		t.Fatalf("warning overlay not open")
	}
	return m
}

// setupNoResultsHelpOverlay creates a no-results model with help open.
func setupNoResultsHelpOverlay(t *testing.T) app.Model {
	t.Helper()
	m := setupHelpNoResults(t)
	m = openHelp(t, m)
	return m
}

// setupFatalNoResultsOverlay creates a fatal no-results model with an
// error overlay open (no underlying state).
func setupFatalNoResultsOverlay(t *testing.T) app.Model {
	t.Helper()
	return setupOverlayFatalNoResults(t, "fatal error")
}

// setupRecordLossNoResultsOverlay creates a record-loss fatal no-results
// model with an error overlay open.
func setupRecordLossNoResultsOverlay(t *testing.T) app.Model {
	t.Helper()
	idx := buildIndexRaw(t, "/work", `{bad json`, summaryRecord())
	m := app.New([]string{"--json", "--", "foo", "."}, "/work")
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = update(t, m, app.SearchCompleteMsg{
		Files: idx.Files(), Lines: idx.Len(), Index: idx,
		Process: app.ProcessResult{ExitCode: 0},
	})
	if !m.OverlayOpen() || m.OverlayKind() != app.OverlayError {
		t.Fatalf("record-loss error overlay not open")
	}
	if !m.OverlayFatal() {
		t.Fatalf("record-loss overlay should be fatal")
	}
	return m
}

// TestDismissalOutcomeTableQ runs the dismissal-outcome table for q as
// the dismissal key. Each row asserts the overlay is open, dismisses
// it with q, and runs state-specific follow-up assertions: from every
// still-running base state a second q exits the fixed status.
func TestDismissalOutcomeTableQ(t *testing.T) {
	cases := []dismissalOutcomeCase{
		{
			name:            "browse with error overlay",
			setup:           setupBrowseErrorOverlay,
			wantOverlay:     app.OverlayError,
			wantStateBefore: app.StateBrowse,
			wantStateAfter:  app.StateBrowse,
			fixedExit:       2,
		},
		{
			name:            "browse with help",
			setup:           setupBrowseHelpOverlay,
			wantOverlay:     app.OverlayHelp,
			wantStateBefore: app.StateBrowse,
			wantStateAfter:  app.StateBrowse,
			fixedExit:       0,
		},
		{
			name:            "browse with error over help",
			setup:           setupBrowseErrorOverHelp,
			wantOverlay:     app.OverlayError,
			wantStateBefore: app.StateBrowse,
			wantStateAfter:  app.StateBrowse,
			helpRestored:    true,
			fixedExit:       0,
		},
		{
			name:            "empty result with warning overlay",
			setup:           setupNoResultsWarningOverlay,
			wantOverlay:     app.OverlayWarning,
			wantStateBefore: app.StateNoResults,
			wantStateAfter:  app.StateNoResults,
			fixedExit:       1,
		},
		{
			name:            "no-results with help open",
			setup:           setupNoResultsHelpOverlay,
			wantOverlay:     app.OverlayHelp,
			wantStateBefore: app.StateNoResults,
			wantStateAfter:  app.StateNoResults,
			fixedExit:       1,
		},
		{
			name:            "fatal with no usable results",
			setup:           setupFatalNoResultsOverlay,
			wantOverlay:     app.OverlayError,
			wantStateBefore: app.StateNoResults,
			fatal:           true,
			fixedExit:       2,
		},
		{
			name:            "record-loss with no results",
			setup:           setupRecordLossNoResultsOverlay,
			wantOverlay:     app.OverlayError,
			wantStateBefore: app.StateNoResults,
			fatal:           true,
			fixedExit:       2,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runDismissalOutcome(t, tc, 'q')
		})
	}
}

// TestDismissalOutcomeTableEsc runs the dismissal-outcome table for Esc
// as the dismissal key. Esc never exits from a base state; dismissing a
// fatal no-results overlay with Esc terminates with status 2 because
// there is no underlying state.
func TestDismissalOutcomeTableEsc(t *testing.T) {
	cases := []dismissalOutcomeCase{
		{
			name:            "browse with error overlay",
			setup:           setupBrowseErrorOverlay,
			wantOverlay:     app.OverlayError,
			wantStateBefore: app.StateBrowse,
			wantStateAfter:  app.StateBrowse,
			fixedExit:       2,
		},
		{
			name:            "browse with help",
			setup:           setupBrowseHelpOverlay,
			wantOverlay:     app.OverlayHelp,
			wantStateBefore: app.StateBrowse,
			wantStateAfter:  app.StateBrowse,
			fixedExit:       0,
		},
		{
			name:            "browse with error over help",
			setup:           setupBrowseErrorOverHelp,
			wantOverlay:     app.OverlayError,
			wantStateBefore: app.StateBrowse,
			wantStateAfter:  app.StateBrowse,
			helpRestored:    true,
			fixedExit:       0,
		},
		{
			name:            "empty result with warning overlay",
			setup:           setupNoResultsWarningOverlay,
			wantOverlay:     app.OverlayWarning,
			wantStateBefore: app.StateNoResults,
			wantStateAfter:  app.StateNoResults,
			fixedExit:       1,
		},
		{
			name:            "no-results with help open",
			setup:           setupNoResultsHelpOverlay,
			wantOverlay:     app.OverlayHelp,
			wantStateBefore: app.StateNoResults,
			wantStateAfter:  app.StateNoResults,
			fixedExit:       1,
		},
		{
			name:            "fatal with no usable results",
			setup:           setupFatalNoResultsOverlay,
			wantOverlay:     app.OverlayError,
			wantStateBefore: app.StateNoResults,
			fatal:           true,
			fixedExit:       2,
		},
		{
			name:            "record-loss with no results",
			setup:           setupRecordLossNoResultsOverlay,
			wantOverlay:     app.OverlayError,
			wantStateBefore: app.StateNoResults,
			fatal:           true,
			fixedExit:       2,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runDismissalOutcome(t, tc, tea.KeyEscape)
		})
	}
}

// --- Error-first key routing ---

// TestErrorFirstKeyRouting verifies that keys route to the error when
// both help and error are open (error takes precedence over help).
// Scrolling the error overlay should not affect the suspended help's
// scroll position; dismissing the error with q restores help at its
// original scroll position.
func TestErrorFirstKeyRouting(t *testing.T) {
	m, loader, loadCh := setupPrecedenceBrowse(t)

	// Open help and scroll to S.
	m, _ = update(t, m, keyPress('h'))
	const helpScrollS = 2
	for i := 0; i < helpScrollS; i++ {
		m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}
	if m.OverlayScroll() != helpScrollS {
		t.Fatalf("help scroll = %d, want %d", m.OverlayScroll(), helpScrollS)
	}

	// Trigger a read failure → error overlay suspends help.
	loader.setFailing("src/a.go", "permission denied")
	loader.release("src/a.go")
	lc := <-loadCh
	m = deliverCompletion(t, m, lc)
	if m.OverlayKind() != app.OverlayError {
		t.Fatalf("OverlayKind = %v, want OverlayError", m.OverlayKind())
	}
	if !m.HelpSuspended() {
		t.Fatalf("HelpSuspended = false, want true")
	}

	// Scroll the error overlay. This should affect the error's scroll,
	// not the suspended help's scroll.
	const errorScrollP = 3
	for i := 0; i < errorScrollP; i++ {
		m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}
	if m.OverlayScroll() != errorScrollP {
		t.Fatalf("error scroll = %d, want %d", m.OverlayScroll(), errorScrollP)
	}

	// Dismiss the error with q; help should be restored at S (not P).
	m, cmd := update(t, m, keyPress('q'))
	if cmd != nil {
		t.Fatalf("q produced a command: %v", cmd)
	}
	if m.OverlayKind() != app.OverlayHelp {
		t.Fatalf("OverlayKind = %v, want OverlayHelp (restored)", m.OverlayKind())
	}
	if m.OverlayScroll() != helpScrollS {
		t.Fatalf("restored help scroll = %d, want %d (original help scroll)", m.OverlayScroll(), helpScrollS)
	}
}

// TestErrorOverHelpEscRestoresHelp verifies the full three-key sequence
// for the error-over-help row with Esc: Esc closes the error and
// restores help, the next Esc closes help to browsing, and only a
// further q exits.
func TestErrorOverHelpEscRestoresHelp(t *testing.T) {
	m, loader, loadCh := setupPrecedenceBrowse(t)

	// Open help and scroll to S.
	m, _ = update(t, m, keyPress('h'))
	const helpScrollS = 1
	for i := 0; i < helpScrollS; i++ {
		m, _ = update(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}

	// Trigger a read failure → error overlay suspends help.
	loader.setFailing("src/a.go", "permission denied")
	loader.release("src/a.go")
	lc := <-loadCh
	m = deliverCompletion(t, m, lc)
	if m.OverlayKind() != app.OverlayError {
		t.Fatalf("OverlayKind = %v, want OverlayError", m.OverlayKind())
	}

	// First Esc: closes the error, restores help at S.
	m, _ = update(t, m, keyPressOrEscape(tea.KeyEscape))
	if m.OverlayKind() != app.OverlayHelp {
		t.Fatalf("after first Esc, OverlayKind = %v, want OverlayHelp", m.OverlayKind())
	}
	if m.OverlayScroll() != helpScrollS {
		t.Fatalf("after first Esc, help scroll = %d, want %d", m.OverlayScroll(), helpScrollS)
	}

	// Second Esc: closes help to browsing.
	m, cmd := update(t, m, keyPressOrEscape(tea.KeyEscape))
	if cmd != nil {
		t.Fatalf("second Esc produced a command: %v", cmd)
	}
	if m.OverlayOpen() {
		t.Fatalf("after second Esc, overlay still open")
	}
	if m.State() != app.StateBrowse {
		t.Fatalf("after second Esc, State = %v, want StateBrowse", m.State())
	}

	// Third Esc: no-op (Esc never exits from a base state).
	m, cmd = update(t, m, keyPressOrEscape(tea.KeyEscape))
	if cmd != nil {
		msg := execCmd(t, cmd)
		if _, ok := msg.(tea.QuitMsg); ok {
			t.Fatalf("third Esc quit from browse, want no-op")
		}
	}
	if m.State() != app.StateBrowse {
		t.Fatalf("after third Esc, State = %v, want StateBrowse", m.State())
	}

	// Only a further q exits with the fixed status (0 for clean rg 0).
	m, cmd = update(t, m, keyPress('q'))
	assertQuit(t, cmd)
	if m.ExitCode() != 0 {
		t.Fatalf("after q, ExitCode = %d, want 0", m.ExitCode())
	}
}

// TestErrorOverHelpQRestoresHelp verifies the full three-key sequence
// for the error-over-help row with q: q closes the error and restores
// help, the next q closes help to browsing, and only a further q exits.
func TestErrorOverHelpQRestoresHelp(t *testing.T) {
	m, loader, loadCh := setupPrecedenceBrowse(t)

	// Open help.
	m, _ = update(t, m, keyPress('h'))

	// Trigger a read failure → error overlay suspends help.
	loader.setFailing("src/a.go", "permission denied")
	loader.release("src/a.go")
	lc := <-loadCh
	m = deliverCompletion(t, m, lc)
	if m.OverlayKind() != app.OverlayError {
		t.Fatalf("OverlayKind = %v, want OverlayError", m.OverlayKind())
	}

	// First q: closes the error, restores help.
	m, _ = update(t, m, keyPress('q'))
	if m.OverlayKind() != app.OverlayHelp {
		t.Fatalf("after first q, OverlayKind = %v, want OverlayHelp", m.OverlayKind())
	}

	// Second q: closes help to browsing.
	m, cmd := update(t, m, keyPress('q'))
	if cmd != nil {
		t.Fatalf("second q produced a command: %v", cmd)
	}
	if m.OverlayOpen() {
		t.Fatalf("after second q, overlay still open")
	}
	if m.State() != app.StateBrowse {
		t.Fatalf("after second q, State = %v, want StateBrowse", m.State())
	}

	// Third q: exits with the fixed status (0 for clean rg 0).
	m, cmd = update(t, m, keyPress('q'))
	assertQuit(t, cmd)
	if m.ExitCode() != 0 {
		t.Fatalf("after third q, ExitCode = %d, want 0", m.ExitCode())
	}
}

// --- Esc no-op in no-results ---

// TestEscNoOpNoResults verifies that Esc with no overlay on the
// no-results screen does nothing.
func TestEscNoOpNoResults(t *testing.T) {
	m := setupHelpNoResults(t)
	m, cmd := update(t, m, keyPressOrEscape(tea.KeyEscape))
	if cmd != nil {
		t.Fatalf("Esc on no-results produced a command: %v", cmd)
	}
	if m.State() != app.StateNoResults {
		t.Fatalf("after Esc, State = %v, want StateNoResults", m.State())
	}
}
