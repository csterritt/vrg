# Requirements critique: VRG PRD — review 1

## Scope and verdict

Reviewed:

- `Notes/PRD-vrg.md`
- `Notes/Ideas.md`, the source ideas
- `Notes/skills/critique-prd/SKILL.md`, the review instructions

This is a requirements-only review. It does not generate issues, implementation tasks, or code. References below use PRD section names and user story numbers (US).

**Verdict: revise before issue generation.** The PRD captures much of the intended interaction design, but contains a direct conflict with the source ideas, contradictory highlight requirements, and several gaps that would force implementers to invent observable behavior. The largest risks concern the supported ripgrep invocation contract, navigation state, wrapped scrolling, and error handling.

“Open Questions: None” is not supported by the document as written.

### What is already strong

- Most key bindings, circular match navigation, popup duration, and basic scroll persistence are explicit.
- The source’s two-panel design, default wrapping, horizontal increments, and per-line indicators are largely preserved.
- Terminal-cell measurement, resizing, and subprocess cancellation are recognized rather than ignored.
- The PRD distinguishes search indexing, file content, viewport state, and styling, and proposes deterministic module tests.
- The out-of-scope section provides a useful starting boundary, although it needs reconciliation with the invocation contract.

### Priority definitions

- **High:** conflicting or missing behavior likely to produce materially different implementations or incorrect core functionality.
- **Medium:** important edge cases, acceptance gaps, or operational requirements that should be settled before implementation.
- **Low:** precision or presentation improvements that do not independently block the core design.

## High-priority findings

### H1. Requiring a directory contradicts the source ideas

**References:** Ideas, invocation bullet; PRD Solution, US1–2, Implementation Decisions / Invocation.

The ideas explicitly specify an **optional directory defaulting to `.`**. The PRD makes the directory mandatory and adds a usage error when it is omitted. This is a user-visible change, not an implementation detail, and no rationale or approval is recorded.

**Required clarification:** Restore `vrg [supported-rg-flags] pattern [dir]`, with `.` as the default, or explicitly record an approved departure from the source. Define usage behavior and exit status for missing pattern, invalid flags, and excess positional arguments consistently.

### H2. “Non-argument rg-flags” is not a safe or complete support contract

**References:** Solution; Implementation Decisions / Invocation and rg subprocess; Out of Scope.

The source ideas do not request arbitrary flag forwarding; the PRD adds it. Restricting forwarding to flags without arguments does not guarantee compatibility with JSON results or local-file highlighting. Flags such as `--files`, `--count`, `--files-with-matches`, and `--quiet` can conflict with the required output or suppress it. Other flags, such as `--multiline`, `--search-zip`, or `--invert-match`, change assumptions about what a match means or which content is being searched.

The PRD also omits:

- How patterns beginning with `-` are accepted and distinguished from options.
- Whether combined short flags and `--option=value` forms are accepted.
- Whether ripgrep configuration from the environment is honored or disabled.
- What happens when a supplied flag conflicts with mandatory JSON output.

**Required clarification:** Define a supported flag set or an explicit compatibility policy, including rejection behavior. Define operand parsing and configuration precedence. A narrower supported search contract is preferable to apparently broad support that fails unpredictably.

### H3. Match highlights are specified as both inverse and identical to the base scheme

**References:** US27–28; Implementation Decisions / Color schemes; Theme interface and tests.

The base schemes are white-on-black in dark mode and black-on-white in light mode. US28 and the implementation prose describe highlights as light-on-dark in dark mode and dark-on-light in light mode—the same polarity as ordinary text—while calling them inverse.

**Required clarification:** Specify actual foreground/background pairs. If inverse video is intended, matches should be black-on-white in dark mode and white-on-black in light mode. Also decide whether the current match is visually distinct from other matches. Style tests cannot validate a contradictory visual contract.

### H4. File-list selection is promised without a way to perform it

**References:** US3–4, US16–18; Implementation Decisions / Help overlay; App and SearchIndex interfaces.

US17 says users can “see and select from the file list again,” and the help description refers to file-list interaction. However, every navigation key is assigned elsewhere: up/down scroll content, left/right hide/show the list, and tab/shift-tab also hide/show it. There is no file-selection or focus mechanism. The help description even says there are no file-list-specific keys while it is open.

**Required clarification:** Decide whether the list is a passive overview controlled only by `n`/`p`, or an independently navigable selector. If it is passive, remove the selection promises. If it is selectable, define focus, keys, and the effect of selection on the current match and viewport. Either way, define how the active file remains visible when the list exceeds terminal height.

### H5. The navigation model does not fully define a “match” or its ordering

**References:** US6, US10–12; Result storage; Match cycling; SearchIndex.

A ripgrep `match` event may contain multiple submatches, and multiline searches can produce spans crossing source lines. The PRD does not say whether `n`/`p` visits events, matching lines, or individual occurrences. Sorting the file list alone also does not establish that the flat match list follows the same file order.

Initialization is unspecified: whether the first match is selected initially and whether the first `n` advances to the second occurrence. Manual scrolling has no defined effect on the match cursor. Zero-result behavior is absent from the navigation interfaces, which always return a match.

**Required clarification:** Define the navigation unit and ordering, ideally by sorted path followed by source position. Define initial selection, the relationship between scrolling and selection, single-match cycling, and no-match navigation. Support multiline and zero-width matches explicitly or exclude the relevant search modes; zero-width regex matches still need a display/navigation policy under ordinary regex search.

### H6. Match navigation conflicts with saved scroll position and horizontal reset

**References:** US6, US19–20; Scroll memory; Match cycling; Indicators.

When `n` or `p` enters a previously viewed file, restoring its old viewport may hide the destination match. On a first visit, “show the first match” conflicts with `p` entering at the last match. Resetting horizontal scroll to zero may likewise hide a destination on a long line.

**Required clarification:** Establish precedence between restoring a viewport and revealing a navigation target. For example, explicit match navigation could reveal its destination vertically while ordinary file selection restores saved position. Decide whether horizontal navigation remains at zero with an indicator or moves to reveal the target. Specify placement when a match spans more than one row or exceeds the viewport.

### H7. Wrapped scrolling lacks a defined coordinate model

**References:** US7–9, US19, US21, US35; Wrap mode; Viewport interface; Further Notes.

“One line,” “full page,” and remembered “vertical scroll offset” could mean source lines or rendered terminal rows. In default wrap mode these differ. A single source line can occupy more than a screen, and source-line-only scrolling may make its middle inaccessible.

The proposed viewport receives a source-line count and maximum line length, which alone cannot determine individual wrap heights. Its single visible source-line range does not express starting within a wrapped line. This exposes a requirements gap, not merely an API preference.

**Required clarification:** Define scrolling units in wrap mode, page size after reserved rows, and an anchor that survives rewrapping. Specify behavior on wrap toggles, panel hide/show, and terminal resize. A long source line must remain fully accessible, and restoring position must have a meaningful definition after width changes.

### H8. Error, popup, help, and quit key precedence is contradictory

**References:** US14, US29–31, US33–34; Pop-up; Help overlay; Error overlay.

`q` is both an immediate application exit and an overlay-dismissal key. “Any key” dismisses the filename popup, but does not say whether that key also executes its normal action. This matters especially for rapid `n` presses and quitting. Help is described as non-modal, but permitted background interactions are not defined.

Overlapping overlays and popup timers also lack an observable contract. For example, an earlier popup’s timeout must not unexpectedly dismiss a later popup.

**Required clarification:** Define key precedence for each overlay state, whether dismissal consumes the key, whether `ctrl+c` always exits, and which interactions remain active under help. Specify overlay precedence and replacement behavior, including how each newly displayed popup receives its intended lifetime.

### H9. Search lifecycle and failure states are underspecified

**References:** US32–34; rg subprocess; Error overlay; App / Init; Further Notes.

The PRD does not distinguish ripgrep’s normal no-match exit status (`1`) from an error (`2`), or define how stderr relates to successful or partial results. A run may find matches and still report inaccessible paths. “If rg emits an error” is too broad to establish whether results are retained or discarded.

Missing cases include a missing `rg` executable, process-start failure, malformed/truncated JSON, failure after partial output, and parser failure while the child is still running. There is also no specified loading screen or supported interaction before collection completes.

**Required clarification:** Define loading, success, no-results, partial-results-with-error, and fatal-error outcomes. Define the screen after an error is dismissed when there is nothing to browse, and relevant application exit statuses. Specify useful diagnostics, stderr handling, and cancellation/terminal restoration expectations on all exit paths. Distinguish “No results found” after a successful search from “no supported results” if matches are filtered out.

### H10. Search results may not correspond to the file later displayed

**References:** US5, US33; File reading; SearchIndex and FileBuffer.

Results are collected before files are loaded on demand. A file may be edited, removed, replaced, or become unreadable in between. Stored byte offsets can then highlight unrelated text or fall outside the loaded content. Ripgrep may also search decoded or transformed content that differs from raw bytes read from disk.

The proposed `FileBuffer` exposes match positions but has no specified input for search matches. This leaves unclear which component establishes correspondence between results and displayed content.

**Required clarification:** Define the consistency policy: detect stale content and warn/skip, display a captured snapshot, or explicitly accept best-effort stale results with safe bounds handling. State how unreadable or stale files affect navigation and whether revisiting them repeats errors. Define supported content encodings/transformations and ownership of match-to-display mapping; do not silently re-search and produce a different result set.

## Medium-priority findings

### M1. JSON and text edge cases need an explicit support policy

**References:** rg subprocess; SearchIndex; FileBuffer; Further Notes.

Terminal-cell measurement is correctly required, but it does not define how ripgrep’s byte offsets become display spans. The requirements should cover:

- JSON `text` versus base64 `bytes` fields for paths and content.
- Tabs, wide characters, combining sequences, and clipping through a multi-cell character.
- CRLF files, missing final newlines, and empty-file line numbering.
- Very long JSON records and source lines, rather than accidental parser-size limits.
- Zero-width matches and partially visible match spans.

**Required clarification:** State what is supported and what produces a controlled diagnostic. Define tab display, line-ending handling, and byte-to-cell mapping expectations so highlighting and scrolling agree.

### M2. Untrusted file content and paths can contain terminal control sequences

**References:** File reading; App / View; filename popup; Error overlay.

Files, filenames, and subprocess diagnostics are untrusted display input. They can contain escape sequences or control characters that alter the terminal, spoof interface content, or corrupt layout. Terminal-cell-aware truncation alone is not a safety policy.

**Required clarification:** Require safe rendering of control characters in content, paths, and diagnostics while retaining usable match-position mapping. Specify that terminal control sequences originating in searched data must not be executed by the terminal.

### M3. Indicator semantics are incomplete and may conflict spatially

**References:** US25–26; Indicators; Viewport.

The ideas say “if a match is not visible”; US25 narrows the edge indicator to the **current** match. This interpretation should be confirmed. It is also unclear whether a partially clipped match counts as hidden, whether a match wider than the viewport activates both indicators, and where the global indicator appears vertically.

A left-edge global marker may collide with the line-number gutter or the per-line marker. The PRD mentions the “first trailing space” but does not preserve the source’s explicit two-space gutter separation.

**Required clarification:** Define current-match versus any-match scope, partial clipping, vertical placement, gutter spacing, and whether markers reserve cells or replace content. Base half-screen horizontal increments on the actual usable content width, not an ambiguous terminal or panel width.

### M4. Layout and minimum-size behavior are not acceptance-ready

**References:** US35–38; Further Notes / panel width; Ideas / file panel design.

“Just wide enough for filenames,” “necessary padding,” and a “reasonable maximum (e.g. 40%)” are not deterministic requirements. “Filename” may mean basename or displayed path. Duplicate basenames make that distinction important. The source also requests a filename embedded in a top display line; the PRD guarantees only a filename row, leaving that visual detail unresolved.

**Required clarification:** Specify path representation, truncation rules, panel-width bounds, and active-file list scrolling. Define behavior when the terminal cannot fit the gutter and content, and how oversized help/error text is made readable. Clarify what selection and position must be restored after shrinking and expanding the terminal; offset clamping alone may permanently lose the former viewport.

### M5. Memory use and responsiveness have no bounded expectations

**References:** Result storage; File reading; App / Update; subprocess cancellation.

All results are retained, and every first-viewed file is read entirely into memory. The PRD does not say whether viewed buffers are retained indefinitely, what happens with huge result sets/files, or whether loading a large file may block input. An unlimited synchronous load conflicts with the promise of immediate quitting.

**Required clarification:** Set a supported scale or explicit resource-limit policy, including user-visible behavior when limits are reached. Require responsiveness during searching and file loading, and establish whether results appear only after search completion. Numerical performance targets can be modest, but resource exhaustion must not be the implicit limit.

### M6. Binary-result exclusion is not sufficiently defined

**References:** rg subprocess; Out of Scope / Binary file preview.

“Ignore binary matches” does not explain what happens when ripgrep emits text matches for a file before later identifying binary content, or when flags change binary detection. A previously collected file could remain in the list despite the exclusion.

**Required clarification:** Define when a file is classified as unsupported binary content, whether all its collected matches are removed, and what users see if every candidate is excluded. Reconcile this with the supported-flags policy.

### M7. The test plan omits most user-visible coordination behavior

**References:** Testing Decisions; App / Tested: no.

Deep-module tests do not establish that key routing, overlay precedence, popup timing, loading/cancellation, file transitions, scroll restoration, and resize behavior work together. Those are core requirements, not optional polish. The proposed viewport tests also do not cover wrapped-row scrolling, despite wrapping being the default.

**Required clarification:** Require behavioral coverage for those acceptance scenarios, including zero results, partial failure, repeated popup transitions, Unicode clipping, and stale/unreadable files. Driving Bubble Tea updates with synthetic messages and explicit timer events is consistent with the PRD’s own no-sleeps guidance. This need not mandate screenshot tests or a particular internal design.

### M8. Diagnostic behavior needs a deliberate scope

**References:** US33–34; Error overlay.

Subsequent errors replace the current overlay, so earlier diagnostics may disappear before users can read them. Long errors may not fit, and there is no stated way to inspect the full diagnostic. Conversely, adding persistent logging without a requirement could unnecessarily retain search terms or file paths.

**Required clarification:** Define whether errors are aggregated, queued, or summarized, and how complete diagnostics remain accessible. Decide whether diagnostics are overlay-only or also emitted to stderr after terminal restoration. Persistent logging is not inherently necessary, but its presence or absence should be intentional.

## Low-priority and document-quality findings

### L1. The problem statement overstates the case

**Reference:** Problem Statement.

`rg --json` is structured output, not “unstructured objects.” The intended point is that the event stream is not a convenient interactive browsing interface. “There is no interactive way” is also an unsupported universal claim.

**Suggested revision:** Describe the concrete workflow deficiency: raw output and pagers do not provide this proposed integrated navigation experience.

### L2. “Only match lines are displayed” contradicts whole-file viewing

**References:** US5; File reading; Out of Scope / rg context lines.

The PRD promises the current file’s contents, but the context-flags exclusion says “only `match` lines are displayed.” These describe different products.

**Suggested revision:** Say that ripgrep `context` events are not used for indexing, while surrounding content is obtained from the file buffer. Unsupported context flags can remain out of scope without implying a matches-only view.

### L3. Defaults and derived decisions should be recorded explicitly

**References:** Color schemes; Out of Scope / persistent preferences; Open Questions.

The startup color scheme is called a default but never chosen. Initial file-list visibility is implied rather than stated. Several additions—flag forwarding, circular navigation, scroll memory, and a 40% width-cap example—are reasonable refinements of the ideas, but should be distinguishable from original requirements and unresolved proposals.

**Suggested revision:** State startup defaults and record material departures or assumptions. Replace “Open Questions: None” with the decisions still awaiting resolution.

### L4. Detailed interfaces give a premature impression of completeness

**Reference:** Module Design.

The interfaces leave key state transitions unexplained: supplying matches to `FileBuffer`, setting viewport wrap/current-match state, selecting files independently, and representing empty search state. The exact methods need not be settled in a requirements review, but their current specificity masks unresolved behavior.

**Suggested revision:** Treat interfaces as provisional design sketches until the observable contracts above are decided. Retain responsibilities and acceptance expectations without allowing draft signatures to dictate missing requirements.

## Recommended decision order

Before issue generation, resolve these requirements in this order:

1. **Invocation:** optional directory, supported flags, operand parsing, and ripgrep configuration policy.
2. **Navigation:** occurrence definition/order, file-list role, current-match state, and scroll-restoration precedence.
3. **Rendering:** actual highlight colors, wrapped-row scrolling, byte-to-cell mapping, safe text rendering, and indicator placement.
4. **Lifecycle:** loading, no results, partial/fatal errors, stale/unreadable files, overlay key routing, and cancellation.
5. **Acceptance boundaries:** minimum layout, resource limits, defaults, and required behavioral coverage.

These decisions can substantially improve implementability without expanding the product into editing, interactive searching, persistent configuration, or other excluded features.
