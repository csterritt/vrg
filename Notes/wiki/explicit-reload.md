# Explicit reload — the `r` reread route (Issue #27)

Delivered by
[Issue #27](../issues/027-explicit-reload-r.md)
([task](../tasks/027-explicit-reload-r.md)):
`r` rereads the current file directly from disk — never rerunning `rg`
— while preserving the cursor and the logical viewport anchor,
clamped to the new content. Relevant PRD section: *File loading,
cache, reload, and selection consistency* (the reload bullets) in
[`Notes/PRD-vrg.md`](../PRD-vrg.md). Builds on the
[Issue #25 async load isolation](async-load-isolation.md) machinery —
the keyed completion, the one-load-per-path drop, and the injected
loader — the [Issue #26 read-failure](read-failures.md) overlay and
placeholder, and the
[Issue #17 prepared-layout pipeline](logical-anchor.md) whose content
revisions and pending-intent seam the reload feeds. Reveal-versus-
reload arbitration beyond the precedence already recorded is
[Issue #28](../issues/028-load-completion-reveal.md)'s; stale-match
notes are Issue #29's.

## The `r` route and the dropped duplicate

`r` is a browse-state key routed to `model.startReload`, placed in the
key switch **before** the modal overlay's precedence case: the route
fires while an open failure overlay is up — it is the retry route the
overlay must not block — and never dismisses it. `startReload` shares
`startLoad`'s minting (`mintLoad(f, reload)`) but skips the
already-cached check: the cached buffer is exactly what the reread
replaces. The in-flight check still applies — a second `r`, like a
re-entry crossing onto a path whose reload is pending, mints nothing:
**dropped, not queued**, per the
[one-load-per-path rule](async-load-isolation.md). The placeholder's
change is the only completion signal — "Loading…" → content or
"(unreadable)" — after which `r` starts a fresh load. `r` works with a
one-stop index: with `n`/`p` strict no-ops there, it is the only
retry route.

## What a reload never touches

The reread goes straight to the file: the search index pointer is
untouched (no `rg` rerun), the matched-line cursor and every file's
search-derived stops are unchanged, and the filename rule keeps
identifying the path throughout — the row derives from the index,
not the content. Cached content is intentionally stable between
reloads: a disk edit triggers no load, and scrolling, resizes, and
there-and-back navigation keep serving the retained buffer until `r`.

## Content revisions and superseded layouts

Every successful load — visit or reload — bumps `m.revs[path]`, so a
reload's content is a **new revision** feeding the existing
`layoutKey{Path, Revision, TextWidth, Wrap}` demand: the old
revision's installed layout goes stale the moment the reread lands,
and a layout prepared for the pre-reload revision that arrives after
completion is discarded by the Issue #17 key check without touching
installed rows, saved viewports, anchors, or pending intents.
`currentRows` additionally hides a model whose path has a load in
flight, so the panel reads "Loading…" for the whole reread and never
presents the old content mid-flight.

## Failure replacement

A failed reload replaces the old display: the error path drops
`bufs[path]` and `rows[path]` so stale content is never presented as
refreshed, marks `failed[path]`, and drives the Issue #26
notification split — a current-file failure shows "(unreadable)" and
the error overlay, a non-current one is diagnostic-only. `r` under
the still-open overlay retries; a second consecutive failure appends
exactly one new occurrence through `openOverlay`'s
append-preserving-scroll primitive with the reader's position
preserved, and a successful reload leaves the prior-failure overlay
displayed until the user dismisses it — the same settlement rule the
re-entry sequence uses.

## Anchor preservation through `pendingAnchor`

A reload owes no reveal. When a `reload`-marked `fileLoadedMsg`
completes for the current file, Update records
`pendingAnchor[path] = true` instead of calling `reveal` — the
**reload-anchor intent** (preserve the anchor, no reveal), minted at
load completion and committed only when the new revision's matching
prepared layout installs through the Issue #17 installation path. The
`Viewport.Restore` at install already maps the retained
`(line, cell)` anchor onto its row in the new rows — content growth
keeps the text location, and a shrink hits the intentionally lossy
EOF clamp that rewrites the anchor to the clamped top — so the commit
is simply consuming the intent. A pending **reveal** takes
precedence: navigation during the load (or a first-visit reveal still
owed) outranks the anchor intent, and `reveal` clears a recorded
`pendingAnchor` whenever it runs or pends — the seam Issue #28 later
generalizes into full reveal/reload arbitration.

## Test seams

No new seams: `r` drives the same `mintLoad` worker — `loadGate`
around the read, `decodeGate` before `Decode`, `options.loader` as
the injected read — and `layoutGate` around preparation. New
test-side helpers in `reload_test.go`:

- `layoutHold` — a layout gate armed mid-test: while armed every
  worker signals `entered` and blocks on `release`, so the initial
  install runs through and only workers minted after arming are
  held — the device that separates "recorded at load completion" from
  "committed at install" for the anchor intent.
- `reloadCmd` / `rewriteFile` — press `r` for its command and replace
  a fixture's bytes on disk for the simulated edit.

## Tests

See [unit-tests.md](unit-tests.md):
`internal/app/reload_test.go` covers the single reread with
"Loading…", unchanged index/cursor/filename row, the dropped
duplicate and dropped re-entry, anchor preservation gated through the
matching layout's install, the shrink clamp, failure replacement and
the second-failure append, the under-overlay retry and the
success-keeps-overlay rule, the one-stop route, disk-change cache
stability, and the pre-reload revision's superseded-layout discard.
