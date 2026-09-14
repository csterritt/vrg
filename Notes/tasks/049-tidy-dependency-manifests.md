# Tasks for #49: Tidy dependency manifests — remove unused Bubbles and Lip Gloss requirements

Parent issue: #49
Parent PRD: PRD-vrg.md
**Blocked by issues**: none
**Decision**: removal selected by the product owner on 2026-09-14; `charm.land/bubbles/v2` and `charm.land/lipgloss/v2` are not used and must not be retained
**Acceptance criteria**: AC1 → Tasks 1, 3; AC2 → Tasks 1–2; AC3–AC4 → Task 2; AC5 → Tasks 2, 4
**Manual verification**: Task 4 owns the issue's manual checks.

## Tasks

### 1. Record the removal decision

**Type**: REVIEW  
**Output**: The issue and task plan record the product owner's removal decision and reject token imports whose sole purpose would be retaining Bubbles or Lip Gloss.
**Depends on**: none

Record the selected outcome: remove `charm.land/bubbles/v2` and `charm.land/lipgloss/v2` because the implementation imports neither library. The remaining TUI dependency is `charm.land/bubbletea/v2`; VRG's existing theme and rendering primitives continue to own presentation. Record every current stated-stack or prescriptive reference that must be aligned: `Notes/PRD-vrg.md` *Further Notes*, `Notes/wiki/project-overview.md`, the Scope paragraph in `Notes/wiki/AGENTS.md`, `Notes/skills/code-writing/styling-tui.md`, and the corresponding entry in `Notes/skills/AGENTS.md`. No adoption issues are created, and no token import may be added to retain a manifest entry.

---

### 2. Commit a clean tidied manifest

**Type**: CONFIG  
**Output**: Bubbles and Lip Gloss are absent from `go.mod`/`go.sum`; direct and indirect requirements match actual imports, required test dependencies are declared, `go mod tidy -diff` reports no drift, `go mod verify` passes, and build/vet/test are green.
**Depends on**: 1

Run `go mod tidy` and commit its complete manifest result. Confirm `charm.land/bubbles/v2` and `charm.land/lipgloss/v2` are absent from both `go.mod` and `go.sum`, while `charm.land/bubbletea/v2` and every real direct/test dependency remain with correct direct/indirect marking. No import may exist solely to retain a manifest entry. Verify the committed state directly with `go mod tidy -diff` as the fail-closed check — any output fails the task — plus `go mod verify`, `go build ./...`, `go vet ./...`, and `go test ./...`. Do not create `scripts/verify.sh`: Issue #50 exclusively owns that file and will adopt this already-green tidy command into the permanent gate after Issue #49 closes.

---

### 3. Align the stated-stack documentation and coding instructions

**Type**: DOCUMENT  
**Output**: Every current stack claim and prescriptive styling instruction states that VRG uses Bubble Tea without Bubbles or Lip Gloss; the wiki log records the removal decision.
**Depends on**: 2

Read and follow `Notes/wiki/wiki-rules.md` and the schema in `Notes/wiki/AGENTS.md`. Update the PRD *Further Notes* stack line, `Notes/wiki/project-overview.md`, the Scope paragraph in `Notes/wiki/AGENTS.md`, `Notes/skills/code-writing/styling-tui.md`, and the `code-writing/styling-tui` entry in `Notes/skills/AGENTS.md` so none directs implementation agents to use the removed libraries. Preserve historical audit/issue/critique references that explain the decision; only current requirements, project descriptions, and instructions must describe the selected stack. Update `Notes/wiki/index.md` only if page membership or its project-overview description changes, and append the required dated removal-decision ingest record to `Notes/wiki/log.md` without rewriting previous entries.

---

### 4. Create the manifest-hygiene walkthrough

**Type**: CODE WALKTHROUGH  
**Output**: Showboat walkthrough exists at `Notes/walkthroughs/049-04/code-walkthrough`.  
**Depends on**: 3

Use showboat, consulting `uvx showboat --help`, to create the walkthrough at exactly `Notes/walkthroughs/049-04/code-walkthrough`, with the main file named `walkthrough.md`. A lightweight walkthrough suffices for this mechanical no-behaviour-change issue: record the selected removal decision; prove Bubbles and Lip Gloss are absent from `go.mod`, `go.sum`, and current stack/instruction documents; capture empty `go mod tidy -diff` output; and record `go mod verify`, `go build ./...`, `go vet ./...`, and `go test ./...` results against the committed manifests. Reference Issue #49 and `Notes/PRD-vrg.md`, and store every generated artifact in the approved directory.

---
