# Minimal horizontal reveal — painted-cell visibility and reveal arithmetic (Issue #19)

Delivered by
[Issue #19](../issues/019-minimal-horizontal-reveal.md)
([task](../tasks/019-minimal-horizontal-reveal.md)): in run-off-edge
mode, at startup and on every actual `n`/`p` transition — same-file
steps included — a horizontally hidden display target moves the pan
offset by the **minimum** number of columns that makes its start cell
visible; an already-visible target keeps the offset. On a file change
the reveal runs after Issue #18's `ResetOff`, so it operates on the
reset zero. Relevant PRD sections: *Navigation, viewport, and logical
anchors* (the horizontal-reveal and file-change-sequence bullets) and
*Layout and indicators* (visibility over rendered cells) in
[`Notes/PRD-vrg.md`](../PRD-vrg.md); user stories 54–55. Builds on
[horizontal-panning.md](horizontal-panning.md) (the `off` offset the
reveal moves and the file-change reset it follows),
[destination-reveal.md](destination-reveal.md) (the `Target` and the
`m.reveal` entry point it shares), and
[wrap-mode.md](wrap-mode.md) (the shared `Clusters` segmentation the
cluster-width arithmetic reads).

## Painted-cell visibility — `viewport.CellVisible`

"Visible" means **actually rendered as painted cells**, the same
criterion Issue #20's hidden-content indicators will use: rendered
cells after grapheme clipping, the reserved indicator column already
excluded because the width is the text width. `CellVisible(line, cell,
off, width)` checks the whole grapheme cluster holding the cell fits
inside the window `[off, off + width)`:

- A cluster straddling the left edge paints its in-window cells as
  clipping blanks — not visible.
- A cluster whose first cell is inside the window but whose width
  crosses the right edge is dropped by the renderer's `col + cw >
  textW` break — its cell is a blank too, so a **geometrically inside
  position can still be hidden**.
- The end-of-line marker position (one cell past the line's last, the
  Issue #23 marker) counts as a one-cell target — an empty matched
  line's cell 0 included.

## Reveal arithmetic — `Viewport.RevealOff(target, row, extent)`

`targetCluster` resolves the target cell to its cluster's start and
width via `Line.Clusters` (a past-end marker cell resolves to
`(cell, 1)`). With `start`/`w` in hand:

- **Hidden left** (`start < off`) → `off = start`: the target column
  itself, the smallest leftward move that paints the whole cluster.
- **Hidden right** (`start + w > off + textW`) →
  `off = start + w − textW`: a single-cell target lands at
  `T − (textW − 1)`; a two-cell cluster at `T + 2 − textW`, fully
  painted at the right edge — never a clipped blank.
- **Painted** → the offset is unchanged.
- **Oversized match** — a span wider than the text area is revealed by
  its start cell alone; the reveal never chases full-span visibility.

The result needs no clamping: for a paintable cluster
`start + w − textW ≤ start ≤ MaxStart(line)`, so the new offset never
exceeds the line's contribution to `MaxOff`.

## The geometric fallback — a cluster wider than the text area

When `w > textW` no offset can paint the cluster: its in-window cells
render as clipping blanks at every offset. The reveal then sets
`off = start` — the start-cell rule's closest achievable position —
and counts the target as **geometrically revealed**: the `w > textW`
case is checked first, so a repeated reveal reproduces the same offset
and there is no panning loop (without it the left and right rules
would oscillate forever). `CellVisible` still reports the cluster
unpainted, so Issue #20's indicators keep counting it as hidden — the
fallback relaxes only the reveal, never the visibility criterion.

## Triggers (`internal/app`)

`model.reveal` remains the single entry point; after resolving the
stop's `TargetRow` and `StopTarget` it runs the vertical `Reveal`
first — a moving reveal re-clamps the stored offset against the newly
visible rows — then `RevealOff`, writing the viewport back when either
moved. Because every trigger routes through `reveal`, the horizontal
rule applies identically at startup (the pending reveal that commits
when the first load's layout installs), on every same-file `n`/`p`, on
a file change after `ResetOff`, and on a load completing for the
current file. In wrap mode `RevealOff` is a strict no-op — no
horizontal reveal is needed when every row starts at cell 0.

## Tests

`internal/viewport/reveal_test.go` (external package): right-edge
arithmetic for one- and two-cell targets, left-edge landing on the
start column, painted-target no-movement at both edges, the
clipped-blank case (`文` cut by the right edge counts hidden and
reveals), oversized-span start-cell reveal with a no-op repeat, the
unpaintable-cluster fallback (offset = start column, `CellVisible`
still false, repeated reveal unmoved), the marker-cell one-cell rule,
wrap/empty-model no-ops, and the `CellVisible` table itself.

`internal/app/hreveal_test.go` (same package): same-file `n` revealing
right to `300 + 1 − 40`, an already-visible match moving nothing, and
wrapping `n` revealing left to the target column; the startup reveal
applying the horizontal rule when run-off-edge was toggled before the
first load; file change resetting the offset *before* the reveal (a
target hidden left of the saved offset is visible after the reset, so
the offset stays 0); a CJK match at cell 298 painting both cells of
its first glyph at the right edge; the clipped-blank trigger through
the real render; and the unpaintable tab cluster rendering all-blank
at its start column with no loop across repeated navigation. See
[unit-tests.md](unit-tests.md).
