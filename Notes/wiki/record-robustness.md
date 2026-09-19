# Record robustness: malformed, oversized, unknown types (Issue #10)

[Issue #10](../issues/010-record-robustness-malformed-oversized-unknown.md)
makes the SearchIndex resilient to damaged streams and completes the
remaining outcome-table rows, from *Result index, records, and stream
integrity* (the malformed/oversized/unknown bullets), *Outcome and
exit-status contract* (the record-loss rows), and *Resources and
responsiveness* (the 64 MiB bullets) in
[`Notes/PRD-vrg.md`](../PRD-vrg.md). It builds on the
[Issue #3 schema matrix](search-collection.md) and the
[Issue #9 lifecycle matrix and outcome function](error-overlay-and-outcomes.md).

## Deterministic dispositions

Every record lands in a deterministic category — the categories are
specified by the matrices, not judged by the implementation:

- **Skipped-and-counted malformed** — invalid JSON, invalid base64, a
  missing or non-string `type`, or a known event violating the Issue #3
  per-record schema matrix (missing/wrongly-typed required fields;
  `line_number` zero, negative, non-integer, or overflowing int64;
  `start > end`; negative `start`; `end` beyond the decoded line bytes;
  negative or non-integer `binary_offset`; an empty `submatches` array).
  `Index.Malformed` counts them. A skipped record carries no lifecycle:
  a malformed `match` between a paired `begin`/`end` leaves the stream
  complete, and parsing resynchronizes on the next record.
- **Stream-integrity failure** — the Issue #9 lifecycle violations
  (duplicate/orphaned `begin`/`end`, `match` after `end`, second
  `summary`, any record after `summary`, a file open at stream end, a
  missing summary). These mark the stream broken but never inflate the
  malformed count, and vice versa — except the two composite rows.
- **Both** — the two composite rows: a trailing unterminated record is
  counted malformed *and* marks the stream incomplete (`FeedTail`), and
  a malformed record after a valid `summary` is counted malformed *and*
  flagged as an after-summary integrity failure. Validation runs before
  the post-summary check, so after-`summary` positioning never
  suppresses malformed accounting. Since Issue #36 these dual
  dispositions are dual **diagnostics** too: the post-summary record
  contributes `CauseAfterSummary` alongside its own malformed,
  oversized, or unknown accounting — see
  [integrity-diagnostics.md](integrity-diagnostics.md).
- **Unknown type** — a string `type` outside the five known events is
  `KindUnknown`, counted separately in `Index.Unknown` and reported as
  "N unrecognised record types skipped". The count is unqualified by
  position: an unknown type after `summary` still counts unknown while
  its position is separately an integrity failure. Unknown records can
  never substitute for required completion events — a stream ending on
  one without a `summary` fails integrity — and the count never
  independently changes the exit status.
- **`context` needs only its `type`** — the entire `data` payload is
  ignored, so `{"type":"context"}` and `{"type":"context","data":5}`
  are valid records, not malformed.

A file missing its `end` event keeps its matches: it is retained with
`File.Incomplete` set and the stream fails integrity (the
missing-`end` row).

## The 64 MiB payload limit

`searchindex.MaxRecordBytes` (64 MiB, excluding the newline delimiter)
bounds each record. `Index.Build` scans the collected stream for
newlines itself — an explicit bounded scan rather than a line reader's
default capacity — and a record over the limit is consumed and
discarded through its next newline, counted in `Index.Oversized`
(never in `Malformed`), and parsing resynchronizes on the following
record. Exactly 64 MiB is still accepted.

Oversized diagnostics name the file when recoverable:
`recoverOversizedPath` runs a bounded `json.Decoder` token pass over the
first `MaxRecordBytes` for `type` and `data.path` — the fields ripgrep
emits before the line payload — and each success appends the raw path
to `Index.OversizedPaths`, reported as "oversized record skipped for
\<sanitized path\>" in addition to the count. A record whose limit was
hit before `data.path` is counted anonymously. A file whose only
records were oversized is absent from the file list, so this diagnostic
is the user's only evidence of the loss. Issue #37 completes the
composition: the pluralized aggregate (`1 oversized record skipped` /
`N oversized records skipped`) is emitted whenever the count is
positive — an anonymous record is never invisible — and the per-path
details are deduplicated by raw path in first-occurrence order, one
line per distinct path beneath the aggregate. See
[oversized-diagnostics.md](oversized-diagnostics.md).

The trailing-unterminated rule carries no oversized exception: an
oversized final record without a newline takes **all three**
dispositions — the oversized count, the malformed count for its missing
termination, and the incomplete-stream integrity failure.

## Record-loss outcome rows

`DecideOutcome` now consumes the record-loss inputs (`RecordLoss`
carries `Malformed`, `Oversized`, and the recoverable `Paths`) and the
caller's `Warnings` — `recordWarnings` composes the "N unrecognised
record types skipped" line. `outcomeDiagnostics` inserts the
record-loss lines between the per-cause integrity lines and the
warnings. The new
matrix rows:

| Condition | Presentation | Dismissal | Exit |
| --- | --- | --- | --- |
| complete stream, unknown-type warnings only, zero results | no-results + warning overlay | no-results | 1 |
| complete stream, records skipped, usable results | browse + error overlay | browse | 0 |
| complete stream, records skipped, **no** usable results | error overlay only | exits | 2 |
| skipped record + binary exclusion → zero retained stops | error overlay only | exits | 2 |
| missing `end`, retained matches | browse + error overlay | browse | 2 |
| missing `end`, no matches | error overlay only | exits | 2 |

"Usable results" is assessed **after all filtering**: a stream whose
sole retained file was binary-excluded after a skipped record has zero
usable results and follows the record-loss fatal row, not the
no-results row. Unknown-type warnings alone still never force exit 2,
even with zero results.

## Tests

See [unit-tests.md](unit-tests.md):
`internal/searchindex/disposition_test.go` is the disposition matrix —
one embedded fixture per Issue #3 schema row (resynchronization proved
by the surrounding retained stops), one per Issue #9 integrity row, and
dedicated tests for both composite rows;
`internal/searchindex/oversized_test.go` covers the 64 MiB boundary,
discard-and-resynchronize, recoverable and anonymous path diagnostics,
the oversized-only absent file, the triple-disposition unterminated
oversized tail, and the unknown-type completion rule; the Issue #9
outcome matrix in `internal/app/outcome_test.go` gains the six
record-loss/unknown/missing-`end` rows above.
