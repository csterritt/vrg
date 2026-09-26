package searchindex_test

import (
	"strings"
	"testing"

	"vrg/internal/searchindex"
)

// recordLimit is the PRD's maximum JSON record payload: 64 MiB
// excluding the newline delimiter. The constant is written out here
// rather than imported so the test pins the contract itself.
const recordLimit = 64 << 20

// matchSized returns a match record for big.txt whose payload is
// exactly size bytes: a run of 'x' inside the lines text pads it out
// while the (0,1) submatch stays within the decoded line.
func matchSized(size int) string {
	pre := `{"type":"match","data":{"path":{"text":"big.txt"},"lines":{"text":"`
	suf := `"},"line_number":1,"submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}`
	return pre + strings.Repeat("x", size-len(pre)-len(suf)) + suf
}

// matchLatePath returns a match record whose giant lines member precedes
// data.path. The extra 200 bytes past the limit keep the 64 MiB cut
// inside the lines value, so the path member is never reached for the
// diagnostic.
func matchLatePath(size int) string {
	pre := `{"type":"match","data":{"lines":{"text":"`
	suf := `"},"path":{"text":"late.txt"},"line_number":1,"submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}`
	return pre + strings.Repeat("x", size-len(pre)-len(suf)) + suf
}

// diagSubstrs asserts each wanted substring appears in some diagnostic
// line, in order.
func diagSubstrs(t *testing.T, diags []string, want ...string) {
	t.Helper()
	if len(diags) != len(want) {
		t.Fatalf("RecordDiagnostics = %v, want %d lines", diags, len(want))
	}
	for i, w := range want {
		if !strings.Contains(diags[i], w) {
			t.Fatalf("RecordDiagnostics[%d] = %q, want it to contain %q (all: %v)",
				i, diags[i], w, diags)
		}
	}
}

// A record exactly at the 64 MiB payload limit is accepted and indexes
// normally; one byte over is skipped and counted oversized.
func TestOversizedBoundary(t *testing.T) {
	t.Run("exactly 64 MiB is accepted", func(t *testing.T) {
		ix := searchindex.New("/work")
		ix.Feed([]byte(streamOf(
			beginRec(jText("big.txt")),
			matchSized(recordLimit),
			endRec(jText("big.txt"), "null"),
			`{"type":"summary","data":{}}`,
		)))
		ix.Prepare()

		if n := ix.Oversized(); n != 0 {
			t.Fatalf("Oversized = %d, want 0 — a record at the limit is accepted", n)
		}
		if n := ix.Malformed(); n != 0 {
			t.Fatalf("Malformed = %d, want 0", n)
		}
		if failures := ix.IntegrityFailures(); len(failures) != 0 {
			t.Fatalf("IntegrityFailures = %v, want an intact stream", failures)
		}
		if n := ix.LineCount(); n != 1 {
			t.Fatalf("LineCount = %d, want the boundary record's stop", n)
		}
	})

	t.Run("one byte over is skipped and counted", func(t *testing.T) {
		ix := searchindex.New("/work")
		ix.Feed([]byte(streamOf(
			beginRec(jText("big.txt")),
			matchSized(recordLimit+1),
			endRec(jText("big.txt"), "null"),
			`{"type":"summary","data":{}}`,
		)))
		ix.Prepare()

		if n := ix.Oversized(); n != 1 {
			t.Fatalf("Oversized = %d, want 1", n)
		}
		if n := ix.Malformed(); n != 0 {
			t.Fatalf("Malformed = %d, want 0 — oversized is a separate count", n)
		}
		if failures := ix.IntegrityFailures(); len(failures) != 0 {
			t.Fatalf("IntegrityFailures = %v, want an intact stream", failures)
		}
		if n := ix.LineCount(); n != 0 {
			t.Fatalf("LineCount = %d, want 0 — the oversized match is skipped", n)
		}
	})
}

// An oversized record is consumed and discarded through its next
// newline: parsing resynchronizes on the following record and the rest
// of the stream indexes.
func TestOversizedResynchronizes(t *testing.T) {
	ix := searchindex.New("/work")
	ix.Feed([]byte(streamOf(
		beginRec(jText("a.txt")),
		matchRec(jText("a.txt"), jText("hit\n"), 1, subRec(jText("hit"), 0, 3)),
		matchSized(recordLimit+1),
		matchRec(jText("a.txt"), jText("hit\n"), 4, subRec(jText("hit"), 0, 3)),
		endRec(jText("a.txt"), "null"),
		`{"type":"summary","data":{}}`,
	)))
	ix.Prepare()

	if n := ix.Oversized(); n != 1 {
		t.Fatalf("Oversized = %d, want 1", n)
	}
	stops := ix.Stops()
	if len(stops) != 2 || stops[0].Line != 1 || stops[1].Line != 4 {
		t.Fatalf("Stops = %+v, want the two valid a.txt matches after resync", stops)
	}
	if failures := ix.IntegrityFailures(); len(failures) != 0 {
		t.Fatalf("IntegrityFailures = %v, want an intact stream", failures)
	}
}

// An oversized match whose type and data.path were parsed before the
// limit — the usual rg field order — reports the sanitized path in
// addition to the count.
func TestOversizedDiagnosticNamesRecoveredPath(t *testing.T) {
	ix := searchindex.New("/work")
	ix.Feed([]byte(streamOf(
		beginRec(jText("big.txt")),
		matchSized(recordLimit+1),
		endRec(jText("big.txt"), "null"),
		`{"type":"summary","data":{}}`,
	)))
	ix.Prepare()

	if n := ix.Oversized(); n != 1 {
		t.Fatalf("Oversized = %d, want 1", n)
	}
	diagSubstrs(t, ix.RecordDiagnostics(),
		"oversized record skipped for big.txt",
		"1 oversized record skipped")
}

// When the limit was hit before data.path was parsed, the oversized
// record is counted anonymously: the count line only, no path.
func TestOversizedDiagnosticAnonymousWhenPathLost(t *testing.T) {
	ix := searchindex.New("/work")
	ix.Feed([]byte(streamOf(
		beginRec(jText("a.txt")),
		matchLatePath(recordLimit+200),
		endRec(jText("a.txt"), "null"),
		`{"type":"summary","data":{}}`,
	)))
	ix.Prepare()

	if n := ix.Oversized(); n != 1 {
		t.Fatalf("Oversized = %d, want 1", n)
	}
	diags := ix.RecordDiagnostics()
	diagSubstrs(t, diags, "1 oversized record skipped")
	if strings.Contains(diags[0], "late.txt") {
		t.Fatalf("RecordDiagnostics = %v, want the count only — path was never parsed", diags)
	}
}

// A file whose only match records were oversized is absent from the
// file list; the recovered-path diagnostic is the only indication of
// that loss.
func TestOversizedOnlyFileAbsentFromList(t *testing.T) {
	ix := searchindex.New("/work")
	ix.Feed([]byte(streamOf(
		beginRec(jText("big.txt")),
		matchSized(recordLimit+1),
		matchSized(recordLimit+1),
		endRec(jText("big.txt"), "null"),
		`{"type":"summary","data":{}}`,
	)))
	ix.Prepare()

	if n := ix.Oversized(); n != 2 {
		t.Fatalf("Oversized = %d, want 2", n)
	}
	if n := ix.FileCount(); n != 0 {
		t.Fatalf("FileCount = %d, want 0 — the oversized-only file is absent", n)
	}
	diags := ix.RecordDiagnostics()
	joined := strings.Join(diags, "\n")
	if !strings.Contains(joined, "oversized record skipped for big.txt") {
		t.Fatalf("RecordDiagnostics = %v, want the lost file's path named", diags)
	}
	if !strings.Contains(joined, "2 oversized records skipped") {
		t.Fatalf("RecordDiagnostics = %v, want the oversized count line", diags)
	}
}

// An oversized final record without a trailing newline sitting after
// the summary carries the dual representation: the oversized count,
// the malformed count for its missing termination, and the sole
// after-summary integrity cause — one cause per physical record.
func TestOversizedUnterminatedFinalRecord(t *testing.T) {
	ix := searchindex.New("/work")
	ix.Feed([]byte(streamOf(
		beginRec(jText("a.txt")),
		matchRec(jText("a.txt"), jText("hit\n"), 1, subRec(jText("hit"), 0, 3)),
		endRec(jText("a.txt"), "null"),
		`{"type":"summary","data":{}}`,
	) + matchSized(recordLimit+1))) // no terminating newline
	ix.Prepare()

	if n := ix.Oversized(); n != 1 {
		t.Fatalf("Oversized = %d, want 1", n)
	}
	if n := ix.Malformed(); n != 1 {
		t.Fatalf("Malformed = %d, want 1 — the unterminated record counts malformed", n)
	}
	failures := ix.IntegrityFailures()
	// After the summary the fragment's only integrity cause is record
	// after summary; its malformed and oversized counts are retained
	// independently — the dual representation.
	want := []string{"record after summary"}
	if len(failures) != len(want) {
		t.Fatalf("IntegrityFailures = %v, want %v", failures, want)
	}
	for i, w := range want {
		if !strings.Contains(failures[i], w) {
			t.Fatalf("IntegrityFailures[%d] = %q, want it to contain %q", i, failures[i], w)
		}
	}
	// The path was still recoverable from the consumed prefix.
	diags := ix.RecordDiagnostics()
	joined := strings.Join(diags, "\n")
	for _, w := range []string{
		"oversized record skipped for big.txt",
		"1 malformed record skipped",
		"1 oversized record skipped",
	} {
		if !strings.Contains(joined, w) {
			t.Fatalf("RecordDiagnostics = %v, want it to contain %q", diags, w)
		}
	}
}

// Unknown string event types are skipped and counted separately: never
// malformed, never an integrity failure by themselves, reported as
// "N unrecognised record types skipped", and never substituting for
// required completion events.
func TestUnknownTypeDispositions(t *testing.T) {
	t.Run("counted separately mid-stream", func(t *testing.T) {
		ix := searchindex.New("/work")
		ix.Feed([]byte(streamOf(
			beginRec(jText("a.txt")),
			matchRec(jText("a.txt"), jText("hit\n"), 1, subRec(jText("hit"), 0, 3)),
			`{"type":"weird","data":{"x":1}}`,
			`{"type":"nope"}`,
			endRec(jText("a.txt"), "null"),
			`{"type":"summary","data":{}}`,
		)))
		ix.Prepare()

		if n := ix.Unknown(); n != 2 {
			t.Fatalf("Unknown = %d, want 2", n)
		}
		if n := ix.Malformed(); n != 0 {
			t.Fatalf("Malformed = %d, want 0 — unknown types are not malformed", n)
		}
		if failures := ix.IntegrityFailures(); len(failures) != 0 {
			t.Fatalf("IntegrityFailures = %v, want an intact stream", failures)
		}
		if n := ix.LineCount(); n != 1 {
			t.Fatalf("LineCount = %d, want 1", n)
		}
		diagSubstrs(t, ix.RecordDiagnostics(), "2 unrecognised record types skipped")
	})

	// The unknown-type rule is unqualified by position: an unknown type
	// after the summary is still counted and reported in the
	// unknown-type count, while its position is separately flagged as
	// an after-summary integrity failure.
	t.Run("unknown type after summary", func(t *testing.T) {
		ix := searchindex.New("/work")
		ix.Feed([]byte(streamOf(
			`{"type":"summary","data":{}}`,
			`{"type":"weird","data":{}}`,
		)))
		ix.Prepare()

		if n := ix.Unknown(); n != 1 {
			t.Fatalf("Unknown = %d, want 1 — position must not suppress the count", n)
		}
		failures := ix.IntegrityFailures()
		if len(failures) != 1 || !strings.Contains(failures[0], "record after summary") {
			t.Fatalf("IntegrityFailures = %v, want the after-summary failure", failures)
		}
		diagSubstrs(t, ix.RecordDiagnostics(), "1 unrecognised record types skipped")
	})

	// Unknown types cannot substitute for required completion events:
	// a stream of only unknown records still fails the summary rule.
	t.Run("unknown types never substitute for a summary", func(t *testing.T) {
		ix := searchindex.New("/work")
		ix.Feed([]byte(streamOf(
			`{"type":"weird","data":{}}`,
			`{"type":"stats","data":{}}`,
		)))
		ix.Prepare()

		if n := ix.Unknown(); n != 2 {
			t.Fatalf("Unknown = %d, want 2", n)
		}
		failures := ix.IntegrityFailures()
		if len(failures) != 1 || !strings.Contains(failures[0], "missing summary") {
			t.Fatalf("IntegrityFailures = %v, want the missing-summary failure", failures)
		}
	})
}
