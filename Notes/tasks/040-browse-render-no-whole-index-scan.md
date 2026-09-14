# Tasks for #40: Browse rendering drops the per-frame whole-index scan

Parent issue: #40
Parent PRD: PRD-vrg.md
**Blocked by issues**: none
**Acceptance criteria**: AC1–AC5 → Tasks 1–2; AC6 → Task 2 (the existing file-list behaviour tests are the unchanged safety net)
**Manual verification**: Task 4 owns the issue's manual checks.

## Tasks

### 1. Specify the bounded-render cost guard

**Type**: RED  
**Output**: Failing tests make any whole-index enumeration during `View()` fail — an index test double or access counter proving `Stops()`/full-stop copying is never invoked per frame, that only the visible file range is materialized, and that resize/gutter-growth re-truncates visible paths without a whole-list scan.  
**Depends on**: none

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Strengthen the file-list cost guard in `internal/app` (alongside the existing `countingFileList`/`FileListProvider` seam in `layout_test.go`). Require that rendering a completed search performs no call enumerating or copying the full stop list — for example an index/provider test double that panics or fails when the whole-stop accessor is invoked during `View()`, or a counter asserting index access bounded by the visible window. Require only the visible file range to be materialized per frame, and add a resize/gutter-growth case asserting visible paths are re-truncated against the new list width at grapheme boundaries with no whole-list scan. These tests fail on the current code, whose `renderBrowse` calls `m.index.Stops()` and `groupByFile` on every frame. Keep this task test-only.

---

### 2. Precompute file groups and render only the visible range

**Type**: GREEN  
**Output**: The cost-guard tests pass; `View()` reads precomputed per-file groups and width-independent path metadata, per-frame cost is bounded by the visible window rather than total stops or files, and file-list behaviour is unchanged.  
**Depends on**: 1

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Restructure `internal/app/app.go` so immutable per-file groups, current-file indexes, and width-independent display metadata — the `safepresentation.EscapePath` text, its grapheme-cluster boundaries, and its full cell width — are prepared once when the search completes and the index is finalized (the `SearchCompleteMsg` handler, where `longestPathWidth` is already computed), not per frame. `renderBrowse` must render only the visible file-list range from the precomputed groups, left-truncating each visible entry against the *current* list width inside `View()` — expected work bounded by the visible window — without splitting graphemes; the truncated string cannot be precomputed because the allotted width changes on terminal resize, gutter growth, wrap-mode indicator changes, and list hide/show. Navigation updates the current-file pointer/index without regrouping or reallocating the whole grouping, keeping per-file structures shared. Preserve deterministic order, current-file underline and scroll-into-view, left-truncation, the width cap, and the hide/show toggle — the existing file-list tests must pass unchanged. Run the focused tests plus `go build ./...`, `go vet ./...`, and `go test ./...`.

---

### 3. Document the bounded render path

**Type**: DOCUMENT  
**Output**: Wiki documentation records the precomputed groups/path metadata and the O(visible rows) render bound.  
**Depends on**: 2

Read and follow `Notes/wiki/wiki-rules.md` and the schema in `Notes/wiki/AGENTS.md`, then ingest the completed Issue #40 implementation into the appropriate pages under `Notes/wiki`. Document what is prepared once at search completion (per-file groups, current-file indexes, escaped path text, cluster boundaries, full cell width), what remains per-frame (visible-range rendering and current-width truncation), the strengthened cost guard, and the roughly 100,000-matched-line responsiveness rationale. Cross-reference Issue #40 and the *Resources and responsiveness* and *File list and layout* sections of `Notes/PRD-vrg.md`, update `Notes/wiki/index.md`, and append the required dated ingest record to `Notes/wiki/log.md` without rewriting previous entries.

---

### 4. Create the bounded-render walkthrough

**Type**: CODE WALKTHROUGH  
**Output**: Showboat walkthrough exists at `Notes/walkthroughs/040-04/code-walkthrough`.  
**Depends on**: 3

Use showboat, consulting `uvx showboat --help`, to create the walkthrough at exactly `Notes/walkthroughs/040-04/code-walkthrough`, with the main file named `walkthrough.md`. Demonstrate the cost-guard tests, then run the issue's manual scenario: generate a large result set (tens of thousands of matched lines across many files) and hold `n` or `down`, or resize repeatedly → navigation and repainting stay responsive with no per-keystroke stall growing with index size. Capture commands, outputs, and exit statuses. Reference Issue #40 and `Notes/PRD-vrg.md`, and store every generated artifact in the approved directory.

---
