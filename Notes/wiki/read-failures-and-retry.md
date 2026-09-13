# Read failures and retry rules (Issue #26)

Issue #26 made unreadable-file behavior explicit and robust so the
TUI never gets stuck on `Loading…`, the search-derived exit status
is preserved when a file read later fails, diagnostics are collected
for replay, and re-entry into a previously failed file follows a
deterministic retry sequence. Relevant PRD section: *File loading,
cache, reload, and selection consistency*. See
[async-load-isolation](async-load-isolation.md) for the keyed load
isolation that Issue #26 builds on,
[outcome-contract](outcome-contract.md) for the fixed exit status
that load failures must not change, and
[browse-tracer](browse-tracer.md) for the base browse view and
overlay handling.

## Problem

Before Issue #26, `FileLoadCompleteMsg.Err` was ignored. A failed
read left the panel showing `Loading…` indefinitely, no diagnostic
was collected, the cursor stops were effectively marooned behind a
placeholder, and re-entering the failed file had no defined behavior.
There was also no guarantee that a later load failure would not
recompute the already-fixed search-derived exit status.

## Solution

Issue #26 added read-failure state, a non-fatal read-failure
overlay, an `(unreadable)` placeholder, a sanitized failed-path
registry, the same-file-step versus cross-file retry distinction,
and a deterministic re-entry sequence.

### Read-failure state

- `Model.readFailed bool` marks the current file's last load attempt
  as failed. When true, the content panel shows `(unreadable)`
  instead of `Loading…`.
- `Model.failedPaths map[string]string` tracks failed raw paths to
  their sanitized diagnostics. A successful load removes the path
  from this map. A started retry does not remove the path until it
  completes (so a re-entry that is dropped under the one-load-per-path
  rule still shows the prior-failure overlay).
- `Model.overlayReadFailure bool` marks the open overlay as a
  read-failure overlay so dismissal can clear the read-failure
  overlay state separately from the fatal-overlay state.

### Current-file failure

When a `FileLoadCompleteMsg` for the current path carries an `Err`:

1. `readFailed` is set true so the panel shows `(unreadable)`.
2. The sanitized diagnostic is recorded in `failedPaths`.
3. The diagnostic is collected for replay.
4. `openReadFailureOverlay` opens a fresh non-fatal `OverlayError`
   (unless a search-complete overlay is already open, in which case
   the search overlay takes precedence and only the panel state and
   `failedPaths` are updated).
5. The cursor and search index are preserved; the cursor stops
   remain navigable.
6. The fixed `ExitCode` is not recomputed.

### Non-current failure

When a `FileLoadCompleteMsg` for a non-current path carries an
`Err`, the visible panel is not disturbed. The sanitized diagnostic
is recorded in `failedPaths` and collected for replay. The failure
is discovered by visiting that file (which reopens the prior-failure
overlay) or through Issue #11 replay. No overlay is opened for a
non-current failure.

### Read-failure overlay

`openReadFailureOverlay` opens or appends to the read-failure
overlay:

- When no overlay is open, a fresh non-fatal `OverlayError` is
  opened with the sanitized diagnostic, `overlayReadFailure` is
  set, and `overlayScroll` is reset to 0.
- When a read-failure overlay is already open (a re-entry retry
  failed again), the new diagnostic occurrence is appended to
  `overlayText` without resetting `overlayScroll`.
- A search-complete overlay takes precedence: if one is already
  open, the read failure is recorded in `failedPaths` and the panel
  state is updated, but the search overlay is not changed.
- The file-change pop-up is cancelled when opening a read-failure
  overlay.

Dismissing a non-fatal overlay with `q` or Escape clears
`overlayReadFailure` and the overlay state, returning to the browse
state. Fatal-overlay dismissal behavior is unchanged.

### Filename row and placeholder rendering

The filename row uses `(unreadable)` as the default status note when
the current file has failed to load and no explicit
`WithStatusNote` callback overrides it. The path is truncated to
make room for the note, including at constrained widths. A
`truncateRightCells` helper truncates the status note when it would
overflow the panel, reserving at least one cell for the path.

`renderContentPanel` shows `(unreadable)` instead of `Loading…` when
`readFailed` is true.

### Same-file step versus cross-file retry

- A same-file `n`/`p` step requests no reload. The cursor moves
  within the same file; only the current matched line styling
  changes.
- Entry from a different file (cross-file navigation into an
  uncached, previously failed file) requests exactly one retry
  load. The retry starts immediately while the prior-failure
  overlay is open; it does not wait for dismissal.

### Re-entry sequence

Re-entering a previously failed file from a different file follows
a deterministic five-step sequence:

1. The prior-failure overlay reopens immediately (before the retry
   completes) with the recorded sanitized diagnostic.
2. The panel switches from `(unreadable)` to `Loading…` (the retry
   is in flight). `readFailed` is cleared; `loading` is set.
3. Exactly one retry load starts immediately while the overlay is
   open. The one-load-per-path rule (Issue #25) is reused: if a
   load is already in flight for the path, the request is dropped
   and no second load starts.
4. `Esc` dismisses the overlay without disturbing the in-flight
   load. The load continues; its completion updates the file's
   state/cache and, if the path is still current, the panel.
5. Settlement updates the placeholder to content (success) or
   `(unreadable)` (failure) without waiting for dismissal:
   - On success, `readFailed` is cleared, the buffer is cached and
     shown, the path is removed from `failedPaths`, no new
     diagnostic is collected, and the prior-failure overlay remains
     displayed until dismissed.
   - On a second failure, exactly one new diagnostic occurrence is
     appended to the open overlay text without resetting
     `overlayScroll` (the reader's scroll position is preserved),
     and exactly one new occurrence is collected for replay.

### Navigation away during retry

Navigating away during an in-flight retry follows Issue #25's
keyed completion isolation: the completion updates only that file's
state/cache, not the visible panel. A later re-entry follows the
same sequence against the new prior state (e.g. a successful retry
removes the path from `failedPaths`, so a later re-entry shows the
cached content with no overlay).

### Composed-view robustness

The composed view stays well-formed at constrained widths: the
truncated safe path follows Issue #24's slot rules, the
`(unreadable)` placeholder is shown, nothing overflows, and layout
dimensions remain nonnegative. `truncateRightCells` truncates the
status note when it would not fit beside even a one-cell path.

### Fixed-status guarantee

Load failures never change the fixed search-derived exit status:

- Every retained file failing to load with fixed status 0 still
  exits 0.
- A current-file failure with fixed status 2 still exits 2.
- The composed row of usable search results with fixed status 2
  where every retained file subsequently fails to load: the
  ordinary status remains 2, the load failures affect only file
  presentation and diagnostics, and the already-fixed fatal-search
  outcome is not recomputed.

## Model fields

- `readFailed bool` — the current file's last load attempt failed.
- `failedPaths map[string]string` — failed raw paths to sanitized
  diagnostics. A successful load removes the path.
- `overlayReadFailure bool` — the open overlay is a read-failure
  overlay.

## Public accessors

- `OverlayFatal() bool` — reports whether the open overlay is fatal
  (dismissal exits 2). A read-failure overlay is non-fatal.
- `OverlayText() string` — the open overlay's text content.
- `OverlayScroll() int` — the open overlay's vertical scroll offset.
- `IsLoading() bool` — the current file's content is loading.
- `ReadFailed() bool` — the current file's last load attempt failed.

## Out of scope

- The `r` retry route is owned by Issue #27.
- Load-completion reveal is owned by Issue #28.
- Stale validation is owned by Issue #29.
- Unsupported encodings are owned by Issue #30.
- The generalized overlay append primitive (all appended errors,
  help/precedence semantics) is owned by Issue #32. Issue #26
  implements only the minimal append-preserving-scroll primitive
  needed for the re-entry retry failure case.

## Testing

See [unit-tests](unit-tests.md) for the `read_failure_test.go` and
`reentry_test.go` catalogs covering the current-file overlay and
placeholder, non-current diagnostic-only collection, same-file
versus cross-file retry, composed-view robustness, the outcome-matrix
rows, and the gated re-entry sequence.
