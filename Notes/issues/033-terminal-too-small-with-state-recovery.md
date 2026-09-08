## Issue 33: "Terminal too small" screen with full state recovery

**Type**: AFK
**Blocked by**: Issue 32, Issue 24

### Parent PRD

`Notes/PRD-vrg.md`

### What to build

- Below 20 columns or 3 rows show centred "Terminal too small" as space permits.
- Only `q` and `ctrl+c` are active: `q` exits with the applicable outcome (130 if still searching, else the fixed status / 1 / 2 as per state); `ctrl+c` exits 130. `Esc` and every other key are no-ops.
- Preserve for recovery on resize: cursor selection, per-file viewport state and logical anchors, list visibility preference, wrap and colour settings, horizontal offset, and full modal state (which overlay is open, help scroll, error scroll, help-suspended-by-error relationship). All survive a too-small round trip unchanged.
- An active pop-up continues its timer and is not displayed on the too-small screen.
- The screen must not trap the user behind an invisible modal overlay (i.e. `q` still exits from too-small even if an overlay is logically open). This too-small `q` rule takes precedence over Issue 32's dismissal semantics; `Esc` remains a no-op on the too-small screen and does not dismiss the hidden overlay.

See PRD *Layout and indicators* (minimum-size bullet).

### How to verify

- **Manual**: open help, scroll it, shrink the terminal to 15×2 → "Terminal too small"; enlarge → help reappears at the same scroll; shrink again and press `q` → exits.
- **Automated**: model tests: resize to 19×10 and 40×2 → too-small; resize back restores view; round trips with a scrolled help overlay, a scrolled error overlay, and an error-over-help stack all restore scroll positions and relationship; a resize wholly within the too-small state (19×2 → 10×1 → 25×8) installs no ordinary layout at 10×1 and recovers at the final 25×8 dimensions with a scrolled help overlay, an error-over-help stack, and a nontrivial viewport state all preserved without loss; `q` on too-small during searching → 130, while browsing → fixed status, on no-results → 1, with a fatal no-results overlay logically open → 2; `q` on too-small with a browse error overlay logically open → exits with the fixed status (not merely dismissing); `Esc` on too-small with an overlay logically open → no-op and the overlay is still open after recovery; `n`/`w` no-ops; pop-up expiry arriving during too-small dismisses it so it is absent after recovery.

### Acceptance criteria

- [ ] Given a terminal below 20×3, then "Terminal too small" is displayed and only `q`/`ctrl+c` act.
- [ ] Given the too-small screen, when the terminal grows, then cursor, viewport anchors, list/wrap/colour settings, horizontal offset, and the exact overlay stack with scroll positions are restored — including when one or more resizes occur wholly within the too-small state before recovery, in which case no ordinary layout is installed at the pathological dimensions and recovery uses the final dimensions.
- [ ] Given the too-small screen while searching, when `q` is pressed, then exit 130; while browsing, then the fixed search-derived status; on no-results, then 1.
- [ ] Given the too-small screen with a modal overlay logically open, when `q` is pressed, then the program exits (it does not merely dismiss); when `Esc` is pressed, then nothing changes and the overlay is still open after recovery.
- [ ] Given a pop-up was active, then its timer continues during too-small and it is not shown there.

### User stories addressed

- User story 32: too-small screen with recovery preserving selection and overlay state

---
