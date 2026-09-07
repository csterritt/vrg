## Issue 5: Browse tracer — file list, file panel with highlights, async "Loading…"

**Type**: AFK
**Blocked by**: Issue 3

### Parent PRD

`Notes/PRD-vrg.md`

### What to build

Replace the interim summary screen with the real two-pane browse view for the first matched file. This is the primary tracer bullet through FileBuffer, Viewport (minimal), Theme (minimal) and App composition.

- Left: file list of every retained file in raw-path order; the current file (the first stop's file) underlined; the list scrolls to keep the current entry visible. Use a simple fixed/heuristic width for now (Issue 24 implements the real formula).
- Right: filename embedded in a horizontal rule, then content rows with a right-justified line-number gutter (digit width of largest line number, min 1) followed by two spaces. No other borders.
- FileBuffer loads the file asynchronously; "Loading…" placeholder until ready. Matches on matched lines render in inverse video (first-pass byte→cell mapping; grapheme/tab/terminator refinements come in Issues 21–23).
- Show the top of the file (no reveal yet); `q` exits 0.
- Loading runs off the UI update path; the model handles key/resize messages while a load is pending.

See PRD *File list and layout* stories, *File loading, cache, reload…* (first two bullets), *Layout and indicators* (gutter bullet), *Module Design → FileBuffer / Viewport / App*.

### How to verify

- **Manual**:
  1. `vrg func .` in a Go repo → file list left, first file's content right with matches in inverse.
  2. Gutter is right-justified with two trailing spaces; filename appears in a rule above the content.
  3. Resize the terminal; layout re-renders. `q` exits 0.
- **Automated**:
  - FileBuffer: loads bytes, reports source-line count and gutter width, exposes highlight spans for a matched line.
  - App model: injected search completion → browse view with "Loading…"; injected load completion → content visible with highlight span; a key message handled while load is pending (worker gate held).
  - Rendering test on the composed `View()` string: file list order, underline on current entry, gutter format.

### Acceptance criteria

- [ ] Given a completed search with results, when browsing begins, then every retained file is listed in raw-path order and the current file is underlined.
- [ ] Given the current file is not yet loaded, then the panel shows "Loading…" and input remains responsive.
- [ ] Given the load completes, then content replaces the placeholder and each matched span is rendered in inverse video.
- [ ] Given a loaded file, then the gutter width is the digit count of the largest line number plus two spaces, right-justified, and no left/right/bottom border is drawn.
- [ ] Given the browse view, when `q` is pressed, then the process exits 0.

### User stories addressed

- User story 24: every retained file listed in deterministic path order
- User story 26: current file underlined and kept within the scrolled list
- User story 31: filename rule, gutter, no other borders
- User story 34: loading leaves input responsive with "Loading…"

---
