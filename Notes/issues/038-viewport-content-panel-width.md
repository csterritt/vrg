## Issue 38: Install the viewport with content-panel width, not terminal width

**Type**: AFK
**Blocked by**: None — can start immediately

### Parent PRD

`Notes/PRD-vrg.md` — audit source: `Notes/critiques/final-audit-vrg.md`, High finding 3

### What to build

Fix the width disagreement between the row model and the viewport (`internal/app/app.go:1026-1040`, `1385-1420`). `LayoutKey` correctly computes the row-model width as terminal width minus file-list width minus separator, but `LayoutReadyMsg` installs the viewport with `viewport.TextWidth(m.width, ...)` — the full terminal width. The viewport therefore pans, clips, reveals, pads, and places hidden-content indicators as if the file list did not exist: the row model and the viewport disagree on their width, and composed rows can exceed the terminal.

- Install the viewport with `msg.Key.TextWidth` (or the same shared panel-width calculation `LayoutKey` uses), so one width computation governs both row construction and viewport behaviour.
- All horizontal behaviour — run-off-edge clipping, `,`/`.`/`<`/`>`/`[`/`]` panning, minimal match reveal, and the left/right hidden-content indicators — must be measured against the content-panel width.
- The same fix applies whether the file list is shown or hidden: panel width follows the current layout state, not the raw terminal width.

The existing tests in `internal/app/reveal_horizontal_test.go:68-94` encode the incorrect full-terminal calculation and mask this defect; correcting them is part of this issue.

See PRD *File list and layout* (list width cap), *Layout and indicators*, and *Navigation, viewport, and logical anchors* (horizontal reveal).

### How to verify

- **Manual**: run a search whose matches include lines longer than the content panel, with the file list visible → pan right with `>` and confirm content, indicators, and padding stay inside the panel boundary and never bleed under or past the file list; toggle the list with `left`/`right` and confirm panning and indicators re-measure against the new panel width.
- **Automated**: app-level horizontal tests that compute expected reveal/clip/indicator positions using terminal width minus actual list width minus separator — covering list-shown, list-hidden, and resize cases — replacing the assertions at `reveal_horizontal_test.go:68-94` that assume full-terminal width; a composed-view test asserting no rendered row exceeds the terminal width.

### Acceptance criteria

- [ ] Given the file list is visible, then the viewport's text width equals terminal width minus list width minus separator, matching the row-model width in `LayoutKey`.
- [ ] Given a match beyond the right edge of the content panel, then minimal horizontal reveal brings its start cell into the panel's visible range — not the terminal's.
- [ ] Given hidden-content indicators, then the reserved rightmost indicator column sits at the panel's right edge and no composed row exceeds the terminal width.
- [ ] Given the file list toggled hidden or a resize, then viewport width tracks the new panel width and pan/reveal state remains consistent with it.
- [ ] Given the updated horizontal tests, then they include list width and separator in every expected-position calculation, so a regression to terminal-width measurement fails.

### User stories addressed

- User story 27: list width capped at 40% of terminal width and constrained by minimum content width
- User story 54: minimal horizontal scrolling to reveal an off-screen first-submatch start
- User story 55: match wider than the viewport considered revealed when its start cell is visible
- User story 65: pan speeds measured against the text-area width
- User stories 67–68: hidden-content indicators placed at the panel edges

---
