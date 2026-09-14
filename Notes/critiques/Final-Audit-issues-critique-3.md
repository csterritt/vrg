# Final-audit issues critique 3 — VRG

## Scope and verdict

Reviewed:

- `Notes/skills/critique-issues/SKILL.md`.
- `Notes/PRD-vrg.md`, revision 6, in full.
- `Notes/critiques/final-audit-vrg.md`, in full.
- Issues 36–50 in `Notes/issues/`, in full.
- The relevant earlier contracts in Issues 9 and 35.
- `Notes/critiques/Final-Audit-issues-critique-1.md` and `Final-Audit-issues-critique-2.md`, and the revised issue text intended to address them.
- Targeted implementation, test-harness, and smoke-harness locations named by the issues, solely to verify that the proposed boundaries and instructions fit the current repository.

This is an **issues-only review** of the second revision of the post-audit issue set. It evaluates whether Issues 36–50 faithfully and completely convert the final-audit findings into executable work with correct dependencies and acceptance criteria. No PRD, issue, task, or implementation file was changed.

**Verdict: all five of critique 2's findings are resolved in the current text, but one high and three medium findings remain before Issues 36–50 should be converted into tasks.** The blocker is Issue 41: its full-scroll model directly contradicts an existing named acceptance test and Issue 9's head/tail contract, and the issue prescribes no reconciliation — a faithful implementation breaks `go test ./...`, and the obvious workaround silently weakens tail verification. Issues 48, 36, and 39 have scoping gaps that permit literal-but-incomplete closure; the remainder are wording and traceability corrections.

### Severity definitions

- **High:** the issue can be completed literally while the audited defect or a material PRD violation remains, or its stated verification cannot work against the rest of the suite once it lands.
- **Medium:** the intended correction is clear, but dependencies, ownership, scope, or verification are ambiguous enough to permit inconsistent implementation or false closure.
- **Low:** local wording, traceability, or factual inconsistency unlikely to affect product behavior but liable to create review disagreement.

## Findings

### F1 — High: Issue 41 removes the head/tail compression that `TestStderrContentFixture` and Issue 9's acceptance contract encode, without reconciling either

**Affected:**

- `Notes/issues/041-overlay-full-scroll-no-head-tail-compression.md:10-31`
- `Notes/issues/009-error-overlay-and-fatal-outcomes.md:57,70`
- `cmd/vrg/outcome_test.go:381-438`
- `internal/app/app.go:2421-2437`

Issue 41 correctly requires all wrapped rows to remain in the scrollable set. However, the existing `TestStderrContentFixture` writes ≥ 1 MiB of stderr and asserts that the rendered PTY output contains **both** `HEADMARKER` and `TAILMARKER` — while sending only `Esc` and `q`, with no scrolling. That assertion passes today only because the head + ellipsis + tail compression renders both markers in one frame. Issue 9's automated guidance ("the overlay contains the head and tail of the stderr text") and its final acceptance criterion ("the overlay contains its head and tail") encode the same contract.

After Issue 41 lands, the tail sits thousands of wrapped rows below the visible window, reachable only through the modal `up`/`down` keys. The issue names neither the test nor the superseded contract. Three bad outcomes are then available: the implementer leaves the test and it fails; the implementer deletes or weakens the assertion to head-only, silently losing tail verification; or the implementer attempts to scroll ~10,000 rows over a PTY, producing an impractical or flaky test. Each is a false-closure mode the issue should have excluded.

**Required correction:** state explicitly that Issue 9's head-and-tail-in-one-view contract is superseded by full scrollability, and name `TestStderrContentFixture` as requiring revision. Prescribe how tail reachability is verified at scale — e.g. a model-level assertion that the scrollable row set equals the complete wrapped content and that `overlayScroll` clamps to `[0, rows-height]`, plus a bounded `down`-traversal PTY/model test on a *moderately* oversized overlay (rows slightly over `maxVisible`), while the ≥ 1 MiB fixture retains its drainage/completeness assertions without requiring simultaneous head+tail visibility. Also note the shared-file ordering with Issue 48, which rewrites the same harness file.

**Ready when:** a faithful Issue 41 implementation has a defined, practical path to a green suite that still proves tail reachability — without weakening a focused test, per Issue 50's own regression rule.

### F2 — Medium: Issue 48's named scope omits sleep-synchronized PTY helpers that Issue 50's required tests run through

**Affected:**

- `Notes/issues/048-pty-tests-deterministic-handshakes.md:10-31`
- `Notes/issues/050-post-audit-reverification.md:23,38`
- `cmd/vrg/outcome_test.go:107-124` (`runVrgKillChild`, 500 ms per key)
- `cmd/vrg/replay_test.go:281,459` (100 ms "model to settle" delays, outside the named ranges)
- `cmd/vrg/search_test.go:86-99` (`runVrgWithQuit`, 200 ms settle + 100 ms inter-key)

Issue 48 enumerates `outcome_test.go:18-58` and `replay_test.go:176-184, 221-234, 315-326`. Two gaps follow:

1. Within the named files, `runVrgKillChild`'s 500 ms per-key delay and the settle sleeps at `replay_test.go:281` and `:459` sit outside the listed ranges. AC1's "outcome and replay PTY harnesses" wording arguably covers them since they live in those files, but the enumerated list invites a literal partial fix.
2. `runVrgWithQuit` in `search_test.go` — not named anywhere — synchronizes on a fixed 200 ms post-handshake delay plus 100 ms between the two `q` presses. It is used by `TestChildArgvAndWorkdir`, `TestStartFailureExit2`, `TestDualPipeBackpressure`, and `TestStderrCapturedWithoutBlocking` — all four explicitly named in Issue 50's list of tests that "must operate on the Issue 48 handshakes." As written, that requirement cannot be literally satisfied: those tests will still pace keys by elapsed time, and Issue 50's no-sleep acceptance only names the other two files.

**Required correction:** scope Issue 48 by harness/helper rather than file-plus-line-ranges — enumerate every PTY helper that sends keys or assumes a transition (`runVrgWithKeys`, `runVrgKillChild`, `runVrgWithQuit`, `runVrgReplay` trigger callbacks, and any others), or equivalently bind the scope to Issue 50's named test list. Make Issue 50's no-sleep criterion cover all `cmd/vrg` PTY helpers, not only the two named files. (`waitForFile`-style bounded condition polls and `cancel_test.go`'s send-after-ready pattern are already conforming and need no change.)

**Ready when:** every test Issue 50 names can satisfy "operates on handshakes" without unlisted harness code still sleeping, and no fixed delay survives inside the named files under a line-range reading.

### F3 — Medium: Issue 36's fixed component order is defined only for fatal branches; unknown-type placement and diagnostic multiplicity elsewhere are underspecified

**Affected:**

- `Notes/issues/036-stream-integrity-fatal-diagnostics.md:29-37`
- `internal/app/app.go:316-333` (`recordLossDiagnostics` emits malformed → unknown → oversized-paths)
- `internal/app/app.go:286-307` (non-fatal composition)

Issue 36's fixed order — process, integrity causes, record loss, unknown-type warnings — is stated for "every fatal branch." But the unknown-type count currently lives **inside** `recordLossDiagnostics`, between the malformed count and the per-path oversized lines. Moving unknown-type warnings to component 4 in the fatal path therefore requires splitting the shared function — which also reorders the *non-fatal* warning overlay (today: stderr → malformed → unknown → oversized-paths; under the fatal order: stderr → malformed → oversized-aggregate → oversized-paths → unknown). The issue does not say whether the component order is universal or fatal-only, so two composition paths can legitimately diverge — fatal placing unknown last, non-fatal leaving it mid-list — while both pass their own tests.

Two further underspecifications:

- **Unbounded multiplicity.** "Report each violation occurrence individually — no aggregation, no deduplication" means a stream of N orphaned `match` records produces N identical diagnostic lines. At the documented ~100,000-line scale this can flood the overlay and stderr replay. If unbounded duplication is deliberate, say so; otherwise cap or aggregate repeated identical causes.
- **Double representation.** A trailing unterminated record appears both as an integrity line (component 2) and inside the malformed count (component 3); a malformed record after `summary` similarly marks integrity while incrementing the malformed count. Acknowledge the dual representation deliberately or a later "deduplication" cleanup will silently drop a required cause.

**Required correction:** state whether the component order applies to every composed diagnostic or only fatal branches, and define the non-fatal ordering either way (including where the unknown-type count sits relative to Issue 37's oversized aggregate and per-path lines). State the multiplicity bound (or the deliberate absence of one) and acknowledge dual representation where a single record is both an integrity cause and a counted record loss.

**Ready when:** fatal and non-fatal compositions cannot disagree on component order, and a conforming implementation cannot produce either silently aggregated causes or an unbounded identical-line flood without an explicit decision.

### F4 — Medium: Issue 39's enumerated rune-equals-cell sites are incomplete, and one unlisted consumer rests on a false assumption

**Affected:**

- `Notes/issues/039-render-from-shared-grapheme-cell-model.md:12-31`
- `internal/app/app.go:3039` (file-list entry padding via `visibleWidth`)
- `internal/app/app.go:2477-2491` (pop-up width/centering via `visibleWidth`)
- `internal/app/app.go:2529-2548` (`truncateLeftCells`, rune-boundary truncation)

Issue 39's "every width/sizing consumer" clause is correct in principle, but its named list (`renderLineWithHighlights`, `cellToBytePos`, `visibleWidth`, Theme overlay sizing) omits two concrete consumers of the same helpers: file-list padding at line 3039 uses `visibleWidth`, and the file-change pop-up both centers with `visibleWidth` and truncates through `truncateLeftCells` — whose own comment justifies rune-boundary truncation on the claim that `EscapePath` output contains "no combining marks." That claim is wrong: `EscapePath` preserves valid printable Unicode, so a path containing combining marks or wide runes is mis-measured and can be split mid-cluster by the pop-up, even while the file list truncates the same path grapheme-safely via `TruncateLeftGrapheme`. A literal fix to only the four named sites satisfies the enumerated acceptance criteria while leaving pop-up rendering of wide/combining paths broken — the same class of defect the audit found.

**Required correction:** either enumerate every rune/byte-derived width consumer in the render path (at minimum: list-entry padding, pop-up truncation and centering, filename-row fitting) or add a mechanically checkable invariant to the acceptance criteria — e.g. no `utf8.DecodeRuneInString`-based width/truncation loops remain outside the single shared cell-width helper. Correct `truncateLeftCells`'s assumption as part of the fix or route the pop-up through `TruncateLeftGrapheme`.

**Ready when:** no conforming implementation can leave a rune-equals-cell width consumer in place while passing the issue's acceptance criteria.

### F5 — Low: Issue 44 resolves a genuine contradiction inside Issue 9's matrix but leaves the contradictory row in place

**Affected:**

- `Notes/issues/044-post-summary-context-integrity-failure.md:12-30`
- `Notes/issues/009-error-overlay-and-fatal-outcomes.md:26,31`

Issue 9's transition matrix contains both "`context(P)` in any position → Ignored — no lifecycle effect" and "Any record after `summary` → Integrity failure." The implementation followed the first row; the audit and Issue 44 correctly follow the second. The chosen resolution is defensible (a record after `summary` indicates a damaged or concatenated stream), but Issue 44 frames this as correcting an implementation mistake while leaving Issue 9's self-contradictory text as the cited lifecycle authority. A future implementer reading only Issue 9 can legitimately reintroduce the exemption.

**Required correction:** amend Issue 9's context row to scope the exemption to pre-summary positions (or annotate it with a pointer to Issue 44), and have Issue 44 acknowledge it is resolving a contradiction between two Issue-9 rows rather than only fixing code and a test.

**Ready when:** no cited contract still states that post-summary context has no lifecycle effect.

### F6 — Low: Issue 37's prescribed diagnostic text is ambiguous about pluralization

**Affected:**

- `Notes/issues/037-oversized-record-aggregate-anonymous-diagnostics.md:17,30`
- `internal/app/app.go:321-327` (malformed count uses proper singular/plural)

The issue prescribes the literal string `N oversized record(s) skipped`, and AC4 asserts the overlay contains `1 oversized record(s) skipped`. Read literally this bakes a permanent "(s)" into output; read as a template it conflicts with the neighboring malformed line, which pluralizes properly ("1 malformed record skipped" / "N malformed records skipped"). Tests will assert exact text, so the ambiguity is material to implementation.

**Required correction:** pin the exact singular and plural forms (presumably `1 oversized record skipped` / `N oversized records skipped`, matching the malformed convention), or state explicitly that the literal "(s)" form is intended.

**Ready when:** two conforming implementations cannot produce differently spelled aggregate diagnostics.

### F7 — Low: fixture-owned `VRG_TEST_*` names blur Issues 45/50's "no `VRG_TEST_*`" claims

**Affected:**

- `Notes/issues/045-remove-test-hooks-from-production-binary.md:19,34`
- `Notes/issues/050-post-audit-reverification.md:24`
- `cmd/vrg/outcome_test.go:165,306-309`, `cmd/vrg/replay_test.go:135-144`, `cmd/vrg/search_test.go:26-40`, `Notes/walkthroughs/035-03/code-walkthrough/smoke.py:239,320-321`

The fake-rg scripts consume `VRG_TEST_HANDSHAKE`, `VRG_TEST_READY`, `VRG_TEST_PID`, `VRG_TEST_ARGV`, and `VRG_TEST_CWD` through inherited environment — they were never vrg hooks and are correctly absent from Issue 45's hook list. But Issue 50's production smoke "must not set or depend on `VRG_TEST_REAP` or any other `VRG_TEST_*` behaviour," which — read literally — also forbids setting the fixture variables the current smoke relies on. The `FAKE_RG_PID_FILE` example implies a rename off the prefix but does not state it. Meanwhile Issue 45's "probe every former `VRG_TEST_*` variable" needs a defined list, since grepping the test files for the prefix also finds fixture names that were never vrg behavior.

**Required correction:** state in Issue 50 that fixture-owned variables are renamed off the `VRG_TEST_` prefix (e.g. `FAKE_RG_*`) so the claim is unambiguous, and have Issue 45's production-boundary probe enumerate the vrg-consumed hook names (its listed set plus the runner-shape controls) rather than "every `VRG_TEST_*` variable."

**Ready when:** a reviewer can tell which environment names the artifact scan and the smoke prohibition each cover without consulting the test sources.

### F8 — Low: Issue 50's clean-checkout gate has an unstated network precondition and no tagged-build check

**Affected:**

- `Notes/issues/050-post-audit-reverification.md:14-22,37`
- `Notes/issues/045-remove-test-hooks-from-production-binary.md:14-21`

Two gaps in the permanent gate:

1. `go run golang.org/x/vuln/cmd/govulncheck@v1.5.0 ./...` requires fetching the tool (absent a warm module cache) and downloading the vulnerability database. "Runnable from a clean checkout" overpromises on an offline machine; the gate will fail for reasons unrelated to the repository. State the network/module-cache precondition explicitly.
2. `verify.sh` runs only default-configuration commands. The `vrg_testhooks` variant created by Issue 45 is compiled only inside `TestMain`; `go build ./...` and `go vet ./...` never see the tagged files. Test failures would catch a compile break, but tagged-only vet diagnostics and dead-code drift would not be caught early. Add `go build -tags vrg_testhooks ./cmd/vrg` (and optionally `go vet -tags vrg_testhooks ./cmd/vrg`) to the script so both variants are gated.

**Ready when:** the script's environmental requirements are documented and both build variants are exercised by the permanent gate.

### F9 — Low: minor factual and scoping notes

**Affected:**

- `Notes/issues/040-browse-render-no-whole-index-scan.md:22` — the manual verification says to hold `n`/`j`; `j` is not a bound key (the scroll bindings are `up`/`down`, `u`/`d`, `page up`/`page down`). As written the step cannot scroll content; use `n` or `down`.
- `Notes/issues/042-dropped-reload-no-intent-mutation.md:12-16` — the atomic-admission fix should be confined to the `r` path. The navigation re-entry path (`internal/app/app.go:2100-2135`) also mutates state before `startLoad`'s drop check, but legitimately: re-entry changes the current selection, so placeholder presentation and `IntentReveal` are correct updates even though the load is dropped. A sentence scoping the atomicity rule to `handleReload` prevents an over-broad refactor from gating the re-entry contract.
- `Notes/issues/049-tidy-dependency-manifests.md:16` — the removal branch names the PRD *Further Notes* and `Notes/wiki/project-overview.md`; `Notes/wiki/AGENTS.md`'s Scope paragraph also states the Bubble Tea/Bubbles/Lip Gloss stack and will go stale under removal. "At minimum" loosely covers it, but naming it (README.md carries no stack claim) avoids a lingering stale reference.

**Ready when:** each note is either applied or consciously declined; none blocks implementation correctness on its own.

## Coverage and readiness assessment

| Issue area | Assessment |
|---|---|
| 36: stream-integrity diagnostics | **Nearly ready.** Matrix, ordering, escaping, and determinism are strong; component-order scope for non-fatal paths, multiplicity bounds, and dual representation need one clarification pass (F3). |
| 37: oversized aggregate/anonymous diagnostics | **Ready modulo wording.** Pin the aggregate's exact singular/plural text (F6). |
| 38: viewport content-panel width | **Ready.** Verified against `LayoutKey`/`LayoutReadyMsg`; the three-width model matches the code and the compose path's unconditional separator. |
| 39: shared grapheme/cell rendering | **Nearly ready.** Enumerate or make invariant all rune-equals-cell consumers, including the pop-up's incorrect no-combining-marks assumption (F4). |
| 40: render cost | **Ready.** Precompute scope is correct; fix the unbound `j` in the manual step (F9). |
| 41: full overlay scrolling | **Not ready.** The head/tail contract encoded by `TestStderrContentFixture` and Issue 9 must be explicitly superseded with a practical tail-reachability verification (F1). |
| 42: dropped reload | **Ready.** Add the scoping sentence for the navigation re-entry path (F9). |
| 43: combining fallback cell | **Ready.** The HITL decision requirements are complete and `Notes/decisions/` exists for the record. |
| 44: post-summary context | **Nearly ready.** Direction is unambiguous; update Issue 9's contradictory context row for traceability (F5). |
| 45: production/test harness separation | **Ready modulo wording.** The `seams_testhooks.go` filename is safe (not a `_test.go` suffix); define the probe list against vrg-consumed hooks only (F7). |
| 46: runtime replay | **Ready.** The runner-seam and ordering contract are now implementable; optionally restate that the appended runtime error passes through the same sanitization rules. |
| 47: single-line read failures | **Ready.** `PathError` unwrap plus escaped-path construction covers all sites including the `failedPaths` store. |
| 48: deterministic PTY handshakes | **Not ready as scoped.** The `search_test.go` helper and same-file sites outside the named ranges leave fixed delays in tests Issue 50 explicitly requires to run on handshakes (F2). |
| 49: dependency manifests | **Ready.** Ownership of `scripts/verify.sh` is now unambiguous; name `wiki/AGENTS.md` in the removal branch (F9). |
| 50: closing verification | **Nearly ready.** The untagged-smoke and condition-driven-harness corrections from critique 2 landed; document the govulncheck network precondition, add a tagged-build step, and align the no-sleep criterion with F2 (F2, F8). |

## Positive observations

- Every critique-2 finding is verifiably resolved: Issue 45 prescribes the runner-level seam Issue 46 consumes; Issue 46's cleanup order now matches the real `Run()` boundary; Issue 49/50 have single-owner script creation; Issue 50's smoke runs the untagged binary without `VRG_TEST_REAP`; and Issue 45's acceptance wording explicitly permits the inert complementary implementations.
- Issue 36's integrity matrix now covers every Issue 9 lifecycle-failure row with deterministic EOF ordering and mandatory `EscapePath` use — a substantial improvement in precision.
- Issue 38's three-width definitions are internally consistent with the actual code, including the unconditional separator column.
- Issue 50's condition-driven smoke requirements (fixture-owned env names, external process-group observation, EOF-driven draining, no-`time.sleep` static assertion) are specific and implementable.
- The `vrg_testhooks` topology survives scrutiny: the tagged file suffix does not collide with Go's test-file rule, and `TestMain` building with `-tags vrg_testhooks` keeps `go test ./...` exercising the hooked binary automatically.
- All fourteen audit findings still map to exactly one issue each, plus the closing verification pass; no coverage was lost across two revision rounds.

## Required revision order

1. Reconcile Issue 41 with `TestStderrContentFixture` and Issue 9's head/tail contract, defining how tail reachability is proven at scale (F1).
2. Rescope Issue 48 by harness/helper (or by Issue 50's named tests) so no fixed-delay synchronization survives in `search_test.go` or outside the named ranges, and align Issue 50's no-sleep criterion accordingly (F2).
3. Clarify Issue 36's component-order scope for non-fatal compositions, its multiplicity bound, and dual-representation acknowledgment (F3).
4. Complete Issue 39's consumer enumeration or add the grep-able invariant; fix `truncateLeftCells`'s false assumption (F4).
5. Apply the Low corrections: Issue 9's context row (F5), Issue 37's pluralization (F6), fixture env naming (F7), Issue 50's network precondition and tagged-build step (F8), and the three F9 notes.
6. Perform one final issues-only closure check; if these changes are made without expanding scope, Issues 36–50 will be ready for task generation.

No application tests were run because this review evaluates issue quality rather than implementation correctness. Targeted code, tests, and the existing smoke harness were inspected only to verify that the proposed work and dependency boundaries are executable in the current repository.
