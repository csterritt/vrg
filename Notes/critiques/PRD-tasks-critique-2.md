# PRD tasks critique 2 — VRG

## Scope and verdict

**Reviewed:** `Notes/skills/critique-tasks/SKILL.md`, `Notes/PRD-vrg.md` revision 4, all 34 files in `Notes/issues/`, all 34 corresponding files in `Notes/tasks/`, and `Notes/critiques/PRD-tasks-critique-1.md`.

This is a **tasks-only review**. It evaluates whether the revised tasks are complete against their issues and the PRD, executable in their declared order, sufficiently precise, and able to produce a working program with verifiable outcomes. It does not review implementation or add product scope.

**Verdict: the revision resolved all eight findings from critique 1, and the task graph is now broadly executable. Make one final targeted revision before implementation.** The most important remaining task-set defect is that no late task owns a clean, repository-wide build, vet, and test pass after all 34 work packages have composed. The other findings are narrower: an authoritative `Esc` contradiction that the tasks currently encode one way, two explicit acceptance combinations missing from RED coverage, one promised sink-safety extension not assigned in Task 034, and two small state/parser edge cases.

No PRD feature is wholly absent. The blocker graph is acyclic, transitive prerequisites generally supply the named seams, and the difficult async, terminal-restoration, stale-layout, reload-intent, grapheme, marker, and diagnostic contracts receive unusually strong deterministic coverage.

### What improved since critique 1

All prior findings are resolved:

- Issue 26 now owns the minimal append-preserving-scroll primitive; Issue 32 explicitly generalizes it.
- Issue 27 now owns a tested reload-anchor pending-intent seam; Issue 28 explicitly generalizes it.
- Task 034 now puts scale, record-limit, memory, termination, and footer content under RED tests.
- Task 011 now covers diagnostic replay for `q` cancellation while search/preparation remains incomplete at both model and PTY boundaries.
- Task 010 now names both composite malformed-plus-integrity fixtures.
- Task 018 now names wrap-toggle re-entry and every pan as re-clamp triggers.
- Task 003 now tests mixed-encoding identity and overlapping-submatch union coverage.
- Task 026's permission walkthrough now uses a disposable fixture and restoration trap.

### Severity definitions

- **Critical:** the declared graph cannot be executed without guessing ownership or violating dependencies.
- **High:** the plan can complete while the resulting program or authoritative behavior remains materially unverified or contradictory.
- **Medium:** an explicit acceptance contract, safety obligation, or meaningful interaction is not deterministically covered.
- **Low:** a focused edge case or task-operational detail is underspecified but unlikely to alter the architecture.

## Findings

### F1 — High: no final task owns a clean repository-wide verification pass

**Affected:**

- `Notes/tasks/001-go-scaffold-cli-positionals-and-root.md:13-17,55-61`
- `Notes/tasks/034-documentation-scale-and-memory-limits.md:11-31,45-51`
- All intervening GREEN and walkthrough tasks, which generally name focused tests rather than a final complete suite

The only explicit `go build ./...` and `go vet ./...` requirements occur in the initial scaffold. The Issue 1 walkthrough also runs those commands and focused early tests. Later tasks require their own tests to pass, and Task 034 finishes with documentation synchronization tests and a documentation walkthrough, but no final task reruns:

- `go test ./...` over the fully composed implementation;
- `go vet ./...` after all packages and asynchronous paths exist;
- `go build ./...` after the final help/footer/documentation integration; and
- a smoke invocation of the final binary rather than an earlier feature slice.

This matters because the task sequence repeatedly extends shared code and tables: the Issue 9 outcome matrix, Issue 6 sink-safety table, binding table, App key routing, prepared-layout message flow, and process-boundary harness. A late change can break an earlier package or test while its focused task still appears green. Walkthroughs do not replace one clean final verification owner.

**Recommended correction:** add a final verification task after Task 034's implementation/documentation work (or a dedicated final integration issue) that, from a clean checkout:

1. runs `go test ./...`, `go vet ./...`, and `go build ./...`;
2. runs the critical PTY/subprocess tests without relying on sleeps;
3. builds and smoke-runs the final `vrg` binary for a successful browse, no results, a fatal fake-rg outcome, and cancellation; and
4. records the commands and results in the final walkthrough or review artifact.

The task should fail closure on any regression; it should not merely say that individual focused tests were previously green.

**Ready when:** one final task proves the fully assembled repository builds, vets, tests, and launches after every feature has landed.

### F2 — High: the authoritative PRD contradicts the task plan over whether `Esc` can terminate

**Affected:**

- `Notes/PRD-vrg.md:172,242`
- `Notes/issues/009-error-overlay-and-fatal-outcomes.md:36-40,61-62`
- `Notes/tasks/009-error-overlay-and-fatal-outcomes.md:43,55`
- `Notes/tasks/032-overlay-precedence-esc-semantics.md:19,26,31,41,51`

The outcome section says a fatal no-results overlay exits 2 when dismissed and Issue 9 explicitly requires either `q` or `Esc` to cause that exit. Task 009 tests it, and Task 032 repeats the fatal/no-results `Esc` → 2 route. However, the later authoritative key-precedence statement says: **“`Esc` never exits the program.”** Task 032's documentation task then calls the fatal-overlay route “the only `Esc` termination,” while its GREEN output says “`Esc` never requests exit by itself.”

The internal implementation distinction—`Esc` dismisses, then a terminal post-dismissal state exits—does not remove the observable contradiction: from the user's perspective, pressing `Esc` exits the program in that state. An implementer following line 242 literally could make `Esc` dismiss into a persistent fatal screen and still believe the global rule was satisfied; an implementer following Issues 9/32 exits 2.

**Recommended correction:** make the PRD explicit: “`Esc` never exits from a base state; dismissing a fatal no-results overlay with `Esc` terminates with status 2 because there is no underlying state.” Use that same wording in Issues 9 and 32 and in Task 032's output. Avoid the unqualified “never exits” claim.

**Ready when:** the PRD, issue acceptance criteria, RED rows, GREEN output, walkthrough, and wiki instructions state one observable rule without relying on an internal-event semantic distinction.

### F3 — Medium: Task 017 maps AC6 to its RED/GREEN pair but omits two required gated inputs

**Affected:**

- `Notes/issues/017-logical-anchor-through-rewrap-and-resize.md:24,37,50`
- `Notes/tasks/017-logical-anchor-through-rewrap-and-resize.md:6,38,43,50-55`

Issue 17 AC6 requires `ctrl+c`, `q`, **`n`/`p`**, **`w`**, and another resize to remain actionable while layout preparation is held. Task 017-3's gated test explicitly exercises `ctrl+c`, `q`, `n`, and a second resize, but not `p` or `w`. Rapid `w` is covered in an out-of-order completion scenario, which proves stale-mode isolation after requests exist; it does not prove that the toggle handler itself remains responsive while a worker is blocked. `p` can also have a distinct circular/file-transition path from `n`.

Because AC6 is mapped entirely to Tasks 3–4, the issue can be marked complete without directly proving all inputs named by its acceptance criterion.

**Recommended correction:** extend Task 017-3's held-gate table with `p` and `w`. For `p`, assert immediate cursor movement and preservation of the latest pending reveal. For `w`, assert immediate mode change and issuance/replacement of the keyed layout request without releasing the existing gate.

**Ready when:** every input enumerated by AC6 has a deterministic held-worker test.

### F4 — Medium: Task 026 omits the explicit “all files fail while fixed status is already 2” combination

**Affected:**

- `Notes/issues/026-read-failures-unreadable-retry-rules.md:22,29,39`
- `Notes/tasks/026-read-failures-unreadable-retry-rules.md:6,14,19,31,65`

The issue deliberately states that load failures do not change the fixed search-derived status both when **every retained file fails** and when the fixed status is already **2**. AC7 combines those dimensions. Task 026-1 tests:

- every retained file failing with fixed status 0; and
- one current-file failure with fixed status 2.

It does not instantiate every retained file failing with fixed status 2. The immutable-status design makes the expected result unsurprising, but this is an explicit acceptance combination and the task's output claims coverage of the new outcome rows. A later implementation could special-case “all unavailable” and overwrite a pre-existing fatal outcome or presentation while still passing the two listed rows.

**Recommended correction:** add a matrix row for usable search results with fixed status 2 where every retained file subsequently fails to load; assert the ordinary status remains 2, the load failures only affect file presentation/diagnostics, and the already-fixed fatal-search outcome is not recomputed.

**Ready when:** AC7's all-fail and fixed-2 dimensions are tested together, not only separately.

### F5 — Medium: Task 034 does not perform the sink-safety extension explicitly assigned to Issue 34

**Affected:**

- `Notes/tasks/006-safe-presentation-utility-for-all-sinks.md:19,31`
- `Notes/issues/034-documentation-scale-and-memory-limits.md:12,20`
- `Notes/tasks/034-documentation-scale-and-memory-limits.md:14,19,26,31`

Task 006 establishes an extensible shared sink-safety table and explicitly names Issue 34 as responsible for adding its own sink row. Task 034 now thoroughly tests README/footer content and says runtime strings must use the Issue 6 utility, but it never explicitly adds the generated help-footer path—or any generated documentation path containing runtime substitutions—to that shared hostile-fixture table.

The static scale text itself is not dangerous, and Issue 31 already tests the base help overlay sink. The gap is narrower: Issue 34 was assigned an explicit extension obligation, and its generated/substituted footer content can be added without the raw-output safety assertion promised by Task 006.

**Recommended correction:** in Task 034-1, add a sink-safety row for the rendered help footer and any generated text path that accepts runtime strings, using Issue 6's hostile fixtures and no-style/raw-output assertions. In Task 034-2, explicitly satisfy that row through the shared utility. If Issue 34 is guaranteed to contain static text only, remove the runtime-string clause and amend Task 006 so it no longer assigns Issue 34 a sink-row obligation.

**Ready when:** either Issue 34 adds the promised hostile-input sink row or the plan explicitly proves and records that it introduces no new runtime-data sink.

### F6 — Low: the too-small recovery tests omit a resize that remains below the threshold

**Affected:**

- `Notes/PRD-vrg.md:228`
- `Notes/issues/033-terminal-too-small-with-state-recovery.md:12-20,26-34`
- `Notes/tasks/033-terminal-too-small-with-state-recovery.md:14,19,31`

Task 033 tests shrinking below 20×3 and growing back while preserving selection, viewport, settings, overlays, scroll positions, and pop-up timing. It does not explicitly send another resize while the terminal remains below the threshold before recovery. That transition is a useful boundary because an implementation might accidentally run ordinary relayout against pathological dimensions, mutate anchors, or partially restore modal state on every resize rather than only when the threshold is crossed.

**Recommended correction:** add a sequence such as 19×2 → 10×1 → 25×8 while help, error-over-help, and a nontrivial viewport state are preserved. Assert no ordinary layout installs at 10×1 and recovery uses the final 25×8 dimensions without state loss.

**Ready when:** recovery is tested across at least one resize wholly within the too-small state.

### F7 — Low: the protected literal pattern `--` is not an explicit parser fixture

**Affected:**

- `Notes/PRD-vrg.md:127,131`
- `Notes/issues/002-cli-flag-allow-list-and-child-argv.md:17,30,40-41`
- `Notes/tasks/002-cli-flag-allow-list-and-child-argv.md:14,19,31,51`

The parser tests cover generic `--` handling, a protected `-foo`, the literal `-`, and an empty pattern. They do not explicitly cover a literal pattern whose bytes are themselves `--`, for example `vrg -- --` and `vrg -- -- .`. A parser that continues treating every `--` token as an option terminator could drop or reject the second token despite the rule that all tokens after the first terminator are positional.

**Recommended correction:** add both default-root and explicit-root rows asserting the second `--` is the verbatim pattern and that the child argv contains the mandatory separator followed by a literal `--` pattern.

**Ready when:** the option terminator and a subsequent identically spelled positional are shown to remain distinct.

## Coverage and readiness assessment

| Area | Assessment |
|---|---|
| CLI, roots, flags, child argv | Strong; add the literal-`--` positional edge fixture (F7). |
| Search parsing and integrity | Strong matrix-driven coverage; critique-1 composite and encoding/overlap gaps are fixed. |
| Child lifecycle and diagnostics | Strong model plus PTY boundaries; both cancellation routes now cover replay. |
| Safe presentation | Strong shared design; close Issue 34's explicitly promised sink-row obligation (F5). |
| Navigation, wrapping, panning, graphemes, markers | Strong; complete the held-layout input table for `p` and `w` (F3). |
| Async loading, retries, reload, stale content | Ownership cycles are fixed and Issue 28 covers away-and-back reload arbitration; add the exact all-fail/fixed-2 row (F4). |
| Overlays and too-small recovery | Detailed, but the PRD's observable `Esc` statement must be reconciled (F2); add the within-too-small resize edge (F6). |
| Documentation and limits | Critique-1 coverage gap is fixed; generated footer safety ownership remains incomplete (F5). |
| Final integrated verification | Missing an owner after all shared components and tables compose (F1). |

## Recommended revision order

1. Add a final repository-wide verification task (F1).
2. Resolve the authoritative `Esc` wording before implementation encodes one side of the contradiction (F2).
3. Add the focused RED rows for layout responsiveness, fixed-status/all-fail composition, and Issue 34 sink safety (F3–F5).
4. Add the two small state/parser edge fixtures (F6–F7).
5. Re-run a traceability pass over the updated AC mappings and blocker headers; no dependency redesign is otherwise needed.

After those targeted changes, the task set is ready for implementation. No broader decomposition or PRD-to-issues rewrite is warranted.
