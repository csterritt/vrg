# Tasks for #22: Line terminators, unterminated final line, empty file, leading UTF-8 BOM

Parent issue: #22
Parent PRD: PRD-vrg.md
**Blocked by issues**: #6
**Acceptance criteria**: AC1–AC5 → Tasks 1–2

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

### 3. Create the structural-line finish marker

**Type**: FINISH MARKER  
**Output**: Finish marker exists at `Notes/finish-markers/022-04/finish-marker.md`.  
**Depends on**: 2

Write `Task 022-04 finished successfully at <time>` to `Notes/finish-markers/022-04/finish-marker.md`, replacing `<time>` with the current UTC timestamp (for example, `date -u +"%Y-%m-%dT%H:%M:%SZ"`). Create the `Notes/finish-markers/022-04/` directory if it does not already exist.

---
