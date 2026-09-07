# PRD issues critique 2 — VRG

## Scope and verdict

**Reviewed:** `Notes/PRD-vrg.md` revision 4, all 34 Markdown files in `Notes/issues/`, `Notes/Ideas.md` as historical context, and `Notes/critiques/PRD-issues-critique-1.md` to assess the revised issue set. Applied `Notes/skills/critique-issues/SKILL.md`. The current PRD is the requirements authority.

This is an **issues-only review**. It does not review implementation, generate tasks, or modify the PRD/issues.

**Verdict: revise before task generation.** The issue set has improved materially since critique 1. It still references all 85 numbered user stories, and most previous contradictions, cleanup gaps, unsafe-output tests, outcome omissions, and incorrect examples have been corrected. The remaining problems are narrower but consequential: one issue introduces an observable product decision absent from the PRD, the asynchronous load-to-layout-to-reveal boundary is not specified coherently, and several dependency claims still do not match issue scope or verification.

### What is good

- Every numbered user story is referenced by at least one issue; there is no wholly unmapped story.
- The revised Issue 4 now owns controlled-failure cleanup, PTY mode restoration, and evidence of the wait/reap path.
- Issues 9–10 now define a consolidated outcome-transition matrix with the previously missing warning, anomalous rg-1, record-loss, binary-filtering, and cancellation combinations.
- Issue 6 now tests raw sink output before ANSI stripping and gives later sinks explicit extension ownership.
- Issue 17 now owns off-UI layout preparation and obsolete-layout isolation; Issue 24 adds anchor checks for list-induced width changes.
- Issue 28 now preserves visible-target no-scroll behavior and tests marker, grapheme, stale-target, and away-and-back reload cases.
- Issue 32 correctly distinguishes overlay dismissal from a standalone quit command and preserves fatal no-results dismissal to exit 2.
- The misleading manual examples identified in critique 1 have generally been repaired.

### Severity definitions

- **High:** an issue changes the product contract without authoritative requirements, cannot be completed from its declared prerequisites, or leaves a central asynchronous state transition materially under-specified.
- **Medium:** an important logic edge, integration owner, or deterministic verification boundary remains incomplete.
- **Low:** a local verification weakness could allow documentation or a narrow behavior to drift.

## Findings

### F1 — High: Issue 18 records an observable product decision that is absent from the PRD

**Affected:**

- `Notes/issues/018-horizontal-panning.md`
- `Notes/PRD-vrg.md`, *Navigation, viewport, and logical anchors* and *Layout and indicators*

Issue 18 says the product owner selected a **visible-lines** horizontal-clamping policy and explicitly rejects the file-wide alternative. That policy has substantial observable consequences: vertically leaving a long line permanently clamps the stored horizontal offset left, and returning to the long line does not restore it.

The PRD only says that panning is clamped to valid extents and that retained offsets are subject to actual-display clamping. It does not define which lines determine the horizontal extent, nor does it record the destructive visible-lines behavior. The PRD also states that no product-choice questions remain. An AFK implementation issue is therefore acting as the sole authority for a user-visible requirement.

This is not a provisional data-structure choice. It affects scrolling, saved per-file state, navigation reveal, wrap-toggle round trips, list/gutter relayout, and what users see when they return to a long line.

**Recommended correction:** either:

1. amend the PRD to record the approved visible-lines policy and its destructive re-clamping consequence, then keep Issue 18 aligned with it; or
2. if that approval has not actually occurred, return the choice to the product owner and make the relevant issue HITL rather than asserting a decision inside the issue.

**Ready when:** the PRD and Issue 18 state the same horizontal extent policy and the same saved-offset behavior.

### F2 — High: the asynchronous load → prepared layout → reveal transition has no coherent completion contract

**Affected:**

- `Notes/issues/017-logical-anchor-through-rewrap-and-resize.md`
- `Notes/issues/024-file-list-layout-width-truncation-toggle.md`
- `Notes/issues/027-explicit-reload-r.md`
- `Notes/issues/028-load-completion-reveal-latest-target.md`
- `Notes/issues/029-stale-match-validation-and-file-changed-note.md`

Issue 17 correctly makes rendered-row preparation asynchronous and keyed by path, content revision, width, and mode. A file-load completion therefore supplies a decoded/mapped `FileBuffer`, but it does not by itself supply the current rendered-row map needed to decide:

- whether the destination row is already visible;
- which wrapped row contains the target;
- the one-third vertical placement and EOF clamp;
- horizontal visibility after grapheme-safe clipping;
- the final text width after gutter and file-list recomputation; or
- the effective anchor in a reloaded revision.

Issue 28 nevertheless says to apply destination reveal **when the load completes**, and its controlled tests gate and release loads rather than the subsequent matching layout preparation. At startup there may be no prior layout at all. On reload, the only installed layout may belong to the old content revision. On a simultaneous resize, toggle, list-width change, or gutter growth, it may also have the wrong dimensions or mode.

An implementation can therefore pass the written load tests while revealing against an obsolete row map, briefly installing an incorrect viewport, losing the pending reveal when the correct layout arrives, or preserving/clamping an anchor against old content. Issue 17 proves that obsolete layouts are rejected, but it does not prove that a pending reveal or reload-anchor intent is committed when the matching current layout eventually arrives. It also tests `n` while a layout is gated only for immediate cursor movement, not for the eventual reveal of that newest target.

**Recommended correction:** define one integration owner for the full transition:

1. load completion validates content, establishes a content revision and final gutter/status inputs, and requests layout for the current path/revision/width/mode;
2. reveal or reload-anchor intent remains pending while no matching layout is installed;
3. only installation of the matching current prepared layout may commit row-based clamping/reveal;
4. discarded layouts cannot consume or mutate that intent; and
5. the committed target is the latest selected valid target, including stale survivor/fallback rules.

Add deterministic tests that gate load and layout separately and cover:

- startup load, then resize before layout completion;
- `n`/`p` while the current layout is pending, followed by matching-layout completion;
- reload with and without navigation while old-revision and new-revision layouts complete out of order;
- gutter/list-width change between load and layout completion; and
- a marker, expanded grapheme, and stale fallback through this two-stage path.

**Ready when:** no issue assumes that raw load completion can perform rendered-row operations before a matching prepared layout exists.

### F3 — High: the dependency graph still does not support independently executable issues

**Affected:** Issues 18, 25, 30, and 32.

The broad dependency repair from critique 1 is good, but these direct mismatches remain:

| Consumer | Declared prerequisites | Scope/verification that is unavailable |
|---|---|---|
| Issue 18 | Issue 16 | Its scope and acceptance criteria require Issue 17's prepared-layout data and no-full-buffer-scan contract. |
| Issue 25 | Issue 13 | Its gated-load verification requires `w` to remain actionable, but wrap mode and the `w` binding are introduced by Issue 16. |
| Issue 30 | Issues 26 and 27 | It requires and tests that stale validation is not invoked, but stale validation is introduced by Issue 29. |
| Issue 32 | Issues 31 and 15 | Its principal error-over-help test releases a current-file load failure, behavior introduced by Issue 26. |

For Issue 32, adding Issue 26 is not necessarily required: the same modal-precedence behavior can be tested by injecting a generic new diagnostic/error through the Issue 9 component. The issue must either change the fixture or declare the dependency. For Issues 18 and 30, the later component is part of the acceptance contract itself, so the dependency should be real. Issue 25 should depend on Issue 16 or stop requiring the `w` behavior in that issue's verification.

**Recommended correction:** audit the transitive prerequisite closure again after making the F2 ownership change. Do not rely on numeric filenames as an implicit merge order.

**Ready when:** each issue can be implemented and verified using only declared completed prerequisites plus explicitly deferred follow-up behavior.

### F4 — Medium: Issue 18's `E − 1` maximum does not guarantee a rendered content cell

**Affected:**

- `Notes/issues/018-horizontal-panning.md`
- `Notes/issues/019-minimal-horizontal-reveal.md`

Issue 18 defines maximum offset as `max(0, E − 1)` and claims this guarantees that at least one content cell of the widest visible line remains rendered. That is false under the same issue set's grapheme-safe clipping rule.

If the trailing content is a two-cell cluster and offset `E − 1` starts inside that cluster, the visible fragment is rendered as blanks rather than as half a glyph. A line whose only remaining content is that cluster can therefore produce an entirely blank text area at the claimed maximum. The acceptance criterion asserting a visible content cell conflicts with the clipping criterion and can also undermine hidden-match visibility calculations.

Issue 19 has a related impossible synthetic branch: if a target cluster is wider than the entire text area, it says to show the cluster's start cell while clipping the remainder to blanks. Grapheme-safe rendering cannot paint a partial wide cluster, so the start cannot simultaneously be a painted cell. Normal VRG dimensions may make this rare, but the issue explicitly specifies and tests the branch.

**Recommended correction:**

- Define the maximum valid offset in terms of a paintable cluster/marker boundary if a nonblank-cell guarantee is intended, not merely the final cell index.
- Add a render-level assertion for a line ending in a wide cluster at maximum pan.
- For a cluster wider than the text area, either rule the geometry unreachable under supported layout invariants, define a visible fallback representation, or relax the painted-start requirement. Do not require mutually incompatible clipping and visibility behavior.

### F5 — Medium: filename-row status integration is described before the statuses exist and never closed later

**Affected:**

- `Notes/issues/024-file-list-layout-width-truncation-toggle.md`
- `Notes/issues/026-read-failures-unreadable-retry-rules.md`
- `Notes/issues/029-stale-match-validation-and-file-changed-note.md`
- `Notes/issues/030-unsupported-encodings-utf16-utf32.md`

The PRD requires the filename row to make room for buffer-status notes and truncate the safe path where possible. Issue 24 mentions notes from Issues 26/29/30, but those statuses do not exist in its prerequisite closure and the behavior has no acceptance criterion there. The later status issues neither depend on Issue 24 nor test narrow filename-row composition.

The final issue set could therefore satisfy all written acceptance checks while an unreadable, stale, or unsupported note overruns the terminal, displaces the path incorrectly, disappears when width is constrained, or causes width calculations inconsistent with Issue 24.

**Recommended correction:** make the first status-introducing issue depend on Issue 24, or assign the final status/layout integration to the last relevant issue. Add composed-view tests for unreadable, stale, and unsupported states at ordinary and constrained widths, including a long escaped path and nonnegative layout dimensions. Assert the PRD's “make room for status where possible” behavior rather than only testing each status string in isolation.

### F6 — Medium: help availability on the no-results base state is not verified

**Affected:**

- `Notes/issues/031-help-overlay.md`
- `Notes/issues/032-overlay-precedence-esc-semantics.md`

The PRD places help in the browse/no-results modal-precedence model and states without qualification that `h`/`?` opens help. Issue 31's build text is broad, but its acceptance criterion and tests start only from browsing. Issue 32 tests dismissal from a warning overlay to no-results, but not opening or closing help once no-results is the base state.

An implementation that ignores `h`/`?` on “No results found” can pass the current issue tests. This also leaves untested whether closing help correctly returns to no-results and whether `q` then exits 1.

**Recommended correction:** add model acceptance for `h` and `?` from no-results, modal key isolation there, each close key returning to no-results, and the subsequent ordinary `q` exit 1. If help is intentionally browse-only, the PRD must say so instead.

### F7 — Medium: the PTY stderr-replay test has no deterministic “collected” handshake

**Affected:** `Notes/issues/011-stderr-replay-of-collected-diagnostics.md`.

Issue 11 defines a diagnostic as collected when the model processes its message. Its PTY cancellation test has fake rg write a stderr line, handshake, and block; the test then sends `ctrl+c` and expects replay. A child-side handshake proves only that bytes were written to the pipe. It does not prove that vrg drained them, converted them to a diagnostic message, and processed that message before cancellation.

The expected result is therefore scheduler-dependent under the issue's own shutdown-boundary definition. A fast cancel may legitimately win before model processing, while the test expects the line. Conversely, weakening the assertion would fail to prove replay at the real drain boundary.

**Recommended correction:** add an application-side test acknowledgement or observable hook after the diagnostic has entered the session collection, and send cancellation only after that acknowledgement. Keep the converse gated test for a diagnostic not yet collected. This preserves the no-sleep, handshake-driven subprocess requirement and precisely proves the intended boundary.

### F8 — Low: documentation synchronization can omit exit 130 while passing its automated check

**Affected:** `Notes/issues/034-documentation-scale-and-memory-limits.md`.

Issue 34 requires documentation of exit statuses, but its automated check compares the README with values produced by Issue 9's search-outcome function. That function is framed around completed process/integrity/result outcomes; cancellation is a later override, and invocation/start failures also sit outside the ordinary search-outcome inputs. A README containing only 0/1/2 could satisfy the described value extraction while omitting the user-important cancellation status 130 and its `q`-while-searching/`ctrl+c` conditions.

**Recommended correction:** synchronize documentation against an explicit public exit-status table covering 0, 1, 2, and 130 with their state-specific meanings, or add direct assertions for cancellation and pre-TUI usage/start failures alongside the outcome-function check.

## Coverage assessment

Numbered-story mapping is complete, but story references alone do not close cross-module timing and composition contracts.

| Area | Assessment |
|---|---|
| CLI syntax, flags, roots, child argv | Well covered. |
| Search indexing, record robustness, outcomes | Strong after the revised consolidated matrix. |
| Cancellation, child cleanup, terminal restoration | Strong; real PTY restoration and controlled-failure ownership are now explicit. |
| Diagnostic safety and replay | Strong sink coverage; the PTY collection-boundary race in F7 remains. |
| Navigation, anchors, wrapping, final target geometry | Detailed, but the load/layout/reveal pipeline in F2 remains a central gap. |
| Horizontal pan/reveal | Product authority and wide-cluster boundary behavior need F1/F4 correction. |
| Async loading/reload/stale handling | Individual cases are strong; two-stage prepared-layout integration remains incomplete. |
| File-list and filename-row layout | List behavior is strong; status-note composition lacks a final owner. |
| Modal behavior and too-small recovery | Strong after revisions; add no-results help coverage. |
| Documentation and operational limits | Faithful overall; explicitly synchronize cancellation status 130. |
| Dependency graph | Much improved, but not yet an independently executable contract. |

## Recommended revision order

1. Put the horizontal extent policy in the PRD, or return it to HITL decision-making.
2. Define and test the two-stage load/layout/reveal commit contract, including latest intent and obsolete completions.
3. Repair the remaining direct dependency mismatches exposed by that contract.
4. Correct the wide-cluster maximum-pan and too-wide-target contradictions.
5. Close filename-status layout integration and no-results help behavior.
6. Make the stderr-replay subprocess boundary deterministic and extend documentation status synchronization.
7. Re-run the story, implementation-decision, testing-decision, and dependency-closure audits before generating tasks.

No new major user-facing feature is needed. The issue set needs authoritative placement of one product decision and tighter ownership of existing cross-boundary behavior.