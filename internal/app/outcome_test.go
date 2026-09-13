package app_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/app"
	"vrg/internal/searchindex"
)

// --- Pure outcome function tests ---

// TestDecideOutcomeMatrix is the single table-driven test covering every
// row of the Issue #9 outcome table through the pure DecideOutcome
// function. Later issues extend this table with new rows rather than
// duplicating the decision. Each row asserts the initial presentation
// (state + overlay), whether the overlay is fatal (dismissal exits),
// and the fixed exit status.
func TestDecideOutcomeMatrix(t *testing.T) {
	complete := searchindex.Integrity{Complete: true}
	incomplete := searchindex.Integrity{Complete: false}

	cases := []struct {
		name           string
		process        app.ProcessResult
		integrity      searchindex.Integrity
		usableResults  int
		diagnostics    string
		wantState      app.State
		wantOverlay    app.OverlayKind
		wantFatal      bool
		wantExitStatus int
	}{
		// rg 0, clean complete stream, results → browse, 0.
		{
			name:           "rg0 clean browse",
			process:        app.ProcessResult{ExitCode: 0},
			integrity:      complete,
			usableResults:  1,
			wantState:      app.StateBrowse,
			wantOverlay:    app.OverlayNone,
			wantExitStatus: 0,
		},
		// rg 1, retained results, complete stream → browse, 0
		// (anomalous-exit-1 case).
		{
			name:           "rg1 retained results complete stream browse 0",
			process:        app.ProcessResult{ExitCode: 1},
			integrity:      complete,
			usableResults:  1,
			wantState:      app.StateBrowse,
			wantOverlay:    app.OverlayNone,
			wantExitStatus: 0,
		},
		// rg 1, empty result → no-results, 1.
		{
			name:           "rg1 empty no-results 1",
			process:        app.ProcessResult{ExitCode: 1},
			integrity:      complete,
			usableResults:  0,
			wantState:      app.StateNoResults,
			wantOverlay:    app.OverlayNone,
			wantExitStatus: 1,
		},
		// rg 0, empty result → no-results, 1.
		{
			name:           "rg0 empty no-results 1",
			process:        app.ProcessResult{ExitCode: 0},
			integrity:      complete,
			usableResults:  0,
			wantState:      app.StateNoResults,
			wantOverlay:    app.OverlayNone,
			wantExitStatus: 1,
		},
		// Fatal exit code with usable results → browse + error overlay,
		// dismiss → browse, q → 2.
		{
			name:           "fatal code with results browse overlay",
			process:        app.ProcessResult{ExitCode: 3},
			integrity:      complete,
			usableResults:  1,
			wantState:      app.StateBrowse,
			wantOverlay:    app.OverlayError,
			wantExitStatus: 2,
		},
		// Fatal exit code with no usable results → fatal overlay,
		// q/Esc → 2.
		{
			name:           "fatal code no results fatal overlay",
			process:        app.ProcessResult{ExitCode: 2},
			integrity:      complete,
			usableResults:  0,
			wantState:      app.StateNoResults,
			wantOverlay:    app.OverlayError,
			wantFatal:      true,
			wantExitStatus: 2,
		},
		// Signal death with usable results → browse + error overlay.
		{
			name:           "signal death with results browse overlay",
			process:        app.ProcessResult{SignalDeath: true, ExitCode: 9},
			integrity:      complete,
			usableResults:  1,
			wantState:      app.StateBrowse,
			wantOverlay:    app.OverlayError,
			wantExitStatus: 2,
		},
		// Signal death with no usable results → fatal overlay.
		{
			name:           "signal death no results fatal overlay",
			process:        app.ProcessResult{SignalDeath: true, ExitCode: 9},
			integrity:      complete,
			usableResults:  0,
			wantState:      app.StateNoResults,
			wantOverlay:    app.OverlayError,
			wantFatal:      true,
			wantExitStatus: 2,
		},
		// Missing summary (incomplete stream) with valid matches → 2.
		{
			name:           "missing summary with matches fatal",
			process:        app.ProcessResult{ExitCode: 0},
			integrity:      incomplete,
			usableResults:  1,
			wantState:      app.StateBrowse,
			wantOverlay:    app.OverlayError,
			wantExitStatus: 2,
		},
		// Orphaned end (incomplete stream) → 2.
		{
			name:           "orphaned end incomplete fatal",
			process:        app.ProcessResult{ExitCode: 0},
			integrity:      incomplete,
			usableResults:  0,
			wantState:      app.StateNoResults,
			wantOverlay:    app.OverlayError,
			wantFatal:      true,
			wantExitStatus: 2,
		},
		// Incomplete stream treated as fatal even if rg exit is 0.
		{
			name:           "incomplete stream rg0 fatal",
			process:        app.ProcessResult{ExitCode: 0},
			integrity:      incomplete,
			usableResults:  0,
			wantState:      app.StateNoResults,
			wantOverlay:    app.OverlayError,
			wantFatal:      true,
			wantExitStatus: 2,
		},
		// Incomplete stream treated as fatal even if rg exit is 1.
		{
			name:           "incomplete stream rg1 with results fatal",
			process:        app.ProcessResult{ExitCode: 1},
			integrity:      incomplete,
			usableResults:  1,
			wantState:      app.StateBrowse,
			wantOverlay:    app.OverlayError,
			wantExitStatus: 2,
		},
		// Stderr warning + results (rg 0) → warning overlay, 0.
		{
			name:           "stderr warning with results browse warning 0",
			process:        app.ProcessResult{ExitCode: 0},
			integrity:      complete,
			usableResults:  1,
			diagnostics:    "warn",
			wantState:      app.StateBrowse,
			wantOverlay:    app.OverlayWarning,
			wantExitStatus: 0,
		},
		// Stderr warning + zero results (rg 0) → warning overlay →
		// no-results → 1.
		{
			name:           "stderr warning zero results warning no-results 1",
			process:        app.ProcessResult{ExitCode: 0},
			integrity:      complete,
			usableResults:  0,
			diagnostics:    "warn",
			wantState:      app.StateNoResults,
			wantOverlay:    app.OverlayWarning,
			wantExitStatus: 1,
		},
		// Stderr warning + zero results (rg 1) → warning overlay →
		// no-results → 1.
		{
			name:           "stderr warning rg1 zero results warning no-results 1",
			process:        app.ProcessResult{ExitCode: 1},
			integrity:      complete,
			usableResults:  0,
			diagnostics:    "warn",
			wantState:      app.StateNoResults,
			wantOverlay:    app.OverlayWarning,
			wantExitStatus: 1,
		},
		// Fatal exit code with stderr and results → error overlay (the
		// stderr is the diagnostic; exit 2).
		{
			name:           "fatal code with stderr results error overlay",
			process:        app.ProcessResult{ExitCode: 3},
			integrity:      complete,
			usableResults:  1,
			diagnostics:    "boom",
			wantState:      app.StateBrowse,
			wantOverlay:    app.OverlayError,
			wantExitStatus: 2,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := app.DecideOutcome(app.OutcomeInput{
				Process:       tc.process,
				Integrity:     tc.integrity,
				UsableResults: tc.usableResults,
				Diagnostics:   tc.diagnostics,
			})
			if got.State != tc.wantState {
				t.Fatalf("State = %v, want %v", got.State, tc.wantState)
			}
			if got.Overlay != tc.wantOverlay {
				t.Fatalf("Overlay = %v, want %v", got.Overlay, tc.wantOverlay)
			}
			if got.OverlayFatal != tc.wantFatal {
				t.Fatalf("OverlayFatal = %v, want %v", got.OverlayFatal, tc.wantFatal)
			}
			if got.ExitStatus != tc.wantExitStatus {
				t.Fatalf("ExitStatus = %d, want %d", got.ExitStatus, tc.wantExitStatus)
			}
		})
	}
}

// TestDecideOutcomeGeneratedDiagnostic verifies that when a failed
// process supplies no stderr, the outcome generates a diagnostic naming
// the exit code or signal.
func TestDecideOutcomeGeneratedDiagnostic(t *testing.T) {
	// Fatal exit code, no stderr → diagnostic names the code.
	got := app.DecideOutcome(app.OutcomeInput{
		Process:       app.ProcessResult{ExitCode: 2},
		Integrity:     searchindex.Integrity{Complete: true},
		UsableResults: 0,
	})
	if !strings.Contains(got.OverlayText, "2") {
		t.Fatalf("OverlayText = %q, want it to name exit code 2", got.OverlayText)
	}

	// Signal death, no stderr → diagnostic names the signal.
	got = app.DecideOutcome(app.OutcomeInput{
		Process:       app.ProcessResult{SignalDeath: true, ExitCode: 9},
		Integrity:     searchindex.Integrity{Complete: true},
		UsableResults: 0,
	})
	if !strings.Contains(strings.ToLower(got.OverlayText), "signal") {
		t.Fatalf("OverlayText = %q, want it to name the signal", got.OverlayText)
	}
}

// TestDecideOutcomeStderrDiagnostic verifies that when a failed process
// supplies stderr, the outcome uses it as the overlay text.
func TestDecideOutcomeStderrDiagnostic(t *testing.T) {
	got := app.DecideOutcome(app.OutcomeInput{
		Process:       app.ProcessResult{ExitCode: 3},
		Integrity:     searchindex.Integrity{Complete: true},
		UsableResults: 1,
		Diagnostics:   "boom",
	})
	if !strings.Contains(got.OverlayText, "boom") {
		t.Fatalf("OverlayText = %q, want it to contain the stderr 'boom'", got.OverlayText)
	}
}

// --- Full-flow outcome matrix through Update ---

// outcomeFlowTestCase extends the pure matrix with dismissal and exit
// assertions through the full Update flow.
type outcomeFlowTestCase struct {
	name           string
	process        app.ProcessResult
	records        []string
	diagnostics    string
	wantState      app.State
	wantOverlay    app.OverlayKind
	wantFatal      bool
	wantExitStatus int
	// dismissKey is the key sent to dismiss the overlay (0 = none).
	dismissKey rune
	// wantStateAfterDismiss is the state after dismissing the overlay.
	wantStateAfterDismiss app.State
	// quitKey is the key sent after dismissal to quit (0 = none).
	quitKey rune
	// wantFinalExit is the exit code after the quit key.
	wantFinalExit int
}

// TestOutcomeMatrixFlow is the single table-driven test covering every
// outcome-table row through the full Update flow, with dismissal and
// exit assertions.
func TestOutcomeMatrixFlow(t *testing.T) {
	completeRecords := []string{
		textBegin("a.go"),
		textMatch("a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
		endRecord("a.go", nil),
		summaryRecord(),
	}
	emptyRecords := []string{summaryRecord()}

	cases := []outcomeFlowTestCase{
		{
			name:           "rg0 clean browse dismiss exits 0",
			process:        app.ProcessResult{ExitCode: 0},
			records:        completeRecords,
			wantState:      app.StateBrowse,
			wantOverlay:    app.OverlayNone,
			wantExitStatus: 0,
			quitKey:        'q',
			wantFinalExit:  0,
		},
		{
			name:           "rg1 retained results browse 0",
			process:        app.ProcessResult{ExitCode: 1},
			records:        completeRecords,
			wantState:      app.StateBrowse,
			wantOverlay:    app.OverlayNone,
			wantExitStatus: 0,
			quitKey:        'q',
			wantFinalExit:  0,
		},
		{
			name:           "rg1 empty no-results 1",
			process:        app.ProcessResult{ExitCode: 1},
			records:        emptyRecords,
			wantState:      app.StateNoResults,
			wantOverlay:    app.OverlayNone,
			wantExitStatus: 1,
			quitKey:        'q',
			wantFinalExit:  1,
		},
		{
			name:                  "fatal code with results browse overlay dismiss browse q 2",
			process:               app.ProcessResult{ExitCode: 3},
			records:               completeRecords,
			diagnostics:           "boom",
			wantState:             app.StateBrowse,
			wantOverlay:           app.OverlayError,
			wantExitStatus:        2,
			dismissKey:            'q',
			wantStateAfterDismiss: app.StateBrowse,
			quitKey:               'q',
			wantFinalExit:         2,
		},
		{
			name:                  "fatal code with results browse overlay esc dismiss browse q 2",
			process:               app.ProcessResult{ExitCode: 3},
			records:               completeRecords,
			diagnostics:           "boom",
			wantState:             app.StateBrowse,
			wantOverlay:           app.OverlayError,
			wantExitStatus:        2,
			dismissKey:            tea.KeyEscape,
			wantStateAfterDismiss: app.StateBrowse,
			quitKey:               'q',
			wantFinalExit:         2,
		},
		{
			name:           "fatal code no results overlay q exits 2",
			process:        app.ProcessResult{ExitCode: 2},
			records:        emptyRecords,
			wantState:      app.StateNoResults,
			wantOverlay:    app.OverlayError,
			wantFatal:      true,
			wantExitStatus: 2,
			quitKey:        'q',
			wantFinalExit:  2,
		},
		{
			name:           "fatal code no results overlay esc exits 2",
			process:        app.ProcessResult{ExitCode: 2},
			records:        emptyRecords,
			wantState:      app.StateNoResults,
			wantOverlay:    app.OverlayError,
			wantFatal:      true,
			wantExitStatus: 2,
			quitKey:        tea.KeyEscape,
			wantFinalExit:  2,
		},
		{
			name:                  "signal death with results browse overlay dismiss q 2",
			process:               app.ProcessResult{SignalDeath: true, ExitCode: 9},
			records:               completeRecords,
			wantState:             app.StateBrowse,
			wantOverlay:           app.OverlayError,
			wantExitStatus:        2,
			dismissKey:            'q',
			wantStateAfterDismiss: app.StateBrowse,
			quitKey:               'q',
			wantFinalExit:         2,
		},
		{
			name:           "signal death no results fatal overlay q 2",
			process:        app.ProcessResult{SignalDeath: true, ExitCode: 9},
			records:        emptyRecords,
			wantState:      app.StateNoResults,
			wantOverlay:    app.OverlayError,
			wantFatal:      true,
			wantExitStatus: 2,
			quitKey:        'q',
			wantFinalExit:  2,
		},
		{
			name:                  "stderr warning with results warning overlay dismiss browse q 0",
			process:               app.ProcessResult{ExitCode: 0},
			records:               completeRecords,
			diagnostics:           "warn",
			wantState:             app.StateBrowse,
			wantOverlay:           app.OverlayWarning,
			wantExitStatus:        0,
			dismissKey:            'q',
			wantStateAfterDismiss: app.StateBrowse,
			quitKey:               'q',
			wantFinalExit:         0,
		},
		{
			name:                  "stderr warning zero results warning overlay dismiss no-results q 1",
			process:               app.ProcessResult{ExitCode: 0},
			records:               emptyRecords,
			diagnostics:           "warn",
			wantState:             app.StateNoResults,
			wantOverlay:           app.OverlayWarning,
			wantExitStatus:        1,
			dismissKey:            'q',
			wantStateAfterDismiss: app.StateNoResults,
			quitKey:               'q',
			wantFinalExit:         1,
		},
		{
			name:                  "stderr warning zero results warning overlay esc dismiss no-results q 1",
			process:               app.ProcessResult{ExitCode: 1},
			records:               emptyRecords,
			diagnostics:           "warn",
			wantState:             app.StateNoResults,
			wantOverlay:           app.OverlayWarning,
			wantExitStatus:        1,
			dismissKey:            tea.KeyEscape,
			wantStateAfterDismiss: app.StateNoResults,
			quitKey:               'q',
			wantFinalExit:         1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			idx := buildIndex(t, "/work", tc.records...)
			m := app.New([]string{"--json", "--", "foo", "."}, "/work")
			m, _ = update(t, m, app.SearchCompleteMsg{
				Files:   idx.Files(),
				Lines:   idx.Len(),
				Index:   idx,
				Process: tc.process,
				Stderr:  tc.diagnostics,
			})

			if m.State() != tc.wantState {
				t.Fatalf("initial State = %v, want %v", m.State(), tc.wantState)
			}
			if m.OverlayKind() != tc.wantOverlay {
				t.Fatalf("initial OverlayKind = %v, want %v", m.OverlayKind(), tc.wantOverlay)
			}
			if m.OverlayOpen() != (tc.wantOverlay != app.OverlayNone) {
				t.Fatalf("initial OverlayOpen = %v, want %v", m.OverlayOpen(), tc.wantOverlay != app.OverlayNone)
			}
			if m.ExitCode() != tc.wantExitStatus {
				t.Fatalf("initial ExitCode = %d, want %d", m.ExitCode(), tc.wantExitStatus)
			}

			// Dismiss the overlay if requested.
			if tc.dismissKey != 0 {
				var cmd tea.Cmd
				m, cmd = update(t, m, keyPressOrEscape(tc.dismissKey))
				if tc.wantFatal {
					// Fatal no-results overlay: dismissal exits 2.
					assertQuit(t, cmd)
					if m.ExitCode() != 2 {
						t.Fatalf("after dismiss, ExitCode = %d, want 2", m.ExitCode())
					}
					return
				}
				// Non-fatal overlay: dismissal closes the overlay.
				if cmd != nil {
					t.Fatalf("non-fatal dismiss produced a command: %v", cmd)
				}
				if m.OverlayOpen() {
					t.Fatalf("after dismiss, overlay still open")
				}
				if m.State() != tc.wantStateAfterDismiss {
					t.Fatalf("after dismiss, State = %v, want %v", m.State(), tc.wantStateAfterDismiss)
				}
			}

			// Quit if requested.
			if tc.quitKey != 0 {
				var cmd tea.Cmd
				m, cmd = update(t, m, keyPressOrEscape(tc.quitKey))
				assertQuit(t, cmd)
				if m.ExitCode() != tc.wantFinalExit {
					t.Fatalf("after quit, ExitCode = %d, want %d", m.ExitCode(), tc.wantFinalExit)
				}
			}
		})
	}
}

// keyPressOrEscape builds a key message for the given code. If code is
// tea.KeyEscape, an escape message is returned; otherwise a printable
// key press.
func keyPressOrEscape(code rune) tea.KeyPressMsg {
	if code == tea.KeyEscape {
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	}
	return tea.KeyPressMsg{Code: code, Text: string(code)}
}

// TestFixedStatusCtrlCOverride verifies that the fixed exit status,
// once decided, is overridden only by ctrl+c (which yields 130).
func TestFixedStatusCtrlCOverride(t *testing.T) {
	cases := []struct {
		name     string
		state    app.State
		setup    func(t *testing.T) app.Model
		wantExit int
	}{
		{
			name:     "ctrl+c after browse completion",
			state:    app.StateBrowse,
			wantExit: 130,
			setup: func(t *testing.T) app.Model {
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
				return m
			},
		},
		{
			name:     "ctrl+c after no-results completion",
			state:    app.StateNoResults,
			wantExit: 130,
			setup: func(t *testing.T) app.Model {
				idx := buildIndex(t, "/work", summaryRecord())
				m := app.New([]string{"--json", "--", "foo", "."}, "/work")
				m, _ = update(t, m, app.SearchCompleteMsg{
					Files: idx.Files(), Lines: idx.Len(), Index: idx,
					Process: app.ProcessResult{ExitCode: 0},
				})
				return m
			},
		},
		{
			name:     "ctrl+c with open overlay",
			state:    app.StateBrowse,
			wantExit: 130,
			setup: func(t *testing.T) app.Model {
				idx := buildIndex(t, "/work",
					textBegin("a.go"),
					textMatch("a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
					endRecord("a.go", nil),
					summaryRecord(),
				)
				m := app.New([]string{"--json", "--", "foo", "."}, "/work")
				m, _ = update(t, m, app.SearchCompleteMsg{
					Files: idx.Files(), Lines: idx.Len(), Index: idx,
					Process: app.ProcessResult{ExitCode: 3},
					Stderr:  "boom",
				})
				return m
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := tc.setup(t)
			m, cmd := update(t, m, ctrlC())
			assertQuit(t, cmd)
			if m.ExitCode() != tc.wantExit {
				t.Fatalf("ExitCode = %d, want %d", m.ExitCode(), tc.wantExit)
			}
		})
	}
}

// TestEscNeverExitsBaseState verifies that Esc never exits from a base
// state (browse or no-results without an overlay).
func TestEscNeverExitsBaseState(t *testing.T) {
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
	if m.State() != app.StateBrowse {
		t.Fatalf("after Esc from browse, State = %v, want StateBrowse", m.State())
	}
	if cmd != nil {
		t.Fatalf("Esc from browse produced a command: %v", cmd)
	}

	// No-results without overlay.
	idx = buildIndex(t, "/work", summaryRecord())
	m = app.New([]string{"--json", "--", "foo", "."}, "/work")
	m, _ = update(t, m, app.SearchCompleteMsg{
		Files: idx.Files(), Lines: idx.Len(), Index: idx,
		Process: app.ProcessResult{ExitCode: 0},
	})
	m, cmd = update(t, m, keyPressOrEscape(tea.KeyEscape))
	if m.State() != app.StateNoResults {
		t.Fatalf("after Esc from no-results, State = %v, want StateNoResults", m.State())
	}
	if cmd != nil {
		t.Fatalf("Esc from no-results produced a command: %v", cmd)
	}
}

// TestFixedStatusNotRecomputed verifies that the exit status is chosen
// once after search and later dismissal does not recompute it.
func TestFixedStatusNotRecomputed(t *testing.T) {
	idx := buildIndex(t, "/work",
		textBegin("a.go"),
		textMatch("a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
		endRecord("a.go", nil),
		summaryRecord(),
	)
	m := app.New([]string{"--json", "--", "foo", "."}, "/work")
	m, _ = update(t, m, app.SearchCompleteMsg{
		Files: idx.Files(), Lines: idx.Len(), Index: idx,
		Process: app.ProcessResult{ExitCode: 3},
		Stderr:  "boom",
	})
	// Fixed status is 2 (fatal with results).
	if m.ExitCode() != 2 {
		t.Fatalf("initial ExitCode = %d, want 2", m.ExitCode())
	}
	// Dismiss the overlay.
	m, _ = update(t, m, keyPress('q'))
	if m.ExitCode() != 2 {
		t.Fatalf("after dismiss, ExitCode = %d, want 2 (fixed)", m.ExitCode())
	}
	// Quit from browse.
	m, cmd := update(t, m, keyPress('q'))
	assertQuit(t, cmd)
	if m.ExitCode() != 2 {
		t.Fatalf("after quit, ExitCode = %d, want 2 (fixed)", m.ExitCode())
	}
}
