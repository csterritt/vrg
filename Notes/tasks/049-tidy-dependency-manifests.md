# Tasks for #49: Tidy dependency manifests — decide the fate of Bubbles and Lip Gloss

Parent issue: #49
Parent PRD: PRD-vrg.md
**Blocked by issues**: none
**Acceptance criteria**: AC1 → Tasks 1–3 (removal branch); AC2 → Tasks 1, 3 (adoption branch — the issue stays open until the scoped adoption issues land with tidy clean); AC3–AC5 → Task 2; AC6 → Tasks 2, 4
**Manual verification**: Task 4 owns the issue's manual checks.

## Tasks

### 1. Decide removal or adoption

**Type**: REVIEW  
**Output**: A recorded human decision selects removal or adoption for `charm.land/bubbles/v2` and `charm.land/lipgloss/v2`; under adoption it names the concrete component/API each library will own.  
**Depends on**: none

Review Issue #49, the *Further Notes* stated stack in `Notes/PRD-vrg.md`, and the current `go mod tidy -diff` output showing Bubbles and Lip Gloss unimported. Decide exactly one bounded outcome: **removal** — accept that tidy drops both modules and plan the stated-stack reference updates (`Notes/PRD-vrg.md` *Further Notes*, `Notes/wiki/project-overview.md`, and the Scope paragraph in `Notes/wiki/AGENTS.md`); or **adoption** — record in this issue or a `Notes/decisions/` entry the concrete component or API each library will own (which viewport, list, theme, overlay, or width code it replaces or implements) and the separately scoped implementation issues to create, each declaring dependencies on the audit-fix issues it touches (Issues #38–#41 and any others) so adoption work cannot invalidate in-flight render fixes. Token imports whose sole purpose is retaining a manifest entry are not adoption and are rejected under either branch. Record the decision before Task 2.

---

### 2. Commit a clean tidied manifest

**Type**: CONFIG  
**Output**: `go.mod`/`go.sum` carry correct direct/indirect marking with required test dependencies declared; `go mod tidy -diff` reports no drift, `go mod verify` passes, and `go build ./...`, `go vet ./...`, `go test ./...` are green.  
**Depends on**: 1

Apply `go mod tidy` to the committed manifests for the branch chosen in Task 1 and commit the result: under removal, Bubbles and Lip Gloss are dropped; under adoption, correct the direct/indirect marking and declare the required test dependencies now, with the tidy-clean check re-verified when the scoped adoption issues land — this issue stays open until then. No import may exist solely to retain a manifest entry. Verify the state directly in this issue with `go mod tidy -diff` (empty diff on the removal branch's final state) as the fail-closed check — a dirty diff fails the task — plus `go mod verify` and the full `go build ./...`/`go vet ./...`/`go test ./...` gates. Do not create `scripts/verify.sh`: Issue #50 exclusively owns that file and will adopt this already-green command into the permanent gate after Issue #49 closes.

---

### 3. Align the stated-stack documentation

**Type**: DOCUMENT  
**Output**: Under removal, the PRD *Further Notes* stack line, `Notes/wiki/project-overview.md`, and the `Notes/wiki/AGENTS.md` Scope paragraph match the tidied graph; under adoption, the decision record and scoped implementation issues exist.  
**Depends on**: 2

Read and follow `Notes/wiki/wiki-rules.md` and the schema in `Notes/wiki/AGENTS.md`. For the removal branch, update the PRD *Further Notes* stated-stack line ("using Bubble Tea, Bubbles, and Lip Gloss"), the pinned-stack bullet in `Notes/wiki/project-overview.md`, and the Scope paragraph in `Notes/wiki/AGENTS.md` to match the tidied graph — a finite, reviewable documentation diff — then update `Notes/wiki/index.md` and append the required dated ingest record to `Notes/wiki/log.md` without rewriting previous entries. For the adoption branch, ensure the decision record exists (in this issue or `Notes/decisions/`), create the separately scoped adoption issues declaring their audit-fix dependencies, and ingest the decision into the wiki the same way; Issue #49 then remains open until those issues land with `go mod tidy -diff` clean.

---

### 4. Create the manifest-hygiene walkthrough

**Type**: CODE WALKTHROUGH  
**Output**: Showboat walkthrough exists at `Notes/walkthroughs/049-04/code-walkthrough`.  
**Depends on**: 3

Use showboat, consulting `uvx showboat --help`, to create the walkthrough at exactly `Notes/walkthroughs/049-04/code-walkthrough`, with the main file named `walkthrough.md`. A lightweight walkthrough suffices for this mechanical no-behaviour-change issue: record the Task 1 decision, the `go mod tidy -diff` output showing no drift, `go mod verify`, and the `go build ./...`/`go vet ./...`/`go test ./...` results against the committed manifests. Reference Issue #49 and `Notes/PRD-vrg.md`, and store every generated artifact in the approved directory.

---
