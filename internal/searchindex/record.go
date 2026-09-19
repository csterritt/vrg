package searchindex

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"strconv"
)

// Kind classifies one JSON record from the ripgrep event stream.
type Kind int

const (
	// KindMalformed records fail decoding or the per-record schema
	// matrix: invalid JSON, invalid base64, a missing or non-string type
	// field, or a known event with missing or mistyped required fields
	// or out-of-range values.
	KindMalformed Kind = iota
	// KindBegin marks the start of a file's result block.
	KindBegin
	// KindMatch carries one matched line and its submatches.
	KindMatch
	// KindEnd closes a file's result block.
	KindEnd
	// KindSummary is the stream's completion record.
	KindSummary
	// KindContext is a known event whose data payload is ignored: file
	// content comes from disk, not from the stream.
	KindContext
	// KindUnknown is a string type outside the five known events. It is
	// counted separately from malformed records (Issue #10).
	KindUnknown
)

// Record is one decoded stream record. Only the fields the record's Kind
// requires are populated: Path for KindBegin, KindMatch, and KindEnd;
// Line, LineNumber, and Submatches for KindMatch; Binary for KindEnd with
// a non-null binary_offset.
type Record struct {
	Kind       Kind
	Path       []byte
	Line       []byte
	LineNumber int64
	Submatches []Submatch
	Binary     bool
}

// ParseRecord decodes one newline-free JSON record and validates exactly
// the required fields of the issue's per-record schema matrix. Fields the
// matrix lists as ignored are never consulted. Cross-record lifecycle
// rules (Issue #9) and skip counting (Issue #10) are out of scope here:
// the caller decides what each Kind means for the stream.
func ParseRecord(raw []byte) Record {
	var head struct {
		Type json.RawMessage `json:"type"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &head); err != nil {
		return Record{Kind: KindMalformed}
	}
	typ, ok := decodeString(head.Type)
	if !ok {
		return Record{Kind: KindMalformed}
	}
	switch typ {
	case "begin":
		return parsePathRecord(head.Data, KindBegin)
	case "end":
		return parseEnd(head.Data)
	case "summary":
		if _, ok := dataObject(head.Data); !ok {
			return Record{Kind: KindMalformed}
		}
		return Record{Kind: KindSummary}
	case "context":
		return Record{Kind: KindContext}
	case "match":
		return parseMatch(head.Data)
	default:
		return Record{Kind: KindUnknown}
	}
}

// parsePathRecord validates the shared begin shape: a data object with a
// decodable path.
func parsePathRecord(data json.RawMessage, kind Kind) Record {
	d, ok := dataObject(data)
	if !ok {
		return Record{Kind: KindMalformed}
	}
	p, ok := decodeTextOrBytes(d["path"])
	if !ok {
		return Record{Kind: KindMalformed}
	}
	return Record{Kind: kind, Path: p}
}

// parseEnd validates the end shape: a path plus a binary_offset field
// that is present and either null or a nonnegative integer.
func parseEnd(data json.RawMessage) Record {
	d, ok := dataObject(data)
	if !ok {
		return Record{Kind: KindMalformed}
	}
	p, ok := decodeTextOrBytes(d["path"])
	if !ok {
		return Record{Kind: KindMalformed}
	}
	bo, present := d["binary_offset"]
	if !present {
		return Record{Kind: KindMalformed}
	}
	if bytes.Equal(bytes.TrimSpace(bo), []byte("null")) {
		return Record{Kind: KindEnd, Path: p}
	}
	n, ok := parseInt(bo)
	if !ok || n < 0 {
		return Record{Kind: KindMalformed}
	}
	return Record{Kind: KindEnd, Path: p, Binary: true}
}

// parseMatch validates the match shape: path and lines in either
// encoding, an integer line_number ≥ 1, and a nonempty submatches array
// whose elements carry a decodable match and integer start/end within
// 0 ≤ start ≤ end ≤ len(lines).
func parseMatch(data json.RawMessage) Record {
	d, ok := dataObject(data)
	if !ok {
		return Record{Kind: KindMalformed}
	}
	path, ok := decodeTextOrBytes(d["path"])
	if !ok {
		return Record{Kind: KindMalformed}
	}
	lines, ok := decodeTextOrBytes(d["lines"])
	if !ok {
		return Record{Kind: KindMalformed}
	}
	num, ok := parseInt(d["line_number"])
	if !ok || num < 1 {
		return Record{Kind: KindMalformed}
	}
	var rawSubs []json.RawMessage
	if err := json.Unmarshal(d["submatches"], &rawSubs); err != nil || len(rawSubs) == 0 {
		return Record{Kind: KindMalformed}
	}
	subs := make([]Submatch, 0, len(rawSubs))
	for _, rs := range rawSubs {
		sm, ok := dataObject(rs)
		if !ok {
			return Record{Kind: KindMalformed}
		}
		text, ok := decodeTextOrBytes(sm["match"])
		if !ok {
			return Record{Kind: KindMalformed}
		}
		start, ok := parseInt(sm["start"])
		if !ok {
			return Record{Kind: KindMalformed}
		}
		end, ok := parseInt(sm["end"])
		if !ok {
			return Record{Kind: KindMalformed}
		}
		if start < 0 || start > end || end > int64(len(lines)) {
			return Record{Kind: KindMalformed}
		}
		subs = append(subs, Submatch{Start: int(start), End: int(end), Bytes: text})
	}
	return Record{
		Kind:       KindMatch,
		Path:       path,
		Line:       lines,
		LineNumber: num,
		Submatches: subs,
	}
}

// dataObject decodes a JSON object into its raw fields. Missing data,
// null, and non-object values fail.
func dataObject(raw json.RawMessage) (map[string]json.RawMessage, bool) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil || m == nil {
		return nil, false
	}
	return m, true
}

// decodeTextOrBytes decodes the {"text": "..."} or {"bytes": "base64"}
// value form. Exactly one key must be present with a string value;
// invalid base64 fails.
func decodeTextOrBytes(raw json.RawMessage) ([]byte, bool) {
	m, ok := dataObject(raw)
	if !ok {
		return nil, false
	}
	tv, hasText := m["text"]
	bv, hasBytes := m["bytes"]
	if hasText == hasBytes {
		return nil, false
	}
	var s string
	if hasText {
		if s, ok = decodeString(tv); !ok {
			return nil, false
		}
		return []byte(s), true
	}
	if s, ok = decodeString(bv); !ok {
		return nil, false
	}
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, false
	}
	return b, true
}

// decodeString decodes a JSON string value; any other JSON value fails.
func decodeString(raw json.RawMessage) (string, bool) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] != '"' {
		return "", false
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", false
	}
	return s, true
}

// parseInt decodes a JSON integer literal that fits in int64. Floats,
// strings, and other JSON values fail.
func parseInt(raw json.RawMessage) (int64, bool) {
	s := bytes.TrimSpace(raw)
	if len(s) == 0 {
		return 0, false
	}
	i := 0
	if s[0] == '-' {
		i = 1
	}
	if i == len(s) {
		return 0, false
	}
	for ; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, false
		}
	}
	n, err := strconv.ParseInt(string(s), 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}
