package searchindex_test

import (
	"bytes"
	"testing"

	"vrg/internal/searchindex"
)

// buildCursor builds an index from the given records and returns a
// cursor over it. The records are auto-completed with begin/end/summary
// via build.
func buildCursor(t *testing.T, workdir string, records ...string) *searchindex.Cursor {
	t.Helper()
	idx := build(t, workdir, records...)
	return searchindex.NewCursor(idx)
}

// cursorStop returns the current stop from a cursor, failing if the
// cursor has no selection.
func cursorStop(t *testing.T, c *searchindex.Cursor) searchindex.Stop {
	t.Helper()
	s, ok := c.Stop()
	if !ok {
		t.Fatalf("Cursor.Stop() returned ok=false, want a selection")
	}
	return s
}

// assertCursorStop requires the cursor's current stop to match the
// given raw path and line number.
func assertCursorStop(t *testing.T, c *searchindex.Cursor, path string, line int) {
	t.Helper()
	s := cursorStop(t, c)
	if string(s.RawPath) != path {
		t.Fatalf("Cursor stop RawPath = %q, want %q", s.RawPath, path)
	}
	if s.LineNumber != line {
		t.Fatalf("Cursor stop LineNumber = %d, want %d", s.LineNumber, line)
	}
}

// TestCursorStartupSelectsFirstStop verifies that a new cursor selects
// the first stop in path-then-line order.
func TestCursorStartupSelectsFirstStop(t *testing.T) {
	c := buildCursor(t, "/work",
		textMatch("src/b.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/a.go", "x\n", 3, subSpec{"x", 0, 1}),
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	assertCursorStop(t, c, "src/a.go", 1)
}

// TestCursorPosition verifies that Position reports the 0-based stop
// index, starting at 0 for the first stop.
func TestCursorPosition(t *testing.T) {
	c := buildCursor(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	if c.Position() != 0 {
		t.Fatalf("Position = %d, want 0 (first stop)", c.Position())
	}
}

// TestCursorNextAdvances verifies that Next advances the cursor to the
// next stop in path-then-line order.
func TestCursorNextAdvances(t *testing.T) {
	c := buildCursor(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/a.go", "x\n", 5, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	_, moved, _ := c.Next()
	if !moved {
		t.Fatalf("Next moved = false, want true")
	}
	assertCursorStop(t, c, "src/a.go", 5)
	if c.Position() != 1 {
		t.Fatalf("Position = %d, want 1", c.Position())
	}
}

// TestCursorPrevRetreats verifies that Prev retreats the cursor to the
// previous stop in path-then-line order.
func TestCursorPrevRetreats(t *testing.T) {
	c := buildCursor(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/a.go", "x\n", 5, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	// Advance to the second stop first.
	_, _, _ = c.Next()
	_, moved, _ := c.Prev()
	if !moved {
		t.Fatalf("Prev moved = false, want true")
	}
	assertCursorStop(t, c, "src/a.go", 1)
}

// TestCursorNextWraps verifies that Next wraps from the last stop back
// to the first stop circularly.
func TestCursorNextWraps(t *testing.T) {
	c := buildCursor(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	// Advance to the last stop.
	_, _, _ = c.Next()
	assertCursorStop(t, c, "src/b.go", 1)
	// Wrap from last to first.
	_, moved, _ := c.Next()
	if !moved {
		t.Fatalf("Next at last stop moved = false, want true (wrap)")
	}
	assertCursorStop(t, c, "src/a.go", 1)
	if c.Position() != 0 {
		t.Fatalf("Position = %d, want 0 (wrapped to first)", c.Position())
	}
}

// TestCursorPrevWraps verifies that Prev wraps from the first stop back
// to the last stop circularly.
func TestCursorPrevWraps(t *testing.T) {
	c := buildCursor(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	// At the first stop; Prev should wrap to the last.
	_, moved, _ := c.Prev()
	if !moved {
		t.Fatalf("Prev at first stop moved = false, want true (wrap)")
	}
	assertCursorStop(t, c, "src/b.go", 1)
	if c.Position() != 1 {
		t.Fatalf("Position = %d, want 1 (wrapped to last)", c.Position())
	}
}

// TestCursorFileChangeFlag verifies that the fileChanged flag is true
// when navigation crosses a file boundary and false within the same
// file.
func TestCursorFileChangeFlag(t *testing.T) {
	c := buildCursor(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/a.go", "x\n", 5, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	// Same-file advance: a.go:1 → a.go:5
	_, moved, fileChanged := c.Next()
	if !moved {
		t.Fatalf("Next moved = false, want true")
	}
	if fileChanged {
		t.Fatalf("Next fileChanged = true, want false (same file)")
	}
	// Cross-file advance: a.go:5 → b.go:1
	_, moved, fileChanged = c.Next()
	if !moved {
		t.Fatalf("Next moved = false, want true")
	}
	if !fileChanged {
		t.Fatalf("Next fileChanged = false, want true (cross-file)")
	}
	// Wrap cross-file: b.go:1 → a.go:1
	_, moved, fileChanged = c.Next()
	if !moved {
		t.Fatalf("Next moved = false, want true (wrap)")
	}
	if !fileChanged {
		t.Fatalf("Next fileChanged = false, want true (wrap cross-file)")
	}
}

// TestCursorPrevFileChangeFlag verifies the fileChanged flag on Prev
// navigation, including wrap.
func TestCursorPrevFileChangeFlag(t *testing.T) {
	c := buildCursor(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "x\n", 3, subSpec{"x", 0, 1}),
	)
	// Wrap cross-file: a.go:1 → b.go:3 (last stop)
	_, moved, fileChanged := c.Prev()
	if !moved {
		t.Fatalf("Prev moved = false, want true (wrap)")
	}
	if !fileChanged {
		t.Fatalf("Prev fileChanged = false, want true (wrap cross-file)")
	}
	assertCursorStop(t, c, "src/b.go", 3)
	// Same-file retreat: b.go:3 → b.go:1
	_, moved, fileChanged = c.Prev()
	if !moved {
		t.Fatalf("Prev moved = false, want true")
	}
	if fileChanged {
		t.Fatalf("Prev fileChanged = true, want false (same file)")
	}
	assertCursorStop(t, c, "src/b.go", 1)
}

// TestCursorSingleStopNoOp verifies that Next and Prev are strict
// no-ops when the index has exactly one stop: the cursor does not move,
// no file change is reported, and the stop remains the same.
func TestCursorSingleStopNoOp(t *testing.T) {
	c := buildCursor(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	// Next is a no-op.
	stop, moved, fileChanged := c.Next()
	if moved {
		t.Fatalf("Next moved = true, want false (single stop no-op)")
	}
	if fileChanged {
		t.Fatalf("Next fileChanged = true, want false (single stop no-op)")
	}
	if string(stop.RawPath) != "src/a.go" || stop.LineNumber != 1 {
		t.Fatalf("Next stop = %q:%d, want src/a.go:1 (unchanged)", stop.RawPath, stop.LineNumber)
	}
	if c.Position() != 0 {
		t.Fatalf("Position = %d, want 0 (unchanged)", c.Position())
	}
	// Prev is also a no-op.
	stop, moved, fileChanged = c.Prev()
	if moved {
		t.Fatalf("Prev moved = true, want false (single stop no-op)")
	}
	if fileChanged {
		t.Fatalf("Prev fileChanged = true, want false (single stop no-op)")
	}
	if string(stop.RawPath) != "src/a.go" || stop.LineNumber != 1 {
		t.Fatalf("Prev stop = %q:%d, want src/a.go:1 (unchanged)", stop.RawPath, stop.LineNumber)
	}
}

// TestCursorEmptyNoOp verifies that Next and Prev are strict no-ops
// when the index has zero stops. Stop() returns ok=false.
func TestCursorEmptyNoOp(t *testing.T) {
	idx := build(t, "/work") // no records → empty index
	c := searchindex.NewCursor(idx)
	if c.Position() != -1 {
		t.Fatalf("Position = %d, want -1 (empty index)", c.Position())
	}
	_, ok := c.Stop()
	if ok {
		t.Fatalf("Stop() ok = true, want false (empty index)")
	}
	stop, moved, fileChanged := c.Next()
	if moved {
		t.Fatalf("Next moved = true, want false (empty no-op)")
	}
	if fileChanged {
		t.Fatalf("Next fileChanged = true, want false (empty no-op)")
	}
	if len(stop.RawPath) != 0 {
		t.Fatalf("Next stop RawPath = %q, want empty (empty index)", stop.RawPath)
	}
	stop, moved, fileChanged = c.Prev()
	if moved {
		t.Fatalf("Prev moved = true, want false (empty no-op)")
	}
	if fileChanged {
		t.Fatalf("Prev fileChanged = true, want false (empty no-op)")
	}
	if len(stop.RawPath) != 0 {
		t.Fatalf("Prev stop RawPath = %q, want empty (empty index)", stop.RawPath)
	}
}

// TestCursorMultipleSubmatchesOneStop verifies that multiple submatches
// on one line count as a single navigation stop: the cursor treats
// them as one position and Next/Prev move past the entire line.
func TestCursorMultipleSubmatchesOneStop(t *testing.T) {
	c := buildCursor(t, "/work",
		textMatch("src/a.go", "hello world\n", 1, subSpec{"hello", 0, 5}),
		textMatch("src/a.go", "hello world\n", 1, subSpec{"world", 6, 11}),
		textMatch("src/b.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	// The two submatches on a.go:1 merge into one stop, so the index
	// has exactly 2 stops: a.go:1 and b.go:1.
	if c.Position() != 0 {
		t.Fatalf("Position = %d, want 0", c.Position())
	}
	assertCursorStop(t, c, "src/a.go", 1)
	// Verify the stop has both submatches.
	s := cursorStop(t, c)
	if len(s.Submatches) != 2 {
		t.Fatalf("Submatches len = %d, want 2 (merged)", len(s.Submatches))
	}
	// Next advances to b.go:1 (the second stop, not a third).
	_, moved, _ := c.Next()
	if !moved {
		t.Fatalf("Next moved = false, want true")
	}
	assertCursorStop(t, c, "src/b.go", 1)
	if c.Position() != 1 {
		t.Fatalf("Position = %d, want 1", c.Position())
	}
}

// TestCursorLen verifies that Len reports the number of stops.
func TestCursorLen(t *testing.T) {
	c := buildCursor(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/a.go", "x\n", 5, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "x\n", 1, subSpec{"x", 0, 1}),
	)
	if c.Len() != 3 {
		t.Fatalf("Len = %d, want 3", c.Len())
	}
}

// TestCursorEmptyLen verifies that Len is 0 for an empty index.
func TestCursorEmptyLen(t *testing.T) {
	idx := build(t, "/work")
	c := searchindex.NewCursor(idx)
	if c.Len() != 0 {
		t.Fatalf("Len = %d, want 0 (empty)", c.Len())
	}
}

// TestCursorNilIndex verifies that a nil index produces an empty cursor
// that does not panic on any operation.
func TestCursorNilIndex(t *testing.T) {
	c := searchindex.NewCursor(nil)
	if c.Position() != -1 {
		t.Fatalf("Position = %d, want -1 (nil index)", c.Position())
	}
	if c.Len() != 0 {
		t.Fatalf("Len = %d, want 0 (nil index)", c.Len())
	}
	_, ok := c.Stop()
	if ok {
		t.Fatalf("Stop() ok = true, want false (nil index)")
	}
	_, moved, fileChanged := c.Next()
	if moved || fileChanged {
		t.Fatalf("Next on nil: moved=%v fileChanged=%v, want false/false", moved, fileChanged)
	}
	_, moved, fileChanged = c.Prev()
	if moved || fileChanged {
		t.Fatalf("Prev on nil: moved=%v fileChanged=%v, want false/false", moved, fileChanged)
	}
}

// TestCursorFullCycle verifies a full circular cycle of Next and Prev
// returns to the starting position.
func TestCursorFullCycleNext(t *testing.T) {
	c := buildCursor(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/a.go", "x\n", 3, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "x\n", 7, subSpec{"x", 0, 1}),
	)
	start := c.Position()
	for i := 0; i < c.Len(); i++ {
		c.Next()
	}
	if c.Position() != start {
		t.Fatalf("After full Next cycle, Position = %d, want %d", c.Position(), start)
	}
}

// TestCursorFullCyclePrev verifies a full circular cycle of Prev returns
// to the starting position.
func TestCursorFullCyclePrev(t *testing.T) {
	c := buildCursor(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/a.go", "x\n", 3, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "x\n", 1, subSpec{"x", 0, 1}),
		textMatch("src/b.go", "x\n", 7, subSpec{"x", 0, 1}),
	)
	start := c.Position()
	for i := 0; i < c.Len(); i++ {
		c.Prev()
	}
	if c.Position() != start {
		t.Fatalf("After full Prev cycle, Position = %d, want %d", c.Position(), start)
	}
}

// TestCursorFileChangeBytesEqual verifies that the fileChanged flag
// uses raw byte comparison, so identical paths in text and bytes
// encoding are the same file.
func TestCursorFileChangeBytesEqual(t *testing.T) {
	path := []byte("src/a.go")
	c := buildCursor(t, "/work",
		textMatch("src/a.go", "x\n", 1, subSpec{"x", 0, 1}),
		bytesMatch(path, []byte("x\n"), 5, subSpec{"x", 0, 1}),
	)
	// Both stops are the same file (src/a.go), so fileChanged should
	// be false even though they used different encodings.
	_, moved, fileChanged := c.Next()
	if !moved {
		t.Fatalf("Next moved = false, want true")
	}
	if fileChanged {
		t.Fatalf("Next fileChanged = true, want false (same file, different encoding)")
	}
	if !bytes.Equal(cursorStop(t, c).RawPath, path) {
		t.Fatalf("RawPath = %q, want %q", cursorStop(t, c).RawPath, path)
	}
}
