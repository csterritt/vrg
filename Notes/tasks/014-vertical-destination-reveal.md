# Tasks for #14: Vertical destination reveal — target row, no-scroll if visible, one-third placement

Parent issue: #14
Parent PRD: PRD-vrg.md
**Blocked by issues**: #13
**Acceptance criteria**: AC1–AC4 → Tasks 1–2
**Manual verification**: Task 4 owns the issue's manual checks.

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

### 3. Document destination reveal

**Type**: DOCUMENT  
**Output**: Wiki documentation records the target definition, no-scroll rule, one-third placement, and starting-viewport sequence.  
**Depends on**: 2

Read and follow `Notes/wiki/wiki-rules.md` and the schema in `Notes/wiki/AGENTS.md`, then ingest the completed Issue #14 implementation and tests into the appropriate pages under `Notes/wiki`. Document the display target as the first submatch's start cell, the rendered-row reveal requirement, visible-target no-scroll, one-third placement with BOF/EOF precedence, the saved-viewport and top-of-file starting points, and the saved-state replacement rules for moving versus no-scroll reveals. Cross-reference Issue #14 and the Navigation, viewport, and logical anchors and Testing Decisions sections of `Notes/PRD-vrg.md`, update `Notes/wiki/index.md`, and append the required dated ingest record to `Notes/wiki/log.md` without rewriting previous entries.

---

### 4. Create the reveal walkthrough

**Type**: CODE WALKTHROUGH  
**Output**: Showboat walkthrough exists at `Notes/walkthroughs/014-04/code-walkthrough`.  
**Depends on**: 3

Use showboat, consulting `uvx showboat --help`, to create the walkthrough at exactly `Notes/walkthroughs/014-04/code-walkthrough`, with the main file named `walkthrough.md`. Demonstrate the Viewport reveal tests for visible, hidden, BOF-clamped, and EOF-clamped targets and the App startup-reveal tests, then run the binary on a long file with matches at lines 5 and 200: `n` placing line 200 about a third down, `p` returning to line 5 near the top, and `n` between two on-screen matches not scrolling. Reference Issue #14 and `Notes/PRD-vrg.md`, and store every generated artifact in the approved directory.

---
