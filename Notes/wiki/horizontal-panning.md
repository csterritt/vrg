# Horizontal panning (Issue #18)

Delivered by
[Issue #18](../issues/018-horizontal-panning.md)
([task](../tasks/018-horizontal-panning.md)): in run-off-edge mode the
file panel shows a horizontal window into each line's display cells,
with `,`/`.`/`<`/`>`/`[`/`]` pan keys, a visible-lines extent policy,
and a paintable-boundary clamp that never splits a grapheme cluster.
Relevant PRD sections: *Navigation, viewport, and logical anchors*
(the pan-unit, extent-policy, and file-change-reset bullets) and
*Layout and indicators* in [`Notes/PRD-vrg.md`](../PRD-vrg.md); user
stories 65–66. Builds on
[wrap-mode.md](wrap-mode.md) (the run-off-edge row model and the
shared `Clusters` segmentation the clipper and extent math consume),
[logical-anchor.md](logical-anchor.md) (the `Viewport` state the
offset rides on and the `Model`/`Extent` interfaces),
[viewport-scrolling.md](viewport-scrolling.md) (the per-file `vps`
map), and [destination-reveal.md](destination-reveal.md) (the reveal
the file-change reset precedes).

## The offset (`internal/viewport`)

`Viewport` gains `off` — the horizontal pan offset in display cells,
the first source cell the run-off-edge window paints. It is **separate
state from the anchor**: saved per file in `m.vps` with `top` and
`anchor`, meaningful only under a flat model, and never consulted in
wrap mode.

- `Off()` reads it; `ResetOff()` clears it — the file-change reset.
- `Pan(d, e, height)` moves it by `d` cells (negative toward the left
  edge) and clamps to `[0, MaxOff]`; under a wrap model `Pan` is a
  strict no-op — the offset is not mutated.
- `HalfText(width)` is the `[`/`]` unit: `max(1, floor(width / 2))`.

## Pan units (`internal/app`)

`isPanKey` recognizes the six keys and `panBy` maps them: `,`/`.` move
one cell, `<`/`>` ten cells, `[`/`]` `HalfText` of the **installed
layout's** text width (`rows.Key().TextWidth`). Every pan re-derives
the bound from the rows visible now — nothing is cached across
keypresses. On the "Loading…"/"(unreadable)" placeholders the keys are
strict no-ops that create no viewport state.

## Three extent definitions

The clamp contract deliberately separates three quantities:

1. **Content extent** — a line's effective display width,
   `Line.Extent()`: the cell count plus one when an end-of-line
   marker sits past the last cell
   ([Issue #23](zero-width-markers.md)).
2. **Extent policy** — which lines count: the **widest currently
   rendered source line** only (the `visible-lines extent policy`,
   confirmed by the product owner) — never the whole file. `MaxOff`
   iterates only `[top, top + n)` of the prepared model.
3. **Maximum valid offset** — the **paintable boundary**: the largest
   cell index where a cluster of that widest line begins *and fits
   entirely within the text width*. At the maximum at least one whole
   cluster still paints — the window is never blank because of
   clamping and never shows half a glyph.

`filebuffer.Line.MaxStart(width)` computes a line's boundary: a line
ending at extent `E` yields `E − 1` for single-cell content; a line
ending in a two-cell cluster yields that cluster's **start** (`E − 2`),
so the offset can never land inside it; a final cluster wider than the
text width is skipped, falling back to the last fitting cluster's
start; a line with no fitting cluster at all reports 0. Issue #23's
end-of-line marker joins the candidate set as a one-cell unit at
`len(Line.Cells)` — a marker-only line reports `MaxStart` 0.

`viewport.MaxOff(e, top, n)` is the model-level bound: `0` for a wrap
model, a degenerate text width, an empty model, or an all-empty
visible set — else the largest `MaxStart` over the visible rows.

## Re-clamping on every visible-set change

`Viewport.clampOff(e, height)` runs at the end of `Scroll`, `Restore`,
`Clamp`, and a moving `Reveal` — so the stored offset follows the
visible set on vertical scrolling, destination reveal, resize, gutter
growth, list-geometry changes, and wrap-toggle re-entry (all of which
arrive through one of those operations or a fresh `layoutReadyMsg`
install). A **wrap model skips the clamp entirely** — that single
check is what makes the offset retained through wrap mode yet re-clamped
on every return to run-off-edge.

Two consequences are contractual:

- **No restoration.** Scrolling from a 300-cell line into "pad" lines
  clamps the offset to 2; scrolling back does **not** restore 200 —
  the clamped value is the new state, and Issue #19's reveal (see
  [horizontal-reveal.md](horizontal-reveal.md)) operates on it.
- **Retention through toggles.** Entering wrap keeps the offset; a
  wrap-mode scroll does not touch it; re-entering run-off-edge clamps
  it against whatever is visible *then* — which may differ from what
  was visible when wrap began.

## File-change reset and render plumbing

`navigate` calls `ResetOff` on the destination's saved viewport
**before** `m.reveal()` on every `Move.FileChanged` — a revisited file
starts at its left edge, not the offset it had when left.

`browseView` reads the offset only when `!m.wrap` and hands it to
`contentCell` → `contentText(row, off, textW, cur)`, which starts the
painted window at `max(row.Start, off)`. A cluster straddling the left
edge renders **one blank per clipped cell** — the `clipped` flag marks
continuation cells whose cluster began left of the window; painted
clusters' continuations still emit nothing. Right-edge clipping is
unchanged: a cell that would cross `textW` ends the row, so no split
glyphs at either edge.

## Interfaces

`viewport.Extent` extends `Model` with `Key()` and `At(i)` — the
contract every position-mutating operation now takes, since each
re-clamps the offset. `*Rows` implements it; the app's `rowSource`
embeds it (adding `Key()` to the test fakes), and `Scroll`, `Restore`,
`Clamp`, `Reveal`, and `Pan` all receive the same value.

## Render-cost bound

Extent evaluation touches only the visible row range — the
`countingExtent` fake proves `Pan`, `Scroll`'s re-clamp, and `MaxOff`
each query exactly `[top, top + n)` of the prepared layout. The frame
render itself never re-derives extents: the offset is maintained by
the write paths and read once per frame.

## Tests

`internal/viewport/pan_test.go` (external package): the `HalfText`
unit table, the 1/10/half pan units symmetrically, clamping to the
paintable boundary with overshoot in both directions, wrap-mode no-op
and zero `MaxOff`, offset retained through a wrap-toggle round trip,
re-entry clamping to a visible set that changed while wrapped, the
300-cell-among-10-cells extent sequence (299 visible, 9 scrolled out,
no restoration, fresh-pan re-evaluation), empty-model and all-empty
views, the trailing two-cell cluster stopping at its start, the
wider-than-width final cluster's fallback, reveal- and shrink-driven
re-clamps, the uniform-lines all-hidden-left legality (Issue #20's `_`
must be free to show on every line), the `ResetOff` contract, and the
`countingExtent` visible-rows-only guard. `internal/app/pan_test.go`
(same package) drives the keys through `Update`: per-unit rendered
shifts, wrap-mode and placeholder no-ops, left-edge clamping, the
`w`/`w` retention, file-change reset on `n`/`p` (including revisit),
the clipped-cluster blank cells, the fully painted final cluster at
the maximum, and the scroll/reveal/resize/wrap-reentry re-clamps. See
[unit-tests.md](unit-tests.md).
