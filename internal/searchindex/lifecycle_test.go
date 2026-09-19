package searchindex_test

import (
	"fmt"
	"strings"
	"testing"

	"vrg/internal/searchindex"
)

// wantFile is the expected retained index entry for one path: its
// resolved path, whether it was retained with incomplete lifecycle
// metadata, and its stop line numbers in index order.
type wantFile struct {
	path       string
	incomplete bool
	stops      []int64
}

// wantCause is one expected structured integrity cause (Issue #36):
// the stable kind of violation plus the raw path bytes the offending
// record named where the kind's diagnostic names one.
type wantCause struct {
	kind searchindex.CauseKind
	path string
}

// lifecycleCase is one row of the Issue #9 lifecycle transition matrix:
// the records fed in order — each newline-terminated — plus an optional
// trailing unterminated fragment, and the expected stream integrity,
// ordered structured integrity causes (Issue #36), retained files, and
// the independent Issue #10 counters. Cause order is detection order
// for mid-stream violations, then the end-of-stream causes: missing
// end for each still-open file in unsigned raw-path order, then
// missing summary, then the unterminated tail.
type lifecycleCase struct {
	name           string
	recs           []string
	tail           string
	complete       bool
	causes         []wantCause
	files          []wantFile
	binary         int
	malformed      int
	unknown        int
	oversized      int
	oversizedPaths []string
}

func TestLifecycleMatrix(t *testing.T) {
	matchA := func(line int) string {
		return matchRec(text("a.txt"), text("hit\n"), line, sub(text("hit"), 0, 3))
	}
	matchB := func(line int) string {
		return matchRec(text("b.txt"), text("hit\n"), line, sub(text("hit"), 0, 3))
	}
	for _, tc := range []lifecycleCase{
		{
			name: "begin while not open opens the file",
			recs: []string{
				beginRec(text("a.txt")),
				matchA(7),
				endRec(text("a.txt")),
				summaryRec(),
			},
			complete: true,
			files:    []wantFile{{"/wd/a.txt", false, []int64{7}}},
		},
		{
			name: "duplicate begin while open is an integrity failure",
			recs: []string{
				beginRec(text("a.txt")),
				beginRec(text("a.txt")),
				matchA(7),
				endRec(text("a.txt")),
				summaryRec(),
			},
			complete: false,
			causes:   []wantCause{{searchindex.CauseDuplicateBegin, "a.txt"}},
			files:    []wantFile{{"/wd/a.txt", false, []int64{7}}},
		},
		{
			name: "match while open indexes under the file",
			recs: []string{
				beginRec(text("a.txt")),
				matchA(3),
				matchA(9),
				endRec(text("a.txt")),
				summaryRec(),
			},
			complete: true,
			files:    []wantFile{{"/wd/a.txt", false, []int64{3, 9}}},
		},
		{
			name: "match for never-opened path is retained incomplete",
			recs: []string{
				matchA(7),
				summaryRec(),
			},
			complete: false,
			causes:   []wantCause{{searchindex.CauseOrphanedMatch, "a.txt"}},
			files:    []wantFile{{"/wd/a.txt", true, []int64{7}}},
		},
		{
			name: "match after end is retained incomplete",
			recs: []string{
				beginRec(text("a.txt")),
				matchA(7),
				endRec(text("a.txt")),
				matchA(12),
				summaryRec(),
			},
			complete: false,
			causes:   []wantCause{{searchindex.CauseOrphanedMatch, "a.txt"}},
			files:    []wantFile{{"/wd/a.txt", true, []int64{7, 12}}},
		},
		{
			// Binary exclusion takes precedence over orphan retention:
			// the late match is dropped and the file stays excluded.
			name: "match after binary end is not retained",
			recs: []string{
				beginRec(text("a.txt")),
				matchA(7),
				endBinaryRec(text("a.txt"), 5),
				matchA(12),
				summaryRec(),
			},
			complete: false,
			causes:   []wantCause{{searchindex.CauseOrphanedMatch, "a.txt"}},
			files:    nil,
			binary:   1,
		},
		{
			name: "end while open closes the file",
			recs: []string{
				beginRec(text("a.txt")),
				endRec(text("a.txt")),
				summaryRec(),
			},
			complete: true,
		},
		{
			// A well-formed end carrying binary_offset is the exclusion
			// transition — exclusion is normal, not a failure.
			name: "binary end while open excludes without failing",
			recs: []string{
				beginRec(text("a.txt")),
				matchA(7),
				endBinaryRec(text("a.txt"), 5),
				summaryRec(),
			},
			complete: true,
			binary:   1,
		},
		{
			name: "end for never-opened path is orphaned",
			recs: []string{
				endRec(text("a.txt")),
				summaryRec(),
			},
			complete: false,
			causes:   []wantCause{{searchindex.CauseOrphanedEnd, "a.txt"}},
		},
		{
			name: "duplicate end is orphaned",
			recs: []string{
				beginRec(text("a.txt")),
				endRec(text("a.txt")),
				endRec(text("a.txt")),
				summaryRec(),
			},
			complete: false,
			causes:   []wantCause{{searchindex.CauseOrphanedEnd, "a.txt"}},
		},
		{
			// A well-formed binary_offset is exclusion evidence even on
			// an orphaned end: the retained orphan matches drop with
			// the file and the exclusion counts. Both records were
			// integrity failures — the orphaned match first, then the
			// orphaned end.
			name: "orphaned binary end excludes and fails",
			recs: []string{
				matchA(7),
				endBinaryRec(text("a.txt"), 5),
				summaryRec(),
			},
			complete: false,
			causes: []wantCause{
				{searchindex.CauseOrphanedMatch, "a.txt"},
				{searchindex.CauseOrphanedEnd, "a.txt"},
			},
			binary: 1,
		},
		{
			name: "context before summary is ignored",
			recs: []string{
				beginRec(text("a.txt")),
				contextRec(),
				matchA(7),
				contextRec(),
				endRec(text("a.txt")),
				summaryRec(),
			},
			complete: true,
			files:    []wantFile{{"/wd/a.txt", false, []int64{7}}},
		},
		{
			// Issue #36 removed the post-summary context exemption:
			// the summary is final, so a context after it is an
			// after-summary integrity failure like any other record
			// (Issue #44 owns dedicated context coverage).
			name: "context after summary is an integrity failure",
			recs: []string{
				beginRec(text("a.txt")),
				matchA(7),
				endRec(text("a.txt")),
				summaryRec(),
				contextRec(),
			},
			complete: false,
			causes:   []wantCause{{searchindex.CauseAfterSummary, ""}},
			files:    []wantFile{{"/wd/a.txt", false, []int64{7}}},
		},
		{
			name: "file still open at stream end fails and retains",
			recs: []string{
				beginRec(text("a.txt")),
				matchA(7),
				summaryRec(),
			},
			complete: false,
			causes:   []wantCause{{searchindex.CauseMissingEnd, "a.txt"}},
			files:    []wantFile{{"/wd/a.txt", true, []int64{7}}},
		},
		{
			name: "summary alone is a complete zero-result stream",
			recs: []string{
				summaryRec(),
			},
			complete: true,
		},
		{
			// The file's own begin/end pair is consistent; the missing
			// summary fails the stream, not the file's metadata.
			name: "missing summary fails the stream",
			recs: []string{
				beginRec(text("a.txt")),
				matchA(7),
				endRec(text("a.txt")),
			},
			complete: false,
			causes:   []wantCause{{searchindex.CauseMissingSummary, ""}},
			files:    []wantFile{{"/wd/a.txt", false, []int64{7}}},
		},
		{
			// The second summary's sole cause is the extra summary:
			// post-summary precedence does not also report a record
			// after summary for it.
			name: "second summary is an integrity failure",
			recs: []string{
				summaryRec(),
				summaryRec(),
			},
			complete: false,
			causes:   []wantCause{{searchindex.CauseExtraSummary, ""}},
		},
		{
			name: "match after summary is an integrity failure",
			recs: []string{
				beginRec(text("a.txt")),
				matchA(7),
				endRec(text("a.txt")),
				summaryRec(),
				matchB(3),
			},
			complete: false,
			causes:   []wantCause{{searchindex.CauseAfterSummary, ""}},
			// The post-summary match must not be indexed.
			files: []wantFile{{"/wd/a.txt", false, []int64{7}}},
		},
		{
			// A post-summary begin is never dispatched: Q cannot open,
			// so no end-of-stream missing end can name it — the sole
			// cause is the record's position.
			name: "begin after summary is an integrity failure",
			recs: []string{
				summaryRec(),
				beginRec(text("a.txt")),
			},
			complete: false,
			causes:   []wantCause{{searchindex.CauseAfterSummary, ""}},
		},
		{
			name: "end after summary is an integrity failure",
			recs: []string{
				summaryRec(),
				endRec(text("a.txt")),
			},
			complete: false,
			causes:   []wantCause{{searchindex.CauseAfterSummary, ""}},
		},
		{
			// Post-summary lifecycle suppression applies to every
			// record kind: neither the begin nor the match dispatches,
			// so Q stays unopened and the match is not orphaned —
			// each contributes only its position's cause.
			name: "post-summary records cannot reopen lifecycle",
			recs: []string{
				summaryRec(),
				beginRec(text("q.txt")),
				matchRec(text("q.txt"), text("hit\n"), 3, sub(text("hit"), 0, 3)),
			},
			complete: false,
			causes: []wantCause{
				{searchindex.CauseAfterSummary, ""},
				{searchindex.CauseAfterSummary, ""},
			},
		},
		{
			// Dual representation: the malformed record's position is
			// an after-summary failure and the record still counts in
			// Malformed — the integrity cause never swallows the
			// independent count.
			name: "malformed record after summary is an integrity failure",
			recs: []string{
				summaryRec(),
				`{"type":"match"`,
			},
			complete:  false,
			causes:    []wantCause{{searchindex.CauseAfterSummary, ""}},
			malformed: 1,
		},
		{
			// The same dual representation for an unknown-type record:
			// the after-summary cause plus the unrecognised-type count.
			name: "unknown record after summary is an integrity failure",
			recs: []string{
				summaryRec(),
				`{"type":"frobnicate","data":{"x":1}}`,
			},
			complete: false,
			causes:   []wantCause{{searchindex.CauseAfterSummary, ""}},
			unknown:  1,
		},
		{
			// The oversized record's position is the only integrity
			// cause; its oversized count and recovered path survive
			// as the record-loss representation.
			name: "oversized record after summary keeps its own count",
			recs: []string{
				summaryRec(),
				oversizedMatchPathFirst("q.txt", searchindex.MaxRecordBytes),
			},
			complete:       false,
			causes:         []wantCause{{searchindex.CauseAfterSummary, ""}},
			oversized:      1,
			oversizedPaths: []string{"q.txt"},
		},
		{
			// The trailing fragment after a valid summary falls under
			// post-summary precedence: its sole integrity cause is the
			// record's position — not a second unterminated-final-record
			// cause — while the missing termination still counts
			// malformed.
			name: "trailing unterminated record fails the stream",
			recs: []string{
				beginRec(text("a.txt")),
				matchA(7),
				endRec(text("a.txt")),
				summaryRec(),
			},
			tail:      matchB(3),
			complete:  false,
			causes:    []wantCause{{searchindex.CauseAfterSummary, ""}},
			malformed: 1,
			files:     []wantFile{{"/wd/a.txt", false, []int64{7}}},
		},
		{
			// Outside the post-summary state the same fragment is the
			// unterminated final record — after the missing summary,
			// in the mandated end-of-stream order — and still counts
			// malformed.
			name: "unterminated fragment as the only summary fails",
			recs: []string{
				beginRec(text("a.txt")),
				matchA(7),
				endRec(text("a.txt")),
			},
			tail:     summaryRec(),
			complete: false,
			causes: []wantCause{
				{searchindex.CauseMissingSummary, ""},
				{searchindex.CauseUnterminated, ""},
			},
			malformed: 1,
			files:     []wantFile{{"/wd/a.txt", false, []int64{7}}},
		},
		{
			name: "text and bytes forms of one path agree",
			recs: []string{
				beginRec(text("a.txt")),
				matchRec(byts("a.txt"), byts("hit\n"), 7, sub(byts("hit"), 0, 3)),
				endRec(byts("a.txt")),
				summaryRec(),
			},
			complete: true,
			files:    []wantFile{{"/wd/a.txt", false, []int64{7}}},
		},
		{
			name: "bytes begin pairs with text end",
			recs: []string{
				beginRec(byts("a.txt")),
				matchA(7),
				endRec(text("a.txt")),
				summaryRec(),
			},
			complete: true,
			files:    []wantFile{{"/wd/a.txt", false, []int64{7}}},
		},
		{
			name: "interleaved open files pair independently",
			recs: []string{
				beginRec(text("a.txt")),
				beginRec(text("b.txt")),
				matchA(7),
				matchB(3),
				endRec(text("a.txt")),
				endRec(text("b.txt")),
				summaryRec(),
			},
			complete: true,
			files: []wantFile{
				{"/wd/a.txt", false, []int64{7}},
				{"/wd/b.txt", false, []int64{3}},
			},
		},
		{
			// Retained orphan matches do not open the file: a later end
			// is still orphaned — two failures, one retained file, and
			// the causes list in detection order.
			name: "orphaned match does not open the file",
			recs: []string{
				matchA(7),
				endRec(text("a.txt")),
				summaryRec(),
			},
			complete: false,
			causes: []wantCause{
				{searchindex.CauseOrphanedMatch, "a.txt"},
				{searchindex.CauseOrphanedEnd, "a.txt"},
			},
			files: []wantFile{{"/wd/a.txt", true, []int64{7}}},
		},
		{
			name: "orphan match then begin keeps retained stops",
			recs: []string{
				matchA(2),
				beginRec(text("a.txt")),
				matchA(7),
				endRec(text("a.txt")),
				summaryRec(),
			},
			complete: false,
			causes:   []wantCause{{searchindex.CauseOrphanedMatch, "a.txt"}},
			files:    []wantFile{{"/wd/a.txt", true, []int64{2, 7}}},
		},
		{
			// Binary exclusion is terminal: the new begin duplicates a
			// path whose lifecycle already ended, and the late match
			// is the orphaned match the exclusion precedence drops.
			name: "begin cannot reopen an excluded file",
			recs: []string{
				beginRec(text("a.txt")),
				matchA(7),
				endBinaryRec(text("a.txt"), 5),
				beginRec(text("a.txt")),
				matchA(12),
				summaryRec(),
			},
			complete: false,
			causes: []wantCause{
				{searchindex.CauseDuplicateBegin, "a.txt"},
				{searchindex.CauseOrphanedMatch, "a.txt"},
			},
			binary: 1,
		},
		{
			name: "duplicate begin on excluded file stays excluded",
			recs: []string{
				beginRec(text("a.txt")),
				endBinaryRec(text("a.txt"), 5),
				beginRec(text("a.txt")),
				endRec(text("a.txt")),
				summaryRec(),
			},
			complete: false,
			causes: []wantCause{
				{searchindex.CauseDuplicateBegin, "a.txt"},
				{searchindex.CauseOrphanedEnd, "a.txt"},
			},
			binary: 1,
		},
		{
			// One cause per offending physical record, uncapped: three
			// identical orphaned matches produce three identical
			// causes — no aggregation, deduplication, or cap.
			name: "repeated identical violations produce one cause each",
			recs: []string{
				matchA(7),
				matchA(8),
				matchA(9),
				summaryRec(),
			},
			complete: false,
			causes: []wantCause{
				{searchindex.CauseOrphanedMatch, "a.txt"},
				{searchindex.CauseOrphanedMatch, "a.txt"},
				{searchindex.CauseOrphanedMatch, "a.txt"},
			},
			files: []wantFile{{"/wd/a.txt", true, []int64{7, 8, 9}}},
		},
		{
			// Still-open files at stream end report missing ends
			// ordered by unsigned raw-path bytes — never map
			// iteration or arrival order: \xff sorts after a.txt
			// even though its begin arrived first.
			name: "still-open files report missing ends in path order",
			recs: []string{
				beginRec(byts("\xff.bin")),
				beginRec(text("a.txt")),
				summaryRec(),
			},
			complete: false,
			causes: []wantCause{
				{searchindex.CauseMissingEnd, "a.txt"},
				{searchindex.CauseMissingEnd, "\xff.bin"},
			},
		},
		{
			// Mid-stream violations report in detection order, ahead
			// of every end-of-stream cause: the orphaned end precedes
			// the orphaned match, and both precede the missing end.
			name: "violations list in detection order",
			recs: []string{
				endRec(text("c.txt")),
				matchA(7),
				beginRec(text("b.txt")),
				summaryRec(),
			},
			complete: false,
			causes: []wantCause{
				{searchindex.CauseOrphanedEnd, "c.txt"},
				{searchindex.CauseOrphanedMatch, "a.txt"},
				{searchindex.CauseMissingEnd, "b.txt"},
			},
			files: []wantFile{{"/wd/a.txt", true, []int64{7}}},
		},
		{
			name:     "empty stream fails",
			recs:     nil,
			complete: false,
			causes:   []wantCause{{searchindex.CauseMissingSummary, ""}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ix := buildLifecycle(tc.recs, tc.tail)
			if got := ix.Integrity().Complete; got != tc.complete {
				t.Errorf("Integrity().Complete = %v, want %v", got, tc.complete)
			}
			checkCauses(t, ix, tc.causes)
			checkLifecycleFiles(t, ix, tc.files)
			if ix.BinaryExcluded != tc.binary {
				t.Errorf("BinaryExcluded = %d, want %d", ix.BinaryExcluded, tc.binary)
			}
			if ix.Malformed != tc.malformed {
				t.Errorf("Malformed = %d, want %d", ix.Malformed, tc.malformed)
			}
			if ix.Unknown != tc.unknown {
				t.Errorf("Unknown = %d, want %d", ix.Unknown, tc.unknown)
			}
			if ix.Oversized != tc.oversized {
				t.Errorf("Oversized = %d, want %d", ix.Oversized, tc.oversized)
			}
			if len(ix.OversizedPaths) != len(tc.oversizedPaths) {
				t.Fatalf("OversizedPaths = %q, want %d paths", ix.OversizedPaths, len(tc.oversizedPaths))
			}
			for i, p := range tc.oversizedPaths {
				if string(ix.OversizedPaths[i]) != p {
					t.Errorf("OversizedPaths[%d] = %q, want %q", i, ix.OversizedPaths[i], p)
				}
			}
			usable := 0
			for _, wf := range tc.files {
				usable += len(wf.stops)
			}
			if got := ix.UsableResults(); got != usable {
				t.Errorf("UsableResults() = %d, want %d", got, usable)
			}
		})
	}
}

// buildLifecycle feeds each record newline-terminated, then the
// unterminated tail fragment, through the same stream entry point the
// collector uses.
func buildLifecycle(recs []string, tail string) *searchindex.Index {
	var sb strings.Builder
	for _, r := range recs {
		sb.WriteString(r)
		sb.WriteByte('\n')
	}
	sb.WriteString(tail)
	return searchindex.Build([]byte(sb.String()), "/wd")
}

// checkCauses asserts the built index's ordered structured integrity
// causes — one per offending physical record, the evidence Issue #36's
// composed diagnostics report.
func checkCauses(t *testing.T, ix *searchindex.Index, want []wantCause) {
	t.Helper()
	got := ix.Integrity().Causes
	if len(got) != len(want) {
		t.Fatalf("Integrity().Causes = %#v, want %d causes", got, len(want))
	}
	for i, w := range want {
		if got[i].Kind != w.kind || string(got[i].Path) != w.path {
			t.Errorf("Causes[%d] = (%v, %q), want (%v, %q)",
				i, got[i].Kind, got[i].Path, w.kind, w.path)
		}
	}
}

func checkLifecycleFiles(t *testing.T, ix *searchindex.Index, want []wantFile) {
	t.Helper()
	if len(ix.Files) != len(want) {
		t.Fatalf("Files = %#v, want %d entries", ix.Files, len(want))
	}
	for i, wf := range want {
		f := ix.Files[i]
		if string(f.Path) != wf.path {
			t.Errorf("Files[%d].Path = %q, want %q", i, f.Path, wf.path)
		}
		if f.Incomplete != wf.incomplete {
			t.Errorf("Files[%d] %q: Incomplete = %v, want %v",
				i, f.Path, f.Incomplete, wf.incomplete)
		}
		got := make([]int64, len(f.Stops))
		for j, st := range f.Stops {
			got[j] = st.Number
		}
		if fmt.Sprint(got) != fmt.Sprint(wf.stops) {
			t.Errorf("Files[%d] %q: stop lines = %v, want %v",
				i, f.Path, got, wf.stops)
		}
	}
}

// A well-formed but unterminated trailing record carries the matrix's
// double disposition: classified malformed (Issue #10 owns the count)
// and marked as an incomplete stream.
func TestTrailingUnterminatedRecordDisposition(t *testing.T) {
	ix := searchindex.New()
	ix.Feed([]byte(beginRec(text("a.txt"))))
	ix.Feed([]byte(matchRec(text("a.txt"), text("hit\n"), 7, sub(text("hit"), 0, 3))))
	ix.Feed([]byte(endRec(text("a.txt"))))
	ix.Feed([]byte(summaryRec()))

	if got := ix.FeedTail([]byte(matchRec(text("b.txt"), text("hit\n"), 9, sub(text("hit"), 0, 3)))); got != searchindex.KindMalformed {
		t.Fatalf("FeedTail = %v, want KindMalformed", got)
	}
	ix.Prepare("/wd")
	if ix.Integrity().Complete {
		t.Fatal("unterminated trailing record must mark the stream incomplete")
	}
	// Skipped like any malformed record: its would-be match is absent.
	if len(ix.Files) != 1 || len(ix.Files[0].Stops) != 1 {
		t.Fatalf("trailing fragment must not be indexed; Files = %#v", ix.Files)
	}
}
