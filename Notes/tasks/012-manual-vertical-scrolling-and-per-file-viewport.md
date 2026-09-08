# Tasks for #12: Manual vertical scrolling, clamping, per-file saved viewport

Parent issue: #12
Parent PRD: PRD-vrg.md
**Blocked by issues**: #5
**Acceptance criteria**: AC1–AC5 → Tasks 1–2
**Manual verification**: Task 4 owns the issue's manual checks.

## Tasks

### 1. Specify vertical scrolling and per-file viewport state

**Type**: RED  
**Output**: Failing Viewport tests cover each scroll unit, both clamps, odd-height half-pages, and files shorter than, equal to, and longer than the viewport; failing App tests cover the placeholder no-op, per-file state, and the visible-rows-only render-cost guard.  
**Depends on**: none

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Begin only after Issue #5 is complete. Add failing tests in `internal/viewport` and `internal/app` for the Issue #12 contracts and the scroll-unit bullet of the Navigation, viewport, and logical anchors section of `Notes/PRD-vrg.md`. Require `up`/`down` to move one rendered row, `u`/`d` to move `max(1, floor(height / 2))`, and `page up`/`page down` a full page of the content height (panel height minus the filename row); require clamping to valid content with top row ≥ 0, no avoidable blank rows below EOF, and unused rows left naturally for files shorter than the viewport; require scroll keys on a "Loading…" placeholder to be no-ops; require per-file vertical viewport state saved in the model for later revisits; and require a render-cost guard proving a frame render queries the row provider only for the visible row range via a counting fake rather than scanning the full buffer. Keep this task test-only.

---

### 2. Implement scrolling and prepared-row rendering

**Type**: GREEN  
**Output**: Scroll, clamp, placeholder, per-file-state, and render-cost tests pass.  
**Depends on**: 1

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Implement the Viewport scroll units and clamps over prepared row data, the per-file saved vertical state in the App model, and the render path that slices prepared rows for the visible range only. Prepared row data is built when a load completes or the layout changes; Issue #17 owns the async and obsolete-layout contract, so synchronous preparation at load or layout time is acceptable in this issue.

---

### 3. Document scrolling and the viewport state model

**Type**: DOCUMENT  
**Output**: Wiki documentation records scroll units, clamping, placeholder behavior, per-file state, and prepared-row rendering.  
**Depends on**: 2

Read and follow `Notes/wiki/wiki-rules.md` and the schema in `Notes/wiki/AGENTS.md`, then ingest the completed Issue #12 `internal/viewport` and `internal/app` implementation and tests into the appropriate pages under `Notes/wiki`. Document the one-row, half-page, and full-page units and their formulas, content height as panel height minus the filename row, BOF and EOF clamping with no avoidable blank rows, the placeholder no-op, per-file saved viewport state for revisits, and rendering from prepared viewport data with the visible-range-only cost guard. Cross-reference Issue #12 and the Navigation, viewport, and logical anchors and Module Design sections of `Notes/PRD-vrg.md`, update `Notes/wiki/index.md`, and append the required dated ingest record to `Notes/wiki/log.md` without rewriting previous entries.

---

### 4. Create the scrolling walkthrough

**Type**: CODE WALKTHROUGH  
**Output**: Showboat walkthrough exists at `Notes/walkthroughs/012-04/code-walkthrough`.  
**Depends on**: 3

Use showboat, consulting `uvx showboat --help`, to create the walkthrough at exactly `Notes/walkthroughs/012-04/code-walkthrough`, with the main file named `walkthrough.md`. Demonstrate the Viewport unit and clamp tests, the placeholder and render-cost tests, then run the binary on a long file showing `down`/`up`, `d`/`u`, and `pgdn`/`pgup` moving by the expected amounts, EOF stopping with the last row at the bottom, and `up` at the top doing nothing. Reference Issue #12 and `Notes/PRD-vrg.md`, and store every generated artifact in the approved directory.

---
