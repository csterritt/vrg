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
overlay still owns the keyboard ahead of them. Since Issue #14 every
actual transition ends in `reveal()` — see
[destination-reveal.md](destination-reveal.md).

- **Current file derives from the cursor.** `currentStop`/`currentPath`
  read `m.index.Current()` — the model no longer keeps its own cursor
  field — so the panel, the filename rule, the gutter, the
  current-matched-line underline (Issue #7's `CurrentMatch` style), and
  the file list's underlined entry all follow the cursor automatically.
- **Same-file steps** change the current-line styling and run the
  destination reveal: an on-screen target leaves the viewport put, a
  hidden one scrolls it to the one-third row (Issue #14; see
  [destination-reveal.md](destination-reveal.md)).
- **File-crossing steps** (`step.FileChanged`) save the departing
  file's logical anchor into `m.saved`, recompute the layout geometry
  (the destination's gutter may differ), install the destination's
  cached rows only while their key matches the current parameters —
  Issue #17's fast path — `SetAnchor` its saved position, top of
  file on a first visit, and — Issue #18 — reset the horizontal pan
  offset to zero before the reveal (see
  [horizontal-panning.md](horizontal-panning.md)). A stale-keyed or missing cached layout gets
  a keyed preparation request; the saved-anchor restore resolves when
  it installs and the carried reveal intent then commits for the
  newest selected stop — see
  [logical-anchor-and-layout.md](logical-anchor-and-layout.md).
  `ensureLoad` requests the destination's load when it is neither
  cached nor in flight, so an uncached destination shows `Loading…`
  until its `loadDoneMsg` arrives — whose completion issues the
  layout request that completes the same sequence. Issue #25 keys that
  completion by request identity and keeps navigation itself live
  while a load runs — see
  [async-load-isolation.md](async-load-isolation.md). Since Issue #15 a
  crossing also opens the file-change pop-up at selection time, its
  instance-keyed expiry batched with the load and layout commands —
  see [file-change-popup.md](file-change-popup.md).
- **Manual scrolling** never touches the cursor: `n`/`p` continue from
  the last selected stop, not from the scrolled position.
- **The file list is passive**: there is no direct selection route —
  `n`/`p` are the only keys that move the cursor, and the underlined
  entry simply follows it.

## Tests

`internal/searchindex/cursor_test.go` pins the index-level contract
against a two-file fixture whose records arrive out of order; the app
side lives in `internal/app/nav_test.go`, with the reveal assertions
Issue #14 added in `internal/app/reveal_test.go`. See
[unit-tests.md](unit-tests.md) § `internal/searchindex` and
`internal/app`.

## Files

- `internal/searchindex/cursor.go` — `Step`, `Current`, `Next`,
  `Prev`, `step`.
- `internal/searchindex/index.go` — the `cursor` field, its clamp in
  `Prepare`, and the shared `export` helper.
- `internal/app/browse.go` — `navigate`, `reveal` (Issue #14; pending
  intent since Issue #17), and `currentStop` reading the index cursor.
- `internal/app/app.go` — the `n`/`p` key case; the model's own cursor
  field is gone.

See also: [logical-anchor-and-layout.md](logical-anchor-and-layout.md)
(the anchor handoff and pending reveal intent a crossing carries),
[async-load-isolation.md](async-load-isolation.md) (the load the
crossing may start and the mid-load navigation contract),
[destination-reveal.md](destination-reveal.md) (the reveal
every actual transition now triggers),
[file-change-popup.md](file-change-popup.md) (the pop-up every file
crossing now opens),
[viewport-scrolling.md](viewport-scrolling.md) (the per-file
saved state this handoff consumes),
[searchindex-records-and-stops.md](searchindex-records-and-stops.md)
(the stop ordering the cursor walks),
[browse-tracer.md](browse-tracer.md) (the two-pane view it drives),
[theme-and-colour-toggle.md](theme-and-colour-toggle.md) (the
current-line underline it moves).
