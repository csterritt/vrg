# Read failures — "(unreadable)", notification, and retry rules (Issue #26)

Delivered by
[Issue #26](../issues/026-read-failures-unreadable-retry-rules.md),
with the single-line diagnostic guarantee pinned by
[Issue #47](../issues/047-read-failure-single-line-filenames.md):
a failed file load shows "(unreadable)" in the panel, the
current-versus-non-current notification split decides whether the
[error overlay](error-overlay-and-outcomes.md) opens, and retries
follow a deterministic five-step re-entry sequence — all without ever
changing the fixed search-derived exit status. Relevant PRD section:
*File loading, cache, reload, and selection consistency* (the
failure/retry bullets and the mid-session diagnostic note) in
[`Notes/PRD-vrg.md`](../PRD-vrg.md). Builds on the
[Issue #25 async load isolation](async-load-isolation.md) — the
keyed-completion and one-load-per-path machinery the retry reuses —
the [Issue #9 error overlay](error-overlay-and-outcomes.md) and its
fixed outcome status, the
[Issue #11 stderr replay](stderr-replay.md) collection, the
[Issue #24 filename-rule slot](file-list-layout.md), and the
[Issue #13 matched-line cursor](match-navigation.md) whose file
crossings drive the sequence. The `r` retry route is Issue #27's —
delivered in [explicit-reload.md](explicit-reload.md); Issue #32
generalizes the appended-diagnostic contract beyond this case in
[overlay-precedence.md](overlay-precedence.md).

## Notification: current versus non-current

Every load failure does two things unconditionally: it records
`failed[path]` with the path's latest `loadDiag` text in `failDiag`,
and it collects that single-line diagnostic into the session
collection — `cannot read <EscapePath(path)>: <reason>` — regardless
of what the display does. Issue #47 pins the construction's
single-line guarantee: `loadDiag` builds the diagnostic from the
`EscapePath`-escaped raw path plus a sanitized reason — a
`*fs.PathError` contributes only its `Err` cause, never
`err.Error()`'s raw-path repetition — so a filename carrying a
newline, tab, invalid UTF-8, or ESC bytes still yields exactly one
diagnostic line in both the overlay row set and the stderr replay,
and the identical construction serves every load site (initial load,
`r` reload, and the re-entry retry, which all funnel through the one
`fileLoadedMsg` settlement). The display side splits on whether the
failed path is current **at completion-arrival time**:

- **Current file**: `openOverlay(diag, false)` shows the error overlay
  — or, while an overlay is already up, appends the single new
  occurrence. The panel reads "(unreadable)", the filename rule keeps
  identifying the (possibly truncated) path, and the file's cursor
  stops stay navigable: a same-file `n`/`p` still steps through them.
- **Non-current file**: diagnostic-only — no overlay, no in-UI
  indicator, no frame change of any kind. The user discovers it by
  visiting that file (which then runs the re-entry sequence below) or
  through the stderr replay at exit. Diagnostics collected after the
  initial post-search overlay is dismissed have no on-demand review
  route in version 1; a long session can accumulate unseen
  diagnostics, an accepted consequence of the non-interruption
  policy.

## Retry rules

Retry happens on entry **from a different file** — an `n`/`p`
transition whose `Move.FileChanged` lands on a failed path — never on
a same-file step between the failed file's own stops, which mints no
load and opens no overlay. `startLoad` now treats `failed` as
retryable: minting the retry clears the failure record so the panel
reads "Loading…" until settlement re-marks it, while an in-flight or
cached path is still dropped not queued per the
[one-load-per-path rule](async-load-isolation.md).

## The five-step re-entry sequence

Deterministic order whenever a previously failed file is entered from
a different file:

1. The prior failure shows immediately — the overlay opens (or
   re-opens) with the recorded `failDiag` diagnostic — and the panel
   switches from "(unreadable)" to "Loading…".
2. Exactly one retry load is minted while the overlay is up, not after
   dismissal. A load somehow already in flight drops the request and
   the existing load's settlement drives step 3.
3. Settlement updates the placeholder to content or "(unreadable)"
   without waiting for overlay dismissal; the overlay stays open and
   dismissible (`q`/`Esc`) throughout, and a successful retry leaves
   the prior-failure overlay displayed until the user dismisses it.
4. A second failure appends exactly one new occurrence to the open
   overlay — the append-preserving-scroll primitive this issue owns:
   `openOverlay` on an open overlay joins the new diagnostic under the
   old without touching `overlay.scroll`, so the reader's position
   holds — and collects exactly one new occurrence for the
   [stderr replay](stderr-replay.md). A success collects nothing new.
   Issue #32 verifies the primitive for all appended errors — see
   [overlay-precedence.md](overlay-precedence.md).
5. Navigating away while the retry is in flight lets it settle per
   Issue #25 — updating only that path's cache/failure state — and a
   later re-entry runs the same sequence again against the new prior
   state.

## Composed-view robustness

The unreadable state composes safely at ordinary and constrained
widths: the filename row keeps the left-truncated safe path in its
Issue #24 slot, the panel shows the placeholder, no row overflows the
frame, and every layout dimension stays nonnegative — the same
guards the ordinary browse frame relies on, since a placeholder is
just a row-less panel.

## Load failures never change the fixed status

The search-derived exit status fixed at `searchDoneMsg` is untouched
by load failures — three new
[outcome-matrix](error-overlay-and-outcomes.md) rows prove it: every
retained file failing under fixed status 0 still exits 0; a
current-file failure under fixed status 2 still exits 2 (the failure
diagnostic appends to the already-open fatal overlay); and the
composed row — usable results with fixed status 2 where every retained
file subsequently fails — keeps status 2, with the failures affecting
only file presentation and diagnostics and the already-fixed
fatal-search outcome never recomputed. Only `ctrl+c` still overrides
to 130.

## Test seams

- `WithLoader` (new, `options.loader`) — substitutes `filebuffer.Read`
  inside each load command, making read failures deterministic in
  model tests instead of depending on filesystem permission bits
  (which do not deny root anyway). `failAllLoader` fails every read;
  `failPathsLoader` fails listed raw paths; `gatedFailLoader` arms a
  mutex-guarded fail set so a gated worker's read observes an outcome
  flipped while it waits.
- `heldNthLoad` — a load-gate helper holding only the nth minted
  load, so the re-entry retry (a deterministic ordinal in the
  a→b→c→retry fixture) pends while later assertions run.
- `heldCallSet` (Issue #47, `readdiag_test.go`) — the generalization
  holding each listed load-call ordinal on its own entered/release
  channel pair, so a test can hold both a file's first load and its
  re-entry retry, removing the fixture under the gate to make the
  real `os.ReadFile` fail deterministically.

## Tests

See [unit-tests.md](unit-tests.md):
`internal/app/readfail_test.go` covers the current-file overlay and
placeholder with retained stops, the non-current diagnostic-only
policy and its replay presence, the same-file no-retry and cross-file
one-retry rules, composed-view robustness at ordinary and constrained
widths, and the gated re-entry sequence — immediate prior-failure
overlay with "Loading…", the dropped duplicate while a retry is in
flight, `Esc` dismissal that leaves the load undisturbed, the
successful retry keeping the prior overlay until dismissed, the
second failure's exactly-one append preserving scroll, and the
navigate-away settle plus later re-entry against the new prior state.
`outcome_test.go`'s `outcomeCase` gains the `failLoads` field driving
the three new matrix rows. `readdiag_test.go` (Issue #47) pins the
single-line diagnostic contract with genuine read failures — gated
loads whose fixtures are removed before the real `os.ReadFile` runs —
for filenames carrying newline, tab, invalid UTF-8, and ESC bytes at
the initial-load, `r`-reload, and re-entry-retry sites, asserting the
escaped-path-plus-sanitized-reason line through the collection, the
overlay row set, and the replay.
