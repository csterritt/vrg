# Tasks for #7: Theme — `c` colour toggle, inverse matches, current-line underline

Parent issue: #7
Parent PRD: PRD-vrg.md
**Blocked by issues**: #5
**Acceptance criteria**: AC1–AC4 → Tasks 1–2
**Manual verification**: Task 4 owns the issue's manual checks.

## Tasks

### 1. Specify the theme schemes and styles

**Type**: RED  
**Output**: Failing Theme tests cover both schemes' colour pairs, true inverse matches, the current-match underline, the current-file underline, and overlay styles; a failing App test covers `c` toggling `View()` styling.  
**Depends on**: none

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Begin only after Issue #5 is complete. Add failing tests in `internal/theme` and `internal/app` for the Issue #7 contracts and the first bullet of the Colours, overlays, and key precedence section of `Notes/PRD-vrg.md`. Require the dark scheme to be initially active as white on black, `c` to toggle to light (black on white) and back with no persistence, the match style to be the true inverse of the active scheme's base colours, current-matched-line matches to add underline (the first stop until Issue #13), the current file-list entry to be underlined in both schemes, and the overlay style to use base colours with a plain single-line border. Require an App-level `c` keypress to flip the composed `View()` styling between the schemes. Keep this task test-only.

---

### 2. Implement the Theme module and wiring

**Type**: GREEN  
**Output**: Scheme, style, and toggle tests pass; rendering consumes Theme styles for base, gutter, match, current-match, indicator, overlay, filename-rule, and file-list styling.  
**Depends on**: 1

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Implement `internal/theme` and its wiring to satisfy Task 1: the toggle with no persistence, both schemes' foreground and background pairs, and the supplied styles for base, gutter, true-inverse matches, underlined current matches, inverse indicators, overlays with base colours and single-line borders, the filename rule, the file list, and the current-file underline. Route the browse rendering from Issue #5 through these styles and handle `c` in the App model.

---

### 3. Document the theme module

**Type**: DOCUMENT  
**Output**: Wiki documentation records both schemes, the style set, the toggle, and the inverse/underline rules.  
**Depends on**: 2

Read and follow `Notes/wiki/wiki-rules.md` and the schema in `Notes/wiki/AGENTS.md`, then ingest the completed Issue #7 `internal/theme` implementation and tests into the appropriate pages under `Notes/wiki`. Document the initially dark scheme, the `c` toggle without persistence, the true-inverse match rule, the current-matched-line and current-file underlines, the indicator and overlay styles, and how rendering consumes the theme. Cross-reference Issue #7 and the Colours, overlays, and key precedence and Module Design sections of `Notes/PRD-vrg.md`, update `Notes/wiki/index.md`, and append the required dated ingest record to `Notes/wiki/log.md` without rewriting previous entries.

---

### 4. Create the theme walkthrough

**Type**: CODE WALKTHROUGH  
**Output**: Showboat walkthrough exists at `Notes/walkthroughs/007-04/code-walkthrough`.  
**Depends on**: 3

Use showboat, consulting `uvx showboat --help`, to create the walkthrough at exactly `Notes/walkthroughs/007-04/code-walkthrough`, with the main file named `walkthrough.md`. Demonstrate the Theme unit tests for both schemes and the App toggle test, then run the binary with a search invocation against a fixture with matches — for example `vrg func .` in a Go repository, as in Issue #5 — and press `c` to show the background and foreground swapping with matches still inverse and current-line matches underlined, and `c` again to return (bare `vrg` prints command-line help and exits, so it cannot demonstrate theme switching). Reference Issue #7 and `Notes/PRD-vrg.md`, and store every generated artifact in the approved directory.

---
