package app

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/searchindex"
	"vrg/internal/theme"
)

// suspendedHelpScroll is the help scroll position the suspension
// fixture scrolls to before the error lands — the position every
// restoration assertion checks.
const suspendedHelpScroll = 5

// suspendRig bundles the suspension fixture: the model browsing
// b.txt with b's load request held in flight and help open scrolled
// to suspendedHelpScroll — an error released against that request
// must suspend help, never close it (Issue #32).
type suspendRig struct {
	m    *model
	idx  *searchindex.Index
	keyB string
}

func newSuspendRig(t *testing.T) *suspendRig {
	t.Helper()
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{
		popupTimer: popupStubTicks.popupTimer,
	})
	idx := navIndex(t, isoFiles)
	finishLoad(t, m, startBrowse(t, m, idx))
	// Crossing into b.txt mints its load request; the returned
	// command stays unrun — the held load whose settlement the test
	// releases while help is open.
	if _, nav := m.Update(keyN); nav == nil {
		t.Fatal("crossing into b.txt minted no load command")
	}
	if reqOf(m, idx.Files[1].Path) == 0 {
		t.Fatal("crossing into b.txt minted no in-flight load request")
	}
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 8})
	m.Update(keyH)
	if !m.helpOpen {
		t.Fatal("h did not open help")
	}
	for i := 0; i < suspendedHelpScroll; i++ {
		m.Update(keyDown)
	}
	if m.help.scroll != suspendedHelpScroll {
		t.Fatalf("help scroll = %d, want %d — help must be scrollable",
			m.help.scroll, suspendedHelpScroll)
	}
	return &suspendRig{m: m, idx: idx, keyB: string(idx.Files[1].Path)}
}

// fail releases the held load as a failure with err while help is
// open — the current-file failure opens the error overlay over the
// suspended help.
func (r *suspendRig) fail(t *testing.T, err error) {
	t.Helper()
	r.m.Update(fileLoadedMsg{
		path: r.idx.Files[1].Path,
		req:  reqOf(r.m, r.idx.Files[1].Path),
		err:  err,
	})
	if !r.m.overlayOpen {
		t.Fatal("the current-file failure did not open the error overlay")
	}
}

// dismissKey asserts the key press returns no command and leaves the
// model running — the shared still-running dismissal contract.
func dismissKey(t *testing.T, m *model, k tea.KeyPressMsg, label string) {
	t.Helper()
	if _, cmd := m.Update(k); cmd != nil {
		t.Fatalf("%s dismissal returned a command, want none", label)
	}
	if m.quitting {
		t.Fatalf("%s dismissal began an exit; the underlying state is still running", label)
	}
}

// followBase runs the still-running base-state follow-ups: a second
// Esc leaves the model running — Esc never exits a base state — and a
// second q exits through the cleanup path with the fixed status.
func followBase(t *testing.T, m *model, status int) {
	t.Helper()
	if _, cmd := m.Update(keyEsc); cmd != nil || m.quitting {
		t.Fatalf("base-state Esc: cmd=%v quitting=%v — Esc never exits a base state",
			cmd != nil, m.quitting)
	}
	_, cmd := m.Update(keyQ)
	if !m.quitting {
		t.Fatal("base-state q did not begin the controlled exit")
	}
	runQuittingCmd(t, cmd)
	if m.status != status {
		t.Fatalf("status = %d, want the fixed %d", m.status, status)
	}
}

// A new error while help is open suspends help at its scroll
// position: either dismissal key — q or Esc — closes only the error
// and restores help at the same position.
func TestErrorSuspendsHelpRestoringScroll(t *testing.T) {
	for _, tc := range []struct {
		name string
		key  tea.KeyPressMsg
	}{{"esc", keyEsc}, {"q", keyQ}} {
		t.Run(tc.name, func(t *testing.T) {
			rig := newSuspendRig(t)
			rig.fail(t, errUnreadable)
			m := rig.m

			if !m.helpOpen {
				t.Fatal("the error closed help instead of suspending it")
			}
			if m.help.scroll != suspendedHelpScroll {
				t.Fatalf("suspension lost the help position: scroll = %d, want %d",
					m.help.scroll, suspendedHelpScroll)
			}
			if m.overlay.scroll != 0 {
				t.Fatalf("the new error opened at scroll %d, want the top", m.overlay.scroll)
			}
			if v := viewText(m); !strings.Contains(v, "cannot read") {
				t.Fatalf("view = %q, want the load diagnostic in the overlay", v)
			}

			// The dismissal closes only the error: help is restored
			// at its retained position — row 5 tops the box and the
			// title row has scrolled off.
			dismissKey(t, m, tc.key, tc.name)
			if m.overlayOpen {
				t.Fatalf("%s did not dismiss the error", tc.name)
			}
			if !m.helpOpen || m.help.scroll != suspendedHelpScroll {
				t.Fatalf("%s: help restored open=%v scroll=%d, want open at %d",
					tc.name, m.helpOpen, m.help.scroll, suspendedHelpScroll)
			}
			v := viewText(m)
			if strings.Contains(v, "cannot read") {
				t.Fatalf("%s: the dismissed error still renders: %q", tc.name, v)
			}
			if strings.Contains(v, "Key bindings") {
				t.Fatalf("%s: help restored at the top, not scroll %d: %q",
					tc.name, suspendedHelpScroll, v)
			}
			row := strings.TrimSpace(m.helpRows()[suspendedHelpScroll])
			if !strings.Contains(v, row) {
				t.Fatalf("%s: view = %q, want help row %d %q at the box top",
					tc.name, v, suspendedHelpScroll, row)
			}
		})
	}
}

// A second error appended to an overlay the reader has scrolled to
// position P keeps the reader at P with the new text reachable — the
// Issue #26 append-preserving-scroll primitive generalized to every
// appended error, here a load failure landing on the search-failure
// overlay rather than on a reload re-entry overlay.
func TestAppendedErrorPreservesReaderPosition(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 30; i++ {
		fmt.Fprintf(&sb, "diag-%03d\n", i)
	}
	m := newTestModel(
		fakeChild{res: Result{Stdout: []byte(happyStream), Stderr: []byte(sb.String()), Code: 3}},
		options{loader: failAllLoader})
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	_, loadCmd := m.Update(runCollectCmd(t, m))
	if !m.overlayOpen {
		t.Fatal("the fatal search did not open the error overlay")
	}

	for i := 0; i < 3; i++ {
		m.Update(keyDown)
	}
	if m.overlay.scroll != 3 {
		t.Fatalf("overlay scroll = %d, want 3 — the diagnostic must be scrollable",
			m.overlay.scroll)
	}

	// The current file's load fails: the second error appends to the
	// open overlay without moving the reader.
	msg, ok := fileLoadOf(loadCmd)
	if !ok {
		t.Fatal("the browse transition's load command produced no completion")
	}
	m.Update(msg)
	if m.overlay.scroll != 3 {
		t.Fatalf("append moved the reader to scroll %d, want the preserved 3",
			m.overlay.scroll)
	}
	if !strings.Contains(m.overlay.text, "cannot read") ||
		!strings.Contains(m.overlay.text, "diag-029") {
		t.Fatalf("overlay = %q, want the diagnostic plus the appended load failure",
			m.overlay.text)
	}

	// The appended text is reachable: scrolling to the bottom lands on it.
	maxScroll := len(m.overlayRows()) - m.overlayVisible()
	for m.overlay.scroll < maxScroll {
		m.Update(keyDown)
	}
	if v := viewText(m); !strings.Contains(v, "cannot read") {
		t.Fatalf("scrolled-to-bottom view = %q, want the appended failure reachable", v)
	}
	if m.quitting || m.state != stateBrowse || m.helpOpen {
		t.Fatalf("append disturbed the model: quitting=%v state=%v helpOpen=%v",
			m.quitting, m.state, m.helpOpen)
	}
}

// An error arriving while a file-change pop-up is up cancels it —
// through the real load-failure message path, not a manual overlay
// open — and no suspended pop-up returns after the error is
// dismissed.
func TestErrorArrivalCancelsPopup(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, popupStubTicks)
	idx := navIndex(t, isoFiles)
	finishLoad(t, m, startBrowse(t, m, idx))
	if _, nav := m.Update(keyN); nav == nil {
		t.Fatal("crossing into b.txt minted no load command")
	}
	wantPopup(t, m, escapedPath(idx.Files[1]))

	m.Update(fileLoadedMsg{
		path: idx.Files[1].Path,
		req:  reqOf(m, idx.Files[1].Path),
		err:  errUnreadable,
	})
	if !m.overlayOpen {
		t.Fatal("the current-file failure did not open the error overlay")
	}
	if m.popupID != 0 {
		t.Fatal("the arriving error did not cancel the pop-up")
	}

	m.Update(keyEsc)
	if m.overlayOpen {
		t.Fatal("Esc did not dismiss the error")
	}
	if m.popupID != 0 {
		t.Fatal("a suspended pop-up returned after the error closed")
	}
}

// With neither help nor error open, Esc does nothing in the base
// states: browsing and the no-results screen keep running with the
// frame untouched — Esc never exits a base state. Its only effect is
// dismissing a pop-up, like any other key.
func TestEscWithNoOverlayIsNoOp(t *testing.T) {
	m := helpBrowseModel(t)
	before := viewText(m)
	if _, cmd := m.Update(keyEsc); cmd != nil {
		t.Fatal("Esc in browse returned a command")
	}
	if m.quitting || m.state != stateBrowse {
		t.Fatalf("Esc in browse: quitting=%v state=%v", m.quitting, m.state)
	}
	if got := viewText(m); got != before {
		t.Fatal("Esc in browse changed the frame")
	}

	nr := newTestModel(fakeChild{res: Result{Stdout: []byte(emptyStream), Code: 1}}, options{})
	nr.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	nr.Update(runCollectCmd(t, nr))
	if nr.state != stateNoResults {
		t.Fatalf("state = %v, want no-results", nr.state)
	}
	before = viewText(nr)
	if _, cmd := nr.Update(keyEsc); cmd != nil {
		t.Fatal("Esc on no-results returned a command")
	}
	if nr.quitting || nr.state != stateNoResults {
		t.Fatalf("Esc on no-results: quitting=%v state=%v", nr.quitting, nr.state)
	}
	if got := viewText(nr); got != before {
		t.Fatal("Esc on no-results changed the frame")
	}
}

// With the error suspended over help, every key routes to the error:
// up/down scroll it while help stays at its retained position, and
// every other key — h, ?, r, the base keys included — is ignored:
// help neither scrolls nor closes, the state behind is untouched, and
// no work issues. A second error appended keeps the reader's position
// with help still suspended; dismissal closes only the error and
// restores help.
func TestErrorOverHelpRoutesKeysToError(t *testing.T) {
	rig := newSuspendRig(t)
	longErr := errors.New("denied: " + strings.Repeat("again ", 120))
	rig.fail(t, longErr)
	m := rig.m
	cur0, _ := m.idx.Cursor()

	// up/down move the error, not the suspended help.
	m.Update(keyDown)
	if m.overlay.scroll != 1 || m.help.scroll != suspendedHelpScroll {
		t.Fatalf("down under error-over-help: error %d help %d, want error 1 help %d",
			m.overlay.scroll, m.help.scroll, suspendedHelpScroll)
	}
	m.Update(keyUp)
	if m.overlay.scroll != 0 || m.help.scroll != suspendedHelpScroll {
		t.Fatalf("up under error-over-help: error %d help %d, want error 0 help %d",
			m.overlay.scroll, m.help.scroll, suspendedHelpScroll)
	}

	for _, k := range []tea.KeyPressMsg{
		keyN, keyP, keyW, keyC, keyR, keyH, keyQuestion,
		keyU, keyD, keyPgUp, keyPgDn,
		keyLeft, keyRight, keyTab, keyShiftTab,
		{Text: ",", Code: ','}, {Text: ".", Code: '.'},
		{Text: "<", Code: '<'}, {Text: ">", Code: '>'},
		{Text: "[", Code: '['}, {Text: "]", Code: ']'},
		{Code: tea.KeyEnter}, {Code: tea.KeyHome}, {Code: tea.KeyEnd},
		{Text: "x", Code: 'x'},
	} {
		if _, cmd := m.Update(k); cmd != nil {
			t.Fatalf("key %v under error-over-help returned a command", k)
		}
		if !m.overlayOpen || !m.helpOpen || m.help.scroll != suspendedHelpScroll ||
			m.overlay.scroll != 0 || m.quitting {
			t.Fatalf("key %v disturbed the suspended stack: overlay=%v help=%v "+
				"hscroll=%d oscroll=%d quitting=%v",
				k, m.overlayOpen, m.helpOpen, m.help.scroll, m.overlay.scroll, m.quitting)
		}
	}
	if cur, _ := m.idx.Cursor(); cur != cur0 {
		t.Fatalf("a swept key moved the cursor to %v under the error", cur)
	}
	if len(m.loading) != 0 {
		t.Fatalf("r minted a load under error-over-help: %v", m.loading)
	}
	if m.theme != theme.Dark() {
		t.Fatal("c toggled the theme under error-over-help")
	}
	if !m.wrap {
		t.Fatal("w toggled wrap under error-over-help")
	}
	if !m.listVisible {
		t.Fatal("a list key toggled the file list under error-over-help")
	}

	// A second error appended over the suspended help keeps the
	// reader's position and leaves help suspended beneath.
	m.scrollOverlay(2)
	req := mintRequest(m, m.idx.Files[1].Path)
	m.Update(fileLoadedMsg{path: m.idx.Files[1].Path, req: req, err: errUnreadable})
	if m.overlay.scroll != 2 {
		t.Fatalf("append over suspended help moved the reader to %d, want 2",
			m.overlay.scroll)
	}
	if n := strings.Count(m.overlay.text, "cannot read"); n != 2 {
		t.Fatalf("overlay occurrences = %d, want the second error appended once: %q",
			n, m.overlay.text)
	}
	if !m.helpOpen || m.help.scroll != suspendedHelpScroll {
		t.Fatalf("append disturbed suspended help: open=%v scroll=%d",
			m.helpOpen, m.help.scroll)
	}

	// The dismissal closes only the error: help returns at 5.
	m.Update(keyEsc)
	if m.overlayOpen || !m.helpOpen || m.help.scroll != suspendedHelpScroll {
		t.Fatalf("dismissal: overlay=%v help=%v scroll=%d, want error closed, help at %d",
			m.overlayOpen, m.helpOpen, m.help.scroll, suspendedHelpScroll)
	}
}

// ctrl+c tops the precedence stack: over the error suspended over
// help it is still the global override — cleanup and exit 130.
func TestCtrlCOverErrorOverHelpExits130(t *testing.T) {
	rig := newSuspendRig(t)
	rig.fail(t, errUnreadable)
	_, cmd := rig.m.Update(keyCtrlC)
	if !rig.m.quitting {
		t.Fatal("ctrl+c over error-over-help did not begin a controlled exit")
	}
	runQuittingCmd(t, cmd)
	if rig.m.status != 130 {
		t.Fatalf("status = %d, want ctrl+c's 130 override", rig.m.status)
	}
}

// The dismissal-outcome table, run for both q and Esc as the
// dismissal key: browse overlays close to browsing, a warning over an
// empty result closes to the no-results screen, error-over-help
// restores help at its scroll position, and the fatal no-results
// overlays exit 2 — Esc's only terminating dismissal, because there
// is no underlying state. Still-running rows take the state-specific
// follow-ups: a base Esc is a no-op and a base q exits the fixed
// status — never a uniform second-q-exits assertion.
func TestDismissalOutcomeTable(t *testing.T) {
	for _, tc := range []struct {
		name string
		key  tea.KeyPressMsg
	}{{"q", keyQ}, {"esc", keyEsc}} {

		// Browse with an error overlay → browsing still running; a
		// base q exits the fixed fatal-search status 2.
		t.Run(tc.name+": browse error overlay to browsing", func(t *testing.T) {
			m := overlayModel(t, "boom\n")
			dismissKey(t, m, tc.key, tc.name)
			if m.overlayOpen || m.state != stateBrowse {
				t.Fatalf("%s: overlay=%v state=%v, want dismissed to browsing",
					tc.name, m.overlayOpen, m.state)
			}
			followBase(t, m, 2)
		})

		// Browse with help → browsing still running; a base q exits
		// the fixed status 0.
		t.Run(tc.name+": browse help to browsing", func(t *testing.T) {
			m := helpBrowseModel(t)
			m.Update(keyH)
			if !m.helpOpen {
				t.Fatal("h did not open help")
			}
			dismissKey(t, m, tc.key, tc.name)
			if m.helpOpen || m.state != stateBrowse {
				t.Fatalf("%s: help=%v state=%v, want closed to browsing",
					tc.name, m.helpOpen, m.state)
			}
			followBase(t, m, 0)
		})

		// Browse with error-over-help → the full three-key sequence:
		// the dismissal closes the error and restores help at its
		// scroll position, the next dismissal key closes the
		// restored help to browsing — still running — and only a
		// further base-state q exits.
		t.Run(tc.name+": browse error-over-help restores help", func(t *testing.T) {
			rig := newSuspendRig(t)
			rig.fail(t, errUnreadable)
			m := rig.m

			dismissKey(t, m, tc.key, tc.name)
			if m.overlayOpen || !m.helpOpen || m.help.scroll != suspendedHelpScroll {
				t.Fatalf("%s: overlay=%v help=%v scroll=%d, want error closed, help at %d",
					tc.name, m.overlayOpen, m.helpOpen, m.help.scroll, suspendedHelpScroll)
			}
			dismissKey(t, m, tc.key, tc.name)
			if m.helpOpen || m.state != stateBrowse {
				t.Fatalf("%s: help=%v state=%v, want help closed to browsing",
					tc.name, m.helpOpen, m.state)
			}
			followBase(t, m, 0)
		})

		// An empty result with a warning overlay → the no-results
		// screen still running; a base q exits 1.
		t.Run(tc.name+": empty result warning to no-results", func(t *testing.T) {
			m := newTestModel(fakeChild{res: Result{
				Stdout: []byte(emptyStream), Stderr: []byte("warn\n"), Code: 1,
			}}, options{})
			m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
			m.Update(runCollectCmd(t, m))
			if m.state != stateNoResults || !m.overlayOpen {
				t.Fatalf("state=%v overlay=%v, want the warning overlay over no-results",
					m.state, m.overlayOpen)
			}
			dismissKey(t, m, tc.key, tc.name)
			if m.overlayOpen || m.state != stateNoResults {
				t.Fatalf("%s: overlay=%v state=%v, want dismissed to no-results",
					tc.name, m.overlayOpen, m.state)
			}
			if v := viewText(m); !strings.Contains(v, "No results found") {
				t.Fatalf("%s: view = %q, want the no-results screen", tc.name, v)
			}
			followBase(t, m, 1)
		})

		// No-results with help open → closing to the no-results
		// screen still running; a base q exits 1.
		t.Run(tc.name+": no-results help to no-results", func(t *testing.T) {
			m := newTestModel(fakeChild{res: Result{
				Stdout: []byte(emptyStream), Code: 1,
			}}, options{})
			m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
			m.Update(runCollectCmd(t, m))
			if m.state != stateNoResults || m.overlayOpen {
				t.Fatalf("state=%v overlay=%v, want bare no-results",
					m.state, m.overlayOpen)
			}
			m.Update(keyH)
			if !m.helpOpen {
				t.Fatal("h did not open help over no-results")
			}
			dismissKey(t, m, tc.key, tc.name)
			if m.helpOpen || m.state != stateNoResults {
				t.Fatalf("%s: help=%v state=%v, want closed to no-results",
					tc.name, m.helpOpen, m.state)
			}
			if v := viewText(m); !strings.Contains(v, "No results found") {
				t.Fatalf("%s: view = %q, want the no-results screen", tc.name, v)
			}
			followBase(t, m, 1)
		})

		// A fatal overlay with no usable results → dismissal exits 2
		// — there is no underlying state, so Esc terminates here and
		// only here.
		t.Run(tc.name+": fatal no usable results exits 2", func(t *testing.T) {
			m := newTestModel(fakeChild{res: Result{
				Stdout: []byte(emptyStream), Stderr: []byte("boom\n"), Code: 3,
			}}, options{})
			m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
			m.Update(runCollectCmd(t, m))
			if m.state != stateOverlayOnly || !m.overlayOpen {
				t.Fatalf("state=%v overlay=%v, want the fatal overlay",
					m.state, m.overlayOpen)
			}
			_, cmd := m.Update(tc.key)
			if !m.quitting {
				t.Fatalf("%s did not exit the fatal overlay", tc.name)
			}
			runQuittingCmd(t, cmd)
			if m.status != 2 {
				t.Fatalf("status = %d, want 2", m.status)
			}
		})

		// Record loss with no results → dismissal exits 2 the same
		// way: the record-loss overlay has no underlying state.
		t.Run(tc.name+": record-loss no results exits 2", func(t *testing.T) {
			m := newTestModel(fakeChild{res: Result{
				Stdout: []byte(malformedOnlyStream), Code: 0,
			}}, options{})
			m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
			m.Update(runCollectCmd(t, m))
			if m.state != stateOverlayOnly || !m.overlayOpen {
				t.Fatalf("state=%v overlay=%v, want the record-loss overlay",
					m.state, m.overlayOpen)
			}
			_, cmd := m.Update(tc.key)
			if !m.quitting {
				t.Fatalf("%s did not exit the record-loss overlay", tc.name)
			}
			runQuittingCmd(t, cmd)
			if m.status != 2 {
				t.Fatalf("status = %d, want 2", m.status)
			}
		})
	}
}
