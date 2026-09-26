# Viewport scrolling and per-file viewport state

Issue #12
(`Notes/issues/012-manual-vertical-scrolling-and-per-file-viewport.md`,
tasks
`Notes/tasks/012-manual-vertical-scrolling-and-per-file-viewport.md`)
grew the `internal/viewport` seam from Issue #5 into a real reading
position: manual vertical scrolling over the current file's prepared
rows, clamped to valid content, with each file's vertical position
saved in the model for revisits.

PRD cross-references: "Navigation, viewport, and logical anchors" (the
scroll-unit bullet and the clamping rules) and "Module Design →
Viewport" in `Notes/PRD-vrg.md`.

## Scroll units

`viewport.Viewport` (`internal/viewport/viewport.go`) exposes the six
scroll keys' units as methods, all in *rendered rows*:

- `Up`/`Down` — one rendered row (`up`/`down` arrow keys).
- `HalfUp`/`HalfDown` — `max(1, floor(height / 2))` (`u`/`d`), so odd
  heights floor and a one-row viewport still moves one row.
- `PageUp`/`PageDown` — one full page, the content height (`pgup`/
  `pgdown`).

*Content height* is the file-panel height minus the filename rule row:
`Model.syncLayout` resizes the viewport with `m.height - 1` and with the
terminal width minus the file list and the gutter.

## Clamping and the logical anchor

`clamp` keeps the top row in `[0, max(0, count − height)]`:

- **BOF**: the top row never goes below 0; `up`/`u`/`pgup` at the top
  of the file do nothing.
- **EOF**: the top never passes the last full page, so no avoidable
  blank rows appear below the final line — at EOF the file's last row
  sits on the bottom row.
- **Short files**: content shorter than the viewport has `maxTop` 0, so
  every scroll key is a no-op and the unused rows are simply left
  blank — natural underscroll, not overscroll.
- **Resize**: `Resize` re-resolves the top from the retained anchor and
  re-clamps; growth that would expose avoidable blanks pulls the top
  upward. Per the PRD this EOF clamp is intentionally lossy — the
  anchor is rewritten to the clamped row, so a later shrink need not
  restore the old top.

Since Issue #17 the true position is the logical anchor — a `(source
line, display-column offset)` `Target` the effective top resolves from
after every rewrap or row swap; a scroll that moves replaces it with
the resulting top row's location while one clamped to no movement
keeps it. See
[logical-anchor-and-layout.md](logical-anchor-and-layout.md).

## Placeholder no-op

A file with no prepared rows — the `Loading…` placeholder, the
`(unreadable)` placeholder, or an empty buffer — gives the viewport a
zero row count: every scroll clamps to top 0, `Reveal` is inert, and
`Visible()` returns nothing. The app additionally gates `Model.scroll`
on a loaded buffer for the current path, so scroll keys outside browse
or on a placeholder change nothing — not even the saved-state map.

## Destination reveal

Issue #14 added `Viewport.Reveal(Target)` — the vertical reveal of the
navigation destination: the target is a `(line, cell)` display location
(the first submatch's start cell), resolved to its rendered row through
the `Rows.RowOf` provider method so wrap mode's many-to-one mapping is
honored; an already-visible row never scrolls, a hidden row lands at
`floor(height / 3)` clamped to `[0, maxTop]` (BOF/EOF content wins),
and the bool report tells the app whether to replace the file's saved
state. The full contract and its triggers live in
[destination-reveal.md](destination-reveal.md).

## Per-file saved vertical state

`Model.saved` (`map[string]viewport.Target`, keyed by raw path bytes —
row ordinals until Issue #17) records each file's logical anchor
whenever a scroll key moves it — the per-file vertical viewport state
the PRD requires for revisits, now width-independent so it survives
rewraps while the file is away. Entering a file `SetAnchor`s its saved
value — absent means a first visit, the zero-valued top-of-file anchor —
and the prepared layout resolves it on install. Since Issue #13 `n`/`p`
file crossings drive the same restore on entry — `navigate` saves the
departing file's anchor and `SetAnchor`s the destination's saved state —
and since Issue #14 the destination reveal then runs over that starting
point: a moving reveal replaces the saved anchor with the new one,
while a no-scroll reveal leaves it, logical column included (see
[destination-reveal.md](destination-reveal.md)). Manual scrolling never
moves the matched-line cursor, which lives in `Index` itself (see
[match-navigation.md](match-navigation.md)).

## Prepared-row rendering

Rendering no longer scans the buffer per frame:

- `viewport.Rows` is the prepared rendered-row provider — `Len()` plus
  `Row(i)` returning a `viewport.Row{Line, Start, Cont, Cells, Spans}`;
  `Start` (Issue #17) is the row's display-column offset in its source
  line, the logical location an anchor restores. Since Issue #16 the
  provider is `viewport.Model` built by `viewport.Prepare` over
  `*filebuffer.Buffer` (satisfying `viewport.Source`): wrap mode makes
  the row→line mapping many-to-one with `Cont` continuation rows, and
  run-off-edge keeps row *i* = source line *i* (the Issue #12
  `bufferRows` adapter is gone). Since Issue #17 `Prepare` runs in a
  worker command and `Update` installs the delivered model only while
  its `(path, revision, width, wrap)` key matches the current
  parameters. See [wrap-mode.md](wrap-mode.md) and
  [logical-anchor-and-layout.md](logical-anchor-and-layout.md).
- `Viewport.Visible()` materializes only the visible slice —
  `min(height, count − top)` rows — so the provider's `Row` is queried
  once per shown row and never for rows outside the frame. The
  counting-fake tests (`TestVisibleQueriesOnlyVisibleRows`,
  `TestRenderQueriesOnlyVisibleRows`) pin this render-cost guard.
- `renderBrowse` takes the `Visible()` slice once per frame and
  `contentRow` reads `vis[row]` for the gutter number, cells, and
  spans.

## Files

- `internal/viewport/viewport.go` — `Row` (with `Start`), `Target`,
  the `Rows` provider interface (now with `RowOf`), and `Viewport`
  (`Resize`, `SetRows`, `SetAnchor`/`Anchor`, `SetTop`/`Top`,
  `Height`, the six scroll methods, `Reveal`, `Visible`, `resolve`).
- `internal/app/browse.go` — `Model.scroll`, `Model.reveal`,
  `syncLayout`/`ensureLayout`'s keyed layout requests and installs,
  `contentRow` over the visible slice (continuation rows behind a
  blank gutter).
- `internal/viewport/rows.go` — `Source`, `Key`, `Prepare`, `Model` —
  the swappable prepared row model (Issue #16; see
  [wrap-mode.md](wrap-mode.md)).
- `internal/app/app.go` — the `rows`/`reqKey`/`saved`/`revs` maps, the
  `listW`/`textW`/`fileIdx` caches, the scroll-key case in `Update`,
  and the `layoutDoneMsg` install guard.

See also: [logical-anchor-and-layout.md](logical-anchor-and-layout.md)
(the Issue #17 anchor and prepared-layout contract),
[destination-reveal.md](destination-reveal.md) (the Issue #14
reveal contract built on this position),
[browse-tracer.md](browse-tracer.md) (the two-pane view this
scrolls), [searchindex-records-and-stops.md](searchindex-records-and-stops.md)
(the raw path keying `saved` inherits),
[cancellation-and-cleanup.md](cancellation-and-cleanup.md) (the exit
path scroll keys still defer to).
