## Issue 10: Record robustness — malformed, oversized, unknown types, missing `end`, warning-before-no-results

**Type**: AFK
**Blocked by**: Issue 9

### Parent PRD

`Notes/PRD-vrg.md`

### What to build

Make SearchIndex resilient to damaged streams and complete the remaining outcome-table rows.

- **Malformed records** (invalid JSON, invalid base64, missing/invalid `type`, known events with missing/wrongly typed required fields or invalid ranges): skip and count. A trailing unterminated record is malformed and makes the stream incomplete.
- **Oversized records**: payload limit 64 MiB excluding the newline. Consume and discard through the next newline, count separately, continue. Do not rely on a small default line-reader limit. If `type` and `data.path` were parsed before the limit, report "oversized record skipped for <sanitized path>" in addition to the count.
- **Unknown string event types**: skip, count separately, report "N unrecognised record types skipped"; never changes exit status alone; cannot substitute for required known completion events.
- **Missing `end`**: retain the file's matches; report incomplete metadata (integrity failure per Issue 9 rules).
- Outcome rows: rg 0/1, complete stream, records skipped, no usable results → error overlay explaining record loss, exit 2 on dismissal. rg 0/1, complete, records skipped, usable results → browse with overlay, exit 0. Unknown-type-only warnings with zero results → warning overlay first, then no-results screen, exit 1.

See PRD *Result index…* (malformed/oversized/unknown bullets), *Outcome and exit-status contract* (rows 6–8), *Resources and responsiveness* (64 MiB bullets).

### How to verify

- **Manual**: fake rg emitting a valid match, a garbage line, a `{"type":"weird"}` line, then a valid summary → browse view with overlay listing "1 malformed record skipped" and "1 unrecognised record types skipped"; `q` exits 0.
- **Automated** (SearchIndex): each malformed category; invalid range; a record exactly at 64 MiB accepted and one byte over skipped, followed by a valid record that is parsed (resynchronisation); oversized `match` with recoverable path names it; oversized record with limit hit before path → count only; file whose only records were oversized is absent from the list while its path appears in the diagnostic; unknown type counted; missing `end` retains matches and flags incomplete metadata; trailing unterminated record. (App): outcome rows above including warning → no-results transition and record-loss fatal overlay → exit 2.

### Acceptance criteria

- [ ] Given a malformed record between valid ones, then it is skipped, counted, and the remaining records are indexed.
- [ ] Given a record exceeding 64 MiB, then it is discarded through its newline, counted, and parsing resumes at the next record.
- [ ] Given an oversized record whose `type` and `data.path` were parsed, then the diagnostic names the sanitized path.
- [ ] Given unknown event types only and no results, then a warning overlay precedes the no-results screen and `q` exits 1.
- [ ] Given skipped records, an otherwise complete stream and no usable results, then the error overlay explains record loss and dismissal exits 2.
- [ ] Given a file missing its `end` event, then its matches are retained and an incomplete-search diagnostic is reported.

### User stories addressed

- User story 17: malformed/oversized records skipped and counted, path named when recoverable
- User story 18: unknown event types counted and reported
- User story 20: matches without `end` retained with diagnostic
- User story 21: nonfatal diagnostics shown before the no-results screen

---
