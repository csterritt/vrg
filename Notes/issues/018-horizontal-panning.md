## Issue 18: Horizontal panning in run-off-edge mode

**Type**: AFK
**Blocked by**: Issue 16

### Parent PRD

`Notes/PRD-vrg.md`

### What to build

- `,`/`.` pan one column left/right; `<`/`>` ten columns; `[`/`]` half the text-area width (`max(1, floor(text width / 2))`).
- Panning is a no-op in wrap mode.
- Horizontal offset is separate state: retained through wrap toggles (subject to display clamping), reset to zero on file change (before the horizontal reveal in Issue 19).
- Horizontal clipping that splits a grapheme cluster renders blank cells for the clipped portion.

**Horizontal extent and the maximum valid offset — decided: visible lines**

The PRD says pan commands "clamp to valid extents" and that the offset is "subject to actual display clamping" without saying which lines define the extent. The product owner has chosen the **visible-lines** policy (recorded here; superseded critique option "file-wide" is rejected). Two definitions are kept distinct:

- *Content extent* of a line: its effective display width in cells, including any end-of-line marker cell (Issue 23) — i.e. the cell after its last rendered cell.
- *Maximum valid offset*: `max(0, E − 1)` where `E` is the extent of the widest **currently rendered** source line. The `− 1` guarantees at least one content cell of that line remains rendered, so the text area is never entirely blank because of clamping. An empty buffer, a placeholder, or a view of only empty lines gives 0.

Consequences of the visible-lines policy, all intended:

- The clamp is re-evaluated whenever the visible line set changes — vertical scrolling, reveal, resize, list hide/show, gutter growth, wrap-toggle re-entry — as well as on every pan.
- Vertical scrolling from a long line into a region of short lines re-clamps the offset leftwards, and the clamp **is applied to the stored offset**; scrolling back to the long line does not restore the old offset. The saved per-file horizontal state and the Issue 19 reveal operate on the clamped value.
- Because the widest visible line always keeps at least one rendered cell, the `_` gutter indicator (Issue 20) can still appear on shorter visible lines, but never on every visible line at once.
- The clamp is computed from the prepared layout's per-line extents for the visible row range only (Issue 17), not by scanning the whole buffer.

`w` → `w` leaves the offset unchanged **unless** the re-entry into run-off-edge mode clamps it (e.g. the terminal was resized while in wrap mode, or the visible set changed).

See PRD *Navigation, viewport, and logical anchors* (scroll-unit and wrap-toggle bullets), *Layout and indicators* (text width), and *Text, graphemes…* (clip bullet).

### How to verify

- **Manual**: `w` to run-off-edge; `.` shifts text left by one column, `>` by ten, `]` by half the width; `,` at offset 0 does nothing; `w` `w` keeps the offset; `n` into another file starts at offset 0; a CJK glyph half-clipped at the left edge shows a blank rather than a broken glyph; pan right until only one cell of the longest visible line remains — further `.`/`>`/`]` do nothing; then scroll down (`d`) into a region of short lines → the text shifts back so the longest of those lines keeps one cell; scroll back up → the offset stays at the clamped value.
- **Automated**: Viewport tests for each pan unit, wrap-mode no-op, offset retained through wrap toggle, wrap-toggle re-entry clamp after a width change, reset on file change, split-cluster blank cells; extent tests: max offset with a short-lines-only view; a mixed-width file (one 300-cell line among 10-cell lines) → max 299 while the long line is visible and 9 once it scrolls out; an empty buffer and a loading placeholder (max 0); a line whose extent is only an EOL marker (extent 1, max 0 for that line); a two-cell cluster at the extent boundary; a vertical scroll that leaves only short lines visible re-clamps the stored offset and scrolling back does not restore it; resize/list-toggle/gutter change that alters the visible set re-clamps; a render-cost guard that extent evaluation touches only visible rows.

### Acceptance criteria

- [ ] Given run-off-edge mode, when `,`/`.`, `<`/`>`, `[`/`]` are pressed, then the offset changes by 1, 10, and `max(1, floor(w/2))` respectively, clamped to `[0, max(0, E − 1)]`.
- [ ] Given the currently visible source lines, then at least one content cell of the widest one is always rendered; an empty buffer, placeholder, or all-empty visible set clamps to 0.
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
