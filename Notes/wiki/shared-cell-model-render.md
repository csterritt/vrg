# Rendering from the shared grapheme/cell model (Issue #39)

Issue #39 makes final rendering use the shared grapheme/cell model end
to end. FileBuffer and Viewport already produced grapheme-aware,
cell-based ranges, but the final render path still re-derived geometry
with rune-count assumptions: a two-cell CJK highlight consumed the
following character, combining sequences were styled, clipped, or
truncated separately from their base, emoji ZWJ sequences were split,
and list, overlay, filename, pop-up, and centering widths were
computed per rune.

Depends on [Issue #38](../issues/038-viewport-content-panel-width.md)
(the installed viewport text width). This page cross-references:

- [safe-presentation](safe-presentation.md) — the package that owns
  the shared helper.
- [wrap-mode-and-grapheme-policy](wrap-mode-and-grapheme-policy.md) —
  the upstream `Cluster`/`GraphemeClusters` policy the renderer now
  consumes directly.
- [viewport-text-width](viewport-text-width.md) — the panel/text-width
  chain the cell measurements feed.
- [theme-module](theme-module.md) — overlay sizing and padding.
- [file-list-layout](file-list-layout.md) — list-entry padding and
  filename-row fitting.
- `Notes/issues/039-render-from-shared-grapheme-cell-model.md` — the
  issue.
- `Notes/PRD-vrg.md` — the *Text, graphemes, and safe presentation*
  and *Navigation, viewport, and logical anchors* sections.

## The shared ANSI-aware cell-width helper

`internal/safepresentation/cellwidth.go` is the single display-geometry
policy for the final render path:

- `GraphemeClustersANSI(s) []Cluster` — ANSI-aware segmentation:
  recognized CSI escape sequences become zero-width clusters at their
  byte positions; the text between them is segmented by the shared
  `GraphemeClusters` policy (`rivo/uniseg`). ANSI bytes therefore never
  contribute cells and never split segmentation.
- `CellWidth(s) int` — total terminal cell width; ANSI escapes
  contribute zero.
- `TruncateLeftCells(s, keep) string` — keeps the trailing `keep`
  cells; never splits a cluster (a cluster straddling the cut drops
  out whole); zero-width units — ANSI sequences and zero-width
  clusters — do not consume the budget and are preserved when adjacent
  to kept text.

The file is the sole production-file allow-list location for
`utf8.DecodeRuneInString`: a static guard test
(`internal/app/decode_guard_test.go`) scans every non-test `.go` file
under `internal/` and `cmd/` recursively — including files and
packages added later — and fails when the symbol appears anywhere else.
This keeps future display-geometry code from bypassing the shared
policy.

## Cluster-driven file-panel renderer

`renderLineWithHighlights` renders each clipped row directly from
`line.Clusters` — the cell spans produced upstream by FileBuffer and
Viewport — instead of re-deriving byte positions from runes
(`cellToBytePos` is gone). It walks the cluster table once: a cluster
whose cell span intersects a highlight range is styled whole; adjacent
covered clusters merge into one styled span; uncovered runs pass
through unstyled. Boundaries therefore always land on cluster
boundaries: a two-cell CJK character, a base-plus-combining sequence,
and an emoji ZWJ sequence each move as one unit, and a highlight can
never swallow the following character or leave a partial glyph styled.
The renderer no longer re-escapes `line.Display` — the display text is
already sanitized upstream.

## Consumers routed through the helper

Every final-render display-geometry consumer now uses the shared
helper:

| Consumer | Now uses |
| --- | --- |
| `renderLineWithHighlights` | `line.Clusters` cell spans |
| `visibleWidth` | `safepresentation.CellWidth` |
| List-entry padding (`renderBrowse`) | `visibleWidth` → `CellWidth` |
| Indicator-column sizing (`clusterCellWidth`, `highlightHasVisibleCells`) | cluster cell spans (unchanged) |
| Filename-row fitting (`renderFilenameRow`) | `TruncateLeftGrapheme`, `CellWidth` |
| Pop-up truncation (`renderPopup`) | `safepresentation.TruncateLeftCells` (replaces `truncateLeftCells`) |
| Pop-up width and centering | `visibleWidth` → `CellWidth` |
| `Theme.Overlay` sizing and padding (`cellWidth`) | `safepresentation.CellWidth` |
| Overlay/help wrapping (`wrapText`/`wrapLine`) | `GraphemeClustersANSI` |
| `TruncateLeftGrapheme` (list entries, filename row) | `CellWidth` + `TruncateLeftCells` |
| `truncateRightCells` (status-note truncation) | `GraphemeClustersANSI` |
| `computeLongestPathWidth` (list width term 1) | `CellWidth` |

The rune-counting `truncateLeftCells` was replaced by
`safepresentation.TruncateLeftCells`, removing the false assumption
that `EscapePath` output contains no combining marks.

## Consequences

- Highlights, clip boundaries, truncation cuts, and wrap breaks all
  land on grapheme-cluster boundaries; wide clusters occupy their
  measured cells; combining sequences stay attached to their base;
  emoji ZWJ sequences stay intact; ANSI escapes contribute zero cells.
- The file list, indicator column, filename row, pop-up, and overlay
  all measure the same way, so padding, borders, and centering align
  under wide or combining content.
- Issue #38's panel/text-width behavior is preserved: the widths are
  the same, only the measurement is now cell-accurate.
- The helper and the width-independent escaped-path/cluster geometry
  are the handoff for Issue #40 (moving grouping work out of `View()`)
  and Issue #43 (a visible one-cell fallback for standalone combining
  clusters, which propagates through rendering, clipping, wrapping,
  highlights, and byte mapping via this model).

## Tests

See [unit-tests](unit-tests.md) for the Issue #39 catalog:
`safepresentation/cellwidth_test.go` (helper semantics),
`theme_test.go` overlay-alignment rows,
`app/cell_render_test.go` (composed-view cell layout), and
`app/decode_guard_test.go` (the mechanical predicate).
