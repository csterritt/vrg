# Standalone combining clusters get a real one-cell display fallback (Issue #43)

Delivered by
[Issue #43](../issues/043-combining-cluster-fallback-cell.md)
([task](../tasks/043-combining-cluster-fallback-cell.md)): a grapheme
cluster with no base and no independent visible cell — one the shared
`rivo/uniseg` policy measures at width 0 — displays as a real one-cell
fallback unit rather than only a widened cell annotation, so clusters,
byte→cell maps, wrapping, clipping, panning, highlight expansion, and
the Issue #39 cluster-driven renderer all agree the cluster occupies
exactly one visible cell and a match on it is never an inaccessible
zero-cell span. Relevant PRD section: *Text, graphemes, and safe
presentation* ("a cluster without a base/independent visible cell must
receive a visible fallback cell") in
[`Notes/PRD-vrg.md`](../PRD-vrg.md); user stories 71, 72, 75. The
representation is a recorded decision:
[`Notes/decisions/043-combining-cluster-fallback-cell.md`](../decisions/043-combining-cluster-fallback-cell.md).
Builds on [safe-presentation.md](safe-presentation.md) (`MapContent`'s
cell and cluster model), [wrap-mode.md](wrap-mode.md) (cluster-boundary
wrapping), [horizontal-panning.md](horizontal-panning.md) (the `off`
clip window and paintable boundary),
[grapheme-highlight-expansion.md](grapheme-highlight-expansion.md)
(the cluster-aligned spans), and
[unified-rendering.md](unified-rendering.md) (the cluster-driven
renderer the cell feeds).

## The recorded representation — Candidate A, dotted-circle base

The fallback unit shown in the cell is `◌` (U+25CC DOTTED CIRCLE, UTF-8
`E2 97 8C`) followed by the cluster's original combining-mark bytes, so
the marks compose onto the conventional "no base" carrier and stay
visually identifiable. Example: a line whose first content is `U+0301`
(combining acute, `CC 81`) followed by `x` displays as
`E2 97 8C CC 81 78` — `◌́x`. The dotted circle is display-only: no
source byte produces it. Rejected candidates: a space base (marks on an
invisible carrier are harder to identify and less consistent across
terminals) and a fixed placeholder such as U+FFFD (discards the marks'
identity and conflates them with the invalid-UTF-8 replacement glyph).

## One-cell propagation

In `safepresentation.MapContent`, a grapheme cluster reporting width
under 1 emits `unit("◌"+cl, 1, s, e)`: one `Cell` carrying the composed
text and one `Cluster{start, start+1}` — the same record shape a real
one-cell glyph produces. `Mapped.Text` gains the three inserted bytes
ahead of the mark bytes and the cell position advances by one, so the
following cluster owns the next cell with no overlap and no shared
cell. Every downstream layer then sees an ordinary cell:

- `wrapLine` counts the fallback toward the row's width and moves it
  whole like any cluster;
- run-off-edge clipping and the `off` pan window drop it whole or
  paint it — a one-cell cluster can never straddle;
- `Line.MaxStart`/`MaxOff` count it as a paintable-boundary candidate;
- `expandToClusters` snaps a match on the mark bytes to the fallback's
  `[c, c+1)` range, so the highlight paints exactly that cell and
  nothing adjacent;
- `contentText` paints the `◌`-plus-marks text in the cell like any
  other, styled when the span covers it.

## Normalization rule for unexpected widths

The fallback cell is *constructed* as one cell, never re-measured: the
cluster table pins the fallback unit's width to 1, so a width-library
report of 0 or greater than 1 for a pathological mark sequence — or a
terminal that composes the unit inconsistently — cannot change the
recorded cell geometry. Byte→cell maps, wrapping, clipping, highlight
expansion, panning, and the renderer all consume that recorded width.

## Byte-mapping contract

Only the *display* gains the fallback cell: the cell's
`Cell{Start, End}` remains the cluster's original source byte range, so
`CellsCovering` resolves the mark's bytes to the fallback cell and the
next byte to the following cell. The inserted `◌` bytes carry no
source-byte mapping; match/highlight mapping, Issue #29 stale
validation, and raw-byte coordinates are unaffected.

## Tests

`internal/filebuffer/filebuffer_test.go`:
`TestHighlightStandaloneCombiningGetsFallbackCell` pins the one-cell
`◌́` form and its `[0,1)` highlight;
`TestStandaloneCombiningFallbackCellGeometry` asserts the display byte
sequence, the one-cell cluster, the preserved source-byte mapping, and
the next cluster's ownership of the next cell — for a line-opening mark
and a two-mark cluster alike;
`TestStandaloneCombiningFallbackHighlightNeverAdjacent` highlights
exactly the fallback cell of a mid-line standalone mark, never the
adjacent cells.
`internal/viewport/wrap_test.go`:
`TestWrapCountsStandaloneCombiningFallbackCell` proves the extent and
both row models count the fallback like any other cell.
`internal/app/grapheme_test.go`:
`TestStandaloneCombiningFallbackCellHighlighted` asserts the composed
frame's styled `◌́` run closes before the following `x` — the next cell,
unstyled — and
`TestStandaloneCombiningFallbackPansAsOneCell` proves one pan step
hides the cell whole, shifts the following text one cell left, and
upgrades the gutter mark to `*` for the entirely hidden match. See
[unit-tests.md](unit-tests.md).
