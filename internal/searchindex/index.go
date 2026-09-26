package searchindex

import (
	"bytes"
	"cmp"
	"path/filepath"
	"slices"
	"strings"
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
//
// Stop slices and byte fields share the index's storage; callers must
// not mutate them.
type Stop struct {
	Path         []byte
	ResolvedPath []byte
	Line         int64
	Submatches   []Submatch
	Highlights   []Range
}

// stopKey merges navigation stops: identity is the raw emitted path
// bytes plus the source line number, so aliases that resolve to the same
// file stay distinct.
type stopKey struct {
	path string
	line int64
}

// Index accumulates decoded match records into navigation stops. It is
// not safe for concurrent use; the collector feeds it from a single
// goroutine.
type Index struct {
	dir    string
	stops  map[stopKey]*stop
	order  []*stop
	sorted bool
	// excluded is the set of raw paths whose file was dropped by a
	// binary end record; a file stays excluded once its end reports a
	// non-null binary_offset.
	excluded map[string]bool
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
	return &Index{dir: dir, stops: make(map[stopKey]*stop), excluded: make(map[string]bool)}
}

// Add applies one decoded record. Match records build stops unless
// their file is already binary-excluded; an end record with a non-null
// binary_offset drops that file and all its previously collected stops
// and counts it once among the excluded. The remaining lifecycle
// meaning of begin, end, summary, and context records is Issue #9's,
// and record-loss accounting is Issue #10's.
func (ix *Index) Add(rec Record) {
	switch rec.Kind {
	case KindEnd:
		if rec.BinaryOffset != nil {
			ix.exclude(rec.Path)
		}
		return
	case KindMatch:
	default:
		return
	}
	if ix.excluded[string(rec.Path)] {
		return
	}
	k := stopKey{path: string(rec.Path), line: rec.LineNumber}
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

// Prepare finalizes the index: stops are sorted by unsigned raw path
// bytes then ascending line number, and each stop's highlight coverage
// is computed as the union of its submatch ranges. It is idempotent and
// runs again only after further Adds.
func (ix *Index) Prepare() {
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
