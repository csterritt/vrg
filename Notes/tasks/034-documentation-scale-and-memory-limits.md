# Tasks for #34: Documentation — scale examples, content assumptions, memory limits

Parent issue: #34
Parent PRD: PRD-vrg.md
**Blocked by issues**: #10, #31
**Acceptance criteria**: AC1–AC5 → Tasks 1–2
**Manual verification**: Task 4 owns the issue's manual checks.

## Tasks

### 1. Specify documentation synchronization

**Type**: RED  
**Output**: Failing tests require the README to contain every binding from Issue #31's table, every allow-listed flag from Issue #2, complete exit-status documentation agreeing with the outcome function, and the ripgrep 15.x and `--no-config` statements.  
**Depends on**: none

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Begin only after Issues #10 and #31 are complete. Add failing tests for the Issue #34 contracts and the Resources and responsiveness section of `Notes/PRD-vrg.md`. Require a test iterating Issue #31's binding table to find each binding in the README, or generating the section from the same table and asserting the committed README is up to date; a test that the README lists each allow-listed flag from Issue #2's table; a test that the README's exit-status documentation covers 0, 1, 2, and 130, including the cancellation triggers (`q` during search or result preparation, `ctrl+c` anywhere) and the pre-TUI usage, root, and start failures, with its search-derived values agreeing with Issue #9's outcome function; and a test that the README identifies ripgrep 15.x as the reference family and states that VRG supplies `--no-config` so ripgrep configuration files are never honoured. Keep this task test-only.

---

### 2. Write the README and help footer

**Type**: GREEN  
**Output**: Documentation synchronization tests pass; the README and the help footer note state the scale examples, record limits, and matched invocation and exit documentation.  
**Depends on**: 1

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Write the user-facing `README.md` at the repository root and the help overlay footer note to satisfy Task 1: the three independent scale examples — approximately 10,000 matched files, 100,000 matched lines, and individual files around 50 MB — explicitly not simultaneous capacity guarantees; the ~50 MB example's UTF-8 and ordinary-line-length assumptions with the base64 expansion caveat that can push a single match record over the 64 MiB limit, and the oversized-record diagnostic naming the path when recoverable; session-long buffer retention with no eviction, no aggregate memory bound, and no reliable OOM or forced-termination cleanup guarantee; the invocation syntax, flag allow-list, and key bindings consumed from Issue #31's binding table; the complete exit-status table for 0, 1, 2, and 130; and the ripgrep 15.x reference family with `--no-config`. Route any runtime string embedded in generated text through the Issue #6 utility; static text needs no sanitization.

---

### 3. Document the release-facing documentation

**Type**: DOCUMENT  
**Output**: Wiki documentation records the README's scope, its synchronization tests, and the footer note.  
**Depends on**: 2

Read and follow `Notes/wiki/wiki-rules.md` and the schema in `Notes/wiki/AGENTS.md`, then ingest the completed Issue #34 README, footer note, and synchronization tests into the appropriate pages under `Notes/wiki`. Document the README as the single user-facing documentation artifact, the binding-table and allow-list synchronization tests that keep it honest, the scale examples and their independence, the 64 MiB record limit and base64 caveat, the memory and termination limits, and the exit-status table's agreement with the outcome function. Cross-reference Issue #34 and the Resources and responsiveness and Out of Scope sections of `Notes/PRD-vrg.md`, update `Notes/wiki/index.md`, and append the required dated ingest record to `Notes/wiki/log.md` without rewriting previous entries.

---

### 4. Create the documentation walkthrough

**Type**: CODE WALKTHROUGH  
**Output**: Showboat walkthrough exists at `Notes/walkthroughs/034-04/code-walkthrough`.  
**Depends on**: 3

Use showboat, consulting `uvx showboat --help`, to create the walkthrough at exactly `Notes/walkthroughs/034-04/code-walkthrough`, with the main file named `walkthrough.md`. Demonstrate the synchronization tests running green against the committed README and the help footer note rendering in the overlay, then read through the README's scale, record-limit, memory, invocation, flag, binding, and exit-status sections against the implementation. Reference Issue #34 and `Notes/PRD-vrg.md`, and store every generated artifact in the approved directory.

---
