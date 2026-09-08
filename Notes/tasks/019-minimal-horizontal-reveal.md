# Tasks for #19: Minimal horizontal reveal of the first-submatch start

Parent issue: #19
Parent PRD: PRD-vrg.md
**Blocked by issues**: #18
**Acceptance criteria**: AC1–AC6 → Tasks 1–2
**Manual verification**: Task 4 owns the issue's manual checks.

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

### 3. Document minimal horizontal reveal

**Type**: DOCUMENT  
**Output**: Wiki documentation records the painted-cell visibility rule, the reveal arithmetic, and the geometric fallback.  
**Depends on**: 2

Read and follow `Notes/wiki/wiki-rules.md` and the schema in `Notes/wiki/AGENTS.md`, then ingest the completed Issue #19 implementation and tests into the appropriate pages under `Notes/wiki`. Document painted-cell visibility as the reveal criterion, the right-edge and left-edge offset rules with cluster widths, the oversized-match start-cell rule, the geometric fallback for a cluster wider than the text area with its no-loop guarantee and indicator interplay, and the startup and per-navigation triggers after the file-change reset. Cross-reference Issue #19 and the Navigation, viewport, and logical anchors and Layout and indicators sections of `Notes/PRD-vrg.md`, update `Notes/wiki/index.md`, and append the required dated ingest record to `Notes/wiki/log.md` without rewriting previous entries.

---

### 4. Create the horizontal-reveal walkthrough

**Type**: CODE WALKTHROUGH  
**Output**: Showboat walkthrough exists at `Notes/walkthroughs/019-04/code-walkthrough`.  
**Depends on**: 3

Use showboat, consulting `uvx showboat --help`, to create the walkthrough at exactly `Notes/walkthroughs/019-04/code-walkthrough`, with the main file named `walkthrough.md`. Demonstrate the reveal arithmetic tests including the clipped-blank and oversized-cluster cases, then run the binary in run-off-edge mode on a file with matches at columns 5 and 300: `n` to the far match scrolling right just enough to show its start at the right edge, `n` back scrolling left just enough, a visible match causing no movement, and a CJK match at column 300 showing both cells of its first glyph painted. Reference Issue #19 and `Notes/PRD-vrg.md`, and store every generated artifact in the approved directory.

---
