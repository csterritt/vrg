## Issue 31: Help overlay (`h`/`?`) with wrapped, scrollable key bindings

**Type**: AFK
**Blocked by**: Issue 9

### Parent PRD

`Notes/PRD-vrg.md`

### What to build

- `h`/`?` open a modal help overlay listing every key binding (navigation, scrolling, panning, wrap, colour, list toggle, reload, help, quit/cancel).
- While open: `up`/`down` scroll rendered rows; `q`/`Esc`/`h`/`?` close; `ctrl+c` exits 130; every other key is ignored (nothing reaches the content behind).
- Text wraps to the interior width, including long unbroken strings; vertical scrolling reaches every row at usable sizes. Any substituted text (e.g. paths) is sanitized.
- At tiny sizes the overlay is clipped to the terminal without a special borderless mode; growing the terminal restores the normal layout.
- Overlay uses Theme overlay style (base colours, single-line border). Shares the wrapped-scrollable overlay component with the error overlay from Issue 9.

See PRD *Colours, overlays, and key precedence* (help bullets and clipping bullet).

### How to verify

- **Manual**: press `?` → bordered help; `down` scrolls; `n` does nothing to the file behind; `Esc` closes; shrink the terminal to 25×8 → help is clipped but present; enlarge → normal.
- **Automated**: model tests: `h` and `?` open; each close key closes; `n`/`p`/`w`/`c`/`r` ignored while open (state behind unchanged); `ctrl+c` exits 130; scroll bounds; long unbroken line wraps within width; rendering at 25×8 does not panic and is clipped; content lists every binding string.

### Acceptance criteria

- [ ] Given browsing, when `h` or `?` is pressed, then a modal help overlay listing all key bindings appears.
- [ ] Given help is open, when any key other than `up`/`down`/`q`/`Esc`/`h`/`?`/`ctrl+c` is pressed, then nothing changes.
- [ ] Given help is open, when `q`, `Esc`, `h`, or `?` is pressed, then help closes and browsing resumes.
- [ ] Given help text wider than the interior, then it wraps (including unbroken strings) and can be scrolled fully.
- [ ] Given a tiny terminal above the 20×3 minimum, then the overlay is clipped and restored on growth.

### User stories addressed

- User story 78: `h`/`?` open modal help
- User story 79: help accepts only scrolling, close keys, and `ctrl+c`
- User story 82: help and diagnostics wrapped and scrollable
- User story 83: clipped overlays at tiny sizes

---
