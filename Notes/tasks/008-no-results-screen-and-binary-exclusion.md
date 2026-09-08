# Tasks for #8: "No results found" screen (exit 1) and binary-file exclusion

Parent issue: #8
Parent PRD: PRD-vrg.md
**Blocked by issues**: #5, #6
**Acceptance criteria**: AC1–AC5 → Tasks 1–2
**Manual verification**: Task 4 owns the issue's manual checks.

## Tasks

### 1. Specify binary exclusion and the no-results screen

**Type**: RED  
**Output**: Failing SearchIndex tests cover binary exclusion after earlier matches and the distinct-file count; failing App tests cover the empty rg-1 stream, the all-binary rg-0 stream with its count text, mixed retention, `Esc` no-op, `ctrl+c` → 130, and `q` → 1.  
**Depends on**: none

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Begin only after Issues #5 and #6 are complete. Add failing tests in `internal/searchindex` and `internal/app` for the Issue #8 contracts and the binary bullet of the Result index section plus the last row of the Outcome and exit-status contract in `Notes/PRD-vrg.md`. Require a valid `end` event with non-null `binary_offset` to drop that file and all its previously collected matches and to count distinct excluded files; require the usable-results value to be retained stops after filtering, never received match events, exposed as the single value the outcome logic consumes; and require the model to present the centred "No results found" screen after a complete successful search (rg exit 0 or 1) with no usable results, appending "(N binary files skipped)" when every matched file was excluded — for rg-1 emptiness and rg-0 all-filtered alike — with `q` exiting 1 through the Issue #4 cleanup path, `Esc` a no-op, and `ctrl+c` 130. Include the mixed stream where one file is binary-excluded and one retained, which browses with usable results of 1. Keep this task test-only.

---

### 2. Implement binary exclusion and the no-results screen

**Type**: GREEN  
**Output**: Exclusion, usable-results, and no-results tests pass; empty and all-binary searches dismiss to exit 1.  
**Depends on**: 1

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Implement only enough in `internal/searchindex` and `internal/app` to satisfy Task 1: binary exclusion on non-null `binary_offset` with distinct-file counting, the retained-stops usable-results value, and the centred no-results screen with its optional binary-skip suffix, `q` → 1, `Esc` no-op, and `ctrl+c` → 130 through existing paths. Do not add error overlays, warnings, or record skipping owned by Issues #9 and #10.

---

### 3. Document the no-results outcome

**Type**: DOCUMENT  
**Output**: Wiki documentation records binary exclusion, usable-results accounting, and the no-results screen contract.  
**Depends on**: 2

Read and follow `Notes/wiki/wiki-rules.md` and the schema in `Notes/wiki/AGENTS.md`, then ingest the completed Issue #8 implementation and tests into the appropriate pages under `Notes/wiki`. Document binary exclusion via non-null `binary_offset` with its distinct-file count, usable results as retained stops after filtering, the "No results found" screen with its binary-skip suffix for both rg-1 and rg-0 all-filtered streams, and the exit-1 dismissal with `Esc` and `ctrl+c` behavior. Cross-reference Issue #8 and the Result index and Outcome and exit-status contract sections of `Notes/PRD-vrg.md`, update `Notes/wiki/index.md`, and append the required dated ingest record to `Notes/wiki/log.md` without rewriting previous entries.

---

### 4. Create the no-results walkthrough

**Type**: CODE WALKTHROUGH  
**Output**: Showboat walkthrough exists at `Notes/walkthroughs/008-04/code-walkthrough`.  
**Depends on**: 3

Use showboat, consulting `uvx showboat --help`, to create the walkthrough at exactly `Notes/walkthroughs/008-04/code-walkthrough`, with the main file named `walkthrough.md`. Demonstrate the SearchIndex exclusion tests and the App outcome tests, then run the binary manually: `vrg zzzznotfound .` showing "No results found" with `q` exiting 1, and a directory containing only a binary file with a match showing "No results found (1 binary files skipped)" with `q` exiting 1. Reference Issue #8 and `Notes/PRD-vrg.md`, and store every generated artifact in the approved directory.

---
