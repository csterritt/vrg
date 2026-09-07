## Issue 16: Wrap mode (default on), `w` toggle, grapheme-boundary wrapping, tab stops

**Type**: AFK
**Blocked by**: Issue 14

### Parent PRD

`Notes/PRD-vrg.md`

### What to build

- Wrapping is on initially; `w` toggles between wrap and run-off-edge modes.
- Text width = panel width − gutter − reserved right-indicator width (0 in wrap mode, 1 in run-off-edge; the column is reserved now, populated in Issue 20).
- Wrap only at grapheme-cluster boundaries. A two-cell cluster that does not fit in the remaining cell moves to the next row, leaving a blank. Continuation rows have a blank gutter aligned with the first row's text.
- Tabs expand to the next multiple of 8 source-display columns independent of gutter and pan.
- One grapheme segmentation / cell-width policy is shared by FileBuffer (boundaries and widths) and Viewport (wrapping and clipping). FileBuffer exposes cluster boundary information; Viewport does not re-derive it.
- Scroll units remain rendered rows; the target-row reveal from Issue 14 must now find the row of a wrapped line that contains the match start.
- Wrapping produces a **prepared row model** (source line → rendered rows for the current text width and mode). `View()` renders from it; it never wraps the full buffer per frame. In this issue the row model may be built synchronously at load/toggle/resize time; **Issue 17 moves preparation off the UI update path and owns the obsolete-layout contract**. Build the row model as a value that can be swapped in, keyed by (path, content revision, text width, wrap mode), so Issue 17 does not need to restructure it.

See PRD *Text, graphemes, and safe presentation* (grapheme/tab/wrap bullets) and *Layout and indicators* (content-width bullet).

### How to verify

- **Manual**: open a file with a 500-character line: it wraps across rows with a blank gutter on continuation rows; `w` shows it as one clipped row; tabs align to 8-column stops; a match near the end of the long line is revealed on its own row after `n`.
- **Automated**: Viewport tests: wrap row counts for ASCII, wide CJK at row boundary (blank cell + wrap), combining sequences kept together, tab expansion; reveal test for a match near the end of a source line taller than several screens landing at row `floor(h/3)`; `w` toggle changes row model; App rendering test for continuation gutter alignment; a render-cost guard that `View()` on a many-line buffer does not invoke the wrapper for lines outside the visible rows (counting fake).

### Acceptance criteria

- [ ] Given startup, then wrapping is on; when `w` is pressed, then run-off-edge mode is active; again returns to wrap.
- [ ] Given a wide cluster that would straddle the row end, then it starts the next row and the previous row ends with a blank cell.
- [ ] Given tabs, then they expand to the next multiple of 8 columns regardless of gutter width or horizontal offset.
- [ ] Given a match far down a wrapped line, when navigated to, then the row containing the match start is placed per Issue 14 rules.
- [ ] Given continuation rows, then their gutter is blank and text aligns with the first row's text.

### User stories addressed

- User story 64: wrapping on initially, `w` toggles, blank continuation gutters
- User story 71: tabs to 8-column stops; one grapheme/cell policy

---
