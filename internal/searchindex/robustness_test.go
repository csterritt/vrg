package searchindex_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"vrg/internal/searchindex"
)

// feed adds every record to a Builder and finishes it, returning the
// index, the stream-integrity result, the record accounting, and the
// per-record Add results — a malformed record's error is the skip
// signal, a lifecycle violation is not an error.
func feed(t *testing.T, workdir string, records ...[]byte) (*searchindex.Index, searchindex.Integrity, searchindex.Report, []error) {
	t.Helper()
	b := searchindex.NewBuilder(workdir)
	errs := make([]error, len(records))
	for i, rec := range records {
		errs[i] = b.Add(rec)
	}
	idx, integrity := b.Finish()
	return idx, integrity, b.Report(), errs
}

// consume drains a stream through a Builder and finishes it, returning
// the index, integrity, and record accounting.
func consume(t *testing.T, workdir, stream string) (*searchindex.Index, searchindex.Integrity, searchindex.Report) {
	t.Helper()
	b := searchindex.NewBuilder(workdir)
	b.Consume(strings.NewReader(stream))
	idx, integrity := b.Finish()
	return idx, integrity, b.Report()
}

// streamOf joins records into the newline-terminated byte stream rg
// writes on stdout.
func streamOf(records ...string) string {
	var b strings.Builder
	for _, r := range records {
		b.WriteString(r)
		b.WriteByte('\n')
	}
	return b.String()
}

// Every row of the Issue 3 per-record schema matrix is a malformed
// record: Add rejects it, it is counted malformed, and it carries no
// lifecycle weight — the surrounding stream stays complete and the
// records after it index normally. Each fixture sits between two valid
// match records, so correct indexing of the trailing match proves the
// skip resynchronizes.
func TestSchemaMatrixMalformedDispositions(t *testing.T) {
	a := textData("a.txt")
	cases := []struct {
		name string
		bad  []byte
	}{
		{"invalid json", []byte("{not json")},
		{"empty record", []byte("")},
		{"json non-object", []byte("[1,2]")},
		{"missing type", marshal(t, map[string]any{"data": map[string]any{}})},
		{"non-string type", marshal(t, map[string]any{"type": 5, "data": map[string]any{}})},
		{"invalid base64", marshal(t, map[string]any{"type": "begin",
			"data": map[string]any{"path": map[string]any{"bytes": "!!!"}}})},
		{"begin missing data", marshal(t, map[string]any{"type": "begin"})},
		{"begin missing path", marshal(t, map[string]any{"type": "begin", "data": map[string]any{}})},
		{"begin path not a blob", marshal(t, map[string]any{"type": "begin",
			"data": map[string]any{"path": "a.txt"}})},
		{"begin path invalid base64", marshal(t, map[string]any{"type": "begin",
			"data": map[string]any{"path": map[string]any{"bytes": "!!!"}}})},
		{"begin path text non-string", marshal(t, map[string]any{"type": "begin",
			"data": map[string]any{"path": map[string]any{"text": 5}}})},
		{"match missing path", marshal(t, map[string]any{"type": "match", "data": map[string]any{
			"lines": textData("x\n"), "line_number": 1, "submatches": []any{submatch(textData("x"), 0, 1)}}})},
		{"match missing lines", marshal(t, map[string]any{"type": "match", "data": map[string]any{
			"path": a, "line_number": 1, "submatches": []any{submatch(textData("x"), 0, 1)}}})},
		{"match missing line_number", marshal(t, map[string]any{"type": "match", "data": map[string]any{
			"path": a, "lines": textData("x\n"), "submatches": []any{submatch(textData("x"), 0, 1)}}})},
		{"match line_number zero", matchEvent(t, a, textData("x\n"), 0, submatch(textData("x"), 0, 1))},
		{"match line_number negative", matchEvent(t, a, textData("x\n"), -2, submatch(textData("x"), 0, 1))},
		{"match line_number non-integer", marshal(t, map[string]any{"type": "match", "data": map[string]any{
			"path": a, "lines": textData("x\n"), "line_number": 1.5,
			"submatches": []any{submatch(textData("x"), 0, 1)}}})},
		{"match line_number string", marshal(t, map[string]any{"type": "match", "data": map[string]any{
			"path": a, "lines": textData("x\n"), "line_number": "1",
			"submatches": []any{submatch(textData("x"), 0, 1)}}})},
		{"match missing submatches", marshal(t, map[string]any{"type": "match", "data": map[string]any{
			"path": a, "lines": textData("x\n"), "line_number": 1}})},
		{"match empty submatches", marshal(t, map[string]any{"type": "match", "data": map[string]any{
			"path": a, "lines": textData("x\n"), "line_number": 1, "submatches": []any{}}})},
		{"match submatches not an array", marshal(t, map[string]any{"type": "match", "data": map[string]any{
			"path": a, "lines": textData("x\n"), "line_number": 1, "submatches": map[string]any{}}})},
		{"submatch missing match", marshal(t, map[string]any{"type": "match", "data": map[string]any{
			"path": a, "lines": textData("x\n"), "line_number": 1,
			"submatches": []any{map[string]any{"start": 0, "end": 1}}}})},
		{"submatch missing start", marshal(t, map[string]any{"type": "match", "data": map[string]any{
			"path": a, "lines": textData("x\n"), "line_number": 1,
			"submatches": []any{map[string]any{"match": textData("x"), "end": 1}}}})},
		{"submatch missing end", marshal(t, map[string]any{"type": "match", "data": map[string]any{
			"path": a, "lines": textData("x\n"), "line_number": 1,
			"submatches": []any{map[string]any{"match": textData("x"), "start": 0}}}})},
		{"submatch start negative", matchEvent(t, a, textData("hit\n"), 1, submatch(textData("hit"), -1, 3))},
		{"submatch start after end", matchEvent(t, a, textData("hit\n"), 1, submatch(textData("hit"), 2, 1))},
		{"submatch end past line", matchEvent(t, a, textData("hit\n"), 1, submatch(textData("hit"), 0, 99))},
		{"submatch invalid base64", marshal(t, map[string]any{"type": "match", "data": map[string]any{
			"path": a, "lines": textData("x\n"), "line_number": 1,
			"submatches": []any{submatch(map[string]any{"bytes": "!!!"}, 0, 1)}}})},
		{"end missing path", marshal(t, map[string]any{"type": "end",
			"data": map[string]any{"binary_offset": nil}})},
		{"end missing binary_offset", marshal(t, map[string]any{"type": "end",
			"data": map[string]any{"path": a}})},
		{"end negative binary_offset", marshal(t, map[string]any{"type": "end",
			"data": map[string]any{"path": a, "binary_offset": -1}})},
		{"end non-integer binary_offset", marshal(t, map[string]any{"type": "end",
			"data": map[string]any{"path": a, "binary_offset": "12"}})},
		{"summary missing data", marshal(t, map[string]any{"type": "summary"})},
		{"summary data not object", marshal(t, map[string]any{"type": "summary", "data": []any{1}})},
		{"summary data null", marshal(t, map[string]any{"type": "summary", "data": nil})},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			records := [][]byte{
				beginEvent(t, a),
				matchEvent(t, a, textData("hit\n"), 1, submatch(textData("hit"), 0, 3)),
				tc.bad,
				matchEvent(t, a, textData("hit\n"), 2, submatch(textData("hit"), 0, 3)),
				endEvent(t, a, nil),
				summaryEvent(t),
			}
			idx, integrity, rep, errs := feed(t, "/w", records...)
			for i, err := range errs {
				if i == 2 {
					if err == nil {
						t.Fatalf("Add(%s) = nil, want the malformed record rejected", tc.bad)
					}
					continue
				}
				if err != nil {
					t.Fatalf("Add(record %d) rejected a valid record: %v\n%s", i, err, records[i])
				}
			}
			if rep.Malformed != 1 || rep.Oversized != 0 || rep.UnknownTypes != 0 {
				t.Fatalf("Report() = %+v, want exactly one malformed record", rep)
			}
			if !integrity.Complete {
				t.Fatal("a skipped malformed record disturbed stream integrity")
			}
			wantStopInfo(t, idx, []stopInfo{{"a.txt", 1, false}, {"a.txt", 2, false}})
		})
	}
}

// recordPayloadLimit is the 64 MiB maximum JSON record payload the PRD
// sets, excluding the record's newline delimiter.
const recordPayloadLimit = 64 << 20

// sizedMatch builds a match record for path whose encoded payload is
// exactly size bytes, padding the lines text. The field order — type
// first, data.path before data.lines — matches ripgrep's emission order
// so the path is recoverable before the payload limit is hit.
func sizedMatch(t *testing.T, size int, path string, line int64) string {
	t.Helper()
	build := func(lines string) string {
		return fmt.Sprintf(`{"type":"match","data":{"path":{"text":%q},"lines":{"text":%q},`+
			`"line_number":%d,"submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}`,
			path, lines, line)
	}
	rec := build("")
	pad := size - len(rec)
	if pad < 0 {
		t.Fatalf("match record is %d bytes before padding, want %d", len(rec), size)
	}
	if rec = build(strings.Repeat("x", pad)); len(rec) != size {
		t.Fatalf("sizedMatch produced %d bytes, want %d", len(rec), size)
	}
	return rec
}

// A malformed record between valid records of a consumed byte stream is
// skipped and counted, and the reader resynchronizes on the following
// newline: the records after it index normally.
func TestMalformedSkipResynchronizes(t *testing.T) {
	stream := `{"type":"begin","data":{"path":{"text":"a.txt"}}}` + "\n" +
		`{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"hit\n"},` +
		`"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}` + "\n" +
		"this is not json\n" +
		`{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"hit\n"},` +
		`"line_number":2,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}` + "\n" +
		`{"type":"end","data":{"path":{"text":"a.txt"},"binary_offset":null}}` + "\n" +
		`{"type":"summary","data":{}}` + "\n"
	idx, integrity, rep := consume(t, "/w", stream)
	if rep.Malformed != 1 || rep.Oversized != 0 || rep.UnknownTypes != 0 {
		t.Fatalf("Report() = %+v, want exactly one malformed record", rep)
	}
	if !integrity.Complete {
		t.Fatal("a skipped malformed record disturbed stream integrity")
	}
	wantStopInfo(t, idx, []stopInfo{{"a.txt", 1, false}, {"a.txt", 2, false}})
}

// A record payload exactly at the 64 MiB limit is accepted and indexed;
// one byte over is skipped and counted oversized, the remainder is
// consumed and discarded through its newline, and parsing resynchronizes
// on the following record.
func TestRecordPayloadLimitBoundary(t *testing.T) {
	t.Run("exactly at the limit is accepted", func(t *testing.T) {
		stream := streamOf(
			`{"type":"begin","data":{"path":{"text":"a.txt"}}}`,
			sizedMatch(t, recordPayloadLimit, "a.txt", 7),
			`{"type":"end","data":{"path":{"text":"a.txt"},"binary_offset":null}}`,
			`{"type":"summary","data":{}}`,
		)
		idx, integrity, rep := consume(t, "/w", stream)
		if rep.Oversized != 0 || rep.Malformed != 0 || rep.UnknownTypes != 0 {
			t.Fatalf("Report() = %+v, want no skipped records", rep)
		}
		if !integrity.Complete {
			t.Fatal("a stream with an at-limit record is incomplete")
		}
		wantStopInfo(t, idx, []stopInfo{{"a.txt", 7, false}})
	})

	t.Run("one byte over is skipped and parsing resynchronizes", func(t *testing.T) {
		stream := streamOf(
			`{"type":"begin","data":{"path":{"text":"a.txt"}}}`,
			`{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"hit\n"},`+
				`"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}`,
			sizedMatch(t, recordPayloadLimit+1, "big.txt", 9),
			`{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"hit\n"},`+
				`"line_number":2,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}`,
			`{"type":"end","data":{"path":{"text":"a.txt"},"binary_offset":null}}`,
			`{"type":"summary","data":{}}`,
		)
		idx, integrity, rep := consume(t, "/w", stream)
		if rep.Oversized != 1 || rep.Malformed != 0 || rep.UnknownTypes != 0 {
			t.Fatalf("Report() = %+v, want exactly one oversized record", rep)
		}
		if !integrity.Complete {
			t.Fatal("a safely skipped oversized record disturbed stream integrity")
		}
		// The record after the oversized one is parsed normally.
		wantStopInfo(t, idx, []stopInfo{{"a.txt", 1, false}, {"a.txt", 2, false}})
	})
}

// An oversized record whose type and data.path were parsed before the
// limit names the recovered path for the diagnostic; a record that hit
// the limit earlier — inside a preceding field, or before its type —
// is counted anonymously and reports the count only.
func TestOversizedPathRecovery(t *testing.T) {
	cases := []struct {
		name      string
		record    string
		wantPaths [][]byte
	}{
		{
			"type and path parsed before the limit",
			sizedMatch(t, recordPayloadLimit+1, "big.txt", 1),
			[][]byte{[]byte("big.txt")},
		},
		{
			"limit hit inside a field before the path",
			fmt.Sprintf(`{"type":"match","data":{"lines":{"text":"%s"},`+
				`"path":{"text":"big.txt"},"line_number":1,`+
				`"submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}`,
				strings.Repeat("x", recordPayloadLimit)),
			nil,
		},
		{
			"limit hit before the record type",
			`{"pad":"` + strings.Repeat("x", recordPayloadLimit) +
				`","type":"match","data":{"path":{"text":"big.txt"}}}`,
			nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stream := streamOf(tc.record, `{"type":"summary","data":{}}`)
			_, integrity, rep := consume(t, "/w", stream)
			if rep.Oversized != 1 || rep.Malformed != 0 || rep.UnknownTypes != 0 {
				t.Fatalf("Report() = %+v, want exactly one oversized record", rep)
			}
			if !integrity.Complete {
				t.Fatal("a safely skipped oversized record disturbed stream integrity")
			}
			var got [][]byte
			got = append(got, rep.OversizedPaths...)
			if !slices.EqualFunc(got, tc.wantPaths, slices.Equal) {
				t.Fatalf("OversizedPaths = %q, want %q", got, tc.wantPaths)
			}
		})
	}
}

// A file whose only records were oversized is absent from the file
// list; its recovered path in the diagnostic input is the only trace of
// the loss.
func TestOversizedOnlyFileAbsent(t *testing.T) {
	stream := streamOf(
		`{"type":"begin","data":{"path":{"text":"big.txt"}}}`,
		sizedMatch(t, recordPayloadLimit+1, "big.txt", 1),
		`{"type":"end","data":{"path":{"text":"big.txt"},"binary_offset":null}}`,
		`{"type":"begin","data":{"path":{"text":"a.txt"}}}`,
		`{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"hit\n"},`+
			`"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}`,
		`{"type":"end","data":{"path":{"text":"a.txt"},"binary_offset":null}}`,
		`{"type":"summary","data":{}}`,
	)
	idx, integrity, rep := consume(t, "/w", stream)
	if rep.Oversized != 1 {
		t.Fatalf("Report().Oversized = %d, want 1", rep.Oversized)
	}
	if len(rep.OversizedPaths) != 1 || string(rep.OversizedPaths[0]) != "big.txt" {
		t.Fatalf("OversizedPaths = %q, want [big.txt]", rep.OversizedPaths)
	}
	if !integrity.Complete {
		t.Fatal("paired begin/end around an oversized record is not complete")
	}
	var files []string
	for _, f := range idx.Files() {
		files = append(files, string(f))
	}
	if !slices.Equal(files, []string{"a.txt"}) {
		t.Fatalf("Files() = %v, want [a.txt]: big.txt's only records were oversized", files)
	}
}

// An oversized final record with no trailing newline carries all three
// dispositions: counted oversized, counted malformed for the missing
// termination, and the stream is incomplete.
func TestOversizedUnterminatedFinalRecord(t *testing.T) {
	stream := streamOf(
		`{"type":"begin","data":{"path":{"text":"a.txt"}}}`,
		`{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"hit\n"},`+
			`"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}`,
		`{"type":"end","data":{"path":{"text":"a.txt"},"binary_offset":null}}`,
		`{"type":"summary","data":{}}`,
	) + sizedMatch(t, recordPayloadLimit+1, "big.txt", 9) // no newline

	idx, integrity, rep := consume(t, "/w", stream)
	if rep.Oversized != 1 || rep.Malformed != 1 || rep.UnknownTypes != 0 {
		t.Fatalf("Report() = %+v, want one oversized and one malformed record", rep)
	}
	if integrity.Complete {
		t.Fatal("an unterminated final record left the stream complete")
	}
	wantStopInfo(t, idx, []stopInfo{{"a.txt", 1, false}})
}

// Unknown string event types are skipped and counted separately — never
// malformed — and never substitute for required completion events. An
// unknown type after summary keeps its unknown-type count while its
// position is separately an integrity failure.
func TestUnknownTypeDispositions(t *testing.T) {
	weird := `{"type":"weird","data":{}}`
	stats := `{"type":"stats","data":{"searches":1}}`

	cases := []struct {
		name         string
		records      []string
		wantUnknown  int
		wantComplete bool
	}{
		{
			"unknown types between valid records are counted and ignored",
			[]string{
				`{"type":"begin","data":{"path":{"text":"a.txt"}}}`,
				`{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"hit\n"},` +
					`"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}`,
				weird, stats,
				`{"type":"end","data":{"path":{"text":"a.txt"},"binary_offset":null}}`,
				`{"type":"summary","data":{}}`,
			},
			2, true,
		},
		{
			"an unknown type after summary counts and fails integrity",
			[]string{`{"type":"summary","data":{}}`, weird},
			1, false,
		},
		{
			"an unknown type cannot substitute for summary",
			[]string{
				`{"type":"begin","data":{"path":{"text":"a.txt"}}}`,
				`{"type":"end","data":{"path":{"text":"a.txt"},"binary_offset":null}}`,
				stats,
			},
			1, false,
		},
		{
			"context is a known type, never counted unknown",
			[]string{
				`{"type":"context","data":{"path":{"text":"a.txt"},"lines":{"text":"c\n"},` +
					`"line_number":1,"submatches":[]}}`,
				`{"type":"summary","data":{}}`,
			},
			0, true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, integrity, rep := consume(t, "/w", streamOf(tc.records...))
			if rep.UnknownTypes != tc.wantUnknown || rep.Malformed != 0 || rep.Oversized != 0 {
				t.Fatalf("Report() = %+v, want %d unknown types and nothing else", rep, tc.wantUnknown)
			}
			if integrity.Complete != tc.wantComplete {
				t.Fatalf("Integrity.Complete = %v, want %v", integrity.Complete, tc.wantComplete)
			}
		})
	}
}
