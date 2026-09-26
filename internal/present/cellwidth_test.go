package present

import "testing"

// The shared helper is the one display-width authority: cell widths
// measure by grapheme cluster under the grapheme policy, with escape
// and control sequences occupying no cells.
func TestCellWidthSharedGraphemePolicy(t *testing.T) {
	for _, tc := range []struct {
		name string
		s    string
		want int
	}{
		{"empty", "", 0},
		{"ascii", "a.txt", 5},
		{"two-cell cjk", "世界", 4},
		{"base plus combining mark is one cell", "é", 1},
		{"zwj emoji is one two-cell cluster", "👨‍👩‍👧", 2},
		{"escape sequences paint no cells", "\x1b[31m世界\x1b[0m", 4},
		{"caret escapes count their cells", "^A\\u0085", 8},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := CellWidth(tc.s); got != tc.want {
				t.Fatalf("CellWidth(%q) = %d, want %d", tc.s, got, tc.want)
			}
		})
	}
}

// A path wider than its allotted cells is left-truncated with a
// leading "…", never splitting a grapheme cluster and never exceeding
// the budget — a wide cluster straddling the cut is dropped whole.
// Styled input keeps its escape sequences: they paint no cells and the
// kept text's styling state survives the cut.
func TestTruncateLeftSharedGraphemePolicy(t *testing.T) {
	for _, tc := range []struct {
		name string
		s    string
		n    int
		want string
	}{
		{"fits", "a.txt", 10, "a.txt"},
		{"exact fit", "abc", 3, "abc"},
		{"one cell over", "abcd", 3, "…cd"},
		{"basename kept", "dir/sub/file.txt", 9, "…file.txt"},
		{"one cell budget", "abcdef", 1, "…"},
		{"zero budget", "abc", 0, ""},
		{"negative budget", "abc", -2, ""},
		{"wide cluster straddles cut", "xx世界", 4, "…界"},
		{"combining cluster survives", "aab" + "é" + "cd", 4, "…" + "é" + "cd"},
		{"combining cluster dropped whole", "x" + "é" + "yz", 3, "…yz"},
		{"combining cluster kept at the cut", "qqézz", 4, "…ézz"},
		{"zwj emoji never split", "a👨‍👩‍👧b", 3, "…b"},
		{"styled text keeps its sequences", "\x1b[31mxx世界\x1b[0m", 4, "…\x1b[31m界\x1b[0m"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := TruncateLeft(tc.s, tc.n)
			if got != tc.want {
				t.Fatalf("TruncateLeft(%q, %d) = %q, want %q", tc.s, tc.n, got, tc.want)
			}
			if w := CellWidth(got); w > max(0, tc.n) {
				t.Fatalf("TruncateLeft(%q, %d) = %q is %d cells — over budget", tc.s, tc.n, got, w)
			}
		})
	}
}

// The right cut, the window cut, and word wrapping share the same
// ANSI-aware grapheme policy as the left cut: clusters are never split
// and sequences are preserved.
func TestTruncateCutAndWrapShareThePolicy(t *testing.T) {
	for _, tc := range []struct {
		name string
		got  string
		want string
	}{
		{"truncate tail inside budget", Truncate("hello", 3, "…"), "he…"},
		{"truncate drops a straddling cluster", Truncate("a世界b", 4, ""), "a世"},
		{"truncate shorter than budget", Truncate("abc", 10, "…"), "abc"},
		{"window cut keeps whole clusters", Cut("a世界b", 1, 4), "世"},
		{"wrap breaks on width", Wrap("hello world", 5, ""), "hello\nworld"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Fatalf("got %q, want %q", tc.got, tc.want)
			}
		})
	}
}
