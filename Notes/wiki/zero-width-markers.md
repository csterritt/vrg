# Zero-width match markers — one inverse cell for an empty match span (Issue #23)

Delivered by
[Issue #23](../issues/023-zero-width-match-markers.md)
([task](../tasks/023-zero-width-match-markers.md)): a zero-width
submatch — `^`, `$`, `a*` over non-`a` text — paints as exactly one
inverse-video space at its mapped display cell, underlined when the
match is on the current matched line, and participates in wrapping,
clipping, horizontal reveal and extent, pan clamping, destination
reveal, and the hidden-content indicators under exactly the rules an
ordinary match follows. Relevant PRD sections: *Text, graphemes, and
safe presentation* (match visibility over removed terminator bytes,
the cluster-no-split rule, the wrap and clip bullets) and *Layout and
indicators* (match visibility driving the indicators) in
[`Notes/PRD-vrg.md`](../PRD-vrg.md); user stories 70, 76, 77, 80.
Builds on [line-structure.md](line-structure.md) (the
coordinate-separated `CellsCovering` that maps zero-width positions —
including terminator-only spans — onto marker cells),
[grapheme-highlight-expansion.md](grapheme-highlight-expansion.md)
(the recorded empty `Line.Highlights` spans this issue paints),
[wrap-mode.md](wrap-mode.md), [horizontal-panning.md](horizontal-panning.md),
[horizontal-reveal.md](horizontal-reveal.md),
[destination-reveal.md](destination-reveal.md), and
[hidden-content-indicators.md](hidden-content-indicators.md).

## The marker cell

A recorded highlight span with `Start == End` is a marker position,
not a painted range. `Line.MarkerAt(cell)` reports whether any
highlight sits empty at a cell, and `contentText` consults it per
painted cell:

- **Inside visible text** the marker marks the existing cell it maps
  to — the cell's own text paints with the match style. Following
  text does not shift; a marker on `hit` at cell 0 underlines `h`,
  not a widened line.
- **At the end-of-line position** — past the last content cell — the
  marker occupies one *new* effective display cell and paints as a
  literal inverse-video space. This is how an empty matched line
  shows its match and how `$` after `hit` paints a fourth cell.
- On the **current matched line** the marker cell takes the
  current-match underline like any match cell; elsewhere the plain
  match style.

An interior position lands on the cell holding its byte — a position
inside a grapheme cluster maps to the cluster's start cell — so a
marker can never split a wide glyph or ZWJ sequence.

## Effective width — `Line.Extent()`

`Line.Extent()` is `len(Line.Cells)` plus one when a marker sits at
the end-of-line position. It is the line's *effective* display width
— what the row model, pan clamping, and reveal arithmetic consume —
distinct from the cell count of visible text:

- An empty matched line (`^` on `\n`) has extent 1: the marker alone
  is one effective cell.
- A marker-only line's `Line.MaxStart` treats the EOL marker as a
  one-cell candidate, so `MaxOff` over it is 0 under Issue #18's
  paintable-boundary rule — the marker fits in any positive text
  width, and panning past it hides it (entirely hidden → the left
  `*`).
- `Rows` flat (run-off-edge) spans use `Extent()`: a line with an EOL
  marker emits one row `[0, extent)` whose `End` can reach
  `len(Line.Cells) + 1`.

## Wrapping

`wrapLine` appends the EOL marker after the content clusters:

- Room left in the current row → the marker joins it, extending the
  row's `End` to `extent` (`hit` at width 4 wraps `[{0,4}]` with the
  marker inside; at width 2 → `[{0,2},{2,4}]`, the marker sharing
  the second row's last cell).
- The preceding row is exactly full → the marker starts a new
  one-cell row (`hit` at width 3 → `[{0,3},{3,4}]`), and that row
  reports `Continuation`.
- An empty matched line emits one row `[0,1)` — the marker cell
  alone.

The marker is a unit like a cluster: it never splits and it moves
whole to the next row.

## Reveal, clipping, indicators — ordinary rules, no special cases

Once the row model carries marker cells, every downstream consumer
treats them as one-cell matches with no marker-specific branches:

- `Rows.StopTarget` resolves an empty first-submatch span to the
  marker cell, so markers are navigable reveal targets — `n`/`p`
  reveals the marker's own wrapped row and `RevealOff` pans to it by
  the single-cell right-edge rule.
- `CellVisible` judges a marker's one cell like any target: hidden
  left of `off`, hidden right of the window, or a clipped blank —
  never painted.
- `LeftMark`/`RightMark` count an entirely hidden marker as an
  entirely hidden match: run-off-edge panning past a marker sets the
  gutter `*`, and a marker hidden right earns the current line's
  reserved `*`.
- `contentText` clips the EOL marker cell like ordinary content —
  left of `off` it is never painted; straddling the right edge it
  clips to nothing, and a wrapped marker row clips by the same
  `col + 1 > textW` bound as a one-cell cluster.

The terminator-only `$` on `hit\r\n` is the canonical case: rg
reports `(4,4)`, `CellsCovering` maps it to display column 3
(Issue #22's terminator→EOL rule), and the recorded `[3,3)` span
renders, wraps, reveals, clips, and indicates exactly like any other
marker — no CRLF-specific handling anywhere.

## Tests

`internal/filebuffer/filebuffer_test.go` gains marker tables
(external package): BOL, in-text, EOL, and empty-line marker
positions through `MarkerAt`; `Extent` adding one only for an EOL
marker; in-cluster positions mapping to the cluster start without
splitting `文`; LF and CRLF terminator positions both landing on the
end-of-line marker cell; and `MaxStart` treating the marker as a
one-cell candidate.

`internal/viewport/marker_test.go` (external package): EOL marker
wrap rows — joins a partially full row, takes its own row after a
full one, marker-only line emits one `[0,1)` row — flat rows
spanning `Extent()`, `MaxOff` including the marker cell and a
marker-only line's `MaxOff` 0, `TargetRow` revealing the marker's
own row after a full-width line, and a deep marker landing its row
at `floor(h/3)` through `Reveal`.
