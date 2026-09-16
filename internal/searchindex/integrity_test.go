package searchindex_test

import (
	"strings"
	"testing"

	"vrg/internal/searchindex"
)

// causeSpec describes one expected structured integrity cause: the
// stable kind plus the affected raw path where the record names one.
type causeSpec struct {
	kind searchindex.IntegrityCauseKind
	// path is the expected raw path. hasPath distinguishes a cause
	// carrying a path from a cause with no path.
	path    string
	hasPath bool
}

// cause builds a path-carrying causeSpec.
func cause(kind searchindex.IntegrityCauseKind, path string) causeSpec {
	return causeSpec{kind: kind, path: path, hasPath: true}
}

// causeNoPath builds a causeSpec for a cause that carries no path.
func causeNoPath(kind searchindex.IntegrityCauseKind) causeSpec {
	return causeSpec{kind: kind}
}

// causesStr renders a cause list for failure messages.
func causesStr(causes []searchindex.IntegrityCause) string {
	var b strings.Builder
	b.WriteString("[")
	for i, c := range causes {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(string(c.Kind))
		if c.Path != nil {
			b.WriteString("(")
			b.WriteString(string(c.Path))
			b.WriteString(")")
		}
	}
	b.WriteString("]")
	return b.String()
}

// assertCauses requires the index integrity causes to equal want
// exactly, in order.
func assertCauses(t *testing.T, idx *searchindex.Index, want []causeSpec) {
	t.Helper()
	got := idx.Integrity().Causes
	if len(got) != len(want) {
		t.Fatalf("Integrity().Causes = %s (len %d), want len %d", causesStr(got), len(got), len(want))
	}
	for i, w := range want {
		if got[i].Kind != w.kind {
			t.Fatalf("Causes[%d].Kind = %q, want %q (full: %s)", i, got[i].Kind, w.kind, causesStr(got))
		}
		if w.hasPath {
			if string(got[i].Path) != w.path {
				t.Fatalf("Causes[%d].Path = %q, want %q (full: %s)", i, got[i].Path, w.path, causesStr(got))
			}
		} else if got[i].Path != nil {
			t.Fatalf("Causes[%d].Path = %q, want nil (full: %s)", i, got[i].Path, causesStr(got))
		}
	}
}

// TestIntegrityCauseMatrix is the single table-driven test covering the
// Issue #36 structured integrity-cause contract: every Issue #9
// lifecycle-failure row produces one structured cause per offending
// physical record (stable kind plus raw path where applicable), in
// detection order for mid-stream violations followed by the mandated
// end-of-stream order (missing ends by unsigned raw-path bytes, then
// missing summary, then the trailing unterminated record). Overlap
// precedence pins one integrity cause per physical record: a second
// summary contributes only extra summary; every other post-summary
// record contributes only record after summary and is not
// lifecycle-processed; a post-summary trailing fragment contributes
// record after summary (not unterminated final record) while retaining
// its malformed count.
func TestIntegrityCauseMatrix(t *testing.T) {
	validMatch := textMatch("a.go", "x\n", 1, subSpec{"x", 0, 1})

	cases := []struct {
		name              string
		records           []string
		trailingMalformed bool
		wantCauses        []causeSpec
		wantComplete      bool
		wantStops         int
		wantMalformed     int
		wantUnknown       int
		wantExcluded      int
	}{
		// --- Baseline ---

		{
			name:         "complete stream carries no causes",
			records:      []string{textBegin("a.go"), validMatch, endRecord("a.go", nil), summaryRecord()},
			wantComplete: true,
			wantStops:    1,
		},

		// --- Issue #9 lifecycle-failure rows ---

		{
			name:       "duplicate begin records duplicate-begin cause with path",
			records:    []string{textBegin("a.go"), textBegin("a.go"), validMatch, endRecord("a.go", nil), summaryRecord()},
			wantCauses: []causeSpec{cause(searchindex.IntegrityCauseDuplicateBegin, "a.go")},
			wantStops:  1,
		},
		{
			name:       "match never opened records orphaned-match cause with path",
			records:    []string{validMatch, summaryRecord()},
			wantCauses: []causeSpec{cause(searchindex.IntegrityCauseOrphanedMatch, "a.go")},
			wantStops:  1,
		},
		{
			name: "match after non-binary end records orphaned-match cause",
			records: []string{
				textBegin("a.go"), validMatch, endRecord("a.go", nil),
				textMatch("a.go", "y\n", 2, subSpec{"y", 0, 1}), summaryRecord(),
			},
			wantCauses: []causeSpec{cause(searchindex.IntegrityCauseOrphanedMatch, "a.go")},
			wantStops:  2,
		},
		{
			name: "match after binary-excluding end records match-after-binary-end cause",
			records: []string{
				textBegin("a.go"), validMatch, endRecord("a.go", 42),
				textMatch("a.go", "y\n", 2, subSpec{"y", 0, 1}), summaryRecord(),
			},
			wantCauses:   []causeSpec{cause(searchindex.IntegrityCauseMatchAfterBinaryEnd, "a.go")},
			wantExcluded: 1,
		},
		{
			name:       "end while not open records orphaned-end cause with path",
			records:    []string{endRecord("a.go", nil), summaryRecord()},
			wantCauses: []causeSpec{cause(searchindex.IntegrityCauseOrphanedEnd, "a.go")},
		},
		{
			name:       "file still open at stream end records missing-end cause with path",
			records:    []string{textBegin("a.go"), validMatch, summaryRecord()},
			wantCauses: []causeSpec{cause(searchindex.IntegrityCauseMissingEnd, "a.go")},
			wantStops:  1,
		},
		{
			name:       "missing summary records missing-summary cause without path",
			records:    []string{textBegin("a.go"), validMatch, endRecord("a.go", nil)},
			wantCauses: []causeSpec{causeNoPath(searchindex.IntegrityCauseMissingSummary)},
			wantStops:  1,
		},
		{
			name:       "second summary records only extra summary",
			records:    []string{summaryRecord(), summaryRecord()},
			wantCauses: []causeSpec{causeNoPath(searchindex.IntegrityCauseExtraSummary)},
		},
		{
			name:              "trailing unterminated record outside post-summary records unterminated-final-record cause plus malformed",
			records:           []string{textBegin("a.go"), validMatch, endRecord("a.go", nil)},
			trailingMalformed: true,
			wantCauses: []causeSpec{
				causeNoPath(searchindex.IntegrityCauseMissingSummary),
				causeNoPath(searchindex.IntegrityCauseUnterminatedRecord),
			},
			wantMalformed: 1,
			wantStops:     1,
		},

		// --- Post-summary precedence ---

		{
			name:       "begin after summary records only record after summary and cannot open the file",
			records:    []string{summaryRecord(), textBegin("q.go")},
			wantCauses: []causeSpec{causeNoPath(searchindex.IntegrityCauseRecordAfterSummary)},
		},
		{
			name:       "context after summary records only record after summary",
			records:    []string{summaryRecord(), contextRecord()},
			wantCauses: []causeSpec{causeNoPath(searchindex.IntegrityCauseRecordAfterSummary)},
		},
		{
			name:       "match after summary records only record after summary and is not indexed",
			records:    []string{summaryRecord(), textMatch("q.go", "x\n", 1, subSpec{"x", 0, 1})},
			wantCauses: []causeSpec{causeNoPath(searchindex.IntegrityCauseRecordAfterSummary)},
			wantStops:  0,
		},
		{
			name:       "end after summary records only record after summary and does not close the file",
			records:    []string{textBegin("a.go"), summaryRecord(), endRecord("a.go", nil)},
			wantCauses: []causeSpec{causeNoPath(searchindex.IntegrityCauseRecordAfterSummary), cause(searchindex.IntegrityCauseMissingEnd, "a.go")},
		},
		{
			name:       "schema-invalid begin after summary records only record after summary without malformed count",
			records:    []string{summaryRecord(), `{"type":"begin","data":{}}`},
			wantCauses: []causeSpec{causeNoPath(searchindex.IntegrityCauseRecordAfterSummary)},
		},
		{
			name:          "malformed JSON after summary records record after summary plus malformed count",
			records:       []string{summaryRecord(), `{bad json`},
			wantCauses:    []causeSpec{causeNoPath(searchindex.IntegrityCauseRecordAfterSummary)},
			wantMalformed: 1,
		},
		{
			name:          "missing-type record after summary records record after summary plus malformed count",
			records:       []string{summaryRecord(), `{"data":{}}`},
			wantCauses:    []causeSpec{causeNoPath(searchindex.IntegrityCauseRecordAfterSummary)},
			wantMalformed: 1,
		},
		{
			name:          "malformed-data second summary records extra summary plus malformed count",
			records:       []string{summaryRecord(), `{"type":"summary"}`},
			wantCauses:    []causeSpec{causeNoPath(searchindex.IntegrityCauseExtraSummary)},
			wantMalformed: 1,
		},
		{
			name:        "unknown type after summary records record after summary plus unknown count",
			records:     []string{summaryRecord(), `{"type":"mystery","data":{}}`},
			wantCauses:  []causeSpec{causeNoPath(searchindex.IntegrityCauseRecordAfterSummary)},
			wantUnknown: 1,
		},
		{
			name:              "trailing unterminated fragment after summary is record after summary plus malformed, not unterminated",
			records:           []string{textBegin("a.go"), validMatch, endRecord("a.go", nil), summaryRecord()},
			trailingMalformed: true,
			wantCauses:        []causeSpec{causeNoPath(searchindex.IntegrityCauseRecordAfterSummary)},
			wantMalformed:     1,
			wantStops:         1,
		},

		// --- Pre-summary context positions (Issue #44) ---

		{
			name:         "context before begin records no causes and is ignored",
			records:      []string{contextRecord(), textBegin("a.go"), validMatch, endRecord("a.go", nil), summaryRecord()},
			wantComplete: true,
			wantStops:    1,
		},
		{
			name:         "context while open records no causes and is ignored",
			records:      []string{textBegin("a.go"), contextRecord(), validMatch, contextRecord(), endRecord("a.go", nil), summaryRecord()},
			wantComplete: true,
			wantStops:    1,
		},
		{
			name:         "context after end before summary records no causes and is ignored",
			records:      []string{textBegin("a.go"), validMatch, endRecord("a.go", nil), contextRecord(), summaryRecord()},
			wantComplete: true,
			wantStops:    1,
		},

		// --- Ordering ---

		{
			name:    "mid-stream causes precede end-of-stream causes in mandated order",
			records: []string{textMatch("x.go", "x\n", 1, subSpec{"x", 0, 1}), textBegin("b.go"), textBegin("a.go")},
			wantCauses: []causeSpec{
				cause(searchindex.IntegrityCauseOrphanedMatch, "x.go"),
				cause(searchindex.IntegrityCauseMissingEnd, "a.go"),
				cause(searchindex.IntegrityCauseMissingEnd, "b.go"),
				causeNoPath(searchindex.IntegrityCauseMissingSummary),
			},
			wantStops: 1,
		},
		{
			name:    "missing ends for still-open files are ordered by unsigned raw-path bytes",
			records: []string{bytesBegin([]byte{0xff, '.', 'g'}), textBegin("a.go"), textBegin("m.go")},
			wantCauses: []causeSpec{
				cause(searchindex.IntegrityCauseMissingEnd, "a.go"),
				cause(searchindex.IntegrityCauseMissingEnd, "m.go"),
				cause(searchindex.IntegrityCauseMissingEnd, "\xff.g"),
				causeNoPath(searchindex.IntegrityCauseMissingSummary),
			},
		},
		{
			name:              "missing ends then missing summary then unterminated record at stream end",
			records:           []string{textBegin("a.go"), validMatch},
			trailingMalformed: true,
			wantCauses: []causeSpec{
				cause(searchindex.IntegrityCauseMissingEnd, "a.go"),
				causeNoPath(searchindex.IntegrityCauseMissingSummary),
				causeNoPath(searchindex.IntegrityCauseUnterminatedRecord),
			},
			wantMalformed: 1,
			wantStops:     1,
		},
		{
			name:       "second summary then post-summary context keeps detection order",
			records:    []string{summaryRecord(), summaryRecord(), contextRecord()},
			wantCauses: []causeSpec{causeNoPath(searchindex.IntegrityCauseExtraSummary), causeNoPath(searchindex.IntegrityCauseRecordAfterSummary)},
		},

		// --- Uncapped multiplicity ---

		{
			name: "repeated orphaned matches for one path produce one cause each without aggregation",
			records: []string{
				textMatch("a.go", "x\n", 1, subSpec{"x", 0, 1}),
				textMatch("a.go", "y\n", 2, subSpec{"y", 0, 1}),
				textMatch("a.go", "z\n", 3, subSpec{"z", 0, 1}),
				summaryRecord(),
			},
			wantCauses: []causeSpec{
				cause(searchindex.IntegrityCauseOrphanedMatch, "a.go"),
				cause(searchindex.IntegrityCauseOrphanedMatch, "a.go"),
				cause(searchindex.IntegrityCauseOrphanedMatch, "a.go"),
			},
			wantStops: 3,
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

			assertCauses(t, idx, tc.wantCauses)
			if got := idx.Integrity().Complete; got != tc.wantComplete {
				t.Fatalf("Integrity().Complete = %v, want %v", got, tc.wantComplete)
			}
			if idx.Len() != tc.wantStops {
				t.Fatalf("Len = %d, want %d", idx.Len(), tc.wantStops)
			}
			if got := idx.MalformedCount(); got != tc.wantMalformed {
				t.Fatalf("MalformedCount = %d, want %d", got, tc.wantMalformed)
			}
			if got := idx.UnknownCount(); got != tc.wantUnknown {
				t.Fatalf("UnknownCount = %d, want %d", got, tc.wantUnknown)
			}
			if got := idx.ExcludedFiles(); got != tc.wantExcluded {
				t.Fatalf("ExcludedFiles = %d, want %d", got, tc.wantExcluded)
			}
		})
	}
}

// TestIntegrityCausePostSummaryOversized covers the post-summary
// oversized dual representation through the bounded record reader: a
// terminated oversized record after a valid summary contributes only
// the record-after-summary integrity cause while retaining its
// oversized count and recoverable-path detail; a trailing unterminated
// oversized fragment after a valid summary likewise contributes only
// record after summary while retaining its oversized and malformed
// counts.
func TestIntegrityCausePostSummaryOversized(t *testing.T) {
	mib := 64 * 1024 * 1024

	t.Run("terminated oversized record after summary", func(t *testing.T) {
		rec := oversizedMatchRecoverable("q.go", mib+100)
		idx := readFrom(t, "/work", summaryRecord()+"\n"+rec+"\n")
		assertCauses(t, idx, []causeSpec{causeNoPath(searchindex.IntegrityCauseRecordAfterSummary)})
		if got := idx.OversizedCount(); got != 1 {
			t.Fatalf("OversizedCount = %d, want 1", got)
		}
		if got := idx.MalformedCount(); got != 0 {
			t.Fatalf("MalformedCount = %d, want 0", got)
		}
		diags := idx.OversizedDiagnostics()
		if len(diags) != 1 || diags[0] != "oversized record skipped for q.go" {
			t.Fatalf("OversizedDiagnostics = %q, want [oversized record skipped for q.go]", diags)
		}
	})

	t.Run("trailing oversized fragment after summary", func(t *testing.T) {
		rec := oversizedMatchRecoverable("q.go", mib+100)
		idx := readFrom(t, "/work", summaryRecord()+"\n"+rec)
		assertCauses(t, idx, []causeSpec{causeNoPath(searchindex.IntegrityCauseRecordAfterSummary)})
		if got := idx.OversizedCount(); got != 1 {
			t.Fatalf("OversizedCount = %d, want 1", got)
		}
		if got := idx.MalformedCount(); got != 1 {
			t.Fatalf("MalformedCount = %d, want 1", got)
		}
		diags := idx.OversizedDiagnostics()
		if len(diags) != 1 || diags[0] != "oversized record skipped for q.go" {
			t.Fatalf("OversizedDiagnostics = %q, want [oversized record skipped for q.go]", diags)
		}
	})
}

// TestIntegrityCausesDeterministic verifies that the cause list is
// deterministic across repeated builds of the same stream: missing-end
// causes for still-open files appear in unsigned raw-path order, never
// map iteration order.
func TestIntegrityCausesDeterministic(t *testing.T) {
	records := []string{
		textBegin("b.go"),
		textBegin("a.go"),
		bytesBegin([]byte{0xff, '.', 'g'}),
	}
	var first []searchindex.IntegrityCause
	for i := 0; i < 20; i++ {
		b := searchindex.NewBuilder("/work")
		for _, rec := range records {
			_ = b.Add([]byte(rec))
		}
		got := b.Build().Integrity().Causes
		if first == nil {
			first = got
			continue
		}
		if len(got) != len(first) {
			t.Fatalf("build %d: %d causes, want %d", i, len(got), len(first))
		}
		for j := range got {
			if got[j].Kind != first[j].Kind || string(got[j].Path) != string(first[j].Path) {
				t.Fatalf("build %d cause %d = %v(%q), want %v(%q)", i, j, got[j].Kind, got[j].Path, first[j].Kind, first[j].Path)
			}
		}
	}
}
