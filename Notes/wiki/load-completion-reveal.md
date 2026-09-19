# Two-stage load-completion reveal — intent, commit, and reload arbitration (Issue #28)

Delivered by
[Issue #28](../issues/028-load-completion-reveal-latest-target.md)
([task](../tasks/028-load-completion-reveal-latest-target.md)):
a load completion and the reveal it owes are **two separate stages**.
Stage one files the result — buffer cached, content revision bumped,
final gutter and text width recomputed, keyed layout requested — and
performs **no row-based decision itself**. The reveal or
anchor-preservation intent is carried by the model and commits when,
and only when, a prepared layout matching the live `(path, revision,
text width, wrap mode)` installs. Relevant PRD sections: *File
loading, cache, reload, and selection consistency* (the second and
reload bullets) and *Navigation, viewport, and logical anchors* (the
visible-target and file-change reveal-sequence bullets) in
[`Notes/PRD-vrg.md`](../PRD-vrg.md). Builds on the
[Issue #17 prepared-layout pipeline](logical-anchor.md) — the keyed
install and the `pendingReveals` seam — and the
[Issue #27 reload-anchor intent](explicit-reload.md), generalized here
into the full reveal-versus-reload arbitration. Stale-match fallback
targets remain Issue #29's.

## Stage one — file the result, never decide

`fileLoadedMsg` handling is limited to validation-adjacent bookkeeping
(stale-match validation itself arrives with Issue #29): the buffer is
cached, `revs[path]` bumps, the layout request mints under
`layoutKey(path, buf)` — whose text width already reflects the new
buffer's gutter, Issue #24's file-list visibility, and the wrap mode —
and the intent is **recorded** rather than run. No visibility test, no
one-third placement, no clamping, no horizontal reveal executes at
load-completion time: the `consultedRows` fake proves the row model is
never consulted, and no `vps` entry is even created. The placeholder
stays up until the matching layout installs.

## The model-carried intent

Two intent kinds live on per-path maps, at most one per path:

- `pendingReveals[path]` — the **reveal intent** owed to the latest
  selection. Recorded at every non-reload current-file completion and
  at every `reveal` that finds no usable layout. It carries no target:
  the commit resolves the cursor's stop *at install time* through the
  installed rows, so the revealed target is always the **latest
  selected cursor's final display target** — never a target captured
  when the load was requested. That target is the cluster-expanded
  start cell of the first submatch
  ([Issue #21](grapheme-highlight-expansion.md)), or the marker cell
  for a zero-width or terminator-only match
  ([Issue #23](zero-width-markers.md)) — `Rows.StopTarget` resolves
  both through the line's byte→cell map.
- `pendingAnchor[path]` — the **reload-anchor intent** (preserve the
  anchor, no reveal; [Issue #27](explicit-reload.md)). Recorded at a
  `reload`-marked completion **only when no reveal intent is
  pending**: navigation during the load replaces the reload's intent,
  so the anchor intent is minted only for an undisturbed reload. The
  `reload` marking itself is fixed at request-mint time — since
  Issue #42 admission sits inside `mintLoad` before any mutation, a
  dropped `r` mints no request and can never stamp reload
  classification onto the load already in flight, so an in-flight
  visit's completion records the reveal intent it always owed; see
  [reload-admission.md](reload-admission.md).

## Installation-guarded commit

`layoutReadyMsg` installs a prepared row model only while its key
still equals the live demand — anything else is **obsolete**: wrong
width, mode, revision, or path. A discarded layout never consumes or
mutates a pending intent: it touches neither installed rows nor saved
viewports, and the intent simply waits for a matching install.

When the installing path is still current, the commit switch runs in
order: a pending reveal is consumed and `reveal` executes the whole
Issue #14/#19 sequence against the freshly installed rows — vertical
reveal first (visible-target no-scroll, else `floor(h/3)` placement
with BOF/EOF precedence), then the minimal horizontal reveal —
followed by a pending anchor intent, which the `Restore` above the
switch already committed by mapping the retained `(line, cell)` anchor
into the new rows. The `!had` fallback covers a first visit whose
intent was never recorded. The horizontal **reset** belongs to the
entry: `navigate` runs `ResetOff` on every file change before the
reveal attempt, so an entry reveal's offset is already zero at
recording time and stays frozen until the commit — same-file `n`/`p`
deliberately keeps the pan (PRD).

## Starting viewports

A first visit — the startup file included — starts from top-of-file
with horizontal offset zero: no `vps` entry exists, so the commit's
reveal runs against the zero viewport. A revisit starts from its saved
per-file viewport: the install's `Restore` first maps the retained
anchor into the new rows, then the reveal applies — a target visible
from the restored position stays; a hidden one moves to the third.
Startup adds its two cases: a first-stop target visible from top 0
keeps top 0 (no state recorded), a hidden one lands at `floor(h/3)`
clamped at EOF, and the first `n` then advances to the second stop.

## Reload transitions — navigation intent, never cursor equality

The reload contract after Issue #28:

- **Undisturbed reload** — no navigation between `r` and the
  completion — records the anchor intent: the matching install's
  `Restore` preserves the `(line, cell)` position and no reveal runs.
- **Any navigation during the load replaces the intent.** Same-file
  `n`/`p`, a file crossing, and away-and-back sequences — A→B→A or
  same-file stop A→stop B→stop A — all record the reveal intent
  through `reveal`'s pend path, so the reload's completion mints no
  anchor intent at all. The committed reveal proves *navigation
  intent*, not cursor equality, decides: an away-and-back whose final
  cursor equals the initial stop still reveals the target rather than
  preserving the scrolled-off position.
- **A return to the reloading file shows "Loading…"** — `currentRows`
  hides a model whose path has a load in flight — and the entry
  reveal applies on commit.
- **Revision-ordered installation**: old- and new-revision layouts
  completing out of order leave the intent committed only against the
  new revision's model; the late old-revision layout is discarded by
  the key check without consuming it.

The file-change pop-up is unaffected by either stage, and a completion
for a non-current file updates only its own cache — no intent is even
recorded for it.

## Test seams

No new production seams: the tests compose `heldLoads`/`heldNthLoad`
(load gates) with `heldLayouts`/`layoutHold` (layout gates) so the two
stages hold separately. New helpers and fakes in
`loadreveal_test.go`:

- `deliverStageOne` — runs the load command and feeds its completion
  through `Update`, returning the layout command stage one issued:
  the seam separating "load completed" from "layout installed".
- `consultedRows` — a `rowSource` recording every call, standing in
  as installed the way `countingRows` does: the probe proving stage
  one performs no row-based decision even when a row model is
  available.

## Tests

See [unit-tests.md](unit-tests.md):
`internal/app/loadreveal_test.go` covers the stage-one contract and
the install-gated commit — separately gated stages, hidden and visible
startup targets, resize and list-hide between stages, navigation while
pending, gutter growth, stale-revision discards preserving intents,
saved-viewport revisits, marker and mid-cluster targets, non-current
isolation, and pop-up independence.
`internal/app/reloadintent_test.go` covers the reload transitions —
undisturbed anchor preservation, navigation-during-load intent
replacement, same-file and cross-file away-and-back entry reveals, and
out-of-order revision commits.
