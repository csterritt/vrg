package present

import (
	"strings"
	"testing"
)

// Path presentation: newline, carriage return, and tab take the short
// \n \r \t forms; backslash doubles; invalid UTF-8 bytes take \xNN;
// remaining C0 controls and DEL use caret notation; C1 controls use
// \uXXXX; valid printable Unicode passes through untouched.
func TestPath(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		want string
	}{
		{"plain", "dir/sub/file.go", "dir/sub/file.go"},
		{"newline", "a\nb", `a\nb`},
		{"carriage return", "a\rb", `a\rb`},
		{"tab", "a\tb", `a\tb`},
		{"backslash", `a\b`, `a\\b`},
		{"escaped backslash keeps escapes distinct", `a\nb` + "\n", `a\\nb\n`},
		{"invalid utf-8 byte", "a\xffb", `a\xffb`},
		{"invalid utf-8 run", "a\xff\xfeb", `a\xff\xfeb`},
		{"truncated utf-8 tail", "a\xe2\x82", `a\xe2\x82`},
		{"escape", "a\x1bb", `a^[b`},
		{"bell", "a\x07b", `a^Gb`},
		{"nul", "a\x00b", `a^@b`},
		{"delete", "a\x7fb", `a^?b`},
		{"c1 nel", "a\xc2\x85b", "a\\u0085b"},
		{"c1 csi", "a\xc2\x9bb", "a\\u009bb"},
		{"printable unicode", "héllö→世", "héllö→世"},
		{"all forms together", "x\ny\tz\\w\xff\x1bv", `x\ny\tz\\w\xff^[v`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Path([]byte(tc.raw)); got != tc.want {
				t.Fatalf("Path(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

// An escaped path never emits a raw control byte: every byte that could
// move the terminal is spelled out.
func TestPathNeverEmitsControls(t *testing.T) {
	for i := 0; i < 0x20; i++ {
		raw := []byte{'a', byte(i), 'b'}
		got := Path(raw)
		for j := 0; j < len(got); j++ {
			if c := got[j]; c < 0x20 || c == 0x7f {
				t.Fatalf("Path(%q) emitted control byte 0x%02x: %q", raw, c, got)
			}
		}
	}
}

// Diagnostic presentation: the message's own line structure survives —
// LF is a real line boundary and CRLF counts as one boundary, a
// standalone CR takes ^M — tabs expand to the next multiple of eight
// display columns, other C0 controls and DEL take caret notation, C1
// takes \uXXXX, invalid UTF-8 takes \xNN, and printable text including
// backslash passes through.
func TestDiagnostic(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		want string
	}{
		{"plain", "plain message", "plain message"},
		{"line boundary preserved", "first\nsecond", "first\nsecond"},
		{"multiple boundaries", "a\nb\nc", "a\nb\nc"},
		{"trailing newline", "line\n", "line\n"},
		{"crlf is one boundary", "a\r\nb", "a\nb"},
		{"standalone cr", "a\rb", "a^Mb"},
		{"tab expands to eight", "a\tb", "a       b"},
		{"tab at line start", "\tx", "        x"},
		{"tab aligns to next stop", "abcdefg\tx", "abcdefg x"},
		{"tab resets per line", "a\tb\nc\td", "a       b\nc       d"},
		{"tab counts escaped cells", "\x1b\tx", "^[      x"},
		{"escape", "a\x1bb", "a^[b"},
		{"bell", "a\x07b", "a^Gb"},
		{"nul", "a\x00b", "a^@b"},
		{"delete", "a\x7fb", "a^?b"},
		{"c1 nel", "a\xc2\x85b", "a\\u0085b"},
		{"invalid utf-8 byte", "a\xffb", `a\xffb`},
		{"invalid utf-8 run", "a\xff\xfeb", `a\xff\xfeb`},
		{"backslash passes through", `a\b`, `a\b`},
		{"literal backslash-n stays literal", `a\nb`, `a\nb`},
		{"printable unicode", "héllö→世", "héllö→世"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Diagnostic(tc.raw); got != tc.want {
				t.Fatalf("Diagnostic(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

// A diagnostic never emits a raw control byte other than the message's
// own line boundaries: every byte that could move the terminal is
// spelled out or expanded.
func TestDiagnosticNeverEmitsControls(t *testing.T) {
	for i := 0; i < 0x20; i++ {
		raw := "a" + string(byte(i)) + "b"
		got := Diagnostic(raw)
		for j := 0; j < len(got); j++ {
			if c := got[j]; (c < 0x20 && c != '\n') || c == 0x7f {
				t.Fatalf("Diagnostic(%q) emitted control byte 0x%02x: %q", raw, c, got)
			}
		}
	}
}

// A filename embedded in a diagnostic is escaped with Path first — a
// single-line filename — so its newline can never become a diagnostic
// paragraph break. Diagnostic leaves the printable escaped form intact:
// only the message's own newlines break lines.
func TestDiagnosticEmbedsSingleLinedFilename(t *testing.T) {
	name := []byte("bad\nna\x1bme\xff.txt")
	got := Diagnostic("cannot read " + Path(name) + ": denied")
	want := `cannot read bad\nna^[me\xff.txt: denied`
	if got != want {
		t.Fatalf("embedded filename diagnostic = %q, want %q", got, want)
	}
	if strings.Contains(got, "\n") {
		t.Fatalf("embedded filename became a diagnostic line break: %q", got)
	}

	// The diagnostic's own boundaries still break lines around an
	// embedded name.
	got = Diagnostic("first\n" + Path(name))
	want = "first\n" + `bad\nna^[me\xff.txt`
	if got != want {
		t.Fatalf("multi-line diagnostic = %q, want %q", got, want)
	}
}
