## Issue 6: Safe-presentation utility for every output sink

**Type**: AFK
**Blocked by**: Issue 5

### Parent PRD

`Notes/PRD-vrg.md`

### What to build

A shared sanitization utility used by every sink — file content, filenames (list, filename rule, pop-ups, diagnostics), help substitutions, usage errors, stderr replay — so raw control sequences from searched data never reach the terminal.

The **core path and content escaping rules, their unit tests, and the hostile-fixture raw-output tests for the browse sinks land in Issue 5**, ahead of the first arbitrary-data render. This issue does not re-introduce that work; it generalizes the Issue 5 core into one shared utility, extends it to the remaining sinks, and establishes the extensible sink-safety table:

- **Paths** (core landed in Issue 5; restated as the canonical contract): escape `\n`, `\r`, `\t` as `\n`, `\r`, `\t`; invalid UTF-8 bytes as `\xNN`; literal backslash as `\\`; other C0/C1 controls safely; preserve valid printable Unicode. Original bytes remain the key for identity, ordering and file access.
- **File content** (core landed in Issue 5; restated as the canonical contract): invalid UTF-8 → U+FFFD while retaining raw-byte mappings; C0 controls and DEL → caret notation (`^[` for ESC), except that **tab is expanded structurally and LF / CRLF are line terminators** (Issue 22). A **standalone CR** (not followed by LF) is *not* a terminator and is escaped as `^M`; it is never emitted raw and never silently dropped. C1 → `\u0085`-style. Escaped forms expose byte→cell mappings so highlights can cover them.
- **Diagnostics** (new in this issue): preserve real line boundaries, expand tabs, escape other controls; any filename embedded in a diagnostic is escaped first as a single-line filename.
- Unify the Issue 5 core and the minimal escaper from Issue 1 into the single shared utility (replacing the Issue 1 escaper) and wire it into every sink that exists at this point: the file list, filename rule, panel content and usage errors. Generalization must not regress any Issue 5 escaping test or sink-safety row.
- **Sink-safety table**: restructure the Issue 5 raw-output fixture set as a shared, extensible table test covering every sink existing at this point (file-list entry, filename rule, panel content, usage-error stderr). **Later-sink ownership**: each issue that introduces a new sink is responsible for routing it through this utility and adding that sink to the sink-safety test table — Issue 9 (error overlay), Issue 11 (stderr replay), Issue 15 (pop-up), Issue 31 (help substitutions), Issue 34 (any README/help generated text). Issue 6 provides the table and the fixtures; those issues extend it.

See PRD *Text, graphemes, and safe presentation* (sanitization bullets).

### How to verify

- **Manual**:
  1. Create a file whose name contains a newline and an ESC byte and a matching line containing `\x1b]0;pwned\x07`; run `vrg`. The list shows `\n`/escaped bytes; the content row shows `^[]0;pwned^G`; the terminal title is not changed.
  2. `vrg foo "$(printf 'bad\x1bdir')"` → stderr shows an escaped name, exit 2.
- **Automated**:
  - Unit tests for path escaping (each rule), content escaping with byte→cell maps (a match covering an ESC byte highlights both `^` and `[`), standalone CR → `^M` — the Issue 5 core cases re-run unchanged against the generalized utility (regression), plus new diagnostic escaping tests preserving line breaks and single-lining embedded filenames.
  - **Sink-safety table test.** The Issue 5 hostile fixture set — OSC (`\x1b]0;x\x07`), CSI (`\x1b[2J`), C0 (`\x07`, `\x08`, `\x1b`), C1 (`\xc2\x85`), DEL, standalone CR, invalid UTF-8 path bytes, embedded filename newline — restructured as a shared table over every sink existing at this point (file list entry, filename rule, panel content, usage-error stderr). For each sink, drive the fixture through the real composition path and assert on the **raw output** — *before* any ANSI stripping — that no fixture control byte survives verbatim. Do not verify by stripping ANSI and comparing; stripping would erase the very evidence being sought.
  - To separate trusted app styling from fixture controls, render with a **no-style composition path** (Theme styles disabled / Lip Gloss colour profile set to ASCII) so the only escape bytes that may legitimately appear are none; assert the raw output contains no `\x1b`, `\x07`, `\x9b`, `\xc2\x85`, or bare `\r` at all. Additionally, with styles enabled, assert the fixture's distinctive payload (e.g. `]0;pwned`) never appears immediately after an unescaped ESC.
  - Rendering tests for panel content with invalid UTF-8: U+FFFD present, and highlights over well-formed escape expansions land exactly; for invalid-byte runs assert only that a highlight is present and does not paint outside the line (best-effort per PRD).

### Acceptance criteria

- [ ] Given a path with newline, tab, backslash, and invalid bytes, when displayed anywhere, then it renders as `\n`, `\t`, `\\`, `\xNN` respectively while the original bytes are still used to open the file.
- [ ] Given content containing ESC or other C0/C1 controls, when rendered, then caret/`\uXXXX` forms appear and a highlight over those bytes covers all their cells.
- [ ] Given content containing a standalone CR, then it renders as `^M` and is neither emitted raw nor dropped.
- [ ] Given invalid UTF-8 content, then U+FFFD is shown, exact highlight placement holds for well-defined escape expansions and valid UTF-8, and invalid-byte highlights are best-effort but never zero cells and never outside the line.
- [ ] Given a diagnostic embedding a filename with a newline, then the diagnostic keeps its own line breaks and the filename appears single-lined.
- [ ] Given every sink existing at this issue, when the hostile fixture set is rendered through the no-style composition path, then the raw output contains no control bytes from the fixture.
- [ ] Given the Issue 5 core escaping tests and browse-sink safety rows, when the shared utility replaces the Issue 5 core and Issue 1 escaper, then all of them still pass without modification (no regression).
- [ ] Given the sink-safety table, then it is structured so later issues add rows for new sinks without duplicating fixtures.

### User stories addressed

- User story 25 (completion): original filename bytes retained; invalid bytes visibly escaped — Issue 5 lands the browse sinks, this issue generalizes to every sink
- User story 75 (completion): invalid UTF-8 replaced and controls escaped in every sink — Issue 5 lands the browse sinks, this issue generalizes to every sink

---
