# Vertical destination reveal — target row, no-scroll, one-third placement (Issue #14)

Delivered by
[Issue #14](../issues/014-vertical-destination-reveal.md)
([task](../tasks/014-vertical-destination-reveal.md)): at startup (once
the file's content loads) and on every actual `n`/`p` transition, the
file panel reveals the **rendered row containing the display target** —
the start cell of the first submatch on the destination line — with
visible-target no-scroll and one-third placement clamped at BOF/EOF.
Relevant PRD sections: *Navigation, viewport, and logical anchors* (the
target-row, placement, and file-change-sequence bullets) and *Testing
Decisions → Viewport* in [`Notes/PRD-vrg.md`](../PRD-vrg.md); user
stories 52, 53, 57. Builds on
[match-navigation.md](match-navigation.md) (the cursor and triggers) and
[viewport-scrolling.md](viewport-scrolling.md) (the per-file viewport
the reveal starts from).

## Display target (`internal/viewport/reveal.go`)

- `Target{Line, Cell}` is a **display location**, not a source-line
  ordinal: `Line` is the destination line's number and `Cell` is the
  display cell where its first submatch starts. `Rows.StopTarget`
  resolves a `searchindex.Stop` through the line's byte→cell map
  (`CellsCovering`), so an escaped byte's widened cells and a
  mid-cluster start land on the right cell — the cell is not the byte
  offset. A submatch whose bytes produced no cell — a zero-width
  position or terminator-only span, Issues #22/#23 — targets the marker
  cell one past the line's last cell. Submatches arrive sorted by
  `(Start, End)`, so `Submatches[0]` is the first submatch.
- `Rows.TargetRow` maps the target to the **rendered row** a reveal must
  show. Since [Issue #16](wrap-mode.md) made the row model wrap-aware,
  it scans the destination line's wrapped rows for the span containing
  the target's start cell — a match deep in a wrapped line reveals its
  own row — and a target cell past the line's cells lands on the line's
  last row, the end-of-line marker position (Issue #23). A line number
  outside the prepared rows clamps to the nearest real row.

## Placement rules — `Viewport.Reveal(row, rows, height)`

- **Visible target → no scroll.** If `row ∈ [top, top + height)` the
  viewport is unchanged and `Reveal` reports no move.
- **Hidden target → one-third placement.** The top becomes
  `row − floor(height / 3)` — the target lands at zero-based row
  `floor(h / 3)` — clamped to `[0, MaxTop(rows, height)]`, so BOF and
  EOF content take precedence over the exact third.
- `Reveal` reports whether the top actually moved: the return value
  distinguishes a moving reveal (which replaces saved vertical state)
  from a no-scroll reveal (which leaves it). Since Issue #17 the same
  distinction drives the logical anchor: a moving reveal replaces it
  with the resulting top row's location while a no-scroll reveal keeps
  the retained logical column (see
  [logical-anchor.md](logical-anchor.md)).

## Starting-viewport sequence and saved state (`internal/app`)

`model.reveal` is the single entry point: read the cursor's current
stop, resolve its target row through the file's `rowSource`, then
`Reveal` against the file's saved `vps` viewport — absent on a first
visit, so the zero value's top-of-file is the start — and write the
viewport back to `vps` **only when it moved**. Since Issue #17 the
placeholder case is not a plain no-op: when no layout matching the
current parameters is installed, `reveal` records a **pending intent**
on `pendingReveals[path]` that commits when the matching layout
completion installs — so navigation under a gated worker still lands
on the latest selection (see
[logical-anchor.md](logical-anchor.md)). Zero content height remains a
no-op.

Two triggers, the ones this issue owns:

- **Navigation time** — `navigate` calls `reveal` after every *actual*
  cursor transition: it compares the cursor before and after the step,
  so a strict no-op (`n`/`p` on a zero- or one-stop index) performs no
  reveal and leaves even an off-screen viewport alone. When the move
  crosses into an uncached file the reveal finds no rows yet and the
  load completion applies it instead.
- **After load** — `fileLoadedMsg` runs `reveal` when the completed path
  is the current file. That covers the startup file's first visit and a
  navigated-to file whose load lands while current; because the cursor
  is read at completion time, the revealed target is always the latest
  selection. Issue #28 owns the deeper contract — a two-stage
  prepared-layout commit with reveal/reload-intent arbitration — and
  Issue #19 owns horizontal reveal.

The saved-state rules fall out of the write-through: a moving reveal
replaces the file's saved top with the revealed position (a later
revisit resumes from it), while a no-scroll reveal leaves the saved
entry — or its absence — untouched.

## Tests

`internal/viewport/reveal_test.go` (external package): the display
target as the first submatch's start cell through escape-widened
byte→cell maps and the zero-width marker-cell fallback, `TargetRow`
line→row mapping with out-of-range clamps and the empty model,
visible-target no-scroll at the window edges, one-third placement above
and below, BOF/EOF precedence over the third, and reveal starting from
whatever top the viewport holds (the saved-versus-first-visit
distinction).

`internal/app/reveal_test.go` (same package): startup reveal after the
first load (hidden target at row 7 of 23, visible target staying at top
with no state recorded), the `n`/`p` round trip on a long file
(one-third down, then the BOF clamp near the top), no-scroll between
on-screen stops, no reveal on a strict no-op, a moving reveal replacing
the saved viewport a later revisit resumes from, a revisit starting
from the saved top, a cached-but-never-visited first visit starting at
the top, and a navigated-to uncached file revealing on load completion.
Issue #12/#13 scroll and navigation tests were updated where they
encoded the pre-reveal contract. See [unit-tests.md](unit-tests.md).
