package app

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"vrg/internal/filebuffer"
)

// tallFailure is a multi-line read failure: the "cannot read" overlay
// it opens is scrollable at the test frame sizes, so the reader can sit
// mid-overlay when more diagnostics arrive.
func tallFailure() error {
	var b strings.Builder
	b.WriteString("denied")
	for i := 0; i < 40; i++ {
		fmt.Fprintf(&b, "\ncontext %02d", i)
	}
	return errors.New(b.String())
}

// A new error while help is open suspends it: the modal error owns the
// keyboard — scrolling moves the error's rows, not the help beneath,
// and the help's own close keys are swallowed — and either dismissal
// key restores help at the scroll position it held when the error
// arrived. The load gate parks the current file's worker so the
// failure lands deterministically while help is open.
func TestErrorSuspendsHelpRestoringScroll(t *testing.T) {
	for _, key := range []string{"q", "esc"} {
		t.Run(key, func(t *testing.T) {
			dir := t.TempDir()
			recs := append(fileWithStops(t, dir, "a.txt", 10, 1), recSummary)
			gate := make(chan struct{})
			m, load := gatedLoaderModel(t, dir, gate, failLoaderFor("a.txt", tallFailure()), recs...)
			// a.txt's worker is in flight, parked on the gate.
			done := make(chan tea.Msg, 1)
			go func() { done <- load() }()

			// Help opens at a small height so the binding list scrolls.
			m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 10})
			m, _ = pressKey(t, m, "?")
			if m.help == nil {
				t.Fatal("? did not open help")
			}
			for i := 0; i < 5; i++ {
				m, _ = pressKey(t, m, "down")
			}
			if m.help.scroll != 5 {
				t.Fatalf("help scroll = %d, want 5", m.help.scroll)
			}

			// Releasing the gate lands the failure while help is open:
			// the error overlay takes over with help suspended beneath.
			close(gate)
			m = pump(t, m, <-done)
			if m.overlay == nil {
				t.Fatal("the gated failure did not open the error overlay")
			}
			if m.help == nil || m.help.scroll != 5 {
				t.Fatalf("help not suspended at position 5: help=%v", m.help)
			}
			if v := m.View().Content; !strings.Contains(v, "cannot read a.txt") {
				t.Fatalf("frame lacks the error overlay over help: %q", v)
			}

			// While both are open the error owns the keyboard: down
			// scrolls the error's rows — the help offset does not move —
			// and the help's own keys plus the browse keys are ignored.
			m, _ = pressKey(t, m, "down")
			if m.overlay == nil || m.overlay.scroll != 1 || m.help.scroll != 5 {
				t.Fatalf("down under error-over-help: overlay=%v help=%v, want error scrolled to 1, help held at 5",
					m.overlay, m.help)
			}
			for _, k := range []string{"h", "n", "r", "w", "c"} {
				m2, cmd := pressKey(t, m, k)
				if cmd != nil || m2.overlay == nil || m2.help == nil || m2.quit {
					t.Fatalf("%q under the error overlay changed state: overlay=%v help=%v quit=%v cmd=%T",
						k, m2.overlay != nil, m2.help != nil, m2.quit, cmd)
				}
				m = m2
			}

			// Dismissal restores the suspended help at its position,
			// still running over browse.
			m, cmd := pressKey(t, m, key)
			if cmd != nil {
				t.Fatalf("%s dismissal produced a command %T, want none", key, cmd)
			}
			if m.overlay != nil {
				t.Fatalf("%s did not dismiss the error overlay", key)
			}
			if m.help == nil || m.help.scroll != 5 {
				t.Fatalf("%s did not restore help at 5: help=%v", key, m.help)
			}
			if m.phase != phaseBrowse || m.quit {
				t.Fatalf("dismissal changed the base state: phase=%d quit=%v", m.phase, m.quit)
			}
			v := m.View().Content
			if !strings.Contains(v, "pan half the text width") || strings.Contains(v, "cannot read") {
				t.Fatalf("restored frame = %q, want scrolled help without the error", v)
			}
		})
	}
}

// A second error appended to the open overlay leaves the reader's
// scroll position untouched with the new text reachable below — the
// Issue #26 append-preserving-scroll primitive generalized to every
// appended error, not just reload re-entry failures. Here the appended
// diagnostic arrives from a different source: an unsupported-encoding
// detection on the same file's next load.
func TestAppendedErrorPreservesReaderScroll(t *testing.T) {
	dir := t.TempDir()
	recs := append(fileWithStops(t, dir, "a.txt", 10, 1), recSummary)
	m, cmd := loaderModel(t, dir, 80, 24, failLoaderFor("a.txt", tallFailure()), recs...)
	m = settle(t, m, cmd)
	if m.overlay == nil {
		t.Fatal("the first failure did not open the overlay")
	}
	for i := 0; i < 3; i++ {
		m, _ = pressKey(t, m, "down")
	}
	if m.overlay.scroll != 3 {
		t.Fatalf("overlay scroll = %d, want the reader 3 rows down", m.overlay.scroll)
	}
	before := len(m.overlay.lines)

	// A successful load detecting an unsupported encoding appends its
	// diagnostic to the open overlay instead of replacing it.
	req := mintLoad(&m, "a.txt")
	m, cmd = update(t, m, loadDoneMsg{
		path: []byte("a.txt"),
		req:  req,
		buf:  filebuffer.Prepare([]byte(u16le), stopsForFile(m.index, []byte("a.txt"))),
	})
	m = settle(t, m, cmd)
	if m.overlay == nil {
		t.Fatal("the appended diagnostic closed the overlay")
	}
	if m.overlay.scroll != 3 {
		t.Fatalf("the append moved the reader's scroll to %d, want 3", m.overlay.scroll)
	}
	if len(m.overlay.lines) <= before {
		t.Fatalf("the encoding diagnostic did not append: %d → %d lines", before, len(m.overlay.lines))
	}

	// The appended text is reachable: scrolling to the bottom shows it.
	for i := 0; i < 50; i++ {
		m, _ = pressKey(t, m, "down")
	}
	if v := m.View().Content; !strings.Contains(v, "unsupported encoding UTF-16 LE") {
		t.Fatalf("the appended diagnostic is unreachable: %q", v)
	}
}

// assertRunning asserts the model is still running in the given phase:
// no quit pending and the press produced no command.
func assertRunning(t *testing.T, m Model, cmd tea.Cmd, want phase) {
	t.Helper()
	if cmd != nil {
		t.Fatalf("produced a command %T, want none", cmd)
	}
	if m.quit || m.phase != want {
		t.Fatalf("phase=%d quit=%v, want still running in phase %d", m.phase, m.quit, want)
	}
}

// assertQuits presses key on m and asserts the tea.Quit command and
// the settled exit status.
func assertQuits(t *testing.T, m Model, key string, code int) {
	t.Helper()
	m2, cmd := pressKey(t, m, key)
	if cmd == nil {
		t.Fatalf("%s returned no command, want tea.Quit", key)
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("%s command = %T, want tea.QuitMsg", key, cmd())
	}
	if !m2.quit || m2.ExitCode() != code {
		t.Fatalf("%s: quit=%v code=%d, want quit at %d", key, m2.quit, m2.ExitCode(), code)
	}
}

// assertNoOp presses key and asserts nothing happened: no command, no
// quit, no overlay or help state change beyond what the caller checks.
func assertNoOp(t *testing.T, m Model, key string, want phase) {
	t.Helper()
	m2, cmd := pressKey(t, m, key)
	assertRunning(t, m2, cmd, want)
}

// dismissalRow is one row of the Issue #32 dismissal-outcome table.
// setup builds the model in the row's initial state; check asserts the
// full outcome of pressing the dismissal key, including the
// state-specific follow-ups — a second q exits a still-running base
// state with its fixed status while a second Esc leaves it running,
// and the error-over-help row asserts the three-key sequence.
type dismissalRow struct {
	name  string
	setup func(t *testing.T) Model
	check func(t *testing.T, m Model, key string)
}

// baseDismissal asserts the shared still-running rows' outcome: the
// key closes only the overlay — the named phase underneath still runs —
// then a second q exits with the fixed status while a second Esc is a
// no-op.
func baseDismissal(t *testing.T, m Model, key string, want phase, code int) {
	t.Helper()
	m2, cmd := pressKey(t, m, key)
	assertRunning(t, m2, cmd, want)
	assertQuits(t, m2, "q", code)
	assertNoOp(t, m2, "esc", want)
}

// The Issue #32 dismissal-outcome table, run for both q and Esc as the
// dismissal key: a browse-state overlay (error, help, or
// error-over-help) returns to its underlying state, the warning
// overlay over an empty result and help over no-results return to the
// no-results screen, and the fatal no-results overlay — process-fatal
// or record-loss — exits 2 because there is no underlying state. Every
// still-running base state then exits on a second q with its fixed
// status and ignores a second Esc; the error-over-help row alone runs
// the full three-key sequence — close the error, close the restored
// help, then a base-state q exits.
func TestDismissalOutcomeTable(t *testing.T) {
	rows := []dismissalRow{
		{
			name: "browse error overlay returns to browsing",
			setup: func(t *testing.T) Model {
				dir := t.TempDir()
				writeWorkFile(t, dir, "f.txt", "hit\n")
				m := newModel(nil, nil)
				m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
				m, _ = update(t, m, searchDoneMsg{
					index:   fixtureIndex(t, dir, recsOneMatch...),
					waitErr: exitErr(t, 3),
				})
				if m.overlay == nil || m.phase != phaseBrowse {
					t.Fatalf("setup: overlay=%v phase=%d, want the error overlay over browse",
						m.overlay != nil, m.phase)
				}
				return m
			},
			check: func(t *testing.T, m Model, key string) {
				m2, cmd := pressKey(t, m, key)
				assertRunning(t, m2, cmd, phaseBrowse)
				if m2.overlay != nil {
					t.Fatalf("%s did not dismiss the error overlay", key)
				}
				// The fatal search fixed the status at 2: browsing
				// resumes under it and q still exits 2.
				assertQuits(t, m2, "q", 2)
				assertNoOp(t, m2, "esc", phaseBrowse)
			},
		},
		{
			name: "browse help returns to browsing",
			setup: func(t *testing.T) Model {
				m := completedModel(t, 1)
				m, _ = pressKey(t, m, "h")
				if m.help == nil {
					t.Fatal("setup: h did not open help")
				}
				return m
			},
			check: func(t *testing.T, m Model, key string) {
				m2, cmd := pressKey(t, m, key)
				assertRunning(t, m2, cmd, phaseBrowse)
				if m2.help != nil {
					t.Fatalf("%s did not close help", key)
				}
				assertQuits(t, m2, "q", 0)
				assertNoOp(t, m2, "esc", phaseBrowse)
			},
		},
		{
			name: "browse error-over-help restores help then browse then exits",
			setup: func(t *testing.T) Model {
				m := completedModel(t, 1)
				m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 10})
				m, _ = pressKey(t, m, "?")
				for i := 0; i < 4; i++ {
					m, _ = pressKey(t, m, "down")
				}
				if m.help == nil || m.help.scroll != 4 {
					t.Fatalf("setup: help=%v, want open at scroll 4", m.help)
				}
				// A current-file load failure opens the error over the
				// suspended help.
				m, _ = update(t, m, loadDoneMsg{
					path: []byte("fa"),
					req:  mintLoad(&m, "fa"),
					err:  errors.New("denied"),
				})
				if m.overlay == nil || m.help == nil {
					t.Fatalf("setup: overlay=%v help=%v, want error over suspended help",
						m.overlay != nil, m.help != nil)
				}
				return m
			},
			check: func(t *testing.T, m Model, key string) {
				// First dismissal: the error closes and help returns
				// at its scroll position, still running.
				m2, cmd := pressKey(t, m, key)
				assertRunning(t, m2, cmd, phaseBrowse)
				if m2.overlay != nil {
					t.Fatalf("%s did not dismiss the error overlay", key)
				}
				if m2.help == nil || m2.help.scroll != 4 {
					t.Fatalf("%s did not restore help at 4: help=%v", key, m2.help)
				}
				// Second dismissal: the restored help closes to
				// browsing, still running — not an exit.
				m3, cmd := pressKey(t, m2, key)
				assertRunning(t, m3, cmd, phaseBrowse)
				if m3.help != nil {
					t.Fatalf("second %s did not close the restored help", key)
				}
				// Only a base-state q exits, at the fixed status 0;
				// a base-state Esc remains a no-op.
				assertQuits(t, m3, "q", 0)
				assertNoOp(t, m3, "esc", phaseBrowse)
			},
		},
		{
			name: "warning overlay over empty result returns to no-results",
			setup: func(t *testing.T) Model {
				m := newModel(nil, nil)
				m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
				m, _ = update(t, m, searchDoneMsg{
					index:   fixtureIndex(t, "/w", recSummary),
					stderr:  []byte("rg: warn: something odd\n"),
					waitErr: exitErr(t, 1),
				})
				if m.overlay == nil || m.phase != phaseNoResults {
					t.Fatalf("setup: overlay=%v phase=%d, want the warning overlay over no-results",
						m.overlay != nil, m.phase)
				}
				return m
			},
			check: func(t *testing.T, m Model, key string) {
				m2, cmd := pressKey(t, m, key)
				assertRunning(t, m2, cmd, phaseNoResults)
				if m2.overlay != nil {
					t.Fatalf("%s did not dismiss the warning overlay", key)
				}
				if v := m2.View().Content; !strings.Contains(v, "No results found") {
					t.Fatalf("%s did not reveal the no-results screen: %q", key, v)
				}
				assertQuits(t, m2, "q", 1)
				assertNoOp(t, m2, "esc", phaseNoResults)
			},
		},
		{
			name: "help over no-results returns to no-results",
			setup: func(t *testing.T) Model {
				m := noResultsModel(t, exitErr(t, 1), recSummary)
				m, _ = pressKey(t, m, "?")
				if m.help == nil {
					t.Fatal("setup: ? did not open help over no-results")
				}
				return m
			},
			check: func(t *testing.T, m Model, key string) {
				m2, cmd := pressKey(t, m, key)
				assertRunning(t, m2, cmd, phaseNoResults)
				if m2.help != nil {
					t.Fatalf("%s did not close help over no-results", key)
				}
				assertQuits(t, m2, "q", 1)
				assertNoOp(t, m2, "esc", phaseNoResults)
			},
		},
		{
			name: "fatal overlay with no usable results exits 2",
			setup: func(t *testing.T) Model {
				m := newModel(nil, nil)
				m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
				m, _ = update(t, m, searchDoneMsg{
					index:   fixtureIndex(t, "/w", recSummary),
					waitErr: exitErr(t, 3),
				})
				if m.overlay == nil || m.phase != phaseFatal {
					t.Fatalf("setup: overlay=%v phase=%d, want the fatal overlay alone",
						m.overlay != nil, m.phase)
				}
				return m
			},
			check: func(t *testing.T, m Model, key string) {
				assertQuits(t, m, key, 2)
			},
		},
		{
			name: "record-loss overlay with no usable results exits 2",
			setup: func(t *testing.T) Model {
				dir := t.TempDir()
				m := newModel(nil, nil)
				m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
				m, _ = update(t, m, searchDoneMsg{
					index: fixtureStream(t, dir, streamMalformedNoResults),
				})
				if m.overlay == nil || m.phase != phaseFatal {
					t.Fatalf("setup: overlay=%v phase=%d, want the record-loss overlay alone",
						m.overlay != nil, m.phase)
				}
				return m
			},
			check: func(t *testing.T, m Model, key string) {
				assertQuits(t, m, key, 2)
			},
		},
	}
	for _, key := range []string{"q", "esc"} {
		for _, row := range rows {
			t.Run(row.name+"/"+key, func(t *testing.T) {
				row.check(t, row.setup(t), key)
			})
		}
	}
}
