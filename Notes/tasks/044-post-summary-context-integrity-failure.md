# Tasks for #44: Post-`summary` `context` records are integrity failures, not exemptions

Parent issue: #44
Parent PRD: PRD-vrg.md
**Blocked by issues**: #36
**Acceptance criteria**: AC1–AC3 → Tasks 1–2; AC4 → Tasks 1–2 (the composed diagnostic comes from Issue #36's contract)
**Manual verification**: Task 4 owns the issue's manual checks.

## Tasks

### 1. Specify `context`-after-`summary` as an integrity failure

**Type**: RED  
**Output**: Failing tests assert a `context` record after `summary` is a stream-integrity failure exactly as for any other post-summary record, pre-`summary` context stays ignored for lifecycle purposes, and the composed diagnostic reports the after-`summary` cause.  
**Depends on**: none

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Begin only after Issue #36 is complete — the outcome-composition assertion requires its composed integrity diagnostic. Correct the contradictory row in `internal/searchindex/lifecycle_test.go` (`"context after summary has no lifecycle effect"`, currently asserting `wantComplete: true`) so the lifecycle matrix asserts the PRD's summary-is-final contract: `context` after `summary` is an integrity failure, uniformly with the other post-summary rows in Issue #9's transition matrix. Keep the neighbouring rows proving `context` before `begin` and before `summary` remains ignored for match/lifecycle semantics. Add an outcome-level assertion in `internal/app` that a stream ending `summary` then `context` produces the fatal integrity outcome whose composed diagnostic identifies an after-`summary` record as the cause, per Issue #36's contract. The corrected row fails on the current code, where `Add` exempts `context` from the after-`summary` check. Keep this task test-only.

---

### 2. Remove the post-`summary` context exemption

**Type**: GREEN  
**Output**: The lifecycle and outcome tests pass; every record after `summary` — including `context` — marks stream integrity failed, while pre-`summary` context records remain ignored for match/lifecycle purposes.  
**Depends on**: 1

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

In `internal/searchindex`, remove the `context` exemption from the after-`summary` check in `Builder.Add` so every record arriving after `summary` sets the after-`summary` integrity failure — including `context` records, which keep their no-op lifecycle semantics only in pre-`summary` positions. Do not change malformed or unknown counting or which records retain stops; only the post-`summary` position is a violation, and a malformed record after `summary` keeps its dual representation from Issue #36. Run the focused tests plus `go build ./...`, `go vet ./...`, and `go test ./...`.

---

### 3. Document the summary-is-final contract

**Type**: DOCUMENT  
**Output**: Wiki documentation records that `context` is lifecycle-exempt only before `summary` and that Issue #9's context row is amended to pre-`summary` positions.  
**Depends on**: 2

Read and follow `Notes/wiki/wiki-rules.md` and the schema in `Notes/wiki/AGENTS.md`, then ingest the completed Issue #44 fix into the appropriate pages under `Notes/wiki`. Document that *any* record after `summary` — including `context` — is a stream-integrity failure per the summary-is-final contract, that pre-`summary` context records remain ignored for match/lifecycle purposes, and that Issue #9's former "context in any position" row is amended to cover only pre-`summary` positions. Cross-reference Issue #44 and the *Result index, records, and stream integrity* section of `Notes/PRD-vrg.md`, update `Notes/wiki/index.md`, and append the required dated ingest record to `Notes/wiki/log.md` without rewriting previous entries.

---

### 4. Create the summary-is-final walkthrough

**Type**: CODE WALKTHROUGH  
**Output**: Showboat walkthrough exists at `Notes/walkthroughs/044-04/code-walkthrough`.  
**Depends on**: 3

Use showboat, consulting `uvx showboat --help`, to create the walkthrough at exactly `Notes/walkthroughs/044-04/code-walkthrough`, with the main file named `walkthrough.md`. Demonstrate the corrected lifecycle matrix row and the outcome assertion, then run the issue's manual scenario: a fake rg emitting valid records, then `summary`, then a `context` record, exiting 0 → the outcome is treated as a stream-integrity failure (fatal path per the outcome matrix) whose diagnostic names the after-`summary` cause, not silently accepted. Capture commands, outputs, and exit statuses. Reference Issue #44 and `Notes/PRD-vrg.md`, and store every generated artifact in the approved directory.

---
