# Tasks for #18: Horizontal panning in run-off-edge mode

Parent issue: #18
Parent PRD: PRD-vrg.md
**Blocked by issues**: #16, #17
**Acceptance criteria**: AC1–AC9 → Tasks 1–2
**Manual verification**: Task 4 owns the issue's manual checks.

## Tasks

### 1. Specify horizontal panning and the visible-lines extent

**Type**: RED  
**Output**: Failing Viewport tests cover each pan unit with clamping, the wrap-mode no-op, offset retention and re-entry clamping, file-change reset, split-cluster blanks, the paintable-boundary maximum across shape cases, visible-set re-clamping without restoration, and the visible-rows render-cost guard.  
**Depends on**: none

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Begin only after Issues #16 and #17 are complete. Add failing tests in `internal/viewport` for the Issue #18 contracts and the visible-lines extent and scroll-unit bullets of the Navigation, viewport, and logical anchors section of `Notes/PRD-vrg.md`. Require `,`/`.`, `<`/`>`, and `[`/`]` to pan one column, ten columns, and `max(1, floor(text width / 2))`, clamped to `[0, max(0, S)]` where `S` is the paintable-boundary maximum of the widest currently rendered source line; panning to be a no-op in wrap mode; the offset to be retained through wrap toggles subject to the re-entry clamp after a width change and reset to zero on file change; and horizontal clipping that splits a grapheme cluster to render blank cells for the clipped portion. Cover the extent policy: a short-lines-only view; a mixed-width file whose 300-cell line yields a maximum of 299 while visible and 9 once it scrolls out, with the stored offset re-clamped and not restored on return; an empty buffer, placeholder, and all-empty view clamping to 0; a line ending in a two-cell cluster stopping at that cluster's start with a render-level assertion that both cells remain fully painted at the maximum; a final cluster wider than the text width falling back to the last fitting cluster's start, or 0 if none fits; re-clamping on every visible-set change including vertical scrolling, reveal, resize, list hide/show, and gutter growth; and a render-cost guard proving extent evaluation touches only visible rows of the prepared layout. Keep this task test-only.

---

### 2. Implement panning and extent clamping

**Type**: GREEN  
**Output**: Pan, extent, retention, reset, and clip tests pass; the widest visible line always keeps one fully painted cluster or marker cell.  
**Depends on**: 1

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Implement the pan units, the visible-lines extent clamp computed from the prepared layout's per-line extents for the visible row range only, the paintable-boundary maximum, wrap-mode no-op, offset retention through toggles with re-entry clamping, file-change reset, and grapheme-safe clipping with blank cells. End-of-line marker extents from Issue #23 will later participate in the same maximum; keep the extent definition forward-compatible without implementing markers here.

---

### 3. Document horizontal panning and the extent policy

**Type**: DOCUMENT  
**Output**: Wiki documentation records pan units, the visible-lines extent policy, the paintable-boundary maximum, and re-clamping consequences.  
**Depends on**: 2

Read and follow `Notes/wiki/wiki-rules.md` and the schema in `Notes/wiki/AGENTS.md`, then ingest the completed Issue #18 implementation and tests into the appropriate pages under `Notes/wiki`. Document the pan units and clamping, the product-confirmed visible-lines extent policy with its three kept-distinct definitions (content extent, extent policy, maximum valid offset), the paintable-boundary maximum guaranteeing one fully painted cluster or marker cell, re-clamping on every visible-set change with no restoration, offset retention through wrap toggles with re-entry clamping, file-change reset, and split-cluster blank rendering. Cross-reference Issue #18 and the Navigation, viewport, and logical anchors and Layout and indicators sections of `Notes/PRD-vrg.md`, update `Notes/wiki/index.md`, and append the required dated ingest record to `Notes/wiki/log.md` without rewriting previous entries.

---

### 4. Create the panning walkthrough

**Type**: CODE WALKTHROUGH  
**Output**: Showboat walkthrough exists at `Notes/walkthroughs/018-04/code-walkthrough`.  
**Depends on**: 3

Use showboat, consulting `uvx showboat --help`, to create the walkthrough at exactly `Notes/walkthroughs/018-04/code-walkthrough`, with the main file named `walkthrough.md`. Demonstrate the pan-unit, extent, and re-clamp tests, then run the binary in run-off-edge mode: `.` and `>` shifting text left by one and ten columns, `]` by half the width, `,` at offset 0 doing nothing, `w` `w` keeping the offset, `n` into another file starting at offset 0, a half-clipped CJK glyph showing a blank rather than a broken glyph, panning to the maximum with the final cluster fully painted and further pans doing nothing, and scrolling into short lines re-clamping leftwards without restoration on return. Reference Issue #18 and `Notes/PRD-vrg.md`, and store every generated artifact in the approved directory.

---
