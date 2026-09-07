## Issue 13: `n`/`p` circular matched-line navigation; cursor-derived current file

**Type**: AFK
**Blocked by**: Issue 7, Issue 12

### Parent PRD

`Notes/PRD-vrg.md`

### What to build

The single global matched-line cursor in SearchIndex and its App wiring.

- Startup selects the first stop (path order, then line number). `n` advances, `p` retreats, both wrap circularly. Zero entries: no-op. Exactly one entry: strict no-op (no pop-up, no reload).
- Multiple submatches on one line are one stop.
- The current file is derived from the cursor. Crossing to another file's stop switches the panel, triggers that file's load if not cached, saves the departing file's viewport and starts the new file from its saved viewport (or top if never visited). The file list underline follows. If the file is cached but its installed layout is stale (e.g. after a resize that only requested a layout for the then-current file), the model requests a prepared layout for it; the navigation intent commits when the matching layout installs (Issue 17 owns this trigger and the prepared-layout contract; Issue 28 owns the two-stage commit).
- Manual scrolling does not move the cursor; `n`/`p` continue from the last selected stop.
- The file list is passive: no direct selection route.
- Matches on the current matched line render with the underline style (Issue 7).

Destination reveal (scrolling to the match) is Issue 14; for now, same-file navigation just updates the current-line styling.

See PRD *Navigation, viewport, and logical anchors* (first three bullets) and *Module Design → SearchIndex*.

### How to verify

- **Manual**: with several files matched, press `n` repeatedly: current-line underline moves through lines, then to the next file, list underline follows; after the last stop `n` returns to the first; `p` reverses. Scroll manually, then `n` continues from the old stop.
- **Automated**: SearchIndex tests for next/prev wrap, file-change flag, single-stop no-op, empty no-op; App tests: startup cursor at first stop; `n` across a file boundary switches the panel and requests a load; scroll then `n` continues from the previous stop; one-stop index ignores `n`/`p`.

### Acceptance criteria

- [ ] Given startup, then the cursor is on the first matched line of the first file in path order.
- [ ] Given the last stop, when `n` is pressed, then the first stop is selected; given the first stop, when `p` is pressed, then the last stop is selected.
- [ ] Given exactly one stop, when `n` or `p` is pressed, then nothing happens.
- [ ] Given a stop in a different file, when selected, then the file panel switches, its load is requested if uncached, and the list underline moves.
- [ ] Given manual scrolling, when `n` is pressed afterwards, then navigation proceeds from the previously selected stop.

### User stories addressed

- User story 30: list is a passive overview; selection only via matched-line navigation
- User story 49: `n`/`p` navigate matched source lines, path then line order
- User story 51: circular wrap; single-entry no-op
- User story 56: manual scrolling leaves the cursor unchanged

---
