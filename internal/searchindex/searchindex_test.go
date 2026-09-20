package searchindex_test

import (
	"encoding/base64"
	"encoding/json"
	"slices"
	"testing"

	"vrg/internal/searchindex"
)

// Fixture helpers build rg --json event records with encoding/json so the
// tests never rely on hand-escaped JSON text.

func marshal(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("fixture does not marshal: %v", err)
	}
	return b
}

// textData renders an rg string blob; bytesData renders the base64 blob
// ripgrep emits for non-UTF-8 data.
func textData(s string) map[string]any { return map[string]any{"text": s} }
func bytesData(b []byte) map[string]any {
	return map[string]any{"bytes": base64.StdEncoding.EncodeToString(b)}
}

func submatch(match map[string]any, start, end int) map[string]any {
	return map[string]any{"match": match, "start": start, "end": end}
}

func matchEvent(t *testing.T, path, lines map[string]any, line int64, subs ...map[string]any) []byte {
	t.Helper()
	return marshal(t, map[string]any{
		"type": "match",
		"data": map[string]any{
			"path":            path,
			"lines":           lines,
			"line_number":     line,
			"absolute_offset": 0,
			"submatches":      subs,
		},
	})
}

func beginEvent(t *testing.T, path map[string]any) []byte {
	t.Helper()
	return marshal(t, map[string]any{
		"type": "begin",
		"data": map[string]any{"path": path},
	})
}

func endEvent(t *testing.T, path map[string]any, binaryOffset any) []byte {
	t.Helper()
	return marshal(t, map[string]any{
		"type": "end",
		"data": map[string]any{"path": path, "binary_offset": binaryOffset},
	})
}

func summaryEvent(t *testing.T) []byte {
	t.Helper()
	return marshal(t, map[string]any{
		"type": "summary",
		"data": map[string]any{
			"elapsed_total": map[string]any{"secs": 0, "nanos": 1, "human": "0.000001s"},
			"stats":         map[string]any{"searches": 1},
		},
	})
}

// collect feeds every record into a fresh index and finishes it; the test
// fails if any record is rejected.
func collect(t *testing.T, workdir string, records ...[]byte) *searchindex.Index {
	t.Helper()
	idx := searchindex.New(workdir)
	for i, rec := range records {
		if err := idx.Add(rec); err != nil {
			t.Fatalf("Add(record %d) rejected a valid record: %v\n%s", i, err, rec)
		}
	}
	idx.Finish()
	return idx
}

// sub describes one expected submatch: its byte range and recorded bytes.
type sub struct {
	start, end int
	text       string
}

// wantStops asserts the prepared index equals the expected ordered stops.
func wantStops(t *testing.T, idx *searchindex.Index, want []searchindex.Stop) {
	t.Helper()
	got := idx.Stops()
	if len(got) != len(want) {
		t.Fatalf("Stops() has %d entries, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		g, w := got[i], want[i]
		if !slices.Equal(g.Path, w.Path) {
			t.Fatalf("stop %d Path = %q, want %q", i, g.Path, w.Path)
		}
		if !slices.Equal(g.Resolved, w.Resolved) {
			t.Fatalf("stop %d Resolved = %q, want %q", i, g.Resolved, w.Resolved)
		}
		if g.Line != w.Line {
			t.Fatalf("stop %d Line = %d, want %d", i, g.Line, w.Line)
		}
		if len(g.Submatches) != len(w.Submatches) {
			t.Fatalf("stop %d has %d submatches, want %d: %+v", i, len(g.Submatches), len(w.Submatches), g.Submatches)
		}
		for j := range w.Submatches {
			gs, ws := g.Submatches[j], w.Submatches[j]
			if gs.Range != ws.Range || !slices.Equal(gs.Text, ws.Text) {
				t.Fatalf("stop %d submatch %d = %+v, want %+v", i, j, gs, ws)
			}
		}
		if !slices.Equal(g.Coverage, w.Coverage) {
			t.Fatalf("stop %d Coverage = %+v, want %+v", i, g.Coverage, w.Coverage)
		}
	}
}

// Both encodings of the same bytes produce the same stop: paths, lines,
// and submatch text may each independently arrive as text or base64
// bytes.
func TestTextAndBytesEncodings(t *testing.T) {
	line := int64(3)
	cases := []struct {
		name string
		rec  func(t *testing.T) []byte
	}{
		{"all text", func(t *testing.T) []byte {
			return matchEvent(t, textData("dir/a.txt"), textData("hello world\n"), line,
				submatch(textData("hello"), 0, 5))
		}},
		{"all bytes", func(t *testing.T) []byte {
			return matchEvent(t, bytesData([]byte("dir/a.txt")), bytesData([]byte("hello world\n")), line,
				submatch(bytesData([]byte("hello")), 0, 5))
		}},
		{"bytes path text rest", func(t *testing.T) []byte {
			return matchEvent(t, bytesData([]byte("dir/a.txt")), textData("hello world\n"), line,
				submatch(textData("hello"), 0, 5))
		}},
		{"text path bytes lines", func(t *testing.T) []byte {
			return matchEvent(t, textData("dir/a.txt"), bytesData([]byte("hello world\n")), line,
				submatch(bytesData([]byte("hello")), 0, 5))
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			idx := collect(t, "/work", tc.rec(t))
			wantStops(t, idx, []searchindex.Stop{{
				Path:     []byte("dir/a.txt"),
				Resolved: []byte("/work/dir/a.txt"),
				Line:     3,
				Submatches: []searchindex.Submatch{
					{Range: searchindex.Range{Start: 0, End: 5}, Text: []byte("hello")},
				},
				Coverage: []searchindex.Range{{Start: 0, End: 5}},
			}})
		})
	}
}

// Two match events for the same path and line merge into one navigation
// stop; its submatches are sorted by start then end, not by arrival order.
func TestSameLineMatchesMerge(t *testing.T) {
	idx := collect(t, "/w",
		beginEvent(t, textData("a.txt")),
		matchEvent(t, textData("a.txt"), textData("aa bb aa\n"), 7,
			submatch(textData("aa"), 6, 8)),
		matchEvent(t, textData("a.txt"), textData("aa bb aa\n"), 7,
			submatch(textData("bb"), 3, 5),
			submatch(textData("aa"), 0, 2)),
		endEvent(t, textData("a.txt"), nil),
		summaryEvent(t),
	)
	wantStops(t, idx, []searchindex.Stop{{
		Path:     []byte("a.txt"),
		Resolved: []byte("/w/a.txt"),
		Line:     7,
		Submatches: []searchindex.Submatch{
			{Range: searchindex.Range{Start: 0, End: 2}, Text: []byte("aa")},
			{Range: searchindex.Range{Start: 3, End: 5}, Text: []byte("bb")},
			{Range: searchindex.Range{Start: 6, End: 8}, Text: []byte("aa")},
		},
		Coverage: []searchindex.Range{{0, 2}, {3, 5}, {6, 8}},
	}})
}

// A text record and a bytes record carrying the same bytes for the same
// path and line are the same value: they merge into one stop. This proves
// cross-encoding identity rather than inferring it from separate
// encodings.
func TestMixedEncodingSameValueMerges(t *testing.T) {
	idx := collect(t, "/w",
		matchEvent(t, textData("a.txt"), textData("x hit y\n"), 2,
			submatch(textData("hit"), 2, 5)),
		matchEvent(t, bytesData([]byte("a.txt")), bytesData([]byte("x hit y\n")), 2,
			submatch(bytesData([]byte("hit")), 2, 5)),
	)
	wantStops(t, idx, []searchindex.Stop{{
		Path:     []byte("a.txt"),
		Resolved: []byte("/w/a.txt"),
		Line:     2,
		Submatches: []searchindex.Submatch{
			{Range: searchindex.Range{Start: 2, End: 5}, Text: []byte("hit")},
			{Range: searchindex.Range{Start: 2, End: 5}, Text: []byte("hit")},
		},
		Coverage: []searchindex.Range{{2, 5}},
	}})
}

// Submatches are ordered by start, then end; both orderings apply within
// one record's array and across merged records.
func TestSubmatchOrdering(t *testing.T) {
	idx := collect(t, "/w",
		matchEvent(t, textData("f"), textData("0123456789\n"), 1,
			submatch(textData("34"), 3, 5),
			submatch(textData("012"), 0, 3),
			submatch(textData("0"), 0, 1),
			submatch(textData("0123"), 0, 4)),
	)
	got := idx.Stops()
	if len(got) != 1 {
		t.Fatalf("Stops() = %d stops, want 1", len(got))
	}
	var ranges []searchindex.Range
	for _, s := range got[0].Submatches {
		ranges = append(ranges, s.Range)
	}
	want := []searchindex.Range{{0, 1}, {0, 3}, {0, 4}, {3, 5}}
	if !slices.Equal(ranges, want) {
		t.Fatalf("submatch order = %v, want %v", ranges, want)
	}
}

// Overlapping submatches within one record are all retained, and the
// prepared highlight coverage is their union.
func TestOverlappingSubmatchUnionCoverage(t *testing.T) {
	idx := collect(t, "/w",
		matchEvent(t, textData("f"), textData("abcdefghij\n"), 1,
			submatch(textData("abcde"), 0, 5),
			submatch(textData("defgh"), 3, 8),
			submatch(textData("j"), 9, 10)),
	)
	got := idx.Stops()
	if len(got) != 1 {
		t.Fatalf("Stops() = %d stops, want 1", len(got))
	}
	if len(got[0].Submatches) != 3 {
		t.Fatalf("overlapping submatches were dropped: %+v", got[0].Submatches)
	}
	want := []searchindex.Range{{0, 8}, {9, 10}}
	if !slices.Equal(got[0].Coverage, want) {
		t.Fatalf("Coverage = %v, want union %v", got[0].Coverage, want)
	}
}

// The index orders by unsigned raw path bytes, then ascending line
// number. Non-UTF-8 path bytes order deterministically against valid
// UTF-8: byte 0xff sorts after every ASCII byte.
func TestUnsignedRawByteOrdering(t *testing.T) {
	raw := []byte{'b', 0xff, 'z'}
	idx := collect(t, "/w",
		matchEvent(t, bytesData(raw), textData("x\n"), 5, submatch(textData("x"), 0, 1)),
		matchEvent(t, textData("b.txt"), textData("x\n"), 9, submatch(textData("x"), 0, 1)),
		matchEvent(t, textData("a.txt"), textData("x\n"), 4, submatch(textData("x"), 0, 1)),
		matchEvent(t, textData("a.txt"), textData("x\n"), 2, submatch(textData("x"), 0, 1)),
		matchEvent(t, bytesData(raw), textData("x\n"), 1, submatch(textData("x"), 0, 1)),
	)
	stops := idx.Stops()
	if len(stops) != 5 {
		t.Fatalf("Stops() = %d, want 5: %+v", len(stops), stops)
	}
	type pl struct {
		path string
		line int64
	}
	var order []pl
	for _, s := range stops {
		order = append(order, pl{string(s.Path), s.Line})
	}
	want := []pl{
		{"a.txt", 2}, {"a.txt", 4}, {"b.txt", 9}, {string(raw), 1}, {string(raw), 5},
	}
	if !slices.Equal(order, want) {
		t.Fatalf("index order = %v, want %v", order, want)
	}
}

// Relative result paths resolve against the invocation working directory
// without canonicalization; absolute paths and the raw reported bytes are
// both preserved.
func TestRelativePathResolution(t *testing.T) {
	idx := collect(t, "/work/dir",
		matchEvent(t, textData("rel/a.txt"), textData("x\n"), 1, submatch(textData("x"), 0, 1)),
		matchEvent(t, textData("./dot/a.txt"), textData("x\n"), 1, submatch(textData("x"), 0, 1)),
		matchEvent(t, textData("up/../a.txt"), textData("x\n"), 1, submatch(textData("x"), 0, 1)),
		matchEvent(t, textData("/abs/a.txt"), textData("x\n"), 1, submatch(textData("x"), 0, 1)),
	)
	stops := idx.Stops()
	got := make(map[string]string)
	for _, s := range stops {
		got[string(s.Path)] = string(s.Resolved)
	}
	want := map[string]string{
		"rel/a.txt":   "/work/dir/rel/a.txt",
		"./dot/a.txt": "/work/dir/./dot/a.txt", // not canonicalized
		"up/../a.txt": "/work/dir/up/../a.txt", // not canonicalized
		"/abs/a.txt":  "/abs/a.txt",
	}
	for p, w := range want {
		if got[p] != w {
			t.Errorf("Resolved[%q] = %q, want %q", p, got[p], w)
		}
	}
}

// Retained data: raw path bytes, line number, submatch byte ranges, and
// the recorded submatch bytes all survive verbatim, including bytes that
// are not valid UTF-8.
func TestRawBytesRetained(t *testing.T) {
	path := []byte{'n', 0xff, 'm'}
	mtext := []byte{0x80, 'k'}
	idx := collect(t, "/w",
		matchEvent(t, bytesData(path), bytesData([]byte{'x', 0x80, 'k', 'y', '\n'}), 42,
			submatch(bytesData(mtext), 1, 3)),
	)
	stops := idx.Stops()
	if len(stops) != 1 {
		t.Fatalf("Stops() = %d, want 1", len(stops))
	}
	s := stops[0]
	if !slices.Equal(s.Path, path) {
		t.Fatalf("Path = %q, want raw %q", s.Path, path)
	}
	if s.Line != 42 {
		t.Fatalf("Line = %d, want 42", s.Line)
	}
	if len(s.Submatches) != 1 || s.Submatches[0].Range != (searchindex.Range{1, 3}) ||
		!slices.Equal(s.Submatches[0].Text, mtext) {
		t.Fatalf("Submatches = %+v, want range [1,3) text %q", s.Submatches, mtext)
	}
}

// Every known event accepts a fixture conforming to the per-record schema
// matrix: begin, match, end (null and integer binary_offset), summary,
// and context (known but ignored). Ignored fields carry arbitrary data.
func TestSchemaMatrixHappyPath(t *testing.T) {
	idx := collect(t, "/w",
		// context before anything: known but ignored, no lifecycle effect.
		marshal(t, map[string]any{"type": "context", "data": map[string]any{
			"path": textData("ctx.txt"), "lines": textData("c\n"), "line_number": 1, "submatches": []any{},
		}}),
		beginEvent(t, textData("a.txt")),
		matchEvent(t, textData("a.txt"), textData("hit\n"), 1, submatch(textData("hit"), 0, 3)),
		endEvent(t, textData("a.txt"), nil),
		beginEvent(t, textData("b.txt")),
		matchEvent(t, textData("b.txt"), textData("hit\n"), 9, submatch(textData("hit"), 0, 3)),
		endEvent(t, textData("b.txt"), 128), // non-null binary_offset: valid record, excluded file
		summaryEvent(t),
	)
	stops := idx.Stops()
	if len(stops) != 1 {
		t.Fatalf("Stops() = %d, want 1: b.txt was binary-excluded: %+v", len(stops), stops)
	}
	if n := idx.BinaryExcluded(); n != 1 {
		t.Fatalf("BinaryExcluded() = %d, want 1", n)
	}
}

// Fields the matrix marks ignored (absolute_offset, stats, extra keys)
// never affect acceptance.
func TestIgnoredFieldsAccepted(t *testing.T) {
	idx := collect(t, "/w",
		marshal(t, map[string]any{
			"type": "match",
			"data": map[string]any{
				"path":            textData("a.txt"),
				"lines":           textData("hit\n"),
				"line_number":     1,
				"absolute_offset": "not a number", // ignored: never type-checked
				"submatches":      []any{submatch(textData("hit"), 0, 3)},
				"surprise":        map[string]any{"nested": []any{1, 2, 3}},
			},
			"top_level_extra": true,
		}),
	)
	if n := len(idx.Stops()); n != 1 {
		t.Fatalf("Stops() = %d, want 1", n)
	}
}

// Records that violate the per-record schema matrix are malformed: Add
// rejects them and indexes nothing. Skip/count accounting is Issue 10's;
// lifecycle validation is Issue 9's.
func TestMalformedRecordsRejected(t *testing.T) {
	validMatch := matchEvent(t, textData("a.txt"), textData("hit\n"), 1, submatch(textData("hit"), 0, 3))

	cases := []struct {
		name string
		rec  []byte
	}{
		{"invalid json", []byte("{not json")},
		{"empty record", []byte("")},
		{"json non-object", []byte("[1,2]")},
		{"missing type", marshal(t, map[string]any{"data": map[string]any{}})},
		{"non-string type", marshal(t, map[string]any{"type": 5, "data": map[string]any{}})},
		{"begin missing data", marshal(t, map[string]any{"type": "begin"})},
		{"begin missing path", marshal(t, map[string]any{"type": "begin", "data": map[string]any{}})},
		{"begin path invalid base64", marshal(t, map[string]any{"type": "begin",
			"data": map[string]any{"path": map[string]any{"bytes": "!!!"}}})},
		{"match missing path", marshal(t, map[string]any{"type": "match", "data": map[string]any{
			"lines": textData("x\n"), "line_number": 1, "submatches": []any{submatch(textData("x"), 0, 1)}}})},
		{"match missing lines", marshal(t, map[string]any{"type": "match", "data": map[string]any{
			"path": textData("a"), "line_number": 1, "submatches": []any{submatch(textData("x"), 0, 1)}}})},
		{"match missing line_number", marshal(t, map[string]any{"type": "match", "data": map[string]any{
			"path": textData("a"), "lines": textData("x\n"), "submatches": []any{submatch(textData("x"), 0, 1)}}})},
		{"match line_number zero", matchEvent(t, textData("a"), textData("x\n"), 0, submatch(textData("x"), 0, 1))},
		{"match line_number negative", matchEvent(t, textData("a"), textData("x\n"), -2, submatch(textData("x"), 0, 1))},
		{"match line_number non-integer", marshal(t, map[string]any{"type": "match", "data": map[string]any{
			"path": textData("a"), "lines": textData("x\n"), "line_number": 1.5,
			"submatches": []any{submatch(textData("x"), 0, 1)}}})},
		{"match line_number string", marshal(t, map[string]any{"type": "match", "data": map[string]any{
			"path": textData("a"), "lines": textData("x\n"), "line_number": "1",
			"submatches": []any{submatch(textData("x"), 0, 1)}}})},
		{"match missing submatches", marshal(t, map[string]any{"type": "match", "data": map[string]any{
			"path": textData("a"), "lines": textData("x\n"), "line_number": 1}})},
		{"match empty submatches", marshal(t, map[string]any{"type": "match", "data": map[string]any{
			"path": textData("a"), "lines": textData("x\n"), "line_number": 1, "submatches": []any{}}})},
		{"submatch missing match", marshal(t, map[string]any{"type": "match", "data": map[string]any{
			"path": textData("a"), "lines": textData("x\n"), "line_number": 1,
			"submatches": []any{map[string]any{"start": 0, "end": 1}}}})},
		{"submatch missing start", marshal(t, map[string]any{"type": "match", "data": map[string]any{
			"path": textData("a"), "lines": textData("x\n"), "line_number": 1,
			"submatches": []any{map[string]any{"match": textData("x"), "end": 1}}}})},
		{"submatch missing end", marshal(t, map[string]any{"type": "match", "data": map[string]any{
			"path": textData("a"), "lines": textData("x\n"), "line_number": 1,
			"submatches": []any{map[string]any{"match": textData("x"), "start": 0}}}})},
		{"submatch start negative", matchEvent(t, textData("a"), textData("hit\n"), 1, submatch(textData("hit"), -1, 3))},
		{"submatch start after end", matchEvent(t, textData("a"), textData("hit\n"), 1, submatch(textData("hit"), 2, 1))},
		{"submatch end past line", matchEvent(t, textData("a"), textData("hit\n"), 1, submatch(textData("hit"), 0, 99))},
		{"submatch invalid base64", marshal(t, map[string]any{"type": "match", "data": map[string]any{
			"path": textData("a"), "lines": textData("x\n"), "line_number": 1,
			"submatches": []any{submatch(map[string]any{"bytes": "!!!"}, 0, 1)}}})},
		{"end missing path", marshal(t, map[string]any{"type": "end",
			"data": map[string]any{"binary_offset": nil}})},
		{"end missing binary_offset", marshal(t, map[string]any{"type": "end",
			"data": map[string]any{"path": textData("a")}})},
		{"end negative binary_offset", marshal(t, map[string]any{"type": "end",
			"data": map[string]any{"path": textData("a"), "binary_offset": -1}})},
		{"end non-integer binary_offset", marshal(t, map[string]any{"type": "end",
			"data": map[string]any{"path": textData("a"), "binary_offset": "12"}})},
		{"summary missing data", marshal(t, map[string]any{"type": "summary"})},
		{"summary data not object", marshal(t, map[string]any{"type": "summary", "data": []any{1}})},
		{"summary data null", marshal(t, map[string]any{"type": "summary", "data": nil})},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			idx := searchindex.New("/w")
			if err := idx.Add(tc.rec); err == nil {
				t.Fatalf("Add(%s) accepted a malformed record", tc.rec)
			}
			// A malformed record must not disturb the surrounding stream.
			if err := idx.Add(validMatch); err != nil {
				t.Fatalf("Add(valid) after malformed = %v", err)
			}
			idx.Finish()
			if n := len(idx.Stops()); n != 1 {
				t.Fatalf("Stops() = %d, want 1: malformed record reached the index", n)
			}
		})
	}
}

// A string type outside the five known events is an unknown type, not a
// malformed record: it is ignored without error (Issue 10 counts it).
func TestUnknownTypesIgnored(t *testing.T) {
	idx := searchindex.New("/w")
	for _, rec := range [][]byte{
		marshal(t, map[string]any{"type": "weird", "data": map[string]any{"x": 1}}),
		marshal(t, map[string]any{"type": "stats", "data": "anything"}),
		marshal(t, map[string]any{"type": "matchx"}),
	} {
		if err := idx.Add(rec); err != nil {
			t.Fatalf("Add(%s) = %v, want unknown type ignored without error", rec, err)
		}
	}
	idx.Finish()
	if n := len(idx.Stops()); n != 0 {
		t.Fatalf("Stops() = %d, want 0", n)
	}
}

// A valid end event carrying a non-null binary_offset confirms its file
// binary: every match already collected for it is dropped and the file
// joins the distinct excluded count.
func TestBinaryEndDropsCollectedMatches(t *testing.T) {
	idx := collect(t, "/w",
		beginEvent(t, textData("bin.dat")),
		matchEvent(t, textData("bin.dat"), textData("hit\n"), 1, submatch(textData("hit"), 0, 3)),
		matchEvent(t, textData("bin.dat"), textData("hit again\n"), 4, submatch(textData("hit"), 0, 3)),
		endEvent(t, textData("bin.dat"), 1024),
		summaryEvent(t),
	)
	if n := len(idx.Stops()); n != 0 {
		t.Fatalf("Stops() = %d, want 0: a binary file kept its matches", n)
	}
	if n := len(idx.Files()); n != 0 {
		t.Fatalf("Files() = %d, want 0", n)
	}
	if n := idx.BinaryExcluded(); n != 1 {
		t.Fatalf("BinaryExcluded() = %d, want 1", n)
	}
	if n := idx.UsableResults(); n != 0 {
		t.Fatalf("UsableResults() = %d, want 0", n)
	}
}

// The exclusion count is over distinct files: two binary files count
// two, while a second binary end for an already-excluded path does not
// count again.
func TestBinaryExclusionCountsDistinctFiles(t *testing.T) {
	idx := collect(t, "/w",
		beginEvent(t, textData("a.bin")),
		matchEvent(t, textData("a.bin"), textData("x\n"), 1, submatch(textData("x"), 0, 1)),
		endEvent(t, textData("a.bin"), 7),
		beginEvent(t, textData("b.bin")),
		matchEvent(t, textData("b.bin"), textData("x\n"), 1, submatch(textData("x"), 0, 1)),
		endEvent(t, textData("b.bin"), 3),
		endEvent(t, textData("a.bin"), 99), // anomalous duplicate: still one file
		summaryEvent(t),
	)
	if n := idx.BinaryExcluded(); n != 2 {
		t.Fatalf("BinaryExcluded() = %d, want 2 distinct files", n)
	}
	if n := idx.UsableResults(); n != 0 {
		t.Fatalf("UsableResults() = %d, want 0", n)
	}
}

// Usable results are the retained stops after binary exclusion — never
// the number of match events received. The excluded file's matches leave
// no trace; the retained file browses normally.
func TestUsableResultsCountsRetainedStops(t *testing.T) {
	idx := collect(t, "/w",
		beginEvent(t, textData("a.bin")),
		matchEvent(t, textData("a.bin"), textData("x\n"), 1, submatch(textData("x"), 0, 1)),
		matchEvent(t, textData("a.bin"), textData("x\n"), 2, submatch(textData("x"), 0, 1)),
		endEvent(t, textData("a.bin"), 4),
		beginEvent(t, textData("b.txt")),
		matchEvent(t, textData("b.txt"), textData("y\n"), 9, submatch(textData("y"), 0, 1)),
		endEvent(t, textData("b.txt"), nil),
		summaryEvent(t),
	)
	if n := idx.UsableResults(); n != 1 {
		t.Fatalf("UsableResults() = %d, want the 1 retained stop, not the 3 match events", n)
	}
	if n := idx.BinaryExcluded(); n != 1 {
		t.Fatalf("BinaryExcluded() = %d, want 1", n)
	}
	var files []string
	for _, f := range idx.Files() {
		files = append(files, string(f))
	}
	if !slices.Equal(files, []string{"b.txt"}) {
		t.Fatalf("Files() = %v, want [b.txt]", files)
	}
}

// A match arriving after its file's binary-excluding end is not
// retained: exclusion drops the file, not only the matches seen so far.
// Issue 9 owns flagging the orphaned record; the index contract here is
// non-retention.
func TestMatchAfterBinaryEndNotRetained(t *testing.T) {
	idx := collect(t, "/w",
		beginEvent(t, textData("a.bin")),
		matchEvent(t, textData("a.bin"), textData("x\n"), 1, submatch(textData("x"), 0, 1)),
		endEvent(t, textData("a.bin"), 8),
		matchEvent(t, textData("a.bin"), textData("x\n"), 2, submatch(textData("x"), 0, 1)),
		summaryEvent(t),
	)
	if n := idx.UsableResults(); n != 0 {
		t.Fatalf("UsableResults() = %d, want 0: a match after the binary end was retained", n)
	}
	if n := idx.BinaryExcluded(); n != 1 {
		t.Fatalf("BinaryExcluded() = %d, want 1", n)
	}
}

// An end with a null binary_offset confirms nothing: the file's matches
// are retained and it is not counted as excluded.
func TestNullBinaryOffsetRetains(t *testing.T) {
	idx := collect(t, "/w",
		beginEvent(t, textData("a.txt")),
		matchEvent(t, textData("a.txt"), textData("x\n"), 1, submatch(textData("x"), 0, 1)),
		endEvent(t, textData("a.txt"), nil),
		summaryEvent(t),
	)
	if n := idx.UsableResults(); n != 1 {
		t.Fatalf("UsableResults() = %d, want 1", n)
	}
	if n := idx.BinaryExcluded(); n != 0 {
		t.Fatalf("BinaryExcluded() = %d, want 0", n)
	}
}

// Files reports the distinct raw paths in index order.
func TestFilesOrderedDistinct(t *testing.T) {
	idx := collect(t, "/w",
		matchEvent(t, textData("b.txt"), textData("x\n"), 3, submatch(textData("x"), 0, 1)),
		matchEvent(t, textData("a.txt"), textData("x\n"), 1, submatch(textData("x"), 0, 1)),
		matchEvent(t, textData("b.txt"), textData("x\n"), 1, submatch(textData("x"), 0, 1)),
		matchEvent(t, textData("a.txt"), textData("x\n"), 7, submatch(textData("x"), 0, 1)),
	)
	var files []string
	for _, f := range idx.Files() {
		files = append(files, string(f))
	}
	if want := []string{"a.txt", "b.txt"}; !slices.Equal(files, want) {
		t.Fatalf("Files() = %v, want %v", files, want)
	}
}
