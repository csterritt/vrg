# Final-audit issues critique 1 — VRG

## Scope and verdict

Reviewed:

- `Notes/skills/critique-issues/SKILL.md`.
- `Notes/PRD-vrg.md`, revision 6, in full.
- `Notes/critiques/final-audit-vrg.md`, in full.
- Issues 36–50 in `Notes/issues/`, in full.
- The relevant earlier contracts in Issues 9 and 35.
- Targeted implementation and test locations named by the audit/issues, solely to verify that the proposed issue boundaries and instructions fit the current repository.

This is an **issues-only review** of the new post-audit issue set. It evaluates whether Issues 36–50 faithfully and completely convert the final-audit findings into executable work with correct dependencies and acceptance criteria. No PRD, issue, task, or implementation file was changed.

**Verdict: targeted corrections are required before converting Issues 36–50 into tasks.** The set covers every final-audit finding and most issue bodies are strong, but the current text contains one material geometry contradiction, an incomplete integrity-diagnostic contract, an unsafe dependency order, and several verification ambiguities. Most importantly, Issue 38's acceptance criteria use “text width” for the larger panel width even though the implementation and PRD require text width to exclude the gutter and right indicator. A literal implementation can therefore preserve the audited width bug under different wording.

Issues 37, 39, 42, and 47 are otherwise ready. Issues 36, 38, 40–46, and 48–50 need the focused revisions below; no additional umbrella feature issue is needed.

### Severity definitions

- **High:** the issue can be completed literally while the audited defect or a material PRD violation remains, or the declared order can cause substantial rework.
- **Medium:** the intended correction is clear, but acceptance, dependencies, or verification are ambiguous enough to permit inconsistent implementation or false closure.
- **Low:** local traceability or wording defect unlikely to affect behavior.

## Findings

### F1 — High: Issue 36 does not define complete, deterministic integrity diagnostics

**Affected:**

- `Notes/issues/036-stream-integrity-fatal-diagnostics.md:12-23,27-32`
- `Notes/issues/009-error-overlay-and-fatal-outcomes.md:18-32`
- `Notes/PRD-vrg.md:149,172,251,273-275`

Issue 36 correctly requires structured integrity information rather than a boolean, but it names only a few examples: missing `summary`, missing `end`, and a record after `summary`. The earlier lifecycle matrix contains additional independently observable causes: duplicate `begin`, orphaned `match`, orphaned/duplicate `end`, second `summary`, inconsistent paths/lifecycle, malformed completion metadata, and a trailing unterminated record. The PRD requires SearchIndex to expose integrity diagnostics, not merely enough detail for the two audit examples.

The issue also leaves diagnostic multiplicity, ordering, and path treatment undefined. For example, multiple files can be missing `end`; map iteration must not make their diagnostic order nondeterministic, and any embedded path must use the single-line path escaper. “Compose all applicable sources” does not establish the ordering of process failure, integrity causes, child stderr, malformed/oversized counts, unknown-type warnings, and per-path oversized details. That matters because the PRD requires replay in collection order and Issue 36 requires the overlay and replay to agree.

A narrow implementation could satisfy every listed acceptance criterion while still reporting only missing-summary and missing-end causes, leaving the other lifecycle failures unexplained.

**Required correction:** add an explicit integrity-diagnostic matrix covering every Issue 9 lifecycle-failure row. Define whether repeated identical violations are represented individually or aggregated, define deterministic ordering (including raw-path ordering for EOF-discovered missing ends), require `EscapePath` for embedded paths, and define the final component order used identically by overlay text and replay. Extend automated tests to every integrity cause, not only the four examples currently listed.

**Ready when:** every way `Integrity.Complete` can become false has a stable user-facing explanation and deterministic composed/replayed output.

### F2 — Medium: Issue 36 incorrectly requires a process-status component even when ripgrep succeeded

**Affected:**

- `Notes/issues/036-stream-integrity-fatal-diagnostics.md:15,22-23,28`
- `Notes/critiques/final-audit-vrg.md:17-22`
- `Notes/PRD-vrg.md:150,164-172`

Issue 36's automated guidance says every fatal-integrity row must contain a process component, and AC2 requires “the process status” alongside stderr and the integrity failure. That is too broad. The PRD assesses process success and stream integrity separately. It requires a generated code/signal diagnostic when the **process failed** without explanatory stderr; it does not require “ripgrep exited with code 0” when exit 0/1 was successful but stream integrity independently failed.

The audit specifically identifies `ripgrep exited with code 0` as a misleading substitute for the missing integrity explanation. Retaining that sentence as a mandatory component adds noise and can suggest that exit 0 itself was erroneous.

**Required correction:** define process status as applicable only for signal death or an exit code other than 0/1. For exit 0/1 plus failed integrity, report the integrity causes and any real stderr/record-loss diagnostics, without manufacturing a process-failure line. Adjust AC2 and the automated component assertion accordingly.

**Ready when:** process and integrity are independently reported, and a successful rg exit is never described as the failure cause.

### F3 — High: Issue 38 confuses panel width with text width in its acceptance contract

**Affected:**

- `Notes/issues/038-viewport-content-panel-width.md:12-16,24-33`
- `Notes/PRD-vrg.md:230-238`
- `internal/app/app.go:1026-1040,1385-1420`

The issue body points to the correct implementation repair: install the viewport with `msg.Key.TextWidth`. However, AC1 says the viewport's **text width** equals:

```text
terminal width - list width - separator
```

That formula is the file-panel width, not the text width. Under the PRD, the actual text width is:

```text
panel width - gutter width - reserved right-indicator width
```

where the reserved indicator is one cell in run-off-edge mode and zero in wrap mode. `LayoutKey` already performs this second calculation through `viewport.TextWidth`; the audited defect is that installation recomputes from the terminal width instead of using the key's result.

The same ambiguity affects AC2–AC3 and the test guidance: panning, reveal, clipping, and match visibility are measured against the **text area**, while the indicator occupies a reserved panel column and the gutter occupies the panel's left side. A test that subtracts only list width and separator can still assert the wrong geometry.

**Required correction:** distinguish and name all three widths throughout the issue:

1. terminal width;
2. panel width = terminal minus list allocation and separator;
3. text width = panel minus gutter and reserved indicator.

State that `SetLayout` receives `msg.Key.TextWidth`, and make every expected reveal/clip/pan calculation use the third value. Indicator placement should be asserted at the panel edge but outside the text area.

**Ready when:** no acceptance criterion calls panel width “text width,” and tests fail if either the gutter or run-off-edge indicator is omitted from the calculation.

### F4 — Medium: Issue 40 asks dynamic path truncation to be precomputed once

**Affected:**

- `Notes/issues/040-browse-render-no-whole-index-scan.md:14-16,23-31`
- `Notes/PRD-vrg.md:51-57,232-235,259`

Issue 40 says “truncated paths” should be prepared once when search completion finalizes the index. Final truncation cannot generally be prepared once: the allotted file-list width changes on terminal resize, gutter growth, wrap-mode indicator changes, and list visibility changes. The PRD requires left truncation against the current allocation without splitting graphemes.

Precomputing the sanitized path, grapheme boundaries, full cell width, file grouping, and current-file lookup is correct. Truncating only the visible rows against the current list width is also bounded by the visible window. Precomputing one final truncated string is not.

**Required correction:** replace “truncated paths” with width-independent path metadata: escaped text, grapheme clusters/boundaries, and full cell width. Explicitly permit current-width truncation for only the visible entries during `View()`. Add a resize/gutter-growth test proving visible paths are retruncated without a whole-list scan.

**Ready when:** the cost requirement remains O(visible rows) while dynamic layout still changes truncation correctly.

### F5 — Medium: Issue 41's manual verification adds overlay keys the PRD requires to be ignored

**Affected:**

- `Notes/issues/041-overlay-full-scroll-no-head-tail-compression.md:18,22-30`
- `Notes/issues/009-error-overlay-and-fatal-outcomes.md:40,68`
- `Notes/PRD-vrg.md:115,244-249`

Issue 41 tells the manual tester to scroll an error overlay with `up`/`down`/**`u`/`d`/page keys**. The authoritative error-overlay contract accepts only `up`, `down`, `q`, `Esc`, and `ctrl+c`; other keys are ignored. The `u`/`d` and page bindings belong to base file-content scrolling, not overlays.

Following the manual instruction either fails despite a correct implementation or encourages an unrequested key-binding expansion that contradicts Issue 9 and the PRD.

**Required correction:** use repeated `up`/`down` in the manual traversal. Keep automated tests that drive `overlayScroll` through those keys and separately retain the ignored-key regression for `u`, `d`, page up, and page down.

**Ready when:** Issue 41 changes only preservation/reachability of overlay rows, not modal key routing.

### F6 — Medium: Issue 43 leaves the visible fallback representation unresolved

**Affected:**

- `Notes/issues/043-combining-cluster-fallback-cell.md:12-23,27-31`
- `Notes/PRD-vrg.md:203-206,335-339`

Issue 43 requires a concrete one-cell fallback but offers only “e.g. a visible escaped/placeholder form on a space base.” Those alternatives are not equivalent. An escaped textual form normally occupies multiple cells; a space-plus-combining-mark sequence can render inconsistently across terminals and width libraries; a fixed placeholder can hide the original combining mark. The acceptance criteria demand exactly one painted cell, so the issue needs one deterministic representation and policy.

This is user-visible behavior, not merely an internal type choice. The PRD requires a visible fallback but does not select its glyph or whether the original mark is composed onto a base.

**Required correction:** either record a product choice in the PRD or make Issue 43 HITL before implementation. Specify the exact display sequence, its expected grapheme segmentation and cell width under the shared library, how unsupported width results are normalized, and how source-byte mapping remains attached. Tests should assert the chosen bytes/grapheme plus terminal cell width, not only that the following cluster starts one cell later.

**Ready when:** two conforming implementations cannot choose visibly different fallback conventions while both passing the issue.

### F7 — High: the declared dependency graph is not execution-safe

**Affected:**

- `Notes/issues/044-post-summary-context-integrity-failure.md:3-4,22-30`
- `Notes/issues/045-remove-test-hooks-from-production-binary.md:12-23`
- `Notes/issues/046-runtime-error-common-diagnostic-replay.md:20-30`
- `Notes/issues/048-pty-tests-deterministic-handshakes.md:3-16,27-31`
- `Notes/issues/049-tidy-dependency-manifests.md:3-16`

Three declared “can start immediately” relationships are unsafe:

1. Issue 44's verification and AC4 explicitly depend on Issue 36's composed integrity diagnostic, but Issue 44 declares no blocker. Its parser change can be developed independently, but the issue cannot satisfy its own acceptance criteria until Issue 36 lands.
2. Issue 48 explicitly requires “the same test-only harness mechanism as Issue 45,” while declaring no blocker. Implementing it first either adds more production hooks that Issue 45 must immediately migrate or invents a competing harness.
3. Issue 46 requires runtime-error injection in a subprocess/PTY test. That injection belongs to the same production-hook migration boundary as Issue 45; without an ordering or a deliberately disjoint mechanism, Issues 45 and 46 can rewrite the same harness twice.

Issue 49 creates a larger conditional dependency. If the human chooses to adopt Bubbles or Lip Gloss rather than remove them, that choice can alter viewport, list, theme, overlay, and width code being changed by Issues 38–41. Allowing the choice to occur after those issues risks invalidating their implementation and tests.

**Required correction:** make Issue 44 blocked by Issue 36; make Issue 48 blocked by Issue 45; and either block Issue 46 on Issue 45 or explicitly assign its injection seam to Issue 45. Resolve Issue 49's HITL choice before Issues 38–41, or constrain Issue 49 to removal/documentation and create separately scoped implementation issues if adoption is selected.

**Ready when:** every acceptance criterion can be met when its issue starts, and no later dependency decision can require the render/test harness fixes to be redone.

### F8 — Medium: Issue 45 does not specify a viable subprocess test-build topology

**Affected:**

- `Notes/issues/045-remove-test-hooks-from-production-binary.md:12-31`
- `cmd/vrg/main_test.go:14-31`
- `cmd/vrg/main.go:76-167`

Issue 45 suggests either a build tag or `_test.go`-only injection, but the current PTY suite does not execute the `go test` package binary. `TestMain` separately runs `go build -o <bin> .`, and every subprocess test executes that built `vrg`. Code in `_test.go` files cannot affect that binary. A tagged solution works only if the test harness deliberately builds a tagged test binary while production checks build an untagged binary.

Without an explicit topology, an implementer may choose the suggested `_test.go` route and discover that none of the process-boundary seams exist, or may leave hook dispatch in the production entry point and merely move helpers behind a tag.

**Required correction:** select one supported topology in the issue. For example: production `cmd/vrg` contains no hook names or dispatch; a `vrg_testhooks`-tagged adapter is compiled only into the binary built by `TestMain` with `go build -tags vrg_testhooks`; and a separate untagged production-binary test probes every former environment variable and inspects the artifact for hook names. Alternatively, define a dedicated test-harness command with the same entry-point wiring. State how default `go test ./...`, race tests, and Issue 48 build and locate the correct binary.

**Ready when:** the subprocess tests retain true executable-boundary control without any test-hook dispatch in the default production artifact.

### F9 — Medium: Issue 49's “adopt where intended” branch is unbounded and cannot prove meaningful dependency use

**Affected:**

- `Notes/issues/049-tidy-dependency-manifests.md:3,12-28`
- `Notes/PRD-vrg.md:261-299,341-347`

The HITL designation is appropriate, but “adopt them where intended (making the requirements real)” does not define what Bubbles or Lip Gloss must own. A token import can satisfy `go mod tidy` without improving the design; a genuine adoption can become a broad rewrite of viewport, list, or theme code and overlap the audit fixes. Dependency-manifest hygiene should not silently own an unspecified UI migration.

The removal branch also says to update “stated-stack documentation” without naming the authoritative PRD location or any other generated/reference documentation that must agree.

**Required correction:** present explicit decision outcomes. If removing, name the exact PRD/further-notes and repository documentation to update, then tidy. If adopting, record the concrete component/API each library will replace or implement and create scoped implementation issue(s) with dependencies on the relevant audit fixes; Issue 49 should then close only after those issues land and tidy is clean. Reject imports whose sole purpose is retaining a manifest entry.

**Ready when:** either removal has a finite documentation diff, or adoption has a separately reviewable behavioral scope and dependency order.

### F10 — Medium: Issue 46 leaves the runtime-failure result and exit contract underspecified

**Affected:**

- `Notes/issues/046-runtime-error-common-diagnostic-replay.md:12-31`
- `Notes/PRD-vrg.md:174-176,251,295-299`
- `cmd/vrg/main.go:173-209`

Issue 46 requires the “latest model's” diagnostics even when `program.Run()` returns an error, but it does not say how those diagnostics survive when `finalModel` is nil or has the wrong type—the exact invalid-model branch it also requires to diagnose. Depending solely on a successful final-model assertion cannot satisfy both requirements. The issue also says only that the runtime-error status remains “non-zero and distinct per the existing failure contract”; the current controlled application-failure convention is exit 2, and acceptance should state that exact result.

**Required correction:** define a shutdown-result or diagnostic snapshot channel that exists independently of the final type assertion, establish the exact ordering between retained session diagnostics, invalid-final-model diagnostics, and the runtime error, and require exit 2. Add separate tests for `(valid model, Run error)`, `(invalid/nil model, Run error)`, and `(invalid/nil model, nil Run error)` so no branch silently loses diagnostics or duplicates the runtime error.

**Ready when:** every `Run()` return shape has one deterministic replay sequence, cleanup path, and exact exit code.

### F11 — Medium: Issue 50 is not yet a reproducible closing gate

**Affected:**

- `Notes/issues/050-post-audit-reverification.md:12-34`
- `Notes/issues/049-tidy-dependency-manifests.md:15-28`
- `Notes/critiques/final-audit-vrg.md:149-158`

Issue 50 says `govulncheck ./...` uses a pinned version but does not provide a pinned invocation. The audit used `golang.org/x/vuln/cmd/govulncheck@v1.5.0`; invoking an arbitrary installed `govulncheck` does not reproduce that gate. The issue also does not name the post-audit walkthrough path or format, does not provide executable commands for the five smoke scenarios, and does not identify the persistent “standard verification routine” to which Issue 49 says the tidy check must be added.

“Critical PTY/subprocess tests explicitly” is also not mapped to package/test commands, so a reviewer cannot distinguish the required uncached/race executions from the broader suite. The artifact can claim deterministic handshakes without recording which synchronization checks were run.

**Required correction:** specify the exact pinned vulnerability command (for example `go run golang.org/x/vuln/cmd/govulncheck@v1.5.0 ./...`), exact focused PTY package/test commands including race mode, exact smoke harness commands or named tests, and the walkthrough output path. Name the repository script/CI/documented verification entry point that permanently owns `go mod tidy -diff`; a one-time note in the walkthrough is not a future gate.

**Ready when:** another reviewer can reproduce every closure checkbox from a clean checkout without inferring tool versions, test selectors, harness setup, or evidence location.

## Coverage and readiness assessment

| Issue area | Assessment |
|---|---|
| 36–37: integrity and record-loss diagnostics | **Not ready.** Aggregate oversized handling is strong, but Issue 36 needs a complete diagnostic matrix and corrected process-status rule (F1–F2). |
| 38: viewport width | **Not ready.** Core fix is correct; acceptance geometry is contradictory (F3). |
| 39, 43: grapheme/cell rendering | Issue 39 is strong. Issue 43 needs an exact fallback representation/product decision (F6). |
| 40: render cost | Correct objective; revise precomputation wording for width-dependent truncation (F4). |
| 41: full overlay scrolling | Correct implementation boundary; manual keys must match modal routing (F5). |
| 42: dropped reload | **Ready.** It accurately addresses the audited admission/mutation ordering defect with focused tests. |
| 44: post-summary context | Behavior is correct; closure dependency on Issue 36 must be declared (F7). |
| 45, 48: production/test harness separation | **Not ready.** Select an executable-boundary build topology and order Issue 48 after it (F7–F8). |
| 46: runtime replay | Correct goal; define snapshot source, return-shape matrix, exact ordering, and exit 2 (F7, F10). |
| 47: single-line read failures | **Ready.** Scope, escaping boundary, and initial/reload/retry coverage are clear. |
| 49: module tidy | HITL is correct, but adoption must not hide an unbounded UI migration (F7, F9). |
| 50: closing verification | Complete gate categories, but commands and evidence are not reproducible enough for closure (F11). |

## Positive observations

- All fourteen numbered final-audit findings have a corresponding focused issue; no audit finding was omitted.
- The issue set correctly separates the closing verification pass (Issue 50) from behavioral fixes.
- Issue 37 covers both anonymous and recoverable oversized records with and without usable results.
- Issue 39 includes composed-view coverage at the final output boundary rather than relying only on FileBuffer/Viewport unit tests.
- Issue 42 correctly targets the admission point rather than trying to repair the completion after state was already mutated.
- Issue 47 correctly distinguishes path escaping from diagnostic escaping and applies the construction across initial load, reload, retry, overlay, and replay.

## Required revision order

1. Correct Issue 38's panel/text-width terminology and formulas before generating layout tasks.
2. Expand Issue 36 into a complete deterministic integrity-diagnostic contract and remove mandatory success-status diagnostics.
3. Resolve Issue 49's HITL dependency decision before render/theme work; split genuine library adoption into scoped issues if selected.
4. Select Issue 45's test-binary topology, then make Issues 46 and 48 consume that seam in dependency order.
5. Correct Issue 44's dependency, Issue 40's dynamic-truncation wording, Issue 41's keys, and Issue 43's fallback representation.
6. Make Issue 50's pinned commands, focused test selectors, permanent tidy gate, and walkthrough artifact explicit.
7. Re-run an issues-only closure review before generating tasks for Issues 36–50.

No application tests were run because this review evaluates issue quality rather than implementation correctness. Targeted code and tests were inspected to confirm that the proposed work and dependency boundaries match the current repository.
