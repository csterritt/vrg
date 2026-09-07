## Issue 24: File-list layout — width formula, `…` truncation, hide/show

**Type**: AFK
**Blocked by**: Issue 17

### Parent PRD

`Notes/PRD-vrg.md`

### What to build

Replace the placeholder list width from Issue 5 with the specified layout.

- `left`/`tab` hide the list; `right`/`shift+tab` show it; initially shown.
- Width when requested visible = nonnegative minimum of: longest sanitized path width + 2; `floor(0.40 × terminal width)`; terminal width − (gutter width + 10 + reserved indicator width). Recompute after loading changes the gutter and after mode/size changes.
- A computed zero width draws no cells but does not change the user's visibility preference; no automatic toggling.
- Paths wider than their cells are left-truncated with a leading `…` without splitting graphemes.
- File-list scrolling keeps the active entry visible.
- The filename row embeds the safe path in a rule and provides a **buffer-status note slot**: the path is truncated to make room for a status note where possible. This issue implements and tests the slot and the truncation with a synthetic status string; the real note texts and their composed-view verification are owned by Issues 26 (unreadable), 29 (stale), and 30 (unsupported), each of which reaches this layout through its dependencies.
- Panel text width follows from list width, gutter and reserved column; pathological sizes never produce negative widths.
- Every text-width change caused here — list hide/show, list width recomputed after gutter growth on load, mode/size changes — is a rewrap that goes through the Issue 17 prepared-layout path and **preserves the logical reading anchor**. The file list's own rendering uses only its visible window, not a per-frame scan of all entries.

See PRD *Layout and indicators* (width bullets) and *File list and layout* stories.

### How to verify

- **Manual**: in an 80-column terminal with long paths, the list is ≤ 32 columns and paths show `…prefix/basename`; load a file with 5-digit line numbers → list narrows if needed; scroll partway into a wrapped line, `tab` hides the list → content widens and the same text remains at the top; `shift+tab` restores it, same text at top; shrink to 30 columns → the list is still present but narrow (with a 7-cell gutter and a reserved column the formula gives `min(longest+2, 12, 12)`, so expect a constrained, nonzero list — it does *not* disappear); widen again → list restored. Zero-width allocation is covered by the automated test with synthetic values.
- **Automated**: layout function tests for each of the three terms winning, the 40% floor rounding, gutter growth after load, list reduced to leave 10 text cells plus indicator, zero-width allocation preserving preference (e.g. W=20, gutter=9, indicator=1 → `W − (9+10+1) = 0`); truncation tests with wide/combining characters, including a synthetic status string occupying the note slot (the real notes arrive with Issues 26/29/30); App tests for toggle keys and list auto-scroll to the active entry; **anchor-through-relayout** tests: top mid-way through a wrapped line, then `tab`, then `shift+tab` → the top row contains the same text location each time; a load completing with a larger gutter narrows the text width and the current file's anchor location is still at the top; a file-list item-provider counting fake proves rendering touches only visible entries.

### Acceptance criteria

- [ ] Given the list is requested visible, then its width is `min(longest+2, floor(0.4·W), W − (gutter + 10 + indicator))`, never negative.
- [ ] Given a path wider than the list, then it is displayed left-truncated with a leading `…` at a grapheme boundary.
- [ ] Given `left`/`tab`, then the list hides; given `right`/`shift+tab`, then it shows; startup shows it.
- [ ] Given a terminal size that yields zero list width, then no list cells are drawn and the visibility preference is retained.
- [ ] Given the current file is outside the visible list rows, then the list scrolls to include it.
- [ ] Given a text-width change from list hide/show or gutter growth, then the logical reading anchor is preserved through the resulting rewrap.
- [ ] Given a file-list render, then only visible entries are formatted.

### User stories addressed

- User story 27: list width just wide enough, capped at 40% and by minimum content width
- User story 28: long paths left-truncated with `…`
- User story 29: `left`/`tab` hide, `right`/`shift+tab` show, initially shown

---
