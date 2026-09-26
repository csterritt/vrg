# Record robustness (Issue #10)

The damaged-stream contract delivered by
[Issue #10](../issues/010-record-robustness-malformed-oversized-unknown.md),
implemented in `internal/searchindex` (`index.go`, `record.go`) and
`internal/app` (`overlay.go`, `app.go`). Relevant PRD sections in
`Notes/PRD-vrg.md`: *Result index, records, and stream integrity*
(malformed/oversized/unknown bullets), *Outcome and exit-status
contract* (the record-loss and warning rows), and *Resources and
responsiveness* (the 64 MiB bullets).

## Deterministic dispositions

Every record position resolves to one deterministic category — never an
implementation judgment:

| Category | Meaning |
|---|---|
| skipped-and-counted malformed | violates the per-record schema (Issue #3 matrix): invalid JSON, invalid base64, missing or non-string `type`, missing/mistyped required fields, invalid ranges (`line_number` < 1 or non-integer, `start > end`, negative `start`, `end` past the decoded line, negative `binary_offset`, empty `submatches`) |
| stream-integrity failure | violates the lifecycle matrix (Issue #9): duplicate `begin`, orphaned `match`/`end`, `match` after `end`, second `summary`, any record after `summary`, missing `summary`, missing `end` |
| oversized | exceeds the 64 MiB payload limit — a separate count, not malformed |
| unknown type | a string `type` outside the five known events — a warning tally, neither malformed nor an integrity failure by itself |

The counters never leak into each other: lifecycle violations never
inflate `Malformed()`, a safely skipped record never by itself damages
intact lifecycle metadata, and exactly two composite cases carry **both**
dispositions:

- A **trailing unterminated record** is counted malformed (no decode is
  attempted — the rule is unconditional on content) *and* fails
  integrity — since Issue #36 as `unterminated final record`, or, when
  the fragment sits after a valid summary, as the sole `record after
  summary` cause (the one-cause-per-record precedence replaces the old
  double integrity disposition).
- A **malformed record after `summary`** is counted malformed *and*
  contributes its sole `record after summary` cause — the independent
  tally and the positional cause both survive (Issue #36's dual
  representation).

`Index.Malformed()` exposes the tally; the malformed count stays
separate from `IntegrityFailures()`.

## The 64 MiB limit

`maxRecordPayload = 64 << 20` is enforced explicitly on each split line
inside `Feed` — an intentional bound, not a borrowed line-reader limit.
A record exactly at the limit decodes normally; one byte over is
oversized. The `bytes.Split` itself performs the discard-through-newline,
so the following record resynchronizes and the rest of the stream
indexes. Oversized records are counted by `Oversized()`; a file whose
only match records were oversized retains no stops and is absent from
the file list.

**Path recovery.** When an oversized record's `type` and `data.path`
were parsed before the limit — the usual case, since rg emits them
ahead of the line payload — the diagnostic names the file:
`oversized record skipped for <sanitized path>` (via `present.Path`,
the Issue #6 utility). `recoverRecordPath` streams a `json.Decoder`
over the record's first 64 MiB token by token — the prefix is not valid
JSON, ending mid-value where the limit fell — and `recoverDataPath`
descends into `data` member by member so `path` counts only when it
decoded whole through `decodeValue`. When the limit cut before the path
arrived, the record is counted anonymously: the tally line only.

**Unterminated oversized final record.** The trailing-unterminated rule
carries no oversized exception: a final record over the limit without
its newline is counted oversized **and** malformed **and** fails
integrity — all three dispositions for one record. Since Issue #36 an
oversized post-summary record keeps the same dual representation: the
sole `record after summary` cause plus the oversized tally and any
recovered path detail.

## Unknown types

`Add` tallies `KindUnknown` records into `Unknown()` — separate from
malformed and reported as `N unrecognised record types skipped`.
Unknown types never change the exit status independently, never satisfy
required completion events (a stream of only unknown records still
reports `missing summary`), and the counting rule is unqualified by
position: an unknown type after `summary` is counted in the
unknown-type tally *and* contributes its sole `record after summary`
cause — the skip reason and the ordering violation recorded
independently (Issue #36's dual representation), mirroring the
malformed-after-summary composite.

`Index.RecordDiagnostics()` assembles the nonfatal record-skip lines in
a fixed order — each recovered `oversized record skipped for <path>`
line in stream order, then the `N malformed record(s) skipped`,
`N oversized record(s) skipped`, and `N unrecognised record types
skipped` tallies for each nonzero count. Since Issue #36 the app
composes the components itself in the universal order — malformed
aggregate, oversized aggregate, the per-path oversized details from
`Index.OversizedDiagnostics()` (the Issue #37 stream order), then the
unknown warnings — so the unknown warning can never sit between the
malformed and oversized components.

## Missing `end`

Unchanged from Issue #9: a file still open at stream end keeps its
matches, marked `Stop.Incomplete`, and reports `missing end for <path>`
— see the lifecycle matrix in
[error-overlay-and-fatal-outcomes.md](error-overlay-and-fatal-outcomes.md).

## Record-loss outcomes

`decideOutcome` in `internal/app/overlay.go` now consumes the record-loss
inputs — since Issue #36 the `outcomeInput` fields `malformed`,
`oversized`, `oversizedDiags`, and `unknown`. "No usable results" is assessed **after all
filtering** — the retained-stop count — so a stream whose sole retained
file was binary-excluded after a skipped record follows the record-loss
fatal row, not the no-results row. The new outcome rows:

| Inputs | Presentation | Exit |
|---|---|---|
| rg 0/1, complete stream, record loss, usable results | browse + error overlay | 0 |
| rg 0/1, complete stream, record loss, **zero** usable results | record-loss overlay alone; `q` **or** `Esc` quits | 2 |
| rg 0/1, skipped record plus binary exclusion leaving zero retained stops | record-loss overlay alone | 2 |
| rg 0/1, unknown-type-only warnings, zero results | warning overlay → no-results | 1 |
| rg 0/1, missing `end`, retained matches | browse + error overlay | 2 |
| rg 0/1, missing `end`, no matches | fatal overlay alone | 2 |

Record loss is fatal only when nothing usable survived; with usable
results the skip tallies are warning diagnostics over browse at status
0. Unknown-type counts are diagnostics only and can never turn fatal on
their own. See the full table in
[error-overlay-and-fatal-outcomes.md](error-overlay-and-fatal-outcomes.md).

## Tests

`internal/searchindex/disposition_test.go` pins the deterministic
categories: `TestMalformedDispositions` drives every Issue #3 schema
violation embedded mid-lifecycle through `Feed` (counted malformed,
intact stream, resynchronized indexing); `TestIntegrityDispositions`
drives the lifecycle rows asserting `Malformed()` stays zero;
`TestCompositeDispositions` covers the two "both" rows.
`internal/searchindex/oversized_test.go` covers the 64 MiB boundary
(accepted at the limit, skipped one byte over), discard-and-resync, the
recoverable and unrecoverable oversized diagnostics, the
oversized-only-file absence, the post-summary triple-disposition
unterminated oversized final record (oversized + malformed + the sole
`record after summary` cause, Issue #36), and the unknown-type counting
rules. `causes_test.go` (Issue #36) pins the dual representations
exactly — sole causes plus independent tallies — including the
recovered-path oversized row.
`internal/app/outcome_test.go` extends the single outcome matrix with
the record-loss and unknown-warning rows (rows feeding malformed bytes
use the new `stream` field through `fixtureStream`). See
[unit-tests.md](unit-tests.md).

## Files

- `internal/searchindex/index.go` — `maxRecordPayload`, the `malformed`/
  `oversized`/`unknown` tallies, `oversizedPaths`, `skipMalformed`/
  `skipOversized`/`countOversized`, `Malformed()`/`Oversized()`/
  `Unknown()`/`RecordDiagnostics()`; Issue #36 added
  `OversizedDiagnostics()` (the per-path detail lines alone) and the
  `Cause`-based integrity model described in
  [stream-integrity-fatal-diagnostics.md](stream-integrity-fatal-diagnostics.md).
- `internal/searchindex/record.go` — `recoverRecordPath`/
  `recoverDataPath` token-streamed path recovery over the consumed
  prefix.
- `internal/app/overlay.go` — `decideOutcome`'s record-loss inputs and
  fatal row; `composeDiagnostics` orders the record-loss components
  (aggregates before per-path details, unknown warnings last).
- `internal/app/app.go` — the `searchDoneMsg` branch wires the index's
  tallies and diagnostics into the outcome decision.
