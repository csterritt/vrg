package safepresentation_test

import (
	"strings"
	"testing"

	"vrg/internal/safepresentation"
)

// Path escaping (Issue #5): \n, \r, \t become the two-character escapes;
// backslashes double; invalid UTF-8 bytes become \xNN; remaining C0
// controls and DEL use caret notation; C1 controls use \u escapes; valid
// printable Unicode passes through.
func TestEscapePath(t *testing.T) {
	cases := []struct {
		name string
		in   []byte
		want string
	}{
		{"plain", []byte("dir/file.go"), "dir/file.go"},
		{"newline", []byte("a\nb"), `a\nb`},
		{"carriage return", []byte("a\rb"), `a\rb`},
		{"tab", []byte("a\tb"), `a\tb`},
		{"backslash", []byte(`a\b`), `a\\b`},
		{"escape", []byte("a\x1bb"), "a^[b"},
		{"bell", []byte("a\ab"), "a^Gb"},
		{"nul", []byte{'a', 0, 'b'}, "a^@b"},
		{"del", []byte("a\x7fb"), "a^?b"},
		{"invalid byte", []byte{'a', 0xff, 'b'}, `a\xffb`},
		{"invalid lead byte", []byte{'a', 0xc0, 0xaf, 'b'}, `a\xc0\xafb`},
		{"truncated sequence", []byte{'a', 0xe2, 0x82, 'b'}, `a\xe2\x82b`},
		{"c1 nel", []byte("a\xc2\x85b"), `a\u0085b`},
		{"c1 csi", []byte("a\xc2\x9bb"), `a\u009bb`},
		{"printable unicode", []byte("héllo 文件"), "héllo 文件"},
		{"combining mark", []byte("cafe\xcc\x81"), "café"},
		{"line separator", []byte("a\u2028b"), `a\u2028b`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := safepresentation.EscapePath(tc.in); got != tc.want {
				t.Fatalf("EscapePath(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// No escaped path may carry a raw control byte or a raw fixture byte
// into a terminal sink.
func TestEscapePathEmitsNoRawControls(t *testing.T) {
	for b := 0; b < 0x20; b++ {
		out := safepresentation.EscapePath([]byte{'x', byte(b), 'y'})
		if strings.Contains(out, string(byte(b))) {
			t.Fatalf("EscapePath left raw byte %#02x in %q", b, out)
		}
	}
	for _, in := range [][]byte{
		{'x', 0x7f, 'y'},             // DEL
		{'x', 0xc2, 0x85, 'y'},       // C1 NEL
		{'x', 0xff, 'y'},             // invalid byte
		{'x', 0xe2, 0x80, 0xa8, 'y'}, // U+2028 line separator
		{'x', 0xef, 0xbf, 0xbd, 'y'}, // genuine U+FFFD is printable
	} {
		out := safepresentation.EscapePath(in)
		for i := 0; i < len(out); i++ {
			if c := out[i]; c < 0x20 || c == 0x7f {
				t.Fatalf("EscapePath(%q) left raw byte %#02x in %q", in, c, out)
			}
		}
	}
}

// Plain content maps one cell per printable ASCII byte.
func TestMapContentPlainText(t *testing.T) {
	m := safepresentation.MapContent([]byte("hello"))
	if m.Text != "hello" {
		t.Fatalf("Text = %q, want %q", m.Text, "hello")
	}
	if len(m.Cells) != 5 {
		t.Fatalf("Cells = %d, want 5", len(m.Cells))
	}
	for i, c := range m.Cells {
		if c.Text != string("hello"[i]) || c.Start != i || c.End != i+1 {
			t.Fatalf("cell %d = %+v, want text %q over bytes [%d,%d)", i, c, string("hello"[i]), i, i+1)
		}
	}
}

// Content escaping rules: C0 controls and DEL render in caret notation,
// a standalone CR is ^M, a tab renders as a single provisional → cell,
// C1 controls use \u escapes, and invalid UTF-8 bytes render as U+FFFD.
func TestMapContentEscapes(t *testing.T) {
	cases := []struct {
		name string
		in   []byte
		want string
	}{
		{"escape", []byte("\x1b"), "^["},
		{"bell", []byte("\a"), "^G"},
		{"backspace", []byte("\b"), "^H"},
		{"nul", []byte{0}, "^@"},
		{"del", []byte("\x7f"), "^?"},
		{"standalone cr", []byte("\r"), "^M"},
		{"tab", []byte("\t"), "→"},
		{"invalid byte", []byte{0xff}, ""},
		{"invalid run", []byte{0xff, 0xfe}, ""},
		{"c1 nel", []byte("\xc2\x85"), `\u0085`},
		{"osc line", []byte("\x1b]0;pwned\x07"), "^[]0;pwned^G"},
		{"csi line", []byte("\x1b[2J"), "^[[2J"},
		{"mixed", []byte("a\x1bb\rc\td"), "a^[b^Mc→d"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := safepresentation.MapContent(tc.in)
			if m.Text != tc.want {
				t.Fatalf("MapContent(%q).Text = %q, want %q", tc.in, m.Text, tc.want)
			}
			assertNoControlRunes(t, m.Text, tc.in)
		})
	}
}

func assertNoControlRunes(t *testing.T, text string, in []byte) {
	t.Helper()
	for _, r := range text {
		if r < 0x20 || r == 0x7f || (r >= 0x80 && r < 0xa0) {
			t.Fatalf("MapContent(%q).Text = %q still carries control rune %#U", in, text, r)
		}
	}
}

// A standalone CR inside a line is not a terminator: it is escaped as
// ^M and never emitted raw or dropped.
func TestMapContentStandaloneCR(t *testing.T) {
	m := safepresentation.MapContent([]byte("a\rb"))
	if m.Text != "a^Mb" {
		t.Fatalf("Text = %q, want %q", m.Text, "a^Mb")
	}
	if len(m.Cells) != 4 {
		t.Fatalf("Cells = %d, want 4", len(m.Cells))
	}
	for _, i := range []int{1, 2} {
		if m.Cells[i].Start != 1 || m.Cells[i].End != 2 {
			t.Fatalf("cell %d maps to bytes [%d,%d), want [1,2)", i, m.Cells[i].Start, m.Cells[i].End)
		}
	}
}

// The provisional tab form: a single → placeholder occupying exactly
// one cell. Issue #5 does not assert cell positions on tab lines beyond
// this contract; Issue #16 owns the structural eight-column-stop rule.
func TestMapContentTabPlaceholder(t *testing.T) {
	m := safepresentation.MapContent([]byte("\t"))
	if m.Text != "→" {
		t.Fatalf("Text = %q, want %q", m.Text, "→")
	}
	if len(m.Cells) != 1 {
		t.Fatalf("Cells = %d, want a single placeholder cell", len(m.Cells))
	}
	if m.Cells[0].Text != "→" || m.Cells[0].Start != 0 || m.Cells[0].End != 1 {
		t.Fatalf("tab cell = %+v, want → over bytes [0,1)", m.Cells[0])
	}
}

// Every cell of an escaped form maps back to the source bytes that
// produced it, so a highlight over those bytes covers the whole escape.
func TestMapContentByteCellMaps(t *testing.T) {
	m := safepresentation.MapContent([]byte("a\x1bb"))
	if m.Text != "a^[b" {
		t.Fatalf("Text = %q, want %q", m.Text, "a^[b")
	}
	if len(m.Cells) != 4 {
		t.Fatalf("Cells = %d, want 4", len(m.Cells))
	}
	want := []struct {
		text       string
		start, end int
	}{
		{"a", 0, 1},
		{"^", 1, 2},
		{"[", 1, 2},
		{"b", 2, 3},
	}
	for i, c := range m.Cells {
		if c.Text != want[i].text || c.Start != want[i].start || c.End != want[i].end {
			t.Fatalf("cell %d = %+v, want text %q over [%d,%d)", i, c, want[i].text, want[i].start, want[i].end)
		}
	}

	m = safepresentation.MapContent([]byte("x\xc2\x85y"))
	if len(m.Cells) != 8 {
		t.Fatalf("C1 line Cells = %d, want 8 (x + six-cell escape + y)", len(m.Cells))
	}
	for i := 1; i <= 6; i++ {
		if m.Cells[i].Start != 1 || m.Cells[i].End != 3 {
			t.Fatalf("escape cell %d maps to [%d,%d), want [1,3)", i, m.Cells[i].Start, m.Cells[i].End)
		}
	}

	m = safepresentation.MapContent([]byte("a\xffb"))
	if len(m.Cells) != 3 || m.Cells[1].Text != "" || m.Cells[1].Start != 1 || m.Cells[1].End != 2 {
		t.Fatalf("invalid-byte cells = %+v, want at cell 1 over [1,2)", m.Cells)
	}
}

// A wide glyph occupies two cells that both map back to its source
// bytes; only the first cell carries the display text.
func TestMapContentWideGlyph(t *testing.T) {
	m := safepresentation.MapContent([]byte("a文b"))
	if m.Text != "a文b" {
		t.Fatalf("Text = %q, want %q", m.Text, "a文b")
	}
	if len(m.Cells) != 4 {
		t.Fatalf("Cells = %d, want 4", len(m.Cells))
	}
	if m.Cells[1].Text != "文" || m.Cells[2].Text != "" {
		t.Fatalf("wide glyph cells = %q, %q, want %q, %q", m.Cells[1].Text, m.Cells[2].Text, "文", "")
	}
	for _, i := range []int{1, 2} {
		if m.Cells[i].Start != 1 || m.Cells[i].End != 4 {
			t.Fatalf("wide cell %d maps to [%d,%d), want [1,4)", i, m.Cells[i].Start, m.Cells[i].End)
		}
	}
}

// CellsCovering maps a source byte range to the cell range covering
// every cell those bytes produced: a match covering the ESC byte
// highlights both ^ and [.
func TestCellsCovering(t *testing.T) {
	m := safepresentation.MapContent([]byte("a\x1bb"))
	cases := []struct {
		name       string
		start, end int
		lo, hi     int
		ok         bool
	}{
		{"esc byte", 1, 2, 1, 3, true},
		{"first byte", 0, 1, 0, 1, true},
		{"whole line", 0, 3, 0, 4, true},
		{"span ending inside escape", 0, 2, 0, 3, true},
		{"span starting inside escape", 1, 3, 1, 4, true},
		{"past end", 3, 4, 0, 0, false},
		{"empty range", 2, 2, 0, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lo, hi, ok := m.CellsCovering(tc.start, tc.end)
			if lo != tc.lo || hi != tc.hi || ok != tc.ok {
				t.Fatalf("CellsCovering(%d,%d) = (%d,%d,%v), want (%d,%d,%v)",
					tc.start, tc.end, lo, hi, ok, tc.lo, tc.hi, tc.ok)
			}
		})
	}
}

// A match over part of a C1 escape's source bytes covers every escape
// cell; a match over one invalid byte covers its U+FFFD cell.
func TestCellsCoveringEscapedForms(t *testing.T) {
	m := safepresentation.MapContent([]byte("x\xc2\x85y"))
	lo, hi, ok := m.CellsCovering(2, 3) // second byte of the C1 encoding
	if !ok || lo != 1 || hi != 7 {
		t.Fatalf("CellsCovering(2,3) = (%d,%d,%v), want (1,7,true)", lo, hi, ok)
	}

	m = safepresentation.MapContent([]byte("a\xffb"))
	lo, hi, ok = m.CellsCovering(1, 2)
	if !ok || lo != 1 || hi != 2 {
		t.Fatalf("CellsCovering(1,2) = (%d,%d,%v), want (1,2,true)", lo, hi, ok)
	}
}
