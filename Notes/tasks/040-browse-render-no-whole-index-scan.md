# Tasks for #40: Browse rendering drops the per-frame whole-index scan

Parent issue: #40
Parent PRD: PRD-vrg.md
**Blocked by issues**: #39 — this issue consumes #39's shared grapheme/cell helper and cluster-safe path geometry; landing it afterward prevents parallel rewrites of `renderBrowse`
**Acceptance criteria**: AC1–AC5 → Tasks 1–2; AC6 → Task 2 (the existing file-list behaviour tests and Issue #39's focused rendering regressions are the unchanged safety net)

## Tasks

### 1. Specify the bounded-render cost guard

**Type**: RED  
**Output**: Failing tests make any whole-index enumeration during a navigation `Update()` plus resulting `View()` fail — an index test double or access counter proving `Stops()`/full-stop copying is never invoked on either half of the model transition, that only the visible file range is materialized, and that resize/gutter-growth re-truncates visible paths without a whole-list scan.  
**Depends on**: none

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Begin only after Issue #39 is complete, and run its focused cluster/path rendering regressions before adding this RED so the shared renderer and grapheme-safe path geometry form the baseline this issue must preserve. Strengthen the file-list cost guard in `internal/app` (alongside the existing `countingFileList`/`FileListProvider` seam in `layout_test.go`). Install an index/provider double that panics on a whole-stop accessor, or a counter whose permitted access is bounded by the visible window, then send an `n`/`p` file-navigation key through `Update()` and call the resulting model's `View()`. Assert the bound across the combined model-transition path, not separately reset between `Update()` and `View()`, so a whole-index scan, copy, regroup, or whole-group reallocation in either half fails. Also require only the visible file range to be materialized per frame, and add a resize/gutter-growth case asserting visible paths are re-truncated against the new list width at grapheme boundaries with no whole-list scan. These tests fail on the current code, whose `renderBrowse` calls `m.index.Stops()` and `groupByFile` on every frame, and must also fail if that work is merely moved into navigation handling. Keep this task test-only.

---

### 2. Precompute file groups and render only the visible range

**Type**: GREEN  
**Output**: The cost-guard tests pass; navigation `Update()` and `View()` read shared precomputed per-file groups and width-independent path metadata, combined per-keystroke transition/render cost is bounded by the visible window rather than total stops or files, and file-list behaviour is unchanged.  
**Depends on**: 1

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Restructure `internal/app/app.go` so immutable per-file groups, current-file indexes, and width-independent display metadata — the `safepresentation.EscapePath` text, its grapheme-cluster boundaries, and its full cell width — are prepared once when the search completes and the index is finalized (the `SearchCompleteMsg` handler, where `longestPathWidth` is already computed), not per frame. `renderBrowse` must render only the visible file-list range from the precomputed groups, left-truncating each visible entry against the *current* list width inside `View()` — expected work bounded by the visible window — without splitting graphemes; the truncated string cannot be precomputed because the allotted width changes on terminal resize, gutter growth, wrap-mode indicator changes, and list hide/show. Navigation updates the current-file pointer/index without regrouping or reallocating the whole grouping, keeping per-file structures shared. Preserve deterministic order, current-file underline and scroll-into-view, left-truncation, the width cap, and the hide/show toggle. Consume Issue #39's shared grapheme/cell helper for full path width and visible-row truncation rather than adding competing geometry, and rerun Issue #39's focused renderer/path regressions alongside the unchanged file-list tests. Run the focused tests plus `go build ./...`, `go vet ./...`, and `go test ./...`.

---

### 3. Create the bounded-render finish marker

**Type**: FINISH MARKER  
**Output**: Finish marker exists at `Notes/finish-markers/040-04/finish-marker.md`.  
**Depends on**: 2

Write `Task 040-04 finished successfully at <time>` to `Notes/finish-markers/040-04/finish-marker.md`, replacing `<time>` with the current UTC timestamp (for example, `date -u +"%Y-%m-%dT%H:%M:%SZ"`). Create the `Notes/finish-markers/040-04/` directory if it does not already exist.

---
