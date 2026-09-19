# Grapheme-cluster highlight expansion — spans snap to whole clusters (Issue #21)

Delivered by
[Issue #21](../issues/021-grapheme-cluster-highlight-expansion.md)
([task](../tasks/021-grapheme-cluster-highlight-expansion.md)): every
nonempty match span FileBuffer records expands outward to the complete
grapheme clusters it touches — a partial-cluster byte match highlights
the whole glyph — and rendering never styles the blank cells wrapping
or clipping introduce, so a highlight is always painted glyph cells or
nothing. Relevant PRD sections: *Text, graphemes, and safe
presentation* (the cluster-expansion, wrap-boundary, and clip-blank
bullets) and *Layout and indicators* (indicator visibility over
actually painted match cells) in
[`Notes/PRD-vrg.md`](../PRD-vrg.md); user story 72. Builds on
[safe-presentation.md](safe-presentation.md) (`Mapped.Clusters` — the
shared segmentation and cell-width policy — plus the Issue #43 `◌`
fallback for standalone combining clusters),
[wrap-mode.md](wrap-mode.md) (cluster-boundary wrapping and the
blank-cell rule for an unfit cluster),
[horizontal-panning.md](horizontal-panning.md) (the `off` clip window),
[horizontal-reveal.md](horizontal-reveal.md) (painted-cell
`CellVisible` and the reveal arithmetic consuming the same spans), and
[hidden-content-indicators.md](hidden-content-indicators.md) (the
`_`/`*` marks judging those spans).

## The single span source — `Line.Highlights`

`makeLine` maps each recorded highlight byte range through
`Line.CellsCovering`, then snaps the result outward with the new
`Line.expandToClusters`: a range touching any cell of a cluster covers
that cluster's whole `[Start, End)` cell range. A span producing no
cells — a zero-width position or a terminator-only range — now lands
on the marker position Issue #22's
[coordinate-separated mapping](line-structure.md) defines (the display
end-of-line position for terminator bytes), recorded as an empty cell
span — [Issue #23](zero-width-markers.md)'s marker position.

The recorded `Line.Highlights` cell spans are the only highlight
source the pipeline consumes:

- `contentText` paints match runs straight from them;
- `LeftMark`/`RightMark` judge hidden-left/hidden-right from them, so
  a match whose recorded bytes begin mid-cluster is indicator-counted
  from the cluster's start cell;
- `StopTarget` derives the reveal target from the first submatch's
  recorded bytes mapped through `CellsCovering` — which lands on the
  cluster's first cell because every cell of a cluster shares the
  cluster's byte range — preserving the first-submatch marker-position
  rule (Issue #14).

`CellsCovering` already returned cluster-aligned ranges in practice —
all cells of a cluster carry the cluster's byte span, so any byte
overlap covers every cell. `expandToClusters` makes that contract
explicit rather than incidental to the shared mapper's cell/byte
bookkeeping.

## What expansion covers

- **Partial-cluster spans** — a start inside a cluster, an end inside
  a cluster, or both (within one cluster or across several) snap
  outward to whole clusters.
- **Combining-only matches** — a match on only the combining-mark
  bytes of a decomposed glyph (`e` + `U+0301`) highlights the whole
  base cluster: the é glyph styles as one unit.
- **Standalone combining clusters** — a cluster with no base or
  independent visible cell keeps the recorded Issue #43 fallback: `◌`
  (U+25CC) plus the original combining-mark bytes occupying exactly
  one cell whose byte mapping resolves to the source bytes, so the
  highlight is never an inaccessible zero-cell span.
- **Wide glyphs never split** — a match touching any byte of a
  two-cell cluster covers both cells; emoji ZWJ sequences are one
  cluster under the shared `rivo/uniseg` policy and expand the same
  way.

## Filler blanks are never match cells

Two kinds of synthetic blanks exist beside real mapped cells, and
neither may carry the match style:

- **Wrap-boundary blanks** — a cluster that cannot fit a row's
  remaining cells moves to the next row and the shortfall pads blank
  (Issue #16); the filler is appended after the row's cells, outside
  every span.
- **Clip blanks** — a cluster straddling a clip edge contributes one
  blank per in-window cell instead of a partial glyph (Issue #18).
  `contentText` now detects the straddle from the cluster record
  (`cl.Start < lo || cl.End - lo > textW`) rather than inferring it
  from byte ranges, and forces such a cell's text to an unstyled
  blank — previously a clipped cluster's in-window blank inherited the
  match style. The check precedes the continuation-cell skip, so a
  cluster wider than the whole window renders all in-window cells
  blank, matching the unpaintable-cluster geometric fallback.

## Tests

`internal/filebuffer/filebuffer_test.go` gains the expansion tables:
partial-cluster snapping in every boundary position over decomposed
`e\u0301` and wide `文`/`日` clusters, the combining-only match
expanding to its base cluster, the standalone combining mark's `◌`
fallback cell highlighted, the wide pair never split, and a
`👨‍👩‍👧` ZWJ sequence expanding as one cluster.

`internal/app/grapheme_test.go` (new, same package) drives the
rendering half: clip blanks unstyled at both clip edges (including a
split tab expansion), the wrap-boundary filler blank unstyled while
the wrapped cluster highlights whole on its own row, a combining-only
match painting the whole é, the `◌` fallback cell highlighted, and a
CJK match painting both cells.

`internal/viewport` gains `loadBufferStops` so indicator tests build
rows through the real FileBuffer path, and `markRowOf` +
`TestMidClusterMatchCountsFromClusterBoundary` verify the marks judge
expanded spans — a byte match inside `文` arrives as cell span `[2,4)`
and counts hidden left/right from the whole cluster. See
[unit-tests.md](unit-tests.md).
