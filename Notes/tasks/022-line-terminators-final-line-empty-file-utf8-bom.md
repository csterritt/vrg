# Tasks for #22: Line terminators, unterminated final line, empty file, leading UTF-8 BOM

Parent issue: #22
Parent PRD: PRD-vrg.md
**Blocked by issues**: #6
**Acceptance criteria**: AC1–AC5 → Tasks 1–2
**Manual verification**: Task 4 owns the issue's manual checks.

## Tasks

### 1. Specify structural line handling and coordinate separation

**Type**: RED  
**Output**: Failing FileBuffer tests cover LF, CRLF, and mixed terminators, unterminated final lines, no phantom trailing line, the zero-line empty file with its three-cell gutter, terminator-to-EOL mapping, text-plus-terminator spans, the leading UTF-8 BOM offset adjustment, and non-leading U+FEFF.  
**Depends on**: none

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Begin only after Issue #6 is complete. Add failing tests in `internal/filebuffer` for the Issue #22 contracts and the first two Text, graphemes, and safe presentation bullets plus the UTF-8 BOM bullet of `Notes/PRD-vrg.md`. Require LF and CRLF to terminate lines without being displayed while original line bytes including terminators are retained for byte-coordinate mapping and later validation; a missing final newline to yield a final line and a trailing newline not to invent an extra empty one; an empty file to produce zero source lines with a one-digit-slot gutter three cells wide and an empty panel; a standalone CR to be escaped as `^M` by the safe-presentation core rather than treated as a terminator; zero-width positions and removed terminator bytes to map to the display end-of-line position (byte 4 of `hit\r\n` → display column 3) and a span covering visible text plus terminator to highlight only the visible text; a leading UTF-8 BOM to be invisible in display with rg first-line offsets omitting its three bytes, so a first-line match at rg offset 0 maps to raw byte 3 and highlights the correct cells, while non-leading U+FEFF is ordinary content; and separate raw-file and rg-line coordinate views maintained throughout. Keep this task test-only.

---

### 2. Implement structural line handling

**Type**: GREEN  
**Output**: Structural line tests pass with raw, search, and display coordinate views separated.  
**Depends on**: 1

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Implement the structural line handling in `internal/filebuffer` to satisfy Task 1: LF and CRLF splitting with retained original bytes, the final-line and empty-file rules with their gutter consequences, terminator-to-EOL display mapping, and the leading UTF-8 BOM adjustment between raw-file and rg-line coordinates. The end-of-line marker for terminator-only matches is owned by Issue #23; the stale validation that consumes the retained bytes is owned by Issue #29.

---

### 3. Document structural line handling

**Type**: DOCUMENT  
**Output**: Wiki documentation records terminator rules, final-line and empty-file handling, coordinate separation, and the BOM adjustment.  
**Depends on**: 2

Read and follow `Notes/wiki/wiki-rules.md` and the schema in `Notes/wiki/AGENTS.md`, then ingest the completed Issue #22 implementation and tests into the appropriate pages under `Notes/wiki`. Document LF and CRLF as undisplayed terminators with retained bytes, the standalone-CR escape, the unterminated final line, no phantom trailing line, the empty file's zero lines and three-cell gutter, terminator-to-EOL mapping for zero-width positions, the visible-text-only highlight for spans crossing terminators, and the leading UTF-8 BOM's three-byte coordinate adjustment with non-leading U+FEFF as content. Cross-reference Issue #22 and the Text, graphemes, and safe presentation and Encodings sections of `Notes/PRD-vrg.md`, update `Notes/wiki/index.md`, and append the required dated ingest record to `Notes/wiki/log.md` without rewriting previous entries.

---

### 4. Create the structural-line walkthrough

**Type**: CODE WALKTHROUGH  
**Output**: Showboat walkthrough exists at `Notes/walkthroughs/022-04/code-walkthrough`.  
**Depends on**: 3

Use showboat, consulting `uvx showboat --help`, to create the walkthrough at exactly `Notes/walkthroughs/022-04/code-walkthrough`, with the main file named `walkthrough.md`. Demonstrate the FileBuffer structural tests, then run the manual cases: a CRLF file searching cleanly with no `^M` shown and highlights landing correctly, a standalone CR mid-line rendering as `^M`, the empty-file case using the fake-rg harness to emit a valid `begin`/`match`/`end`/`summary` stream for a zero-byte `a.txt` showing the empty panel and three-cell gutter, and a UTF-8 BOM file showing no visible BOM with its first-line match highlighted in the right place. Reference Issue #22 and `Notes/PRD-vrg.md`, and store every generated artifact in the approved directory.

---
