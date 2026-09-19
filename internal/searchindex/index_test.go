package searchindex_test

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"testing"

	"vrg/internal/searchindex"
)

// text renders a fixture string as the ripgrep JSON {"text": ...}
// encoding form; byts renders raw bytes as the {"bytes": base64} form.
// The two forms carrying identical bytes are the same value.
func text(s string) string { return `{"text":` + strconv.Quote(s) + `}` }
func byts(s string) string { return `{"bytes":"` + base64.StdEncoding.EncodeToString([]byte(s)) + `"}` }

// Record builders emit single-line records in the ripgrep 15.x shape,
// including the fields the schema matrix intentionally ignores
// (absolute_offset, stats blocks, nested elapsed data), so happy-path
// fixtures conform to the full per-record matrix rather than a reduced
// subset of it.
func beginRec(path string) string {
	return fmt.Sprintf(`{"type":"begin","data":{"path":%s}}`, path)
}

func sub(match string, start, end int) string {
	return fmt.Sprintf(`{"match":%s,"start":%d,"end":%d}`, match, start, end)
}

func matchRec(path, lines string, line int, subs ...string) string {
	return fmt.Sprintf(
		`{"type":"match","data":{"path":%s,"lines":%s,"line_number":%d,"absolute_offset":0,"submatches":[%s]}}`,
		path, lines, line, joinSubs(subs))
}

func joinSubs(subs []string) string {
	out := ""
	for i, s := range subs {
		if i > 0 {
			out += ","
		}
		out += s
	}
	return out
}

func endRec(path string) string {
	return fmt.Sprintf(
		`{"type":"end","data":{"path":%s,"binary_offset":null,"stats":{"elapsed":{"secs":0,"nanos":1,"human":"0.000001s"},"searches":1,"searches_with_match":1,"bytes_searched":12,"bytes_printed":0,"matched_lines":1,"matches":1}}}`,
		path)
}

func summaryRec() string {
	return `{"data":{"elapsed_total":{"human":"0.005s","nanos":5000000,"secs":0},"stats":{"bytes_printed":482,"bytes_searched":34,"elapsed":{"human":"0.000039s","nanos":38542,"secs":0},"matched_lines":2,"matches":2,"searches":2,"searches_with_match":2}},"type":"summary"}`
}

func contextRec() string {
	return `{"type":"context","data":{"path":{"text":"ctx.txt"},"lines":{"text":"nearby\n"},"line_number":2,"absolute_offset":5,"submatches":[]}}`
}

// build feeds a whole fixture stream through the index the way the app's
// collection path does.
func build(t *testing.T, workdir string, recs ...string) *searchindex.Index {
	t.Helper()
	ix := searchindex.New()
	for _, r := range recs {
		ix.Feed([]byte(r))
	}
	ix.Prepare(workdir)
	return ix
}

func wantStops(t *testing.T, f searchindex.File, want []int64) {
	t.Helper()
	if len(f.Stops) != len(want) {
		t.Fatalf("path %q: %d stops, want %d (%v)", f.Path, len(f.Stops), len(want), want)
	}
	for i, n := range want {
		if f.Stops[i].Number != n {
			t.Fatalf("path %q stop %d: number %d, want %d", f.Path, i, f.Stops[i].Number, n)
		}
	}
}

func wantSubmatches(t *testing.T, st searchindex.Stop, want []searchindex.Submatch) {
	t.Helper()
	if len(st.Submatches) != len(want) {
		t.Fatalf("line %d: %d submatches, want %d", st.Number, len(st.Submatches), len(want))
	}
	for i, w := range want {
		got := st.Submatches[i]
		if got.Start != w.Start || got.End != w.End || string(got.Bytes) != string(w.Bytes) {
			t.Fatalf("line %d submatch %d = (%d,%d,%q), want (%d,%d,%q)",
				st.Number, i, got.Start, got.End, got.Bytes, w.Start, w.End, w.Bytes)
		}
	}
}

func wantHighlights(t *testing.T, st searchindex.Stop, want []searchindex.Span) {
	t.Helper()
	if len(st.Highlights) != len(want) {
		t.Fatalf("line %d: %d highlight spans, want %d (%v)", st.Number, len(st.Highlights), len(want), want)
	}
	for i, w := range want {
		if st.Highlights[i] != w {
			t.Fatalf("line %d highlight %d = %v, want %v", st.Number, i, st.Highlights[i], w)
		}
	}
}

// The same happy-path stream in each encoding produces the same retained
// bytes: decoded path (resolved against the working directory), raw line
// bytes, line number, submatch ranges, and recorded submatch bytes.
func TestEncodingFormsRetainIdenticalBytes(t *testing.T) {
	cases := []struct {
		name string
		enc  func(string) string
	}{
		{"text encoding", text},
		{"bytes encoding", byts},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ix := build(t, "/wd",
				beginRec(tc.enc("sub/a.txt")),
				matchRec(tc.enc("sub/a.txt"), tc.enc("hit me\n"), 7,
					sub(tc.enc("hit"), 0, 3)),
				endRec(tc.enc("sub/a.txt")),
				summaryRec(),
			)
			if len(ix.Files) != 1 {
				t.Fatalf("files = %d, want 1", len(ix.Files))
			}
			f := ix.Files[0]
			if string(f.Path) != "/wd/sub/a.txt" {
				t.Fatalf("path = %q, want %q", f.Path, "/wd/sub/a.txt")
			}
			wantStops(t, f, []int64{7})
			st := f.Stops[0]
			if string(st.Bytes) != "hit me\n" {
				t.Fatalf("line bytes = %q, want %q", st.Bytes, "hit me\n")
			}
			wantSubmatches(t, st, []searchindex.Submatch{
				{Start: 0, End: 3, Bytes: []byte("hit")},
			})
			wantHighlights(t, st, []searchindex.Span{{Start: 0, End: 3}})
		})
	}
}

// Raw bytes that are not valid UTF-8 survive base64 round-trips in paths,
// line content, and submatch bytes.
func TestNonUTF8BytesRetained(t *testing.T) {
	ix := build(t, "/wd",
		matchRec(byts("bin\xff.dat"), byts("\xff\xfe\n"), 2,
			sub(byts("\xff"), 0, 1)),
	)
	if len(ix.Files) != 1 {
		t.Fatalf("files = %d, want 1", len(ix.Files))
	}
	f := ix.Files[0]
	if string(f.Path) != "/wd/bin\xff.dat" {
		t.Fatalf("path = %q, want raw bytes %q", f.Path, "/wd/bin\xff.dat")
	}
	st := f.Stops[0]
	if string(st.Bytes) != "\xff\xfe\n" {
		t.Fatalf("line bytes = %q, want raw %q", st.Bytes, "\xff\xfe\n")
	}
	wantSubmatches(t, st, []searchindex.Submatch{
		{Start: 0, End: 1, Bytes: []byte("\xff")},
	})
}

// Two match records for the same path and line merge into one navigation
// stop whose submatches are sorted by start then end, independent of
// record and in-record order.
func TestSamePathSameLineMerge(t *testing.T) {
	ix := build(t, "/wd",
		matchRec(text("a.txt"), text("abcdefghij\n"), 5,
			sub(text("hij"), 7, 10),
			sub(text("abc"), 0, 3)),
		matchRec(text("a.txt"), text("abcdefghij\n"), 5,
			sub(text("def"), 3, 6),
			sub(text("ab"), 0, 2)),
	)
	if len(ix.Files) != 1 {
		t.Fatalf("files = %d, want 1", len(ix.Files))
	}
	f := ix.Files[0]
	wantStops(t, f, []int64{5})
	wantSubmatches(t, f.Stops[0], []searchindex.Submatch{
		{Start: 0, End: 2, Bytes: []byte("ab")},
		{Start: 0, End: 3, Bytes: []byte("abc")},
		{Start: 3, End: 6, Bytes: []byte("def")},
		{Start: 7, End: 10, Bytes: []byte("hij")},
	})
	// Union coverage merges overlapping and touching ranges.
	wantHighlights(t, f.Stops[0], []searchindex.Span{
		{Start: 0, End: 6},
		{Start: 7, End: 10},
	})
}

// A text record and a bytes record carrying the same bytes for the same
// path and line merge into one stop — cross-encoding value identity, not
// just same-encoding deduplication.
func TestMixedEncodingSameValueMerge(t *testing.T) {
	ix := build(t, "/wd",
		matchRec(text("a.txt"), text("xx hit yy\n"), 3,
			sub(text("hit"), 3, 6)),
		matchRec(byts("a.txt"), byts("xx hit yy\n"), 3,
			sub(byts("yy"), 7, 9)),
	)
	if len(ix.Files) != 1 {
		t.Fatalf("files = %d, want 1: text and bytes records for the same path/line must merge", len(ix.Files))
	}
	f := ix.Files[0]
	if string(f.Path) != "/wd/a.txt" {
		t.Fatalf("path = %q, want %q", f.Path, "/wd/a.txt")
	}
	wantStops(t, f, []int64{3})
	wantSubmatches(t, f.Stops[0], []searchindex.Submatch{
		{Start: 3, End: 6, Bytes: []byte("hit")},
		{Start: 7, End: 9, Bytes: []byte("yy")},
	})
}

// Overlapping submatches within one record are all retained, and the
// prepared highlight coverage is their union.
func TestOverlappingSubmatchesUnionCoverage(t *testing.T) {
	ix := build(t, "/wd",
		matchRec(text("f.txt"), text("0123456789abcde\n"), 1,
			sub(text("234567"), 2, 8),
			sub(text("56789"), 5, 10),
			sub(text("cd"), 12, 14)),
	)
	st := ix.Files[0].Stops[0]
	wantSubmatches(t, st, []searchindex.Submatch{
		{Start: 2, End: 8, Bytes: []byte("234567")},
		{Start: 5, End: 10, Bytes: []byte("56789")},
		{Start: 12, End: 14, Bytes: []byte("cd")},
	})
	wantHighlights(t, st, []searchindex.Span{
		{Start: 2, End: 10},
		{Start: 12, End: 14},
	})
}

// Index order: unsigned raw path bytes, then ascending line number. High
// bytes (≥0x80, valid or invalid UTF-8) sort after ASCII by byte value;
// a resolved absolute path participates on equal footing.
func TestIndexOrderingRawPathThenLine(t *testing.T) {
	ix := build(t, "/wd",
		matchRec(byts("\xff.bin"), text("x\n"), 4, sub(text("x"), 0, 1)),
		matchRec(byts("\xff.bin"), text("x\n"), 1, sub(text("x"), 0, 1)),
		matchRec(text("b.go"), text("x\n"), 2, sub(text("x"), 0, 1)),
		matchRec(text("a.go"), text("x\n"), 9, sub(text("x"), 0, 1)),
		matchRec(text("a.go"), text("x\n"), 3, sub(text("x"), 0, 1)),
		matchRec(byts("\x80x"), text("x\n"), 1, sub(text("x"), 0, 1)),
		matchRec(text("/abs/z"), text("x\n"), 1, sub(text("x"), 0, 1)),
	)
	want := []string{"/abs/z", "/wd/a.go", "/wd/b.go", "/wd/\x80x", "/wd/\xff.bin"}
	if len(ix.Files) != len(want) {
		t.Fatalf("files = %v, want %v", filePaths(ix), want)
	}
	for i, w := range want {
		if string(ix.Files[i].Path) != w {
			t.Fatalf("file %d path = %q, want %q (order %v)", i, ix.Files[i].Path, w, filePaths(ix))
		}
	}
	wantStops(t, ix.Files[1], []int64{3, 9})
	wantStops(t, ix.Files[4], []int64{1, 4})
}

func filePaths(ix *searchindex.Index) []string {
	var out []string
	for _, f := range ix.Files {
		out = append(out, string(f.Path))
	}
	return out
}

// Relative result paths resolve against the invocation working directory
// without canonicalization: interior "." / ".." elements stay literal.
func TestRelativePathResolution(t *testing.T) {
	ix := build(t, "/wd/root",
		matchRec(text("sub/f.txt"), text("x\n"), 1, sub(text("x"), 0, 1)),
		matchRec(text("/abs/f.txt"), text("x\n"), 1, sub(text("x"), 0, 1)),
		matchRec(text("a/../b.txt"), text("x\n"), 1, sub(text("x"), 0, 1)),
		matchRec(text("./dot.txt"), text("x\n"), 1, sub(text("x"), 0, 1)),
	)
	got := filePaths(ix)
	want := []string{
		"/abs/f.txt",
		"/wd/root/./dot.txt",
		"/wd/root/a/../b.txt",
		"/wd/root/sub/f.txt",
	}
	if len(got) != len(want) {
		t.Fatalf("paths = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("path %d = %q, want %q (all: %v)", i, got[i], want[i], got)
		}
	}
}

// The five known events classify; context is recognized but contributes
// nothing to the index, and an unrecognized string type is unknown rather
// than malformed.
func TestRecordKindsAndIgnoredContext(t *testing.T) {
	ix := searchindex.New()
	cases := []struct {
		name string
		rec  string
		want searchindex.Kind
	}{
		{"begin", beginRec(text("f.txt")), searchindex.KindBegin},
		{"match", matchRec(text("f.txt"), text("hit\n"), 4, sub(text("hit"), 0, 3)), searchindex.KindMatch},
		{"end", endRec(text("f.txt")), searchindex.KindEnd},
		{"end with binary offset", `{"type":"end","data":{"path":{"text":"f.txt"},"binary_offset":12,"stats":{}}}`, searchindex.KindEnd},
		{"summary", summaryRec(), searchindex.KindSummary},
		{"context ignored", contextRec(), searchindex.KindContext},
		{"unknown type", `{"type":"stats","data":{"searches":1}}`, searchindex.KindUnknown},
	}
	for _, tc := range cases {
		if got := ix.Feed([]byte(tc.rec)); got != tc.want {
			t.Errorf("%s: Feed kind = %v, want %v", tc.name, got, tc.want)
		}
	}
	ix.Prepare("/wd")
	if len(ix.Files) != 1 {
		t.Fatalf("files = %d, want 1 — only the match record contributes a stop", len(ix.Files))
	}
	wantStops(t, ix.Files[0], []int64{4})
	if string(ix.Files[0].Path) != "/wd/f.txt" {
		t.Fatalf("path = %q, want %q; context data must be ignored", ix.Files[0].Path, "/wd/f.txt")
	}
}

// Malformed records — invalid JSON, missing or non-string type, invalid
// base64, missing or mistyped required fields, out-of-range values — are
// skipped rather than indexed. (Skip counting lands with Issue #10.)
func TestMalformedRecordsSkipped(t *testing.T) {
	ix := searchindex.New()
	recs := []string{
		`this is not json`,
		`{"data":{}}`,                     // missing type
		`{"type":5,"data":{}}`,            // non-string type
		`{"type":"begin","data":{"path":`, // truncated record
		`{"type":"begin","data":{}}`,      // missing data.path
		`{"type":"match","data":{"path":{"bytes":"!!!not-base64!!!"},"lines":{"text":"x\n"},"line_number":1,"submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}`,
		`{"type":"match","data":{"path":{"text":"f"},"lines":{"text":"x\n"},"line_number":0,"submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}`,                    // line_number < 1
		`{"type":"match","data":{"path":{"text":"f"},"lines":{"text":"x\n"},"line_number":1.5,"submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}`,                  // non-integer
		`{"type":"match","data":{"path":{"text":"f"},"lines":{"text":"x\n"},"line_number":99999999999999999999,"submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}`, // overflows int64
		`{"type":"match","data":{"path":{"text":"f"},"lines":{"text":"x\n"},"line_number":1,"submatches":[]}}`,                                                            // empty submatches
		`{"type":"match","data":{"path":{"text":"f"},"lines":{"text":"x\n"},"line_number":1,"submatches":[{"match":{"text":"x"},"start":1,"end":0}]}}`,                    // start > end
		`{"type":"match","data":{"path":{"text":"f"},"lines":{"text":"x\n"},"line_number":1,"submatches":[{"match":{"text":"x"},"start":0,"end":9}]}}`,                    // end beyond line
		`{"type":"match","data":{"path":{"text":"f"},"lines":{"text":"x\n"},"submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}`,                                    // missing line_number
		`{"type":"end","data":{"path":{"text":"f"},"stats":{}}}`,                                                                                                          // missing binary_offset
		`{"type":"end","data":{"path":{"text":"f"},"binary_offset":-2,"stats":{}}}`,                                                                                       // negative binary_offset
		`{"type":"end","data":{"path":{"text":"f"},"binary_offset":"none","stats":{}}}`,                                                                                   // non-integer binary_offset
		`{"type":"summary","data":5}`, // data not an object
		`{"type":"summary"}`,          // missing data
	}
	for i, r := range recs {
		if got := ix.Feed([]byte(r)); got != searchindex.KindMalformed {
			t.Errorf("record %d: Feed kind = %v, want KindMalformed: %s", i, got, r)
		}
	}
	ix.Prepare("/wd")
	if len(ix.Files) != 0 {
		t.Fatalf("malformed records produced %d files: %v", len(ix.Files), filePaths(ix))
	}
}

// A boundary-value matrix: submatch ranges may touch the ends of the
// decoded line bytes and zero-width matches are in range.
func TestSubmatchRangeBoundaries(t *testing.T) {
	ix := build(t, "/wd",
		matchRec(text("f"), text("abc\n"), 1,
			sub(text("abc"), 0, 3), // whole line content
			sub(text(""), 4, 4),    // zero-width at the line's end (after \n? no — end of bytes)
			sub(text("\n"), 3, 4)), // terminator bytes themselves are in range
	)
	st := ix.Files[0].Stops[0]
	wantSubmatches(t, st, []searchindex.Submatch{
		{Start: 0, End: 3, Bytes: []byte("abc")},
		{Start: 3, End: 4, Bytes: []byte("\n")},
		{Start: 4, End: 4, Bytes: []byte("")},
	})
	wantHighlights(t, st, []searchindex.Span{{Start: 0, End: 4}})
}
