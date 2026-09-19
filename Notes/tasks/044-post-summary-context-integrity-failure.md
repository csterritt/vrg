# Tasks for #44: Post-`summary` `context` records are integrity failures, not exemptions

Parent issue: #44
Parent PRD: PRD-vrg.md
**Blocked by issues**: #36
**Acceptance criteria**: AC1–AC4 → Task 1

## Tasks

### 1. Add dedicated `context`-after-`summary` regression coverage

**Type**: REFACTOR
**Output**: Focused lifecycle, structured-cause, and outcome tests pass for post-`summary` `context`, while neighbouring pre-`summary` context rows remain green.
**Depends on**: none

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Begin only after Issue #36 is complete. Issue #36 owns removing the `context` exemption in `Builder.Add` and correcting the contradictory `"context after summary has no lifecycle effect"` row in `internal/searchindex/lifecycle_test.go`; this task begins from that green parser behavior and must not repeat or claim ownership of the parser change. Strengthen the green behavioral safety net with a dedicated `internal/searchindex` matrix row asserting that `summary` followed by `context` yields exactly the single structured `record after summary` cause, plus neighbouring rows proving `context` before `begin` and before `summary` remains ignored for match/lifecycle semantics. Add an `internal/app` outcome assertion that the same stream produces the fatal integrity outcome whose complete composed diagnostic identifies exactly the after-`summary` cause and is retained for stderr replay. Do not add a second production-code change or restore a RED premise that Issue #36 has already resolved. Run the existing focused safety net before editing, then run the expanded focused tests, `go build ./...`, `go vet ./...`, and `go test ./...`.

---

### 2. Create the summary-is-final finish marker

**Type**: FINISH MARKER
**Output**: Finish marker exists at `Notes/finish-markers/044-03/finish-marker.md`.
**Depends on**: 1

Write `Task 044-03 finished successfully at <time>` to `Notes/finish-markers/044-03/finish-marker.md`, replacing `<time>` with the current UTC timestamp (for example, `date -u +"%Y-%m-%dT%H:%M:%SZ"`). Create the `Notes/finish-markers/044-03/` directory if it does not already exist.

---
