## Issue 36: Stream-integrity fatal outcomes report complete diagnostics

**Type**: AFK
**Blocked by**: None — can start immediately

### Parent PRD

`Notes/PRD-vrg.md` — audit source: `Notes/critiques/final-audit-vrg.md`, High finding 1

### What to build

Close the diagnostic gap in the fatal stream-integrity outcome path (`internal/app/app.go:255-280`). `DecideOutcome` already classifies incomplete stream integrity as fatal, but the fatal branch ignores `RecordLossDiagnostics` and has no integrity diagnostic of its own. The result is that an incomplete stream is explained as `ripgrep exited with code 0` when ripgrep produced no stderr, or as stderr-only when it did — the actual reason (missing `summary`, missing `end`, or other lifecycle violation) is never stated.

- Carry structured integrity diagnostics from `searchindex` into `OutcomeInput` so the outcome decision can see *why* the stream was marked incomplete, not only that it was.
- Compose diagnostics in every fatal branch from all applicable sources — process exit status, stream-integrity failures, collected ripgrep stderr, and record-loss notes — rather than letting one source suppress the others.
- The overlay and the exit replay must both state the integrity failure (e.g. missing `summary`, file missing `end`) alongside any process/stderr information.

See PRD *Result index, records, and stream integrity*, *Outcome and exit-status contract* (fatal rows), and user stories 19–22.

### How to verify

- **Manual**: fake rg that emits valid `begin`/`match`/`end` records but terminates without `summary`, exiting 0 → fatal overlay that names the incomplete-metadata cause (not merely "exited with code 0"), exit 2 on dismissal; the same diagnostic appears in stderr replay. Repeat with a missing `end` for the only file and with a damaged stream plus real stderr on the child — all causes appear together.
- **Automated**: outcome tests that assert the *text* of the composed diagnostic for each fatal integrity row — no-stderr incomplete stream, stderr-plus-incomplete stream, missing `end`, record after `summary` — asserting that integrity, process, stderr, and record-loss components each appear, rather than asserting only overlay kind and exit status.

### Acceptance criteria

- [ ] Given an incomplete stream (missing `summary`) with empty ripgrep stderr and exit code 0, then the fatal overlay explains the missing completion metadata instead of presenting only `ripgrep exited with code 0`.
- [ ] Given an incomplete stream with ripgrep stderr present, then the diagnostic includes the integrity failure and the stderr content and the process status, with none suppressed.
- [ ] Given a fatal stream-integrity outcome where records were also skipped, then the record-loss diagnostics are included in the composed explanation.
- [ ] Given a file missing its `end` event, then the retained matches remain browsable per the existing contract and the diagnostic names the incomplete-metadata cause.
- [ ] Given any fatal integrity outcome, then the same composed diagnostics are replayed to stderr after terminal restoration.
- [ ] Given the new outcome tests, then they assert the actual explanation content, so a regression that drops the integrity component fails the test.

### User stories addressed

- User story 19: missing completion metadata reported even when every received line is valid JSON
- User story 20: matches from files missing an `end` event retained with an incomplete-search diagnostic
- User story 22: all collected diagnostics safely replayed to stderr after terminal restoration
- User story 15: usable partial results browsable alongside an error overlay
- User story 16: fatal search with no usable results shows an error overlay and exits 2

---
