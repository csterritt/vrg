# Read failures — "(unreadable)", notification, and retry rules

Issue #26
(`Notes/issues/026-read-failures-unreadable-retry-rules.md`, tasks
`Notes/tasks/026-read-failures-unreadable-retry-rules.md`) delivered the
read-failure contract: a failed load on the current file interrupts with
the error overlay and the `(unreadable)` placeholder while a non-current
failure stays diagnostic-only, navigation retries a failed file only on
a cross-file re-entry through a deterministic five-step sequence, and a
load failure can never move the fixed search-derived exit status.

PRD cross-references: "File loading, cache, reload, and selection
consistency" (the failure/retry bullets and the mid-session diagnostic
note), "Outcome and exit-status contract" (the fixed-status bullet),
and "Text, graphemes, and safe presentation" (the embedded-filename
rule — escape it first as a single-line filename — which Issue #47
extends to the failure's reason half) in `Notes/PRD-vrg.md`.

## Failure notification: current versus non-current

A `loadDoneMsg` carrying an error marks its path in `m.failed`, retains
the sanitized diagnostic's display lines in `m.failLines[path]`, and
collects one occurrence into the session diagnostics through
`CollectDiagnostic` (`cannot read <safe path>: <reason>` — the path
escaped through `present.Path` before `present.Diagnostic` runs over
the whole line). Since Issue #47
(`Notes/tasks/047-read-failure-single-line-filenames.md`) the reason is
sanitized too: `readReason` unwraps a `*os.PathError` to its bare `Err`
— the errno text carries no path — rather than reusing
`PathError.Error()`, which embeds the raw resolved path; an embedded
newline in the filename would otherwise survive `present.Diagnostic`
as a real line boundary and split one failure into two diagnostic
lines. The filename appears exactly once, in the `Path`-escaped
prefix, so one failed read always produces exactly one diagnostic line
in the overlay row set and the stderr replay alike — and the
construction is uniform: the initial load, the `r` reload, and the
re-entry retry all funnel through the same `loadDoneMsg` error branch.
Non-path errors keep their own text. What happens next depends
entirely on whether the failed path is current:

- **Current file**: the diagnostic opens through `openOverlay` — the
  Issue #9 modal — which appends one occurrence when an overlay is
  already open, preserving the reader's scroll (see
  [error-overlay-and-fatal-outcomes.md](error-overlay-and-fatal-outcomes.md)).
  The panel reads `(unreadable)`, the filename row keeps identifying the
  path and gains the `(unreadable)` status note (`bufferNote`, the real
  provider behind the `statusNote` seam — see
  [file-list-layout.md](file-list-layout.md)), and the file's cursor
  stops stay navigable.
- **Non-current file**: diagnostic-only — collected for the Issue #11
  exit replay with no overlay and no in-UI indicator; the visible frame
  is byte-identical. The user discovers it by visiting the file (which
  then runs the re-entry sequence below) or at exit. This mid-session
  invisibility is deliberate: diagnostics collected after the initial
  overlay is dismissed have no indicator and no on-demand review key in
  version 1. See [stderr-replay.md](stderr-replay.md).

Either way the placeholder is clipped to the text width so a constrained
terminal cannot overflow: the composed view stays well-formed at any
size — the filename row keeps the truncated safe path per Issue #24's
slot rules and the layout dimensions stay nonnegative.

## The retry rule: same-file steps never, cross-file entries once

`ensureLoad` still drops requests for failed paths, so an `n`/`p` step
between stops of a failed file requests nothing and re-opens nothing —
the overlay stays dismissed and `(unreadable)` stays up. A crossing into
a previously failed file is different: `navigate` routes the destination
through `entryLoad`, which runs the re-entry sequence. (`r` is the other
retry route — Issue #27's explicit reload re-opens the retained overlay
the same way; see
[explicit-reload.md](explicit-reload.md) — and the only one a one-stop
index has.)

## The re-entry sequence

Entering a previously failed file from a different file is deterministic:

1. The prior failure's overlay opens immediately — `entryLoad` calls
   `openOverlay` with the retained `failLines` — and the panel switches
   from `(unreadable)` to `Loading…`: `contentRow` shows `Loading…`
   whenever `loading[path]` holds a request, even over a `failed` mark.
   The file-change pop-up is skipped for a failed destination; the
   overlay supersedes it.
2. Exactly one retry load starts while the overlay is still open. If a
   load for the path is somehow already in flight the request is
   dropped, not queued — the Issue #25 one-load-per-path rule — and the
   existing load's settlement drives step 3.
3. When the retry settles, the placeholder updates to content or back to
   `(unreadable)` without waiting for the overlay's dismissal; the
   overlay stays open and dismissible (`q`/`Esc`) throughout. A
   successful retry clears `failed`/`failLines`, installs the buffer,
   and collects nothing new — the prior-failure overlay remains
   displayed until the user dismisses it.
4. A second failure appends exactly one new diagnostic occurrence to the
   open overlay — `openOverlay`'s append preserves the reader's scroll
   position — and collects exactly one new occurrence for the replay.
   This append-preserving-scroll behavior is the minimal primitive this
   issue owns; Issue #32 generalizes it to all appended errors.
5. Navigating away while the retry is in flight lets it settle per Issue
   #25 — it updates only that path's cache/status, so a failure landing
   non-current is diagnostic-only — and a later re-entry runs this same
   sequence again against the new prior state.

## Load failures never change the fixed status

The exit status fixed by `decideOutcome` at search completion is
untouched by anything a load does afterwards: the Issue #9 outcome
matrix gained rows for every retained file failing under fixed status 0
(still 0), a current-file failure under fixed status 2 (still 2), and
the composed case — usable results with fixed status 2 where every
retained file subsequently fails — where the ordinary status stays 2,
the failures affect only file presentation and diagnostics, and the
already-fixed fatal-search outcome is not recomputed. See
[error-overlay-and-fatal-outcomes.md](error-overlay-and-fatal-outcomes.md).

## The injected loader seam

Tests never rely on filesystem permissions (`chmod 000` does not deny
root). `Model.readFile` is the read-phase seam — nil selects
`filebuffer.ReadFile` — which `loadCmd` consults alongside `loadGate`
and `mapGate`; test models substitute a failing or scripted loader.
`failures_test.go`'s helpers are `loaderModel` (browse model wired with
an injected loader), `gatedLoaderModel` (the loader plus `loadGate`, so
a retry can be parked in flight), `failLoader`/`failLoaderFor`
(all-fail and per-path failing loaders), `contentRow1` (the stripped
first content row the placeholders occupy), and `overlayOccurrences`.

## Tests

- `internal/app/readdiag_test.go` — the Issue #47 single-line
  contract: `TestInitialReadFailureIsOneLine`,
  `TestReloadReadFailureIsOneLine`, and
  `TestReEntryRetryReadFailureIsOneLine` drive the four embedded-byte
  filename cases (newline, tab, invalid UTF-8, ESC) through real
  `os.ReadFile` failures at every load site — the fixture is indexed,
  the worker parked on `loadGate`, the file removed, the read released
  to a genuine `*os.PathError` — asserting exactly one diagnostic line
  carrying the `present.Path`-escaped path and the unwrapped reason in
  the overlay row set, `failLines`, the session collection, and the
  stderr replay; `TestReadFailureReasonUnwrapsPathError` pins the
  reason sanitation for wrapped path errors and plain errors alike.
- `internal/app/failures_test.go` — the Issue #26 contracts:
  `TestCurrentFileReadFailureNotifies` covers the current-file overlay,
  the `(unreadable)` placeholder, the filename row still naming the
  path, and the retained stops — a same-file `n` moves the cursor and
  requests no reload;
  `TestNonCurrentReadFailureIsDiagnosticOnly` lands a completion after
  the cursor left — no overlay, no indicator, one collected occurrence —
  then proves visiting the file surfaces its overlay;
  `TestCrossFileReEntryRetriesOnce` pins the re-entry's immediate
  prior-failure overlay, the `Loading…` panel, and exactly one issued
  retry; `TestUnreadableComposedViewStaysWellFormed` drives a long
  escaped path through 80×24 down to 20×3 — truncated safe path in the
  row, clipped placeholder, no overflow, nonnegative layout;
  `TestReEntrySequenceGated` covers the gated sequence — immediate
  overlay and `Loading…`, one parked retry, `Esc` dismissal without
  disturbing the load, and settlement updating the panel;
  `TestReEntrySecondFailureAppendsPreservingScroll` pins the append of
  exactly one occurrence to the open overlay with the reader's scroll
  preserved and one occurrence collected;
  `TestReEntryRetrySuccessKeepsPriorOverlay` proves a successful retry
  collects nothing and leaves the prior-failure overlay up until
  dismissed; `TestReEntryRetrySettlesAfterNavigatingAway` covers the
  away-settled retry as diagnostic-only and a later re-entry repeating
  the sequence; `TestReEntryDuringInflightRetryIsDropped` pins the
  dropped-not-queued re-entry against an in-flight retry.
- `internal/app/outcome_test.go` — the matrix gained the
  `failAll`/`absent`/`viewHas`/`replayHas` row fields and three Issue
  #26 rows: fixed-0 all-fail, fixed-2 current-file failure, and the
  composed all-fail-with-fixed-2 row asserting the unrecomputed status
  and the replay presence of the non-current failure.

See [unit-tests.md](unit-tests.md) § `internal/app`.

## Files

- `internal/app/app.go` — `failed`/`failLines` (the prior-failure
  retention), the `readFile` read-phase seam, and the `loadDoneMsg`
  error branch: mark, retain, collect, overlay-if-current; the success
  branch clears both failure maps.
- `internal/app/browse.go` — `entryLoad` (the re-entry sequence: overlay
  plus exactly one retry on a cross-file entry into a failed file),
  `navigate` routing destinations through it and skipping the pop-up for
  a failed destination, `readReason` (Issue #47's sanitized reason —
  the `*os.PathError` unwrap to `Err` the diagnostic composes with the
  `present.Path` prefix), `contentRow`'s `Loading…`-while-loading
  placeholder clipped to the text width, and `bufferNote` (the real
  `(unreadable)` status note behind the `statusNote` seam).
- `internal/app/overlay.go` — `openOverlay`'s open-or-append is the
  append-preserving-scroll primitive the second failure rides on.

See also: [async-load-isolation.md](async-load-isolation.md) (the
one-load-per-path rule and non-current settlement isolation the re-entry
reuses), [explicit-reload.md](explicit-reload.md) (the `r` retry route
that re-opens the retained overlay),
[match-navigation.md](match-navigation.md) (the cursor steps
that distinguish same-file from cross-file), [browse-tracer.md](browse-tracer.md)
(the placeholders and filename row), [stderr-replay.md](stderr-replay.md)
(the diagnostic-only collection and exit replay),
[error-overlay-and-fatal-outcomes.md](error-overlay-and-fatal-outcomes.md)
(the overlay and the fixed-status matrix rows),
[unsupported-encodings.md](unsupported-encodings.md) (Issue #30's
diagnostic following the same current/non-current split and re-entry
overlay), and
[file-list-layout.md](file-list-layout.md) (the status-note slot the
unreadable note occupies).
