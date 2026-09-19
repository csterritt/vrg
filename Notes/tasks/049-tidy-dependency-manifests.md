# Tasks for #49: Tidy dependency manifests — remove unused Bubbles and Lip Gloss requirements

Parent issue: #49
Parent PRD: PRD-vrg.md
**Blocked by issues**: none
**Decision**: removal selected by the product owner on 2026-09-14; `charm.land/bubbles/v2` and `charm.land/lipgloss/v2` are not used and must not be retained
**Acceptance criteria**: AC1 → Task 1; AC2 → Tasks 1–2; AC3–AC4 → Task 2; AC5 → Tasks 2, 3

## Tasks

### 1. Record the removal decision

**Type**: REVIEW  
**Output**: The issue and task plan record the product owner's removal decision and reject token imports whose sole purpose would be retaining Bubbles or Lip Gloss.
**Depends on**: none

Record the selected outcome: remove `charm.land/bubbles/v2` and `charm.land/lipgloss/v2` because the implementation imports neither library. The remaining TUI dependency is `charm.land/bubbletea/v2`; VRG's existing theme and rendering primitives continue to own presentation. Record every current stated-stack or prescriptive reference that must be aligned: `Notes/PRD-vrg.md` *Further Notes*, `Notes/skills/code-writing/styling-tui.md`, and the corresponding entry in `Notes/skills/AGENTS.md`. No adoption issues are created, and no token import may be added to retain a manifest entry.

---

### 2. Commit a clean tidied manifest

**Type**: CONFIG  
**Output**: Bubbles and Lip Gloss are absent from `go.mod`/`go.sum`; direct and indirect requirements match actual imports, required test dependencies are declared, `go mod tidy -diff` reports no drift, `go mod verify` passes, and build/vet/test are green.
**Depends on**: 1

Run `go mod tidy` and commit its complete manifest result. Confirm `charm.land/bubbles/v2` and `charm.land/lipgloss/v2` are absent from both `go.mod` and `go.sum`, while `charm.land/bubbletea/v2` and every real direct/test dependency remain with correct direct/indirect marking. No import may exist solely to retain a manifest entry. Verify the committed state directly with `go mod tidy -diff` as the fail-closed check — any output fails the task — plus `go mod verify`, `go build ./...`, `go vet ./...`, and `go test ./...`. Do not create `scripts/verify.sh`: Issue #50 exclusively owns that file and will adopt this already-green tidy command into the permanent gate after Issue #49 closes.

---

### 3. Create the manifest-hygiene finish marker

**Type**: FINISH MARKER  
**Output**: Finish marker exists at `Notes/finish-markers/049-04/finish-marker.md`.  
**Depends on**: 2

Write `Task 049-04 finished successfully at <time>` to `Notes/finish-markers/049-04/finish-marker.md`, replacing `<time>` with the current UTC timestamp (for example, `date -u +"%Y-%m-%dT%H:%M:%SZ"`). Create the `Notes/finish-markers/049-04/` directory if it does not already exist.

---
