# Stream-integrity diagnostics (Issue #36)

[Issue #36](../issues/036-stream-integrity-fatal-diagnostics.md)
([task](../tasks/036-stream-integrity-fatal-diagnostics.md)) replaces
the generic "ripgrep event stream incomplete" note with structured,
per-violation diagnostics: `internal/searchindex` records one
`Cause` per offending physical record, and `internal/app` maps each
cause to a stable user-facing line inside the universal diagnostic
composition. From *Result index, records, and stream integrity* (the
lifecycle matrix) and *Outcome and exit-status contract* (the
diagnostic content) in [`Notes/PRD-vrg.md`](../PRD-vrg.md). Builds on
the [Issue #9 lifecycle matrix and outcome
function](error-overlay-and-outcomes.md), the [Issue #10 record-loss
components](record-robustness.md), the [Issue #11 shared
composition](stderr-replay.md), and the [safe-presentation escaping
contract](safe-presentation.md).

## The structured cause model

`searchindex.Integrity` gains `Causes []Cause` alongside `Complete` —
whose semantics are unchanged. `Cause{Kind CauseKind; Path []byte}`
carries the stable failure class plus the decoded raw path bytes where
the kind's diagnostic names a file (`nil` otherwise):

| `CauseKind` | Offending record | Diagnostic line |
| --- | --- | --- |
| `CauseDuplicateBegin` | `begin(P)` while P is open, or after P's terminal binary exclusion | `duplicate begin for <path>` |
| `CauseOrphanedMatch` | `match(P)` while P is not open — never begun, already ended, or binary-excluded | `orphaned match for <path>` |
| `CauseOrphanedEnd` | `end(P)` while P is not open | `orphaned end for <path>` |
| `CauseMissingEnd` | file still open at stream end | `missing end for <path>` |
| `CauseMissingSummary` | stream ended without its `summary` | `missing summary record` |
| `CauseExtraSummary` | a second `summary` record | `extra summary record` |
| `CauseAfterSummary` | any other record after the `summary` | `record after summary` |
| `CauseUnterminated` | trailing unterminated fragment (pre-summary) | `unterminated final record` |

`integrityLine` in `internal/app/outcome.go` renders each kind once;
every embedded path passes through `safepresentation.EscapePath` at
composition time, so a path carrying LF or ESC bytes can never forge a
line boundary or a control sequence in either sink.

## One cause per physical record — the precedence

Each physical record contributes **at most one** cause, chosen by the
most-specific applicable rule:

- A second `summary` is `CauseExtraSummary` — never `CauseAfterSummary`.
- Once a valid `summary` has been seen, every later record is
  `CauseAfterSummary` and is **not dispatched** to a lifecycle parser:
  a post-summary `begin(Q)` cannot open Q (so it can never yield a
  missing `end`), and a post-summary `match`/`end` is never orphaned.
  Issue #36 removed Issue #9's `context` exemption — a post-summary
  `context` fails integrity like any other post-summary record — and
  owns the corrected lifecycle-matrix row; [Issue
  #44](../issues/044-post-summary-context-integrity-failure.md)
  ([task](../tasks/044-post-summary-context-integrity-failure.md))
  supplies the dedicated `context` regression coverage described
  below.
- The trailing unterminated fragment's cause is resolved at
  `Integrity()` time: `CauseAfterSummary` when a valid summary
  preceded it, else `CauseUnterminated` — never both.

## Ordering, multiplicity, determinism

`Integrity()` returns mid-stream causes in **detection order**, then
the end-of-stream causes: `CauseMissingEnd` for each still-open file
ordered by **unsigned raw path bytes**, then `CauseMissingSummary`,
then the tail fragment's cause. The list is deliberately **uncapped**:
repeated identical violations produce one cause each — no aggregation,
deduplication, or truncation — and `Integrity()` is deterministic,
rebuilding identically on every call.

## The universal component order

`outcomeDiagnostics` composes every overlay/replay line list in one
fixed order — `processDiags` then `tailDiags`:

1. **Process** — captured stderr lines; else, only when the process
   failed (signal death or a code other than 0/1), the generated
   `ripgrep exited with code N` / `ripgrep died: <signal>` line. A 0/1
   exit **never** produces a process-status line — an rg-0 stream
   missing its summary names the summary, not `exited with code 0`.
2. **Integrity** — one `integrityLine` per structured cause, in cause
   order.
3. **Record loss** — the malformed aggregate, the oversized aggregate,
   then one `oversized record skipped for <path>` line per recovered
   path. Issue #37 pins the oversized half: the pluralized aggregate is
   always emitted (an anonymous record is never invisible) and the
   details are deduplicated by raw path in first-occurrence order — see
   [oversized-diagnostics.md](oversized-diagnostics.md).
4. **Warnings** — caller-composed lines such as "N unrecognised record
   types skipped".

The overlay shows exactly this list, and `collectSearchDiags` feeds
the *same* `tailDiags` output into the session collection — composing
once satisfies both sinks, so the replayed stderr text matches the
overlay verbatim. A failed child's real stderr therefore appears
**alongside** the integrity causes rather than replacing them.

## Dual representation of post-summary record loss

Post-summary malformed, oversized, and unknown-type records are
counted twice over, deliberately: once as `CauseAfterSummary` and once
in their own independent counters. A post-summary malformed record
yields `record after summary` **plus** the malformed aggregate; a
post-summary oversized record yields the cause plus the oversized
aggregate and any recovered path detail (`feedOversized`'s
`terminated` flag keeps an oversized trailing fragment under the
single tail cause); a post-summary unknown-type record yields the
cause plus the unknown-type warning. Validation runs before the
post-summary check, so position never suppresses the record's own
accounting.

## Summary-is-final and `context` (Issue #44)

The PRD's *Result index, records, and stream integrity* section makes
the `summary` final: **any** record after it — `context` included — is
a stream-integrity failure, never an exemption. Issue #9's former
"`context` in any position" matrix row is therefore amended to cover
only pre-`summary` positions: before `begin`, inside a file block, or
between `end` and `summary`, a `context` payload stays ignored for
matching and lifecycle purposes — no cause, no retained file, even
when its path never opens.

Ownership is split at the parser boundary: [Issue
#36](../issues/036-stream-integrity-fatal-diagnostics.md) removed the
`context` exemption in `Index.Feed`'s post-summary branch and
corrected the contradictory "context after summary has no lifecycle
effect" lifecycle row; [Issue
#44](../issues/044-post-summary-context-integrity-failure.md) begins
from that green behavior and adds the focused coverage — no second
parser change:

- `lifecycle_test.go` gains the dedicated `summary` → `context` row
  asserting exactly the single `CauseAfterSummary`, plus the
  neighbouring pre-`summary` rows (context before `begin`, context
  between `end` and `summary`) proving the exemption's surviving half.
- `internal/app` gains the `context`-after-`summary` outcome-matrix
  row — retained results browse under the error overlay at exit 2 —
  and `TestContextAfterSummaryOutcome`, asserting the fatal
  presentation, that the complete composed diagnostic is exactly
  `record after summary`, and that the same line list is collected
  for the post-restoration stderr replay.

## Tests

See [unit-tests.md](unit-tests.md):
`internal/searchindex/lifecycle_test.go` asserts each matrix row's
`wantCause` list through `checkCauses` — kinds, raw paths, detection
and end-of-stream ordering, per-record multiplicity, and the corrected
post-summary `context` row — plus the independent counters;
`internal/app/outcome_test.go`'s `TestIntegrityDiagnostics` asserts
the complete ordered overlay **and** collected-replay line list per
fixture — every cause's text, escaping, multiplicity, dual
representation, and the universal component order —
`TestIntegrityDiagnosticsDeterministic` pins the unsigned-raw-path
missing-`end` ordering across rebuilds, and Issue #44's
`TestContextAfterSummaryOutcome` plus its outcome-matrix row pin the
post-`summary` `context` stream's fatal path and exact diagnostic.
