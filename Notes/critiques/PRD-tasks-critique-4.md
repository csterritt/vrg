# PRD tasks critique 4 — VRG

## Scope and verdict

**Reviewed:**

- `Notes/skills/critique-tasks/SKILL.md`.
- `Notes/Ideas.md`.
- `Notes/PRD-vrg.md`, revision 6, in full.
- All 35 files in `Notes/issues/`, in full.
- All 35 corresponding files in `Notes/tasks/`, in full.
- `Notes/critiques/PRD-tasks-critique-1.md`, `-2.md`, `-3.md`, and `Notes/critiques/PRD-issues-critique-6.md` for the disposition of prior findings.

This is a **tasks-only review**. It evaluates whether the issue-derived tasks are complete against their current parent issues and the current PRD, executable in their declared order, faithful to the requirements, and backed by enough deterministic and end-to-end verification to produce and verify the expected program. No PRD, issue, task, or implementation file was changed.

**Verdict: the task set is ready for implementation after one small targeted addition.** The regeneration of Tasks 001 and 002 (the files now carry revision-6 `mow.cli` content and were modified after the prior critiques) resolves every finding from `PRD-issues-critique-6.md`, including the two Critical and two High items that blocked implementation. The remaining 33 task files remain strong, and the non-CLI corrections carried over from the earlier task critiques are still present. No critical or high-severity finding remains.

One Medium finding is worth fixing before code generation: Task 001 never tests the Issue 1 contract that help **assignment** spellings (`--help=false`, `-h=false`) before `--` are not help requests. The issue states this explicitly, the natural preflight implementation (exact-token match) satisfies it, but no RED row pins it, so a prefix/substring-matching preflight could silently produce a help-only result where Issue 1 forbids one. One Low finding records a traceability gap around the architecture-decision checkpoint. Neither requires re-deriving the task graph.

### What improved since the prior critiques

`PRD-issues-critique-6.md` found Tasks 001 and 002 obsolete (still the pre-revision-6 CLI) and demanded regeneration. The current files have been regenerated and now satisfy those findings:

- **F1 (Critical, Task 001 obsolete) — resolved.** Task 001 now pins `github.com/jawher/mow.cli v1.2.0` (Task 2 CONFIG), implements the adapter behind the CLI interface with `flag.ContinueOnError` (Task 4 GREEN), establishes the shared option/argument declaration table and ordered raw-token preflight (Task 4), produces distinct help-only / parsed-search / usage-error result kinds (Tasks 3 and 4), and covers bare, first-token, later-token, combined, flag-preceded, and after-`--` help with subprocess sentinels proving no root validation, child startup, TUI initialization, or stub dispatch on help-only paths (Task 3 RED). The documentation and walkthrough tasks (5 and 6) cover `mow.cli`, the output strategy, and command-line help.
- **F2 (High, Task 002 obsolete) — resolved.** Task 002 now extends Issue 1's shared declarations and preflight rather than replacing them; asserts exact public child argv (not `VarOpt` callback logs) for `-i -s -i`, `-isi`, mixed long/short aliases, and options interleaved with operands; covers cumulative `-u` across mixed tokens including the `-iu`/`-iuu`/`-iuuu` boundary; lexically rejects boolean assignment forms (`--ignore-case=false`, `-i=false`, `--unrestricted=false`, `--help=false`, `-h=false`) and preserves them as positionals after `--`; retains all Issue 1 help-only regressions; and asserts generated help lists every allow-listed flag in short and long form from the shared metadata.
- **F3 (Critical, downstream dependencies unmet) — resolved.** Task 006 reruns "Issue #1 CLI output tests" and the generated CLI-help stdout sink; Task 034 consumes the CLI's shared option declarations and verifies local help, no-argument spellings, both exit-0 reasons, and the flags-only usage error; Task 035 smoke-runs the no-child/no-TUI help branch. Task 001 now creates the named generated-help/CLI-output test groups and shared declarations these downstream tasks reference (Task 3 retains them "so Issue #6's tasks can rerun them unchanged"; Task 7 confirms they exist for Tasks 6, 34, and 35).
- **F4 (High, decision checkpoint after implementation) — resolved.** Task 001 now begins with a REVIEW task (Task 1) that records the module path, six-package layout, Go version, all dependency pins including `mow.cli v1.2.0`, and the complete native-output prevention-or-containment strategy, before the CONFIG/RED/GREEN tasks that depend on it (Task 2 depends on 1; Task 3 on 2; Task 4 on 3). A final review (Task 7) verifies implementation against the recorded decision.
- **F5 (Medium, false AC-mapping claims) — resolved.** Task 001 maps AC1–AC15 to RED 3 / GREEN 4 (with CONFIG 2 establishing the pin for AC1), matching the issue's fifteen acceptance criteria. Task 002 maps AC1–AC11 to RED 1 / GREEN 2, matching the issue's eleven. The mappings are now mechanically auditable against the current issues.

The non-CLI corrections from the earlier task critiques are still present in the current files: Task 006 routes CLI-help stdout through the shared sanitizer and preserves the provisional tab ownership; Task 010 names both composite malformed-plus-integrity records; Task 018 re-clamps on wrap re-entry and every pan; Task 024 carries the buffer-status slot AC; Task 026 uses a disposable, self-restoring permission fixture in both task and issue; Task 027 carries the path-identity-during-reload AC; Task 032 asserts the full error-over-help dismissal sequence; Tasks 034 and 035 include revision-6 command-line help documentation and final smoke coverage; Issue 018 AC6 now states the unconditional re-entry clamp; Issue 024 AC8 and Issue 027 AC cover the filename-row obligations; Issue 026's manual section uses a disposable fixture with a restoration trap.

### Severity definitions

- **Critical:** the declared task graph cannot be executed without guessing ownership or violating a dependency.
- **High:** the plan can complete while a material PRD behavior remains absent or incorrectly implemented.
- **Medium:** an explicit acceptance contract or edge case lacks deterministic coverage, so a regression can pass closure.
- **Low:** local traceability or operational-hygiene weakness unlikely to change the implementation.

## Findings

### F1 — Medium: Task 001 does not test that help assignment spellings are not help requests

**Affected:**

- `Notes/issues/001-go-scaffold-cli-positionals-and-root.md:17,32`
- `Notes/tasks/001-go-scaffold-cli-positionals-and-root.md:41,45`
- `Notes/tasks/002-cli-flag-allow-list-and-child-argv.md:23,37`

Issue 1 states an explicit contract on the preflight: "Assignment spellings such as `-h=false` and `--help=false` are **not help requests**." The verified-library-behaviour bullet adds that `mow.cli` accepts `=`-assignment for boolean-valued options (`--ignore-case=false` and `-i=false` parse, delivering `false`), so the library itself would happily set a local help value to `false`; the preflight must therefore classify the token on its exact spelling, not on a prefix or substring of `--help`/`-h`.

Task 001's RED (Task 3) enumerates the help-only cases it covers — bare invocation, first-token `-h`/`--help`, later help after a pattern/invalid root/excess operands/unsupported option, flag-preceded help (`-i --help`), combined help (`-ih`), and help-like tokens after `--` — and its table tests enumerate positionals, roots, `--`, and unsupported options. Neither list includes `--help=false` or `-h=false` before `--` as a **non-help** case. There is no failing assertion that these spellings do not produce the help-only result kind, do not print help to stdout, and do not exit via the help path.

The gap is observable and cross-issue-sensitive:

- A preflight that matches `--help` as a prefix (or `-h` as a substring inside a token) would classify `--help=false` as help and emit the help-only result, violating AC3/AC4's "exactly one help copy on stdout … without root validation, ripgrep startup, TUI initialization, or success-stub output" by producing help where Issue 1 forbids it.
- Issue 2 later **rejects** these spellings as exit-2 usage errors (AC6, Task 002 RED row for `--help=false`/`-h=false`). If Task 001 instead pinned `--help=false foo` to the success stub (exit 0), Task 002 would have to delete or rewrite that test rather than merely extend the suite. The clean scope for Task 001 is the Issue 1 contract only: "not a help request," not "exit 0" and not "exit 2."

**Recommended correction:** add a RED row to Task 001-3 asserting that `--help=false` and `-h=false` before `--` (with a valid pattern) do **not** produce the help-only result — no help on stdout, result kind ≠ help-only, and the help-only sentinels (no root validation / child / TUI / stub-via-help) are not triggered — **without** pinning the eventual exit code, which Issue 2 changes. Mirror the same "not a help request" assertion in the Task 001 walkthrough (Task 6). Leave the exit-2 rejection to Task 002, which already owns it.

**Ready when:** a preflight that treats `--help=false` or `-h=false` as help fails Task 001's RED, and the assertion survives unchanged through Task 002's change of that route to exit 2.

### F2 — Low: the architecture-decision checkpoint has no acceptance-criterion counterpart

**Affected:**

- `Notes/issues/001-go-scaffold-cli-positionals-and-root.md:36,76-80`
- `Notes/tasks/001-go-scaffold-cli-positionals-and-root.md:6-7,11-17`

Issue 1's Definition of Done makes the human decision — module path, package layout, Go version, all dependency pins including `mow.cli v1.2.0`, and the complete native-output prevention-or-containment strategy — a merge gate, and the chosen output strategy changes the CLI adapter architecture and its tests. None of the fifteen acceptance criteria asserts that this decision was recorded or that the selected strategy covers every library emission point; AC4 covers only the behavioral effect (native emissions never reach stderr). Task 001 owns the decision in Task 1 (REVIEW) and verifies it in Task 7 (REVIEW), and the header's manual-verification note references both, but the AC-mapping line maps only AC1–AC15 to RED 3 / GREEN 4 and never names Task 1 or Task 7.

This is low-risk in practice: Issue 1 is HITL, Task 2 depends on Task 1, so the decision cannot be skipped, and Task 7 re-checks it. The residual risk is closure-by-checklist: an AC-based closure pass would not confirm the output-strategy decision was recorded or that it covered every emission point, relying instead on the free-standing review tasks.

**Recommended correction:** either add a Definition-of-Done acceptance item (or an AC) asserting the recorded decision names `mow.cli v1.2.0` and the selected emission-control strategy with its serialization/drainage/restoration or emission-prevention obligations, and reference Task 1 in the task header's mapping; or note explicitly in the header that the DoD decision gate is owned by Task 1 and verified by Task 7 outside the AC map. No task-content change is required.

**Ready when:** an AC/DoD-level closure check can confirm the architecture decision was recorded before implementation, without relying only on the manual-verification note.

## Coverage and readiness assessment

| Area | Assessment |
|---|---|
| Scaffold, `mow.cli`, help, parser output ownership | **Ready.** Task 001 is regenerated for revision 6: `mow.cli v1.2.0` pin, `ContinueOnError`, shared declarations, ordered preflight, distinct result kinds, native-output strategy, and executable-boundary help tests. Add the help-assignment "not a help request" RED row (F1) and close the decision-gate traceability (F2). |
| Search flags, ordered argv, assignment rejection, generated help | **Ready.** Task 002 extends the shared scan/declarations, asserts public child argv, rejects assignment forms, preserves aliases, and adds generated search-flag help. All eleven ACs are mapped. |
| Downstream CLI safety/docs/final verification | **Ready.** Tasks 006, 034, and 035 reference CLI artifacts (CLI-help stdout, shared declarations, help-only smoke) that Tasks 001/002 now create. |
| Search parsing, lifecycle, outcomes, diagnostics | Strong. Matrix-driven record/integrity coverage, oversized/unterminated and post-summary classifications, and both cancellation routes with replay are intact. |
| Child cancellation, reaping, terminal restoration, stderr replay | Strong model and PTY boundary plan with handshakes, acknowledgements, and exactly-once ordering. |
| Browse layout, wrapping, panning, graphemes, markers, indicators | Strong. Visible-lines extent policy, re-clamp triggers, grapheme-safe clipping, and indicator scope are covered. |
| Async loading, reload, stale content, unsupported encodings | Strong and executable. Pending-intent arbitration, late-result isolation, and disposable retry fixtures are intact. |
| Overlays, pop-ups, too-small recovery | Strong. Observable `Esc` semantics, append-preserving-scroll, error-over-help, scroll recovery, and within-too-small resize are explicit. |
| Documentation and final integration | Strong. README/footer synchronization, sink-safety extension, and clean-checkout build/vet/test plus uncached PTY reruns and five smoke outcomes (including help-only 0) are owned by Issues 34 and 35. |

## Recommended revision order

1. Add the help-assignment "not a help request" RED row to Task 001-3 and the corresponding walkthrough step (F1). Assert non-help-only without pinning the exit code.
2. Close the architecture-decision traceability gap in Task 001's header/DoD (F2).
3. Re-run a focused closure check on Tasks 001, 002, 006, 034, and 035 to confirm the regenerated early-task outputs still match the interfaces the downstream tasks name.

No application tests were run because this is a plan/task review and the greenfield implementation is not present. After the F1 addition (and the optional F2 wording), the task graph is ready for code generation; no broader decomposition, blocker redesign, or PRD-to-issues rewrite is warranted.
