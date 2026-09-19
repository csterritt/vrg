package searchindex_test

import (
	"testing"

	"vrg/internal/searchindex"
)

// wantA79 is the retained index for the embedded-stream lifecycle: the
// candidate record sits between two valid match records, so asserting
// both stops proves a skipped record resynchronizes — the remaining
// records are still indexed under an intact lifecycle.
var wantA79 = []wantFile{{"/wd/a.txt", false, []int64{7, 9}}}

// embeddedStream wraps one candidate record inside a valid lifecycle
// for a.txt — begin, match@7, the candidate, match@9, end, summary — so
// the per-record schema matrix rows isolate the record's own
// disposition: skipped-and-counted versus indexed, with surrounding
// lifecycle metadata intact.
func embeddedStream(candidate string) []string {
	return []string{
		beginRec(text("a.txt")),
		matchRec(text("a.txt"), text("hit\n"), 7, sub(text("hit"), 0, 3)),
		candidate,
		matchRec(text("a.txt"), text("hit\n"), 9, sub(text("hit"), 0, 3)),
		endRec(text("a.txt")),
		summaryRec(),
	}
}

// checkDisposition asserts the Issue #10 counters and the stream's
// integrity result: the malformed and unknown skip counts, and whether
// the lifecycle matrix holds the stream complete.
func checkDisposition(t *testing.T, ix *searchindex.Index, malformed, unknown int, complete bool) {
	t.Helper()
	if ix.Malformed != malformed {
		t.Errorf("Malformed = %d, want %d", ix.Malformed, malformed)
	}
	if ix.Unknown != unknown {
		t.Errorf("Unknown = %d, want %d", ix.Unknown, unknown)
	}
	if got := ix.Integrity().Complete; got != complete {
		t.Errorf("Integrity().Complete = %v, want %v", got, complete)
	}
}

// TestSchemaMatrixDispositions covers every row of the Issue #3
// per-record schema matrix: each missing or wrongly typed required
// field and each invalid range is skipped-and-counted malformed, while
// the lifecycle-neutral rows — a context record needs only its type,
// and an unrecognized string type is unknown rather than malformed —
// keep their own dispositions. Every candidate sits between two valid
// matches, so the retained pair of stops also proves indexing
// resynchronizes after a skip.
func TestSchemaMatrixDispositions(t *testing.T) {
	for _, tc := range []struct {
		name      string
		rec       string
		malformed int
		unknown   int
	}{
		{"invalid JSON", `this is not json`, 1, 0},
		{"truncated JSON record", `{"type":"begin","data":{"path":`, 1, 0},
		{"bare JSON scalar", `5`, 1, 0},
		{"missing type", `{"data":{}}`, 1, 0},
		{"non-string type", `{"type":5,"data":{}}`, 1, 0},
		{"object type", `{"type":{"x":1},"data":{}}`, 1, 0},

		{"begin missing data", `{"type":"begin"}`, 1, 0},
		{"begin data not an object", `{"type":"begin","data":5}`, 1, 0},
		{"begin missing path", `{"type":"begin","data":{}}`, 1, 0},
		{"begin path not a text/bytes value", `{"type":"begin","data":{"path":"a.txt"}}`, 1, 0},
		{"begin path with both text and bytes", `{"type":"begin","data":{"path":{"text":"a.txt","bytes":"YS50eHQ="}}}`, 1, 0},
		{"begin path with neither text nor bytes", `{"type":"begin","data":{"path":{"raw":"a.txt"}}}`, 1, 0},
		{"begin path text not a string", `{"type":"begin","data":{"path":{"text":5}}}`, 1, 0},
		{"begin path invalid base64", `{"type":"begin","data":{"path":{"bytes":"!!!"}}}`, 1, 0},

		{"match missing path", `{"type":"match","data":{"lines":{"text":"x\n"},"line_number":1,"submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}`, 1, 0},
		{"match path invalid base64", `{"type":"match","data":{"path":{"bytes":"!!!"},"lines":{"text":"x\n"},"line_number":1,"submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}`, 1, 0},
		{"match missing lines", `{"type":"match","data":{"path":{"text":"f"},"line_number":1,"submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}`, 1, 0},
		{"match lines invalid base64", `{"type":"match","data":{"path":{"text":"f"},"lines":{"bytes":"!!!"},"line_number":1,"submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}`, 1, 0},
		{"match lines not a text/bytes value", `{"type":"match","data":{"path":{"text":"f"},"lines":"x\n","line_number":1,"submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}`, 1, 0},
		{"match missing line_number", `{"type":"match","data":{"path":{"text":"f"},"lines":{"text":"x\n"},"submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}`, 1, 0},
		{"match line_number zero", `{"type":"match","data":{"path":{"text":"f"},"lines":{"text":"x\n"},"line_number":0,"submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}`, 1, 0},
		{"match line_number negative", `{"type":"match","data":{"path":{"text":"f"},"lines":{"text":"x\n"},"line_number":-3,"submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}`, 1, 0},
		{"match line_number non-integer", `{"type":"match","data":{"path":{"text":"f"},"lines":{"text":"x\n"},"line_number":1.5,"submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}`, 1, 0},
		{"match line_number string", `{"type":"match","data":{"path":{"text":"f"},"lines":{"text":"x\n"},"line_number":"7","submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}`, 1, 0},
		{"match line_number overflows int64", `{"type":"match","data":{"path":{"text":"f"},"lines":{"text":"x\n"},"line_number":99999999999999999999,"submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}`, 1, 0},
		{"match missing submatches", `{"type":"match","data":{"path":{"text":"f"},"lines":{"text":"x\n"},"line_number":1}}`, 1, 0},
		{"match empty submatches", `{"type":"match","data":{"path":{"text":"f"},"lines":{"text":"x\n"},"line_number":1,"submatches":[]}}`, 1, 0},
		{"match submatches not an array", `{"type":"match","data":{"path":{"text":"f"},"lines":{"text":"x\n"},"line_number":1,"submatches":{}}}`, 1, 0},
		{"match submatch not an object", `{"type":"match","data":{"path":{"text":"f"},"lines":{"text":"x\n"},"line_number":1,"submatches":[5]}}`, 1, 0},
		{"match submatch missing match", `{"type":"match","data":{"path":{"text":"f"},"lines":{"text":"x\n"},"line_number":1,"submatches":[{"start":0,"end":1}]}}`, 1, 0},
		{"match submatch missing start", `{"type":"match","data":{"path":{"text":"f"},"lines":{"text":"x\n"},"line_number":1,"submatches":[{"match":{"text":"x"},"end":1}]}}`, 1, 0},
		{"match submatch missing end", `{"type":"match","data":{"path":{"text":"f"},"lines":{"text":"x\n"},"line_number":1,"submatches":[{"match":{"text":"x"},"start":0}]}}`, 1, 0},
		{"match submatch match invalid base64", `{"type":"match","data":{"path":{"text":"f"},"lines":{"text":"x\n"},"line_number":1,"submatches":[{"match":{"bytes":"!!!"},"start":0,"end":1}]}}`, 1, 0},
		{"match submatch start non-integer", `{"type":"match","data":{"path":{"text":"f"},"lines":{"text":"x\n"},"line_number":1,"submatches":[{"match":{"text":"x"},"start":0.5,"end":1}]}}`, 1, 0},
		{"match submatch end non-integer", `{"type":"match","data":{"path":{"text":"f"},"lines":{"text":"x\n"},"line_number":1,"submatches":[{"match":{"text":"x"},"start":0,"end":"1"}]}}`, 1, 0},
		{"match submatch start greater than end", `{"type":"match","data":{"path":{"text":"f"},"lines":{"text":"x\n"},"line_number":1,"submatches":[{"match":{"text":"x"},"start":1,"end":0}]}}`, 1, 0},
		{"match submatch start negative", `{"type":"match","data":{"path":{"text":"f"},"lines":{"text":"x\n"},"line_number":1,"submatches":[{"match":{"text":"x"},"start":-1,"end":1}]}}`, 1, 0},
		{"match submatch end beyond decoded line", `{"type":"match","data":{"path":{"text":"f"},"lines":{"text":"x\n"},"line_number":1,"submatches":[{"match":{"text":"x"},"start":0,"end":3}]}}`, 1, 0},

		{"end missing path", `{"type":"end","data":{"binary_offset":null}}`, 1, 0},
		{"end missing binary_offset", `{"type":"end","data":{"path":{"text":"f"},"stats":{}}}`, 1, 0},
		{"end binary_offset string", `{"type":"end","data":{"path":{"text":"f"},"binary_offset":"none","stats":{}}}`, 1, 0},
		{"end binary_offset non-integer", `{"type":"end","data":{"path":{"text":"f"},"binary_offset":1.5,"stats":{}}}`, 1, 0},
		{"end binary_offset negative", `{"type":"end","data":{"path":{"text":"f"},"binary_offset":-2,"stats":{}}}`, 1, 0},

		{"summary missing data", `{"type":"summary"}`, 1, 0},
		{"summary data not an object", `{"type":"summary","data":5}`, 1, 0},
		{"summary data null", `{"type":"summary","data":null}`, 1, 0},

		// context's entire data payload is ignored: the record needs
		// only its type, so neither shape is malformed.
		{"context without data is valid", `{"type":"context"}`, 0, 0},
		{"context non-object data is valid", `{"type":"context","data":5}`, 0, 0},

		// A string type outside the five known events is the separate
		// unknown count, never the malformed count.
		{"unknown event type", `{"type":"weird","data":{"x":1}}`, 0, 1},
		{"unknown event type without data", `{"type":"stats"}`, 0, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ix := buildLifecycle(embeddedStream(tc.rec), "")
			checkDisposition(t, ix, tc.malformed, tc.unknown, true)
			checkLifecycleFiles(t, ix, wantA79)
		})
	}
}

// TestLifecycleMatrixDispositions covers the integrity rows of the
// Issue #9 lifecycle matrix: a well-formed record violating the
// transition rules is a stream-integrity failure and never inflates the
// malformed count — the two categories are deterministic, not
// implementation judgment. The unknown-type row keeps its own count
// while its after-summary position fails integrity.
func TestLifecycleMatrixDispositions(t *testing.T) {
	matchA := func(line int) string {
		return matchRec(text("a.txt"), text("hit\n"), line, sub(text("hit"), 0, 3))
	}
	for _, tc := range []struct {
		name      string
		recs      []string
		malformed int
		unknown   int
		files     []wantFile
	}{
		{
			name: "duplicate begin is integrity only",
			recs: []string{
				beginRec(text("a.txt")), beginRec(text("a.txt")),
				matchA(7), endRec(text("a.txt")), summaryRec(),
			},
			files: []wantFile{{"/wd/a.txt", false, []int64{7}}},
		},
		{
			name:  "orphaned match is integrity only",
			recs:  []string{matchA(7), summaryRec()},
			files: []wantFile{{"/wd/a.txt", true, []int64{7}}},
		},
		{
			name: "match after end is integrity only",
			recs: []string{
				beginRec(text("a.txt")), matchA(7), endRec(text("a.txt")),
				matchA(12), summaryRec(),
			},
			files: []wantFile{{"/wd/a.txt", true, []int64{7, 12}}},
		},
		{
			name: "orphaned end is integrity only",
			recs: []string{endRec(text("a.txt")), summaryRec()},
		},
		{
			name: "duplicate end is integrity only",
			recs: []string{
				beginRec(text("a.txt")), endRec(text("a.txt")),
				endRec(text("a.txt")), summaryRec(),
			},
		},
		{
			name: "second summary is integrity only",
			recs: []string{summaryRec(), summaryRec()},
		},
		{
			name: "match after summary is integrity only",
			recs: []string{
				beginRec(text("a.txt")), matchA(7), endRec(text("a.txt")),
				summaryRec(), matchA(12),
			},
			files: []wantFile{{"/wd/a.txt", false, []int64{7}}},
		},
		{
			name: "begin after summary is integrity only",
			recs: []string{summaryRec(), beginRec(text("a.txt"))},
		},
		{
			name: "end after summary is integrity only",
			recs: []string{summaryRec(), endRec(text("a.txt"))},
		},
		{
			// The unknown-type count is unqualified by position; the
			// after-summary position is separately an integrity
			// failure — the record is counted unknown, not malformed.
			name:    "unknown type after summary counts unknown and fails",
			recs:    []string{summaryRec(), `{"type":"weird","data":{"x":1}}`},
			unknown: 1,
		},
		{
			name: "missing end fails and retains",
			recs: []string{
				beginRec(text("a.txt")), matchA(7), summaryRec(),
			},
			files: []wantFile{{"/wd/a.txt", true, []int64{7}}},
		},
		{
			name: "missing summary is integrity only",
			recs: []string{
				beginRec(text("a.txt")), matchA(7), endRec(text("a.txt")),
			},
			files: []wantFile{{"/wd/a.txt", false, []int64{7}}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ix := buildLifecycle(tc.recs, "")
			checkDisposition(t, ix, tc.malformed, tc.unknown, false)
			checkLifecycleFiles(t, ix, tc.files)
		})
	}
}

// The Issue #9 matrix's first composite row: an ordinary, non-oversized
// trailing record without a terminating newline is counted malformed
// AND marks the stream incomplete — both counters asserted, distinct
// from Task 3's oversized tail which adds the oversized count.
func TestUnterminatedTailIsMalformedAndIncomplete(t *testing.T) {
	tail := matchRec(text("b.txt"), text("hit\n"), 3, sub(text("hit"), 0, 3))
	ix := buildLifecycle([]string{
		beginRec(text("a.txt")),
		matchRec(text("a.txt"), text("hit\n"), 7, sub(text("hit"), 0, 3)),
		endRec(text("a.txt")),
		summaryRec(),
	}, tail)
	checkDisposition(t, ix, 1, 0, false)
	if ix.Oversized != 0 {
		t.Errorf("Oversized = %d, want 0 — the tail is under the limit", ix.Oversized)
	}
	checkLifecycleFiles(t, ix, []wantFile{{"/wd/a.txt", false, []int64{7}}})
}

// The matrix's second composite row: a malformed record after a valid
// summary is counted malformed AND flagged as an after-summary
// integrity failure — the ordering violation does not suppress the
// malformed accounting.
func TestMalformedAfterSummaryIsMalformedAndIncomplete(t *testing.T) {
	ix := buildLifecycle([]string{
		beginRec(text("a.txt")),
		matchRec(text("a.txt"), text("hit\n"), 7, sub(text("hit"), 0, 3)),
		endRec(text("a.txt")),
		summaryRec(),
		`{"type":"match"`,
	}, "")
	checkDisposition(t, ix, 1, 0, false)
	checkLifecycleFiles(t, ix, []wantFile{{"/wd/a.txt", false, []int64{7}}})
}
