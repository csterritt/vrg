## Issue 28: Load-completion reveal to the latest selected target

**Type**: AFK
**Blocked by**: Issue 27, Issue 19

### Parent PRD

`Notes/PRD-vrg.md`

### What to build

When a load completes for the **current** file, apply the same destination-reveal rules as file-change navigation (Issues 14 and 19: target row, visible → no scroll, one-third placement with clamping, horizontal reset then minimal horizontal reveal) to the **latest** selected cursor target — never a target captured when the load was requested.

- Startup: the first stop is selected immediately; once its file loads, its first match is revealed with the normal rules (lands at `floor(h/3)` if not otherwise visible from top-of-file). The first `n` then advances to the second stop.
- First visit (no saved per-file state): starting viewport is top-of-file with horizontal offset zero, then reveal.
- Reload (Issue 27) preserves the anchor by default, but if match navigation changed the selection during the load, the latest selection's reveal takes precedence on completion.
- The file-change pop-up is unaffected by load completion.

See PRD *File loading, cache, reload, and selection consistency* (second bullet) and *Testing Decisions → Asynchronous App behavior*.

### How to verify

- **Manual**: with a slow first file whose first match is at line 500, startup shows "Loading…"; when it appears, line 500 is about a third down; press `n` → second stop. Reload a file, immediately press `n` to another stop in the same file → on completion the new stop is revealed, not the old position.
- **Automated**: model tests with gated loads: startup file completes → first match at row `floor(h/3)`, offset per horizontal reveal; select stop A, then B in the same file before completion → B revealed; select A then navigate to another file → A's completion does not touch the panel; reload with no navigation → anchor preserved; reload with navigation → latest reveal wins; pop-up timing independent of completion.

### Acceptance criteria

- [ ] Given the startup file loads after the first stop was selected, then the first match's row is placed at `floor(h/3)` (or clamped) and horizontally revealed.
- [ ] Given selection changes during a load of the current file, when the load completes, then the newest selection's target is revealed.
- [ ] Given a reload with no intervening navigation, then the prior anchor is preserved and no reveal is performed.
- [ ] Given a load completes for a file that is no longer current, then the visible panel is not affected.

### User stories addressed

- User story 41: reload preserves position unless navigation intervened
- User story 50: startup selects the first stop and reveals its match once loaded

---
