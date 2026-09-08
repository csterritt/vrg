# Tasks for #16: Wrap mode (default on), `w` toggle, grapheme-boundary wrapping, tab stops

Parent issue: #16
Parent PRD: PRD-vrg.md
**Blocked by issues**: #14
**Acceptance criteria**: AC1–AC5 → Tasks 1–2
**Manual verification**: Task 4 owns the issue's manual checks.

## Tasks

### 1. Specify wrap mode, grapheme boundaries, and tab stops

**Type**: RED  
**Output**: Failing Viewport and FileBuffer tests cover wrap row counts for ASCII, wide, and combining content, tab stops, wrapped-target reveal, toggle row-model changes, continuation gutters, the swappable row model, and the visible-rows render-cost guard.  
**Depends on**: none

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Begin only after Issue #14 is complete. Add failing tests in `internal/viewport` and `internal/filebuffer` for the Issue #16 contracts and the grapheme, tab, and wrap bullets of the Text, graphemes, and safe presentation section of `Notes/PRD-vrg.md`. Require wrapping on initially with `w` toggling to run-off-edge mode and back; text width derived as panel width minus gutter minus the reserved indicator width (zero in wrap mode, one in run-off-edge, the column reserved now and populated by Issue #20); wrapping only at grapheme-cluster boundaries with a two-cell cluster that cannot fit in a row's remaining cells moving to the next row and leaving a blank; continuation rows with a blank gutter aligned to the first row's text; tabs expanding to the next multiple of 8 source-display columns independent of gutter and horizontal pan, replacing Issue #5's provisional `→` placeholder and now asserting the tab cell positions Issue #5 deferred; one shared grapheme segmentation and cell-width policy with FileBuffer exposing cluster boundaries and widths that Viewport consumes without re-deriving; scroll units remaining rendered rows; the Issue #14 target reveal finding the row of a wrapped line containing the match start, including a source line taller than several screens landing at `floor(h / 3)`; and the prepared row model built as a swappable value keyed by (path, content revision, text width, wrap mode), with a render-cost guard proving `View()` never invokes the wrapper for lines outside the visible rows via a counting fake. Keep this task test-only.

---

### 2. Implement the shared grapheme policy and wrap row model

**Type**: GREEN  
**Output**: Wrap, tab, reveal, and row-model tests pass with one grapheme policy shared by FileBuffer and Viewport.  
**Depends on**: 1

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Implement the shared grapheme policy in `internal/filebuffer` with its cluster boundaries and cell widths, the wrap and run-off-edge row-model construction in `internal/viewport` for the current text width and mode, structural tab expansion, the reserved indicator width, and the `w` toggle. Build the row model as a value swappable at load, toggle, or resize time and keyed exactly as Task 1 requires so Issue #17 can move preparation off the UI update path without restructuring it; synchronous preparation at those moments is acceptable in this issue.

---

### 3. Document wrap mode and the grapheme policy

**Type**: DOCUMENT  
**Output**: Wiki documentation records wrap and run-off-edge modes, the grapheme policy, tab stops, the row model, and the reserved indicator width.  
**Depends on**: 2

Read and follow `Notes/wiki/wiki-rules.md` and the schema in `Notes/wiki/AGENTS.md`, then ingest the completed Issue #16 implementation and tests into the appropriate pages under `Notes/wiki`. Document wrapping on by default with the `w` toggle, grapheme-boundary wrapping with the blank-cell rule for unclusterable wide glyphs, the single shared segmentation and cell-width policy with FileBuffer as its source, eight-column tab stops independent of gutter and pan, continuation-row gutters, the reserved right-indicator width of zero or one, and the prepared row model keyed by path, content revision, text width, and wrap mode. Cross-reference Issue #16 and the Text, graphemes, and safe presentation and Layout and indicators sections of `Notes/PRD-vrg.md`, update `Notes/wiki/index.md`, and append the required dated ingest record to `Notes/wiki/log.md` without rewriting previous entries.

---

### 4. Create the wrap-mode walkthrough

**Type**: CODE WALKTHROUGH  
**Output**: Showboat walkthrough exists at `Notes/walkthroughs/016-04/code-walkthrough`.  
**Depends on**: 3

Use showboat, consulting `uvx showboat --help`, to create the walkthrough at exactly `Notes/walkthroughs/016-04/code-walkthrough`, with the main file named `walkthrough.md`. Demonstrate the wrap row-count, tab, reveal, and render-cost tests, then run the binary on a file with a 500-character line: wrapped rows with blank continuation gutters, `w` showing it as one clipped row, tabs aligned to eight-column stops, and a match near the end of the long line revealed on its own row after `n`. Reference Issue #16 and `Notes/PRD-vrg.md`, and store every generated artifact in the approved directory.

---
