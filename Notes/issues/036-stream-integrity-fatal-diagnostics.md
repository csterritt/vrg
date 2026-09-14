## Issue 36: Stream-integrity fatal outcomes report complete, deterministic diagnostics

**Type**: AFK
**Blocked by**: None — can start immediately

### Parent PRD

`Notes/PRD-vrg.md` — audit source: `Notes/critiques/final-audit-vrg.md`, High finding 1

### What to build

Close the diagnostic gap in the fatal stream-integrity outcome path (`internal/app/app.go:255-280`). `DecideOutcome` already classifies incomplete stream integrity as fatal, but the fatal branch ignores `RecordLossDiagnostics` and has no integrity diagnostic of its own. The result is that an incomplete stream is explained as `ripgrep exited with code 0` when ripgrep produced no stderr, or as stderr-only when it did — the actual reason the stream is incomplete is never stated.

- Carry **structured integrity diagnostics** from `searchindex` into `OutcomeInput`: replace the bare `Integrity{Complete}` signal with a list of integrity-cause records (kind + affected raw path where applicable) so the outcome decision can see *why* the stream was marked incomplete, not only that it was.
- Cover **every** Issue 9 lifecycle-failure row, not only missing `summary`/`end`. Each integrity failure produces a stable user-facing explanation:

  | Integrity cause | Diagnostic |
  |---|---|
  | `begin(P)` while P already open | duplicate `begin` naming P |
  | `match(P)` while P not open | orphaned `match` naming P |
  | `match(P)` after a binary-excluding `end(P)` | orphaned `match` naming P (late match not retained, per the binary-exclusion precedence rule) |
  | `end(P)` while P not open | orphaned/duplicate `end` naming P |
  | P still open when the stream ends | missing `end` naming P |
  | `summary` missing | missing `summary` (covers lost or malformed completion metadata) |
  | A second `summary` | extra `summary` record |
  | Any record after `summary` (including `context` per Issue 44) | record after `summary` |
  | Trailing unterminated record | unterminated final record (also counted malformed per Issue 9) |

- **Multiplicity (deliberately uncapped)**: report each violation occurrence individually, in detection order — no aggregation, deduplication, or per-kind/per-path cap. Repeated identical violations (e.g. several orphaned `match` records for the same path) produce one line each, even at the documented large-stream scale. This unbounded one-to-one evidence is an explicit product decision; do not silently introduce a cap or summary during implementation.
- **Deterministic ordering**: violations detected mid-stream are reported in detection order. Violations discovered only at end of stream follow, in this order: missing `end` for each still-open file ordered by unsigned raw-path bytes (never map iteration order), then missing `summary`, then the trailing unterminated record.
- **Dual representation is intentional**: when one physical record is both an integrity violation and counted record loss, retain both representations in their respective components. In particular, a trailing unterminated record produces an integrity-cause line and increments the malformed aggregate; a malformed record after `summary` likewise produces the after-`summary` integrity cause and contributes to malformed record loss. Do not deduplicate one away.
- **Path escaping**: every path embedded in an integrity diagnostic passes through the single-line path escaper (`safepresentation.EscapePath`), so path newlines cannot forge diagnostic paragraph breaks.
- **Compose every diagnostic, fatal or non-fatal, in one universal component order** — identical for overlay text and post-restoration stderr replay:
  1. Process component: the collected ripgrep stderr in collection order; or, when the process **failed** (signal death or exit code other than 0/1) and supplied no explanatory stderr, a generated diagnostic naming the exit code or signal. No process-status line is ever emitted for a 0/1 exit — `ripgrep exited with code 0` is never the explanation for a failed stream.
  2. Stream-integrity cause lines, when present, in the ordering defined above.
  3. Record-loss diagnostics in Issue 37's order: malformed aggregate, oversized aggregate, then per-path oversized details.
  4. Unknown-type warnings.
  Non-fatal compositions omit the inapplicable integrity component and any absent process/record-loss component, but preserve the relative order: stderr → malformed aggregate → oversized aggregate → per-path oversized details → unknown-type warning. Split the current `recordLossDiagnostics` composition as needed so unknown types cannot remain between malformed and oversized diagnostics.
- All applicable sources compose into every branch — none suppresses the others. Fatal overlays state the integrity failure(s) alongside any real process/stderr/record-loss information, and non-fatal warning overlays use the same component ordering.

See PRD *Result index, records, and stream integrity* (integrity bullets: "SearchIndex exposes integrity diagnostics"), *Outcome and exit-status contract* (fatal rows, generated-diagnostic rule), and user stories 19–22.

### How to verify

- **Manual**: fake rg that emits valid `begin`/`match`/`end` records but terminates without `summary`, exiting 0 → fatal overlay that names the missing `summary` (not "exited with code 0"), exit 2 on dismissal; the same diagnostic appears in stderr replay. Repeat with a missing `end` for the only file, and with a damaged stream plus real stderr on the child — all causes appear together.
- **Automated**: outcome tests asserting the *text* of the composed diagnostic for **every integrity cause in the matrix above** — duplicate `begin`, orphaned `match`, `match` after binary-excluding `end`, orphaned `end`, missing `end`, missing `summary`, second `summary`, record after `summary`, trailing unterminated record — each asserting the integrity component appears in both overlay text and replay. Plus: fatal and non-fatal cases asserting the universal component order (stderr lines before integrity causes before malformed/oversized record-loss lines before unknown-type warnings); a repeated-violation case proving identical causes are emitted one-for-one without a cap; dual-representation cases proving an unterminated record and a malformed post-summary record each appear both as an integrity cause and in the malformed aggregate; a determinism case (two files missing `end` produce lines in unsigned raw-path order, stable across repeated runs); a case asserting an embedded path is escaped via `EscapePath`; and a case asserting no process-status line is emitted for a 0/1 exit with failed integrity.

### Acceptance criteria

- [ ] Given an incomplete stream (missing `summary`) with empty ripgrep stderr and exit code 0, then the fatal overlay explains the missing `summary` instead of presenting only `ripgrep exited with code 0`, and no process-status line is emitted.
- [ ] Given an incomplete stream with ripgrep stderr present and exit 0/1, then the diagnostic includes the integrity cause(s) and the stderr content, with none suppressed and no manufactured process-failure line.
- [ ] Given a process that died by signal or exited with a code other than 0/1 and supplied no stderr, then a generated diagnostic names the exit code or signal; when such a process also produced a failed stream and/or stderr, all applicable components appear.
- [ ] Given every Issue 9 lifecycle-failure row, then a stable user-facing integrity diagnostic names the cause (and the escaped path where applicable): duplicate `begin`, orphaned `match`, `match` after binary-excluding `end`, orphaned/duplicate `end`, missing `end`, missing `summary`, second `summary`, record after `summary`, and trailing unterminated record.
- [ ] Given multiple still-open files at end of stream, then their missing-`end` diagnostics appear in unsigned raw-path order and the composed output is deterministic across runs.
- [ ] Given fatal or non-fatal diagnostics with multiple applicable components, then both paths use the universal process → integrity → malformed/oversized record loss → unknown-type order; omitted components do not change the relative order of those present.
- [ ] Given repeated identical integrity violations, then every occurrence produces its own line without aggregation, deduplication, or a cap; given a record that is both an integrity violation and malformed record loss, then both the cause line and malformed aggregate contribution remain visible.
- [ ] Given a file missing its `end` event, then the retained matches remain browsable per the existing contract and the diagnostic names the missing-`end` cause with the file's escaped path.
- [ ] Given any fatal integrity outcome, then the same composed diagnostics — same lines, same order — are replayed to stderr after terminal restoration.
- [ ] Given the new outcome tests, then they assert the actual explanation content and ordering, so a regression that drops or reorders the integrity component fails the test.

### User stories addressed

- User story 19: missing completion metadata reported even when every received line is valid JSON
- User story 20: matches from files missing an `end` event retained with an incomplete-search diagnostic
- User story 22: all collected diagnostics safely replayed to stderr after terminal restoration
- User story 15: usable partial results browsable alongside an error overlay
- User story 16: fatal search with no usable results shows an error overlay and exits 2

---
