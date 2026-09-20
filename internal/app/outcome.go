package app

import (
	"errors"
	"fmt"
	"strings"

	"vrg/internal/safepresentation"
	"vrg/internal/searchindex"
)

// outcomeInput is everything the outcome decision consumes. loss is the
// Issue 10 record-loss input: it is carried here so its later rows plug
// into the same matrix, but no row consumes it yet.
type outcomeInput struct {
	procErr   error                 // the child's wait status
	integrity searchindex.Integrity // stream completeness
	usable    int                   // retained matched-line stops
	loss      int                   // malformed/skipped record counts
	stderr    []byte                // captured stderr, pre-classification
}

// outcome is the fixed product of one completed search, decided once:
// the base state to present, the escaped diagnostic lines an overlay
// shows (empty when none opens), and the exit status the run reports
// unless ctrl+c overrides it. Post-dismissal presentation derives from
// state: a stateFatal overlay has no underlying state, so its dismissal
// exits; every other overlay dismisses back to its base state.
type outcome struct {
	state       state
	diagnostics []string
	status      int
}

// decideOutcome is the single outcome-transition matrix: process result,
// stream integrity, usable-result count, and the stderr diagnostic
// classification decide the initial presentation and the fixed exit
// status together. An unexpected wait status or an incomplete stream is
// fatal (exit 2, error overlay); nonfatal stderr on a successful process
// is a warning overlay over the ordinary outcome.
func decideOutcome(in outcomeInput) outcome {
	fatal := procFatal(in.procErr) || !in.integrity.Complete
	diags := diagnosticLines(in)
	switch {
	case in.usable > 0:
		if fatal {
			return outcome{state: stateBrowse, diagnostics: diags, status: 2}
		}
		return outcome{state: stateBrowse, diagnostics: diags, status: 0}
	case fatal:
		return outcome{state: stateFatal, diagnostics: diags, status: 2}
	default:
		return outcome{state: stateNoResults, diagnostics: diags, status: 1}
	}
}

// procFatal reports whether the child's wait status is a fatal process
// outcome: anything other than a clean exit or rg's no-matches exit 1 —
// another exit code, signal death, or a Wait error.
func procFatal(err error) bool {
	return err != nil && !rgSucceeded(err)
}

// diagnosticLines classifies the captured stderr into escaped overlay
// lines regardless of exit code. A fatal process that supplied no stderr
// gets a generated line naming its exit code or signal rather than an
// empty overlay, and an incomplete stream appends a note explaining the
// retained results' provenance.
func diagnosticLines(in outcomeInput) []string {
	var out []string
	if s := strings.TrimSuffix(string(in.stderr), "\n"); s != "" {
		out = strings.Split(safepresentation.EscapeDiagnostic(s), "\n")
	}
	if len(out) == 0 && procFatal(in.procErr) {
		out = append(out, processDiagnostic(in.procErr))
	}
	if !in.integrity.Complete {
		out = append(out, "the search result stream was incomplete")
	}
	return out
}

// processDiagnostic is the generated line for a fatal process outcome
// with no stderr of its own: the exit code when there is one, else the
// wait status (a signal death or Wait error), escaped to one line.
func processDiagnostic(err error) string {
	var ex interface{ ExitCode() int }
	if errors.As(err, &ex) && ex.ExitCode() >= 0 {
		return fmt.Sprintf("ripgrep exited with code %d", ex.ExitCode())
	}
	return "ripgrep terminated: " + safepresentation.EscapePath([]byte(err.Error()))
}
