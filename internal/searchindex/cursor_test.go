package searchindex_test

import (
	"testing"

	"vrg/internal/searchindex"
)

// wantCurrent asserts the cursor sits on the stop for path:line.
func wantCurrent(t *testing.T, idx *searchindex.Index, path string, line int64) {
	t.Helper()
	stop, ok := idx.Current()
	if !ok {
		t.Fatalf("Current() = false, want the stop %q:%d", path, line)
	}
	if string(stop.Path) != path || stop.Line != line {
		t.Fatalf("Current() = %q:%d, want %q:%d", stop.Path, stop.Line, path, line)
	}
}

// Startup places the cursor on the first stop in path-then-line order —
// the lowest raw path's smallest line number — regardless of arrival
// order.
func TestStartupSelectsFirstStop(t *testing.T) {
	idx := collect(t, "/w",
		matchEvent(t, textData("b.txt"), textData("x\n"), 3, submatch(textData("x"), 0, 1)),
		matchEvent(t, textData("a.txt"), textData("x\n"), 9, submatch(textData("x"), 0, 1)),
		matchEvent(t, textData("a.txt"), textData("x\n"), 2, submatch(textData("x"), 0, 1)),
	)
	wantCurrent(t, idx, "a.txt", 2)
}

// Next advances and Prev retreats through the stop list circularly: Next
// past the last stop wraps to the first and Prev past the first wraps to
// the last. Every step reports whether it wrapped and whether the new
// stop is in a different file than the departed one.
func TestNextPrevCircular(t *testing.T) {
	idx := collect(t, "/w",
		matchEvent(t, textData("a.txt"), textData("x\n"), 2, submatch(textData("x"), 0, 1)),
		matchEvent(t, textData("a.txt"), textData("x\n"), 5, submatch(textData("x"), 0, 1)),
		matchEvent(t, textData("b.txt"), textData("x\n"), 1, submatch(textData("x"), 0, 1)),
	)

	steps := []struct {
		name string
		step func() searchindex.Move
		want searchindex.Move
		path string
		line int64
	}{
		{"next same file", idx.Next, searchindex.Move{}, "a.txt", 5},
		{"next across files", idx.Next, searchindex.Move{FileChanged: true}, "b.txt", 1},
		{"next wraps to first", idx.Next, searchindex.Move{Wrapped: true, FileChanged: true}, "a.txt", 2},
		{"prev wraps to last", idx.Prev, searchindex.Move{Wrapped: true, FileChanged: true}, "b.txt", 1},
		{"prev across files", idx.Prev, searchindex.Move{FileChanged: true}, "a.txt", 5},
		{"prev same file", idx.Prev, searchindex.Move{}, "a.txt", 2},
	}
	for _, s := range steps {
		if mv := s.step(); mv != s.want {
			t.Fatalf("%s = %+v, want %+v", s.name, mv, s.want)
		}
		wantCurrent(t, idx, s.path, s.line)
	}
}

// Wrapping inside a single file reports the wrap without a file change:
// the departed and arrived stops share one path.
func TestWrapWithinOneFile(t *testing.T) {
	idx := collect(t, "/w",
		matchEvent(t, textData("a.txt"), textData("x\n"), 1, submatch(textData("x"), 0, 1)),
		matchEvent(t, textData("a.txt"), textData("x\n"), 5, submatch(textData("x"), 0, 1)),
	)
	if mv := idx.Next(); mv != (searchindex.Move{}) {
		t.Fatalf("same-file Next() = %+v, want a zero Move", mv)
	}
	if mv := idx.Next(); mv != (searchindex.Move{Wrapped: true}) {
		t.Fatalf("wrapping Next() = %+v, want Wrapped without FileChanged", mv)
	}
	wantCurrent(t, idx, "a.txt", 1)
	if mv := idx.Prev(); mv != (searchindex.Move{Wrapped: true}) {
		t.Fatalf("wrapping Prev() = %+v, want Wrapped without FileChanged", mv)
	}
	wantCurrent(t, idx, "a.txt", 5)
}

// Every submatch on one matched line is a single navigation stop: a line
// carrying several submatches is visited once by Next.
func TestOneStopPerMatchedLine(t *testing.T) {
	idx := collect(t, "/w",
		matchEvent(t, textData("a.txt"), textData("aa bb aa\n"), 7,
			submatch(textData("aa"), 0, 2),
			submatch(textData("bb"), 3, 5),
			submatch(textData("aa"), 6, 8)),
		matchEvent(t, textData("a.txt"), textData("z\n"), 9, submatch(textData("z"), 0, 1)),
	)
	if n := len(idx.Stops()); n != 2 {
		t.Fatalf("Stops() = %d, want 2: line 7's submatches are one stop", n)
	}
	idx.Next()
	wantCurrent(t, idx, "a.txt", 9)
}

// With exactly one stop, Next and Prev are strict no-ops: the cursor
// does not move and no movement is reported — no wrap, no file change.
func TestSingleStopIsStrictNoop(t *testing.T) {
	idx := collect(t, "/w",
		matchEvent(t, textData("a.txt"), textData("x\n"), 4, submatch(textData("x"), 0, 1)),
	)
	for _, step := range []struct {
		name string
		fn   func() searchindex.Move
	}{{"Next", idx.Next}, {"Prev", idx.Prev}} {
		if mv := step.fn(); mv != (searchindex.Move{}) {
			t.Fatalf("single-stop %s() = %+v, want a zero Move", step.name, mv)
		}
		wantCurrent(t, idx, "a.txt", 4)
	}
}

// An empty index has no current stop and navigation is a no-op.
func TestEmptyIndexNavigationNoop(t *testing.T) {
	idx := collect(t, "/w")
	if _, ok := idx.Current(); ok {
		t.Fatal("Current() = true on an empty index, want false")
	}
	if mv := idx.Next(); mv != (searchindex.Move{}) {
		t.Fatalf("empty Next() = %+v, want a zero Move", mv)
	}
	if mv := idx.Prev(); mv != (searchindex.Move{}) {
		t.Fatalf("empty Prev() = %+v, want a zero Move", mv)
	}
}
