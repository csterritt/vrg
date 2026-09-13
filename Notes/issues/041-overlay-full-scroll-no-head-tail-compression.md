## Issue 41: Error overlays keep every row scrollable — no destructive head/tail compression

**Type**: AFK
**Blocked by**: None — can start immediately

### Parent PRD

`Notes/PRD-vrg.md` — audit source: `Notes/critiques/final-audit-vrg.md`, High finding 6

### What to build

Fix overlay truncation in `internal/app/app.go:2384-2459`. Non-help overlays longer than the visible height are currently compressed to head + ellipsis + tail *before* scrolling, so the omitted middle rows can never be reached. This contradicts the requirement that diagnostics wrap and remain vertically scrollable and accessible at usable terminal sizes — and is especially damaging for large ripgrep stderr output or long appended-error lists, exactly the cases where scrolling matters most.

- Keep all wrapped overlay rows in the scrollable row set; clamp `overlayScroll` over the complete set rather than a pre-truncated one.
- Every row of a long diagnostic is reachable by scrolling; nothing is elided from the model.
- If head/tail summarisation is ever wanted, it must be an explicit alternate view the user chooses — not destructive preprocessing. (Not required by this issue.)

See PRD *Colours, overlays, and key precedence* (wrap + vertical scrolling) and user stories 80–83.

### How to verify

- **Manual**: trigger a fatal outcome with a fake rg that emits stderr longer than the overlay's visible height → open the overlay and scroll from the first row to the last with repeated `up`/`down` (the error-overlay contract accepts only `up`, `down`, `q`, `Esc`, and `ctrl+c`; `u`/`d`/page keys are base file-content bindings and must be ignored), confirming every middle row is reachable and nothing is replaced by an ellipsis; dismiss and confirm scroll clamping at both ends.
- **Automated**: overlay tests asserting the wrapped row count equals the full diagnostic content (no ellipsis marker injected), scroll offset clamps to `[0, rows-height]`, and a row-by-row `up`/`down` scroll traversal reaches the final line; regression test that an appended error extends the scrollable set rather than being summarised away; keep the existing ignored-key regression asserting `u`, `d`, page up, and page down have no effect while an error overlay is open.

### Acceptance criteria

- [ ] Given a diagnostic longer than the visible overlay height, then all wrapped rows remain in the scrollable set and every row is reachable by `up`/`down` scrolling; the modal key contract is unchanged — `u`, `d`, page up, and page down remain ignored while the overlay is open.
- [ ] Given scrolling through a long overlay, then no head/tail ellipsis substitutes for content rows.
- [ ] Given new errors appended while an overlay is open, then they extend the scrollable content per the existing append contract without moving the reader's position.
- [ ] Given tiny terminal sizes, then the existing clipped-overlay acceptance (story 83) still applies — clipping at render time is fine, removal from the scrollable set is not.
- [ ] Given the help overlay, then its scrolling behaviour is unchanged.

### User stories addressed

- User story 82: help and diagnostics wrapped and vertically scrollable, accessible at usable sizes
- User story 81: new errors appended without moving the diagnostic reader
- User story 80: errors suspend help and restore its scroll position
- User story 83: clipped overlays at tiny sizes accepted (render-time clipping only)

---
