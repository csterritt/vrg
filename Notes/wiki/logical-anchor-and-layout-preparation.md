# Logical anchor and layout preparation (Issue #17)

Issue #17 adds a width-independent logical viewport anchor that
survives rewraps, wrap-mode toggles, and terminal resizes; moves layout
preparation off the Bubble Tea update path with keyed installation
guards and out-of-order discard; carries pending reveal intents on the
model until a matching layout installs; and adds render-cost guards for
visible file-list entries alongside the existing visible-row guard.

Cross-references: [wrap-mode-and-grapheme-policy](wrap-mode-and-grapheme-policy.md),
[manual-vertical-scrolling](manual-vertical-scrolling.md),
[destination-reveal](destination-reveal.md),
[browse-tracer](browse-tracer.md). PRD sections: Navigation, viewport,
and logical anchors; Resources and responsiveness.

## Logical anchor model

`viewport.Anchor{LineIndex, Column int}` is a width-independent source
location: `LineIndex` identifies the source line and `Column`
identifies the display-cell offset within that line. The viewport
stores a single `anchor` field and exposes `Anchor()`, `SetAnchor`,
and anchor-aware `SetRows`/`SetPanelHeight`.

`RowModel.RowFromCell(lineIndex, column int) int` maps a source line
and display column to the 0-based rendered row containing that cell.
`RowModel.RowAnchor(row int) Anchor` returns the anchor for a rendered
row. Both use the per-row `startCell` recorded during wrapping.

### Anchor retention

A same-file rebuild (wrap toggle or resize) preserves the existing
anchor so the same text location remains at the top:

- `buildViewport()` captures the old viewport's anchor before
  rebuilding and reapplies it after constructing the new row model.
- `LayoutReadyMsg` installation preserves the anchor for same-file
  rebuilds (the viewport is non-nil) and restores the saved per-file
  anchor for fresh loads (the viewport is nil).

### Anchor replacement

User scrolling and moving reveals replace the anchor with the new top
row's location:

- `SetOffset`, `ScrollDown`, `ScrollUp`, `ScrollHalfDown`,
  `ScrollHalfUp`, `ScrollPageDown`, `ScrollPageUp`, and `Reveal` all
  call `syncAnchorToOffset` after moving, replacing the anchor with
  `RowAnchor(newTopRow)`.
- A no-scroll reveal (target already visible) leaves the anchor
  unchanged.

### Lossy EOF clamp

When the viewport clamps to EOF (the offset would exceed `maxOffset`),
the anchor is replaced with the clamped top row's anchor. This is
intentionally lossy: the former position is discarded so a subsequent
widen does not restore a position that was only valid because of the
narrow width's larger row count.

## Off-UI layout preparation

Layout preparation is off the Bubble Tea update path. `buildViewport()`
returns a `tea.Cmd` that builds the `RowModel` asynchronously and
emits a `LayoutReadyMsg`. The command is returned from `Update` so the
runtime executes it; the UI never blocks on preparation.

### Layout key

`LayoutKey()` returns the current `RowModelKey{Path, Revision,
TextWidth, WrapMode}`. The revision field is in place for Issue #27's
reload-supersession; it is currently always 1.

### Layout gate (test seam)

`WithLayoutGate(ch chan struct{})` injects a channel that the
preparation command blocks on until closed or receiving a value. This
is a test seam for verifying the browse view stays responsive while a
layout is pending. Production passes no gate, so preparation completes
immediately.

### LayoutReadyMsg and installation guard

`LayoutReadyMsg{Key, RowModel}` is the completion message. The handler
installs the prepared `RowModel` only when `msg.Key` equals the model's
current `LayoutKey()`. Out-of-order completions (from a prior resize,
wrap toggle, or file switch) are discarded without touching the
visible panel, the saved per-file state, or the pending reveal intent.

On installation the handler caches the `RowModel` in `layoutCache`
keyed by path, preserves the anchor for same-file rebuilds, restores
the saved per-file anchor for fresh loads, and commits any pending
reveal intent.

## Pending reveal intent

When the viewport is nil (layout pending) and a reveal is requested
(navigation or load completion), the intent is preserved in
`pendingReveal bool` and committed once the layout installs. This
ensures a stop selected while the gate was held is revealed per the
Issue #14 rules once a matching layout is installed.

`HasPendingReveal()` exposes the intent state for tests.

## Cached-file navigation

`layoutCache map[string]*RowModel` stores installed row models per
file path. When navigating to a cached file:

- **Matching layout** (same text width, wrap mode, and revision): the
  cached row model is installed immediately with no preparation
  request. This is the fast path.
- **Stale layout** (different text width, wrap mode, or revision): a
  new layout preparation is requested for the current parameters, and
  the entry-reveal or saved-viewport intent is carried until
  installation.

`HasPendingLayout()` exposes the pending state for tests.

## AC6 responsiveness during gated preparation

While a layout preparation is held by the gate, every AC6 input is
actionable without waiting:

- `ctrl+c` exits 130 immediately.
- `q` exits with the fixed status immediately.
- `n` advances the cursor immediately; `p` moves to the previous
  stop immediately. The latest pending reveal is preserved for
  whichever stop is newest.
- `w` changes wrap mode immediately and issues (or replaces) a keyed
  layout request for the new mode without releasing the existing
  gate.
- A second resize is accepted without releasing the gate.

## Render-cost guards

The render path queries only the visible range:

- **Visible rows**: the viewport's `Visible()` method returns only the
  rows in `[offset, offset+contentHeight)`. The row provider is never
  queried beyond this range. (Issue #12)
- **Visible file-list entries**: `renderBrowse` limits the file-list
  iteration to `min(fileCount, terminalHeight)`. The file-list provider
  (or the default `groupByFile` output) is never queried beyond the
  visible range. (Issue #17)

The `WithFileListProvider` test seam injects a counting fake to prove
the file-list render-cost guard.

## App integration

`buildViewport()` distinguishes three contexts:

1. **RowProviderFactory test seam**: installs a row provider directly
   (synchronous bypass for existing render-cost tests).
2. **Cache hit**: a cached `RowModel` with a matching key is installed
   immediately with no preparation request.
3. **Cache miss or stale**: issues a layout preparation command. The
   old layout (if any) remains installed until the new one arrives;
   for a fresh load the viewport is nil and the render path shows the
   loading placeholder.

Cross-file navigation sets `m.viewport = nil` before `buildViewport()`
so the installation treats it as a fresh load (restoring the saved
per-file anchor) rather than a same-file rebuild (preserving the
current anchor).

See [wrap-mode-and-grapheme-policy](wrap-mode-and-grapheme-policy.md),
[manual-vertical-scrolling](manual-vertical-scrolling.md), and
[destination-reveal](destination-reveal.md).
