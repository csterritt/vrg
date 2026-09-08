# Tasks for #10: Record robustness — malformed, oversized, unknown types, missing `end`, warning-before-no-results

Parent issue: #10
Parent PRD: PRD-vrg.md
**Blocked by issues**: #9
**Acceptance criteria**: AC1, AC6, AC8 → Tasks 1–2; AC2–AC5, AC7 → Tasks 3–4
**Manual verification**: Task 6 owns the issue's manual checks.

## Tasks

### 1. Specify malformed-record dispositions

**Type**: RED  
**Output**: Failing table-driven tests cover one fixture per row of the Issue #3 schema matrix and the Issue #9 lifecycle matrix, each asserting its stated disposition, dedicated fixtures for both composite malformed-and-integrity rows asserted in both counters, plus resynchronization after a skip.  
**Depends on**: none

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Begin only after Issue #9 is complete. Add failing table-driven tests in `internal/searchindex` asserting the deterministic disposition — skipped-and-counted malformed, stream-integrity failure, or both — for every row of the Issue #3 per-record schema matrix (each missing or wrongly typed required field, and each invalid range including `line_number` zero, negative, or non-integer, `start > end`, negative `start`, `end` beyond the decoded line bytes, negative `binary_offset`, and an empty `submatches` array) and every integrity row of the Issue #9 lifecycle matrix (duplicate `begin`, orphaned `match` and `end`, `match` after `end`, second `summary`, and a record after `summary`), plus invalid JSON, invalid base64, and missing or non-string `type`. Require a malformed record between valid ones to be skipped, counted, and followed by correct indexing of the remaining records, with lifecycle violations never inflating the malformed count and vice versa except where the matrices mark both. Give both composite rows dedicated fixtures distinct from the oversized cases of Task 3: an ordinary, non-oversized trailing record without a terminating newline, asserted both skipped-and-counted malformed and marked stream-incomplete; and a malformed record appearing after a valid `summary`, asserted both counted malformed and an after-`summary` integrity failure — with both counters and the incomplete-stream status asserted in each. Keep this task test-only.

---

### 2. Implement malformed-record skipping

**Type**: GREEN  
**Output**: Disposition tests pass with malformed counts separate from integrity failures.  
**Depends on**: 1

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Implement per-record validation and skip/count accounting in `internal/searchindex` to satisfy Task 1, keeping malformed counts separate from integrity failures except in the two cases the matrices mark both, routing validation so after-`summary` positioning does not suppress malformed accounting and a trailing unterminated record's malformed accounting does not disappear into a generic decode failure, and keeping a safely skipped match record from by itself making otherwise intact lifecycle metadata incomplete.

---

### 3. Specify oversized records, unknown types, and record-loss outcomes

**Type**: RED  
**Output**: Failing tests cover the 64 MiB boundary, discard-through-newline resynchronization, recoverable and unrecoverable oversized diagnostics, the oversized-only absent file, the unterminated oversized final record, unknown-type counting rules, and the new outcome-matrix rows.  
**Depends on**: 2

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Add failing tests for the remaining Issue #10 contracts and the 64 MiB bullets of the Resources and responsiveness section of `Notes/PRD-vrg.md`. Require a record exactly at the 64 MiB payload limit (excluding its newline) to be accepted and one byte over to be skipped, with the oversized record consumed and discarded through its next newline and parsing resynchronizing on the following record; an oversized `match` whose `type` and `data.path` were parsed before the limit to report "oversized record skipped for <sanitized path>" in addition to the count, and one whose limit was hit before the path to report the count only; a file whose only records were oversized to be absent from the file list while its path appears in the diagnostic; an oversized final record without a trailing newline to be counted oversized — not additionally malformed — and to make the stream incomplete; and unknown string event types to be counted separately, reported as "N unrecognised record types skipped", never independently changing exit status, never substituting for required completion events, and an unknown type after `summary` to be an integrity failure skipped by the after-`summary` rule rather than counted as unknown. Add the new rows to the Issue #9 outcome matrix: unknown-type-only warnings with zero results → warning overlay → no-results → 1; malformed skipped with usable results → browse with overlay → 0; malformed skipped with zero usable results → record-loss overlay → `q` 2 and `Esc` 2; a skipped record plus binary exclusion leaving zero retained stops → the record-loss fatal row 2; missing `end` with retained matches → browse with overlay → 2; and missing `end` with no matches → overlay → 2. Keep this task test-only.

---

### 4. Implement oversized handling and record-loss outcomes

**Type**: GREEN  
**Output**: Oversized, unknown-type, and outcome rows pass; no usable results is assessed after all filtering.  
**Depends on**: 3

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Implement the 64 MiB payload limit with an explicit bounded reader rather than a small default line-reader limit, discard-through-newline consumption, best-effort path recovery for the oversized-record diagnostic through the Issue #6 utility, separate unknown-type counting with the after-`summary` rule, and the record-loss inputs to the Issue #9 outcome function plus its new matrix rows. Assess "no usable results" after all filtering so a stream whose sole retained file was binary-excluded after a skipped record follows the record-loss fatal row rather than the no-results row.

---

### 5. Document record robustness

**Type**: DOCUMENT  
**Output**: Wiki documentation records the malformed, oversized, and unknown-type rules with their dispositions and the record-loss outcome rows.  
**Depends on**: 4

Read and follow `Notes/wiki/wiki-rules.md` and the schema in `Notes/wiki/AGENTS.md`, then ingest the completed Issue #10 implementation and tests into the appropriate pages under `Notes/wiki`. Document the per-record and lifecycle disposition matrices as deterministic categories, the 64 MiB limit with its discard-and-resynchronize behavior and path-recovery diagnostics, the unterminated oversized final record rule, unknown-type counting and its after-`summary` interaction, missing-`end` retention with incomplete metadata, and the record-loss outcome rows including the after-filtering usable-results assessment. Cross-reference Issue #10 and the Result index, records, and stream integrity and Resources and responsiveness sections of `Notes/PRD-vrg.md`, update `Notes/wiki/index.md`, and append the required dated ingest record to `Notes/wiki/log.md` without rewriting previous entries.

---

### 6. Create the record-robustness walkthrough

**Type**: CODE WALKTHROUGH  
**Output**: Showboat walkthrough exists at `Notes/walkthroughs/010-06/code-walkthrough`.  
**Depends on**: 5

Use showboat, consulting `uvx showboat --help`, to create the walkthrough at exactly `Notes/walkthroughs/010-06/code-walkthrough`, with the main file named `walkthrough.md`. Demonstrate the disposition matrix tests, the 64 MiB boundary and resynchronization tests, and the extended outcome matrix, then run the manual fake-rg stream from the issue: a valid `begin`, `match`, a garbage line, an unknown-type line, a valid `end`, and a `summary` with exit 0, showing the browse view, the overlay listing both skip counts, and `q` exiting 0. Reference Issue #10 and `Notes/PRD-vrg.md`, and store every generated artifact in the approved directory.

---
