# PRD tasks critique 1 — VRG

## Scope and verdict

**Reviewed:** `Notes/skills/critique-tasks/SKILL.md`, `Notes/PRD-vrg.md` revision 4, all 34 files in `Notes/issues/`, and all 34 corresponding files in `Notes/tasks/`.

This is a **tasks-only review**. It evaluates whether the tasks are executable, correctly ordered, complete against their issues and the PRD, and backed by adequate verification. It does not review implementation or propose new product scope.

**Verdict: make a targeted task revision before implementation.** The task set is unusually detailed and generally preserves the PRD's difficult contracts. The RED/GREEN sequencing is strong, acceptance-criteria ownership is usually explicit, expensive work is kept off the UI path, and subprocess/terminal behavior is tested at the appropriate boundary rather than only through model assertions.

Two groups of tasks nevertheless have contradictory ownership across the declared issue order: Issue 26 requires Issue 32's append behavior even though Issue 32 is blocked by Issue 26, and Issue 27 requires the Issue 28 two-stage commit even though Issue 28 is blocked by Issue 27. The work might be made executable by treating the earlier issue as a local precursor and the later issue as a generalization, but the tasks do not say that. As written, an implementer cannot tell whether to implement a future-owned contract early or wait for a formally blocked issue. These ownership problems should be resolved before work reaches Issues 26–28.

The other material gaps are verification omissions: Issue 34 maps all acceptance criteria to its RED/GREEN pair but does not test its main scale and memory statements; Issue 11 does not exercise replay on `q`-while-searching; and Issue 10 does not explicitly instantiate both composite malformed/integrity cases. No PRD feature is wholly absent from the 34-task set.

### What is good

- Every issue has a corresponding task file, explicit issue blockers, per-file task dependencies, acceptance-criteria mapping, and an assigned manual-verification walkthrough.
- Behavioral work generally follows RED then GREEN. Tests are specified before implementation and focus on stable behavior rather than provisional Go signatures.
- The task set preserves the single outcome matrix, shared sink-safety table, binding table, and flag allow-list rather than creating competing sources of truth.
- Subprocess cleanup, reaping, terminal restoration, stderr ordering, dual-pipe backpressure, and cancellation use fake-process/PTY boundaries with handshakes rather than sleeps or model-only assertions.
- Async load/layout work is keyed by path, revision, width, and mode; stale completions and obsolete layouts receive explicit tests.
- The cached-file stale-layout gap identified during the issues review is addressed in Task 017 with both the request path and matching-layout fast path.
- The tasks stay within version-1 scope: no streaming browse, direct list selection, diagnostic-review UI, transcoding, cache eviction, or load queuing is introduced.

### Severity definitions

- **Critical:** the declared task graph is not executable without guessing ownership or violating a blocker.
- **High:** an acceptance contract can be omitted or materially misimplemented while the listed tasks still appear complete.
- **Medium:** an explicit edge case or boundary lacks deterministic coverage or instructions.
- **Low:** local traceability, test precision, or operational-hygiene weakness unlikely to change the architecture.

## Findings

### F1 — Critical: Issue 26 consumes Issue 32's append rule while Issue 32 is blocked by Issue 26

**Affected:**

- `Notes/tasks/026-read-failures-unreadable-retry-rules.md:5,35-55`
- `Notes/tasks/032-overlay-precedence-esc-semantics.md:5,19-31`
- Corresponding issue ownership in `Notes/issues/026-read-failures-unreadable-retry-rules.md:16-20` and `Notes/issues/032-overlay-precedence-esc-semantics.md:3-15`

Task 026's RED and GREEN work requires a second failure to append to an open error overlay while preserving the reader's scroll position. Task 026-4 explicitly says to reuse “the Issue #32 append rule.” Issue 32, however, cannot begin until Issue 26 is complete.

That creates contradictory implementation ownership. If Task 026 waits for the Issue 32 implementation, the graph cycles. If Task 026 implements append preservation itself, it is implementing a contract assigned to a future issue, but neither Task 026 nor Task 032 says that Issue 26 provides a provisional/local primitive which Issue 32 later generalizes. The same uncertainty affects tests: Task 032 expects the behavior to exist only after Task 026 is done, while Task 026 names it as Issue 32 behavior.

**Recommended correction:** choose and document one direction:

1. Preferably make Task 026 own a minimal error-overlay `AppendPreservingScroll` primitive and its second-failure test, then state that Task 032 generalizes that existing primitive to all errors and adds help/precedence semantics; or
2. Move the generic append primitive into an earlier independent task, remove Issue 26 from Issue 32's blockers, and block Issue 26 on that primitive.

Update the issue blockers, task blockers, RED/GREEN descriptions, and AC mapping together. Do not merely add each issue to the other's blocker list.

**Ready when:** Issue 26 can be completed in declared order without calling an unimplemented Issue 32-owned behavior, and Issue 32's later responsibility is unambiguous.

### F2 — Critical: Issue 27 requires Issue 28's two-stage commit while Issue 28 is blocked by Issue 27

**Affected:**

- `Notes/tasks/027-explicit-reload-r.md:5,11-32`
- `Notes/tasks/028-load-completion-reveal-latest-target.md:5,11-31`
- Corresponding issue ownership in `Notes/issues/027-explicit-reload-r.md:13-19` and `Notes/issues/028-load-completion-reveal-latest-target.md:12-27`

Task 027 requires reload anchor intent to be recorded on load completion and committed only when the new revision's matching prepared layout installs. It calls this “the two-stage commit contract owned by Issue 28.” Task 028 is explicitly blocked by Issue 27 and says Issues 17 and 27 provide its pieces.

A staged implementation is possible, but the boundary is not stated consistently. Task 027's GREEN step appears to require the complete installation-gated commit before the task that owns that contract may begin. Conversely, deferring the commit to Issue 28 means Task 027's anchor-preservation tests cannot pass as written. This is a task-graph ownership cycle even though the explicit issue-blocker graph itself remains acyclic.

**Recommended correction:** split the ownership explicitly. A clean option is:

- Task 027 owns reload request/state, content revisions, duplicate dropping, failure replacement, and a generic `reload-anchor` pending intent carried by the Issue 17 matching-layout installation path.
- Task 028 owns the generalized two-stage load/layout commit and reveal-vs-reload intent arbitration, building on the already-tested generic pending-intent seam.

Rewrite Task 027 so it does not claim to consume a future Issue 28 implementation, and state the exact seam Task 028 inherits. Alternatively merge Issues 27 and 28 into one work package.

**Ready when:** the RED tests for Issue 27 can pass before Issue 28 begins, and Task 028 adds integration behavior rather than retroactively supplying a prerequisite.

### F3 — High: Task 034 maps AC1–AC5 to RED/GREEN but does not test AC1–AC3 or the help-footer content

**Affected:**

- `Notes/tasks/034-documentation-scale-and-memory-limits.md:6,11-31`
- `Notes/issues/034-documentation-scale-and-memory-limits.md:12-19,24-35`
- `Notes/PRD-vrg.md:250-255`

The header says AC1–AC5 are owned by Tasks 1–2. Task 1's failing tests cover binding synchronization, the flag allow-list, exit statuses, ripgrep 15.x, and `--no-config`. It does not require failing assertions for:

- the three scale examples and the “independent, not simultaneous” qualification (AC1);
- the 64 MiB record limit, base64 expansion, and recoverable-path oversized diagnostic (AC2);
- session-long buffer retention, no eviction/aggregate bound, and no reliable OOM or forced-termination cleanup (AC3).

Task 2 tells the implementer to write those statements, but they are outside the RED test contract that Task 2 is instructed to satisfy. The issue also requires a help-overlay footer note, while neither the issue acceptance criteria nor Task 1 verifies that the footer carries the promised scale/memory/exit content or remains synchronized with the README.

**Recommended correction:** extend Task 034-1 with failing documentation tests for each AC1–AC3 clause and for the help footer. Prefer a shared structured source for the scale/limit text rendered into README/help, or at minimum assert the same required tokens and qualifications in both sinks. Update the task output and AC mapping to name this coverage.

**Ready when:** deleting any scale, record-limit, memory, termination, or footer statement makes Task 034's RED test fail.

### F4 — High: Task 011 does not verify diagnostic replay for `q`-while-searching cancellation

**Affected:**

- `Notes/tasks/011-stderr-replay-of-collected-diagnostics.md:11-19,35-55`
- `Notes/issues/011-stderr-replay-of-collected-diagnostics.md:31-38`
- `Notes/PRD-vrg.md:158-160,173,248`

Issue 11 AC3 explicitly covers cancellation by both `ctrl+c` and `q` while searching. Task 011's shutdown-boundary model test uses only `ctrl+c`; its cancellation PTY case also uses only `ctrl+c`. The other `q` PTY case is a normal quit after a completed stream, not cancellation while searching/result preparation remains incomplete.

Those paths have different state routing and can regress independently. Existing Issue 4 tests prove `q` cancels and Issue 11 tests prove replay on `ctrl+c`, but neither composition proves that already-collected diagnostics are replayed on the `q` cancellation route.

**Recommended correction:** add:

- a model test where a diagnostic is acknowledged as collected, then `q` arrives while searching or gate-held result preparation is incomplete, asserting exit 130 and replay without waiting for undelivered work; and
- a PTY case that waits for the application-side collection acknowledgement, sends `q` while the fake rg or preparation gate is still blocked, and asserts restored terminal state plus ordered exactly-once replay.

**Ready when:** both cancellation keys have end-to-end replay-ordering coverage.

### F5 — Medium: Task 010 does not explicitly instantiate the two records with combined malformed and integrity dispositions

**Affected:**

- `Notes/tasks/010-record-robustness-malformed-oversized-unknown.md:11-31,35-55`
- `Notes/issues/010-record-robustness-malformed-oversized-unknown.md:14-16,24-27`
- `Notes/issues/009-error-overlay-and-fatal-outcomes.md:28-32`

Task 010 says every schema-matrix row and lifecycle-matrix row receives a fixture and says malformed/integrity counts remain separate “except in the two cases the matrices mark both.” It does not explicitly require concrete fixtures for both composite cases:

1. an ordinary, non-oversized trailing unterminated record, counted malformed and stream-incomplete; and
2. a malformed record after a valid `summary`, counted malformed and also an after-summary integrity failure.

Testing schema rows and lifecycle rows independently is not necessarily a test of their composition. The oversized trailing record in Task 3 is a different disposition: oversized plus incomplete, explicitly not malformed.

**Recommended correction:** name both composite fixtures in Task 010-1 and assert both counters/statuses in each. In Task 010-2, explicitly route validation so after-summary positioning does not suppress malformed accounting, and trailing-unterminated accounting does not disappear into a generic decode failure.

**Ready when:** both “both” cases have dedicated test rows, distinct from the oversized unterminated case.

### F6 — Medium: Task 018's re-clamp trigger list is narrower than the authoritative contract

**Affected:**

- `Notes/tasks/018-horizontal-panning.md:11-31`
- `Notes/issues/018-horizontal-panning.md:27-34,38-53`
- `Notes/PRD-vrg.md:218-221`

Task 018-1 explicitly lists re-clamping on vertical scroll, reveal, resize, list hide/show, and gutter growth. The PRD and issue also require clamp re-evaluation on **wrap-toggle re-entry** and **on every pan**. The task mentions re-entry clamping after a width change and generally says pan operations are clamped, but it does not state or test the full invariant that every return to run-off-edge mode and every pan recomputes the maximum from the current visible rows.

This distinction matters when prepared visible-row extents change without the specific width-change scenario named in the task. A cached maximum could satisfy the listed shape tests while violating the authoritative re-evaluation rule.

**Recommended correction:** add explicit RED rows for wrap → run-off-edge re-entry after the visible set changes and for a pan using newly changed visible extents; update Task 018-2 to enumerate “wrap-toggle re-entry and every pan” alongside the other triggers.

**Ready when:** all PRD re-clamp triggers are named in both RED and GREEN work.

### F7 — Low: Task 003 does not explicitly test overlapping-submatch union or encoding equivalence

**Affected:**

- `Notes/tasks/003-spawn-rg-collect-results-searching-screen.md:11-31`
- `Notes/issues/003-spawn-rg-collect-results-searching-screen.md:15-28`

Task 003 tests both `text` and `bytes` encodings, same-line merging, and submatch ordering. The issue additionally states that overlapping submatches are retained and highlighted as their union, and that `text` and `bytes` forms containing the same bytes are the same value. Merely having separate fixtures for both encodings does not prove cross-encoding path identity or overlap composition.

Issue 9 later tests `text`/`bytes` path identity for lifecycle tracking, but Task 003 still lacks a focused index fixture for mixed-encoding records on the same path/line. Highlight union is especially easy to defer accidentally because Issue 003's acceptance criteria mention sorting but not overlap rendering.

**Recommended correction:** add mixed-encoding same-path/same-line fixtures that merge into one stop, and an overlapping-submatch fixture asserting the prepared highlight coverage is the union without dropping either original range.

**Ready when:** normalization behavior in Issue 3 line 26 is directly tested rather than inferred from separate encoding fixtures.

### F8 — Low: the unreadable-file walkthrough changes permissions without requiring cleanup

**Affected:**

- `Notes/tasks/026-read-failures-unreadable-retry-rules.md:69-75`

The walkthrough instructs the implementer to run `chmod 000` on a matched file but does not instruct them to use a disposable fixture or restore the original mode. A walkthrough should not leave a repository file unreadable, especially if interrupted after the permission change.

**Recommended correction:** require a temporary/disposable fixture directory and a cleanup step that restores the original mode (preferably through a shell trap), while retaining the injected-loader test as the authoritative deterministic verification.

**Ready when:** completing or interrupting the walkthrough does not leave user files with altered permissions.

## Coverage and readiness assessment

| Area | Assessment |
|---|---|
| CLI, roots, flags, child argv | Strong and executable. Combined short flags, cumulative `-u`, protected dash-leading operands, raw roots, and sanitized pre-TUI failures are covered. |
| Search parsing and stream integrity | Strong matrix-driven plan. Add the two composite disposition fixtures (F5) and explicit overlap/equivalence fixtures (F7). |
| Child lifecycle and diagnostics | Strong PTY/handshake strategy. Add `q`-cancellation replay coverage (F4). |
| Safe presentation | Strong. Sanitization lands before arbitrary-data rendering and later sinks extend the shared raw-output table. |
| Browse, navigation, wrapping, viewport | Strong. Cached stale-layout navigation is explicitly owned. Complete the horizontal re-clamp trigger list (F6). |
| Async loading, failures, reload | Behavior is detailed, but Issues 26–28 have future-owned contract ambiguity that must be untangled (F1–F2). |
| Overlays, pop-ups, help, too-small state | Comprehensive after ownership is clarified for append-preserving-scroll. |
| Documentation and limits | Content requirements are complete, but RED coverage does not enforce the main resource statements or footer (F3). |
| Operational walkthroughs | Generally specific and reproducible; unreadable-file cleanup needs tightening (F8). |

## Recommended revision order

1. Resolve F1 and F2 first by assigning the shared primitives and rewriting blockers/ownership. These affect whether the later task sequence can be executed at all.
2. Fix Task 034's RED contract (F3) and Task 011's missing cancellation route (F4).
3. Add the focused edge-case fixtures in Tasks 010, 018, and 003 (F5–F7).
4. Make the Issue 026 walkthrough disposable and self-cleaning (F8).
5. Recheck every task header's blocker list and AC mapping after the ownership edits; avoid creating explicit bidirectional blockers while fixing forward references.

After those targeted changes, the task set should be ready for implementation without a broader decomposition pass.
