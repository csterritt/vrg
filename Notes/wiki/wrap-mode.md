# Wrap mode and the `w` toggle

Issue #16
(`Notes/issues/016-wrap-mode-and-toggle.md`, tasks
`Notes/tasks/016-wrap-mode-and-toggle.md`) delivered wrap mode: lines
wrap at grapheme-cluster boundaries by default, `w` toggles to
run-off-edge mode and back, tabs expand structurally to eight-column
stops, and the viewport's prepared row model became a swappable value
keyed by what it was laid out for.

PRD cross-references: "Text, graphemes, and safe presentation" (the
shared grapheme policy, wrap-at-boundaries, and tab bullets), "Layout
and indicators" (the reserved-indicator and continuation-gutter
bullets), "Navigation, viewport, and logical anchors" (the rendered-row
scroll unit and the wrapped-target reveal), and user stories 64, 68,
and 71 under "Wrapping, indicators, and text display" in
`Notes/PRD-vrg.md`.

## The two modes and the toggle

- **Wrap mode is on initially** (`Model.wrap = true` in `newModel`).
  Every source line packs its grapheme clusters into rendered rows of
  at most the text width.
- **`w` in browse flips the flag** immediately and issues a keyed
  layout request for the new mode — since Issue #17 preparation runs
  off the update path (`syncLayout`/`ensureLayout` → `layoutCmd` →
  `layoutDoneMsg`), so the toggle stays responsive while the rewrap
  runs; a rapid second `w` supersedes the first request and its stale
  completion is discarded by the install guard. The retained anchor
  resolves against whichever model installs, restoring the logical
  column. In run-off-edge mode each source line is one rendered row
  carrying its full cells for the frame to clip — there is no
  horizontal panning yet (Issues #18–19 own pan).
- **Text width** is `panel width − gutter width − reserved indicator
  width`, where `reservedW()` returns **0 in wrap mode and 1 in
  run-off-edge mode** — the rightmost indicator column is reserved now
  and populated by Issue #20. Toggling therefore changes the text
  width even at the same terminal size, which is itself a layout
  change forcing row-model rebuild.

## Grapheme-boundary wrapping

`viewport.Prepare` (`internal/viewport/rows.go`) lays out wrap-mode
rows by packing *clusters*, never raw cells:

- A cluster that cannot fit the row's remaining cells moves whole to
  the next row, leaving the remainder of the previous row blank — the
  PRD's two-cell-cluster rule generalized to any multi-cell unit.
- A cluster wider than the whole row splits across rows as a last
  resort; `Row` blanks the clipped lead cell so its glyph cannot spill
  onto the next row, and a row entirely inside the unit carries only
  `Cont` cells that paint nothing.
- An end-of-line marker (a zero-width span at the line's end) extends
  the line by one cell: after a completely full final wrap row it
  occupies another row — a continuation row with no cells, painting
  one marker cell at column zero.
- **Continuation rows carry `Row.Cont`** and `contentRow` paints them
  behind a blank gutter of the same width, so wrapped text stays
  aligned with the row its source line leads with.
- Scroll units stay **rendered rows**; wrap mode simply makes the
  rendered-row→source-line mapping many-to-one.

## One shared grapheme policy

Segmentation and cell widths live in exactly one place:
`present.LineOf` emits each grapheme cluster (or escape form) as
`Cell`s where `Lead` marks the cluster's first cell — the only legal
wrap boundary — and `Cont` marks trailing cells of a multi-cell unit
whose lead's text already covers them. A cluster mixing printable and
dangerous forms falls back to per-rune escapes that still lead only on
the first unit, keeping the whole cluster one wrap unit.
`*filebuffer.Buffer` exposes these cells via `Cells(i)` — it satisfies
the new `viewport.Source` interface (`LineCount`/`Cells`/`Spans`), so
the row model consumes cluster boundaries without re-deriving
segmentation.

## Structural tab expansion

Tabs no longer render as Issue #5's provisional single-cell `→`.
`LineOf` expands a tab with real space cells to the **next multiple of
eight source-display columns** — measured in the line's own cell
coordinates, independent of the gutter, list width, and (future)
horizontal pan. The whole expansion is one cluster: its first cell
leads, so a wrap row boundary can only fall before the expansion or
inside it as a last-resort split. The tab byte's `lo`/`hi` map covers
the whole expansion, so a recorded submatch on a tab highlights every
expansion cell.

## The prepared row model

`viewport.Model` is the swappable `Rows` implementation, built by
`Prepare(src, Key)` and keyed by exactly `(Path, Rev, Width, Wrap)` —
path, content revision, text width, and wrap mode — the contract
Issue #17's asynchronous preparation relies on to detect stale
results. Layout runs once in `Prepare` (a `rowSpan` list plus each
line's first-row index); `Row(i)` materializes that row's cells and
translates the line's spans into row-local cells on demand — coverage
spans clipped to the row, marker spans painted on the row owning their
position — so a frame touches only the lines behind its visible rows.

App-side, `Model.rows` caches the prepared model per path as
`installed{key, rows}` and `m.revs` counts each path's content
revision (bumped on every successful load). Since Issue #17
preparation is asynchronous: `syncLayout` recomputes the geometry and
`ensureLayout` issues a `layoutCmd` worker keyed by the current
parameters when the installed model is missing or stale; the
`layoutDoneMsg` installs only while its key still equals
`layoutKey(path)`, so out-of-order and superseded completions are
inert — and `reqKey` keeps a superseded in-flight request from being
reissued or installed. See
[logical-anchor-and-layout.md](logical-anchor-and-layout.md).
`Viewport.Reveal` resolves a wrapped display target through
`RowOf`: the last of the line's rows whose first cell does not pass
the target cell, with boundary positions belonging to the next row —
so a match far down a source line taller than several screens lands
its containing rendered row at `floor(height/3)`.

## Tests

- `internal/viewport/wrap_test.go` — the row-model contract against a
  counting-fake `Source` backed by real `present.Line` segmentation:
  ASCII/wide/combining wrap row counts, the oversized-cluster split
  and its blanking, continuation and run-off-edge models, the key
  semantics, per-row span translation, end-of-line marker rows, the
  wrapped-target reveal landing `floor(h/3)`, and the
  render-cost guard proving `Visible()` queries only shown rows'
  lines.
- `internal/filebuffer/cluster_test.go` — the buffer's `Cells`
  carrying `Lead`/`Cont` boundaries for ASCII, wide, combining, and
  escaped units, plus eight-column tab-stop cells with the byte's span
  covering the expansion.
- `internal/app/wrap_test.go` — the toggle wiring: wrap on by default
  with blank continuation gutters, `w` switching to one clipped
  run-off-edge row (reserved column subtracted from text width) and
  back, and `n`/`p` reveals inside a screen-tall wrapped line.
- `internal/present/line_test.go` — the deferred Issue #5 tab
  positions now asserted: expansion width, stop positions, and the
  tab byte's span.

See [unit-tests.md](unit-tests.md) for the full catalog.

## Files

- `internal/viewport/rows.go` — `Source`, `Key`, `Model`, `Prepare`,
  `Row` (Issue #17 adds `Start`, the row's logical location),
  `RowOf`, `wrapLine`.
- `internal/viewport/viewport.go` — `Row.Cont`; `Rows`/`Target` doc
  updates for the many-to-one mapping; Issue #17's `anchor` resolves
  the effective top through the installed model.
- `internal/present/line.go` — `Cell.Lead`, the tab expansion, the
  `lead` flag through `emit` and the fallback cluster path.
- `internal/app/browse.go` — `reservedW`, `syncLayout`/`ensureLayout`/
  `layoutCmd` (Issue #17's off-path preparation), `currentRows`'s
  keyed install read, `contentRow`'s blank continuation gutter.
- `internal/app/app.go` — the `wrap` field (on initially), the `w`
  key case issuing the new-mode layout request, `revs`/`reqKey`
  preparation bookkeeping, the `layoutDoneMsg` install guard, the
  `loadDoneMsg` revision bump.

See also: [logical-anchor-and-layout.md](logical-anchor-and-layout.md)
(the async preparation and install-guard contract built on this key),
[viewport-scrolling.md](viewport-scrolling.md) (the rendered-
row position this model feeds),
[destination-reveal.md](destination-reveal.md) (the `RowOf` seam this
implements),
[safe-presentation.md](safe-presentation.md) (the line-presentation
policy it shares),
[browse-tracer.md](browse-tracer.md) (the panel it lays out), and
[theme-and-colour-toggle.md](theme-and-colour-toggle.md) (the other
browse-phase toggle).
