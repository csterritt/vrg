package app

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/searchindex"
)

// errorOverHelp drives a model into the composed error-over-help
// state: the current file's load is held behind the load gate, help
// opens and scrolls scroll rows down, and the gate's release lands the
// file's read failure while help is open — suspending it under the new
// error overlay at its retained position. It returns the model and the
// suspended help's scroll position.
func errorOverHelp(t *testing.T, w, h, scroll int) (Model, int) {
	t.Helper()
	dir := t.TempDir()
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "hit a\n", 1, 0, 3, "hit"))
	idx.Finish()
	loader := &stubLoader{
		fail: map[string]bool{"a.txt": true},
		// Long enough to wrap past one interior page: the error overlay
		// itself is scrollable, so keys routed to it move observably.
		err: errors.New("denied: " + strings.Repeat("x", 300)),
		src: &stubSource{gutter: 3, widths: []int{8}},
	}
	m, gate, job := gatedLoaderBrowse(t, dir, idx, w, h, loader)

	m, _ = update(t, m, keyMsg("h"))
	if m.help == nil {
		t.Fatal("h did not open help over the held load")
	}
	for i := 0; i < scroll; i++ {
		m, _ = update(t, m, keyPress("down"))
	}
	if m.help == nil || m.help.scroll != scroll {
		t.Fatalf("setup: help scroll = %v/%d, want %d", m.help != nil, m.help.scroll, scroll)
	}

	close(gate)
	m, _ = update(t, m, collectMsg(t, job, "a.txt's held load"))
	if m.overlay == nil {
		t.Fatal("the current-file failure opened no error overlay over help")
	}
	if m.help == nil {
		t.Fatal("the error overlay discarded the suspended help")
	}
	if m.help.scroll != scroll {
		t.Fatalf("the error moved the suspended help to scroll %d, want %d", m.help.scroll, scroll)
	}
	return m, scroll
}

// A new error arriving while help is open suspends it: the error
// overlay takes the screen, and dismissing it with q or Esc alike
// restores help at its retained scroll position.
func TestErrorSuspendsHelpRestoringScroll(t *testing.T) {
	for _, key := range []string{"q", "esc"} {
		t.Run(key, func(t *testing.T) {
			m, s := errorOverHelp(t, 40, 8, 3)
			wantTop := strings.TrimRight(m.help.rows(38)[s], " ")

			v := m.View().Content
			if !strings.Contains(v, "cannot read a.txt: denied") {
				t.Fatalf("the error overlay does not render over help:\n%s", v)
			}
			if strings.Contains(v, "next / previous") {
				t.Fatalf("suspended help still renders over the error:\n%s", v)
			}

			m, cmd := update(t, m, keyPress(key))
			if cmd != nil {
				t.Fatalf("%s dismissing the error returned a command: %v", key, cmd)
			}
			if m.overlay != nil {
				t.Fatalf("%s did not dismiss the error overlay", key)
			}
			if m.help == nil {
				t.Fatalf("%s dropped the suspended help", key)
			}
			if m.help.scroll != s {
				t.Fatalf("%s restored help at scroll %d, want %d", key, m.help.scroll, s)
			}
			if got := m.View().Content; !strings.Contains(got, wantTop) {
				t.Fatalf("restored help lacks its scroll-%d top row %q:\n%s", s, wantTop, got)
			}
		})
	}
}

// While an error suspends help the error is the modal: scrolling moves
// the error's rows — never the suspended help's — help-toggle and base
// keys are ignored, and an appended failure keeps the reader's
// position. Dismissal then restores the untouched help.
func TestErrorOverHelpRoutesKeysToError(t *testing.T) {
	m, s := errorOverHelp(t, 40, 8, 3)
	if max := m.overlay.maxScroll(m.width, m.height); max < 1 {
		t.Fatalf("the error overlay is not scrollable at 40x8 (maxScroll %d)", max)
	}

	m, _ = update(t, m, keyPress("down"))
	if m.overlay.scroll != 1 {
		t.Fatalf("down moved the error scroll to %d, want 1", m.overlay.scroll)
	}
	if m.help.scroll != s {
		t.Fatalf("down under the error moved suspended help to %d, want %d", m.help.scroll, s)
	}
	for _, key := range []string{"h", "?", "n", "x"} {
		m2, cmd := update(t, m, keyPress(key))
		if cmd != nil {
			t.Fatalf("%s under the error overlay returned a command: %v", key, cmd)
		}
		m = m2
		if m.overlay == nil || m.help == nil {
			t.Fatalf("%s disturbed the error-over-help stack", key)
		}
		if m.help.scroll != s || m.overlay.scroll != 1 {
			t.Fatalf("%s moved scrolls to error %d help %d, want 1 and %d",
				key, m.overlay.scroll, m.help.scroll, s)
		}
	}
	if stop, _ := m.index.Current(); string(stop.Path) != "a.txt" || stop.Line != 1 {
		t.Fatalf("n reached the content behind the overlays: %+v", stop)
	}

	// A second failure appends without moving the reader: the error
	// position holds at 1 and the suspended help is untouched.
	m, req := beginLoad(m, "a.txt")
	m, _ = update(t, m, loadResult{
		path: []byte("a.txt"), req: req,
		err: errors.New("second failure"),
	})
	if len(m.overlay.lines) != 2 {
		t.Fatalf("the second failure left %d overlay lines, want 2", len(m.overlay.lines))
	}
	if m.overlay.scroll != 1 {
		t.Fatalf("the appended failure moved the reader to %d, want 1", m.overlay.scroll)
	}
	if m.help == nil || m.help.scroll != s {
		t.Fatal("the appended failure disturbed the suspended help")
	}

	m, _ = update(t, m, keyPress("esc"))
	if m.overlay != nil {
		t.Fatal("Esc did not dismiss the error")
	}
	if m.help == nil || m.help.scroll != s {
		t.Fatalf("Esc did not restore the suspended help at %d", s)
	}
}

// An appended diagnostic extends the open overlay's tail without
// moving the reader — the Issue 26 append-preserving-scroll primitive,
// proven there for reload re-entry failures, generalized to every
// appended error: a failure landing on the outcome overlay keeps the
// reader at position P and the new text stays reachable by scrolling.
func TestAppendedErrorPreservesScroll(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 40; i++ {
		fmt.Fprintf(&sb, "diag %02d\n", i)
	}
	m := overlayModel(t, validRecords(), nil, sb.String())
	if m.overlay == nil || m.state != stateBrowse {
		t.Fatalf("setup: state %d overlay %v, want browse under the warning overlay",
			m.state, m.overlay != nil)
	}
	for i := 0; i < 2; i++ {
		m, _ = update(t, m, keyPress("down"))
	}
	if m.overlay.scroll != 2 {
		t.Fatalf("setup: overlay scroll = %d, want 2", m.overlay.scroll)
	}

	// The current file's in-flight startup load fails: the diagnostic
	// appends to the open overlay and the reader stays at 2.
	req, ok := m.loading["a.txt"]
	if !ok {
		t.Fatal("a.txt's startup load is not in flight")
	}
	m, _ = update(t, m, loadResult{
		path: []byte("a.txt"), req: req,
		err: errors.New("appended failure tail"),
	})
	if got := len(m.overlay.lines); got != 41 {
		t.Fatalf("overlay lines = %d, want the 40 diagnostics plus the appended failure", got)
	}
	if m.overlay.scroll != 2 {
		t.Fatalf("the appended error moved the reader: scroll = %d, want 2", m.overlay.scroll)
	}

	// The new text is reachable: scrolling to the bottom shows it.
	for i := 0; i < 100; i++ {
		m, _ = update(t, m, keyPress("down"))
	}
	if v := m.View().Content; !strings.Contains(v, "cannot read a.txt: appended failure tail") {
		t.Fatalf("scrolled-to-bottom overlay lacks the appended diagnostic:\n%s", v)
	}
}

// Opening help or an error cancels the file-change pop-up outright:
// nothing is suspended, the cancelled instance's own expiry cannot
// revive it, and dismissing the overlay does not bring it back.
func TestOverlayOpenCancelsPopupPermanently(t *testing.T) {
	m := twoFileBrowse(t, 80, 24)
	m, _ = update(t, m, keyMsg("n")) // → b.txt pop-up, load in flight
	if m.popup == nil {
		t.Fatal("cross-file n opened no pop-up")
	}
	id := m.popup.id

	m, _ = update(t, m, keyMsg("?"))
	if m.help == nil || m.popup != nil {
		t.Fatal("? did not open help and cancel the pop-up")
	}
	// The cancelled instance's own expiry cannot revive it, and closing
	// help does not bring it back.
	m, _ = update(t, m, popupExpiredMsg{id: id})
	m, _ = update(t, m, keyPress("esc"))
	if m.popup != nil {
		t.Fatal("the cancelled pop-up returned after help closed")
	}

	// The error overlay cancels a pop-up the same way: hop away and
	// back onto b.txt — its load is still in flight — so a fresh
	// pop-up is up when the failure lands current.
	m, _ = update(t, m, keyMsg("p")) // → a.txt pop-up
	m, _ = update(t, m, keyMsg("n")) // → b.txt pop-up again
	if m.popup == nil {
		t.Fatal("the return to b.txt opened no pop-up")
	}
	id = m.popup.id
	m, _ = update(t, m, loadResult{
		path: []byte("b.txt"), req: m.loading["b.txt"],
		err: errors.New("denied"),
	})
	if m.overlay == nil || m.popup != nil {
		t.Fatal("the error did not open and cancel the pop-up")
	}
	m, _ = update(t, m, popupExpiredMsg{id: id})
	m, _ = update(t, m, keyPress("q"))
	if m.overlay != nil {
		t.Fatal("q did not dismiss the error overlay")
	}
	if m.popup != nil {
		t.Fatal("the cancelled pop-up returned after the error closed")
	}
}

// Esc is an overlay-dismissal key only: with nothing open it changes
// nothing — no command, no state change, an identical frame — in
// browsing and on the no-results screen alike. Its one base-state
// effect is dismissing a pop-up like any other key.
func TestEscInBaseStatesIsNoop(t *testing.T) {
	t.Run("browse", func(t *testing.T) {
		m := twoFileBrowse(t, 80, 24)
		before := m.View().Content
		m, cmd := update(t, m, keyPress("esc"))
		if cmd != nil {
			t.Fatalf("Esc in browse returned a command: %v", cmd)
		}
		if m.state != stateBrowse || m.status != 0 {
			t.Fatalf("Esc in browse changed state %d status %d", m.state, m.status)
		}
		if got := m.View().Content; got != before {
			t.Fatalf("Esc in browse changed the view:\nbefore:\n%s\nafter:\n%s", before, got)
		}
	})
	t.Run("no-results", func(t *testing.T) {
		m := noResultsModel(t)
		before := m.View().Content
		m, cmd := update(t, m, keyPress("esc"))
		if cmd != nil {
			t.Fatalf("Esc on no-results returned a command: %v", cmd)
		}
		if m.state != stateNoResults || m.status != 1 {
			t.Fatalf("Esc on no-results changed state %d status %d", m.state, m.status)
		}
		if got := m.View().Content; got != before {
			t.Fatalf("Esc on no-results changed the view:\nbefore:\n%s\nafter:\n%s", before, got)
		}
	})
	t.Run("pop-up dismissal only", func(t *testing.T) {
		m := twoFileBrowse(t, 80, 24)
		m, _ = update(t, m, keyMsg("n"))
		if m.popup == nil {
			t.Fatal("cross-file n opened no pop-up")
		}
		m, cmd := update(t, m, keyPress("esc"))
		if cmd != nil {
			t.Fatalf("Esc under a pop-up returned a command: %v", cmd)
		}
		if m.popup != nil {
			t.Fatal("Esc did not dismiss the pop-up")
		}
		if m.state != stateBrowse || m.status != 0 {
			t.Fatalf("Esc under a pop-up changed state %d status %d", m.state, m.status)
		}
		if stop, _ := m.index.Current(); string(stop.Path) != "b.txt" {
			t.Fatalf("Esc acted on the content behind the pop-up: %+v", stop)
		}
	})
}

// recordLossModel drives a model to the fatal record-loss outcome: a
// complete stream that retained no usable results after a malformed
// record was skipped — an error overlay with no underlying state.
func recordLossModel(t *testing.T) Model {
	t.Helper()
	b := searchindex.NewBuilder("/w")
	b.Consume(strings.NewReader("{not json\n" + summaryRec() + "\n"))
	idx, integrity := b.Finish()
	m := New(&fakeChild{stdout: strings.NewReader(""), stderr: strings.NewReader("")}, "/w")
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = update(t, m, searchResult{
		index: idx, integrity: integrity, report: b.Report(), err: exitError(1),
	})
	if m.state != stateFatal || m.overlay == nil {
		t.Fatalf("setup: state %d overlay %v, want the fatal record-loss overlay",
			m.state, m.overlay != nil)
	}
	return m
}

// dismissalRow is one row of the Issue 32 dismissal-outcome table: a
// model driven to its overlay state, whether the dismissal key itself
// exits, and the still-running expectations — the base state dismissal
// returns to, whether it restores suspended help, the text the returned
// base state shows, and the fixed status the final base-state q yields.
type dismissalRow struct {
	name         string
	setup        func(t *testing.T) Model
	quits        bool
	wantState    state
	restoresHelp bool
	wantView     string
	wantStatus   int
}

// TestDismissalOutcomeTable runs the dismissal-outcome table for both
// dismissal keys: overlays over a live base state return to it still
// running — error-over-help restoring help at its retained position —
// while the stateless fatal overlays exit 2 on either key. Still-running
// rows then assert state-specifically: a second Esc is a no-op and a
// base-state q exits at the fixed status — never a uniform second-q
// exit, because over restored help q only closes help.
func TestDismissalOutcomeTable(t *testing.T) {
	cases := []dismissalRow{
		{
			name: "browse with error overlay",
			setup: func(t *testing.T) Model {
				m := overlayModel(t, validRecords(), exitError(2), "boom\n")
				if m.overlay == nil || m.state != stateBrowse {
					t.Fatalf("setup: state %d overlay %v, want browse under the error overlay",
						m.state, m.overlay != nil)
				}
				return m
			},
			wantState:  stateBrowse,
			wantView:   "a.txt",
			wantStatus: 2,
		},
		{
			name: "browse with help",
			setup: func(t *testing.T) Model {
				return helpModel(t, "h")
			},
			wantState:  stateBrowse,
			wantView:   "a.txt",
			wantStatus: 0,
		},
		{
			name: "browse with error over help",
			setup: func(t *testing.T) Model {
				m, _ := errorOverHelp(t, 40, 8, 3)
				return m
			},
			wantState:    stateBrowse,
			restoresHelp: true,
			wantView:     "a.txt",
			wantStatus:   0,
		},
		{
			name: "empty result with warning overlay",
			setup: func(t *testing.T) Model {
				m := overlayModel(t, []string{summaryRec()}, exitError(1), "warn\n")
				if m.overlay == nil || m.state != stateNoResults {
					t.Fatalf("setup: state %d overlay %v, want no-results under the warning overlay",
						m.state, m.overlay != nil)
				}
				return m
			},
			wantState:  stateNoResults,
			wantView:   "No results found",
			wantStatus: 1,
		},
		{
			name: "no-results with help",
			setup: func(t *testing.T) Model {
				m := noResultsModel(t)
				m, _ = update(t, m, keyMsg("h"))
				if m.help == nil {
					t.Fatal("h did not open help over no-results")
				}
				return m
			},
			wantState:  stateNoResults,
			wantView:   "No results found",
			wantStatus: 1,
		},
		{
			name: "fatal with no usable results",
			setup: func(t *testing.T) Model {
				m := overlayModel(t, []string{summaryRec()}, exitError(2), "boom\n")
				if m.overlay == nil || m.state != stateFatal {
					t.Fatalf("setup: state %d overlay %v, want the fatal overlay",
						m.state, m.overlay != nil)
				}
				return m
			},
			quits:      true,
			wantStatus: 2,
		},
		{
			name:       "record-loss with no results",
			setup:      recordLossModel,
			quits:      true,
			wantStatus: 2,
		},
	}

	for _, tc := range cases {
		for _, key := range []string{"q", "esc"} {
			t.Run(tc.name+" / "+key, func(t *testing.T) {
				m := tc.setup(t)
				wantScroll := 0
				if tc.restoresHelp {
					if m.help == nil {
						t.Fatal("setup lost the suspended help")
					}
					wantScroll = m.help.scroll
				}

				m, cmd := update(t, m, keyPress(key))
				if tc.quits {
					// There is no underlying state to return to:
					// dismissal is the exit, at the fixed status — Esc
					// included.
					requireQuit(t, cmd, key+" dismissing the stateless overlay")
					if m.status != tc.wantStatus {
						t.Fatalf("exit status = %d, want %d", m.status, tc.wantStatus)
					}
					return
				}
				if cmd != nil {
					t.Fatalf("%s dismissal returned a command: %v", key, cmd)
				}
				if m.overlay != nil {
					t.Fatalf("%s left the error overlay open", key)
				}
				if tc.restoresHelp {
					// The dismissal closed the error and restored help
					// at its suspended position; the next q or Esc
					// closes help to the base state — a second q does
					// not exit here.
					if m.help == nil || m.help.scroll != wantScroll {
						t.Fatalf("%s did not restore help at scroll %d", key, wantScroll)
					}
					m, cmd = update(t, m, keyPress(key))
					if cmd != nil {
						t.Fatalf("second %s closing restored help returned a command: %v", key, cmd)
					}
				}
				if m.help != nil {
					t.Fatalf("%s left help open", key)
				}
				if m.state != tc.wantState {
					t.Fatalf("post-dismissal state = %d, want %d", m.state, tc.wantState)
				}
				if got := m.View().Content; !strings.Contains(got, tc.wantView) {
					t.Fatalf("post-dismissal view lacks %q:\n%s", tc.wantView, got)
				}

				// From the still-running base state a second Esc is a
				// no-op and q exits at the fixed status.
				m, cmd = update(t, m, keyPress("esc"))
				if cmd != nil {
					t.Fatalf("Esc in the base state returned a command: %v", cmd)
				}
				if m.state != tc.wantState {
					t.Fatalf("Esc changed the base state to %d", m.state)
				}
				m, cmd = update(t, m, keyPress("q"))
				requireQuit(t, cmd, "q in the base state")
				if m.status != tc.wantStatus {
					t.Fatalf("exit status = %d, want %d", m.status, tc.wantStatus)
				}
			})
		}
	}
}
