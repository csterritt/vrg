# Oversized-record diagnostics (Issue #37)

[Issue #37](../issues/037-oversized-record-aggregate-anonymous-diagnostics.md)
([task](../tasks/037-oversized-record-aggregate-anonymous-diagnostics.md))
completes the oversized-record half of the record-loss diagnostic: the
pluralized aggregate is always emitted and the recoverable per-path
details are deduplicated by raw path, so an oversized record whose path
never parsed can no longer produce an empty or absent diagnostic. From
the oversized bullets of *Result index, records, and stream integrity*
and the 64 MiB limit of *Resources and responsiveness* in
[`Notes/PRD-vrg.md`](../PRD-vrg.md). Builds on the [Issue #10 64 MiB
limit and path recovery](record-robustness.md) and the [Issue #36
universal component order](integrity-diagnostics.md).

## The aggregate is always emitted

`recordLossLines` in `internal/app/outcome.go` leads the oversized
component with a grammatically pluralized per-record aggregate built
from the oversized count — never from the number of recovered paths:

- exactly `1 oversized record skipped` for one oversized record;
- exactly `N oversized records skipped` for every other count.

The aggregate is emitted whenever the count is positive, **regardless
of path recovery** — an oversized record whose `type`/`data.path` never
parsed before the 64 MiB limit (the anonymous case) still surfaces in
the diagnostic through the aggregate alone.

## Per-path details: deduplicated, after the aggregate

One `oversized record skipped for <sanitized path>` line per **distinct
raw path** follows the aggregate, deduplicated by raw path bytes while
preserving deterministic first-occurrence order. The aggregate counts
records; the details name paths — two oversized records naming the same
recoverable path produce `2 oversized records skipped` followed by
**one** detail line, never one detail per record. Each path passes
through `safepresentation.EscapePath` at composition time, the same
escaping the integrity causes use, so a hostile path can never forge a
line break. `Index.OversizedPaths` keeps one entry per *named record*
— duplicates included — and the deduplication lives at composition in
`recordLossLines`, not in the index's accounting.

Inside [Issue #36's universal component
order](integrity-diagnostics.md) the record-loss component holds its
own internal order — the malformed aggregate, then the oversized
aggregate, then the per-path details — between the integrity causes
and the caller's warnings.

## Anonymous records are never silent

An anonymous oversized record — the byte limit hit before `type` and
`data.path` were parsed — contributes to the aggregate but to no
per-path detail:

- **Zero usable results** — record loss on an otherwise-complete
  stream is fatal: the overlay-only presentation contains *exactly*
  `1 oversized record skipped` (never an empty overlay) and dismissing
  it exits 2.
- **Usable results** — the browse view opens under a visible overlay
  showing the aggregate, and the same line enters the session
  collection replayed to stderr after terminal restoration (exit 0) —
  the loss never passes silently.

A mixed-recoverability stream behaves the same way: the aggregate
reflects the total count across named and anonymous records alike, and
only the distinct recoverable paths appear beneath it.

## Tests

See [unit-tests.md](unit-tests.md):
`internal/app/outcome_test.go`'s `TestOversizedAggregateDiagnostics`
pins the composition at the `DecideOutcome` level — singular/plural
aggregates, anonymous counts, deduplication by raw path, and
first-occurrence ordering — while `TestOversizedDiagnostics` feeds
real oversized streams through the collection command and asserts the
complete ordered overlay **and** replayed-diagnostic line lists,
including the mixed-recoverability case and the aggregates-before-
details order. The outcome matrix gains the anonymous fatal row
(overlay-only, exactly the aggregate, exit 2) and the anonymous and
named non-fatal rows (browse under the overlay, exit 0).
