# Two-stage load completion (Issue #28)

Issue #28 establishes a two-stage load-completion contract that
separates file-load completion from viewport reveal/reload-anchor
decisions. Stage one (load completion) validates the request, stores
the loaded data, updates the content revision, and starts
asynchronous layout preparation. Stage two (matching layout
installation) commits the reveal or reload-anchor intent against the
installed rows. The latest selection/navigation intent survives
asynchronous loading and layout preparation; stale layouts are
discarded without consuming or changing the latest intent.

Cross-references: [destination-reveal](destination-reveal.md),
[logical-anchor-and-layout-preparation](logical-anchor-and-layout-preparation.md),
[explicit-reload](explicit-reload.md),
[async-load-isolation](async-load-isolation.md). Relevant PRD section:
*File loading, cache, reload, and selection consistency*.

## Problem

Before Issue #28, the `FileLoadCompleteMsg` handler could make
row-based reveal decisions against the old or not-yet-matching row
model. The `needsReveal` flag was tied to the next file-load
completion, and `pendingReveal`/`pendingReloadAnchor` were separate
seams. A navigation during an in-flight load could be lost if the
load completion revealed the original target instead of the latest
selection. Stale layouts could consume a pending intent, leaving a
later matching layout with no intent to commit.

## Solution

Issue #28 consolidates the completion arbitration into a single
`loadIntent LoadIntent` field with three values:

- `IntentNone` — no pending intent. A layout installation preserves
  the existing anchor (same-file rebuild) or restores the saved
  per-file offset (fresh load).
- `IntentReveal` — the layout installation should apply the
  [destination-reveal](destination-reveal.md) against the installed
  rows. The target is the latest selected cursor, never a target
  captured when the load was requested.
- `IntentReloadAnchor` — the layout installation should preserve the
  anchor without revealing a match (PRD: "Reload by itself does not
  reveal a match"). See [explicit-reload](explicit-reload.md).

### Stage one: load completion

`FileLoadCompleteMsg` is stage one. It:

1. Validates request identity (rejects stale completions).
2. Updates the file cache and failure records.
3. Increments the content revision for reloads.
4. Installs the loaded buffer for the current path.
5. Starts layout preparation via `buildViewport()`.
6. Does **not** make row-based reveal decisions against an old or
   not-yet-matching row model.
7. If the layout installed synchronously (cache hit or factory test
   seam), commits the intent immediately via `commitLoadIntent()`.
   Otherwise the intent is carried to stage two.

### Stage two: matching layout installation

`LayoutReadyMsg` is stage two. It:

1. Installs the prepared `RowModel` only when `msg.Key` equals the
   current `LayoutKey()`. Stale completions are discarded without
   touching the visible panel, saved per-file state, or the intent.
2. Preserves the anchor for same-file rebuilds; restores the saved
   per-file anchor for fresh loads.
3. Commits the pending intent via `commitLoadIntent()`.

`commitLoadIntent()` applies the intent against the installed rows:

- `IntentReveal` calls `revealTarget()`.
- `IntentReloadAnchor` is a no-op (the anchor was already preserved
  by the installation path).
- `IntentNone` is a no-op.

The intent is cleared after commit so obsolete layouts and later
loads start from `IntentNone`.

### Intent transitions

The intent is set by the action that selects a load:

- **Startup** (`SearchCompleteMsg` entering `StateBrowse`): sets
  `IntentReveal`.
- **Cross-file navigation** (`handleNavigate`, file changed): sets
  `IntentReveal`. A cached destination with a matching layout commits
  immediately; a stale-cache miss or uncached destination carries the
  intent to stage two.
- **Same-file navigation** (`handleNavigate`, same file): if the
  viewport is installed, reveals immediately. If a layout is pending
  (`pendingLayout`), replaces the pending intent with `IntentReveal`
  so the commit targets the latest selection.
- **Explicit reload** (`handleReload`): sets `IntentReloadAnchor`.

A navigation that occurs while a reload is in flight replaces
`IntentReloadAnchor` with `IntentReveal`, because the latest
selection wins. The reload's layout installation then commits the
reveal, not the anchor preservation.

### Stale layout discard

A `LayoutReadyMsg` whose key does not match the current `LayoutKey()`
is discarded without consuming or mutating the intent. This covers:

- A reload whose new revision invalidates a cached layout.
- A resize or wrap-mode change that invalidates the text width or
  wrap mode.
- A file switch that invalidates the path.
- Any other state change that supersedes the original preparation.

## Why reveal decisions cannot use the old row model

The destination reveal computes the target row via
`targetRow(stop)`, which uses `RowModel.RowFromByte(lineIndex,
byteOffset)` to map the match start to the rendered row. An old or
not-yet-matching row model may have a different row count, different
wrapping, or a different text width. Revealing against it would
scroll to the wrong row or fail to find the target. Stage two commits
the reveal only after the matching `RowModel` is installed, so
`targetRow` sees the correct rows.

## Model fields

`internal/app/app.go`:

- `loadIntent LoadIntent` — the pending intent carried from stage one
  to stage two. Supersedes the Issue #14 `needsReveal` flag, the Issue
  #17 `pendingReveal` flag, and the Issue #27 `pendingReloadAnchor`
  flag.

## Public accessors

- `LoadIntent() LoadIntent` — returns the pending intent.
- `HasPendingReveal() bool` — reports `loadIntent == IntentReveal`.
- `HasPendingReloadAnchor() bool` — reports
  `loadIntent == IntentReloadAnchor`.

## Testing

See [unit-tests](unit-tests.md) for the `two_stage_test.go` and
`reload_intent_test.go` catalogs covering the two-stage contract and
the reload-intent transitions.
