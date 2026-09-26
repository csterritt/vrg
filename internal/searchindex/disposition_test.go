package searchindex_test

import (
	"strings"
	"testing"

	"vrg/internal/searchindex"
)

// streamOf joins fixture records into a collected stdout stream: each
// record newline-terminated, as rg emits it.
func streamOf(records ...string) string {
	return strings.Join(records, "\n") + "\n"
}

// damaged embeds one record inside a valid file lifecycle: the
// well-formed match records before and after it prove a skipped record
// resynchronizes — the rest of the stream still indexes — and the
// intact lifecycle keeps integrity diagnostics at zero.
func damaged(record string) string {
	return streamOf(
		beginRec(jText("a.txt")),
		matchRec(jText("a.txt"), jText("hit\n"), 1, subRec(jText("hit"), 0, 3)),
		record,
		matchRec(jText("a.txt"), jText("hit\n"), 4, subRec(jText("hit"), 0, 3)),
		endRec(jText("a.txt"), "null"),
		`{"type":"summary","data":{}}`,
	)
}

// TestMalformedDispositions covers every row of the Issue #3 per-record
// schema matrix — invalid JSON, invalid base64, missing or non-string
// type, each missing or wrongly typed required field, and each invalid
// range — asserting the deterministic disposition: skipped and counted
// malformed, with no stream-integrity failure, no unknown-type count,
// and the surrounding records still indexed.
func TestMalformedDispositions(t *testing.T) {
	cases := []struct {
		name   string
		record string
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
		{"match missing path", `{"type":"match","data":{"lines":{"text":"x\n"},"line_number":1,"submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}`},
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
			ix := searchindex.New("/work")
			ix.Feed([]byte(damaged(tc.record)))
			ix.Prepare()

			if n := ix.Malformed(); n != 1 {
				t.Fatalf("Malformed = %d, want 1 skipped-and-counted record", n)
			}
			if failures := ix.IntegrityFailures(); len(failures) != 0 {
				t.Fatalf("IntegrityFailures = %v, want an intact stream — "+
					"a skipped record must not inflate integrity failures", failures)
			}
			stops := ix.Stops()
			if len(stops) != 2 {
				t.Fatalf("len(Stops) = %d, want 2 — the records around the skip must index", len(stops))
			}
		})
	}
}

// TestIntegrityDispositions covers the integrity rows of the Issue #9
// lifecycle matrix, asserting the opposite side of the separation:
// lifecycle violations are stream-integrity failures and never inflate
// the malformed count.
func TestIntegrityDispositions(t *testing.T) {
	cases := []struct {
		name       string
		stream     string
		failures   []string
		stops      int
		incomplete int
	}{
		{
			name: "duplicate begin",
			stream: streamOf(
				beginRec(jText("a.txt")),
				beginRec(jText("a.txt")),
				matchRec(jText("a.txt"), jText("hit\n"), 1, subRec(jText("hit"), 0, 3)),
				endRec(jText("a.txt"), "null"),
				`{"type":"summary","data":{}}`,
			),
			failures: []string{"duplicate begin"},
			stops:    1,
		},
		{
			name: "orphaned match",
			stream: streamOf(
				matchRec(jText("a.txt"), jText("hit\n"), 1, subRec(jText("hit"), 0, 3)),
				`{"type":"summary","data":{}}`,
			),
			failures:   []string{"orphaned match"},
			stops:      1,
			incomplete: 1,
		},
		{
			name: "match after end",
			stream: streamOf(
				beginRec(jText("a.txt")),
				matchRec(jText("a.txt"), jText("hit\n"), 1, subRec(jText("hit"), 0, 3)),
				endRec(jText("a.txt"), "null"),
				matchRec(jText("a.txt"), jText("hit\n"), 5, subRec(jText("hit"), 0, 3)),
				`{"type":"summary","data":{}}`,
			),
			failures:   []string{"orphaned match"},
			stops:      2,
			incomplete: 2,
		},
		{
			name: "orphaned end",
			stream: streamOf(
				endRec(jText("a.txt"), "null"),
				`{"type":"summary","data":{}}`,
			),
			failures: []string{"orphaned end"},
		},
		{
			name: "second summary",
			stream: streamOf(
				`{"type":"summary","data":{}}`,
				`{"type":"summary","data":{}}`,
			),
			failures: []string{"extra summary record"},
		},
		{
			name: "decodable record after summary",
			stream: streamOf(
				`{"type":"summary","data":{}}`,
				ctxRec(jText("a.txt")),
			),
			failures: []string{"record after summary"},
		},
		{
			name: "missing end retains matches",
			stream: streamOf(
				beginRec(jText("a.txt")),
				matchRec(jText("a.txt"), jText("hit\n"), 1, subRec(jText("hit"), 0, 3)),
				`{"type":"summary","data":{}}`,
			),
			failures:   []string{"missing end"},
			stops:      1,
			incomplete: 1,
		},
		{
			name: "missing summary",
			stream: streamOf(
				beginRec(jText("a.txt")),
				matchRec(jText("a.txt"), jText("hit\n"), 1, subRec(jText("hit"), 0, 3)),
				endRec(jText("a.txt"), "null"),
			),
			failures: []string{"missing summary"},
			stops:    1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ix := searchindex.New("/work")
			ix.Feed([]byte(tc.stream))
			ix.Prepare()

			if n := ix.Malformed(); n != 0 {
				t.Fatalf("Malformed = %d, want 0 — "+
					"lifecycle violations must not inflate the malformed count", n)
			}
			failures := ix.IntegrityFailures()
			if len(failures) != len(tc.failures) {
				t.Fatalf("IntegrityFailures = %v, want %d diagnostics", failures, len(tc.failures))
			}
			for i, want := range tc.failures {
				if !strings.Contains(failures[i], want) {
					t.Fatalf("IntegrityFailures[%d] = %q, want it to contain %q (all: %v)",
						i, failures[i], want, failures)
				}
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
		})
	}
}

// TestCompositeDispositions covers the two rows the matrices mark
// "both": a record counted malformed *and* flagged as a stream-integrity
// failure. Both counters and the incomplete-stream status are asserted
// in each. These fixtures are deliberately small — the oversized
// composite (an unterminated record over 64 MiB) is Task 3's, and this
// first row must stay an ordinary record to remain distinct from it.
func TestCompositeDispositions(t *testing.T) {
	// A trailing record that would decode cleanly — a complete end
	// record — still counts malformed when its newline never arrives:
	// the rule is unconditional on the record's content. Sitting after
	// the summary its whole integrity representation is the
	// after-summary cause — one cause per physical record — so no
	// unterminated-final-record diagnostic fires alongside it.
	t.Run("unterminated trailing record", func(t *testing.T) {
		ix := searchindex.New("/work")
		ix.Feed([]byte(streamOf(
			beginRec(jText("a.txt")),
			matchRec(jText("a.txt"), jText("hit\n"), 1, subRec(jText("hit"), 0, 3)),
			endRec(jText("a.txt"), "null"),
			`{"type":"summary","data":{}}`,
		) + endRec(jText("a.txt"), "null"))) // no terminating newline
		ix.Prepare()

		if n := ix.Malformed(); n != 1 {
			t.Fatalf("Malformed = %d, want 1 for the unterminated record", n)
		}
		failures := ix.IntegrityFailures()
		want := []string{"record after summary"}
		if len(failures) != len(want) {
			t.Fatalf("IntegrityFailures = %v, want %v", failures, want)
		}
		for i, w := range want {
			if !strings.Contains(failures[i], w) {
				t.Fatalf("IntegrityFailures[%d] = %q, want it to contain %q", i, failures[i], w)
			}
		}
		if len(ix.Stops()) != 1 {
			t.Fatalf("len(Stops) = %d, want 1 — the intact file still indexes", len(ix.Stops()))
		}
	})

	// A malformed record after a valid summary is counted malformed
	// *and* flagged as a record after summary — the skip reason and the
	// ordering violation are recorded independently.
	t.Run("malformed record after summary", func(t *testing.T) {
		ix := searchindex.New("/work")
		ix.Feed([]byte(streamOf(
			beginRec(jText("a.txt")),
			matchRec(jText("a.txt"), jText("hit\n"), 1, subRec(jText("hit"), 0, 3)),
			endRec(jText("a.txt"), "null"),
			`{"type":"summary","data":{}}`,
			`{"type":"match","data":{"path":`, // cut mid-record: malformed
		)))
		ix.Prepare()

		if n := ix.Malformed(); n != 1 {
			t.Fatalf("Malformed = %d, want 1 — after-summary position must not "+
				"suppress malformed accounting", n)
		}
		failures := ix.IntegrityFailures()
		if len(failures) != 1 || !strings.Contains(failures[0], "record after summary") {
			t.Fatalf("IntegrityFailures = %v, want the after-summary integrity failure", failures)
		}
		if len(ix.Stops()) != 1 {
			t.Fatalf("len(Stops) = %d, want 1", len(ix.Stops()))
		}
	})
}
