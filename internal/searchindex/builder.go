package searchindex

import (
	"bufio"
	"io"
)

// Integrity reports whether the consumed event stream was complete:
// exactly one summary as the final record and valid paired begin/end
// metadata for every encountered file. It is assessed separately from
// process success and from per-record validity — a schema-malformed
// record is skipped without lifecycle effect unless it arrives after
// summary.
type Integrity struct {
	Complete bool
}

// Builder consumes an rg JSON event stream, layering the lifecycle
// transition matrix on top of the index's per-record schema validation,
// and produces the index plus the stream-integrity result.
//
// Per-path open state is tracked over decoded raw path bytes, so the
// text and bytes encodings of one path are the same file, and multiple
// files may be open simultaneously when rg interleaves events across
// parallel searches. context records carry no lifecycle weight in any
// position. The transition dispositions:
//
//	begin(P) while P is not open   — P opens
//	begin(P) while P is open       — failure (duplicate begin)
//	match(P) while P is open       — indexes under P
//	match(P) while P is not open   — failure; retained with incomplete
//	                                metadata unless P is binary-excluded
//	end(P) while P is open         — P closes; non-null binary_offset
//	                                excludes P
//	end(P) while P is not open     — failure (orphaned/duplicate end)
//	context(P)                     — ignored; no lifecycle effect
//	P still open when stream ends  — failure; retained incomplete
//	exactly one summary, final     — complete; a summary alone is a
//	                                valid zero-result stream
//	summary missing                — failure
//	a second summary               — failure
//	any non-context record after summary — failure
//	trailing unterminated record   — failure (also malformed; Issue 10
//	                                owns the count)
type Builder struct {
	idx *Index
	// open holds the raw path keys of files between their begin and end.
	open map[string]struct{}
	// sawSummary records that the one valid summary has been consumed;
	// every later non-context record is an integrity failure.
	sawSummary bool
	// failed accumulates any lifecycle transition violation.
	failed bool
}

// NewBuilder returns a Builder that resolves relative result paths
// against workdir, the directory the search was invoked from.
func NewBuilder(workdir string) *Builder {
	return &Builder{
		idx:  New(workdir),
		open: make(map[string]struct{}),
	}
}

// Add consumes one complete rg JSON event record — a single line of
// --json output without its newline. Malformed records return an error
// and are not indexed; they carry no lifecycle weight unless they arrive
// after summary. Lifecycle violations are integrity failures, not record
// errors: a violating but schema-valid record returns nil.
func (b *Builder) Add(record []byte) error {
	typ, err := eventType(record)
	if err != nil {
		if b.sawSummary {
			b.failed = true
		}
		return err
	}
	if b.sawSummary {
		return b.postSummary(record, typ)
	}
	switch typ {
	case "begin":
		return b.addBegin(record)
	case "end":
		return b.addEnd(record)
	case "match":
		return b.addMatch(record)
	case "summary":
		if err := checkSummary(record); err != nil {
			return err
		}
		b.sawSummary = true
		return nil
	default:
		// context and unknown types carry no lifecycle weight.
		return nil
	}
}

// addBegin applies the begin transition: the path opens unless already
// open, where a second begin is a duplicate-begin failure.
func (b *Builder) addBegin(record []byte) error {
	path, _, err := b.idx.checkLifecycle(record, "begin")
	if err != nil {
		return err
	}
	key := string(path)
	if _, open := b.open[key]; open {
		b.failed = true
		return nil
	}
	b.open[key] = struct{}{}
	return nil
}

// addEnd applies the end transition: an end for an open path closes it;
// otherwise the end is orphaned. A non-null binary_offset confirms the
// file binary regardless of pairing, dropping its collected matches.
func (b *Builder) addEnd(record []byte) error {
	path, binary, err := b.idx.checkLifecycle(record, "end")
	if err != nil {
		return err
	}
	key := string(path)
	if _, open := b.open[key]; !open {
		b.failed = true
	} else {
		delete(b.open, key)
	}
	if binary {
		b.idx.excludeBinary(path)
	}
	return nil
}

// addMatch applies the match transition: a match for an open path
// indexes normally; a match for a path that is not open is an orphaned
// match — retained with incomplete metadata, unless the path was
// binary-excluded, in which case exclusion takes precedence over the
// orphan-retention rule and the match is not retained.
func (b *Builder) addMatch(record []byte) error {
	path, err := b.idx.addMatch(record)
	if err != nil {
		return err
	}
	key := string(path)
	if _, open := b.open[key]; !open {
		b.failed = true
		if _, excluded := b.idx.binary[key]; !excluded {
			b.idx.markIncomplete(path)
		}
	}
	return nil
}

// postSummary handles a record arriving after the stream's summary: the
// summary is final, so any record after it is an integrity failure —
// except context, which carries no lifecycle weight in any position.
// Post-summary records are not dispatched to lifecycle validation; a
// schema-malformed one still returns its error so the caller can count
// it.
func (b *Builder) postSummary(record []byte, typ string) error {
	if typ == "context" {
		return nil
	}
	b.failed = true
	switch typ {
	case "match":
		_, err := b.idx.addMatch(record)
		return err
	case "begin", "end":
		path, binary, err := b.idx.checkLifecycle(record, typ)
		if err != nil {
			return err
		}
		if binary {
			b.idx.excludeBinary(path)
		}
		return nil
	case "summary":
		return checkSummary(record)
	default:
		return nil
	}
}

// Consume drains a byte stream of newline-terminated records, feeding
// each through Add — malformed records are skipped (Issue 10 owns the
// count). A trailing fragment without its newline is an unterminated
// record: it is never dispatched and the stream is incomplete.
func (b *Builder) Consume(r io.Reader) {
	br := bufio.NewReader(r)
	for {
		rec, err := br.ReadBytes('\n')
		if n := len(rec); n > 0 {
			if rec[n-1] == '\n' {
				_ = b.Add(rec[:n-1])
			} else {
				b.failed = true
			}
		}
		if err != nil {
			return
		}
	}
}

// Finish prepares the index and reports stream integrity. Files still
// open when the stream ends are missing their end: their matches are
// retained with incomplete metadata. A stream without a summary is
// incomplete; a summary alone is a complete zero-result stream.
func (b *Builder) Finish() (*Index, Integrity) {
	for key := range b.open {
		b.failed = true
		b.idx.incomplete[key] = struct{}{}
	}
	if !b.sawSummary {
		b.failed = true
	}
	b.idx.Finish()
	return b.idx, Integrity{Complete: !b.failed}
}
