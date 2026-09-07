## Issue 17: Width-independent logical anchor through rewrap, wrap toggle and resize; lossy EOF clamp; off-UI layout preparation and obsolete-layout isolation

**Type**: AFK
**Blocked by**: Issue 16

### Parent PRD

`Notes/PRD-vrg.md`

### What to build

Viewport owns a logical anchor `(source line, display-column offset)` independent of wrap width, and layout preparation (rewrap / row-model construction) runs off the UI update path with late results isolated.

**Anchor**

- After a resize or wrap toggle, the effective top row is the row containing the anchor location, not the row with the same former ordinal.
- Turning wrap off shows the anchor's source line as one row but retains the logical column; turning wrap on restores the row containing that column.
- User vertical scrolling replaces the anchor with the resulting top row's location. A match reveal that moves the viewport likewise replaces it; a no-scroll reveal does not.
- EOF clamping may pull the effective top upward and **updates** the anchor to the resulting top (intentionally lossy: a subsequent shrink need not restore the old top).
- Resize preserves the cursor selection and the reading position except for this documented EOF clamping.

**Layout preparation off the UI path**

- Building the row model for a (path, content revision, text width, wrap mode) is a prepared-layout job that completes via a message, like file loads (PRD *Module Design → App*: "prepared layout" messages). `Update` for a resize or `w` records the new dimensions/mode, keeps the logical anchor, requests preparation, and returns; it does not rewrap the whole buffer inline. Until the prepared layout for the current parameters arrives, the panel may keep showing the previous layout or a placeholder — but input (`ctrl+c`, `q`, `n`/`p`, toggles, further resizes) remains actionable.
- **Obsolete-layout isolation**: a prepared layout is installed only if its (path, content revision, width, mode) equals the model's *current* parameters at arrival time. A layout prepared for an earlier width, an earlier wrap mode, a superseded content revision (reload), or a file that is no longer current is discarded; it must not replace the current display, its anchor, or its saved per-file state. Discarding an obsolete layout also cannot consume or mutate a **pending reveal intent**: when navigation occurs while no matching layout is installed, the model carries the reveal for the newest stop and commits it — applying the normal Issue 14 rules against the installed rows — once a layout matching the current parameters arrives. Issue 28 extends this contract to load completions and reload-anchor intent with the final marker/grapheme/stale target geometry.
- Frame rendering never recomputes a full-buffer mapping or scans the whole file list; it uses the installed prepared layout and the list's visible window.
- Small buffers may still be prepared quickly; the contract is about where the work happens and how late results are handled, not about a minimum size threshold. Do not prescribe a particular goroutine/channel design.

See PRD *Navigation, viewport, and logical anchors* (last four bullets), *Resources and responsiveness* (last bullet), and *Testing Decisions → Responsiveness boundaries*.

### How to verify

- **Manual**: scroll partway into a wrapped long line; narrow the terminal → the same text is still at the top (more rows); widen back → same text at top; press `w` twice → same text; scroll to EOF then widen → top moves up and stays there after narrowing again. Open a file with a ~50 MB single-line-heavy fixture, resize repeatedly and press `ctrl+c` mid-rewrap → exits promptly with 130.
- **Automated**:
  - Viewport tests: anchor across width change round trip (no loss when no clamp); wrap-off/wrap-on round trip preserving the logical column; manual scroll replaces anchor; reveal-with-scroll replaces, no-scroll reveal preserves; deliberate EOF-clamp loss scenario; App test that resize keeps the cursor.
  - **Gated layout preparation**: with the layout worker held by a test gate after a resize, assert `ctrl+c` exits 130, `q` exits with the fixed status, `n` advances the cursor, and a second resize is accepted, all without releasing the gate. Then release a layout matching the new parameters and assert the stop selected while the gate was held is revealed per the Issue 14 rules (the pending intent was preserved, not lost).
  - **Out-of-order completion**: resize W1→W2→W3, release preparations in the order W2, W1, W3 → after each release, the installed layout is either the previous current one or W3's; W1/W2 layouts never become visible once W3 was requested, and the anchor stays at the same text location throughout.
  - Same pattern for: wrap toggle (`w`, `w` quickly; an `off`-mode layout arriving after the mode is back `on` is discarded); content revision (reload from Issue 27 supersedes the pre-reload layout — the pre-reload layout arriving late is discarded; test may be added when Issue 27 lands, but the keying must be in place now); file change (a layout for file A arriving after navigation to B does not touch B's panel).
  - Render-cost guard: `View()` on a large prepared buffer queries only visible rows; a counting fake of the row provider and of the file-list item provider proves neither is called O(N).

### Acceptance criteria

- [ ] Given a viewport whose top is mid-way through a wrapped line, when the width changes, then the top row is the one containing the anchor's text location.
- [ ] Given wrap is toggled off then on, when no scroll/reveal/clamp intervened, then the effective top returns to the row containing the original logical column.
- [ ] Given user scrolling, then the anchor becomes the new top row's location.
- [ ] Given a resize that would leave avoidable blank rows below EOF, then the top is clamped upward and the anchor updated to that top.
- [ ] Given any resize, then the cursor selection is unchanged.
- [ ] Given layout preparation is still in progress, when `ctrl+c`, `q`, `n`/`p`, `w`, or another resize arrives, then it is handled without waiting for preparation to complete.
- [ ] Given prepared layouts complete out of order after successive resizes or wrap toggles, then only a layout matching the current (path, revision, width, mode) is installed, and the logical anchor is unaffected by discarded layouts.
- [ ] Given navigation occurs while the current layout is pending, when a matching layout installs, then the newest stop's target is revealed per the Issue 14 rules.
- [ ] Given a prepared layout for a file that is no longer current or for a superseded content revision, then it is discarded without touching the visible panel or saved viewport state.
- [ ] Given a frame render, then no full-buffer rewrap or whole-file-list scan occurs.

### User stories addressed

- User story 33: resize preserves selection and reading position except EOF clamping
- User story 34 (part): expensive processing (rewrap) leaves input responsive
- User story 62: width-independent logical anchor
- User story 63: EOF clamping updates the anchor (accepted loss)

---
