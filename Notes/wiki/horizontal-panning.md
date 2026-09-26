# Horizontal panning in run-off-edge mode

Issue #18
(`Notes/issues/018-horizontal-panning.md`, tasks
`Notes/tasks/018-horizontal-panning.md`) delivered horizontal panning:
the six pan keys move a per-viewport cell offset over run-off-edge
rows, clamped by the *visible-lines extent policy*, with
grapheme-safe clipping at both window edges.

PRD cross-references: "Navigation, viewport, and logical anchors" (the
pan-unit bullet, the visible-lines extent policy bullet, and the
wrap-toggle offset-retention bullet) and "Layout and indicators" (the
reserved-indicator width and the hidden-left `_` signposting this
geometry enables) in `Notes/PRD-vrg.md`.

## Pan units

`Viewport` exposes the six pan keys' units as methods, all in *display
cells*:

- `Left`/`Right` — one column (`,`/`.`).
- `TenLeft`/`TenRight` — ten columns (`<`/`>`).
- `HalfLeft`/`HalfRight` — `max(1, floor(text width / 2))` (`[`/`]`),
  so odd widths floor and a one-cell text area still moves one column.

`Model.pan` (`internal/app/browse.go`) routes the keys to the current
file's viewport only in browse with a loaded buffer — the same gate as
`scroll`, so panning on a `Loading…`/`(unreadable)` placeholder or
outside browse is a strict no-op.

## The offset is separate, dormant state

`Viewport.off` is independent of the logical anchor:

- **Panning exists only in run-off-edge mode.** While a wrap `Model`
  is installed every pan is a no-op — the `Rows` interface gained
  `Wrap() bool` so the viewport knows which layout it holds.
- **Retained through wrap toggles.** A `w w` round trip leaves the
  stored offset untouched while the wrap model is up; on the
  run-off-edge re-install it is *re-clamped* against the newly current
  visible rows — a visible set that changed while wrapped (a scroll,
  or a width change re-fitting clusters) can move the clamp and the
  re-entry offset reflects it.
- **Reset on file change.** `navigate`'s `FileChanged` branch calls
  `SetOffset(0)` ahead of the destination reveal, and the
  `loadDoneMsg` current-file path does the same for the
  load-completion entry sequence — the reset half of the PRD's
  file-change rule, on which Issue #19's horizontal reveal then runs
  (see [minimal-horizontal-reveal.md](minimal-horizontal-reveal.md)).
  A revisit never restores a departed file's old offset.

## Visible-lines extent policy

The clamp follows the **widest currently rendered source line**, not
the widest line in the file. Three definitions stay distinct:

- **Content extent** — a line's effective display width in cells; an
  Issue #23 end-of-line marker cell will extend it by one.
- **Extent policy** — the maximum is taken over only the visible rows
  of the prepared layout.
- **Maximum valid offset** — `max(0, S)` where `S` is the *paintable
  boundary* of the widest visible line: the largest cell index at
  which one of its grapheme clusters (or a marker cell) starts and
  whose cell width fits within the text width. At that offset the
  whole cluster still paints, so clamping alone can never blank the
  text area or draw half a glyph. Among equally wide lines the
  smallest boundary wins; a widest line with no cluster fitting at all
  reports 0 — the documented **unpaintable-cluster exception**, where
  the in-window portion renders clipping blanks by design.

A 300-cell single-cell line with a 10-cell text area gives S = 299;
once it scrolls out and only 10-cell lines remain the maximum is 9.
An empty buffer, a placeholder, or a view of only empty lines clamps
to 0.

**Re-clamping on every visible-set change.** `clampOff` runs inside
`clamp` and `resolve`, so vertical scrolling, a moving reveal, a
resize (which is also how a list hide/show or gutter growth reaches
the viewport), a row-model swap, and wrap-toggle re-entry all
re-clamp — and every pan recomputes the maximum from the current
visible rows rather than a cached value. The loss is permanent:
scrolling back to the wide line does not restore the clamped-away
offset, and Issue #19's horizontal reveal operates on the clamped
value (its own offset writes deliberately bypass the boundary — see
[minimal-horizontal-reveal.md](minimal-horizontal-reveal.md)).

**Visible rows only.** `maxOffset` iterates `Row(i)` over
`[top, top+height)` — the counting-fake guard
`TestExtentQueriesOnlyVisibleRows` proves a pan or scroll never scans
the buffer.

**Hidden-left geometry is legal.** At a nonzero offset *every* visible
line may have text hidden left — the uniform-lines test pins this so
[Issue #20's `_` indicator](hidden-content-indicators.md) can
signpost every line at once.

## Grapheme-safe clipping

`Visible()` materializes the visible rows then clips each run-off-edge
row to `[off, off + width)` (`clipRow`): cells outside the window are
dropped, spans are translated into window cells and clipped (marker
spans keep following their cell, hidden when it leaves), and `Start`
advances to the first painted column. A grapheme cluster split by
either clip edge paints its in-window cells blank — never a half
glyph — the same policy `wrapLine` applies to a wrap-row split. Wrap
models pass through unclipped; the dormant offset never leaks into the
wrapped frame.

## Tests

- `internal/viewport/pan_test.go` — the pan-unit table including
  clamps at 0 and S; `SetOffset` clamping; wrap-mode dormancy and
  no-op pans; retention through `w w` with re-entry clamping on a
  changed visible set and a changed width; the mixed-width extent
  policy (299 while the long line shows, 9 after, no restoration);
  empty/placeholder/all-empty clamps to 0; the paintable boundary on a
  two-cell final cluster (S = its start, both cells painted) and on an
  unfittable final cluster (last fitting start, or 0 with clipping
  blanks); re-clamping on scroll, reveal, width resize, and row-model
  swap with a per-pan recomputation; split-cluster blanking at both
  edges; span translation; the uniform-lines hidden-left geometry; and
  the visible-rows-only query guard.
- `internal/app/pan_test.go` — the key wiring through `Update`: the
  six pan keys and their units, wrap-mode no-op, `w w` retention, the
  `n`-then-`p` file-change reset, a half-clipped CJK glyph painting
  blank in the real frame, and panning to the maximum leaving the
  final cluster fully painted with further pans inert.

See [unit-tests.md](unit-tests.md) § `internal/viewport` and
`internal/app`.

## Files

- `internal/viewport/viewport.go` — `off`, `Offset`/`SetOffset`, the
  six pan methods, `pan`, `halfW`, `clampOff`, `maxOffset`,
  `lineExtent`, `clipRow`, `Rows.Wrap`, and the `clamp`/`resolve`
  re-clamp points; `Visible` clips under a run-off-edge model.
- `internal/viewport/rows.go` — `Model.Wrap` reports the key's mode.
- `internal/app/browse.go` — `Model.pan`, `navigate`'s
  `SetOffset(0)` on file change.
- `internal/app/app.go` — the `,`/`.`/`<`/`>`/`[`/`]` key case and the
  `loadDoneMsg` current-path `SetOffset(0)`.

See also: [viewport-scrolling.md](viewport-scrolling.md) (the vertical
position this offset sits beside),
[wrap-mode.md](wrap-mode.md) (the mode panning is inert in),
[logical-anchor-and-layout.md](logical-anchor-and-layout.md) (the
resolve/clamp path the re-clamp rides),
[destination-reveal.md](destination-reveal.md) (the entry sequence the
reset precedes),
[match-navigation.md](match-navigation.md) (the file crossings that
reset the offset), and
[safe-presentation.md](safe-presentation.md) (the shared grapheme
policy the clipping consumes).
