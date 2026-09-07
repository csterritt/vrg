## Issue 28: Load-completion reveal to the latest selected target through the prepared layout

**Type**: AFK
**Blocked by**: Issue 15, Issue 17, Issue 19, Issue 21, Issue 23, Issue 27

### Parent PRD

`Notes/PRD-vrg.md`

### What to build

The two-stage completion contract for a current-file load: a load completion produces a decoded/mapped buffer, and reveal commits only when a prepared layout matching the model's current parameters installs. This issue owns the integrated transition; Issues 17 and 27 provide its pieces.

**Stage 1 — load completion.** When a load completes for path P:

- Validate content (stale validation arrives with Issue 29), establish the new content revision, and compute the final gutter width. Update only P's cache/status; the visible panel is affected only if P is current (Issue 25).
- If P is current, first recompute the text width — gutter growth or file-list changes caused by this load participate (Issue 24) — then request prepared layout for the current (P, revision, text width, wrap mode). Load completion performs no row-based decision itself: no visibility test, no one-third placement, no clamping, no horizontal reveal.

**Stage 2 — pending intent and commit.**

- **Reveal intent** is recorded for the **latest** selected cursor target — never a target captured when the load was requested. The target is the final display target: the start cell of the first submatch's **cluster-expanded** span (Issue 21), or the marker cell for a zero-width / terminator-only match (Issue 23). Issue 29 later substitutes the stale-entry fallback target.
- **Reload intent**: if the completion was a reload with no intervening navigation, the intent is anchor preservation (Issue 27), not reveal. Navigation during the load — including away-and-back (A→B→A, or same-file stop A→stop B→stop A) — replaces it with the entry reveal for the latest selection, even when the final cursor value equals the initial one.
- The intent is carried by the model, not by any in-flight layout. Obsolete layouts — wrong width, mode, revision, or path (Issue 17) — are discarded **without consuming or mutating** the intent.
- The intent commits when, and only when, a prepared layout matching the model's current (path, content revision, text width, wrap mode) installs: visible-target no-scroll, else one-third placement with BOF/EOF clamping, plus horizontal reset then minimal horizontal reveal (Issues 14/19) — all evaluated against the installed row model. Until then the panel may keep the previous layout or a placeholder while input remains actionable (Issue 17).
- Starting viewports: a first visit (including the startup file) starts from top-of-file with horizontal offset zero; a revisit starts from its saved per-file viewport. If the target row is already visible from the starting viewport, the viewport stays; only a hidden target moves.
- Startup: the first stop is selected immediately; its reveal commits once the startup file's first matching layout installs. **If the target row is already visible from top 0, the viewport stays at top 0**; only if it is hidden does it land at `floor(h/3)` (clamped at EOF). The first `n` then advances to the second stop.
- The file-change pop-up (Issue 15) is unaffected by load or layout completion.

See PRD *File loading, cache, reload, and selection consistency* (second and reload bullets), *Navigation, viewport, and logical anchors* (visible-target no-scroll and file-change reveal sequence), *Resources and responsiveness* (prepared viewport data), and *Testing Decisions → Asynchronous App behavior / Responsiveness boundaries*.

### How to verify

- **Manual**: with a slow first file whose first match is at line 500, startup shows "Loading…"; when it appears, line 500 is about a third down; press `n` → second stop. With a first file whose first match is at line 3, startup shows the file from the top, line 3 in place, no scroll. Reload a file, immediately press `n` to another stop in the same file → on completion the new stop is revealed, not the old position. Scroll a file so its current match is off-screen, press `r`, then `n` `p` (away and back) before the reload finishes → on completion the match is revealed. Resize the terminal while the first file is still loading → the reveal lands correctly for the new size.
- **Automated** (model tests with **separately gated loads and layout preparation**):
  - Startup, target hidden from top 0 (first match at rendered row ≥ h) → after both the load and its layout complete, top = target − floor(h/3); offset per horizontal reveal.
  - Startup, target visible from top 0 (first match at rendered row < h, e.g. row 8 with h = 12) → top stays 0 after both stages.
  - **Startup load completes, then a resize arrives before its layout does** → the old-width layout is discarded, the intent survives, and the reveal commits against the new width when its layout installs.
  - **`n`/`p` while the current layout is pending** → the cursor moves immediately; when the matching layout installs, the newest target is revealed (not the one current when the load completed).
  - **Reload with old-revision and new-revision layouts completing out of order** → with no navigation, the anchor commits against the new revision's layout; with navigation during the load, the latest selection's reveal commits; a late old-revision layout is discarded without consuming the intent.
  - **Gutter growth (e.g. 5-digit line numbers) or list hide/show between load and layout completion** → the intent commits against the final text width.
  - Saved-viewport revisit whose load completes: target visible from the saved top → top unchanged; hidden → one-third placement.
  - Async **marker** target: current file's first stop is a terminator-only `$` match on `hit\r\n` far down; on commit the marker's row is revealed and, in run-off-edge mode, the marker cell is painted.
  - Async **cluster** target: first submatch starts mid-cluster at column 300 in run-off-edge mode; on commit the cluster is fully painted at the right edge (Issue 19 rule).
  - Select stop A, then B in the same file before completion → B revealed; select A then navigate to another file → A's completion does not touch the panel.
  - Reload with no navigation → anchor preserved, no reveal. Reload with navigation to a different stop → latest reveal wins. Reload, then A→B→A (same file) with the match scrolled off-screen before `r` → on commit A's target is revealed (viewport moved), proving navigation intent rather than cursor equality decides. Reload with A→file B→A → on return to A during the reload, A shows "Loading…"; on commit the entry reveal applies.
  - Pop-up timing independent of completion.

### Acceptance criteria

- [ ] Given a load completion for the current file, then the viewport does not change at load-completion time; a prepared layout is requested for the current (path, revision, text width, wrap mode) and the commit happens when that layout installs.
- [ ] Given reveal or reload-anchor intent, when obsolete layouts arrive, then they are discarded without consuming or mutating the intent.
- [ ] Given the startup file loads and its target row is hidden from top-of-file, then once the matching layout installs, the target row is placed at `floor(h/3)` (clamped at EOF) and horizontally revealed.
- [ ] Given the startup file loads and its target row is already visible from top-of-file, then the viewport remains at top 0 and the horizontal reveal alone is applied.
- [ ] Given a current-file load completes with a saved viewport from which the target is visible, then the viewport does not move.
- [ ] Given selection changes during a load of the current file, when the matching layout installs, then the newest selection's target is revealed.
- [ ] Given a reload with no intervening navigation, then the prior anchor is preserved on commit and no reveal is performed.
- [ ] Given a reload during which the user navigated away and back to the same stop, when the matching layout installs, then the entry reveal is applied to that stop.
- [ ] Given the latest target is a zero-width/terminator marker or a mid-cluster start, then the committed reveal uses the marker cell / cluster-expanded span.
- [ ] Given a load or layout completes for a file that is no longer current, then the visible panel is not affected.

### User stories addressed

- User story 41: reload preserves position unless navigation intervened
- User story 50: startup selects the first stop and reveals its match once loaded

---
