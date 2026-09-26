# Tasks for #38: Install the viewport with the layout key's text width, not a terminal-derived width

Parent issue: #38
Parent PRD: PRD-vrg.md
**Blocked by issues**: none
**Acceptance criteria**: AC1–AC5 → Tasks 1–2
**Manual verification**: Task 4 owns the issue's manual checks.

## Tasks

### 1. Specify text-width-based horizontal behaviour

**Type**: RED  
**Output**: Failing tests compute expected reveal, clip, and indicator positions from the text width — terminal width minus list width minus separator minus gutter minus reserved indicator — across list-shown, list-hidden, wrap-mode, and resize cases, plus a composed-view assertion that no row exceeds the terminal width and the right indicator sits at the panel's right edge.  
**Depends on**: none

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Replace the incorrect full-terminal-width expectations in `internal/app/reveal_horizontal_test.go` — the `textWidthAt80` helper and every dependent assertion — with expectations computed from the **text width**: terminal width minus the actual `ListWidth()` minus the one-cell separator minus the buffer gutter minus `viewport.ReservedWidth` for the mode (one cell in run-off-edge, zero in wrap), matching `LayoutKey`'s `TextWidth`. Extend coverage to the file list hidden (panel width follows the current layout state, not the raw terminal width), wrap mode (zero indicator reservation), and resize re-measurement. Add a composed-view test asserting no rendered row exceeds the terminal width and the reserved right-indicator column sits at the panel's right edge outside the text area. Every expected-position calculation must subtract list width, separator, gutter, and the reserved indicator so a regression omitting either fails. The tests fail on the current code, which installs the viewport with `viewport.TextWidth(m.width, …)` — terminal width in place of panel width. Keep this task test-only.

---

### 2. Install the viewport with the layout key's text width

**Type**: GREEN  
**Output**: The corrected horizontal tests pass; the single `LayoutKey` computation governs row construction and viewport behaviour, and pan/reveal/indicator state tracks the current panel width.  
**Depends on**: 1

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Fix every site in `internal/app/app.go` that recomputes `viewport.TextWidth(m.width, …)` from the raw terminal width: the `LayoutReadyMsg` installation (install with `m.viewport.SetLayout(msg.Key.TextWidth, m.wrapMode)` so the single `LayoutKey` computation governs), the synchronous factory-seam and cache-hit installs in `buildViewport` (use the computed layout key's `TextWidth`), and the `WindowSizeMsg` resize path (recompute the panel width as `m.width - m.ListWidth() - 1` before subtracting gutter and reserved indicator, or reuse `LayoutKey`). Keep the three width definitions distinct — terminal width, panel width, text width — and keep run-off-edge clipping, `,`/`.`/`<`/`>`/`[`/`]` panning, minimal match reveal, and the left/right hidden-content indicators all measured against the text width, with the reserved right-indicator column and the gutter occupying the panel outside the text area. Run the corrected focused tests plus `go build ./...`, `go vet ./...`, and `go test ./...`.

---

### 3. Document the viewport-width fix

**Type**: DOCUMENT  
**Output**: Wiki documentation records the three width definitions and that the layout key's text width governs the viewport.  
**Depends on**: 2

Read and follow `Notes/wiki/wiki-rules.md` and the schema in `Notes/wiki/AGENTS.md`, then ingest the completed Issue #38 fix into the appropriate pages under `Notes/wiki`. Document the terminal/panel/text width chain — terminal width; panel width as terminal minus list width minus separator; text width as panel minus gutter minus reserved right-indicator width — and that `LayoutKey`'s `TextWidth` is installed at every viewport install site, including resize and file-list hide/show. Cross-reference Issue #38 and the *File list and layout*, *Layout and indicators*, and *Navigation, viewport, and logical anchors* sections of `Notes/PRD-vrg.md`, update `Notes/wiki/index.md`, and append the required dated ingest record to `Notes/wiki/log.md` without rewriting previous entries.

---

### 4. Create the viewport-width walkthrough

**Type**: CODE WALKTHROUGH  
**Output**: Showboat walkthrough exists at `Notes/walkthroughs/038-04/code-walkthrough`.  
**Depends on**: 3

Use showboat, consulting `uvx showboat --help`, to create the walkthrough at exactly `Notes/walkthroughs/038-04/code-walkthrough`, with the main file named `walkthrough.md`. Demonstrate the corrected horizontal tests, then run the issue's manual scenario: a search whose matches include lines longer than the text width with the file list visible → pan right with `>` and confirm content, indicators, and padding stay inside the panel boundary and never bleed under or past the file list; toggle the list with `left`/`right` and confirm panning and indicators re-measure against the new panel width. Capture commands, outputs, and exit statuses. Reference Issue #38 and `Notes/PRD-vrg.md`, and store every generated artifact in the approved directory.

---
