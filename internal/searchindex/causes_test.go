package searchindex_test

import (
	"testing"

	"vrg/internal/searchindex"
)

// causeInfo is the asserted form of one structured integrity cause:
// its stable kind and the raw path it names where the offending record
// names one.
type causeInfo struct {
	kind searchindex.CauseKind
	path string
}

// wantCauses asserts the integrity result's ordered structured cause
// list — exactly one cause per offending physical record, mid-stream
// violations in detection order and end-of-stream causes in their
// mandated order.
func wantCauses(t *testing.T, integrity searchindex.Integrity, want []causeInfo) {
	t.Helper()
	got := integrity.Causes
	if len(got) != len(want) {
		t.Fatalf("Integrity.Causes = %+v, want %+v", got, want)
	}
	for i, w := range want {
		g := got[i]
		if g.Kind != w.kind || string(g.Path) != w.path {
			t.Fatalf("cause %d = {kind %d, path %q}, want {kind %d, path %q}",
				i, g.Kind, g.Path, w.kind, w.path)
		}
	}
}

// wantReport asserts the full record accounting alongside a cause list:
// integrity violations and the independent skip counters coexist under
// dual representation, neither discarding the other.
func wantReport(t *testing.T, rep searchindex.Report, malformed, oversized, unknown int, oversizedPaths ...string) {
	t.Helper()
	if rep.Malformed != malformed || rep.Oversized != oversized || rep.UnknownTypes != unknown {
		t.Fatalf("Report() = %+v, want malformed=%d oversized=%d unknown=%d",
			rep, malformed, oversized, unknown)
	}
	if len(rep.OversizedPaths) != len(oversizedPaths) {
		t.Fatalf("Report().OversizedPaths = %q, want %q", rep.OversizedPaths, oversizedPaths)
	}
	for i, p := range oversizedPaths {
		if string(rep.OversizedPaths[i]) != p {
			t.Fatalf("Report().OversizedPaths[%d] = %q, want %q", i, rep.OversizedPaths[i], p)
		}
	}
}

// Every Issue 9 lifecycle-failure row produces its structured integrity
// cause — a stable kind plus the raw path where the record names one —
// in the order the violations are detected, followed by the
// end-of-stream causes. The overlap-precedence rows pin one cause per
// physical record: a second summary is only an extra summary, and any
// other record after the summary is only a record after summary that no
// lifecycle parser ever sees.
func TestIntegrityCauseMatrix(t *testing.T) {
	a, b, q := textData("a.txt"), textData("b.txt"), textData("q.txt")
	high := bytesData([]byte{0xff, 'z'}) // 0xff orders after ASCII under unsigned byte order
	matchA1 := matchEvent(t, a, textData("hit\n"), 1, submatch(textData("hit"), 0, 3))
	matchA2 := matchEvent(t, a, textData("hit\n"), 2, submatch(textData("hit"), 0, 3))
	matchA3 := matchEvent(t, a, textData("hit\n"), 3, submatch(textData("hit"), 0, 3))
	matchQ1 := matchEvent(t, q, textData("hit\n"), 1, submatch(textData("hit"), 0, 3))

	cases := []struct {
		name    string
		records [][]byte
		// errAt, when >= 0, is the index of the one record Add must
		// reject as malformed; every other record must be accepted.
		errAt int
		want  []causeInfo
		// The independent counters asserted alongside the causes.
		malformed, oversized, unknown int
	}{
		{
			"valid stream has no causes",
			[][]byte{beginEvent(t, a), matchA1, endEvent(t, a, nil), summaryEvent(t)},
			-1, nil, 0, 0, 0,
		},
		{
			"begin while already open is a duplicate begin",
			[][]byte{beginEvent(t, a), beginEvent(t, a), matchA1, endEvent(t, a, nil), summaryEvent(t)},
			-1, []causeInfo{{searchindex.CauseDuplicateBegin, "a.txt"}}, 0, 0, 0,
		},
		{
			"match while never opened is an orphaned match",
			[][]byte{matchA1, summaryEvent(t)},
			-1, []causeInfo{{searchindex.CauseOrphanedMatch, "a.txt"}}, 0, 0, 0,
		},
		{
			"match after its end is an orphaned match",
			[][]byte{beginEvent(t, a), matchA1, endEvent(t, a, nil), matchA2, summaryEvent(t)},
			-1, []causeInfo{{searchindex.CauseOrphanedMatch, "a.txt"}}, 0, 0, 0,
		},
		{
			"match after a binary-excluding end is an orphaned match",
			[][]byte{beginEvent(t, a), matchA1, endEvent(t, a, 7), matchA2, summaryEvent(t)},
			-1, []causeInfo{{searchindex.CauseOrphanedMatch, "a.txt"}}, 0, 0, 0,
		},
		{
			"end while never opened is an orphaned end",
			[][]byte{endEvent(t, a, nil), summaryEvent(t)},
			-1, []causeInfo{{searchindex.CauseOrphanedEnd, "a.txt"}}, 0, 0, 0,
		},
		{
			"end after its end is an orphaned end",
			[][]byte{beginEvent(t, a), endEvent(t, a, nil), endEvent(t, a, nil), summaryEvent(t)},
			-1, []causeInfo{{searchindex.CauseOrphanedEnd, "a.txt"}}, 0, 0, 0,
		},
		{
			"begin/end path disagreement orphans the end then misses the open file's end",
			[][]byte{beginEvent(t, a), matchA1, endEvent(t, b, nil), summaryEvent(t)},
			-1, []causeInfo{
				{searchindex.CauseOrphanedEnd, "b.txt"},
				{searchindex.CauseMissingEnd, "a.txt"},
			}, 0, 0, 0,
		},
		{
			"file still open when the stream ends is a missing end",
			[][]byte{beginEvent(t, a), matchA1, summaryEvent(t)},
			-1, []causeInfo{{searchindex.CauseMissingEnd, "a.txt"}}, 0, 0, 0,
		},
		{
			"still-open files order by unsigned raw path bytes",
			[][]byte{beginEvent(t, high), beginEvent(t, textData("z.txt")), beginEvent(t, a), summaryEvent(t)},
			-1, []causeInfo{
				{searchindex.CauseMissingEnd, "a.txt"},
				{searchindex.CauseMissingEnd, "z.txt"},
				{searchindex.CauseMissingEnd, string([]byte{0xff, 'z'})},
			}, 0, 0, 0,
		},
		{
			"missing summary",
			[][]byte{beginEvent(t, a), matchA1, endEvent(t, a, nil)},
			-1, []causeInfo{{searchindex.CauseMissingSummary, ""}}, 0, 0, 0,
		},
		{
			"empty stream misses its summary",
			nil,
			-1, []causeInfo{{searchindex.CauseMissingSummary, ""}}, 0, 0, 0,
		},
		{
			"a second summary is only an extra summary",
			[][]byte{summaryEvent(t), summaryEvent(t)},
			-1, []causeInfo{{searchindex.CauseExtraSummary, ""}}, 0, 0, 0,
		},
		{
			"match after summary is only a record after summary",
			[][]byte{beginEvent(t, a), matchA1, endEvent(t, a, nil), summaryEvent(t), matchA2},
			-1, []causeInfo{{searchindex.CauseAfterSummary, ""}}, 0, 0, 0,
		},
		{
			"begin after summary cannot open the file or later miss its end",
			[][]byte{summaryEvent(t), beginEvent(t, q)},
			-1, []causeInfo{{searchindex.CauseAfterSummary, ""}}, 0, 0, 0,
		},
		{
			"context after summary is a record after summary",
			[][]byte{beginEvent(t, a), matchA1, endEvent(t, a, nil), summaryEvent(t), contextEvent(t, a)},
			-1, []causeInfo{{searchindex.CauseAfterSummary, ""}}, 0, 0, 0,
		},
		{
			"end after summary is not lifecycle-processed: the open file still misses its end",
			[][]byte{beginEvent(t, a), summaryEvent(t), endEvent(t, a, nil)},
			-1, []causeInfo{
				{searchindex.CauseAfterSummary, ""},
				{searchindex.CauseMissingEnd, "a.txt"},
			}, 0, 0, 0,
		},
		{
			"unknown type after summary keeps its unknown count",
			[][]byte{summaryEvent(t), unknownEvent(t, "weird")},
			-1, []causeInfo{{searchindex.CauseAfterSummary, ""}}, 0, 0, 1,
		},
		{
			"malformed record after summary keeps its malformed count",
			[][]byte{beginEvent(t, a), matchA1, endEvent(t, a, nil), summaryEvent(t), []byte("{not json")},
			4, []causeInfo{{searchindex.CauseAfterSummary, ""}}, 1, 0, 0,
		},
		{
			"repeated identical violations produce one cause each",
			[][]byte{matchA1, matchA2, matchA3, summaryEvent(t)},
			-1, []causeInfo{
				{searchindex.CauseOrphanedMatch, "a.txt"},
				{searchindex.CauseOrphanedMatch, "a.txt"},
				{searchindex.CauseOrphanedMatch, "a.txt"},
			}, 0, 0, 0,
		},
		{
			"mid-stream causes precede end-of-stream causes",
			[][]byte{beginEvent(t, a), beginEvent(t, a), endEvent(t, b, nil), summaryEvent(t)},
			-1, []causeInfo{
				{searchindex.CauseDuplicateBegin, "a.txt"},
				{searchindex.CauseOrphanedEnd, "b.txt"},
				{searchindex.CauseMissingEnd, "a.txt"},
			}, 0, 0, 0,
		},
		{
			"post-summary records are not lifecycle-processed and produce no later causes",
			[][]byte{summaryEvent(t), beginEvent(t, q), matchQ1},
			-1, []causeInfo{
				{searchindex.CauseAfterSummary, ""},
				{searchindex.CauseAfterSummary, ""},
			}, 0, 0, 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, integrity, rep, errs := feed(t, "/w", tc.records...)
			for i, err := range errs {
				if i == tc.errAt {
					if err == nil {
						t.Fatalf("Add(record %d) = nil, want the malformed record rejected", i)
					}
					continue
				}
				if err != nil {
					t.Fatalf("Add(record %d) rejected a valid record: %v\n%s", i, err, tc.records[i])
				}
			}
			wantCauses(t, integrity, tc.want)
			wantReport(t, rep, tc.malformed, tc.oversized, tc.unknown)
			if integrity.Complete != (len(tc.want) == 0) {
				t.Fatalf("Integrity.Complete = %v, want %v", integrity.Complete, len(tc.want) == 0)
			}
		})
	}
}

// End-of-stream and stream-consumed dispositions: the trailing
// unterminated fragment and the oversized record produce their causes
// under the same precedence — after summary they are only records after
// summary retaining their independent counts, while before summary the
// unterminated fragment is the final end-of-stream cause, after missing
// ends and the missing summary.
func TestIntegrityCauseStreamEnds(t *testing.T) {
	a := textData("a.txt")
	live := streamOf(
		string(beginEvent(t, a)),
		string(matchEvent(t, a, textData("hit\n"), 1, submatch(textData("hit"), 0, 3))),
	)
	closed := live + streamOf(
		string(endEvent(t, a, nil)),
		string(summaryEvent(t)),
	)
	// A well-formed match record that simply lacks its newline: still
	// an unterminated fragment, never dispatched.
	frag := string(matchEvent(t, textData("b.txt"), textData("hit\n"), 9, submatch(textData("hit"), 0, 3)))
	big := sizedMatch(t, recordPayloadLimit+1, "big.txt", 9)

	cases := []struct {
		name          string
		stream        string
		want          []causeInfo
		malformed     int
		oversized     int
		oversizedPath []string
	}{
		{
			"trailing unterminated record ends the cause list",
			live + frag,
			[]causeInfo{
				{searchindex.CauseMissingEnd, "a.txt"},
				{searchindex.CauseMissingSummary, ""},
				{searchindex.CauseUnterminated, ""},
			},
			1, 0, nil,
		},
		{
			"unterminated fragment after summary is only a record after summary",
			closed + frag,
			[]causeInfo{{searchindex.CauseAfterSummary, ""}},
			1, 0, nil,
		},
		{
			"oversized record after summary keeps its oversized count and recovered path",
			streamOf(string(summaryEvent(t))) + streamOf(big),
			[]causeInfo{{searchindex.CauseAfterSummary, ""}},
			0, 1, []string{"big.txt"},
		},
		{
			"oversized unterminated tail after summary is only a record after summary",
			streamOf(string(summaryEvent(t))) + big,
			[]causeInfo{{searchindex.CauseAfterSummary, ""}},
			1, 1, []string{"big.txt"},
		},
		{
			"oversized unterminated tail before summary carries all three dispositions",
			streamOf(string(beginEvent(t, a))) + big,
			[]causeInfo{
				{searchindex.CauseMissingEnd, "a.txt"},
				{searchindex.CauseMissingSummary, ""},
				{searchindex.CauseUnterminated, ""},
			},
			1, 1, []string{"big.txt"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			idx, integrity, rep := consume(t, "/w", tc.stream)
			wantCauses(t, integrity, tc.want)
			wantReport(t, rep, tc.malformed, tc.oversized, 0, tc.oversizedPath...)
			if integrity.Complete {
				t.Fatal("a violating stream reported complete")
			}
			for _, s := range idx.Stops() {
				if string(s.Path) == "b.txt" || string(s.Path) == "big.txt" {
					t.Fatalf("the undispatched record was indexed: %+v", s)
				}
			}
		})
	}
}

// Cause order is deterministic across repeated consumption of the same
// violating stream — the end-of-stream missing ends come in unsigned
// raw-path order, never map iteration order.
func TestIntegrityCausesDeterministic(t *testing.T) {
	stream := streamOf(
		string(beginEvent(t, textData("b.txt"))),
		string(beginEvent(t, textData("a.txt"))),
		string(summaryEvent(t)),
	)
	want := []causeInfo{
		{searchindex.CauseMissingEnd, "a.txt"},
		{searchindex.CauseMissingEnd, "b.txt"},
	}
	for i := 0; i < 20; i++ {
		_, integrity, _ := consume(t, "/w", stream)
		wantCauses(t, integrity, want)
	}
}
