## Issue 42: A dropped `r` request must not change revision or reveal intent

**Type**: AFK
**Blocked by**: None — can start immediately

### Parent PRD

`Notes/PRD-vrg.md` — audit source: `Notes/critiques/final-audit-vrg.md`, High finding 7

### What to build

Make the one-load-per-path admission check atomic with reload-state mutation (`internal/app/app.go:1305-1318`, `2140-2177`, `2852-2873`). `handleReload` currently marks the path as reloading, switches to `IntentReloadAnchor`, and changes presentation *before* `startLoad` checks whether that path already has a load in flight. If `startLoad` drops the request, the already-running startup/navigation load is later misclassified as a reload: its revision is incremented and its pending destination reveal can be replaced by anchor preservation. Pressing `r` during the initial or navigation load can therefore suppress the required match reveal even though no reload actually started — a silent change of load intent.

- Only set reload flags, reload presentation, and `IntentReloadAnchor` after a new load request has actually been accepted; the admission check and the state mutation happen atomically (single decision point, no intermediate committed state).
- A dropped `r` leaves revision, intent, and presentation exactly as they were; the in-flight load completes under its original classification.
- The accepted-reload path is unchanged: `IntentReloadAnchor`, reloading presentation, and revision bump happen precisely when the new request starts.
- Scope this atomic-admission rule specifically to `handleReload` and the `r` path. Do not gate the navigation re-entry path (`internal/app/app.go:2100-2135`) behind successful load admission: re-entry legitimately changes the current selection, placeholder presentation, and `IntentReveal` even when `startLoad` drops a duplicate load.

See PRD *File loading, cache, reload, and selection consistency* (duplicate `r` dropped; placeholder change is the completion signal) and *Navigation, viewport, and logical anchors*.

### How to verify

- **Manual**: with a slow-loading file (test seam or genuinely large file), press `r` while the startup or navigation load is still in flight → nothing visible changes, and when the load completes the destination match is revealed per the normal rules — not anchor-preserved as if a reload had happened.
- **Automated**: a test pressing `r` during startup/navigation loading asserting the in-flight load's completion is classified by its original intent (destination reveal, no extra revision bump), alongside the existing test for a second `r` during an *accepted* reload; plus a test that an accepted `r` still produces exactly one revision increment and anchor-preserving intent.

### Acceptance criteria

- [ ] Given an in-flight load for the current path, when `r` is pressed and the new request is dropped, then revision, reveal intent, and presentation are unchanged.
- [ ] Given `r` dropped during a startup load, then the load's completion performs the required first-match reveal per story 50, not anchor preservation.
- [ ] Given `r` dropped during a navigation load, then the pending destination reveal proceeds per the latest-target rules and is not replaced by `IntentReloadAnchor` behaviour.
- [ ] Given an `r` accepted after the previous load finished, then reload flags, presentation, intent, and exactly one revision increment are applied.
- [ ] Given rapid repeated `r` presses, then at most one reload is in flight per path and the completion signal (placeholder → content/unreadable) remains the visible contract.
- [ ] Given navigation re-entry while a duplicate path load is already in flight, then selection, placeholder presentation, and `IntentReveal` still update; the atomic-admission restriction applies only to `handleReload`.

### User stories addressed

- User story 42: repeated load requests for a file already loading are dropped rather than queued
- User story 41: reload preserves reading position unless the user navigates during the load
- User story 50: startup selects the first matched line and reveals its first match
- User story 54: minimal horizontal reveal applies at startup and on every navigation action

---
