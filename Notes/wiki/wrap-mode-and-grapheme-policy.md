# Wrap mode and grapheme policy (Issue #16)

Issue #16 adds wrapping on by default, a `w` key toggle between wrap
and run-off-edge modes, grapheme-aware layout, eight-column tab stops,
a prepared swappable row model keyed by (path, content revision, text
width, wrap mode), integration with the Issue #14 destination reveal,
and the reserved right-side indicator column for the future Issue #20
overflow indicator.

## Shared grapheme segmentation and cell-width policy

`internal/safepresentation` owns the one shared grapheme segmentation and
terminal cell-width policy. `Cluster{StartByte, EndByte, Width}` is the
canonical grapheme cluster type. `GraphemeClusters(display string)
[]Cluster` segments an escaped display string into grapheme clusters
using `github.com/rivo/uniseg` and computes each cluster's terminal cell
width via `uniseg.StringWidth`. `filebuffer.Cluster` is an alias for
`safepresentation.Cluster` so callers can use `filebuffer.Cluster`
without importing `safepresentation` directly.

`filebuffer.Load` populates `Line.Clusters` by calling
`safepresentation.GraphemeClusters` on each line's escaped display text.
The viewport consumes those clusters for wrapping without re-deriving.

The policy handles ASCII (width 1), wide characters (width 2), combining
marks (width 0, attached to the base cluster), wide characters with
combining marks (one cluster, width 2), and mixed text.

## Tab expansion to eight-column stops

`EscapeContent` expands tabs to the next multiple of 8 source-display
columns, replacing Issue #5's provisional `→` placeholder. Tab stops
count from column 0 (the start of line content), independent of the
gutter and the horizontal pan. A tab at column 0 expands to 8 spaces; a
tab at column 1 expands to 7 spaces; consecutive tabs advance to
successive eight-column stops. The byte→cell map records the expanded
cell range for each original tab byte.

## Wrap and run-off-edge modes

`viewport.WrapMode` selects wrap or run-off-edge row-model construction.
`WrapOn` (the default) wraps long lines at grapheme-cluster boundaries
to fit the text width. `WrapOff` is run-off-edge mode: each source line
is one rendered row, clipped by the text width, with a reserved
right-indicator column. `WrapMode.Toggle()` returns the opposite mode.

`ReservedWidth(mode)` returns 0 in wrap mode and 1 in run-off-edge mode.
The reserved column is unpopulated for now; Issue #20 will populate it.

`TextWidth(panelWidth, gutterWidth, mode)` returns the number of display
cells available for text: panel width minus gutter width minus the
reserved indicator width, clamped to at least 1 cell.

## Row model

`viewport.RowModel` is the prepared, swappable row model for a file at a
given text width and wrap mode. It stores the rendered rows, source
mappings, and its key. It implements `RowProvider` so the viewport
queries only the visible row range.

`RowModelKey{Path, Revision, TextWidth, WrapMode}` identifies a row
model. Two models with the same key produce the same rows. The key
allows Issue #17 to move preparation off the UI update path without
restructuring it.

`BuildRowModel(buf, textWidth, mode, key)` constructs the row model. In
wrap mode, long lines are broken at grapheme-cluster boundaries; a
two-cell cluster that cannot fit in the remaining row cells moves to
the next row, leaving the remaining cells blank. In run-off-edge mode,
each source line is one rendered row. Continuation rows carry
`Continuation=true` and a blank gutter.

`RowFromByte(lineIndex, byteOffset)` maps a 0-based source line index
and byte offset to the 0-based rendered row containing that byte, or -1
if not found. Used by the Issue #14 target reveal to find the wrapped
row containing the match start.

`Viewport.RowCount()` returns the total number of rendered rows in the
viewport's row provider.

## Wrapped destination reveal

`targetRow(stop)` now uses the row model's `RowFromByte` to map the
first submatch's byte start to the wrapped row containing the match.
In run-off-edge mode (or when the row model is unavailable), it falls
back to the 0-based source line index. The reveal behavior (visible-
target no-scroll, one-third placement with BOF/EOF precedence) is
unchanged from Issue #14.

## `w` toggle and app integration

The app carries a `wrapMode viewport.WrapMode` field (initial `WrapOn`)
and a `rowModel *viewport.RowModel` field. `buildViewport()` constructs
the viewport's row provider from the current buffer, wrap mode, and
panel dimensions: when a `RowProviderFactory` test seam is set it is
used directly; otherwise a `RowModel` is built at the current text width
and wrap mode. The per-file saved offset is restored after building.

The `w` key toggles the wrap mode in browse mode (when a buffer is
loaded) and rebuilds the viewport. `w` is a no-op outside browse mode.
`WindowSizeMsg` rebuilds the row model when the text width changes so
wrapping reflects the new panel width, preserving the current offset.

`renderContentPanel` shows a blank gutter for continuation rows
(`line.Continuation`), aligned with the first row's text.

See [browse-tracer](browse-tracer.md),
[manual-vertical-scrolling](manual-vertical-scrolling.md), and
[destination-reveal](destination-reveal.md).
