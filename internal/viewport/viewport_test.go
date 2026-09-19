package viewport_test

import (
	"os"
	"path/filepath"
	"testing"

	"vrg/internal/filebuffer"
	"vrg/internal/viewport"
)

// loadBuffer loads content into a real buffer through the public
// filebuffer path.
func loadBuffer(t *testing.T, content string) *filebuffer.Buffer {
	t.Helper()
	p := filepath.Join(t.TempDir(), "f.txt")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	buf, err := filebuffer.Load([]byte(p), nil)
	if err != nil {
		t.Fatal(err)
	}
	return buf
}

// Each scroll unit moves the top row by its rendered-row count on a file
// longer than the viewport: one row for up/down, half a page for u/d,
// a full page for pgup/pgdown — and back again symmetrically.
func TestScrollUnitsMoveRenderedRows(t *testing.T) {
	const rows, height = 100, 10
	units := []struct {
		name string
		down int
	}{
		{"one row", 1},
		{"half page", viewport.HalfPage(height)},
		{"full page", height},
	}
	for _, u := range units {
		var v viewport.Viewport
		v.Scroll(u.down, rows, height)
		if v.Top() != u.down {
			t.Fatalf("%s: after scrolling down top = %d, want %d", u.name, v.Top(), u.down)
		}
		v.Scroll(-u.down, rows, height)
		if v.Top() != 0 {
			t.Fatalf("%s: after scrolling back up top = %d, want 0", u.name, v.Top())
		}
	}
}

// The half-page unit is max(1, floor(height / 2)) — including odd
// heights and degenerate ones.
func TestHalfPageUnit(t *testing.T) {
	cases := []struct{ height, want int }{
		{0, 1}, {1, 1}, {2, 1}, {3, 1}, {4, 2}, {5, 2},
		{7, 3}, {8, 4}, {9, 4}, {23, 11},
	}
	for _, c := range cases {
		if got := viewport.HalfPage(c.height); got != c.want {
			t.Fatalf("HalfPage(%d) = %d, want %d", c.height, got, c.want)
		}
	}
}

// The top row clamps to valid content for files shorter than, equal to,
// and longer than the viewport: never below 0 and never past the last
// top that leaves no avoidable blank rows below EOF.
func TestScrollClampByFileLength(t *testing.T) {
	cases := []struct {
		name         string
		rows, height int
		maxTop       int
	}{
		{"empty file", 0, 10, 0},
		{"shorter than viewport", 5, 10, 0},
		{"equal to viewport", 10, 10, 0},
		{"one row over", 11, 10, 1},
		{"longer than viewport", 100, 10, 90},
		{"no content height", 100, 0, 0},
	}
	for _, c := range cases {
		if got := viewport.MaxTop(c.rows, c.height); got != c.maxTop {
			t.Fatalf("%s: MaxTop(%d, %d) = %d, want %d", c.name, c.rows, c.height, got, c.maxTop)
		}
		var v viewport.Viewport
		v.Scroll(1000, c.rows, c.height)
		if v.Top() != c.maxTop {
			t.Fatalf("%s: scrolling past EOF gave top = %d, want %d", c.name, v.Top(), c.maxTop)
		}
		v.Scroll(1000, c.rows, c.height)
		if v.Top() != c.maxTop {
			t.Fatalf("%s: a second scroll at EOF moved top to %d, want it to stay %d", c.name, v.Top(), c.maxTop)
		}
	}
}

// Scrolling up never moves the top row below 0.
func TestScrollClampBOF(t *testing.T) {
	var v viewport.Viewport
	for _, d := range []int{-1, -viewport.HalfPage(10), -10} {
		v.Scroll(d, 100, 10)
		if v.Top() != 0 {
			t.Fatalf("scroll %d at BOF gave top = %d, want 0", d, v.Top())
		}
	}
	v.Scroll(50, 100, 10)
	v.Scroll(-1000, 100, 10)
	if v.Top() != 0 {
		t.Fatalf("scrolling past BOF gave top = %d, want 0", v.Top())
	}
}

// When the content height or the row count shrinks beneath the saved
// top, Clamp pulls it up to the last position leaving no avoidable
// blank rows below EOF — the documented lossy EOF clamp.
func TestClampPullsTopUp(t *testing.T) {
	var v viewport.Viewport
	v.Scroll(90, 100, 10)
	if v.Top() != 90 {
		t.Fatalf("top = %d, want 90", v.Top())
	}
	// The viewport grows to 50 rows: tops past 50 would leave blanks.
	v.Clamp(100, 50)
	if v.Top() != 50 {
		t.Fatalf("after growing the height top = %d, want 50", v.Top())
	}
	// The content shrinks to 30 rows: every position past 0 leaves
	// blanks, so the top collapses to the top of the file.
	v.Clamp(30, 50)
	if v.Top() != 0 {
		t.Fatalf("after shrinking the content top = %d, want 0", v.Top())
	}
}

// Prepare builds the rendered-row model a frame render slices: in the
// unwrapped panel each source line is exactly one rendered row, in file
// order, with the buffer's gutter width. A nil buffer prepares an empty
// model whose queries are safe.
func TestPrepareRows(t *testing.T) {
	buf := loadBuffer(t, "one\ntwo\nthree\n")
	rows := viewport.Prepare(buf)
	if rows.Len() != 3 {
		t.Fatalf("Len = %d, want 3", rows.Len())
	}
	for i := 0; i < rows.Len(); i++ {
		if got := rows.At(i).Number; got != int64(i+1) {
			t.Fatalf("At(%d).Number = %d, want %d", i, got, i+1)
		}
	}
	if rows.GutterWidth() != buf.GutterWidth() {
		t.Fatalf("GutterWidth = %d, want the buffer's %d", rows.GutterWidth(), buf.GutterWidth())
	}

	empty := viewport.Prepare(nil)
	if empty.Len() != 0 {
		t.Fatalf("empty model Len = %d, want 0", empty.Len())
	}
}
