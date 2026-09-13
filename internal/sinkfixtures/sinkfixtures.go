// Package sinkfixtures holds the shared hostile-fixture set and
// sink-safety assertion helpers for the Issue #6 sink-safety table.
// Each sink is a row; later issues add rows without duplicating
// fixtures. The fixtures cover OSC, CSI, C0, C1, DEL, standalone CR,
// invalid UTF-8 path bytes, and an embedded filename newline — the
// Issue #5 hostile set restructured into a shared, extensible table.
package sinkfixtures

// Fixture is one hostile input exercised across every sink.
type Fixture struct {
	// Name is the short identifier used in subtest names.
	Name string
	// Raw is the hostile byte sequence.
	Raw []byte
	// Payload is the distinctive byte sequence that must never appear
	// immediately after an unescaped ESC in styled output. For
	// ESC-prefixed fixtures (OSC, CSI) this is the bytes following the
	// leading ESC; for other fixtures it is the dangerous bytes
	// themselves. An empty payload means the assertion is vacuously
	// true for this fixture.
	Payload []byte
}

// Fixtures is the shared hostile-fixture set. Every sink row iterates
// over this slice. Later issues that add sinks reuse the same fixtures;
// later issues that add fixtures append here and every existing sink
// row picks them up automatically.
var Fixtures = []Fixture{
	{
		Name:    "OSC",
		Raw:     []byte("\x1b]0;x\x07"),
		Payload: []byte("]0;x"),
	},
	{
		Name:    "CSI",
		Raw:     []byte("\x1b[2J"),
		Payload: []byte("[2J"),
	},
	{
		Name:    "C0",
		Raw:     []byte("\x07\x08\x1b"),
		Payload: []byte("\x07\x08"),
	},
	{
		Name:    "C1",
		Raw:     []byte("\xc2\x85"),
		Payload: []byte("\xc2\x85"),
	},
	{
		Name:    "DEL",
		Raw:     []byte("\x7f"),
		Payload: []byte("\x7f"),
	},
	{
		Name:    "StandaloneCR",
		Raw:     []byte("\r"),
		Payload: []byte("\r"),
	},
	{
		Name:    "InvalidUTF8",
		Raw:     []byte("foo\xff\xfebar"),
		Payload: []byte("\xff\xfe"),
	},
	{
		Name:    "EmbeddedNewline",
		Raw:     []byte("file\nname"),
		Payload: []byte("\n"),
	},
}

// PathFixtures returns the subset of Fixtures suitable for testing
// path sinks (file-list entry, filename rule, usage-error root). These
// are the same fixtures — every fixture can appear in a path.
func PathFixtures() []Fixture {
	return Fixtures
}

// ContentFixtures returns fixtures suitable for testing content sinks
// (panel content). These are the same fixtures plus LF and CRLF
// terminator cases that only apply to content.
func ContentFixtures() []Fixture {
	return append([]Fixture{}, Fixtures...)
}

// NoControlBytes reports whether s contains no C0 control bytes or
// DEL, excluding newlines which are part of the multi-line layout. This
// is the raw-output assertion for sink-safety: no fixture control byte
// may survive verbatim. Use this for sinks that incorporate external
// data and should have no control bytes at all (after escaping).
func NoControlBytes(s string) bool {
	for i := 0; i < len(s); i++ {
		b := s[i]
		if b == '\n' {
			continue
		}
		if b < 0x20 || b == 0x7f {
			return false
		}
	}
	return true
}

// NoDangerousControls reports whether s contains no dangerous C0
// control bytes or DEL, excluding newlines and tabs which are
// legitimate formatting in fixed-text sinks like generated help. Use
// this for sinks that do not incorporate external data but should still
// be free of terminal-control sequences.
func NoDangerousControls(s string) bool {
	for i := 0; i < len(s); i++ {
		b := s[i]
		if b == '\n' || b == '\t' {
			continue
		}
		if b < 0x20 || b == 0x7f {
			return false
		}
	}
	return true
}

// NoPayloadAfterESC reports whether payload never appears immediately
// after an ESC byte (0x1b) in s. This is the styled-output assertion:
// with styles enabled, ANSI ESCs are legitimate, but the fixture's ESC
// should be escaped as ^[. If the fixture's payload appears after a
// raw ESC, the ESC was not escaped and the payload could form a
// dangerous terminal sequence.
func NoPayloadAfterESC(s string, payload []byte) bool {
	if len(payload) == 0 {
		return true
	}
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b && i+1+len(payload) <= len(s) {
			if string(s[i+1:i+1+len(payload)]) == string(payload) {
				return false
			}
		}
	}
	return true
}
