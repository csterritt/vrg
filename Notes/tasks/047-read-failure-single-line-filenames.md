# Tasks for #47: Read-failure diagnostics single-line embedded filenames

Parent issue: #47
Parent PRD: PRD-vrg.md
**Blocked by issues**: none
**Acceptance criteria**: AC1–AC5 → Tasks 1–2
**Manual verification**: Task 4 owns the issue's manual checks.

## Tasks

### 1. Specify single-line read-failure diagnostics

**Type**: RED  
**Output**: Failing tests drive real load failures for filenames containing newline, tab, invalid UTF-8, and ESC bytes, asserting exactly one diagnostic line carrying the `EscapePath`-escaped path and a sanitized reason with no raw-path repetition — through the overlay row set and the stderr replay, at every load site.  
**Depends on**: none

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Add failing tests in `internal/app` (and `internal/filebuffer` if the reason is unwrapped there) that trigger genuine read failures — e.g. permission removal after indexing — for filenames containing newline, tab, invalid UTF-8, and ESC bytes. Require each failure to produce exactly one diagnostic line: the embedded path in `safepresentation.EscapePath`-escaped form, and a reason portion that does not repeat the raw unescaped path — no `PathError.Error()` passthrough; unwrap to `PathError.Err` or an equivalent sanitized reason. Assert the same single line through the overlay row set and the collected diagnostics replayed to stderr (replay introduces no re-splitting), and the same construction at the initial-load, `r` reload, and failed-path re-entry retry sites. The current code passes `err.Error()` to `EscapeDiagnostic`, which preserves an embedded newline as a diagnostic boundary — the newline-filename cases fail. Keep this task test-only.

---

### 2. Build read-failure diagnostics from escaped path plus sanitized reason

**Type**: GREEN  
**Output**: The single-line tests pass; every file-load diagnostic site composes `EscapePath(path)` plus a sanitized reason that never repeats the raw path, and one failed read always produces exactly one diagnostic line.  
**Depends on**: 1

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Change the file-load diagnostic construction so `internal/app`'s `FileLoadCompleteMsg` error handling (and `internal/filebuffer` if the diagnostic string is composed at load time) builds each read-failure diagnostic as the `safepresentation.EscapePath`-escaped path plus a sanitized reason that does not embed the raw path — unwrap `*os.PathError` to its `Err` rather than reusing `err.Error()`. Apply the same construction at every site producing a file-load diagnostic: initial load, `r` reload, and the failed-path re-entry retry. Whatever bytes the filename contains, one failed read must produce exactly one diagnostic line in both the overlay and the exit stderr replay. Run the focused tests plus `go build ./...`, `go vet ./...`, and `go test ./...`.

---

### 3. Document single-line read-failure diagnostics

**Type**: DOCUMENT  
**Output**: Wiki documentation records the escaped-path-plus-reason construction and its single-line guarantee at every load site.  
**Depends on**: 2

Read and follow `Notes/wiki/wiki-rules.md` and the schema in `Notes/wiki/AGENTS.md`, then ingest the completed Issue #47 fix into the appropriate pages under `Notes/wiki`. Document that read-failure diagnostics are composed from an `EscapePath`-escaped path and a sanitized reason that never repeats the raw path, that one failed read always yields exactly one diagnostic line in both the overlay and the replay, and that the construction is uniform across initial load, reload, and retry. Cross-reference Issue #47 and the *Text, graphemes, and safe presentation* and *File loading, cache, reload, and selection consistency* sections of `Notes/PRD-vrg.md`, update `Notes/wiki/index.md`, and append the required dated ingest record to `Notes/wiki/log.md` without rewriting previous entries.

---

### 4. Create the read-failure-diagnostic walkthrough

**Type**: CODE WALKTHROUGH  
**Output**: Showboat walkthrough exists at `Notes/walkthroughs/047-04/code-walkthrough`.  
**Depends on**: 3

Use showboat, consulting `uvx showboat --help`, to create the walkthrough at exactly `Notes/walkthroughs/047-04/code-walkthrough`, with the main file named `walkthrough.md`. Demonstrate the single-line diagnostic tests, then run the issue's manual scenario: files named with embedded newline, tab, invalid UTF-8, and ESC bytes with a read failure arranged (permission removal after indexing) → each failure surfaces as exactly one diagnostic line with the path escaped inline, in both the overlay and the exit stderr replay. Capture commands, outputs, and exit statuses. Reference Issue #47 and `Notes/PRD-vrg.md`, and store every generated artifact in the approved directory.

---
