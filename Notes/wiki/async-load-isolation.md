# Asynchronous load isolation — keyed completions, one load per path (Issue #25)

Delivered by
[Issue #25](../issues/025-async-load-isolation.md):
navigation stays fully active while a file loads, load completions are
keyed by raw path **and** request identity so a late answer updates
only its own path's cache/status and never another file's panel, at
most one load is in flight per raw path, successful buffers are
retained for the session, and the decode/map phase is separately
gatable with input staying responsive while it is held. Relevant PRD
sections: *File loading, cache, reload, and selection consistency*
(first three bullets) and *Testing Decisions → Asynchronous App
behavior* in [`Notes/PRD-vrg.md`](../PRD-vrg.md). Builds on the
[Issue #5 browse view](browse-tracer.md), the
[Issue #13 matched-line cursor](match-navigation.md), the
[Issue #4 `quitting` discard](cancellation-cleanup.md), and the
[Issue #16 wrap mode](wrap-mode.md) `w` toggle. Load-completion reveal
semantics are
[Issue #28's two-stage contract](load-completion-reveal.md) and
failure/retry handling is
[Issue #26's](read-failures.md).

## Navigation during loads

Navigation never waits on a load. `n`/`p` step the matched-line cursor
and return at once; a file crossing mints the destination's own load
request, opens the [file-change pop-up](file-change-popup.md), and
names the destination in the filename rule over a "Loading…"
placeholder — the user moves straight past a slow file. While any
placeholder is up, scroll keys remain strict no-ops and every other key
keeps its normal meaning: `ctrl+c` cancels, `w` toggles wrap, `c`
toggles the scheme, resizes apply, and `q` still exits.

## Path-and-request-identity keying

`model.loading` is now `map[string]int` — the in-flight **request
identity** per raw path — fed by a `loadSeq` counter the load
minter increments each time it issues a load. `fileLoadedMsg` carries
`{path, req, reload, buf, err}`: the command echoes back the identity it was
minted with, and `Update` applies a completion only when its
`(path, req)` pair matches a live request. A completion naming a path
with no request in flight, or carrying a different identity, is
discarded without touching the cache, the failure record, the
diagnostics, or the panel — unrequested, stale, forged, and duplicate
completions are all the same non-event. This is the same
instance-keying discipline the
[file-change pop-up](file-change-popup.md) uses for expiry and the
[prepared-layout pipeline](logical-anchor.md) uses for `layoutReadyMsg`.

## Panel isolation

A valid completion deletes only its own path's in-flight record and
updates only that path's slot: `bufs[key]` on success (plus a `revs`
bump), `failed[key]` on failure. The visible panel changes only when
the completed path is current **at arrival time** — the A→B→C case:
A's completion landing while C is current caches A and leaves C's
panel, cursor, and viewport untouched; even the layout A's completion
prepares installs invisibly for later. When the path *is* current the
completion records the reveal intent owed to the latest cursor target
and requests the keyed layout — the intent commits at install under
[Issue #28's two-stage contract](load-completion-reveal.md).

## One load in flight per path

`startLoad` returns nil — the request is **dropped, not queued** —
when the destination is already loading or already cached.
Re-entering a path whose load is pending therefore mints no
second request: the crossing's batch carries only the pop-up leaf, the
live request keeps its identity, and settlement leaves nothing queued
behind. There is no load cancellation; a settled placeholder is the
user's only signal a load finished. Issue #26 has since made a failed
path retryable: minting the retry clears the failure record — see
[read-failures.md](read-failures.md). Issue #27 shares the rule with
`r`: `startReload` mints through the same `mintLoad` worker, so a
duplicate `r` — or a re-entry crossing — while the path's reload is
in flight is dropped the same way; see
[explicit-reload.md](explicit-reload.md). Issue #42 made `mintLoad`
itself the **single admission point**: the in-flight check sits inside
it, ahead of every mutation, so a dropped request touches nothing —
not even the in-flight load's classification — while the re-entry's
selection, placeholder, and reveal-intent updates are deliberately
ungated by it; see
[reload-admission.md](reload-admission.md).

## Session-long retention

A successful buffer enters `bufs` and stays for the session — no
eviction, no memory bound (Issue #27's failed reload is the one
dropper: it evicts stale content so it is never presented as
refreshed). Revisiting a cached path serves the stored buffer and
issues no load; the PRD explicitly accepts that cached content
ignores disk edits until an `r` reload — delivered in
[explicit-reload.md](explicit-reload.md).

## Post-cancellation rejection

Issue #4's `quitting` flag already discards every message once a
controlled exit begins, so a load completion — success or failure —
released after `ctrl+c` mutates nothing: no cache write, no failure
record, no collected diagnostic. The gated tests prove the released
worker lands on a cancelled UI as a non-event.

## The separately gated decode/map phase

`internal/filebuffer` now splits `Load` into `Read` (the disk read by
raw path bytes) and `Decode` (line splitting plus the
safe-presentation byte→cell mapping); `Load` remains the composition
for callers that want both. `startLoad`'s command runs the optional
`loadGate` around the whole load as before, then `Read`, then the new
`decodeGate` (`WithDecodeGate`) before `Decode` — so tests and
walkthroughs can hold decode/map with the read already done and prove
the UI stays responsive: `ctrl+c` → 130, `n`/`p` navigate, `w`/`c`
toggle, resize applies, none waiting on the worker. The completion
still carries the fully prepared `*filebuffer.Buffer`; `Update` never
reads, decodes, or maps a full file itself.

## Test seams

- `WithLoadGate` (existing, `VRG_TEST_LOAD_GATE`) — holds the whole
  load command; `heldLoads`/`heldFirstLoad` gate helpers count entered
  workers and hold all or only the first.
- `WithDecodeGate` (new) — runs inside the load command after `Read`,
  before `Decode`; no env seam yet since the manual case needs only
  the whole-load gate and a genuinely large file.
- Test-side `reqOf`/`mintRequest` read and mint request identities so
  fabricated completions stand in for real ones — including the reload
  requests Issue #27 mints through `mintLoad` — while deliberately
  wrong or absent identities exercise the stale/forged drop.

## Tests

See [unit-tests.md](unit-tests.md):
`internal/app/loadiso_test.go` covers held-load navigation, A→B→C
late-completion isolation, dropped re-entry, cached revisit,
unrequested/stale/settled completion rejection, post-cancellation
discard, and gated decode/map responsiveness. `injectLoad` in
`layout_test.go` now fills fabricated completions with the live
request identity, and the replay/sink-safety/reveal/scroll/popup
fabrications mint requests the way a real visit or reload would.
