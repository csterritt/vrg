# PRD issues/tasks critique 6 — VRG

## Scope and verdict

Reviewed:

- `Notes/skills/critique-tasks/SKILL.md`.
- `Notes/Ideas.md`.
- `Notes/PRD-vrg.md`, revision 6, in full.
- All 35 files in `Notes/issues/`, in full.
- All 35 corresponding files in `Notes/tasks/`, in full.
- `Notes/critiques/PRD-issues-critique-5.md` and `Notes/critiques/PRD-tasks-critique-3.md` for the disposition of prior findings.

This is a **tasks-focused review**. It evaluates whether the issue-derived tasks are complete, executable in their declared order, faithful to the current PRD and current parent issues, and sufficient to produce and verify the expected program. No PRD, issue, task, or implementation file was changed.

**Verdict: the task set is not ready for implementation.** The current Issues 1 and 2 have been substantially corrected for revision 6, but their task files still describe the pre-revision-6 CLI. The gap is foundational: Task 001 never adds or uses `mow.cli`, never implements command-line help, and never establishes the raw-token preflight/shared declarations that Task 002's parent issue requires. Task 002 likewise omits the revised help, assignment-rejection, ordered-scan, and generated-help work. Later tasks now assume those missing artifacts and tests already exist.

The remaining 33 task files are strong. The corrections from the prior issue and task critiques are represented in Tasks 006, 007, 010, 018, 024, 026, 027, 032, 034, and 035. No further feature-area omission or blocker-cycle problem was found. The necessary repair is concentrated in Tasks 001 and 002, but those files need regeneration against their current issues rather than small wording edits.

### Severity definitions

- **Critical:** the declared task sequence cannot be executed without inventing missing prerequisite work or violating a parent issue.
- **High:** the tasks can complete while a material PRD behavior remains absent or incorrectly implemented.
- **Medium:** acceptance, review, documentation, or verification can falsely report completion despite a substantive gap.
- **Low:** local traceability or operational wording defect unlikely to affect the implementation.

## Findings

### F1 — Critical: Task 001 is still the pre-revision-6 plan and does not implement its current parent issue

**Affected:**

- `Notes/issues/001-go-scaffold-cli-positionals-and-root.md:12-36,58-80`
- `Notes/tasks/001-go-scaffold-cli-positionals-and-root.md:1-71`
- `Notes/PRD-vrg.md:127-137,265-269,305`

Issue 1 now owns the revision-6 CLI foundation: pinned `github.com/jawher/mow.cli v1.2.0`, generated command-line help, bare-invocation help, local help anywhere before `--`, an ordered raw-token preflight, shared option declarations, suppression or containment of native library output, explicit help/search/error result kinds, and executable-boundary tests proving no root validation, child startup, or TUI initialization on help-only paths.

Task 001 owns none of that work:

- Task 001-1 pins only Bubble Tea, Bubbles, and Lip Gloss and does not add `mow.cli`.
- Task 001-2 tests only positionals, root validation, and usage failures.
- Task 001-3 explicitly says to implement only positional/root behavior and not add flag parsing.
- No RED task covers bare help, first/later/combined help, help precedence, native stderr suppression, exact stdout/stderr behavior, generated help content, or the explicit result kind.
- No GREEN task configures `flag.ContinueOnError`, creates the shared declaration table and raw-token preflight, integrates `mow.cli`, or implements either approved output strategy.
- The documentation and walkthrough tasks describe the old `vrg pattern [root]` stub and omit command-line help entirely.

This is not merely incomplete acceptance wording. Executing Task 001 exactly as written produces a CLI that violates the first six acceptance criteria of its current issue and leaves no implementation seam on which the revised Issue 2 can safely build.

**Required correction:** regenerate Task 001 from the current Issue 1. Preserve red-green ordering, but add explicit work for:

1. the `mow.cli v1.2.0` dependency and adapter boundary;
2. the shared option-declaration metadata and extensible raw-token preflight;
3. no-argument and explicit/local help precedence before validation or search work;
4. `ContinueOnError` and entry-point-owned statuses;
5. the selected complete native-output prevention/containment strategy;
6. distinct help-only, parsed-search, and usage-error results;
7. hostile-substitution and subprocess tests for exactly one stdout help copy, empty stderr, no child, and no TUI;
8. updated wiki and walkthrough coverage.

**Ready when:** every Issue 1 acceptance criterion has a named RED/GREEN owner, Task 001 builds a runnable `mow.cli`-using binary, and its executable-boundary tests cover bare, first-token, later-token, combined, missing-pattern, excess-operand, unsupported-option, invalid-root, and after-`--` help cases.

### F2 — High: Task 002 omits most of its revised issue's parser and help contracts

**Affected:**

- `Notes/issues/002-cli-flag-allow-list-and-child-argv.md:14-20,24-38,40-52`
- `Notes/tasks/002-cli-flag-allow-list-and-child-argv.md:11-51`

Issue 2 was correctly revised after the upstream `mow.cli` probes. It now requires the Issue 1 preflight and declarations to be extended so that the implementation:

- preserves cross-option encounter order independently of nondeterministic `VarOpt` callback order;
- preserves supplied short/long spellings;
- recognizes local help before allow-list validation and never forwards it;
- rejects boolean assignment forms such as `--ignore-case=false`, `-i=false`, `--unrestricted=false`, and help assignments;
- keeps those strings positional after `--`;
- adds every search flag to generated command-line help from the shared declarations;
- retains all Issue 1 help-only regressions.

Task 002 still contains the older generic instructions to test and implement an allow-list, combined shorts, cumulative `-u`, `--`, and child argv. It does not instruct the implementer to use or extend the shared declaration table/raw-token scan, avoid callback order, reject assignment forms, preserve supplied aliases in repeated mixed options, add generated help entries, or keep help-only sentinels green. Its documentation and walkthrough tasks omit these behaviors too.

The phrase “forwarded in user order” is not enough. The previous upstream verification established that the obvious `VarOpt` implementation cannot satisfy it; the task must prescribe the tested adapter seam that the issue now requires. Likewise, the parent issue has eleven acceptance criteria, but Task 002's RED/GREEN content does not cover AC2, AC5-AC6, or AC11 completely.

**Required correction:** regenerate Task 002 against the current issue, explicitly extending Task 001's shared scan/declarations. RED tests must assert public child argv rather than callback logs for `-i -s -i`, `-isi`, mixed long/short aliases, and options interleaved with both operands; cover all help regressions and all rejected assignment forms; and assert generated help from the shared metadata. Carry those requirements into GREEN, documentation, and walkthrough tasks.

**Ready when:** Task 002 can be followed without guessing how order survives `mow.cli`, and all eleven parent acceptance criteria have explicit test and implementation ownership.

### F3 — Critical: downstream tasks depend on Issue 1 artifacts and tests that Tasks 001-002 never create

**Affected:**

- `Notes/tasks/006-safe-presentation-utility-for-all-sinks.md:14,19,26,31,51`
- `Notes/tasks/034-documentation-scale-and-memory-limits.md:14,19,31`
- `Notes/tasks/035-final-integration-verification.md:14,19,29,39`
- Tasks 001 and 002 as above.

The later task updates correctly assume revision-6 prerequisites:

- Task 006 reruns “Issue #1 CLI output tests,” including generated CLI-help stdout, and routes that sink through the shared sanitizer.
- Task 034 consumes the CLI's shared option declarations and verifies local help, no-argument spellings, both reasons for exit 0, and flags-only usage errors.
- Task 035 expects the composed suite to include focused help tests and smoke-runs the final no-child/no-TUI help branch.

Those are sound downstream checks, but the current Tasks 001 and 002 never create the referenced help output tests, generated-help path, or shared declarations. An implementer reaching Task 006 must either invent substantial unplanned CLI work inside a sanitization issue or skip a stated prerequisite. Task 034 cannot consume a nonexistent source of truth. Task 035's final smoke test may expose the defect, but a final integration gate is not a substitute for missing foundational RED/GREEN work.

**Required correction:** repair Tasks 001 and 002 before implementation begins; do not move their omitted behavior into Tasks 006, 034, or 035. After regeneration, check that the names/interfaces used by the downstream tasks match the actual outputs of the early tasks.

**Ready when:** every downstream reference to Issue 1 CLI tests, CLI-help stdout, and shared declarations resolves to a concrete earlier task output.

### F4 — High: Task 001's human-decision checkpoint is incomplete and occurs after the architecture-dependent implementation

**Affected:**

- `Notes/issues/001-go-scaffold-cli-positionals-and-root.md:29,36,76-80`
- `Notes/tasks/001-go-scaffold-cli-positionals-and-root.md:65-71`

Issue 1 requires a human decision covering the module path, package layout, Go version, all pinned dependencies including `mow.cli`, and—most importantly—the complete library-output strategy. The choice between file-descriptor containment and metadata rendering plus proven emission prevention changes the CLI adapter architecture and its tests.

Task 001-6 is the old review task. It mentions only the module path, package layout, Go version, and Bubble Tea/Bubbles/Lip Gloss pins. It omits `mow.cli` and the output strategy entirely. It also runs after implementation, documentation, and walkthrough work, even though the omitted strategy must guide those earlier tasks.

A late review can approve completed work, but it cannot serve as the required design choice on which that work depends. Implementers would have to select a strategy without the required human decision and potentially redo the entire CLI/output boundary afterward.

**Required correction:** place a HITL/REVIEW decision gate before the architecture-dependent RED/GREEN tasks, recording all choices required by Issue 1. A final review may remain, but it should verify implementation against the recorded decision rather than make the decision for the first time.

**Ready when:** no implementation task requires choosing an unapproved output architecture, and the recorded decision includes `mow.cli v1.2.0` plus the exact emission-control strategy and its concurrency/restoration obligations where applicable.

### F5 — Medium: acceptance mappings in Tasks 001 and 002 claim coverage that their task bodies do not provide

**Affected:**

- `Notes/tasks/001-go-scaffold-cli-positionals-and-root.md:6`
- `Notes/issues/001-go-scaffold-cli-positionals-and-root.md:58-74`
- `Notes/tasks/002-cli-flag-allow-list-and-child-argv.md:6`
- `Notes/issues/002-cli-flag-allow-list-and-child-argv.md:40-52`

Task 001 says “AC1–AC7 → Tasks 2–3,” but the current issue has fifteen acceptance criteria, and even AC1-AC7 are not covered: the tasks do not use `mow.cli`, configure `ContinueOnError`, implement help, control native emissions, or generate help content.

Task 002 says “AC1–AC11 → Tasks 1–2,” but Tasks 1-2 omit the explicit mechanisms and fixtures for ordered repeated/mixed aliases, local-help precedence, assignment rejection, and generated search-flag help. The metadata therefore gives a false impression of complete closure coverage.

**Required correction:** after regenerating both task files, map each current AC to specific RED and GREEN task numbers. Do not use a blanket range unless every criterion is concretely represented in the task text and outputs.

**Ready when:** the mappings are mechanically auditable and no mapped AC depends only on a parent-issue sentence or a late final smoke run.

## Coverage and readiness assessment

| Area | Assessment |
|---|---|
| Scaffold, `mow.cli`, help, parser output ownership | **Not ready.** Task 001 is obsolete (F1, F4-F5). |
| Search flags, ordered argv, assignment rejection, generated help | **Not ready.** Task 002 is obsolete (F2, F5). |
| Downstream CLI safety/docs/final verification | Correctly revised, but blocked on missing early artifacts (F3). |
| Search parsing, lifecycle, outcomes, and diagnostics | Strong. The oversized/unterminated and post-summary unknown classifications now agree with the PRD. |
| Child cancellation, reaping, terminal restoration, stderr replay | Strong model and PTY boundary plan with acknowledgements and exactly-once ordering. |
| Browse layout, wrapping, panning, graphemes, markers, indicators | Strong. Prior extent/indicator and acceptance mismatches are corrected. |
| Async loading, reload, stale content, unsupported encodings | Strong and executable, including pending-intent arbitration and late-result isolation. |
| Overlays, pop-ups, and too-small recovery | Strong. The error-over-help key sequence and modal scroll recovery are explicit. |
| Documentation and final integration | Strong in isolation; cannot compensate for Tasks 001-002. |

## Prior-finding disposition

The non-CLI corrections from the previous critiques are now reflected in both issues and tasks:

- Task 006 includes command-line help as a distinct sanitized sink and preserves the provisional tab ownership.
- Task 007 uses a real search invocation for theme verification.
- Task 010 matches the PRD's combined counts for oversized unterminated records and unknown records after `summary`.
- Task 018 allows `_` on every visible line and qualifies the paintable-cluster rule.
- Tasks 024 and 027 align their acceptance mappings with the filename-row status/path obligations.
- Task 026 uses a disposable, self-restoring permission fixture.
- Task 032 asserts the full error-over-help dismissal sequence.
- Tasks 034 and 035 include revision-6 command-line help documentation and final smoke coverage.

The remaining failure is propagation in the opposite direction: the updated downstream tasks presume that the revised Issue 1 and Issue 2 contracts were converted to tasks, but that conversion did not occur.

## Required revision order

1. Resolve and record Issue 1's architecture choices before implementation work.
2. Regenerate Task 001 from the current Issue 1, including RED/GREEN, documentation, walkthrough, and accurate AC mappings.
3. Regenerate Task 002 from the current Issue 2, extending Task 001's shared declarations/preflight and retaining all help regressions.
4. Recheck the interfaces named by Tasks 006, 034, and 035 against the regenerated early-task outputs.
5. Rerun this task critique or at minimum perform a focused closure check on Issues/Tasks 001, 002, 006, 034, and 035 before code generation.

No application tests were run because this is a plan/task review and the greenfield implementation is not present. The task set will be ready once the foundational CLI tasks are synchronized with their current issues; the remaining task graph already provides credible implementation and end-to-end verification coverage.
