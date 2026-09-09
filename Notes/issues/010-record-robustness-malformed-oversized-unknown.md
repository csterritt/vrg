## Issue 10: Record robustness — malformed, oversized, unknown types, missing `end`, warning-before-no-results

**Type**: AFK
**Blocked by**: Issue 9

### Parent PRD

`Notes/PRD-vrg.md`

### What to build

Make SearchIndex resilient to damaged streams and complete the remaining outcome-table rows.

- **Malformed records** (invalid JSON, invalid base64, missing/invalid `type`, and known events violating the per-record schema matrix in Issue 3 — missing/wrongly typed required fields or invalid ranges): skip and count. A trailing unterminated record is malformed **and** makes the stream incomplete (both dispositions). Lifecycle violations follow Issue 9's transition matrix: they are stream-integrity failures, not skipped-malformed counts, and vice versa; a record can be both only in the two cases those matrices mark "both" (trailing unterminated record; a malformed record appearing after `summary`).
- **Oversized records**: payload limit 64 MiB excluding the newline. Consume and discard through the next newline, count separately, continue. Do not rely on a small default line-reader limit. If `type` and `data.path` were parsed before the limit, report "oversized record skipped for <sanitized path>" in addition to the count. An oversized record with no trailing newline (the last record in the stream) is counted as **oversized** and, because the PRD's trailing-unterminated rule carries no exception for oversized records, is **also** counted as malformed for its missing termination and makes the stream incomplete: all three dispositions — the oversized count, the malformed count, and the incomplete-stream integrity failure — hold for that one record. The eventual outcome is fatal either way; the counts and diagnostics remain observable requirements.
- **Unknown string event types**: skip, count separately, report "N unrecognised record types skipped"; never changes exit status alone; cannot substitute for required known completion events. The PRD's unknown-type rule is unqualified by position, so an unknown event type appearing after `summary` is still counted and reported in the unknown-type count, **and** its after-`summary` position is separately an integrity failure per Issue 9's rule — the skip reason and the ordering violation are recorded independently, mirroring how a malformed record after `summary` is both counted malformed and an integrity failure.
- **Missing `end`**: retain the file's matches; report incomplete metadata (integrity failure per Issue 9 rules).
- Outcome rows: rg 0/1, complete stream, records skipped, no usable results → error overlay explaining record loss, exit 2 on dismissal (`q` or `Esc`). rg 0/1, complete, records skipped, usable results → browse with overlay, exit 0. Unknown-type-only warnings with zero results → warning overlay first, then no-results screen, exit 1.
- "No usable results" is assessed **after** all filtering: a stream where a malformed match was skipped and the only retained file was then binary-excluded has zero usable results and follows the record-loss fatal row (exit 2), not the no-results row.
- Add these rows to the Issue 9 outcome matrix rather than writing a parallel decision test.

See PRD *Result index…* (malformed/oversized/unknown bullets), *Outcome and exit-status contract* (rows 6–8), *Resources and responsiveness* (64 MiB bullets).

### How to verify

- **Manual**: fake rg emitting a valid `begin` for `a.txt`, a valid `match` for it, a garbage line, a `{"type":"weird"}` line, a valid `end` for `a.txt` (null `binary_offset`), then a valid `summary`, exiting 0 → browse view with overlay listing "1 malformed record skipped" and "1 unrecognised record types skipped"; `Esc` dismisses; `q` exits 0. (Without the paired `begin`/`end`, integrity fails and the correct result is exit 2.)
- **Automated** (SearchIndex): one fixture per row of Issue 3's schema matrix (each missing/wrongly-typed required field, each invalid range — `line_number` 0/negative/non-integer, `start > end`, negative `start`, `end` beyond the decoded line bytes, negative `binary_offset` — and an empty `submatches` array) and per integrity row of Issue 9's lifecycle matrix (duplicate `begin`, orphaned `match`/`end`, `match` after `end`, second `summary`, record after `summary`), asserting the matrix's disposition (skipped+counted vs integrity failure vs both) for each; a record exactly at 64 MiB accepted and one byte over skipped, followed by a valid record that is parsed (resynchronisation); oversized `match` with recoverable path names it; oversized record with limit hit before path → count only; file whose only records were oversized is absent from the list while its path appears in the diagnostic; unknown type counted, including an unknown type after `summary` (counted and reported in the unknown-type count and flagged as an after-summary integrity failure); missing `end` retains matches and flags incomplete metadata; trailing unterminated record counted malformed and marked incomplete; an oversized final record without a trailing newline asserted in all three dispositions — oversized count, malformed count, and incomplete integrity. (App, as new rows in the Issue 9 outcome matrix): unknown-type-only warning with zero results → warning overlay → no-results → 1; malformed skipped + usable results → browse+overlay → 0; malformed skipped + zero results → record-loss overlay → `q` 2 and `Esc` 2; malformed skipped + sole retained file binary-excluded → record-loss overlay → 2; missing `end` + retained matches → browse+overlay → 2 (integrity); missing `end` + no matches → overlay → 2.

### Acceptance criteria

- [ ] Given a malformed record between valid ones, then it is skipped, counted, and the remaining records are indexed.
- [ ] Given a record exceeding 64 MiB, then it is discarded through its newline, counted, and parsing resumes at the next record.
- [ ] Given an oversized final record without a trailing newline, then it is counted in both the oversized and malformed counts and the stream is marked incomplete.
- [ ] Given an unknown event type after `summary`, then it is counted and reported in the unknown-type count and its position is flagged as an after-summary integrity failure.
- [ ] Given an oversized record whose `type` and `data.path` were parsed, then the diagnostic names the sanitized path.
- [ ] Given unknown event types only and no results, then a warning overlay precedes the no-results screen and `q` exits 1.
- [ ] Given skipped records, an otherwise complete stream and no usable results, then the error overlay explains record loss and dismissal exits 2.
- [ ] Given a file missing its `end` event, then its matches are retained and an incomplete-search diagnostic is reported.
- [ ] Given a skipped record and a binary exclusion that together leave zero retained stops, then the record-loss fatal row applies (exit 2), assessed from retained stops rather than received match events.
- [ ] Given the per-record schema matrix (Issue 3) and the lifecycle transition matrix (Issue 9), then every row has a fixture asserting its stated disposition, so "malformed" and "integrity failure" are deterministic categories rather than implementation judgments.

### User stories addressed

- User story 17: malformed/oversized records skipped and counted, path named when recoverable
- User story 18: unknown event types counted and reported
- User story 20: matches without `end` retained with diagnostic
- User story 21: nonfatal diagnostics shown before the no-results screen

---
