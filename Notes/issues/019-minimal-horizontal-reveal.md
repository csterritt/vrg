## Issue 19: Minimal horizontal reveal of the first-submatch start

**Type**: AFK
**Blocked by**: Issue 18

### Parent PRD

`Notes/PRD-vrg.md`

### What to build

In run-off-edge mode, at startup and on every navigation action (including same-file `n`/`p`), if the target cell (start cell of the first submatch on the destination line) is horizontally hidden, move the horizontal offset by the **minimum** number of columns that makes it visible. If already visible, keep the offset. On file change this happens after the reset from Issue 18.

- A match wider than the text area is revealed by showing its start cell; full-span visibility is not required.
- This is minimal scrolling, not one-third-across placement.
- In wrap mode no horizontal reveal is needed.

See PRD *Navigation, viewport, and logical anchors* (horizontal reveal bullet) and *Further Notes* (horizontal reveal note).

### How to verify

- **Manual**: `w` to run-off-edge in a file with a match at column 300; `n` to it → the view scrolls right just enough to show the match start at the right edge; `n` to a match at column 5 → view scrolls left just enough; a match already visible → no horizontal movement.
- **Automated**: Viewport tests: target right of view → offset = target − (text width − 1); target left → offset = target; target visible → unchanged; oversized span reveals start cell only; same-file navigation triggers reveal; startup reveal after load applies horizontal reveal too.

### Acceptance criteria

- [ ] Given the target cell is right of the visible text area, when navigating, then the offset increases by exactly enough for the cell to be the last visible column.
- [ ] Given the target cell is left of the visible area, then the offset decreases to exactly the target column.
- [ ] Given the target cell is visible, then the offset does not change.
- [ ] Given a match wider than the text area, then it is considered revealed when its start cell is visible.

### User stories addressed

- User story 54: minimal horizontal scrolling on startup and every navigation action
- User story 55: oversized match revealed by its start cell

---
