package searchindex

import "vrg/internal/present"

// CauseKind identifies the stable kind of one stream-integrity
// violation. Every Issue #9 lifecycle-failure row maps to exactly one
// kind; the kind — not the rendered text — is what callers compare.
type CauseKind int

const (
	// CauseDuplicateBegin is a begin(P) arriving while P is already
	// open.
	CauseDuplicateBegin CauseKind = iota
	// CauseOrphanedMatch is a match(P) while P is not open — never
	// opened, or closed by a non-binary end. The match is retained
	// with incomplete metadata.
	CauseOrphanedMatch
	// CauseMatchAfterEnd is a match(P) after a binary-excluding
	// end(P): binary exclusion takes precedence over orphan
	// retention, so the match is not retained and the kind is
	// distinct from an ordinary orphaned match.
	CauseMatchAfterEnd
	// CauseOrphanedEnd is an end(P) while P is not open — never
	// opened, or a duplicate end after close.
	CauseOrphanedEnd
	// CauseMissingEnd is a file still open when the stream ends; its
	// matches are retained with incomplete metadata.
	CauseMissingEnd
	// CauseMissingSummary is a stream that ended without a valid
	// summary.
	CauseMissingSummary
	// CauseExtraSummary is a second (or later) summary record.
	CauseExtraSummary
	// CauseRecordAfterSummary is any other record after the first
	// valid summary. Post-summary records are not lifecycle-processed:
	// a begin cannot open, a match is neither retained nor marked
	// incomplete, and context is not exempt.
	CauseRecordAfterSummary
	// CauseUnterminatedFinal is a trailing record fragment that never
	// received its newline — an end-of-stream cause emitted only when
	// after-summary precedence does not replace it.
	CauseUnterminatedFinal
)

// Cause is one structured stream-integrity violation: the stable kind
// plus the raw path bytes of the file the offending record names, where
// it names one. Path is nil for record-level causes that name no file —
// summary violations, post-summary records without a decoded path, and
// the unterminated final fragment. The slice an index returns shares
// its storage; callers must not mutate it.
type Cause struct {
	Kind CauseKind
	Path []byte
}

// Line renders the cause's stable user-facing diagnostic line, with any
// embedded path escaped through the safe-presentation path utility so
// hostile path bytes can never forge line breaks or terminal control.
func (c Cause) Line() string {
	p := present.Path(c.Path)
	switch c.Kind {
	case CauseDuplicateBegin:
		return "duplicate begin for " + p
	case CauseOrphanedMatch:
		return "orphaned match for " + p
	case CauseMatchAfterEnd:
		return "match for " + p + " arrived after a binary-excluding end"
	case CauseOrphanedEnd:
		return "orphaned end for " + p
	case CauseMissingEnd:
		return "missing end for " + p
	case CauseMissingSummary:
		return "missing summary"
	case CauseExtraSummary:
		return "extra summary record"
	case CauseRecordAfterSummary:
		return "record after summary"
	case CauseUnterminatedFinal:
		return "unterminated final record"
	default:
		return "unknown integrity failure"
	}
}
