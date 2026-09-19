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

// CauseKind identifies the class of one stream-integrity violation
// (Issue #36): which row of the Issue #9 lifecycle matrix the
// offending physical record failed.
type CauseKind int

const (
	// CauseDuplicateBegin is a begin for a path already open, or for a
	// path whose lifecycle already ended in binary exclusion — either
	// way the path cannot be begun again.
	CauseDuplicateBegin CauseKind = iota
	// CauseOrphanedMatch is a match for a path that is not open:
	// never opened, already ended, or terminally binary-excluded.
	CauseOrphanedMatch
	// CauseOrphanedEnd is an end for a path that is not open —
	// orphaned or duplicated.
	CauseOrphanedEnd
	// CauseMissingEnd is a file still open when the stream ends.
	CauseMissingEnd
	// CauseMissingSummary is a stream that ended without its summary.
	CauseMissingSummary
	// CauseExtraSummary is a second summary record.
	CauseExtraSummary
	// CauseAfterSummary is any record other than a second summary
	// arriving after the stream's summary — including context, a
	// malformed or unknown-type record, an oversized record, or the
	// trailing unterminated fragment.
	CauseAfterSummary
	// CauseUnterminated is the trailing unterminated fragment when no
	// valid summary placed it under after-summary precedence.
	CauseUnterminated
)

// Cause is one structured stream-integrity violation: the stable kind
// of failure plus the raw path bytes the offending record named where
// the kind's diagnostic names one (nil otherwise). Each physical
// record contributes at most one cause, chosen by the most-specific
// applicable rule — the precedence Issue #36 pins.
type Cause struct {
	Kind CauseKind
	Path []byte
}

// Integrity is the stream's lifecycle-validation result, assessed
// separately from the child's process result: a complete stream held
// exactly one summary as its final newline-terminated record, every
// begun file was closed by its end, no record followed the summary, and
// no record was orphaned, duplicated, or left unterminated. Causes
// carries one structured record per offending physical record:
// mid-stream violations in detection order, then the end-of-stream
// causes — missing end for each still-open file ordered by unsigned
// raw-path bytes, then missing summary, then the unterminated tail.
// The list is deliberately uncapped: repeated identical violations
// produce one cause each, never aggregation or deduplication.
type Integrity struct {
	Complete bool
	Causes   []Cause
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
	// unterminated tail. causes is the structured form of the same
	// violations in detection order (Issue #36); tail records that the
	// stream ended in an unterminated fragment, whose end-of-stream
	// cause Integrity derives under the after-summary precedence.
	open       map[string]struct{}
	sawSummary bool
	broken     bool
	causes     []Cause
	tail       bool
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
				ix.feedOversized(stream, false)
			}
			ix.FeedTail(stream)
			break
		}
		rec := stream[:i]
		stream = stream[i+1:]
		if len(rec) > MaxRecordBytes {
			ix.feedOversized(rec, true)
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
		// The summary is final: a second summary is the extra-summary
		// failure; every other record — context included, its Issue
		// #9 exemption removed by Issue #36 — is a record after the
		// summary. Neither is dispatched to a lifecycle parser, so a
		// post-summary record can never open a file or orphan one.
		if rec.Kind == KindSummary {
			ix.recordCause(Cause{Kind: CauseExtraSummary})
		} else {
			ix.recordCause(Cause{Kind: CauseAfterSummary})
		}
		return rec.Kind
	}
	key := string(rec.Path)
	switch rec.Kind {
	case KindBegin:
		switch {
		case ix.isExcluded(key):
			// Binary exclusion is terminal: the path cannot reopen.
			ix.recordCause(Cause{Kind: CauseDuplicateBegin, Path: rec.Path})
		case ix.isOpen(key):
			ix.recordCause(Cause{Kind: CauseDuplicateBegin, Path: rec.Path})
		default:
			if ix.open == nil {
				ix.open = make(map[string]struct{})
			}
			ix.open[key] = struct{}{}
		}
	case KindMatch:
		if ix.isExcluded(key) {
			ix.recordCause(Cause{Kind: CauseOrphanedMatch, Path: rec.Path})
			return rec.Kind
		}
		fa := ix.fileAccFor(key, rec.Path)
		if !ix.isOpen(key) {
			ix.recordCause(Cause{Kind: CauseOrphanedMatch, Path: rec.Path})
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
			ix.recordCause(Cause{Kind: CauseOrphanedEnd, Path: rec.Path})
		}
	case KindSummary:
		ix.sawSummary = true
	}
	return rec.Kind
}

// recordCause notes one lifecycle violation: the stream is broken and
// the structured cause joins the detection-order list Integrity
// reports.
func (ix *Index) recordCause(c Cause) {
	ix.broken = true
	ix.causes = append(ix.causes, c)
}

// FeedTail consumes the stream's trailing fragment — the bytes after
// the last newline that no newline terminated. Its disposition is
// double: the fragment is counted malformed and the stream is marked
// incomplete. Its integrity cause is resolved at Integrity time under
// the after-summary precedence: the fragment is a record after the
// summary when a valid summary preceded it, else the unterminated
// final record — never both.
func (ix *Index) FeedTail(raw []byte) Kind {
	ix.broken = true
	ix.tail = true
	ix.Malformed++
	return KindMalformed
}

// Integrity reports the lifecycle-validation result for the fed stream:
// Complete only when the summary closed the stream as its final record
// and no violation — orphaned, duplicated, post-summary, or
// unterminated — occurred and no begun file was left open. It is
// meaningful once feeding is complete, independently of the child's
// exit status. Causes appends the end-of-stream violations after the
// detection-order mid-stream causes: missing end for each still-open
// file ordered by unsigned raw-path bytes, then missing summary, then
// the tail fragment's cause.
func (ix *Index) Integrity() Integrity {
	causes := make([]Cause, 0, len(ix.causes)+len(ix.open)+2)
	for _, c := range ix.causes {
		causes = append(causes, Cause{Kind: c.Kind, Path: slices.Clone(c.Path)})
	}
	if len(ix.open) > 0 {
		paths := make([][]byte, 0, len(ix.open))
		for p := range ix.open {
			paths = append(paths, []byte(p))
		}
		slices.SortFunc(paths, bytes.Compare)
		for _, p := range paths {
			causes = append(causes, Cause{Kind: CauseMissingEnd, Path: p})
		}
	}
	if !ix.sawSummary {
		causes = append(causes, Cause{Kind: CauseMissingSummary})
	}
	if ix.tail {
		kind := CauseUnterminated
		if ix.sawSummary {
			kind = CauseAfterSummary
		}
		causes = append(causes, Cause{Kind: kind})
	}
	return Integrity{
		Complete: ix.sawSummary && !ix.broken && len(ix.open) == 0,
		Causes:   causes,
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
