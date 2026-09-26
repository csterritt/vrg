package searchindex_test

import (
	"strings"
	"testing"

	"vrg/internal/searchindex"
)

// ctxRec renders a context record, which participates in no lifecycle
// validation before the summary: rg emits it between a file's begin and
// end. Positioned after the summary it is a positional record-after-
// summary violation like any other record.
func ctxRec(path string) string {
	return `{"type":"context","data":{"path":` + path +
		`,"lines":{"text":"ctx\n"},"line_number":2,"submatches":[]}}`
}

// lifecycleCase is one row of the stream-integrity transition matrix.
// records are decoded and Added in order; when stream is set it is
// consumed through Feed instead, exercising record-boundary handling
// (skipped malformed records and the trailing unterminated record).
// failures lists a substring of each expected integrity diagnostic, in
// order — an empty list means the stream is intact. stops and
// incomplete count the retained stops and how many carry the
// incomplete-metadata flag; excluded is the binary-excluded
// distinct-file tally.
type lifecycleCase struct {
	name       string
	records    []string
	stream     string
	failures   []string
	stops      int
	incomplete int
	excluded   int
}

// TestLifecycleMatrix covers every row of the Issue #9 lifecycle
// transition matrix: path identity is the decoded raw path bytes (so
// text and bytes encodings of one path agree), per-path open state is
// tracked independently so interleaved files pair correctly, orphaned
// matches are retained with incomplete metadata, and binary exclusion
// takes precedence over orphan retention. A summary alone is a complete
// zero-result stream; a record after the summary and a trailing
// unterminated record are each integrity failures.
func TestLifecycleMatrix(t *testing.T) {
	cases := []lifecycleCase{
		{
			name: "begin while not open opens the file",
			records: []string{
				beginRec(jText("a.txt")),
				matchRec(jText("a.txt"), jText("hit\n"), 1, subRec(jText("hit"), 0, 3)),
				endRec(jText("a.txt"), "null"),
				`{"type":"summary","data":{}}`,
			},
			stops: 1,
		},
		{
			name: "begin while already open",
			records: []string{
				beginRec(jText("a.txt")),
				beginRec(jText("a.txt")),
				matchRec(jText("a.txt"), jText("hit\n"), 1, subRec(jText("hit"), 0, 3)),
				endRec(jText("a.txt"), "null"),
				`{"type":"summary","data":{}}`,
			},
			failures: []string{"duplicate begin for a.txt"},
			stops:    1,
		},
		{
			name: "match while open indexes normally",
			records: []string{
				beginRec(jText("a.txt")),
				matchRec(jText("a.txt"), jText("hit\n"), 1, subRec(jText("hit"), 0, 3)),
				matchRec(jText("a.txt"), jText("hit\n"), 4, subRec(jText("hit"), 0, 3)),
				endRec(jText("a.txt"), "null"),
				`{"type":"summary","data":{}}`,
			},
			stops: 2,
		},
		{
			name: "match for a file never opened is retained incomplete",
			records: []string{
				matchRec(jText("a.txt"), jText("hit\n"), 1, subRec(jText("hit"), 0, 3)),
				`{"type":"summary","data":{}}`,
			},
			failures:   []string{"orphaned match for a.txt"},
			stops:      1,
			incomplete: 1,
		},
		{
			name: "match after a file's end is retained incomplete",
			records: []string{
				beginRec(jText("a.txt")),
				matchRec(jText("a.txt"), jText("hit\n"), 1, subRec(jText("hit"), 0, 3)),
				endRec(jText("a.txt"), "null"),
				matchRec(jText("a.txt"), jText("hit\n"), 5, subRec(jText("hit"), 0, 3)),
				`{"type":"summary","data":{}}`,
			},
			failures:   []string{"orphaned match for a.txt"},
			stops:      2,
			incomplete: 2,
		},
		{
			// Binary exclusion takes precedence over the general
			// orphan-retention rule: the late match is not retained
			// and the file stays excluded. The binary-excluding end
			// gives the violation its own cause kind — it is not an
			// ordinary orphaned match.
			name: "match after a binary end is not retained",
			records: []string{
				beginRec(jText("a.bin")),
				matchRec(jText("a.bin"), jText("hit\n"), 1, subRec(jText("hit"), 0, 3)),
				endRec(jText("a.bin"), "42"),
				matchRec(jText("a.bin"), jText("hit\n"), 7, subRec(jText("hit"), 0, 3)),
				`{"type":"summary","data":{}}`,
			},
			failures: []string{"match for a.bin arrived after a binary-excluding end"},
			excluded: 1,
		},
		{
			name: "end while open closes the file",
			records: []string{
				beginRec(jText("a.txt")),
				endRec(jText("a.txt"), "null"),
				`{"type":"summary","data":{}}`,
			},
		},
		{
			name: "end for a file never opened",
			records: []string{
				endRec(jText("a.txt"), "null"),
				`{"type":"summary","data":{}}`,
			},
			failures: []string{"orphaned end for a.txt"},
		},
		{
			name: "duplicate end after close",
			records: []string{
				beginRec(jText("a.txt")),
				matchRec(jText("a.txt"), jText("hit\n"), 1, subRec(jText("hit"), 0, 3)),
				endRec(jText("a.txt"), "null"),
				endRec(jText("a.txt"), "null"),
				`{"type":"summary","data":{}}`,
			},
			failures: []string{"orphaned end for a.txt"},
			stops:    1,
		},
		{
			name: "binary end for a file never opened still excludes",
			records: []string{
				endRec(jText("a.bin"), "9"),
				matchRec(jText("a.bin"), jText("hit\n"), 1, subRec(jText("hit"), 0, 3)),
				`{"type":"summary","data":{}}`,
			},
			failures: []string{"orphaned end for a.bin", "match for a.bin arrived after a binary-excluding end"},
			excluded: 1,
		},
		{
			// Issue #9's former "context in any position" row, amended
			// to cover only pre-summary positions: before the summary a
			// context record has no lifecycle effect wherever it sits.
			name: "context before the summary has no lifecycle effect",
			records: []string{
				ctxRec(jText("a.txt")),
				beginRec(jText("a.txt")),
				ctxRec(jText("a.txt")),
				matchRec(jText("a.txt"), jText("hit\n"), 1, subRec(jText("hit"), 0, 3)),
				ctxRec(jText("b.txt")), // a context path is never opened
				endRec(jText("a.txt"), "null"),
				ctxRec(jText("a.txt")),
				`{"type":"summary","data":{}}`,
			},
			stops: 1,
		},
		{
			name: "file still open when the stream ends",
			records: []string{
				beginRec(jText("a.txt")),
				matchRec(jText("a.txt"), jText("hit\n"), 1, subRec(jText("hit"), 0, 3)),
				`{"type":"summary","data":{}}`,
			},
			failures:   []string{"missing end for a.txt"},
			stops:      1,
			incomplete: 1,
		},
		{
			name: "file open and no summary",
			records: []string{
				beginRec(jText("a.txt")),
				matchRec(jText("a.txt"), jText("hit\n"), 1, subRec(jText("hit"), 0, 3)),
			},
			failures:   []string{"missing end for a.txt", "missing summary"},
			stops:      1,
			incomplete: 1,
		},
		{
			name:    "summary alone is a complete zero-result stream",
			records: []string{`{"type":"summary","data":{}}`},
		},
		{
			name: "missing summary",
			records: []string{
				beginRec(jText("a.txt")),
				matchRec(jText("a.txt"), jText("hit\n"), 1, subRec(jText("hit"), 0, 3)),
				endRec(jText("a.txt"), "null"),
			},
			failures: []string{"missing summary"},
			stops:    1,
		},
		{
			// A second summary is its own cause kind: it contributes
			// only extra summary, never record after summary.
			name: "second summary",
			records: []string{
				`{"type":"summary","data":{}}`,
				`{"type":"summary","data":{}}`,
			},
			failures: []string{"extra summary record"},
		},
		{
			name: "match record after summary",
			records: []string{
				beginRec(jText("a.txt")),
				matchRec(jText("a.txt"), jText("hit\n"), 1, subRec(jText("hit"), 0, 3)),
				endRec(jText("a.txt"), "null"),
				`{"type":"summary","data":{}}`,
				matchRec(jText("b.txt"), jText("hit\n"), 2, subRec(jText("hit"), 0, 3)),
			},
			// One cause per physical record: the late match is only a
			// record after summary — it is not lifecycle-processed, so
			// it is neither retained nor marked incomplete.
			failures: []string{"record after summary"},
			stops:    1,
		},
		{
			// The summary-final rule is positional: any record after
			// the summary — even a context record, which otherwise has
			// no lifecycle effect — means the summary was not final.
			name: "context record after summary",
			records: []string{
				`{"type":"summary","data":{}}`,
				ctxRec(jText("a.txt")),
			},
			failures: []string{"record after summary"},
		},
		{
			name: "begin after summary",
			records: []string{
				`{"type":"summary","data":{}}`,
				beginRec(jText("a.txt")),
			},
			// Post-summary records are not lifecycle-processed: the
			// begin cannot open a.txt, so no missing end is owed.
			failures: []string{"record after summary"},
		},
		{
			// A record that fails decoding after the summary is both
			// skipped/counted (Issue #10) and a positional violation.
			name:     "malformed record after summary",
			stream:   `{"type":"summary","data":{}}` + "\n" + `{"type":` + "\n",
			failures: []string{"record after summary"},
		},
		{
			// A trailing unterminated record is counted malformed (the
			// tally is Issue #10's) and makes the stream incomplete;
			// sitting after the summary the after-summary precedence
			// replaces its unterminated cause with record after
			// summary — one integrity cause for the one fragment.
			name: "trailing unterminated record after a complete stream",
			stream: beginRec(jText("a.txt")) + "\n" +
				matchRec(jText("a.txt"), jText("hit\n"), 1, subRec(jText("hit"), 0, 3)) + "\n" +
				endRec(jText("a.txt"), "null") + "\n" +
				`{"type":"summary","data":{}}` + "\n" +
				`{"type":"sum`,
			failures: []string{"record after summary"},
			stops:    1,
		},
		{
			// End-of-stream causes append in their mandated order
			// after every detection-time cause: still-open files by
			// unsigned raw path bytes, then the missing summary, then
			// the unterminated final record.
			name: "trailing unterminated record mid-stream",
			stream: beginRec(jText("a.txt")) + "\n" +
				matchRec(jText("a.txt"), jText("hit\n"), 1, subRec(jText("hit"), 0, 3)) + "\n" +
				`{"type":"end"`,
			failures:   []string{"missing end for a.txt", "missing summary", "unterminated final record"},
			stops:      1,
			incomplete: 1,
		},
		{
			// A safely skipped malformed record does not by itself
			// make otherwise intact lifecycle metadata incomplete.
			name: "malformed record mid-stream stays intact",
			stream: beginRec(jText("a.txt")) + "\n" +
				`{"type":"match","data":{"path":` + "\n" +
				matchRec(jText("a.txt"), jText("hit\n"), 1, subRec(jText("hit"), 0, 3)) + "\n" +
				endRec(jText("a.txt"), "null") + "\n" +
				`{"type":"summary","data":{}}` + "\n",
			stops: 1,
		},
		{
			// Path identity is the decoded raw path bytes: the text
			// and bytes encodings of one path open, match, and close
			// the same file.
			name: "text and bytes path encodings agree",
			records: []string{
				beginRec(jText("a.txt")),
				matchRec(jBytes([]byte("a.txt")), jText("hit\n"), 1, subRec(jText("hit"), 0, 3)),
				endRec(jBytes([]byte("a.txt")), "null"),
				`{"type":"summary","data":{}}`,
			},
			stops: 1,
		},
		{
			// A non-UTF-8 path compares on decoded bytes too; the text
			// form cannot carry it, so every event uses bytes.
			name: "non-UTF-8 path identity",
			records: []string{
				beginRec(jBytes([]byte{'a', 0xff})),
				matchRec(jBytes([]byte{'a', 0xff}), jText("hit\n"), 1, subRec(jText("hit"), 0, 3)),
				endRec(jBytes([]byte{'a', 0xff}), "null"),
				`{"type":"summary","data":{}}`,
			},
			stops: 1,
		},
		{
			name: "path disagreement orphans the later event",
			records: []string{
				beginRec(jText("a.txt")),
				matchRec(jText("a.txt"), jText("hit\n"), 1, subRec(jText("hit"), 0, 3)),
				endRec(jText("b.txt"), "null"), // b.txt was never opened
				endRec(jText("a.txt"), "null"),
				`{"type":"summary","data":{}}`,
			},
			failures: []string{"orphaned end for b.txt"},
			stops:    1,
		},
		{
			// rg may interleave events across files during parallel
			// search: per-path open state tracks each file's pair
			// independently of the other.
			name: "interleaved open files pair independently",
			records: []string{
				beginRec(jText("a.txt")),
				beginRec(jText("b.txt")),
				matchRec(jText("a.txt"), jText("hit\n"), 1, subRec(jText("hit"), 0, 3)),
				matchRec(jText("b.txt"), jText("hit\n"), 2, subRec(jText("hit"), 0, 3)),
				endRec(jText("b.txt"), "null"),
				matchRec(jText("a.txt"), jText("hit\n"), 4, subRec(jText("hit"), 0, 3)),
				endRec(jText("a.txt"), "null"),
				`{"type":"summary","data":{}}`,
			},
			stops: 3,
		},
		{
			name: "interleaved files with a violation on one",
			records: []string{
				beginRec(jText("a.txt")),
				beginRec(jText("b.txt")),
				matchRec(jText("a.txt"), jText("hit\n"), 1, subRec(jText("hit"), 0, 3)),
				endRec(jText("a.txt"), "null"),
				endRec(jText("a.txt"), "null"), // a.txt already closed
				matchRec(jText("b.txt"), jText("hit\n"), 2, subRec(jText("hit"), 0, 3)),
				endRec(jText("b.txt"), "null"),
				`{"type":"summary","data":{}}`,
			},
			failures: []string{"orphaned end for a.txt"},
			stops:    2,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ix := searchindex.New("/work")
			if tc.stream != "" {
				ix.Feed([]byte(tc.stream))
			} else {
				feed(t, ix, tc.records...)
			}
			ix.Prepare()

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
			if n := ix.BinaryExcluded(); n != tc.excluded {
				t.Fatalf("BinaryExcluded = %d, want %d", n, tc.excluded)
			}
		})
	}
}

// Feed consumes the collected stdout stream end to end, decoding each
// newline-terminated record and noting boundary damage, so a complete
// well-formed stream reports intact lifecycle metadata.
func TestFeedDecodesTheWholeStream(t *testing.T) {
	ix := searchindex.New("/work")
	ix.Feed([]byte(beginRec(jText("a.txt")) + "\n" +
		matchRec(jText("a.txt"), jText("hit\n"), 1, subRec(jText("hit"), 0, 3)) + "\n" +
		endRec(jText("a.txt"), "null") + "\n" +
		`{"type":"summary","data":{}}` + "\n"))

	if failures := ix.IntegrityFailures(); len(failures) != 0 {
		t.Fatalf("IntegrityFailures = %v, want an intact stream", failures)
	}
	if n := ix.LineCount(); n != 1 {
		t.Fatalf("LineCount = %d, want 1", n)
	}
}

// An empty stream has no summary and no records: integrity fails.
func TestEmptyStreamIntegrity(t *testing.T) {
	ix := searchindex.New("/work")
	ix.Feed(nil)
	if failures := ix.IntegrityFailures(); len(failures) == 0 {
		t.Fatal("empty stream reported intact, want a missing-summary failure")
	}
}
