// Package sinktest is the shared sink-safety harness for the
// safe-presentation utility: one hostile fixture set plus the raw-output
// assertions every output sink must satisfy. It is test support —
// imported only by _test.go files — so it never reaches the binary.
//
// The sink-safety table is a []Sink declared in the owning package's
// test file; each row drives every fixture through that sink's real
// composition path and returns the raw output. Issues that add sinks —
// the error overlay (#9), stderr replay (#11), the pop-up (#15), the TUI
// help dialog (#31), generated documentation (#34) — add a row where
// the sink lives, reusing Fixtures rather than duplicating them.
package sinktest

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// Fixture is one hostile byte string injected into every sink row.
// Payload is the fixture's distinctive byte string — the part that would
// complete an escape sequence — which must never appear immediately
// after an unescaped ESC in styled output.
type Fixture struct {
	Name    string
	Bytes   []byte
	Payload string
}

// Fixtures is the hostile fixture set every sink must survive: an OSC
// title-set sequence, a CSI sequence, a run of C0 controls, a C1
// control, DEL, a standalone CR, invalid UTF-8 bytes, and an embedded
// filename newline.
var Fixtures = []Fixture{
	{"osc title-set", []byte("\x1b]0;pwned\x07"), "]0;pwned"},
	{"csi erase", []byte("\x1b[2J"), "[2J"},
	{"c0 run", []byte("\x07\x08\x1b"), "\x07\x08\x1b"},
	{"c1 nel", []byte("\xc2\x85"), "\xc2\x85"},
	{"del", []byte("\x7f"), "\x7f"},
	{"standalone cr", []byte("\r"), "\r"},
	{"invalid utf-8", []byte("\xff"), "\xff"},
	{"embedded filename newline", []byte("bad\nname"), "bad\nname"},
}

// Sink is one output sink under test.
type Sink struct {
	Name string
	// Render drives the fixture through the sink's real composition
	// path on the no-style path and returns the raw output — before any
	// ANSI stripping. It should also fail the test when the fixture
	// never reached the sink, so a silent drop cannot masquerade as a
	// pass.
	Render func(t *testing.T, fx Fixture) string
	// RenderStyled, when non-nil, renders the same fixture with the
	// styled theme so Run can assert the fixture's payload never
	// completes an escape sequence among legitimate style bytes. Nil
	// for sinks with no styled variant.
	RenderStyled func(t *testing.T, fx Fixture) string
}

// Run drives every fixture through every sink row as a subtest named
// "<sink>/<fixture>".
func Run(t *testing.T, sinks []Sink) {
	t.Helper()
	for _, s := range sinks {
		for _, fx := range Fixtures {
			t.Run(s.Name+"/"+fx.Name, func(t *testing.T) {
				AssertRawOutput(t, s.Render(t, fx))
				if s.RenderStyled != nil {
					AssertPayloadNotEscaped(t, s.RenderStyled(t, fx), fx.Payload)
				}
			})
		}
	}
}

// AssertRawOutput asserts a sink's raw output — before any ANSI
// stripping — is clean: valid UTF-8 carrying no C0 control other than
// the '\n' framing, no DEL, and no C1 control. On the no-style
// composition path no decorator emits bytes, so any control byte
// present can only be a fixture byte that survived verbatim. Stripping
// ANSI instead would erase the very evidence being sought.
func AssertRawOutput(t *testing.T, out string) {
	t.Helper()
	if !utf8.ValidString(out) {
		t.Fatalf("raw output contains invalid UTF-8: %q", out)
	}
	for _, r := range out {
		if r == '\n' {
			continue // row separators are vrg's own framing, not data
		}
		if r < 0x20 || r == 0x7f || (r >= 0x80 && r < 0xa0) {
			t.Fatalf("raw output contains a verbatim control rune %#U: %q", r, out)
		}
	}
}

// AssertPayloadNotEscaped asserts the fixture's distinctive payload
// never appears immediately after an unescaped ESC: among the style
// sequences a styled composition legitimately emits, fixture bytes must
// not be able to complete an escape sequence.
func AssertPayloadNotEscaped(t *testing.T, out, payload string) {
	t.Helper()
	if payload == "" {
		return
	}
	if i := strings.Index(out, "\x1b"+payload); i >= 0 {
		t.Fatalf("styled output has payload %q immediately after an unescaped ESC at byte %d: %q", payload, i, out)
	}
}
