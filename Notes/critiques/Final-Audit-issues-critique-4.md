# Final-audit issues critique 4 — VRG

## Scope and verdict

Reviewed:

- `Notes/skills/critique-issues/SKILL.md`.
- `Notes/PRD-vrg.md`, revision 6, in full.
- `Notes/critiques/final-audit-vrg.md`, in full.
- Issues 36–50 in `Notes/issues/`, in full (third revision).
- The amended contracts in Issues 9 and 35.
- `Notes/critiques/Final-Audit-issues-critique-1.md`, `Final-Audit-issues-critique-2.md`, and `Final-Audit-issues-critique-3.md`, and the revised issue text intended to address them.
- Targeted implementation, test-harness, and smoke-harness locations named by the issues, solely to verify that the proposed boundaries and instructions fit the current repository.

This is an **issues-only review** of the third revision of the post-audit issue set. It evaluates whether Issues 36–50 faithfully and completely convert the final-audit findings into executable work with correct dependencies and acceptance criteria. No PRD, issue, task, or implementation file was changed.

**Verdict: every one of critique 3's nine findings is verifiably resolved in the current text, and the set is close to task-ready.** Three medium findings remain — all are scope/interaction gaps rather than wrong direction: Issue 38 names only one of the three call sites carrying the wrong width formula, Issue 39's consumer enumeration still omits the overlay text-wrapping loop, and Issues 45/46 collide on the single-callback `WithOnCollect` seam. Six low findings cover ordering, environment, and documentation details. No high-severity false-closure path remains: no issue can now be implemented literally while leaving its audited defect in place under a green suite.

### Severity definitions

- **High:** the issue can be completed literally while the audited defect or a material PRD violation remains, or its stated verification cannot work against the rest of the suite once it lands.
- **Medium:** the intended correction is clear, but dependencies, ownership, scope, or verification are ambiguous enough to permit inconsistent implementation or false closure.
- **Low:** local wording, traceability, or factual inconsistency unlikely to affect product behavior but liable to create review disagreement.

## Findings

### F1 — Medium: Issue 38 names the audited `LayoutReadyMsg` site but not the two sibling sites carrying the identical wrong formula

**Affected:**

- `Notes/issues/038-viewport-content-panel-width.md:12-24,35-39`
- `internal/app/app.go:1417` (named site, `LayoutReadyMsg`)
- `internal/app/app.go:1660` (resize path when `rowProviderFactory` is installed)
- `internal/app/app.go:1903` (`buildViewport` synchronous factory-seam install)
- `internal/app/app.go:1936` (cache-hit install — already correct, uses `key.TextWidth`)

The issue correctly diagnoses that `LayoutReadyMsg` installation recomputes `viewport.TextWidth(m.width, ...)` from terminal width instead of using `msg.Key.TextWidth`. But the same `viewport.TextWidth(m.width, ...)` expression appears at two more call sites the issue never mentions:

- `buildViewport`'s `rowProviderFactory` branch (line 1903) installs a viewport with the full-terminal formula.
- The `WindowSizeMsg` resize path (line 1660) re-`SetLayout`s the existing viewport with the same wrong formula whenever the factory seam is installed and a buffer is loaded.

Both are reachable whenever `rowProviderFactory` is set — which is exactly the seam the existing render-cost and horizontal tests run through. Meanwhile the cache-hit path (line 1936) already uses `key.TextWidth`, so the *same* row model is installed with different widths depending on whether it came from fresh preparation or the layout cache — a further symptom the issue does not mention.

A literal implementation that fixes only the named site leaves the seam-installed viewport mis-measuring; new Issue-38 tests written through that seam would either fail spuriously or be "corrected" to the wrong geometry. This is how the defect was masked originally (the issue itself notes the horizontal tests encode the wrong calculation), so the seam paths are part of the defect surface, not incidental.

**Required correction:** require every viewport width installation and update to derive text width through the panel-width computation (`LayoutKey`'s `panelWidth` chain), and name all four `SetLayout` call sites: 1417 (the fix), 1660 and 1903 (same formula under the factory seam), and 1936 (already correct). State explicitly whether `rowProviderFactory` remains a supported seam — if so it must follow the same width rule; the strongest resolution is a single shared "current text width" helper so no site can recompute from `m.width` again.

**Ready when:** no conforming implementation can leave a `TextWidth(m.width, ...)` call in any install or resize path, including seam-only ones.

### F2 — Medium: Issue 39's consumer enumeration still omits the overlay text-wrapping loop

**Affected:**

- `Notes/issues/039-render-from-shared-grapheme-cell-model.md:14-17,24,33`
- `internal/app/app.go:2557-2606` (`wrapText`/`wrapLine` — rune-decode wrap loop over `visibleWidth`)
- `internal/app/app.go:2612-2614` (`centerText` via `visibleWidth`)

Critique 3's F4 asked for the enumeration to be completed or a grep-able invariant added; the revision did both for width/truncation consumers but still misses one display-geometry loop. `renderOverlay` wraps diagnostic and help text through `wrapText`/`wrapLine`, which counts columns with `utf8.DecodeRuneInString` and inserts line breaks at rune boundaries (line 2596-2603). For wide or combining overlay text this both mis-measures row widths *and* can break a line mid-cluster — a behavior defect of the same class the audit found, in a different verb (wrapping rather than truncating or sizing).

The mechanical invariant ("no `utf8.DecodeRuneInString`-based display-width or truncation loop may remain outside the single shared grapheme/cell helper") arguably catches `wrapLine`, but the wording names "display-width or truncation" loops; a wrap-insertion loop is neither literally, and the enumerated consumer list ("line rendering … `visibleWidth`; file-list entry padding; indicator-column sizing; filename-row fitting; Theme overlay sizing; pop-up truncation, width, and centering") never names overlay/help text wrapping. `centerText` needs no separate mention only because it delegates to `visibleWidth`.

**Required correction:** add overlay/help text wrapping (`wrapText`/`wrapLine`) to the enumerated consumers and require wrap boundaries to fall on grapheme-cluster boundaries measured in cells, consistent with the viewport's own wrap rule. Broaden the invariant's wording to cover any rune-decode loop that measures, truncates, or breaks display text — e.g. "no `utf8.DecodeRuneInString`-based display-geometry loop outside the shared helper."

**Ready when:** a conforming implementation cannot leave a rune-boundary wrap of overlay text in place while passing the issue's acceptance criteria.

### F3 — Medium: Issues 45 and 46 collide on the single-callback `WithOnCollect` option

**Affected:**

- `Notes/issues/046-runtime-error-common-diagnostic-replay.md:14`
- `Notes/issues/045-remove-test-hooks-from-production-binary.md:17-19,34`
- `internal/app/app.go:773-781` (`WithOnCollect` assigns `c.onCollect = f` — last-wins, single callback)
- `cmd/vrg/main.go:89,157-167` (`opts` built first, test-seam options appended after)

Issue 46 requires a diagnostic snapshot "owned by `runSearch` … in the same option family as `WithOnCollect`," so the session collector survives an invalid/nil final model. That means production wiring will register an always-on `onCollect` callback. Issue 45's tagged seam, however, keeps `VRG_TEST_COLLECT_ACK` — which also wires `WithOnCollect` — and its `testSeamOptions(proc)` results are *appended after* the production options. Since the option stores a single function, whichever registration lands last wins outright:

- If the tagged seam wins, `runSearch`'s snapshot stays empty in exactly the tagged-binary subprocess tests that assert replay order — or an implementer "fixes" this by reading `m.Diagnostics()` when the snapshot is empty, reintroducing the final-model dependency the issue exists to remove.
- If the production collector wins, the ack file goes silent and the replay tests that wait for ack lines hang until timeout.

Neither issue mentions the interaction. The fix is small — support multiple callbacks, compose them inside the option, or have `runSearch`'s collector fan out to the test ack — but it must be specified, because the two issues are implemented by different passes with a blocker between them.

**Required correction:** state in Issue 46 (or 45) that the always-on snapshot collector and the test-only `VRG_TEST_COLLECT_ACK` acknowledgement must compose rather than replace each other — e.g. `WithOnCollect` accepts/accumulates multiple callbacks, or the production collector invokes the ack sink. Specify option ordering so tagged and production wiring cannot silently displace each other.

**Ready when:** a tagged binary with `VRG_TEST_COLLECT_ACK` set feeds both the snapshot and the ack file, and no implementation choice in Issue 46 can drop one.

### F4 — Low: Issue 36's end-of-stream ordering does not position a post-`summary` unterminated record

**Affected:**

- `Notes/issues/036-stream-integrity-fatal-diagnostics.md:26-31`
- `internal/searchindex/searchindex.go:492-510` (an unparseable record after `summary` already sets `afterSummary`)

The issue's EOF-discovered ordering is "missing `end`s (raw-path order) → missing `summary` → trailing unterminated record." A trailing unterminated record that arrives *after* a `summary` satisfies both the "record after `summary`" row (the current code sets `afterSummary` for unparseable post-summary records) and the "trailing unterminated record" row. The issue's dual-representation rule covers integrity-cause + malformed-count, but not integrity-cause + integrity-cause: it is undefined whether that record produces one line ("unterminated final record"), two lines ("record after `summary`" and "unterminated final record"), and — if two — where the after-`summary` line sits, since it is only discoverable at EOF and is absent from the EOF ordering.

**Required correction:** state the treatment explicitly — most plausibly that a trailing unterminated record produces exactly the "unterminated final record" cause regardless of position, or that it produces both causes with a defined order. One sentence suffices.

**Ready when:** two conforming implementations cannot emit different cause sets for a `summary`-then-garbage-tail stream.

### F5 — Low: Issue 50's `verify.sh` tagged-build step writes a `vrg` binary into the repo root

**Affected:**

- `Notes/issues/050-post-audit-reverification.md:17,40`

`go build -tags vrg_testhooks ./cmd/vrg`, listed as step 3 of the permanent gate, names a single main package and therefore writes an executable named `vrg` into the working directory. A gate advertised as "runnable from a clean checkout" dirties the tree on every run (and could be mistaken for the production artifact the smoke builds into `/tmp`).

**Required correction:** build the tagged variant to a discarded path — e.g. `go build -tags vrg_testhooks -o "$tmpdir/vrg-tagged" ./cmd/vrg` — or rely on `go vet -tags vrg_testhooks` alone for tagged compile coverage.

**Ready when:** `verify.sh` leaves the checkout clean.

### F6 — Low: Issue 50's "no `time.sleep`" static rule conflicts with its own bounded-polling allowance

**Affected:**

- `Notes/issues/050-post-audit-reverification.md:28,45`

The issue permits "bounded condition polling … only to fail a missing explicit condition" but also requires "a static/review assertion that the harness contains no `time.sleep` call." A condition poll needs pacing: on the Go side the issue (and Issue 48) explicitly permits `waitForFile`-style loops that sleep between checks; on the Python side a literal no-`time.sleep` rule forces every wait onto `select`/timeout primitives — implementable, but a different contract than the Go helpers and than the sentence that permits polling. As written, a conforming poll loop using `time.sleep(0.02)` between condition checks fails the static assertion, while a renamed `settle()` helper could hide a real settling delay.

**Required correction:** pick one: either require `select`/deadline-based waits with no `time.sleep` anywhere (and say so), or mirror the Go rule — `time.sleep` permitted only inside a bounded loop that checks an explicit named condition each iteration. A static assertion can only enforce the former; the latter is a review assertion.

**Ready when:** the smoke-harness rule is mechanically checkable without forbidding the polling style the same paragraph allows.

### F7 — Low: Issue 49's removal branch omits the prescriptive styling skill from the documentation diff

**Affected:**

- `Notes/issues/049-tidy-dependency-manifests.md:16,28`
- `Notes/skills/code-writing/styling-tui.md:3,8,19-20`
- `Notes/skills/AGENTS.md:20`

The removal branch now names the PRD *Further Notes*, `wiki/project-overview.md`, and `wiki/AGENTS.md` — good. But `Notes/skills/code-writing/styling-tui.md` is prescriptive, not historical: it instructs agents to "use Lip Gloss for terminal presentation and Bubbles components where they fit," and `Notes/skills/AGENTS.md` indexes it for all terminal-layout work. Under removal these actively instruct using removed libraries — worse than a stale wiki sentence. (The skill also describes itself as rules "in Sqloid," a different project name, so it may already be copied boilerplate worth checking regardless.) `Notes/Ideas.md` and `Notes/decisions/001-…` are historical records and reasonably stay.

**Required correction:** name `styling-tui.md` and its `skills/AGENTS.md` entry in the removal-branch documentation diff — update or retire them.

**Ready when:** no prescriptive document still directs Lip Gloss/Bubbles usage after a removal decision.

### F8 — Low: Issue 40's precomputed cell-width metadata does not name its width source

**Affected:**

- `Notes/issues/040-browse-render-no-whole-index-scan.md:14`

Issue 40 requires precomputing "the escaped path text, its grapheme-cluster boundaries, and its full cell width" once at search completion, and declares no ordering against Issue 39. Since `visibleWidth` is still rune-based until Issue 39 lands, an implementation landing Issue 40 first could compute "full cell width" with it — baking the rune-equals-cell error into the precomputed metadata and forcing Issue 39 to revisit it.

**Required correction:** state that the full cell width is measured with the shared grapheme/cell policy (`safepresentation.GraphemeClusters` widths, as `TruncateLeftGrapheme` and `graphemeCellWidthString` already do), not `visibleWidth`.

**Ready when:** the width source is named and independent of issue landing order.

### F9 — Low: minor notes

- `Notes/issues/048-pty-tests-deterministic-handshakes.md:27` — Issue 50's no-sleep AC names `runVrgCancel` while Issue 48's AC1 does not; Issue 48's helper-scope catch-all does cover it, and `runVrgCancel` is in fact already conforming (send-after-ready, no sleeps), so this is naming symmetry only.
- `Notes/issues/041-…:4` — the shared-file ordering note is workable but convoluted: "land Issue 48 first" is impossible while Issue 48 remains blocked by Issue 45, so in practice the note reads "Issue 41 either completes before Issue 45 lands or waits for 45→48." Worth one clarifying clause; also note that when Issue 41 lands before Issue 48, the bounded-traversal verification can only be the model-level variant (the PTY option presupposes a harness that does not yet exist).
- `Notes/issues/037-…:17,28` — "for every other count" leaves the zero case implicit; "Given oversized records" scopes it adequately, but "always emit" in the body can be misread as unconditional. One word ("when any oversized records were skipped") removes the ambiguity.

**Ready when:** each note is either applied or consciously declined; none blocks implementation correctness on its own.

## Coverage and readiness assessment

| Issue area | Assessment |
|---|---|
| 36: stream-integrity diagnostics | **Nearly ready.** Universal component order, uncapped multiplicity, and dual representation are now explicit; only the post-`summary` unterminated-record position needs one sentence (F4). |
| 37: oversized aggregate/anonymous diagnostics | **Ready.** Exact singular/plural forms are pinned (F9's zero-case note is cosmetic). |
| 38: viewport content-panel width | **Nearly ready.** The fix and three-width model are correct, but two sibling `TextWidth(m.width, …)` sites under the factory seam must be named and covered (F1). |
| 39: shared grapheme/cell rendering | **Nearly ready.** Consumer coverage is far better and the mechanical invariant exists; add the overlay wrap loop and broaden the invariant to display-geometry loops (F2). |
| 40: render cost | **Ready modulo wording.** Name the grapheme-policy width source for precomputed metadata (F8). |
| 41: full overlay scrolling | **Ready.** The Issue 9 supersession is now explicit in both issues, `TestStderrContentFixture` revision is prescribed, and tail reachability is proven at model level plus bounded traversal. Clarify the ordering note's implications (F9). |
| 42: dropped reload | **Ready.** Atomic admission correctly scoped to `handleReload`; the navigation re-entry exclusion is stated. |
| 43: combining fallback cell | **Ready.** HITL decision requirements remain complete and `Notes/decisions/` exists for the record. |
| 44: post-summary context | **Ready.** Issue 9's contradictory row is amended in place (line 26) and the issue acknowledges resolving the contradiction. |
| 45: production/test harness separation | **Nearly ready.** The manifest exactly matches the seven env reads in `main.go` plus the two runner controls; the remaining gap is the `WithOnCollect` single-callback collision with Issue 46 (F3). |
| 46: runtime replay | **Nearly ready.** Runner seam, snapshot, ordering, and exit-2 contract are all implementable; specify callback composition with the ack seam (F3). |
| 47: single-line read failures | **Ready.** Verified: all load failures funnel through the single `msg.Err.Error()` site at `app.go:1301-1303`, so one construction change covers initial/reload/retry, overlay, and replay. |
| 48: deterministic PTY handshakes | **Ready.** Helper-scoped coverage now reaches `runVrgWithQuit` and the out-of-range settle sleeps; `runVrgCancel` is already conforming and `waitForFile`/`waitForAckLines` polls are correctly exempted. |
| 49: dependency manifests | **Ready modulo wording.** Sole ownership of `verify.sh` rests with Issue 50; add the styling skill to the removal diff (F7). |
| 50: closing verification | **Nearly ready.** All named tests and helpers verified to exist; fix the stray `vrg` binary from the tagged-build step and reconcile the `time.sleep` rule with bounded polling (F5, F6). |

## Positive observations

- All nine critique-3 findings are resolved in text, and the two hardest ones were resolved correctly: Issue 41 explicitly supersedes Issue 9's head/tail contract *and* Issue 9's own text now records the supersession (lines 26, 57, 70), so no cited contract still demands simultaneous head/tail rendering; Issue 48's scope is now helper-based and matches every sleep site found in `outcome_test.go`, `replay_test.go`, and `search_test.go`.
- Issue 36's universal component order is now stated for both fatal and non-fatal compositions, with deliberate uncapped multiplicity and dual representation — a complete answer to critique-3 F3.
- Issue 45's hook manifest is exact: it enumerates precisely the seven `os.Getenv` calls present in `cmd/vrg/main.go` plus the two new runner controls, and explicitly excludes fixture-owned variables pending the `FAKE_RG_*` rename.
- Issue 50's named test list was checked against the suite — all 21 named tests exist, and `runVrgCancel` is a real helper that genuinely needs no conversion.
- Issue 46's return-shape matrix, replay ordering, and exit-2 contract are now internally consistent and the cleanup/restoration ordering is physically implementable.
- The dependency graph is execution-safe: `37→36`, `39→38`, `43→39`, `44→36`, `46→45`, `48→45`, `50→36-49`, with Issues 41/48's shared-file hazard handled by an explicit non-concurrency note.
- No audit coverage has been lost across three revision rounds; all fourteen findings still map to exactly one issue each plus the closing pass.

## Required revision order

1. Extend Issue 38 to all `SetLayout`/`TextWidth` install and resize sites, including the `rowProviderFactory` seam paths (F1).
2. Add overlay/help text wrapping to Issue 39's consumers and broaden its invariant to display-geometry loops (F2).
3. Specify `WithOnCollect` callback composition across Issues 45/46 so the snapshot and the ack seam cannot displace each other (F3).
4. Apply the Low corrections: Issue 36's post-`summary` unterminated position (F4), `verify.sh`'s build output path (F5), the smoke-harness sleep rule (F6), the styling-skill documentation diff (F7), Issue 40's width source (F8), and the F9 notes.
5. If these changes are made without expanding scope, Issues 36–50 are ready for task generation; no further full critique round is warranted — a targeted closure check on the diffs suffices.

No application tests were run because this review evaluates issue quality rather than implementation correctness. Targeted code, tests, and the existing smoke harness were inspected only to verify that the proposed work and dependency boundaries are executable in the current repository.
