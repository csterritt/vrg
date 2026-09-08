# Tasks for #5: Browse tracer — file list, file panel with highlights, async "Loading…"

Parent issue: #5
Parent PRD: PRD-vrg.md
**Blocked by issues**: #3, #4
**Acceptance criteria**: AC7–AC8 → Tasks 1–2; AC1–AC6, AC9 → Tasks 3–4
**Manual verification**: Task 6 owns the issue's manual checks.

## Tasks

### 1. Specify the safe-presentation core for paths and content

**Type**: RED  
**Output**: Failing unit tests cover every path escape rule, content escapes with retained raw-byte mappings, the standalone-CR and provisional tab forms, and byte→cell maps for escaped forms.  
**Depends on**: none

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Begin only after Issues #3 and #4 are complete. Add failing unit tests for the safe-presentation core Issue #5 lands ahead of the first arbitrary-data render, per the Text, graphemes, and safe presentation section of `Notes/PRD-vrg.md`. For paths require `\n`, `\r`, `\t` escaped as `\n`, `\r`, `\t`, invalid UTF-8 bytes as `\xNN`, literal backslash as `\\`, other C0/C1 controls escaped safely, and valid printable Unicode preserved. For file content require invalid UTF-8 to render as U+FFFD while raw-byte mappings are retained, C0 controls and DEL in caret notation (`^[` for ESC), C1 controls as `\u0085`-style escapes, LF and CRLF never displayed, a standalone CR escaped as `^M`, and tab rendered as a single `→` placeholder cell pending Issue #16, with no test asserting specific cell positions on tab-containing lines. Require escaped forms to expose byte→cell mappings so a match covering an ESC byte highlights both `^` and `[`. Keep this task test-only.

---

### 2. Implement the safe-presentation core

**Type**: GREEN  
**Output**: Path and content escaping tests pass with byte→cell mappings for every escaped form.  
**Depends on**: 1

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Implement only the escaping core and its byte→cell mappings, placed within the established layout as a focused safe-presentation source that Issue #6 will unify with the Issue #1 escaper. Preserve original bytes for identity, ordering, and file access, never let displayed strings become filesystem keys, and keep mappings accurate for every escaped form so later highlight rendering can consume them. Do not render any browse view yet.

---

### 3. Specify FileBuffer loading and the browse composition

**Type**: RED  
**Output**: Failing FileBuffer tests cover byte loading, line counts, gutter width, and highlight spans; failing App model tests cover the browse view, placeholder, gated-load responsiveness, and exit paths; failing rendering tests cover list order, underline, gutter format, and the hostile-fixture raw-output checks for the three browse sinks.  
**Depends on**: 2

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Add failing tests for the Issue #5 browse tracer. In `internal/filebuffer` require loading a file's bytes to yield the source-line count, the gutter width (digit width of the loaded file's largest line number plus two spaces, minimum one digit slot), and highlight spans for a matched line, with the completion message carrying a prepared, decoded and mapped buffer so `Update` does no full-file work. In `internal/app` require a completed search to present the two-pane browse view with "Loading…" until the current file's load completes, content then rendered with matches in inverse video, key and resize messages handled while the load — read and decode/map — is held by a worker gate, `ctrl+c` through the Issue #4 path to 130, and `q` to exit 0 through the same cleanup path. Add rendering tests on the composed `View()` string for file-list order, current-entry underline, the filename rule, and gutter format, plus the hostile-fixture raw-output tests: OSC, CSI, C0, C1, DEL, standalone CR, invalid UTF-8 path bytes, and an embedded filename newline driven through the real composition path for the file-list, filename-rule, and panel-content sinks via a no-style composition path, asserting on raw output before any ANSI stripping that no fixture control byte survives verbatim. Keep this task test-only.

---

### 4. Implement FileBuffer and the browse composition

**Type**: GREEN  
**Output**: FileBuffer, model, rendering, and sink-safety tests pass; the browse view shows the list, filename rule, gutter, and inverse-video matches with responsive async loading.  
**Depends on**: 3

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Implement the first FileBuffer path, a minimal Viewport and Theme seam, and the App browse composition to satisfy Task 3. Load full files asynchronously off the UI update path with the completion message carrying the prepared buffer, render the file list in raw-path order with a simple fixed or heuristic width (Issue #24 owns the real formula), keep the current file underlined and the list scrolled to it, embed the escaped filename in a horizontal rule, right-justify the gutter followed by two spaces, draw no other file-panel borders, and render matched spans in inverse video over the escaped display text using the Task #2 core's byte→cell maps. Show the top of the file with no reveal yet, route every sink through the safe-presentation core, and exit through the Issue #4 cleanup path.

---

### 5. Document the browse tracer

**Type**: DOCUMENT  
**Output**: Wiki documentation records the browse composition, FileBuffer first path, safe-presentation core rules, and the sink-safety method.  
**Depends on**: 4

Read and follow `Notes/wiki/wiki-rules.md` and the schema in `Notes/wiki/AGENTS.md`, then ingest the completed Issue #5 `internal/filebuffer`, `internal/viewport`, `internal/theme`, and `internal/app` implementation and tests into the appropriate pages under `Notes/wiki`. Document the two-pane browse layout with its filename rule, gutter, and border rules; async loading with prepared buffers and the "Loading…" placeholder; the safe-presentation core's path and content rules with their byte→cell mappings including the provisional tab form; the hostile-fixture raw-output sink-safety method with its no-style composition path; and the fixed-width file list pending Issue #24. Cross-reference Issue #5 and the File list and layout, Text, graphemes, and safe presentation, and Module Design sections of `Notes/PRD-vrg.md`, update `Notes/wiki/index.md`, and append the required dated ingest record to `Notes/wiki/log.md` without rewriting previous entries.

---

### 6. Create the browse-tracer walkthrough

**Type**: CODE WALKTHROUGH  
**Output**: Showboat walkthrough exists at `Notes/walkthroughs/005-06/code-walkthrough`.  
**Depends on**: 5

Use showboat, consulting `uvx showboat --help`, to create the walkthrough at exactly `Notes/walkthroughs/005-06/code-walkthrough`, with the main file named `walkthrough.md`. Demonstrate the escaping unit tests, FileBuffer tests, model tests including the gated load, rendering tests, and the sink-safety raw-output tests, then run the binary in a repository to show the file list, filename rule, gutter, and inverse-video matches, a resize, and `q` exiting 0. Include the hostile-fixture manual case: a filename containing a newline and an ESC byte with a matching line containing an OSC sequence, showing escaped forms in the list, rule, and content with the terminal title unchanged. Reference Issue #5 and `Notes/PRD-vrg.md`, and store every generated artifact in the approved directory.

---
