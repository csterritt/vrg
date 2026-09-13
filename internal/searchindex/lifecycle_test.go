package searchindex_test

import (
	"testing"

	"vrg/internal/searchindex"
)

// countIncomplete returns the number of stops with Incomplete set.
func countIncomplete(stops []searchindex.Stop) int {
	n := 0
	for _, s := range stops {
		if s.Incomplete {
			n++
		}
	}
	return n
}

// TestLifecycleMatrix is the single table-driven test covering every row
// of the Issue #9 lifecycle transition matrix and the stream-integrity
// rules. Later issues extend this table with new rows rather than
// duplicating the decision. Each row feeds a sequence of well-formed
// ripgrep JSON records through a Builder, optionally signals a trailing
// malformed record, and asserts the resulting stream integrity, stop
// count, incomplete-stop count, and excluded-file count.
func TestLifecycleMatrix(t *testing.T) {
	cases := []struct {
		name              string
		records           []string
		trailingMalformed bool
		wantComplete      bool
		wantStops         int
		wantIncomplete    int
		wantExcluded      int
	}{
		// --- Transition matrix rows ---

		{
			name:         "begin while not open opens the file",
			records:      []string{textBegin("a.go"), textMatch("a.go", "x\n", 1, subSpec{"x", 0, 1}), endRecord("a.go", nil), summaryRecord()},
			wantComplete: true,
			wantStops:    1,
		},
		{
			name:         "begin while already open is integrity failure duplicate begin",
			records:      []string{textBegin("a.go"), textBegin("a.go"), textMatch("a.go", "x\n", 1, subSpec{"x", 0, 1}), endRecord("a.go", nil), summaryRecord()},
			wantComplete: false,
			wantStops:    1,
		},
		{
			name:         "match while open indexes under the file",
			records:      []string{textBegin("a.go"), textMatch("a.go", "x\n", 1, subSpec{"x", 0, 1}), endRecord("a.go", nil), summaryRecord()},
			wantComplete: true,
			wantStops:    1,
		},
		{
			name:           "match never opened is orphaned retained incomplete",
			records:        []string{textMatch("a.go", "x\n", 1, subSpec{"x", 0, 1}), summaryRecord()},
			wantComplete:   false,
			wantStops:      1,
			wantIncomplete: 1,
		},
		{
			name:           "match after non-binary end is orphaned retained incomplete",
			records:        []string{textBegin("a.go"), textMatch("a.go", "x\n", 1, subSpec{"x", 0, 1}), endRecord("a.go", nil), textMatch("a.go", "y\n", 2, subSpec{"y", 0, 1}), summaryRecord()},
			wantComplete:   false,
			wantStops:      2,
			wantIncomplete: 1,
		},
		{
			name:         "end while open closes the file",
			records:      []string{textBegin("a.go"), endRecord("a.go", nil), summaryRecord()},
			wantComplete: true,
			wantStops:    0,
		},
		{
			name:         "end while not open is orphaned integrity failure",
			records:      []string{endRecord("a.go", nil), summaryRecord()},
			wantComplete: false,
			wantStops:    0,
		},
		{
			name:         "context before begin has no lifecycle effect",
			records:      []string{contextRecord(), textBegin("a.go"), textMatch("a.go", "x\n", 1, subSpec{"x", 0, 1}), endRecord("a.go", nil), summaryRecord()},
			wantComplete: true,
			wantStops:    1,
		},
		{
			name:         "context after end has no lifecycle effect",
			records:      []string{textBegin("a.go"), textMatch("a.go", "x\n", 1, subSpec{"x", 0, 1}), endRecord("a.go", nil), contextRecord(), summaryRecord()},
			wantComplete: true,
			wantStops:    1,
		},
		{
			name:         "context after summary has no lifecycle effect",
			records:      []string{summaryRecord(), contextRecord()},
			wantComplete: true,
			wantStops:    0,
		},
		{
			name:           "file still open at stream end is integrity failure retained incomplete",
			records:        []string{textBegin("a.go"), textMatch("a.go", "x\n", 1, subSpec{"x", 0, 1}), summaryRecord()},
			wantComplete:   false,
			wantStops:      1,
			wantIncomplete: 1,
		},
		{
			name:         "summary alone is complete zero-result stream",
			records:      []string{summaryRecord()},
			wantComplete: true,
			wantStops:    0,
		},
		{
			name:         "summary missing is integrity failure",
			records:      []string{textBegin("a.go"), textMatch("a.go", "x\n", 1, subSpec{"x", 0, 1}), endRecord("a.go", nil)},
			wantComplete: false,
			wantStops:    1,
		},
		{
			name:         "second summary is integrity failure",
			records:      []string{summaryRecord(), summaryRecord()},
			wantComplete: false,
			wantStops:    0,
		},
		{
			name:         "any record after summary is integrity failure",
			records:      []string{summaryRecord(), textBegin("a.go")},
			wantComplete: false,
			wantStops:    0,
		},
		{
			name:              "trailing unterminated record is malformed and stream incomplete",
			records:           []string{textBegin("a.go"), textMatch("a.go", "x\n", 1, subSpec{"x", 0, 1}), endRecord("a.go", nil), summaryRecord()},
			trailingMalformed: true,
			wantComplete:      false,
			wantStops:         1,
		},

		// --- Path-identity and interleaving ---

		{
			name:         "text and bytes path identity agreement",
			records:      []string{textBegin("a.go"), bytesMatch([]byte("a.go"), []byte("x\n"), 1, subSpec{"x", 0, 1}), endRecord("a.go", nil), summaryRecord()},
			wantComplete: true,
			wantStops:    1,
		},
		{
			name:         "interleaved open files with valid pairing",
			records:      []string{textBegin("a.go"), textBegin("b.go"), textMatch("a.go", "x\n", 1, subSpec{"x", 0, 1}), textMatch("b.go", "y\n", 1, subSpec{"y", 0, 1}), endRecord("a.go", nil), endRecord("b.go", nil), summaryRecord()},
			wantComplete: true,
			wantStops:    2,
		},

		// --- Binary-exclusion precedence ---

		{
			name:         "binary exclusion precedence over orphan retention",
			records:      []string{textBegin("a.go"), textMatch("a.go", "x\n", 1, subSpec{"x", 0, 1}), endRecord("a.go", 42), textMatch("a.go", "y\n", 2, subSpec{"y", 0, 1}), summaryRecord()},
			wantComplete: false,
			wantStops:    0,
			wantExcluded: 1,
		},
		{
			name:           "orphaned match after non-binary end retained with incomplete metadata",
			records:        []string{textBegin("a.go"), textMatch("a.go", "x\n", 1, subSpec{"x", 0, 1}), endRecord("a.go", nil), textMatch("a.go", "y\n", 2, subSpec{"y", 0, 1}), summaryRecord()},
			wantComplete:   false,
			wantStops:      2,
			wantIncomplete: 1,
		},
		{
			name:           "one file orphaned one file valid in same stream",
			records:        []string{textBegin("a.go"), textMatch("a.go", "x\n", 1, subSpec{"x", 0, 1}), endRecord("a.go", nil), textMatch("b.go", "y\n", 1, subSpec{"y", 0, 1}), endRecord("b.go", nil), summaryRecord()},
			wantComplete:   false,
			wantStops:      2,
			wantIncomplete: 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := searchindex.NewBuilder("/work")
			for _, rec := range tc.records {
				if err := b.Add([]byte(rec)); err != nil {
					t.Fatalf("Add %q: %v", rec, err)
				}
			}
			if tc.trailingMalformed {
				b.MarkTrailingMalformed()
			}
			idx := b.Build()

			gotComplete := idx.Integrity().Complete
			if gotComplete != tc.wantComplete {
				t.Fatalf("Integrity().Complete = %v, want %v", gotComplete, tc.wantComplete)
			}
			if idx.Len() != tc.wantStops {
				t.Fatalf("Len = %d, want %d", idx.Len(), tc.wantStops)
			}
			gotIncomplete := countIncomplete(idx.Stops())
			if gotIncomplete != tc.wantIncomplete {
				t.Fatalf("incomplete stops = %d, want %d", gotIncomplete, tc.wantIncomplete)
			}
			if idx.ExcludedFiles() != tc.wantExcluded {
				t.Fatalf("ExcludedFiles = %d, want %d", idx.ExcludedFiles(), tc.wantExcluded)
			}
		})
	}
}

// TestPathIdentityTextBytesAgreement verifies that a begin with text
// encoding and a match with bytes encoding for the same decoded path
// bytes are treated as the same file — the match is "while open" and
// indexes normally, not as an orphaned match.
func TestPathIdentityTextBytesAgreement(t *testing.T) {
	b := searchindex.NewBuilder("/work")
	records := []string{
		textBegin("src/a.go"),
		bytesMatch([]byte("src/a.go"), []byte("hello\n"), 1, subSpec{"hello", 0, 5}),
		endRecord("src/a.go", nil),
		summaryRecord(),
	}
	for _, rec := range records {
		if err := b.Add([]byte(rec)); err != nil {
			t.Fatalf("Add %q: %v", rec, err)
		}
	}
	idx := b.Build()
	if !idx.Integrity().Complete {
		t.Fatalf("Integrity().Complete = false, want true (text/bytes path identity: begin and match are the same file)")
	}
	if idx.Len() != 1 {
		t.Fatalf("Len = %d, want 1", idx.Len())
	}
	stops := idx.Stops()
	if stops[0].Incomplete {
		t.Fatalf("stop is marked Incomplete, want false (match was while open)")
	}
}

// TestInterleavedOpenFiles verifies that per-path open state is tracked
// independently so interleaved events across multiple files produce
// correct lifecycle validation.
func TestInterleavedOpenFiles(t *testing.T) {
	b := searchindex.NewBuilder("/work")
	records := []string{
		textBegin("a.go"),
		textBegin("b.go"),
		textMatch("a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("b.go", "y\n", 1, subSpec{"y", 0, 1}),
		textMatch("a.go", "z\n", 3, subSpec{"z", 0, 1}),
		endRecord("a.go", nil),
		endRecord("b.go", nil),
		summaryRecord(),
	}
	for _, rec := range records {
		if err := b.Add([]byte(rec)); err != nil {
			t.Fatalf("Add %q: %v", rec, err)
		}
	}
	idx := b.Build()
	if !idx.Integrity().Complete {
		t.Fatalf("Integrity().Complete = false, want true (interleaved files with valid pairing)")
	}
	if idx.Len() != 3 {
		t.Fatalf("Len = %d, want 3", idx.Len())
	}
	for _, s := range idx.Stops() {
		if s.Incomplete {
			t.Fatalf("stop %q line %d is Incomplete, want false", s.RawPath, s.LineNumber)
		}
	}
}

// TestBinaryExclusionPrecedenceOverOrphanRetention verifies that a match
// arriving after a binary-excluding end is NOT retained — binary
// exclusion takes precedence over the general orphan-retention rule.
// The file stays excluded from the file list.
func TestBinaryExclusionPrecedenceOverOrphanRetention(t *testing.T) {
	b := searchindex.NewBuilder("/work")
	records := []string{
		textBegin("a.go"),
		textMatch("a.go", "x\n", 1, subSpec{"x", 0, 1}),
		endRecord("a.go", 42),
		textMatch("a.go", "y\n", 2, subSpec{"y", 0, 1}),
		summaryRecord(),
	}
	for _, rec := range records {
		if err := b.Add([]byte(rec)); err != nil {
			t.Fatalf("Add %q: %v", rec, err)
		}
	}
	idx := b.Build()
	// The late match after a binary end must NOT be retained.
	if idx.Len() != 0 {
		t.Fatalf("Len = %d, want 0 (binary exclusion precedence: late match not retained)", idx.Len())
	}
	if idx.ExcludedFiles() != 1 {
		t.Fatalf("ExcludedFiles = %d, want 1", idx.ExcludedFiles())
	}
	// The stream is incomplete (orphaned match after end).
	if idx.Integrity().Complete {
		t.Fatalf("Integrity().Complete = true, want false (orphaned match after binary end)")
	}
}

// TestOrphanedMatchRetainedIncompleteMetadata verifies that an orphaned
// match (file never opened) is retained in the index but marked with
// incomplete metadata.
func TestOrphanedMatchRetainedIncompleteMetadata(t *testing.T) {
	b := searchindex.NewBuilder("/work")
	records := []string{
		textMatch("a.go", "x\n", 1, subSpec{"x", 0, 1}),
		summaryRecord(),
	}
	for _, rec := range records {
		if err := b.Add([]byte(rec)); err != nil {
			t.Fatalf("Add %q: %v", rec, err)
		}
	}
	idx := b.Build()
	if idx.Len() != 1 {
		t.Fatalf("Len = %d, want 1 (orphaned match retained)", idx.Len())
	}
	stops := idx.Stops()
	if !stops[0].Incomplete {
		t.Fatalf("stop is not marked Incomplete, want true (orphaned match has incomplete metadata)")
	}
	if idx.Integrity().Complete {
		t.Fatalf("Integrity().Complete = true, want false (orphaned match)")
	}
}

// TestFileStillOpenRetainedIncompleteMetadata verifies that a file still
// open when the stream ends has its matches retained with incomplete
// metadata, and the stream is incomplete.
func TestFileStillOpenRetainedIncompleteMetadata(t *testing.T) {
	b := searchindex.NewBuilder("/work")
	records := []string{
		textBegin("a.go"),
		textMatch("a.go", "x\n", 1, subSpec{"x", 0, 1}),
		summaryRecord(),
	}
	for _, rec := range records {
		if err := b.Add([]byte(rec)); err != nil {
			t.Fatalf("Add %q: %v", rec, err)
		}
	}
	idx := b.Build()
	if idx.Len() != 1 {
		t.Fatalf("Len = %d, want 1 (matches retained for still-open file)", idx.Len())
	}
	stops := idx.Stops()
	if !stops[0].Incomplete {
		t.Fatalf("stop is not marked Incomplete, want true (file still open: missing end)")
	}
	if idx.Integrity().Complete {
		t.Fatalf("Integrity().Complete = true, want false (file still open at stream end)")
	}
}

// TestSummaryAloneCompleteZeroResults verifies that a summary record
// alone is a complete zero-result stream.
func TestSummaryAloneCompleteZeroResults(t *testing.T) {
	b := searchindex.NewBuilder("/work")
	if err := b.Add([]byte(summaryRecord())); err != nil {
		t.Fatalf("Add summary: %v", err)
	}
	idx := b.Build()
	if !idx.Integrity().Complete {
		t.Fatalf("Integrity().Complete = false, want true (summary alone is complete)")
	}
	if idx.Len() != 0 {
		t.Fatalf("Len = %d, want 0", idx.Len())
	}
}

// TestTrailingUnterminatedRecordMalformedAndIncomplete verifies that
// signaling a trailing unterminated record marks the stream as
// incomplete.
func TestTrailingUnterminatedRecordMalformedAndIncomplete(t *testing.T) {
	b := searchindex.NewBuilder("/work")
	records := []string{
		textBegin("a.go"),
		textMatch("a.go", "x\n", 1, subSpec{"x", 0, 1}),
		endRecord("a.go", nil),
		summaryRecord(),
	}
	for _, rec := range records {
		if err := b.Add([]byte(rec)); err != nil {
			t.Fatalf("Add %q: %v", rec, err)
		}
	}
	b.MarkTrailingMalformed()
	idx := b.Build()
	if idx.Integrity().Complete {
		t.Fatalf("Integrity().Complete = true, want false (trailing unterminated record)")
	}
}
