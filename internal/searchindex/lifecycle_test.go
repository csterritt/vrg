package searchindex_test

import (
	"strings"
	"testing"

	"vrg/internal/searchindex"
)

// build feeds records through a Builder and finishes it, failing if any
// record is rejected: every matrix fixture is schema-valid. It returns
// the index, the stream-integrity result, and the record accounting —
// the dispositions each matrix row asserts.
func build(t *testing.T, workdir string, records ...[]byte) (*searchindex.Index, searchindex.Integrity, searchindex.Report) {
	t.Helper()
	b := searchindex.NewBuilder(workdir)
	for i, rec := range records {
		if err := b.Add(rec); err != nil {
			t.Fatalf("Add(record %d) rejected a valid record: %v\n%s", i, err, rec)
		}
	}
	idx, integrity := b.Finish()
	return idx, integrity, b.Report()
}

// stopInfo is the observable lifecycle result for one retained stop:
// its identity and whether its file's metadata is incomplete.
type stopInfo struct {
	path       string
	line       int64
	incomplete bool
}

func wantStopInfo(t *testing.T, idx *searchindex.Index, want []stopInfo) {
	t.Helper()
	stops := idx.Stops()
	if len(stops) != len(want) {
		t.Fatalf("Stops() = %d entries, want %d: %+v", len(stops), len(want), stops)
	}
	for i, w := range want {
		g := stops[i]
		if string(g.Path) != w.path || g.Line != w.line || g.Incomplete != w.incomplete {
			t.Fatalf("stop %d = {%q, %d, incomplete=%v}, want {%q, %d, incomplete=%v}",
				i, g.Path, g.Line, g.Incomplete, w.path, w.line, w.incomplete)
		}
	}
}

func contextEvent(t *testing.T, path map[string]any) []byte {
	t.Helper()
	return marshal(t, map[string]any{
		"type": "context",
		"data": map[string]any{
			"path": path, "lines": textData("ctx\n"), "line_number": 1,
			"submatches": []any{},
		},
	})
}

func unknownEvent(t *testing.T, typ string) []byte {
	t.Helper()
	return marshal(t, map[string]any{"type": typ, "data": map[string]any{}})
}

// Every row of the Issue 9 lifecycle transition matrix, asserted on the
// retained stops, the per-path incomplete-metadata flag, the binary
// exclusion count, and the stream-integrity result.
func TestLifecycleTransitionMatrix(t *testing.T) {
	a, b := textData("a.txt"), textData("b.txt")
	matchA1 := matchEvent(t, a, textData("hit\n"), 1, submatch(textData("hit"), 0, 3))
	matchA2 := matchEvent(t, a, textData("hit\n"), 2, submatch(textData("hit"), 0, 3))
	matchA3 := matchEvent(t, a, textData("hit\n"), 3, submatch(textData("hit"), 0, 3))
	matchB2 := matchEvent(t, b, textData("hit\n"), 2, submatch(textData("hit"), 0, 3))

	cases := []struct {
		name         string
		records      [][]byte
		wantComplete bool
		wantStops    []stopInfo
		wantBinary   int
	}{
		{
			"begin while not open is valid",
			[][]byte{beginEvent(t, a), matchA1, endEvent(t, a, nil), summaryEvent(t)},
			true,
			[]stopInfo{{"a.txt", 1, false}},
			0,
		},
		{
			"duplicate begin while already open",
			[][]byte{beginEvent(t, a), beginEvent(t, a), matchA1, endEvent(t, a, nil), summaryEvent(t)},
			false,
			[]stopInfo{{"a.txt", 1, false}},
			0,
		},
		{
			"match while open is valid",
			[][]byte{beginEvent(t, a), matchA1, matchA2, endEvent(t, a, nil), summaryEvent(t)},
			true,
			[]stopInfo{{"a.txt", 1, false}, {"a.txt", 2, false}},
			0,
		},
		{
			"orphaned match never opened is retained incomplete",
			[][]byte{matchA1, summaryEvent(t)},
			false,
			[]stopInfo{{"a.txt", 1, true}},
			0,
		},
		{
			"match after its end is retained incomplete",
			[][]byte{beginEvent(t, a), matchA1, endEvent(t, a, nil), matchA2, summaryEvent(t)},
			false,
			[]stopInfo{{"a.txt", 1, true}, {"a.txt", 2, true}},
			0,
		},
		{
			"orphaned match then normal lifecycle stays incomplete",
			[][]byte{matchA1, beginEvent(t, a), matchA2, endEvent(t, a, nil), summaryEvent(t)},
			false,
			[]stopInfo{{"a.txt", 1, true}, {"a.txt", 2, true}},
			0,
		},
		{
			"end while open is valid",
			[][]byte{beginEvent(t, a), endEvent(t, a, nil), summaryEvent(t)},
			true,
			nil,
			0,
		},
		{
			"orphaned end never opened",
			[][]byte{endEvent(t, a, nil), summaryEvent(t)},
			false,
			nil,
			0,
		},
		{
			"end after its end is orphaned",
			[][]byte{beginEvent(t, a), matchA1, endEvent(t, a, nil), endEvent(t, a, nil), summaryEvent(t)},
			false,
			[]stopInfo{{"a.txt", 1, false}},
			0,
		},
		{
			"begin and end path disagreement orphans the end",
			[][]byte{beginEvent(t, a), matchA1, endEvent(t, b, nil), summaryEvent(t)},
			false,
			[]stopInfo{{"a.txt", 1, true}},
			0,
		},
		{
			"context before summary has no lifecycle effect",
			[][]byte{
				contextEvent(t, b),
				beginEvent(t, a),
				contextEvent(t, a),
				matchA1,
				endEvent(t, a, nil),
				contextEvent(t, b),
				summaryEvent(t),
			},
			true,
			[]stopInfo{{"a.txt", 1, false}},
			0,
		},
		{
			"context after summary has no lifecycle effect",
			[][]byte{beginEvent(t, a), matchA1, endEvent(t, a, nil), summaryEvent(t), contextEvent(t, a)},
			true,
			[]stopInfo{{"a.txt", 1, false}},
			0,
		},
		{
			"file still open when the stream ends",
			[][]byte{beginEvent(t, a), matchA1, summaryEvent(t)},
			false,
			[]stopInfo{{"a.txt", 1, true}},
			0,
		},
		{
			"exactly one summary as the final record",
			[][]byte{beginEvent(t, a), matchA1, endEvent(t, a, nil), summaryEvent(t)},
			true,
			[]stopInfo{{"a.txt", 1, false}},
			0,
		},
		{
			"a summary alone is a complete zero-result stream",
			[][]byte{summaryEvent(t)},
			true,
			nil,
			0,
		},
		{
			"summary missing",
			[][]byte{beginEvent(t, a), matchA1, endEvent(t, a, nil)},
			false,
			[]stopInfo{{"a.txt", 1, false}},
			0,
		},
		{
			"empty stream misses its summary",
			nil,
			false,
			nil,
			0,
		},
		{
			"a second summary",
			[][]byte{summaryEvent(t), summaryEvent(t)},
			false,
			nil,
			0,
		},
		{
			"match after summary",
			[][]byte{beginEvent(t, a), matchA1, endEvent(t, a, nil), summaryEvent(t), matchA2},
			false,
			[]stopInfo{{"a.txt", 1, false}, {"a.txt", 2, false}},
			0,
		},
		{
			"begin after summary",
			[][]byte{summaryEvent(t), beginEvent(t, a)},
			false,
			nil,
			0,
		},
		{
			"end after summary",
			[][]byte{summaryEvent(t), endEvent(t, a, nil)},
			false,
			nil,
			0,
		},
		{
			"unknown type after summary",
			[][]byte{summaryEvent(t), unknownEvent(t, "weird")},
			false,
			nil,
			0,
		},
		{
			"unknown type before summary is ignored",
			[][]byte{unknownEvent(t, "weird"), summaryEvent(t)},
			true,
			nil,
			0,
		},
		{
			"interleaved open files pair independently",
			[][]byte{
				beginEvent(t, a), beginEvent(t, b),
				matchA1, matchB2,
				endEvent(t, b, nil),
				matchA3,
				endEvent(t, a, nil),
				summaryEvent(t),
			},
			true,
			[]stopInfo{{"a.txt", 1, false}, {"a.txt", 3, false}, {"b.txt", 2, false}},
			0,
		},
		{
			"binary exclusion takes precedence over orphan retention",
			[][]byte{beginEvent(t, a), matchA1, endEvent(t, a, 42), matchA2, summaryEvent(t)},
			false,
			nil,
			1,
		},
		{
			"orphaned end with binary offset still excludes",
			[][]byte{matchA1, endEvent(t, a, 7), summaryEvent(t)},
			false,
			nil,
			1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			idx, integrity, rep := build(t, "/w", tc.records...)
			if integrity.Complete != tc.wantComplete {
				t.Fatalf("Integrity.Complete = %v, want %v", integrity.Complete, tc.wantComplete)
			}
			// Lifecycle violations are integrity failures, never record
			// loss: no matrix row may inflate the malformed or oversized
			// counts. Unknown-type counting is asserted separately.
			if rep.Malformed != 0 || rep.Oversized != 0 {
				t.Fatalf("Report() = %+v, want no malformed or oversized records", rep)
			}
			wantStopInfo(t, idx, tc.wantStops)
			if n := idx.BinaryExcluded(); n != tc.wantBinary {
				t.Fatalf("BinaryExcluded() = %d, want %d", n, tc.wantBinary)
			}
		})
	}
}

// Path identity is compared on decoded raw path bytes: a begin emitted
// as text and a match or end emitted as base64 bytes — or the reverse —
// are the same file, so mixed-encoding lifecycle events pair normally.
func TestLifecyclePathIdentityAcrossEncodings(t *testing.T) {
	raw := []byte("a.txt")
	cases := []struct {
		name    string
		records [][]byte
	}{
		{"text begin bytes rest", [][]byte{
			beginEvent(t, textData("a.txt")),
			matchEvent(t, bytesData(raw), bytesData([]byte("hit\n")), 1, submatch(bytesData([]byte("hit")), 0, 3)),
			endEvent(t, bytesData(raw), nil),
			summaryEvent(t),
		}},
		{"bytes begin text rest", [][]byte{
			beginEvent(t, bytesData(raw)),
			matchEvent(t, textData("a.txt"), textData("hit\n"), 1, submatch(textData("hit"), 0, 3)),
			endEvent(t, textData("a.txt"), nil),
			summaryEvent(t),
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			idx, integrity, _ := build(t, "/w", tc.records...)
			if !integrity.Complete {
				t.Fatal("mixed-encoding lifecycle events for one path did not pair")
			}
			wantStopInfo(t, idx, []stopInfo{{"a.txt", 1, false}})
		})
	}
}

// A malformed record after a valid summary carries both dispositions:
// Add rejects it as malformed — counted malformed even though the
// after-summary position suppresses nothing — and the after-summary
// position is an integrity failure. Both counters are asserted: the
// malformed count and the incomplete-stream status, while the
// oversized and unknown-type counts stay at zero.
func TestMalformedRecordAfterSummaryIsBoth(t *testing.T) {
	b := searchindex.NewBuilder("/w")
	for _, rec := range [][]byte{
		beginEvent(t, textData("a.txt")),
		matchEvent(t, textData("a.txt"), textData("hit\n"), 1, submatch(textData("hit"), 0, 3)),
		endEvent(t, textData("a.txt"), nil),
		summaryEvent(t),
	} {
		if err := b.Add(rec); err != nil {
			t.Fatalf("Add rejected a valid record: %v", err)
		}
	}
	if err := b.Add([]byte("{not json")); err == nil {
		t.Fatal("Add accepted a malformed record after summary")
	}
	idx, integrity := b.Finish()
	if rep := b.Report(); rep.Malformed != 1 || rep.Oversized != 0 || rep.UnknownTypes != 0 {
		t.Fatalf("Report() = %+v, want exactly one malformed record", rep)
	}
	if integrity.Complete {
		t.Fatal("a malformed record after summary left the stream complete")
	}
	wantStopInfo(t, idx, []stopInfo{{"a.txt", 1, false}})
}

// Consume drains a byte stream of newline-terminated records, equivalent
// to feeding each record through Add.
func TestConsumeDrainsRecords(t *testing.T) {
	stream := `{"type":"begin","data":{"path":{"text":"a.txt"}}}` + "\n" +
		`{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"hit\n"},` +
		`"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}` + "\n" +
		`{"type":"end","data":{"path":{"text":"a.txt"},"binary_offset":null}}` + "\n" +
		`{"type":"summary","data":{}}` + "\n"
	b := searchindex.NewBuilder("/w")
	b.Consume(strings.NewReader(stream))
	idx, integrity := b.Finish()
	if !integrity.Complete {
		t.Fatal("a well-formed terminated stream is incomplete")
	}
	wantStopInfo(t, idx, []stopInfo{{"a.txt", 1, false}})
}

// A trailing fragment without its newline is an unterminated record: it
// is not indexed — even when it happens to be parseable — and the stream
// is incomplete. This ordinary, non-oversized record carries both
// composite dispositions: counted malformed and marked stream-incomplete.
func TestTrailingUnterminatedRecord(t *testing.T) {
	stream := `{"type":"begin","data":{"path":{"text":"a.txt"}}}` + "\n" +
		`{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"hit\n"},` +
		`"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}` + "\n" +
		`{"type":"end","data":{"path":{"text":"a.txt"},"binary_offset":null}}` + "\n" +
		`{"type":"summary","data":{}}` + "\n" +
		// A well-formed match record that simply lacks its terminating
		// newline: still an unterminated record, never indexed.
		`{"type":"match","data":{"path":{"text":"b.txt"},"lines":{"text":"hit\n"},` +
		`"line_number":9,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}`

	b := searchindex.NewBuilder("/w")
	b.Consume(strings.NewReader(stream))
	idx, integrity := b.Finish()
	if rep := b.Report(); rep.Malformed != 1 || rep.Oversized != 0 || rep.UnknownTypes != 0 {
		t.Fatalf("Report() = %+v, want the unterminated record counted malformed and nothing else", rep)
	}
	if integrity.Complete {
		t.Fatal("a stream with a trailing unterminated record is complete")
	}
	for _, s := range idx.Stops() {
		if string(s.Path) == "b.txt" {
			t.Fatal("the unterminated record was indexed")
		}
	}
}

// Integrity.MissingSummary isolates the absent-summary cause: a stream
// that ends without a valid summary sets it, while a complete stream
// and a stream whose only violation is a missing end leave it clear.
func TestIntegrityMissingSummary(t *testing.T) {
	_, integrity, _ := build(t, "/w",
		beginEvent(t, textData("a.txt")),
		endEvent(t, textData("a.txt"), nil))
	if integrity.Complete || !integrity.MissingSummary {
		t.Fatalf("summary-less stream: Integrity = %+v, want incomplete with MissingSummary", integrity)
	}

	_, integrity, _ = build(t, "/w",
		beginEvent(t, textData("a.txt")),
		endEvent(t, textData("a.txt"), nil),
		summaryEvent(t))
	if integrity.MissingSummary {
		t.Fatal("a complete stream reports MissingSummary")
	}

	_, integrity, _ = build(t, "/w",
		beginEvent(t, textData("a.txt")),
		summaryEvent(t))
	if integrity.Complete || integrity.MissingSummary {
		t.Fatalf("missing-end stream: Integrity = %+v, want incomplete without MissingSummary", integrity)
	}
}
