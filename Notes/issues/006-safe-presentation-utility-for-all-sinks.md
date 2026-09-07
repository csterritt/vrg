## Issue 6: Safe-presentation utility for every output sink

**Type**: AFK
**Blocked by**: Issue 5

### Parent PRD

`Notes/PRD-vrg.md`

### What to build

A shared sanitization utility used by every sink — file content, filenames (list, filename rule, pop-ups, diagnostics), help substitutions, usage errors, stderr replay — so raw control sequences from searched data never reach the terminal.

- **Paths**: escape `\n`, `\r`, `\t` as `\n`, `\r`, `\t`; invalid UTF-8 bytes as `\xNN`; literal backslash as `\\`; other C0/C1 controls safely; preserve valid printable Unicode. Original bytes remain the key for identity, ordering and file access.
- **File content**: invalid UTF-8 → U+FFFD while retaining raw-byte mappings; C0 controls and DEL (other than tab/LF/CR handled structurally) → caret notation (`^[` for ESC); C1 → `\u0085`-style. Escaped forms expose byte→cell mappings so highlights can cover them.
- **Diagnostics**: preserve real line boundaries, expand tabs, escape other controls; any filename embedded in a diagnostic is escaped first as a single-line filename.
- Replace the minimal escaper from Issue 1 and wire the utility into the file list, filename rule, panel content and usage errors.

See PRD *Text, graphemes, and safe presentation* (sanitization bullets).

### How to verify

- **Manual**:
  1. Create a file whose name contains a newline and an ESC byte and a matching line containing `\x1b]0;pwned\x07`; run `vrg`. The list shows `\n`/escaped bytes; the content row shows `^[]0;pwned^G`; the terminal title is not changed.
  2. `vrg foo "$(printf 'bad\x1bdir')"` → stderr shows an escaped name, exit 2.
- **Automated**: unit tests for path escaping (each rule), content escaping with byte→cell maps (a match covering an ESC byte highlights both `^` and `[`), diagnostic escaping preserving line breaks and single-lining embedded filenames; rendering tests asserting `View()` output contains no raw `\x1b` beyond the app's own styling (compare against a stripped-style render).

### Acceptance criteria

- [ ] Given a path with newline, tab, backslash, and invalid bytes, when displayed anywhere, then it renders as `\n`, `\t`, `\\`, `\xNN` respectively while the original bytes are still used to open the file.
- [ ] Given content containing ESC or other C0/C1 controls, when rendered, then caret/`\uXXXX` forms appear and a highlight over those bytes covers all their cells.
- [ ] Given invalid UTF-8 content, then U+FFFD is shown and highlights still map to the right cells.
- [ ] Given a diagnostic embedding a filename with a newline, then the diagnostic keeps its own line breaks and the filename appears single-lined.
- [ ] Given any sink, then no searched-data control sequence is emitted as a terminal instruction.

### User stories addressed

- User story 25: original filename bytes retained; invalid bytes visibly escaped
- User story 75: invalid UTF-8 replaced and controls escaped in every sink

---
