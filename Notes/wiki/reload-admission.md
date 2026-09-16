# Atomic reload admission (Issue #42)

Issue #42 made the one-load-per-path admission check and the
reload-state mutation a single decision point in `handleReload`
(`internal/app/app.go`). Reload flags, "Loading…" presentation, and the
`IntentReloadAnchor` intent are applied only when a new load request is
actually accepted; a dropped `r` leaves revision, intent, and
presentation exactly as they were so the in-flight startup or
navigation load completes under its original classification. Relevant
PRD sections: *File loading, cache, reload, and selection consistency*
and *Navigation, viewport, and logical anchors*. See
[explicit-reload](explicit-reload.md) for the `r` contract,
[async-load-isolation](async-load-isolation.md) for the
one-load-per-path rule the admission check enforces, and
[load-completion-two-stage](load-completion-two-stage.md) for the
intent the mutation would otherwise corrupt.

## Problem

Before Issue #42, `handleReload` recorded the current path in
`reloadingPaths`, switched the panel to `Loading…`, and set
`loadIntent = IntentReloadAnchor` *before* `startLoad` checked whether
a load was already in flight for that path. When `startLoad` then
dropped the duplicate request, the mutations were already committed:

- `reloadingPaths[path]` stayed marked, so the in-flight startup or
  navigation load's completion was misclassified as a reload and the
  path's content revision was incremented even though no reload ever
  started.
- `loadIntent` was replaced with `IntentReloadAnchor`, so the in-flight
  load's pending destination reveal could be supplanted by anchor
  preservation — pressing `r` during the startup or a navigation load
  silently suppressed the required first-match reveal (story 50).

## Solution

`handleReload` now checks `loadingPaths` for the current path *first*.
If a load is already in flight, the request is dropped and the model is
returned unchanged — no `reloadingPaths` record, no presentation
change, no intent replacement. Only when the new load request will be
accepted does `handleReload` apply the reload mutations (reload flag,
`Loading…` presentation with `readFailed`/`unsupportedEncoding`
cleared, `IntentReloadAnchor`) and call `startLoad`. There is no
intermediate committed state: admission and mutation are one decision
point.

### Dropped `r` contract

- Revision, `loadIntent`, and presentation are untouched, so the
  in-flight load completes under its original classification.
- A dropped `r` during the startup load preserves `IntentReveal`, so
  the completion performs the required first-match reveal (story 50)
  rather than anchor preservation.
- A dropped `r` during a navigation load likewise leaves the pending
  destination reveal intact per the latest-target rules.

### Accepted `r` contract (unchanged)

When no load is in flight, `r` applies the reload flags, the `Loading…`
presentation, `IntentReloadAnchor`, and exactly one revision increment
at completion — precisely when the new request starts. Rapid repeated
`r` presses keep at most one reload in flight per path; the
placeholder→content/`(unreadable)` transition remains the only
completion signal.

### Navigation re-entry is ungated

The atomic-admission restriction applies only to `handleReload`.
Navigation re-entry into a path whose load is already in flight still
updates the current selection, the placeholder presentation, and
`IntentReveal` even though `startLoad` drops the duplicate load —
re-entry legitimately reselects the file the user is looking at.

## Testing

See [unit-tests](unit-tests.md) for the `reload_admission_test.go`
catalog covering the dropped-`r` intent/revision/presentation
preservation during startup and navigation loads, the accepted-`r`
single revision increment, the rapid-press single-in-flight rule, and
ungated navigation re-entry.
