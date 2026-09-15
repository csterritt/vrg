# Post-final-audit tasks critique 3 — VRG

## Scope and verdict

**Reviewed:**

- `Notes/skills/critique-tasks/SKILL.md`.
- `Notes/PRD-vrg.md`, revision 6, in full.
- `Notes/critiques/final-audit-vrg.md` as the source audit for Issues 36–50.
- `Notes/critiques/PRD-Post-FA-tasks-critique-1.md` and `Notes/critiques/PRD-Post-FA-tasks-critique-2.md` as the prior critiques whose findings this revision cycle was meant to resolve.
- Issues 36–50 in `Notes/issues/`, in full, plus Issue 9's amended lifecycle matrix and `internal/searchindex/searchindex.go:487-526` / `lifecycle_test.go:84-97` to confirm the current `context` exemption and test row.
- Corresponding Tasks 36–50 in `Notes/tasks/`, in full.
- `Notes/Ideas.md`, `Notes/decisions/001-cli-scaffold-and-output-architecture.md`, and the relevant wiki pages (`AGENTS.md`, `outcome-contract.md`, `final-verification.md`).

This is a **tasks-only review**. It evaluates whether Tasks 36–50 remain complete against their parent issues and the PRD, executable in their declared order, and backed by deterministic verification sufficient to produce and verify the expected program. It does not review the implementation or add product scope. No PRD, issue, task, or implementation file was changed.

**Verdict: nearly ready — apply one ownership/ordering repair and four small amendments before implementation; no structural rework is needed.** Every critique-2 finding is resolved: Task 36-1/36-3 now pin one-integrity-cause-per-record precedence with exact composed-output rows for every overlap; Task 40-1's cost guard spans a combined navigation `Update()`+`View()`; Task 36's header maps all ten ACs; Task 39-1 gives the static guard an exact allow-list predicate; Task 37-1 pins per-distinct-path deduplication; and Task 50-2 gives the smoke harness a canonical home at `scripts/smoke.py` while leaving the 035-03 artifact frozen. The remaining findings are narrower: the `context`-after-`summary` exemption and its lifecycle test row have ambiguous ownership between Tasks 36 and 44 (F1), Issue 46 lacks the symmetric shared-harness ordering rule Issues 41/48 carry for the same `cmd/vrg` PTY files (F2), and three low-severity specification gaps.

### What is strong

- Task 36-1's precedence table is exact and testable: at most one cause per physical record, second-`summary` precedence, post-summary lifecycle suppression, and the post-summary unterminated-fragment rule are all pinned with exact-list RED rows; Task 36-3 asserts complete ordered line slices rather than presence of acceptable lines.
- Task 37-1 resolves the detail-line multiplicity question deterministically — per-record aggregate, per-distinct-raw-path details in first-occurrence order — with a duplicate-path RED case.
- Task 38-2 names every install site that recomputes the terminal-derived width (`LayoutReadyMsg`, `buildViewport` factory-seam and cache-hit installs, `WindowSizeMsg`), going beyond the audit's two cited ranges.
- Task 40-1's access counter is shared across `Update()` and `View()` without reset, so moving the whole-index scan from render into navigation still fails — exactly the evasion critique-2 warned about.
- Task 45-1 proves both halves of the build topology in RED, and Task 48-1's finite helper/action/postcondition/acknowledgement matrix with per-occurrence correlation satisfies the earlier handshake concerns.
- Task 50 is a credible closing gate: ten ordered `verify.sh` gates, named uncached/race PTY reruns, `FAKE_RG_*` rename separated from the vrg-consumed hook manifest, and a condition-driven smoke harness against the untagged binary.

### Severity definitions

- **Critical:** the declared task graph cannot be executed for an allowed branch without guessing ownership, violating a dependency, or claiming an impossible output.
- **High:** the plan can complete while a material PRD or issue behavior remains absent or incorrectly implemented.
- **Medium:** an explicit acceptance boundary lacks deterministic coverage, or two tasks have ambiguous shared ownership/order such that each can be implemented per spec yet leave a red suite or unowned work.
- **Low:** local traceability, specification precision, or guard-scope weakness unlikely to change the architecture.

## Findings

### F1 — Medium: The `context`-after-`summary` exemption and its lifecycle row have ambiguous ownership between Tasks 36 and 44

**Affected:**

- `Notes/tasks/036-stream-integrity-fatal-diagnostics.md:19,31`
- `Notes/tasks/044-post-summary-context-integrity-failure.md:19,31`
- `Notes/issues/044-post-summary-context-integrity-failure.md:4`
- `internal/searchindex/searchindex.go:508`, `internal/searchindex/lifecycle_test.go:96-97`

Task 36-2 requires enforcing the post-summary precedence table "before dispatching post-summary records to lifecycle parsers: … every other post-summary record records only `record after summary`". The current code exempts `context` at `searchindex.go:508` (`b.sawSummary && rec.Type != "context"`). Implementing Task 36 literally therefore removes the exemption Issue 44 was created to remove — and flips the existing `lifecycle_test.go` row (`"context after summary has no lifecycle effect"`, `wantComplete: true`) to failing. Yet Task 44-1 claims that exact row correction as its own RED and asserts it "fails on the current code, where `Add` exempts `context`" — a premise that is stale in every allowed execution order, because Issue 44 is hard-blocked by Issue 36.

Whichever way the implementer resolves this, something written down is wrong: fix the row inside Issue 36 (doing Task 44's owned work, and Task 44-1's RED is then green on arrival and Task 44-2's "remove the exemption" is a no-op), leave the suite red at Issue 36's close (violating Task 36-2's own `go test ./...` verification, with the repair owned by an issue that has not started), or preserve the exemption through Issue 36 (contradicting Task 36-1's explicit precedence rule). Issue 44's own text even notes "the parser change itself can be developed first" — the task file's blanket "Begin only after Issue #36 is complete" over-tightens this into the collision.

**Recommended correction:** pick one ownership split and write it down. The cleaner option: Task 36 owns removing the exemption and updating the contradictory lifecycle row as part of its GREEN (the row is a test encoding the behavior Task 36's spec deliberately changes), and Task 44-1 is restated to acknowledge the failure is already implemented — its residual deliverables being the dedicated `context`-after-`summary` matrix row, the outcome-level composed-diagnostic assertion, and documentation. The alternative — letting Task 44-1/44-2's parser fix land before or alongside Issue 36 while keeping only the outcome assertion gated on it — also works, but requires relaxing Task 44-1's blocker wording. Either way, `go test ./...` must be green at the end of Issue 36 without anyone guessing who owns the row.

**Ready when:** exactly one task owns the `context` exemption removal and the `lifecycle_test.go` row correction, the other task's text acknowledges the state it will actually find, and no execution order leaves a red suite at an issue boundary.

### F2 — Medium: Task 46 has no ordering rule with Issues 41/48 over the shared `cmd/vrg` PTY harness

**Affected:**

- `Notes/issues/046-runtime-error-common-diagnostic-replay.md:4`
- `Notes/tasks/046-runtime-error-common-diagnostic-replay.md:19`
- `Notes/issues/041-overlay-full-scroll-no-head-tail-compression.md:4`, `Notes/tasks/041-overlay-full-scroll-no-head-tail-compression.md:5`
- `Notes/tasks/048-pty-tests-deterministic-handshakes.md:5,31`

Issues 41 and 48 carry an explicit, symmetric shared-file ordering rule for `cmd/vrg/outcome_test.go`: never concurrent, and whichever lands second adapts to the other's harness. Issue 46 is blocked only by Issue 45 — same as Issue 48 — and its Task 1 adds new subprocess/PTY tests to the same `cmd/vrg` test package, driving a real PTY lifecycle, with no statement about which helpers it may use or whether fixed delays are permitted.

The asymmetry is one-directional: Task 48-2 says "If Issue #46's new subprocess tests exist, they use these helpers too" (48 adapts to 46), but nothing on the 46 side says what to do if 48 has landed first. An implementer executing Task 46-1 after Issue 48 could write the return-shape tests with `time.Sleep` settling delays or a new unacknowledged helper — reintroducing exactly the synchronization Issue 48's acceptance criteria and Issue 50's closing gate forbid. The failure is caught eventually (Issue 50's "no fixed settling delay in any helper" check), but the regression-to-owning-issue rule then attributes a failure to a contract Task 46 never stated. There is a secondary overlap with Issue 41, which rewrites `TestStderrContentFixture` in the same file.

**Recommended correction:** mirror the 41/48 clause on the 46 side — a shared-file/harness ordering rule covering `cmd/vrg` PTY helpers, stating that if Issue 48 has landed, Task 46's tests must use the handshake harness and add no fixed settling delays (and add any new key-sending helper to Task 48's matrix and Issue 45's hook manifest); if Issue 46 lands first, Task 48's existing adaptation clause already covers the reverse direction. Note the `outcome_test.go` adjacency with Issue 41 as well.

**Ready when:** every allowed landing order for Issues 41, 46, and 48 leaves the no-fixed-delay invariant intact and assigns adaptation duty to whichever issue lands second — with no gap where Task 46's new tests are written unaware of the handshake harness.

### F3 — Low: Issue 36's cause-precedence rule exists only in the tasks, not in the issue's contract

**Affected:**

- `Notes/issues/036-stream-integrity-fatal-diagnostics.md:17-31`
- `Notes/tasks/036-stream-integrity-fatal-diagnostics.md:19,43`
- `Notes/critiques/PRD-Post-FA-tasks-critique-2.md:53` (F1 recommended correction)

Critique-2's F1 recommended adding the overlap rule "to Issue 36 (and mirror it in Task 36-1/36-3's RED)". The tasks now carry the full rule — at most one cause per physical record, most-specific row wins, post-summary records not lifecycle-processed — but Issue 36's cause matrix still lists the overlapping rows ("a second `summary`" *and* "any record after `summary`") with no precedence statement, and its dual-representation clause still leaves "one physical record, two violations" undecided at the issue level. The tasks are the contract the implementer executes, so behavior is pinned; but the issue — the artifact later critiques, audits, and regenerations diff against — remains nominally ambiguous in exactly the way the critique found defective.

**Recommended correction:** add the precedence paragraph to Issue 36's "What to build" (or record it in a `Notes/decisions/` entry the issue references), matching the task wording: each physical record contributes at most one integrity cause chosen by the most specific applicable row; post-`summary` records produce the after-`summary` cause only and do not mutate lifecycle state; a post-`summary` unterminated fragment produces the after-`summary` cause plus the malformed count.

**Ready when:** the issue's matrix and the tasks' RED tables cannot be read to disagree, and an auditor reading only Issue 36 reaches the same cause set as an implementer reading only Task 36.

### F4 — Low: Dual representation is unpinned for post-`summary` oversized and unknown-type records

**Affected:**

- `Notes/issues/036-stream-integrity-fatal-diagnostics.md:29,31`
- `Notes/tasks/036-stream-integrity-fatal-diagnostics.md:19,31,43`
- `Notes/issues/009-error-overlay-and-fatal-outcomes.md:31` ("also skipped/counted if the record is itself malformed")

The dual-representation rule is pinned for exactly two record kinds: the post-`summary` unterminated fragment and malformed records after `summary` (integrity cause + malformed count). Two neighboring cases are not covered by any required assertion:

- A post-`summary` **oversized** record (> 64 MiB, consumed through the next newline). It is simultaneously a record after `summary` and an oversized record. Does it produce `record after summary` + a count toward the oversized aggregate, or only one representation?
- A post-`summary` **unknown-type** record. Unknown types are counted and warned "without independently changing the exit status" — but a post-`summary` position *is* an integrity violation. Both, or only the cause?

The natural reading is "both" in each case (counting is positional-agnostic; the after-`summary` check is type-agnostic), but the entire purpose of Issue 36 is deterministic, exactly-specified output, and no RED row pins either case — the same genus of ambiguity critique-2's F1 repaired for `summary`/`begin`/unterminated overlaps.

**Recommended correction:** extend the precedence/dual-representation rule one sentence — e.g. "a post-`summary` record that is independently counted (oversized, unknown-type) produces the `record after summary` cause and retains its count" — plus one exact-line RED row for each case in Task 36-1 or 36-3. If the intended answer differs, state it instead; the point is that the matrix decides it, not the implementer.

**Ready when:** every record kind that can appear after `summary` — `begin`/`match`/`end`/`context`/`summary`, malformed, unterminated, oversized, unknown-type — has an asserted cause-and-count outcome.

### F5 — Low: Task 39's mechanical guard does not cover the other display-geometry packages

**Affected:**

- `Notes/tasks/039-render-from-shared-grapheme-cell-model.md:19,31`
- `Notes/issues/039-render-from-shared-grapheme-cell-model.md:17,33`

The static guard scans production `.go` files under `internal/app`, `internal/theme`, and `internal/safepresentation` and permits `utf8.DecodeRuneInString` only in `cellwidth.go`. That matches Issue 39's AC6 scope ("any consumer of display width in the final render path"), and it correctly resolves critique-2's F4 predicate ambiguity. But the PRD's single grapheme/cell policy also binds `internal/viewport` (wrap/clip/cell mapping) and `internal/filebuffer` (cluster construction) — packages that perform exactly the display-geometry work the guard exists to protect. A rune-decoding width or truncation loop introduced there later evades the scan entirely while violating the same contract.

**Recommended correction:** widen the scan to all non-test production `.go` files under `internal/` (and `cmd/`) with the same single allow-list entry — the predicate stays mechanically decidable — or state in the task why the guard is intentionally scoped to the final render path only. If widening, confirm no existing legitimate decoder in `viewport`/`filebuffer` trips the allow-list (the same `for range`/stdlib rewrite rule applies).

**Ready when:** the guard's package list either covers every package that can consume display width, or the narrower scope is a recorded decision rather than an accident of where the defect happened to be found.

## Coverage and readiness assessment

| Area | Assessment |
|---|---|
| Integrity and diagnostic composition (36–37, 44) | **Strong, minus F1/F3/F4.** Exact-line composition, precedence, ordering, dedup, and replay parity are pinned; the `context` exemption ownership and the two unpinned post-`summary` count cases are the residual gaps. |
| Panel width and Unicode rendering (38–39, 43) | **Ready.** All terminal-derived install sites named; cluster-driven renderer with an exact guard predicate (F5 is a scope nit); HITL fallback decision properly gated and recorded. |
| Bounded browse rendering (40) | **Ready.** Combined `Update()`+`View()` access bound prevents scan relocation; width-independent metadata and current-width truncation correctly separated. |
| Overlay scrolling and reload admission (41–42) | **Ready.** Complete-row scroll set, clamp bounds, ignored modal keys, append behavior, and both sides of the admission boundary fully specified. |
| Production seams and runtime shutdown (45–46) | **Nearly ready.** Topology, manifest, and tagged-runner reachability are solid; add the 46↔48 (and 41) harness-ordering clause per F2. |
| Read-failure diagnostic safety (47) | **Ready.** Deterministic disposable fixtures, gate-synchronized real failures, uniform construction at all three load sites. |
| PTY handshakes (48) | **Ready.** Finite matrix, per-occurrence correlation, bounded-timeout failure, manifest/artifact-probe extension; F2 covers only the missing reciprocal clause on Task 46. |
| Dependency manifests (49) | **Ready.** Recorded removal decision, linear graph, `verify.sh` ownership deferred to Issue 50. |
| Closing verification (50) | **Ready.** Canonical `scripts/smoke.py`, frozen 035-03, `FAKE_RG_*` rename, ten ordered gates, named reruns, untagged-binary smoke, owning-issue regression rule. |

## Recommended revision order

1. Resolve the `context`-exemption ownership between Tasks 36 and 44 (F1) — it is the only finding that can produce a red suite or unowned work at an issue boundary.
2. Add the symmetric harness-ordering clause to Task 46 (F2) — cheap to write now, expensive to untangle after a sleep-bearing test lands.
3. Mirror the precedence rule into Issue 36's text or a decisions entry (F3); pin post-`summary` oversized/unknown-type dual representation with RED rows (F4).
4. Decide Task 39's guard scope (F5) — widen the package list or record the intentional narrowing.
5. Recheck task-header blockers after the edits — none should change — then proceed with implementation.

All critique-1 and critique-2 findings are resolved; the remaining items are local amendments rather than a regeneration. After these corrections, Tasks 36–50 are ready to execute. No application tests were run because this was a plan/task critique; the requested output is the review of whether the tasks are sufficient to guide and verify the later implementation.
