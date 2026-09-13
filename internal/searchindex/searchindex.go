// Package searchindex owns parsed ripgrep result data, stream-integrity
// accounting, exclusions, and the circular matched-line cursor.
package searchindex

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// Index is the prepared navigation index of matched lines, ordered by
// unsigned raw path bytes then ascending line number.
type Index struct {
	stops []Stop
	// excludedFiles is the count of distinct files dropped by a
	// non-null binary_offset in their end event.
	excludedFiles int
	// integrity is the stream-integrity assessment, kept separate from
	// process success so the App can assess them independently.
	integrity Integrity
}

// Integrity is the stream-integrity assessment, kept separate from
// process success. A complete stream requires a valid summary and
// valid paired begin/end metadata for encountered files. Orphaned or
// inconsistent lifecycle records, missing end events, missing summary,
// a second summary, any record after summary, or a trailing
// unterminated record make integrity fail.
type Integrity struct {
	// Complete is true when the stream passed all lifecycle validation
	// rules.
	Complete bool
}

// Integrity returns the stream-integrity assessment, kept separate
// from process success.
func (idx *Index) Integrity() Integrity {
	if idx == nil {
		return Integrity{}
	}
	return idx.integrity
}

// Stops returns a copy of the navigation stops ordered by unsigned raw
// path bytes then ascending line number. Modifying the returned slice
// does not affect the index.
func (idx *Index) Stops() []Stop {
	if idx == nil {
		return nil
	}
	stops := make([]Stop, len(idx.stops))
	copy(stops, idx.stops)
	return stops
}

// Len returns the number of navigation stops.
func (idx *Index) Len() int {
	if idx == nil {
		return 0
	}
	return len(idx.stops)
}

// Files returns the number of distinct files in the index.
func (idx *Index) Files() int {
	if idx == nil || len(idx.stops) == 0 {
		return 0
	}
	seen := make(map[string]bool, len(idx.stops))
	for _, s := range idx.stops {
		seen[string(s.RawPath)] = true
	}
	return len(seen)
}

// ExcludedFiles returns the number of distinct files dropped by a
// non-null binary_offset in their end event. These files and all
// their previously collected matches were removed from the index.
func (idx *Index) ExcludedFiles() int {
	if idx == nil {
		return 0
	}
	return idx.excludedFiles
}

// Stop is one navigation stop: one matched source line in one file.
type Stop struct {
	// RawPath is the original path bytes from the result record, decoded
	// from either text or base64 bytes encoding. It is the identity for
	// stop merging and index ordering.
	RawPath []byte
	// Path is the resolved path: relative paths are joined with the
	// working directory without canonicalization; absolute paths are
	// unchanged.
	Path []byte
	// LineNumber is the 1-based source line number.
	LineNumber int
	// Line is the decoded line bytes from the match record, including
	// any line terminator.
	Line []byte
	// Submatches are the match spans on this line, ordered by byte start
	// then end. Overlapping submatches are all retained.
	Submatches []Submatch
	// Coverage is the prepared union of submatch byte ranges, as sorted
	// non-overlapping [start, end) intervals. Overlapping and adjacent
	// ranges are merged.
	Coverage []Range
	// Incomplete is true when the stop's metadata is incomplete because
	// its file was never opened by a begin event, was already closed by
	// an end event, or was still open when the stream ended. The match
	// is retained for browsing but its lifecycle is not intact.
	Incomplete bool
}

// Submatch is one match span within a stop.
type Submatch struct {
	// Match is the recorded match bytes, decoded from either text or
	// base64 bytes encoding.
	Match []byte
	// Start is the byte offset of the match within the line.
	Start int
	// End is the exclusive byte offset of the match within the line.
	End int
}

// Range is a [start, end) byte interval.
type Range struct {
	Start int
	End   int
}

// Builder accumulates parsed ripgrep JSON records into a navigation
// index. It resolves relative result paths against the supplied working
// directory without canonicalization.
type Builder struct {
	workdir string
	stops   map[stopKey]*stopAccum
	// excluded tracks raw paths dropped by a non-null binary_offset in
	// their end event. Matches for these paths are dropped and not
	// re-added.
	excluded map[string]bool
	// excludedCount is the number of distinct excluded files.
	excludedCount int
	// open tracks raw paths whose begin event has been seen without a
	// matching end event. Per-path open state is tracked independently so
	// rg may interleave events across files during parallel search.
	open map[string]bool
	// closed tracks raw paths whose end event has been seen. A match
	// arriving after the file's end is an orphaned match retained with
	// incomplete metadata, unless the end was binary-excluding.
	closed map[string]bool
	// sawSummary is true once a summary record has been seen.
	sawSummary bool
	// afterSummary is true once any record arrives after a summary. The
	// stream is incomplete thereafter.
	afterSummary bool
	// trailingMalformed is set by MarkTrailingMalformed to indicate the
	// scanner saw a trailing unterminated record.
	trailingMalformed bool
	// integrityFailed is true once any lifecycle rule has been violated.
	integrityFailed bool
}

// MarkTrailingMalformed signals that the scanner saw a trailing
// unterminated record. The record is counted as malformed and the
// stream is marked incomplete.
func (b *Builder) MarkTrailingMalformed() {
	b.trailingMalformed = true
}

// stopKey identifies one navigation stop by raw path bytes and line
// number. The path is stored as a string to use as a map key; Go strings
// preserve exact bytes.
type stopKey struct {
	path string
	line int
}

// stopAccum collects submatches for one (raw path, line number) pair.
type stopAccum struct {
	rawPath    []byte
	lineNumber int
	line       []byte
	submatches []Submatch
	// incomplete is true when the stop's metadata is incomplete because
	// its file was never opened by a begin event, was already closed by
	// an end event, or was still open when the stream ended.
	incomplete bool
}

// NewBuilder creates a Builder that resolves relative result paths
// against workdir. Relative paths are joined with workdir using the
// platform separator without canonicalization; absolute paths are
// unchanged.
func NewBuilder(workdir string) *Builder {
	return &Builder{
		workdir:  workdir,
		stops:    make(map[stopKey]*stopAccum),
		excluded: make(map[string]bool),
		open:     make(map[string]bool),
		closed:   make(map[string]bool),
	}
}

// Add parses one JSON record line and indexes it. It recognizes begin,
// match, end, summary, and context events. Context events are ignored
// for lifecycle purposes. Match events are merged into navigation stops
// by raw path and line number. Unknown event types are accepted
// without effect. It returns nil for valid records and a non-nil error
// for malformed records; the caller is responsible for skip and count
// handling. Lifecycle validation is applied per the Issue #9
// transition matrix; violations mark the stream integrity as failed
// but never reject an otherwise well-formed record.
func (b *Builder) Add(line []byte) error {
	var rec struct {
		Type string          `json:"type"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(line, &rec); err != nil {
		return fmt.Errorf("invalid json: %w", err)
	}
	if rec.Type == "" {
		return errors.New("missing or empty type field")
	}
	// Any record after a summary is an integrity failure. Context
	// records participate in no lifecycle validation, so they do not
	// trigger this check on their own; the afterSummary flag is set
	// only by non-context records arriving after the summary.
	if b.sawSummary && rec.Type != "context" {
		b.afterSummary = true
	}
	switch rec.Type {
	case "begin":
		return b.parseBegin(rec.Data)
	case "match":
		return b.parseMatch(rec.Data)
	case "end":
		return b.parseEnd(rec.Data)
	case "summary":
		return b.parseSummary(rec.Data)
	case "context":
		return nil
	default:
		return nil
	}
}

// parseBegin validates a begin record's path field and tracks the
// per-path open state. A begin while the path is already open is an
// integrity failure (duplicate begin); the file remains open.
func (b *Builder) parseBegin(data json.RawMessage) error {
	var d struct {
		Path textBytes `json:"path"`
	}
	if err := json.Unmarshal(data, &d); err != nil {
		return fmt.Errorf("begin: invalid data: %w", err)
	}
	rawPath, err := d.Path.decode()
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	pathKey := string(rawPath)
	if b.open[pathKey] {
		// Duplicate begin: integrity failure. The file remains open.
		b.integrityFailed = true
		return nil
	}
	b.open[pathKey] = true
	// A begin clears any prior closed state for the path so a later
	// end can close it again.
	delete(b.closed, pathKey)
	return nil
}

// parseMatch validates a match record, decodes its fields, and merges it
// into the navigation index. A match while the path is not open (never
// opened, or after its end) is an orphaned match: it is retained with
// incomplete metadata and the stream integrity fails. A match after a
// binary-excluding end is dropped (binary exclusion takes precedence
// over orphan retention).
func (b *Builder) parseMatch(data json.RawMessage) error {
	var d struct {
		Path       textBytes     `json:"path"`
		Lines      textBytes     `json:"lines"`
		LineNumber int           `json:"line_number"`
		Submatches []rawSubmatch `json:"submatches"`
	}
	if err := json.Unmarshal(data, &d); err != nil {
		return fmt.Errorf("match: invalid data: %w", err)
	}
	rawPath, err := d.Path.decode()
	if err != nil {
		return fmt.Errorf("match: path: %w", err)
	}
	pathKey := string(rawPath)
	// Drop matches for files already excluded by a binary end event.
	// Binary exclusion takes precedence over orphan retention.
	if b.excluded[pathKey] {
		// The match is an orphan after a binary end; integrity fails
		// but the match is not retained.
		b.integrityFailed = true
		return nil
	}
	line, err := d.Lines.decode()
	if err != nil {
		return fmt.Errorf("match: lines: %w", err)
	}
	if d.LineNumber < 1 {
		return fmt.Errorf("match: line_number %d < 1", d.LineNumber)
	}
	if len(d.Submatches) == 0 {
		return errors.New("match: submatches is empty")
	}
	subs := make([]Submatch, 0, len(d.Submatches))
	for i, sm := range d.Submatches {
		match, err := sm.Match.decode()
		if err != nil {
			return fmt.Errorf("match: submatch %d: %w", i, err)
		}
		if sm.Start < 0 || sm.End < sm.Start || sm.End > len(line) {
			return fmt.Errorf("match: submatch %d: range [%d, %d) invalid for line length %d", i, sm.Start, sm.End, len(line))
		}
		subs = append(subs, Submatch{Match: match, Start: sm.Start, End: sm.End})
	}
	// An orphaned match (path never opened, or after its end) is
	// retained with incomplete metadata and the stream integrity fails.
	orphaned := !b.open[pathKey]
	if orphaned {
		b.integrityFailed = true
	}
	key := stopKey{path: pathKey, line: d.LineNumber}
	accum, ok := b.stops[key]
	if !ok {
		accum = &stopAccum{
			rawPath:    rawPath,
			lineNumber: d.LineNumber,
			line:       line,
			incomplete: orphaned,
		}
		b.stops[key] = accum
	} else if orphaned {
		accum.incomplete = true
	}
	accum.submatches = append(accum.submatches, subs...)
	return nil
}

// parseEnd validates an end record's path and binary_offset fields and
// tracks the per-path open state. An end while the path is not open is
// an integrity failure (orphaned/duplicate end). A non-null
// binary_offset drops that file and all its previously collected
// matches from the builder and counts it as a distinct excluded file.
func (b *Builder) parseEnd(data json.RawMessage) error {
	var d struct {
		Path         textBytes       `json:"path"`
		BinaryOffset json.RawMessage `json:"binary_offset"`
	}
	if err := json.Unmarshal(data, &d); err != nil {
		return fmt.Errorf("end: invalid data: %w", err)
	}
	rawPath, err := d.Path.decode()
	if err != nil {
		return fmt.Errorf("end: path: %w", err)
	}
	if len(d.BinaryOffset) == 0 {
		return errors.New("end: missing binary_offset")
	}
	pathKey := string(rawPath)
	// An end while the path is not open is an integrity failure.
	if !b.open[pathKey] {
		b.integrityFailed = true
		// Do not add to closed; an orphaned end does not close anything.
		return nil
	}
	// Close the file.
	b.open[pathKey] = false
	b.closed[pathKey] = true
	if string(d.BinaryOffset) != "null" {
		var n int64
		if err := json.Unmarshal(d.BinaryOffset, &n); err != nil {
			return fmt.Errorf("end: binary_offset is not an integer: %w", err)
		}
		if n < 0 {
			return fmt.Errorf("end: binary_offset %d < 0", n)
		}
		// Non-null binary_offset: exclude this file and drop its
		// previously collected matches.
		if !b.excluded[pathKey] {
			b.excluded[pathKey] = true
			b.excludedCount++
		}
		b.dropStops(pathKey)
	}
	return nil
}

// dropStops removes all stops for the given raw path from the builder.
func (b *Builder) dropStops(pathKey string) {
	for key := range b.stops {
		if key.path == pathKey {
			delete(b.stops, key)
		}
	}
}

// parseSummary validates that a summary record has a data object and
// records that a summary has been seen. A second summary is an
// integrity failure.
func (b *Builder) parseSummary(data json.RawMessage) error {
	if len(data) == 0 {
		return errors.New("summary: missing data")
	}
	if !strings.HasPrefix(strings.TrimSpace(string(data)), "{") {
		return errors.New("summary: data is not an object")
	}
	if b.sawSummary {
		// Second summary: integrity failure.
		b.integrityFailed = true
	}
	b.sawSummary = true
	return nil
}

// Build returns the prepared navigation index with stops ordered by
// unsigned raw path bytes then ascending line number. Submatches within
// each stop are sorted by byte start then end, and coverage is computed
// as the union of submatch ranges. Stream integrity is finalized:
// the stream is complete only if a summary was seen, no record arrived
// after the summary, no per-path lifecycle rule was violated, no file
// was still open at stream end, and no trailing unterminated record was
// signaled.
func (b *Builder) Build() *Index {
	// A file still open when the stream ends is an integrity failure;
	// its matches are retained with incomplete metadata.
	for pathKey, isOpen := range b.open {
		if isOpen {
			b.integrityFailed = true
			b.markPathIncomplete(pathKey)
		}
	}
	complete := b.sawSummary && !b.afterSummary && !b.integrityFailed && !b.trailingMalformed
	stops := make([]Stop, 0, len(b.stops))
	for _, accum := range b.stops {
		sort.SliceStable(accum.submatches, func(i, j int) bool {
			if accum.submatches[i].Start != accum.submatches[j].Start {
				return accum.submatches[i].Start < accum.submatches[j].Start
			}
			return accum.submatches[i].End < accum.submatches[j].End
		})
		stops = append(stops, Stop{
			RawPath:    accum.rawPath,
			Path:       resolvePath(b.workdir, accum.rawPath),
			LineNumber: accum.lineNumber,
			Line:       accum.line,
			Submatches: accum.submatches,
			Coverage:   computeCoverage(accum.submatches),
			Incomplete: accum.incomplete,
		})
	}
	sort.SliceStable(stops, func(i, j int) bool {
		c := bytes.Compare(stops[i].RawPath, stops[j].RawPath)
		if c != 0 {
			return c < 0
		}
		return stops[i].LineNumber < stops[j].LineNumber
	})
	return &Index{
		stops:         stops,
		excludedFiles: b.excludedCount,
		integrity:     Integrity{Complete: complete},
	}
}

// markPathIncomplete marks all stops for the given raw path as having
// incomplete metadata (the file was still open when the stream ended).
func (b *Builder) markPathIncomplete(pathKey string) {
	for _, accum := range b.stops {
		if string(accum.rawPath) == pathKey {
			accum.incomplete = true
		}
	}
}

// resolvePath joins a relative raw path with the working directory
// without canonicalization. Absolute paths are returned unchanged.
func resolvePath(workdir string, rawPath []byte) []byte {
	p := string(rawPath)
	if filepath.IsAbs(p) {
		return rawPath
	}
	sep := string(filepath.Separator)
	if workdir == "" {
		return rawPath
	}
	if strings.HasSuffix(workdir, sep) {
		return []byte(workdir + p)
	}
	return []byte(workdir + sep + p)
}

// computeCoverage returns the union of submatch byte ranges as sorted
// non-overlapping [start, end) intervals. Overlapping and adjacent
// ranges are merged. The input must be sorted by start.
func computeCoverage(subs []Submatch) []Range {
	if len(subs) == 0 {
		return nil
	}
	var cov []Range
	cur := Range{Start: subs[0].Start, End: subs[0].End}
	for _, s := range subs[1:] {
		if s.Start <= cur.End {
			if s.End > cur.End {
				cur.End = s.End
			}
		} else {
			cov = append(cov, cur)
			cur = Range{Start: s.Start, End: s.End}
		}
	}
	cov = append(cov, cur)
	return cov
}

// textBytes holds either a text string or base64-encoded bytes from a
// ripgrep JSON record. Exactly one field should be non-nil.
type textBytes struct {
	Text  *string `json:"text"`
	Bytes *string `json:"bytes"`
}

// decode returns the raw bytes represented by the text or bytes field.
func (tb textBytes) decode() ([]byte, error) {
	if tb.Text != nil {
		return []byte(*tb.Text), nil
	}
	if tb.Bytes != nil {
		dec, err := base64.StdEncoding.DecodeString(*tb.Bytes)
		if err != nil {
			return nil, fmt.Errorf("invalid base64: %w", err)
		}
		return dec, nil
	}
	return nil, errors.New("neither text nor bytes present")
}

// rawSubmatch is one submatch element from a match record's submatches
// array before decoding.
type rawSubmatch struct {
	Match textBytes `json:"match"`
	Start int       `json:"start"`
	End   int       `json:"end"`
}
