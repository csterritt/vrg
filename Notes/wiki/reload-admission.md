# Atomic load admission — a dropped request mutates nothing (Issue #42)

Delivered by
[Issue #42](../issues/042-dropped-reload-no-intent-mutation.md)
([task](../tasks/042-dropped-reload-no-intent-mutation.md)):
`mintLoad` is the **single admission point** for every file-load
request — the in-flight check and the request's state mutation are one
decision point, so a duplicate request is **dropped whole** before it
can touch anything: no minted identity, no cleared failure record, no
revision bump, no reveal or anchor intent, no presentation change, and
— critically — no reclassification of the load already in flight.
Relevant PRD sections: *File loading, cache, reload, and selection
consistency* (the load-dedup and reload bullets) and *Navigation,
viewport, and logical anchors* (the reveal owed to the latest
selection) in [`Notes/PRD-vrg.md`](../PRD-vrg.md). Builds on the
[Issue #25 keyed-completion rule](async-load-isolation.md), the
[Issue #27 `r` route](explicit-reload.md), and the
[Issue #28 two-stage completion](load-completion-reveal.md).

## The admission point

`startLoad` (navigation/startup) and `startReload` (`r`) both end in
`mintLoad(f, reload)`, whose first act is the admission test: if
`m.loading[path]` is already set the request returns nil **before**
`m.failed` is cleared, `loadSeq` is incremented, or the live identity
is recorded. Only an accepted request performs those mutations and
returns the gated worker. Because `fileLoadedMsg.reload` is captured
at mint time inside the accepted request, a dropped request can never
rewrite the in-flight load's classification — a dropped `r` during a
startup or navigation load leaves that load a plain visit, so its
completion records the destination/first-match reveal intent rather
than reload-anchor preservation.

## Dropped versus accepted `r`

- **Dropped** (`r` while the path is already loading): the live
  request keeps its identity, `revs`, `pendingReveals`, and
  `pendingAnchor` are untouched, and the frame is unchanged — the
  in-flight "Loading…" is the load's own placeholder, not a new
  reload's.
- **Accepted** (`r` once the path has settled): the fresh identity
  mints, the panel drops to "Loading…" (`currentRows` hides the
  in-flight model), the completion arrives marked `reload`, and the
  successful filing bumps `revs[path]` **exactly once** — the bump
  belongs to completion, never to request minting — and records the
  anchor intent when no reveal is pending. Rapid repeated `r` presses
  therefore keep at most one load in flight per path, and the
  placeholder→content/unreadable transition remains the only
  completion signal: only a settled load lets the next `r` mint.

## Navigation re-entry is deliberately ungated

Admission gates **load requests**, not navigation. `navigate` updates
the matched-line cursor, the panel's presentation (the destination's
placeholder or cached content), and the reveal intent before its
`startLoad` leaf is even considered — so re-entering a path whose load
is still in flight drops only the duplicate request while the
selection and `pendingReveals` update as usual. The in-flight load's
completion then commits the re-entry's reveal against the installed
layout. See
[match-navigation.md](match-navigation.md) and
[destination-reveal.md](destination-reveal.md).

## Tests

See [unit-tests.md](unit-tests.md):
`internal/app/admission_test.go` covers the dropped `r` during a held
startup load and a held navigation load (identity, revision, intents,
and frame all preserved; completion reveals rather than preserving an
anchor), the accepted `r` (fresh identity, "Loading…", reload marking,
exactly one revision increment, anchor intent), rapid repeated `r`
presses keeping one load in flight with the placeholder transition as
the completion signal, and the ungated navigation re-entry updating
selection, placeholder, and reveal intent while its duplicate load is
dropped.
