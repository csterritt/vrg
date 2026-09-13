# Explicit reload (Issue #27)

Issue #27 added the `r` explicit reload: pressing `r` rereads the
current file exactly once without rerunning ripgrep, without changing
cursor stops, and without revealing a match. The reload shows the
`Loading…` placeholder while pending, replaces stale content on
failure with `(unreadable)`, supports retries and the one-stop index,
and discards superseded layouts safely through content revisions and
the Issue #17 installation guard. Relevant PRD section: *File loading,
cache, reload, and selection consistency*. See
[async-load-isolation](async-load-isolation.md) for the one-load-per-path
rule that reload reuses,
[read-failures-and-retry](read-failures-and-retry.md) for the
`(unreadable)` placeholder and read-failure overlay that a failed
reload routes through, and
[logical-anchor-and-layout-preparation](logical-anchor-and-layout-preparation.md)
for the keyed prepared-layout installation and supersession guard that
the reload-anchor pending intent commits through.

## Problem

Before Issue #27, the model had no explicit reload path. The
`LayoutKey()` revision field was hardcoded to `1`, so a reread could
not invalidate a stale cached layout: a layout prepared before a
reread would still match the post-reread key and could install stale
rows over reloaded content. There was also no pending intent
distinguishing a reload (preserve the anchor, no reveal) from a
navigation/load completion (reveal the latest target). The PRD
required `r` to reread the current file without rerunning the search,
preserve the cursor and logical viewport anchor clamped to new
content, replace stale content with `(unreadable)` on failure, and
keep the cached content stable until `r` is pressed.

## Solution

Issue #27 added an `r` key route, a per-path content revision, a
reload-anchor pending intent, and a `reloadingPaths` record so the
completion handler can increment the revision only for reloads (not
for navigation or startup loads).

### The `r` key route

`handleReload()` is the entry point for `r` from the browse state and
from a read-failure overlay (so the user can retry a failed file
without dismissing the overlay first). It:

1. Records the current path in `reloadingPaths` so the
   `FileLoadCompleteMsg` handler knows this completion is a reload
   and increments the content revision.
2. Sets `loading = true` and clears `readFailed` so the panel switches
   from `(unreadable)` to `Loading…` on retry.
3. Does **not** set `needsReveal`: the anchor is preserved without
   revealing a match (PRD: "Reload by itself does not reveal a match").
4. Calls `startLoad`, which enforces the one-load-per-path rule
   (Issue #25): if a load is already in flight for the path, the
   request is dropped, not queued. No second load starts.

`r` does not rerun ripgrep, add or remove cursor stops, or change the
search index. The cursor and search-index stops are preserved across
the reload.

### Dropped duplicates and re-entry

Because `handleReload` routes through `startLoad`, the Issue #25
one-load-per-path rule applies:

- A duplicate `r` while the path's load is already in flight is
  dropped. The loader is not called again; no second load is queued.
- Re-entry (n/p navigation away and back) while a reload is in flight
  is likewise dropped under the one-load-per-path rule.
- The placeholder transition (`Loading…` to content or
  `(unreadable)`) is the only completion signal. After completion,
  another `r` starts a new load.

### Content revisions

`Model.revisions map[string]int` tracks per-path content revisions.
The default revision for a first load is `1`; each reload increments
the path's revision. `contentRevision(path)` returns the current
revision (defaulting to `1` when absent).

`LayoutKey()` now uses `m.contentRevision(string(m.currentPath))`
instead of the hardcoded `1`. The revision feeds the layout key so a
stale cached layout from a prior revision no longer matches the
current key and is discarded by the Issue #17 installation guard.

The revision is incremented in the `FileLoadCompleteMsg` handler
before the current-path check, so it is updated even if the user
navigated away while the reload was in flight. Only loads recorded in
`reloadingPaths` increment the revision; navigation and startup loads
do not.

### Reload-anchor pending intent

Issue #28 generalized the reload-anchor pending intent into the
`LoadIntent` enum. `Model.loadIntent LoadIntent` carries the intent
from load completion to the matching layout installation:

- `handleReload()` sets `loadIntent = IntentReloadAnchor`.
- When a reload's `FileLoadCompleteMsg` arrives for the current path
  with a buffer, the handler calls `buildViewport()`. The viewport may
  be nil while the new layout is pending. The intent is not committed
  at stage one.
- The `LayoutReadyMsg` handler installs the prepared `RowModel` only
  when `msg.Key` equals the current `LayoutKey()` (which now carries
  the new revision). On installation, the same-file anchor-preservation
  path (the viewport was non-nil from the prior revision's layout)
  reapplies the old anchor through `SetAnchor`, clamping to the new
  content. The handler then commits the intent via
  `commitLoadIntent()`, which for `IntentReloadAnchor` is a no-op
  (the anchor was already preserved) and clears the intent.
- A stale layout from a prior revision (e.g. a gated pre-reload layout
  released after the reload completes) has a key that no longer
  matches and is discarded without touching the visible panel, the
  anchor, or the intent.

If navigation occurs while a reload is in flight, `handleNavigate`
replaces `IntentReloadAnchor` with `IntentReveal` so the commit targets
the latest selection, not the saved anchor. See
[load-completion-two-stage](load-completion-two-stage.md).

### Anchor preservation and clamping

The anchor is preserved through the same-file rebuild path in the
`LayoutReadyMsg` handler: the old viewport's anchor is captured
before the new `RowModel` installs and reapplied via `SetAnchor`,
which recomputes the offset from the anchor and clamps to
`[0, maxOffset]`. When the new content is shorter than the old
content, the offset is clamped to the new `maxOffset` and the anchor
is updated to the new top row's location (the Issue #17 lossy EOF
clamp).

The anchor is asserted after the new revision's matching prepared
layout installs, not against the old revision's layout. A test that
gates the layout preparation verifies the anchor is preserved only
after the matching layout installs.

### Failure replacement

A failed reload routes through the Issue #26 read-failure path:

- `readFailed` is set true so the panel shows `(unreadable)`.
- `openReadFailureOverlay` opens a fresh non-fatal `OverlayError`
  (or appends to an already-open read-failure overlay).
- The old content is replaced: the panel no longer shows the prior
  buffer's rows.
- The cursor stops and search index are preserved; the filename row
  still identifies the path.

A second consecutive reload failure appends exactly one new
diagnostic occurrence to the open overlay text without resetting
`overlayScroll` (the Issue #26 append-preserving-scroll primitive).
The reader's overlay scroll position is preserved.

### One-stop index

`r` works with a one-stop index (a single matched line in a single
file). Because there is no navigation-based escape from a slow load
in the one-stop case, each retry attempt must complete before the
next `r` takes effect — the one-load-per-path rule ensures this.

### Cache stability until `r`

Cached content intentionally ignores disk edits until `r` is pressed
(PRD: "Cached content intentionally ignores disk edits until reload").
A simulated disk change (the loader would return new content) does
not trigger a reload on its own. The panel continues to show the
cached content until the user presses `r`.

## Model fields

- `loadIntent LoadIntent` — Issue #28: the pending intent carried for
  the next matching layout installation. `IntentReloadAnchor` for an
  explicit reload; `IntentReveal` for startup/navigation; `IntentNone`
  when no intent is pending. Supersedes the Issue #27
  `pendingReloadAnchor bool` field.
- `revisions map[string]int` — per-path content revisions. The
  default is `1`; each reload increments the path's revision.
- `reloadingPaths map[string]bool` — paths with an active reload
  request, so the completion handler increments the revision only
  for reloads.

## Public accessors

- `LoadIntent() LoadIntent` — Issue #28: returns the pending intent.
- `HasPendingReloadAnchor() bool` — reports whether a reload-anchor
  intent is carried (`loadIntent == IntentReloadAnchor`).

## Out of scope

- Match validation against reloaded content is owned by Issue #29.
  Issue #27 does not validate original search matches against the
  reloaded content.
- Unsupported encodings are owned by Issue #30.
- The generalized overlay append primitive is owned by Issue #32.

## Testing

See [unit-tests](unit-tests.md) for the `reload_test.go` catalog
covering the reload lifecycle, dropped duplicate reloads, dropped
re-entry while the same path is loading, the `Loading…` placeholder,
exactly one reread, no `rg` rerun, no cursor-stop changes, anchor
preservation after the matching new layout installs, clamping on
content shrink, failure replacement with `(unreadable)`, the
current-file failure overlay, second-failure append with overlay
scroll preserved, the one-stop index route, no reload on simulated
disk change, the filename row retaining the path, content revision
advancement, and the Issue #17 revision-supersession discard of a
gated pre-reload layout released after reload completion.
