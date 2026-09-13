## Issue 37: Oversized-record diagnostics — always emit the aggregate count, cover anonymous records

**Type**: AFK
**Blocked by**: Issue 36 — both issues edit the outcome/diagnostic composition in `internal/app/app.go`; landing 36 first avoids conflicting rewrites of the same branches

### Parent PRD

`Notes/PRD-vrg.md` — audit source: `Notes/critiques/final-audit-vrg.md`, High finding 2

### What to build

Fix the record-loss diagnostic composition (`internal/app/app.go:283-304`, `316-332`). `recordLossDiagnostics` currently emits malformed and unknown-type counts plus only *per-path* oversized messages; it never emits the required aggregate oversized count. Two consequences:

- An oversized record whose path could not be recovered (limit hit before `type`/`data.path` parsed) produces no oversized diagnostic at all.
- If path recovery fails on an oversized record and no usable results remain, the fatal outcome renders an empty overlay; if usable results do remain, the skipped record can produce no overlay or replay diagnostic — the loss is silent.

Always emit `N oversized record(s) skipped` as the aggregate diagnostic, then append recoverable per-path details (`oversized record skipped for <sanitized path>`) when known. The aggregate must appear even when no per-path detail is recoverable, so an anonymous oversized record is never invisible.

See PRD *Result index, records, and stream integrity* (oversized bullets) and *Resources and responsiveness* (64 MiB limit).

### How to verify

- **Manual**: fake rg emitting one oversized record (payload over 64 MiB) whose path is recoverable, then valid records for another file, then `summary` → browse view with overlay containing both the aggregate count and the named-path line. Repeat with an oversized record that hits the limit before its path field → aggregate count present with no path line, never an empty or absent diagnostic.
- **Automated**: outcome tests for anonymous oversized records in both directions — (a) anonymous oversized + zero usable results → fatal overlay that still contains `1 oversized record(s) skipped`, exit 2; (b) anonymous oversized + usable results → browse with overlay showing the aggregate, exit 0; plus a mixed case asserting aggregate count and per-path lines coexist.

### Acceptance criteria

- [ ] Given any oversized record, then the diagnostics include the aggregate `N oversized record(s) skipped` count regardless of whether a path was recovered.
- [ ] Given an oversized record with a recoverable path, then the per-path line naming the sanitized path is appended after the aggregate count.
- [ ] Given an anonymous oversized record and no usable results, then the fatal overlay contains the aggregate oversized diagnostic and is never empty.
- [ ] Given an anonymous oversized record and usable results, then the record loss produces a visible overlay and stderr replay diagnostic rather than passing silently.
- [ ] Given multiple oversized records with mixed path recoverability, then the aggregate reflects the total and each recoverable path is named once.

### User stories addressed

- User story 17: malformed and oversized records skipped and counted, path named when recoverable
- User story 21: nonfatal diagnostics with no results shown before the no-results screen
- User story 22: all collected diagnostics safely replayed to stderr after terminal restoration

---
