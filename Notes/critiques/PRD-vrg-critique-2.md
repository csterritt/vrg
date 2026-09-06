# Requirements critique: VRG PRD — review 2

## Scope and verdict

Reviewed:

- `Notes/skills/critique-prd/SKILL.md`
- `Notes/PRD-vrg.md` (revision 2)
- `Notes/Ideas.md`
- `Notes/critiques/PRD-vrg-critique-1.md`
- The revision interview in this conversation, to distinguish approved product choices from unresolved consequences.

This is a **requirements-only review**. It does not generate implementation issues, tasks, or code. References use revision-2 user story numbers (US) and section headings. Finding IDs below are local to this review.

**Verdict: revision 2 is substantially improved, but revise before issue generation.** Most of the original product choices are now explicit. However, several combinations of those choices still contradict their promised outcomes. In particular, navigation can land on a wrapped line without revealing any match, rewrapping cannot preserve content with the stated anchor algorithm, and the 90% file-list width rule can leave an unusable file panel even in a terminal that passes the minimum-size test.

The earlier assertion that every finding was resolved is too strong. A choice was made for many concerns, but its acceptance contract is not always complete. This review does not ask to reopen approved choices such as matched-line navigation, a passive file list, inverse colours, or non-streaming results.

### Improvements that should be retained

- Optional directory and default `.` are restored.
- Flags are restricted, combined short flags and `--` are addressed, and ripgrep configuration is disabled.
- Matched-line navigation, initial selection, circular ordering, and the passive file list are explicit.
- Colours are unambiguous, including underlining the current line's matches.
- Rendered-row scrolling, modal key routing, and per-instance popup timers are specified.
- Whole-file viewing is no longer confused with parsing ripgrep context events.
- Binary filtering, partial results, asynchronous loads, and diagnostic replay have explicit policies.
- Module interfaces are provisional, receive the information they need, and have required behavioural tests.

### Priority definitions

- **High:** a contradiction or missing contract that can break core behaviour or produce materially different implementations.
- **Medium:** an important edge case, safety issue, operational boundary, or acceptance gap.
- **Low:** document precision or scope hygiene.

## High-priority findings

### H1. Revealing a matched source line does not necessarily reveal a match

**References:** US28, US37, US44–47; Implementation Decisions / Viewport and scrolling; Viewport interface.

The reveal rule says that if the destination **line** is already visible, no vertical scrolling occurs. In wrap mode, a source line can occupy many screens. Its first rendered row may be visible while its first submatch is hundreds of rows below. Even when the line is not visible, placing the start of that source line one third down the screen may reveal no matched text.

Example: a long line has its only match at display column 5,000. The viewport shows the beginning of the line. `n` selects that line and decides it is already visible. The source's central promise—start with the first match visible—is not met.

There is a related ambiguity in run-off-edge mode: does “visible” mean any part of the first submatch or the whole submatch? A submatch wider than the viewport cannot be fully revealed. The user-story rule applies to every match-navigation action, whereas the implementation paragraph can be read as applying horizontal reveal only after a file switch.

**Required clarification:** Keep matched lines as the navigation unit, but define a display target within each line, such as the start of its first submatch. Apply vertical reveal to the target's rendered row, not merely to the source line. Define partial visibility, oversize spans, initial selection, and same-file transitions consistently. State where the target lands when there is insufficient context to place it one third down.

**Acceptance example:** A selected source line taller than the viewport, with its first match near the end, must show that match after navigation without manual scrolling.

### H2. The anchor algorithm contradicts content preservation and shrink–grow restoration

**References:** US50–52, US71; Implementation Decisions / Viewport and scrolling; Viewport tests.

The PRD promises to keep the same content at the top using `(source line, row within that line)`, preserving the sub-row index or clamping it after rewrap. A row index is width-dependent:

- At width 20, sub-row 5 begins around display column 100.
- At width 40, sub-row 5 begins around column 200.

Keeping the index does not keep the same content. Toggling wrapping off clamps every sub-row to zero; toggling it back cannot restore the old sub-row unless the original anchor is retained separately. Likewise, increasing viewport height near EOF may require moving the effective top row to avoid blank space, conflicting with the instruction to keep the anchor at the top.

**Required clarification:** Choose between preserving the source-line/sub-row ordinal and preserving the underlying text location; they are not equivalent. If content preservation remains the goal, specify a width-independent logical anchor and distinguish it from a temporarily clamped display position. State what survives shrink–grow and wrap-off–on, and when user scrolling replaces a saved anchor. If sub-row clamping is intentionally lossy, narrow the preservation promises accordingly rather than claiming lossless restoration.

### H3. The 90% file-list cap conflicts with minimum content width

**References:** US23, US70–71; Implementation Decisions / Layout.

An 80-column terminal with long paths allocates 72 columns to the list. Only 8 remain for the file panel; a three-digit line number plus two spaces leaves 3 content columns. The terminal itself passes the stated minimum of gutter + 10 columns, yet the file panel does not.

It is unspecified whether the minimum-size test applies to the whole terminal or the remaining file panel. If it applies to the panel, merely encountering a long pathname can turn a normal terminal into a “Terminal too small” screen. The required recovery keys are not defined for that screen. The three-row height minimum also permits a bordered overlay with effectively no usable text area if its layout is not handled separately.

**Required clarification:** Define precedence between the approved 90% cap and the content minimum. For example, treat 90% as an upper bound further limited by the need to leave gutter + 10 columns, or explicitly choose an auto-hide/too-small policy. Specify rounding, recovery keys (including list hiding), and layout for tiny overlays. Preserve user-requested list visibility separately if it is temporarily overridden.

**Acceptance example:** A long path must not silently reduce an otherwise valid file panel below its declared minimum width.

### H4. Search outcome and application exit status still lack a complete contract

**References:** US12–19, US77–78; Implementation Decisions / Subprocess and results, Overlays and key precedence; App interface.

The current mapping covers the common rg exits but not combinations created by parsing and filtering:

- rg exits 0 but every candidate is dropped as binary: the prose says both “browse” and “No results found (N binary files skipped)”; vrg's exit status is missing.
- rg exits 0 but malformed records leave no usable results: an error overlay is required, but what follows dismissal and which status is returned?
- rg exits 2 with usable results: browsing is defined, but vrg's eventual exit status is not explicit.
- User cancels before search completion: there is no completed search outcome from which App can derive a status.
- A signal or unexpected subprocess exit code is not necessarily a truncated JSON record; a process can die between complete records.
- stderr can be nonempty outside the rg-2 case. The Solution says errors are never silently dropped, but no general stderr policy is stated.
- A successful search followed by unreadable-file errors may or may not change the application exit status.

The parser also has no observable rule for distinguishing an incomplete event stream from a well-formed stream with no matches. Missing or malformed `end` events affect binary classification; missing completion records can occur without a partial final JSON line.

**Required clarification:** Supply an outcome table combining process result, stream integrity, usable result count, and exclusions. For each outcome state the initial screen, diagnostic, state after dismissal, and final exit status, including cancellation and file-load errors. Preserve the approved skip-and-count policy for malformed lines, but distinguish safe record skipping from loss of file/completion metadata. Define how unknown event types and structurally invalid known events are classified.

### H5. Expanding allowed short flags can accidentally admit an excluded search mode

**References:** US5–8; Implementation Decisions / Invocation; binary exclusion policy.

The list includes `-u` and `-uu`, while the algorithm expands combined short flags and validates each component. Under that rule, `-uuu` and `-u -uu` contain only allowed components and are accepted. Ripgrep gives the third `-u` a different meaning: it enables binary searching in addition to removing ignore and hidden-file filtering. This may be a deliberate future-compatible behaviour, but the apparent two-level restriction is not enforced by the written contract.

Other operand details remain implicit: whether options are accepted after the pattern, whether a directory operand may be a file or `-` (stdin), and whether conflicting allowed flags preserve their original order for ripgrep to resolve. Accepting stdin would break the disk-backed FileBuffer model. A literal pattern beginning with `-` must also remain an operand in the child invocation, not merely pass vrg's parser.

**Required clarification:** Define cumulative `-u` semantics across all tokens. Either reject a third occurrence or explicitly support its binary behaviour. Specify option placement, flag precedence, accepted search-root types, and child operand protection. A directory-only contract is adequate; stdin support need not be added.

**Evidence:** Local ripgrep 15.2.0 help documents the third `-u` as enabling `--binary`; a local `rg --json --no-config -uuu hit <file>` invocation was accepted.

### H6. Asynchronous file loads need a navigation consistency contract

**References:** US32–33, US40–45; Implementation Decisions / File loading; App messages and tests.

Making file loading asynchronous preserves input responsiveness, but creates an unaddressed race. The user can navigate A → B → C before B finishes loading. B's late success must not replace C's content or reveal B's target; B's late failure must have a deliberate notification policy. Popup timing is also unspecified relative to slow loads: a popup may expire before its file content is visible.

Unreadable-file retries interact badly with the matched-line cursor. “Revisiting” could mean moving to another matched line in the same failed file, leaving and re-entering the file, or any `n`/`p`. A failed file with many matched lines could produce an overlay on every key press; a result set with just one matched line has mandatory no-op navigation and therefore no way to retry it without restarting.

**Required clarification:** Define which selection a load result may affect, allowed navigation while loading, and treatment of late failures. Define exactly what triggers an unreadable-file retry, including the single-line case, and whether unavailable matched lines remain cursor stops. State whether file-change popups begin at selection or successful display. This is an observable contract, not a requirement for particular concurrency primitives.

## Medium-priority findings

### M1. The stale-content rule checks bounds, not correspondence

**References:** US28, US34; Implementation Decisions / Text rendering; FileBuffer; Out of Scope.

The approved bounds guard prevents some invalid ranges, but cannot establish that a highlight still denotes the searched text. Replacing `hit` with `cat` at the same position leaves a perfectly in-bounds but incorrect highlight. Inserted lines can similarly preserve all ranges while moving the intended match. The PRD should not imply that bounds checks prevent misleading highlights.

After a dropped highlight, the matched-line entry remains in SearchIndex. The contract does not say where navigation lands if that line no longer exists or has no remaining valid submatches. “One-time” filename-row warning is also unclear for a persistent cached buffer: does it disappear after a transition, persist on revisits, or have a timer?

**Required clarification:** Retain the approved lightweight guard if desired, but explicitly label undetected content changes as a best-effort limitation. Alternatively, propose stronger comparison as a new decision, not as an already-approved requirement. Define navigation to stale entries, warning lifetime, and whether cached buffers intentionally ignore subsequent disk edits.

### M2. Ripgrep can transcode by default, without an allowed encoding flag

**References:** US28, US69; Implementation Decisions / Text rendering; Out of Scope / transcoding.

Excluding user encoding flags does not ensure rg searches raw file bytes. Ripgrep's default encoding detection can decode BOM-marked UTF-16 input. FileBuffer then reads different bytes from those represented by the JSON line and offsets. These spans may be in bounds, so the stale guard does not catch the mismatch.

**Evidence:** With ripgrep 15.2.0, a temporary UTF-16 BOM file containing `hello hit\n` produced a JSON text line `hello hit\n` and submatch offsets 6–9 under `rg --json --no-config hit <file>`. Those are offsets in decoded text, not positions for highlighting the raw UTF-16 buffer.

Correct highlighting for non-UTF-8 is explicitly out of scope, so this does not require adding transcoding support. It does require deciding what an unsupported transformed input looks like in the UI instead of relying on flag rejection to exclude it.

**Required clarification:** Choose an internal raw-encoding search policy, controlled exclusion of detected unsupported encodings, or explicit best-effort display without trustworthy highlights. Distinguish internal mandatory rg arguments from the user flag allow-list.

### M3. Zero-width markers and removed line endings still have undefined display positions

**References:** US47, US60, US65–69; Implementation Decisions / Text rendering; FileBuffer and Viewport.

A one-cell inverse marker needs content and space. At end of line there is no underlying character; on an empty line there may be no display width at all. If the line exactly fills a wrap row, a marker at end of line may require another rendered row. Horizontal clamping based only on text width may make it unreachable.

CRLF introduces an additional mapping case: the display removes `\r`, but ripgrep can produce a match on that byte or a zero-width match after it. In a local `hit\r\n` fixture, the default `$` pattern produced start=end=4, while the displayed `hit` has length 3. The PRD does not distinguish bounds against original line bytes from bounds after newline removal.

Combining-only matches and matches covering part of a grapheme also need a rule: their byte spans may map to no independent terminal cell. Per-line widths alone cannot identify safe wrapping boundaries through wide or multi-code-point graphemes.

**Required clarification:** Define the actual zero-width marker glyph/style, its effect on display width and wrapping, and mapping for removed terminators and partial graphemes. Preserve original-byte coordinates separately from displayed text. Require a single consistent grapheme/cell policy for wrapping, clipping, highlight mapping, and clamping; exact internal signatures can remain provisional.

### M4. Safe rendering must also cover diagnostic replay and raw path identity

**References:** US19–20, US68–69; Implementation Decisions / Text rendering, Overlays and key precedence; CLI and FileBuffer.

The prohibition on raw control sequences should explicitly extend to stderr after terminal restoration. Replaying raw rg stderr or a raw filename would reintroduce terminal-control injection after an otherwise safe TUI session. Caret notation also does not by itself define a representation for all C1 controls.

For paths, invalid-byte replacement is suitable for display but not for file access or identity. Two distinct raw paths may render identically after replacement or escaping. “Sorted by path text” is undefined when the JSON path uses `bytes`.

**Required clarification:** Apply a safe presentation policy to every output sink, including usage errors and replayed diagnostics, while retaining readable diagnostic line structure. Preserve original path bytes for opening and identity; specify deterministic ordering separately from lossy display text. Distinguish escaped filename newlines from actual diagnostic paragraph boundaries.

### M5. The approved scale target is not a memory bound

**References:** US11, US32; Implementation Decisions / Subprocess and results; Out of Scope / memory limits.

This revision explicitly chooses indefinite buffer retention and best-effort behaviour beyond the target. That is a product decision, not an accidentally missing cache policy. However, the target itself can exceed ordinary machine capacity: 10,000 viewed files of 50 MB each approach 500 GB before indexes or decoded display buffers. Tab expansion and control-character escaping can further multiply storage.

Very long JSON records remain unspecified; a record much larger than common default line-reader limits can occur well within the approved 50 MB file size. Asynchronous reads also do not guarantee responsive quitting if parsing, decoding, sorting, rewrapping, or rendering later performs unbounded work in the UI update path.

**Required clarification:** State whether the scale figures are simultaneous guarantees or independent examples, and document the absence of an aggregate memory guarantee. Define a supported record size or controlled oversized-record diagnostic, and require input responsiveness during all expensive processing, not just disk I/O. If hard resource limits remain out of scope, record OOM/termination risk honestly rather than describing the scale target as bounded support.

### M6. Error/help interaction and complete diagnostic access need final details

**References:** US73–76; Implementation Decisions / Overlays and key precedence.

Error > help specifies visual priority but not what happens after the error closes: does previously open help return, or was it dismissed? Appending errors is sensible, but it was introduced in the written revision without a separate explicit interview answer; it should not be described as individually confirmed.

Vertical scrolling alone cannot expose a long, unbroken diagnostic line unless overlay text wraps. A long filename popup also needs a fit rule. Diagnostics received just before quitting might never be shown in an overlay; the current replay wording guarantees only text that was shown, despite the Solution promising errors are never silently dropped.

**Required clarification:** State overlay restoration, text wrapping/truncation, appended-error scroll behaviour, and whether replay includes all collected diagnostics or only displayed ones. Confirm aggregation as the intended policy. Preserve the approved no-persistent-log decision.

### M7. A right-edge marker can hide another match it declares visible

**References:** US58–60; Implementation Decisions / Indicators; Further Notes.

The right-edge `*` replaces the last content cell. Suppose one one-cell match occupies that cell and another match on the current line is farther right. The marker for the latter overwrites the former, although visibility calculations using the full content width classify the former as visible.

There is also a direct document contradiction: Further Notes scopes **per-line gutter markers and edge markers** to the current matched line, whereas US58 and the Indicators section require gutter markers on any visible line. The sentence “no edge indicators are shown” when the current line is vertically hidden should not accidentally suppress other lines' gutter markers.

**Required clarification:** Define visibility after marker composition, or reserve a cell consistently for the right indicator. Keep per-line markers on all visible source lines and current-line edge indicators scoped separately. Correct Further Notes to match that distinction. Do not reopen the approved rule that a genuinely partially visible match needs no hidden-match marker.

## Low-priority and document-quality findings

### L1. “All resolved” and “all approved” should distinguish decisions from consequences

**References:** Open Questions; Further Notes.

Replace the blanket closure claim with the remaining decisions above. Many original questions were answered, but this is not equivalent to having a consistent acceptance contract. The matched-line navigation choice is an important refinement worth recording alongside the other departures; the inaccurate marker-scope summary should be corrected.

### L2. Latent file-selection stories and tests are outside the version-1 product

**References:** US51; Viewport and scrolling; Testing Decisions / App; Out of Scope.

There is no non-match-navigation file entry point, yet US51 promises restoration through that route and tests refer to its precedence. Per-file view state can still be useful when `n`/`p` re-enters a file, but the latent selection feature need not be implemented or tested as if it exists.

**Suggested revision:** State the observable version-1 revisit behaviour, and move hypothetical future selection behaviour to Further Notes or remove it.

### L3. Extend the improved tests to the remaining cross-feature boundaries

**References:** Testing Decisions.

The new behavioural suite is a major improvement. Once the above contracts are decided, add acceptance cases for wrapped match revelation, shrink–grow/wrap round trips, panel-width contention, all-filtered outcomes, malformed completion metadata, cumulative `-u`, late file-load completion, zero-width end-of-line markers, and sanitized stderr replay.

A synthetic “quit produces a cancel command” assertion alone does not establish process termination and terminal cleanup. Require a small deterministic subprocess-boundary test (for example, a controllable fake rg process) in addition to model-state tests; screenshots and sleep-based tests are unnecessary.

## Disposition of review-1 findings

| Review-1 finding | Revision-2 disposition |
|---|---|
| H1 — optional directory | Resolved. |
| H2 — flag contract | Substantially resolved; cumulative `-u` and operand boundaries remain (H5). |
| H3 — colours | Resolved. |
| H4 — file-list selection | Resolved as passive overview with active-file scrolling. |
| H5 — navigation unit/state | Core resolved as matched-line navigation; marker boundaries remain (M3). |
| H6 — reveal vs. saved scroll | Priority resolved; wrapped target visibility remains incorrect (H1). |
| H7 — wrapped scrolling | Rendered-row unit resolved; preservation algorithm remains contradictory (H2). |
| H8 — overlays | Main key routing and timers resolved; nested overlay/diagnostic details remain (M6). |
| H9 — lifecycle | Common states resolved; outcome combinations and statuses remain (H4). |
| H10 — file consistency | Bounds-only policy chosen; limitations and async transitions need clarity (H6, M1–M2). |
| M1 — text/JSON edges | Many cases specified; terminators, graphemes, markers, and record sizes remain (M3, M5). |
| M2 — terminal safety | Core policy added; replay and raw-path boundaries need explicit coverage (M4). |
| M3 — indicators | Main semantics resolved; marker composition and summary conflict remain (M7). |
| M4 — layout | Numeric policy added; cap/minimum interaction remains (H3). |
| M5 — resources | Best-effort/unbounded retention deliberately chosen; scale claims still need qualification (M5). |
| M6 — binary exclusion | Normal completed-file rule resolved; flag repetition and incomplete metadata remain (H4–H5). |
| M7 — tests | Substantially resolved; add boundary cases and subprocess verification (L3). |
| M8 — diagnostics | Aggregation/replay added; full-access details remain (M4, M6). |
| L1–L2 — wording/context | Resolved. |
| L3 — defaults/closure | Defaults resolved; blanket closure claim remains unsupported (L1). |
| L4 — interfaces | Substantially resolved by provisional contracts; rendering boundary information still needs clarification (M3). |

## Recommended resolution order

1. Resolve match-target visibility, anchor preservation, and the layout minimum together (H1–H3).
2. Complete the outcome/exit-status table and input contract (H4–H5).
3. Define asynchronous navigation, retries, and stale/unsupported content behaviour (H6, M1–M2).
4. Finish text mapping, safe output, diagnostic presentation, and marker composition (M3–M4, M6–M7).
5. Qualify resource expectations, update acceptance tests, and remove unsupported closure claims (M5, L1–L3).

These are requirements corrections and boundary decisions, not a request to expand version 1 into editing, interactive search, direct file selection, streaming results, or persistent configuration.
