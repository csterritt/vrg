## Issue 18: Horizontal panning in run-off-edge mode

**Type**: AFK
**Blocked by**: Issue 16

### Parent PRD

`Notes/PRD-vrg.md`

### What to build

- `,`/`.` pan one column left/right; `<`/`>` ten columns; `[`/`]` half the text-area width (`max(1, floor(text width / 2))`).
- Clamp to valid extents (offset ≥ 0 and not beyond the widest visible line's effective extent). Panning is a no-op in wrap mode.
- Horizontal offset is separate state: retained through wrap toggles (subject to display clamping), reset to zero on file change (before the horizontal reveal in Issue 19).
- Horizontal clipping that splits a grapheme cluster renders blank cells for the clipped portion.

See PRD *Navigation, viewport, and logical anchors* (scroll-unit and wrap-toggle bullets) and *Text, graphemes…* (clip bullet).

### How to verify

- **Manual**: `w` to run-off-edge; `.` shifts text left by one column, `>` by ten, `]` by half the width; `,` at offset 0 does nothing; `w` `w` keeps the offset; `n` into another file starts at offset 0; a CJK glyph half-clipped at the left edge shows a blank rather than a broken glyph.
- **Automated**: Viewport tests for each pan unit, clamps, wrap-mode no-op, offset retained through wrap toggle, reset on file change, split-cluster blank cells.

### Acceptance criteria

- [ ] Given run-off-edge mode, when `,`/`.`, `<`/`>`, `[`/`]` are pressed, then the offset changes by 1, 10, and `max(1, floor(w/2))` respectively, clamped.
- [ ] Given wrap mode, when a pan key is pressed, then nothing changes.
- [ ] Given a nonzero offset, when `w` is pressed twice, then the offset is unchanged.
- [ ] Given navigation to a different file, then the offset is reset to zero before any reveal.
- [ ] Given a wide cluster split by the left clip edge, then blank cells are drawn for the clipped portion.

### User stories addressed

- User story 65: pan speeds
- User story 66: offset preserved through wrap toggles, reset on file change

---
