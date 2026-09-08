# Tasks for #20: Hidden-content indicators — gutter `_`/`*` and reserved right column `*`

Parent issue: #20
Parent PRD: PRD-vrg.md
**Blocked by issues**: #19
**Acceptance criteria**: AC1–AC5 → Tasks 1–2
**Manual verification**: Task 4 owns the issue's manual checks.

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

### 3. Document the indicators

**Type**: DOCUMENT  
**Output**: Wiki documentation records the gutter and right-column indicators, their scopes, and the visibility rule.  
**Depends on**: 2

Read and follow `Notes/wiki/wiki-rules.md` and the schema in `Notes/wiki/AGENTS.md`, then ingest the completed Issue #20 implementation and tests into the appropriate pages under `Notes/wiki`. Document the per-line left `_`/`*` gutter indicator, the current-matched-line-only right `*` with its off-screen absence and no-overwrite guarantee, partial visibility counting as visible, the rendered-cells-after-clipping visibility basis excluding the reserved column, split-glyph blanks not counting as visible, and wrap mode's absence of indicators. Cross-reference Issue #20 and the Layout and indicators section of `Notes/PRD-vrg.md`, update `Notes/wiki/index.md`, and append the required dated ingest record to `Notes/wiki/log.md` without rewriting previous entries.

---

### 4. Create the indicator walkthrough

**Type**: CODE WALKTHROUGH  
**Output**: Showboat walkthrough exists at `Notes/walkthroughs/020-04/code-walkthrough`.  
**Depends on**: 3

Use showboat, consulting `uvx showboat --help`, to create the walkthrough at exactly `Notes/walkthroughs/020-04/code-walkthrough`, with the main file named `walkthrough.md`. Demonstrate the indicator rendering tests, then run the binary in run-off-edge mode on a file with several long matched lines: panning right showing `_` on lines with hidden text and `*` on lines whose match is fully hidden, a right `*` in the last column on the current matched line with a second match far right, a half-visible match producing no star for that side, and wrap mode showing no indicators or reserved column. Reference Issue #20 and `Notes/PRD-vrg.md`, and store every generated artifact in the approved directory.

---
