## Issue 22: Line terminators, unterminated final line, empty file, leading UTF-8 BOM

**Type**: AFK
**Blocked by**: Issue 6

### Parent PRD

`Notes/PRD-vrg.md`

### What to build

FileBuffer structural line handling with raw/search/display coordinate separation.

- LF and CRLF terminate lines and are not displayed, but original line bytes (including terminators) are retained for byte-coordinate mapping and later validation (Issue 29).
- A missing final newline yields a final line; a trailing newline does not add an empty line; an empty file has zero lines and shows an empty panel (gutter reserves at least one digit slot).
- Zero-width positions and removed terminator bytes map to the display end-of-line position (e.g. byte 4 of `hit\r\n` → display column 3). A span covering visible text plus terminator highlights the visible text only. (The end-of-line *marker* for terminator-only matches is Issue 23.)
- A leading UTF-8 BOM is invisible in display and adjusts coordinates: ripgrep's line offsets for the first line omit the three BOM bytes, so maintain separate raw-file and rg-line views and map between them. Non-leading U+FEFF is ordinary content.

See PRD *Text, graphemes, and safe presentation* (first two bullets) and *Encodings and stale-content validation* (UTF-8 BOM bullet).

### How to verify

- **Manual**: search in a CRLF file → no `^M` shown and highlights land correctly; an empty matched file (possible via `^` with `--` on an empty file? — use a fixture-driven fake rg) shows an empty panel; a file with a UTF-8 BOM shows no `\uFEFF`/garbage and the first-line match is highlighted in the right place.
- **Automated**: FileBuffer tests: LF, CRLF, mixed; no final newline → last line counted; trailing newline → no phantom line; empty file → zero lines and gutter width 1; mapping of terminator bytes to EOL column; span crossing text+terminator; UTF-8 BOM invisible with a first-line match at rg offset 0 mapping to raw byte 3 and highlighting the right cells; non-leading U+FEFF displayed/counted normally.

### Acceptance criteria

- [ ] Given a CRLF file, then no terminator cells are displayed and byte offsets still map correctly to display columns.
- [ ] Given a file without a final newline, then its last line is counted and displayed.
- [ ] Given an empty file, then the panel is empty with zero source lines.
- [ ] Given a leading UTF-8 BOM, then it is not displayed and first-line match coordinates from rg are shifted by three bytes when mapped to raw bytes.
- [ ] Given a span that covers visible text and the terminator, then only the visible text is highlighted.

### User stories addressed

- User story 48: empty file shows an empty panel with zero lines
- User story 73: LF/CRLF removed from display, retained in byte mapping
- User story 74: unterminated final line counted; UTF-8 BOM invisible with adjusted coordinates

---
