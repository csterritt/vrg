## Issue 17: Width-independent logical anchor through rewrap, wrap toggle and resize; lossy EOF clamp

**Type**: AFK
**Blocked by**: Issue 16

### Parent PRD

`Notes/PRD-vrg.md`

### What to build

Viewport owns a logical anchor `(source line, display-column offset)` independent of wrap width.

- After a resize or wrap toggle, the effective top row is the row containing the anchor location, not the row with the same former ordinal.
- Turning wrap off shows the anchor's source line as one row but retains the logical column; turning wrap on restores the row containing that column.
- User vertical scrolling replaces the anchor with the resulting top row's location. A match reveal that moves the viewport likewise replaces it; a no-scroll reveal does not.
- EOF clamping may pull the effective top upward and **updates** the anchor to the resulting top (intentionally lossy: a subsequent shrink need not restore the old top).
- Resize preserves the cursor selection and the reading position except for this documented EOF clamping.

See PRD *Navigation, viewport, and logical anchors* (last four bullets).

### How to verify

- **Manual**: scroll partway into a wrapped long line; narrow the terminal → the same text is still at the top (more rows); widen back → same text at top; press `w` twice → same text; scroll to EOF then widen → top moves up and stays there after narrowing again.
- **Automated**: Viewport tests: anchor across width change round trip (no loss when no clamp); wrap-off/wrap-on round trip preserving the logical column; manual scroll replaces anchor; reveal-with-scroll replaces, no-scroll reveal preserves; deliberate EOF-clamp loss scenario; App test that resize keeps the cursor.

### Acceptance criteria

- [ ] Given a viewport whose top is mid-way through a wrapped line, when the width changes, then the top row is the one containing the anchor's text location.
- [ ] Given wrap is toggled off then on, when no scroll/reveal/clamp intervened, then the effective top returns to the row containing the original logical column.
- [ ] Given user scrolling, then the anchor becomes the new top row's location.
- [ ] Given a resize that would leave avoidable blank rows below EOF, then the top is clamped upward and the anchor updated to that top.
- [ ] Given any resize, then the cursor selection is unchanged.

### User stories addressed

- User story 33: resize preserves selection and reading position except EOF clamping
- User story 62: width-independent logical anchor
- User story 63: EOF clamping updates the anchor (accepted loss)

---
