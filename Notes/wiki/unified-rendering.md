# Unified rendering — one grapheme/cell helper for the whole frame (Issue #39)

Delivered by
[Issue #39](../issues/039-render-from-shared-grapheme-cell-model.md)
([task](../tasks/039-render-from-shared-grapheme-cell-model.md)):
the final rendering stage now uses the same grapheme/cell model as
every upstream stage — `FileBuffer` and `Viewport` already produced
cell-based, grapheme-aware ranges, but the last-mile consumers
measured runes. One shared ANSI-aware helper owns cell measurement,
the file panel renders straight from `Line.Clusters`, and every
display-width and truncation consumer routes through the helper.
Relevant PRD sections: *Text, graphemes, and safe presentation*
(single consistent grapheme/cell policy) and *Navigation, viewport,
and logical anchors* in [`Notes/PRD-vrg.md`](../PRD-vrg.md); user
stories 71–72, 75, 82. Builds on
[safe-presentation.md](safe-presentation.md) (the `Mapped` byte→cell
model and `Clusters` segmentation),
[grapheme-highlight-expansion.md](grapheme-highlight-expansion.md)
(the `Line.Highlights` cell spans the renderer consumes),
[panel-text-width.md](panel-text-width.md) (the text band the
renderer draws into),
[file-list-layout.md](file-list-layout.md) (entry padding and the
filename row),
[file-change-popup.md](file-change-popup.md) (the pop-up box),
[hidden-content-indicators.md](hidden-content-indicators.md) (the
indicator columns), and [theme.md](theme.md) (`Overlay`).

## The shared helper — `safepresentation.CellWidth`

`internal/safepresentation/cellwidth.go` holds the single
ANSI-aware cell-width helper: `CellWidth(s)` iterates s's grapheme
clusters under the existing `rivo/uniseg` policy and sums their
terminal-cell widths, skipping ANSI CSI sequences — `ESC [` plus
parameter and intermediate bytes closed by one final byte in
`0x40–0x7E` (`csiPrefix`) — so a styled string measures exactly its
content cells. The escapers never emit a raw ESC, so no legitimate
display string opens a sequence accidentally. The same file holds
the package-internal rune decoders `decodeRune`/`decodeRuneInString`.

The boundary is mechanically enforced: across every non-test
production `.go` file under `internal/` and `cmd/`, the symbol
`utf8.DecodeRuneInString` may appear only in
`internal/safepresentation/cellwidth.go`.
`internal/safepresentation/cellwidth_test.go`'s
`TestDecodeRuneInStringAllowList` walks both trees and fails on any
other occurrence — including one in a newly added package — so
display-geometry code in `internal/viewport`,
`internal/filebuffer`, or any later consumer cannot reintroduce a
rune-decoding width or truncation loop; test-only decoder utilities
are excluded from the scan.

## The cluster-driven file-panel renderer

`contentText` (in `internal/app/browse.go`) renders a row's
`[Start, End)` cell span straight from `row.Line.Clusters` and the
cells' recorded text — never re-deriving positions from runes or
bytes: a cluster straddling either window edge contributes one
unstyled clip blank per in-window cell (never a partial glyph, never
a match cell), a wide glyph's continuation cells are skipped after
its first painted cell, and a zero-width marker paints its one-cell
unit. Highlight runs are the maximal painted-cell runs of
`Line.Highlights`, styled `CurrentMatch`/`Match`; the byte→cell
mapping that earlier code re-derived per frame is the `Mapped`
model's own.

## Every display-width consumer routes through `CellWidth`

- **Line rendering and highlight styling** — `contentText` measures
  each painted cell's text with `CellWidth` and accumulates painted
  cells, so a two-cell CJK glyph, a base-plus-combining cluster, and
  an emoji ZWJ sequence each occupy exactly their measured cells and
  a partial-cluster match highlights the whole cluster without
  swallowing the following character.
- **Centered text** — `center` (the searching, no-results, and
  too-small screens) pads by `CellWidth`, replacing rune counting.
- **List-entry padding** — `listCell` pads each entry to the list
  width with `CellWidth` on the clipped path.
- **Indicator-column sizing** — the reserved right column
  (`viewport.ReservedIndicator`) and the gutter mark slot stay one
  cell each while `LeftMark`/`RightMark` judge visibility over
  grapheme clusters, so a straddling wide glyph counts hidden.
- **Filename-row fitting** — `filenameRule` measures the note, the
  clipped path, and the rule fill in cells via `CellWidth`.
- **Pop-up truncation/width/centering** — `compositePopup` measures
  the truncated text and the box width in cells;
  `leftTruncate`/`tailCells` — the replacement for the retired
  rune-boundary `truncateLeftCells` — clip on whole grapheme
  clusters with no assumption that `EscapePath`'s output lacks
  combining marks, so wide and combining paths are never split or
  miscentered.
- **Theme overlay sizing** — `theme.Overlay` sizes the border and
  pads short rows by `CellWidth`, replacing its rune-per-cell
  `cellWidth`, so borders align for wide and combining interior
  text.
- **Overlay/scroll boxes and help** — `compositeBox`, `wrapCells`,
  `padTo`, `clipCells`, and the help binding table already measured
  and wrapped on the same uniseg policy.

`truncateLeftCells`'s rune-boundary implementation and its false
no-combining-marks assumption are gone: the pop-up routes through
`leftTruncate`/`tailCells`, the same shared cluster/cell primitive
the file list and filename row use.

Issue #40 consumes this as the handoff: the shared helper and the
width-independent escaped-path/cluster geometry stay intact while
per-file grouping work moves out of `View()`.

## Tests

`internal/safepresentation/cellwidth_test.go` — `CellWidth`'s
grapheme policy over CJK, combining, and ZWJ clusters; its ANSI
awareness over SGR-wrapped and mid-string-styled strings; and the
`DecodeRuneInString` allow-list scan. `internal/theme/theme_test.go`
— `TestOverlayPadsToCellWidth` pads a two-cell and a one-cell
combining row to aligned borders. `internal/app` composed-view
tests — `TestCJKPartialMatchNeverSwallowsNextChar`,
`TestWideCombiningClusterClipsAsOne`,
`TestZWJClusterPaintsWholeAndClipsWhole` (grapheme_test.go),
`TestWideMatchIndicatorColumns` (indicators_test.go),
`TestListEntryWidePathPadsInCells`, `TestFilenameRuleWidePath`
(filelist_test.go), `TestPopupWideCombiningPathCells`
(popup_test.go), and `TestCenterMeasuresCells` — assert the emitted
cell layout: whole-cluster highlights that never swallow the next
character, clip blanks for straddling clusters, cell-exact padding,
fitting, truncation, and centring. See
[unit-tests.md](unit-tests.md).
