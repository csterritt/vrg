# Post-final-audit tasks critique 1 — VRG

## Scope and verdict

**Reviewed:**

- `Notes/skills/critique-tasks/SKILL.md`.
- `Notes/PRD-vrg.md`, revision 6, in full.
- `Notes/critiques/final-audit-vrg.md` as the source audit for Issues 36–50.
- Issues 36–50 in `Notes/issues/`, in full.
- Corresponding Tasks 36–50 in `Notes/tasks/`, in full.
- Earlier task critiques for review conventions and previously established project constraints.

This is a **tasks-only review**. It evaluates whether Tasks 36–50 are complete against their parent issues and the PRD, executable in their declared order, faithful to the audit corrections, and backed by deterministic verification sufficient to produce and verify the expected program. It does not review the implementation or add product scope. No PRD, issue, task, or implementation file was changed.

**Verdict: make a targeted task revision before implementation.** Tasks 36–44, 46, and 50 generally translate the audit findings into strong RED/GREEN work with explicit acceptance ownership and end-to-end closure. Task 49, however, has one critical branch-specific dependency defect: if the human chooses adoption, its declared tasks cannot keep Bubbles/Lip Gloss, obtain a clean `go mod tidy -diff`, and wait for the required adoption issues in a coherent order. Four narrower findings concern shared-renderer ordering, missing RED coverage for Issue 45's tagged runner, under-specified and under-tested acknowledgement correlation in Issue 48, and unsafe/nondeterministic read-failure fixtures in Task 47.

No PRD behavior is wholly absent from Tasks 36–50. The required revision is local: repair Task 49's adoption branch and tighten the affected test/ordering clauses. A broader regeneration is not warranted.

### What is strong

- Tasks 36 and 37 establish one diagnostic composition order, preserve dual integrity/record-loss representation, prohibit false process-status explanations for exits 0/1, and cover anonymous oversized records.
- Task 38 distinguishes terminal, panel, and text widths and tests the composed terminal-width boundary rather than fixing only one assignment.
- Task 39 carries the grapheme/cell model through the final renderer and names all important width consumers, including overlays and pop-ups.
- Task 41 correctly replaces destructive head/tail compression with complete scrollable rows while retaining bounded model-level verification for very large diagnostics.
- Task 42 pins both sides of the admission boundary: dropped reloads preserve intent, while navigation re-entry remains deliberately ungated.
- Tasks 43 and 49 correctly identify their product decisions as HITL checkpoints rather than silently choosing product behavior during implementation.
- Tasks 45, 46, and 48 select a coherent build-tagged test topology and keep production/test executable boundaries distinct.
- Task 50 is a credible closing gate: both build variants, uncached and race tests, repeated PTY runs, tidy, module verification, pinned vulnerability scanning, untagged production smoke, and recorded evidence are all present.

### Severity definitions

- **Critical:** the declared task graph cannot be executed for an allowed branch without guessing ownership, violating a dependency, or claiming an impossible output.
- **High:** the plan can complete while a material PRD or issue behavior remains absent or incorrectly implemented.
- **Medium:** an explicit acceptance boundary lacks deterministic coverage or two tasks have ambiguous shared ownership/order, creating a meaningful regression risk.
- **Low:** local operational-safety, traceability, or test-fixture weakness unlikely to change the architecture.

## Findings

### F1 — Critical: Task 49's adoption branch has no executable path to a tidy-clean close

**Affected:**

- `Notes/issues/049-tidy-dependency-manifests.md:14-24,26-33`
- `Notes/tasks/049-tidy-dependency-manifests.md:6,11-47`
- `Notes/issues/050-post-audit-reverification.md:3-5`

Issue 49 permits either removal or adoption. Under adoption, it requires concrete ownership decisions, separately scoped implementation issues, those issues to land, and only then a clean final `go mod tidy -diff`; Issue 49 must remain open until that sequence finishes.

The task graph cannot execute that branch as written:

1. Task 49-2 depends only on the decision and has an output claiming correct manifests and clean tidy/build/vet/test gates. At that point the adoption implementation issues do not yet exist or have not landed, so Bubbles and Lip Gloss are still unimported and `go mod tidy` will remove them. Retaining them contradicts the tidy-clean output; allowing tidy to remove them contradicts the adoption decision.
2. Task 49-3, which creates or confirms the scoped adoption issues, runs only **after** Task 2. The prerequisite work is therefore created after the task that already needs its result.
3. No later Task 49 step depends on the external adoption issues. Task 4 depends only on Task 3 and claims the final clean tidy evidence, so the local checklist can reach its walkthrough while the adoption implementation is still pending.
4. Issue 50 is blocked by Issue 49. It cannot supply the missing adoption work or final recheck without either violating its blocker or silently taking ownership of implementation that Issue 49 assigned elsewhere.

The prose says Issue 49 remains open, but an open issue is not an executable task dependency. An implementer must guess whether to pause Task 2, partially complete it, skip to Task 3, or close local tasks while waiting for newly created work.

**Recommended correction:** make Task 49 explicitly branch-shaped.

- Keep Task 1 as the decision gate.
- For **removal**, retain the current tidy → documentation → walkthrough sequence.
- For **adoption**, create and record the adoption issues immediately after Task 1, before any task claims a final manifest state. Give those issues explicit dependencies on Issues 38–41 and any other touched fixes.
- Add a Task 49 finalization step blocked by all adoption issues. That step verifies real imports/ownership, rejects token imports, runs `go mod tidy`, and requires clean tidy/build/vet/test results.
- Make documentation finalization and the walkthrough depend on that finalization step. Update the header's AC mapping for each branch.

**Ready when:** either allowed decision produces a finite dependency path ending with real code ownership, a clean final module graph, aligned documentation, and an Issue 49 walkthrough; no task claims tidy-clean adoption before the adopted libraries are used.

### F2 — Medium: Tasks 39 and 40 have undeclared shared ownership of the browse renderer and path geometry

**Affected:**

- `Notes/issues/039-render-from-shared-grapheme-cell-model.md:3-17`
- `Notes/tasks/039-render-from-shared-grapheme-cell-model.md:5,19-31`
- `Notes/issues/040-browse-render-no-whole-index-scan.md:3-16`
- `Notes/tasks/040-browse-render-no-whole-index-scan.md:5,19-31`

Issue 39 is blocked only by Issue 38, while Issue 40 says it can start immediately. Both are AFK work and both rewrite `renderBrowse`-adjacent behavior in `internal/app/app.go`:

- Task 39 routes file-list padding, filename fitting, truncation, and final geometry through the new shared grapheme/cell helper.
- Task 40 restructures browse rendering, precomputes escaped path text/cluster boundaries/full cell width, and rewrites visible-row truncation and current-file lookup.

The final design is compatible, but the ownership/order is not declared. Parallel implementations will collide in the same renderer and metadata structures. Landing Task 40 after Task 39 can accidentally bypass the new shared helper; landing Task 39 after Task 40 can replace the bounded visible-range path with code derived from the old render structure. Unlike Tasks 41 and 48, these files contain no shared-file ordering rule.

**Recommended correction:** choose one explicit order. The cleaner sequence is Issue 38 → Issue 39 → Issue 40: Task 40 can consume the shared grapheme/cell helper and preserve it while moving metadata out of `View()`. Add Issue 39 to Issue 40's issue/task blockers and state that Task 40's RED cost guard runs against the Issue 39 renderer. Alternatively, order Issue 40 before Issue 39 and make Task 39 explicitly preserve the precomputed groups and O(visible rows) guard. Do not leave both marked independently startable.

**Ready when:** the renderer, path metadata, grapheme-safe truncation, and bounded-render cost guard have one declared handoff, and the second issue reruns the first issue's focused regressions.

### F3 — Medium: Issue 45 can close without a RED test proving its tagged runner supports the required return shapes

**Affected:**

- `Notes/issues/045-remove-test-hooks-from-production-binary.md:14-23,27-38`
- `Notes/tasks/045-remove-test-hooks-from-production-binary.md:6,11-31`
- `Notes/issues/046-runtime-error-common-diagnostic-replay.md:3-16`
- `Notes/tasks/046-runtime-error-common-diagnostic-replay.md:11-19`

Issue 45's acceptance criteria require the `vrg_testhooks` runner boundary to produce every return shape needed by Issue 46 at the real `program.Run()` call site. Its verification section also calls for runner-shape subprocess tests.

Task 45-1's only RED work is the clean **untagged** production-artifact boundary. Task 45-2 then implements the tagged runner and claims all return shapes in its output, but no preceding failing test pins that contract. The first full matrix appears in Task 46-1, after Issue 45 is supposed to be complete and after Issue 46 has committed to consuming the seam. A malformed seam can therefore pass Task 45's automated closure and only be discovered by its dependent issue.

This also weakens the intended red-green process: the most important positive behavior of the tagged variant is introduced in GREEN without a RED specification, while only the negative production-artifact behavior is test-driven.

**Recommended correction:** extend Task 45-1 with focused tagged-runner boundary tests proving each selectable tuple can be returned from the executable's actual runner call site: valid model, nil/invalid model, nil error, and non-nil error as required by the two controls. These tests need not assert Issue 46's shutdown order or exit status; that remains Task 46's responsibility. They only establish the seam's reachability and tuple fidelity. Keep Task 46's PTY matrix as the behavioral integration test.

**Ready when:** Issue 45 cannot close unless both halves of its topology are proven: the untagged artifact is clean and the tagged runner actually supplies every downstream-required return shape.

### F4 — Medium: Task 48's RED step proves only a representative acknowledgement, not complete or correctly correlated handshakes

**Affected:**

- `Notes/issues/048-pty-tests-deterministic-handshakes.md:12-23,25-31`
- `Notes/tasks/048-pty-tests-deterministic-handshakes.md:11-31`
- `Notes/issues/045-remove-test-hooks-from-production-binary.md:19-21,32-38`
- `Notes/tasks/045-remove-test-hooks-from-production-binary.md:19,31`

Issue 48 requires an application acknowledgement for **each** state transition or key-processing boundary that a critical helper waits on. Task 48-1's output repeats that requirement, but its detailed RED instructions require only one representative named acknowledgement plus a static check that fixed sleeps disappeared.

That is insufficient to prove the actual contract. A suite can satisfy the proposed RED checks while:

- acknowledging state entry but not overlay dismissal before the next `q`;
- reusing a stale acknowledgement from an earlier key or state transition;
- treating file existence or an accumulated line count as proof of the wrong transition;
- leaving one helper synchronized on output that is not causally tied to the sent key; or
- adding new acknowledgement environment names without extending Issue 45's untagged artifact probe.

The GREEN prose says to fix all helpers, but there is no auditable transition inventory or correlation rule against which closure can be checked. Removing `time.Sleep` proves only that elapsed-time synchronization disappeared, not that each replacement observes the correct postcondition.

**Recommended correction:** make Task 48-1 build a finite handshake matrix. For every key send/assumed transition in the named helpers, record: helper/test group, triggering action, exact application-side postcondition, acknowledgement hook/event, and the subsequent action it unlocks. Require acknowledgements to be correlated per process and per occurrence (for example, monotonic sequence/event records), so a stale signal cannot satisfy a later wait. Add failing coverage for at least repeated same-kind events and overlay-dismissal-then-quit, not only one representative event. Task 48-2 should update the explicit hook manifest and the untagged artifact probe from Task 45, then rerun it to prove every newly added name is absent from production.

**Ready when:** every removed settling delay has a named causal replacement, repeated events cannot consume stale acknowledgements, and adding a handshake hook without extending the clean-production boundary test fails review or automation.

### F5 — Low: Task 47's real read-failure fixtures are neither deterministic nor explicitly disposable

**Affected:**

- `Notes/issues/047-read-failure-single-line-filenames.md:20-23`
- `Notes/tasks/047-read-failure-single-line-filenames.md:19,31,45-51`

Task 47 asks tests and the walkthrough to arrange genuine read failures, suggesting permission removal after indexing. It does not require a temporary fixture, restoration of original permissions, or a cleanup trap. Permission-based failure is also environment-dependent: it can succeed under elevated privileges or unusual filesystem/ACL settings, so a supposedly failing RED case may not fail for the intended reason.

This project already corrected the same operational weakness for the earlier unreadable-file task. Reintroducing it here risks leaving unusual newline/invalid-byte filenames or unreadable files behind and makes the test less reproducible than necessary.

**Recommended correction:** require a disposable temporary directory and deterministic synchronization: index the file, wait on an explicit load gate, then remove/rename the fixture before allowing the real `os.ReadFile` attempt, asserting the expected path error. Where a permission case is retained for manual demonstration, require a restoration trap and skip/fail clearly when the environment cannot enforce the denial. Apply the same cleanup rule to newline, tab, invalid-UTF-8, and ESC filename fixtures.

**Ready when:** the automated fixture fails for a controlled filesystem reason on supported test environments and neither successful nor interrupted walkthrough execution leaves altered permissions or hostile filenames outside its temporary directory.

## Coverage and readiness assessment

| Area | Assessment |
|---|---|
| Integrity and diagnostic composition (36–37, 44) | **Strong.** Cause multiplicity, deterministic order, dual representation, aggregate oversized loss, post-summary context, overlay/replay composition, and false 0/1 process-status suppression are explicit. |
| Panel width and Unicode rendering (38–39, 43) | **Strong individually.** Correct width chain, cluster-driven rendering, and fallback decision/propagation are covered. Preserve their handoff into Task 40 per F2. |
| Bounded browse rendering (40) | **Behaviorally complete**, with an effective cost guard and current-width truncation. Add shared-renderer ordering per F2. |
| Overlay scrolling and reload admission (41–42) | **Ready.** Complete-row access, clamp semantics, ignored modal keys, append behavior, and both accepted/dropped reload paths are well specified. |
| Production seams and runtime shutdown (45–46) | **Architecture is sound.** Add tagged-runner RED coverage before Issue 45 closes (F3); Task 46's shutdown/replay matrix is otherwise strong. |
| Read-failure diagnostic safety (47) | **Behavior is complete**, but fixture setup/cleanup should be made deterministic and disposable (F5). |
| PTY handshakes (48) | **Correct direction but insufficiently pinned.** Add the transition/correlation matrix and production-manifest regression (F4). |
| Dependency manifests (49) | **Removal branch ready; adoption branch blocked.** Repair the branch graph before implementation (F1). |
| Closing verification (50) | **Strong.** It composes both build variants, race/repeat runs, production smoke, fixture-name separation, tidy/vulnerability gates, and walkthrough evidence. It becomes executable once Issue 49 has a real close path. |

## Recommended revision order

1. Repair Task 49's adoption branch first; it is the only critical execution blocker and directly controls whether Issue 50 may begin.
2. Declare the Task 39/40 renderer handoff and blocker order.
3. Add Issue 45's tagged-runner RED boundary tests before its GREEN topology task.
4. Replace Task 48's representative-only RED clause with a complete, correlated transition/acknowledgement matrix and require updates to the untagged artifact probe.
5. Make Task 47's real-failure fixtures temporary, synchronized, deterministic, and self-cleaning.
6. Recheck task-header blockers and AC mappings for Tasks 39, 40, 45, 48, and 49 after those edits, then proceed with implementation.

After these targeted corrections, Tasks 36–50 should be ready to execute. No application tests were run because this was a plan/task critique; the requested output is the review of whether the tasks are sufficient to guide and verify the later implementation.
