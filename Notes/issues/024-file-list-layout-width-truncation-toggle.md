## Issue 24: File-list layout — width formula, `…` truncation, hide/show

**Type**: AFK
**Blocked by**: Issue 20

### Parent PRD

`Notes/PRD-vrg.md`

### What to build

Replace the placeholder list width from Issue 5 with the specified layout.

- `left`/`tab` hide the list; `right`/`shift+tab` show it; initially shown.
- Width when requested visible = nonnegative minimum of: longest sanitized path width + 2; `floor(0.40 × terminal width)`; terminal width − (gutter width + 10 + reserved indicator width). Recompute after loading changes the gutter and after mode/size changes.
- A computed zero width draws no cells but does not change the user's visibility preference; no automatic toggling.
- Paths wider than their cells are left-truncated with a leading `…` without splitting graphemes.
- File-list scrolling keeps the active entry visible.
- The filename row embeds the safe path in a rule and makes room for buffer-status notes (Issues 26/29/30), truncating the path where possible.
- Panel text width follows from list width, gutter and reserved column; pathological sizes never produce negative widths.

See PRD *Layout and indicators* (width bullets) and *File list and layout* stories.

### How to verify

- **Manual**: in an 80-column terminal with long paths, the list is ≤ 32 columns and paths show `…prefix/basename`; load a file with 5-digit line numbers → list narrows if needed; `tab` hides the list and content widens; `shift+tab` restores it; shrink to 30 columns → list disappears but `shift+tab`/`tab` state is unchanged when widening.
- **Automated**: layout function tests for each of the three terms winning, the 40% floor rounding, gutter growth after load, list reduced to leave 10 text cells plus indicator, zero-width allocation preserving preference; truncation tests with wide/combining characters; App tests for toggle keys and list auto-scroll to the active entry.

### Acceptance criteria

- [ ] Given the list is requested visible, then its width is `min(longest+2, floor(0.4·W), W − (gutter + 10 + indicator))`, never negative.
- [ ] Given a path wider than the list, then it is displayed left-truncated with a leading `…` at a grapheme boundary.
- [ ] Given `left`/`tab`, then the list hides; given `right`/`shift+tab`, then it shows; startup shows it.
- [ ] Given a terminal size that yields zero list width, then no list cells are drawn and the visibility preference is retained.
- [ ] Given the current file is outside the visible list rows, then the list scrolls to include it.

### User stories addressed

- User story 27: list width just wide enough, capped at 40% and by minimum content width
- User story 28: long paths left-truncated with `…`
- User story 29: `left`/`tab` hide, `right`/`shift+tab` show, initially shown

---
