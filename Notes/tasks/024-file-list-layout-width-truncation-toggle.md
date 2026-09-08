# Tasks for #24: File-list layout — width formula, `…` truncation, hide/show

Parent issue: #24
Parent PRD: PRD-vrg.md
**Blocked by issues**: #17
**Acceptance criteria**: AC1–AC8 → Tasks 1–2
**Manual verification**: Task 4 owns the issue's manual checks.

## Tasks

### 1. Specify the file-list layout

**Type**: RED  
**Output**: Failing layout tests cover each width term winning, 40% floor rounding, gutter growth, the ten-cell minimum, zero-width preference retention, and grapheme-safe truncation with the synthetic status slot; failing App tests cover toggles, auto-scroll, anchor preservation through relayout, and the visible-entries render-cost guard.  
**Depends on**: none

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Begin only after Issue #17 is complete. Add failing tests in `internal/app` and its layout seam for the Issue #24 contracts and the width bullets of the Layout and indicators section of `Notes/PRD-vrg.md`. Require the visible list width to be the nonnegative minimum of the longest sanitized path width plus two, `floor(0.40 × terminal width)`, and terminal width minus (gutter width + 10 + reserved indicator width), recomputed after loading changes the gutter and after mode and size changes; `left`/`tab` to hide and `right`/`shift+tab` to show the list, initially shown; a computed zero width to draw no cells while preserving the visibility preference with no automatic toggling (for example W=20, gutter=9, indicator=1 → 0); paths wider than their cells to be left-truncated with a leading `…` without splitting graphemes; the filename row to provide a buffer-status note slot with the path truncating to make room for a synthetic status string (the real notes are owned by Issues #26, #29, and #30); list scrolling to keep the active entry visible; every text-width change from hide/show, gutter growth, or mode and size changes to go through the Issue #17 prepared-layout path preserving the logical reading anchor, with the top mid-way through a wrapped line surviving `tab`/`shift+tab` and gutter-growth round trips; and a counting fake proving rendering formats only visible entries. Keep this task test-only.

---

### 2. Implement the list layout and toggles

**Type**: GREEN  
**Output**: Layout, truncation, toggle, anchor, and render-cost tests pass; pathological dimensions never produce negative widths.  
**Depends on**: 1

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Implement the width formula with its recomputation triggers, grapheme-safe left truncation, the hide/show toggles with preference retention through zero-width allocation, the filename-row status slot with path truncation, list auto-scroll to the active entry, and the text-width changes routed through the Issue #17 prepared-layout path so anchors survive relayout. Panel text width follows from list width, gutter, and the reserved column without ever going negative.

---

### 3. Document the file-list layout

**Type**: DOCUMENT  
**Output**: Wiki documentation records the width formula, truncation, toggles, the status slot, and anchor preservation through relayout.  
**Depends on**: 2

Read and follow `Notes/wiki/wiki-rules.md` and the schema in `Notes/wiki/AGENTS.md`, then ingest the completed Issue #24 implementation and tests into the appropriate pages under `Notes/wiki`. Document the three-term width formula with its floor rounding and recomputation triggers, zero-width allocation with preference retention, grapheme-safe `…` truncation, the hide/show toggles, list auto-scroll, the filename-row status slot with its synthetic testing basis and the issues that will fill it, and anchor preservation through every relayout cause. Cross-reference Issue #24 and the Layout and indicators and File list and layout sections of `Notes/PRD-vrg.md`, update `Notes/wiki/index.md`, and append the required dated ingest record to `Notes/wiki/log.md` without rewriting previous entries.

---

### 4. Create the file-list walkthrough

**Type**: CODE WALKTHROUGH  
**Output**: Showboat walkthrough exists at `Notes/walkthroughs/024-04/code-walkthrough`.  
**Depends on**: 3

Use showboat, consulting `uvx showboat --help`, to create the walkthrough at exactly `Notes/walkthroughs/024-04/code-walkthrough`, with the main file named `walkthrough.md`. Demonstrate the layout-function and anchor-relayout tests, then run the manual cases in an 80-column terminal with long paths: the list at most 32 columns with `…`-prefixed basenames, loading a file with five-digit line numbers narrowing the list, scrolling partway into a wrapped line and pressing `tab` then `shift+tab` with the same text remaining at the top, and shrinking to 30 columns showing a constrained but present list that widens back on enlargement. Reference Issue #24 and `Notes/PRD-vrg.md`, and store every generated artifact in the approved directory.

---
