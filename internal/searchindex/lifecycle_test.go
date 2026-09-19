package searchindex_test

import (
	"fmt"
	"strings"
	"testing"

	"vrg/internal/searchindex"
)

// wantFile is the expected retained index entry for one path: its
// resolved path, whether it was retained with incomplete lifecycle
// metadata, and its stop line numbers in index order.
type wantFile struct {
	path       string
	incomplete bool
	stops      []int64
}

// lifecycleCase is one row of the Issue #9 lifecycle transition matrix:
// the records fed in order — each newline-terminated — plus an optional
// trailing unterminated fragment, and the expected stream integrity,
// retained files, and distinct binary-exclusion count.
type lifecycleCase struct {
	name     string
	recs     []string
	tail     string
	complete bool
	files    []wantFile
	binary   int
}

func TestLifecycleMatrix(t *testing.T) {
	matchA := func(line int) string {
		return matchRec(text("a.txt"), text("hit\n"), line, sub(text("hit"), 0, 3))
	}
	matchB := func(line int) string {
		return matchRec(text("b.txt"), text("hit\n"), line, sub(text("hit"), 0, 3))
	}
	for _, tc := range []lifecycleCase{
		{
			name: "begin while not open opens the file",
			recs: []string{
				beginRec(text("a.txt")),
				matchA(7),
				endRec(text("a.txt")),
				summaryRec(),
			},
			complete: true,
			files:    []wantFile{{"/wd/a.txt", false, []int64{7}}},
		},
		{
			name: "duplicate begin while open is an integrity failure",
			recs: []string{
				beginRec(text("a.txt")),
				beginRec(text("a.txt")),
				matchA(7),
				endRec(text("a.txt")),
				summaryRec(),
			},
			complete: false,
			files:    []wantFile{{"/wd/a.txt", false, []int64{7}}},
		},
		{
			name: "match while open indexes under the file",
			recs: []string{
				beginRec(text("a.txt")),
				matchA(3),
				matchA(9),
				endRec(text("a.txt")),
				summaryRec(),
			},
			complete: true,
			files:    []wantFile{{"/wd/a.txt", false, []int64{3, 9}}},
		},
		{
			name: "match for never-opened path is retained incomplete",
			recs: []string{
				matchA(7),
				summaryRec(),
			},
			complete: false,
			files:    []wantFile{{"/wd/a.txt", true, []int64{7}}},
		},
		{
			name: "match after end is retained incomplete",
			recs: []string{
				beginRec(text("a.txt")),
				matchA(7),
				endRec(text("a.txt")),
				matchA(12),
				summaryRec(),
			},
			complete: false,
			files:    []wantFile{{"/wd/a.txt", true, []int64{7, 12}}},
		},
		{
			// Binary exclusion takes precedence over orphan retention:
			// the late match is dropped and the file stays excluded.
			name: "match after binary end is not retained",
			recs: []string{
				beginRec(text("a.txt")),
				matchA(7),
				endBinaryRec(text("a.txt"), 5),
				matchA(12),
				summaryRec(),
			},
			complete: false,
			files:    nil,
			binary:   1,
		},
		{
			name: "end while open closes the file",
			recs: []string{
				beginRec(text("a.txt")),
				endRec(text("a.txt")),
				summaryRec(),
			},
			complete: true,
		},
		{
			// A well-formed end carrying binary_offset is the exclusion
			// transition — exclusion is normal, not a failure.
			name: "binary end while open excludes without failing",
			recs: []string{
				beginRec(text("a.txt")),
				matchA(7),
				endBinaryRec(text("a.txt"), 5),
				summaryRec(),
			},
			complete: true,
			binary:   1,
		},
		{
			name: "end for never-opened path is orphaned",
			recs: []string{
				endRec(text("a.txt")),
				summaryRec(),
			},
			complete: false,
		},
		{
			name: "duplicate end is orphaned",
			recs: []string{
				beginRec(text("a.txt")),
				endRec(text("a.txt")),
				endRec(text("a.txt")),
				summaryRec(),
			},
			complete: false,
		},
		{
			// A well-formed binary_offset is exclusion evidence even on
			// an orphaned end: the retained orphan matches drop with
			// the file and the exclusion counts.
			name: "orphaned binary end excludes and fails",
			recs: []string{
				matchA(7),
				endBinaryRec(text("a.txt"), 5),
				summaryRec(),
			},
			complete: false,
			binary:   1,
		},
		{
			name: "context before summary is ignored",
			recs: []string{
				beginRec(text("a.txt")),
				contextRec(),
				matchA(7),
				contextRec(),
				endRec(text("a.txt")),
				summaryRec(),
			},
			complete: true,
			files:    []wantFile{{"/wd/a.txt", false, []int64{7}}},
		},
		{
			// Issue #9's matrix keeps context lifecycle-neutral in
			// every position; Issue #36 removes this exemption and
			// Issue #44 fixes the post-summary row to fail.
			name: "context after summary has no lifecycle effect",
			recs: []string{
				beginRec(text("a.txt")),
				matchA(7),
				endRec(text("a.txt")),
				summaryRec(),
				contextRec(),
			},
			complete: true,
			files:    []wantFile{{"/wd/a.txt", false, []int64{7}}},
		},
		{
			name: "file still open at stream end fails and retains",
			recs: []string{
				beginRec(text("a.txt")),
				matchA(7),
				summaryRec(),
			},
			complete: false,
			files:    []wantFile{{"/wd/a.txt", true, []int64{7}}},
		},
		{
			name: "summary alone is a complete zero-result stream",
			recs: []string{
				summaryRec(),
			},
			complete: true,
		},
		{
			// The file's own begin/end pair is consistent; the missing
			// summary fails the stream, not the file's metadata.
			name: "missing summary fails the stream",
			recs: []string{
				beginRec(text("a.txt")),
				matchA(7),
				endRec(text("a.txt")),
			},
			complete: false,
			files:    []wantFile{{"/wd/a.txt", false, []int64{7}}},
		},
		{
			name: "second summary is an integrity failure",
			recs: []string{
				summaryRec(),
				summaryRec(),
			},
			complete: false,
		},
		{
			name: "match after summary is an integrity failure",
			recs: []string{
				beginRec(text("a.txt")),
				matchA(7),
				endRec(text("a.txt")),
				summaryRec(),
				matchB(3),
			},
			complete: false,
			// The post-summary match must not be indexed.
			files: []wantFile{{"/wd/a.txt", false, []int64{7}}},
		},
		{
			name: "begin after summary is an integrity failure",
			recs: []string{
				summaryRec(),
				beginRec(text("a.txt")),
			},
			complete: false,
		},
		{
			name: "end after summary is an integrity failure",
			recs: []string{
				summaryRec(),
				endRec(text("a.txt")),
			},
			complete: false,
		},
		{
			name: "malformed record after summary is an integrity failure",
			recs: []string{
				summaryRec(),
				`{"type":"match"`,
			},
			complete: false,
		},
		{
			name: "unknown record after summary is an integrity failure",
			recs: []string{
				summaryRec(),
				`{"type":"frobnicate","data":{"x":1}}`,
			},
			complete: false,
		},
		{
			name: "trailing unterminated record fails the stream",
			recs: []string{
				beginRec(text("a.txt")),
				matchA(7),
				endRec(text("a.txt")),
				summaryRec(),
			},
			tail:     matchB(3),
			complete: false,
			files:    []wantFile{{"/wd/a.txt", false, []int64{7}}},
		},
		{
			name: "unterminated fragment as the only summary fails",
			recs: []string{
				beginRec(text("a.txt")),
				matchA(7),
				endRec(text("a.txt")),
			},
			tail:     summaryRec(),
			complete: false,
			files:    []wantFile{{"/wd/a.txt", false, []int64{7}}},
		},
		{
			name: "text and bytes forms of one path agree",
			recs: []string{
				beginRec(text("a.txt")),
				matchRec(byts("a.txt"), byts("hit\n"), 7, sub(byts("hit"), 0, 3)),
				endRec(byts("a.txt")),
				summaryRec(),
			},
			complete: true,
			files:    []wantFile{{"/wd/a.txt", false, []int64{7}}},
		},
		{
			name: "bytes begin pairs with text end",
			recs: []string{
				beginRec(byts("a.txt")),
				matchA(7),
				endRec(text("a.txt")),
				summaryRec(),
			},
			complete: true,
			files:    []wantFile{{"/wd/a.txt", false, []int64{7}}},
		},
		{
			name: "interleaved open files pair independently",
			recs: []string{
				beginRec(text("a.txt")),
				beginRec(text("b.txt")),
				matchA(7),
				matchB(3),
				endRec(text("a.txt")),
				endRec(text("b.txt")),
				summaryRec(),
			},
			complete: true,
			files: []wantFile{
				{"/wd/a.txt", false, []int64{7}},
				{"/wd/b.txt", false, []int64{3}},
			},
		},
		{
			// Retained orphan matches do not open the file: a later end
			// is still orphaned — two failures, one retained file.
			name: "orphaned match does not open the file",
			recs: []string{
				matchA(7),
				endRec(text("a.txt")),
				summaryRec(),
			},
			complete: false,
			files:    []wantFile{{"/wd/a.txt", true, []int64{7}}},
		},
		{
			name: "orphan match then begin keeps retained stops",
			recs: []string{
				matchA(2),
				beginRec(text("a.txt")),
				matchA(7),
				endRec(text("a.txt")),
				summaryRec(),
			},
			complete: false,
			files:    []wantFile{{"/wd/a.txt", true, []int64{2, 7}}},
		},
		{
			name: "begin cannot reopen an excluded file",
			recs: []string{
				beginRec(text("a.txt")),
				matchA(7),
				endBinaryRec(text("a.txt"), 5),
				beginRec(text("a.txt")),
				matchA(12),
				summaryRec(),
			},
			complete: false,
			binary:   1,
		},
		{
			name: "duplicate begin on excluded file stays excluded",
			recs: []string{
				beginRec(text("a.txt")),
				endBinaryRec(text("a.txt"), 5),
				beginRec(text("a.txt")),
				endRec(text("a.txt")),
				summaryRec(),
			},
			complete: false,
			binary:   1,
		},
		{
			name:     "empty stream fails",
			recs:     nil,
			complete: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ix := buildLifecycle(tc.recs, tc.tail)
			if got := ix.Integrity().Complete; got != tc.complete {
				t.Errorf("Integrity().Complete = %v, want %v", got, tc.complete)
			}
			checkLifecycleFiles(t, ix, tc.files)
			if ix.BinaryExcluded != tc.binary {
				t.Errorf("BinaryExcluded = %d, want %d", ix.BinaryExcluded, tc.binary)
			}
			usable := 0
			for _, wf := range tc.files {
				usable += len(wf.stops)
			}
			if got := ix.UsableResults(); got != usable {
				t.Errorf("UsableResults() = %d, want %d", got, usable)
			}
		})
	}
}

// buildLifecycle feeds each record newline-terminated, then the
// unterminated tail fragment, through the same stream entry point the
// collector uses.
func buildLifecycle(recs []string, tail string) *searchindex.Index {
	var sb strings.Builder
	for _, r := range recs {
		sb.WriteString(r)
		sb.WriteByte('\n')
	}
	sb.WriteString(tail)
	return searchindex.Build([]byte(sb.String()), "/wd")
}

func checkLifecycleFiles(t *testing.T, ix *searchindex.Index, want []wantFile) {
	t.Helper()
	if len(ix.Files) != len(want) {
		t.Fatalf("Files = %#v, want %d entries", ix.Files, len(want))
	}
	for i, wf := range want {
		f := ix.Files[i]
		if string(f.Path) != wf.path {
			t.Errorf("Files[%d].Path = %q, want %q", i, f.Path, wf.path)
		}
		if f.Incomplete != wf.incomplete {
			t.Errorf("Files[%d] %q: Incomplete = %v, want %v",
				i, f.Path, f.Incomplete, wf.incomplete)
		}
		got := make([]int64, len(f.Stops))
		for j, st := range f.Stops {
			got[j] = st.Number
		}
		if fmt.Sprint(got) != fmt.Sprint(wf.stops) {
			t.Errorf("Files[%d] %q: stop lines = %v, want %v",
				i, f.Path, got, wf.stops)
		}
	}
}

// A well-formed but unterminated trailing record carries the matrix's
// double disposition: classified malformed (Issue #10 owns the count)
// and marked as an incomplete stream.
func TestTrailingUnterminatedRecordDisposition(t *testing.T) {
	ix := searchindex.New()
	ix.Feed([]byte(beginRec(text("a.txt"))))
	ix.Feed([]byte(matchRec(text("a.txt"), text("hit\n"), 7, sub(text("hit"), 0, 3))))
	ix.Feed([]byte(endRec(text("a.txt"))))
	ix.Feed([]byte(summaryRec()))

	if got := ix.FeedTail([]byte(matchRec(text("b.txt"), text("hit\n"), 9, sub(text("hit"), 0, 3)))); got != searchindex.KindMalformed {
		t.Fatalf("FeedTail = %v, want KindMalformed", got)
	}
	ix.Prepare("/wd")
	if ix.Integrity().Complete {
		t.Fatal("unterminated trailing record must mark the stream incomplete")
	}
	// Skipped like any malformed record: its would-be match is absent.
	if len(ix.Files) != 1 || len(ix.Files[0].Stops) != 1 {
		t.Fatalf("trailing fragment must not be indexed; Files = %#v", ix.Files)
	}
}
