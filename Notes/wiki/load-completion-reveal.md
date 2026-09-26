# Load completion — two stages and the latest-target commit

Issue #28
(`Notes/issues/028-load-completion-reveal-latest-target.md`, tasks
`Notes/tasks/028-load-completion-reveal-latest-target.md`) integrated
the two-stage load-completion contract: a current-file load completion
prepares nothing visible itself — it requests a prepared layout keyed
to the post-load parameters — and the reveal or reload-anchor intent
commits only when a layout matching the current (path, content
revision, text width, wrap mode) installs. Issues #17 and #27 supplied
the pieces (the keyed install guard and the generic `pendingIntent`
seam); this issue pins the integrated arbitration for *all* load
completions.

PRD cross-references: "File loading, cache, reload, and selection
consistency" (the load-completion and reload bullets) and "Navigation,
viewport, and logical anchors" (the file-change reveal sequence —
saved-or-top starting viewport, visible-target no-scroll, one-third
placement, horizontal reset then reveal) in `Notes/PRD-vrg.md`.

## Stage 1 — the load completion makes no row decision

A `loadDoneMsg` accepted for the current path (request identity
matching, per Issue #25) does exactly this and nothing more:

1. Installs the decoded/mapped buffer, bumps the path's content
   revision, clears any failed state.
2. Stale-keys the installed row model and any in-flight layout request
   for the old revision; drops the viewport's rows (`SetRows(nil)`)
   and resets the horizontal offset — nothing superseded can paint as
   refreshed.
3. Recomputes the geometry through `syncLayout`: the final gutter
   width — which may grow with the new line count — and the file
   list's current visibility both participate in the new text width.
4. Requests the prepared layout for `(path, new revision, new text
   width, wrap mode)`.

No visibility test, no one-third placement, no clamping, no horizontal
reveal runs at this point — every row-based decision waits for the row
model. A completion for a non-current path stops after step 2's cache
update: the visible panel is byte-identical.

## Stage 2 — the intent commits on a matching install

The intent lives in the model as `pending pendingIntent`, never in an
in-flight layout — so obsolete `layoutDoneMsg`s (wrong width, wrap
mode, revision, or path) are discarded by the install guard *without
consuming or mutating* it. Only an install whose key equals the
current parameters runs `commitIntent`:

- `intentReveal` → `reveal()` runs against the installed rows: the
  saved-anchor-or-top starting point is already resolved by
  `SetRows`, then the visible-target no-scroll / one-third placement
  / BOF-EOF clamp applies, followed by the minimal horizontal reveal
  in run-off-edge mode.
- `intentAnchor` → the anchor resolve already happened inside
  `SetRows`; the commit only clears the intent. No reveal runs.

## Latest selection, never a captured target

The reveal target is computed at *commit* time from `currentStop()`
and the newest buffer — `Buffer.RevealTarget` answering the
cluster-expanded first-submatch start cell (Issue #21), the marker
cell for a zero-width / terminator-only match (Issue #23), or Issue
#29's stale-entry fallback (first survivor, clamped recorded start, or
last source line — see
[stale-match-validation.md](stale-match-validation.md)). Navigation
while the load or the layout is in flight therefore redirects the
reveal: `n`/`p` move the cursor immediately, `reveal()` re-pends
`intentReveal`, and the commit lands on whichever stop is newest —
with the reloaded content's own stale verdict behind its target. A
target captured when the load was requested is never stored.

## Reveal-versus-reload arbitration

The completion records `intentAnchor` only for an `r` reread
(`reloading[path]`) when no reveal is already carried
(`m.pending == intentNone`). Navigation during the load — same-file or
cross-file, including away-and-back sequences that return the cursor
to the stop it started on — leaves `intentReveal` outstanding, and the
completion keeps it: **navigation intent, not cursor equality, decides
the commit.** An undisturbed reload preserves the prior logical
anchor; a navigated one commits the entry reveal. The reread mark can
only ever sit on an admitted request — Issue #42's atomic admission
checks `loading[path]` before any reload state moves, so a dropped `r`
cannot lend an in-flight first or navigation load the reload
classification (see [explicit-reload.md](explicit-reload.md)).

## Starting viewports

The sequence the commit applies over is the PRD's file-change rule:

- **First visit** (including the startup file): top of file, offset
  zero — a target visible from top 0 keeps top 0; a hidden one lands
  at `floor(content height / 3)`, clamped at EOF.
- **Revisit**: the saved per-file anchor becomes the reading position
  while the placeholder or previous layout shows; a target inside the
  restored window leaves the top alone, a hidden one moves it.

The file-change pop-up is independent of both stages — neither the
load completion nor the layout install dismisses or restarts it.

## Tests

- `internal/app/completion_test.go` (Issue #28) drives each stage
  separately — the load command's message through `Update`, the held
  layout command invoked on the test's schedule:
  `TestLoadCompletionMakesNoRowDecision` (stage 1's buffer/revision/
  gutter/width work with an unmoved viewport and a request keyed to
  the grown gutter's width; the commit on install);
  `TestNavigationDuringLayoutGapCommitsNewestTarget` (`n`/`p` during
  the gap move the cursor, not the top; the newest target commits);
  `TestResizeBetweenStagesCommitsAtNewWidth` and
  `TestListToggleBetweenStagesCommitsAtFinalWidth` (superseded
  geometry's layout discarded, the commit against the final width —
  distinguishable through a wrap-changing first line);
  `TestObsoleteLayoutsNeverConsumeTheIntent` (wrong width/mode/
  revision/path completions are all inert);
  `TestStartupHiddenTargetCommitsOnInstall` /
  `TestStartupVisibleTargetKeepsTopZero` (one-third versus no-scroll,
  then the first `n` advancing);
  `TestSavedViewportRevisitCommitsOnLoad` /
  `TestSavedViewportRevisitHiddenTargetMoves` (the saved-anchor start
  kept or overridden on commit);
  `TestMarkerTargetCommitPaintsMarkerCell` (a terminator-only `$`
  target reveals its row and paints the marker at the right edge in
  run-off-edge mode);
  `TestClusterTargetCommitPaintsWholeCluster` (a mid-cluster target
  reveals its expanded start cell, painting the whole `^A` cluster);
  `TestNonCurrentCompletionLeavesPanelUntouched` (a departed file's
  load and layout leave the current panel byte-identical);
  `TestPopupUnaffectedByCompletionStages`.
- `internal/app/reload_test.go` gained the Issue #28 transition
  cases: `TestReloadAnchorIntentSurvivesTheLayoutGap` (the anchor
  intent held through the gap, the commit preserving the anchor with
  no reveal), `TestReloadSameFileAwayAndBackCommitsEntryReveal` and
  `TestReloadCrossFileAwayAndBackCommitsEntryReveal` (away-and-back
  ending on the initial cursor still reveals — intent over equality;
  the cross-file return shows `Loading…` with the saved anchor
  dormant), and `TestReloadLateOldRevisionLayoutIsInert` /
  `TestReloadLateOldRevisionLayoutAfterNavigation` (out-of-order
  revision layouts: the new revision's install commits the anchor or
  the reveal, the late old-revision layout rewinds nothing).

See [unit-tests.md](unit-tests.md) § `internal/app`.

## Files

- `internal/app/app.go` — the `loadDoneMsg` success branch (stage 1:
  buffer, revision, geometry, request — no row decision) and the
  `layoutDoneMsg` install guard feeding `commitIntent`.
- `internal/app/browse.go` — `pendingIntent` (`intentNone`,
  `intentReveal`, `intentAnchor`), `reveal()` reading the live cursor
  and newest buffer, `commitIntent` discharging whichever intent
  survives to install, `syncLayout`'s post-load geometry.
- `internal/viewport/viewport.go` — `SetRows`' anchor resolve (the
  starting point both intents commit over) and `Reveal`'s placement.

See also: [logical-anchor-and-layout.md](logical-anchor-and-layout.md)
(the keyed install guard and pending-intent seam this rides),
[explicit-reload.md](explicit-reload.md) (the anchor intent and the
navigation-supersedes rule),
[destination-reveal.md](destination-reveal.md) (the reveal contract
the commit runs),
[stale-match-validation.md](stale-match-validation.md) (the fallback
target the commit can land on),
[async-load-isolation.md](async-load-isolation.md) (the
path-and-request keying feeding stage 1),
[minimal-horizontal-reveal.md](minimal-horizontal-reveal.md) (the
horizontal half of the reveal),
[zero-width-match-markers.md](zero-width-match-markers.md) and
[grapheme-highlight-expansion.md](grapheme-highlight-expansion.md)
(the marker and cluster-expanded target geometry),
[file-change-popup.md](file-change-popup.md) (the pop-up the stages
leave alone), and [file-list-layout.md](file-list-layout.md) (the
list/gutter width changes that participate in stage 1's recompute).
