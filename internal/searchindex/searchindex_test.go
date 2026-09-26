package searchindex_test

import (
	"encoding/base64"
	"strconv"
	"strings"
	"testing"

	"vrg/internal/searchindex"
)

func ptr(v int64) *int64 { return &v }

// jText and jBytes render the two rg JSON encodings for a value: the
// {"text": ...} string form and the {"bytes": ...} base64 form.
func jText(s string) string  { return `{"text":` + strconv.Quote(s) + `}` }
func jBytes(b []byte) string { return `{"bytes":"` + base64.StdEncoding.EncodeToString(b) + `"}` }

// Record fixture builders. Each takes pre-rendered value fields so a
// fixture can mix text and bytes encodings freely.
func beginRec(path string) string {
	return `{"type":"begin","data":{"path":` + path + `}}`
}

func matchRec(path, lines string, num int64, subs ...string) string {
	return `{"type":"match","data":{"path":` + path +
		`,"lines":` + lines +
		`,"line_number":` + strconv.FormatInt(num, 10) +
		`,"submatches":[` + strings.Join(subs, ",") + `]}}`
}

func subRec(match string, start, end int) string {
	return `{"match":` + match +
		`,"start":` + strconv.Itoa(start) +
		`,"end":` + strconv.Itoa(end) + `}`
}

func endRec(path, binaryOffset string) string {
	return `{"type":"end","data":{"path":` + path + `,"binary_offset":` + binaryOffset + `}}`
}

// feed decodes each fixture line — every happy-path fixture must decode
// cleanly — and applies it to the index.
func feed(t *testing.T, ix *searchindex.Index, records ...string) {
	t.Helper()
	for _, r := range records {
		rec, err := searchindex.DecodeRecord([]byte(r))
		if err != nil {
			t.Fatalf("DecodeRecord(%s) error: %v", r, err)
		}
		ix.Add(rec)
	}
}

// Every known event decodes to its typed record with the required fields
// carried through, under either encoding.
func TestDecodeRecordKinds(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		kind    searchindex.Kind
		path    string // decoded path bytes, "" when the record has none
		binOff  *int64 // end records only
		wantNil bool   // end records: binary_offset was null
	}{
		{"begin text path", beginRec(jText("src/f.go")), searchindex.KindBegin, "src/f.go", nil, false},
		{"begin bytes path", beginRec(jBytes([]byte("src/f.go"))), searchindex.KindBegin, "src/f.go", nil, false},
		{"begin non-UTF8 bytes path", beginRec(jBytes([]byte{'f', 0xff})), searchindex.KindBegin, "f\xff", nil, false},
		{"end null offset", endRec(jText("a"), "null"), searchindex.KindEnd, "a", nil, true},
		{"end integer offset", endRec(jText("a"), "3"), searchindex.KindEnd, "a", ptr(int64(3)), false},
		{"end bytes path", endRec(jBytes([]byte("a")), "null"), searchindex.KindEnd, "a", nil, true},
		{"summary empty data", `{"type":"summary","data":{}}`, searchindex.KindSummary, "", nil, false},
		{"summary full data", `{"type":"summary","data":{"elapsed_total":{"secs":1,"nanos":2,"human":"0.1s"},"stats":{"searches":4}}}`, searchindex.KindSummary, "", nil, false},
		{"context with data", `{"type":"context","data":{"path":{"text":"a"},"lines":{"text":"x\n"},"line_number":2,"submatches":[]}}`, searchindex.KindContext, "", nil, false},
		{"context without data", `{"type":"context"}`, searchindex.KindContext, "", nil, false},
		{"context arbitrary data", `{"type":"context","data":[1,2,3]}`, searchindex.KindContext, "", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec, err := searchindex.DecodeRecord([]byte(tc.raw))
			if err != nil {
				t.Fatalf("DecodeRecord(%s) error: %v", tc.raw, err)
			}
			if rec.Kind != tc.kind {
				t.Fatalf("DecodeRecord(%s).Kind = %v, want %v", tc.raw, rec.Kind, tc.kind)
			}
			if string(rec.Path) != tc.path {
				t.Fatalf("DecodeRecord(%s).Path = %q, want %q", tc.raw, rec.Path, tc.path)
			}
			if tc.kind == searchindex.KindEnd {
				if tc.wantNil && rec.BinaryOffset != nil {
					t.Fatalf("BinaryOffset = %v, want null", *rec.BinaryOffset)
				}
				if tc.binOff != nil && (rec.BinaryOffset == nil || *rec.BinaryOffset != *tc.binOff) {
					t.Fatalf("BinaryOffset = %v, want %v", rec.BinaryOffset, *tc.binOff)
				}
			}
		})
	}
}

// A match record decodes path, lines, and submatch fields in either
// encoding, retaining raw bytes, line number, ranges, and recorded
// submatch bytes.
func TestDecodeMatchRecord(t *testing.T) {
	line := "hit and hit\n"
	cases := []struct {
		name string
		raw  string
	}{
		{"all text", matchRec(jText("f"), jText(line), 7,
			subRec(jText("hit"), 0, 3), subRec(jText("hit"), 8, 11))},
		{"all bytes", matchRec(jBytes([]byte("f")), jBytes([]byte(line)), 7,
			subRec(jBytes([]byte("hit")), 0, 3), subRec(jBytes([]byte("hit")), 8, 11))},
		{"mixed encodings", matchRec(jBytes([]byte("f")), jText(line), 7,
			subRec(jBytes([]byte("hit")), 0, 3), subRec(jText("hit"), 8, 11))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec, err := searchindex.DecodeRecord([]byte(tc.raw))
			if err != nil {
				t.Fatalf("DecodeRecord(%s) error: %v", tc.raw, err)
			}
			if rec.Kind != searchindex.KindMatch {
				t.Fatalf("Kind = %v, want KindMatch", rec.Kind)
			}
			if string(rec.Path) != "f" {
				t.Fatalf("Path = %q, want %q", rec.Path, "f")
			}
			if string(rec.Line) != line {
				t.Fatalf("Line = %q, want %q", rec.Line, line)
			}
			if rec.LineNumber != 7 {
				t.Fatalf("LineNumber = %d, want 7", rec.LineNumber)
			}
			want := []searchindex.Submatch{
				{Start: 0, End: 3, Bytes: []byte("hit")},
				{Start: 8, End: 11, Bytes: []byte("hit")},
			}
			if len(rec.Submatches) != len(want) {
				t.Fatalf("Submatches = %+v, want %+v", rec.Submatches, want)
			}
			for i, s := range rec.Submatches {
				if s.Start != want[i].Start || s.End != want[i].End || string(s.Bytes) != string(want[i].Bytes) {
					t.Fatalf("Submatches[%d] = %+v, want %+v", i, s, want[i])
				}
			}
		})
	}
}

// Ignored fields ride along without effect: absolute_offset on match and
// stats on end are not required and not read.
func TestDecodeIgnoresUnrequiredFields(t *testing.T) {
	rec, err := searchindex.DecodeRecord([]byte(
		`{"type":"match","data":{"path":{"text":"f"},"lines":{"text":"x\n"},"line_number":1,"absolute_offset":99,"submatches":[{"match":{"text":"x"},"start":0,"end":1}],"extra":[1]}}`))
	if err != nil {
		t.Fatalf("DecodeRecord error: %v", err)
	}
	if rec.Kind != searchindex.KindMatch || rec.LineNumber != 1 {
		t.Fatalf("rec = %+v", rec)
	}
	if _, err := searchindex.DecodeRecord([]byte(
		`{"type":"end","data":{"path":{"text":"f"},"binary_offset":null,"stats":{"searches":1}}}`)); err != nil {
		t.Fatalf("end with stats error: %v", err)
	}
}

// Per-record schema violations are decode errors: invalid JSON, invalid
// base64, missing or non-string type, missing or mistyped required
// fields, and out-of-range values. A string type outside the five known
// events is not malformed — it decodes as KindUnknown.
func TestMalformedRecords(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"not json", `this is not json`},
		{"json array", `[1,2,3]`},
		{"json bare string", `"match"`},
		{"missing type", `{"data":{}}`},
		{"non-string type", `{"type":5,"data":{}}`},
		{"null type", `{"type":null,"data":{}}`},
		{"begin missing data", `{"type":"begin"}`},
		{"begin missing path", `{"type":"begin","data":{}}`},
		{"begin null data", `{"type":"begin","data":null}`},
		{"begin path neither field", `{"type":"begin","data":{"path":{}}}`},
		{"begin path non-string text", `{"type":"begin","data":{"path":{"text":5}}}`},
		{"begin path bad base64", `{"type":"begin","data":{"path":{"bytes":"!!notbase64"}}}`},
		{"begin path scalar", `{"type":"begin","data":{"path":"f"}}`},
		{"match missing lines", `{"type":"match","data":{"path":{"text":"f"},"line_number":1,"submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}`},
		{"match missing line_number", `{"type":"match","data":{"path":{"text":"f"},"lines":{"text":"x\n"},"submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}`},
		{"match missing submatches", `{"type":"match","data":{"path":{"text":"f"},"lines":{"text":"x\n"},"line_number":1}}`},
		{"match empty submatches", `{"type":"match","data":{"path":{"text":"f"},"lines":{"text":"x\n"},"line_number":1,"submatches":[]}}`},
		{"match null submatches", `{"type":"match","data":{"path":{"text":"f"},"lines":{"text":"x\n"},"line_number":1,"submatches":null}}`},
		{"match line_number zero", `{"type":"match","data":{"path":{"text":"f"},"lines":{"text":"x\n"},"line_number":0,"submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}`},
		{"match line_number negative", `{"type":"match","data":{"path":{"text":"f"},"lines":{"text":"x\n"},"line_number":-2,"submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}`},
		{"match line_number non-integer", `{"type":"match","data":{"path":{"text":"f"},"lines":{"text":"x\n"},"line_number":1.5,"submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}`},
		{"match line_number string", `{"type":"match","data":{"path":{"text":"f"},"lines":{"text":"x\n"},"line_number":"1","submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}`},
		{"match line_number beyond int64", `{"type":"match","data":{"path":{"text":"f"},"lines":{"text":"x\n"},"line_number":9223372036854775808,"submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}`},
		{"submatch missing match", `{"type":"match","data":{"path":{"text":"f"},"lines":{"text":"x\n"},"line_number":1,"submatches":[{"start":0,"end":1}]}}`},
		{"submatch missing start", `{"type":"match","data":{"path":{"text":"f"},"lines":{"text":"x\n"},"line_number":1,"submatches":[{"match":{"text":"x"},"end":1}]}}`},
		{"submatch missing end", `{"type":"match","data":{"path":{"text":"f"},"lines":{"text":"x\n"},"line_number":1,"submatches":[{"match":{"text":"x"},"start":0}]}}`},
		{"submatch negative start", `{"type":"match","data":{"path":{"text":"f"},"lines":{"text":"x\n"},"line_number":1,"submatches":[{"match":{"text":"x"},"start":-1,"end":1}]}}`},
		{"submatch start after end", `{"type":"match","data":{"path":{"text":"x\n"},"line_number":1,"submatches":[{"match":{"text":"x"},"start":2,"end":1}]}}`},
		{"submatch end beyond line", `{"type":"match","data":{"path":{"text":"x\n"},"line_number":1,"submatches":[{"match":{"text":"x"},"start":0,"end":3}]}}`},
		{"submatch bad base64 match", `{"type":"match","data":{"path":{"text":"f"},"lines":{"text":"x\n"},"line_number":1,"submatches":[{"match":{"bytes":"!!"},"start":0,"end":1}]}}`},
		{"submatch start beyond int64", `{"type":"match","data":{"path":{"text":"f"},"lines":{"text":"x\n"},"line_number":1,"submatches":[{"match":{"text":"x"},"start":99999999999999999999,"end":1}]}}`},
		{"end missing binary_offset", `{"type":"end","data":{"path":{"text":"f"}}}`},
		{"end negative binary_offset", `{"type":"end","data":{"path":{"text":"f"},"binary_offset":-1}}`},
		{"end string binary_offset", `{"type":"end","data":{"path":{"text":"f"},"binary_offset":"0"}}`},
		{"summary missing data", `{"type":"summary"}`},
		{"summary scalar data", `{"type":"summary","data":5}`},
		{"summary null data", `{"type":"summary","data":null}`},
		{"summary array data", `{"type":"summary","data":[1]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec, err := searchindex.DecodeRecord([]byte(tc.raw))
			if err == nil {
				t.Fatalf("DecodeRecord(%s) = %+v, want malformed-record error", tc.raw, rec)
			}
		})
	}
}

// A string type outside the five known events is a valid decode with
// KindUnknown; it is not a malformed record and contributes nothing.
func TestUnknownRecordType(t *testing.T) {
	for _, raw := range []string{
		`{"type":"stats","data":{"searches":3}}`,
		`{"type":"nope"}`,
		`{"type":"match2","data":{}}`,
	} {
		rec, err := searchindex.DecodeRecord([]byte(raw))
		if err != nil {
			t.Fatalf("DecodeRecord(%s) error: %v", raw, err)
		}
		if rec.Kind != searchindex.KindUnknown {
			t.Fatalf("DecodeRecord(%s).Kind = %v, want KindUnknown", raw, rec.Kind)
		}
	}
}

// The happy-path stream: begin, match, end, summary, and ignored context
// events all decode and flow through the index; only matches build
// stops.
func TestHappyPathStream(t *testing.T) {
	ix := searchindex.New("/work")
	feed(t, ix,
		beginRec(jText("src/a.go")),
		matchRec(jText("src/a.go"), jText("one hit\n"), 1, subRec(jText("hit"), 4, 7)),
		`{"type":"context","data":{"path":{"text":"src/a.go"},"lines":{"text":"ctx\n"},"line_number":2,"submatches":[]}}`,
		endRec(jText("src/a.go"), "null"),
		beginRec(jBytes([]byte("src/b.go"))),
		matchRec(jBytes([]byte("src/b.go")), jBytes([]byte("two hit hit\n")), 9,
			subRec(jBytes([]byte("hit")), 4, 7), subRec(jText("hit"), 8, 11)),
		endRec(jBytes([]byte("src/b.go")), "null"),
		`{"type":"summary","data":{"elapsed_total":{"secs":0},"stats":{"searches":2}}}`,
	)
	ix.Prepare()

	stops := ix.Stops()
	if len(stops) != 2 {
		t.Fatalf("len(Stops) = %d, want 2: %+v", len(stops), stops)
	}
	if ix.FileCount() != 2 {
		t.Fatalf("FileCount = %d, want 2", ix.FileCount())
	}
	first, second := stops[0], stops[1]
	if string(first.Path) != "src/a.go" || first.Line != 1 {
		t.Fatalf("stops[0] = %+v, want src/a.go:1", first)
	}
	if string(second.Path) != "src/b.go" || second.Line != 9 {
		t.Fatalf("stops[1] = %+v, want src/b.go:9", second)
	}
}

// Same-path/same-line match records merge into one navigation stop whose
// submatches are sorted by start then end, even when a record carries
// them out of order.
func TestSameLineMerging(t *testing.T) {
	ix := searchindex.New("/work")
	feed(t, ix,
		beginRec(jText("f")),
		// Deliberately unsorted within the record.
		matchRec(jText("f"), jText("aa bb aa\n"), 3,
			subRec(jText("aa"), 6, 8), subRec(jText("aa"), 0, 2)),
		// A second match record on the same path and line.
		matchRec(jText("f"), jText("aa bb aa\n"), 3, subRec(jText("bb"), 3, 5)),
		endRec(jText("f"), "null"),
		`{"type":"summary","data":{}}`,
	)
	ix.Prepare()

	stops := ix.Stops()
	if len(stops) != 1 {
		t.Fatalf("len(Stops) = %d, want one merged stop: %+v", len(stops), stops)
	}
	s := stops[0]
	if s.Line != 3 {
		t.Fatalf("stop.Line = %d, want 3", s.Line)
	}
	type rng struct{ start, end int }
	want := []rng{{0, 2}, {3, 5}, {6, 8}}
	if len(s.Submatches) != len(want) {
		t.Fatalf("Submatches = %+v, want %d ranges", s.Submatches, len(want))
	}
	for i, sub := range s.Submatches {
		if sub.Start != want[i].start || sub.End != want[i].end {
			t.Fatalf("Submatches[%d] = (%d,%d), want %v", i, sub.Start, sub.End, want[i])
		}
	}
	// Recorded bytes retained per submatch.
	for _, want := range []string{"aa", "bb", "aa"} {
		found := false
		for _, sub := range s.Submatches {
			if string(sub.Bytes) == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("recorded submatch bytes %q not retained: %+v", want, s.Submatches)
		}
	}
}

// A text record and a bytes record carrying the same bytes for the same
// path and line merge into one stop as the same value: encoding is a
// transport detail, not identity.
func TestMixedEncodingSameValueMerge(t *testing.T) {
	ix := searchindex.New("/work")
	feed(t, ix,
		matchRec(jText("src/f"), jText("hit\n"), 4, subRec(jText("hit"), 0, 3)),
		matchRec(jBytes([]byte("src/f")), jBytes([]byte("hit\n")), 4, subRec(jBytes([]byte("hit")), 0, 3)),
	)
	ix.Prepare()

	stops := ix.Stops()
	if len(stops) != 1 {
		t.Fatalf("len(Stops) = %d, want one merged stop across encodings: %+v", len(stops), stops)
	}
	if len(stops[0].Submatches) != 1 || stops[0].Submatches[0].Start != 0 || stops[0].Submatches[0].End != 3 {
		t.Fatalf("merged stop submatches = %+v, want the single shared (0,3) value", stops[0].Submatches)
	}
	if ix.FileCount() != 1 {
		t.Fatalf("FileCount = %d, want 1", ix.FileCount())
	}
}

// Overlapping submatches in one record are all retained; the prepared
// highlight coverage is their union.
func TestOverlappingSubmatchUnionCoverage(t *testing.T) {
	ix := searchindex.New("/work")
	feed(t, ix,
		matchRec(jText("f"), jText("hello wo\n"), 1,
			subRec(jText("hello"), 0, 5), subRec(jText("llo wo"), 2, 8)),
		matchRec(jText("g"), jText("ab cd\n"), 1,
			subRec(jText("ab"), 0, 2), subRec(jText("cd"), 4, 6)),
		matchRec(jText("h"), jText("abcd\n"), 1,
			subRec(jText("ab"), 0, 2), subRec(jText("cd"), 2, 4)),
	)
	ix.Prepare()

	stops := ix.Stops()
	if len(stops) != 3 {
		t.Fatalf("len(Stops) = %d, want 3", len(stops))
	}
	// Overlapping pair: both original ranges retained, union is (0,8).
	if len(stops[0].Submatches) != 2 {
		t.Fatalf("overlapping submatches dropped: %+v", stops[0].Submatches)
	}
	assertRanges(t, "overlapping union", stops[0].Highlights, [][2]int{{0, 8}})
	// Disjoint pair: union keeps both ranges.
	assertRanges(t, "disjoint union", stops[1].Highlights, [][2]int{{0, 2}, {4, 6}})
	// Adjacent pair: union is the contiguous span.
	assertRanges(t, "adjacent union", stops[2].Highlights, [][2]int{{0, 4}})
}

func assertRanges(t *testing.T, name string, got []searchindex.Range, want [][2]int) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: Highlights = %+v, want %v", name, got, want)
	}
	for i, r := range got {
		if r.Start != want[i][0] || r.End != want[i][1] {
			t.Fatalf("%s: Highlights[%d] = (%d,%d), want %v", name, i, r.Start, r.End, want[i])
		}
	}
}

// Index order is unsigned lexicographic raw path bytes, then ascending
// line number; non-UTF-8 path bytes order deterministically against
// valid UTF-8.
func TestIndexOrdering(t *testing.T) {
	ix := searchindex.New("/work")
	feed(t, ix,
		matchRec(jBytes([]byte{0xff, 'x'}), jText("h\n"), 5, subRec(jText("h"), 0, 1)),
		matchRec(jText("b/f"), jText("h\n"), 1, subRec(jText("h"), 0, 1)),
		matchRec(jText("a/f"), jText("h\n"), 10, subRec(jText("h"), 0, 1)),
		matchRec(jText("a/f"), jText("h\n"), 2, subRec(jText("h"), 0, 1)),
		matchRec(jText("ab"), jText("h\n"), 1, subRec(jText("h"), 0, 1)),
		matchRec(jBytes([]byte{'a', 0x80, 'z'}), jText("h\n"), 1, subRec(jText("h"), 0, 1)),
	)
	ix.Prepare()

	stops := ix.Stops()
	type wantStop struct {
		path string
		line int64
	}
	want := []wantStop{
		{"a/f", 2},    // '/' (0x2f) precedes 'b' and the raw 0x80 byte
		{"a/f", 10},   // same path: ascending line number
		{"ab", 1},     // 'b' (0x62) precedes raw byte 0x80
		{"a\x80z", 1}, // 0x80 sorts as the unsigned byte it is
		{"b/f", 1},
		{"\xffx", 5}, // non-UTF-8 bytes order deterministically after ASCII
	}
	if len(stops) != len(want) {
		t.Fatalf("len(Stops) = %d, want %d: %+v", len(stops), len(want), stops)
	}
	for i, s := range stops {
		if string(s.Path) != want[i].path || s.Line != want[i].line {
			t.Fatalf("Stops()[%d] = (%q, %d), want (%q, %d)", i, s.Path, s.Line, want[i].path, want[i].line)
		}
	}
}

// Raw path bytes, line numbers, submatch ranges, and recorded submatch
// bytes are retained on each stop.
func TestStopRetainsRawData(t *testing.T) {
	ix := searchindex.New("/work")
	raw := []byte{'d', 0xff, 'r'}
	feed(t, ix,
		matchRec(jBytes(raw), jBytes([]byte("xy\n")), 42, subRec(jBytes([]byte("xy")), 0, 2)),
	)
	ix.Prepare()

	s := ix.Stops()[0]
	if string(s.Path) != string(raw) {
		t.Fatalf("Path = %q, want raw bytes %q", s.Path, raw)
	}
	if s.Line != 42 {
		t.Fatalf("Line = %d, want 42", s.Line)
	}
	if len(s.Submatches) != 1 || s.Submatches[0].Start != 0 || s.Submatches[0].End != 2 ||
		string(s.Submatches[0].Bytes) != "xy" {
		t.Fatalf("Submatches = %+v, want one recorded (0,2) 'xy'", s.Submatches)
	}
}

// Relative result paths resolve against the invocation working
// directory with no canonicalization; absolute paths pass through.
// Ordering still uses the raw emitted bytes, not the resolved form.
func TestRelativePathResolution(t *testing.T) {
	ix := searchindex.New("/work/dir")
	feed(t, ix,
		matchRec(jText("src/f.go"), jText("h\n"), 1, subRec(jText("h"), 0, 1)),
		matchRec(jText("/abs/f.go"), jText("h\n"), 1, subRec(jText("h"), 0, 1)),
		matchRec(jText("./rel.go"), jText("h\n"), 1, subRec(jText("h"), 0, 1)),
		matchRec(jText("a/../b.go"), jText("h\n"), 1, subRec(jText("h"), 0, 1)),
	)
	ix.Prepare()

	resolved := map[string]string{}
	for _, s := range ix.Stops() {
		resolved[string(s.Path)] = string(s.ResolvedPath)
	}
	want := map[string]string{
		"src/f.go":  "/work/dir/src/f.go",
		"/abs/f.go": "/abs/f.go",
		"./rel.go":  "/work/dir/./rel.go", // no canonicalization
		"a/../b.go": "/work/dir/a/../b.go",
	}
	for raw, w := range want {
		if resolved[raw] != w {
			t.Fatalf("ResolvedPath for %q = %q, want %q", raw, resolved[raw], w)
		}
	}
}
