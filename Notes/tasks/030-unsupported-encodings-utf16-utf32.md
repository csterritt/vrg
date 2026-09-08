# Tasks for #30: Unsupported encodings — UTF-16/UTF-32 BOM placeholder

Parent issue: #30
Parent PRD: PRD-vrg.md
**Blocked by issues**: #26, #27, #29
**Acceptance criteria**: AC1–AC6 → Tasks 1–2
**Manual verification**: Task 4 owns the issue's manual checks.

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

### 3. Document unsupported encodings

**Type**: DOCUMENT  
**Output**: Wiki documentation records the BOM detection order, the placeholder, notification rules, and the no-validation exclusion.  
**Depends on**: 2

Read and follow `Notes/wiki/wiki-rules.md` and the schema in `Notes/wiki/AGENTS.md`, then ingest the completed Issue #30 implementation and tests into the appropriate pages under `Notes/wiki`. Document the four BOMs with the longer-before-shorter overlap ordering and the UTF-8 non-misclassification, the placeholder with its diagnostic, retained indexing and reloadability, the current/non-current notification distinction, the exclusion of stale validation on these bytes, and the fixed-status guarantee. Cross-reference Issue #30 and the Encodings and stale-content validation and Invocation sections of `Notes/PRD-vrg.md`, update `Notes/wiki/index.md`, and append the required dated ingest record to `Notes/wiki/log.md` without rewriting previous entries.

---

### 4. Create the unsupported-encoding walkthrough

**Type**: CODE WALKTHROUGH  
**Output**: Showboat walkthrough exists at `Notes/walkthroughs/030-04/code-walkthrough`.  
**Depends on**: 3

Use showboat, consulting `uvx showboat --help`, to create the walkthrough at exactly `Notes/walkthroughs/030-04/code-walkthrough`, with the main file named `walkthrough.md`. Demonstrate the BOM detection tests including the overlap case, then run the manual route: creating a UTF-16 LE file with `printf '\xff\xfeh\0i\0\n\0'`, running `vrg hi .`, entering the file to show "(unsupported encoding)" with the overlay, dismissing it, pressing `r` for "Loading…" then the placeholder with a new overlay, dismissing again, and `q` exiting 0 with each encoding diagnostic on stderr. Reference Issue #30 and `Notes/PRD-vrg.md`, and store every generated artifact in the approved directory.

---
