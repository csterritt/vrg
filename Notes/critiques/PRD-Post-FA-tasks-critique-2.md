# Post-final-audit tasks critique 2 — VRG

## Scope and verdict

**Reviewed:**

- `Notes/skills/critique-tasks/SKILL.md`.
- `Notes/PRD-vrg.md`, revision 6, in full.
- `Notes/critiques/final-audit-vrg.md` as the source audit for Issues 36–50.
- `Notes/critiques/PRD-Post-FA-tasks-critique-1.md` as the prior critique whose findings this revision cycle was meant to resolve.
- Issues 36–50 in `Notes/issues/`, in full, plus Issue 9's lifecycle matrix as amended for Issue 44.
- Corresponding Tasks 36–50 in `Notes/tasks/`, in full.
- Current stated-stack references (PRD *Further Notes*, `Notes/wiki/project-overview.md`, `Notes/wiki/AGENTS.md`, `Notes/skills/code-writing/styling-tui.md`, `Notes/skills/AGENTS.md`) for Task 49's documentation scope.

This is a **tasks-only review**. It evaluates whether Tasks 36–50 remain complete against their parent issues and the PRD, executable in their declared order, and backed by deterministic verification. It does not review the implementation or add product scope. No PRD, issue, task, or implementation file was changed.

**Verdict: nearly ready — apply two small determinism/coverage amendments before implementation; no structural rework is needed.** Every critique-1 finding is resolved: Issue 40 is now blocked by Issue 39 with an explicit renderer handoff; Task 45-1 carries tagged-runner RED boundary tests; Task 48-1 requires a finite, correlated handshake matrix; Task 47's fixtures are disposable and gate-synchronized; and Issue/Task 49 record the product owner's removal decision, eliminating the unexecutable adoption branch entirely. The remaining findings are narrower than critique-1's: two records that satisfy more than one integrity-cause row are not assigned a deterministic cause set (Task 36), the bounded-render cost guard does not cover the navigation update path (Task 40), and three low-severity specification/fixture nits.

### What is strong

- Task 49 is now a clean linear flow (recorded decision → tidy → documentation → walkthrough), correctly keeps `scripts/verify.sh` out of scope for Issue 50, and the five documentation targets it names already exist and are enumerable.
- Task 45-1 now proves both halves of the build topology in RED — the clean untagged artifact and the tagged runner's tuple fidelity at the real `program.Run()` call site — while explicitly leaving shutdown/replay semantics to Issue 46.
- Task 48-1 specifies a per-row helper/action/postcondition/acknowledgement matrix with per-process, per-occurrence correlation, repeated same-kind events, overlay-dismissal-before-quit, and bounded-timeout failure; Task 48-2 extends Issue 45's manifest and untagged artifact probe to the new hook names.
- Tasks 41 and 48 carry a symmetric, non-concurrent shared-file ordering rule for `cmd/vrg/outcome_test.go`, with adaptation obligations stated in both directions.
- Task 47's fixtures are now deterministic (index → gate → remove/rename → real `os.ReadFile`) with no `chmod` reliance and cleanup on every outcome.
- Task 50 composes both build variants, uncached/race/repeated runs, the renamed `FAKE_RG_*` fixture boundary, a condition-driven smoke harness against the untagged binary, the tidy/vuln gates, the regression-to-owning-issue rule, and recorded evidence.

### Severity definitions

- **Critical:** the declared task graph cannot be executed for an allowed branch without guessing ownership, violating a dependency, or claiming an impossible output.
- **High:** the plan can complete while a material PRD or issue behavior remains absent or incorrectly implemented.
- **Medium:** an explicit acceptance boundary lacks deterministic coverage or two behaviors can each be implemented correctly yet disagree, creating a meaningful regression risk.
- **Low:** local traceability, specification precision, or fixture/artifact weakness unlikely to change the architecture.

## Findings

### F1 — Medium: Tasks 36's cause model does not pin behavior for records matching multiple integrity rows

**Affected:**

- `Notes/issues/036-stream-integrity-fatal-diagnostics.md:15-31`
- `Notes/tasks/036-stream-integrity-fatal-diagnostics.md:19,31,43`
- `Notes/issues/009-error-overlay-and-fatal-outcomes.md:20-32` (amended matrix)

Issue 36's cause matrix and Issue 9's amended transition matrix overlap on real records, and neither the issue nor the tasks pin the resulting cause set:

1. **A second `summary`** matches both "a second `summary` → extra `summary` record" and "any record after `summary` → record after `summary`". An implementer may emit either one cause (most specific row wins) or two (one per violated row). Issue 36's "each violation occurrence individually" clause does not disambiguate, because it is unstated whether one physical record may commit two integrity violations.
2. **A post-`summary` `begin(Q)`/`match(Q)`/`end(Q)`** — does the record produce only the after-`summary` cause, or does it also update lifecycle state (e.g. `begin(Q)` opens Q, producing a "missing `end`" end-of-stream cause for Q)? The EOS ordering rule assumes a defined set of still-open files but never says whether post-`summary` records are lifecycle-processed.
3. **A trailing unterminated fragment after a valid `summary`** matches "trailing unterminated record" and, under the dual-representation clause's wording ("a malformed record after `summary`"), arguably the after-`summary` cause as well — plus the malformed count. One, two, or three outputs are all defensible.

The issue's core promise is deterministic, exactly-specified diagnostic output; Task 36-1/36-3's RED matrices cover each row in isolation, so two conforming implementations can produce different overlays and replays for the same stream, and no required test distinguishes them.

**Recommended correction:** add one rule to Issue 36 (and mirror it in Task 36-1/36-3's RED): state whether each physical record produces at most one integrity cause (chosen by the most specific applicable row — e.g. second `summary` → extra `summary` only) or one cause per violated row; state that post-`summary` records produce the after-`summary` cause only and do not mutate lifecycle state (or the intended alternative); and state the cause set for a post-`summary` trailing unterminated fragment. Add explicit RED rows for a second `summary`, a post-`summary` `begin`, and a post-`summary` unterminated fragment asserting the exact composed lines.

**Ready when:** the RED suite asserts the complete line set — not merely the presence of one acceptable cause — for every record matching multiple matrix rows, so divergent implementations cannot both pass.

### F2 — Medium: Task 40's cost guard is scoped to `View()` only; AC4's no-regroup-on-navigation has no failing test

**Affected:**

- `Notes/issues/040-browse-render-no-whole-index-scan.md:16,30`
- `Notes/tasks/040-browse-render-no-whole-index-scan.md:6,14-19,31`

Issue 40's AC4 requires navigation to update current-file tracking "by index into precomputed groups without reallocation of the whole grouping", and the motivating defect is per-keystroke cost at ~100,000 matched lines. Task 40-1's RED, however, requires only that *rendering* a completed search perform no whole-index enumeration — "an index/provider test double that panics or fails when the whole-stop accessor is invoked during `View()`". A `groupByFile`/`Stops()` call placed in the `Update` navigation path would evade every required test while still performing O(index) work on each `n`/`p` keystroke — the same responsiveness violation, one function earlier.

**Recommended correction:** extend the Task 40-1 RED to cover a full navigation step: with the counting/panicking index double installed, send a navigation key through `Update` plus the resulting `View()` and assert bounded index access across both halves. State that the bound applies to any model-transition path, not only the render.

**Ready when:** a whole-index scan in either `Update` or `View` during navigation fails the cost guard, matching AC4 and the per-keystroke intent of the audit finding.

### F3 — Low: Task 36's header maps only nine of Issue 36's ten acceptance criteria

**Affected:**

- `Notes/issues/036-stream-integrity-fatal-diagnostics.md:50-59`
- `Notes/tasks/036-stream-integrity-fatal-diagnostics.md:6`

Issue 36 has ten acceptance criteria. The task header maps AC4–AC6 → Tasks 1–4, AC1–AC3 and AC7–AC8 → Tasks 3–4, and "AC9 → Task 3". As numbered, AC9 is the replay-parity criterion (same lines, same order, replayed to stderr) — which Task 4's composition work delivers, so it belongs on Tasks 3–4 — while AC10 ("the new outcome tests assert the actual explanation content and ordering") is unreferenced; the header appears to have conflated the last two criteria under "AC9".

**Recommended correction:** remap the tail as AC9 → Tasks 3–4 (replay parity is asserted in RED and delivered by the single composition in GREEN) and AC10 → Task 3.

**Ready when:** all ten ACs are named in the header mapping and each maps to the tasks that actually deliver it.

### F4 — Low: Task 39's mechanical guard has no defined detection rule

**Affected:**

- `Notes/issues/039-render-from-shared-grapheme-cell-model.md:17,24,33`
- `Notes/tasks/039-render-from-shared-grapheme-cell-model.md:19,31`

Both the issue and the task require "a grep/static guard that rejects rune-decoding width or truncation loops outside the single shared grapheme/cell helper", while also promising that "byte decoding for purposes unrelated to display geometry is unaffected". A source scan for `utf8.DecodeRuneInString` cannot by itself distinguish a display-width loop from legitimate decoding — the sanitizers that detect invalid UTF-8 and control bytes plausibly decode runes, and they live in the same `internal/safepresentation` package proposed as the helper's home. As written, the guard either false-positives on the escapers or is so narrowly phrased it never fires, and neither failure mode is detectable by the required tests.

**Recommended correction:** pin the guard's rule — e.g. "the symbol `utf8.DecodeRuneInString` may appear only in file X/the shared helper, with escaping implemented via `range`-loops or a named allow-listed decoder" — or specify an annotation/allow-list convention the scan enforces. The choice matters less than making the check mechanically decidable.

**Ready when:** the guard's pass/fail predicate is exact enough that a reviewer can predict its verdict on the existing escapers without running it, and a disguised geometry loop outside the helper provably fails it.

### F5 — Low: Task 50-2 modernizes `smoke.py` inside the completed 035-03 walkthrough artifact

**Affected:**

- `Notes/issues/050-post-audit-reverification.md:27-28,35`
- `Notes/tasks/050-post-audit-reverification.md:24-31`
- `Notes/walkthroughs/035-03/code-walkthrough/smoke.py` (target path)

The canonical smoke harness lives inside `Notes/walkthroughs/035-03/code-walkthrough/` — a completed showboat walkthrough whose recorded transcript was captured against the harness as it then existed. Rewriting the harness in place means the 035-03 artifact's recorded commands no longer correspond to the code on disk, weakening it as evidence while every other walkthrough in the project is treated as append-only.

**Recommended correction:** either move the canonical harness to a neutral location (`scripts/smoke.py` or a fixture directory outside `walkthroughs/`, leaving 035-03 frozen and pointing its documentation at the new home) or, if in-place modernization is deliberate, require Task 50-2 to record in the walkthrough evidence that 035-03's harness was superseded and why its transcript diverges from the current file.

**Ready when:** the smoke harness has a stable canonical home that does not rewrite a closed evidence artifact, or the divergence is explicitly documented.

### F6 — Low: Issue/Task 37 does not pin per-path versus per-record lines for duplicate oversized paths

**Affected:**

- `Notes/issues/037-oversized-record-aggregate-anonymous-diagnostics.md:17,32`
- `Notes/tasks/037-oversized-record-aggregate-anonymous-diagnostics.md:19,31`

"Each recoverable path is named once" is satisfied by both "one detail line per oversized record" and "one detail line per distinct path" when two oversized records share a path — the aggregate is pinned exactly (`N oversized records skipped`) but the detail section's multiplicity is not. Given Issue 36's deliberate uncapped one-line-per-violation multiplicity for integrity causes, an implementer could reasonably choose either behavior for record loss.

**Recommended correction:** state explicitly whether per-path oversized details are emitted per record or deduplicated per path, and add the duplicate-path case to Task 37-1's RED.

**Ready when:** two oversized records naming the same path have an asserted, unambiguous expected output.

## Coverage and readiness assessment

| Area | Assessment |
|---|---|
| Integrity and diagnostic composition (36–37, 44) | **Strong, minus F1/F6.** Cause recording, ordering, dual representation, universal component order, and the anonymous/aggregate oversized rules are well specified; only the multi-row record overlaps and detail-line multiplicity need pinning. Note also that existing tests encoding the superseded `ripgrep exited with code 0` fatal text will fail under Task 36-4 — expected, and surfaced by the required full-suite run. |
| Panel width and Unicode rendering (38–39, 43) | **Ready.** The three-width chain is correctly distinguished at every install site including resize and list toggle; the cluster-driven renderer handoff to Issue 40 is explicit; the HITL fallback decision is properly gated. Only the guard-detection nit (F4) remains. |
| Bounded browse rendering (40) | **Nearly ready.** Groups/metadata precomputation and visible-range truncation are correct; extend the cost guard across navigation per F2. |
| Overlay scrolling and reload admission (41–42) | **Ready.** Complete-row scroll set, clamp bounds, append behavior, ignored modal keys, and both sides of the reload admission boundary are fully specified, with the 41/48 shared-file ordering resolved symmetrically. |
| Production seams and runtime shutdown (45–46) | **Ready.** The manifest is explicit, fixture variables are correctly excluded, the untagged artifact probe and tagged-runner reachability are both covered in RED, and Issue 46's ordered replay/exit-2 matrix is complete. |
| Read-failure diagnostic safety (47) | **Ready.** Deterministic disposable fixtures, gate-synchronized real failures, single-line construction at all three load sites, and cleanup traps are all present. |
| PTY handshakes (48) | **Ready.** The finite matrix, per-occurrence correlation, dedicated regressions, and manifest/artifact-probe extension satisfy critique-1's concerns. |
| Dependency manifests (49) | **Ready.** The removal decision is recorded, the task graph is linear, and `verify.sh` ownership is cleanly deferred to Issue 50. Note that the named documentation targets already record the "not dependencies" stance, so Task 49-3 is largely a confirm-and-polish pass rather than a rewrite — harmless, but the walkthrough should not manufacture diffs. |
| Closing verification (50) | **Ready.** All ten gates, the named PTY reruns, the `FAKE_RG_*` boundary, and the condition-driven smoke harness are complete; F5 covers only the artifact-location nit. |

## Recommended revision order

1. Pin Issue/Task 36's cause-assignment rule for records matching multiple matrix rows (F1) — this belongs in the issue text since it defines diagnostic output, then mirror it in Tasks 36-1 and 36-3's RED rows.
2. Extend Task 40-1's cost guard across a full navigation `Update`+`View` step (F2).
3. Fix Task 36's header AC mapping for AC9/AC10 (F3); pin Issue/Task 37's detail-line multiplicity (F6).
4. Define Task 39's mechanical-guard predicate (F4); decide Task 50-2's smoke-harness location or document the 035-03 divergence (F5).
5. Recheck task-header blockers after the edits — none should change — then proceed with implementation.

All critique-1 findings are resolved and the remaining items are local amendments rather than a regeneration. After these corrections, Tasks 36–50 are ready to execute. No application tests were run because this was a plan/task critique; the requested output is the review of whether the tasks are sufficient to guide and verify the later implementation.
