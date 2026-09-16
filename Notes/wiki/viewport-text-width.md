# Viewport text width (Issue #38)

Issue #38 fixes the width disagreement between the content row model
and the installed viewport. `LayoutKey` already derived the text width
from the content panel width, but several viewport install sites
recomputed `viewport.TextWidth(m.width, …)` from the raw terminal
width — skipping the file-list width and separator subtraction — so
the viewport panned, clipped, revealed, padded, and placed
hidden-content indicators as if the file list did not exist, and
composed rows could exceed the terminal width.

This page cross-references:

- [file-list-layout](file-list-layout.md) — the list width and
  one-cell separator that produce the panel width.
- [wrap-mode-and-grapheme-policy](wrap-mode-and-grapheme-policy.md) —
  `ReservedWidth`, `viewport.TextWidth`, and the keyed prepared row
  model built at the text width.
- [logical-anchor-and-layout-preparation](logical-anchor-and-layout-preparation.md)
  — `LayoutKey`, `LayoutReadyMsg`, `buildViewport`, and the layout
  cache whose install sites now consume the key's text width.
- [horizontal-panning](horizontal-panning.md),
  [horizontal-reveal](horizontal-reveal.md), and
  [hidden-content-indicators](hidden-content-indicators.md) — the
  run-off-edge behaviours measured against the text width.
- `Notes/issues/038-viewport-content-panel-width.md` — the issue.
- `Notes/PRD-vrg.md` — the *File list and layout*, *Layout and
  indicators*, and *Navigation, viewport, and logical anchors*
  sections.

## Three distinct widths

1. **Terminal width** — `m.width`, the raw terminal columns.
2. **Panel width** — `m.width - m.ListWidth() - 1`: the terminal width
   minus the file-list width (Issue #24) minus the one-cell pane
   separator.
3. **Text width** — `viewport.TextWidth(panelWidth, gutterWidth,
   wrapMode)`: the panel width minus the buffer gutter minus the
   reserved right-indicator width (`ReservedWidth` — one cell in
   run-off-edge mode, zero in wrap mode), clamped to at least one
   cell.

## LayoutKey is the single source of truth

`Model.LayoutKey()` performs the whole chain and stores the resulting
text width in `RowModelKey.TextWidth`. The prepared `RowModel` is built
at that width (it determines wrapping), and Issue #38 makes every
viewport installation consume the same value instead of recomputing
from `m.width`:

- **`LayoutReadyMsg` installation** — `m.viewport.SetLayout(
  msg.Key.TextWidth, m.wrapMode)`; the installation guard already
  requires `msg.Key == m.LayoutKey()`.
- **Synchronous factory seam** (`buildViewport` with
  `rowProviderFactory`) — installs `m.LayoutKey().TextWidth`.
- **Cache hit** (`buildViewport`) — already installed
  `key.TextWidth`; unchanged.
- **`WindowSizeMsg` resize path** (factory-seam viewport retained) —
  `m.viewport.SetLayout(m.LayoutKey().TextWidth, m.wrapMode)` after the
  dimension update, so the text width tracks the new panel width.

Because `ListWidth()` returns 0 when the list is hidden, the panel
width — and therefore the installed text width — follows the current
layout state on file-list hide/show, wrap toggles, and resizes alike.

## Consequences

- Run-off-edge clipping, `,`/`.`/`<`/`>`/`[`/`]` panning (half-pan is
  `max(1, floor(textWidth/2))`), and the Issue #19 minimal match
  reveal are all measured against the text width — the panel minus
  the gutter minus the reserved indicator.
- The gutter and the reserved right-indicator column occupy the panel
  outside the text area: the right `*` indicator sits at the panel's
  right edge and no composed row exceeds the terminal width.
- With an 80-column terminal, a width-10 list, and gutter 3: panel
  width 69, run-off-edge text width 65, wrap text width 66.

## Tests

`internal/app/reveal_horizontal_test.go` computes every expected
position through `textWidthFor` — terminal width minus the actual
`ListWidth()` minus separator minus gutter minus
`viewport.ReservedWidth(mode)` — replacing the `textWidthAt80` helper
that encoded the terminal-width defect. Coverage spans list-shown,
list-hidden, wrap mode (zero reservation), resize re-measurement, the
factory-seam install, and a composed-view assertion that no rendered
row exceeds the terminal width and the reserved right-indicator column
sits at the panel's right edge outside the text area.
`internal/app/pan_test.go` and `internal/app/indicator_test.go` were
recalibrated to the same chain (text width 65 at 80×24 with the
`src/a.go` fixture list).
