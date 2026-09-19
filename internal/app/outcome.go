package app

import (
	"fmt"
	"strings"

	"vrg/internal/safepresentation"
	"vrg/internal/searchindex"
)

// presentation is the base screen an outcome opens beneath any overlay.
type presentation int

const (
	presentBrowse presentation = iota
	presentNoResults
	// presentOverlayOnly is a fatal outcome with no usable results: the
	// error overlay is the whole presentation and dismissing it exits —
	// there is no underlying state to reveal.
	presentOverlayOnly
)

// OutcomeInput is everything the outcome decision needs, gathered once
// searching completes: how rg finished, whether its event stream was
// whole, how many usable results survived filtering, the record-loss
// counts, and any warning diagnostics composed by the caller.
type OutcomeInput struct {
	Result    Result
	Integrity searchindex.Integrity
	Usable    int
	// RecordLoss holds Issue #10's skipped-record counts — malformed and
	// oversized records — plus the recoverable oversized-record paths.
	RecordLoss RecordLoss
	// Warnings are nonfatal diagnostic lines composed by the caller,
	// appended after the process, stream-integrity, and record-loss
	// components.
	Warnings []string
}

// RecordLoss counts records dropped before they could be indexed:
// malformed records and records over the payload limit. Paths holds the
// raw path bytes of each oversized record whose type and data.path were
// recovered before the limit — best-effort, one entry per named record.
type RecordLoss struct {
	Malformed, Oversized int
	Paths                [][]byte
}

// lost reports whether any record was skipped: record loss makes a
// resultless complete stream fatal.
func (rl RecordLoss) lost() bool {
	return rl.Malformed+rl.Oversized > 0
}

// Outcome is the decided search outcome: the initial presentation, the
// escaped diagnostic lines the modal overlay opens with (nil when there
// is no overlay), whether dismissing the overlay exits — true only when
// there is no underlying state — and the fixed search-derived exit
// status.
type Outcome struct {
	Presentation presentation
	Overlay      []string
	DismissExits bool
	Status       int
}

// processFailed reports the child's process failure: any exit code other
// than rg's 0/1, or death by signal (Code -1 with a wait error).
func processFailed(res Result) bool {
	return res.Code < 0 || res.Code > 1
}

// DecideOutcome applies the PRD's outcome table to one finished search.
// Stream integrity and process success are assessed independently: a
// fatal condition — signal death, an exit code other than 0/1, or a
// stream-integrity failure — fixes exit 2, browsing with the error
// overlay when usable results exist and presenting the overlay alone
// when none do. A clean run exits 0 with usable results and 1 without;
// any diagnostics open a warning overlay on top. The status is decided
// once here; only ctrl+c overrides it afterwards.
func DecideOutcome(in OutcomeInput) Outcome {
	fatal := processFailed(in.Result) || !in.Integrity.Complete
	overlay := outcomeDiagnostics(in)
	switch {
	case fatal && in.Usable > 0:
		return Outcome{Presentation: presentBrowse, Overlay: overlay, Status: 2}
	case fatal:
		return Outcome{Presentation: presentOverlayOnly, Overlay: overlay, DismissExits: true, Status: 2}
	case in.RecordLoss.lost() && in.Usable == 0:
		// rg 0/1, complete stream, records skipped, nothing usable:
		// the error overlay explains the record loss and its
		// dismissal exits 2. Usable is assessed after all filtering,
		// so a skipped record plus a binary exclusion lands here
		// rather than on the no-results row.
		return Outcome{Presentation: presentOverlayOnly, Overlay: overlay, DismissExits: true, Status: 2}
	case in.Usable > 0:
		return Outcome{Presentation: presentBrowse, Overlay: overlay, Status: 0}
	default:
		return Outcome{Presentation: presentNoResults, Overlay: overlay, Status: 1}
	}
}

// outcomeDiagnostics composes the escaped lines an overlay shows, in
// the universal component order: the process component — the child's
// captured stderr, or a generated line naming the exit code or signal
// when a failed child left none — then a stream-integrity note when the
// stream was not whole, then the record-loss counts and recoverable
// oversized paths, then the caller's warnings. It returns nil when
// there is nothing to report, meaning no overlay opens.
func outcomeDiagnostics(in OutcomeInput) []string {
	var lines []string
	switch {
	case len(in.Result.Stderr) > 0:
		lines = append(lines, splitDiagnostic(in.Result.Stderr)...)
	case processFailed(in.Result):
		lines = append(lines, failedProcessLine(in.Result))
	}
	if !in.Integrity.Complete {
		lines = append(lines, "ripgrep event stream incomplete")
	}
	lines = append(lines, recordLossLines(in.RecordLoss)...)
	return append(lines, in.Warnings...)
}

// recordLossLines composes the skipped-record diagnostics: the
// malformed and oversized counts, plus one "oversized record skipped
// for <path>" line per recovered path — the only evidence of a file
// whose every record was discarded.
func recordLossLines(rl RecordLoss) []string {
	var lines []string
	if rl.Malformed > 0 {
		lines = append(lines, fmt.Sprintf("%d malformed %s skipped",
			rl.Malformed, pluralize(rl.Malformed, "record", "records")))
	}
	if rl.Oversized > 0 {
		lines = append(lines, fmt.Sprintf("%d oversized %s skipped",
			rl.Oversized, pluralize(rl.Oversized, "record", "records")))
	}
	for _, p := range rl.Paths {
		lines = append(lines, "oversized record skipped for "+safepresentation.EscapePath(p))
	}
	return lines
}

// pluralize picks the singular or plural noun for a count.
func pluralize(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// failedProcessLine is the generated diagnostic for a failed child that
// left no explanatory stderr: it names the exit code, or the wait
// error's signal.
func failedProcessLine(res Result) string {
	if res.Code >= 0 {
		return fmt.Sprintf("ripgrep exited with code %d", res.Code)
	}
	if res.Err != nil {
		return "ripgrep died: " + res.Err.Error()
	}
	return "ripgrep died before reporting an exit status"
}

// splitDiagnostic escapes raw diagnostic bytes into display lines: the
// whole text passes through the safe-presentation diagnostic escaping
// first, so control bytes can never reach the terminal, then splits on
// the preserved newline boundaries.
func splitDiagnostic(raw []byte) []string {
	escaped := safepresentation.EscapeDiagnostic(string(raw))
	return strings.Split(strings.TrimSuffix(escaped, "\n"), "\n")
}
