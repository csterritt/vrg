# Tasks for #13: `n`/`p` circular matched-line navigation; cursor-derived current file

Parent issue: #13
Parent PRD: PRD-vrg.md
**Blocked by issues**: #7, #12
**Acceptance criteria**: AC1–AC5 → Tasks 1–2
**Manual verification**: Task 4 owns the issue's manual checks.

## Tasks

### 1. Specify the circular matched-line cursor

**Type**: RED  
**Output**: Failing SearchIndex tests cover wrap, file-change flags, and the single-stop and empty no-ops; failing App tests cover startup selection, cross-file switching with load requests, manual-scroll independence, and the one-stop rule.  
**Depends on**: none

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Begin only after Issues #7 and #12 are complete. Add failing tests in `internal/searchindex` and `internal/app` for the Issue #13 contracts and the first three bullets of the Navigation, viewport, and logical anchors section of `Notes/PRD-vrg.md`. Require startup to select the first stop in path-then-line order, `n`/`p` to advance and retreat circularly with wrap at both ends, zero entries and exactly one entry to be strict no-ops without pop-up or reload, and multiple submatches on one line to be one stop; expose the cursor's file-change information for App wiring. At the App level require crossing to another file's stop to switch the panel, request that file's load when uncached, save the departing file's viewport and start the new file from its saved viewport or the top, move the list underline, and render the current matched line's matches with the Issue #7 underline style; require manual scrolling to leave the cursor unchanged so `n`/`p` continue from the last selected stop; and require the file list to remain passive with no direct selection route. Keep this task test-only.

---

### 2. Implement the cursor and App wiring

**Type**: GREEN  
**Output**: Cursor and wiring tests pass; the current file derives from the cursor and the list underline follows it.  
**Depends on**: 1

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Implement the circular cursor in `internal/searchindex` and its App wiring to satisfy Task 1. Destination reveal is owned by Issue #14 — same-file navigation here only updates current-line styling — and the cached-file stale-layout request path is owned by Issue #17, so a simple immediate panel switch with the saved-viewport handoff is acceptable in this issue.

---

### 3. Document the navigation cursor

**Type**: DOCUMENT  
**Output**: Wiki documentation records the cursor model, stop ordering, no-op rules, and the passive file list.  
**Depends on**: 2

Read and follow `Notes/wiki/wiki-rules.md` and the schema in `Notes/wiki/AGENTS.md`, then ingest the completed Issue #13 implementation and tests into the appropriate pages under `Notes/wiki`. Document the single global matched-line cursor, path-then-line stop order, circular `n`/`p` with the zero- and one-entry no-ops, the cursor-derived current file with load requests and viewport handoff, the list underline following selection, manual scrolling leaving the cursor unchanged, and the passive file list with no direct selection route. Cross-reference Issue #13 and the Navigation, viewport, and logical anchors section of `Notes/PRD-vrg.md`, update `Notes/wiki/index.md`, and append the required dated ingest record to `Notes/wiki/log.md` without rewriting previous entries.

---

### 4. Create the navigation walkthrough

**Type**: CODE WALKTHROUGH  
**Output**: Showboat walkthrough exists at `Notes/walkthroughs/013-04/code-walkthrough`.  
**Depends on**: 3

Use showboat, consulting `uvx showboat --help`, to create the walkthrough at exactly `Notes/walkthroughs/013-04/code-walkthrough`, with the main file named `walkthrough.md`. Demonstrate the SearchIndex cursor tests and the App wiring tests, then run the binary with several matched files: `n` repeatedly moving the current-line underline through lines and across files with the list underline following, wrapping from the last stop to the first, `p` reversing, and a manual scroll followed by `n` continuing from the previous stop. Reference Issue #13 and `Notes/PRD-vrg.md`, and store every generated artifact in the approved directory.

---
