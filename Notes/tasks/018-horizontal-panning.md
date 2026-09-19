# Tasks for #18: Horizontal panning in run-off-edge mode

Parent issue: #18
Parent PRD: PRD-vrg.md
**Blocked by issues**: #16, #17
**Acceptance criteria**: AC1–AC9 → Tasks 1–2

## Tasks

### 1. Specify horizontal panning and the visible-lines extent

**Type**: RED  
**Output**: Failing Viewport tests cover each pan unit with clamping, the wrap-mode no-op, offset retention and re-entry clamping, file-change reset, split-cluster blanks, the paintable-boundary maximum across shape cases, visible-set re-clamping without restoration including wrap-toggle re-entry and every pan, and the visible-rows render-cost guard.  
**Depends on**: none

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Begin only after Issues #16 and #17 are complete. Add failing tests in `internal/viewport` for the Issue #18 contracts and the visible-lines extent and scroll-unit bullets of the Navigation, viewport, and logical anchors section of `Notes/PRD-vrg.md`. Require `,`/`.`, `<`/`>`, and `[`/`]` to pan one column, ten columns, and `max(1, floor(text width / 2))`, clamped to `[0, max(0, S)]` where `S` is the paintable-boundary maximum of the widest currently rendered source line; panning to be a no-op in wrap mode; the offset to be retained through wrap toggles subject to the re-entry clamp on every return to run-off-edge mode — not only after a width change — and reset to zero on file change; and horizontal clipping that splits a grapheme cluster to render blank cells for the clipped portion. Cover the extent policy: a short-lines-only view; a mixed-width file whose 300-cell line yields a maximum of 299 while visible and 9 once it scrolls out, with the stored offset re-clamped and not restored on return; an empty buffer, placeholder, and all-empty view clamping to 0; a line ending in a two-cell cluster stopping at that cluster's start with a render-level assertion that both cells remain fully painted at the maximum; a final cluster wider than the text width falling back to the last fitting cluster's start, or 0 if none fits; re-clamping on every visible-set change including vertical scrolling, reveal, resize, list hide/show, gutter growth, and wrap-toggle re-entry, with every pan likewise re-evaluating the maximum from the current visible rows rather than a cached value — including a wrap → run-off-edge re-entry whose visible set changed while in wrap mode clamping against the new visible rows' maximum, and a pan issued after the visible rows' extents changed clamping against the newly computed maximum; a uniform-lines fixture where every visible line has hidden-left text at a nonzero offset while the widest line still paints a fitting cluster — geometry that must stay legal for Issue #20's `_` indicator to appear on every visible line, so no extent or clamp assertion may forbid it; and a render-cost guard proving extent evaluation touches only visible rows of the prepared layout. Keep this task test-only.

---

### 2. Implement panning and extent clamping

**Type**: GREEN  
**Output**: Pan, extent, retention, reset, and clip tests pass; the widest visible line keeps one fully painted cluster or marker cell whenever a cluster fits the text width, with the documented unpaintable-cluster exception rendering clipping blanks by design.  
**Depends on**: 1

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Implement the pan units, the visible-lines extent clamp computed from the prepared layout's per-line extents for the visible row range only, the paintable-boundary maximum, wrap-mode no-op, offset retention through toggles with re-entry clamping on every return to run-off-edge mode, re-clamping on every visible-set change — vertical scrolling, reveal, resize, list hide/show, gutter growth, wrap-toggle re-entry — and on every pan, always recomputing the maximum from the current visible rows, file-change reset, and grapheme-safe clipping with blank cells. End-of-line marker extents from Issue #23 will later participate in the same maximum; keep the extent definition forward-compatible without implementing markers here.

---

### 3. Create the panning finish marker

**Type**: FINISH MARKER  
**Output**: Finish marker exists at `Notes/finish-markers/018-04/finish-marker.md`.  
**Depends on**: 2

Write `Task 018-04 finished successfully at <time>` to `Notes/finish-markers/018-04/finish-marker.md`, replacing `<time>` with the current UTC timestamp (for example, `date -u +"%Y-%m-%dT%H:%M:%SZ"`). Create the `Notes/finish-markers/018-04/` directory if it does not already exist.

---
