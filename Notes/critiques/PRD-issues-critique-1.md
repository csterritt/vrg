# PRD issues critique 1 — VRG

## Scope and verdict

**Reviewed:** `Notes/PRD-vrg.md` revision 4, all 34 Markdown files in `Notes/issues/`, and `Notes/Ideas.md` as historical context. Applied `Notes/skills/critique-issues/SKILL.md`. The current PRD, not the original ideas or superseded critiques, is the requirements authority.

This is an **issues-only review**: no implementation review, task generation, or changes to the PRD/issues. Small read-only checks were used to inspect dependencies/story coverage and verify a questionable ripgrep example.

**Verdict: revise before task generation.** The decomposition is broadly strong and accounts for every numbered user story, but its dependency graph does not support the claimed independently executable issues. Several acceptance criteria contradict the PRD, and important cross-boundary responsiveness and cleanup tests have no sufficiently explicit owner. These are targeted corrections, not grounds to discard the issue set.

### What is good

- All **85 user stories** are referenced in at least one issue. There is no wholly unmapped numbered story.
- The issues generally have clear scope, verification guidance, acceptance criteria, and PRD references.
- The progression from invocation to search to a browse tracer is sensible. Explicit interim behavior in Issues 1–5 is acceptable when its replacement is clearly owned.
- Difficult requirements have not been forgotten: raw path identity, binary exclusion after earlier matches, missing completion metadata, record limits/resynchronization, diagnostic replay, dropped duplicate loads, stale validation, UTF BOM handling, logical anchors, and marker geometry are represented.
- Issue 4 calls for a real subprocess/PTY harness; Issue 15 uses instance-keyed timers; Issues 25–28 use controlled load completions instead of timing-dependent tests.
- Revision-4 clarifications are mostly reflected, including late diagnostic limitations, dropped reloads, too-small modal recovery, ordinary CRLF markers, and popup recentering.
- The issues do not improperly add streaming browsing, direct file selection, cache eviction, diagnostic-review UI, or encoding transcoding.

### Severity definitions

- **High:** missing ownership or conflicting requirements likely to produce a materially incorrect application or an issue that cannot be completed from its declared prerequisites.
- **Medium:** an important edge case, acceptance ambiguity, or verification gap that permits an incorrect implementation to pass.
- **Low:** misleading examples or local terminology errors that should be corrected before they become tests.

## Findings

### F1 — High: the dependency graph is not a reliable execution contract

**Affected:** the `Blocked by` fields throughout `Notes/issues/`, especially Issues 4, 6, 7, 11, 13, 15, 21, 32–34.

The graph has no cycles or nonexistent issue references, but several consumers can start before infrastructure that their own scope or verification requires. Examples:

| Consumer | Declared prerequisites | Missing prerequisite or explicit integration ownership |
|---|---|---|
| Issue 11, stderr replay | Issue 9 | Uses the Issue 6 sanitizer and requires the Issue 4 terminal-restoration/PTY boundary; neither is in its dependency ancestry. |
| Issue 13, navigation | Issue 12 | Explicitly requires the underline style from Issue 7, which is not an ancestor. |
| Issue 15, popup | Issue 13 | Requires safe path presentation and error/help cancellation; Issues 6, 9, and 31 are not ancestors. Its verification already calls for opening help. |
| Issue 21, grapheme highlights | Issue 19 | Says Issue 20 indicators use expanded spans, but Issue 20 is a sibling rather than a prerequisite. |
| Issue 32, modal precedence | Issues 31 and 15 | Tests too-small input behavior, which is introduced only by its dependent Issue 33. |
| Issue 34, documentation | Issue 10 | Requires a help footer and a test extracting bindings from help, but Issue 31 is not an ancestor. |

More generally, **no issue depends on Issue 4**, and **no issue depends on Issue 7**. The sanitizer in Issue 6 is an ancestor only of Issues 22, 23, and 29, despite being the shared requirement for every output sink. Numeric file order currently hides these defects; it is not encoded as a merge constraint.

Not every mention of a future issue is a dependency error. The deliberately deferred reveal in Issue 13 and layout in Issue 5 are clearly labeled. The problem is asking an issue to deliver or test a feature that is neither available nor explicitly deferred.

**Recommended correction:** audit each issue against the transitive dependency closure. Add actual prerequisites where the feature is needed now; move cross-feature verification to a later integration owner where adding an edge would create a cycle. In particular, make cancellation/cleanup, sanitization, and theme integration explicit prerequisites or explicitly owned follow-ups. Avoid solving this by mechanically making every issue depend on all lower numbers.

**Ready when:** an issue can be implemented and verified using only its declared completed prerequisites, with every deliberately deferred behavior assigned to a later issue.

### F2 — High: expensive rewrapping and obsolete layout completion have no clear owner

**Affected:**
- `Notes/issues/003-spawn-rg-collect-results-searching-screen.md`
- `Notes/issues/005-browse-tracer-file-list-and-file-panel.md`
- `Notes/issues/012-manual-vertical-scrolling-and-per-file-viewport.md`
- `Notes/issues/016-wrap-mode-and-toggle.md`
- `Notes/issues/017-logical-anchor-through-rewrap-and-resize.md`
- `Notes/issues/025-async-load-isolation.md`

**PRD basis:** *Resources and responsiveness* prohibits unbounded disk I/O, parsing, sorting, decoding, mapping, and rewrapping on the UI update path. *Testing Decisions → Responsiveness boundaries* explicitly calls for controlled gates around search processing, loading, decoding, and layout preparation, plus obsolete-result isolation.

The issues cover off-thread search collection, asynchronous reads, and prepared frame data. Issues 16–17, however, describe wrap/resize behavior without assigning asynchronous preparation, cancellation responsiveness, or stale prepared-layout handling. Issue 25 isolates **file-load** results only. It does not establish what happens when a layout prepared for a previous width, wrap mode, content revision, or selection finishes late.

An implementation could satisfy these issue tests while rewrapping a large buffer synchronously in `Update`, or while installing an old-width row map after a newer resize. Merely rendering from prepared rows does not prove that preparing those rows leaves the UI responsive.

**Recommended correction:** explicitly assign the missing behavioral contract and verification to the wrap/anchor or async issues, or a focused integration issue. Include:

- Hold sorting/index preparation after rg has exited; `q` must still cancel with 130 rather than being treated as browse quit.
- Hold decoding/mapping and layout preparation; relevant normal input and `ctrl+c` remain actionable.
- Complete preparations out of order after resize, wrap toggle, file change, and reload. Obsolete data must not replace the current display or its anchor.
- Ensure frame rendering does not recompute full-buffer mappings or scan the whole file list for every frame.

This does not require prescribing channels, worker pools, or private message fields. It requires owning the externally observable responsiveness and late-result contract.

### F3 — High: Issue 32 contradicts fatal-overlay exit behavior

**Affected:** `Notes/issues/032-overlay-precedence-esc-semantics.md`, particularly its final acceptance criterion and the unqualified statement “Esc never exits.” Also relevant: Issues 9, 10, and 33.

Issue 32 says:

> Given an overlay is open, when `q` is pressed, then only the overlay closes.

That is false for a fatal no-results overlay. The PRD outcome table, Issue 9, and Issue 10 require dismissal with **either `q` or `Esc` to exit 2** when there is no usable browse state. Issue 33 also correctly makes `q` exit from the too-small screen even when a modal overlay is logically open.

The PRD itself uses the shorthand “Esc never exits,” but its more specific outcome table explicitly allows fatal-overlay dismissal to terminate the application. Issue 32 reproduces that tension as an unconditional test expectation instead of preserving the specific outcome rule.

**Recommended correction:** scope the acceptance criteria by base state and overlay continuation:

- Browse overlay → dismiss back to browsing or suspended help.
- Nonfatal warning over an empty result → dismiss to the no-results screen.
- Fatal no-results/record-loss overlay → dismissal exits 2.
- No overlay → `Esc` does not request exit.
- Too-small → dedicated `q` exit rule takes precedence over hidden modal state.

Describe `Esc` as “not a standalone quit command,” rather than claiming it can never lead to termination. If the product owner intended a different interpretation, resolve it explicitly rather than allowing the later issue to override the outcome table.

**Verification needed:** the same modal-routing suite should exercise both dismissal keys across these outcome classes. The current “q dismisses before quitting” example is not a universal oracle.

### F4 — Medium: Issue 28's startup acceptance criterion loses the visible-target no-scroll rule

**Affected:** `Notes/issues/028-load-completion-reveal-latest-target.md`, first acceptance criterion and automated startup example.

The scope correctly says the first match lands one third down **“if not otherwise visible from top-of-file.”** Its acceptance criterion instead requires the startup match to be placed at `floor(h/3)` “or clamped” without that condition.

For example, with content height 12 and the first target at rendered row 8, starting at top 0 already reveals it. The PRD and Issue 14 require top 0 to remain unchanged. Unconditional one-third placement would move the top to 4. This is not a BOF/EOF clamp case; both positions can be valid in a long file.

**Recommended correction:** qualify one-third placement with “when the target row is hidden from the starting viewport.” Explicitly preserve the visible-target no-scroll rule at startup and at current-file completion.

**Verification needed:** two startup fixtures, one already visible and one hidden, plus a saved-viewport completion case. Keep ordinary reload-without-navigation as the separate no-reveal exception already described in Issues 27–28.

### F5 — Medium: the claimed complete outcome coverage omits important combinations

**Affected:**
- `Notes/issues/008-no-results-screen-and-binary-exclusion.md`
- `Notes/issues/009-error-overlay-and-fatal-outcomes.md`
- `Notes/issues/010-record-robustness-malformed-oversized-unknown.md`
- `Notes/issues/026-read-failures-unreadable-retry-rules.md`
- `Notes/issues/029-stale-match-validation-and-file-changed-note.md`
- `Notes/issues/030-unsupported-encodings-utf16-utf32.md`

Issues 9–10 collectively claim the outcome table, but the concrete cases do not fully carry it through:

1. Issue 9 explicitly covers stderr warnings **with** usable results; Issue 10 explicitly covers unknown-type warnings **without** results. Neither clearly owns the **stderr-only warning, complete stream, zero retained results** transition to warning overlay → no-results → exit 1.
2. A complete stream with retained results and anomalous rg exit 1 must still browse successfully and exit 0. The PRD calls this out, but there is no explicit fixture for it.
3. Safe record loss plus binary exclusion can leave zero retained stops. This must be assessed after filtering, not from the number of received match events.
4. “Usable results” is based on retained stops, not successfully loaded files. Issue 26 asserts read failures do not change status, but the same rule lacks explicit verification for all-stale or all-unsupported results and for preserving an already-fatal search status.
5. “Fixed exit status never changes” needs the PRD's explicit `ctrl+c` override to 130; otherwise later outcome code can accidentally suppress cancellation.

**Recommended correction:** assign one issue responsibility for a consolidated outcome/transition test matrix, with later loaders extending it rather than redefining the decision. Include the cases above, fatal versus warning dismissal, both rg 0/1, and cancellation after search completion. The matrix can be small and table-driven; it need not enumerate every irrelevant Cartesian combination.

### F6 — High: cleanup verification is weaker than the PRD's controlled-failure contract

**Affected:**
- `Notes/issues/004-cancellation-child-cleanup-terminal-restore.md`
- `Notes/issues/009-error-overlay-and-fatal-outcomes.md`
- `Notes/issues/011-stderr-replay-of-collected-diagnostics.md`

The subprocess harness is a good start, but its written checks leave several consequential gaps:

- Issue 4 tests **a restoration escape sequence**, which does not prove terminal input modes were restored. A terminal can receive an alternate-screen exit sequence and still be left in raw/no-echo mode.
- A child being “gone” does not by itself establish that the wait/reap path ran. The harness needs observability appropriate to the termination/reaping requirement, rather than treating one condition as both.
- Issue 4 scopes cleanup to cancellation and ordinary exit. Issue 11 mentions replay on controlled application failure, but neither clearly owns cleanup when terminal/TUI startup or execution fails **after rg has started**. The PRD App contract specifically includes terminal failures.
- Issue 9 asks for concurrent stderr draining but tests a small diagnostic/nonzero exit. That does not catch a pipe backpressure deadlock.
- Issue 11 asks for replay of diagnostics collected immediately before exit but its main model test starts with three already-collected diagnostics. It does not establish the shutdown boundary against the real drain/collection path.

**Recommended correction:** strengthen the existing boundary-test ownership to include:

- PTY mode restoration as well as application display restoration on normal quit, cancellation, and injected controlled failure.
- Actual child termination and evidence that the application completes its wait/reap path.
- A terminal startup/execution failure after child start, with no surviving child and safe diagnostics.
- A fake rg writing more than pipe capacity to stderr while stdout also progresses, coordinated by explicit handshakes.
- Replay of an occurrence confirmed collected immediately before cancellation, once and after restoration; no waiting for unrelated file loads just to collect future errors.

Keep these deterministic and handshake-driven. No reliable OOM recovery or forced-termination cleanup should be added; both are deliberately outside scope.

### F7 — Medium: horizontal clamping adds an unapproved policy, while wide-glyph reveal remains ambiguous

**Affected:**
- `Notes/issues/018-horizontal-panning.md`
- `Notes/issues/019-minimal-horizontal-reveal.md`
- `Notes/issues/021-grapheme-cluster-highlight-expansion.md`

Issue 18 introduces clamping against the **“widest visible line's effective extent.”** The PRD says to clamp to valid extents but does not select a visible-lines-only rather than file-wide policy. This is observable: scrolling vertically from a long line to a short one could clamp away an offset that would still be valid on other lines. It also interacts with saved horizontal state, reveal, and wrap toggles.

The upper-bound wording is incomplete even under a chosen policy: clamping to the line's width itself can permit an entirely blank text area. The issue should distinguish a content extent from a maximum valid viewport offset and include marker extent.

Issue 19 also fixes rightward reveal to `target − (text width − 1)`. For a two-cell cluster this can put its start in the final text column, where Issue 18 requires clipping to blanks. The PRD simultaneously requires target-start revelation and grapheme-safe clipping, so this is an inherited boundary ambiguity, not justification to silently replace minimal scrolling with a new product rule.

**Recommended correction:**

- Resolve or explicitly flag the horizontal extent policy rather than introducing “visible” incidentally in an AFK issue.
- Specify the maximum-offset behavior with short lines, mixed-width files, empty content, and EOL markers.
- Add a cross-boundary wide-glyph-at-right-edge reveal example and clarify whether the existing “start cell visible” rule is geometric or requires a painted target cell. If the requirements cannot be satisfied together, return that specific choice to the product owner.
- Qualify Issue 18's unconditional “`w` twice leaves offset unchanged” criterion with the PRD's actual-display-clamping exception.

### F8 — Medium: sanitization verification can erase the evidence it is supposed to detect

**Affected:** `Notes/issues/006-safe-presentation-utility-for-all-sinks.md`, automated verification; related sinks in Issues 9–11, 15, 31, and 34.

Issue 6 suggests checking `View()` for raw escapes beyond app styling by comparing against a **stripped-style render**. If the stripping operation removes arbitrary ANSI/OSC controls, an injected control sequence can disappear from the test observation. That method alone cannot prove the terminal-safety contract.

Other acceptance language overstates the PRD: invalid UTF-8 highlights are required to “map to the right cells,” whereas the PRD explicitly accepts best-effort highlighting for invalid input. Exact raw path preservation and control escaping remain mandatory; arbitrary invalid-byte highlight exactness does not.

There is also a local ambiguity in “tab/LF/CR handled structurally.” Only LF and **CRLF** are specified line endings. A standalone CR must not slip through as an unescaped cursor-control byte or be silently discarded under a blanket CR exemption.

**Recommended correction:**

- Assert sanitizer outputs and raw composed output before generic ANSI stripping. Where app styles are enabled, distinguish trusted style sequences from fixture-supplied controls; a no-style composition path can also help.
- Exercise malicious content and filenames through the actual list, filename row, popup, diagnostic overlay, usage/start errors, and stderr replay, not only the shared helper.
- Include OSC, CSI, C0/C1 controls, standalone CR, invalid path bytes, and embedded filename newlines. Preserve actual diagnostic line boundaries.
- Keep exact mapping tests for well-defined escape expansions and supported UTF-8, but word invalid-UTF-8 highlight acceptance consistently with the PRD's best-effort limit.

The all-sink claim also needs the dependency/integration repair in F1; Issue 6 cannot validate sinks that do not yet exist without explicit later ownership.

### F9 — Medium: final viewport integration is distributed without a clear acceptance owner

**Affected:** Issues 17, 21–24, and 27–30, especially:
- `Notes/issues/024-file-list-layout-width-truncation-toggle.md`
- `Notes/issues/028-load-completion-reveal-latest-target.md`
- `Notes/issues/029-stale-match-validation-and-file-changed-note.md`

The pieces have reasonable unit cases, but the issues do not clearly close the final integration loop:

- Issue 24 changes text width when list visibility or gutter size changes, yet its tests focus on the width formula and keys. It does not verify the logical reading anchor through those rewraps. Issue 17 is not its dependency ancestor.
- Marker and expanded-grapheme geometry are introduced after basic reveal, but the dependency graph lets Issue 28 finish without Issues 21–23. Issue 23 covers marker reveal generally; nobody clearly owns late-load/reload reveal using the final geometry.
- Issue 29 changes the target to a surviving submatch or stale fallback, but its dependency graph does not include Issue 28. The integrated case—latest selected stop changes during reload, its first recorded submatch is dropped, and completion must reveal the first survivor—has no explicit owner.
- The PRD specifically requires navigation away and back during reload to use normal entry reveal. Issue 28 says navigation overrides preservation but does not concretely test an away-and-back trip or same-file A→B→A. Comparing only initial and final cursor values would miss the navigation intent.

**Recommended correction:** assign these focused cross-feature cases to the issue that integrates the last necessary feature, with appropriate dependencies. Include list hide/show and gutter-growth anchor preservation, an async marker/grapheme target, an async stale-survivor/fallback target, and navigation returning to the original stop during reload. Do not require private generation-counter designs; assert the final visible behavior.

### F10 — Low: several verification instructions have incorrect or ambiguous expected results

These should be corrected before implementers translate them into automated assertions.

| Issue/file | Problem | Correction |
|---|---|---|
| `021-grapheme-cluster-highlight-expansion.md`, manual test | Searching precomposed `é` does not ordinarily match decomposed `e\u0301`. ripgrep does not supply Unicode normalization here. | Search the combining mark itself (for partial-cluster expansion) or the exact decomposed bytes. A local ripgrep 15.2.0 check returned exit 1 for precomposed `é`, and a match for U+0301. |
| `024-file-list-layout-width-truncation-toggle.md`, manual test | “Shrink to 30 columns → list disappears” contradicts the formula under the accompanying ordinary gutter assumptions. With a five-digit gutter (7 cells) and a right indicator, the list can still be 12 cells wide. | Expect a constrained but nonzero list. Test zero-width allocation separately with synthetic dimensions/gutter values that actually produce it. |
| `022-line-terminators-final-line-empty-file-utf8-bom.md`, automated test | “Empty file → ... gutter width 1” conflicts with the definition used elsewhere: gutter width includes the digit slot **plus two spaces**. | Say one digit slot, three total gutter cells, with no source rows. Avoid overloading `gutter width`. |
| `010-record-robustness-malformed-oversized-unknown.md`, manual fixture | A match, malformed line, unknown event, and summary are described as exiting 0, but the listed stream lacks paired begin/end metadata. | Explicitly include valid begin/end events. Otherwise the documented result is integrity failure and exit 2. |
| `026-read-failures-unreadable-retry-rules.md`, manual test | After a final retry failure, a single `q` dismisses the browse error overlay rather than quitting. `chmod 000` is also not a reliable failure fixture under elevated privileges. | Spell out dismissal then ordinary quit, and use an injected failing loader for automated coverage. |
| `030-unsupported-encodings-utf16-utf32.md`, manual test | The initial unsupported overlay blocks `r`; one `q` after a reload diagnostic can merely dismiss an overlay. | Dismiss the initial overlay before `r`, then dismiss the new overlay before ordinary quit. |
| `032-overlay-precedence-esc-semantics.md`, manual test | The failure-trigger sequence ends with “then…” and is not reproducible. | Replace it with a complete controllable sequence or label the deterministic gated model test as the verification method. |
| `003-spawn-rg-collect-results-searching-screen.md`, manual start-failure test | `PATH=/nonexistent vrg foo` may fail to find `vrg` itself, testing the shell instead of rg startup handling. | Invoke the built VRG binary by explicit path while providing an rg-free PATH. |

Issue 22's empty-file manual example also contains a speculative parenthetical about whether rg can match an empty file. Use a precise fixture (or search first, then truncate before loading) rather than an unresolved question in verification instructions.

## Coverage assessment

Numbered-story coverage is necessary but insufficient. The missing ownership is mainly in implementation and testing contracts that span stories.

| Area | Assessment |
|---|---|
| CLI syntax, flags, roots, child argv | Substantially covered by Issues 1–3. Keep raw operands and sanitized presentation distinct. |
| Search indexing, records, completion metadata | Substantially covered by Issues 3 and 8–10; repair the misleading fixture and complete outcome combinations. |
| Cancellation, cleanup, stderr replay | Explicitly represented, but dependency and real-boundary coverage need F1/F6 corrections. |
| File list, layout, theme | Core behavior covered; anchor integration and incorrect manual layout expectation remain. |
| Navigation, reveal, anchors, panning | Detailed coverage, with concrete contradictions/ambiguities in startup reveal and horizontal extent/visibility. |
| Graphemes, terminators, BOMs, markers | Good specialist coverage; integration and verification examples need repair. |
| Async loads, retry, reload, stale content | Strong individual scopes; layout work and navigation-intent cross-cases remain underassigned. |
| Modal behavior and too-small recovery | Broadly covered, but Issue 32 must not override fatal-dismissal or too-small exit rules. |
| Resource limitations/documentation | Faithful to the PRD; Issue 34's dependency on the help source must be made real. |
| Responsiveness during all expensive processing | **Incomplete issue-level ownership**, despite numbered story coverage. |

## Recommended revision order

1. Correct the contradictory acceptance criteria: fatal-overlay dismissal, visible startup target, horizontal clamping qualifications, and gutter terminology.
2. Repair the dependency graph and explicitly allocate deferred integration tests. Treat numeric order as presentation, not an implicit scheduler.
3. Assign off-UI rewrap/layout preparation, obsolete-result isolation, and the final viewport integration cases.
4. Strengthen controlled-failure/PTY, concurrent stderr, shutdown replay, and all-sink safety verification.
5. Consolidate outcome-transition coverage and fix the manual fixtures.
6. Recheck story coverage **and** PRD implementation/testing clauses after the revisions.

No new user-facing feature is needed to address these findings. The desired result is an issue set whose prerequisites, acceptance criteria, and verification all describe the same revision-4 product.
