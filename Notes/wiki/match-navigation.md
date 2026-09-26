# Match navigation — the circular matched-line cursor

Issue #13
(`Notes/issues/013-match-navigation-n-p-circular-cursor.md`, tasks
`Notes/tasks/013-match-navigation-n-p-circular-cursor.md`) delivered the
single global matched-line cursor in `internal/searchindex` and its App
wiring: `n`/`p` step through the stops circularly, the current file
derives from the cursor, and the file list's underline follows it.

PRD cross-references: "Navigation, viewport, and logical anchors" (the
first three bullets — one global cursor, cursor-derived current file,
no direct file-selection route) and "Module Design → SearchIndex" in
`Notes/PRD-vrg.md`.

## The cursor

`Index` owns one cursor (`index.go`'s `cursor` field — an index into
the sorted `order` slice). Its zero value selects the first stop once
`Prepare` has sorted, which is the startup contract: browsing begins on
the first matched line of the first file in path-then-line order
(unsigned raw path bytes, then ascending line number — see
[searchindex-records-and-stops.md](searchindex-records-and-stops.md)).
`Prepare` clamps the cursor back into range whenever the sorted stop
set is rebuilt.

`internal/searchindex/cursor.go` exposes the cursor:

- `Current() (Stop, bool)` — the selected stop, false on an empty
  index.
- `Next() Step` / `Prev() Step` — circular advance/retreat. `Step`
  carries the selected `Stop` plus the transition flags the App wires
  on: `Moved` (false for the strict no-ops), `FileChanged` (the
  destination is in a different file than the departed stop), and
  `Wrapped` (the step passed an index end).

## No-op rules and stop granularity

- **Empty index**: `Current` is false and both directions report
  `Moved == false` — an explicit no-op.
- **One stop**: both directions are strict no-ops — no movement, no
  `FileChanged`, no `Wrapped` — so the app issues neither a pop-up nor
  a reload. `r` remains the explicit one-entry retry route (Issue #27).
- **Multiple submatches on one line** are one stop: the line is visited
  once however many ranges it carries (stops merge by raw path + line
  since Issue #3).
- **Wrap**: `n` on the last stop selects the first; `p` on the first
  selects the last — each flagged `Wrapped`, and `FileChanged` when the
  ends live in different files.

## App wiring

`Update` routes `n`/`p` to `Model.navigate` (`internal/app/browse.go`)
only in `phaseBrowse`; during searching they stay inert, and an open
overlay still owns the keyboard ahead of them.

- **Current file derives from the cursor.** `currentStop`/`currentPath`
  read `m.index.Current()` — the model no longer keeps its own cursor
  field — so the panel, the filename rule, the gutter, the
  current-matched-line underline (Issue #7's `CurrentMatch` style), and
  the file list's underlined entry all follow the cursor automatically.
- **Same-file steps** change only the current-line styling; the
  viewport does not move — destination reveal is Issue #14's.
- **File-crossing steps** (`step.FileChanged`) save the departing
  file's top row into `m.saved`, relayout the panel (installing the new
  file's prepared rows or none), and `SetTop` to the new file's saved
  vertical state — top of file on a first visit. `ensureLoad` then
  requests the destination's load when it is neither cached nor in
  flight, so an uncached destination shows `Loading…` until its
  `loadDoneMsg` arrives — whose existing completion path applies the
  same saved-state restore. Issue #17 owns the stale-layout request
  path and Issue #14 the reveal; this issue's handoff is the immediate
  switch plus the saved-viewport restore.
- **Manual scrolling** never touches the cursor: `n`/`p` continue from
  the last selected stop, not from the scrolled position.
- **The file list is passive**: there is no direct selection route —
  `n`/`p` are the only keys that move the cursor, and the underlined
  entry simply follows it.

## Tests

`internal/searchindex/cursor_test.go` pins the index-level contract
against a two-file fixture whose records arrive out of order; the app
side lives in `internal/app/nav_test.go`. See
[unit-tests.md](unit-tests.md) § `internal/searchindex` and
`internal/app`.

## Files

- `internal/searchindex/cursor.go` — `Step`, `Current`, `Next`,
  `Prev`, `step`.
- `internal/searchindex/index.go` — the `cursor` field, its clamp in
  `Prepare`, and the shared `export` helper.
- `internal/app/browse.go` — `navigate`, `currentStop` reading the
  index cursor.
- `internal/app/app.go` — the `n`/`p` key case; the model's own cursor
  field is gone.

See also: [viewport-scrolling.md](viewport-scrolling.md) (the per-file
saved state this handoff consumes),
[searchindex-records-and-stops.md](searchindex-records-and-stops.md)
(the stop ordering the cursor walks),
[browse-tracer.md](browse-tracer.md) (the two-pane view it drives),
[theme-and-colour-toggle.md](theme-and-colour-toggle.md) (the
current-line underline it moves).
