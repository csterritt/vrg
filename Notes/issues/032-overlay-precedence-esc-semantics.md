## Issue 32: Overlay precedence — error suspends help, append preserves scroll, `Esc`/`q` dismissal semantics

**Type**: AFK
**Blocked by**: Issue 31, Issue 15

### Parent PRD

`Notes/PRD-vrg.md`

### What to build

- Key precedence in browse/no-results states: `ctrl+c` → modal error → help → pop-up → base keys.
- A new error while help is open suspends help, retaining its scroll position; dismissing the error restores help at that position.
- New errors append to the error overlay without moving the reader's scroll position.
- Opening help or an error cancels any pop-up; no suspended pop-up returns afterward.
- **`Esc` is not a standalone quit command.** It only ever dismisses: it closes help or an error overlay; with neither open (browsing, no-results, searching, too-small) it does nothing except dismiss a pop-up like any other key. Whether dismissal *leads to* termination depends on the base state, exactly as for `q`, per the PRD outcome table:
  - Browse-state overlay (error or help, including error-over-help) → dismiss returns to browsing (or to suspended help). `q` and `Esc` behave identically here.
  - Nonfatal warning overlay over an empty result → dismiss shows the no-results screen. `q` and `Esc` identical.
  - **Fatal overlay with no usable results** (fatal process/integrity failure, or record-loss with zero results) → dismissal with **either** `q` or `Esc` exits 2. There is no state to return to; this is the one route by which `Esc` ends the program, and it is dismissal, not quit.
  - No overlay: `q` quits with the fixed status (browse) or 1 (no-results); `Esc` does not request exit.
  - Too-small screen: Issue 33's dedicated rule takes precedence (`q` exits even if a modal overlay is logically open); `Esc` is a no-op there.

See PRD *Colours, overlays, and key precedence* (precedence, `Esc`, error/help bullets) and *Outcome and exit-status contract* (table and `q`/`Esc` bullet).

### How to verify

- **Manual**:
  1. Fake rg with two files where file 2 is made unreadable, plus stderr "warn": startup shows the warning overlay; `Esc` dismisses. Press `?`, scroll help down, then press `Esc` (close help) and `n` into file 2 → error overlay (unreadable); press `Esc` → browsing (help was not open at that moment, so it does not return). Now open `?`, scroll, and press `r` — ignored while help is open. This manual route cannot produce an error *while help is open*; that case is verified by the deterministic gated model test below.
  2. `Esc` with no overlay → nothing. `q` with an error overlay open in browse → error closes, app still running; `q` again → exits with the fixed status.
  3. Fake rg exiting 3 with no output → fatal overlay; `Esc` → the program exits 2 (compare with `q` → also 2).
- **Automated** (model tests):
  - Help scrolled to row 5, gated current-file load failure released while help is open → error shown; dismiss with `Esc` → help at row 5; repeat dismissing with `q` → help at row 5.
  - Error at scroll 3, second error appended → still at 3 and new text present.
  - Pop-up active, `?` → pop-up gone, help open, close help → no pop-up; pop-up active, injected error → same.
  - `Esc` no-op in browsing and no-results (searching is covered by Issue 4; too-small by Issue 33).
  - **Dismissal-outcome table**, run for both `q` and `Esc` as the dismissal key: browse + error overlay → browsing, still running; browse + help → browsing; browse + error-over-help → help restored; empty result + warning overlay → no-results screen, still running; fatal, no results → exit 2; record-loss, no results → exit 2. Then a second `q` from each still-running state → the fixed status (0/2) or 1, and a second `Esc` → still running.
  - Precedence when both help and error are open: keys route to the error.

### Acceptance criteria

- [ ] Given help is open at scroll position S, when an error arrives, then the error overlay is shown; when dismissed with `q` or `Esc`, then help is visible at S.
- [ ] Given an error overlay scrolled to position P, when another error is appended, then the position remains P and the new text is reachable.
- [ ] Given a pop-up is visible, when help or an error opens, then the pop-up is cancelled and does not reappear.
- [ ] Given no overlay is open in browse or no-results, when `Esc` is pressed, then nothing happens (other than dismissing a pop-up), and the program does not exit.
- [ ] Given a browse-state overlay or a warning overlay over an empty result, when `q` or `Esc` is pressed, then only the overlay closes and the program continues in the underlying state.
- [ ] Given a fatal overlay with no usable results, when `q` or `Esc` is pressed, then the program exits 2.
- [ ] Given no overlay in browse, when `q` is pressed, then the program exits with the fixed status; given the no-results screen, then 1.

### User stories addressed

- User story 80: errors suspend help and restore its scroll position
- User story 81: new errors appended without moving the reader
- User story 84: `q`/`Esc` dismissal semantics; `Esc` is never a quit command; `ctrl+c` always 130

---
