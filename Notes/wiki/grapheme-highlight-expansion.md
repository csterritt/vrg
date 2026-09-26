# Grapheme-cluster highlight expansion

Issue #21
(`Notes/issues/021-grapheme-cluster-highlight-expansion.md`, tasks
`Notes/tasks/021-grapheme-cluster-highlight-expansion.md`) delivered
cluster-expanded highlight spans: a recorded submatch landing inside a
grapheme cluster highlights the whole cluster, the wrap/clip filler
blanks of Issues #16 and #18 are never painted as match cells, and the
expanded span is the single source the reveal and the hidden-match
indicators consume.

PRD cross-references: "Text, graphemes, and safe presentation" (the
single-policy bullet and the grapheme-expansion bullets) and "Testing
Decisions → FileBuffer" in `Notes/PRD-vrg.md`.

## Outward cluster expansion

`filebuffer.Load` maps each validated submatch's raw byte range through
`present.Line.Span` — the byte→cell map already expands interior bytes
to their whole display *unit* — and then through `clusterSpan`, which
walks a nonempty span's endpoints outward to the cluster boundaries the
cells' `Lead` marks carry: a start inside a cluster walks back to its
first cell, an end inside walks forward to the next boundary, and a
strictly interior range expands both ways. Marker positions
(`Start == End`) are already cell-precise and pass through.

The cases the extra layer exists for are the **zero-width clusters**
`LineOf` joins into a host cell's text: a standalone combining mark
after an escape borrows the escape's trailing cell (`a\x01́` — a match
on the mark's bytes expands to both `^A` cells), one after a tab
borrows the last expansion cell (expanding over the whole tab cluster),
and a zero-width rune after a wide glyph borrows its trailing cell. A
combining-only match inside a base cluster (`café` decomposed — the
mark's own zero-width cluster joined to the `e` cell) highlights the
whole cluster, so the visible `é` glyph is what paints. Emoji ZWJ
sequences are one cluster under the shared `x/ansi` policy
(`👨‍👩‍👧` is two cells), so a match on inner emoji bytes covers both
cells; the same holds for a wide pair like `世` — a two-cell glyph is
never split by a highlight boundary.

## Fallback cells

A cluster with no base or independent visible cell still gets a
highlightable cell: `emit` gives a zero-width unit at line start a
**provisional cell of its own on a `◌` (U+25CC) dotted-circle base**
(the standard base for an isolated combining mark). A bare mark would
merge into the previous terminal cell and paint nothing — the base
keeps the cluster in its own `Lead` cell, so a standalone combining
mark's match is one visible highlighted cell — never a zero-cell
highlight. Mid-line zero-width units join the previous cell's text and
byte-map onto it, which is what makes their partial matches expand
outward.

## Blanks are never match cells

`present.Cell` gained a third mark, `Blank`, carried only by
substituted filler — never produced by `LineOf`:

- `viewport.Model.Row` marks the cell it blanks when a wrap row ends
  inside a last-resort-split cluster, so the filler under a covering
  span is identifiable.
- `clipRow` marks the in-window cells it blanks for a cluster split by
  a clip edge.

`renderCells` (`internal/app/browse.go`) paints a `Blank` cell's space
unstyled even when a coverage span crosses it — the span still spans
it, but the cell is filler, not a match cell. A marker still paints
its own position, blanked cell included. The same rule covers the
clip-edge split `renderCells` detects itself (a `Cont` cell past the
text width). Net effect: a highlight at a wrap boundary or clip edge
never turns filler into inverse video.

## One expanded-span source

The cluster-expanded span is the **sole** span `Buffer.Spans` hands
down; nothing downstream re-derives or narrows the recorded bytes:

- `reveal` (`internal/app/browse.go`) takes the destination's target
  cell from the smallest expanded span start — the Issue #14/#19
  reveals ([destination-reveal.md](destination-reveal.md),
  [minimal-horizontal-reveal.md](minimal-horizontal-reveal.md)) —
  so a match whose recorded bytes start mid-cluster reveals from the
  cluster's start cell.
- `hiddenMarks` classifies the same expanded spans, so the Issue #20
  indicators
  ([hidden-content-indicators.md](hidden-content-indicators.md)) count
  a mid-cluster match from its cluster start: once the expanded span
  stands entirely left of the window the gutter shows `*`, not `_`.
- `revealCell`'s cluster-width arithmetic and `hiddenMarks`'s
  painted-cell visibility are unchanged — they read the same cells and
  spans, now expanded.

## Tests

- `internal/filebuffer/expand_test.go` — `clusterSpan`'s cell-space
  boundary table (start inside, end inside, strictly interior, whole
  cluster, neighbour, line end, marker passthrough) and the end-to-end
  `Load` table: combining-only match, mark on an escape's trailing
  cell, mark on a tab expansion cell, zero-width rune on a wide
  glyph's trailing cell, mark joined to a replaced invalid byte,
  interior byte of a wide pair, interior bytes of a ZWJ sequence, and
  the standalone-mark fallback cell (`◌́`, also pinned by
  `TestLeadingCombiningCluster` and `TestLineText`).
- `internal/viewport/blanks_test.go` — the wrap row's substituted lead
  cell is `Blank`-marked under a covering span (continuation rows keep
  unmarked `Cont` cells) and `clipRow`'s edge-split blanks are marked
  at either edge.
- `internal/app/cluster_test.go` — `renderCells` leaves a covered
  blank and a clip-edge split blank unstyled while a marker on a blank
  still paints its inverse space.
- `internal/app/indicators_test.go` — Issue #20's tests now consume
  the expanded spans: a match recorded mid-cluster counts hidden-left
  once its cluster start is hidden (`TestMidClusterMatchCountsFromClusterStart`).
- `internal/app/hreveal_test.go` — `n` to a mid-cluster match reveals
  and paints the whole expanded cluster
  (`TestHRevealMidClusterMatchPaintsWholeCluster`).

See [unit-tests.md](unit-tests.md) § `internal/filebuffer`,
`internal/viewport`, and `internal/app`.

## Files

- `internal/filebuffer/filebuffer.go` — `Load` runs each validated
  submatch's mapped span through `clusterSpan`, the `Lead`-boundary
  outward expansion.
- `internal/present/line.go` — `Cell` gained the `Blank` filler mark.
- `internal/viewport/rows.go` — `Row` marks the wrap boundary's
  substituted lead cell `Blank`.
- `internal/viewport/viewport.go` — `clipRow` marks edge-split blank
  cells `Blank`.
- `internal/app/browse.go` — `renderCells` never styles `Blank` (or
  clip-edge-split) cells as match cells; markers still paint.

See also: [safe-presentation.md](safe-presentation.md) (the shared
grapheme/cell policy and `Line.Span` this expansion builds on),
[wrap-mode.md](wrap-mode.md) (the wrap blanks),
[horizontal-panning.md](horizontal-panning.md) (the clip blanks),
[minimal-horizontal-reveal.md](minimal-horizontal-reveal.md) and
[hidden-content-indicators.md](hidden-content-indicators.md) (the
expanded spans' consumers), and
[browse-tracer.md](browse-tracer.md) (the match styling `renderCells`
applies).
