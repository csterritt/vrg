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
	"strconv"
	"strings"
)

// Range is a half-open byte range [Start, End) into a matched source
// line.
type Range struct {
	Start, End int
}

// Submatch is one recorded ripgrep submatch: its byte range within the
// matched line and the match bytes ripgrep reported for it.
type Submatch struct {
	Range Range
	Text  []byte
}

// Stop is one navigation stop: one matched source line of one file.
type Stop struct {
	// Path is the raw decoded path bytes exactly as ripgrep reported
	// them; it is the result's identity and its display and ordering
	// basis. Resolved is Path resolved against the working directory the
	// search was invoked from, for filesystem access; relative paths are
	// joined without canonicalization.
	Path     []byte
	Resolved []byte
	// Line is the 1-based source line number ripgrep reported.
	Line int64
	// Submatches holds every recorded submatch sorted by start then end;
	// overlapping ranges are all retained. Coverage is their union:
	// sorted, disjoint highlight ranges prepared for display.
	Submatches []Submatch
	Coverage   []Range
	// Incomplete reports that the file's lifecycle metadata is
	// incomplete: its matches were retained from a never-opened or
	// never-closed stream region, so confirmed nonbinary status must not
	// be inferred for it.
	Incomplete bool
}

// Index collects ripgrep's JSON event stream into the ordered navigation
// index. Per-record schema validation happens in Add; cross-record
// lifecycle validation over a whole stream is Builder's concern, and
// skip counting is Issue 10's.
type Index struct {
	workdir string
	byPath  map[string]map[int64]*Stop
	// binary holds the raw path keys of files confirmed binary by a
	// valid end event's non-null binary_offset. Their collected matches
	// are dropped and later records for them are never retained.
	binary map[string]struct{}
	// incomplete holds the raw path keys of files whose lifecycle
	// metadata is incomplete: orphaned matches retained, or still open
	// when the stream ended.
	incomplete map[string]struct{}
	sorted     []Stop
}

// New returns an Index that resolves relative result paths against
// workdir, the directory the search was invoked from.
func New(workdir string) *Index {
	return &Index{
		workdir:    workdir,
		byPath:     make(map[string]map[int64]*Stop),
		binary:     make(map[string]struct{}),
		incomplete: make(map[string]struct{}),
	}
}

// errMalformed marks a record that violates the per-record schema
// contract; Add rejects such records without indexing them.
var errMalformed = errors.New("malformed record")

func malformed(format string, args ...any) error {
	return fmt.Errorf("%w: %s", errMalformed, fmt.Sprintf(format, args...))
}

// Add consumes one rg JSON event record — a single line of --json output
// without its newline — validated against the per-record schema matrix.
// Malformed records return an error and are not indexed. context events
// and unknown string event types are known-but-ignored and return nil.
// Add applies only per-record validation and index effects; Builder owns
// the stream-level lifecycle contract.
func (x *Index) Add(record []byte) error {
	typ, err := eventType(record)
	if err != nil {
		return err
	}
	switch typ {
	case "begin", "end":
		path, binary, err := x.checkLifecycle(record, typ)
		if err != nil {
			return err
		}
		if binary {
			x.excludeBinary(path)
		}
		return nil
	case "match":
		_, err := x.addMatch(record)
		return err
	case "summary":
		return checkSummary(record)
	default:
		return nil
	}
}

// eventType decodes only the record's type field: a record that is not
// an object or lacks a string type is malformed.
func eventType(record []byte) (string, error) {
	var head struct {
		Type *string `json:"type"`
	}
	if err := json.Unmarshal(record, &head); err != nil {
		return "", malformed("record is not a JSON object: %s", err)
	}
	if head.Type == nil {
		return "", malformed("missing type field")
	}
	return *head.Type, nil
}

// data extracts the record's data object as raw JSON for per-event
// decoding.
func data(record []byte) (json.RawMessage, error) {
	var body struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(record, &body); err != nil {
		return nil, err
	}
	return body.Data, nil
}

// decodeBlob decodes an rg data blob: {"text": string} or
// {"bytes": base64}. text wins if both are present, matching ripgrep's
// either/or emission.
func decodeBlob(raw json.RawMessage) ([]byte, error) {
	var b struct {
		Text  *string `json:"text"`
		Bytes *string `json:"bytes"`
	}
	if err := json.Unmarshal(raw, &b); err != nil {
		return nil, err
	}
	if b.Text != nil {
		return []byte(*b.Text), nil
	}
	if b.Bytes != nil {
		return base64.StdEncoding.DecodeString(*b.Bytes)
	}
	return nil, errors.New("neither text nor bytes present")
}

// checkLifecycle validates the shared begin/end contract and decodes the
// event without mutating the index: data.path is required, and end
// additionally requires binary_offset present as null or a non-negative
// integer. It returns the decoded raw path and whether a valid end
// carried a non-null binary_offset — the binary exclusion evidence the
// caller applies.
func (x *Index) checkLifecycle(record []byte, typ string) (path []byte, binary bool, err error) {
	raw, err := data(record)
	if err != nil {
		return nil, false, malformed("%s.data: %s", typ, err)
	}
	var d struct {
		Path         json.RawMessage `json:"path"`
		BinaryOffset json.RawMessage `json:"binary_offset"`
	}
	if err := json.Unmarshal(raw, &d); err != nil {
		return nil, false, malformed("%s.data: %s", typ, err)
	}
	path, err = decodeBlob(d.Path)
	if err != nil {
		return nil, false, malformed("%s.data.path: %s", typ, err)
	}
	if typ == "end" {
		if d.BinaryOffset == nil {
			return nil, false, malformed("end.data.binary_offset missing")
		}
		if b := bytes.TrimSpace(d.BinaryOffset); !bytes.Equal(b, []byte("null")) {
			v, err := decodeInt(b)
			if err != nil || v < 0 {
				return nil, false, malformed("end.data.binary_offset %s is not a non-negative integer", b)
			}
			binary = true
		}
	}
	return path, binary, nil
}

// excludeBinary drops every match collected for path and records the
// file in the distinct exclusion count; already-excluded files are not
// counted again.
func (x *Index) excludeBinary(path []byte) {
	key := string(path)
	if _, seen := x.binary[key]; seen {
		return
	}
	x.binary[key] = struct{}{}
	delete(x.byPath, key)
	delete(x.incomplete, key)
}

// markIncomplete flags a file's lifecycle metadata as incomplete; every
// retained stop for the path carries the flag after Finish.
func (x *Index) markIncomplete(path []byte) {
	x.incomplete[string(path)] = struct{}{}
}

// addMatch validates and indexes one match record, returning the
// decoded raw path so the caller can apply lifecycle rules. A file
// confirmed binary stays excluded: a valid match arriving after its
// binary end is not retained.
func (x *Index) addMatch(record []byte) ([]byte, error) {
	raw, err := data(record)
	if err != nil {
		return nil, malformed("match.data: %s", err)
	}
	var d struct {
		Path  json.RawMessage `json:"path"`
		Lines json.RawMessage `json:"lines"`
		Line  json.RawMessage `json:"line_number"`
		Subs  []struct {
			Match json.RawMessage `json:"match"`
			Start json.RawMessage `json:"start"`
			End   json.RawMessage `json:"end"`
		} `json:"submatches"`
	}
	if err := json.Unmarshal(raw, &d); err != nil {
		return nil, malformed("match.data: %s", err)
	}
	path, err := decodeBlob(d.Path)
	if err != nil {
		return nil, malformed("match.data.path: %s", err)
	}
	lines, err := decodeBlob(d.Lines)
	if err != nil {
		return nil, malformed("match.data.lines: %s", err)
	}
	line, err := decodeInt(d.Line)
	if err != nil || line < 1 {
		return nil, malformed("match.data.line_number %s is not an integer >= 1", d.Line)
	}
	if len(d.Subs) == 0 {
		return nil, malformed("match.data.submatches is missing or empty")
	}
	subs := make([]Submatch, 0, len(d.Subs))
	for i, s := range d.Subs {
		text, err := decodeBlob(s.Match)
		if err != nil {
			return nil, malformed("match.data.submatches[%d].match: %s", i, err)
		}
		start, err1 := decodeInt(s.Start)
		end, err2 := decodeInt(s.End)
		if err1 != nil || err2 != nil || start < 0 || start > end || end > int64(len(lines)) {
			return nil, malformed("match.data.submatches[%d] range %s..%s is outside 0..%d",
				i, s.Start, s.End, len(lines))
		}
		subs = append(subs, Submatch{Range: Range{Start: int(start), End: int(end)}, Text: text})
	}
	if _, excluded := x.binary[string(path)]; excluded {
		return path, nil
	}
	x.merge(path, line, subs)
	return path, nil
}

// decodeInt decodes a raw JSON integer literal into int64. Quoted
// numbers, fractions, and exponents are not integer literals; values must
// fit in int64.
func decodeInt(raw json.RawMessage) (int64, error) {
	s := string(bytes.TrimSpace(raw))
	i := 0
	if i < len(s) && s[0] == '-' {
		i = 1
	}
	if i == len(s) {
		return 0, errors.New("not an integer")
	}
	for ; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, fmt.Errorf("not an integer: %s", s)
		}
	}
	return strconv.ParseInt(s, 10, 64)
}

// checkSummary validates the summary contract: data must be a JSON
// object; its contents are ignored.
func checkSummary(record []byte) error {
	raw, err := data(record)
	if err != nil {
		return malformed("summary.data: %s", err)
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil || obj == nil {
		return malformed("summary.data is not an object")
	}
	return nil
}

// merge folds one match's submatches into the stop for its raw path and
// line, creating the stop on first sight. Submatches are kept sorted by
// start then end across merges.
func (x *Index) merge(path []byte, line int64, subs []Submatch) {
	key := string(path)
	lines := x.byPath[key]
	if lines == nil {
		lines = make(map[int64]*Stop)
		x.byPath[key] = lines
	}
	stop := lines[line]
	if stop == nil {
		stop = &Stop{Path: path, Resolved: resolvePath(x.workdir, path), Line: line}
		lines[line] = stop
	}
	stop.Submatches = append(stop.Submatches, subs...)
	sort.Slice(stop.Submatches, func(i, j int) bool {
		a, b := stop.Submatches[i].Range, stop.Submatches[j].Range
		return a.Start < b.Start || (a.Start == b.Start && a.End < b.End)
	})
}

// Finish prepares the collected records for navigation: stops are ordered
// by unsigned raw path bytes then ascending line number, and each stop's
// highlight coverage is computed as the union of its submatch ranges.
func (x *Index) Finish() {
	for _, lines := range x.byPath {
		for _, stop := range lines {
			x.sorted = append(x.sorted, *stop)
		}
	}
	sort.Slice(x.sorted, func(i, j int) bool {
		if c := bytes.Compare(x.sorted[i].Path, x.sorted[j].Path); c != 0 {
			return c < 0
		}
		return x.sorted[i].Line < x.sorted[j].Line
	})
	for i := range x.sorted {
		x.sorted[i].Coverage = unionRanges(x.sorted[i].Submatches)
		_, x.sorted[i].Incomplete = x.incomplete[string(x.sorted[i].Path)]
	}
}

// unionRanges merges sorted submatches into sorted, disjoint ranges —
// the union used for highlight coverage.
func unionRanges(subs []Submatch) []Range {
	var out []Range
	for _, s := range subs {
		r := s.Range
		if n := len(out); n > 0 && r.Start <= out[n-1].End {
			if r.End > out[n-1].End {
				out[n-1].End = r.End
			}
			continue
		}
		out = append(out, r)
	}
	return out
}

// Stops returns the prepared navigation stops in index order. It is
// valid after Finish and is owned by the index; callers must not mutate
// it.
func (x *Index) Stops() []Stop {
	return x.sorted
}

// UsableResults reports the count of retained matched-line stops after
// binary exclusion — the single usable-results value the outcome logic
// consumes. It is valid after Finish.
func (x *Index) UsableResults() int {
	return len(x.sorted)
}

// BinaryExcluded reports the distinct count of files dropped because a
// valid end event carried a non-null binary_offset.
func (x *Index) BinaryExcluded() int {
	return len(x.binary)
}

// Files returns the distinct raw path bytes in index order, one entry
// per file that has at least one retained stop.
func (x *Index) Files() [][]byte {
	var files [][]byte
	for _, s := range x.sorted {
		if len(files) == 0 || !bytes.Equal(files[len(files)-1], s.Path) {
			files = append(files, s.Path)
		}
	}
	return files
}

// resolvePath resolves a relative result path against the working
// directory the search was invoked from. It preserves the raw bytes —
// no canonicalization, cleaning, or alias merging — so "./a" under "/w"
// resolves to "/w/./a".
func resolvePath(workdir string, raw []byte) []byte {
	if workdir == "" || filepath.IsAbs(string(raw)) {
		return append([]byte(nil), raw...)
	}
	dir := strings.TrimRight(workdir, "/")
	out := make([]byte, 0, len(dir)+1+len(raw))
	out = append(out, dir...)
	out = append(out, '/')
	out = append(out, raw...)
	return out
}
