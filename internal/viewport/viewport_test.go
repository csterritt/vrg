package viewport

import (
	"testing"
)

// countingRows is the prepared-rows fake: it records every Row query so
// tests can prove a frame touches only the visible range rather than
// scanning the full buffer.
type countingRows struct {
	n     int
	calls []int
}

func (r *countingRows) Len() int { return r.n }

func (r *countingRows) Row(i int) Row {
	r.calls = append(r.calls, i)
	return Row{Line: i}
}

// RowOf is the unwrapped target mapping: a display target's rendered
// row is its source line.
func (r *countingRows) RowOf(t Target) int { return t.Line }

func rowsN(n int) *countingRows { return &countingRows{n: n} }

// Each scroll unit moves the top row by its contracted amount over a
// file longer than the viewport: up/down one rendered row, u/d
// max(1, floor(height/2)), page up/down the full content height.
func TestScrollUnits(t *testing.T) {
	for _, tc := range []struct {
		name   string
		height int
		start  int
		act    func(*Viewport)
		want   int
	}{
		{"down moves one rendered row", 4, 0, func(v *Viewport) { v.Down() }, 1},
		{"up moves one rendered row", 4, 5, func(v *Viewport) { v.Up() }, 4},
		{"d moves half a page", 4, 0, func(v *Viewport) { v.HalfDown() }, 2},
		{"u moves half a page", 4, 9, func(v *Viewport) { v.HalfUp() }, 7},
		{"half of odd height floors", 5, 0, func(v *Viewport) { v.HalfDown() }, 2},
		{"half of odd height up floors", 5, 9, func(v *Viewport) { v.HalfUp() }, 7},
		{"half of height one is one row", 1, 0, func(v *Viewport) { v.HalfDown() }, 1},
		{"pgdown moves a full page", 4, 0, func(v *Viewport) { v.PageDown() }, 4},
		{"pgup moves a full page", 4, 9, func(v *Viewport) { v.PageUp() }, 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var v Viewport
			v.Resize(10, tc.height)
			v.SetRows(rowsN(100))
			v.SetTop(tc.start)
			tc.act(&v)
			if v.Top() != tc.want {
				t.Fatalf("top = %d, want %d", v.Top(), tc.want)
			}
		})
	}
}

// A mixed scroll sequence lands on the accumulated offsets.
func TestScrollSequence(t *testing.T) {
	var v Viewport
	v.Resize(10, 4)
	v.SetRows(rowsN(100))
	for _, step := range []struct {
		act  func(*Viewport)
		want int
	}{
		{func(v *Viewport) { v.PageDown() }, 4},
		{func(v *Viewport) { v.PageDown() }, 8},
		{func(v *Viewport) { v.Down() }, 9},
		{func(v *Viewport) { v.HalfUp() }, 7},
		{func(v *Viewport) { v.PageUp() }, 3},
		{func(v *Viewport) { v.Up() }, 2},
	} {
		step.act(&v)
		if v.Top() != step.want {
			t.Fatalf("top = %d, want %d", v.Top(), step.want)
		}
	}
}

// Scrolling up at the top of the file does nothing under any unit.
func TestClampAtBOF(t *testing.T) {
	for _, tc := range []struct {
		name string
		act  func(*Viewport)
	}{
		{"up", func(v *Viewport) { v.Up() }},
		{"u", func(v *Viewport) { v.HalfUp() }},
		{"pgup", func(v *Viewport) { v.PageUp() }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var v Viewport
			v.Resize(10, 4)
			v.SetRows(rowsN(100))
			tc.act(&v)
			if v.Top() != 0 {
				t.Fatalf("top = %d, want 0", v.Top())
			}
		})
	}
}

// Scrolling down past EOF stops at the last full page: the file's final
// row sits on the bottom row and no avoidable blank rows appear.
func TestClampAtEOF(t *testing.T) {
	var v Viewport
	v.Resize(10, 4)
	v.SetRows(rowsN(10))
	v.PageDown()
	if v.Top() != 4 {
		t.Fatalf("top = %d, want 4", v.Top())
	}
	v.PageDown()
	if v.Top() != 6 {
		t.Fatalf("clamped top = %d, want 6", v.Top())
	}
	v.Down()
	if v.Top() != 6 {
		t.Fatalf("top past EOF = %d, want 6", v.Top())
	}
	vis := v.Visible()
	if len(vis) != 4 || vis[0].Line != 6 || vis[3].Line != 9 {
		t.Fatalf("visible = %+v, want rows 6..9", vis)
	}
}

// A file shorter than the viewport pins the top row at 0 under every
// scroll key and leaves the unused rows blank naturally.
func TestFileShorterThanViewport(t *testing.T) {
	var v Viewport
	v.Resize(10, 5)
	v.SetRows(rowsN(3))
	for _, act := range []func(*Viewport){
		(*Viewport).Down, (*Viewport).HalfDown, (*Viewport).PageDown,
	} {
		act(&v)
		if v.Top() != 0 {
			t.Fatalf("short file top = %d, want 0", v.Top())
		}
	}
	if vis := v.Visible(); len(vis) != 3 {
		t.Fatalf("short file visible = %d rows, want 3", len(vis))
	}
}

// A file exactly the viewport's height is already fully visible: every
// scroll key is a no-op.
func TestFileEqualToViewport(t *testing.T) {
	var v Viewport
	v.Resize(10, 4)
	v.SetRows(rowsN(4))
	for _, act := range []func(*Viewport){
		(*Viewport).Down, (*Viewport).HalfDown, (*Viewport).PageDown,
	} {
		act(&v)
		if v.Top() != 0 {
			t.Fatalf("exact-fit top = %d, want 0", v.Top())
		}
	}
	if vis := v.Visible(); len(vis) != 4 {
		t.Fatalf("exact-fit visible = %d rows, want 4", len(vis))
	}
}

// Unavailable content — no rows prepared — gives safe empty queries and
// scroll no-ops, never fictitious lines.
func TestEmptyContentIsInert(t *testing.T) {
	var v Viewport
	v.Resize(10, 4)
	v.Down()
	v.PageDown()
	if v.Top() != 0 || v.Visible() != nil {
		t.Fatalf("empty viewport: top=%d visible=%v", v.Top(), v.Visible())
	}
	v.SetRows(rowsN(0))
	v.Down()
	if v.Top() != 0 || v.Visible() != nil {
		t.Fatalf("zero-row viewport: top=%d visible=%v", v.Top(), v.Visible())
	}
}

// A frame queries the row provider only for the visible range.
func TestVisibleQueriesOnlyVisibleRows(t *testing.T) {
	var v Viewport
	v.Resize(10, 4)
	rows := rowsN(100)
	v.SetRows(rows)
	v.SetTop(8)
	rows.calls = nil
	vis := v.Visible()
	want := []int{8, 9, 10, 11}
	if len(vis) != 4 {
		t.Fatalf("visible = %d rows, want 4", len(vis))
	}
	if len(rows.calls) != len(want) {
		t.Fatalf("provider calls = %v, want %v", rows.calls, want)
	}
	for i, c := range rows.calls {
		if c != want[i] {
			t.Fatalf("provider calls = %v, want %v", rows.calls, want)
		}
	}
	for i, r := range vis {
		if r.Line != want[i] {
			t.Fatalf("visible row %d has Line %d, want %d", i, r.Line, want[i])
		}
	}
}

// Near EOF the provider sees only the remaining rows.
func TestVisibleNearEOFQueriesRemainder(t *testing.T) {
	var v Viewport
	v.Resize(10, 4)
	rows := rowsN(10)
	v.SetRows(rows)
	v.SetTop(7) // clamps to 6
	rows.calls = nil
	vis := v.Visible()
	want := []int{6, 7, 8, 9}
	if len(rows.calls) != len(want) {
		t.Fatalf("provider calls = %v, want %v", rows.calls, want)
	}
	for i := range want {
		if rows.calls[i] != want[i] || vis[i].Line != want[i] {
			t.Fatalf("call/row %d = %d/%d, want %d", i, rows.calls[i], vis[i].Line, want[i])
		}
	}
}

// A taller viewport re-clamps a top that would leave avoidable blank
// rows below EOF — the documented lossy EOF clamp.
func TestResizeReclampsTop(t *testing.T) {
	var v Viewport
	v.Resize(10, 4)
	v.SetRows(rowsN(12))
	v.SetTop(8)
	v.Resize(10, 10)
	if v.Top() != 2 {
		t.Fatalf("top after growth = %d, want 2", v.Top())
	}
	v.Resize(40, 10)
	if v.Top() != 2 {
		t.Fatalf("width-only resize moved top to %d", v.Top())
	}
}

// Swapping prepared rows preserves the top row clamped to the new
// content; the loss is permanent per the EOF-clamp rule.
func TestSetRowsClampsToNewContent(t *testing.T) {
	var v Viewport
	v.Resize(10, 5)
	v.SetRows(rowsN(100))
	v.SetTop(40)
	v.SetRows(rowsN(3))
	if v.Top() != 0 {
		t.Fatalf("top after shrinking content = %d, want 0", v.Top())
	}
	v.SetRows(rowsN(100))
	if v.Top() != 0 {
		t.Fatalf("clamped top restored to %d, want 0", v.Top())
	}
}
