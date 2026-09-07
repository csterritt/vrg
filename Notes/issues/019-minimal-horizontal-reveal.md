## Issue 19: Minimal horizontal reveal of the first-submatch start

**Type**: AFK
**Blocked by**: Issue 18

### Parent PRD

`Notes/PRD-vrg.md`

### What to build

In run-off-edge mode, at startup and on every navigation action (including same-file `n`/`p`), if the target cell (start cell of the first submatch on the destination line) is horizontally hidden, move the horizontal offset by the **minimum** number of columns that makes it visible. If already visible, keep the offset. On file change this happens after the reset from Issue 18.

- "Visible" for reveal purposes means the target's start cell is **actually rendered as a painted cell**, consistent with how Issue 20 computes indicator visibility (rendered cells after grapheme clipping, excluding the reserved column). A geometric position inside the text area that Issue 18's grapheme-safe clipping renders as a blank is *not* visible.
- Consequence for wide targets at the right edge: if the target is a two-cell cluster whose start column is `T`, revealing it from the right requires offset `T − (text width − 2)` so both cells fit; the general rule is `offset = T + cluster width − text width` when revealing from the right. From the left the rule remains `offset = T`. Issue 21's cluster-expanded spans supply the width; until Issue 21 lands, cluster width comes from the shared grapheme policy already used by Issue 16 for wrapping.
- If the target cluster is wider than the whole text area, it can never be fully painted: grapheme-safe clipping renders its in-window portion as blanks at every offset. Set the offset to `T` — the start-cell rule gives the closest achievable position — and treat the target as revealed **geometrically** in this one case; the painted-cell visibility rule above does not apply. Indicator visibility (Issue 20) still treats the blank-rendered cluster as not visible, and no further reveal movement is attempted for it (no panning loop).
- A match wider than the text area is revealed by showing its start cell; full-span visibility is not required.
- This is minimal scrolling, not one-third-across placement.
- In wrap mode no horizontal reveal is needed.

See PRD *Navigation, viewport, and logical anchors* (horizontal reveal bullet), *Layout and indicators* (visibility over rendered cells), and *Further Notes* (horizontal reveal note).

### How to verify

- **Manual**: `w` to run-off-edge in a file with a match at column 300; `n` to it → the view scrolls right just enough to show the match start at the right edge; `n` to a match at column 5 → view scrolls left just enough; a match already visible → no horizontal movement; a CJK match at column 300 → both cells of its first glyph are visible at the right edge, not a blank.
- **Automated**: Viewport tests: single-cell target right of view → offset = target − (text width − 1); two-cell target right of view → offset = target − (text width − 2) and the cluster is fully painted; target left → offset = target; target visible → unchanged; target geometrically inside the area but clipped to blank by a two-cell glyph at the right edge → treated as hidden and revealed; oversized span reveals start cell only; cluster wider than the text area → offset = target, its in-window cells render as clipping blanks (not a partial glyph), indicator logic counts it as not visible, and a repeat navigation does not move the offset further; same-file navigation triggers reveal; startup reveal after load applies horizontal reveal too.

### Acceptance criteria

- [ ] Given the target cell is right of the visible text area, when navigating, then the offset increases by exactly enough for the target cluster to be fully painted at the right edge.
- [ ] Given the target cell is left of the visible area, then the offset decreases to exactly the target column.
- [ ] Given the target's start cell is already painted, then the offset does not change.
- [ ] Given the target's start cell is within the text-area geometry but rendered as a clipping blank, then it is treated as hidden and revealed.
- [ ] Given a match wider than the text area, then it is considered revealed when its start cell is visible.
- [ ] Given a target cluster wider than the whole text area, then the offset is set to its start column, the cluster renders as clipping blanks, and no further reveal movement occurs.

### User stories addressed

- User story 54: minimal horizontal scrolling on startup and every navigation action
- User story 55: oversized match revealed by its start cell

---
