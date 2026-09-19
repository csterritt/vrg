# Panel text width — terminal/panel/text width chain and the separator cell (Issue #38)

Delivered by
[Issue #38](../issues/038-viewport-content-panel-width.md)
([task](../tasks/038-viewport-content-panel-width.md)):
the prepared layout's text width is derived from the actual content
panel — the terminal width minus the file list minus a one-cell
separator — so every width-sensitive behavior (wrapping, clipping,
horizontal panning and reveal, hidden-content indicators, padding)
measures against the width the panel really paints instead of a raw
terminal-derived width that omitted the separator cell. Relevant PRD
sections: *File list and layout* (the list/panel split) and *Layout
and indicators* (the width-formula bullet) in
[`Notes/PRD-vrg.md`](../PRD-vrg.md). Builds on
[file-list-layout.md](file-list-layout.md) (the `listWidthFor` formula
the panel width subtracts), [wrap-mode.md](wrap-mode.md) (the keyed
`viewport.Rows` pipeline and the `ReservedIndicator` width),
[horizontal-panning.md](horizontal-panning.md) and
[horizontal-reveal.md](horizontal-reveal.md) (the consumers the
corrected width feeds), and
[logical-anchor.md](logical-anchor.md) (the install-time key match
that keeps a stale layout invisible).

## The width chain

Three widths describe a composed row, left to right:

- **Terminal width** (`m.width`) — the frame's full cell count.
- **Panel width** — `width − listWidth − 1`: the file list's cells,
  one separator cell, and the remainder belongs to the content
  panel. While the list is hidden (`listWidth` 0) the panel still
  loses the separator cell — the blank column stays.
- **Text width** — `textWidth(gutter)` =
  `width − listWidth − 1 − gutter − ReservedIndicator(wrap)`: the
  panel minus the line-number gutter and the reserved
  right-indicator column (one cell in run-off-edge mode, zero in
  wrap). The result clamps to zero rather than going negative.

`browseView` composes each content row as `listCell + " " +
contentCell` — the separator is a literal blank cell between them,
always written even while `listW` is 0, so the panel's left edge and
every cell offset inside it match the arithmetic exactly. The
composed row totals `listW + 1 + panelW = width` cells whenever the
dimensions are non-degenerate.

## The layout key's `TextWidth` governs installation

`textWidth(gutter)` is the only producer of the `TextWidth` field in
`layoutKey(path, buf)` — minted per buffer under **that buffer's own
gutter**, so a key for any path is self-consistent. Because the key
is the live demand `layoutReadyMsg` must still match to install
(see [logical-anchor.md](logical-anchor.md)), every width the
separator touches flows through one chain: a completion minted under
a different list width, separator accounting, gutter, or mode can
never install, and `currentRows` hides a stale model behind the
"Loading…" placeholder until its replacement arrives. All of the
viewport's horizontal behavior — `rows.Key().TextWidth` — therefore
reads the corrected panel-derived width with no separate code path.

## Installation paths

Any change that shifts the text width mints a new key and routes
through `requestLayout`:

- **`fileLoadedMsg`** — the buffer's own gutter (and the revision
  bump) re-keys the layout; a gutter-growing load narrows both the
  list's reservation term and the panel.
- **`WindowSizeMsg`** — the new terminal width re-keys every layout.
- **`w` wrap toggle** — flips `ReservedIndicator` between 1 and 0,
  changing the text width by one cell as well as the row model.
- **`left`/`tab` and `right`/`shift+tab`** — hiding the list moves
  the panel's left edge to the separator column and widens the text
  width by `listWidth`; showing reverses it. The separator column
  itself does not move.
- **`navigate`** — requests the destination's layout so a cached
  file with a stale key is re-prepared before it can render.

Each install preserves the saved logical `(line, column)` anchor —
never a rendered-row ordinal — so the reading position survives
every separator-aware relayout.

## Tests

`internal/app/reveal_horizontal_test.go` (same package) drives the
Issue #38 contracts through `Update`/`View` with the `textWidthFor`
helper mirroring the chain
`width − listWidthFor(gutter) − 1 − gutter − ReservedIndicator(wrap)`
— a regression that omits the list, separator, gutter, or indicator
term, or that sizes the panel from the raw terminal width, fails the
dependent assertions:

- `TestPanelDerivedTextWidth` — the installed layout key's
  `TextWidth` equals the panel-derived width under a visible list.
- `TestSameFileRevealUsesPanelDerivedTextWidth` — a same-file `n`
  step to a hidden-right match reveals to
  `off = start + w − textWidth` at the panel-derived width.
- `TestListHiddenHorizontalReveal` — hiding the list re-keys the
  layout at the wider width (`listWidth` 0, separator still
  subtracted) and the reveal follows it.
- `TestWrapModeZeroReservedIndicator` — the wrap-mode key reserves
  no indicator column, and toggling to run-off-edge narrows the
  text width by exactly one cell for the reveal.
- `TestResizeRemeasuresTextWidth` — a terminal resize re-keys the
  layout at the new panel-derived width and the reveal uses it.
- `TestCacheHitRevealUsesInstalledTextWidth` — navigating back to a
  file whose installed layout still matches the live key issues no
  redundant request and reveals at the installed `TextWidth`.
- `TestComposedViewRowsFitTerminal` — cell-level structure of the
  composed row: no row exceeds the terminal width, the separator
  cell at column `listW` is blank, the last text-area cell holds
  content, and the reserved `*` lands at `width − 1` on the
  hidden-right-match row while staying blank on an ordinary row.

The pre-existing panning, reveal, indicator, grapheme, wrap, and
layout suites were re-based on the corrected geometry:
`frameWidthForG` derives its frame width as
`tw + lw + 1 + gutter + ind`, and every panel-row slice starts at
`listW + 1` rather than `listW`.
