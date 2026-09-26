package searchindex

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Kind identifies which event type a decoded record carries. The five
// known ripgrep JSON events decode to their own kinds; a string type
// outside them is a valid decode with KindUnknown, not a malformed
// record.
type Kind int

const (
	// KindBegin is a file begin event; Path is set.
	KindBegin Kind = iota
	// KindMatch is a match event; Path, LineNumber, Line, and Submatches
	// are set.
	KindMatch
	// KindEnd is a file end event; Path and BinaryOffset are set.
	KindEnd
	// KindSummary is the stream summary event; only the type is read.
	KindSummary
	// KindContext is a context event; it is recognized but entirely
	// ignored because displayed content comes from disk.
	KindContext
	// KindUnknown is a record whose string type is not one of the known
	// events. Unknown types are counted separately from malformed
	// records (Issue #10) and never substitute for required events.
	KindUnknown
)

// ErrMalformed marks a record that violates the per-record schema: the
// envelope is not a JSON object with a string type, a required field is
// missing or mistyped, a base64 value does not decode, or a numeric
// field falls outside its range.
var ErrMalformed = errors.New("malformed record")

// Record is one decoded ripgrep JSON stream event. Only the fields the
// schema matrix requires are read; every other field is ignored.
type Record struct {
	Kind Kind
	// Path is the decoded raw path bytes for begin, match, and end
	// records.
	Path []byte
	// LineNumber is the 1-based matched line number for match records.
	LineNumber int64
	// Line is the decoded data.lines bytes for match records, including
	// any line terminator.
	Line []byte
	// Submatches are the validated submatches of a match record, in
	// emitted order; index merging sorts them.
	Submatches []Submatch
	// BinaryOffset carries an end record's binary_offset: nil for JSON
	// null, otherwise the validated nonnegative offset.
	BinaryOffset *int64
}

// Submatch is one match span within a record's line bytes. Start and End
// are byte offsets satisfying 0 <= Start <= End <= len(Line); Bytes is
// the recorded match bytes, retained for later stale-content validation.
type Submatch struct {
	Start, End int
	Bytes      []byte
}

// envelope is the two-member shape every rg JSON record shares.
type envelope struct {
	Type json.RawMessage `json:"type"`
	Data json.RawMessage `json:"data"`
}

// DecodeRecord validates one raw JSON record (the newline delimiter may
// be present or absent) and returns its typed form. Records that fail
// the per-record schema return an error wrapping ErrMalformed; skipping
// and counting them is Issue #10's, and cross-record lifecycle rules are
// Issue #9's.
func DecodeRecord(raw []byte) (Record, error) {
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return Record{}, fmt.Errorf("%w: %v", ErrMalformed, err)
	}
	// The type field must be present and a JSON string; unmarshal of a
	// JSON null into string does not error, so check the literal form.
	if len(env.Type) == 0 || env.Type[0] != '"' {
		return Record{}, fmt.Errorf("%w: missing or non-string type", ErrMalformed)
	}
	var typ string
	if err := json.Unmarshal(env.Type, &typ); err != nil {
		return Record{}, fmt.Errorf("%w: invalid type", ErrMalformed)
	}
	switch typ {
	case "begin":
		data, err := decodeData(env.Data)
		if err != nil {
			return Record{}, err
		}
		path, err := decodeValue(data["path"])
		if err != nil {
			return Record{}, fmt.Errorf("%w: begin path", ErrMalformed)
		}
		return Record{Kind: KindBegin, Path: path}, nil
	case "match":
		return decodeMatch(env.Data)
	case "end":
		return decodeEnd(env.Data)
	case "summary":
		// data is required to be an object; its contents are ignored.
		if _, err := decodeData(env.Data); err != nil {
			return Record{}, fmt.Errorf("%w: summary data", ErrMalformed)
		}
		return Record{Kind: KindSummary}, nil
	case "context":
		return Record{Kind: KindContext}, nil
	default:
		return Record{Kind: KindUnknown}, nil
	}
}

// decodeData decodes a record's data member, which must be a JSON object
// for every event that reads it.
func decodeData(raw json.RawMessage) (map[string]json.RawMessage, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("%w: missing data", ErrMalformed)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil || m == nil {
		return nil, fmt.Errorf("%w: data is not an object", ErrMalformed)
	}
	return m, nil
}

// decodeValue decodes the rg text/bytes union: {"text": string} or
// {"bytes": base64}. Both forms yield the same bytes for the same value.
func decodeValue(raw json.RawMessage) ([]byte, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("%w: missing value", ErrMalformed)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil || m == nil {
		return nil, fmt.Errorf("%w: value is not an object", ErrMalformed)
	}
	if t, ok := m["text"]; ok {
		var s string
		if err := json.Unmarshal(t, &s); err != nil {
			return nil, fmt.Errorf("%w: text is not a string", ErrMalformed)
		}
		return []byte(s), nil
	}
	if b, ok := m["bytes"]; ok {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return nil, fmt.Errorf("%w: bytes is not a string", ErrMalformed)
		}
		out, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid base64", ErrMalformed)
		}
		return out, nil
	}
	return nil, fmt.Errorf("%w: value has neither text nor bytes", ErrMalformed)
}

// decodeInt decodes a required integer field: a JSON literal in strict
// integer spelling that fits in int64. Strings, floats, and exponents
// are malformed.
func decodeInt(raw json.RawMessage) (int64, error) {
	if len(raw) == 0 {
		return 0, fmt.Errorf("%w: missing integer field", ErrMalformed)
	}
	n, err := strconv.ParseInt(strings.TrimSpace(string(raw)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: not an integer", ErrMalformed)
	}
	return n, nil
}

// decodeMatch validates a match record: path and lines values, a
// line_number integer >= 1, and a non-empty submatches array whose
// start/end satisfy 0 <= start <= end <= len(lines).
func decodeMatch(raw json.RawMessage) (Record, error) {
	data, err := decodeData(raw)
	if err != nil {
		return Record{}, err
	}
	path, err := decodeValue(data["path"])
	if err != nil {
		return Record{}, fmt.Errorf("%w: match path", ErrMalformed)
	}
	line, err := decodeValue(data["lines"])
	if err != nil {
		return Record{}, fmt.Errorf("%w: match lines", ErrMalformed)
	}
	num, err := decodeInt(data["line_number"])
	if err != nil {
		return Record{}, fmt.Errorf("%w: match line_number", ErrMalformed)
	}
	if num < 1 {
		return Record{}, fmt.Errorf("%w: line_number %d < 1", ErrMalformed, num)
	}
	var rawSubs []json.RawMessage
	if err := json.Unmarshal(data["submatches"], &rawSubs); err != nil || len(rawSubs) == 0 {
		return Record{}, fmt.Errorf("%w: submatches is not a non-empty array", ErrMalformed)
	}
	subs := make([]Submatch, 0, len(rawSubs))
	for _, rs := range rawSubs {
		var m map[string]json.RawMessage
		if err := json.Unmarshal(rs, &m); err != nil || m == nil {
			return Record{}, fmt.Errorf("%w: submatch is not an object", ErrMalformed)
		}
		mb, err := decodeValue(m["match"])
		if err != nil {
			return Record{}, fmt.Errorf("%w: submatch match", ErrMalformed)
		}
		start, err := decodeInt(m["start"])
		if err != nil {
			return Record{}, fmt.Errorf("%w: submatch start", ErrMalformed)
		}
		end, err := decodeInt(m["end"])
		if err != nil {
			return Record{}, fmt.Errorf("%w: submatch end", ErrMalformed)
		}
		if start < 0 || start > end || end > int64(len(line)) {
			return Record{}, fmt.Errorf("%w: submatch range (%d,%d) outside line of %d bytes", ErrMalformed, start, end, len(line))
		}
		subs = append(subs, Submatch{Start: int(start), End: int(end), Bytes: mb})
	}
	return Record{Kind: KindMatch, Path: path, Line: line, LineNumber: num, Submatches: subs}, nil
}

// decodeEnd validates an end record: path and a binary_offset field that
// is present and either null or a nonnegative integer.
func decodeEnd(raw json.RawMessage) (Record, error) {
	data, err := decodeData(raw)
	if err != nil {
		return Record{}, err
	}
	path, err := decodeValue(data["path"])
	if err != nil {
		return Record{}, fmt.Errorf("%w: end path", ErrMalformed)
	}
	off, ok := data["binary_offset"]
	if !ok {
		return Record{}, fmt.Errorf("%w: end missing binary_offset", ErrMalformed)
	}
	if strings.TrimSpace(string(off)) == "null" {
		return Record{Kind: KindEnd, Path: path}, nil
	}
	n, err := decodeInt(off)
	if err != nil {
		return Record{}, fmt.Errorf("%w: end binary_offset", ErrMalformed)
	}
	if n < 0 {
		return Record{}, fmt.Errorf("%w: binary_offset %d < 0", ErrMalformed, n)
	}
	return Record{Kind: KindEnd, Path: path, BinaryOffset: &n}, nil
}
