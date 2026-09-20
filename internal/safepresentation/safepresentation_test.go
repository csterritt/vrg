package safepresentation_test

import (
	"strings"
	"testing"

	"vrg/internal/safepresentation"
)

// EscapePath escapes raw path bytes into a single safe display line:
// newline, carriage return, and tab become backslash forms, a literal
// backslash doubles, invalid UTF-8 bytes become \xNN, other C0 controls
// and DEL use caret notation, C1 controls use \uXXXX escapes, and
// printable text — including valid multi-byte Unicode — passes through
// untouched. (Issue 5 core cases, unchanged against the generalized
// utility.)
func TestPathEscapes(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want string
	}{
		{"plain", "dir/file.txt", "dir/file.txt"},
		{"newline", "a\nb", `a\nb`},
		{"carriage return", "a\rb", `a\rb`},
		{"tab", "a\tb", `a\tb`},
		{"backslash", `a\b`, `a\\b`},
		{"escape byte", "a\x1bb", `a^[b`},
		{"bell", "a\x07b", `a^Gb`},
		{"delete", "a\x7fb", `a^?b`},
		{"invalid utf-8 byte", "a\xffb", `a\xffb`},
		{"truncated utf-8", "a\xc2b", `a\xc2b`},
		{"c1 nel", "a\u0085b", `a\u0085b`},
		{"valid unicode", "héllo/世界", "héllo/世界"},
		{"empty", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := safepresentation.EscapePath([]byte(tc.in)); got != tc.want {
				t.Fatalf("EscapePath(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// cellText joins cell display texts into the rendered line.
func cellText(cells []safepresentation.Cell) string {
	var b strings.Builder
	for _, c := range cells {
		b.WriteString(c.Text)
	}
	return b.String()
}

// EscapeContent escapes one source line's raw bytes — its terminator
// already removed — into display cells: C0 controls and DEL use caret
// notation, a standalone carriage return renders as ^M, a tab expands
// to the next multiple of eight source-display columns, C1 controls use
// \uXXXX escapes, invalid UTF-8 becomes U+FFFD, and printable text
// passes through. (Issue 5 core cases, with structural tabs.)
func TestContentEscapes(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want string
	}{
		{"plain", "hello", "hello"},
		{"escape byte", "\x1b", "^["},
		{"osc payload", "\x1b]0;x\x07", "^[]0;x^G"},
		{"csi payload", "\x1b[2J", "^[[2J"},
		{"bell", "\x07", "^G"},
		{"backspace", "\x08", "^H"},
		{"nul", "\x00", "^@"},
		{"unit separator", "\x1f", "^_"},
		{"delete", "\x7f", "^?"},
		{"standalone carriage return", "a\rb", "a^Mb"},
		{"tab expands to the next stop", "a\tb", "a       b"},
		{"c1 nel", "a\u0085b", `a\u0085b`},
		{"invalid utf-8 byte", "a\xffb", "a\uFFFDb"},
		{"invalid utf-8 run", "a\xff\xfeb", "a\uFFFD\uFFFDb"},
		{"valid unicode", "héllo 世界", "héllo 世界"},
		{"empty", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cells, _ := safepresentation.EscapeContent([]byte(tc.in))
			if got := cellText(cells); got != tc.want {
				t.Fatalf("EscapeContent(%q) text = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// Every content cell maps back to the exact half-open source-byte range
// it presents, so a highlight over source bytes can cover every cell of
// an escaped form.
func TestContentCellByteMappings(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want []safepresentation.Cell
	}{
		{"plain", "ab", []safepresentation.Cell{
			{Text: "a", Start: 0, End: 1},
			{Text: "b", Start: 1, End: 2},
		}},
		{"escape byte", "\x1b", []safepresentation.Cell{
			{Text: "^", Start: 0, End: 1},
			{Text: "[", Start: 0, End: 1},
		}},
		{"c1 nel", "\u0085", []safepresentation.Cell{
			{Text: `\`, Start: 0, End: 2},
			{Text: `u`, Start: 0, End: 2},
			{Text: `0`, Start: 0, End: 2},
			{Text: `0`, Start: 0, End: 2},
			{Text: `8`, Start: 0, End: 2},
			{Text: `5`, Start: 0, End: 2},
		}},
		{"invalid utf-8 byte", "\xff", []safepresentation.Cell{
			{Text: "\uFFFD", Start: 0, End: 1},
		}},
		{"tab at column zero", "\t", []safepresentation.Cell{
			{Text: " ", Start: 0, End: 1},
			{Text: " ", Start: 0, End: 1},
			{Text: " ", Start: 0, End: 1},
			{Text: " ", Start: 0, End: 1},
			{Text: " ", Start: 0, End: 1},
			{Text: " ", Start: 0, End: 1},
			{Text: " ", Start: 0, End: 1},
			{Text: " ", Start: 0, End: 1},
		}},
		{"multi-byte rune", "é", []safepresentation.Cell{
			{Text: "é", Start: 0, End: 2},
		}},
		{"standalone carriage return", "\r", []safepresentation.Cell{
			{Text: "^", Start: 0, End: 1},
			{Text: "M", Start: 0, End: 1},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, _ := safepresentation.EscapeContent([]byte(tc.in))
			if len(got) != len(tc.want) {
				t.Fatalf("EscapeContent(%q) produced %d cells %+v, want %d", tc.in, len(got), got, len(tc.want))
			}
			for i, c := range got {
				if c != tc.want[i] {
					t.Fatalf("EscapeContent(%q) cell %d = %+v, want %+v", tc.in, i, c, tc.want[i])
				}
			}
		})
	}
}

// EscapeContent also reports the line's grapheme clusters: the half-open
// display-cell range each cluster occupies and its terminal cell width.
// Combining sequences and emoji ZWJ sequences are one cluster; a tab is
// one cluster spanning its expansion; escaped forms keep their cells in
// a single cluster.
func TestContentClusters(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want []safepresentation.Cluster
	}{
		{"plain", "ab", []safepresentation.Cluster{
			{Start: 0, End: 1, Width: 1},
			{Start: 1, End: 2, Width: 1},
		}},
		{"combining sequence", "e\u0301x", []safepresentation.Cluster{
			{Start: 0, End: 2, Width: 1},
			{Start: 2, End: 3, Width: 1},
		}},
		{"wide runes", "世界", []safepresentation.Cluster{
			{Start: 0, End: 1, Width: 2},
			{Start: 1, End: 2, Width: 2},
		}},
		{"emoji zwj sequence", "👨‍👩‍👧!", []safepresentation.Cluster{
			{Start: 0, End: 5, Width: 2},
			{Start: 5, End: 6, Width: 1},
		}},
		{"escaped control", "\x1bx", []safepresentation.Cluster{
			{Start: 0, End: 2, Width: 2},
			{Start: 2, End: 3, Width: 1},
		}},
		{"c1 escape", "\u0085x", []safepresentation.Cluster{
			{Start: 0, End: 6, Width: 6},
			{Start: 6, End: 7, Width: 1},
		}},
		{"invalid utf-8", "\xffx", []safepresentation.Cluster{
			{Start: 0, End: 1, Width: 1},
			{Start: 1, End: 2, Width: 1},
		}},
		{"tab at column zero", "\tx", []safepresentation.Cluster{
			{Start: 0, End: 8, Width: 8},
			{Start: 8, End: 9, Width: 1},
		}},
		{"tab at column one", "a\tb", []safepresentation.Cluster{
			{Start: 0, End: 1, Width: 1},
			{Start: 1, End: 8, Width: 7},
			{Start: 8, End: 9, Width: 1},
		}},
		{"tab after wide rune", "世\tx", []safepresentation.Cluster{
			{Start: 0, End: 1, Width: 2},
			{Start: 1, End: 7, Width: 6},
			{Start: 7, End: 8, Width: 1},
		}},
		{"empty", "", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, got := safepresentation.EscapeContent([]byte(tc.in))
			if len(got) != len(tc.want) {
				t.Fatalf("EscapeContent(%q) clusters = %+v, want %+v", tc.in, got, tc.want)
			}
			for i, c := range got {
				if c != tc.want[i] {
					t.Fatalf("EscapeContent(%q) cluster %d = %+v, want %+v",
						tc.in, i, c, tc.want[i])
				}
			}
		})
	}
}

// Span maps a half-open source-byte range to the half-open cell range it
// covers: a match covering an ESC byte highlights both caret cells, and a
// match inside a multi-cell escape covers the whole escape.
func TestContentSpanMapping(t *testing.T) {
	cells, _ := safepresentation.EscapeContent([]byte("a\x1bb")) // renders a ^ [ b
	if got := cellText(cells); got != "a^[b" {
		t.Fatalf("fixture text = %q, want %q", got, "a^[b")
	}
	for _, tc := range []struct {
		name       string
		start, end int
		wantS      int
		wantE      int
	}{
		{"plain byte", 0, 1, 0, 1},
		{"escape byte covers caret pair", 1, 2, 1, 3},
		{"escape plus neighbour", 0, 2, 0, 3},
		{"trailing byte", 2, 3, 3, 4},
		{"whole line", 0, 3, 0, 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, e := safepresentation.Span(cells, tc.start, tc.end)
			if s != tc.wantS || e != tc.wantE {
				t.Fatalf("Span(%d, %d) = (%d, %d), want (%d, %d)",
					tc.start, tc.end, s, e, tc.wantS, tc.wantE)
			}
		})
	}
}

// A span over a multi-byte C1 escape covers all six cells of its
// \uXXXX-style form.
func TestContentSpanCoversC1Escape(t *testing.T) {
	cells, _ := safepresentation.EscapeContent([]byte("x\u0085y"))
	s, e := safepresentation.Span(cells, 1, 3)
	if s != 1 || e != 7 {
		t.Fatalf("Span(1, 3) = (%d, %d), want (1, 7)", s, e)
	}
}

// EscapeDiagnostic renders diagnostic text terminal-safe while keeping
// its structure: real line boundaries survive (LF, and CRLF as a single
// boundary), a standalone carriage return renders as ^M, tabs expand to
// the next multiple of eight cells, and every other control escapes with
// the same policy as paths — caret notation for C0 and DEL, \uXXXX for
// C1, \xNN for invalid UTF-8.
func TestDiagnosticEscapes(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want string
	}{
		{"plain", "all good", "all good"},
		{"line boundaries preserved", "alpha\nbeta\ngamma", "alpha\nbeta\ngamma"},
		{"trailing newline kept", "warn\n", "warn\n"},
		{"crlf is one boundary", "a\r\nb", "a\nb"},
		{"standalone carriage return", "a\rb", "a^Mb"},
		{"cr between boundaries", "a\rb\nc", "a^Mb\nc"},
		{"tab at line start", "\tx", "        x"},
		{"tab mid-line", "a\tb", "a       b"},
		{"tab at column two", "ab\tc", "ab      c"},
		{"consecutive tabs", "a\t\tb", "a               b"},
		{"tab resets at boundary", "ab\nc\td", "ab\nc       d"},
		{"tab after escaped control", "a\x1b\tb", "a^[     b"},
		{"tab after wide rune", "世\tx", "世      x"},
		{"escape byte", "a\x1bb", "a^[b"},
		{"osc payload", "x\x1b]0;pwned\x07y", "x^[]0;pwned^Gy"},
		{"csi payload", "x\x1b[2Jy", "x^[[2Jy"},
		{"delete", "a\x7fb", "a^?b"},
		{"c1 nel", "a\u0085b", `a\u0085b`},
		{"invalid utf-8 byte", "a\xffb", `a\xffb`},
		{"backslash passes through", `a\b`, `a\b`},
		{"escaped form passes through", `a\nb`, `a\nb`},
		{"valid unicode", "héllo 世界\n次", "héllo 世界\n次"},
		{"empty", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := safepresentation.EscapeDiagnostic(tc.in); got != tc.want {
				t.Fatalf("EscapeDiagnostic(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// A filename embedded in a diagnostic is single-line-escaped first, so
// its newline becomes a literal \n and can never forge a diagnostic
// paragraph break, while the diagnostic's own real line boundaries are
// preserved.
func TestDiagnosticEmbedsSingleLinedFilename(t *testing.T) {
	name := []byte("ho\nstile\x1bf.txt")
	diag := "cannot read " + safepresentation.EscapePath(name) + "\npermission denied"
	got := safepresentation.EscapeDiagnostic(diag)
	want := "cannot read ho\\nstile^[f.txt\npermission denied"
	if got != want {
		t.Fatalf("EscapeDiagnostic(%q) = %q, want %q", diag, got, want)
	}
	if n := strings.Count(got, "\n"); n != 1 {
		t.Fatalf("embedded filename forged %d extra line boundaries: %q", n-1, got)
	}
}
