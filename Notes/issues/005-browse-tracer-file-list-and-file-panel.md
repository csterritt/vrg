## Issue 5: Browse tracer — file list, file panel with highlights, async "Loading…"

**Type**: AFK
**Blocked by**: Issue 3, Issue 4

### Parent PRD

`Notes/PRD-vrg.md`

### What to build

Replace the interim summary screen with the real two-pane browse view for the first matched file. This is the primary tracer bullet through FileBuffer, Viewport (minimal), Theme (minimal) and App composition.

**Safe presentation core — prerequisite of the first arbitrary-data render.** This issue is the first to render external path and content bytes, so it lands the core safe-presentation rules for the sinks it creates (file-list entries, filename rule, panel content) before any raw path/content byte reaches `View()`:

- **Paths**: escape `\n`, `\r`, `\t` as `\n`, `\r`, `\t`; invalid UTF-8 bytes as `\xNN`; literal backslash as `\\`; other C0/C1 controls safely; preserve valid printable Unicode. Original bytes remain the key for identity, ordering and file access; displayed strings never become filesystem keys.
- **File content**: invalid UTF-8 → U+FFFD while retaining raw-byte mappings; C0 controls and DEL → caret notation (`^[` for ESC); C1 → `\u0085`-style escapes. LF and CRLF are line terminators and are never displayed; a standalone CR (no following LF) is *not* a terminator and is escaped as `^M`; tab is never emitted raw in this first pass (a safe placeholder rendering is acceptable; the structural eight-column-stop rule is Issue 22). Escaped forms expose byte→cell mappings so highlights can cover them.
- Issue 6 generalizes this core into the shared all-sink utility (diagnostics, usage errors, later sinks) and establishes the extensible sink-safety table; generalizing must not regress these rules or their tests.

- Left: file list of every retained file in raw-path order; the current file (the first stop's file) underlined; the list scrolls to keep the current entry visible. Use a simple fixed/heuristic width for now (Issue 24 implements the real formula).
- Right: filename embedded in a horizontal rule, then content rows with a right-justified line-number gutter (digit width of largest line number, min 1) followed by two spaces. No other borders.
- FileBuffer loads the file asynchronously; "Loading…" placeholder until ready. Matches on matched lines render in inverse video over the escaped display text (first-pass byte→cell mapping including escaped forms; grapheme and terminator refinements come in Issues 21–23, tab's structural expansion in Issue 22).
- Show the top of the file (no reveal yet); `q` exits 0.
- Loading runs off the UI update path; the model handles key/resize messages while a load is pending. "Loading" includes decoding and byte→cell mapping, not only the disk read: the completion message delivers a prepared buffer, and `Update` does not decode or map the full file.
- Browse-state `q` goes through the Issue 4 cleanup/exit path (terminal restore, child reap if still running).

See PRD *File list and layout* stories, *File loading, cache, reload…* (first two bullets), *Layout and indicators* (gutter bullet), *Text, graphemes, and safe presentation* (sanitization bullets), and *Module Design → FileBuffer / Viewport / App*.

### How to verify

- **Manual**:
  1. `vrg func .` in a Go repo → file list left, first file's content right with matches in inverse.
  2. Gutter is right-justified with two trailing spaces; filename appears in a rule above the content.
  3. Resize the terminal; layout re-renders. `q` exits 0.
  4. Create a file whose name contains a newline and an ESC byte and a matching line containing `\x1b]0;pwned\x07`; run `vrg`. The list and filename rule show escaped `\n`/`\xNN` forms; the content row shows `^[]0;pwned^G`; the terminal title is not changed.
- **Automated**:
  - FileBuffer: loads bytes, reports source-line count and gutter width, exposes highlight spans for a matched line.
  - App model: injected search completion → browse view with "Loading…"; injected load completion → content visible with highlight span; a key message and a resize handled while the load (read **and** decode/map) is held by a worker gate; `ctrl+c` while the gate is held exits 130 (Issue 4 path).
  - Rendering test on the composed `View()` string: file list order, underline on current entry, gutter format.
  - Unit tests for the core path and content escaping rules: each path escape (`\n`, `\r`, `\t`, `\\`, `\xNN`, other controls), invalid UTF-8 → U+FFFD with retained raw-byte mappings, standalone CR → `^M`, and byte→cell maps for escapes (a match covering an ESC byte highlights both `^` and `[`).
  - **Sink-safety raw-output test** for every sink existing at this issue (file-list entry, filename rule, panel content): a fixture set of hostile inputs — OSC (`\x1b]0;x\x07`), CSI (`\x1b[2J`), C0 (`\x07`, `\x08`, `\x1b`), C1 (`\xc2\x85`), DEL, standalone CR, invalid UTF-8 path bytes, embedded filename newline — driven through the real composition path, asserting on the **raw output**, *before* any ANSI stripping, that no fixture control byte survives verbatim. Render via a **no-style composition path** (Theme styles disabled / Lip Gloss colour profile set to ASCII) so no escape byte may legitimately appear; do not verify by stripping ANSI and comparing, which would erase the evidence being sought. Issue 6 restructures this fixture set into the shared, extensible sink-safety table.

### Acceptance criteria

- [ ] Given a completed search with results, when browsing begins, then every retained file is listed in raw-path order and the current file is underlined.
- [ ] Given the current file is not yet loaded, then the panel shows "Loading…" and input remains responsive.
- [ ] Given the load completes, then content replaces the placeholder and each matched span is rendered in inverse video.
- [ ] Given a loaded file, then the gutter width is the digit count of the largest line number plus two spaces, right-justified, and no left/right/bottom border is drawn.
- [ ] Given the browse view, when `q` is pressed, then the process exits 0 via the Issue 4 cleanup path.
- [ ] Given a load whose decode/map phase is held by a test gate, when `ctrl+c` or a resize arrives, then it is handled without waiting on the gate.
- [ ] Given a path with newline, tab, backslash, and invalid bytes, when displayed in the file list or filename rule, then it renders as `\n`, `\t`, `\\`, `\xNN` respectively while the original bytes are still used to open the file.
- [ ] Given content containing ESC or other C0/C1 controls or a standalone CR, when rendered, then caret/`\uXXXX`/`^M` forms appear and a highlight over those bytes covers all their cells.
- [ ] Given the hostile fixture set rendered through the no-style composition path for every sink existing at this issue, then the raw output contains no control bytes from the fixture.

### User stories addressed

- User story 24: every retained file listed in deterministic path order
- User story 25 (part): original filename bytes retained and invalid bytes visibly escaped in the browse sinks (Issue 6 generalizes to every sink)
- User story 26: current file underlined and kept within the scrolled list
- User story 31: filename rule, gutter, no other borders
- User story 34: loading leaves input responsive with "Loading…"
- User story 75 (part): invalid UTF-8 replaced and controls escaped in the browse sinks (Issue 6 generalizes to every sink)

---
