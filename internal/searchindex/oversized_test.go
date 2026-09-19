package searchindex_test

import (
	"strings"
	"testing"

	"vrg/internal/searchindex"
)

// paddedBegin builds a begin record for path of exactly n payload
// bytes: an ignored extra data field carries the padding, so the record
// stays well-formed at any size.
func paddedBegin(path string, n int) string {
	pre := `{"type":"begin","data":{"path":{"text":"` + path + `"},"pad":"`
	suf := `"}}`
	return pre + strings.Repeat("a", n-len(pre)-len(suf)) + suf
}

// oversizedMatchPathFirst builds a match record that exceeds the
// payload limit through a giant lines value; type and data.path sit
// well before the boundary, so the path is recoverable.
func oversizedMatchPathFirst(path string, pad int) string {
	return `{"type":"match","data":{"path":{"text":"` + path + `"},"lines":{"text":"` +
		strings.Repeat("a", pad) +
		`"},"line_number":1,"submatches":[{"match":{"text":"a"},"start":0,"end":1}]}}`
}

// oversizedMatchPathLast puts the giant lines value before data.path,
// so the byte limit is hit before the path field is ever parsed.
func oversizedMatchPathLast(path string, pad int) string {
	return `{"type":"match","data":{"lines":{"text":"` +
		strings.Repeat("a", pad) + `"},"path":{"text":"` + path +
		`"},"line_number":1,"submatches":[{"match":{"text":"a"},"start":0,"end":1}]}}`
}

// The 64 MiB payload limit is exclusive of the newline: a record of
// exactly MaxRecordBytes is still parsed, and one byte over is consumed
// and discarded through its next newline while parsing resynchronizes
// on the following record.
func TestOversizedBoundary(t *testing.T) {
	at := buildLifecycle([]string{
		paddedBegin("big.txt", searchindex.MaxRecordBytes),
		endRec(text("big.txt")),
		beginRec(text("a.txt")),
		matchRec(text("a.txt"), text("hit\n"), 7, sub(text("hit"), 0, 3)),
		endRec(text("a.txt")),
		summaryRec(),
	}, "")
	checkDisposition(t, at, 0, 0, true)
	if at.Oversized != 0 {
		t.Errorf("at-limit record: Oversized = %d, want 0 — exactly %d bytes is accepted",
			at.Oversized, searchindex.MaxRecordBytes)
	}
	checkLifecycleFiles(t, at, []wantFile{{"/wd/a.txt", false, []int64{7}}})

	over := buildLifecycle([]string{
		paddedBegin("big.txt", searchindex.MaxRecordBytes+1),
		// The discarded begin never opened big.txt, so this end is
		// orphaned — proving the record was consumed through its
		// newline and the next record was parsed and dispatched.
		endRec(text("big.txt")),
		beginRec(text("a.txt")),
		matchRec(text("a.txt"), text("hit\n"), 7, sub(text("hit"), 0, 3)),
		endRec(text("a.txt")),
		summaryRec(),
	}, "")
	if over.Oversized != 1 {
		t.Errorf("over-limit record: Oversized = %d, want 1", over.Oversized)
	}
	if over.Malformed != 0 {
		t.Errorf("over-limit record: Malformed = %d, want 0 — oversized is a separate count", over.Malformed)
	}
	if over.Integrity().Complete {
		t.Error("over-limit begin must leave its end orphaned: want integrity failure")
	}
	checkLifecycleFiles(t, over, []wantFile{{"/wd/a.txt", false, []int64{7}}})
}

// When an oversized record's type and data.path were parsed before the
// limit — the usual case, since ripgrep emits them before the line
// payload — the diagnostic can name the file; when the limit is hit
// first, only the count reports the loss. A file whose only records
// were oversized never reaches the file list, so the recovered path is
// the user's only evidence it existed.
func TestOversizedRecordPathDiagnostics(t *testing.T) {
	named := buildLifecycle([]string{
		beginRec(text("big.txt")),
		oversizedMatchPathFirst("big.txt", searchindex.MaxRecordBytes),
		endRec(text("big.txt")),
		summaryRec(),
	}, "")
	if named.Oversized != 1 {
		t.Fatalf("Oversized = %d, want 1", named.Oversized)
	}
	if len(named.OversizedPaths) != 1 || string(named.OversizedPaths[0]) != "big.txt" {
		t.Fatalf("OversizedPaths = %q, want [\"big.txt\"]", named.OversizedPaths)
	}
	if len(named.Files) != 0 {
		t.Fatalf("oversized-only file must be absent from the file list; Files = %v", filePaths(named))
	}
	if !named.Integrity().Complete {
		t.Error("the paired begin/end kept lifecycle whole: want complete stream")
	}

	anon := buildLifecycle([]string{
		oversizedMatchPathLast("late.txt", searchindex.MaxRecordBytes),
		summaryRec(),
	}, "")
	if anon.Oversized != 1 {
		t.Fatalf("Oversized = %d, want 1", anon.Oversized)
	}
	if len(anon.OversizedPaths) != 0 {
		t.Fatalf("OversizedPaths = %q, want none — the limit hit before data.path", anon.OversizedPaths)
	}
}

// The PRD's trailing-unterminated rule carries no exception for
// oversized records: an oversized final record without a trailing
// newline is counted oversized AND counted malformed for its missing
// termination AND makes the stream incomplete — all three dispositions
// hold for the one record.
func TestOversizedUnterminatedTailTripleDisposition(t *testing.T) {
	ix := buildLifecycle([]string{
		beginRec(text("a.txt")),
		matchRec(text("a.txt"), text("hit\n"), 7, sub(text("hit"), 0, 3)),
		endRec(text("a.txt")),
		summaryRec(),
	}, oversizedMatchPathFirst("big.txt", searchindex.MaxRecordBytes))
	if ix.Oversized != 1 {
		t.Errorf("Oversized = %d, want 1", ix.Oversized)
	}
	if ix.Malformed != 1 {
		t.Errorf("Malformed = %d, want 1 — the missing termination counts", ix.Malformed)
	}
	if ix.Integrity().Complete {
		t.Error("unterminated oversized tail must mark the stream incomplete")
	}
	checkLifecycleFiles(t, ix, []wantFile{{"/wd/a.txt", false, []int64{7}}})
}

// An unknown-type record cannot substitute for the required known
// completion events: a stream that ends on one without a summary fails
// integrity while the record still counts as unknown, not malformed.
func TestUnknownTypeCannotSubstituteForSummary(t *testing.T) {
	ix := buildLifecycle([]string{
		beginRec(text("a.txt")),
		matchRec(text("a.txt"), text("hit\n"), 7, sub(text("hit"), 0, 3)),
		endRec(text("a.txt")),
		`{"type":"weird","data":{"x":1}}`,
	}, "")
	checkDisposition(t, ix, 0, 1, false)
	checkLifecycleFiles(t, ix, []wantFile{{"/wd/a.txt", false, []int64{7}}})
}
