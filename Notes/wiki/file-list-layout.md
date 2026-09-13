# File list layout, truncation, and toggle (Issue #24)

Issue #24 replaces the Issue #5 placeholder file-list width with a
responsive formula, adds hide/show toggles, grapheme-safe left
truncation, active-entry auto-scroll, a filename-row buffer-status
note slot, and routes every text-width change through Issue #17's
prepared-layout path while preserving the logical reading anchor.

## List width formula

`ComputeListWidth(termWidth, longestPathWidth, gutterWidth,
reservedIndicator, visible)` returns the nonnegative minimum of:

- `longestPathWidth + 2` (the longest sanitized path plus a two-cell
  margin),
- `floor(0.40 × termWidth)` (the 40% cap, integer floor),
- `termWidth − (gutterWidth + 10 + reservedIndicator)` (leaves at
  least ten content text cells plus the reserved indicator width for
  the content panel).

The result is clamped to be nonnegative. When `visible` is false the
result is 0. The reserved indicator width is
`viewport.ReservedWidth(m.wrapMode)` (0 in wrap mode, 1 in run-off-edge
mode — see [wrap-mode-and-grapheme-policy](wrap-mode-and-grapheme-policy.md)
and [hidden-content-indicators](hidden-content-indicators.md)).

`longestPathWidth` is computed by `computeLongestPathWidth`, which
iterates the search index stops, sanitizes each distinct path through
`safepresentation.EscapePath`, and measures its terminal cell width
using the shared grapheme-cluster policy
(`safepresentation.GraphemeClusters` + per-cluster `Width`).

## Visibility preference vs. computed width

The visibility preference (`listVisible bool`, initially true) is
distinct from the computed width. `ListWidth()` returns 0 when the
preference is false, but a computed zero width (e.g. a very narrow
terminal) does not change the preference and does not automatically
toggle the list. When the list is requested visible but the computed
width is zero, no list cells are drawn and the preference remains
visible.

## Toggle keys

- `left` arrow or `tab` → hide the list.
- `right` arrow or `shift+tab` → show the list.
- Startup is shown.

`toggleListVisible(visible)` updates the preference and, when a
buffer is loaded, invokes `buildViewport()` so the relayout goes
through Issue #17's prepared-layout path (see
[logical-anchor-and-layout-preparation](logical-anchor-and-layout-preparation.md)).
The logical reading anchor is preserved through the relayout.

## Recomputation triggers

The list width is recomputed (and the layout rebuilt when a buffer is
loaded) whenever:

- Loading changes the gutter (`FileLoadCompleteMsg` rebuilds via
  `buildViewport`).
- The terminal size changes (`WindowSizeMsg` rebuilds via
  `buildViewport`).
- The wrap/layout mode changes (`w` toggle rebuilds via
  `buildViewport`).
- The list visibility changes (`toggleListVisible` rebuilds via
  `buildViewport`).

`LayoutKey()` uses `m.ListWidth()` for the panel width, so every
text-width change caused by this issue flows through Issue #17's
prepared-layout path.

## Grapheme-safe left truncation

`TruncateLeftGrapheme(s string, maxCells int) string` left-truncates
a display string to fit `maxCells` terminal cells, inserting a
leading `…` (one cell) when truncation occurs. It segments the string
through `safepresentation.GraphemeClusters` and accumulates trailing
clusters until the next cluster would exceed the budget. Wide
glyphs, combining marks, and multi-cell escaped forms are never
split — a cluster that does not fit moves entirely out of the
kept window. When `maxCells <= 0` the result is empty.

## Active-entry auto-scroll

The file-list viewport keeps the active entry visible. The list
offset (`listOffset int`) is the 0-based top row of the visible
file-list window. `updateListOffset(currentFileIdx, visibleRows)`
scrolls up when the current file is above the window and scrolls
down (placing the current file at the bottom) when it is below. It
is called from `handleNavigate` (on every actual navigation) and
from `renderBrowse` (so the initial render and any state change
converge). `currentFileIndex(path)` derives the 0-based file-group
index from the cursor's current stop.

## Visible-window-only rendering

`renderBrowse` queries the file-list provider only for the
`[listOffset, listOffset+visibleRows)` window (Issue #17 render-cost
guard, re-verified with the Issue #24 layout in place). When the
computed list width is zero, no list cells are drawn.

## Filename-row status-note slot

`renderFilenameRow(escapedName)` renders the filename row with a
buffer-status note slot at the right. The path is left-truncated
(through `TruncateLeftGrapheme`) to fit the content panel width,
reserving space for the note plus separators when a note is present.
When no note is set, the path is still truncated to fit the panel
width. The real note texts are owned by Issues #26 (unreadable),
#29 (stale), and #30 (unsupported); this issue implements and tests
the slot and truncation with a synthetic status string injected via
the `WithStatusNote(func() string)` test seam.

## Pathological dimensions

`ComputeListWidth` clamps the result to be nonnegative, so a very
narrow terminal (or a very large gutter) produces a zero list width
rather than a negative panel width. The content panel width
(`m.width - m.ListWidth() - 1`) is clamped to at least 1 in
`renderFilenameRow` and the viewport's `TextWidth` clamps the text
width to at least 1 (see
[wrap-mode-and-grapheme-policy](wrap-mode-and-grapheme-policy.md)).

## Anchor preservation

All text-width changes caused by this issue (resize, wrap toggle,
load completion, hide/show) route through `buildViewport`, which
preserves the logical reading anchor (`viewport.Anchor{LineIndex,
Column}`) across rewrap and resize (see
[logical-anchor-and-layout-preparation](logical-anchor-and-layout-preparation.md)).
The anchor is a width-independent (source line, column) position, so
a relayout that changes the list width lands the top on the same text
location in the new row model.

## App integration

- `listVisible bool` (initially true), `listOffset int`,
  `longestPathWidth int`, `statusNote func() string` fields.
- `ListVisible()`, `ListWidth()`, `ListOffset()`, `ViewportAnchor()`
  accessors.
- `ComputeListWidth`, `TruncateLeftGrapheme`, `computeLongestPathWidth`
  helpers.
- `toggleListVisible`, `updateListOffset`, `currentFileIndex`,
  `renderFilenameRow`, `graphemeCellWidthString` methods.
- `LayoutKey()` uses `m.ListWidth()` for the panel width.
- `handleNavigate` calls `updateListOffset` on every actual
  navigation.
- `renderBrowse` uses `m.ListWidth()`, auto-scrolls via
  `updateListOffset`, queries only visible provider entries, applies
  `TruncateLeftGrapheme`, and skips list rendering for zero width.
- `renderContentPanel` delegates its first line to
  `renderFilenameRow`.
- The old Issue #5 placeholder `fileListWidth` function was removed.

See [browse-tracer](browse-tracer.md),
[wrap-mode-and-grapheme-policy](wrap-mode-and-grapheme-policy.md),
[logical-anchor-and-layout-preparation](logical-anchor-and-layout-preparation.md),
and [horizontal-panning](horizontal-panning.md).
