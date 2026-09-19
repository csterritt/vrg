package searchindex

// Cursor identifies one navigation stop: an index into Index.Files and
// an index into that file's Stops. The zero value selects the first
// stop in path-then-line order — the startup selection.
type Cursor struct {
	File, Stop int
}

// Move reports how one cursor step resolved: Wrapped marks a step that
// crossed an index end — last stop to first for Next, first to last for
// Prev — and FileChanged marks the selected stop landing in a different
// file than the step departed from. A strict no-op step — an empty
// index or a single stop — is the zero Move.
type Move struct {
	Wrapped     bool
	FileChanged bool
}

// Cursor reports the cursor's current position; ok is false when the
// index holds no stops — an empty index has no position at all.
func (ix *Index) Cursor() (Cursor, bool) {
	return ix.cur, ix.stops > 0
}

// Next advances the cursor to the following stop in path-then-line
// order, wrapping circularly from the last stop to the first. Prev
// retreats to the preceding stop, wrapping from the first to the last.
// Multiple submatches on one matched line share their stop. With zero
// or one stop both are strict no-ops returning the zero Move.
func (ix *Index) Next() Move { return ix.step(1) }

// Prev retreats the cursor; see Next.
func (ix *Index) Prev() Move { return ix.step(-1) }

// step moves the cursor one stop in direction dir (+1 next, −1 prev)
// and reports how the step resolved.
func (ix *Index) step(dir int) Move {
	if ix.stops <= 1 {
		return Move{}
	}
	cur := ix.cur
	from := cur.File
	var mv Move
	if dir > 0 {
		cur.Stop++
		if cur.Stop >= len(ix.Files[cur.File].Stops) {
			cur.Stop = 0
			cur.File++
			if cur.File >= len(ix.Files) {
				cur.File = 0
				mv.Wrapped = true
			}
		}
	} else {
		if cur.Stop == 0 {
			cur.File--
			if cur.File < 0 {
				cur.File = len(ix.Files) - 1
				mv.Wrapped = true
			}
			cur.Stop = len(ix.Files[cur.File].Stops) - 1
		} else {
			cur.Stop--
		}
	}
	// A wrap within a one-file index crosses an index end without
	// landing in a different file.
	mv.FileChanged = cur.File != from
	ix.cur = cur
	return mv
}
