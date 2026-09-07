## Issue 18: Horizontal panning in run-off-edge mode

**Type**: AFK
**Blocked by**: Issue 16, Issue 17

### Parent PRD

`Notes/PRD-vrg.md`

### What to build

- `,`/`.` pan one column left/right; `<`/`>` ten columns; `[`/`]` half the text-area width (`max(1, floor(text width / 2))`).
- Panning is a no-op in wrap mode.
- Horizontal offset is separate state: retained through wrap toggles (subject to display clamping), reset to zero on file change (before the horizontal reveal in Issue 19).
- Horizontal clipping that splits a grapheme cluster renders blank cells for the clipped portion.

**Horizontal extent and the maximum valid offset — product decision: visible lines**

The PRD records the **visible-lines extent policy** in *Navigation, viewport, and logical anchors*, confirmed by the product owner (the alternative file-wide policy was considered and rejected). The clamp follows the widest **currently rendered** source line, and the maximum is defined at a paintable cluster/marker boundary so grapheme-safe clipping never blanks the text area. Three definitions are kept distinct:

- *Content extent* of a line: its effective display width in cells, including any end-of-line marker cell (Issue 23; marker-extent pan tests are owned there) — i.e. the cell after its last rendered cell.
- *Extent policy*: the widest **currently rendered** source line defines the clamp, re-evaluated whenever the visible line set changes (see consequences below).
- *Maximum valid offset*: `max(0, S)` where `S` is the largest cell index at which a grapheme cluster or end-of-line marker of that line starts **and** its cell width fits within the text width, so it is fully paintable at offset `S`. For single-cell content this equals the widest line's final cell (`E − 1`). When the line ends with a multi-cell cluster, `S` is that cluster's start — smaller than `E − 1`, because an offset landing inside the cluster would leave grapheme-safe clipping rendering only blanks. If no cluster of the line fits the text width at all (a cluster wider than the text area — Issue 19's exception), the maximum is 0. An empty buffer, a placeholder, or a view of only empty lines gives 0.

The paintable-boundary maximum delivers the guarantee the simpler `E − 1` form intended: at the maximum offset, at least one **whole** cluster or marker cell of the widest visible line is painted, so the text area is never entirely blank because of clamping and never shows half a glyph.

Consequences of the visible-lines policy, all intended:

- The clamp is re-evaluated whenever the visible line set changes — vertical scrolling, reveal, resize, list hide/show, gutter growth, wrap-toggle re-entry — as well as on every pan.
- Vertical scrolling from a long line into a region of short lines re-clamps the offset leftwards, and the clamp **is applied to the stored offset**; scrolling back to the long line does not restore the old offset. The saved per-file horizontal state and the Issue 19 reveal operate on the clamped value.
- Because the widest visible line always keeps at least one fully painted cluster or marker cell, the `_` gutter indicator (Issue 20) can still appear on shorter visible lines, but never on every visible line at once.
- The clamp is computed from the prepared layout's per-line extents for the visible row range only (Issue 17), not by scanning the whole buffer.

`w` → `w` leaves the offset unchanged **unless** the re-entry into run-off-edge mode clamps it (e.g. the terminal was resized while in wrap mode, or the visible set changed); the re-entry clamp uses the paintable-boundary maximum above.

See PRD *Navigation, viewport, and logical anchors* (scroll-unit, visible-lines extent, and wrap-toggle bullets), *Layout and indicators* (text width), and *Text, graphemes…* (clip bullet).

### How to verify

- **Manual**: `w` to run-off-edge; `.` shifts text left by one column, `>` by ten, `]` by half the width; `,` at offset 0 does nothing; `w` `w` keeps the offset; `n` into another file starts at offset 0; a CJK glyph half-clipped at the left edge shows a blank rather than a broken glyph; pan right until only the final cluster of the longest visible line remains fully painted — further `.`/`>`/`]` do nothing, and no half glyph or blank-only text area appears; then scroll down (`d`) into a region of short lines → the text shifts back so the longest of those lines keeps one fully painted cell; scroll back up → the offset stays at the clamped value.
- **Automated**: Viewport tests for each pan unit, wrap-mode no-op, offset retained through wrap toggle, wrap-toggle re-entry clamp after a width change, reset on file change, split-cluster blank cells; extent tests: max offset with a short-lines-only view; a mixed-width file (one 300-cell line of single-cell clusters among 10-cell lines) → max 299 while the long line is visible and 9 once it scrolls out; an empty buffer and a loading placeholder (max 0); a line ending in a two-cell cluster → its maximum is that cluster's start (`E − 2`), with a render-level assertion that at the maximum the cluster's two cells are fully painted (no blanks, no partial glyph); a line whose final cluster is wider than the text width → the maximum falls back to the last fitting cluster's start (0 if none fits); a vertical scroll that leaves only short lines visible re-clamps the stored offset and scrolling back does not restore it; resize/list-toggle/gutter change that alters the visible set re-clamps; a render-cost guard that extent evaluation touches only visible rows.

### Acceptance criteria

- [ ] Given run-off-edge mode, when `,`/`.`, `<`/`>`, `[`/`]` are pressed, then the offset changes by 1, 10, and `max(1, floor(w/2))` respectively, clamped to `[0, max(0, S)]` with `S` the paintable-boundary maximum defined above.
- [ ] Given the currently visible source lines, then at least one fully painted cluster or marker cell of the widest one is always on screen; an empty buffer, placeholder, or all-empty visible set clamps to 0.
- [ ] Given the widest visible line ends in a multi-cell cluster, when panning reaches the maximum, then the offset stops at that cluster's start and the cluster is painted whole — no blank-only text area and no split glyph.
- [ ] Given a stored offset and a change to the visible line set (scroll, reveal, resize, list toggle, gutter growth), then the stored offset is clamped to the new maximum and is not restored when the wider line becomes visible again.
- [ ] Given wrap mode, when a pan key is pressed, then nothing changes.
- [ ] Given a nonzero offset, when `w` is pressed twice with no intervening resize, then the offset is unchanged; given the width changed in between, then the offset is the prior value clamped to the new maximum.
- [ ] Given navigation to a different file, then the offset is reset to zero before any reveal.
- [ ] Given a wide cluster split by the left clip edge, then blank cells are drawn for the clipped portion.
- [ ] Given a frame, then the extent clamp is derived from the prepared layout's visible rows only, not a full-buffer scan.

### User stories addressed

- User story 65: pan speeds
- User story 66: offset preserved through wrap toggles, reset on file change

---
