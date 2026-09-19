package searchindex

import (
	"bytes"
	"slices"
)

// Span is a half-open byte range [Start, End) within a matched line.
type Span struct {
	Start, End int
}

// Submatch is one recorded match: its byte range in the decoded line and
// the bytes ripgrep recorded for it.
type Submatch struct {
	Start, End int
	Bytes      []byte
}

// Stop is one navigation stop: a matched source line. Submatches are
// sorted by (Start, End) with overlapping ranges retained; Highlights is
// their union — the prepared highlight coverage.
type Stop struct {
	Number     int64      // the record's line_number
	Bytes      []byte     // decoded data.lines content
	Submatches []Submatch // sorted by (Start, End); overlaps retained
	Highlights []Span     // union of the submatch ranges
}

// File groups one result path's stops. Path holds the raw path bytes with
// relative spellings resolved against the invocation working directory;
// it is never canonicalized.
type File struct {
	Path  []byte
	Stops []Stop // ascending Number
}

// Index accumulates stream records and prepares them into the navigation
// index: files ordered by unsigned raw path bytes, each holding matched
// lines in ascending order. The zero value is ready for use; Prepare must
// be called once feeding is complete.
type Index struct {
	Files []File
	// BinaryExcluded counts the distinct files dropped because a valid
	// end event reported a non-null binary_offset for them.
	BinaryExcluded int
	acc            map[string]*fileAcc
	excluded       map[string]struct{}
}

// fileAcc is the per-path accumulation of stops before preparation.
type fileAcc struct {
	path  []byte
	stops map[int64]*Stop
}

// New returns an empty index.
func New() *Index { return &Index{} }

// Build feeds every newline-delimited record in stream and prepares the
// index against workdir. A trailing unterminated record is still fed;
// its accounting lands with Issue #10.
func Build(stream []byte, workdir string) *Index {
	ix := New()
	for len(stream) > 0 {
		line := stream
		if i := bytes.IndexByte(stream, '\n'); i >= 0 {
			line, stream = stream[:i], stream[i+1:]
		} else {
			stream = nil
		}
		ix.Feed(line)
	}
	ix.Prepare(workdir)
	return ix
}

// Feed parses one JSON record and applies it to the index, returning the
// record's classification. KindMatch contributes stops; a KindEnd with a
// non-null binary_offset drops that file's collected matches and counts
// it in BinaryExcluded. The remaining lifecycle effects are Issue #9's,
// and malformed/unknown counting is Issue #10's.
func (ix *Index) Feed(raw []byte) Kind {
	rec := ParseRecord(raw)
	switch rec.Kind {
	case KindMatch:
		if ix.acc == nil {
			ix.acc = make(map[string]*fileAcc)
		}
		fa := ix.acc[string(rec.Path)]
		if fa == nil {
			fa = &fileAcc{path: rec.Path, stops: make(map[int64]*Stop)}
			ix.acc[string(rec.Path)] = fa
		}
		st := fa.stops[rec.LineNumber]
		if st == nil {
			st = &Stop{Number: rec.LineNumber, Bytes: rec.Line}
			fa.stops[rec.LineNumber] = st
		}
		st.Submatches = append(st.Submatches, rec.Submatches...)
	case KindEnd:
		if rec.Binary {
			ix.exclude(rec.Path)
		}
	}
	return rec.Kind
}

// exclude drops every stop collected for path — its end event's non-null
// binary_offset confirmed the file binary — and counts the file once in
// BinaryExcluded, however many times the stream repeats the end.
func (ix *Index) exclude(path []byte) {
	key := string(path)
	delete(ix.acc, key)
	if ix.excluded == nil {
		ix.excluded = make(map[string]struct{})
	}
	if _, seen := ix.excluded[key]; !seen {
		ix.excluded[key] = struct{}{}
		ix.BinaryExcluded++
	}
}

// UsableResults is the number of retained stops after filtering —
// confirmed binary exclusion now, record skipping with Issue #10 — and
// the single value the outcome logic consumes; it is never the count of
// match events received. It is meaningful once Prepare has run.
func (ix *Index) UsableResults() int {
	n := 0
	for _, f := range ix.Files {
		n += len(f.Stops)
	}
	return n
}

// Prepare resolves relative paths against workdir, orders files by
// unsigned raw path bytes, orders each file's stops by line number, sorts
// each stop's submatches by (Start, End), and computes the union
// highlight coverage.
func (ix *Index) Prepare(workdir string) {
	files := make([]File, 0, len(ix.acc))
	for _, fa := range ix.acc {
		f := File{Path: resolvePath(workdir, fa.path)}
		for _, st := range fa.stops {
			slices.SortFunc(st.Submatches, func(a, b Submatch) int {
				if a.Start != b.Start {
					return a.Start - b.Start
				}
				return a.End - b.End
			})
			st.Highlights = unionSpans(st.Submatches)
			f.Stops = append(f.Stops, *st)
		}
		slices.SortFunc(f.Stops, func(a, b Stop) int {
			switch {
			case a.Number < b.Number:
				return -1
			case a.Number > b.Number:
				return 1
			}
			return 0
		})
		files = append(files, f)
	}
	slices.SortFunc(files, func(a, b File) int {
		return bytes.Compare(a.Path, b.Path)
	})
	ix.Files = files
}

// resolvePath resolves a relative result path against the invocation
// working directory by byte joining; it never canonicalizes, so interior
// dot elements and repeated separators stay literal.
func resolvePath(workdir string, p []byte) []byte {
	if len(p) > 0 && p[0] == '/' {
		return slices.Clone(p)
	}
	out := make([]byte, 0, len(workdir)+1+len(p))
	out = append(out, workdir...)
	if n := len(out); n > 0 && out[n-1] != '/' {
		out = append(out, '/')
	}
	return append(out, p...)
}

// unionSpans merges sorted submatch ranges into coverage spans:
// overlapping or touching ranges collapse into one.
func unionSpans(subs []Submatch) []Span {
	var out []Span
	for _, s := range subs {
		if n := len(out); n > 0 && s.Start <= out[n-1].End {
			if s.End > out[n-1].End {
				out[n-1].End = s.End
			}
			continue
		}
		out = append(out, Span{Start: s.Start, End: s.End})
	}
	return out
}
