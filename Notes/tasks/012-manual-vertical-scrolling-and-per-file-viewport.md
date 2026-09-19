# Tasks for #12: Manual vertical scrolling, clamping, per-file saved viewport

Parent issue: #12
Parent PRD: PRD-vrg.md
**Blocked by issues**: #5
**Acceptance criteria**: AC1–AC5 → Tasks 1–2

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

### 3. Create the scrolling finish marker

**Type**: FINISH MARKER  
**Output**: Finish marker exists at `Notes/finish-markers/012-04/finish-marker.md`.  
**Depends on**: 2

Write `Task 012-04 finished successfully at <time>` to `Notes/finish-markers/012-04/finish-marker.md`, replacing `<time>` with the current UTC timestamp (for example, `date -u +"%Y-%m-%dT%H:%M:%SZ"`). Create the `Notes/finish-markers/012-04/` directory if it does not already exist.

---
