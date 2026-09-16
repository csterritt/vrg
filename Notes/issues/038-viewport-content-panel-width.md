## Issue 38: Install the viewport with the layout key's text width, not a terminal-derived width

**Type**: AFK
**Blocked by**: None — can start immediately

### Parent PRD

`Notes/PRD-vrg.md` — audit source: `Notes/critiques/final-audit-vrg.md`, High finding 3

### What to build

Fix the width disagreement between the row model and the viewport (`internal/app/app.go:1026-1040`, `1385-1420`). Three distinct widths are involved — keep them separate throughout this work:

1. **Terminal width** — `m.width`, the raw terminal columns.
2. **Panel width** — terminal width minus file-list width minus the one-cell separator (`m.width - m.ListWidth() - 1`, as `LayoutKey` computes `panelWidth`).
3. **Text width** — panel width minus gutter width minus the reserved right-indicator width (one cell in run-off-edge mode, zero in wrap mode): `viewport.TextWidth(panelWidth, gutterWidth, wrapMode)`.

`LayoutKey` already performs the full chain and stores the resulting **text width** in `RowModelKey.TextWidth`. The audited defect is that `LayoutReadyMsg` installation ignores that result and recomputes `viewport.TextWidth(m.width, ...)` from the **terminal** width — skipping the list-width and separator subtraction entirely. The viewport therefore pans, clips, reveals, pads, and places hidden-content indicators as if the file list did not exist: the row model and the viewport disagree on their width, and composed rows can exceed the terminal.

- Install the viewport with `msg.Key.TextWidth` — i.e. `m.viewport.SetLayout(msg.Key.TextWidth, m.wrapMode)` — so the single `LayoutKey` computation governs both row construction and viewport behaviour.
- All horizontal behaviour — run-off-edge clipping, `,`/`.`/`<`/`>`/`[`/`]` panning, minimal match reveal, and the left/right hidden-content indicators — is measured against the **text width** (definition 3). The reserved right-indicator column and the gutter occupy the panel (definition 2), outside the text area.
- The same fix applies whether the file list is shown or hidden: panel width follows the current layout state, not the raw terminal width.

The existing tests in `internal/app/reveal_horizontal_test.go:68-94` encode the incorrect full-terminal calculation and mask this defect; correcting them is part of this issue.

See PRD *File list and layout* (list width cap), *Layout and indicators* ("File-panel content width is panel width minus gutter and reserved right-indicator column"), and *Navigation, viewport, and logical anchors* (horizontal reveal).

### How to verify

- **Manual**: run a search whose matches include lines longer than the text width, with the file list visible → pan right with `>` and confirm content, indicators, and padding stay inside the panel boundary and never bleed under or past the file list; toggle the list with `left`/`right` and confirm panning and indicators re-measure against the new panel width.
- **Automated**: app-level horizontal tests that compute expected reveal/clip/indicator positions from the **text width** — terminal width minus actual list width minus separator minus gutter minus reserved right-indicator width — covering list-shown, list-hidden, wrap-mode (indicator width zero), and resize cases, replacing the assertions at `reveal_horizontal_test.go:68-94` that assume full-terminal width; a composed-view test asserting no rendered row exceeds the terminal width and the right indicator sits at the panel's right edge outside the text area.

### Acceptance criteria

- [ ] Given the file list is visible, then the viewport's text width equals panel width minus gutter width minus reserved right-indicator width — i.e. terminal width minus list width minus separator minus gutter minus indicator — matching `LayoutKey`'s `TextWidth`.
- [ ] Given a match beyond the right edge of the text area, then minimal horizontal reveal brings its start cell into the text area's visible range — computed from the text width, not the panel or terminal width.
- [ ] Given hidden-content indicators, then the reserved rightmost indicator column sits at the panel's right edge, outside the text area, and no composed row exceeds the terminal width.
- [ ] Given the file list toggled hidden or a resize, then the viewport text width tracks the new panel width (and gutter/indicator reservations) and pan/reveal state remains consistent with it.
- [ ] Given the updated horizontal tests, then every expected-position calculation subtracts list width, separator, gutter, and the reserved indicator — so a regression that omits either the gutter or the run-off-edge indicator reservation fails the test.

### User stories addressed

- User story 27: list width capped at 40% of terminal width and constrained by minimum content width
- User story 54: minimal horizontal scrolling to reveal an off-screen first-submatch start
- User story 55: match wider than the viewport considered revealed when its start cell is visible
- User story 65: pan speeds measured against the text-area width
- User stories 67–68: hidden-content indicators placed at the panel edges

---
