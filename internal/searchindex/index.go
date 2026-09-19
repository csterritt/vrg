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
	// Incomplete marks a file retained with partial lifecycle metadata:
	// its matches arrived without a pairing begin/end — orphaned, or the
	// file was still open when the stream ended.
	Incomplete bool
}

// Integrity is the stream's lifecycle-validation result, assessed
// separately from the child's process result: a complete stream held
// exactly one summary as its final newline-terminated record, every
// begun file was closed by its end, no record followed the summary, and
// no record was orphaned, duplicated, or left unterminated.
type Integrity struct {
	Complete bool
}

// Index accumulates stream records and prepares them into the navigation
// index: files ordered by unsigned raw path bytes, each holding matched
// lines in ascending order. The zero value is ready for use; Prepare must
// be called once feeding is complete.
type Index struct {
	Files []File
	// BinaryExcluded counts the distinct files dropped because an end
	// record reported a non-null binary_offset for them.
	BinaryExcluded int
	// Malformed counts records skipped as malformed: invalid JSON,
	// invalid base64, a missing or non-string type field, a known event
	// violating the per-record schema matrix, or a trailing
	// unterminated fragment.
	Malformed int
	// Unknown counts records whose string type is outside the five
	// known events — the separate skip count reported as "N
	// unrecognised record types skipped".
	Unknown int
	// Oversized counts records discarded for exceeding the
	// MaxRecordBytes payload limit. OversizedPaths holds the recovered
	// raw path of each oversized record whose type and data.path were
	// parsed before the limit — best-effort, one entry per named
	// record — so diagnostics can name the lost file.
	Oversized      int
	OversizedPaths [][]byte
	// cur is the single global matched-line cursor — the navigation
	// position Next and Prev move. stops is the prepared total stop
	// count the no-op rules consult.
	cur      Cursor
	stops    int
	acc      map[string]*fileAcc
	excluded map[string]struct{}
	// Lifecycle validation state: open holds the decoded raw path bytes
	// of files with an unclosed begin; sawSummary records that the
	// final record arrived; broken accumulates every violation of the
	// Issue #9 transition matrix — a duplicate or excluded-path begin,
	// an orphaned match or end, a record after the summary, or an
	// unterminated tail.
	open       map[string]struct{}
	sawSummary bool
	broken     bool
}

// fileAcc is the per-path accumulation of stops before preparation.
type fileAcc struct {
	path       []byte
	stops      map[int64]*Stop
	incomplete bool
}

// New returns an empty index.
func New() *Index { return &Index{} }

// Build feeds every newline-delimited record in stream and prepares the
// index against workdir. A record longer than MaxRecordBytes is
// consumed and discarded through its next newline — counted oversized,
// never parsed — and parsing resynchronizes on the following record. A
// trailing unterminated fragment goes through FeedTail, not Feed; an
// oversized one takes both dispositions: counted oversized and, for its
// missing termination, counted malformed with the stream incomplete.
func Build(stream []byte, workdir string) *Index {
	ix := New()
	for len(stream) > 0 {
		i := bytes.IndexByte(stream, '\n')
		if i < 0 {
			// The trailing unterminated fragment: malformed and
			// incomplete — and counted oversized too when it also
			// exceeds the payload limit.
			if len(stream) > MaxRecordBytes {
				ix.feedOversized(stream)
			}
			ix.FeedTail(stream)
			break
		}
		rec := stream[:i]
		stream = stream[i+1:]
		if len(rec) > MaxRecordBytes {
			ix.feedOversized(rec)
			continue
		}
		ix.Feed(rec)
	}
	ix.Prepare(workdir)
	return ix
}

// Feed parses one newline-terminated JSON record and applies it to the
// index, returning the record's classification. Every lifecycle
// transition of the Issue #9 matrix applies: begin opens its path,
// match indexes under an open path, end closes it — a non-null
// binary_offset drops the file's collected matches and counts it in
// BinaryExcluded — and summary ends the stream. Records are compared on
// decoded raw path bytes, so a path's text and bytes encodings pair.
//
// Violations never silently destroy retained data: an orphaned match is
// retained with incomplete metadata unless the path is binary-excluded,
// an orphaned or duplicated begin/end simply marks the stream broken,
// and nothing after the summary is dispatched. Malformed and unknown
// records carry no lifecycle; they are counted in Malformed and Unknown
// by the record's own classification, unqualified by position — so a
// malformed or unknown record after the summary still counts while its
// position separately fails integrity, and a safely skipped match
// record leaves otherwise intact lifecycle metadata complete.
func (ix *Index) Feed(raw []byte) Kind {
	rec := ParseRecord(raw)
	switch rec.Kind {
	case KindMalformed:
		ix.Malformed++
	case KindUnknown:
		ix.Unknown++
	}
	if ix.sawSummary {
		// The summary is final: any record after it — except context,
		// which the Issue #9 matrix keeps lifecycle-neutral in every
		// position — is an integrity failure and never dispatched.
		// Issue #36 removes the context exemption.
		if rec.Kind != KindContext {
			ix.broken = true
		}
		return rec.Kind
	}
	key := string(rec.Path)
	switch rec.Kind {
	case KindBegin:
		switch {
		case ix.isExcluded(key):
			// Binary exclusion is terminal: the path cannot reopen.
			ix.broken = true
		case ix.isOpen(key):
			ix.broken = true
		default:
			if ix.open == nil {
				ix.open = make(map[string]struct{})
			}
			ix.open[key] = struct{}{}
		}
	case KindMatch:
		if ix.isExcluded(key) {
			ix.broken = true
			return rec.Kind
		}
		fa := ix.fileAccFor(key, rec.Path)
		if !ix.isOpen(key) {
			ix.broken = true
			fa.incomplete = true
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
		if ix.isOpen(key) {
			delete(ix.open, key)
		} else {
			ix.broken = true
		}
	case KindSummary:
		ix.sawSummary = true
	}
	return rec.Kind
}

// FeedTail consumes the stream's trailing fragment — the bytes after
// the last newline that no newline terminated. Its disposition is
// double: the fragment is counted malformed and the stream is marked
// incomplete.
func (ix *Index) FeedTail(raw []byte) Kind {
	ix.broken = true
	ix.Malformed++
	return KindMalformed
}

// Integrity reports the lifecycle-validation result for the fed stream:
// Complete only when the summary closed the stream as its final record
// and no violation — orphaned, duplicated, post-summary, or
// unterminated — occurred and no begun file was left open. It is
// meaningful once feeding is complete, independently of the child's
// exit status.
func (ix *Index) Integrity() Integrity {
	return Integrity{
		Complete: ix.sawSummary && !ix.broken && len(ix.open) == 0,
	}
}

func (ix *Index) isOpen(key string) bool {
	_, ok := ix.open[key]
	return ok
}

func (ix *Index) isExcluded(key string) bool {
	_, ok := ix.excluded[key]
	return ok
}

// fileAcc returns the accumulation for key, creating it with path's raw
// bytes when the path is first seen.
func (ix *Index) fileAccFor(key string, path []byte) *fileAcc {
	if ix.acc == nil {
		ix.acc = make(map[string]*fileAcc)
	}
	fa := ix.acc[key]
	if fa == nil {
		fa = &fileAcc{path: path, stops: make(map[int64]*Stop)}
		ix.acc[key] = fa
	}
	return fa
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
// highlight coverage. Files whose begin never closed are marked
// incomplete.
func (ix *Index) Prepare(workdir string) {
	for key := range ix.open {
		if fa := ix.acc[key]; fa != nil {
			fa.incomplete = true
		}
	}
	files := make([]File, 0, len(ix.acc))
	for _, fa := range ix.acc {
		f := File{Path: resolvePath(workdir, fa.path), Incomplete: fa.incomplete}
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
	ix.stops = 0
	for _, f := range files {
		ix.stops += len(f.Stops)
	}
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
