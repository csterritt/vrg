## Issue 7: Theme — `c` colour toggle, inverse matches, current-line underline

**Type**: AFK
**Blocked by**: Issue 5

### Parent PRD

`Notes/PRD-vrg.md`

### What to build

The Theme module and its wiring into rendering.

- Dark scheme initially (white on black); `c` toggles to light (black on white) and back. No persistence.
- Styles supplied by Theme: base, gutter, inverse match, underlined current match, indicator (inverse), overlay (base colours, plain single-line border), filename rule, file list, current-file underline.
- Matches use true inverse of the active scheme; matches on the current matched line are additionally underlined; the current file-list entry is underlined. (Until Issue 13 the "current matched line" is the first stop.)

See PRD *Colours, overlays, and key precedence* (first bullet) and *Module Design → Theme*.

### How to verify

- **Manual**: run a search against a fixture with matches (for example `vrg func .` in a Go repository, as in Issue 5), wait for the browse view, then press `c`; background/foreground swap, matches remain inverse, current-line matches underlined; press `c` again to return. (Bare `vrg` prints command-line help and exits; it cannot demonstrate theme switching.)
- **Automated**: Theme tests assert both schemes' foreground/background pairs, that match style is the true inverse in each scheme, current-match style adds underline, current-file style is underlined, overlay style uses base colours. App test: `c` toggles `View()` styling between schemes.

### Acceptance criteria

- [ ] Given startup, then the dark scheme is active.
- [ ] Given `c`, when pressed, then the scheme flips; when pressed again, then it flips back.
- [ ] Given either scheme, then match cells are the inverse of base colours and current-matched-line match cells are also underlined.
- [ ] Given the file list, then the current entry is underlined in both schemes.

### User stories addressed

- User story 76: `c` toggles white-on-black / black-on-white, initially dark
- User story 77: inverse matches, underlined on the current matched line

---
