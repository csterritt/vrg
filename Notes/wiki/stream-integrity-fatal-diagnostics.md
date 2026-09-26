# Stream integrity fatal diagnostics (Issue #36)

The structured-cause and universal-composition contract delivered by
[Issue #36](../tasks/036-stream-integrity-fatal-diagnostics.md),
implemented in `internal/searchindex` (`cause.go`, `index.go`) and
`internal/app` (`overlay.go`, the `searchDoneMsg` branch of `app.go`).
Relevant PRD sections in `Notes/PRD-vrg.md`: *Result index, records,
and stream integrity* (the lifecycle matrix this completes) and
*Outcome and exit-status contract* (the fatal/diagnostic ordering
this pins). It completes the Issue #9 contract in
[error-overlay-and-fatal-outcomes.md](error-overlay-and-fatal-outcomes.md)
and interlocks with Issue #10's dispositions in
[record-robustness.md](record-robustness.md).

## Structured integrity causes

`Index` no longer stores integrity diagnostics as strings. Each
violation appends one `searchindex.Cause` — a stable `CauseKind` plus
the offending record's **raw path bytes** where the record names one
(`nil` for record-level causes naming no file). `IntegrityCauses()`
returns the ordered slice; `IntegrityFailures()` is retained as the
derived view — one `Cause.Line()` per cause, in the same order — for
callers and tests that want the rendered text.

The kinds and their stable lines:

| `CauseKind` | Trigger | `Line()` |
|---|---|---|
| `CauseDuplicateBegin` | `begin(P)` while P open | `duplicate begin for <P>` |
| `CauseOrphanedMatch` | `match(P)` while P not open (never opened, or after a non-binary `end`) | `orphaned match for <P>` |
| `CauseMatchAfterEnd` | `match(P)` after a binary-excluding `end(P)` | `match for <P> arrived after a binary-excluding end` |
| `CauseOrphanedEnd` | `end(P)` while P not open | `orphaned end for <P>` |
| `CauseMissingEnd` | P still open at stream end | `missing end for <P>` |
| `CauseMissingSummary` | stream ended without a summary | `missing summary` |
| `CauseExtraSummary` | a second (or later) summary | `extra summary record` |
| `CauseRecordAfterSummary` | any other record after the first valid summary | `record after summary` |
| `CauseUnterminatedFinal` | trailing fragment outside the post-summary state | `unterminated final record` |

Every `<P>` is escaped through `present.Path` — the single-line path
utility — at composition time, so a hostile path (embedded newline,
ESC, invalid UTF-8) can never forge a paragraph break or a terminal
control. Raw bytes remain the identity and ordering keys; the escaped
form is display-only (see
[safe-presentation.md](safe-presentation.md)).

## One cause per physical record

Each physical record contributes **at most one** integrity cause, and
repeated identical violations are emitted **one-for-one — deliberately
uncapped, unaggregated, undeduplicated**: three orphaned matches for the
same path are three causes.

The overlap precedence, pinned by `TestIntegrityCauses`:

- A second summary contributes only `CauseExtraSummary` — never a
  `record after summary` cause for the same record.
- Once the first valid summary is seen, every subsequent record is
  checked **before** lifecycle dispatch: it contributes only
  `CauseRecordAfterSummary` (carrying the record's raw path when it
  names one) and is **never lifecycle-processed** — a post-summary
  `begin(Q)` cannot open Q and owes no later `missing end`, a
  post-summary `match` is neither retained nor marked incomplete, and
  a post-summary `context` is not exempt. **Issue #36 owns the removal
  of the old context exemption** and the correction of the Issue #9
  lifecycle row that read "context after summary has no lifecycle
  effect" — it now expects the `record after summary` cause.
- Post-summary malformed, oversized, and unknown records keep their
  **dual representation**: the sole `record after summary` cause plus
  their independent tally — the malformed count, the oversized count
  with any recoverable path detail (`OversizedDiagnostics()`), and the
  unknown-type count/warning.
- A trailing unterminated fragment splits on summary state: after a
  valid summary it is the `record after summary` cause plus the
  malformed count — **no separate unterminated cause**; outside the
  post-summary state it defers `CauseUnterminatedFinal` to seal plus
  the malformed count (plus oversized when the fragment also exceeds
  the limit).

## Deterministic ordering

Causes are emitted in two phases:

1. **Detection order** — mid-stream violations in stream order.
2. **End-of-stream**, appended by `seal` in this mandated order:
   missing end for each file still open, sorted by **unsigned raw path
   bytes** (`slices.Sort` on the stringified raw keys — never map
   iteration order); then `missing summary`; then
   `unterminated final record` where after-summary precedence did not
   already claim the fragment.

`Integrity().Complete` semantics are unchanged: integrity is still
assessed separately from the child's exit status.

## Universal diagnostic composition

`internal/app` composes **one** ordered component list —
`composeDiagnostics(outcomeInput)` — that feeds both the overlay
(`decideOutcome`'s `diags`) and, through `completionDiagnostics` (the
same list minus the stderr the live collection already took), the
session collection `cmd/vrg` replays verbatim at exit. The order:

1. **Process component** — the child's sanitized stderr in collection
   order; or the generated `rg failed: exit status N` /
   `rg failed: signal: killed` line **only** when the process died
   fatally (signal or exit code other than 0/1) *and* stderr carried
   nothing. Explanatory stderr suppresses the generated line. Exit 0
   and exit 1 never produce a process-status line — a missing-summary
   stream under a clean exit names `missing summary`, never
   `ripgrep exited with code 0`.
2. **Integrity causes** — one `Cause.Line()` per cause, in cause
   order.
3. **Record-loss components** — the malformed aggregate
   (`N malformed record(s) skipped`), then the oversized component
   completed by Issue #37: the pluralized per-record aggregate —
   exactly `1 oversized record skipped` or `N oversized records
   skipped`, emitted whenever the count is positive even with no
   detail — followed by the per-path oversized details
   (`oversized record skipped for <escaped path>`), one line per
   **distinct raw path** deduplicated in first-occurrence stream order
   (the `text` and `bytes` encodings of one path agree). An oversized
   record whose limit cut before `type`/`data.path` were parsed is
   anonymous: it contributes only the aggregate — never an empty
   overlay — so record loss can never pass silently. See
   [record-robustness.md](record-robustness.md).
4. **Unknown-type warnings** — `N unrecognised record types skipped`.

`outcomeInput` carries the structured inputs — wait error, stderr,
causes, usable-stop count, the malformed/oversized/unknown tallies,
and the per-path oversized detail lines — so fatal branches can no
longer drop record-loss diagnostics or replace integrity causes with a
bare process status: a fatal stream with real child stderr shows the
stderr, every cause, and every tally together.

## Tests

`internal/searchindex/causes_test.go` is the Issue #36 structured-cause
coverage: `TestIntegrityCauses` pins the exact ordered cause list
(kind + raw path) and the independent counters per row — every Issue #9
lifecycle-failure row, the second-summary and third-summary overlap
rows, every post-summary exact-state row (begin, match, context,
unterminated fragment, oversized, unknown-type), the composed
post-summary suppression row, detection-order-then-end-of-stream
ordering with two open files proving unsigned raw-path sort, and the
uncapped one-cause-per-record repetition row — plus
`TestOversizedAfterSummaryKeepsRecoveredPath` for the recovered-path
dual representation. `lifecycle_test.go`, `disposition_test.go`, and
`oversized_test.go` rows were corrected to the new strings and
precedence (binary-late-match wording, `extra summary record`,
post-summary rows expecting only `record after summary`, the
unterminated cause order, and the Issue #36-owned context-after-summary
correction).

`internal/app/diagnostics_test.go` pins the composition:
`TestComposedDiagnostics` asserts the complete exact line slices for
both `composeDiagnostics` and `completionDiagnostics` — stderr
precedence, the generated line only for fatal-without-stderr, no
process line for 0/1, the full universal order, the post-summary dual
representation (Issue #37's exact post-summary oversized slice:
`record after summary`, `1 oversized record skipped`, the recovered
`big.txt` detail), uncapped repetition, and the escaped newline path —
`TestOutcomeCodesFromInput` the fixed statuses from the structured
input, `TestComposedDiagnosticsFromIndexOrdering` the index-derived
missing-end ordering across 20 repeated builds, and
`TestOverlayAndReplayShareComposedDiagnostics` /
`TestFatalOverlayKeepsStderrCausesAndRecordLoss` the shared
overlay/replay text and the unsuppressed fatal composition through the
real `searchDoneMsg` path. Issue #37 added the anonymous and
mixed-recoverability model-level tests through `fixtureStream`'s real
64 MiB records: the fatal overlay carrying exactly the aggregate, the
browse warning plus replay, and the deduplicated detail lines.
`readme_test.go`'s exit-status agreement
drove `decideOutcome` onto the `outcomeInput` signature. See
[unit-tests.md](unit-tests.md).

## Files

- `internal/searchindex/cause.go` — `CauseKind`, the nine-cause const
  block, `Cause{Kind, Path}`, and `Cause.Line()` rendering through
  `present.Path`.
- `internal/searchindex/index.go` — the `causes`/`unterminated` fields,
  `fail(kind, path)`, the `Add` post-summary gate (extra-summary
  special-case, record-after-summary for everything else, no lifecycle
  dispatch), the `Feed` trailing-fragment split, `seal`'s mandated
  end-of-stream order, `IntegrityCauses()`/`IntegrityFailures()`, and
  `OversizedDiagnostics()`.
- `internal/app/overlay.go` — `outcomeInput`, `decideOutcome`'s
  structured inputs, `composeDiagnostics` (the universal ordered list)
  and `completionDiagnostics` (the collection subset), replacing
  `collectDiagnostics`/`processDiagnostic`/`streamDiagnostics`.
- `internal/app/app.go` — the `searchDoneMsg` branch gathers the
  index's structured accessors into `outcomeInput`.

See also:
[error-overlay-and-fatal-outcomes.md](error-overlay-and-fatal-outcomes.md)
— the Issue #9 contract this completes;
[record-robustness.md](record-robustness.md) — the Issue #10
dispositions the dual representation preserves;
[stderr-replay.md](stderr-replay.md) — the session collection the
shared composition feeds.
