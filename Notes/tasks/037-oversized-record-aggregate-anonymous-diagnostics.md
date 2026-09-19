# Tasks for #37: Oversized-record diagnostics — always emit the aggregate count, cover anonymous records

Parent issue: #37
Parent PRD: PRD-vrg.md
**Blocked by issues**: #36
**Acceptance criteria**: AC1–AC5 → Tasks 1–2

## Tasks

### 1. Specify the oversized aggregate and anonymous-record diagnostics

**Type**: RED  
**Output**: Failing `internal/app` outcome tests assert the always-emitted pluralized aggregate, per-path details appended after it, the anonymous-record fatal and non-fatal cases, and mixed path recoverability.  
**Depends on**: none

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Begin only after Issue #36 is complete — both issues edit the outcome/diagnostic composition in `internal/app/app.go`, and landing #36 first avoids conflicting rewrites of the same branches. Add failing tests asserting the record-loss component emits exactly `1 oversized record skipped` for one oversized record and `N oversized records skipped` for every other count, regardless of path recovery; each distinct recoverable path's `oversized record skipped for <sanitized path>` line is appended after the aggregate exactly once. Per-path details are deduplicated by raw path, not emitted per oversized record: two oversized records naming the same recoverable path produce `2 oversized records skipped` followed by one detail line for that path. Require: an anonymous oversized record (the limit hit before `type`/`data.path` was parsed) with zero usable results produces a fatal overlay containing exactly the aggregate line — never an empty overlay — exiting 2; the same anonymous record with usable results produces a visible overlay and the aggregate in the stderr replay rather than passing silently; and a mixed-recoverability multi-record case where the aggregate reflects the total and only the distinct recoverable paths appear. Assert the aggregate sits inside Issue #36's universal component order, after the malformed aggregate and before the per-path details. Update Issue #36's exact post-`summary` oversized fixture from `[record after summary, oversized record skipped for <escaped Q>]` to exactly `[record after summary, 1 oversized record skipped, oversized record skipped for <escaped Q>]`, retaining the same sole integrity cause and oversized count. Keep this task test-only.

---

### 2. Implement the oversized aggregate diagnostic

**Type**: GREEN  
**Output**: The aggregate tests pass; anonymous oversized records are never silent and the fatal overlay is never empty when records were lost.  
**Depends on**: 1

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Extend the record-loss composition reworked by Issue #36 in `internal/app/app.go` so the oversized component always leads with the pluralized aggregate built from `Index.OversizedCount()` — `1 oversized record skipped` or `N oversized records skipped` — followed by one recoverable per-path line per distinct raw path from `Index.OversizedDiagnostics()`. Deduplicate only the detail lines by raw path while preserving their deterministic first-occurrence order; never deduplicate the aggregate's per-record count. The aggregate must be emitted whenever the count is positive even when no per-path detail exists, so an anonymous oversized record always surfaces in the overlay and the replay. Keep the component ordering established by Issue #36. Run the focused tests plus `go build ./...`, `go vet ./...`, and `go test ./...`.

---

### 3. Create the oversized-diagnostics finish marker

**Type**: FINISH MARKER  
**Output**: Finish marker exists at `Notes/finish-markers/037-04/finish-marker.md`.  
**Depends on**: 2

Write `Task 037-04 finished successfully at <time>` to `Notes/finish-markers/037-04/finish-marker.md`, replacing `<time>` with the current UTC timestamp (for example, `date -u +"%Y-%m-%dT%H:%M:%SZ"`). Create the `Notes/finish-markers/037-04/` directory if it does not already exist.

---
