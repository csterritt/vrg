# PRD issues critique 4 — VRG

## Scope and verdict

**Reviewed:** `Notes/skills/critique-issues/SKILL.md`, `Notes/PRD-vrg.md` revision 4, all 34 Markdown files in `Notes/issues/`, and critique 3 to assess what was fixed and what remains.

This is an **issues-only review**. It does not review implementation, generate tasks, or modify the PRD/issues.

**Verdict: make a small targeted revision before task generation.** The issue set is now exceptionally thorough. All 85 user stories are mapped, the critique-3 fixes are present and sound, and the difficult contracts — safe presentation before first render, dual-pipe drainage from first spawn, the schema/lifecycle matrices, the re-entry retry sequence, controlled-failure exactly-once, the two-stage load→layout→reveal commit, and the outcome matrix — are all explicitly owned with deterministic verification. No core feature is missing, and no broad redesign or new umbrella issue is warranted.

The remaining concerns are narrower than those in critiques 1–3. Two should be fixed before treating each issue as independently mergeable: tab structural expansion is misattributed to Issue 22 when Issue 16 implements it, and no issue explicitly owns layout preparation when navigating to a cached file whose prepared layout is stale. The rest are low-severity edge-case ambiguities and a dependency over-specification that can be corrected locally.

### What is good

- Every numbered user story from 1 through 85 is referenced by at least one issue. Split ownership of stories 8, 21, 25, 34, 75 is explicit and sensible.
- **Critique-3 F1 is resolved.** Issue 5 now lands the core path/content sanitizer and hostile-fixture raw-output tests before any arbitrary-data render. Issue 6 generalizes the core into the shared utility and establishes the extensible sink-safety table. The safety boundary is in place from the first browse render.
- **Critique-3 F2 is resolved.** Issue 3 now owns concurrent dual-pipe drainage from the first spawn, with a ≥1 MiB backpressure test. Issue 9 reuses the fixture shape for diagnostic-content assertions. The child-process foundation cannot deadlock on either pipe.
- **Critique-3 F3 is resolved.** The revision-4 PRD adopts the geometric-reveal fallback for an unpaintable target cluster in *Navigation, viewport, and logical anchors*. Issue 19 references the PRD's authoritative contract rather than inventing an issue-local exception.
- **Critique-3 F4 is resolved.** Issue 3 includes a per-record schema validation matrix (required fields, range constraints, intentionally ignored fields) based on the ripgrep 15.x JSON schema. Issue 9 includes a lifecycle transition matrix (every begin/match/end/summary/context transition and its disposition). Issue 10 asserts every row of both matrices.
- **Critique-3 F5 is resolved.** Issue 26 now specifies a deterministic five-step re-entry sequence: prior failure shown immediately, panel switches to "Loading…", one retry starts while the overlay is open, placeholder updates on settlement, and a second failure appends one occurrence. A gated model test verifies the slow-retry transition.
- **Critique-3 F6 is resolved.** Issue 4's controlled-failure diagnostic uses a single post-restoration stderr writer. Issue 11 generalizes it into the session-collection replay with exactly-once across mechanisms. The PTY tests count the application-failure diagnostic across both the direct-write and replay paths.
- **Critique-3 F7 is resolved.** Issue 34 now specifies `README.md` at the repository root as the single documentation artifact. Acceptance criteria require ripgrep 15.x reference family and `--no-config` documentation.
- **Critique-3 F8 is resolved.** Issues 1 and 2 now include `./-` root acceptance, empty-pattern forwarding, and combined-short `-u` boundary cases (`-iu`, `-iuu`, `-iuuu`) in both manual and automated coverage.
- **Critique-3 F9 is partially addressed.** Issue 1 separates the human-decision governance item into a `Definition of done` section. Several other issues (9, 11, 17, 28) still mix behavioral, architectural, and governance assertions in `Acceptance criteria`, but the stable performance/responsiveness invariants are justified by PRD contracts.
- The dependency graph is a DAG with no cycles. The outcome-transition matrix is a single table-driven test that later issues extend rather than duplicate. The key-binding table (Issue 31) and flag allow-list (Issue 2) are single sources of truth consumed by Issue 34's documentation tests.
- The issue set stays within version-1 scope. It does not accidentally add streaming browsing, direct list selection, load cancellation, cache eviction, encoding transcoding, or a diagnostics-review UI.

### Severity definitions

- **High:** an independently mergeable issue can introduce a security/correctness failure, or issue behavior materially conflicts with the authoritative PRD.
- **Medium:** an important state transition, schema rule, or acceptance boundary permits divergent implementations.
- **Low:** a local traceability, governance, or verification omission that is unlikely to change the architecture.

## Findings

### F1 — Medium: tab structural expansion is misattributed to Issue 22; Issue 16 implements it

**Affected:**

- `Notes/issues/005-browse-tracer-file-list-and-file-panel.md`
- `Notes/issues/016-wrap-mode-and-toggle.md`
- `Notes/issues/022-line-terminators-final-line-empty-file-utf8-bom.md`

Issue 5 says: "tab is never emitted raw in this first pass (a safe placeholder rendering is acceptable; the structural eight-column-stop rule is Issue 22)." This points the implementer to Issue 22 for the structural tab rule.

Issue 22's "What to build" section covers line terminators (LF/CRLF), standalone CR, missing final newline, trailing newline, empty file, zero-width position mapping, and leading UTF-8 BOM. **It does not mention tabs at all.** The structural eight-column-stop rule is not in Issue 22.

Issue 16 says: "Tabs expand to the next multiple of 8 source-display columns independent of gutter and pan." Issue 16 implements the structural tab rule as part of wrap mode. This is consistent with the PRD's *Text, graphemes, and safe presentation* bullet: "Tabs expand to the next multiple of 8 source-display columns, independent of gutter and horizontal pan."

The misattribution in Issue 5 could cause an implementer to defer tab expansion to Issue 22 (which never does it) and discover the gap only when Issue 16's tests fail. It also means the Issue 5 → Issue 22 dependency for tab expansion is wrong — Issue 5 should point to Issue 16 for the structural rule, or Issue 22 should explicitly state that tab expansion is not its scope.

**Recommended correction:** change Issue 5's parenthetical to "the structural eight-column-stop rule is Issue 16" (or simply "a later issue"). Optionally add a one-line note to Issue 22 clarifying that tab expansion is owned by Issue 16, not Issue 22, to prevent the reverse confusion.

**Ready when:** Issue 5's forward reference for tab expansion points to the issue that actually implements it.

### F2 — Medium: no issue explicitly owns layout preparation when navigating to a cached file with a stale layout

**Affected:**

- `Notes/issues/017-logical-anchor-through-rewrap-and-resize.md`
- `Notes/issues/028-load-completion-reveal-latest-target.md`
- `Notes/issues/013-match-navigation-n-p-circular-cursor.md`

The prepared-layout contract has two documented triggers:

1. **Resize or wrap toggle** (Issue 17): "`Update` for a resize or `w` records the new dimensions/mode, keeps the logical anchor, requests preparation, and returns."
2. **Load completion** (Issue 28): "If P is current, first recompute the text width… then request prepared layout for the current (P, revision, text width, wrap mode)."

Neither covers the case where the user navigates to a **cached** file whose prepared layout is stale. Consider: the user views file A at width 80, resizes to 100 (Issue 17 requests a new layout for A), navigates to file B (cached, last prepared at width 80), and B's layout is now stale. There is no load completion for B (it is cached), and the resize handler requested a layout only for the then-current file (A). Who requests a layout for B at width 100?

Issue 13 says "Crossing to another file's stop switches the panel, triggers that file's load if not cached, saves the departing file's viewport and starts the new file from its saved viewport." It does not mention requesting a layout preparation for a cached file whose layout doesn't match current parameters.

Issue 17 says obsolete layouts are discarded: "a layout for file A arriving after navigation to B does not touch B's panel." This means layouts are not cached for non-current files — they are discarded on arrival. So navigating to a cached file B always needs a layout if B was not current when its layout arrived.

If the model does not request a layout for B, the panel either shows B's stale layout (wrong width) or a placeholder with no layout ever arriving. The intent (reveal or saved-viewport) cannot commit without a matching layout.

**Recommended correction:** add an explicit trigger to Issue 13 or Issue 17: when the current file changes and the cached file has no installed layout matching the current (path, revision, width, mode), the model requests a prepared layout. The intent (entry reveal or saved-viewport) is carried and commits when the matching layout installs, per Issue 28's two-stage contract. Add a gated model test: navigate to a cached file after a resize, with the layout worker gated, and verify the intent commits when the layout releases.

**Ready when:** every navigation to a cached file either finds a matching installed layout or requests one, and the intent commits against it.

### F3 — Low: Issue 24's direct dependency on Issue 20 appears unnecessary

**Affected:**

- `Notes/issues/024-file-list-layout-width-truncation-toggle.md`

Issue 24 is blocked by Issues 17 and 20. The width formula uses "reserved indicator width," which is defined in Issue 16: "reserved right-indicator width (0 in wrap mode, 1 in run-off-edge mode)." Issue 20 populates the indicator column but does not change its width — the column is reserved even when blank.

Issue 24 transitively depends on Issue 16 through Issue 17 (17 → 16). The direct dependency on Issue 20 adds the chain 18 → 19 → 20 to Issue 24's critical path. If Issue 24 only needs the reserved column width (from Issue 16) and the prepared-layout path (from Issue 17), the direct dependency on Issue 20 lengthens the critical path without adding a required input.

This does not cause correctness issues — Issue 24 simply waits for Issue 20 unnecessarily. But if the dependency graph is used for parallelization or milestone planning, the extra dependency makes Issue 24 appear later than it could start.

**Recommended correction:** drop Issue 20 from Issue 24's `Blocked by` unless there is an unstated reason Issue 24 needs the indicator *logic* (not just the reserved width). If the reason exists, document it in Issue 24's "What to build."

**Ready when:** Issue 24's dependencies match its actual inputs.

### F4 — Low: orphaned match after a binary-excluded end — retention contradicts exclusion

**Affected:**

- `Notes/issues/009-error-overlay-and-fatal-outcomes.md`
- `Notes/issues/008-no-results-screen-and-binary-exclusion.md`

Issue 9's lifecycle transition matrix says: "`match(P)` while P is not open (never opened, or after its `end`) → Integrity failure (orphaned/inconsistent match); P's matches are retained with incomplete metadata."

Issue 8 says: "a valid `end` event with non-null `binary_offset` drops that file and all its previously collected matches."

If a file P receives a valid `end` with non-null `binary_offset` (binary exclusion), and a `match(P)` arrives afterward, the transition matrix says P's matches are "retained with incomplete metadata." But P was binary-excluded — its earlier matches were dropped. Does the late match get retained (contradicting the binary exclusion) or dropped (contradicting the "retained" language)?

This is an edge case that ripgrep is unlikely to produce in practice, but the two rules conflict on paper. An implementer could reasonably choose either disposition.

**Recommended correction:** add a clarifying note to Issue 9's transition matrix: a `match(P)` after a binary-excluding `end(P)` is an integrity failure, and the late match is not retained (binary exclusion takes precedence over the general retention rule for orphaned matches). Alternatively, state that the late match is retained but the file remains binary-excluded from the file list, so the retained match is not browsable.

**Ready when:** the transition matrix and binary exclusion rule do not conflict on a late match after a binary end.

### F5 — Low: oversized trailing unterminated record — count interaction undefined

**Affected:**

- `Notes/issues/010-record-robustness-malformed-oversized-unknown.md`
- `Notes/issues/003-spawn-rg-collect-results-searching-screen.md`

Issue 10 says "a record can be both [malformed and integrity-failing] only in the two cases those matrices mark 'both' (trailing unterminated record; a malformed record appearing after `summary`)."

An oversized record is consumed and discarded through the next newline. If the oversized record has no trailing newline (it is the last record in the stream), it is also a trailing unterminated record, which Issue 9's matrix marks as "both" (malformed + incomplete). Is this record counted as oversized, malformed, or both? Does the oversized count include it, or does the malformed count include it, or both?

The PRD says: "A trailing unterminated record is counted as malformed and makes the stream incomplete." And: "Consume and discard an oversized record through the next newline, count it separately." If there is no next newline, the oversized consumption never completes, and the record is also trailing-unterminated. The two counting paths interact in an undefined way.

**Recommended correction:** state explicitly that an oversized record without a trailing newline is counted as oversized (the oversized path takes precedence, since the limit is hit before the newline is sought) and also makes the stream incomplete (trailing unterminated). Or state that it is counted as malformed only (the trailing-unterminated path takes precedence). Either choice is defensible; the ambiguity is not.

**Ready when:** an oversized record without a trailing newline has a deterministic count disposition.

### F6 — Low: unknown event type after `summary` — counted as unknown?

**Affected:**

- `Notes/issues/009-error-overlay-and-fatal-outcomes.md`
- `Notes/issues/010-record-robustness-malformed-oversized-unknown.md`

Issue 9's transition matrix says: "Any record after `summary` → Integrity failure (also skipped/counted if the record is itself malformed)." Issue 10 says unknown event types are "counted separately" and "never changes exit status alone."

An unknown event type after `summary` is an integrity failure. Is it also counted in the unknown-type count? Issue 10 says "a record can be both only in the two cases those matrices mark 'both'" — and an unknown type after summary is not one of those two cases. So it should be an integrity failure only, not counted as unknown. But the issue does not explicitly state this, and an implementer might count it in both buckets.

**Recommended correction:** add a note to Issue 10 clarifying that an unknown event type appearing after `summary` is an integrity failure and is not additionally counted in the unknown-type count (it is skipped by the after-summary rule, not by the unknown-type rule).

**Ready when:** an unknown event after `summary` has a deterministic count disposition.

### F7 — Low: Issue 5's temporary tab placeholder rendering is unspecified

**Affected:**

- `Notes/issues/005-browse-tracer-file-list-and-file-panel.md`

Issue 5 says: "tab is never emitted raw in this first pass (a safe placeholder rendering is acceptable; the structural eight-column-stop rule is Issue 22 [sic — see F1])."

The "safe placeholder rendering" is not specified. It could be spaces, `→`, `»`, or any other non-raw representation. Different choices produce different byte→cell mappings, which affects highlight placement between Issue 5 and Issue 16. An implementer who chooses spaces (width 1 per tab) and then switches to 8-column stops in Issue 16 would see highlight positions shift, potentially breaking Issue 5's tests if they assert specific cell positions for tab-containing lines.

This is temporary and replaced by Issue 16, but the placeholder should be specified to avoid test churn.

**Recommended correction:** specify the placeholder rendering (e.g., "tabs render as a single `→` glyph occupying one cell" or "tabs render as one space per tab") and note that byte→cell mappings for tabs are provisional until Issue 16. Alternatively, state that Issue 5's tests do not assert highlight positions on tab-containing lines, defending those tests to Issue 16.

**Ready when:** the tab placeholder between Issue 5 and Issue 16 is deterministic and does not cause test churn.

## Coverage and readiness assessment

| Area | Assessment |
|---|---|
| CLI syntax, roots, flags, child argv | Complete. The `./-`, empty-pattern, and combined-`u` boundary fixtures from critique 3 are present. |
| Child lifecycle, cancellation, terminal restoration | Strong. Dual-pipe drainage from first spawn; controlled-failure exactly-once across mechanisms; PTY harness with reap evidence. |
| Search parsing, integrity, outcomes | Complete. Per-record schema matrix (Issue 3) and lifecycle transition matrix (Issue 9) make "malformed" and "integrity failure" deterministic. Two edge-case count interactions are unresolved (F5, F6). |
| Safe presentation | Strong. Core sanitizer lands in Issue 5 before first arbitrary-data render; Issue 6 generalizes with an extensible sink-safety table. |
| Browse layout, theme, navigation, wrapping, anchors | Strong and internally well integrated. Tab ownership misattribution (F1) and cached-file layout gap (F2) are the only structural concerns. |
| Horizontal pan/reveal/indicators | Revision-4 visible-lines policy and paintable-boundary maximum are reflected. Geometric-reveal fallback for unpaintable target clusters is PRD-authoritative. |
| Async loading, reload, stale/unsupported content | Detailed. Re-entry retry sequence is deterministic. Two-stage load→layout→reveal commit is coherent. Cached-file layout preparation gap is F2. |
| Overlays, help, pop-ups, too-small recovery | Complete, including state precedence, scroll restoration, `Esc` semantics, and too-small `q`-exits-over-overlay rule. |
| Documentation and resource limits | Complete. README.md at repo root; four-status exit table; ripgrep 15.x and `--no-config` documented; binding table and flag list synchronized. |
| Dependency graph | DAG with no cycles. One unnecessary direct dependency (F3) lengthens Issue 24's critical path. |

## Disposition of critique 3

All eight findings from critique 3 are resolved in the current issue set:

| Critique-3 finding | Current disposition |
|---|---|
| F1: arbitrary bytes rendered before sanitizer | Resolved. Issue 5 lands the core sanitizer and hostile-output tests before first render. |
| F2: stderr deadlock before Issue 9 | Resolved. Issue 3 owns concurrent dual-pipe drainage from first spawn with a backpressure test. |
| F3: unpaintable target-cluster authority gap | Resolved. Revision-4 PRD adopts the geometric-reveal fallback; Issue 19 references it as authoritative. |
| F4: schema/lifecycle matrix absent | Resolved. Issue 3 has the per-record schema matrix; Issue 9 has the lifecycle transition matrix. |
| F5: re-entry retry sequence undefined | Resolved. Issue 26 specifies a five-step deterministic sequence with a gated model test. |
| F6: controlled-failure diagnostic double-write | Resolved. Issue 4 uses a single post-restoration writer; Issue 11 generalizes with exactly-once. |
| F7: Issue 34 artifact/acceptance mismatch | Resolved. Issue 34 specifies README.md at repo root with ripgrep 15.x and `--no-config` criteria. |
| F8: invocation boundary cases omitted | Resolved. `./-`, empty pattern, and combined-`u` cases are in Issues 1 and 2. |
| F9: acceptance criteria mix governance and behavior | Partially addressed. Issue 1 separates `Definition of done`. Other issues still mix but the invariants are PRD-justified. |

## Recommended revision order

1. Fix Issue 5's tab forward reference to point to Issue 16 (F1).
2. Add an explicit layout-preparation trigger for navigation to a cached file with a stale layout (F2).
3. Drop Issue 20 from Issue 24's `Blocked by` unless a stated reason exists (F3).
4. Resolve the three edge-case count interactions: orphaned match after binary end (F4), oversized trailing unterminated record (F5), unknown type after summary (F6).
5. Specify Issue 5's temporary tab placeholder rendering (F7).
6. Re-run a dependency-closure and acceptance-coverage check, then proceed to task generation.

The issue set does **not** need more feature scope, a new umbrella issue, or further restructuring. The remaining work is to fix one misattribution, close one layout-preparation gap, and settle a handful of edge-case ambiguities — all local corrections that do not change the architecture or the dependency graph's shape.
