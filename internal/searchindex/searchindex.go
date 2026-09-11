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
}

// NewBuilder creates a Builder that resolves relative result paths
// against workdir. Relative paths are joined with workdir using the
// platform separator without canonicalization; absolute paths are
// unchanged.
func NewBuilder(workdir string) *Builder {
	return &Builder{
		workdir: workdir,
		stops:   make(map[stopKey]*stopAccum),
	}
}

// Add parses one JSON record line and indexes it. It recognizes begin,
// match, end, summary, and context events. Context events are ignored.
// Match events are merged into navigation stops by raw path and line
// number. Unknown event types are accepted without effect. It returns
// nil for valid records and a non-nil error for malformed records; the
// caller is responsible for skip and count handling.
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

// parseBegin validates a begin record's path field.
func (b *Builder) parseBegin(data json.RawMessage) error {
	var d struct {
		Path textBytes `json:"path"`
	}
	if err := json.Unmarshal(data, &d); err != nil {
		return fmt.Errorf("begin: invalid data: %w", err)
	}
	if _, err := d.Path.decode(); err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	return nil
}

// parseMatch validates a match record, decodes its fields, and merges it
// into the navigation index.
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
	key := stopKey{path: string(rawPath), line: d.LineNumber}
	accum, ok := b.stops[key]
	if !ok {
		accum = &stopAccum{
			rawPath:    rawPath,
			lineNumber: d.LineNumber,
			line:       line,
		}
		b.stops[key] = accum
	}
	accum.submatches = append(accum.submatches, subs...)
	return nil
}

// parseEnd validates an end record's path and binary_offset fields.
func (b *Builder) parseEnd(data json.RawMessage) error {
	var d struct {
		Path         textBytes       `json:"path"`
		BinaryOffset json.RawMessage `json:"binary_offset"`
	}
	if err := json.Unmarshal(data, &d); err != nil {
		return fmt.Errorf("end: invalid data: %w", err)
	}
	if _, err := d.Path.decode(); err != nil {
		return fmt.Errorf("end: path: %w", err)
	}
	if len(d.BinaryOffset) == 0 {
		return errors.New("end: missing binary_offset")
	}
	if string(d.BinaryOffset) != "null" {
		var n int64
		if err := json.Unmarshal(d.BinaryOffset, &n); err != nil {
			return fmt.Errorf("end: binary_offset is not an integer: %w", err)
		}
		if n < 0 {
			return fmt.Errorf("end: binary_offset %d < 0", n)
		}
	}
	return nil
}

// parseSummary validates that a summary record has a data object.
func (b *Builder) parseSummary(data json.RawMessage) error {
	if len(data) == 0 {
		return errors.New("summary: missing data")
	}
	if !strings.HasPrefix(strings.TrimSpace(string(data)), "{") {
		return errors.New("summary: data is not an object")
	}
	return nil
}

// Build returns the prepared navigation index with stops ordered by
// unsigned raw path bytes then ascending line number. Submatches within
// each stop are sorted by byte start then end, and coverage is computed
// as the union of submatch ranges.
func (b *Builder) Build() *Index {
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
		})
	}
	sort.SliceStable(stops, func(i, j int) bool {
		c := bytes.Compare(stops[i].RawPath, stops[j].RawPath)
		if c != 0 {
			return c < 0
		}
		return stops[i].LineNumber < stops[j].LineNumber
	})
	return &Index{stops: stops}
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
