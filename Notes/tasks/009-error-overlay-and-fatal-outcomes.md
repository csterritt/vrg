# Tasks for #9: Error overlay, fatal search outcomes (exit 2), and the outcome-transition matrix

Parent issue: #9
Parent PRD: PRD-vrg.md
**Blocked by issues**: #8
**Acceptance criteria**: AC3 → Tasks 1–2; AC1–AC2, AC4–AC10 → Tasks 3–4
**Manual verification**: Task 6 owns the issue's manual checks.

## Tasks

### 1. Specify stream-integrity lifecycle validation

**Type**: RED  
**Output**: Failing table-driven tests cover every row of the lifecycle transition matrix, binary-exclusion precedence over orphan retention, `text`/`bytes` path-identity agreement, interleaved open files, and the trailing-unterminated double disposition.  
**Depends on**: none

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Begin only after Issue #8 is complete. Add failing table-driven tests in `internal/searchindex` for the Issue #9 lifecycle matrix and the Result index, records, and stream integrity section of `Notes/PRD-vrg.md`. Cover `begin` while open and not open, `match` while open and orphaned (never opened and after its `end`), `end` while open and orphaned, `context` in any position with no lifecycle effect, a file still open when the stream ends, exactly one `summary` as the final record with a summary alone a complete zero-result stream, missing and second `summary`, any record after `summary`, and a trailing unterminated record counted malformed and marked incomplete. Require path identity compared on decoded raw path bytes so `text` and `bytes` forms of the same path agree, per-path state tracked independently for interleaved open files, orphaned matches retained with incomplete metadata, and binary exclusion to take precedence over the general orphan-retention rule so a match after a binary-excluding `end` is not retained and the file stays excluded. Keep this task test-only.

---

### 2. Implement lifecycle validation and integrity accounting

**Type**: GREEN  
**Output**: Lifecycle tests pass with integrity assessed separately from process success.  
**Depends on**: 1

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Implement only enough lifecycle tracking in `internal/searchindex` to satisfy Task 1: per-path open state over decoded raw path bytes, the transition dispositions of the matrix, orphan retention with incomplete metadata flags, binary-exclusion precedence, the summary positioning rules, and a stream-integrity result kept separate from process success so the App can assess them independently. Record-loss counting stays owned by Issue #10.

---

### 3. Specify the outcome matrix and error overlay

**Type**: RED  
**Output**: A failing single table-driven App outcome matrix covers every outcome-table row with dismissal and exit assertions; failing overlay tests cover key routing, wrapping, and the sink-safety row; failing PTY tests cover the non-zero-exit handshake row and the stderr-content fixture.  
**Depends on**: 2

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Add the failing outcome/transition matrix for Issue #9 as a single table-driven test in `internal/app`, structured so later issues extend it with rows rather than duplicating the decision. Cover rg 0 clean → browse 0; anomalous rg 1 with retained results and a complete stream → browse 0; rg 1 empty → no-results 1; a fatal code or signal death with usable results → browse with error overlay, dismissal → browse, `q` → 2; the same with no usable results → overlay with `q` and separately `Esc` → 2; missing summary and orphaned `end` with valid matches → 2; stderr warning with results → warning overlay, eventual 0; stderr warning with zero results and a complete stream → warning overlay → no-results → 1; all-binary after a warning → no-results with count → 1; and `ctrl+c` after completion in browse, no-results, and open-overlay states → 130 — each row asserting initial presentation, post-dismissal presentation (and which key dismissed), and final exit status. Add failing overlay tests for `up`/`down` scrolling, `q`/`Esc` dismissal, `ctrl+c` → 130, other keys ignored, long unbroken diagnostic wrapping within the border, and the hostile fixture passing the Issue #6 sink-safety check as a new table row; require a failed process supplying no stderr to yield a generated diagnostic naming its exit code or signal. Add failing PTY tests on the Issue #4 harness for a fake rg exiting non-zero after a handshake, and the stderr-content fixture — at least 1 MiB of stderr interleaved with a valid stdout stream — asserting the stdout stream is complete, captured stderr is included in diagnostics, and the overlay contains the head and tail of the stderr text. Keep this task test-only.

---

### 4. Implement the outcome function and error overlay

**Type**: GREEN  
**Output**: Outcome matrix, overlay, and PTY rows pass; the fixed status is decided once and overridden only by `ctrl+c`.  
**Depends on**: 3

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Implement the pure outcome function of process result, integrity, usable-result count, record-loss counts, and warning diagnostics, returning initial presentation, post-dismissal state, and exit status; the modal error overlay with base colours, a single-line border, `up`/`down` scrolling, `q`/`Esc` dismissal, `ctrl+c` → 130, other keys ignored, and interior text wrapped and routed through the Issue #6 utility; stderr classification into diagnostics regardless of exit code with generated code-or-signal diagnostics when stderr is empty; and the fixed-status rule with the `ctrl+c` override. Leave the record-loss inputs unused until Issue #10 extends the matrix.

---

### 5. Document the outcome contract

**Type**: DOCUMENT  
**Output**: Wiki documentation records the outcome table, lifecycle validation, the error overlay, and stderr classification.  
**Depends on**: 4

Read and follow `Notes/wiki/wiki-rules.md` and the schema in `Notes/wiki/AGENTS.md`, then ingest the completed Issue #9 implementation and tests into the appropriate pages under `Notes/wiki`. Document the full lifecycle transition matrix with its dispositions and binary-exclusion precedence, stream integrity assessed separately from process success, the pure outcome function and every outcome-table row, the modal error overlay's keys, wrapping, and sanitization, stderr classification with generated code-or-signal diagnostics, and the fixed-status rule with the `ctrl+c` override. Cross-reference Issue #9 and the Outcome and exit-status contract and Colours, overlays, and key precedence sections of `Notes/PRD-vrg.md`, update `Notes/wiki/index.md`, and append the required dated ingest record to `Notes/wiki/log.md` without rewriting previous entries.

---

### 6. Create the outcome-matrix walkthrough

**Type**: CODE WALKTHROUGH  
**Output**: Showboat walkthrough exists at `Notes/walkthroughs/009-06/code-walkthrough`.  
**Depends on**: 5

Use showboat, consulting `uvx showboat --help`, to create the walkthrough at exactly `Notes/walkthroughs/009-06/code-walkthrough`, with the main file named `walkthrough.md`. Demonstrate the lifecycle matrix tests, the table-driven outcome matrix, the overlay key-routing and wrapping tests, and the PTY stderr-content fixture, then run manual fake-rg cases: two valid matches then exit 3 with stderr "boom" browsing with the overlay and exiting 2 after dismissal; exit 2 with no output naming the code; SIGKILL mid-stream naming the signal; and a "warn" stderr with a summary-only stream showing the warning overlay, the no-results screen, and exit 1. Reference Issue #9 and `Notes/PRD-vrg.md`, and store every generated artifact in the approved directory.

---
