package safepresentation_test

import (
	"strings"
	"testing"
	"unicode/utf8"

	"vrg/internal/safepresentation"
)

// Diagnostic escaping (Issue #6): real diagnostic line boundaries are
// preserved — LF stays and CRLF normalizes to LF — tabs expand to the
// next multiple of eight columns, other controls escape under the same
// safe policy as paths, and a backslash is literal because a diagnostic
// is not a path.
func TestEscapeDiagnostic(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "vrg: something failed", "vrg: something failed"},
		{"line boundaries preserved", "first\nsecond\nthird", "first\nsecond\nthird"},
		{"crlf is a boundary", "first\r\nsecond", "first\nsecond"},
		{"trailing crlf", "a\r\n", "a\n"},
		{"standalone cr", "a\rb", "a^Mb"},
		{"tab at start", "\tx", "        x"},
		{"tab to next stop", "a\tb", "a       b"},
		{"tab at a stop", "abcdefgh\tx", "abcdefgh        x"},
		{"tab after wide rune", "文\tx", "文      x"},
		{"tab resets per line", "abcdefgh\nx\ty", "abcdefgh\nx       y"},
		{"escape", "bad\x1b[31m", "bad^[[31m"},
		{"bell", "a\ab", "a^Gb"},
		{"nul", "a\x00b", "a^@b"},
		{"del", "a\x7fb", "a^?b"},
		{"c1 nel", "a\xc2\x85b", `a\u0085b`},
		{"invalid byte", "a\xffb", `a\xffb`},
		{"truncated utf-8", "a\xe2\x82b", `a\xe2\x82b`},
		{"backslash is literal", `a\b`, `a\b`},
		{"printable unicode", "héllo 文件", "héllo 文件"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := safepresentation.EscapeDiagnostic(tc.in); got != tc.want {
				t.Fatalf("EscapeDiagnostic(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// A filename embedded in a diagnostic is escaped first as a single-line
// filename, so its newline cannot become a diagnostic line boundary:
// the diagnostic keeps only its own real breaks.
func TestEscapeDiagnosticEmbeddedFilename(t *testing.T) {
	name := []byte("bad\nname\x1b\xff.txt")
	diag := safepresentation.EscapeDiagnostic(
		"cannot load\npath: " + safepresentation.EscapePath(name) + "\ndone")
	want := "cannot load\npath: bad\\nname^[\\xff.txt\ndone"
	if diag != want {
		t.Fatalf("EscapeDiagnostic = %q, want %q", diag, want)
	}
	if n := strings.Count(diag, "\n"); n != 2 {
		t.Fatalf("diagnostic has %d line boundaries, want 2 — the embedded filename's newline must not count", n)
	}
}

// No diagnostic output may carry a raw control byte: every C0 byte is
// either a preserved boundary (\n, or \r inside CRLF) or escapes.
func TestEscapeDiagnosticEmitsNoRawControls(t *testing.T) {
	for b := 0; b < 0x20; b++ {
		out := safepresentation.EscapeDiagnostic("x" + string(byte(b)) + "y")
		for _, r := range out {
			if r < 0x20 && r != '\n' {
				t.Fatalf("EscapeDiagnostic left raw byte %#02x in %q", b, out)
			}
		}
	}
	for _, in := range []string{"x\x7fy", "x\xc2\x85y", "x\xffy", "x\xc0\xafy"} {
		out := safepresentation.EscapeDiagnostic(in)
		if !utf8.ValidString(out) {
			t.Fatalf("EscapeDiagnostic(%q) emitted invalid UTF-8: %q", in, out)
		}
		for _, r := range out {
			if r < 0x20 || r == 0x7f || (r >= 0x80 && r < 0xa0) {
				t.Fatalf("EscapeDiagnostic(%q) left raw control rune %#U in %q", in, r, out)
			}
		}
	}
}
