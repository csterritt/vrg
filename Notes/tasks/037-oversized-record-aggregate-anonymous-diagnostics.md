# Tasks for #37: Oversized-record diagnostics — always emit the aggregate count, cover anonymous records

Parent issue: #37
Parent PRD: PRD-vrg.md
**Blocked by issues**: #36
**Acceptance criteria**: AC1–AC5 → Tasks 1–2
**Manual verification**: Task 4 owns the issue's manual checks.

## Tasks

### 1. Specify the oversized aggregate and anonymous-record diagnostics

**Type**: RED  
**Output**: Failing `internal/app` outcome tests assert the always-emitted pluralized aggregate, per-path details appended after it, the anonymous-record fatal and non-fatal cases, and mixed path recoverability.  
**Depends on**: none

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Begin only after Issue #36 is complete — both issues edit the outcome/diagnostic composition in `internal/app/app.go`, and landing #36 first avoids conflicting rewrites of the same branches. Add failing tests asserting the record-loss component emits exactly `1 oversized record skipped` for one oversized record and `N oversized records skipped` for every other count, regardless of path recovery; each distinct recoverable path's `oversized record skipped for <sanitized path>` line is appended after the aggregate exactly once. Per-path details are deduplicated by raw path, not emitted per oversized record: two oversized records naming the same recoverable path produce `2 oversized records skipped` followed by one detail line for that path. Require: an anonymous oversized record (the limit hit before `type`/`data.path` was parsed) with zero usable results produces a fatal overlay containing exactly the aggregate line — never an empty overlay — exiting 2; the same anonymous record with usable results produces a visible overlay and the aggregate in the stderr replay rather than passing silently; and a mixed-recoverability multi-record case where the aggregate reflects the total and only the distinct recoverable paths appear. Assert the aggregate sits inside Issue #36's universal component order, after the malformed aggregate and before the per-path details. Keep this task test-only.

---

### 2. Implement the oversized aggregate diagnostic

**Type**: GREEN  
**Output**: The aggregate tests pass; anonymous oversized records are never silent and the fatal overlay is never empty when records were lost.  
**Depends on**: 1

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Extend the record-loss composition reworked by Issue #36 in `internal/app/app.go` so the oversized component always leads with the pluralized aggregate built from `Index.OversizedCount()` — `1 oversized record skipped` or `N oversized records skipped` — followed by one recoverable per-path line per distinct raw path from `Index.OversizedDiagnostics()`. Deduplicate only the detail lines by raw path while preserving their deterministic first-occurrence order; never deduplicate the aggregate's per-record count. The aggregate must be emitted whenever the count is positive even when no per-path detail exists, so an anonymous oversized record always surfaces in the overlay and the replay. Keep the component ordering established by Issue #36. Run the focused tests plus `go build ./...`, `go vet ./...`, and `go test ./...`.

---

### 3. Document oversized-record diagnostics

**Type**: DOCUMENT  
**Output**: Wiki documentation records the always-emitted aggregate, the per-path details, and the anonymous-record guarantees.  
**Depends on**: 2

Read and follow `Notes/wiki/wiki-rules.md` and the schema in `Notes/wiki/AGENTS.md`, then ingest the completed Issue #37 implementation and tests into the appropriate pages under `Notes/wiki`. Document the pluralized per-record aggregate rule and its exact strings, the one-detail-line-per-distinct-raw-path deduplication rule and deterministic first-occurrence ordering, the per-path details' position after the aggregate, the anonymous-record behaviour in both fatal and non-fatal outcomes, and where the component sits in Issue #36's universal order. Cross-reference Issue #37 and the oversized bullets of *Result index, records, and stream integrity* plus the 64 MiB limit in *Resources and responsiveness* of `Notes/PRD-vrg.md`, update `Notes/wiki/index.md`, and append the required dated ingest record to `Notes/wiki/log.md` without rewriting previous entries.

---

### 4. Create the oversized-diagnostics walkthrough

**Type**: CODE WALKTHROUGH  
**Output**: Showboat walkthrough exists at `Notes/walkthroughs/037-04/code-walkthrough`.  
**Depends on**: 3

Use showboat, consulting `uvx showboat --help`, to create the walkthrough at exactly `Notes/walkthroughs/037-04/code-walkthrough`, with the main file named `walkthrough.md`. Demonstrate the outcome tests, then run the issue's manual scenarios with the fake-rg harness: one oversized record with a recoverable path followed by valid records and `summary` → browse with an overlay containing both the aggregate count and the named-path line; and an oversized record hitting the limit before its path field → the aggregate present with no path line, never an empty or absent diagnostic. Capture commands, outputs, and exit statuses. Reference Issue #37 and `Notes/PRD-vrg.md`, and store every generated artifact in the approved directory.

---
