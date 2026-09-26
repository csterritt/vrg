# Error overlay and fatal outcomes (Issue #9)

The outcome contract delivered by
[Issue #9](../issues/009-error-overlay-and-fatal-outcomes.md),
implemented in `internal/searchindex` (`index.go` lifecycle tracking)
and `internal/app` (`overlay.go`, the `searchDoneMsg` branch of
`app.go`). Relevant PRD sections in `Notes/PRD-vrg.md`: *Result index,
records, and stream integrity* (the lifecycle matrix), *Outcome and
exit-status contract* (the outcome table), and *Colours, overlays, and
key precedence* (the modal overlay rules).

## Stream lifecycle validation

`Index` now validates the record stream's lifecycle while it
accumulates stops, tracked per path over **decoded raw path bytes** —
a `text` record and a `bytes` record carrying the same path agree on
one identity, and interleaved open files keep independent state.
`Index.Feed(stream)` consumes the collected stdout: each
newline-terminated record is decoded and `Add`ed, a record failing the
per-record schema is skipped and counted (Issue #10), and a
non-empty trailing chunk without its terminator is an unterminated
record — skipped, counted malformed, and an integrity failure, since a
cut stream is incomplete.

The transition matrix and its dispositions:

| Transition | Disposition |
|---|---|
| `begin(P)` while P is not open | valid; P opens |
| `begin(P)` while P is open | integrity failure: `duplicate begin` |
| `match(P)` while P is open | valid; indexed under P |
| `match(P)` while P is not open (never opened, or after `end`) | integrity failure: `orphaned match`; the match is **retained** and P's stops are marked `Incomplete` |
| `end(P)` while P is open | valid; P closes; a non-null `binary_offset` excludes P |
| `end(P)` while P is not open | integrity failure: `orphaned end`; a non-null `binary_offset` still excludes — exclusion is a safety property, not a reward for valid metadata |
| `context(P)` before the summary | ignored; no lifecycle effect |
| P open when the stream ends | integrity failure: `missing end for P`; P's stops are marked `Incomplete` |
| exactly one `summary`, final | valid; a lone summary is a complete zero-result stream |
| missing `summary` | integrity failure: `missing summary` |
| second `summary` | integrity failure: `extra summary record` (its sole cause — never also `record after summary`) |
| any other record after `summary`, including malformed bytes and `context` | integrity failure: `record after summary`; the record is **not lifecycle-processed** — a `begin` cannot open, a `match` is neither retained nor marked incomplete |
| trailing unterminated record | malformed **and** an integrity failure: `unterminated final record` — or, positioned after the summary, only `record after summary` plus the malformed count |

**Binary exclusion takes precedence over orphan retention**: a match
arriving after a binary-excluding `end(P)` is not retained and P stays
excluded — its distinct cause line is `match for P arrived after a
binary-excluding end`. Issue #36 owns the removal of the old
`context`-after-`summary` exemption — the summary-is-final rule is
positional — and the one-cause-per-physical-record precedence:
post-summary malformed/oversized/unknown records keep their
independent tallies alongside the sole `record after summary` cause.

Since Issue #36 the violations are structured `Cause` values (stable
`CauseKind` plus the record's raw path bytes), returned by
`IntegrityCauses()` — `IntegrityFailures()` derives one rendered line
per cause. Ordering is deterministic: mid-stream causes in detection
order, then the end-of-stream causes `seal` appends — missing ends
sorted by unsigned raw path bytes, then missing summary, then the
unterminated final record.

End-of-stream sealing runs once inside `Prepare` (and inside
`IntegrityFailures`), so either accessor finalizes the lifecycle.
`IntegrityFailures()` returns the diagnostics in stream order, empty
when the stream was intact — integrity is assessed **separately from
process success**: rg can die between complete records, and a clean
exit can still carry a damaged stream.

`Stop.Incomplete` marks stops whose file's lifecycle metadata is
damaged (orphaned match, or end never arrived): their binary
classification is unconfirmed even though they remain browsable.

## The outcome decision

`decideOutcome` in `internal/app/overlay.go` is the pure function of
the completed search's independent inputs — the `outcomeInput` struct
since Issue #36: the child's wait error, the captured stderr, the
structured stream-integrity causes (`Index.IntegrityCauses`), the
usable-results count (retained stops), and the record-loss components
(the malformed and oversized tallies, the per-path oversized detail
lines from `Index.OversizedDiagnostics`, and the unknown-type tally).
It returns the underlying
screen, whether the overlay opens, the fixed exit status, and the
diagnostic lines. `Update`'s `searchDoneMsg` branch applies it exactly
once; the status never changes afterwards except through `ctrl+c`.

The outcome table:

| Process result | Stream | Usable results | Presentation | Exit |
|---|---|---|---|---|
| rg 0 | intact | > 0 | browse | 0 |
| rg 0 | intact | 0 | no-results | 1 |
| rg 1 (anomalous — results present) | intact | > 0 | browse | 0 |
| rg 1 | intact | 0 | no-results | 1 |
| fatal code or signal | any | > 0 | browse + error overlay; `q`/`Esc` reveals browse | 2 |
| fatal code or signal | any | 0 | fatal-only overlay, nothing beneath; `q` **or** `Esc` quits | 2 |
| any | integrity failure | > 0 | browse + error overlay | 2 |
| rg 0/1 | intact | > 0, stderr non-empty | browse + warning overlay | 0 |
| rg 0/1 | intact | 0, stderr non-empty | no-results + warning overlay; dismissal reveals no-results | 1 |
| rg 1 | intact, all files binary-excluded | 0 | no-results + warning overlay → `(N binary files skipped)` | 1 |
| rg 0/1 | intact, unknown-type warnings only | 0 | warning overlay → no-results | 1 |
| rg 0/1 | intact, malformed/oversized records skipped | > 0 | browse + warning overlay (skip tallies) | 0 |
| rg 0/1 | intact, malformed/oversized records skipped | 0 | record-loss fatal overlay; `q`/`Esc` quits | 2 |
| rg 0/1 | skipped record + binary exclusion emptied the index | 0 | record-loss fatal overlay | 2 |

Usable results is assessed **after all filtering** — a stream whose
sole retained file was binary-excluded after a skipped record lands on
the record-loss fatal row, not no-results. Record loss is fatal only
when nothing usable survived; unknown-type tallies are diagnostics only
and never fatal alone. See
[record-robustness.md](record-robustness.md).

`Esc` exits only in the fatal no-results case, where there is no
underlying state to reveal; it is a dismissal everywhere else and a
no-op in base states. `ctrl+c` overrides any fixed status with 130,
including with the overlay open. Issue #32 pinned the full precedence
and dismissal semantics — error suspending help with scroll
restoration, appended errors preserving the reader's position, and
the `Esc`/`q` dismissal-outcome table — in
[overlay-precedence.md](overlay-precedence.md).

## Diagnostics and stderr classification

Captured stderr is classified regardless of rg's exit code: non-empty
stderr under a clean exit or exit 1 is a *warning* diagnostic shown in
the overlay, and under a fatal outcome it is the process component —
since Issue #36 explanatory stderr **suppresses** the generated line,
which exists only when a fatal process result (signal death or exit
code other than 0/1) supplies **no** stderr: `rg failed: exit status 3`
or `rg failed: signal: killed`. Exit 0 and exit 1 never produce a
process-status line — a damaged stream under a clean exit names its
causes, never `ripgrep exited with code 0`.
Since Issue #36 diagnostics assemble in the universal order
`composeDiagnostics` produces — process component, integrity causes,
record-loss components (malformed aggregate, oversized aggregate —
always emitted for any positive count since Issue #37 — then the
per-path `oversized record skipped for <escaped path>` details,
deduplicated by raw path in first-occurrence order),
then the unrecognised-type warnings — and fatal branches drop none of
them: a fatal stream with real stderr shows stderr, causes, and
tallies together. Issue #37 also pinned the anonymous oversized
guarantee: a record whose limit cut before its path was parsed still
surfaces the aggregate — a fatal overlay is never empty when records
were lost.
Everything
passes through `present.Diagnostic` — the Issue #6 utility — before it
can reach the screen; embedded paths were already escaped through
`present.Path` inside `Cause.Line()`. See
[safe-presentation.md](safe-presentation.md).

The **session collection** takes the completion-only subset from
`completionDiagnostics` — the same composition minus the child stderr
the collection already took line-by-line through `diagMsg` while the
search ran, so collecting it again would double-count. One composition
therefore feeds both sinks: the overlay displays the full list and
`cmd/vrg` replays the collected text verbatim. Display and collection
are independent; see
[stderr-replay.md](stderr-replay.md) and
[stream-integrity-fatal-diagnostics.md](stream-integrity-fatal-diagnostics.md).

## The modal overlay

`overlay` (`internal/app/overlay.go`) holds the sanitized diagnostic
lines and the scroll offset. Since Issue #15 every opening goes
through `openOverlay`: it appends the new lines when an overlay is
already open (preserving the reader's scroll) and cancels any live
file-change pop-up — an arriving error overlay removes the pop-up and
it cannot return when the overlay is dismissed (see
[file-change-popup.md](file-change-popup.md)). Since Issue #31 the
type is the shared wrapped-scrollable component the help overlay also
instantiates: the geometry helpers are value methods `layout(w, h)`
and `maxScroll(w, h)`, `scrollOverlay` is the shared clamped up/down
handler, and `renderOverlay(base, o)` composites whichever instance is
passed (see [help-overlay.md](help-overlay.md)). While `m.overlay` is
non-nil it owns the
keyboard — `overlayKey` runs before the base-state switch (and before
help's `helpKey`, the error-over-help precedence):

- `up`/`down` scroll one row, clamped at both ends.
- `q` and `Esc` dismiss to the underlying screen — or quit with the
  fixed status 2 when the outcome was fatal with no usable results
  (`phaseFatal`: the overlay stands alone on a blank frame).
- `ctrl+c` takes the cancellation path to 130.
- Every other key is ignored — `c` cannot reach the colour toggle
  beneath an open overlay.

`layout(w, h)` resolves geometry from the frame: interior width is the
smaller of the frame minus the border and the widest line; each line
hard-wraps with `ansi.Wrap` so an unbroken diagnostic never overflows
the border; interior height is the smaller of frame minus border and
the wrapped count. `renderOverlay` composites
`theme.Overlay`'s single-line bordered box — base colours, see
[theme-and-colour-toggle.md](theme-and-colour-toggle.md) — centred over
the underlying frame's rows via `composite`, the cell-exact
`ansi.Truncate`/`ansi.Cut` splice Issue #15 refactored out so the
file-change pop-up shares it. The overlay is a new output sink and
carries its
`sinkSafetySinks` row (`error overlay`): the hostile fixture set drives
hostile bytes in as captured stderr and asserts the Diagnostic-escaped
form inside the border.

A `loadDoneMsg` failure for the **current** path also opens the
overlay — the `cannot read <safe path>: <err>` line through
`openOverlay`, alongside the `(unreadable)` panel placeholder — and
Issue #26 completed the read-failure rules around it: the retained
`failLines` reopen on cross-file re-entry, a second failure appends
one occurrence through the same scroll-preserving route, and a
non-current failure stays diagnostic-only for the session collection.
See [read-failures.md](read-failures.md).

## Fixed-status rule

The exit status is decided once at completion — 2 for any fatal
outcome (fatal process result, integrity failure), 0 for usable
results, 1 for an intact empty stream — and is thereafter overridden
only by `ctrl+c` → 130. The process boundary's terminate-and-reap
cleanup runs identically on every exit; see
[cancellation-and-cleanup.md](cancellation-and-cleanup.md).

## Tests

`internal/searchindex/lifecycle_test.go` pins the lifecycle matrix
table-driven — every transition row, binary-exclusion precedence,
`text`/`bytes` path-identity agreement, interleaved open files, the
trailing-unterminated disposition mid-stream and after a complete
stream, summary positioning, orphan retention with `Incomplete`, and
open-at-end sealing — with Issue #36 correcting the
post-`summary` rows (sole `record after summary` causes, the removed
`context` exemption) and `causes_test.go` adding the exact structured
`Cause` assertions.
`internal/app/outcome_test.go` is the single table-driven outcome
matrix covering every row above with dismissal and exit assertions —
Issue #10 added the record-loss and unknown-warning rows plus a
`stream`/`fixtureStream` path for fixtures containing undecodable
bytes, and Issue #26 added the read-failure rows: `failAll` marks
every retained file's `loadDoneMsg` an error and the row's
`viewHas`/`absent`/`replayHas` assertions prove the fixed status is
never recomputed — all-fail under fixed 0 still exits 0, a
current-file failure under fixed 2 still exits 2, and the composed
row (usable results, fixed 2, every retained file failing) stays 2
with only presentation and diagnostics affected.
`internal/app/overlay_test.go` pins scrolling, both dismissal keys,
`ctrl+c`, ignored keys (including the unreachable `c` toggle),
unbroken-line wrapping, and generated code-or-signal diagnostics.
`sinksafety_test.go` gained the `error overlay` row with per-fixture
`wantDiag` expectations. `diagnostics_test.go` (Issue #36) pins the
universal composition: exact `composeDiagnostics`/`completionDiagnostics`
slices, stderr precedence over the generated line, no process line for
exit 0/1, the post-summary dual representation, uncapped repetition,
escaped paths, index-derived ordering, and the shared overlay/replay
text. `cmd/vrg/pty_test.go` gained the non-zero-exit
handshake test and the 1 MiB stderr-content fixture asserting head and
tail visibility on a large PTY; `main_test.go`'s flood boundary test
now steps through the warning overlay (`^@` marker) before the browse
marker. See [unit-tests.md](unit-tests.md).

## Files

- `internal/searchindex/index.go` — `Feed`, the `open`/`incomplete`
  lifecycle state, `IntegrityFailures`, `Stop.Incomplete`, `seal`;
  Issue #36 added `cause.go` (`Cause`/`CauseKind`/`Cause.Line()`),
  `IntegrityCauses()`, the `Add` post-summary gate, and
  `OversizedDiagnostics()`.
- `internal/app/overlay.go` — `decideOutcome`, `processFatal`, the
  `overlay` type, `overlayKey`, `layout`,
  `maxScroll`, `renderOverlay`, `renderBlank`; Issue #11 split the
  collection side out as `completionDiagnostics`; Issue #36 unified the
  composition — `outcomeInput`, `composeDiagnostics` (the universal
  ordered list) and `completionDiagnostics` (its collection subset),
  replacing `collectDiagnostics`/`processDiagnostic`/
  `streamDiagnostics`;
  Issue #15 added `openOverlay` (open-or-append plus pop-up
  cancellation) and `composite` (the shared centred splice);
  Issue #31 generalized the component into value methods and the
  shared `scrollOverlay` handler the help overlay reuses.
- `internal/app/app.go` — `phaseFatal`, the `overlay` field, the
  `decideOutcome` branch in `Update`, overlay-first key routing, and
  `View` compositing.
- `internal/app/search.go` — `prepareIndex` feeds the stream through
  `Index.Feed`; `searchDoneMsg`'s `stderr`/`waitErr` are now consumed.

See also:
[unsupported-encodings.md](unsupported-encodings.md) — Issue #30's
detection overlay rides the same open-or-append route and the
all-unsupported outcome-matrix row keeps the fixed status at 0;
[help-overlay.md](help-overlay.md) — Issue #31's modal help, the
second instance of the shared overlay component.
