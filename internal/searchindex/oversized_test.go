package searchindex_test

import (
	"strings"
	"testing"

	"vrg/internal/searchindex"
)

// matchRecordOfSize builds a valid match record whose JSON encoding is
// exactly size bytes (excluding any newline delimiter). The record uses
// a lines.text field padded with 'x' characters to reach the target
// size. Field order follows json.Marshal (alphabetical), so path comes
// after lines — path may not be recoverable from partial oversized
// data.
func matchRecordOfSize(path string, size int) string {
	overhead := len(textMatch(path, "", 1, subSpec{"x", 0, 1}))
	paddingLen := size - overhead
	if paddingLen < 1 {
		paddingLen = 1
	}
	return textMatch(path, strings.Repeat("x", paddingLen), 1, subSpec{"x", 0, 1})
}

// oversizedMatchRecoverable builds a match record of exactly size bytes
// with the path field placed early (before the large lines.text field)
// so the path is recoverable from the first MaxRecordSize+1 bytes of
// partial oversized data.
func oversizedMatchRecoverable(path string, size int) string {
	prefix := `{"type":"match","data":{"path":{"text":"` + path + `"},"line_number":1,"submatches":[{"match":{"text":"x"},"start":0,"end":1}],"lines":{"text":"`
	suffix := `"}}}`
	paddingLen := size - len(prefix) - len(suffix)
	if paddingLen < 1 {
		paddingLen = 1
	}
	return prefix + strings.Repeat("x", paddingLen) + suffix
}

// readFrom builds a new Builder, feeds records through ReadFrom, and
// returns the index.
func readFrom(t *testing.T, workdir string, data string) *searchindex.Index {
	t.Helper()
	b := searchindex.NewBuilder(workdir)
	if _, err := b.ReadFrom(strings.NewReader(data)); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}
	return b.Build()
}

// TestOversizedBoundary tests the 64 MiB record payload limit.
func TestOversizedBoundary(t *testing.T) {
	mib := 64 * 1024 * 1024

	// Exactly 64 MiB record is accepted.
	t.Run("exactly 64 MiB is accepted", func(t *testing.T) {
		rec := matchRecordOfSize("a.go", mib)
		if len(rec) != mib {
			t.Fatalf("record size = %d, want %d", len(rec), mib)
		}
		data := textBegin("a.go") + "\n" + rec + "\n" + endRecord("a.go", nil) + "\n" + summaryRecord() + "\n"
		idx := readFrom(t, "/work", data)
		if idx.OversizedCount() != 0 {
			t.Fatalf("OversizedCount = %d, want 0", idx.OversizedCount())
		}
		if idx.MalformedCount() != 0 {
			t.Fatalf("MalformedCount = %d, want 0", idx.MalformedCount())
		}
		if idx.Len() != 1 {
			t.Fatalf("Len = %d, want 1", idx.Len())
		}
		if !idx.Integrity().Complete {
			t.Fatalf("Integrity().Complete = false, want true")
		}
	})

	// 64 MiB + 1 byte record is skipped as oversized.
	t.Run("64 MiB plus 1 is oversized", func(t *testing.T) {
		rec := matchRecordOfSize("a.go", mib+1)
		if len(rec) != mib+1 {
			t.Fatalf("record size = %d, want %d", len(rec), mib+1)
		}
		data := rec + "\n" + summaryRecord() + "\n"
		idx := readFrom(t, "/work", data)
		if idx.OversizedCount() != 1 {
			t.Fatalf("OversizedCount = %d, want 1", idx.OversizedCount())
		}
		if idx.Len() != 0 {
			t.Fatalf("Len = %d, want 0", idx.Len())
		}
	})
}

// TestOversizedResynchronization tests that an oversized record is
// discarded through the next newline and parsing resumes at the next
// record.
func TestOversizedResynchronization(t *testing.T) {
	mib := 64 * 1024 * 1024
	oversized := matchRecordOfSize("a.go", mib+100)
	valid := textMatch("b.go", "y\n", 1, subSpec{"y", 0, 1})
	data := oversized + "\n" +
		textBegin("b.go") + "\n" +
		valid + "\n" +
		endRecord("b.go", nil) + "\n" +
		summaryRecord() + "\n"
	idx := readFrom(t, "/work", data)
	if idx.OversizedCount() != 1 {
		t.Fatalf("OversizedCount = %d, want 1", idx.OversizedCount())
	}
	if idx.Len() != 1 {
		t.Fatalf("Len = %d, want 1 (valid record after oversized should be indexed)", idx.Len())
	}
	stops := idx.Stops()
	if string(stops[0].RawPath) != "b.go" {
		t.Fatalf("stop path = %q, want b.go", stops[0].RawPath)
	}
	if !idx.Integrity().Complete {
		t.Fatalf("Integrity().Complete = false, want true")
	}
}

// TestOversizedFinalRecord tests that a final oversized record without
// a newline increments oversized count, malformed count, and marks the
// stream incomplete.
func TestOversizedFinalRecord(t *testing.T) {
	mib := 64 * 1024 * 1024
	oversized := matchRecordOfSize("a.go", mib+100)
	// No trailing newline on the oversized record.
	data := textBegin("a.go") + "\n" +
		endRecord("a.go", nil) + "\n" +
		summaryRecord() + "\n" +
		oversized // no newline
	idx := readFrom(t, "/work", data)
	if idx.OversizedCount() != 1 {
		t.Fatalf("OversizedCount = %d, want 1", idx.OversizedCount())
	}
	if idx.MalformedCount() != 1 {
		t.Fatalf("MalformedCount = %d, want 1", idx.MalformedCount())
	}
	if idx.Integrity().Complete {
		t.Fatalf("Integrity().Complete = true, want false")
	}
}

// TestOversizedDiagnostics tests path recovery and diagnostics for
// oversized records.
func TestOversizedDiagnostics(t *testing.T) {
	mib := 64 * 1024 * 1024

	// Oversized match with recoverable path reports sanitized path.
	t.Run("recoverable path diagnostic", func(t *testing.T) {
		rec := oversizedMatchRecoverable("a.go", mib+100)
		data := rec + "\n" + summaryRecord() + "\n"
		idx := readFrom(t, "/work", data)
		if idx.OversizedCount() != 1 {
			t.Fatalf("OversizedCount = %d, want 1", idx.OversizedCount())
		}
		diags := idx.OversizedDiagnostics()
		if len(diags) != 1 {
			t.Fatalf("len(OversizedDiagnostics) = %d, want 1", len(diags))
		}
		if !strings.Contains(diags[0], "a.go") {
			t.Fatalf("diagnostic = %q, want it to contain a.go", diags[0])
		}
		if !strings.Contains(diags[0], "oversized record skipped for") {
			t.Fatalf("diagnostic = %q, want it to contain 'oversized record skipped for'", diags[0])
		}
	})

	// Oversized record where path is not recoverable reports count only.
	t.Run("no path recovery count only", func(t *testing.T) {
		rec := matchRecordOfSize("a.go", mib+100)
		data := rec + "\n" + summaryRecord() + "\n"
		idx := readFrom(t, "/work", data)
		if idx.OversizedCount() != 1 {
			t.Fatalf("OversizedCount = %d, want 1", idx.OversizedCount())
		}
		diags := idx.OversizedDiagnostics()
		if len(diags) != 0 {
			t.Fatalf("len(OversizedDiagnostics) = %d, want 0 (path not recoverable)", len(diags))
		}
	})

	// File whose only match records were oversized is absent from stops
	// while its path appears in the diagnostic.
	t.Run("file with only oversized matches absent from stops", func(t *testing.T) {
		oversizedA := oversizedMatchRecoverable("a.go", mib+100)
		validB := textMatch("b.go", "y\n", 1, subSpec{"y", 0, 1})
		data := oversizedA + "\n" +
			textBegin("b.go") + "\n" +
			validB + "\n" +
			endRecord("b.go", nil) + "\n" +
			summaryRecord() + "\n"
		idx := readFrom(t, "/work", data)
		if idx.OversizedCount() != 1 {
			t.Fatalf("OversizedCount = %d, want 1", idx.OversizedCount())
		}
		if idx.Len() != 1 {
			t.Fatalf("Len = %d, want 1 (only b.go)", idx.Len())
		}
		stops := idx.Stops()
		if string(stops[0].RawPath) != "b.go" {
			t.Fatalf("stop path = %q, want b.go", stops[0].RawPath)
		}
		diags := idx.OversizedDiagnostics()
		found := false
		for _, d := range diags {
			if strings.Contains(d, "a.go") {
				found = true
			}
		}
		if !found {
			t.Fatalf("no diagnostic contains a.go: %v", diags)
		}
	})
}

// TestUnknownType tests that unknown string event types are counted
// separately from malformed records.
func TestUnknownType(t *testing.T) {
	t.Run("unknown type is counted separately", func(t *testing.T) {
		b := searchindex.NewBuilder("/work")
		_ = b.Add([]byte(`{"type":"unknown","data":{}}`))
		_ = b.Add([]byte(summaryRecord()))
		idx := b.Build()
		if idx.UnknownCount() != 1 {
			t.Fatalf("UnknownCount = %d, want 1", idx.UnknownCount())
		}
		if idx.MalformedCount() != 0 {
			t.Fatalf("MalformedCount = %d, want 0 (unknown is not malformed)", idx.MalformedCount())
		}
	})

	t.Run("unknown type does not substitute for summary", func(t *testing.T) {
		b := searchindex.NewBuilder("/work")
		_ = b.Add([]byte(textBegin("a.go")))
		_ = b.Add([]byte(textMatch("a.go", "x\n", 1, subSpec{"x", 0, 1})))
		_ = b.Add([]byte(endRecord("a.go", nil)))
		_ = b.Add([]byte(`{"type":"unknown","data":{}}`))
		// No summary — unknown does not substitute.
		idx := b.Build()
		if idx.UnknownCount() != 1 {
			t.Fatalf("UnknownCount = %d, want 1", idx.UnknownCount())
		}
		if idx.Integrity().Complete {
			t.Fatalf("Integrity().Complete = true, want false (no summary)")
		}
	})

	t.Run("unknown type after summary is unknown and after-summary integrity failure", func(t *testing.T) {
		b := searchindex.NewBuilder("/work")
		_ = b.Add([]byte(summaryRecord()))
		_ = b.Add([]byte(`{"type":"unknown","data":{}}`))
		idx := b.Build()
		if idx.UnknownCount() != 1 {
			t.Fatalf("UnknownCount = %d, want 1", idx.UnknownCount())
		}
		if idx.Integrity().Complete {
			t.Fatalf("Integrity().Complete = true, want false (after-summary)")
		}
		if idx.MalformedCount() != 0 {
			t.Fatalf("MalformedCount = %d, want 0 (unknown is not malformed)", idx.MalformedCount())
		}
	})
}
