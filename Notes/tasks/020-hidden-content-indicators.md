# Tasks for #20: Hidden-content indicators — gutter `_`/`*` and reserved right column `*`

Parent issue: #20
Parent PRD: PRD-vrg.md
**Blocked by issues**: #19
**Acceptance criteria**: AC1–AC5 → Tasks 1–2

## Tasks

### 1. Specify hidden-content indicators

**Type**: RED  
**Output**: Failing rendering tests cover `_` versus `*` gutters, the current-line-only right marker with off-screen absence, both sides hidden, partial visibility, the last-cell match with another farther right, split-glyph blanks, and wrap-mode absence.  
**Depends on**: none

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Begin only after Issue #19 is complete. Add failing rendering tests for the Issue #20 contracts and the indicator bullets of the Layout and indicators section of `Notes/PRD-vrg.md`. In run-off-edge mode require every visible source line's first trailing gutter space to show an inverse `_` when any text is hidden left, upgraded to an inverse `*` when a match or marker on that line is entirely hidden left, and blank otherwise; the reserved rightmost column to show an inverse `*` only on the current matched line's visible row when at least one match or marker is entirely hidden right, to be absent when that line is vertically off-screen, and never to overwrite text; partially visible matches to produce no hidden-match indicator for that side, with visibility computed over actually rendered cells after grapheme clipping and excluding the reserved column; a split wide glyph rendered as blanks not to count as visible; both stars to be able to appear together; and wrap mode to draw neither indicators nor a reserved column. Keep this task test-only.

---

### 2. Implement the indicators

**Type**: GREEN  
**Output**: Indicator tests pass; gutter and reserved-column markers derive from rendered-cell visibility.  
**Depends on**: 1

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Implement the indicators in the rendering path to satisfy Task 1, deriving all visibility from the actually rendered cells the Issue #19 visibility computation exposes, styling them with the Theme's inverse indicator style, and leaving wrap mode free of indicators and the reserved column.

---

### 3. Create the indicator finish marker

**Type**: FINISH MARKER  
**Output**: Finish marker exists at `Notes/finish-markers/020-04/finish-marker.md`.  
**Depends on**: 2

Write `Task 020-04 finished successfully at <time>` to `Notes/finish-markers/020-04/finish-marker.md`, replacing `<time>` with the current UTC timestamp (for example, `date -u +"%Y-%m-%dT%H:%M:%SZ"`). Create the `Notes/finish-markers/020-04/` directory if it does not already exist.

---
