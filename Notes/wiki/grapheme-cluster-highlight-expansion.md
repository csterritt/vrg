# Grapheme cluster highlight expansion

Issue #21: match highlights expand to grapheme-cluster boundaries so
they never split a cluster, combining-only matches highlight the whole
base cluster, standalone zero-width clusters receive a visible fallback
cell, wide glyphs are never split, and wrap/clip blank filler cells are
never painted as match cells. The expanded spans are the single source
for highlights, reveal, and indicator visibility.

Cross-references: [wrap-mode-and-grapheme-policy](wrap-mode-and-grapheme-policy.md)
(Issue #16 shared grapheme policy), [browse-tracer](browse-tracer.md)
(FileBuffer Load and rendering), [horizontal-reveal](horizontal-reveal.md)
(Issue #19 reveal consumes ByteCells), [hidden-content-indicators](hidden-content-indicators.md)
(Issue #20 indicators consume Highlights), [safe-presentation](safe-presentation.md)
(EscapeContent and GraphemeClusters). PRD: `Notes/PRD-vrg.md` "Text,
graphemes, and safe presentation".

## Motivation

Before Issue #21, `filebuffer.Load` converted each `Stop.Submatch` to a
display-cell span by taking the first submatch byte's cell start and
the last submatch byte's cell end directly from
`safepresentation.EscapeContent`'s `ByteCells`. That mapping is
1-cell-per-rune: a combining mark gets its own cell, a CJK ideograph's
partial bytes each map to the same cell, and a ZWJ joiner gets its own
cell. The resulting highlight could:

- Split a wide glyph (highlight one cell of a two-cell CJK character).
- Highlight only the combining mark, leaving the base character
  unhighlighted (a combining-only match produced a zero-width or
  misplaced highlight).
- Produce a zero-cell highlight for a standalone combining mark with no
  base (width 0).
- Inconsistently feed the Issue #19 reveal (which targets
  `ByteCells[sm.Start][0]`) and the Issue #20 indicators (which derive
  visibility from `Highlights` and cluster geometry).

Issue #21 makes highlights grapheme-safe by expanding each submatch's
byte range to the enclosing grapheme-cluster boundaries before storing
`Line.Highlights` and `Line.ByteCells`.

## Outward expansion to cluster boundaries

`filebuffer.Load` now:

1. Escapes each raw line through `safepresentation.EscapeContent`,
   producing `ContentDisplay` with `Text`, `ByteCells`, and (Issue #21)
   `ByteOffsets`.
2. Segments the display text into grapheme clusters via
   `safepresentation.GraphemeClusters`.
3. Remaps `ByteCells` through `expandedByteCells` so every byte in a
   cluster (including combining marks and ZWJ joiners) maps to the
   cluster's full cell range.
4. Converts each submatch to a display-cell span through
   `expandedHighlights`, expanding outward to the enclosing clusters'
   cell boundaries.

`expandedHighlights` starts from the original cell range
(`ByteCells[sm.Start][0]` to `ByteCells[sm.End-1][1]`) and replaces the
start with the start cluster's cell start and the end with the end
cluster's cell end. The expansion only grows the range outward to
cluster boundaries; it never shrinks it. This preserves multi-cell
escaped forms (e.g. ESC → `^[` covers both cells) while expanding
partial-cluster matches (e.g. a combining-only match expands to the
base cluster).

The distinction between "replace with cluster range" and "preserve
original range" is the `spanMultiCluster` check: a raw byte whose
display text spans multiple grapheme clusters (like ESC producing `^`
and `[` as two clusters) keeps its original multi-cell range; a raw
byte whose display text sits within one cluster (like a combining mark
inside a base+combining cluster) is replaced with the cluster's range.

## Combining-only match behavior

A submatch covering only the combining mark bytes (e.g. the `\u0301` in
`e\u0301`) expands to the whole base-plus-combining cluster. The
combining mark's bytes map (via `ByteOffsets`) to the cluster
containing the base, so `expandedHighlights` replaces the combining
mark's cell range with the cluster's cell range, and `expandedByteCells`
remaps the combining mark's `ByteCells` entry to the cluster's cell
range. The Issue #19 reveal targets the cluster start (the base), and
the Issue #20 indicators see the whole cluster as the highlight.

## Standalone-cluster fallback cell

A standalone cluster with width 0 (a combining mark with no base, no
independent visible cell) receives a visible fallback cell of width 1
so the highlight is never zero cells. `expandedByteCells` and
`expandedHighlights` both compute cluster cell ranges with
`width = max(c.Width, 1)`, so a zero-width cluster's cell range is
`[cellStart, cellStart+1)` instead of `[cellStart, cellStart)`.

## Wide glyphs never split

A wide glyph (2 cells, e.g. a CJK ideograph) is one grapheme cluster.
A submatch partially covering the wide cluster expands to both cells.
The cluster's cell range is `[cellStart, cellStart+2)`, so the highlight
covers both display cells and is never split by the highlight boundary.

## Emoji ZWJ sequences

An emoji ZWJ sequence (e.g. `👨‍👩‍👧`) is one grapheme cluster by the
shared `uniseg` policy. A submatch partially covering the sequence
(e.g. matching only the ZWJ character) expands to the entire sequence's
cell range, following the same outward-expansion rule as any other
cluster.

## Wrap and clip blank filler cells never painted

Wrap mode introduces blank filler cells when a wide cluster cannot fit
in the remaining row cells (the cluster moves to the next row, leaving
blanks). Clip mode introduces blank filler cells when a cluster is split
by the left or right clip edge (the split portion renders as blanks).

Issue #21 ensures these blank filler cells are never painted as match
cells:

- **Wrap**: `viewport.adjustHighlights` shifts the source line's
  highlight cell ranges to each wrapped row's local coordinates, clamped
  to the row's cell extent. The wrap blank filler cells are outside the
  row's cluster range, so the highlight does not cover them.
- **Clip**: `viewport.clipLineToWindow` now tracks `paintableRanges` —
  the source-line cell ranges of fully-visible (non-split) clusters —
  and `clipHighlightsToPaintable` intersects each highlight with the
  paintable ranges before shifting to local coordinates. Split-blank
  filler cells are not in any paintable range, so they are never
  highlighted.

The previous `clipHighlights` helper (which only shifted and clamped
to the window) is replaced by `clipHighlightsToPaintable` (which also
intersects with paintable ranges).

## Expanded spans as the single source

`Line.Highlights` and `Line.ByteCells` are the sole source of highlight
spans for Viewport and App:

- **Viewport wrapping**: `adjustHighlights` consumes `Line.Highlights`
  (the expanded spans) when building wrapped rows.
- **Viewport clipping**: `clipHighlightsToPaintable` consumes the
  clipped row's `Highlights` (derived from the expanded spans).
- **App rendering**: `renderLineWithHighlights` consumes `Line.Highlights`
  to apply match styling.
- **Issue #19 reveal**: `revealTarget` reads `line.ByteCells[sm.Start][0]`
  for the horizontal reveal target cell. With `expandedByteCells`, the
  combining mark's `ByteCells` entry maps to the cluster start, so the
  reveal targets the cluster start.
- **Issue #20 indicators**: `leftIndicator`, `hasHiddenMatchRight`, and
  `highlightHasVisibleCells` consume `Line.Highlights` and
  `Line.Clusters`. With expanded spans, a mid-cluster match whose
  cluster start is hidden left counts as hidden-left (`*`), and a
  partially visible cluster (not split) counts as visible.

## safepresentation ByteOffsets

Issue #21 added `ByteOffsets []int` to `safepresentation.ContentDisplay`.
`ByteOffsets[i]` is the display byte offset where original byte `i`
starts in `Text`. This lets `filebuffer` map raw bytes to grapheme
clusters by display byte range (the cluster whose `[StartByte, EndByte)`
contains `ByteOffsets[i]`), which is necessary because `ByteCells` gives
cell ranges (not byte ranges) and a combining mark's cell is outside its
cluster's cell range (the cluster's width is the base's width only).

`EscapeContent` populates `ByteOffsets` alongside `ByteCells` and
`Text`. Line-terminator bytes (LF, CRLF) produce no display text; their
`ByteOffsets` point at the end of the display text (equal to the last
display byte offset).
