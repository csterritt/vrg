package app

import (
	"errors"
	"fmt"
	"strings"

	"vrg/internal/safepresentation"
	"vrg/internal/searchindex"
)

// outcomeInput is everything the outcome decision consumes. report is
// the Issue 10 record accounting: malformed and oversized skips are
// record loss, while unknown-type skips are warnings only.
type outcomeInput struct {
	procErr   error                 // the child's wait status
	integrity searchindex.Integrity // stream completeness
	report    searchindex.Report    // skipped-record accounting
	usable    int                   // retained matched-line stops
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
// stream integrity, record loss, the usable-result count — assessed
// after all filtering, including binary exclusion — and the stderr
// diagnostic classification decide the initial presentation and the
// fixed exit status together. An unexpected wait status or an
// incomplete stream is fatal (exit 2, error overlay); skipped
// malformed/oversized records with nothing usable left are fatal record
// loss, but with usable results only warn. Nonfatal diagnostics on a
// successful process are a warning overlay over the ordinary outcome.
func decideOutcome(in outcomeInput) outcome {
	fatal := procFatal(in.procErr) || !in.integrity.Complete
	loss := in.report.Malformed + in.report.Oversized
	diags := diagnosticLines(in)
	switch {
	case in.usable > 0:
		if fatal {
			return outcome{state: stateBrowse, diagnostics: diags, status: 2}
		}
		return outcome{state: stateBrowse, diagnostics: diags, status: 0}
	case fatal || loss > 0:
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
// empty overlay. The record accounting follows: the malformed and
// oversized aggregate counts, a line per recoverable oversized path
// (escaped to a single line each), and the unknown-type warning —
// unknown types warn but are not record loss. An incomplete stream
// closes the list with a note explaining the retained results'
// provenance.
func diagnosticLines(in outcomeInput) []string {
	var out []string
	if s := strings.TrimSuffix(string(in.stderr), "\n"); s != "" {
		out = strings.Split(safepresentation.EscapeDiagnostic(s), "\n")
	}
	if len(out) == 0 && procFatal(in.procErr) {
		out = append(out, processDiagnostic(in.procErr))
	}
	out = append(out, recordLossLines(in.report)...)
	if !in.integrity.Complete {
		out = append(out, "the search result stream was incomplete")
	}
	return out
}

// recordLossLines composes the record-accounting diagnostics: aggregate
// skip counts for malformed and oversized records, then each recovered
// oversized path named on its own escaped line, then the unknown-type
// count.
func recordLossLines(r searchindex.Report) []string {
	var out []string
	if r.Malformed > 0 {
		out = append(out, skipCount(r.Malformed, "malformed"))
	}
	if r.Oversized > 0 {
		out = append(out, skipCount(r.Oversized, "oversized"))
	}
	for _, p := range r.OversizedPaths {
		out = append(out, "oversized record skipped for "+safepresentation.EscapePath(p))
	}
	if r.UnknownTypes > 0 {
		out = append(out, fmt.Sprintf("%d unrecognised record types skipped", r.UnknownTypes))
	}
	return out
}

// skipCount is the aggregate skip-count line for one record class:
// "N malformed record(s) skipped", pluralized on the count.
func skipCount(n int, class string) string {
	s := ""
	if n != 1 {
		s = "s"
	}
	return fmt.Sprintf("%d %s record%s skipped", n, class, s)
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
