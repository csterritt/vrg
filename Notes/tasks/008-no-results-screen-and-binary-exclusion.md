# Tasks for #8: "No results found" screen (exit 1) and binary-file exclusion

Parent issue: #8
Parent PRD: PRD-vrg.md
**Blocked by issues**: #5, #6
**Acceptance criteria**: AC1–AC5 → Tasks 1–2

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

### 3. Create the no-results finish marker

**Type**: FINISH MARKER  
**Output**: Finish marker exists at `Notes/finish-markers/008-04/finish-marker.md`.  
**Depends on**: 2

Write `Task 008-04 finished successfully at <time>` to `Notes/finish-markers/008-04/finish-marker.md`, replacing `<time>` with the current UTC timestamp (for example, `date -u +"%Y-%m-%dT%H:%M:%SZ"`). Create the `Notes/finish-markers/008-04/` directory if it does not already exist.

---
