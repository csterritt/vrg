# Tasks for #47: Read-failure diagnostics single-line embedded filenames

Parent issue: #47
Parent PRD: PRD-vrg.md
**Blocked by issues**: none
**Acceptance criteria**: AC1–AC5 → Tasks 1–2
**Manual verification**: Task 3 owns the issue's manual checks.

## Tasks

### 1. Specify single-line read-failure diagnostics

**Type**: RED  
**Output**: Failing tests drive real load failures for filenames containing newline, tab, invalid UTF-8, and ESC bytes, asserting exactly one diagnostic line carrying the `EscapePath`-escaped path and a sanitized reason with no raw-path repetition — through the overlay row set and the stderr replay, at every load site.  
**Depends on**: none

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Add failing tests in `internal/app` (and `internal/filebuffer` if the reason is unwrapped there) that trigger genuine read failures for filenames containing newline, tab, invalid UTF-8, and ESC bytes. Build every fixture in a disposable temporary directory and synchronize deterministically: index the file, hold the load at an explicit gate, remove or rename the fixture, then release the real `os.ReadFile` attempt and assert the expected path error. Do not rely on `chmod` denial, which may succeed under elevated privileges or unusual ACLs. Require each failure to produce exactly one diagnostic line: the embedded path in `safepresentation.EscapePath`-escaped form, and a reason portion that does not repeat the raw unescaped path — no `PathError.Error()` passthrough; unwrap to `PathError.Err` or an equivalent sanitized reason. Assert the same single line through the overlay row set and the collected diagnostics replayed to stderr (replay introduces no re-splitting), and the same construction at the initial-load, `r` reload, and failed-path re-entry retry sites. Require cleanup on success and failure so no hostile filename or changed permission survives outside the temporary directory. The current code passes `err.Error()` to `EscapeDiagnostic`, which preserves an embedded newline as a diagnostic boundary — the newline-filename cases fail. Keep this task test-only.

---

### 2. Build read-failure diagnostics from escaped path plus sanitized reason

**Type**: GREEN  
**Output**: The single-line tests pass; every file-load diagnostic site composes `EscapePath(path)` plus a sanitized reason that never repeats the raw path, and one failed read always produces exactly one diagnostic line.  
**Depends on**: 1

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Change the file-load diagnostic construction so `internal/app`'s `FileLoadCompleteMsg` error handling (and `internal/filebuffer` if the diagnostic string is composed at load time) builds each read-failure diagnostic as the `safepresentation.EscapePath`-escaped path plus a sanitized reason that does not embed the raw path — unwrap `*os.PathError` to its `Err` rather than reusing `err.Error()`. Apply the same construction at every site producing a file-load diagnostic: initial load, `r` reload, and the failed-path re-entry retry. Whatever bytes the filename contains, one failed read must produce exactly one diagnostic line in both the overlay and the exit stderr replay. Run the focused tests plus `go build ./...`, `go vet ./...`, and `go test ./...`.

---

### 3. Create the read-failure-diagnostic walkthrough

**Type**: CODE WALKTHROUGH  
**Output**: Showboat walkthrough exists at `Notes/walkthroughs/047-04/code-walkthrough`.  
**Depends on**: 2

Use showboat, consulting `uvx showboat --help`, to create the walkthrough at exactly `Notes/walkthroughs/047-04/code-walkthrough`, with the main file named `walkthrough.md`. Demonstrate the single-line diagnostic tests, then run the issue's manual scenario entirely inside a disposable temporary directory: create files named with embedded newline, tab, invalid UTF-8, and ESC bytes, index each file, then deterministically remove or rename it before the gated read attempt so the real load fails → each failure surfaces as exactly one diagnostic line with the path escaped inline, in both the overlay and the exit stderr replay. If a permission-denial variant is also shown, install a cleanup trap that restores the mode and report an unsupported environment rather than passing when elevated privileges or ACLs permit the read. Capture cleanup evidence along with commands, outputs, and exit statuses. Reference Issue #47 and `Notes/PRD-vrg.md`, and store every generated artifact in the approved directory.

---
