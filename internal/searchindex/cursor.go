package searchindex

import "bytes"

// Step reports one matched-line cursor movement for the App wiring:
// Stop is the selected stop and the flags describe the transition.
// Moved is false for the strict no-ops — an empty index or an index
// holding exactly one stop — where the selection is unchanged and the
// caller issues neither a pop-up nor a reload. FileChanged marks a
// destination in a different file than the departed stop; Wrapped
// marks a step that passed an index end.
type Step struct {
	Stop        Stop
	Moved       bool
	FileChanged bool
	Wrapped     bool
}

// Current returns the cursor's selected stop — the first stop in
// index order until Next or Prev moves it — or false when the index
// holds no stops.
func (ix *Index) Current() (Stop, bool) {
	ix.Prepare()
	if ix.cursor >= len(ix.order) {
		return Stop{}, false
	}
	return ix.export(ix.order[ix.cursor]), true
}

// Next advances the cursor to the following stop in index order,
// wrapping from the last stop to the first.
func (ix *Index) Next() Step { return ix.step(1) }

// Prev retreats the cursor to the preceding stop in index order,
// wrapping from the first stop to the last.
func (ix *Index) Prev() Step { return ix.step(-1) }

// step moves the cursor circularly by dir and reports the transition.
// Zero-stop and one-stop indexes are strict no-ops: the cursor does
// not move and no transition flags are raised.
func (ix *Index) step(dir int) Step {
	ix.Prepare()
	if len(ix.order) < 2 {
		// A one-stop index still reports its stop so the caller can
		// see the unchanged selection; an empty index reports the
		// zero Stop.
		var st Step
		if cur, ok := ix.Current(); ok {
			st.Stop = cur
		}
		return st
	}
	from := ix.order[ix.cursor]
	to := ix.cursor + dir
	st := Step{Moved: true}
	switch {
	case to < 0:
		to = len(ix.order) - 1
		st.Wrapped = true
	case to >= len(ix.order):
		to = 0
		st.Wrapped = true
	}
	ix.cursor = to
	st.Stop = ix.export(ix.order[to])
	st.FileChanged = !bytes.Equal(from.path, ix.order[to].path)
	return st
}
