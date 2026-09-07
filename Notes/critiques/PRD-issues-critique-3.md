# PRD issues critique 3 — VRG

## Scope and verdict

**Reviewed:** `Notes/skills/critique-issues/SKILL.md`, `Notes/PRD-vrg.md` revision 4, all 34 Markdown files in `Notes/issues/`, and the two previous issue critiques to assess the revised issue set. The current PRD is the requirements authority.

This is an **issues-only review**. It does not review implementation, generate tasks, or modify the PRD/issues.

**Verdict: make a small targeted revision before task generation.** The issue set is now unusually thorough. All 85 numbered user stories are mapped, the dependency repairs from critique 2 are present, and the difficult asynchronous, terminal-safety, outcome, viewport, and encoding contracts have explicit owners and deterministic verification. No core feature is missing, and no broad redesign or new umbrella issue is warranted.

The remaining concerns are narrower than those in critiques 1 and 2, but two should be fixed before treating each issue as an independently mergeable increment: Issue 5 can render hostile external bytes before the sanitizer exists, and Issue 3 can own a child process before concurrent stderr drainage is required. There is also one residual PRD/issue mismatch for a target grapheme wider than the text area, plus several precision gaps that can be corrected locally.

### What is good

- Every numbered user story from 1 through 85 is referenced by at least one issue. Split ownership of stories 8, 21, and 34 is explicit and sensible.
- The current `Blocked by` graph repairs the direct mismatches identified in critique 2: Issue 18 includes Issue 17; Issue 25 includes Issue 16; Issue 30 includes Issue 29; and Issue 32 includes Issue 26.
- Issue 17 now owns off-update-path layout preparation and obsolete-layout isolation. Issue 28 coherently closes the two-stage load → layout → latest-intent transition with separately gated tests.
- Issue 18 now matches the revision-4 PRD's approved visible-lines horizontal-extent policy and uses a paintable cluster/marker boundary rather than the incorrect `E - 1` maximum.
- Issues 9–10 provide one outcome-transition matrix and include warning-only emptiness, anomalous rg exit 1 with retained results, record loss after filtering, fatal dismissal, and the global cancellation override.
- Issue 4 requires actual PTY input-mode restoration and evidence of child reaping. Issues 9 and 11 add backpressure and application-side diagnostic-collection handshakes rather than relying on sleeps.
- Issue 6's no-style raw-output tests avoid stripping away injected terminal controls, and later sink owners explicitly extend the shared safety table.
- Filename-row status composition, no-results help, documentation status 130, popup recentering, too-small overlay scroll recovery, and CRLF marker behavior are now covered.
- The issue set stays within version-1 scope. It does not accidentally add streaming browsing, direct list selection, load cancellation, cache eviction, encoding transcoding, or a diagnostics-review UI.

### Severity definitions

- **High:** an independently mergeable issue can introduce a security/correctness failure, or issue behavior materially conflicts with the authoritative PRD.
- **Medium:** an important state transition, schema rule, or acceptance boundary permits divergent implementations.
- **Low:** a local traceability, governance, or verification omission that is unlikely to change the architecture.

## Findings

### F1 — High: arbitrary file bytes are rendered before the all-sink sanitizer exists

**Affected:**

- `Notes/issues/005-browse-tracer-file-list-and-file-panel.md`
- `Notes/issues/006-safe-presentation-utility-for-all-sinks.md`

Issue 5 introduces the first real browse view and renders raw-path-derived filenames and file content. Its acceptance criteria require those values to reach the composed `View()`. Issue 6, which is blocked by Issue 5, only afterward installs the shared safe-presentation utility.

That ordering creates an independently mergeable milestone in which a matched file can emit OSC, CSI, C0/C1, or invalid-byte content into the terminal. This is not merely an incomplete visual refinement: the PRD's all-sink sanitization rule is a terminal-safety boundary. The later Issue 6 tests do not make the Issue 5 increment safe while it exists.

Issue 1's explicitly temporary path escaper does not solve this. It covers usage diagnostics, not arbitrary result paths and content in the TUI. Likewise, Issue 5's “first-pass byte→cell mapping” does not promise control escaping.

**Recommended correction:** make safe presentation a prerequisite of the first arbitrary-data render. Either:

1. move the core path/content sanitizer and hostile-output tests into Issue 5, leaving Issue 6 to generalize the API and establish the extensible sink table; or
2. split Issue 6 into an earlier core sanitizer issue and a later all-sink integration issue, then block Issue 5 on the core.

Do not accept “Issue 6 follows immediately” as the safety control if issues are intended to be independently mergeable.

**Ready when:** no completed issue can render external path/content control bytes verbatim through a TUI or stderr sink.

### F2 — High: Issue 3 can deadlock on child stderr before Issue 9 requires concurrent drainage

**Affected:**

- `Notes/issues/003-spawn-rg-collect-results-searching-screen.md`
- `Notes/issues/009-error-overlay-and-fatal-outcomes.md`

Issue 3 first spawns and waits for ripgrep while collecting stdout. Concurrent stderr drainage is introduced only in Issue 9. Calling Issue 3 “happy-path only” does not remove the pipe obligation: ripgrep may write diagnostics while still exiting 0 or 1, and any implementation that creates a stderr pipe without draining it can block once the pipe fills. If stderr is inherited instead, Issue 3 avoids the deadlock but temporarily violates the eventual capture/sanitization contract and forces a subprocess-boundary refactor later.

Issue 9's ≥1 MiB backpressure test is strong, but it arrives after six intervening issues that depend on a potentially unsafe child-process foundation.

**Recommended correction:** Issue 3 should own concurrent, cancellation-compatible drainage of both stdout and stderr from the moment pipes are introduced. It may defer classification, overlays, and outcome effects to Issue 9, but it should buffer or safely hand off stderr without allowing backpressure. Move the basic dual-pipe handshake/backpressure assertion to Issue 3; Issue 9 can retain the assertions about diagnostic content and outcome presentation.

**Ready when:** the first issue that owns an rg child cannot block because either output pipe is not consumed.

### F3 — Medium: Issue 19 resolves a too-wide-target conflict that the PRD does not resolve

**Affected:**

- `Notes/issues/019-minimal-horizontal-reveal.md`
- `Notes/PRD-vrg.md`, *Navigation, viewport, and logical anchors* and *Text, graphemes, and safe presentation*

The PRD requires minimal panning until the target start cell is visible, while also requiring a grapheme split by clipping to render as blanks. For a target grapheme wider than the entire text area, both requirements cannot hold: no offset can paint the start cell without painting a partial grapheme.

Issue 19 chooses a new observable exception: set the offset to the target start, render clipping blanks, and treat the target as geometrically revealed so repeated navigation does not loop. This is internally coherent, but the PRD does not authorize the geometric-visibility exception. The PRD only defines “match wider than the viewport” in terms of showing its start; a match spanning many ordinary clusters is different from a single unpaintable cluster.

This case may be unreachable under the intended terminal-width and cell-width invariants, but both the PRD and Issue 19 explicitly discuss it, so it should not be settled solely in an AFK issue.

**Recommended correction:** choose one authoritative contract:

- state and test an invariant that supported text areas are always at least as wide as any renderable grapheme, and remove the unreachable exception; or
- amend the PRD to adopt Issue 19's geometric fallback (or a visible replacement-cell fallback) for an unpaintable target cluster.

**Ready when:** the PRD and Issue 19 use the same definition of “revealed” for a target cluster wider than the text area.

### F4 — Medium: record schema and lifecycle consistency are not concrete enough to make “malformed” deterministic

**Affected:**

- `Notes/issues/003-spawn-rg-collect-results-searching-screen.md`
- `Notes/issues/009-error-overlay-and-fatal-outcomes.md`
- `Notes/issues/010-record-robustness-malformed-oversized-unknown.md`

Issue 10 requires malformed known events to be skipped for “missing/wrongly typed required fields or invalid ranges,” while Issue 9 treats orphaned or inconsistent lifecycle events as stream-integrity failures. Neither issue enumerates the required fields and invariants for `begin`, `match`, `end`, `summary`, and ignored `context` records.

The PRD narrows this somewhat—validate fields needed for indexing or lifecycle validation, and do not require ignored context payloads to become display data—but an implementer still has to invent important boundaries. Examples include duplicate `begin` or `end`, a `match` before `begin` or after `end`, path disagreement across events, line-number zero/overflow, submatch ordering/overlap, `summary` multiplicity or placement, and malformed fields that are present but irrelevant to VRG.

Without a schema/invariant matrix, “each malformed category” is not a reproducible acceptance test. Different strictness can either reject usable ripgrep output or silently accept a stream whose completion metadata should fail integrity.

**Recommended correction:** add a compact validation table, based on the ripgrep 15.x JSON schema, that states per known event:

- fields/types VRG requires;
- range constraints it enforces;
- fields intentionally ignored;
- whether a violation counts as a skipped malformed record, stream-integrity failure, or both; and
- lifecycle transitions that are valid, recoverable, or fatal.

This belongs in Issues 3/9/10 or a shared fixture table, not in private implementation assumptions.

### F5 — Medium: re-entry retry behavior does not define the visible transition for a previously failed file

**Affected:**

- `Notes/issues/025-async-load-isolation.md`
- `Notes/issues/026-read-failures-unreadable-retry-rules.md`
- `Notes/issues/027-explicit-reload-r.md`

Issue 26 says a non-current failure is discovered by visiting the file, at which point it shows an overlay and placeholder. It also says crossing into a failed file from another file immediately retries it. The issue does not define the order or visible state when that retry is slow:

- Is the previous diagnostic shown immediately on entry?
- Does the panel show the previous `(unreadable)` state or switch immediately to `Loading…`?
- Is retry started while the error overlay is open, or only after dismissal?
- If the retry succeeds while the old error overlay is still open, does that overlay remain?
- If it fails again, is a second diagnostic appended while preserving the reader's position?

The manual test assumes an effectively immediate second failure and therefore does not settle the asynchronous transition. This intersects Issue 25's path/request-keyed completion and Issue 32's append-without-scroll behavior.

**Recommended correction:** specify one entry sequence and add a gated model test. A straightforward contract would be: entering records/shows the prior failure, immediately starts one retry, displays `Loading…`, and lets the result update the placeholder while the existing overlay remains dismissible; a new failure appends one new occurrence. Another choice is valid, but it must be explicit.

### F6 — Medium: controlled-failure diagnostics can be written twice unless collection ownership is defined

**Affected:**

- `Notes/issues/004-cancellation-child-cleanup-terminal-restore.md`
- `Notes/issues/011-stderr-replay-of-collected-diagnostics.md`

Issue 4 requires a controlled application failure to write a sanitized diagnostic to stderr after cleanup. Issue 11 later requires every collected diagnostic occurrence to be replayed exactly once after restoration, and explicitly includes controlled application failures.

The intended result is probably one stderr occurrence, but the handoff is ambiguous. If the Issue 4 failure message is both directly written and added to the session collection, Issue 11 replays it again. If it is never collected, the phrase “collected diagnostics are replayed” remains true but the application failure follows a separate ordering path. The PTY tests do not explicitly count the controlled-failure diagnostic across both mechanisms.

**Recommended correction:** make the controlled-failure diagnostic enter one path only. Prefer adding it to the collection before shutdown and using the common post-restoration replay writer. Add an assertion that the application-failure message appears once, after restoration, alongside any earlier diagnostics in defined collection order.

### F7 — Medium: Issue 34 allows one artifact but accepts another

**Affected:** `Notes/issues/034-documentation-scale-and-memory-limits.md`.

The build scope permits “a user-facing README (or `docs/` page),” but every acceptance criterion and most synchronization tests specifically require “the README.” An implementation that chooses the explicitly allowed `docs/` page cannot satisfy the written acceptance criteria; an implementation that writes only a README is valid, but the output location remains needlessly undecided at task-generation time.

The separate instruction to “confirm” ripgrep 15.x and `--no-config` behavior also has no acceptance criterion, so those details can be omitted while all checkboxes pass.

**Recommended correction:** select an exact documentation path before generating tasks, or phrase every acceptance/test in terms of the selected user-facing document. Add an acceptance criterion that it identifies ripgrep 15.x as the reference family and explains that VRG supplies `--no-config`.

### F8 — Low: invocation tests omit two boundary cases implied by the PRD

**Affected:**

- `Notes/issues/001-go-scaffold-cli-positionals-and-root.md`
- `Notes/issues/002-cli-flag-allow-list-and-child-argv.md`

First, the PRD explicitly says an actual regular file named `-` can be addressed as `./-`. Issue 1 tests rejection of root `-` but not acceptance of `./-`. Add this to both manual and automated root-validation coverage.

Second, the PRD rejects a **missing** pattern but does not reject a present empty argument. Under ordinary argv semantics, `vrg "" .` therefore forwards an empty pattern to ripgrep. Because that can create a very large set of zero-width matches, the behavior deserves an explicit test rather than accidental parser behavior. If the product instead wants to reject an empty string, that is a PRD change, not an implementation inference.

Also add explicit combined-short unrestricted examples such as `-iu` and `-iuu`; the general Issue 2 rule covers them, but the parser edge is easy to miss.

### F9 — Low: acceptance criteria mix product behavior, implementation architecture, and governance

**Affected:** especially Issues 1, 9, 11, 17, and 28.

Examples include reviewer approval recorded before merge, requiring one table-driven test, requiring a PTY harness, and prescribing that viewport state commits only at a particular internal stage. Some internal constraints are justified because responsiveness and obsolete-result isolation are stable PRD contracts, but the current acceptance lists mix three different notions of done:

1. user-observable behavior;
2. architecture/performance invariants; and
3. review/test-process requirements.

This does not make the issues wrong, but it makes task generation and completion reporting less precise.

**Recommended correction:** keep behavioral and stable performance invariants under `Acceptance criteria`; move human approval to a `Definition of done` section and concrete harness shape to `How to verify`. Preserve the deterministic tests—the problem is classification, not their existence.

## Coverage and readiness assessment

| Area | Assessment |
|---|---|
| CLI syntax, roots, flags, and child argv | Complete; add `./-`, empty-pattern, and combined-`u` boundary fixtures. |
| Child lifecycle, cancellation, terminal restoration | Strong; define one controlled-failure diagnostic path. |
| Search parsing, integrity, outcomes | Broadly complete; add a concrete known-event schema/lifecycle matrix. |
| Safe presentation | Final contract and tests are strong; move the safety boundary before first arbitrary-data rendering. |
| Browse layout, theme, navigation, wrapping, anchors | Strong and internally well integrated. |
| Horizontal pan/reveal/indicators | Revision-4 policy is reflected; settle the unpaintable target-cluster exception in the PRD. |
| Async loading, reload, stale/unsupported content | Detailed; define the visible state sequence for retry-on-entry. |
| Overlays, help, pop-ups, too-small recovery | Complete, including state precedence and scroll restoration. |
| Documentation and resource limits | Substantively complete; choose one artifact and add the omitted reference-family criterion. |
| Dependency graph | Direct critique-2 mismatches are repaired; the remaining ordering defects are the safety and stderr foundations in F1/F2. |

## Disposition of critique 2

The current revisions resolve critique 2's major findings:

| Critique-2 finding | Current disposition |
|---|---|
| Visible-lines pan policy absent from PRD | Resolved in revision-4 PRD and Issue 18. |
| Load → prepared-layout → reveal transition incoherent | Resolved by Issues 17, 27, and 28's two-stage intent/commit contract. |
| Direct dependency mismatches | Resolved for Issues 18, 25, 30, and 32. |
| `E - 1` may end inside a wide cluster | Resolved by Issue 18's paintable-boundary maximum. The distinct too-wide-target authority gap remains as F3 above. |
| Filename-row status integration missing | Resolved by Issue 24 dependencies and composed-view tests in Issues 26, 29, and 30. |
| Help unavailable/untested on no-results | Resolved in Issue 31. |
| Stderr replay collection race | Resolved by Issue 11's application-side acknowledgement. |
| Documentation may omit status 130 | Resolved by Issue 34's explicit four-status table and tests. |

## Recommended revision order

1. Move the core sanitizer before the first browse rendering and require dual-pipe drainage when rg is first spawned.
2. Align the PRD and Issue 19 on the unpaintable target-grapheme case.
3. Add the known-event schema/lifecycle validation matrix.
4. Specify retry-on-entry presentation and deduplicate controlled-failure diagnostic output.
5. Fix Issue 34's artifact/acceptance mismatch and add the small invocation fixtures.
6. Separate governance and harness notes from behavioral acceptance criteria where practical.
7. Re-run a dependency-closure and acceptance-coverage check, then proceed to task generation.

The issue set does **not** need more feature scope or a generic “App integration”/“testing traceability” issue. The existing issues already assign and test those contracts in detail. The remaining work is to make the earliest increments safe and remove a handful of local ambiguities.