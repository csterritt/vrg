# Wrap mode and the `w` toggle (Issue #16)

Delivered by
[Issue #16](../issues/016-wrap-mode-and-toggle.md)
([task](../tasks/016-wrap-mode-and-toggle.md)): the file panel wraps
long lines by default at grapheme-cluster boundaries, `w` toggles the
session between wrapped rows and run-off-edge clipping, tabs expand
structurally to eight-column stops, and the rendered-row model becomes
a keyed, swappable value prepared once per layout rather than
recomputed by `View()`. Relevant PRD sections: *Text, graphemes, and
safe presentation* and *Layout and indicators* (the text-width and
indicator bullets) in [`Notes/PRD-vrg.md`](../PRD-vrg.md). Builds on
[viewport-scrolling.md](viewport-scrolling.md) (the prepared `Rows`
model and rendered-row scroll units),
[destination-reveal.md](destination-reveal.md) (the reveal the wrapped
rows feed), and [safe-presentation.md](safe-presentation.md) (the
escaping core the segmentation extends).

## Shared grapheme policy (`internal/safepresentation` + `internal/filebuffer`)

`Mapped` gains `Clusters []Cluster` — each `Cluster{Start, End}` is one
grapheme cluster's half-open cell range, so `End − Start` is its
terminal cell width. `MapContent` populates the segmentation as it
escapes: each character of an escaped form and each invalid-byte U+FFFD
replacement is its own single-cell cluster, while a wide glyph is one
multi-cell cluster whose continuation cells stay blank, and a tab
expansion is one unbreakable multi-cell cluster. `filebuffer.Line`
embeds `Mapped`, so `Line.Clusters` is the shared boundary set every
consumer uses — Viewport wraps from these ranges and never re-segments
Unicode itself (the Issue #39 shared-cell-model direction).

Tabs now expand **structurally**: `8 − (cell position mod 8)` blank
cells to the next multiple of eight source-display columns, each cell
mapping back to the tab's byte range so a match covering the tab
highlights the whole expansion. Because the position is the line's own
cell column, stops never shift with gutter width or horizontal pan. The
Issue #5 provisional single-cell `→` placeholder is gone.

## The two row models (`internal/viewport`)

`Prepare(buf, Key)` builds the model the frame render slices, under the
layout the key describes:

- `Key{Path, Revision, TextWidth, Wrap}` — the exact inputs a model was
  built for. `Rows.Key()` reports it; a change in any field makes the
  model stale. The model is a **swappable value**: Issue #17 can
  prepare a replacement off the update path and swap it in while the
  key still matches the live layout. Issue #16 prepares synchronously.
- **Wrap mode** partitions each source line's cells into rows of at
  most `TextWidth` cells, broken only at cluster boundaries. A cluster
  that does not fit in the row's remaining cells starts the next row,
  so a row may end with blank cells but never with a split cluster —
  the blank-cell rule for an unfit wide glyph. An empty line still
  yields one row.
- **Run-off-edge mode** maps each source line to exactly one row; the
  render clips at the text width without splitting a grapheme.
- `Row{Line, Start, End}` is one rendered row — a half-open cell range
  of its source line — and `Row.Continuation()` (Start > 0) marks the
  rows that continue a wrapped line: their gutter is blank and their
  text aligns with the first row's column.
- `Rows` keeps the spans plus `firstRow`, a line-index → first-row map
  with a final sentinel, so line *i* occupies
  `rows[firstRow[i] : firstRow[i+1]]` — the index `TargetRow` scans.
- `ReservedIndicator(wrap)` is the file panel's reserved right-edge
  width: **zero while wrapping, one in run-off-edge mode**. The column
  is reserved now — rendered blank — and populated by Issue #20.

## App wiring (`internal/app`)

- `model.wrap` starts **true**; `w` (browse state only) flips it and
  rebuilds every prepared row model under the new key — the text width
  itself changes with the reserved indicator column.
- `model.revs` is the per-path content revision: it bumps on every
  successful `fileLoadedMsg`, so a reload's row model never aliases the
  old content's.
- `textWidth(gutter)` = frame width − file-list width − gutter −
  `ReservedIndicator(wrap)`; all wrapping, clipping, and reveal math
  uses it.
- `rebuildRows` swaps in a fresh `viewport.Prepare` for every cached
  buffer whose key changed — after a `w` toggle or a `WindowSizeMsg` —
  then re-clamps each saved `vps` viewport to the new row count.
- `contentText` renders a row's `[Start, End)` cells; `contentCell`
  emits a blank gutter for continuation rows and pads the reserved
  indicator column. A frame still queries `rowSource` only for the
  visible range — `View()` never wraps the buffer.

## Wrapped destination reveal

`Rows.TargetRow` now maps the Issue #14 display target into the wrapped
rows: it scans the destination line's rows for the span containing the
target's start cell, so a match deep inside a wrapped line reveals its
own row — not the line's first — under the unchanged visible-target
no-scroll and one-third placement rules. A target cell past the line's
cells lands on the line's last row, the end-of-line marker position
(Issue #23).

## Tests

`internal/viewport/wrap_test.go` (external package): ASCII wrap row
counts, a wide cluster moving whole to the next row leaving blank
cells, combining sequences kept together, the tab as one unbreakable
cluster, rows aligning to cluster boundaries, run-off-edge one row per
line, the key carried by the model, wrapped `TargetRow`, and a reveal
landing deep in a wrapped line. `internal/app/wrap_test.go` (same
package): wrap on by default with blank continuation gutters, the `w`
toggle's row-model swap and reserved column, a match near the end of a
wrapped line revealed on its own continuation row, and the resize
rebuild. `internal/filebuffer` and `internal/safepresentation` gained
the cluster/tab-stop tests. See [unit-tests.md](unit-tests.md).
