## Issue 12: Manual vertical scrolling, clamping, per-file saved viewport

**Type**: AFK
**Blocked by**: Issue 5

### Parent PRD

`Notes/PRD-vrg.md`

### What to build

Viewport scrolling for the current file in the (currently unwrapped) panel.

- `up`/`down` scroll one rendered row; `u`/`d` half a page (`max(1, floor(height/2))`); `page up`/`page down` a full page. Content height = panel height minus the filename row.
- Clamp to valid content: top row ≥ 0 and no avoidable blank rows below EOF. Files shorter than the viewport naturally leave unused rows.
- Scrolling a "Loading…" placeholder is a no-op.
- Save per-file vertical viewport state so a file revisited later (Issue 13) can start from it.
- Rendering uses prepared viewport data, not a scan of the full buffer per frame.

See PRD *Navigation, viewport, and logical anchors* (scroll-unit bullet) and *Module Design → Viewport*.

### How to verify

- **Manual**: open a long file; `down`/`up`, `d`/`u`, `pgdn`/`pgup` move by expected amounts; scrolling past EOF stops with the last row at the bottom; `up` at top does nothing.
- **Automated**: Viewport unit tests for each scroll unit and both clamps at BOF/EOF with a file shorter than, equal to, and longer than the viewport; half-page for odd heights; App test that scroll keys on a loading placeholder change nothing; per-file state persisted in the model.

### Acceptance criteria

- [ ] Given a loaded file, when `down`/`up` is pressed, then the top row moves by one rendered row.
- [ ] Given content height `h`, when `d`/`u` is pressed, then the top row moves by `max(1, floor(h/2))`; when `page down`/`page up`, by `h`.
- [ ] Given the viewport is at EOF, when scrolling down further, then the top row does not change.
- [ ] Given a "Loading…" placeholder, when a scroll key is pressed, then nothing changes.

### User stories addressed

- User story 60: scroll one row, half page, full page
- User story 61: scrolling clamped to valid content

---
