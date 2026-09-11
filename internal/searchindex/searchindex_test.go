package searchindex_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"vrg/internal/searchindex"
)

// b64 encodes raw bytes as base64 for bytes-encoding fixtures.
func b64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// subSpec describes one submatch for fixture building: the match text and
// its byte range within the decoded line.
type subSpec struct {
	match string
	start int
	end   int
}

// textMatch builds a match record with text encoding for path, line, and
// every submatch's match field.
func textMatch(path, line string, lineNo int, subs ...subSpec) string {
	return matchRecord(
		map[string]any{"text": path},
		map[string]any{"text": line},
		lineNo, subs, false,
	)
}

// bytesMatch builds a match record with base64 bytes encoding for path,
// line, and every submatch's match field.
func bytesMatch(path, line []byte, lineNo int, subs ...subSpec) string {
	return matchRecord(
		map[string]any{"bytes": b64(path)},
		map[string]any{"bytes": b64(line)},
		lineNo, subs, true,
	)
}

// matchRecord assembles a match JSON record from the given path/line
// encodings and submatch specs.
func matchRecord(pathEnc, lineEnc map[string]any, lineNo int, subs []subSpec, bytesMatch bool) string {
	submatches := make([]map[string]any, len(subs))
	for i, s := range subs {
		var matchEnc map[string]any
		if bytesMatch {
			matchEnc = map[string]any{"bytes": b64([]byte(s.match))}
		} else {
			matchEnc = map[string]any{"text": s.match}
		}
		submatches[i] = map[string]any{
			"match": matchEnc,
			"start": s.start,
			"end":   s.end,
		}
	}
	rec := map[string]any{
		"type": "match",
		"data": map[string]any{
			"path":            pathEnc,
			"lines":           lineEnc,
			"line_number":     lineNo,
			"submatches":      submatches,
			"absolute_offset": 0,
		},
	}
	b, _ := json.Marshal(rec)
	return string(b)
}

// textBegin builds a begin record with a text-encoded path.
func textBegin(path string) string {
	rec := map[string]any{
		"type": "begin",
		"data": map[string]any{"path": map[string]any{"text": path}},
	}
	b, _ := json.Marshal(rec)
	return string(b)
}

// bytesBegin builds a begin record with a bytes-encoded path.
func bytesBegin(path []byte) string {
	rec := map[string]any{
		"type": "begin",
		"data": map[string]any{"path": map[string]any{"bytes": b64(path)}},
	}
	b, _ := json.Marshal(rec)
	return string(b)
}

// endRecord builds an end record. binaryOffset may be nil, an int, or any
// json value.
func endRecord(path string, binaryOffset any) string {
	data := map[string]any{
		"path":          map[string]any{"text": path},
		"binary_offset": binaryOffset,
	}
	rec := map[string]any{"type": "end", "data": data}
	b, _ := json.Marshal(rec)
	return string(b)
}

// summaryRecord builds a summary record with a data object.
func summaryRecord() string {
	rec := map[string]any{
		"type": "summary",
		"data": map[string]any{
			"elapsed_total": map[string]any{"human": "0.001s", "nanos": 1000000, "secs": 0},
			"stats":         map[string]any{"matches": 1, "matched_lines": 1},
		},
	}
	b, _ := json.Marshal(rec)
	return string(b)
}

// contextRecord builds a context record whose data payload is ignored.
func contextRecord() string {
	rec := map[string]any{
		"type": "context",
		"data": map[string]any{
			"path":        map[string]any{"text": "ignored.go"},
			"lines":       map[string]any{"text": "ignored\n"},
			"line_number": 99,
		},
	}
	b, _ := json.Marshal(rec)
	return string(b)
}

// build feeds records through a new builder and returns the index.
func build(t *testing.T, workdir string, records ...string) *searchindex.Index {
	t.Helper()
	b := searchindex.NewBuilder(workdir)
	for _, rec := range records {
		if err := b.Add([]byte(rec)); err != nil {
			t.Fatalf("Add failed for %q: %v", rec, err)
		}
	}
	return b.Build()
}

// stopStr renders a stop for failure messages.
func stopStr(s searchindex.Stop) string {
	return fmt.Sprintf("RawPath=%q Path=%q LineNumber=%d Line=%q Submatches=%v Coverage=%v",
		s.RawPath, s.Path, s.LineNumber, s.Line, s.Submatches, s.Coverage)
}

// assertStops requires the index stops to equal want in order.
func assertStops(t *testing.T, idx *searchindex.Index, want []searchindex.Stop) {
	t.Helper()
	got := idx.Stops()
	if len(got) != len(want) {
		t.Fatalf("Len = %d, want %d\ngot: %v\nwant: %v", len(got), len(want), stopsStr(got), stopsStr(want))
	}
	for i := range got {
		if !stopEqual(got[i], want[i]) {
			t.Fatalf("stop %d mismatch:\ngot:  %s\nwant: %s", i, stopStr(got[i]), stopStr(want[i]))
		}
	}
}

func stopsStr(stops []searchindex.Stop) string {
	parts := make([]string, len(stops))
	for i, s := range stops {
		parts[i] = "  " + stopStr(s)
	}
	return strings.Join(parts, "\n")
}

func stopEqual(got, want searchindex.Stop) bool {
	if !bytes.Equal(got.RawPath, want.RawPath) {
		return false
	}
	if !bytes.Equal(got.Path, want.Path) {
		return false
	}
	if got.LineNumber != want.LineNumber {
		return false
	}
	if !bytes.Equal(got.Line, want.Line) {
		return false
	}
	if len(got.Submatches) != len(want.Submatches) {
		return false
	}
	for i := range got.Submatches {
		if !submatchEqual(got.Submatches[i], want.Submatches[i]) {
			return false
		}
	}
	if len(got.Coverage) != len(want.Coverage) {
		return false
	}
	for i := range got.Coverage {
		if got.Coverage[i] != want.Coverage[i] {
			return false
		}
	}
	return true
}

func submatchEqual(got, want searchindex.Submatch) bool {
	return bytes.Equal(got.Match, want.Match) && got.Start == want.Start && got.End == want.End
}

func wantStop(rawPath, path string, lineNo int, line string, subs []searchindex.Submatch, coverage []searchindex.Range) searchindex.Stop {
	return searchindex.Stop{
		RawPath:    []byte(rawPath),
		Path:       []byte(path),
		LineNumber: lineNo,
		Line:       []byte(line),
		Submatches: subs,
		Coverage:   coverage,
	}
}

func sub(match string, start, end int) searchindex.Submatch {
	return searchindex.Submatch{Match: []byte(match), Start: start, End: end}
}

func rng(start, end int) searchindex.Range {
	return searchindex.Range{Start: start, End: end}
}

// TestEventTypesAccepted verifies that begin, match, end, summary, and
// context records are all accepted without error and that only match
// records produce navigation stops. Context is ignored.
func TestEventTypesAccepted(t *testing.T) {
	idx := build(t, "/work",
		textBegin("src/a.go"),
		textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
		endRecord("src/a.go", nil),
		summaryRecord(),
		contextRecord(),
	)
	if idx.Len() != 1 {
		t.Fatalf("Len = %d, want 1 (only match records produce stops)", idx.Len())
	}
	assertStops(t, idx, []searchindex.Stop{
		wantStop("src/a.go", "/work/src/a.go", 1, "hello\n",
			[]searchindex.Submatch{sub("hello", 0, 5)},
			[]searchindex.Range{rng(0, 5)}),
	})
}

// TestTextEncoding verifies that text-encoded paths, lines, and submatch
// text are decoded into raw bytes and retained.
func TestTextEncoding(t *testing.T) {
	idx := build(t, "/work",
		textMatch("src/a.go", "hello world\n", 3, subSpec{"hello", 0, 5}),
	)
	assertStops(t, idx, []searchindex.Stop{
		wantStop("src/a.go", "/work/src/a.go", 3, "hello world\n",
			[]searchindex.Submatch{sub("hello", 0, 5)},
			[]searchindex.Range{rng(0, 5)}),
	})
}

// TestBytesEncoding verifies that base64 bytes-encoded paths, lines, and
// submatch text are decoded into the same raw bytes as text encoding.
func TestBytesEncoding(t *testing.T) {
	path := []byte("src/a.go")
	line := []byte("hello world\n")
	idx := build(t, "/work",
		bytesMatch(path, line, 3, subSpec{"hello", 0, 5}),
	)
	assertStops(t, idx, []searchindex.Stop{
		wantStop("src/a.go", "/work/src/a.go", 3, "hello world\n",
			[]searchindex.Submatch{sub("hello", 0, 5)},
			[]searchindex.Range{rng(0, 5)}),
	})
}

// TestTextAndBytesProduceSameValue verifies that a text record and a bytes
// record carrying the same decoded bytes produce identical stops — proving
// cross-encoding path identity rather than inferring it from separate
// fixtures.
func TestTextAndBytesProduceSameValue(t *testing.T) {
	path := "src/a.go"
	line := "hello world\n"
	for _, tc := range []struct {
		name    string
		records []string
	}{
		{"text then bytes", []string{
			textMatch(path, line, 1, subSpec{"hello", 0, 5}),
			bytesMatch([]byte(path), []byte(line), 1, subSpec{"world", 6, 11}),
		}},
		{"bytes then text", []string{
			bytesMatch([]byte(path), []byte(line), 1, subSpec{"world", 6, 11}),
			textMatch(path, line, 1, subSpec{"hello", 0, 5}),
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			idx := build(t, "/work", tc.records...)
			assertStops(t, idx, []searchindex.Stop{
				wantStop(path, "/work/src/a.go", 1, line,
					[]searchindex.Submatch{
						sub("hello", 0, 5),
						sub("world", 6, 11),
					},
					[]searchindex.Range{rng(0, 5), rng(6, 11)}),
			})
		})
	}
}

// TestSameLineMerging verifies that multiple match records for the same
// raw path and line number merge into one navigation stop with all
// submatches collected.
func TestSameLineMerging(t *testing.T) {
	idx := build(t, "/work",
		textMatch("src/a.go", "hello world\n", 1, subSpec{"hello", 0, 5}),
		textMatch("src/a.go", "hello world\n", 1, subSpec{"world", 6, 11}),
	)
	if idx.Len() != 1 {
		t.Fatalf("Len = %d, want 1 (same path+line must merge)", idx.Len())
	}
	assertStops(t, idx, []searchindex.Stop{
		wantStop("src/a.go", "/work/src/a.go", 1, "hello world\n",
			[]searchindex.Submatch{
				sub("hello", 0, 5),
				sub("world", 6, 11),
			},
			[]searchindex.Range{rng(0, 5), rng(6, 11)}),
	})
}

// TestMixedEncodingSameValueMerging verifies that a text record and a bytes
// record carrying the same decoded bytes for the same path and line merge
// into one stop — proving cross-encoding path identity.
func TestMixedEncodingSameValueMerging(t *testing.T) {
	path := "src/a.go"
	line := "hello world\n"
	idx := build(t, "/work",
		textMatch(path, line, 1, subSpec{"hello", 0, 5}),
		bytesMatch([]byte(path), []byte(line), 1, subSpec{"world", 6, 11}),
	)
	if idx.Len() != 1 {
		t.Fatalf("Len = %d, want 1 (mixed encoding same-value must merge)", idx.Len())
	}
	assertStops(t, idx, []searchindex.Stop{
		wantStop(path, "/work/src/a.go", 1, line,
			[]searchindex.Submatch{
				sub("hello", 0, 5),
				sub("world", 6, 11),
			},
			[]searchindex.Range{rng(0, 5), rng(6, 11)}),
	})
}

// TestSubmatchOrdering verifies that submatches within a stop are sorted by
// byte start then end, regardless of the order they appear in the JSON.
func TestSubmatchOrdering(t *testing.T) {
	cases := []struct {
		name string
		subs []subSpec
		want []searchindex.Submatch
	}{
		{
			"reverse order sorted",
			[]subSpec{{"world", 6, 11}, {"hello", 0, 5}},
			[]searchindex.Submatch{sub("hello", 0, 5), sub("world", 6, 11)},
		},
		{
			"same start sorted by end",
			[]subSpec{{"long", 0, 10}, {"short", 0, 5}},
			[]searchindex.Submatch{sub("short", 0, 5), sub("long", 0, 10)},
		},
		{
			"same start and end kept both",
			[]subSpec{{"dup", 0, 5}, {"dup", 0, 5}},
			[]searchindex.Submatch{sub("dup", 0, 5), sub("dup", 0, 5)},
		},
		{
			"three out of order",
			[]subSpec{{"c", 6, 11}, {"a", 0, 5}, {"b", 3, 4}},
			[]searchindex.Submatch{sub("a", 0, 5), sub("b", 3, 4), sub("c", 6, 11)},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			idx := build(t, "/work",
				textMatch("src/a.go", "hello world\n", 1, tc.subs...),
			)
			stops := idx.Stops()
			if len(stops) != 1 {
				t.Fatalf("Len = %d, want 1", len(stops))
			}
			if len(stops[0].Submatches) != len(tc.want) {
				t.Fatalf("Submatches len = %d, want %d: %v", len(stops[0].Submatches), len(tc.want), stops[0].Submatches)
			}
			for i := range stops[0].Submatches {
				if !submatchEqual(stops[0].Submatches[i], tc.want[i]) {
					t.Fatalf("Submatch %d = %v, want %v", i, stops[0].Submatches[i], tc.want[i])
				}
			}
		})
	}
}

// TestSubmatchOrderingAcrossRecords verifies that submatches from merged
// records are also sorted by (start, end).
func TestSubmatchOrderingAcrossRecords(t *testing.T) {
	idx := build(t, "/work",
		textMatch("src/a.go", "hello world\n", 1, subSpec{"world", 6, 11}),
		textMatch("src/a.go", "hello world\n", 1, subSpec{"hello", 0, 5}),
	)
	assertStops(t, idx, []searchindex.Stop{
		wantStop("src/a.go", "/work/src/a.go", 1, "hello world\n",
			[]searchindex.Submatch{
				sub("hello", 0, 5),
				sub("world", 6, 11),
			},
			[]searchindex.Range{rng(0, 5), rng(6, 11)}),
	})
}

// TestOverlappingSubmatchesRetained verifies that overlapping submatches
// within one record are all retained, with the prepared highlight coverage
// asserted as their union without dropping either original range.
func TestOverlappingSubmatchesRetained(t *testing.T) {
	cases := []struct {
		name     string
		subs     []subSpec
		wantSubs []searchindex.Submatch
		wantCov  []searchindex.Range
	}{
		{
			"overlapping ranges union",
			[]subSpec{{"hello", 0, 5}, {"lo wo", 3, 8}},
			[]searchindex.Submatch{sub("hello", 0, 5), sub("lo wo", 3, 8)},
			[]searchindex.Range{rng(0, 8)},
		},
		{
			"contained range union",
			[]subSpec{{"hello world", 0, 11}, {"world", 6, 11}},
			[]searchindex.Submatch{sub("hello world", 0, 11), sub("world", 6, 11)},
			[]searchindex.Range{rng(0, 11)},
		},
		{
			"adjacent ranges merge",
			[]subSpec{{"hello", 0, 5}, {"world", 5, 10}},
			[]searchindex.Submatch{sub("hello", 0, 5), sub("world", 5, 10)},
			[]searchindex.Range{rng(0, 10)},
		},
		{
			"non-overlapping stay separate",
			[]subSpec{{"hello", 0, 5}, {"world", 6, 11}},
			[]searchindex.Submatch{sub("hello", 0, 5), sub("world", 6, 11)},
			[]searchindex.Range{rng(0, 5), rng(6, 11)},
		},
		{
			"three overlapping union",
			[]subSpec{{"a", 0, 3}, {"b", 2, 7}, {"c", 5, 10}},
			[]searchindex.Submatch{sub("a", 0, 3), sub("b", 2, 7), sub("c", 5, 10)},
			[]searchindex.Range{rng(0, 10)},
		},
		{
			"two groups overlapping",
			[]subSpec{{"a", 0, 3}, {"b", 5, 8}, {"c", 2, 6}},
			[]searchindex.Submatch{sub("a", 0, 3), sub("c", 2, 6), sub("b", 5, 8)},
			[]searchindex.Range{rng(0, 8)},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			idx := build(t, "/work",
				textMatch("src/a.go", "hello world\n", 1, tc.subs...),
			)
			stops := idx.Stops()
			if len(stops) != 1 {
				t.Fatalf("Len = %d, want 1", len(stops))
			}
			if len(stops[0].Submatches) != len(tc.wantSubs) {
				t.Fatalf("Submatches len = %d, want %d: %v", len(stops[0].Submatches), len(tc.wantSubs), stops[0].Submatches)
			}
			for i := range stops[0].Submatches {
				if !submatchEqual(stops[0].Submatches[i], tc.wantSubs[i]) {
					t.Fatalf("Submatch %d = %v, want %v (both original ranges must be retained)", i, stops[0].Submatches[i], tc.wantSubs[i])
				}
			}
			if len(stops[0].Coverage) != len(tc.wantCov) {
				t.Fatalf("Coverage len = %d, want %d: %v", len(stops[0].Coverage), len(tc.wantCov), stops[0].Coverage)
			}
			for i := range stops[0].Coverage {
				if stops[0].Coverage[i] != tc.wantCov[i] {
					t.Fatalf("Coverage %d = %v, want %v (union without dropping either range)", i, stops[0].Coverage[i], tc.wantCov[i])
				}
			}
		})
	}
}

// TestRawByteRetention verifies that raw path bytes, line numbers, submatch
// byte ranges, and recorded submatch bytes are all retained in the index.
func TestRawByteRetention(t *testing.T) {
	path := "src/sub/a.go"
	line := "hello world\n"
	idx := build(t, "/work",
		textMatch(path, line, 42, subSpec{"hello", 0, 5}),
	)
	stops := idx.Stops()
	if len(stops) != 1 {
		t.Fatalf("Len = %d, want 1", len(stops))
	}
	s := stops[0]
	if !bytes.Equal(s.RawPath, []byte(path)) {
		t.Fatalf("RawPath = %q, want %q", s.RawPath, path)
	}
	if s.LineNumber != 42 {
		t.Fatalf("LineNumber = %d, want 42", s.LineNumber)
	}
	if !bytes.Equal(s.Line, []byte(line)) {
		t.Fatalf("Line = %q, want %q", s.Line, line)
	}
	if len(s.Submatches) != 1 {
		t.Fatalf("Submatches len = %d, want 1", len(s.Submatches))
	}
	sm := s.Submatches[0]
	if sm.Start != 0 || sm.End != 5 {
		t.Fatalf("Submatch range = [%d, %d), want [0, 5)", sm.Start, sm.End)
	}
	if !bytes.Equal(sm.Match, []byte("hello")) {
		t.Fatalf("Submatch Match = %q, want %q", sm.Match, "hello")
	}
}

// TestRawByteRetentionNonUTF8 verifies that non-UTF-8 raw path bytes and
// line bytes are retained verbatim.
func TestRawByteRetentionNonUTF8(t *testing.T) {
	path := []byte("src/\xff/a.go")
	line := []byte("hello \xff world\n")
	idx := build(t, "/work",
		bytesMatch(path, line, 1, subSpec{"hello", 0, 5}),
	)
	stops := idx.Stops()
	if len(stops) != 1 {
		t.Fatalf("Len = %d, want 1", len(stops))
	}
	s := stops[0]
	if !bytes.Equal(s.RawPath, path) {
		t.Fatalf("RawPath = %q, want %q", s.RawPath, path)
	}
	if !bytes.Equal(s.Line, line) {
		t.Fatalf("Line = %q, want %q", s.Line, line)
	}
}

// TestIndexOrdering verifies that stops are ordered by unsigned raw path
// bytes then ascending line number.
func TestIndexOrdering(t *testing.T) {
	idx := build(t, "/work",
		textMatch("c.txt", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("a.txt", "x\n", 3, subSpec{"x", 0, 1}),
		textMatch("a.txt", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("b.txt", "x\n", 2, subSpec{"x", 0, 1}),
		textMatch("a.txt", "x\n", 1, subSpec{"x", 0, 1}),
	)
	stops := idx.Stops()
	wantPaths := []string{"a.txt", "a.txt", "b.txt", "c.txt"}
	wantLines := []int{1, 3, 2, 1}
	if len(stops) != len(wantPaths) {
		t.Fatalf("Len = %d, want %d", len(stops), len(wantPaths))
	}
	for i, s := range stops {
		if string(s.RawPath) != wantPaths[i] {
			t.Fatalf("stop %d RawPath = %q, want %q", i, s.RawPath, wantPaths[i])
		}
		if s.LineNumber != wantLines[i] {
			t.Fatalf("stop %d LineNumber = %d, want %d", i, s.LineNumber, wantLines[i])
		}
	}
}

// TestNonUTF8PathOrdering verifies that non-UTF-8 path bytes are ordered
// deterministically against valid UTF-8 by unsigned byte comparison.
func TestNonUTF8PathOrdering(t *testing.T) {
	// 0xFF sorts after all ASCII/UTF-8 bytes in unsigned comparison.
	paths := [][]byte{
		[]byte("z.txt"),
		[]byte("\xff.txt"),
		[]byte("a.txt"),
		[]byte{0x80, 0x01, '.', 't', 'x', 't'},
	}
	idx := build(t, "/work",
		bytesMatch(paths[0], []byte("x\n"), 1, subSpec{"x", 0, 1}),
		bytesMatch(paths[1], []byte("x\n"), 1, subSpec{"x", 0, 1}),
		bytesMatch(paths[2], []byte("x\n"), 1, subSpec{"x", 0, 1}),
		bytesMatch(paths[3], []byte("x\n"), 1, subSpec{"x", 0, 1}),
	)
	stops := idx.Stops()
	// Expected unsigned byte order: 'a' (0x61) < 'z' (0x7A) < 0x80 < 0xFF
	wantOrder := [][]byte{paths[2], paths[0], paths[3], paths[1]}
	if len(stops) != len(wantOrder) {
		t.Fatalf("Len = %d, want %d", len(stops), len(wantOrder))
	}
	for i, s := range stops {
		if !bytes.Equal(s.RawPath, wantOrder[i]) {
			t.Fatalf("stop %d RawPath = %q, want %q (unsigned byte ordering)", i, s.RawPath, wantOrder[i])
		}
	}
}

// TestRelativePathResolution verifies that relative result paths are
// resolved against the working directory without canonicalization.
func TestRelativePathResolution(t *testing.T) {
	cases := []struct {
		name     string
		workdir  string
		rawPath  string
		wantPath string
	}{
		{"simple relative", "/home/chris/vrg", "src/a.go", "/home/chris/vrg/src/a.go"},
		{"nested relative", "/work", "src/sub/a.go", "/work/src/sub/a.go"},
		{"dotdot not canonicalized", "/work", "src/../a.go", "/work/src/../a.go"},
		{"dot not canonicalized", "/work", "./a.go", "/work/./a.go"},
		{"trailing slash workdir", "/work/", "src/a.go", "/work/src/a.go"},
		{"absolute unchanged", "/work", "/abs/path/a.go", "/abs/path/a.go"},
		{"root path", "/", "a.go", "/a.go"},
		{"root workdir relative", "/", "src/a.go", "/src/a.go"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			idx := build(t, tc.workdir,
				textMatch(tc.rawPath, "x\n", 1, subSpec{"x", 0, 1}),
			)
			stops := idx.Stops()
			if len(stops) != 1 {
				t.Fatalf("Len = %d, want 1", len(stops))
			}
			if string(stops[0].Path) != tc.wantPath {
				t.Fatalf("Path = %q, want %q (resolved without canonicalization)", stops[0].Path, tc.wantPath)
			}
		})
	}
}

// TestRelativePathResolutionNonUTF8 verifies that non-UTF-8 relative paths
// are resolved against the working directory.
func TestRelativePathResolutionNonUTF8(t *testing.T) {
	rawPath := []byte("src/\xff/a.go")
	idx := build(t, "/work",
		bytesMatch(rawPath, []byte("x\n"), 1, subSpec{"x", 0, 1}),
	)
	stops := idx.Stops()
	if len(stops) != 1 {
		t.Fatalf("Len = %d, want 1", len(stops))
	}
	want := []byte("/work/src/\xff/a.go")
	if !bytes.Equal(stops[0].Path, want) {
		t.Fatalf("Path = %q, want %q", stops[0].Path, want)
	}
}

// TestSchemaMatrixConformance verifies that happy-path fixtures for every
// event type conform to the per-record schema matrix and its range
// constraints.
func TestSchemaMatrixConformance(t *testing.T) {
	t.Run("begin text path", func(t *testing.T) {
		b := searchindex.NewBuilder("/work")
		if err := b.Add([]byte(textBegin("src/a.go"))); err != nil {
			t.Fatalf("Add begin text: %v", err)
		}
	})

	t.Run("begin bytes path", func(t *testing.T) {
		b := searchindex.NewBuilder("/work")
		if err := b.Add([]byte(bytesBegin([]byte("src/a.go")))); err != nil {
			t.Fatalf("Add begin bytes: %v", err)
		}
	})

	t.Run("match all required fields", func(t *testing.T) {
		line := "hello world\n"
		b := searchindex.NewBuilder("/work")
		if err := b.Add([]byte(textMatch("src/a.go", line, 1, subSpec{"hello", 0, 5}))); err != nil {
			t.Fatalf("Add match: %v", err)
		}
	})

	t.Run("match line_number minimum 1", func(t *testing.T) {
		b := searchindex.NewBuilder("/work")
		if err := b.Add([]byte(textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}))); err != nil {
			t.Fatalf("Add match line_number=1: %v", err)
		}
	})

	t.Run("match start 0 end equals line length", func(t *testing.T) {
		line := "hello\n"
		b := searchindex.NewBuilder("/work")
		if err := b.Add([]byte(textMatch("src/a.go", line, 1, subSpec{"hello\n", 0, len(line)}))); err != nil {
			t.Fatalf("Add match start=0 end=lineLen: %v", err)
		}
	})

	t.Run("match zero-width submatch start equals end", func(t *testing.T) {
		b := searchindex.NewBuilder("/work")
		if err := b.Add([]byte(textMatch("src/a.go", "hello\n", 1, subSpec{"", 3, 3}))); err != nil {
			t.Fatalf("Add zero-width submatch: %v", err)
		}
	})

	t.Run("match bytes encoding all fields", func(t *testing.T) {
		b := searchindex.NewBuilder("/work")
		if err := b.Add([]byte(bytesMatch([]byte("src/a.go"), []byte("hello\n"), 1, subSpec{"hello", 0, 5}))); err != nil {
			t.Fatalf("Add bytes match: %v", err)
		}
	})

	t.Run("end binary_offset null", func(t *testing.T) {
		b := searchindex.NewBuilder("/work")
		if err := b.Add([]byte(endRecord("src/a.go", nil))); err != nil {
			t.Fatalf("Add end null offset: %v", err)
		}
	})

	t.Run("end binary_offset integer", func(t *testing.T) {
		b := searchindex.NewBuilder("/work")
		if err := b.Add([]byte(endRecord("src/a.go", 42))); err != nil {
			t.Fatalf("Add end integer offset: %v", err)
		}
	})

	t.Run("summary with data object", func(t *testing.T) {
		b := searchindex.NewBuilder("/work")
		if err := b.Add([]byte(summaryRecord())); err != nil {
			t.Fatalf("Add summary: %v", err)
		}
	})

	t.Run("context ignored", func(t *testing.T) {
		b := searchindex.NewBuilder("/work")
		if err := b.Add([]byte(contextRecord())); err != nil {
			t.Fatalf("Add context: %v", err)
		}
		if b.Build().Len() != 0 {
			t.Fatal("context record produced a stop")
		}
	})

	t.Run("full stream lifecycle", func(t *testing.T) {
		b := searchindex.NewBuilder("/work")
		records := []string{
			textBegin("src/a.go"),
			textMatch("src/a.go", "hello\n", 1, subSpec{"hello", 0, 5}),
			endRecord("src/a.go", nil),
			summaryRecord(),
			contextRecord(),
		}
		for _, rec := range records {
			if err := b.Add([]byte(rec)); err != nil {
				t.Fatalf("Add %q: %v", rec, err)
			}
		}
		idx := b.Build()
		if idx.Len() != 1 {
			t.Fatalf("Len = %d, want 1", idx.Len())
		}
	})
}

// TestMultipleFilesMultipleLines verifies the full index with multiple
// files and multiple lines per file, checking ordering and merging.
func TestMultipleFilesMultipleLines(t *testing.T) {
	idx := build(t, "/work",
		textMatch("b.go", "line1\n", 1, subSpec{"line1", 0, 5}),
		textMatch("a.go", "line2\n", 5, subSpec{"line2", 0, 5}),
		textMatch("a.go", "line1\n", 1, subSpec{"line1", 0, 5}),
		textMatch("a.go", "line1\n", 1, subSpec{"ne1", 1, 4}),
		textMatch("b.go", "line2\n", 2, subSpec{"line2", 0, 5}),
	)
	assertStops(t, idx, []searchindex.Stop{
		wantStop("a.go", "/work/a.go", 1, "line1\n",
			[]searchindex.Submatch{sub("line1", 0, 5), sub("ne1", 1, 4)},
			[]searchindex.Range{rng(0, 5)}),
		wantStop("a.go", "/work/a.go", 5, "line2\n",
			[]searchindex.Submatch{sub("line2", 0, 5)},
			[]searchindex.Range{rng(0, 5)}),
		wantStop("b.go", "/work/b.go", 1, "line1\n",
			[]searchindex.Submatch{sub("line1", 0, 5)},
			[]searchindex.Range{rng(0, 5)}),
		wantStop("b.go", "/work/b.go", 2, "line2\n",
			[]searchindex.Submatch{sub("line2", 0, 5)},
			[]searchindex.Range{rng(0, 5)}),
	})
}

// TestEmptyIndex verifies that an index with no match records has zero
// stops.
func TestEmptyIndex(t *testing.T) {
	idx := build(t, "/work",
		textBegin("src/a.go"),
		endRecord("src/a.go", nil),
		summaryRecord(),
	)
	if idx.Len() != 0 {
		t.Fatalf("Len = %d, want 0", idx.Len())
	}
	if len(idx.Stops()) != 0 {
		t.Fatalf("Stops = %v, want empty", idx.Stops())
	}
}

// TestLineBytesWithTerminator verifies that line bytes including the
// terminator are retained and submatch ranges extend to the full line
// byte length.
func TestLineBytesWithTerminator(t *testing.T) {
	line := "hello world\r\n"
	idx := build(t, "/work",
		textMatch("src/a.go", line, 1, subSpec{"hello", 0, 5}, subSpec{"\r\n", 11, 13}),
	)
	assertStops(t, idx, []searchindex.Stop{
		wantStop("src/a.go", "/work/src/a.go", 1, line,
			[]searchindex.Submatch{sub("hello", 0, 5), sub("\r\n", 11, 13)},
			[]searchindex.Range{rng(0, 5), rng(11, 13)}),
	})
}

// TestSubmatchBytesEncoding verifies that bytes-encoded submatch match
// fields are decoded to the same raw bytes as text encoding.
func TestSubmatchBytesEncoding(t *testing.T) {
	path := []byte("src/a.go")
	line := []byte("hello world\n")
	idx := build(t, "/work",
		bytesMatch(path, line, 1, subSpec{"hello", 0, 5}),
	)
	stops := idx.Stops()
	if len(stops) != 1 {
		t.Fatalf("Len = %d, want 1", len(stops))
	}
	sm := stops[0].Submatches
	if len(sm) != 1 {
		t.Fatalf("Submatches len = %d, want 1", len(sm))
	}
	if !bytes.Equal(sm[0].Match, []byte("hello")) {
		t.Fatalf("Match = %q, want %q", sm[0].Match, "hello")
	}
}

// TestMixedEncodingSubmatchText verifies that text and bytes submatch
// encodings within the same stop produce the same match bytes.
func TestMixedEncodingSubmatchText(t *testing.T) {
	path := "src/a.go"
	line := "hello world\n"
	idx := build(t, "/work",
		textMatch(path, line, 1, subSpec{"hello", 0, 5}),
		bytesMatch([]byte(path), []byte(line), 1, subSpec{"world", 6, 11}),
	)
	stops := idx.Stops()
	if len(stops) != 1 {
		t.Fatalf("Len = %d, want 1", len(stops))
	}
	if len(stops[0].Submatches) != 2 {
		t.Fatalf("Submatches len = %d, want 2", len(stops[0].Submatches))
	}
	if !bytes.Equal(stops[0].Submatches[0].Match, []byte("hello")) {
		t.Fatalf("Submatch 0 Match = %q, want %q", stops[0].Submatches[0].Match, "hello")
	}
	if !bytes.Equal(stops[0].Submatches[1].Match, []byte("world")) {
		t.Fatalf("Submatch 1 Match = %q, want %q", stops[0].Submatches[1].Match, "world")
	}
}

// TestCoverageOrdering verifies that the prepared coverage ranges are
// sorted by start and non-overlapping.
func TestCoverageOrdering(t *testing.T) {
	idx := build(t, "/work",
		textMatch("src/a.go", "hello world foo\n", 1,
			subSpec{"foo", 12, 15},
			subSpec{"hello", 0, 5},
			subSpec{"world", 6, 11},
		),
	)
	stops := idx.Stops()
	if len(stops) != 1 {
		t.Fatalf("Len = %d, want 1", len(stops))
	}
	cov := stops[0].Coverage
	want := []searchindex.Range{rng(0, 5), rng(6, 11), rng(12, 15)}
	if len(cov) != len(want) {
		t.Fatalf("Coverage len = %d, want %d: %v", len(cov), len(want), cov)
	}
	for i := range cov {
		if cov[i] != want[i] {
			t.Fatalf("Coverage %d = %v, want %v", i, cov[i], want[i])
		}
	}
}

// TestBuilderIsReusable verifies that Build can be called and the index
// reflects all added records.
func TestBuilderIsReusable(t *testing.T) {
	b := searchindex.NewBuilder("/work")
	_ = b.Add([]byte(textMatch("a.go", "x\n", 1, subSpec{"x", 0, 1})))
	idx1 := b.Build()
	_ = b.Add([]byte(textMatch("b.go", "x\n", 1, subSpec{"x", 0, 1})))
	idx2 := b.Build()
	if idx1.Len() != 1 {
		t.Fatalf("idx1 Len = %d, want 1", idx1.Len())
	}
	if idx2.Len() != 2 {
		t.Fatalf("idx2 Len = %d, want 2", idx2.Len())
	}
	// Verify ordering.
	stops := idx2.Stops()
	if string(stops[0].RawPath) != "a.go" || string(stops[1].RawPath) != "b.go" {
		t.Fatalf("Ordering wrong: %q %q", stops[0].RawPath, stops[1].RawPath)
	}
}

// TestStopsReturnsCopy verifies that modifying the returned stops slice
// does not affect the index's internal state.
func TestStopsReturnsCopy(t *testing.T) {
	idx := build(t, "/work",
		textMatch("a.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	stops1 := idx.Stops()
	stops1[0].LineNumber = 999
	stops2 := idx.Stops()
	if stops2[0].LineNumber == 999 {
		t.Fatal("modifying returned stops affected index state")
	}
}

// TestLineBytesRetentionFromBytesEncoding verifies that bytes-encoded line
// data with non-UTF-8 content is retained verbatim.
func TestLineBytesRetentionFromBytesEncoding(t *testing.T) {
	line := []byte("hello \xff\xfe\n")
	idx := build(t, "/work",
		bytesMatch([]byte("src/a.go"), line, 1, subSpec{"hello", 0, 5}),
	)
	stops := idx.Stops()
	if len(stops) != 1 {
		t.Fatalf("Len = %d, want 1", len(stops))
	}
	if !bytes.Equal(stops[0].Line, line) {
		t.Fatalf("Line = %q, want %q", stops[0].Line, line)
	}
}

// TestSubmatchRangesAcrossMerging verifies that submatch ranges from
// multiple records merging into the same stop are all retained and
// correctly reference byte offsets within the line.
func TestSubmatchRangesAcrossMerging(t *testing.T) {
	line := "hello world\n"
	idx := build(t, "/work",
		textMatch("src/a.go", line, 1, subSpec{"world", 6, 11}),
		textMatch("src/a.go", line, 1, subSpec{"hello", 0, 5}),
		textMatch("src/a.go", line, 1, subSpec{"o w", 4, 7}),
	)
	stops := idx.Stops()
	if len(stops) != 1 {
		t.Fatalf("Len = %d, want 1", len(stops))
	}
	s := stops[0]
	// Submatches sorted by (start, end): [0,5), [4,7), [6,11)
	wantSubs := []searchindex.Submatch{
		sub("hello", 0, 5),
		sub("o w", 4, 7),
		sub("world", 6, 11),
	}
	if len(s.Submatches) != len(wantSubs) {
		t.Fatalf("Submatches len = %d, want %d: %v", len(s.Submatches), len(wantSubs), s.Submatches)
	}
	for i := range wantSubs {
		if !submatchEqual(s.Submatches[i], wantSubs[i]) {
			t.Fatalf("Submatch %d = %v, want %v", i, s.Submatches[i], wantSubs[i])
		}
	}
	// Coverage: [0,5) and [4,7) overlap → [0,7); [6,11) overlaps → [0,11)
	wantCov := []searchindex.Range{rng(0, 11)}
	if len(s.Coverage) != len(wantCov) {
		t.Fatalf("Coverage len = %d, want %d: %v", len(s.Coverage), len(wantCov), s.Coverage)
	}
	for i := range wantCov {
		if s.Coverage[i] != wantCov[i] {
			t.Fatalf("Coverage %d = %v, want %v", i, s.Coverage[i], wantCov[i])
		}
	}
}

// TestPathIdentityForMerging verifies that different raw paths do not
// merge even when they resolve to the same filesystem path.
func TestPathIdentityForMerging(t *testing.T) {
	idx := build(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("./src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	if idx.Len() != 2 {
		t.Fatalf("Len = %d, want 2 (different raw paths do not merge even if they resolve to the same file)", idx.Len())
	}
}
