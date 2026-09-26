package searchindex

import (
	"bytes"
	"cmp"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"vrg/internal/present"
)

// Range is a byte range within a stop's source line.
type Range struct {
	Start, End int
}

// Stop is one navigation stop: a matched source line in a file. Path
// holds the raw path bytes exactly as rg emitted them — the identity and
// ordering key — while ResolvedPath carries Path resolved against the
// invocation working directory for filesystem access. Submatches are
// sorted by start then end with identical values deduplicated;
// Highlights is the union of their ranges, computed by Prepare.
// Incomplete marks stops whose file's lifecycle metadata is damaged —
// the file's matches arrived outside an open begin or its end never
// arrived — so their binary classification is unconfirmed.
//
// Stop slices and byte fields share the index's storage; callers must
// not mutate them.
type Stop struct {
	Path         []byte
	ResolvedPath []byte
	Line         int64
	Submatches   []Submatch
	Highlights   []Range
	Incomplete   bool
}

// stopKey merges navigation stops: identity is the raw emitted path
// bytes plus the source line number, so aliases that resolve to the same
// file stay distinct.
type stopKey struct {
	path string
	line int64
}

// Index accumulates decoded match records into navigation stops and
// validates the stream's lifecycle: per-path begin/end pairing, exactly
// one summary as the final record, and no unterminated trailing record.
// It is not safe for concurrent use; the collector feeds it from a
// single goroutine.
type Index struct {
	dir    string
	stops  map[stopKey]*stop
	order  []*stop
	sorted bool
	// excluded is the set of raw paths whose file was dropped by a
	// binary end record; a file stays excluded once its end reports a
	// non-null binary_offset.
	excluded map[string]bool
	// open holds the raw paths with an unmatched begin; per-path state
	// is independent so interleaved files pair correctly.
	open map[string]bool
	// incomplete holds raw paths whose lifecycle metadata is damaged —
	// an orphaned match was retained or the end never arrived.
	incomplete map[string]bool
	// failures accumulates integrity diagnostics in stream order.
	failures   []string
	sawSummary bool
	sealed     bool
	// malformed counts records skipped for violating the per-record
	// schema, including the trailing unterminated record. It is kept
	// strictly separate from the integrity failures: lifecycle
	// violations never inflate it, and a skip never becomes a lifecycle
	// violation except in the two composite cases the matrices mark
	// both.
	malformed int
	// oversized counts records discarded for exceeding the 64 MiB
	// payload limit; oversizedPaths holds the sanitized path of each
	// one whose type and data.path were recovered from its prefix.
	oversized      int
	oversizedPaths []string
	// unknown counts records whose string type is outside the five
	// known events — a warning tally, never record loss.
	unknown int
}

// stop is the mutable per-stop accumulation behind Stop.
type stop struct {
	path       []byte
	resolved   []byte
	line       int64
	subs       []Submatch
	highlights []Range
}

// New returns an Index that resolves relative result paths against dir,
// the invocation working directory. Resolution is a plain join — no
// canonicalization — so raw path identity is preserved.
func New(dir string) *Index {
	return &Index{
		dir:        dir,
		stops:      make(map[stopKey]*stop),
		excluded:   make(map[string]bool),
		open:       make(map[string]bool),
		incomplete: make(map[string]bool),
	}
}

// maxRecordPayload is the JSON record size ceiling: 64 MiB excluding
// the newline delimiter. Feed enforces it explicitly on each split
// line — an intentional bound, not a line-reader's incidental limit —
// so a record one byte over is discarded while one exactly at the limit
// decodes normally.
const maxRecordPayload = 64 << 20

// Feed consumes one collected stdout stream: each newline-terminated
// line is decoded and Added, a line that fails the per-record schema is
// skipped and counted malformed, and a line over the payload limit is
// skipped and counted oversized — the split itself is the discard
// through the next newline, so the following record resynchronizes. A
// non-empty trailing chunk without its terminator is an unterminated
// record — counted malformed without a decode attempt and an integrity
// failure, since a cut stream is incomplete — and when it also exceeds
// the payload limit it is counted oversized too: all three
// dispositions hold for that one record.
func (ix *Index) Feed(stream []byte) {
	lines := bytes.Split(stream, []byte("\n"))
	for _, line := range lines[:len(lines)-1] {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		if len(line) > maxRecordPayload {
			ix.skipOversized(line[:maxRecordPayload])
			continue
		}
		rec, err := DecodeRecord(line)
		if err != nil {
			ix.skipMalformed()
			continue
		}
		ix.Add(rec)
	}
	if last := lines[len(lines)-1]; len(bytes.TrimSpace(last)) != 0 {
		if ix.sawSummary {
			ix.fail("record after summary", nil)
		}
		ix.malformed++
		if len(last) > maxRecordPayload {
			ix.countOversized(last[:maxRecordPayload])
		}
		ix.fail("unterminated trailing record", nil)
	}
}

// skipMalformed counts one record position whose bytes failed the
// per-record schema. A skipped record does not by itself make intact
// lifecycle metadata incomplete; only the positional summary-final rule
// still applies to it — a malformed record after the summary is counted
// malformed and flagged as an integrity failure independently.
func (ix *Index) skipMalformed() {
	ix.malformed++
	if ix.sawSummary {
		ix.fail("record after summary", nil)
	}
}

// skipOversized counts one oversized record — a line that exceeded the
// payload limit — applying the same positional summary rule a malformed
// skip does, and naming the file when the record's type and data.path
// were recovered from the consumed prefix.
func (ix *Index) skipOversized(prefix []byte) {
	if ix.sawSummary {
		ix.fail("record after summary", nil)
	}
	ix.countOversized(prefix)
}

// countOversized records one oversized record and its best-effort path
// diagnostic: prefix is the record's first maxRecordPayload bytes, from
// which recoverRecordPath lifts the emitted path when it arrived intact
// before the limit.
func (ix *Index) countOversized(prefix []byte) {
	ix.oversized++
	if path := recoverRecordPath(prefix); path != nil {
		ix.oversizedPaths = append(ix.oversizedPaths, present.Path(path))
	}
}

// fail records one integrity diagnostic; path is escaped for display.
func (ix *Index) fail(format string, path []byte) {
	if path == nil {
		ix.failures = append(ix.failures, format)
		return
	}
	ix.failures = append(ix.failures, fmt.Sprintf(format, present.Path(path)))
}

// Add applies one decoded record to the index and the lifecycle
// tracker. Any record after the summary is a positional violation —
// the summary must be final — and the record's own transition is then
// evaluated as usual. Path identity is the decoded raw path bytes, so
// the text and bytes encodings of one path agree.
func (ix *Index) Add(rec Record) {
	if ix.sawSummary && rec.Kind != KindSummary {
		ix.fail("record after summary", nil)
	}
	switch rec.Kind {
	case KindBegin:
		key := string(rec.Path)
		if ix.open[key] {
			ix.fail("duplicate begin for %s", rec.Path)
		} else {
			ix.open[key] = true
		}
		return
	case KindEnd:
		key := string(rec.Path)
		if !ix.open[key] {
			ix.fail("orphaned end for %s", rec.Path)
		} else {
			delete(ix.open, key)
		}
		// A non-null binary_offset drops the file even when the end
		// itself was orphaned: exclusion is a safety property, not a
		// reward for valid metadata.
		if rec.BinaryOffset != nil {
			ix.exclude(rec.Path)
		}
		return
	case KindSummary:
		// A second summary is the duplicate kind of record after
		// summary; it gets its own diagnostic.
		if ix.sawSummary {
			ix.fail("second summary", nil)
		}
		ix.sawSummary = true
		return
	case KindMatch:
	case KindUnknown:
		// Unknown types carry no lifecycle meaning but are tallied
		// separately and reported; the after-summary check above has
		// already flagged the position independently, so an unknown
		// type there is both counted and an integrity failure.
		ix.unknown++
		return
	default:
		// Context records carry no lifecycle meaning.
		return
	}
	key := string(rec.Path)
	if ix.excluded[key] {
		// Binary exclusion takes precedence over the general
		// orphan-retention rule: a match after a binary-excluding end
		// is not retained and the file stays excluded.
		ix.fail("orphaned match for %s", rec.Path)
		return
	}
	if !ix.open[key] {
		ix.fail("orphaned match for %s", rec.Path)
		ix.incomplete[key] = true
	}
	k := stopKey{path: key, line: rec.LineNumber}
	s := ix.stops[k]
	if s == nil {
		s = &stop{
			path:     bytes.Clone(rec.Path),
			resolved: resolvePath(ix.dir, rec.Path),
			line:     rec.LineNumber,
		}
		ix.stops[k] = s
	}
	s.subs = mergeSubmatches(s.subs, rec.Submatches)
	ix.sorted = false
}

// exclude drops every stop collected for a raw path and marks the file
// binary-excluded. Exclusion is idempotent: the distinct-file tally
// counts each path once.
func (ix *Index) exclude(path []byte) {
	key := string(path)
	if ix.excluded[key] {
		return
	}
	ix.excluded[key] = true
	for k := range ix.stops {
		if k.path == key {
			delete(ix.stops, k)
		}
	}
	ix.sorted = false
}

// Prepare finalizes the index: the stream's lifecycle is sealed (files
// still open report a missing end and a stream without a summary is
// incomplete), stops are sorted by unsigned raw path bytes then
// ascending line number, and each stop's highlight coverage is computed
// as the union of its submatch ranges. It is idempotent and runs again
// only after further Adds, though the seal applies once.
func (ix *Index) Prepare() {
	ix.seal()
	if ix.sorted {
		return
	}
	ix.order = make([]*stop, 0, len(ix.stops))
	for _, s := range ix.stops {
		s.highlights = unionRanges(s.subs)
		ix.order = append(ix.order, s)
	}
	slices.SortFunc(ix.order, func(a, b *stop) int {
		if c := bytes.Compare(a.path, b.path); c != 0 {
			return c
		}
		return cmp.Compare(a.line, b.line)
	})
	ix.sorted = true
}

// seal applies the end-of-stream lifecycle rules once: every file still
// open is missing its end (its matches stay retained, marked
// incomplete) and a stream without a summary is incomplete.
func (ix *Index) seal() {
	if ix.sealed {
		return
	}
	ix.sealed = true
	var dangling []string
	for p := range ix.open {
		dangling = append(dangling, p)
		ix.incomplete[p] = true
	}
	slices.Sort(dangling)
	for _, p := range dangling {
		ix.fail("missing end for %s", []byte(p))
	}
	if !ix.sawSummary {
		ix.fail("missing summary", nil)
	}
}

// IntegrityFailures returns the stream-integrity diagnostics in stream
// order, empty when the lifecycle metadata is complete and consistent.
// Integrity is assessed separately from the child's exit status: a
// process can fail between complete records and a clean exit can carry
// a damaged stream. The slice shares the index's storage and must not
// be mutated.
func (ix *Index) IntegrityFailures() []string {
	ix.seal()
	return ix.failures
}

// Malformed returns the count of records skipped for violating the
// per-record schema — invalid JSON, invalid base64, missing or invalid
// type fields, and known events with missing or mistyped required
// fields or out-of-range values — plus the trailing unterminated
// record, counted malformed for its missing termination. Lifecycle
// violations are integrity failures, not malformed records, and never
// enter this count.
func (ix *Index) Malformed() int {
	return ix.malformed
}

// Oversized returns the count of records discarded for exceeding the
// 64 MiB payload limit — a record-loss tally separate from malformed
// records, though an unterminated oversized final record enters both.
func (ix *Index) Oversized() int {
	return ix.oversized
}

// Unknown returns the count of records whose string type is outside the
// five known events. Unknown types are a warning tally only: they never
// count as malformed, never satisfy a required completion event, and
// never by themselves change the outcome's exit status.
func (ix *Index) Unknown() int {
	return ix.unknown
}

// RecordDiagnostics returns the nonfatal record-skip diagnostics in a
// fixed order: one "oversized record skipped for <path>" line per
// oversized record whose path was recovered, then the malformed,
// oversized, and unrecognised-type tally lines for each nonzero count.
// Unlike IntegrityFailures these describe recoverable record loss;
// whether their presence makes the outcome fatal is the app's outcome
// decision. The slice does not share the index's storage.
func (ix *Index) RecordDiagnostics() []string {
	var diags []string
	for _, p := range ix.oversizedPaths {
		diags = append(diags, "oversized record skipped for "+p)
	}
	if ix.malformed > 0 {
		diags = append(diags, fmt.Sprintf("%d malformed record%s skipped",
			ix.malformed, plural(ix.malformed)))
	}
	if ix.oversized > 0 {
		diags = append(diags, fmt.Sprintf("%d oversized record%s skipped",
			ix.oversized, plural(ix.oversized)))
	}
	if ix.unknown > 0 {
		diags = append(diags, fmt.Sprintf("%d unrecognised record types skipped",
			ix.unknown))
	}
	return diags
}

// plural is the English plural suffix for a count's noun.
func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// Stops returns the navigation stops in index order: unsigned raw path
// bytes, then ascending line number. The slice shares the index's
// storage and must not be mutated.
func (ix *Index) Stops() []Stop {
	ix.Prepare()
	out := make([]Stop, len(ix.order))
	for i, s := range ix.order {
		out[i] = Stop{
			Path:         s.path,
			ResolvedPath: s.resolved,
			Line:         s.line,
			Submatches:   s.subs,
			Highlights:   s.highlights,
			Incomplete:   ix.incomplete[string(s.path)],
		}
	}
	return out
}

// LineCount returns the number of navigation stops — the count of
// matched lines retained after binary exclusion. This is the
// usable-results value the outcome logic consumes.
func (ix *Index) LineCount() int {
	ix.Prepare()
	return len(ix.order)
}

// BinaryExcluded returns the number of distinct files dropped by a
// binary end record — each raw path whose end event carried a non-null
// binary_offset, counted once.
func (ix *Index) BinaryExcluded() int {
	return len(ix.excluded)
}

// FileCount returns the number of distinct raw paths holding at least
// one stop.
func (ix *Index) FileCount() int {
	ix.Prepare()
	n := 0
	var last []byte
	for i, s := range ix.order {
		if i == 0 || !bytes.Equal(s.path, last) {
			n++
		}
		last = s.path
	}
	return n
}

// mergeSubmatches merges a record's submatches into a stop's set: the
// result is sorted by start then end (then bytes for determinism) with
// identical (start, end, bytes) values deduplicated, so the same value
// arriving under either JSON encoding merges once while distinct
// overlapping ranges are all retained.
func mergeSubmatches(dst, src []Submatch) []Submatch {
	out := append(dst, src...)
	slices.SortFunc(out, func(a, b Submatch) int {
		if a.Start != b.Start {
			return a.Start - b.Start
		}
		if c := a.End - b.End; c != 0 {
			return c
		}
		return bytes.Compare(a.Bytes, b.Bytes)
	})
	return slices.CompactFunc(out, func(a, b Submatch) bool {
		return a.Start == b.Start && a.End == b.End && bytes.Equal(a.Bytes, b.Bytes)
	})
}

// unionRanges computes the union of sorted submatch ranges; overlapping
// and adjacent ranges merge into one coverage span.
func unionRanges(subs []Submatch) []Range {
	var out []Range
	for _, s := range subs {
		if n := len(out); n > 0 && s.Start <= out[n-1].End {
			if s.End > out[n-1].End {
				out[n-1].End = s.End
			}
			continue
		}
		out = append(out, Range{Start: s.Start, End: s.End})
	}
	return out
}

// resolvePath resolves a result path against the working directory:
// absolute paths are retained as-is and relative paths join dir with a
// single separator, without canonicalization, so the raw path form
// (including "." and ".." elements) survives.
func resolvePath(dir string, p []byte) []byte {
	if dir == "" || filepath.IsAbs(string(p)) {
		return bytes.Clone(p)
	}
	return []byte(strings.TrimRight(dir, "/") + "/" + string(p))
}
