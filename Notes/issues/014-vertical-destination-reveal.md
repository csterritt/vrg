## Issue 14: Vertical destination reveal — target row, no-scroll if visible, one-third placement

**Type**: AFK
**Blocked by**: Issue 13

### Parent PRD

`Notes/PRD-vrg.md`

### What to build

On startup (once content is loaded) and on every actual `n`/`p` transition, reveal the **rendered row containing the start cell of the first submatch** on the destination line (not merely any row of the source line — this matters once wrapping exists in Issue 16; implement the target as a display location from the outset).

- If the target row is already visible, do not scroll vertically.
- Otherwise move the viewport so the target row sits at zero-based row `floor(content height / 3)`, clamped to valid top positions; at BOF/EOF available content takes precedence over one-third placement.
- On file change: start from the saved per-file viewport (revisit) or top of file (first visit, including the startup file), then apply the reveal.
- A reveal that moves the viewport replaces the saved vertical state; a no-scroll reveal leaves it.

Horizontal reveal is Issue 19; load-completion reveal to the latest target is Issue 28.

See PRD *Navigation, viewport, and logical anchors* (bullets 3–6) and *Testing Decisions → Viewport*.

### How to verify

- **Manual**: in a long file with matches at lines 5 and 200, `n` from line 5 → viewport jumps so line 200 is about a third down; `p` back → line 5 is visible near the top (BOF clamp); with two matches already on screen, `n` between them does not scroll.
- **Automated**: Viewport tests: visible target → top unchanged; hidden target far down → top = target − floor(h/3); target near BOF → top 0; target near EOF → EOF clamp; startup reveal after load places first match correctly; revisit uses saved viewport and is overridden only when the target is hidden; first visit starts from top then reveals.

### Acceptance criteria

- [ ] Given the destination target row is on screen, when navigating, then the vertical viewport does not change.
- [ ] Given the target row is off screen, when navigating, then it lands at row `floor(h/3)` unless BOF/EOF clamping prevents it.
- [ ] Given a file being revisited, when navigating into it, then its saved viewport is the starting point and the reveal only adjusts if needed.
- [ ] Given a first visit (including at startup), then the starting viewport is the top of the file before reveal.

### User stories addressed

- User story 52: reveal the rendered row containing the first submatch's start
- User story 53: visible target stays put; hidden target lands a third down
- User story 57: revisits use saved viewport, first visits start at top, reveal takes precedence

---
