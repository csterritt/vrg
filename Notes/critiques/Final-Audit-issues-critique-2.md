# Final-audit issues critique 2 — VRG

## Scope and verdict

Reviewed:

- `Notes/skills/critique-issues/SKILL.md`.
- `Notes/PRD-vrg.md`, revision 6, in full.
- `Notes/critiques/final-audit-vrg.md`, in full.
- Issues 36–50 in `Notes/issues/`, in full.
- The relevant earlier contracts in Issues 9 and 35.
- `Notes/critiques/Final-Audit-issues-critique-1.md` and the revised issue text intended to address it.
- Targeted implementation, test-harness, and smoke-harness locations named by the issues, solely to verify that the proposed boundaries and instructions fit the current repository.

This is an **issues-only review** of the revised post-audit issue set. It evaluates whether Issues 36–50 faithfully and completely convert the final-audit findings into executable work with correct dependencies and acceptance criteria. No PRD, issue, task, or implementation file was changed.

**Verdict: the first critique's substantive product and implementation-contract findings have largely been corrected, but five targeted corrections remain before Issues 36–50 should be converted into tasks.** Issues 36–44, 47, and the behavioral intent of 45–46 and 48–50 are now substantially stronger. The remaining blockers are concentrated in the executable-boundary verification plan: Issues 49 and 50 have circular ownership of the permanent verification script, Issue 50's required untagged smoke run still depends on a hook Issue 45 removes, the reused smoke harness still synchronizes through sleeps, and Issue 46 does not define a seam capable of producing its required `program.Run()` return-shape matrix.

### Severity definitions

- **High:** the issue can be completed literally while the audited defect or a material PRD violation remains, or its stated verification cannot work after declared blockers land.
- **Medium:** the intended correction is clear, but dependencies, ownership, or verification are ambiguous enough to permit inconsistent implementation or false closure.
- **Low:** local wording inconsistency unlikely to affect product behavior but liable to create review disagreement.

## Findings

### F1 — High: Issue 50's untagged smoke run depends on the production hook Issue 45 removes

**Affected:**

- `Notes/issues/045-remove-test-hooks-from-production-binary.md:14-23,29-38`
- `Notes/issues/050-post-audit-reverification.md:23-25,31,36-42`
- `Notes/walkthroughs/035-03/code-walkthrough/smoke.py:20,309-349`

Issue 45 requires every `VRG_TEST_*` hook name and behavior to be absent from the untagged production binary. Issue 50 then explicitly builds an untagged binary with:

```text
go build -o /tmp/vrg-smoke/vrg ./cmd/vrg
```

and reuses `Notes/walkthroughs/035-03/code-walkthrough/smoke.py`. The cancellation scenario in that harness sets `VRG_TEST_REAP` and requires the resulting reap file as evidence. Once Issue 45 lands, the untagged binary must ignore that variable, so the prescribed smoke run will fail its `reap evidence present` check even if cancellation and cleanup are correct.

Changing the smoke command to build with `-tags vrg_testhooks` would make the hook work, but would no longer smoke-test the production artifact that Issue 50 calls the final binary. The current issue therefore offers no literal path that simultaneously satisfies Issue 45 and Issue 50.

**Required correction:** keep the Issue 50 smoke run on an untagged production binary and revise the smoke harness to prove child termination/cleanup externally, without `VRG_TEST_REAP` or any other removed production hook. The fake rg can expose its PID/process group through its own fixture environment, while the parent harness observes process termination and terminal restoration. Retain tagged hook-based evidence in the focused `cmd/vrg` subprocess suite, where Issue 45 explicitly permits it. State the artifact distinction in Issue 50: tagged binary for test-only seam tests, untagged binary for final production smoke.

**Ready when:** the exact smoke command succeeds after Issue 45, and no acceptance criterion for the production artifact depends on a `VRG_TEST_*` behavior.

### F2 — Medium: Issues 49 and 50 create circular ownership of `scripts/verify.sh`

**Affected:**

- `Notes/issues/049-tidy-dependency-manifests.md:19-27,34-36`
- `Notes/issues/050-post-audit-reverification.md:3-5,12-22,34-42`

Issue 49 requires the tidy check to be added to the permanent verification entry point and makes that requirement part of closure. Issue 50 says it will **create** that entry point, `scripts/verify.sh`, but is blocked by Issues 36–49. As written, Issue 49 either cannot close until its successor, Issue 50, creates the script—even though Issue 50 is blocked by Issue 49—or must create/partially own the script that Issue 50 says it owns. The first interpretation is a dependency cycle; the second gives two issues overlapping creation and acceptance ownership.

The final state is correct—the repository should have a permanent tidy gate—but the graph is not execution-safe.

**Required correction:** assign ownership once. The simplest ordering is:

1. Issue 49 resolves the dependency decision, commits tidy manifests/documentation, and verifies `go mod tidy -diff` directly.
2. Issue 50, already blocked by Issue 49, creates `scripts/verify.sh` and permanently incorporates the tidy check as part of the closing gate.

Remove Issue 49's requirement that the not-yet-created permanent script already contain the check, while retaining the direct clean-tidy acceptance criterion. Alternatively, make Issue 49 create the complete script and revise Issue 50 to consume and verify it rather than create it; do not split partial script ownership across both issues.

**Ready when:** Issue 49 can close before Issue 50 starts, and exactly one issue owns creation of `scripts/verify.sh`.

### F3 — Medium: Issue 46's inherited test seam cannot produce the required `program.Run()` return shapes

**Affected:**

- `Notes/issues/045-remove-test-hooks-from-production-binary.md:14-23,34-38`
- `Notes/issues/046-runtime-error-common-diagnostic-replay.md:3-4,14-29,31-37`
- `cmd/vrg/main.go:169-209`

Issue 46 now correctly requires an independent diagnostic snapshot and a three-row matrix covering valid, invalid/nil, error, and nil-error returns from `program.Run()`. It says its runtime-error injection will consume Issue 45's tagged harness. Issue 45's selected injection point, however, is `testSeamOptions(proc)`, which only supplies `app.Option` values and process wiring before `tea.NewProgram` is constructed. App options can trigger model messages or controlled quits; they cannot make `tea.Program.Run()` return an arbitrary error or substitute an invalid/nil final model. Consequently, a test can appear to cover a controlled model failure while never exercising the audited entry-point branches at lines 173–194.

Issue 46 also describes the shutdown sequence as terminating/reaping the child and then letting Bubble Tea restore the terminal. At this entry point, `program.Run()` has already returned—and Bubble Tea has already performed its restoration—before `runSearch` regains control to clean up the process. The actual invariant should be that restoration and child cleanup both complete before replay, not the currently stated impossible call order.

**Required correction:** define a dedicated build-constrained program-runner seam in the Issue 45 topology, separate from app options—for example, complementary tagged/untagged wrappers around program construction and `Run()` whose production implementation delegates directly and whose tagged implementation can return each matrix shape. Require subprocess tests to prove they reached the actual `Run()` result branches, not merely a controlled App quit. Rewrite the shutdown order as: `Run()` returns after terminal restoration; then the common path terminates/reaps the child as needed; then it emits the deterministic replay. If cleanup can occur inside the model before `Run()` returns, state that alternate ownership explicitly while preserving replay-after-restoration.

**Ready when:** each required return-shape row is injectable at the real executable boundary and the stated cleanup/restoration order is implementable.

### F4 — Medium: Issue 50 reuses a sleep-synchronized smoke harness despite the deterministic-handshake contract

**Affected:**

- `Notes/PRD-vrg.md:301-316`
- `Notes/issues/048-pty-tests-deterministic-handshakes.md:10-31`
- `Notes/issues/050-post-audit-reverification.md:23-25,31,37-40`
- `Notes/walkthroughs/035-03/code-walkthrough/smoke.py:51-57,93-99,157-167,169-200`

Issue 48 removes fixed sleeps from the named Go PTY tests, but Issue 50's closing smoke evidence reuses a Python PTY harness that waits for a child-created file, sleeps `key_delay`, sleeps between keys, polls process exit with sleeps, and sleeps again before draining output. The child-created handshake only proves the fake rg reached a point in its script; it does not prove the App processed the stream, entered the expected overlay/base state, or handled the previous key. The 0.15–0.3 second delays are still elapsed-time proxies for exactly those transitions.

This conflicts with the PRD's “no sleep-based synchronization” and explicit readiness/completion-handshake decisions. It is also especially risky for the newly required multi-key flows: dismissing an overlay and then quitting can race if the second key arrives before the first state transition is processed.

Polling an explicit condition with a bounded timeout is acceptable; sleeping for an assumed settling interval is not. Draining should be completion/EOF-driven rather than delayed by a fixed post-exit interval.

**Required correction:** make Issue 50 require modernization of `smoke.py`, not merely reuse/extension for new assertions. For the untagged production smoke, wait on externally observable PTY output/state markers and process/pipe completion rather than internal test hooks or fixed settling delays. Each key in a multi-key scenario must be sent only after the preceding expected state is observed. Bounded timeout polling may remain solely to fail a missing condition. Add an automated/review assertion that the smoke harness contains no fixed delay used as a progress proxy.

**Ready when:** the closing smoke scenarios are deterministic on a slow machine and no key or output assertion depends on an arbitrary settling duration.

### F5 — Low: Issue 45's selected topology contradicts its “no test-hook dispatch” acceptance wording

**Affected:**

- `Notes/issues/045-remove-test-hooks-from-production-binary.md:14-21,32-38`

The selected topology deliberately leaves a call such as `testSeamOptions(proc)` in the common production entry point and supplies a no-op untagged implementation. That can be a safe and idiomatic build-constrained design: the released artifact contains no hook environment names, watchers, or behavior. However, the same issue later requires “no `VRG_TEST_*` strings or dispatch” and “no test-hook code” in the production artifact. A reviewer can reasonably interpret the prescribed `testSeamOptions` call and no-op function as test-hook dispatch/code, making the issue fail its own acceptance criteria despite implementing the selected topology exactly.

**Required correction:** align the acceptance wording with the chosen design. Require no test-control names, environment reads, side effects, watchers, or conditional behavior in the untagged artifact; explicitly permit the inert complementary implementation and common call site. If the stronger “no test-named call or function at all” requirement is intended, select a different topology that can actually provide tagged executable-boundary seams without that common call.

**Ready when:** one implementation cannot be both prescribed by the build-topology section and rejected by the acceptance criteria.

## Coverage and readiness assessment

| Issue area | Assessment |
|---|---|
| 36: stream-integrity diagnostics | **Ready.** The revised matrix, multiplicity, path escaping, deterministic ordering, process/integrity separation, and content-level tests address critique-1 F1–F2. |
| 37: oversized aggregate/anonymous diagnostics | **Ready.** Aggregate and per-path behavior are explicit for usable and zero-result outcomes. |
| 38: viewport width | **Ready.** Terminal, panel, and text widths are now correctly distinguished, including gutter and indicator reservations. |
| 39, 43: grapheme/cell rendering | **Ready for the declared order.** Issue 39 has a coherent end-to-end renderer boundary; Issue 43 correctly makes the unresolved visible fallback representation HITL before implementation. |
| 40: render cost | **Ready.** It now precomputes only width-independent metadata and permits bounded visible-row truncation after resize. |
| 41: full overlay scrolling | **Ready.** The corrected verification preserves the modal key contract and makes all rows reachable. |
| 42: dropped reload | **Ready.** Admission and mutation are correctly made one state decision with focused startup/navigation/reload cases. |
| 44: post-summary context | **Ready.** The dependency on Issue 36 is now explicit and pre-summary context behavior remains unchanged. |
| 45: production/test harness separation | **Nearly ready.** The build topology is viable, but acceptance wording must permit its own inert common seam (F5), and Issue 46 needs a runner-level extension (F3). |
| 46: runtime replay | **Not ready.** The result and replay contract is much improved, but the declared seam cannot inject the required `Run()` matrix and the shutdown sequence is misstated (F3). |
| 47: single-line read failures | **Ready.** It correctly separates escaped path identity from the underlying error reason and covers initial/reload/retry, overlay, and replay. |
| 48: deterministic PTY handshakes | **Ready for the named Go tests.** It is correctly blocked by Issue 45 and distinguishes condition polling with a timeout from sleeping as a state proxy. |
| 49: dependency manifests | **Not execution-ready.** The decision branches are now bounded, but permanent-script ownership cycles with Issue 50 (F2). |
| 50: closing verification | **Not ready.** Commands and evidence paths are now reproducible, but the production smoke conflicts with Issue 45 and remains sleep-synchronized (F1, F4). |

## Positive observations

- Every final-audit finding remains represented by a focused issue; no audit coverage was lost during revision.
- The revised Issue 36 now covers every lifecycle failure, distinguishes successful process status from stream failure, defines stable path escaping and ordering, and tests diagnostic content rather than only outcome shape.
- Issue 38 now uses the PRD's actual three-level width model and prevents a test from silently omitting gutter or indicator reservations.
- Issues 40 and 41 correct the prior dynamic-truncation and modal-key ambiguities without weakening their performance/accessibility goals.
- Issue 43 appropriately stops for a product decision instead of allowing different visible fallback conventions to pass the same acceptance criteria.
- The revised dependency edges `44 → 36`, `46 → 45`, and `48 → 45` remove the most important implementation-order hazards identified in critique 1.
- Issue 49's adoption branch no longer permits token imports or an unbounded UI migration hidden inside manifest cleanup.
- Issue 50 now pins `govulncheck`, names focused PTY tests, defines a permanent gate path, and names the walkthrough artifact; those were material improvements over the first draft.

## Required revision order

1. Resolve Issue 49/50 ownership so Issue 49 can close before its declared blocker starts.
2. Extend Issue 45's selected build topology with the runner-level seam Issue 46 actually needs, and correct Issue 46's restoration/cleanup wording.
3. Rework Issue 50's untagged production smoke so it does not consume `VRG_TEST_REAP` or any other hook removed by Issue 45.
4. Require the Issue 50 smoke harness to use observable conditions/EOF rather than fixed settling sleeps.
5. Reconcile Issue 45's “no dispatch/test-hook code” wording with its prescribed inert common injection point.
6. Perform one final issues-only closure check; if these changes are made without expanding scope, Issues 36–50 will be ready for task generation.

No application tests were run because this review evaluates issue quality rather than implementation correctness. Targeted code, tests, and the existing smoke harness were inspected only to verify that the proposed work and dependency boundaries are executable in the current repository.
