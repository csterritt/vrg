# Asynchronous load isolation — keyed completions and responsiveness

Issue #25
(`Notes/issues/025-async-load-isolation.md`, tasks
`Notes/tasks/025-async-load-isolation.md`) pinned the asynchronous
file-load contracts: navigation stays live while a file loads, load
completions are keyed by raw path *and* request identity so a late
result updates only its own file, at most one load is in flight per
path, successful buffers are retained for the session, and the
decode/map phase is separately gatable with input still actionable.

PRD cross-references: "File loading, cache, reload, and selection
consistency" (the first three bullets — async first-view loading with
session retention, path-plus-request-identity keying with panel
isolation, and navigation active while loading) and "Resources and
responsiveness" (no unbounded work on the update path; prepared
viewport data for rendering) in `Notes/PRD-vrg.md`.

## Navigation during a load

`n`/`p` step the matched-line cursor regardless of load state: the
cursor lives in `searchindex.Index` and a crossing into an uncached
file switches the panel immediately — filename rule, list underline,
`Loading…` placeholder — and issues that file's load. A user can move
past a slow file without waiting for it. Scrolling and panning a
placeholder are strict no-ops (`scroll`/`pan` return early when the
current path has no buffer and write no saved state), while keys with
ordinary browse meanings — `w`, `c`, `left`/`tab`/`right`/`shift+tab`,
`q`, `ctrl+c` — keep working mid-load.

## Path-and-request-identity keying

`ensureLoad` mints a request identity (`loadSeq`) per issued load and
records it in `loading[path]`; the worker command carries it and the
completion is `loadDoneMsg{path, req, buf, err}`. `Update` installs a
completion only when its request identity matches the path's in-flight
request — a stale or unsolicited completion is dropped without
touching the cache, the status maps, the diagnostics, or the panel.
After cancellation the `quit` guard at the top of `Update` discards
every message before this check runs, so a late worker result can
neither revive the UI nor fill the cache.

A completion accepted for a non-current path updates only that path's
entries — `bufs`, `revs`, and the stale-keyed `rows`/`reqKey` — and
returns without touching the viewport, the saved anchors, or the
visible panel. Only when the completed path is the current one does
the handler reset the horizontal offset and run `syncLayout` so the
prepared layout installs and the pending reveal commits (Issue #14's
destination reveal; the latest-target rule is Issue #28's).

## One load per path, session retention

- `ensureLoad` drops a request when the path is already `loading`,
  cached, or failed — dropped, not queued: re-entering a loading path
  returns no command and mints nothing, so the in-flight request's
  identity is unchanged and its completion still installs. The
  `failed` half has an Issue #26 exception: a cross-file entry routes
  through `entryLoad`, which deliberately mints exactly one retry —
  still dropped if a request is somehow in flight — see
  [read-failures.md](read-failures.md).
- `bufs` retains every successful buffer for the session — no
  eviction. A revisit shows the cached content and issues no new load;
  the only command it may return is an Issue #17 layout request when
  no installed row model matches the current parameters.

## Separately gated decode/map

`filebuffer.Load` is split into `ReadFile` (the read phase) and
`Prepare` (the decode/map phase — split, escape, map, validate), so a
test can hold the expensive phase while the read has already
completed. `loadCmd` consults three seams: `loadGate` before the read
holds the whole worker, `mapGate` after the read holds decode/map
alone — a read-phase failure (missing file) completes without ever
reaching `mapGate`, proving the two phases are independently gated —
and Issue #26's `readFile` swaps the read phase itself, so failure
tests inject deterministic loaders instead of relying on filesystem
permissions.
With `mapGate` held, `ctrl+c` exits 130, `n`/`p` navigate, `w`/`c`
toggle, and resizes apply — none wait on the worker — because the
completion already carries a *prepared* buffer and `Update` does no
full-file decoding. The analogous contract for layout/rewrap is Issue
#17's (see
[logical-anchor-and-layout.md](logical-anchor-and-layout.md)).

## Tests

- `internal/app/load_test.go` — the Issue #25 contracts:
  `gatedModel` wires the seams (`loadGate`, `mapGate`, suppressed
  pop-up timer); `mintLoad` registers an in-flight request identity so
  tests can inject completions whose real worker never ran.
  `TestNavigationActiveWhileLoadInFlight` navigates past a gate-held
  load; `TestPlaceholderScrollAndPanAreNoOp` covers the placeholder
  no-ops and the still-normal keys; `TestReEnterLoadingPathIsDroppedNotQueued`
  pins the one-load rule; `TestLateCompletionIsolatedToItsPath` runs
  A→B→C with late completions for the departed files leaving the
  current panel byte-identical and the cached revisits issuing no new
  load; `TestLoadCompletionKeyedByRequestIdentity` drops stale and
  unsolicited completions;
  `TestLoadCompletionAfterQuitDiscarded` covers post-cancellation
  rejection; `TestDecodeMapGateKeepsInputsActionable` holds the
  decode/map phase while every listed input applies; and
  `TestMapGateHoldsDecodeMapNotRead` proves the gate sits after the
  read. Existing tests that inject `loadDoneMsg` directly now mint or
  read the in-flight request identity first (`browse_test.go`,
  `popup_test.go`, `replay_test.go`, `filelist_test.go`,
  `sinksafety_test.go`).

See [unit-tests.md](unit-tests.md) § `internal/app`.

## Files

- `internal/app/app.go` — `loading` (path → in-flight request
  identity), `loadSeq` (the mint), `mapGate` and Issue #26's
  `readFile` (the decode/map and read-phase test seams),
  and the `loadDoneMsg` install guard in `Update`.
- `internal/app/browse.go` — `loadDoneMsg`'s `req` field,
  `ensureLoad`'s mint-and-drop rule, `loadCmd`'s two-phase worker
  (`readFile`/`ReadFile` then `Prepare` behind `mapGate`), and Issue
  #26's `entryLoad` (the failed-path re-entry exception minting
  exactly one retry).
- `internal/filebuffer/filebuffer.go` — `Load` split into `ReadFile` +
  `Prepare`; `Prepare` carries the split/escape/map/validate body.

See also: [browse-tracer.md](browse-tracer.md) (the browse view and
placeholders this loading feeds),
[match-navigation.md](match-navigation.md) (the cursor steps that stay
live mid-load),
[destination-reveal.md](destination-reveal.md) (the reveal a
current-path completion commits),
[logical-anchor-and-layout.md](logical-anchor-and-layout.md) (the
analogous keyed layout preparation),
[viewport-scrolling.md](viewport-scrolling.md) (the saved-state and
scroll contracts the placeholder no-op protects),
[stderr-replay.md](stderr-replay.md) (where a non-current failure's
diagnostic surfaces),
[cancellation-and-cleanup.md](cancellation-and-cleanup.md) (the quit
guard a late completion hits), and
[file-list-layout.md](file-list-layout.md) (the status-note slot a
settled placeholder hands to).
