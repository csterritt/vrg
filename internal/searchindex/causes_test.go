package searchindex_test

import (
	"bytes"
	"slices"
	"testing"

	"vrg/internal/searchindex"
)

// causeCase is one exact-state row of the structured integrity-cause
// contract (Issue #36): the decoded records or raw stream fed to the
// index, the complete ordered cause list the built index must carry —
// a stable kind plus the offending record's raw path bytes where it
// names one — and the independent counters that survive alongside the
// causes. An empty causes list means the stream is intact.
type causeCase struct {
	name       string
	records    []string
	stream     string
	causes     []searchindex.Cause
	malformed  int
	oversized  int
	unknown    int
	stops      int
	incomplete int
	excluded   int
}

// TestIntegrityCauses pins the structured cause list: one cause record
// per offending physical record, mid-stream violations in detection
// order, then the end-of-stream causes — missing end for each still-open
// file in unsigned raw-path order, missing summary, and the
// unterminated final record when after-summary precedence does not
// replace it. Overlaps contribute at most one integrity cause per
// record while the independent malformed, oversized, and unknown
// counters keep their own tallies.
func TestIntegrityCauses(t *testing.T) {
	cases := []causeCase{
		{
			name: "begin while already open",
			records: []string{
				beginRec(jText("a.txt")),
				beginRec(jText("a.txt")),
				matchRec(jText("a.txt"), jText("hit\n"), 1, subRec(jText("hit"), 0, 3)),
				endRec(jText("a.txt"), "null"),
				`{"type":"summary","data":{}}`,
			},
			causes: []searchindex.Cause{
				{Kind: searchindex.CauseDuplicateBegin, Path: []byte("a.txt")},
			},
			stops: 1,
		},
		{
			name: "match for a file never opened",
			records: []string{
				matchRec(jText("a.txt"), jText("hit\n"), 1, subRec(jText("hit"), 0, 3)),
				`{"type":"summary","data":{}}`,
			},
			causes: []searchindex.Cause{
				{Kind: searchindex.CauseOrphanedMatch, Path: []byte("a.txt")},
			},
			stops:      1,
			incomplete: 1,
		},
		{
			name: "match after a file's end",
			records: []string{
				beginRec(jText("a.txt")),
				matchRec(jText("a.txt"), jText("hit\n"), 1, subRec(jText("hit"), 0, 3)),
				endRec(jText("a.txt"), "null"),
				matchRec(jText("a.txt"), jText("hit\n"), 5, subRec(jText("hit"), 0, 3)),
				`{"type":"summary","data":{}}`,
			},
			causes: []searchindex.Cause{
				{Kind: searchindex.CauseOrphanedMatch, Path: []byte("a.txt")},
			},
			stops:      2,
			incomplete: 2,
		},
		{
			name: "match after a binary-excluding end",
			records: []string{
				beginRec(jText("a.bin")),
				matchRec(jText("a.bin"), jText("hit\n"), 1, subRec(jText("hit"), 0, 3)),
				endRec(jText("a.bin"), "42"),
				matchRec(jText("a.bin"), jText("hit\n"), 7, subRec(jText("hit"), 0, 3)),
				`{"type":"summary","data":{}}`,
			},
			causes: []searchindex.Cause{
				{Kind: searchindex.CauseMatchAfterEnd, Path: []byte("a.bin")},
			},
			excluded: 1,
		},
		{
			name: "end for a file never opened",
			records: []string{
				endRec(jText("a.txt"), "null"),
				`{"type":"summary","data":{}}`,
			},
			causes: []searchindex.Cause{
				{Kind: searchindex.CauseOrphanedEnd, Path: []byte("a.txt")},
			},
		},
		{
			name: "duplicate end after close",
			records: []string{
				beginRec(jText("a.txt")),
				matchRec(jText("a.txt"), jText("hit\n"), 1, subRec(jText("hit"), 0, 3)),
				endRec(jText("a.txt"), "null"),
				endRec(jText("a.txt"), "null"),
				`{"type":"summary","data":{}}`,
			},
			causes: []searchindex.Cause{
				{Kind: searchindex.CauseOrphanedEnd, Path: []byte("a.txt")},
			},
			stops: 1,
		},
		{
			name: "file still open when the stream ends",
			records: []string{
				beginRec(jText("a.txt")),
				matchRec(jText("a.txt"), jText("hit\n"), 1, subRec(jText("hit"), 0, 3)),
				`{"type":"summary","data":{}}`,
			},
			causes: []searchindex.Cause{
				{Kind: searchindex.CauseMissingEnd, Path: []byte("a.txt")},
			},
			stops:      1,
			incomplete: 1,
		},
		{
			// Still-open files report their missing ends ordered by
			// unsigned raw path bytes, never emission or map order:
			// z.txt opened first but a.txt sorts first.
			name: "two open files sort missing ends by raw path",
			records: []string{
				beginRec(jText("z.txt")),
				beginRec(jText("a.txt")),
				`{"type":"summary","data":{}}`,
			},
			causes: []searchindex.Cause{
				{Kind: searchindex.CauseMissingEnd, Path: []byte("a.txt")},
				{Kind: searchindex.CauseMissingEnd, Path: []byte("z.txt")},
			},
			incomplete: 0,
		},
		{
			name: "missing summary",
			records: []string{
				beginRec(jText("a.txt")),
				matchRec(jText("a.txt"), jText("hit\n"), 1, subRec(jText("hit"), 0, 3)),
				endRec(jText("a.txt"), "null"),
			},
			causes: []searchindex.Cause{
				{Kind: searchindex.CauseMissingSummary},
			},
			stops: 1,
		},
		{
			// A second summary contributes only the extra-summary
			// cause — never a record-after-summary cause for the same
			// physical record.
			name: "second summary contributes only extra summary",
			records: []string{
				`{"type":"summary","data":{}}`,
				`{"type":"summary","data":{}}`,
			},
			causes: []searchindex.Cause{
				{Kind: searchindex.CauseExtraSummary},
			},
		},
		{
			// A third summary repeats the violation uncapped — one
			// cause per offending record.
			name: "third summary repeats extra summary",
			records: []string{
				`{"type":"summary","data":{}}`,
				`{"type":"summary","data":{}}`,
				`{"type":"summary","data":{}}`,
			},
			causes: []searchindex.Cause{
				{Kind: searchindex.CauseExtraSummary},
				{Kind: searchindex.CauseExtraSummary},
			},
		},
		{
			// A post-summary begin cannot open its file: it is not
			// lifecycle-processed, so no missing end is owed for
			// q.txt and the sole cause carries the record's raw path.
			name: "begin after summary cannot open the file",
			records: []string{
				`{"type":"summary","data":{}}`,
				beginRec(jText("q.txt")),
			},
			causes: []searchindex.Cause{
				{Kind: searchindex.CauseRecordAfterSummary, Path: []byte("q.txt")},
			},
		},
		{
			// A post-summary match is an ordinary record after
			// summary: the sole cause, no orphaned-match cause, no
			// retention, no incomplete mark.
			name: "match after summary is not lifecycle-processed",
			records: []string{
				beginRec(jText("a.txt")),
				matchRec(jText("a.txt"), jText("hit\n"), 1, subRec(jText("hit"), 0, 3)),
				endRec(jText("a.txt"), "null"),
				`{"type":"summary","data":{}}`,
				matchRec(jText("q.txt"), jText("hit\n"), 2, subRec(jText("hit"), 0, 3)),
			},
			causes: []searchindex.Cause{
				{Kind: searchindex.CauseRecordAfterSummary, Path: []byte("q.txt")},
			},
			stops: 1,
		},
		{
			// Context records participate in no lifecycle validation
			// before the summary, but the summary-is-final rule is
			// positional: a post-summary context is a record after
			// summary carrying no path.
			name: "context after summary",
			records: []string{
				`{"type":"summary","data":{}}`,
				ctxRec(jText("a.txt")),
			},
			causes: []searchindex.Cause{
				{Kind: searchindex.CauseRecordAfterSummary},
			},
		},
		{
			// The malformed tally is independent of the cause: the
			// fragment is skipped-and-counted while contributing only
			// the after-summary cause.
			name:     "unterminated fragment after summary",
			stream:   `{"type":"summary","data":{}}` + "\n" + `{"type":"sum`,
			malformed: 1,
			causes: []searchindex.Cause{
				{Kind: searchindex.CauseRecordAfterSummary},
			},
		},
		{
			// The oversized tally is independent too: an oversized
			// record after the summary contributes only the
			// after-summary cause while the count and its recoverable
			// path detail are retained.
			name: "oversized record after summary",
			stream: `{"type":"summary","data":{}}` + "\n" +
				matchSized(recordLimit+1) + "\n",
			oversized: 1,
			causes: []searchindex.Cause{
				{Kind: searchindex.CauseRecordAfterSummary},
			},
		},
		{
			// The unknown-type tally is likewise independent: counted
			// while contributing only the after-summary cause.
			name: "unknown type after summary",
			records: []string{
				`{"type":"summary","data":{}}`,
				`{"type":"weird","data":{"x":1}}`,
			},
			unknown: 1,
			causes: []searchindex.Cause{
				{Kind: searchindex.CauseRecordAfterSummary},
			},
		},
		{
			// Outside the post-summary state the trailing fragment is
			// the unterminated final record — an end-of-stream cause
			// appended after missing ends and the missing summary.
			name: "unterminated final record mid-stream",
			stream: beginRec(jText("a.txt")) + "\n" +
				matchRec(jText("a.txt"), jText("hit\n"), 1, subRec(jText("hit"), 0, 3)) + "\n" +
				`{"type":"end"`,
			malformed: 1,
			causes: []searchindex.Cause{
				{Kind: searchindex.CauseMissingEnd, Path: []byte("a.txt")},
				{Kind: searchindex.CauseMissingSummary},
				{Kind: searchindex.CauseUnterminatedFinal},
			},
			stops:      1,
			incomplete: 1,
		},
		{
			// A stream that is only an unterminated fragment: the
			// missing summary precedes the unterminated cause.
			name:      "unterminated fragment as the whole stream",
			stream:    `{"type":"matc`,
			malformed: 1,
			causes: []searchindex.Cause{
				{Kind: searchindex.CauseMissingSummary},
				{Kind: searchindex.CauseUnterminatedFinal},
			},
		},
		{
			// Mid-stream violations record in detection order; the
			// end-of-stream missing end follows them all.
			name: "detection order then end-of-stream causes",
			records: []string{
				beginRec(jText("a.txt")),
				beginRec(jText("a.txt")), // duplicate
				endRec(jText("b.txt"), "null"), // b never opened
				`{"type":"summary","data":{}}`,
			},
			causes: []searchindex.Cause{
				{Kind: searchindex.CauseDuplicateBegin, Path: []byte("a.txt")},
				{Kind: searchindex.CauseOrphanedEnd, Path: []byte("b.txt")},
				{Kind: searchindex.CauseMissingEnd, Path: []byte("a.txt")},
			},
		},
		{
			// Repeated identical violations produce one cause each:
			// no aggregation, deduplication, or cap.
			name: "repeated orphaned matches are one cause each",
			records: []string{
				matchRec(jText("a.txt"), jText("hit\n"), 1, subRec(jText("hit"), 0, 3)),
				matchRec(jText("a.txt"), jText("hit\n"), 2, subRec(jText("hit"), 0, 3)),
				matchRec(jText("a.txt"), jText("hit\n"), 3, subRec(jText("hit"), 0, 3)),
				`{"type":"summary","data":{}}`,
			},
			causes: []searchindex.Cause{
				{Kind: searchindex.CauseOrphanedMatch, Path: []byte("a.txt")},
				{Kind: searchindex.CauseOrphanedMatch, Path: []byte("a.txt")},
				{Kind: searchindex.CauseOrphanedMatch, Path: []byte("a.txt")},
			},
			stops:      3,
			incomplete: 3,
		},
		{
			// Every overlap in one stream: a second summary is only
			// extra summary; the post-summary begin, match, and
			// context are each only record after summary and cannot
			// open or mark anything; the still-open file then reports
			// its missing end at the end-of-stream position.
			name: "post-summary records never touch lifecycle state",
			records: []string{
				beginRec(jText("a.txt")),
				matchRec(jText("a.txt"), jText("hit\n"), 1, subRec(jText("hit"), 0, 3)),
				`{"type":"summary","data":{}}`,
				`{"type":"summary","data":{}}`,
				beginRec(jText("q.txt")),
				matchRec(jText("q.txt"), jText("hit\n"), 2, subRec(jText("hit"), 0, 3)),
				ctxRec(jText("q.txt")),
			},
			causes: []searchindex.Cause{
				{Kind: searchindex.CauseExtraSummary},
				{Kind: searchindex.CauseRecordAfterSummary, Path: []byte("q.txt")},
				{Kind: searchindex.CauseRecordAfterSummary, Path: []byte("q.txt")},
				{Kind: searchindex.CauseRecordAfterSummary},
				{Kind: searchindex.CauseMissingEnd, Path: []byte("a.txt")},
			},
			stops:      1,
			incomplete: 1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ix := searchindex.New("/work")
			if tc.stream != "" {
				ix.Feed([]byte(tc.stream))
			} else {
				feed(t, ix, tc.records...)
			}
			ix.Prepare()

			causes := ix.IntegrityCauses()
			same := slices.EqualFunc(causes, tc.causes,
				func(a, b searchindex.Cause) bool {
					return a.Kind == b.Kind && bytes.Equal(a.Path, b.Path)
				})
			if !same {
				t.Fatalf("IntegrityCauses = %+v, want %+v", causes, tc.causes)
			}

			// The rendered view carries exactly one stable line per
			// cause, in the same order.
			failures := ix.IntegrityFailures()
			if len(failures) != len(causes) {
				t.Fatalf("IntegrityFailures = %v, want one line per cause %+v", failures, causes)
			}
			for i, c := range causes {
				if failures[i] != c.Line() {
					t.Fatalf("IntegrityFailures[%d] = %q, want the cause's line %q",
						i, failures[i], c.Line())
				}
			}

			if n := ix.Malformed(); n != tc.malformed {
				t.Fatalf("Malformed = %d, want %d", n, tc.malformed)
			}
			if n := ix.Oversized(); n != tc.oversized {
				t.Fatalf("Oversized = %d, want %d", n, tc.oversized)
			}
			if n := ix.Unknown(); n != tc.unknown {
				t.Fatalf("Unknown = %d, want %d", n, tc.unknown)
			}
			stops := ix.Stops()
			if len(stops) != tc.stops {
				t.Fatalf("len(Stops) = %d, want %d: %+v", len(stops), tc.stops, stops)
			}
			incomplete := 0
			for _, s := range stops {
				if s.Incomplete {
					incomplete++
				}
			}
			if incomplete != tc.incomplete {
				t.Fatalf("incomplete stops = %d, want %d", incomplete, tc.incomplete)
			}
			if n := ix.BinaryExcluded(); n != tc.excluded {
				t.Fatalf("BinaryExcluded = %d, want %d", n, tc.excluded)
			}
		})
	}
}

// The oversized record's recoverable path detail survives the
// after-summary precedence: the sole cause is record after summary
// while the recovered path stays available for the record-loss
// component — the dual representation.
func TestOversizedAfterSummaryKeepsRecoveredPath(t *testing.T) {
	ix := searchindex.New("/work")
	ix.Feed([]byte(`{"type":"summary","data":{}}` + "\n" +
		matchSized(recordLimit+1) + "\n"))
	ix.Prepare()

	causes := ix.IntegrityCauses()
	if len(causes) != 1 || causes[0].Kind != searchindex.CauseRecordAfterSummary {
		t.Fatalf("IntegrityCauses = %+v, want the sole record-after-summary cause", causes)
	}
	diags := ix.OversizedDiagnostics()
	if len(diags) != 1 || diags[0] != "oversized record skipped for big.txt" {
		t.Fatalf("OversizedDiagnostics = %v, want the recovered big.txt detail", diags)
	}
}
