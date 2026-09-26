package searchindex_test

import (
	"testing"

	"vrg/internal/searchindex"
)

// cursorIndex builds a prepared two-file index whose records arrive
// out of order — b.txt first, a.txt's lines descending — so the index
// order a.txt:1, a.txt:3, a.txt:5, b.txt:2, b.txt:4 proves both the
// raw-path ordering and the ascending line ordering within a file.
func cursorIndex(t *testing.T) *searchindex.Index {
	t.Helper()
	ix := searchindex.New("/w")
	feed(t, ix,
		beginRec(jText("b.txt")),
		matchRec(jText("b.txt"), jText("b4\n"), 4, subRec(jText("b4"), 0, 2)),
		matchRec(jText("b.txt"), jText("b2\n"), 2, subRec(jText("b2"), 0, 2)),
		endRec(jText("b.txt"), "null"),
		beginRec(jText("a.txt")),
		matchRec(jText("a.txt"), jText("a5\n"), 5, subRec(jText("a5"), 0, 2)),
		matchRec(jText("a.txt"), jText("a1\n"), 1, subRec(jText("a1"), 0, 2)),
		matchRec(jText("a.txt"), jText("a3\n"), 3, subRec(jText("a3"), 0, 2)),
		endRec(jText("a.txt"), "null"),
		`{"type":"summary","data":{}}`,
	)
	ix.Prepare()
	return ix
}

// wantStop fails the test unless s is the stop at path:line.
func wantStop(t *testing.T, s searchindex.Stop, path string, line int64) {
	t.Helper()
	if string(s.Path) != path || s.Line != line {
		t.Fatalf("stop = %s:%d, want %s:%d", s.Path, s.Line, path, line)
	}
}

// The cursor selects the first stop in index order — unsigned raw path
// bytes, then ascending line number — before any navigation, however
// the records arrived.
func TestCursorStartsAtFirstStop(t *testing.T) {
	ix := cursorIndex(t)
	s, ok := ix.Current()
	if !ok {
		t.Fatal("Current on a nonempty index returned false")
	}
	wantStop(t, s, "a.txt", 1)
}

// Next advances through the stops in index order and wraps from the
// last stop back to the first; each step reports whether it crossed
// into a different file and whether it wrapped an index end.
func TestCursorNextAdvancesAndWraps(t *testing.T) {
	ix := cursorIndex(t)
	steps := []struct {
		path        string
		line        int64
		fileChanged bool
		wrapped     bool
	}{
		{"a.txt", 3, false, false},
		{"a.txt", 5, false, false},
		{"b.txt", 2, true, false},
		{"b.txt", 4, false, false},
		{"a.txt", 1, true, true},
	}
	for i, w := range steps {
		st := ix.Next()
		if !st.Moved {
			t.Fatalf("Next #%d reported no movement", i+1)
		}
		wantStop(t, st.Stop, w.path, w.line)
		if st.FileChanged != w.fileChanged {
			t.Fatalf("Next #%d FileChanged = %v, want %v", i+1, st.FileChanged, w.fileChanged)
		}
		if st.Wrapped != w.wrapped {
			t.Fatalf("Next #%d Wrapped = %v, want %v", i+1, st.Wrapped, w.wrapped)
		}
	}
	s, ok := ix.Current()
	if !ok {
		t.Fatal("Current after navigation returned false")
	}
	wantStop(t, s, "a.txt", 1)
}

// Prev retreats through the stops in reverse index order and wraps
// from the first stop to the last, reporting the same transition
// flags.
func TestCursorPrevRetreatsAndWraps(t *testing.T) {
	ix := cursorIndex(t)
	steps := []struct {
		path        string
		line        int64
		fileChanged bool
		wrapped     bool
	}{
		{"b.txt", 4, true, true},
		{"b.txt", 2, false, false},
		{"a.txt", 5, true, false},
		{"a.txt", 3, false, false},
		{"a.txt", 1, false, false},
	}
	for i, w := range steps {
		st := ix.Prev()
		if !st.Moved {
			t.Fatalf("Prev #%d reported no movement", i+1)
		}
		wantStop(t, st.Stop, w.path, w.line)
		if st.FileChanged != w.fileChanged {
			t.Fatalf("Prev #%d FileChanged = %v, want %v", i+1, st.FileChanged, w.fileChanged)
		}
		if st.Wrapped != w.wrapped {
			t.Fatalf("Prev #%d Wrapped = %v, want %v", i+1, st.Wrapped, w.wrapped)
		}
	}
	s, ok := ix.Current()
	if !ok {
		t.Fatal("Current after navigation returned false")
	}
	wantStop(t, s, "a.txt", 1)
}

// An index holding exactly one stop makes both directions strict
// no-ops: no movement, no file change, no wrap — the one-stop rule
// requires no pop-up and no reload at the app level.
func TestCursorOneStopIsStrictNoOp(t *testing.T) {
	ix := searchindex.New("/w")
	feed(t, ix,
		beginRec(jText("a.txt")),
		matchRec(jText("a.txt"), jText("x\n"), 1, subRec(jText("x"), 0, 1)),
		endRec(jText("a.txt"), "null"),
		`{"type":"summary","data":{}}`,
	)
	ix.Prepare()
	for _, st := range []searchindex.Step{ix.Next(), ix.Prev()} {
		if st.Moved || st.FileChanged || st.Wrapped {
			t.Fatalf("one-stop step = %+v, want a strict no-op", st)
		}
		wantStop(t, st.Stop, "a.txt", 1)
	}
	s, ok := ix.Current()
	if !ok {
		t.Fatal("Current after no-op steps returned false")
	}
	wantStop(t, s, "a.txt", 1)
}

// An empty index is the explicit no-op case: there is no current stop
// and neither direction reports movement.
func TestCursorEmptyIndexIsNoOp(t *testing.T) {
	ix := searchindex.New("/w")
	feed(t, ix, `{"type":"summary","data":{}}`)
	ix.Prepare()
	if s, ok := ix.Current(); ok {
		t.Fatalf("Current on an empty index = %+v, want false", s)
	}
	for _, st := range []searchindex.Step{ix.Next(), ix.Prev()} {
		if st.Moved || st.FileChanged || st.Wrapped {
			t.Fatalf("empty-index step = %+v, want a strict no-op", st)
		}
	}
}

// Multiple submatches on one source line are one navigation stop: n
// steps over the line once, however many ranges it carries, and the
// wrap back stays within the same file.
func TestCursorSubmatchesShareOneStop(t *testing.T) {
	ix := searchindex.New("/w")
	feed(t, ix,
		beginRec(jText("a.txt")),
		matchRec(jText("a.txt"), jText("hit and hit\n"), 2,
			subRec(jText("hit"), 0, 3),
			subRec(jText("and"), 4, 7),
			subRec(jText("hit"), 8, 11)),
		matchRec(jText("a.txt"), jText("hit again\n"), 5, subRec(jText("hit"), 0, 3)),
		endRec(jText("a.txt"), "null"),
		`{"type":"summary","data":{}}`,
	)
	ix.Prepare()
	if n := ix.LineCount(); n != 2 {
		t.Fatalf("LineCount = %d, want 2 stops for 2 matched lines", n)
	}
	st := ix.Next()
	wantStop(t, st.Stop, "a.txt", 5)
	st = ix.Next()
	if !st.Moved || !st.Wrapped || st.FileChanged {
		t.Fatalf("wrap within one file = %+v, want Moved and Wrapped only", st)
	}
	wantStop(t, st.Stop, "a.txt", 2)
}
