package app_test

import (
	"encoding/json"
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
		recordLoss     app.RecordLoss
		recordLossDiag string
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
		// Issue #10: unknown-type-only warnings with zero results →
		// warning overlay, then no-results, exit 1.
		{
			name:           "unknown-only zero results warning no-results 1",
			process:        app.ProcessResult{ExitCode: 0},
			integrity:      complete,
			usableResults:  0,
			recordLoss:     app.RecordLoss{Unknown: 1},
			recordLossDiag: "1 unrecognised record types skipped",
			wantState:      app.StateNoResults,
			wantOverlay:    app.OverlayWarning,
			wantExitStatus: 1,
		},
		// Issue #10: malformed skipped with usable results →
		// browse with warning overlay, exit 0.
		{
			name:           "malformed with usable results browse warning 0",
			process:        app.ProcessResult{ExitCode: 0},
			integrity:      complete,
			usableResults:  1,
			recordLoss:     app.RecordLoss{Malformed: 1},
			recordLossDiag: "1 malformed record skipped",
			wantState:      app.StateBrowse,
			wantOverlay:    app.OverlayWarning,
			wantExitStatus: 0,
		},
		// Issue #10: malformed skipped with zero usable results →
		// record-loss fatal overlay, exit 2.
		{
			name:           "malformed zero usable results record-loss fatal 2",
			process:        app.ProcessResult{ExitCode: 0},
			integrity:      complete,
			usableResults:  0,
			recordLoss:     app.RecordLoss{Malformed: 1},
			recordLossDiag: "1 malformed record skipped",
			wantState:      app.StateNoResults,
			wantOverlay:    app.OverlayError,
			wantFatal:      true,
			wantExitStatus: 2,
		},
		// Issue #10: oversized with zero usable results →
		// record-loss fatal overlay, exit 2.
		{
			name:           "oversized zero usable results record-loss fatal 2",
			process:        app.ProcessResult{ExitCode: 0},
			integrity:      complete,
			usableResults:  0,
			recordLoss:     app.RecordLoss{Oversized: 1},
			recordLossDiag: "1 oversized record skipped\noversized record skipped for a.go",
			wantState:      app.StateNoResults,
			wantOverlay:    app.OverlayError,
			wantFatal:      true,
			wantExitStatus: 2,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := app.DecideOutcome(app.OutcomeInput{
				Process:               tc.process,
				Integrity:             tc.integrity,
				UsableResults:         tc.usableResults,
				Diagnostics:           tc.diagnostics,
				RecordLoss:            tc.recordLoss,
				RecordLossDiagnostics: tc.recordLossDiag,
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

// --- Issue #36: structured integrity causes and universal composition ---

// TestDecideOutcomeIntegrityCauseLines covers every integrity cause in
// the Issue #36 matrix: each cause produces its stable user-facing line
// naming the EscapePath-escaped path where applicable. Each row asserts
// the complete overlay text so a regression that drops, reorders, or
// rephrases the integrity component fails.
func TestDecideOutcomeIntegrityCauseLines(t *testing.T) {
	cases := []struct {
		name     string
		causes   []searchindex.IntegrityCause
		wantText string
	}{
		{
			name:     "duplicate begin names escaped path",
			causes:   []searchindex.IntegrityCause{{Kind: searchindex.IntegrityCauseDuplicateBegin, Path: []byte("a.go")}},
			wantText: "duplicate begin record for a.go",
		},
		{
			name:     "orphaned match names escaped path",
			causes:   []searchindex.IntegrityCause{{Kind: searchindex.IntegrityCauseOrphanedMatch, Path: []byte("a.go")}},
			wantText: "orphaned match record for a.go",
		},
		{
			name:     "match after binary-excluding end names escaped path",
			causes:   []searchindex.IntegrityCause{{Kind: searchindex.IntegrityCauseMatchAfterBinaryEnd, Path: []byte("a.go")}},
			wantText: "orphaned match record for a.go",
		},
		{
			name:     "orphaned end names escaped path",
			causes:   []searchindex.IntegrityCause{{Kind: searchindex.IntegrityCauseOrphanedEnd, Path: []byte("a.go")}},
			wantText: "orphaned end record for a.go",
		},
		{
			name:     "missing end names escaped path",
			causes:   []searchindex.IntegrityCause{{Kind: searchindex.IntegrityCauseMissingEnd, Path: []byte("a.go")}},
			wantText: "missing end record for a.go",
		},
		{
			name:     "missing summary",
			causes:   []searchindex.IntegrityCause{{Kind: searchindex.IntegrityCauseMissingSummary}},
			wantText: "missing summary record",
		},
		{
			name:     "extra summary",
			causes:   []searchindex.IntegrityCause{{Kind: searchindex.IntegrityCauseExtraSummary}},
			wantText: "extra summary record",
		},
		{
			name:     "record after summary",
			causes:   []searchindex.IntegrityCause{{Kind: searchindex.IntegrityCauseRecordAfterSummary}},
			wantText: "record after summary",
		},
		{
			name:     "unterminated final record",
			causes:   []searchindex.IntegrityCause{{Kind: searchindex.IntegrityCauseUnterminatedRecord}},
			wantText: "unterminated final record",
		},
		{
			name:     "path with newline escaped through EscapePath",
			causes:   []searchindex.IntegrityCause{{Kind: searchindex.IntegrityCauseDuplicateBegin, Path: []byte("a\nb.go")}},
			wantText: `duplicate begin record for a\nb.go`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := app.DecideOutcome(app.OutcomeInput{
				Process:       app.ProcessResult{ExitCode: 0},
				Integrity:     searchindex.Integrity{Complete: false, Causes: tc.causes},
				UsableResults: 1,
			})
			if got.OverlayText != tc.wantText {
				t.Fatalf("OverlayText = %q, want %q", got.OverlayText, tc.wantText)
			}
			if got.ExitStatus != 2 {
				t.Fatalf("ExitStatus = %d, want 2", got.ExitStatus)
			}
		})
	}
}

// TestDecideOutcomeComposedOrder covers the Issue #36 universal
// component order — process diagnostics, then integrity-cause lines,
// then record-loss components (malformed aggregate, oversized details),
// then unknown-type warnings — in fatal and non-fatal branches alike.
// Each row asserts equality of the complete ordered overlay text.
func TestDecideOutcomeComposedOrder(t *testing.T) {
	incomplete := func(causes ...searchindex.IntegrityCause) searchindex.Integrity {
		return searchindex.Integrity{Complete: false, Causes: causes}
	}
	orphanA := searchindex.IntegrityCause{Kind: searchindex.IntegrityCauseOrphanedMatch, Path: []byte("a.go")}
	ras := searchindex.IntegrityCause{Kind: searchindex.IntegrityCauseRecordAfterSummary}

	cases := []struct {
		name           string
		process        app.ProcessResult
		integrity      searchindex.Integrity
		usableResults  int
		diagnostics    string
		recordLoss     app.RecordLoss
		recordLossDiag string
		wantText       string
		wantExitStatus int
	}{
		// Fatal: real stderr and integrity causes compose; neither
		// suppresses the other and no process-status line appears for
		// a 0/1 exit.
		{
			name:          "fatal integrity with stderr composes both in order",
			process:       app.ProcessResult{ExitCode: 0},
			integrity:     incomplete(orphanA, searchindex.IntegrityCause{Kind: searchindex.IntegrityCauseMissingSummary}),
			usableResults: 1,
			diagnostics:   "warn-one\nwarn-two",
			wantText: "warn-one\nwarn-two\n" +
				"orphaned match record for a.go\n" +
				"missing summary record",
			wantExitStatus: 2,
		},
		// Fatal: failed process with no stderr emits the generated
		// line, then integrity causes, then record loss.
		{
			name:           "fatal exit no stderr generated line then causes then record loss",
			process:        app.ProcessResult{ExitCode: 3},
			integrity:      incomplete(searchindex.IntegrityCause{Kind: searchindex.IntegrityCauseMissingEnd, Path: []byte("a.go")}),
			usableResults:  0,
			recordLoss:     app.RecordLoss{Malformed: 1},
			recordLossDiag: "1 malformed record skipped",
			wantText: "ripgrep exited with code 3\n" +
				"missing end record for a.go\n" +
				"1 malformed record skipped",
			wantExitStatus: 2,
		},
		// Fatal: signal death with no stderr names the signal, then
		// the integrity cause.
		{
			name:          "signal death no stderr names signal then causes",
			process:       app.ProcessResult{SignalDeath: true, ExitCode: 9},
			integrity:     incomplete(ras),
			usableResults: 0,
			wantText: "ripgrep killed by signal 9\n" +
				"record after summary",
			wantExitStatus: 2,
		},
		// Fatal: a failed process that supplied stderr emits the
		// stderr, not the generated line, then the causes.
		{
			name:           "fatal exit with stderr uses stderr then causes",
			process:        app.ProcessResult{ExitCode: 3},
			integrity:      incomplete(orphanA),
			usableResults:  1,
			diagnostics:    "boom",
			wantText:       "boom\norphaned match record for a.go",
			wantExitStatus: 2,
		},
		// Overlap: a second summary has the sole integrity line
		// "extra summary record" and no "record after summary" line.
		{
			name:           "second summary sole line is extra summary record",
			process:        app.ProcessResult{ExitCode: 0},
			integrity:      incomplete(searchindex.IntegrityCause{Kind: searchindex.IntegrityCauseExtraSummary}),
			usableResults:  0,
			wantText:       "extra summary record",
			wantExitStatus: 2,
		},
		// Overlap: a post-summary begin has the sole integrity line
		// "record after summary" with no lifecycle line and no
		// end-of-stream missing end.
		{
			name:           "post-summary begin sole line is record after summary",
			process:        app.ProcessResult{ExitCode: 0},
			integrity:      incomplete(ras),
			usableResults:  0,
			wantText:       "record after summary",
			wantExitStatus: 2,
		},
		// Overlap: a post-summary unterminated fragment has "record
		// after summary" followed by the malformed aggregate, with no
		// "unterminated final record" line.
		{
			name:           "post-summary unterminated fragment is record after summary plus malformed",
			process:        app.ProcessResult{ExitCode: 0},
			integrity:      incomplete(ras),
			usableResults:  1,
			recordLoss:     app.RecordLoss{Malformed: 1},
			recordLossDiag: "1 malformed record skipped",
			wantText:       "record after summary\n1 malformed record skipped",
			wantExitStatus: 2,
		},
		// Overlap: a post-summary oversized record has "record after
		// summary" followed by the oversized component — the Issue #37
		// aggregate, then the per-path oversized detail — with no
		// oversized integrity cause.
		{
			name:           "post-summary oversized is record after summary plus oversized detail",
			process:        app.ProcessResult{ExitCode: 0},
			integrity:      incomplete(ras),
			usableResults:  1,
			recordLoss:     app.RecordLoss{Oversized: 1},
			recordLossDiag: "1 oversized record skipped\noversized record skipped for q.go",
			wantText: "record after summary\n" +
				"1 oversized record skipped\n" +
				"oversized record skipped for q.go",
			wantExitStatus: 2,
		},
		// Overlap: a post-summary unknown-type record has "record
		// after summary" followed by the unknown-type warning in the
		// unknown-warning slot, with no second integrity cause.
		{
			name:           "post-summary unknown type is record after summary plus unknown warning",
			process:        app.ProcessResult{ExitCode: 0},
			integrity:      incomplete(ras),
			usableResults:  1,
			recordLoss:     app.RecordLoss{Unknown: 1},
			recordLossDiag: "1 unrecognised record types skipped",
			wantText:       "record after summary\n1 unrecognised record types skipped",
			wantExitStatus: 2,
		},
		// Dual representation: a trailing unterminated record outside
		// the post-summary state produces its integrity line and the
		// malformed aggregate.
		{
			name:    "unterminated record dual representation with malformed aggregate",
			process: app.ProcessResult{ExitCode: 0},
			integrity: incomplete(
				searchindex.IntegrityCause{Kind: searchindex.IntegrityCauseMissingSummary},
				searchindex.IntegrityCause{Kind: searchindex.IntegrityCauseUnterminatedRecord},
			),
			usableResults:  1,
			recordLoss:     app.RecordLoss{Malformed: 1},
			recordLossDiag: "1 malformed record skipped",
			wantText: "missing summary record\n" +
				"unterminated final record\n" +
				"1 malformed record skipped",
			wantExitStatus: 2,
		},
		// Uncapped multiplicity: repeated identical violations emit
		// one line each, in detection order.
		{
			name:    "repeated orphaned matches emit one line each uncapped",
			process: app.ProcessResult{ExitCode: 0},
			integrity: incomplete(
				searchindex.IntegrityCause{Kind: searchindex.IntegrityCauseOrphanedMatch, Path: []byte("a.go")},
				searchindex.IntegrityCause{Kind: searchindex.IntegrityCauseOrphanedMatch, Path: []byte("a.go")},
				searchindex.IntegrityCause{Kind: searchindex.IntegrityCauseOrphanedMatch, Path: []byte("a.go")},
			),
			usableResults: 3,
			wantText: "orphaned match record for a.go\n" +
				"orphaned match record for a.go\n" +
				"orphaned match record for a.go",
			wantExitStatus: 2,
		},
		// Deterministic ordering: missing ends appear in unsigned
		// raw-path order.
		{
			name:    "two missing ends ordered by unsigned raw-path bytes",
			process: app.ProcessResult{ExitCode: 0},
			integrity: incomplete(
				searchindex.IntegrityCause{Kind: searchindex.IntegrityCauseMissingEnd, Path: []byte("a.go")},
				searchindex.IntegrityCause{Kind: searchindex.IntegrityCauseMissingEnd, Path: []byte("b.go")},
			),
			usableResults: 2,
			wantText: "missing end record for a.go\n" +
				"missing end record for b.go",
			wantExitStatus: 2,
		},
		// Non-fatal: a complete stream with stderr and record loss
		// composes process then record-loss components in the same
		// universal order.
		{
			name:           "non-fatal warning composes stderr then record loss then unknown",
			process:        app.ProcessResult{ExitCode: 0},
			integrity:      searchindex.Integrity{Complete: true},
			usableResults:  1,
			diagnostics:    "warn",
			recordLoss:     app.RecordLoss{Malformed: 1, Oversized: 1, Unknown: 2},
			recordLossDiag: "1 malformed record skipped\n1 oversized record skipped\noversized record skipped for b.go\n2 unrecognised record types skipped",
			wantText: "warn\n" +
				"1 malformed record skipped\n" +
				"1 oversized record skipped\n" +
				"oversized record skipped for b.go\n" +
				"2 unrecognised record types skipped",
			wantExitStatus: 0,
		},
		// A 0/1 exit never emits a process-status line, even when the
		// stream is incomplete and there is no stderr.
		{
			name:           "incomplete stream rg0 no stderr names cause not process status",
			process:        app.ProcessResult{ExitCode: 0},
			integrity:      incomplete(searchindex.IntegrityCause{Kind: searchindex.IntegrityCauseMissingSummary}),
			usableResults:  0,
			wantText:       "missing summary record",
			wantExitStatus: 2,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := app.DecideOutcome(app.OutcomeInput{
				Process:               tc.process,
				Integrity:             tc.integrity,
				UsableResults:         tc.usableResults,
				Diagnostics:           tc.diagnostics,
				RecordLoss:            tc.recordLoss,
				RecordLossDiagnostics: tc.recordLossDiag,
			})
			if got.OverlayText != tc.wantText {
				t.Fatalf("OverlayText = %q, want %q", got.OverlayText, tc.wantText)
			}
			if got.ExitStatus != tc.wantExitStatus {
				t.Fatalf("ExitStatus = %d, want %d", got.ExitStatus, tc.wantExitStatus)
			}
			if strings.Contains(got.OverlayText, "ripgrep exited with code 0") ||
				strings.Contains(got.OverlayText, "ripgrep exited with code 1") {
				t.Fatalf("OverlayText = %q contains a process-status line for a 0/1 exit", got.OverlayText)
			}
		})
	}
}

// contextRecord builds a context record whose data payload is ignored.
func contextRecord() string {
	rec := map[string]any{
		"type": "context",
		"data": map[string]any{
			"path":        map[string]any{"text": "ignored.go"},
			"lines":       map[string]any{"text": "ignored\n"},
			"line_number": 99,
		},
	}
	b, _ := json.Marshal(rec)
	return string(b)
}

// buildIndexStream builds a searchindex.Index by feeding raw stream
// data through the bounded ReadFrom record reader, so oversized-record
// and trailing-fragment paths are exercised exactly as in production.
func buildIndexStream(t *testing.T, workdir string, data string) *searchindex.Index {
	t.Helper()
	b := searchindex.NewBuilder(workdir)
	if _, err := b.ReadFrom(strings.NewReader(data)); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}
	return b.Build()
}

// oversizedMatchRecoverable builds a match record of exactly size bytes
// with the path field placed early so the path is recoverable from the
// first MaxRecordSize+1 bytes of partial oversized data.
func oversizedMatchRecoverable(path string, size int) string {
	prefix := `{"type":"match","data":{"path":{"text":"` + path + `"},"line_number":1,"submatches":[{"match":{"text":"x"},"start":0,"end":1}],"lines":{"text":"`
	suffix := `"}}}`
	paddingLen := size - len(prefix) - len(suffix)
	if paddingLen < 1 {
		paddingLen = 1
	}
	return prefix + strings.Repeat("x", paddingLen) + suffix
}

// oversizedMatchAnonymous builds an oversized match record of exactly
// size bytes whose data carries no path field, so path recovery finds
// nothing and the record produces no per-path detail — Issue #37's
// anonymous oversized case.
func oversizedMatchAnonymous(size int) string {
	prefix := `{"type":"match","data":{"lines":{"text":"`
	suffix := `"},"line_number":1,"submatches":[]}}`
	paddingLen := size - len(prefix) - len(suffix)
	if paddingLen < 1 {
		paddingLen = 1
	}
	return prefix + strings.Repeat("x", paddingLen) + suffix
}

// TestOutcomeIntegrityDiagnosticsFlow verifies that the composed
// diagnostic text reaches both sinks identically through the full
// Update flow: the overlay text and the collected diagnostic replayed
// to stderr carry the same complete ordered lines.
func TestOutcomeIntegrityDiagnosticsFlow(t *testing.T) {
	cases := []struct {
		name      string
		build     func(t *testing.T) *searchindex.Index
		process   app.ProcessResult
		stderr    string
		wantText  string
		wantFatal bool
	}{
		{
			name: "missing summary names the cause not the exit code",
			build: func(t *testing.T) *searchindex.Index {
				return buildIndexRaw(t, "/work",
					textBegin("a.go"),
					textMatch("a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
					endRecord("a.go", nil),
				)
			},
			process:  app.ProcessResult{ExitCode: 0},
			wantText: "missing summary record",
		},
		{
			name: "missing end names the escaped path",
			build: func(t *testing.T) *searchindex.Index {
				return buildIndexRaw(t, "/work",
					textBegin("a.go"),
					textMatch("a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
					summaryRecord(),
				)
			},
			process:  app.ProcessResult{ExitCode: 0},
			wantText: "missing end record for a.go",
		},
		{
			name: "context after summary is record after summary",
			build: func(t *testing.T) *searchindex.Index {
				return buildIndexRaw(t, "/work", summaryRecord(), contextRecord())
			},
			process:   app.ProcessResult{ExitCode: 0},
			wantText:  "record after summary",
			wantFatal: true,
		},
		{
			name: "damaged stream with real stderr composes all causes together",
			build: func(t *testing.T) *searchindex.Index {
				return buildIndexRaw(t, "/work",
					textMatch("a.go", "x\n", 1, subSpec{"x", 0, 1}),
					textBegin("b.go"),
				)
			},
			process: app.ProcessResult{ExitCode: 0},
			stderr:  "rg warning",
			wantText: "rg warning\n" +
				"orphaned match record for a.go\n" +
				"missing end record for b.go\n" +
				"missing summary record",
		},
		{
			name: "post-summary unterminated fragment is record after summary plus malformed",
			build: func(t *testing.T) *searchindex.Index {
				return buildIndexStream(t, "/work",
					textBegin("a.go")+"\n"+
						textMatch("a.go", "x\n", 1, subSpec{"x", 0, 1})+"\n"+
						endRecord("a.go", nil)+"\n"+
						summaryRecord()+"\n"+
						`{"type":"summary"` /* unterminated fragment, no newline */)
			},
			process:  app.ProcessResult{ExitCode: 0},
			wantText: "record after summary\n1 malformed record skipped",
		},
		{
			name: "post-summary unknown type is record after summary plus unknown warning",
			build: func(t *testing.T) *searchindex.Index {
				return buildIndexRaw(t, "/work",
					summaryRecord(), `{"type":"mystery","data":{}}`)
			},
			process:   app.ProcessResult{ExitCode: 0},
			wantText:  "record after summary\n1 unrecognised record types skipped",
			wantFatal: true,
		},
		{
			name: "post-summary oversized is record after summary plus oversized detail",
			build: func(t *testing.T) *searchindex.Index {
				rec := oversizedMatchRecoverable("q.go", 64*1024*1024+100)
				return buildIndexStream(t, "/work", summaryRecord()+"\n"+rec+"\n")
			},
			process:   app.ProcessResult{ExitCode: 0},
			wantText:  "record after summary\n1 oversized record skipped\noversized record skipped for q.go",
			wantFatal: true,
		},
		{
			name: "path with newline cannot forge paragraph breaks",
			build: func(t *testing.T) *searchindex.Index {
				return buildIndexRaw(t, "/work",
					textBegin("a\nb.go"),
					textBegin("a\nb.go"),
					endRecord("a\nb.go", nil),
					summaryRecord(),
				)
			},
			process:   app.ProcessResult{ExitCode: 0},
			wantText:  `duplicate begin record for a\nb.go`,
			wantFatal: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			idx := tc.build(t)
			m := app.New([]string{"--json", "--", "foo", "."}, "/work")
			m, _ = update(t, m, app.SearchCompleteMsg{
				Files:   idx.Files(),
				Lines:   idx.Len(),
				Index:   idx,
				Process: tc.process,
				Stderr:  tc.stderr,
			})
			if m.OverlayText() != tc.wantText {
				t.Fatalf("OverlayText = %q, want %q", m.OverlayText(), tc.wantText)
			}
			if m.ExitCode() != 2 {
				t.Fatalf("ExitCode = %d, want 2", m.ExitCode())
			}
			// The same composed diagnostic — same lines, same order —
			// is collected for post-restoration stderr replay.
			assertDiagnosticsEq(t, m, []string{tc.wantText})
			// Dismissal of a fatal no-results overlay exits 2; a
			// browse overlay dismisses back to browse.
			m, cmd := update(t, m, keyPress('q'))
			if tc.wantFatal {
				assertQuit(t, cmd)
				if m.ExitCode() != 2 {
					t.Fatalf("after dismiss, ExitCode = %d, want 2", m.ExitCode())
				}
			}
		})
	}
}

// TestOutcomeMissingEndDeterministic verifies that missing-end cause
// lines for several still-open files appear in unsigned raw-path order
// and the composed diagnostic is stable across repeated builds.
func TestOutcomeMissingEndDeterministic(t *testing.T) {
	want := "missing end record for a.go\n" +
		"missing end record for b.go\n" +
		`missing end record for \xff.g` + "\n" +
		"missing summary record"
	for i := 0; i < 20; i++ {
		b := searchindex.NewBuilder("/work")
		_ = b.Add([]byte(textBegin("b.go")))
		_ = b.Add([]byte(textBegin("a.go")))
		_ = b.Add([]byte(bytesBeginRecord([]byte{0xff, '.', 'g'})))
		idx := b.Build()
		m := app.New([]string{"--json", "--", "foo", "."}, "/work")
		m, _ = update(t, m, app.SearchCompleteMsg{
			Files:   idx.Files(),
			Lines:   idx.Len(),
			Index:   idx,
			Process: app.ProcessResult{ExitCode: 0},
		})
		if m.OverlayText() != want {
			t.Fatalf("build %d: OverlayText = %q, want %q", i, m.OverlayText(), want)
		}
	}
}

// --- Issue #37: oversized aggregate and anonymous records ---

// TestOutcomeOversizedAggregateDiagnostics covers the Issue #37
// oversized component through the full Update flow: the oversized
// component always leads with the pluralized aggregate built from
// Index.OversizedCount — exactly "1 oversized record skipped" for one
// and "N oversized records skipped" for every other count — followed by
// one "oversized record skipped for <sanitized path>" detail per
// distinct recoverable raw path in first-occurrence order. The
// aggregate is emitted whenever the count is positive even when no path
// detail exists, so an anonymous oversized record (the limit hit before
// data.path was parsed) is never invisible: with zero usable results
// the fatal overlay contains exactly the aggregate line, and with
// usable results the record loss produces a warning overlay and the
// aggregate reaches the stderr replay.
func TestOutcomeOversizedAggregateDiagnostics(t *testing.T) {
	const mib = 64 * 1024 * 1024
	validStream := textBegin("b.go") + "\n" +
		textMatch("b.go", "y\n", 1, subSpec{"y", 0, 1}) + "\n" +
		endRecord("b.go", nil) + "\n" +
		summaryRecord() + "\n"

	cases := []struct {
		name        string
		stream      string
		wantText    string
		wantOverlay app.OverlayKind
		wantFatal   bool
		wantExit    int
	}{
		{
			name:   "one oversized record emits singular aggregate then detail",
			stream: oversizedMatchRecoverable("a.go", mib+100) + "\n" + validStream,
			wantText: "1 oversized record skipped\n" +
				"oversized record skipped for a.go",
			wantOverlay: app.OverlayWarning,
			wantExit:    0,
		},
		{
			name: "two oversized records for same path emit plural aggregate and one detail",
			stream: oversizedMatchRecoverable("a.go", mib+100) + "\n" +
				oversizedMatchRecoverable("a.go", mib+200) + "\n" +
				validStream,
			wantText: "2 oversized records skipped\n" +
				"oversized record skipped for a.go",
			wantOverlay: app.OverlayWarning,
			wantExit:    0,
		},
		{
			name:        "anonymous oversized zero usable results fatal aggregate only",
			stream:      oversizedMatchAnonymous(mib+100) + "\n" + summaryRecord() + "\n",
			wantText:    "1 oversized record skipped",
			wantOverlay: app.OverlayError,
			wantFatal:   true,
			wantExit:    2,
		},
		{
			name:        "anonymous oversized with usable results surfaces aggregate in overlay and replay",
			stream:      oversizedMatchAnonymous(mib+100) + "\n" + validStream,
			wantText:    "1 oversized record skipped",
			wantOverlay: app.OverlayWarning,
			wantExit:    0,
		},
		{
			name: "mixed recoverability aggregate totals every record and names distinct paths once",
			stream: oversizedMatchAnonymous(mib+100) + "\n" +
				oversizedMatchRecoverable("a.go", mib+100) + "\n" +
				oversizedMatchRecoverable("a.go", mib+200) + "\n" +
				oversizedMatchRecoverable("c.go", mib+300) + "\n" +
				validStream,
			wantText: "4 oversized records skipped\n" +
				"oversized record skipped for a.go\n" +
				"oversized record skipped for c.go",
			wantOverlay: app.OverlayWarning,
			wantExit:    0,
		},
		{
			name: "aggregate sits after malformed aggregate and before per-path details",
			stream: "{bad json\n" +
				oversizedMatchRecoverable("a.go", mib+100) + "\n" +
				validStream,
			wantText: "1 malformed record skipped\n" +
				"1 oversized record skipped\n" +
				"oversized record skipped for a.go",
			wantOverlay: app.OverlayWarning,
			wantExit:    0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			idx := buildIndexStream(t, "/work", tc.stream)
			m := app.New([]string{"--json", "--", "foo", "."}, "/work")
			m, _ = update(t, m, app.SearchCompleteMsg{
				Files:   idx.Files(),
				Lines:   idx.Len(),
				Index:   idx,
				Process: app.ProcessResult{ExitCode: 0},
			})
			if !m.OverlayOpen() {
				t.Fatalf("overlay not open; oversized record loss must never pass silently")
			}
			if m.OverlayKind() != tc.wantOverlay {
				t.Fatalf("OverlayKind = %v, want %v", m.OverlayKind(), tc.wantOverlay)
			}
			if m.OverlayFatal() != tc.wantFatal {
				t.Fatalf("OverlayFatal = %v, want %v", m.OverlayFatal(), tc.wantFatal)
			}
			if m.OverlayText() != tc.wantText {
				t.Fatalf("OverlayText = %q, want %q", m.OverlayText(), tc.wantText)
			}
			if m.ExitCode() != tc.wantExit {
				t.Fatalf("ExitCode = %d, want %d", m.ExitCode(), tc.wantExit)
			}
			// The same composed diagnostic — same lines, same order —
			// is collected for post-restoration stderr replay.
			assertDiagnosticsEq(t, m, []string{tc.wantText})
			// A fatal no-results overlay dismisses to exit 2.
			if tc.wantFatal {
				m, cmd := update(t, m, keyPress('q'))
				assertQuit(t, cmd)
				if m.ExitCode() != 2 {
					t.Fatalf("after dismiss, ExitCode = %d, want 2", m.ExitCode())
				}
			}
		})
	}
}

// --- Full-flow outcome matrix through Update ---

// outcomeFlowTestCase extends the pure matrix with dismissal and exit
// assertions through the full Update flow.
type outcomeFlowTestCase struct {
	name    string
	process app.ProcessResult
	records []string
	// raw is true when records should be fed through buildIndexRaw
	// (no auto-completion, no fatal on Add errors) instead of
	// buildIndex. This is needed for record-loss tests that include
	// malformed or unknown records.
	raw            bool
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
		// Issue #10: unknown-type-only warnings with zero results →
		// warning overlay, dismiss → no-results, q → 1.
		{
			name:                  "unknown-only zero results warning overlay dismiss no-results q 1",
			process:               app.ProcessResult{ExitCode: 0},
			records:               []string{`{"type":"unknown","data":{}}`, summaryRecord()},
			raw:                   true,
			wantState:             app.StateNoResults,
			wantOverlay:           app.OverlayWarning,
			wantExitStatus:        1,
			dismissKey:            'q',
			wantStateAfterDismiss: app.StateNoResults,
			quitKey:               'q',
			wantFinalExit:         1,
		},
		// Issue #10: malformed skipped with usable results →
		// browse with warning overlay, dismiss → browse, q → 0.
		{
			name:                  "malformed with usable results browse warning dismiss browse q 0",
			process:               app.ProcessResult{ExitCode: 0},
			records:               []string{textBegin("a.go"), `{bad json`, textMatch("a.go", "hello\n", 1, subSpec{"hello", 0, 5}), endRecord("a.go", nil), summaryRecord()},
			raw:                   true,
			wantState:             app.StateBrowse,
			wantOverlay:           app.OverlayWarning,
			wantExitStatus:        0,
			dismissKey:            'q',
			wantStateAfterDismiss: app.StateBrowse,
			quitKey:               'q',
			wantFinalExit:         0,
		},
		// Issue #10: malformed skipped with zero usable results →
		// record-loss fatal overlay, q → 2.
		{
			name:           "malformed zero usable results record-loss fatal q 2",
			process:        app.ProcessResult{ExitCode: 0},
			records:        []string{`{bad json`, summaryRecord()},
			raw:            true,
			wantState:      app.StateNoResults,
			wantOverlay:    app.OverlayError,
			wantFatal:      true,
			wantExitStatus: 2,
			quitKey:        'q',
			wantFinalExit:  2,
		},
		// Issue #10: malformed skipped with zero usable results →
		// record-loss fatal overlay, Esc → 2.
		{
			name:           "malformed zero usable results record-loss fatal esc 2",
			process:        app.ProcessResult{ExitCode: 0},
			records:        []string{`{bad json`, summaryRecord()},
			raw:            true,
			wantState:      app.StateNoResults,
			wantOverlay:    app.OverlayError,
			wantFatal:      true,
			wantExitStatus: 2,
			quitKey:        tea.KeyEscape,
			wantFinalExit:  2,
		},
		// Issue #10: skipped record plus binary exclusion leaving zero
		// retained stops → record-loss fatal, exit 2.
		{
			name:           "malformed plus binary exclusion zero stops record-loss fatal 2",
			process:        app.ProcessResult{ExitCode: 0},
			records:        []string{textBegin("a.go"), `{bad json`, textMatch("a.go", "hello\n", 1, subSpec{"hello", 0, 5}), endRecord("a.go", 42), summaryRecord()},
			raw:            true,
			wantState:      app.StateNoResults,
			wantOverlay:    app.OverlayError,
			wantFatal:      true,
			wantExitStatus: 2,
			quitKey:        'q',
			wantFinalExit:  2,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var idx *searchindex.Index
			if tc.raw {
				idx = buildIndexRaw(t, "/work", tc.records...)
			} else {
				idx = buildIndex(t, "/work", tc.records...)
			}
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
