# Tasks for #7: Theme — `c` colour toggle, inverse matches, current-line underline

Parent issue: #7
Parent PRD: PRD-vrg.md
**Blocked by issues**: #5
**Acceptance criteria**: AC1–AC4 → Tasks 1–2

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

### 3. Create the theme finish marker

**Type**: FINISH MARKER  
**Output**: Finish marker exists at `Notes/finish-markers/007-04/finish-marker.md`.  
**Depends on**: 2

Write `Task 007-04 finished successfully at <time>` to `Notes/finish-markers/007-04/finish-marker.md`, replacing `<time>` with the current UTC timestamp (for example, `date -u +"%Y-%m-%dT%H:%M:%SZ"`). Create the `Notes/finish-markers/007-04/` directory if it does not already exist.

---
