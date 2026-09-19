# Matched-line navigation: circular cursor, cursor-derived current file (Issue #13)

Delivered by
[Issue #13](../issues/013-match-navigation-n-p-circular-cursor.md)
([task](../tasks/013-match-navigation-n-p-circular-cursor.md)): the
single global matched-line cursor in SearchIndex and its App wiring —
`n`/`p` circular navigation with the current file derived from the
cursor rather than a separate selection. Relevant PRD sections:
*Navigation, viewport, and logical anchors* (the first three bullets)
and *Module Design → SearchIndex* in
[`Notes/PRD-vrg.md`](../PRD-vrg.md); user stories 30, 49, 51, 56. See
also [search-collection.md](search-collection.md) for the index the
cursor walks, [viewport-scrolling.md](viewport-scrolling.md) for the
per-file saved viewports the file-change handoff consumes, and
[theme.md](theme.md) for the underline styles the selection drives.

## The cursor (`internal/searchindex/cursor.go`)

`Cursor{File, Stop}` identifies one navigation stop as indexes into
`Index.Files` and that file's `Stops`; the zero value is the startup
selection — the first stop in path-then-line order. `Index` holds the
position in `cur` plus `stops`, the prepared total stop count `Prepare`
computes (the same number `UsableResults` reports) for the no-op rules.

- `Index.Cursor() (Cursor, bool)` reports the position; `ok` is false
  when the index holds no stops — an empty index has no position at
  all.
- `Index.Next()` / `Index.Prev()` step one stop in path-then-line
  order and return a `Move{Wrapped, FileChanged}` report: `Wrapped`
  marks a step that crossed an index end (last→first, first→last) and
  `FileChanged` marks the destination landing in a different file than
  the step departed. A wrap inside a one-file index crosses the end
  without a file change, so the two flags are tracked independently —
  `FileChanged` compares the destination file to the departing one.
- **No-op rules**: with zero stops or exactly one stop both directions
  are strict no-ops returning the zero `Move` — no wrap, no file
  change, so no downstream reload or pop-up (`r` is the explicit
  one-entry retry route, Issue #27's). Multiple submatches on one
  matched line share their stop — the line, not the submatch, is the
  unit of navigation.
- Stop order is the prepared order — unsigned raw path bytes, then
  ascending line number — never stream order.

## App wiring (`internal/app`)

- `model.cur` is gone: `model.curFile()` reads `Index.Cursor()` and
  every consumer — `curKey`, `startLoad`, the file-list underline and
  scroll-keeping, the filename rule — derives the current file from it.
  The current matched line is likewise the cursor's stop's line number,
  so `browseView`'s `curLine` (which drives the Issue #7
  inverse-plus-underline `CurrentMatch` style) follows `n`/`p`
  directly.
- `n`/`p` are browse-state keys (`m.state == stateBrowse` in the key
  switch, after the overlay's precedence and `ctrl+c`'s global
  override) routed to `model.navigate(next bool)`. `navigate` steps the
  cursor, detects a strict no-op by comparing the cursor before and
  after, and on an actual transition applies Issue #14's destination
  reveal ([destination-reveal.md](destination-reveal.md)) before
  returning `tea.Batch(startLoad(), startPopup())` for the destination —
  only when the returned `Move.FileChanged`. The load leaf is
  deduplicated: a cached, in-flight, or failed path contributes no load
  command, so the Issue #15 file-change pop-up
  ([file-change-popup.md](file-change-popup.md)) is then the only leaf —
  it starts at selection on every crossing. A same-file move returns nil
  and re-styles
  the current matched line plus reveals its target row; the
  stale-layout prepared-layout path is Issue #17's.
- **Viewport handoff**: the departing file's viewport needs no explicit
  save — scrolling and moving reveals already write through to
  `model.vps` — so the destination's reveal starts from its saved `vps`
  entry or, for a first visit (including the startup file), the
  zero-value top of the file. While a destination file is uncached the
  panel switches to its "Loading…" placeholder immediately and
  navigation stays live: the cursor keeps moving under an in-flight
  load, a second request for the same path is dropped by `startLoad`'s
  dedup, and the reveal lands when the load completes.
  [Issue #25](async-load-isolation.md) has since keyed each completion
  by raw path plus request identity, so a late answer updates only its
  own path's cache — never the file the cursor has since moved to.
- **Manual scrolling leaves the cursor unchanged**: `scrollBy` writes
  only `m.vps`, so `n` after any amount of scrolling continues from the
  last selected stop (Issue #12's contract, now load-bearing).
- **The file list is passive**: it renders entries and underlines the
  cursor's file — the only way the underline moves — with no direct
  selection route; other keys never move the cursor.

## Tests

`internal/searchindex/cursor_test.go` (external package) drives the
cursor contract through real built indexes: startup at the first stop,
`Next`/`Prev` walking stops in path-then-line order with the
`Wrapped`/`FileChanged` reports, the within-one-file wrap reporting no
file change, the single-stop strict no-op, the empty index's absent
position, submatches sharing one stop, and prepared order beating
stream order.

`internal/app/nav_test.go` (same package) drives `n`/`p` through
`Update`: startup on the first stop with the current-line underline on
its match, a same-file `n` moving the underline with no command, a
cross-file `n` switching the panel — list underline and filename rule
move, the uncached load is requested, the completed file starts from
the top — circular wrap at both ends, the single-stop strict no-op (no
command, no frame change), scrolling leaving the cursor on its stop so
`n` continues from it and Issue #14's reveal moves the viewport to the
destination's target row, the departing file's saved viewport surviving
a leave-and-revisit as the reveal's starting point, navigation under an
in-flight load with dedup, and the passive file list ignoring
selection-shaped keys. See
[unit-tests.md](unit-tests.md).
