package searchindex

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"slices"
)

// CauseKind identifies the stable kind of one stream-integrity
// violation — the user-facing diagnostic each cause produces.
type CauseKind int

const (
	// CauseDuplicateBegin is a begin for a path that is already open.
	CauseDuplicateBegin CauseKind = iota
	// CauseOrphanedMatch is a match for a path that is not open,
	// including a match after the path's binary-excluding end.
	CauseOrphanedMatch
	// CauseOrphanedEnd is an end for a path that is not open.
	CauseOrphanedEnd
	// CauseMissingEnd is a file still open when the stream ends.
	CauseMissingEnd
	// CauseMissingSummary is a stream that ended without a valid
	// summary.
	CauseMissingSummary
	// CauseExtraSummary is a second valid summary record.
	CauseExtraSummary
	// CauseAfterSummary is any other record after the stream's
	// summary — context included.
	CauseAfterSummary
	// CauseUnterminated is a trailing record fragment that never
	// reached its newline, outside the post-summary state.
	CauseUnterminated
)

// IntegrityCause is one structured stream-integrity violation: its
// stable kind and the raw path bytes the offending record names, where
// applicable — nil for causes that name no path, such as a missing or
// extra summary. One offending physical record contributes exactly one
// cause.
type IntegrityCause struct {
	Kind CauseKind
	Path []byte
}

// Integrity reports whether the consumed event stream was complete:
// exactly one summary as the final record and valid paired begin/end
// metadata for every encountered file. It is assessed separately from
// process success and from per-record validity — a schema-malformed
// record is skipped without lifecycle effect unless it arrives after
// summary.
type Integrity struct {
	Complete bool
	// MissingSummary reports that the stream ended without a valid
	// summary — the summary was absent, lost, or malformed — while any
	// other lifecycle violation leaves it clear.
	MissingSummary bool
	// Causes holds one structured cause per offending physical record:
	// mid-stream violations in detection order, then the end-of-stream
	// causes — a missing end per still-open file in unsigned raw-path
	// order, then the missing summary, then a trailing unterminated
	// record that was not already counted as a record after summary.
	Causes []IntegrityCause
}

// Report is the stream-level record accounting a Builder accumulates
// while consuming records. Malformed counts records skipped for
// per-record schema violations; Oversized counts records discarded for
// exceeding the payload limit, with OversizedPaths holding each
// recovered raw path in encounter order — fewer than Oversized when a
// record's type and data.path were not parsed before the limit.
// UnknownTypes counts records skipped for an unrecognized string event
// type. The counts are independent of Integrity: lifecycle violations
// never add to them, except the two composite cases the lifecycle
// matrix marks as both — a trailing unterminated record and a malformed
// record after summary each count malformed while failing integrity.
type Report struct {
	Malformed      int
	Oversized      int
	UnknownTypes   int
	OversizedPaths [][]byte
}

// Builder consumes an rg JSON event stream, layering the lifecycle
// transition matrix on top of the index's per-record schema validation,
// and produces the index plus the stream-integrity result.
//
// Per-path open state is tracked over decoded raw path bytes, so the
// text and bytes encodings of one path are the same file, and multiple
// files may be open simultaneously when rg interleaves events across
// parallel searches. context records carry no lifecycle weight before
// summary. The transition dispositions:
//
//	begin(P) while P is not open   — P opens
//	begin(P) while P is open       — failure (duplicate begin)
//	match(P) while P is open       — indexes under P
//	match(P) while P is not open   — failure; retained with incomplete
//	                                metadata unless P is binary-excluded
//	end(P) while P is open         — P closes; non-null binary_offset
//	                                excludes P
//	end(P) while P is not open     — failure (orphaned/duplicate end)
//	context(P) before summary      — ignored; no lifecycle effect
//	P still open when stream ends  — failure; retained incomplete
//	exactly one summary, final     — complete; a summary alone is a
//	                                valid zero-result stream
//	summary missing                — failure
//	a second summary               — failure
//	any record after summary       — failure
//	trailing unterminated record   — failure, and counted malformed
type Builder struct {
	idx *Index
	// open holds the raw path keys of files between their begin and end.
	open map[string]struct{}
	// sawSummary records that the one valid summary has been consumed;
	// every later record is an integrity failure.
	sawSummary bool
	// causes accumulates one structured integrity cause per offending
	// record in detection order; Finish appends the end-of-stream
	// causes. unterminated marks a trailing unterminated fragment seen
	// outside the post-summary state — its cause is deferred to Finish
	// so it lands after the missing-end and missing-summary causes; a
	// post-summary fragment records its after-summary cause at
	// detection instead.
	causes       []IntegrityCause
	unterminated bool
	// malformed, oversized, and unknown accumulate the Report counts;
	// oversizedPaths holds each recovered oversized-record path.
	malformed      int
	oversized      int
	unknown        int
	oversizedPaths [][]byte
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
// --json output without its newline. Malformed records return an error,
// are counted in the report, and are not indexed; they carry no
// lifecycle weight unless they arrive after summary. Lifecycle
// violations are integrity failures, not record errors: a violating but
// schema-valid record returns nil. An unknown string event type is
// skipped and counted separately.
func (b *Builder) Add(record []byte) error {
	err := b.add(record)
	if errors.Is(err, errMalformed) {
		b.malformed++
	}
	return err
}

// add dispatches one record to its schema validation and lifecycle
// transition. The malformed count belongs to Add so that a record
// malformed after summary still counts; unknown types count here, where
// their known-type status is decided.
func (b *Builder) add(record []byte) error {
	typ, err := eventType(record)
	if err != nil {
		if b.sawSummary {
			b.cause(CauseAfterSummary, nil)
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
	case "context":
		return nil
	default:
		// Unknown string event types are skipped and counted
		// separately; they carry no lifecycle weight.
		b.unknown++
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
		b.cause(CauseDuplicateBegin, path)
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
		b.cause(CauseOrphanedEnd, path)
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
		b.cause(CauseOrphanedMatch, path)
		if _, excluded := b.idx.binary[key]; !excluded {
			b.idx.markIncomplete(path)
		}
	}
	return nil
}

// postSummary handles a record arriving after the stream's summary: the
// summary is final, so any record after it is an integrity failure —
// context included. Each post-summary record contributes exactly one
// cause under the precedence rule: a second valid summary is only an
// extra summary, and every other record is only a record after summary.
// Post-summary records are not dispatched to lifecycle validation — a
// post-summary begin cannot open its path nor an end close it — but
// their per-record schema and index effects still apply, and a
// schema-malformed one still returns its error so the caller can count
// it.
func (b *Builder) postSummary(record []byte, typ string) error {
	if typ == "summary" {
		if err := checkSummary(record); err != nil {
			// A malformed record after summary contributes the
			// after-summary cause plus its malformed count.
			b.cause(CauseAfterSummary, nil)
			return err
		}
		b.cause(CauseExtraSummary, nil)
		return nil
	}
	b.cause(CauseAfterSummary, nil)
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
	case "context":
		return nil
	default:
		// An unknown type after summary keeps its unknown-type count;
		// the ordering violation is the integrity failure above.
		b.unknown++
		return nil
	}
}

// cause records one structured integrity violation: its kind and the
// raw path the offending record names, where applicable.
func (b *Builder) cause(kind CauseKind, path []byte) {
	b.causes = append(b.causes, IntegrityCause{Kind: kind, Path: path})
}

// maxRecordPayload is the largest accepted JSON record payload: 64 MiB,
// excluding the record's newline delimiter. A larger record is consumed
// and discarded through its next newline, counted oversized, and parsing
// resynchronizes on the following record.
const maxRecordPayload = 64 << 20

// Consume drains a byte stream of newline-terminated records, feeding
// each through Add — malformed records are skipped and counted there.
// Records over the payload limit are discarded whole and counted
// oversized; when their type and data.path parsed before the limit the
// recovered raw path is retained for the diagnostic. A trailing
// fragment without its newline is an unterminated record: never
// dispatched, counted malformed, and the stream is incomplete — an
// oversized unterminated tail carries all three dispositions. A record
// consumed after summary is an integrity failure whatever its kind.
func (b *Builder) Consume(r io.Reader) {
	br := bufio.NewReader(r)
	for {
		rec, oversized, terminated, err := readRecord(br)
		switch {
		case oversized:
			b.oversized++
			if path, ok := recoverPath(rec); ok {
				b.oversizedPaths = append(b.oversizedPaths, path)
			}
			if b.sawSummary {
				b.cause(CauseAfterSummary, nil)
			}
			if !terminated {
				b.malformed++
				if !b.sawSummary {
					b.unterminated = true
				}
			}
		case terminated:
			_ = b.Add(rec)
		case len(rec) > 0:
			b.malformed++
			if b.sawSummary {
				b.cause(CauseAfterSummary, nil)
			} else {
				b.unterminated = true
			}
		}
		if err != nil || !terminated {
			return
		}
	}
}

// readRecord consumes one stream record — up to and including its
// newline — reporting the retained payload prefix, whether the payload
// exceeded maxRecordPayload, and whether its terminating newline was
// present. The prefix is capped at the limit: once a record is known
// oversized the rest is discarded without buffering, so memory stays
// bounded while the stream is consumed through the next newline. At end
// of stream the returned record is the unterminated tail fragment,
// empty when the stream ended on a record boundary.
func readRecord(br *bufio.Reader) (rec []byte, oversized, terminated bool, err error) {
	for {
		frag, ferr := br.ReadSlice('\n')
		if !oversized {
			payload := frag
			if ferr == nil {
				payload = payload[:len(payload)-1] // the newline is not payload
			}
			rec = append(rec, payload...)
			if len(rec) > maxRecordPayload {
				oversized = true
				rec = rec[:maxRecordPayload]
			}
		}
		switch ferr {
		case nil:
			return rec, oversized, true, nil
		case bufio.ErrBufferFull:
			continue
		case io.EOF:
			return rec, oversized, false, nil
		default:
			return rec, oversized, false, ferr
		}
	}
}

// recoverPath makes a best-effort pass over an oversized record's
// retained prefix for the type and data.path fields ripgrep emits
// before the line payload: when both decoded before the limit, the raw
// path identifies the affected file in the oversized-record diagnostic.
// A prefix that ran out before either field recovers nothing.
func recoverPath(prefix []byte) ([]byte, bool) {
	dec := json.NewDecoder(bytes.NewReader(prefix))
	tok, err := dec.Token()
	if err != nil || tok != json.Delim('{') {
		return nil, false
	}
	var haveType, havePath bool
	var path []byte
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			break
		}
		key, ok := keyTok.(string)
		if !ok {
			break
		}
		switch key {
		case "type":
			v, err := dec.Token()
			if err != nil {
				return nil, false
			}
			_, haveType = v.(string)
		case "data":
			p, found, complete := recoverDataPath(dec)
			if found {
				path, havePath = p, true
			}
			if !complete {
				return path, haveType && havePath
			}
		default:
			if err := skipValue(dec); err != nil {
				return path, haveType && havePath
			}
		}
	}
	return path, haveType && havePath
}

// recoverDataPath consumes the data object of a truncated record
// prefix, extracting the decoded path blob when it is present and
// whole. complete reports that the data object was consumed to its
// close, so the caller may keep scanning the outer object; a record
// that ran out mid-data leaves the decoder unusable.
func recoverDataPath(dec *json.Decoder) (path []byte, found, complete bool) {
	tok, err := dec.Token()
	if err != nil {
		return nil, false, false
	}
	if tok != json.Delim('{') {
		// A scalar data value is already consumed; the outer scan can
		// continue, but there is no path to find.
		return nil, false, true
	}
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return path, found, false
		}
		key, _ := keyTok.(string)
		if key == "path" {
			p, ok := recoverBlob(dec)
			if !ok {
				return path, found, false
			}
			path, found = p, true
			continue
		}
		if err := skipValue(dec); err != nil {
			return path, found, false
		}
	}
	if _, err := dec.Token(); err != nil {
		return path, found, false
	}
	return path, found, true
}

// recoverBlob consumes one {"text": …} or {"bytes": …} blob object from
// a truncated record prefix, applying the same either/or rule as
// decodeBlob: text wins when both are present.
func recoverBlob(dec *json.Decoder) ([]byte, bool) {
	tok, err := dec.Token()
	if err != nil || tok != json.Delim('{') {
		return nil, false
	}
	var text, b64 *string
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return nil, false
		}
		key, _ := keyTok.(string)
		vt, err := dec.Token()
		if err != nil {
			return nil, false
		}
		s, ok := vt.(string)
		if !ok {
			return nil, false
		}
		switch key {
		case "text":
			text = &s
		case "bytes":
			b64 = &s
		}
	}
	if _, err := dec.Token(); err != nil {
		return nil, false
	}
	switch {
	case text != nil:
		return []byte(*text), true
	case b64 != nil:
		p, err := base64.StdEncoding.DecodeString(*b64)
		if err != nil {
			return nil, false
		}
		return p, true
	default:
		return nil, false
	}
}

// skipValue consumes one complete JSON value from the decoder, whatever
// its shape, so the enclosing object scan resumes at the next key. A
// prefix that ends inside the value reports an error.
func skipValue(dec *json.Decoder) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	d, ok := tok.(json.Delim)
	if !ok {
		return nil
	}
	var depth int
	switch d {
	case '{', '[':
		depth = 1
	default:
		return nil
	}
	for depth > 0 {
		t, err := dec.Token()
		if err != nil {
			return err
		}
		if dd, ok := t.(json.Delim); ok {
			switch dd {
			case '{', '[':
				depth++
			case '}', ']':
				depth--
			}
		}
	}
	return nil
}

// Report returns the stream's record accounting so far: the malformed,
// oversized, and unknown-type skip counts and the recovered oversized
// paths. It is valid during and after consumption.
func (b *Builder) Report() Report {
	return Report{
		Malformed:      b.malformed,
		Oversized:      b.oversized,
		UnknownTypes:   b.unknown,
		OversizedPaths: b.oversizedPaths,
	}
}

// Finish prepares the index and reports stream integrity. Files still
// open when the stream ends are missing their end: their matches are
// retained with incomplete metadata. A stream without a summary is
// incomplete; a summary alone is a complete zero-result stream. The
// end-of-stream causes append after the mid-stream causes in their
// mandated order: a missing end per still-open file in unsigned
// raw-path order, then the missing summary, then a trailing
// unterminated record — unless a valid summary already counted that
// fragment as a record after summary.
func (b *Builder) Finish() (*Index, Integrity) {
	open := make([]string, 0, len(b.open))
	for key := range b.open {
		open = append(open, key)
	}
	// String comparison is unsigned byte order: the missing-end causes
	// order by raw path bytes, never by map iteration order.
	slices.Sort(open)
	for _, key := range open {
		b.cause(CauseMissingEnd, []byte(key))
		b.idx.incomplete[key] = struct{}{}
	}
	if !b.sawSummary {
		b.cause(CauseMissingSummary, nil)
	}
	if b.unterminated {
		b.cause(CauseUnterminated, nil)
	}
	b.idx.Finish()
	return b.idx, Integrity{
		Complete:       len(b.causes) == 0,
		MissingSummary: !b.sawSummary,
		Causes:         b.causes,
	}
}
