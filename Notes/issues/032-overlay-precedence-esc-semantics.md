## Issue 32: Overlay precedence — error suspends help, append preserves scroll, `Esc` semantics

**Type**: AFK
**Blocked by**: Issue 31, Issue 15

### Parent PRD

`Notes/PRD-vrg.md`

### What to build

- Key precedence in browse/no-results states: `ctrl+c` → modal error → help → pop-up → base keys.
- A new error while help is open suspends help, retaining its scroll position; dismissing the error restores help at that position.
- New errors append to the error overlay without moving the reader's scroll position.
- Opening help or an error cancels any pop-up; no suspended pop-up returns afterward.
- `Esc` is dismissal-only: it closes help or an error overlay; with neither open (browsing, no-results, searching, too-small) it is a no-op except for dismissing a pop-up like any other key. `Esc` never exits. `q` dismisses an overlay when one is open, otherwise quits with the fixed status.

See PRD *Colours, overlays, and key precedence* (precedence, `Esc`, error/help bullets) and *Outcome and exit-status contract* (`q`/`Esc` bullet).

### How to verify

- **Manual**: open help and scroll down; trigger a late current-file load failure (e.g. `chmod 000` the next file, `Esc` help, `n`, `?` then… ) — easier via test; `Esc` with no overlay → nothing; `q` with error open → error closes, app still running; `q` again → exits.
- **Automated**: model tests: help scrolled to row 5, error arrives → error shown; dismiss → help at row 5; error at scroll 3, second error appended → still at 3 and new text present; pop-up active, `?` → pop-up gone, help open, close help → no pop-up; `Esc` no-op in browsing/no-results/searching/too-small; `Esc` closes help and error; `q` dismisses before quitting; precedence when both help and error are open.

### Acceptance criteria

- [ ] Given help is open at scroll position S, when an error arrives, then the error overlay is shown; when dismissed, then help is visible at S.
- [ ] Given an error overlay scrolled to position P, when another error is appended, then the position remains P and the new text is reachable.
- [ ] Given a pop-up is visible, when help or an error opens, then the pop-up is cancelled and does not reappear.
- [ ] Given no overlay is open, when `Esc` is pressed, then nothing happens (other than dismissing a pop-up).
- [ ] Given an overlay is open, when `q` is pressed, then only the overlay closes; given no overlay, then the program exits with the fixed status.

### User stories addressed

- User story 80: errors suspend help and restore its scroll position
- User story 81: new errors appended without moving the reader
- User story 84: `q`/`Esc` dismissal semantics; `Esc` never quits; `ctrl+c` always 130

---
