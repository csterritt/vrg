package viewport

import (
	"strings"
	"testing"

	"vrg/internal/present"
)

// hid installs a one-line run-off-edge row model w cells wide, pans to
// off, and returns the visible row — the carrier of the Issue #20
// hidden-content indicator flags a frame turns into the gutter
// "_"/"*" and the reserved column's "*".
func hid(t *testing.T, w, off int, text string, spans []present.Span) Row {
	t.Helper()
	v := &Viewport{}
	v.Resize(w, 4)
	v.SetRows(Prepare(source(t, text, map[int][]present.Span{0: spans}), Key{Width: w}))
	v.SetOffset(off)
	return v.Visible()[0]
}

// Every visible source line's first trailing gutter space signposts
// text hidden left of the window with "_": any of the line's cells
// before the offset counts — including a line fully left of the
// window — while an empty line has no text to hide and offset zero
// hides nothing.
func TestHiddenLeftTextFlag(t *testing.T) {
	for _, tc := range []struct {
		name string
		off  int
		text string
		want bool
	}{
		{"offset zero hides nothing", 0, "abcdef\n", false},
		{"offset hides the head", 3, "abcdef\n", true},
		{"offset at the last cell still hides", 5, "abcdef\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if r := hid(t, 10, tc.off, tc.text, nil); r.HiddenLeft != tc.want {
				t.Fatalf("HiddenLeft = %v, want %v", r.HiddenLeft, tc.want)
			}
		})
	}

	// A 40-cell sibling permits offset 10: a 5-cell line is then
	// fully left of the window and still signposts, while an empty
	// line has no hidden text and stays blank.
	src := source(t, "abcde\n\n"+strings.Repeat("x", 40)+"\n", nil)
	var v Viewport
	v.Resize(10, 4)
	v.SetRows(Prepare(src, Key{Width: 10}))
	v.SetOffset(10)
	vis := v.Visible()
	if !vis[0].HiddenLeft {
		t.Fatal("line fully left of the window: HiddenLeft = false, want true")
	}
	if vis[1].HiddenLeft {
		t.Fatal("empty line: HiddenLeft = true, want false — no text to hide")
	}
	if !vis[2].HiddenLeft {
		t.Fatal("long line: HiddenLeft = false, want true")
	}
}

// A match or marker entirely hidden left of the window upgrades the
// line's gutter signpost to "*"; a partially painted match counts as
// visible, and a marker is one painted cell wherever it sits.
func TestMatchHiddenLeftFlag(t *testing.T) {
	// "hit" covers cells 5–7 of a 40-cell line.
	text := strings.Repeat("x", 5) + "hit" + strings.Repeat("x", 32) + "\n"
	for _, tc := range []struct {
		name  string
		off   int
		spans []present.Span
		want  bool
	}{
		{"match fully painted at offset zero", 0,
			[]present.Span{{Start: 5, End: 8}}, false},
		{"match half hidden is visible", 7,
			[]present.Span{{Start: 5, End: 8}}, false},
		{"match flush with the window edge is visible", 5,
			[]present.Span{{Start: 5, End: 8}}, false},
		{"match entirely hidden left", 8,
			[]present.Span{{Start: 5, End: 8}}, true},
		{"marker hidden left", 8,
			[]present.Span{{Start: 5, End: 5}}, true},
		{"marker at the window's first cell is visible", 8,
			[]present.Span{{Start: 8, End: 8}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if r := hid(t, 10, tc.off, text, tc.spans); r.MatchHiddenLeft != tc.want {
				t.Fatalf("MatchHiddenLeft = %v, want %v", r.MatchHiddenLeft, tc.want)
			}
		})
	}
}

// A match or marker entirely hidden right of the window sets the flag
// the reserved column paints on the current matched line; partial
// visibility counts as visible.
func TestMatchHiddenRightFlag(t *testing.T) {
	text := strings.Repeat("x", 60) + "\n"
	for _, tc := range []struct {
		name  string
		w     int
		spans []present.Span
		want  bool
	}{
		{"match entirely hidden right", 10,
			[]present.Span{{Start: 20, End: 23}}, true},
		{"match half hidden right is visible", 10,
			[]present.Span{{Start: 8, End: 12}}, false},
		{"match ending on the last text cell is visible", 10,
			[]present.Span{{Start: 7, End: 10}}, false},
		{"marker hidden right", 10,
			[]present.Span{{Start: 15, End: 15}}, true},
		{"marker on the first column past the window", 10,
			[]present.Span{{Start: 10, End: 10}}, true},
		{"marker inside the window is visible", 10,
			[]present.Span{{Start: 9, End: 9}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if r := hid(t, tc.w, 0, text, tc.spans); r.MatchHiddenRight != tc.want {
				t.Fatalf("MatchHiddenRight = %v, want %v", r.MatchHiddenRight, tc.want)
			}
		})
	}
}

// The two flags are independent: a line can hide a match left and
// another right at once, letting both stars appear together.
func TestHiddenBothSidesFlags(t *testing.T) {
	text := "hit" + strings.Repeat("x", 50) + "far" + strings.Repeat("x", 10) + "\n"
	r := hid(t, 10, 10, text, []present.Span{{Start: 0, End: 3}, {Start: 53, End: 56}})
	if !r.HiddenLeft || !r.MatchHiddenLeft || !r.MatchHiddenRight {
		t.Fatalf("flags = {left %v, matchL %v, matchR %v}, want all true",
			r.HiddenLeft, r.MatchHiddenLeft, r.MatchHiddenRight)
	}
}

// A grapheme cluster split by a clip edge renders its in-window cells
// blank, so a match on it has no painted cell: it counts as entirely
// hidden — right when the cluster straddles the right edge, left when
// it straddles the left edge — and becomes visible again only when the
// whole cluster paints.
func TestSplitClusterBlankCountsAsHidden(t *testing.T) {
	// 世 covers cells 4–5; the match covers the whole cluster.
	text := "xxxx世yyyy\n"
	spans := []present.Span{{Start: 4, End: 6}}

	r := hid(t, 5, 0, text, spans)
	if !r.MatchHiddenRight || r.MatchHiddenLeft {
		t.Fatalf("right-edge split: flags matchL=%v matchR=%v, want hidden right only",
			r.MatchHiddenLeft, r.MatchHiddenRight)
	}
	if got := rowText(r); got != "xxxx " {
		t.Fatalf("right-edge split row = %q, want the split cell blank", got)
	}

	r = hid(t, 5, 5, text, spans)
	if !r.MatchHiddenLeft || r.MatchHiddenRight {
		t.Fatalf("left-edge split: flags matchL=%v matchR=%v, want hidden left only",
			r.MatchHiddenLeft, r.MatchHiddenRight)
	}

	// Offsets that paint the whole cluster make the match visible.
	for _, tc := range []struct{ w, off int }{{5, 4}, {6, 0}} {
		if r = hid(t, tc.w, tc.off, text, spans); r.MatchHiddenLeft || r.MatchHiddenRight {
			t.Fatalf("painted cluster at w=%d off=%d: flags %v/%v, want visible",
				tc.w, tc.off, r.MatchHiddenLeft, r.MatchHiddenRight)
		}
	}
}

// A cluster wider than the whole text area renders the window as
// clipping blanks at every offset: a match on it is never visible.
// With the offset at its start the cluster's remainder is hidden
// right — the text hidden left signposts "_" only.
func TestUnpaintableClusterCountsNotVisible(t *testing.T) {
	// "ab" + a six-cell -escape cluster at cells 2–7 + "cd".
	r := hid(t, 4, 2, "ab\xc2\x85cd\n", []present.Span{{Start: 2, End: 8}})
	if got := rowText(r); got != "    " {
		t.Fatalf("row = %q, want four clipping blanks", got)
	}
	if r.MatchHiddenLeft || !r.MatchHiddenRight {
		t.Fatalf("flags matchL=%v matchR=%v, want hidden right only",
			r.MatchHiddenLeft, r.MatchHiddenRight)
	}
	if !r.HiddenLeft {
		t.Fatal("HiddenLeft = false, want true — the 'ab' cells are hidden left")
	}
}

// Wrap mode has no indicators: a wrap model's rows carry no flags even
// with spans recorded, so the frame draws neither the gutter marks nor
// a reserved column.
func TestWrapModeNoIndicatorFlags(t *testing.T) {
	src := source(t, "hit"+strings.Repeat("x", 50)+"\n",
		map[int][]present.Span{0: {{Start: 0, End: 3}}})
	var v Viewport
	v.Resize(10, 4)
	v.SetRows(Prepare(src, Key{Width: 10, Wrap: true}))
	for i, r := range v.Visible() {
		if r.HiddenLeft || r.MatchHiddenLeft || r.MatchHiddenRight {
			t.Fatalf("wrap row %d carries flags %v/%v/%v, want none",
				i, r.HiddenLeft, r.MatchHiddenLeft, r.MatchHiddenRight)
		}
	}
}
