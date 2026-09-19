# Tasks for #43: Standalone combining clusters get a real one-cell display fallback

Parent issue: #43
Parent PRD: PRD-vrg.md
**Blocked by issues**: #39
**Acceptance criteria**: AC1 → Task 1 (recorded decision) and Tasks 2–3; AC2–AC5 → Tasks 2–3; AC6 → Task 2

## Tasks

### 1. Decide and record the fallback representation

**Type**: REVIEW  
**Output**: A recorded human decision selects exactly one deterministic fallback representation — dotted-circle base, space base, or fixed placeholder — with its display byte sequence, segmentation/width expectation, normalization rule, and byte-mapping contract.  
**Depends on**: none

Review Issue #43 and the *Text, graphemes, and safe presentation* section of `Notes/PRD-vrg.md`. Choose exactly one fallback representation for standalone combining clusters: (A) `U+25CC ◌` followed by the original combining marks so they compose onto a conventional "no base" carrier, (B) a space followed by the original combining marks, or (C) a single fixed visible glyph such as U+FFFD or U+25CC displayed *instead of* the original marks. Record the decision in this issue and in the PRD's *Text, graphemes, and safe presentation* section or a `Notes/decisions/` entry, stating: (a) the exact display byte sequence; (b) its expected grapheme segmentation and cell width under the shared `rivo/uniseg` policy — it must segment as a single cluster occupying exactly one terminal cell — plus how an unexpected width result (0 or >1 from the width library, or an inconsistent terminal) is normalized to the required one cell; and (c) that byte-to-cell mapping still resolves the fallback cell to the cluster's original source bytes. Do not begin Task 2 until this decision is recorded.

---

### 2. Specify the recorded fallback cell

**Type**: RED  
**Output**: Failing tests assert a standalone combining cluster paints exactly one cell with the recorded display bytes/grapheme, `cellPos` advances by the fallback width, byte mappings resolve to the original source bytes, and wrapping/clipping/highlighting treat the fallback as a real cell.  
**Depends on**: 1

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Begin only after Issue #39 is complete — the fallback cell must propagate through the cluster-based renderer that issue installs — and only after the Task 1 decision is recorded. Add failing tests in `internal/filebuffer` plus a composed terminal-output test in `internal/app`: a line containing a standalone combining mark with no base produces a cluster occupying exactly one cell displaying the recorded fallback bytes/grapheme (assert the actual display bytes, not merely that the next cluster starts one cell later); the following cluster renders in the next cell with no overlap and no shared cell; `cellPos` and the `expandedByteCells` remapping advance by the fallback width while byte-to-cell mappings still resolve the fallback cell to the cluster's original source bytes; wrapping, clipping, and horizontal panning count the fallback like any other cell; and a match covering the standalone cluster highlights exactly the fallback cell and nothing adjacent. Keep this task test-only.

---

### 3. Materialize the fallback cell

**Type**: GREEN  
**Output**: The fallback tests pass; every layer — clusters, byte-to-cell mappings, wrapping, clipping, highlight expansion, and the Issue #39 renderer — agrees the standalone combining cluster occupies one visible cell showing the recorded representation.  
**Depends on**: 2

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Implement the recorded fallback in `internal/filebuffer` so a standalone zero-width cluster gains the chosen one-cell display representation rather than only a widened `clusterCells` annotation: the cluster's display text and width reflect the fallback, `cellPos` advances by it, and the Issue #39 cluster-driven renderer has a real cell to paint. Keep byte-to-cell mapping pointing at the original source bytes — only the *display* gains the fallback cell — and propagate the fallback width consistently through `expandedByteCells`, `expandedHighlights`, wrapping, clipping, and rendering so no following cluster can overlap it. Run the focused tests plus `go build ./...`, `go vet ./...`, and `go test ./...`.

---

### 4. Create the fallback-cell finish marker

**Type**: FINISH MARKER  
**Output**: Finish marker exists at `Notes/finish-markers/043-05/finish-marker.md`.  
**Depends on**: 3

Write `Task 043-05 finished successfully at <time>` to `Notes/finish-markers/043-05/finish-marker.md`, replacing `<time>` with the current UTC timestamp (for example, `date -u +"%Y-%m-%dT%H:%M:%SZ"`). Create the `Notes/finish-markers/043-05/` directory if it does not already exist.

---
