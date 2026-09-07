## Issue 28: Load-completion reveal to the latest selected target

**Type**: AFK
**Blocked by**: Issue 27, Issue 19, Issue 21, Issue 23

### Parent PRD

`Notes/PRD-vrg.md`

### What to build

When a load completes for the **current** file, apply the same destination-reveal rules as file-change navigation (Issues 14 and 19: target row, visible → no scroll, one-third placement with clamping, horizontal reset then minimal horizontal reveal) to the **latest** selected cursor target — never a target captured when the load was requested.

- The target is the final display target: the start cell of the first submatch's **cluster-expanded** span (Issue 21), or the marker cell for a zero-width / terminator-only match (Issue 23). Issue 29 later substitutes the stale-entry fallback target.
- Startup: the first stop is selected immediately; once its file loads, its first match is revealed with the normal rules. Starting from top-of-file: **if the target row is already visible from top 0, the viewport stays at top 0**; only if it is hidden does it land at `floor(h/3)` (clamped at EOF). The first `n` then advances to the second stop.
- First visit (no saved per-file state): starting viewport is top-of-file with horizontal offset zero, then reveal. Revisit with a saved viewport: start from the saved viewport, then reveal only if the target is hidden.
- Reload (Issue 27) preserves the anchor by default, but if match navigation changed the selection during the load, the latest selection's reveal takes precedence on completion. **Navigation away and back** during a reload (A→B→A, or same-file stop A→stop B→stop A) counts as navigation: on completion the entry reveal for the latest selection is applied even though the final cursor value equals the initial one.
- The file-change pop-up is unaffected by load completion.

See PRD *File loading, cache, reload, and selection consistency* (second and reload bullets), *Navigation, viewport, and logical anchors* (visible-target no-scroll), and *Testing Decisions → Asynchronous App behavior*.

### How to verify

- **Manual**: with a slow first file whose first match is at line 500, startup shows "Loading…"; when it appears, line 500 is about a third down; press `n` → second stop. With a first file whose first match is at line 3, startup shows the file from the top, line 3 in place, no scroll. Reload a file, immediately press `n` to another stop in the same file → on completion the new stop is revealed, not the old position. Scroll a file so its current match is off-screen, press `r`, then `n` `p` (away and back) before the reload finishes → on completion the match is revealed.
- **Automated** (model tests with gated loads):
  - Startup, target hidden from top 0 (first match at rendered row ≥ h) → top = target − floor(h/3); offset per horizontal reveal.
  - Startup, target visible from top 0 (first match at rendered row < h, e.g. row 8 with h = 12) → top stays 0.
  - Saved-viewport revisit whose load completes: target visible from the saved top → top unchanged; hidden → one-third placement.
  - Async **marker** target: current file's first stop is a terminator-only `$` match on `hit\r\n` far down; on completion the marker's row is revealed and, in run-off-edge mode, the marker cell is painted.
  - Async **cluster** target: first submatch starts mid-cluster at column 300 in run-off-edge mode; completion reveals with the cluster fully painted at the right edge (Issue 19 rule).
  - Select stop A, then B in the same file before completion → B revealed; select A then navigate to another file → A's completion does not touch the panel.
  - Reload with no navigation → anchor preserved, no reveal. Reload with navigation to a different stop → latest reveal wins. Reload, then A→B→A (same file) with the match scrolled off-screen before `r` → on completion A's target is revealed (viewport moved), proving navigation intent rather than cursor equality decides.
  - Reload with A→file B→A → on return to A during the reload, A shows "Loading…"; on completion the entry reveal applies.
  - Pop-up timing independent of completion.

### Acceptance criteria

- [ ] Given the startup file loads after the first stop was selected and its target row is hidden from top-of-file, then the target row is placed at `floor(h/3)` (clamped at EOF) and horizontally revealed.
- [ ] Given the startup file loads and its target row is already visible from top-of-file, then the viewport remains at top 0 and the horizontal reveal alone is applied.
- [ ] Given a current-file load completes with a saved viewport from which the target is visible, then the viewport does not move.
- [ ] Given selection changes during a load of the current file, when the load completes, then the newest selection's target is revealed.
- [ ] Given a reload with no intervening navigation, then the prior anchor is preserved and no reveal is performed.
- [ ] Given a reload during which the user navigated away and back to the same stop, when the load completes, then the entry reveal is applied to that stop.
- [ ] Given the latest target is a zero-width/terminator marker or a mid-cluster start, when the load completes, then the reveal uses the marker cell / cluster-expanded span.
- [ ] Given a load completes for a file that is no longer current, then the visible panel is not affected.

### User stories addressed

- User story 41: reload preserves position unless navigation intervened
- User story 50: startup selects the first stop and reveals its match once loaded

---
