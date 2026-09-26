# Explicit reload — `r` rereads the current file

Issue #27
(`Notes/issues/027-explicit-reload-r.md`, tasks
`Notes/tasks/027-explicit-reload-r.md`) delivered the explicit reload:
`r` rereads the current file from disk without rerunning rg or touching
the cursor stops, drops duplicate presses while the load is in flight,
preserves the cursor and logical anchor clamped to the new content
through the keyed prepared-layout install, and replaces the old display
with `(unreadable)` plus the failure overlay when the reread fails.

PRD cross-references: "File loading, cache, reload, and selection
consistency" (the one-load-per-path bullet, the reload bullets, and the
intentional cache-stability bullet) in `Notes/PRD-vrg.md`.

## The reload request

`r` is a browse-phase key routed to `Model.reload()`: it mints a
request identity through the shared `loadSeq`/`loading[path]`
bookkeeping and returns the same `loadCmd` worker command every load
uses — the read (`readFile` seam, `filebuffer.ReadFile` in production)
and the decode/map (`filebuffer.Prepare`) run off the update path, so a
reload is an ordinary reread of the raw path. rg is never rerun, the
stop list and the matched-line cursor are untouched, and the file stays
identifiable throughout: the filename row still names the path.

The cached buffer is dropped the moment `r` lands (`delete(m.bufs,
path)`), so the panel switches to `Loading…` for the duration and
scrolling/panning fall back to the placeholder no-op — and a failed
reread leaves no old buffer behind to be presented as refreshed.

## Dropped, not queued

A second `r` — or a cross-file re-entry — while a load for the path is
in flight returns no command and mints nothing: the Issue #25
one-load-per-path rule covers reloads too (`loading[path]` is nonzero).
There is no cancellation and no queue; the placeholder's change from
`Loading…` to content or `(unreadable)` is the only completion signal,
and only then does the next `r` start a new load. In a one-stop index —
where `n`/`p` are strict no-ops — `r` is the only retry route.

## Content revisions and the anchor intent

A successful reload bumps the path's content revision (`revs[path]++`),
which stale-keys the installed row model and any in-flight layout
request for the old revision: a prepared layout keyed to the
pre-reload revision that arrives after the reload completes fails the
`layoutDoneMsg` install guard and is discarded — it cannot replace the
reloaded content or move the anchor. This is Issue #17's
superseded-revision case, tested now that reload exists. On a
current-path completion the viewport also drops its rows
(`vp.SetRows(nil)`): nothing installed can match the new revision, so
the superseded layout never paints as refreshed while the new one is
prepared.

The anchor itself survives because it is logical state, not row state:
`SetRows(nil)` empties the viewport but retains the `anchor Target`,
and the new revision's matching layout resolves it through `resolve()`
— clamped to the new content, including the deliberately lossy EOF
clamp. What must be suppressed is the *reveal*: a load completion for
the current file normally reveals the newest selected stop. The reload
records a different pending intent instead — `intentAnchor`, "keep the
anchor, no reveal" — through the new `pending pendingIntent` field that
replaces the boolean `pendingReveal`:

- `reveal()` sets `intentReveal` when it cannot run and `intentNone`
  when it commits, so navigation taken during the in-flight reload
  still wins — the latest selection's reveal takes precedence, as the
  PRD requires.
- `reloading[path]` marks the request at mint time so its
  `loadDoneMsg` can tell a reread from a first load; the completion
  records `intentAnchor` only when no reveal is already carried.
- `commitIntent` — renamed from `commitReveal` — discharges the intent
  when a matching layout installs for the current path: a reveal
  commits for the newest stop, while the anchor intent's work already
  happened inside `SetRows`' resolve, so it only clears.

The reread also recomputes the stale-match verdict: the completed
load's `Buffer.Stale()` result replaces the path's `stale` mark, so
the Issue #29 `file changed since search` note survives the reread's
placeholder and clears only when the newly loaded content validates
fully — `r` recomputes, it does not clear (see
[stale-match-validation.md](stale-match-validation.md)).

This generic pending-intent seam is owned by Issue #27; Issue #28
completed the reveal-versus-reload arbitration on top of it for all
load completions — any navigation during the load, including
away-and-back sequences that end on the starting cursor, replaces the
anchor intent with the entry reveal, so navigation intent rather than
cursor equality decides the commit. See
[load-completion-reveal.md](load-completion-reveal.md).

## Failure replaces content

A failed reload goes through the Issue #26 current-file failure path
unchanged: `failed[path]` marked, one `cannot read <safe path>: <err>`
occurrence collected, the overlay opened — or appended to, preserving
the reader's scroll — and the panel reading `(unreadable)`; the old
revision's buffer was already dropped at `r` time. For a previously
failed file `r` *is* the retry route: `reload()` re-opens the retained
`failLines` overlay before minting the request, mirroring `entryLoad`'s
re-entry sequence, so a second consecutive failure lands on an open
overlay and appends exactly one new occurrence with the reader's
position preserved.

## Cache stability

Nothing observes the disk except a load a human asked for: rewriting
the file produces no message and no reread, and the frame keeps showing
the cached revision until `r`. Filesystem watching and automatic
refresh are explicitly out of scope.

## Tests

- `internal/app/reload_test.go` — the Issue #27 contracts, driven
  through the gated loader and held layout commands:
  `TestExplicitReloadRereadsCurrentFile` (one reread, `Loading…`,
  filename row intact, cursor and stops unchanged, new bytes install);
  `TestReloadDuplicateDroppedNotQueued` (duplicate `r` and mid-load
  re-entry mint nothing; the settled placeholder gates the next `r`);
  `TestReloadPreservesAnchorThroughMatchingLayout` (anchor and top
  asserted only after the new revision's layout installs — never
  against the superseded one);
  `TestReloadAnchorClampsToShrunkContent` (the lossy clamp rewrites an
  anchor past shrunken EOF); `TestFailedReloadReplacesContent`
  (`(unreadable)` plus overlay, no stale text);
  `TestReloadSecondFailureAppendsPreservingScroll` (one appended
  occurrence, reader position kept, one collection);
  `TestReloadIsTheOneStopRetryRoute` (`n`/`p` no-ops, `r` retries to
  success, the prior-failure overlay kept up);
  `TestDiskChangeWithoutRIsNotObserved` (disk edits produce no load and
  no frame change until `r`);
  `TestPreReloadLayoutDiscardedAfterReload` (the Issue #17
  revision-superseded case: a held pre-reload layout released after the
  reload completes is discarded without touching the panel or the
  anchor); `TestNavigationDuringReloadOverridesAnchor` (a selection
  made during the load wins over the anchor intent). Issue #28 adds
  the transition contracts:
  `TestReloadAnchorIntentSurvivesTheLayoutGap` (the anchor intent held
  through the layout gap commits with no reveal);
  `TestReloadSameFileAwayAndBackCommitsEntryReveal` and
  `TestReloadCrossFileAwayAndBackCommitsEntryReveal` (away-and-back
  ending on the initial cursor still reveals — navigation intent, not
  cursor equality, decides; the cross-file return shows `Loading…`
  with the saved anchor dormant);
  `TestReloadLateOldRevisionLayoutIsInert` and
  `TestReloadLateOldRevisionLayoutAfterNavigation` (old- and
  new-revision layouts completing out of order — the new revision's
  install commits the anchor or the reveal, and the late old-revision
  layout consumes nothing).

See [unit-tests.md](unit-tests.md) § `internal/app`.

## Files

- `internal/app/app.go` — the `r` key case (browse-phase only),
  `pending pendingIntent` replacing `pendingReveal`, `reloading` (the
  per-path in-flight reread mark), and the `loadDoneMsg` success
  branch's `SetRows(nil)` plus anchor-intent recording.
- `internal/app/browse.go` — `pendingIntent` (`intentNone`,
  `intentReveal`, `intentAnchor`), `reload()` (drop the buffer,
  re-open the prior-failure overlay, mint, mark), `reveal()` carrying
  `intentReveal`, and `commitIntent` committing whichever intent
  survives to install time.

See also: [read-failures.md](read-failures.md) (the overlay,
placeholder, and retry-append contracts the reload reuses),
[async-load-isolation.md](async-load-isolation.md) (the
one-load-per-path rule the duplicate `r` rides on),
[logical-anchor-and-layout.md](logical-anchor-and-layout.md) (the
logical anchor, keyed layout installs, and the pending-intent seam),
[destination-reveal.md](destination-reveal.md) (the reveal the
navigation intent commits),
[match-navigation.md](match-navigation.md) (the stops and cursor `r`
never touches), [stderr-replay.md](stderr-replay.md) (where a
reload failure's collected occurrence replays), and
[unsupported-encodings.md](unsupported-encodings.md) (the BOM-marked
file `r` rereads into the restored placeholder and a fresh overlay).
