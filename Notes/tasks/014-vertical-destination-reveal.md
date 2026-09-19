# Tasks for #14: Vertical destination reveal — target row, no-scroll if visible, one-third placement

Parent issue: #14
Parent PRD: PRD-vrg.md
**Blocked by issues**: #13
**Acceptance criteria**: AC1–AC4 → Tasks 1–2

## Tasks

### 1. Specify vertical destination reveal

**Type**: RED  
**Output**: Failing Viewport tests cover visible-target no-scroll, one-third placement with BOF and EOF clamps, and saved-viewport versus first-visit starting points; failing App tests cover startup reveal after load and reveal's effect on saved state.  
**Depends on**: none

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Begin only after Issue #13 is complete. Add failing tests in `internal/viewport` and `internal/app` for the Issue #14 contracts and the target-row and placement bullets of the Navigation, viewport, and logical anchors section of `Notes/PRD-vrg.md`. Identify the display target as the start cell of the first submatch on the destination line from the outset, not merely a source-line ordinal. Require an already-visible target row to leave the vertical viewport unchanged; a hidden target row to land at zero-based row `floor(content height / 3)` by moving the viewport, clamped to valid top positions, with BOF and EOF content taking precedence over one-third placement; a file change to start from the saved per-file viewport on a revisit or the top of the file on a first visit — including the startup file — before applying the reveal; a reveal that moves the viewport to replace the saved vertical state; and a no-scroll reveal to leave it. Keep this task test-only.

---

### 2. Implement target-row reveal

**Type**: GREEN  
**Output**: Reveal tests pass; startup and every actual `n`/`p` transition reveal the rendered row containing the target.  
**Depends on**: 1

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Implement the reveal rules in `internal/viewport` and their App triggers to satisfy Task 1: display-target identification, visible-target no-scroll, one-third placement with clamping, the starting-viewport sequence on file change, and the saved-state replacement rules. Horizontal reveal is owned by Issue #19 and load-completion reveal by Issue #28; wire only the navigation-time and startup-after-load triggers this issue owns.

---

### 3. Create the reveal finish marker

**Type**: FINISH MARKER  
**Output**: Finish marker exists at `Notes/finish-markers/014-04/finish-marker.md`.  
**Depends on**: 2

Write `Task 014-04 finished successfully at <time>` to `Notes/finish-markers/014-04/finish-marker.md`, replacing `<time>` with the current UTC timestamp (for example, `date -u +"%Y-%m-%dT%H:%M:%SZ"`). Create the `Notes/finish-markers/014-04/` directory if it does not already exist.

---
