# Tasks for #36: Stream-integrity fatal outcomes report complete, deterministic diagnostics

Parent issue: #36
Parent PRD: PRD-vrg.md
**Blocked by issues**: none
**Acceptance criteria**: AC4–AC6 → Tasks 1–4; AC1–AC3, AC7–AC8 → Tasks 3–4; AC9 → Tasks 3–4; AC10 → Task 3
**Manual verification**: Task 5 owns the issue's manual checks.

## Tasks

### 1. Specify structured integrity-cause records

**Type**: RED  
**Output**: Failing tests in `internal/searchindex` correct the `context` lifecycle row and assert an ordered list of structured integrity causes (kind plus raw path where applicable) covering every Issue #9 lifecycle-failure row, with end-of-stream causes in the mandated order and post-summary malformed/oversized/unknown dual representations retained.
**Depends on**: none

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Add failing tests to `internal/searchindex` (extend `lifecycle_test.go` or add a focused cause-list test file) requiring the built `Index` — via `Integrity()` or a new accessor — to carry one structured cause record per offending physical record: a stable kind plus the affected raw path where applicable. Cover every Issue #9 lifecycle-failure row with its expected kind: `begin(P)` while P already open, `match(P)` while P not open, `match(P)` after a binary-excluding `end(P)`, `end(P)` while P not open, P still open at stream end, missing `summary`, a second `summary`, a record after `summary`, and a trailing unterminated record. Pin overlap precedence exactly: each physical record contributes at most one integrity cause; a second `summary` contributes only `extra summary`, not `record after summary`; after the first valid `summary`, every other subsequent record contributes only `record after summary` and is not lifecycle-processed, so a post-summary `begin(Q)` cannot open Q or later produce `missing end`; and a trailing unterminated fragment after a valid `summary` contributes `record after summary` plus the malformed-count representation, but not a second `unterminated final record` integrity cause. Outside the post-summary state, a trailing unterminated fragment contributes `unterminated final record` plus the malformed count. Correct the existing `"context after summary has no lifecycle effect"` lifecycle row to expect incomplete integrity in this RED, making Issue #36 the sole owner of that superseded expectation. Require explicit exact-state RED rows for a second `summary`, a post-summary `begin(Q)`, a post-summary `context`, a post-summary unterminated fragment, a post-summary oversized record, and a post-summary unknown-type record. Each row asserts the complete cause list and all independent counters: the `context` and ordinary-record rows contain only `record after summary`; the unterminated row contains only that cause plus malformed count 1; the oversized row contains only that cause plus oversized count 1 (and its recoverable raw-path detail when supplied); and the unknown-type row contains only that cause plus unknown count 1. Reject every additional integrity cause without discarding the independent count. Require mid-stream violations to be recorded in detection order, then end-of-stream causes in this order: missing `end` for each still-open file ordered by unsigned raw-path bytes (never map iteration order), then missing `summary`, then the trailing unterminated record when the post-summary precedence rule does not replace it. Require repeated identical violations (several orphaned `match` records for one path) to produce one cause record each with no aggregation, deduplication, or cap. Keep this task test-only.

---

### 2. Implement integrity-cause recording in SearchIndex

**Type**: GREEN  
**Output**: The structured-cause tests pass; `Integrity().Complete` semantics are unchanged and the cause list is deterministic across runs.  
**Depends on**: 1

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Extend `internal/searchindex` so the `Builder` records a structured cause at each lifecycle-violation site — duplicate `begin` in `parseBegin`, orphaned `match` including the binary-exclusion precedence path in `parseMatch`, orphaned `end` in `parseEnd`, second `summary` in `parseSummary`, and after-`summary` detection in `Add` — carrying the record's raw path where the record names one. Enforce the RED precedence table before dispatching post-summary records to lifecycle parsers: a second `summary` records only `extra summary`; every other post-summary record records only `record after summary` and cannot mutate open-file state; a post-summary trailing unterminated fragment records only that after-summary integrity cause while retaining its malformed count; post-summary oversized and unknown-type records likewise record only the after-summary integrity cause while retaining their oversized or unknown count and any recoverable oversized-path detail. Remove the `context` exemption in `Builder.Add` so the corrected `lifecycle_test.go` row from Task 1 passes; Issue #44 must find this parser behavior and row already green. In `Build`, append the end-of-stream causes in the mandated order: missing `end` for each still-open file sorted by unsigned raw-path bytes, then missing `summary`, then a trailing unterminated cause only when no valid summary put the fragment under after-summary precedence. Carry the ordered cause list on `Integrity` or an equivalent accessor so `internal/app` can consume it. Do not change which conditions set `Complete` to false, do not change malformed/unknown/oversized counting, and do not cap or aggregate causes; repeated distinct physical records still produce distinct causes.

---

### 3. Specify the composed-diagnostic contract

**Type**: RED  
**Output**: Failing `internal/app` outcome tests assert the composed diagnostic text and ordering for every integrity cause, the universal component order in fatal and non-fatal branches, uncapped multiplicity, dual representation, path escaping, determinism, and the absence of a process-status line for 0/1 exits.  
**Depends on**: 2

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Add failing tests in `internal/app` (`outcome_test.go` plus the model-level overlay/collection tests) asserting the *complete text* of `DecideOutcome` results. Require each integrity cause from the Issue #36 matrix to produce its stable user-facing line — duplicate `begin`, orphaned `match`, `match` after a binary-excluding `end`, orphaned/duplicate `end`, missing `end`, missing `summary`, extra `summary`, record after `summary`, and the unterminated final record — naming the `safepresentation.EscapePath`-escaped path where applicable, present in both the overlay text and the collected diagnostics replayed to stderr. Add exact composed-output rows for all overlaps: a second `summary` has the sole integrity line `extra summary record` and no `record after summary` line; a post-summary `begin(Q)` has the sole integrity line `record after summary`, no duplicate/orphan lifecycle line, and no end-of-stream `missing end` for Q; a post-summary unterminated fragment has `record after summary` followed in universal component order by exactly `1 malformed record skipped`, with no `unterminated final record` line; a post-summary oversized record with recoverable path Q has `record after summary` followed by the currently available `oversized record skipped for <escaped Q>` detail, with no oversized integrity cause, while its exact-state SearchIndex row proves oversized count 1 (Issue #37 will prepend the aggregate and update this complete expected slice); and a post-summary unknown-type record has `record after summary` followed in the unknown-warning slot by exactly `1 unrecognised record types skipped`, with no second integrity cause. Assert equality of the complete ordered line slice rather than checking that one acceptable line is present. Require the universal component order in fatal and non-fatal compositions: collected ripgrep stderr in collection order — or, only for signal death or a non-0/1 exit that supplied no explanatory stderr, a generated line naming the exit code or signal, never `ripgrep exited with code 0` or any process-status line for a 0/1 exit — then integrity causes, then the malformed aggregate, oversized aggregate, and per-path oversized details in Issue #37's order, then unknown-type warnings. Cover: a fatal stream with real stderr and integrity causes suppressing neither; repeated identical violations emitted one-for-one without a cap; the dual-representation cases visible in both components under the pinned precedence; two files missing `end` producing unsigned raw-path-ordered lines stable across repeated builds; and an embedded path containing a newline escaped through `EscapePath` so it cannot forge paragraph breaks. Keep this task test-only.

---

### 4. Implement universal diagnostic composition

**Type**: GREEN  
**Output**: The composed-diagnostic tests pass; every fatal and non-fatal branch composes all applicable components in the universal order, and the same lines reach the overlay and the post-restoration stderr replay.  
**Depends on**: 3

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Rework `DecideOutcome` and `recordLossDiagnostics` in `internal/app/app.go`. Extend `OutcomeInput` to carry the structured integrity causes from `Index.Integrity()` (populated in the `SearchCompleteMsg` handler). Build one ordered component list — process component, integrity-cause lines, record-loss components in Issue #37's order, unknown-type warnings — and use it for every branch, so the fatal branches no longer drop record-loss diagnostics or substitute a bare process-status line for the real integrity cause. Emit the generated exit-code/signal diagnostic only when the process failed (signal death or exit code other than 0/1) and supplied no stderr; never emit a process-status line for a 0/1 exit. Split `recordLossDiagnostics` as needed so unknown-type warnings cannot sit between the malformed and oversized components, and structure the oversized component so Issue #37's aggregate slots in before the per-path details. Escape every embedded path through `safepresentation.EscapePath` at composition time. Because the overlay text is collected via `collectDiagnostic` and replayed verbatim by `cmd/vrg`, composing once satisfies both sinks — verify no second composition diverges. Run the focused tests plus `go build ./...`, `go vet ./...`, and `go test ./...`.

---

### 5. Create the integrity-diagnostics walkthrough

**Type**: CODE WALKTHROUGH  
**Output**: Showboat walkthrough exists at `Notes/walkthroughs/036-06/code-walkthrough`.  
**Depends on**: 4

Use showboat, consulting `uvx showboat --help`, to create the walkthrough at exactly `Notes/walkthroughs/036-06/code-walkthrough`, with the main file named `walkthrough.md`. Demonstrate the structured-cause and composed-diagnostic test suites, then run the issue's manual scenarios with the Issue #4 fake-rg harness: valid `begin`/`match`/`end` records terminating without `summary` and exiting 0 → the fatal overlay names the missing `summary` rather than `ripgrep exited with code 0`, dismissal exits 2, and the same diagnostic appears in stderr replay; repeat with a missing `end` for the only file, and with a damaged stream plus real child stderr showing all causes together. Capture every command, output, and exit status. Reference Issue #36 and `Notes/PRD-vrg.md`, and store every generated artifact in the approved directory.

---
