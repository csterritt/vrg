## Issue 31: Help overlay (`h`/`?`) with wrapped, scrollable key bindings

**Type**: AFK
**Blocked by**: Issue 6, Issue 9, Issue 15

### Parent PRD

`Notes/PRD-vrg.md`

### What to build

- `h`/`?` open a modal help overlay listing every key binding (navigation, scrolling, panning, wrap, colour, list toggle, reload, help, quit/cancel).
- Help opens from both ordinary browsing and the no-results screen; closing it returns to the underlying base state (the PRD's modal-precedence model covers browse and no-results states).
- While open: `up`/`down` scroll rendered rows; `q`/`Esc`/`h`/`?` close; `ctrl+c` exits 130; every other key is ignored (nothing reaches the content behind).
- Text wraps to the interior width, including long unbroken strings; vertical scrolling reaches every row at usable sizes. Any substituted text (e.g. paths) is sanitized.
- At tiny sizes the overlay is clipped to the terminal without a special borderless mode; growing the terminal restores the normal layout.
- Overlay uses Theme overlay style (base colours, single-line border). Shares the wrapped-scrollable overlay component with the error overlay from Issue 9.
- Opening help cancels an active file-change pop-up (Issue 15); it does not return when help closes.
- Help substitutions are routed through the Issue 6 utility; add help to the sink-safety table.
- The key-binding list is defined once as data (binding → description) that both the help renderer and Issue 34's documentation test consume, plus a footer slot for Issue 34's scale/limits note.

See PRD *Colours, overlays, and key precedence* (help bullets and clipping bullet).

### How to verify

- **Manual**: press `?` → bordered help; `down` scrolls; `n` does nothing to the file behind; `Esc` closes; shrink the terminal to 25×8 → help is clipped but present; enlarge → normal.
- **Automated**: model tests: `h` and `?` open; each close key closes; `h` and `?` from the no-results screen open help, other keys stay ignored there, each close key returns to the no-results screen, and a subsequent `q` exits 1; `n`/`p`/`w`/`c`/`r` ignored while open (state behind unchanged); `ctrl+c` exits 130; scroll bounds; long unbroken line wraps within width; rendering at 25×8 does not panic and is clipped; content lists every binding string from the binding table; pop-up active then `?` → pop-up gone, help open, close help → no pop-up; a hostile substitution string passes the Issue 6 sink-safety check.

### Acceptance criteria

- [ ] Given browsing, when `h` or `?` is pressed, then a modal help overlay listing all key bindings appears.
- [ ] Given help is open, when any key other than `up`/`down`/`q`/`Esc`/`h`/`?`/`ctrl+c` is pressed, then nothing changes.
- [ ] Given help is open, when `q`, `Esc`, `h`, or `?` is pressed, then help closes and browsing resumes.
- [ ] Given help text wider than the interior, then it wraps (including unbroken strings) and can be scrolled fully.
- [ ] Given a tiny terminal above the 20×3 minimum, then the overlay is clipped and restored on growth.
- [ ] Given a pop-up is visible, when help opens, then the pop-up is cancelled and does not reappear.
- [ ] Given the no-results screen, when `h` or `?` is pressed, then help opens; when it is closed, then the no-results screen returns and `q` exits 1.
- [ ] Given the help overlay, then its bindings are rendered from a single binding table exposed for documentation tests.

### User stories addressed

- User story 78: `h`/`?` open modal help
- User story 79: help accepts only scrolling, close keys, and `ctrl+c`
- User story 82: help and diagnostics wrapped and scrollable
- User story 83: clipped overlays at tiny sizes

---
