# Tasks for #30: Unsupported encodings — UTF-16/UTF-32 BOM placeholder

Parent issue: #30
Parent PRD: PRD-vrg.md
**Blocked by issues**: #26, #27, #29
**Acceptance criteria**: AC1–AC6 → Tasks 1–2

## Tasks

### 1. Specify unsupported-encoding detection and presentation

**Type**: RED  
**Output**: Failing FileBuffer tests cover all four BOMs with the overlap ordering and UTF-8 non-misclassification; failing App tests cover current versus non-current notification, `r` reloadability, absent stale validation, the all-unsupported outcome row, and composed-view robustness.  
**Depends on**: none

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Begin only after Issues #26, #27, and #29 are complete. Add failing tests in `internal/filebuffer` and `internal/app` for the Issue #30 contracts and the first Encodings bullet of `Notes/PRD-vrg.md`. Require detection of UTF-16 LE/BE and UTF-32 LE/BE BOMs with longer BOMs checked before overlapping shorter ones so `FF FE 00 00` classifies as UTF-32 LE rather than UTF-16 LE, and a UTF-8 BOM not to be misclassified; a detected file to show "(unsupported encoding)" with no file text, no highlights, and an explanatory diagnostic, while remaining an indexed cursor stop that `r` can reload — reissuing the load and preserving the placeholder when unchanged; notification to follow the Issue #26 current/non-current distinction; no stale-match validation to run against these raw bytes even though Issue #29's validator exists; the all-unsupported outcome-matrix row to leave the fixed exit status; and the composed view at ordinary and constrained widths to keep the truncated safe path identified with nothing overflowing and nonnegative dimensions. Keep this task test-only.

---

### 2. Implement BOM detection and the placeholder

**Type**: GREEN  
**Output**: Detection, presentation, notification, reload, and outcome-row tests pass.  
**Depends on**: 1

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Implement the BOM detection with its overlap ordering in `internal/filebuffer`, the "(unsupported encoding)" placeholder with its explanatory diagnostic, the current/non-current notification split, reloadability through `r`, and the all-unsupported outcome-matrix row. Keep rg's default BOM detection enabled — no forced encoding flag exists in the child argv.

---

### 3. Create the unsupported-encoding finish marker

**Type**: FINISH MARKER  
**Output**: Finish marker exists at `Notes/finish-markers/030-04/finish-marker.md`.  
**Depends on**: 2

Write `Task 030-04 finished successfully at <time>` to `Notes/finish-markers/030-04/finish-marker.md`, replacing `<time>` with the current UTC timestamp (for example, `date -u +"%Y-%m-%dT%H:%M:%SZ"`). Create the `Notes/finish-markers/030-04/` directory if it does not already exist.

---
