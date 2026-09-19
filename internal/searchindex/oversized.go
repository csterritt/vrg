package searchindex

import (
	"bytes"
	"encoding/json"
)

// MaxRecordBytes is the maximum JSON record payload: 64 MiB excluding
// the record's newline delimiter. A record longer than this is consumed
// and discarded through its next newline rather than parsed — the limit
// is enforced by an explicit bounded scan, not a line reader's default
// capacity.
const MaxRecordBytes = 64 << 20

// feedOversized accounts for one record that exceeded MaxRecordBytes:
// it counts in Oversized, never in Malformed, and never dispatches
// lifecycle — a skipped record leaves otherwise intact metadata alone.
// The record's path is recovered on a best-effort basis for the
// "oversized record skipped for <path>" diagnostic; when the byte limit
// was hit before type and data.path were parsed, the record is counted
// anonymously. Arriving after the summary, the record's position is
// still an after-summary integrity failure.
func (ix *Index) feedOversized(rec []byte) {
	ix.Oversized++
	if p, ok := recoverOversizedPath(rec); ok {
		ix.OversizedPaths = append(ix.OversizedPaths, p)
	}
	if ix.sawSummary {
		ix.broken = true
	}
}

// recoverOversizedPath makes a bounded best-effort pass over an
// oversized record for the two fields the diagnostic needs: the
// record's type and its data.path bytes. It reports the path only when
// both were parsed within the first MaxRecordBytes; a truncated or
// unexpected shape reports nothing.
func recoverOversizedPath(rec []byte) ([]byte, bool) {
	if len(rec) > MaxRecordBytes {
		rec = rec[:MaxRecordBytes]
	}
	dec := json.NewDecoder(bytes.NewReader(rec))
	t, err := dec.Token()
	if d, ok := t.(json.Delim); err != nil || !ok || d != '{' {
		return nil, false
	}
	var haveType bool
	var path []byte
	for dec.More() {
		kt, err := dec.Token()
		if err != nil {
			return nil, false
		}
		switch kt {
		case "type":
			vt, err := dec.Token()
			if err != nil {
				return nil, false
			}
			_, haveType = vt.(string)
		case "data":
			p, ok := recoverDataPath(dec)
			if !ok {
				return nil, false
			}
			path = p
		default:
			if !skipValue(dec) {
				return nil, false
			}
		}
		if haveType && path != nil {
			// Both fields the diagnostic needs are parsed; the rest
			// of the record — the oversized payload — is discarded.
			return path, true
		}
	}
	return nil, false
}

// recoverDataPath reads a data object's entries until it finds the
// path value, which it decodes through the same text/bytes rule as a
// parsed record. A giant sibling field — the payload that made the
// record oversized — ends the pass at the byte limit.
func recoverDataPath(dec *json.Decoder) ([]byte, bool) {
	t, err := dec.Token()
	if d, ok := t.(json.Delim); err != nil || !ok || d != '{' {
		return nil, false
	}
	for dec.More() {
		kt, err := dec.Token()
		if err != nil {
			return nil, false
		}
		if kt == "path" {
			var raw json.RawMessage
			if err := dec.Decode(&raw); err != nil {
				return nil, false
			}
			p, ok := decodeTextOrBytes(raw)
			return p, ok
		}
		if !skipValue(dec) {
			return nil, false
		}
	}
	return nil, false
}

// skipValue consumes one JSON value from the token stream, returning
// false when the value runs past the byte limit.
func skipValue(dec *json.Decoder) bool {
	var raw json.RawMessage
	return dec.Decode(&raw) == nil
}
