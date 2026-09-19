# Tasks for #29: Stale-match validation and "file changed since search" note

Parent issue: #29
Parent PRD: PRD-vrg.md
**Blocked by issues**: #22, #27, #28
**Acceptance criteria**: AC1, AC5 → Tasks 1–2; AC2–AC4, AC6–AC7 → Tasks 3–4

## Tasks

### 1. Specify stale-match validation

**Type**: RED  
**Output**: Failing FileBuffer tests cover out-of-bounds and same-length replacement, partial survival, the clamped-start and last-line fallbacks, the empty file, reload recomputation, validation against original/search bytes, and CRLF terminator matches.  
**Depends on**: none

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Begin only after Issues #22, #27, and #28 are complete. Add failing tests in `internal/filebuffer` for the Issue #29 contracts and the Encodings and stale-content validation section of `Notes/PRD-vrg.md`. Require each submatch to be checked on first load and every reload for line existence, range validity against original line bytes including terminators with the Issue #22 UTF-8 BOM adjustment, and byte equality with the recorded submatch bytes in either JSON encoding; any failure to drop that submatch, mark the buffer stale, and keep other valid highlights; a stale stop with surviving submatches to expose the first survivor as the reveal target; a stop whose line exists but has no survivors to expose the first recorded start clamped to the available line bytes and mapped to a valid display cell, with the end-of-line fallback clamped to the last rendered cell when there is no marker cell; a missing line to land at the last source line's start; an empty file to remain a zero-line panel; reload to recompute staleness so the note clears only on fully validating content; and validation to compare against original and search bytes rather than stripped display text, with a CRLF terminator match still validating. Keep this task test-only.

---

### 2. Implement stale validation and fallback targets

**Type**: GREEN  
**Output**: Validation, survival, fallback, and reload-recomputation tests pass.  
**Depends on**: 1

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Implement the best-effort validation in `internal/filebuffer` to satisfy Task 1: line, range, and byte-equality checks against original/search bytes with BOM adjustment, per-submatch drops with stale marking, surviving highlight retention, and the fallback reveal targets — first survivor, clamped recorded start, last source line — with fallbacks inventing no highlights or markers. Do not run validation against UTF-16/32 bytes; Issue #30 excludes them.

---

### 3. Specify the stale note and fallback reveal integration

**Type**: RED  
**Output**: Failing App tests cover the filename-row note through Issue #24's slot at ordinary and constrained widths, the gated-reload survivor and fallback reveals through the Issue #28 commit, the all-stale outcome-matrix row, and the no-invented-highlight rule.  
**Depends on**: 2

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Add failing App-level tests for the Issue #29 integration. Require a stale buffer to show "file changed since search" in the filename row's status slot on every display with no timer, composed at ordinary and constrained widths with the path truncating to make room, nothing overflowing, and nonnegative dimensions; the latest selected stop's first recorded submatch dropped by content loaded during a gated reload to reveal the first surviving submatch — or the clamped fallback — when the reload's matching prepared layout installs per Issue #28's two-stage path; an all-dropped stop with its line present to reveal the clamped start with no highlight; a missing line to land at the last source line; and an all-stale index to leave the fixed exit status unchanged as a new outcome-matrix row. Keep this task test-only.

---

### 4. Implement the stale note and fallback reveal

**Type**: GREEN  
**Output**: Note, fallback-reveal, and outcome-row tests pass.  
**Depends on**: 3

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Implement the stale note in the Issue #24 status slot with its reload recomputation, and feed the stale fallback targets into the Issue #28 reveal intent so the commit reveals the first survivor or fallback. Stale state never changes the fixed exit status; add the outcome-matrix row.

---

### 5. Create the stale-validation finish marker

**Type**: FINISH MARKER  
**Output**: Finish marker exists at `Notes/finish-markers/029-06/finish-marker.md`.  
**Depends on**: 4

Write `Task 029-06 finished successfully at <time>` to `Notes/finish-markers/029-06/finish-marker.md`, replacing `<time>` with the current UTC timestamp (for example, `date -u +"%Y-%m-%dT%H:%M:%SZ"`). Create the `Notes/finish-markers/029-06/` directory if it does not already exist.

---
