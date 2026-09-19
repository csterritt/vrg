package searchindex_test

import (
	"testing"

	"vrg/internal/searchindex"
)

// cursorIndex builds an index holding a.txt stops at lines 2 and 5 and
// a b.txt stop at line 1 — three stops across two files in path order.
func cursorIndex(t *testing.T) *searchindex.Index {
	t.Helper()
	return build(t, "/wd",
		beginRec(text("a.txt")),
		matchRec(text("a.txt"), text("hit one\n"), 2, sub(text("hit"), 0, 3)),
		matchRec(text("a.txt"), text("hit two\n"), 5, sub(text("hit"), 0, 3)),
		endRec(text("a.txt")),
		beginRec(text("b.txt")),
		matchRec(text("b.txt"), text("hit three\n"), 1, sub(text("hit"), 0, 3)),
		endRec(text("b.txt")),
		summaryRec(),
	)
}

func wantCursor(t *testing.T, ix *searchindex.Index, file, stop int) {
	t.Helper()
	cur, ok := ix.Cursor()
	if !ok {
		t.Fatalf("Cursor() reports no position, want {%d %d}", file, stop)
	}
	if cur.File != file || cur.Stop != stop {
		t.Fatalf("Cursor() = %v, want {%d %d}", cur, file, stop)
	}
}

func wantMove(t *testing.T, mv searchindex.Move, wrapped, fileChanged bool) {
	t.Helper()
	if mv.Wrapped != wrapped || mv.FileChanged != fileChanged {
		t.Fatalf("Move = %+v, want Wrapped=%v FileChanged=%v", mv, wrapped, fileChanged)
	}
}

// curStopLine is the cursor's current matched line number.
func curStopLine(t *testing.T, ix *searchindex.Index) int64 {
	t.Helper()
	cur, ok := ix.Cursor()
	if !ok {
		t.Fatal("Cursor() reports no position")
	}
	return ix.Files[cur.File].Stops[cur.Stop].Number
}

// Startup selects the first stop in path-then-line order: the first
// file's first matched line.
func TestCursorStartsAtFirstStop(t *testing.T) {
	ix := cursorIndex(t)
	wantCursor(t, ix, 0, 0)
	if got := curStopLine(t, ix); got != 2 {
		t.Fatalf("current stop line = %d, want a.txt's line 2", got)
	}
}

// Next walks every stop in path-then-line order and wraps from the last
// stop back to the first; the move report marks file crossings and the
// wrap separately.
func TestNextWalksStopsAndWraps(t *testing.T) {
	ix := cursorIndex(t)
	wantMove(t, ix.Next(), false, false) // a.txt line 2 → line 5
	wantCursor(t, ix, 0, 1)
	wantMove(t, ix.Next(), false, true) // a.txt → b.txt
	wantCursor(t, ix, 1, 0)
	wantMove(t, ix.Next(), true, true) // last stop wraps to the first
	wantCursor(t, ix, 0, 0)
	if got := curStopLine(t, ix); got != 2 {
		t.Fatalf("after wrap current stop line = %d, want 2", got)
	}
}

// Prev retreats one stop and wraps from the first stop to the last.
func TestPrevRetreatsAndWraps(t *testing.T) {
	ix := cursorIndex(t)
	wantMove(t, ix.Prev(), true, true) // first stop wraps to the last
	wantCursor(t, ix, 1, 0)
	if got := curStopLine(t, ix); got != 1 {
		t.Fatalf("after wrap-back current stop line = %d, want b.txt's line 1", got)
	}
	wantMove(t, ix.Prev(), false, true) // b.txt → a.txt's last stop
	wantCursor(t, ix, 0, 1)
	wantMove(t, ix.Prev(), false, false)
	wantCursor(t, ix, 0, 0)
}

// Wrapping within a single file reports the wrap but no file change:
// the last stop's successor is the first stop of the same file.
func TestWrapWithinOneFileReportsNoFileChange(t *testing.T) {
	ix := build(t, "/wd",
		matchRec(text("only.txt"), text("hit a\n"), 1, sub(text("hit"), 0, 3)),
		matchRec(text("only.txt"), text("hit b\n"), 4, sub(text("hit"), 0, 3)),
		summaryRec(),
	)
	wantMove(t, ix.Next(), false, false)
	wantCursor(t, ix, 0, 1)
	wantMove(t, ix.Next(), true, false) // wrap, same file
	wantCursor(t, ix, 0, 0)
	wantMove(t, ix.Prev(), true, false) // wrap back, same file
	wantCursor(t, ix, 0, 1)
}

// An index holding exactly one stop makes both directions strict
// no-ops: the move reports nothing and the position cannot change.
func TestSingleStopStrictNoOp(t *testing.T) {
	ix := build(t, "/wd",
		matchRec(text("solo.txt"), text("hit\n"), 3, sub(text("hit"), 0, 3)),
		summaryRec(),
	)
	wantMove(t, ix.Next(), false, false)
	wantMove(t, ix.Prev(), false, false)
	wantMove(t, ix.Next(), false, false)
	wantCursor(t, ix, 0, 0)
}

// An index with no stops has no cursor position; navigation is an
// explicit no-op rather than an error.
func TestEmptyIndexNavigationNoOp(t *testing.T) {
	ix := build(t, "/wd", summaryRec())
	if _, ok := ix.Cursor(); ok {
		t.Fatal("Cursor() reports a position on an empty index")
	}
	wantMove(t, ix.Next(), false, false)
	wantMove(t, ix.Prev(), false, false)
	if _, ok := ix.Cursor(); ok {
		t.Fatal("Cursor() reports a position after no-op navigation")
	}
}

// Several submatches on one matched line are one stop: Next crosses to
// the next matched line, never to a second submatch on the same line.
func TestSubmatchesShareOneStop(t *testing.T) {
	ix := build(t, "/wd",
		matchRec(text("a.txt"), text("hit and hit\n"), 2,
			sub(text("hit"), 0, 3),
			sub(text("hit"), 8, 11)),
		matchRec(text("a.txt"), text("hit\n"), 9, sub(text("hit"), 0, 3)),
		summaryRec(),
	)
	wantStops(t, ix.Files[0], []int64{2, 9})
	wantMove(t, ix.Next(), false, false)
	if got := curStopLine(t, ix); got != 9 {
		t.Fatalf("Next visited line %d, want line 9 — submatches are not stops", got)
	}
}

// The cursor follows prepared order — unsigned raw path bytes, then
// ascending line number — not the order records arrived in.
func TestCursorFollowsPreparedOrder(t *testing.T) {
	ix := build(t, "/wd",
		matchRec(text("b.txt"), text("hit\n"), 1, sub(text("hit"), 0, 3)),
		matchRec(text("a.txt"), text("hit\n"), 7, sub(text("hit"), 0, 3)),
		matchRec(text("a.txt"), text("hit\n"), 3, sub(text("hit"), 0, 3)),
		summaryRec(),
	)
	// Prepared order is a.txt{3, 7} then b.txt{1} regardless of stream
	// order; the walk visits lines 3, 7, 1, then wraps to 3.
	for i, want := range []int64{3, 7, 1, 3} {
		if got := curStopLine(t, ix); got != want {
			t.Fatalf("stop %d: line %d, want %d", i, got, want)
		}
		ix.Next()
	}
}
