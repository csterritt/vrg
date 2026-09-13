package searchindex_test

import (
	"testing"

	"vrg/internal/searchindex"
)

// TestMalformedDispositionMatrix is the single table-driven test covering
// every row of the Issue #3 per-record schema matrix and the Issue #9
// lifecycle matrix with respect to malformed-record dispositions. Each
// row feeds a sequence of ripgrep JSON records through a Builder,
// optionally signals a trailing malformed record, and asserts the
// resulting malformed count, stream integrity, stop count,
// incomplete-stop count, and excluded-file count.
//
// Schema-matrix rows assert that a malformed record is skipped and
// counted as malformed without creating a stream-integrity failure
// (except where the matrix explicitly marks both). Lifecycle-matrix
// rows assert that a lifecycle violation marks the stream integrity as
// failed but never inflates the malformed count. Composite rows assert
// both where the matrices explicitly mark both.
func TestMalformedDispositionMatrix(t *testing.T) {
	// A valid match record for reuse in fixtures.
	validMatch := textMatch("a.go", "x\n", 1, subSpec{"x", 0, 1})

	cases := []struct {
		name              string
		records           []string
		trailingMalformed bool
		wantMalformed     int
		wantComplete      bool
		wantStops         int
		wantIncomplete    int
		wantExcluded      int
	}{
		// --- Issue #3 per-record schema matrix: malformed, skipped, counted ---

		{
			name:          "invalid JSON is malformed",
			records:       []string{`{bad json`, summaryRecord()},
			wantMalformed: 1,
			wantComplete:  true,
		},
		{
			name:          "missing type is malformed",
			records:       []string{`{"data":{}}`, summaryRecord()},
			wantMalformed: 1,
			wantComplete:  true,
		},
		{
			name:          "non-string type is malformed",
			records:       []string{`{"type":123}`, summaryRecord()},
			wantMalformed: 1,
			wantComplete:  true,
		},
		{
			name:          "begin missing path is malformed",
			records:       []string{`{"type":"begin","data":{}}`, summaryRecord()},
			wantMalformed: 1,
			wantComplete:  true,
		},
		{
			name:          "begin wrongly typed path is malformed",
			records:       []string{`{"type":"begin","data":{"path":123}}`, summaryRecord()},
			wantMalformed: 1,
			wantComplete:  true,
		},
		{
			name:          "begin invalid base64 path is malformed",
			records:       []string{`{"type":"begin","data":{"path":{"bytes":"!!!"}}}`, summaryRecord()},
			wantMalformed: 1,
			wantComplete:  true,
		},
		{
			name:          "match missing path is malformed",
			records:       []string{`{"type":"match","data":{"lines":{"text":"x\n"},"line_number":1,"submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}`, summaryRecord()},
			wantMalformed: 1,
			wantComplete:  true,
		},
		{
			name:          "match missing lines is malformed",
			records:       []string{`{"type":"match","data":{"path":{"text":"a.go"},"line_number":1,"submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}`, summaryRecord()},
			wantMalformed: 1,
			wantComplete:  true,
		},
		{
			name:          "match missing line_number defaults to zero is malformed",
			records:       []string{`{"type":"match","data":{"path":{"text":"a.go"},"lines":{"text":"x\n"},"submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}`, summaryRecord()},
			wantMalformed: 1,
			wantComplete:  true,
		},
		{
			name:          "match missing submatches is malformed",
			records:       []string{`{"type":"match","data":{"path":{"text":"a.go"},"lines":{"text":"x\n"},"line_number":1}}`, summaryRecord()},
			wantMalformed: 1,
			wantComplete:  true,
		},
		{
			name:          "match wrongly typed path is malformed",
			records:       []string{`{"type":"match","data":{"path":123,"lines":{"text":"x\n"},"line_number":1,"submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}`, summaryRecord()},
			wantMalformed: 1,
			wantComplete:  true,
		},
		{
			name:          "match wrongly typed lines is malformed",
			records:       []string{`{"type":"match","data":{"path":{"text":"a.go"},"lines":123,"line_number":1,"submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}`, summaryRecord()},
			wantMalformed: 1,
			wantComplete:  true,
		},
		{
			name:          "match wrongly typed line_number is malformed",
			records:       []string{`{"type":"match","data":{"path":{"text":"a.go"},"lines":{"text":"x\n"},"line_number":"1","submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}`, summaryRecord()},
			wantMalformed: 1,
			wantComplete:  true,
		},
		{
			name:          "match wrongly typed submatches is malformed",
			records:       []string{`{"type":"match","data":{"path":{"text":"a.go"},"lines":{"text":"x\n"},"line_number":1,"submatches":"notarray"}}`, summaryRecord()},
			wantMalformed: 1,
			wantComplete:  true,
		},
		{
			name:          "match line_number zero is malformed",
			records:       []string{`{"type":"match","data":{"path":{"text":"a.go"},"lines":{"text":"x\n"},"line_number":0,"submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}`, summaryRecord()},
			wantMalformed: 1,
			wantComplete:  true,
		},
		{
			name:          "match negative line_number is malformed",
			records:       []string{`{"type":"match","data":{"path":{"text":"a.go"},"lines":{"text":"x\n"},"line_number":-1,"submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}`, summaryRecord()},
			wantMalformed: 1,
			wantComplete:  true,
		},
		{
			name:          "match non-integer line_number is malformed",
			records:       []string{`{"type":"match","data":{"path":{"text":"a.go"},"lines":{"text":"x\n"},"line_number":1.5,"submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}`, summaryRecord()},
			wantMalformed: 1,
			wantComplete:  true,
		},
		{
			name:          "match empty submatches array is malformed",
			records:       []string{`{"type":"match","data":{"path":{"text":"a.go"},"lines":{"text":"x\n"},"line_number":1,"submatches":[]}}`, summaryRecord()},
			wantMalformed: 1,
			wantComplete:  true,
		},
		{
			name:          "match submatch start greater than end is malformed",
			records:       []string{`{"type":"match","data":{"path":{"text":"a.go"},"lines":{"text":"x\n"},"line_number":1,"submatches":[{"match":{"text":"x"},"start":1,"end":0}]}}`, summaryRecord()},
			wantMalformed: 1,
			wantComplete:  true,
		},
		{
			name:          "match submatch negative start is malformed",
			records:       []string{`{"type":"match","data":{"path":{"text":"a.go"},"lines":{"text":"x\n"},"line_number":1,"submatches":[{"match":{"text":"x"},"start":-1,"end":1}]}}`, summaryRecord()},
			wantMalformed: 1,
			wantComplete:  true,
		},
		{
			name:          "match submatch end beyond line length is malformed",
			records:       []string{`{"type":"match","data":{"path":{"text":"a.go"},"lines":{"text":"x\n"},"line_number":1,"submatches":[{"match":{"text":"x"},"start":0,"end":99}]}}`, summaryRecord()},
			wantMalformed: 1,
			wantComplete:  true,
		},
		{
			name:          "match invalid base64 submatch is malformed",
			records:       []string{`{"type":"match","data":{"path":{"text":"a.go"},"lines":{"text":"x\n"},"line_number":1,"submatches":[{"match":{"bytes":"!!!"},"start":0,"end":1}]}}`, summaryRecord()},
			wantMalformed: 1,
			wantComplete:  true,
		},
		{
			name:          "end missing binary_offset is malformed",
			records:       []string{`{"type":"end","data":{"path":{"text":"a.go"}}}`, summaryRecord()},
			wantMalformed: 1,
			wantComplete:  true,
		},
		{
			name:          "end wrongly typed binary_offset is malformed",
			records:       []string{`{"type":"end","data":{"path":{"text":"a.go"},"binary_offset":"notint"}}`, summaryRecord()},
			wantMalformed: 1,
			wantComplete:  true,
		},
		{
			name:          "end negative binary_offset is malformed",
			records:       []string{`{"type":"end","data":{"path":{"text":"a.go"},"binary_offset":-1}}`, summaryRecord()},
			wantMalformed: 1,
			wantComplete:  true,
		},
		{
			name:          "summary missing data is malformed",
			records:       []string{textBegin("a.go"), validMatch, endRecord("a.go", nil), `{"type":"summary"}`},
			wantMalformed: 1,
			wantComplete:  false,
			wantStops:     1,
		},
		{
			name:          "summary data not object is malformed",
			records:       []string{textBegin("a.go"), validMatch, endRecord("a.go", nil), `{"type":"summary","data":123}`},
			wantMalformed: 1,
			wantComplete:  false,
			wantStops:     1,
		},

		// --- Issue #9 lifecycle matrix: integrity failure, NOT malformed ---

		{
			name:          "duplicate begin is integrity failure not malformed",
			records:       []string{textBegin("a.go"), textBegin("a.go"), validMatch, endRecord("a.go", nil), summaryRecord()},
			wantMalformed: 0,
			wantComplete:  false,
			wantStops:     1,
		},
		{
			name:           "orphaned match is integrity failure not malformed",
			records:        []string{validMatch, summaryRecord()},
			wantMalformed:  0,
			wantComplete:   false,
			wantStops:      1,
			wantIncomplete: 1,
		},
		{
			name:          "orphaned end is integrity failure not malformed",
			records:       []string{endRecord("a.go", nil), summaryRecord()},
			wantMalformed: 0,
			wantComplete:  false,
		},
		{
			name:           "match after end is integrity failure not malformed",
			records:        []string{textBegin("a.go"), validMatch, endRecord("a.go", nil), textMatch("a.go", "y\n", 2, subSpec{"y", 0, 1}), summaryRecord()},
			wantMalformed:  0,
			wantComplete:   false,
			wantStops:      2,
			wantIncomplete: 1,
		},
		{
			name:          "second summary is integrity failure not malformed",
			records:       []string{summaryRecord(), summaryRecord()},
			wantMalformed: 0,
			wantComplete:  false,
		},
		{
			name:          "record after summary is integrity failure not malformed",
			records:       []string{summaryRecord(), textBegin("a.go")},
			wantMalformed: 0,
			wantComplete:  false,
		},

		// --- Composite cases: both malformed and integrity failure ---

		{
			name:              "trailing unterminated ordinary record is malformed and stream incomplete",
			records:           []string{textBegin("a.go"), validMatch, endRecord("a.go", nil), summaryRecord()},
			trailingMalformed: true,
			wantMalformed:     1,
			wantComplete:      false,
			wantStops:         1,
		},
		{
			name:          "malformed record after valid summary is malformed and after-summary integrity failure",
			records:       []string{summaryRecord(), `{bad json`},
			wantMalformed: 1,
			wantComplete:  false,
		},
		{
			name:          "missing-type record after valid summary is malformed and after-summary integrity failure",
			records:       []string{summaryRecord(), `{"data":{}}`},
			wantMalformed: 1,
			wantComplete:  false,
		},

		// --- Malformed between valid records is skipped, later records indexed ---

		{
			name:          "malformed match between valid records is skipped and later records indexed",
			records:       []string{textBegin("a.go"), `{bad json`, validMatch, endRecord("a.go", nil), summaryRecord()},
			wantMalformed: 1,
			wantComplete:  true,
			wantStops:     1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := searchindex.NewBuilder("/work")
			for _, rec := range tc.records {
				_ = b.Add([]byte(rec))
			}
			if tc.trailingMalformed {
				b.MarkTrailingMalformed()
			}
			idx := b.Build()

			if got := idx.MalformedCount(); got != tc.wantMalformed {
				t.Fatalf("MalformedCount = %d, want %d", got, tc.wantMalformed)
			}
			if got := idx.Integrity().Complete; got != tc.wantComplete {
				t.Fatalf("Integrity().Complete = %v, want %v", got, tc.wantComplete)
			}
			if idx.Len() != tc.wantStops {
				t.Fatalf("Len = %d, want %d", idx.Len(), tc.wantStops)
			}
			if got := countIncomplete(idx.Stops()); got != tc.wantIncomplete {
				t.Fatalf("incomplete stops = %d, want %d", got, tc.wantIncomplete)
			}
			if idx.ExcludedFiles() != tc.wantExcluded {
				t.Fatalf("ExcludedFiles = %d, want %d", idx.ExcludedFiles(), tc.wantExcluded)
			}
		})
	}
}
