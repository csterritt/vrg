# PRD tasks critique 3 — VRG

## Scope and verdict

**Reviewed:** `Notes/skills/critique-tasks/SKILL.md`, `Notes/critiques/PRD-vrg-critique-3.md`, `Notes/PRD-vrg.md` revision 5, all 35 files in `Notes/issues/`, all 35 corresponding files in `Notes/tasks/`, and the two earlier task critiques.

This is a **tasks-only review**. It evaluates whether the issue-derived tasks are complete, executable in their declared order, faithful to the requirements, and backed by enough deterministic and end-to-end verification to produce the expected program. It does not review implementation or add product scope.

**Verdict: the task set is ready for implementation.** No critical or high-severity findings remain. The revisions resolve all findings from task critiques 1 and 2, cover all eight clarifications from `PRD-vrg-critique-3.md`, preserve an acyclic ownership graph, and add a credible final integration gate. The remaining findings are documentation and acceptance-traceability cleanups; the tasks themselves already specify the correct behavior in each case.

The plan is unusually strong in the areas most likely to fail in a terminal application: child termination and reaping, terminal restoration, dual-pipe backpressure, diagnostic ordering, raw-byte path identity, malformed and oversized stream records, asynchronous load/layout isolation, width-independent anchors, grapheme-safe clipping, stale-content fallback, modal precedence, and too-small recovery. RED/GREEN work is consistently ordered, asynchronous tests use gates and identities rather than sleeps, and Issue 35 verifies the fully assembled repository rather than relying on previously green slices.

### Severity definitions

- **Critical:** the declared graph cannot be executed without guessing ownership or violating a dependency.
- **High:** the plan can complete while materially incorrect program behavior remains unimplemented or unverified.
- **Medium:** a requirement is implemented by the tasks but omitted or contradicted by the issue-level closure contract, creating a meaningful traceability risk.
- **Low:** local documentation, mapping, or operational-hygiene inconsistency unlikely to alter the implementation.

## What is now resolved

All findings from `PRD-tasks-critique-1.md` and `PRD-tasks-critique-2.md` are resolved:

- Issue 26 owns the initial append-preserving-scroll primitive, and Issue 32 clearly generalizes it.
- Issue 27 owns a tested reload-anchor pending-intent seam, and Issue 28 generalizes it into reveal-versus-reload arbitration.
- Task 034 tests the scale, record-limit, base64, memory, termination, help-footer, and sink-safety statements.
- Task 011 covers diagnostic replay through both `ctrl+c` and `q` cancellation at model and PTY boundaries.
- Task 010 names both malformed-plus-integrity composite records.
- Task 018 tests re-clamping on wrap re-entry and every pan.
- Task 003 tests mixed `text`/`bytes` identity and overlapping-submatch union behavior.
- Task 026 uses a disposable permission fixture with restoration.
- Task 002 tests a positional whose literal bytes are `--`.
- Task 017 keeps `p` and `w` actionable while layout preparation is held.
- Task 026 tests the composed fixed-status-2/all-files-fail outcome.
- Task 033 tests a resize wholly within the too-small state.
- The observable `Esc` rule is reconciled: it never exits a base state, while dismissing a fatal overlay exits 2 because no underlying state exists.
- Issue 35 now owns clean repository-wide build, vet, test, no-cache PTY/subprocess reruns, and final-binary smoke outcomes 0, 1, 2, and 130.

The requirements-critique-3 clarifications are also represented in executable work:

| Requirement clarification | Issue/task ownership |
|---|---|
| Load completion uses the normal destination-reveal algorithm; first visit starts at top | Issue/Task 028 |
| Late non-current diagnostics have deliberately limited mid-session visibility | Issue/Task 026 documentation and behavior |
| ~50 MB assumes UTF-8/ordinary lines; base64 can exceed 64 MiB; recoverable path is named | Issues/Tasks 010 and 034 |
| Duplicate `r` is dropped, placeholder settlement is the signal, and one-stop retry friction is accepted | Issue/Task 027 |
| Too-small recovery preserves help/error scroll and modal relationships | Issue/Task 033 |
| Terminator-only CRLF marker follows ordinary marker rules | Issue/Task 023, with async integration in 028/029 |
| `Esc` is documented and is a no-op in base states | Issues/Tasks 031–033 |
| File-change pop-up recentres and re-truncates on resize | Issue/Task 015 |

## Findings

### F1 — Medium: several issue acceptance lists omit requirements that their tasks correctly implement

**Affected:**

- `Notes/issues/024-file-list-layout-width-truncation-toggle.md:19,30-38`
- `Notes/tasks/024-file-list-layout-width-truncation-toggle.md:14,19,31`
- `Notes/issues/027-explicit-reload-r.md:18,30-37`
- `Notes/tasks/027-explicit-reload-r.md:14,20,32`

Issue 24's body requires the filename row's buffer-status note slot, including path truncation to leave room for a synthetic status. Its RED and GREEN tasks explicitly test and implement that slot, but AC1–AC7 never mention it. Issue 27 similarly requires the filename row to keep identifying the path during reload, and Task 027-1 explicitly tests it, but AC1–AC6 omit it.

The implementation plan is complete, so these are not missing-code findings. The risk is closure by acceptance checklist: an issue could appear accepted while one of its body requirements failed, or a later editor could remove the task clause as apparently unmapped work.

**Recommended correction:** add an Issue 24 acceptance criterion for the buffer-status slot and an Issue 27 acceptance criterion for path identity during reload. Update the task-file AC mappings to include the new criterion numbers without changing task contents.

**Ready when:** every behavior in these issues' “What to build” sections that already has a RED test is also represented in the numbered acceptance list.

### F2 — Low: Issue 18 AC6 is narrower than the authoritative task and PRD re-entry rule

**Affected:**

- `Notes/issues/018-horizontal-panning.md:29-34,43-53`
- `Notes/tasks/018-horizontal-panning.md:14,19,31`
- `Notes/PRD-vrg.md`, Navigation, viewport, and logical anchors

The issue body and Task 018 correctly require the horizontal maximum to be recomputed on every return to run-off-edge mode, including when the visible line set changed while wrapped. AC6 says `w` then `w` retains the offset with “no intervening resize” and mentions clamping only when width changed. A user can scroll while wrapped without resizing, changing the visible extents and requiring a clamp on re-entry.

Task 018's RED and GREEN instructions are correct and explicitly test that case, but the weaker AC wording can make the test appear stricter than the accepted product behavior.

**Recommended correction:** change AC6 to retain the offset only when neither width nor the visible line set changed enough to require clamping; state that every run-off-edge re-entry recomputes the maximum from current visible rows.

**Ready when:** the acceptance criterion and Task 018 describe the same unconditional re-entry clamp rule.

### F3 — Low: Issue 26's manual instructions retain the unsafe permission-change wording fixed in its task

**Affected:**

- `Notes/issues/026-read-failures-unreadable-retry-rules.md:26-29`
- `Notes/tasks/026-read-failures-unreadable-retry-rules.md:69-75`

Task 026-6 correctly requires a disposable temporary fixture and a restoration trap so `chmod 000` cannot leave repository or user files unreadable. The parent issue still says simply to make the second matched file unreadable. Someone following the issue's manual section rather than the task could alter a real file and fail to restore it after interruption.

**Recommended correction:** copy Task 026-6's disposable-fixture and cleanup requirements into the issue's Manual section. Keep the injected loader as the deterministic authority.

**Ready when:** neither issue nor task directs a reviewer to change permissions on a non-disposable file.

### F4 — Low: Task 002's documentation step does not explicitly preserve its literal-`--` parser edge case

**Affected:**

- `Notes/tasks/002-cli-flag-allow-list-and-child-argv.md:14,19,31,35-41,45-51`

The RED, GREEN, and walkthrough tasks now correctly distinguish the first `--` option terminator from a subsequent positional whose bytes are also `--`. The documentation task says to document `--`, dash-leading patterns, literal `-`, and the empty pattern, but does not explicitly require the literal-`--` positional case. This is a subtle parser rule and is precisely the kind of edge behavior likely to disappear from prose if not named.

**Recommended correction:** add the literal-`--` positional rule and examples `vrg -- --` / `vrg -- -- .` to Task 002-3's documentation requirements.

**Ready when:** implementation tests, walkthrough, and wiki-ingest instructions all preserve the distinction between the first terminator and a later identically spelled positional.

## Coverage and readiness assessment

| Area | Assessment |
|---|---|
| CLI, roots, flags, child argv | Strong. Exact argv, combined shorts, cumulative `-u`, protected operands, empty patterns, and literal `--` are tested. Add the low-priority documentation clause in F4. |
| Search parsing and stream integrity | Strong matrix-driven plan. Malformed, oversized, unknown, lifecycle, binary-exclusion, and composite dispositions have deterministic fixtures. |
| Child lifecycle and diagnostics | Strong model plus PTY coverage. Cancellation keys, reaping evidence, terminal modes, replay order, and exactly-once behavior are explicit. |
| Safe presentation | Strong shared sink table extended by every later sink, including replay, pop-ups, overlays, list/filename rows, placeholders, and help footer. |
| Navigation, wrapping, panning, graphemes, markers | Strong. Target geometry, anchors, prepared layouts, cluster boundaries, marker extents, indicators, and async reveal are covered. Align Issue 18 AC6 per F2. |
| Async loading, failures, reload, stale content | Strong and executable. Earlier ownership cycles are resolved, latest-selection arbitration is gated, and obsolete work cannot consume intents. Add acceptance traceability per F1. |
| Overlays, pop-ups, help, too-small recovery | Strong. Observable `Esc` semantics, append/suspension behavior, scroll restoration, resize behavior, and timer isolation are explicit. |
| Documentation and limits | Strong synchronization tests and explicit resource qualifications; no unsupported capacity guarantee is implied. |
| Final integrated verification | Strong. Issue 35 blocks transitively on every feature and requires clean build/vet/test plus uncached boundary tests and final-binary smoke outcomes. |

## Recommended cleanup order

1. Add the missing acceptance criteria for Issues 24 and 27 and update their task mappings (F1).
2. Align Issue 18 AC6 with its already-correct task contract (F2).
3. Make Issue 26's manual permission fixture disposable and self-restoring (F3).
4. Add the literal-`--` edge case to Task 002's documentation step (F4).

These edits improve traceability and operational safety but do not require task decomposition, blocker changes, or additional implementation work. The current task graph is ready to execute, and Issue 35 provides sufficient final evidence that the resulting program builds, runs, and produces the expected outcomes.
