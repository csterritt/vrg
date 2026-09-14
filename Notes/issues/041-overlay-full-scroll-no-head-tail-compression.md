## Issue 41: Error overlays keep every row scrollable — no destructive head/tail compression

**Type**: AFK
**Blocked by**: None — can start immediately. Shared-file ordering: this issue and Issue 48 both revise `cmd/vrg/outcome_test.go`; do not implement them concurrently. If both are pending, land Issue 48's handshake refactor first, then adapt the overlay regressions to that harness (or land this issue completely before Issue 48 begins).

### Parent PRD

`Notes/PRD-vrg.md` — audit source: `Notes/critiques/final-audit-vrg.md`, High finding 6

### What to build

Fix overlay truncation in `internal/app/app.go:2384-2459`. Non-help overlays longer than the visible height are currently compressed to head + ellipsis + tail *before* scrolling, so the omitted middle rows can never be reached. This contradicts the requirement that diagnostics wrap and remain vertically scrollable and accessible at usable terminal sizes — and is especially damaging for large ripgrep stderr output or long appended-error lists, exactly the cases where scrolling matters most.

- Keep all wrapped overlay rows in the scrollable row set; clamp `overlayScroll` over the complete set rather than a pre-truncated one.
- Every row of a long diagnostic is reachable by scrolling; nothing is elided from the model.
- This full-scroll contract explicitly supersedes Issue 9's original requirement that the ≥ 1 MiB stderr fixture show both head and tail in one rendered frame. Revise `TestStderrContentFixture` accordingly: retain its complete-stdout, captured-stderr inclusion, pipe-drainage, and completion assertions, but do not require simultaneous head/tail visibility. Do not weaken tail verification; move that proof to the complete-row model and a bounded traversal fixture as described below.
- If head/tail summarisation is ever wanted, it must be an explicit alternate view the user chooses — not destructive preprocessing. (Not required by this issue.)

See PRD *Colours, overlays, and key precedence* (wrap + vertical scrolling) and user stories 80–83.

### How to verify

- **Manual**: trigger a fatal outcome with a fake rg that emits stderr longer than the overlay's visible height → open the overlay and scroll from the first row to the last with repeated `up`/`down` (the error-overlay contract accepts only `up`, `down`, `q`, `Esc`, and `ctrl+c`; `u`/`d`/page keys are base file-content bindings and must be ignored), confirming every middle row is reachable and nothing is replaced by an ellipsis; dismiss and confirm scroll clamping at both ends.
- **Automated**: model-level overlay tests assert that the scrollable row set equals the complete wrapped diagnostic (including both first and last markers), no ellipsis marker is injected, and `overlayScroll` clamps exactly to `[0, max(0, rows-height)]`, proving tail reachability even for the ≥ 1 MiB content shape without sending thousands of PTY keys. Separately, use a moderately oversized diagnostic whose wrapped row count is only slightly greater than `maxVisible` for a bounded row-by-row `down` traversal to the final line and `up` traversal back to the first; this may be a model test or a PTY test using Issue 48's handshake harness. Revise `TestStderrContentFixture` to retain its large-pipe drainage/completeness checks without asserting head and tail are simultaneously rendered. Add a regression that an appended error extends the scrollable set rather than being summarised away; keep the existing ignored-key regression asserting `u`, `d`, page up, and page down have no effect while an error overlay is open.

### Acceptance criteria

- [ ] Given a diagnostic longer than the visible overlay height, then the model's scrollable rows equal the complete wrapped content, `overlayScroll` clamps to `[0, max(0, rows-height)]`, and every row is reachable by `up`/`down` scrolling; the modal key contract is unchanged — `u`, `d`, page up, and page down remain ignored while the overlay is open.
- [ ] Given a moderately oversized fixture, then a bounded `down` traversal reaches its final row and an `up` traversal returns to its first row without an ellipsis substituting for content.
- [ ] Given `TestStderrContentFixture` with ≥ 1 MiB stderr, then its stdout/drainage/completion and captured-diagnostic assertions remain intact, while complete model rows and clamp/traversal tests — not simultaneous head/tail rendering — prove tail reachability.
- [ ] Given new errors appended while an overlay is open, then they extend the scrollable content per the existing append contract without moving the reader's position.
- [ ] Given tiny terminal sizes, then the existing clipped-overlay acceptance (story 83) still applies — clipping at render time is fine, removal from the scrollable set is not.
- [ ] Given the help overlay, then its scrolling behaviour is unchanged.

### User stories addressed

- User story 82: help and diagnostics wrapped and vertically scrollable, accessible at usable sizes
- User story 81: new errors appended without moving the diagnostic reader
- User story 80: errors suspend help and restore its scroll position
- User story 83: clipped overlays at tiny sizes accepted (render-time clipping only)

---
