# Record robustness (Issue #10)

Robust handling of malformed, oversized, and unknown-type records in
ripgrep JSON streams, delivered by
[Issue #10](../issues/010-record-robustness-malformed-oversized-unknown.md).
The collection pipeline distinguishes five deterministic categories —
malformed records, oversized records, unknown event types,
stream-integrity failures, and process failures — and a sixth
post-filtering assessment: usable search results after filtering.
Relevant PRD sections: *Result index, records, and stream integrity*,
*Outcome and exit-status contract*, and *Resources and responsiveness
(64 MiB record limit)*. See also
[search-collection-path](search-collection-path.md) and
[outcome-contract](outcome-contract.md).

## Disposition categories

Issue #10 keeps five categories separate so the app can report and
act on them independently:

- **Malformed records** — invalid JSON, invalid base64, missing or
  non-string `type`, or a known event with an invalid per-record schema
  (missing required fields, invalid line numbers, empty submatches,
  invalid submatch ranges, malformed `end.binary_offset`, malformed
  summary data). Skipped and counted via `Index.MalformedCount()`.
- **Oversized records** — records exceeding the 64 MiB payload limit.
  Discarded through the next newline and counted via
  `Index.OversizedCount()`. Sanitized per-record path diagnostics are
  available via `Index.OversizedDiagnostics()` when path recovery
  succeeds.
- **Unknown event types** — records with an unrecognised string `type`.
  Counted separately from malformed via `Index.UnknownCount()`. Unknown
  types never substitute for required completion events and never
  independently change exit status.
- **Stream-integrity failures** — lifecycle rule violations (duplicate
  `begin`, orphaned `match`/`end`, `match` after `end`, second
  `summary`, records after `summary`, files still open at stream end,
  trailing unterminated records). Tracked by `Index.Integrity()` and
  kept separate from malformed counts except where the matrices mark
  both.
- **Process failures** — fatal exit codes (other than 0 or 1) and
  signal death. Tracked by `ProcessResult` and assessed independently
  from stream integrity.

Malformed counts are kept separate from integrity failures except in
the two cases the Issue #3 schema matrix and Issue #9 lifecycle matrix
mark both:

- A trailing unterminated record (ordinary or oversized) is counted
  malformed **and** marks the stream incomplete.
- A malformed record arriving after a valid `summary` is counted
  malformed **and** flagged as an after-`summary` integrity failure.

Unknown string event types are never treated as malformed. An unknown
type after `summary` is counted unknown and its after-`summary`
position is separately flagged as an integrity failure.

## Per-record schema matrix

`Builder.Add` validates each record's schema before applying lifecycle
rules. Per-record validation order for `end` records: decode and
validate the path, require `binary_offset`, validate a non-null
`binary_offset` as a non-negative integer, then check lifecycle open
state and apply binary exclusion. This ordering ensures an invalid
`end.binary_offset` is classified as malformed rather than as an
orphaned lifecycle violation.

Malformed inputs include:

- invalid JSON;
- missing or empty `type`;
- non-string `type`;
- known event records with invalid schema (missing required fields,
  invalid line numbers, empty submatches, invalid submatch ranges,
  malformed `end.binary_offset`, malformed summary data);
- invalid base64.

A malformed record between valid ones is skipped, counted, and
followed by correct indexing of the remaining records. A safely
skipped match record does not by itself make otherwise intact
lifecycle metadata incomplete.

## 64 MiB record limit

`Builder.ReadFrom` reads newline-delimited JSON records with an
explicit bounded reader (`bufio.NewReaderSize` with
`MaxRecordSize+1`). The 64 MiB payload limit excludes the newline
delimiter:

- A record exactly at the 64 MiB limit is accepted.
- A record one byte over is discarded as oversized.
- An oversized record is consumed and discarded through its next
  newline, so parsing resynchronizes on the following record. Oversized
  records never permanently desynchronize the stream.
- A trailing oversized record without a newline is counted oversized
  **and also counted malformed** for its missing termination, and marks
  the stream incomplete — all three dispositions asserted (oversized
  count, malformed count, incomplete integrity), matching the PRD's
  unqualified trailing-unterminated rule.
- An ordinary trailing record without a newline is counted malformed
  and marks the stream incomplete (the Issue #9 rule, unchanged).

Scanner/reader behavior never double-counts the same record: an
oversized record that fits in the buffer is counted once as oversized;
a trailing oversized record without a newline increments the
oversized count once and the malformed count once.

## Oversized-record diagnostics

Best-effort path recovery produces a sanitized per-record diagnostic
when the `type` and `data.path` fields were parsed before the limit
was reached. The diagnostic is of the form "oversized record skipped
for <sanitized path>", where the path is sanitized through
`safepresentation.EscapePath` (the Issue #6 utility). Records where
the limit was reached before path recovery produce no diagnostic
entry; only the count is reported.

A file whose only records were oversized is absent from the file list
while its path appears in the diagnostic.

## Unknown event types

Unknown string event types are counted separately from malformed
records and never independently change exit status. An unknown type
after `summary` is counted unknown and its after-`summary` position is
separately flagged as an integrity failure. Unknown types never
substitute for required completion events: a stream with only unknown
types and no `summary` is incomplete.

## Record-loss outcome rows

`RecordLoss{Malformed, Unknown, Oversized}` is part of the outcome
decision. `DecideOutcome` assesses "no usable results" after all
filtering (binary exclusion and record loss), so a stream whose sole
retained file was binary-excluded after a skipped record follows the
record-loss fatal row rather than the no-results row.

The new outcome-matrix rows (extending the Issue #9 matrix):

| Process | Integrity | Usable results | Record loss | Initial state | Overlay | Fatal | Exit |
|---------|-----------|----------------|-------------|---------------|---------|-------|------|
| 0       | complete  | >0             | malformed   | browse        | warning | no    | 0    |
| 0       | complete  | 0              | malformed   | no-results    | error   | yes   | 2    |
| 0       | complete  | 0              | oversized   | no-results    | error   | yes   | 2    |
| 0       | complete  | 0              | unknown     | no-results    | warning | no    | 1    |
| 0       | complete  | 0              | malformed + binary exclusion | no-results | error | yes | 2 |

Record-loss diagnostics are combined with stderr diagnostics for the
overlay text. The `recordLossDiagnostics` helper formats the malformed
count ("N malformed record(s) skipped"), the unknown count ("N
unrecognised record types skipped"), and the per-record oversized path
diagnostics.

## Missing `end` retention

A file still open at `Build()` is marked incomplete and makes the
stream incomplete. Existing matches for the open file are retained
with `Stop.Incomplete = true`. The outcome matrix treats missing
`end` with retained matches as a browse-with-overlay fatal outcome
(exit 2); missing `end` with no matches as an overlay fatal outcome
(exit 2).
