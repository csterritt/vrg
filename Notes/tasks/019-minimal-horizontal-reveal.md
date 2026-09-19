# Tasks for #19: Minimal horizontal reveal of the first-submatch start

Parent issue: #19
Parent PRD: PRD-vrg.md
**Blocked by issues**: #18
**Acceptance criteria**: AC1–AC6 → Tasks 1–2

## Tasks

### 1. Specify minimal horizontal reveal

**Type**: RED  
**Output**: Failing Viewport tests cover right and left reveal with cluster-width arithmetic, painted-cell visibility including the clipped-blank case, oversized spans, the unpaintable-cluster geometric fallback without panning loops, and the same-file and startup triggers.  
**Depends on**: none

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Begin only after Issue #18 is complete. Add failing tests in `internal/viewport` for the Issue #19 contracts and the horizontal-reveal bullet of the Navigation, viewport, and logical anchors section of `Notes/PRD-vrg.md`. Require a hidden target to be revealed by the minimum column movement: a single-cell target right of view moves the offset to `target − (text width − 1)`, a two-cell cluster right of view to `target + cluster width − text width` so it is fully painted at the right edge, and a target left of view to the target column; an already-painted target start cell to leave the offset unchanged; visibility to mean actually rendered painted cells after grapheme clipping excluding the reserved column, so a geometrically inside position clipped to a blank by a two-cell glyph at the right edge is treated as hidden and revealed; a match wider than the text area to be revealed by its start cell alone; a cluster wider than the whole text area to set the offset to its start column, render its in-window portion as clipping blanks, be treated as geometrically revealed so repeated navigation does not loop, and still count as not visible for Issue #20's indicators; and the reveal to run at startup and on every navigation action including same-file `n`/`p`, after the Issue #18 file-change reset. Keep this task test-only.

---

### 2. Implement painted-cell reveal

**Type**: GREEN  
**Output**: Reveal tests pass; minimal horizontal movement accompanies startup and every navigation action in run-off-edge mode.  
**Depends on**: 1

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Implement the minimal horizontal reveal in `internal/viewport` and its App triggers to satisfy Task 1: painted-cell visibility over rendered cells after clipping, the right-edge rule using the target cluster's width from the shared grapheme policy, the left-edge rule, the oversized-span start-cell rule, the geometric fallback for an unpaintable cluster with no further movement, and the triggers at startup and on every navigation action. No horizontal reveal is needed in wrap mode.

---

### 3. Create the horizontal-reveal finish marker

**Type**: FINISH MARKER  
**Output**: Finish marker exists at `Notes/finish-markers/019-04/finish-marker.md`.  
**Depends on**: 2

Write `Task 019-04 finished successfully at <time>` to `Notes/finish-markers/019-04/finish-marker.md`, replacing `<time>` with the current UTC timestamp (for example, `date -u +"%Y-%m-%dT%H:%M:%SZ"`). Create the `Notes/finish-markers/019-04/` directory if it does not already exist.

---
