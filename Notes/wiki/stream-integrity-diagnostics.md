# Stream-integrity diagnostics (Issue #36)

Structured integrity-cause reporting and universal diagnostic
composition, delivered by
[Issue #36](../issues/036-stream-integrity-fatal-diagnostics.md)
(tasks: `Notes/tasks/036-stream-integrity-fatal-diagnostics.md`) and
extended by
[Issue #37](../issues/037-oversized-record-aggregate-anonymous-diagnostics.md),
which landed the oversized aggregate in the record-loss component, and
[Issue #44](../issues/044-post-summary-context-integrity-failure.md),
which added dedicated `context`-after-`summary` regression coverage
pinning the summary-is-final contract. It
closes the gap where a fatal stream-integrity outcome was explained as
`ripgrep exited with code 0` (when ripgrep produced no stderr) or as
stderr-only (when it did) — the actual reason the stream was incomplete
was never stated. Relevant PRD sections: *Result index, records, and
stream integrity* (integrity bullets) and *Outcome and exit-status
contract* (fatal rows, generated-diagnostic rule). See also
[outcome-contract](outcome-contract.md),
[record-robustness](record-robustness.md),
[search-collection-path](search-collection-path.md), and
[safe-presentation](safe-presentation.md).

## Structured cause model

`searchindex.Integrity` now carries `Causes []IntegrityCause` alongside
`Complete`. Each `IntegrityCause` is a stable `IntegrityCauseKind` plus
the affected raw path (`Path []byte`, nil when the offending record
names no path). One cause is recorded per offending physical record —
no aggregation, deduplication, or cap; repeated identical violations
(e.g. several orphaned `match` records for one path) produce one cause
each. `Causes` is empty exactly when `Complete` is true.

| Cause kind | Offending record | Diagnostic line |
|---|---|---|
| `IntegrityCauseDuplicateBegin` | `begin(P)` while P already open | `duplicate begin record for <P>` |
| `IntegrityCauseOrphanedMatch` | `match(P)` while P not open | `orphaned match record for <P>` |
| `IntegrityCauseMatchAfterBinaryEnd` | `match(P)` after a binary-excluding `end(P)` | `orphaned match record for <P>` (late match not retained) |
| `IntegrityCauseOrphanedEnd` | `end(P)` while P not open | `orphaned end record for <P>` |
| `IntegrityCauseMissingEnd` | P still open when the stream ends | `missing end record for <P>` |
| `IntegrityCauseMissingSummary` | stream has no valid `summary` | `missing summary record` |
| `IntegrityCauseExtraSummary` | a second `summary` | `extra summary record` |
| `IntegrityCauseRecordAfterSummary` | any other record after a valid `summary` | `record after summary` |
| `IntegrityCauseUnterminatedRecord` | trailing unterminated record outside the post-summary state | `unterminated final record` |

Every embedded path passes through `safepresentation.EscapePath` at
composition time, so a path newline cannot forge a diagnostic paragraph
break.

## Overlap precedence: one cause per physical record

The most-specific applicable row wins:

- A second `summary` contributes only `extra summary`, never `record
  after summary`. A malformed second `summary` still contributes the
  extra-summary cause plus its malformed count.
- After the first valid `summary`, every other record contributes only
  `record after summary` and is not dispatched to a lifecycle parser:
  it cannot mutate open-file state (a post-summary `begin(Q)` cannot
  open Q or later produce `missing end`), is not indexed (a
  post-summary `match` adds no stop), and is not schema-validated
  beyond its `type` field. Issue #36 owns this post-summary transition:
  it removed the `context` exemption in `Builder.Add` and corrected the
  superseded `"context after summary has no lifecycle effect"` row in
  `lifecycle_test.go`; Issue #44 begins from that green boundary.
- A trailing unterminated fragment after a valid `summary` contributes
  only `record after summary` (plus its malformed-count
  representation), not a second `unterminated final record` cause.
  Outside the post-summary state it contributes `unterminated final
  record` plus the malformed count — which means the unterminated cause
  always co-occurs with `missing summary`, since a stream whose last
  bytes are an unterminated fragment never carried a valid summary.

## Summary-is-final and the `context` record (Issue #44)

The summary-is-final contract is unqualified: *any* record after a
valid `summary` — including `context` — is a stream-integrity failure
contributing exactly the `record after summary` cause. Issue #9's
former "`context` in any position" lifecycle-matrix row is amended to
cover only pre-`summary` positions: `context` payloads stay ignored
for match and lifecycle purposes *before* `summary` — before `begin`,
while a file is open (ripgrep's real position for context records),
and between `end` and `summary` — producing no causes, no stops, and
a complete stream. Only the post-`summary` position is a violation.

Ownership is split across two issues. Issue #36 owned the parser
change — removing the `context` exemption in `Builder.Add` and
correcting the contradictory `"context after summary has no lifecycle
effect"` row in `lifecycle_test.go` — as part of its post-summary
precedence GREEN. Issue #44 began from that green boundary and added
dedicated coverage only, with no second parser change:

- `lifecycle_test.go` gained a neighbouring `context while open` row;
- `TestIntegrityCauseMatrix` gained neighbouring pre-`summary`
  `context` rows (before `begin`, while open, after `end` before
  `summary`) asserting an empty cause list beside the dedicated
  `summary` → `context` row asserting exactly the single
  `record after summary` cause;
- `internal/app/outcome_test.go` gained
  `TestContextAfterSummaryOutcome`, asserting the `summary` →
  `context` stream takes the outcome matrix's fatal path (exit 2) in
  both the zero-results fatal-overlay and retained-results
  browse-overlay dispositions, with the complete composed diagnostic
  `record after summary` retained for post-restoration stderr replay.

Cross-references:
[Issue #36](../issues/036-stream-integrity-fatal-diagnostics.md),
[Issue #44](../issues/044-post-summary-context-integrity-failure.md),
and `Notes/PRD-vrg.md` *Result index, records, and stream integrity*
(the complete-stream bullets) and *Outcome and exit-status contract*
(the fatal rows).

## Ordering

Violations detected mid-stream appear in detection order. Violations
discovered only at end of stream follow, in this order:

1. `missing end` for each still-open file, ordered by unsigned raw-path
   bytes (never map iteration order — `Build` sorts the still-open key
   set before recording);
2. `missing summary`;
3. the trailing-fragment cause: `unterminated final record`, or
   `record after summary` when a valid summary put the fragment under
   post-summary precedence.

The composed cause list is therefore deterministic across runs and
builds of the same stream.

## Dual representation is intentional

When one physical record is both an integrity violation and an
independently counted record loss or warning, both representations are
retained in their respective components:

- a trailing unterminated record produces its integrity-cause line and
  increments the malformed aggregate;
- a malformed record after `summary` produces `record after summary`
  and contributes to the malformed count;
- a post-`summary` oversized record produces `record after summary` and
  retains its oversized count — the Issue #37 aggregate
  `N oversized record(s) skipped` — plus the recoverable-path detail
  `oversized record skipped for <escaped path>` when available;
- a post-`summary` unknown-type record produces `record after summary`
  and retains its unknown-type count/warning.

## Universal component order

`composeDiagnostics` in `internal/app/app.go` builds one ordered
component list used identically by every `DecideOutcome` branch, fatal
and non-fatal:

1. **Process component** — collected ripgrep stderr in collection
   order; or, only when the process failed (signal death or exit code
   other than 0/1) and supplied no explanatory stderr, a generated line
   naming the exit code (`ripgrep exited with code N`) or signal
   (`ripgrep killed by signal N`). No process-status line is ever
   emitted for a 0/1 exit.
2. **Integrity-cause lines** in `Integrity.Causes` order.
3. **Record-loss components** in Issue #37's order: malformed
   aggregate, then the oversized component — the always-emitted
   pluralized aggregate (`1 oversized record skipped` /
   `N oversized records skipped`) built from `Index.OversizedCount()`,
   followed by the per-path oversized details deduplicated by raw path
   in first-occurrence order. The aggregate is emitted whenever the
   count is positive even when no detail exists, so an anonymous
   oversized record never yields an empty fatal overlay or a silent
   non-fatal loss. (Issue #36 reordered `recordLossDiagnostics` so the
   oversized component sits ahead of the unknown-type warning;
   Issue #37 prepended the aggregate and deduplicated the details.)
4. **Unknown-type warnings.**

Omitted components do not change the relative order of those present.
Because the overlay text is collected once via `collectDiagnostic` and
replayed verbatim by `cmd/vrg`, composing once satisfies both the
overlay and the post-restoration stderr replay with the same lines in
the same order.

## Recording sites

`Builder` records each cause at the violation site: duplicate `begin`
in `parseBegin`, orphaned `match` and the binary-exclusion precedence
path in `parseMatch`, orphaned `end` in `parseEnd`, second `summary` in
`parseSummary` (before data validation, so a malformed second summary
keeps the extra-summary cause), after-`summary` detection in `Add`
(which also covers post-summary malformed, unknown-type, and
terminated-oversized records), and the trailing-fragment cause in
`Build`. `Integrity().Complete` semantics, malformed/unknown/oversized
counting, and retained-match behavior are unchanged.
