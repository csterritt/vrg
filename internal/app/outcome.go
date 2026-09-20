package app

import (
	"errors"
	"fmt"

	"vrg/internal/safepresentation"
	"vrg/internal/searchindex"
)

// outcomeInput is everything the outcome decision consumes. integrity
// carries the stream's structured cause list — one record per offending
// physical record — alongside the completeness verdict. report is the
// Issue 10 record accounting: malformed and oversized skips are record
// loss, while unknown-type skips are warnings only.
type outcomeInput struct {
	procErr   error                 // the child's wait status
	integrity searchindex.Integrity // stream completeness and its causes
	report    searchindex.Report    // skipped-record accounting
	usable    int                   // retained matched-line stops
	stderr    []string              // collected stderr diagnostics, already escaped
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

// diagnosticLines composes the escaped diagnostic lines in the
// universal component order every branch shares: the process component
// — the collected stderr lines regardless of exit code, or only for a
// failed process that supplied none, a generated line naming its exit
// code or signal; never a process-status line for a 0/1 exit — then
// one line per structured stream-integrity cause in its order, then the
// record-loss diagnostics, then the unknown-type warnings. The same
// list is the overlay's text and, minus the already-collected stderr
// prefix, joins the session collection replayed to stderr.
func diagnosticLines(in outcomeInput) []string {
	out := append([]string(nil), in.stderr...)
	if len(out) == 0 && procFatal(in.procErr) {
		out = append(out, processDiagnostic(in.procErr))
	}
	for _, c := range in.integrity.Causes {
		out = append(out, integrityLine(c))
	}
	out = append(out, recordLossLines(in.report)...)
	out = append(out, unknownTypeLines(in.report)...)
	return out
}

// integrityLine renders one structured integrity cause as its stable
// user-facing line, escaping the raw path it names to a single line.
func integrityLine(c searchindex.IntegrityCause) string {
	switch c.Kind {
	case searchindex.CauseDuplicateBegin:
		return "duplicate begin for " + safepresentation.EscapePath(c.Path)
	case searchindex.CauseOrphanedMatch:
		return "orphaned match for " + safepresentation.EscapePath(c.Path)
	case searchindex.CauseOrphanedEnd:
		return "orphaned end for " + safepresentation.EscapePath(c.Path)
	case searchindex.CauseMissingEnd:
		return "missing end for " + safepresentation.EscapePath(c.Path)
	case searchindex.CauseMissingSummary:
		return "missing summary record"
	case searchindex.CauseExtraSummary:
		return "extra summary record"
	case searchindex.CauseAfterSummary:
		return "record after summary"
	case searchindex.CauseUnterminated:
		return "unterminated final record"
	default:
		return "stream integrity violation"
	}
}

// recordLossLines composes the record-loss diagnostics in Issue 37's
// order: the malformed aggregate, the oversized aggregate — emitted on
// the count alone so an oversized record with no recoverable path still
// surfaces — then one escaped line per distinct recovered oversized
// path in first-occurrence order. The detail lines deduplicate by raw
// path; the aggregate never does. Unknown types warn but are not record
// loss — unknownTypeLines emits them — so the warning can never sit
// between the malformed and oversized components.
func recordLossLines(r searchindex.Report) []string {
	var out []string
	if r.Malformed > 0 {
		out = append(out, skipCount(r.Malformed, "malformed"))
	}
	if r.Oversized > 0 {
		out = append(out, skipCount(r.Oversized, "oversized"))
	}
	seen := make(map[string]struct{}, len(r.OversizedPaths))
	for _, p := range r.OversizedPaths {
		key := string(p)
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, "oversized record skipped for "+safepresentation.EscapePath(p))
	}
	return out
}

// unknownTypeLines composes the unknown-type warning — the last
// component of every composition.
func unknownTypeLines(r searchindex.Report) []string {
	if r.UnknownTypes == 0 {
		return nil
	}
	return []string{fmt.Sprintf("%d unrecognised record types skipped", r.UnknownTypes)}
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
