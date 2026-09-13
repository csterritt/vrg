# Minimal horizontal reveal (Issue #19)

Issue #19 adds minimal horizontal reveal of the first-submatch start
cell in run-off-edge mode. When a navigation target is hidden
horizontally, the viewport pans by the minimum movement needed to
paint the target's grapheme cluster. An already-painted target does
not move the offset. The reveal runs on startup after the initial
file loads and on every actual match-navigation transition, after the
new viewport/layout is installed and after the Issue #18 file-change
horizontal reset. The reveal is a no-op in wrap mode.

This page cross-references:

- [destination-reveal](destination-reveal.md) — the Issue #14 vertical
  reveal that Issue #19 extends with horizontal movement.
- [horizontal-panning](horizontal-panning.md) — the Issue #18
  horizontal offset, paintable-boundary maximum, grapheme-safe
  clipping, and file-change reset that Issue #19 builds on.
- [wrap-mode-and-grapheme-policy](wrap-mode-and-grapheme-policy.md) —
  the shared grapheme segmentation and cell-width policy that the
  reveal consults for cluster widths.
- [logical-anchor-and-layout-preparation](logical-anchor-and-layout-preparation.md)
  — the prepared-layout architecture and pending-reveal intent that
  Issue #19 commits through.
- `Notes/PRD-vrg.md` — Navigation, viewport, and logical anchors
  section (run-off-edge horizontal reveal).

## Target cell definition

The display target is the start cell of the first submatch on the
destination line. The app derives it from the loaded line's
`ByteCells`: `line.ByteCells[sm.Start][0]` maps the first submatch's
raw byte offset to the display cell where the cluster begins. The
cluster width is derived from the line's grapheme clusters at that
cell via `viewport.ClusterWidthAtCell(line.Clusters, targetCell)`.

Submatches are ordered by byte start then end by the search index, so
the first submatch identifies the target. This matches the Issue #14
vertical reveal's target-row derivation.

## Painted-cell visibility versus nominal cell visibility

A target is "painted" only when its entire grapheme cluster is fully
within the visible window `[hOffset, hOffset + textWidth)`. A cluster
split by either clip edge renders as blank cells (Issue #18
grapheme-safe clipping), so a target whose nominal cell range overlaps
the window but whose cluster is split is **not** painted and must be
revealed.

`Viewport.RevealHorizontal(line, targetCell)` checks painted-cell
visibility:

```
windowEnd := hOffset + textWidth
if targetCell >= hOffset && targetCell+clusterWidth <= windowEnd {
    return // already painted, no move
}
```

This is stricter than a nominal cell-range overlap test. The
`TestRevealHorizontalPaintedCellSplitRightEdge` and
`TestRevealHorizontalPaintedCellSplitLeftEdge` tests verify that a
split cluster is treated as hidden and revealed.

## Minimum horizontal movement

The reveal uses the minimum movement needed to paint the target
cluster:

- **Right-side reveal** (target right of view, or split by the right
  edge): right-edge arithmetic so the entire cluster fits at the right
  edge: `offset = targetCell + clusterWidth - textWidth`.
- **Left-side reveal** (target left of view, or split by the left
  edge): the target start lands at the left edge: `offset = targetCell`.

The offset is clamped to `[0, MaxHOffset()]` after the move, respecting
the Issue #18 paintable-boundary maximum.

Only the start cell needs to become visible; the entire match does
not. The `TestRevealHorizontalOversizedMatchStartCell` test verifies
that a match wider than the text area is revealed by its start cell
alone (cluster width 1, not the match width).

## Unpaintable cluster fallback

A grapheme cluster wider than the entire text area cannot be fully
painted. The reveal uses a geometric fallback: the offset is set to the
target start column, the in-window portion renders as clipping blanks,
and the target is treated as geometrically revealed. The offset is not
clamped here because the paintable-boundary maximum is zero for an
unpaintable cluster and would undo the geometric position.

Repeated navigation to an unpaintable cluster is idempotent: once the
offset equals the target cell, the geometric check short-circuits and
no panning loop occurs. The `TestRevealHorizontalUnpaintableCluster`
and `TestRevealHorizontalUnpaintableClusterNoLoop` tests verify this.

## Wrap-mode no-op

The horizontal reveal is a no-op in wrap mode, matching the Issue #18
panning no-op. `RevealHorizontal` returns immediately when `wrapMode ==
WrapOn`. The `TestRevealHorizontalNoOpInWrapMode` test verifies this.

## Startup and navigation triggers

The reveal runs through the existing `revealTarget()` path, which
Issue #14 introduced for vertical reveal. Issue #19 adds a horizontal
reveal step after the vertical reveal:

1. The vertical reveal runs first so the target row is visible and the
   horizontal clamp uses the correct visible rows.
2. The horizontal reveal runs only in run-off-edge mode (`wrapMode ==
   WrapOff`), when a buffer is loaded, and when the stop has
   submatches.
3. The target cell is derived from `line.ByteCells[sm.Start][0]` and
   the cluster width from `ClusterWidthAtCell`.

The reveal triggers on:

- **Startup** after the initial file loads: `needsReveal` is set when
  the browse state is entered, and `revealTarget()` runs after
  `buildViewport` (or the pending reveal is committed on
  `LayoutReadyMsg`).
- **Same-file navigation** (`n`/`p` within the same file):
  `handleNavigate` calls `revealTarget()` directly when the viewport is
  non-nil, or carries `pendingReveal` for the next layout installation.
- **Cross-file navigation** (`n`/`p` to another file): the file-change
  reset runs first (Issue #18), then `revealTarget()` runs after the
  new viewport is installed.

The `TestStartupHorizontalRevealRunOffEdge`,
`TestSameFileNavigationHorizontalReveal`,
`TestSameFileNavigationHorizontalRevealBack`,
`TestEveryNavigationTriggersHorizontalReveal`, and
`TestFileChangeResetThenHorizontalReveal` tests cover these triggers.

## Interaction with Issue #18 reset and clamping

The horizontal reveal integrates with the Issue #18 horizontal state
in two ways:

- **File-change reset ordering**: on a cross-file navigation, the
  Issue #18 reset to zero runs in `buildViewport` before the reveal.
  The reveal then applies from offset zero, so the new file's match is
  revealed correctly. The `TestFileChangeResetOrdering` test verifies
  that a far match in the first file does not carry over to the second
  file's near match: the reset zeroes the offset, then the no-op
  reveal leaves it at zero.
- **Paintable-boundary clamping**: the reveal clamps the resulting
  offset to `[0, MaxHOffset()]`, respecting the Issue #18
  paintable-boundary maximum. The maximum is recomputed from the
  visible rows after the vertical reveal, so the clamp uses the
  correct visible set.

The `WithWrapMode(viewport.WrapOff)` test seam starts the app in
run-off-edge mode so tests can verify horizontal reveal at startup
without toggling. The production default remains `WrapOn`.

## Implementation and test locations

- `internal/viewport/viewport.go` — `ClusterWidthAtCell` and
  `RevealHorizontal` (after `Reveal`).
- `internal/viewport/reveal_horizontal_test.go` — viewport arithmetic
  tests (right/left reveal, cluster-width arithmetic, painted-cell
  visibility, oversized matches, unpaintable clusters, wrap-mode
  no-op, `ClusterWidthAtCell` helper).
- `internal/app/app.go` — `revealTarget` horizontal step, `WithWrapMode`
  option, `wrapMode` config field.
- `internal/app/reveal_horizontal_test.go` — app trigger tests (startup,
  same-file navigation, every-navigation, file-change reset ordering).
