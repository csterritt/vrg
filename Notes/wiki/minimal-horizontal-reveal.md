# Minimal horizontal reveal of the first-submatch start

Issue #19
(`Notes/issues/019-minimal-horizontal-reveal.md`, tasks
`Notes/tasks/019-minimal-horizontal-reveal.md`) delivered the
horizontal half of the destination reveal: in run-off-edge mode, at
startup and on every actual `n`/`p` navigation, a first-submatch start
cell that is not painted moves the horizontal offset by the *minimum*
columns that paint it.

PRD cross-references: "Navigation, viewport, and logical anchors" (the
horizontal-reveal bullet and the file-change entry sequence) and
"Layout and indicators" (visibility over rendered cells, excluding the
reserved column) in `Notes/PRD-vrg.md`.

## Painted-cell visibility

"Visible" for reveal purposes means the target's start cell is
**actually rendered as a painted cell** after grapheme clipping —
consistent with how [Issue #20's indicators](
hidden-content-indicators.md) compute visibility — not merely
geometrically inside the text area. A position inside
`[off, off + text width)` whose grapheme cluster is split by a clip
edge renders blank and counts as *hidden*; the reveal then moves the
offset even though the position was inside the window geometry. The
reserved right-indicator column is already excluded: `Viewport.width`
is the text width.

The unit that must paint is the target's **own grapheme cluster**,
found through the `Lead`/`Cont` marks on the row's cells — the same
shared grapheme policy `wrapLine` consumes (Issue #16); Issue #21's
cluster-expanded spans will keep supplying unit-aligned starts. A
marker target — a zero-width match's marker cell, detected as a
`Start == End` span at the target cell — is one painted cell wherever
it sits: it renders as an inverse space even on a clipped cluster's
lead, so it needs no cluster whole.

## Reveal arithmetic

`Viewport.Reveal` (`internal/viewport/viewport.go`) runs the vertical
placement first — including the visible-set re-clamp of the stored
offset — then `revealCell` resolves the target unit `[s, s+cw)` on the
target's rendered row and:

- **Hidden left** (`s < off`): `off = s` — the window's first column
  lands exactly on the target column.
- **Hidden right or right-edge-clipped** (`s + cw > off + width`):
  `off = s + cw − width` — a single-cell target lands at
  `T − (text width − 1)`; a two-cell cluster at `T + 2 − text width`
  so both cells paint at the right edge.
- **Already painted**: the offset is retained unchanged.
- **Unpaintable cluster** (`cw > width`): no offset can paint it —
  grapheme-safe clipping renders its in-window portion as blanks at
  every offset. The offset is set to `s`, the start-column rule's
  closest achievable position, and the target counts as
  **geometrically revealed**: a repeat reveal computes the same offset
  and does not move — no panning loop. Issue #20's indicator
  visibility still counts the blank-rendered cluster as not visible.

This is minimal scrolling, not one-third-across placement — the PRD's
explicit-interview refinement. A match *wider* than the text area is
revealed by its start cell alone: only the start cell's cluster must
paint, so a 150-cell span reveals to `off = start + 1 − width` with
the rest of the span still hidden right.

The reveal **sets the offset directly**, ignoring the visible-lines
extent's paintable-boundary maximum ([horizontal-panning.md](
horizontal-panning.md)): the unpaintable-cluster contract mandates the
start column even when that exceeds the boundary (a line whose
unfittable cluster is its widest line can have boundary < `s`). The
stored offset remains subject to the destructive re-clamp on the next
visible-set change, per the extent policy.

## Triggers

The reveal rides the existing `Model.reveal` seam
([destination-reveal.md](destination-reveal.md)), which now covers
both axes through the one `Reveal` call:

- **Startup** — the first stop's pending reveal commits when the
  startup file's layout installs (`loadDoneMsg` → `syncLayout` →
  `layoutDoneMsg` → `commitReveal`), after the load-completion
  `SetOffset(0)`.
- **Every navigation action** — including same-file `n`/`p`, where no
  reset runs and the reveal applies on the current offset.
- **File change** — `navigate`'s `FileChanged` branch resets
  `SetOffset(0)` first (Issue #18), then the reveal applies on the
  clamped-from-zero value.

In wrap mode there is no horizontal reveal: `revealCell` returns
early and the dormant offset is untouched.

## Tests

- `internal/viewport/hreveal_test.go` — the reveal arithmetic: right
  reveal `T − (w−1)`, the two-cell cluster `T + cw − w` fully painted,
  left reveal to the target column including a left-edge split,
  painted-target no-ops including a cluster flush with the right edge,
  the geometrically-inside clipped blank treated as hidden, the
  oversized span revealed by its start cell (clipped span `{9,10}` at
  the window's edge), the unpaintable cluster's start-column offset
  with clipping blanks and no repeat movement, marker-cell positions
  (end-of-line reveal, marker on a clipped cluster's lead), the
  combined vertical+horizontal move, and wrap-mode dormancy.
  `pan_test.go`'s `moving_reveal` subtest now targets cell 9 so its
  painted landing proves the reveal leaves a clamped offset alone.
- `internal/app/reveal_horizontal_test.go` — the triggers through
  `Update`: same-file `n` scrolling right minimally with the match
  start on the last text column, `p` back landing on the target
  column, an already-visible match moving nothing, the startup reveal
  applying horizontally (run-off-edge from before the completion), the
  file-change reset-then-reveal sequence, and a CJK match painting
  both cells of its first glyph at the right edge. Since Issue #38
  every expected offset and painted position is computed from
  `wantTextW()` — the terminal-minus-list-minus-gutter-minus-reserved
  chain — rather than the cached `m.textW`, and the file adds the
  list-hidden re-measure, wrap mode's zero-reservation layout key and
  full-width wrap rows, resize re-measurement in both directions, and
  `TestComposedViewContentStaysInsidePanel` (no rendered row exceeds
  the terminal width, panned content never bleeds under the list, and
  the reserved indicator column is the panel's right edge outside the
  text area).

See [unit-tests.md](unit-tests.md) § `internal/viewport` and
`internal/app`.

## Files

- `internal/viewport/viewport.go` — `Reveal` gained the horizontal
  half; `revealCell` resolves the target unit (cluster via `Lead`/
  `Cont`, or marker cell via `markedAt`) and sets `off` per the
  arithmetic above.
- `internal/app/browse.go` — `reveal` unchanged in shape: the one
  `vp.Reveal` call now moves both axes; `navigate`'s file-change
  `SetOffset(0)` precedes it as before.

See also: [destination-reveal.md](destination-reveal.md) (the vertical
half and the entry sequence this extends),
[horizontal-panning.md](horizontal-panning.md) (the offset, the extent
clamp the reveal deliberately bypasses, and `clipRow`'s blanking),
[wrap-mode.md](wrap-mode.md) (the mode with no horizontal reveal),
[match-navigation.md](match-navigation.md) (the steps that trigger
it),
[grapheme-highlight-expansion.md](grapheme-highlight-expansion.md)
(the expanded spans the target's start cell comes from),
[zero-width-match-markers.md](zero-width-match-markers.md) (the marker
targets `markedAt` resolves),
[safe-presentation.md](safe-presentation.md) (the shared grapheme
policy), and [logical-anchor-and-layout.md](
logical-anchor-and-layout.md) (the pending intent that carries the
reveal across layout installs).
