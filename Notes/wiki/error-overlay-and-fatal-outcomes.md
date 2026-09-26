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
| `context(P)` anywhere | ignored; no lifecycle effect |
| P open when the stream ends | integrity failure: `missing end for P`; P's stops are marked `Incomplete` |
| exactly one `summary`, final | valid; a lone summary is a complete zero-result stream |
| missing `summary` | integrity failure: `missing summary` |
| second `summary` | integrity failure: `second summary` |
| any record after `summary`, including malformed bytes | integrity failure: `record after summary` |
| trailing unterminated record | malformed **and** incomplete: `unterminated trailing record` — plus `record after summary` when it follows the summary, a deliberate double disposition |

**Binary exclusion takes precedence over orphan retention**: a match
arriving after a binary-excluding `end(P)` is not retained and P stays
excluded.

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
the completed search's independent inputs: the child's wait error, the
captured stderr, the stream-integrity failures, the usable-results
count (retained stops — `Index.LineCount`), and — since Issue #10 —
the record-loss count (malformed + oversized) with the record-skip
diagnostic lines (`Index.RecordDiagnostics`). It returns the underlying
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
including with the overlay open.

## Diagnostics and stderr classification

Captured stderr is classified regardless of rg's exit code: non-empty
stderr under a clean exit or exit 1 is a *warning* diagnostic shown in
the overlay, and under a fatal outcome it joins the process diagnostic.
When a fatal process result supplies **no** stderr, a generated
diagnostic names the wait status — `rg failed: exit status 3` or
`rg failed: signal: killed` — so the overlay never opens empty.
Diagnostics assemble in order: the generated process line, the
sanitized stderr text, the stream-integrity failures, then — since
Issue #10 — the record-skip diagnostics (`oversized record skipped for
<path>` lines first, then the malformed/oversized/unknown tallies).
Everything
passes through `present.Diagnostic` — the Issue #6 utility — before it
can reach the screen; see
[safe-presentation.md](safe-presentation.md).

Since Issue #11 the display list is assembled by `collectDiagnostics`
while the **session collection** takes the completion-only subset from
`completionDiagnostics` (process line, integrity failures, record-skip
lines — `processDiagnostic`/`streamDiagnostics`) — child stderr is
excluded there because the collection already took it line-by-line
through `diagMsg` while the search ran, so collecting it again would
double-count. Display and collection are independent; see
[stderr-replay.md](stderr-replay.md).

## The modal overlay

`overlay` (`internal/app/overlay.go`) holds the sanitized diagnostic
lines and the scroll offset. Since Issue #15 every opening goes
through `openOverlay`: it appends the new lines when an overlay is
already open (preserving the reader's scroll) and cancels any live
file-change pop-up — an arriving error overlay removes the pop-up and
it cannot return when the overlay is dismissed (see
[file-change-popup.md](file-change-popup.md)). While `m.overlay` is
non-nil it owns the
keyboard — `overlayKey` runs before the base-state switch:

- `up`/`down` scroll one row, clamped at both ends.
- `q` and `Esc` dismiss to the underlying screen — or quit with the
  fixed status 2 when the outcome was fatal with no usable results
  (`phaseFatal`: the overlay stands alone on a blank frame).
- `ctrl+c` takes the cancellation path to 130.
- Every other key is ignored — `c` cannot reach the colour toggle
  beneath an open overlay.

`overlayLayout` resolves geometry from the frame: interior width is the
smaller of the frame minus the border and the widest diagnostic line;
each line hard-wraps with `ansi.Wrap` so an unbroken diagnostic never
overflows the border; interior height is the smaller of frame minus
border and the wrapped count. `renderOverlay` composites
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
`openOverlay`, alongside the `(unreadable)` panel placeholder — the
first slice of Issue #26's read-failure rules, delivered as the
injected trigger Issue #15's pop-up cancellation is tested against. A
failure for a non-current path still only joins the session collection
for stderr replay.

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
trailing-unterminated double disposition, summary positioning, orphan
retention with `Incomplete`, and open-at-end sealing.
`internal/app/outcome_test.go` is the single table-driven outcome
matrix covering every row above with dismissal and exit assertions —
Issue #10 added the record-loss and unknown-warning rows plus a
`stream`/`fixtureStream` path for fixtures containing undecodable
bytes.
`internal/app/overlay_test.go` pins scrolling, both dismissal keys,
`ctrl+c`, ignored keys (including the unreachable `c` toggle),
unbroken-line wrapping, and generated code-or-signal diagnostics.
`sinksafety_test.go` gained the `error overlay` row with per-fixture
`wantDiag` expectations. `cmd/vrg/pty_test.go` gained the non-zero-exit
handshake test and the 1 MiB stderr-content fixture asserting head and
tail visibility on a large PTY; `main_test.go`'s flood boundary test
now steps through the warning overlay (`^@` marker) before the browse
marker. See [unit-tests.md](unit-tests.md).

## Files

- `internal/searchindex/index.go` — `Feed`, the `open`/`incomplete`
  lifecycle state, `IntegrityFailures`, `Stop.Incomplete`, `seal`.
- `internal/app/overlay.go` — `decideOutcome`, `collectDiagnostics`,
  `processFatal`, the `overlay` type, `overlayKey`, `overlayLayout`,
  `renderOverlay`, `renderBlank`; Issue #11 split the collection side
  out as `completionDiagnostics`/`processDiagnostic`/`streamDiagnostics`;
  Issue #15 added `openOverlay` (open-or-append plus pop-up
  cancellation) and `composite` (the shared centred splice).
- `internal/app/app.go` — `phaseFatal`, the `overlay` field, the
  `decideOutcome` branch in `Update`, overlay-first key routing, and
  `View` compositing.
- `internal/app/search.go` — `prepareIndex` feeds the stream through
  `Index.Feed`; `searchDoneMsg`'s `stderr`/`waitErr` are now consumed.
