# Requirements critique: VRG PRD — review 3

## Scope and verdict

Reviewed:

- `Notes/skills/critique-prd/SKILL.md`
- `Notes/PRD-vrg.md` (revision 3)
- `Notes/Ideas.md`, the source ideas
- `Notes/critiques/PRD-vrg-critique-1.md` and `Notes/critiques/PRD-vrg-critique-2.md`
- A set of preliminary findings from another agent, verified and expanded below.

This is a **requirements-only review**. It does not generate implementation issues, tasks, or code. References use revision-3 user story numbers (US) and Implementation Decisions section headings. Finding IDs below are local to this review.

**Verdict: revision 3 is ready for issue generation with minor clarifications.** All high-priority findings from reviews 1 and 2 have been resolved. The outcome table is complete, the reveal algorithm is explicit, the anchor model is width-independent with acknowledged EOF loss, the layout cap respects minimum content width, asynchronous selection isolation is defined, encoding placeholders are specified, marker/grapheme mapping is consistent, safe presentation covers all sinks, and resource qualifications are honest. No remaining finding is a contradiction or a missing core contract that would produce materially different implementations.

The findings below are refinements, edge cases, and specification-precision gaps. They should be settled before or during issue generation so that implementers do not need to infer the intended behavior, but none blocks the overall design. The preliminary findings from the other agent were verified against the PRD text; several were re-prioritized based on what the PRD actually specifies, and two additional low-priority findings are included.

### Priority definitions

- **High:** a contradiction or missing contract that can break core behaviour or produce materially different implementations.
- **Medium:** an important edge case, safety issue, operational boundary, or acceptance gap that should be settled before implementation.
- **Low:** document precision or scope hygiene.

No high-priority findings remain in revision 3.

## Medium-priority findings

### M1. Load-completion reveal does not explicitly reference the entry reveal algorithm

**References:** US50, US52–54, US57; Implementation Decisions / File loading (line 177), / Navigation (lines 212–215).

The entry reveal algorithm is fully specified in the Navigation section:

> On file change, use saved per-file vertical state as the starting viewport and then apply destination reveal. Reset horizontal offset to zero and then apply horizontal reveal. (line 215)

This includes target-row identification (line 212), one-third vertical placement with clamping (line 213), and minimal horizontal reveal (line 214). The File loading section separately states:

> Load completion is associated with the requested raw path and request identity. It updates only that path's cache/status. It may affect the visible panel only when that path is current; reveal the **latest** selected cursor target, never a target captured from an earlier selection. (line 177)

And:

> Navigation away and back likewise uses the normal entry reveal rule. (line 182)

Line 182 explicitly connects navigation-away-and-back to "the normal entry reveal rule." But line 177, which governs load completion, says only "reveal the latest selected cursor target" without stating that this means applying the entry reveal algorithm. An implementer could reasonably interpret "reveal" as merely making the target visible (minimal scroll to bring it on-screen) rather than applying the full entry reveal sequence (saved viewport starting point, one-third placement, horizontal offset reset, horizontal reveal).

This matters most for the **initial load** at startup. US50 says "startup to select and reveal the first match on the first matched line," but the first file is loading asynchronously (US34). The reveal cannot happen until content is available. At load completion, line 177 governs: "reveal the latest selected cursor target." For a first visit, there is no saved per-file vertical state (the entry reveal algorithm's starting point), so the starting viewport is implicitly the top of the file—but this is not stated. The implementer must infer both that load completion uses the entry reveal algorithm and that a first visit starts from the top of the file.

The gap is narrow but affects a stable contract ("target revelation" is listed as a stable contract in the Module Design section, line 255). The intent is reasonably clear from context—an implementer reading the whole PRD would likely apply the same reveal algorithm—but the connection should be explicit rather than implied.

**Required clarification:** State that load completion applies the same entry reveal algorithm as file-change navigation, using the latest cursor target as the destination. For a first visit with no saved per-file vertical state, specify that the starting viewport is the top of the file before destination reveal is applied. Alternatively, state that "reveal" in line 177 means "apply the destination reveal rules of lines 212–215."

**Acceptance example:** At startup with a slow-loading first file, the first match must appear at one-third down the content area after load completes, not merely somewhere on-screen.

### M2. Diagnostics arriving during active browsing have no in-UI indication

**References:** US22, US37, US38; Implementation Decisions / File loading (line 179), / Colours and overlays (line 244); Outcome table (line 163).

The outcome table (line 163) specifies that a successful search with usable results presents "Browse; error overlay if any diagnostics." So diagnostics known at search completion time **are** shown in an initial error overlay. The finding from the other agent—"no in-UI indication during a normal browse ever shows warnings"—is too strong as written; the initial overlay does surface diagnostics.

However, diagnostics that arrive **after** the initial overlay is dismissed have no in-UI indication. US37 and the implementation (line 179) state:

> Non-current failures are diagnostic-only, not modal interruptions.

US22 and the implementation (line 244) state:

> all collected diagnostics safely replayed to stderr after terminal restoration, including diagnostics never shown in an overlay

The specific scenario: the user navigates to file A, load starts; the user navigates to file B before A finishes loading; A's load fails. A is non-current, so the failure is diagnostic-only (US37). The user sees no indication that A failed. The failure is recorded in the session diagnostic collection and replayed to stderr at exit (US22, line 244). The user discovers the failure only when they later navigate to A (US38: current-file read failure shows an error overlay and "(unreadable)"). If the user never returns to A, the failure is visible only in stderr after quitting.

This is a deliberate design choice—US37 explicitly chose not to interrupt browsing with late failure overlays—and the behavior is fully defined, not missing. The gap is that the user has **no way to review collected diagnostics mid-session** without quitting. A long browsing session could accumulate many non-current failures that the user is unaware of until exit. There is no "show diagnostics" key or status indicator that would alert the user to the existence of unseen diagnostics.

This is a usability consequence of a deliberate choice, not a contradiction. It does not require changing the non-interruption policy. But the PRD should acknowledge that mid-session diagnostic access is limited to visiting affected files, or provide a lightweight non-modal indicator (e.g., a status-bar count of unseen diagnostics).

**Required clarification:** Either explicitly acknowledge that mid-session diagnostics are discoverable only by visiting affected files or via stderr at exit, or specify a lightweight non-modal indicator for the count of collected-but-unshown diagnostics. Do not change the approved non-interruption policy for non-current failures.

### M3. The ~50 MB scale example can exceed the 64 MiB record cap for non-UTF-8 single-line files

**References:** US17, US85; Implementation Decisions / Result index (line 145), / Resources (lines 248, 250).

The PRD states:

> Independent scale examples: approximately 10,000 matched files, 100,000 matched lines, or individual files around 50 MB. (line 248)

And:

> Maximum JSON record payload: **64 MiB**, excluding its newline delimiter. (line 145)

And explicitly acknowledges:

> A 64 MiB JSON record limit is independent of source-file size: escaping or base64 can exceed it even for a source file under 50 MB. Oversized records use the explicit skip/count and integrity rules, not silent truncation. (line 250)

The PRD is honest about the disconnect. The consequence is worth spelling out: a ~50 MB file with a single huge non-UTF-8 line produces a `match` record whose `lines.bytes` field is base64-encoded. Base64 expands by ~33%, so a 50 MB (47.7 MiB) line becomes ~63.6 MiB of base64 text, plus JSON field names, path data, submatch metadata, and structural overhead. This can exceed 64 MiB. The record is skipped and counted (US17), and the match is lost from the navigation index.

The diagnostic reports "N oversized records skipped" (line 144, US17) but does not identify which files or matches were affected. A user searching a ~50 MB file that they know contains matches would see the file absent from the results with no explanation beyond a count. The file might not appear in the file list at all if its only match record was the oversized one, making it difficult to correlate the count with the missing file.

This is an edge case within an acknowledged limitation, not a design flaw. But the scale example and the record cap create a reasonable user expectation that a ~50 MB file is supported, and the PRD should clarify what the user sees when that expectation is not met.

**Required clarification:** Clarify that the ~50 MB scale example assumes UTF-8 or near-UTF-8 content with reasonable line lengths, and that files with very long non-UTF-8 lines may produce oversized records even below 50 MB. Consider whether the oversized-record diagnostic should identify affected paths, or state that file-level identification is out of scope for version 1.

### M4. `r` during an in-flight load is ignored with no cancellation path

**References:** US39, US42; Implementation Decisions / File loading (lines 176, 179, 210).

US42 and the implementation (line 176) state:

> Requesting `r` or re-entering a path already loading does not start another load.

US39 and the implementation (line 179) state:

> `r` permits retry even with a one-stop index.

And line 210:

> `r` is the explicit one-entry retry route.

The interaction is functional but creates a minor friction point. When a file's load fails and the user presses `r` to retry, the retry starts and "Loading..." is shown. If the user presses `r` again during the retry, it is ignored (US42). The user must wait for the retry to complete (success or failure) before pressing `r` again. There is no way to cancel an in-flight load; the user can only navigate away (US35: "navigation to continue during loading") and wait.

For the **one-stop index** case (a file with exactly one matched line), navigation is a strict no-op (line 210: "one entry makes both keys strict no-ops"). The user cannot navigate away and back to trigger a new load. Their only retry mechanism is `r`, which is ignored while a load is in flight. So the user must wait for each retry attempt to complete before initiating the next one. If a file is intermittently unreadable (e.g., on a slow or flaky network filesystem), the user may need multiple retry cycles, each requiring a wait-and-press-`r` sequence.

This is a deliberate consequence of the "at most one load per raw path" rule (line 176), which prevents overlapping reads. The behavior is correct—starting a new load while one is in flight would violate that rule. But the PRD does not acknowledge the one-stop retry friction or specify whether the user has any way to know when an in-flight load has completed (other than watching for the placeholder to change from "Loading..." to content or "(unreadable)").

**Required clarification:** Acknowledge that `r` is queued-then-dropped rather than queued-then-executed for in-flight loads, and that the one-stop case has no navigation-based escape from a slow retry. Confirm that the placeholder text change ("Loading..." → content or "(unreadable)") is the completion signal. No design change is required unless the product owner wants to add load cancellation or request queuing.

### M5. Too-small terminal round trip with an open modal does not explicitly preserve overlay scroll position

**References:** US32, US80; Implementation Decisions / Layout (line 225), / Colours and overlays (line 242).

The implementation (line 225) states:

> Preserve selection, modal state, and visibility preference for recovery on resize. Do not let this screen trap the user behind an invisible modal overlay.

And line 242:

> At tiny sizes overlays are clipped to the terminal, without a special borderless mode. Resize restores access.

US80 states:

> errors suspend help and restore its previous scroll position when dismissed

So help scroll position is explicitly preserved across error suspension (US80). But the too-small round trip—terminal shrinks below 20 columns or 3 rows, "Terminal too small" screen appears, terminal grows back—is a different scenario. The PRD says to preserve "modal state" (line 225), which implies the overlay is restored, but does not explicitly say that the overlay's **scroll position** survives the round trip.

Consider: the user has scrolled halfway through a long help text. The terminal briefly shrinks below 20x3 (e.g., a window manager resize accident). The "Terminal too small" screen appears. The terminal is resized back. The help overlay is restored—but at what scroll position? "Modal state" could reasonably include scroll position, but it is not stated. If the scroll position is lost, the user must re-navigate to where they were reading.

This is an edge case. The PRD's "modal state" preservation could be interpreted to include scroll position, and US80 establishes a precedent for preserving help scroll position across interruptions. But the too-small round trip is not explicitly covered.

**Required clarification:** State that overlay scroll position is included in "modal state" for too-small recovery, or explicitly list what is and is not preserved through a too-small round trip. Given US80's precedent, preserving scroll position is the consistent choice.

## Low-priority findings

### L1. CRLF end-of-line zero-width marker: confirm single-cell policy consistency

**References:** US70, US73; Implementation Decisions / Text (lines 197, 201).

The PRD specifies the CRLF end-of-line marker case in detail:

> For example, `$` at byte 4 of `hit\r\n` maps to display column 3. A match solely on removed terminator bytes becomes an end-of-line marker; a span also covering visible text highlights that visible text. (line 197)

> A zero-width submatch is shown as one inverse-video space at its mapped location, underlined on the current matched line. It marks an existing display cell without shifting following text; if at end of line, it extends effective line width by one cell. (line 201)

The policy appears consistent: the `$` match on CRLF maps to display column 3 (end of visible `hit`), the zero-width marker is one inverse-video space, and it extends effective line width by one cell. The marker participates in reveal, wrap, clipping, indicators, and horizontal extent (line 201). Line 201 also states: "a marker after a completely full wrap row occupies another row," confirming that the marker's cell extension interacts with wrapping.

The finding from the other agent asks to "confirm consistent single-cell policy holds here." Based on the PRD text, the policy does appear consistent. The only residual question is whether the marker cell at display column 3 (end of line) is correctly treated as a "match/marker" for the **left gutter indicator** in run-off-edge mode. Line 230 states:

> every visible source line uses the first trailing gutter space for inverse `_` if text is hidden left, upgraded to inverse `*` if a match/marker is entirely hidden left

Since the marker is explicitly listed alongside "match" in the indicator rules, and line 201 says "Marker cells participate in reveal, wrap, clipping, indicators, and horizontal extent," the treatment appears consistent. No gap is identified; this finding is a confirmation request rather than a defect.

**Suggested clarification:** Add a brief note in the Text section or an acceptance test case confirming that a `$`-only match on CRLF at end of line produces a single-cell end-of-line marker that is treated identically to any other zero-width marker for indicator, reveal, and wrapping purposes. The FileBuffer test list (line 299) already includes "zero-width positions at beginning/end/empty lines and removed terminators," which should cover this case.

### L2. `Esc` key appears in implementation decisions but not in user stories

**References:** US78–79, US84; Implementation Decisions / Colours and overlays (lines 171, 239–240).

The implementation decisions mention `Esc` as a dismissal key for both help and error overlays:

> `q`/`Esc` dismiss an error overlay (line 171)

> `q`/`Esc`/`h`/`?` close (line 239, help)

> `q`/`Esc` dismiss (line 240, error overlay)

But the user stories do not mention `Esc`:

- US79 says help accepts "its documented dismissal keys, and `ctrl+c`"—vague, does not enumerate `Esc`.
- US84 says "`q` to dismiss an overlay before quitting ordinary browsing, and `ctrl+c` always to exit 130"—mentions only `q`, not `Esc`.

`Esc` is a reasonable addition for overlay dismissal (it is conventional in TUI applications), but it should be reflected in the user stories for consistency. The help dialog (US78: "all key bindings") would list `Esc`, but the user stories that define the help dialog's behavior do not mention it.

Additionally, the PRD does not specify what `Esc` does during **normal browsing** (no overlay open). Presumably it is a no-op, but this is not stated. The key precedence rules (line 238) list "base-state keys" but do not enumerate `Esc` among them.

**Suggested revision:** Add `Esc` to US79 and US84 as an overlay dismissal key alongside `q`, or add a note that the implementation decisions enumerate the full key set including `Esc`. State that `Esc` is a no-op during normal browsing with no modal overlay.

### L3. Pop-up repositioning on resize is unspecified

**References:** US58–59; Implementation Decisions / Colours and overlays (line 243).

Line 243 states that pop-ups are "centered." The resize handling (line 225) says to "Preserve selection, modal state, and visibility preference for recovery on resize" but does not mention pop-ups. If the terminal is resized during a pop-up's one-second lifetime, does the pop-up recenter? The pop-up has its own fresh timer (line 243: "Each has its own fresh one-second lifetime"), so the timer is not affected by resize. But the centering could change if the terminal dimensions change.

This is a very minor edge case. The pop-up lasts only one second, and a resize during that second is unlikely. If the pop-up is not repositioned, it may appear off-center or partially clipped until it expires. If it is repositioned, it stays centered.

**Suggested clarification:** State that pop-ups recenter on resize, or that their position is fixed for their one-second lifetime. Either choice is acceptable for version 1.

## Disposition of review-2 findings

All review-2 findings have been addressed in revision 3. The following table summarizes the disposition of each.

| Review-2 finding | Revision-3 disposition |
|---|---|
| H1 — reveal a match, not just a line | **Resolved.** US52, lines 212–215 specify target-row reveal using the first submatch's start cell, with one-third placement and horizontal reveal. |
| H2 — anchor algorithm contradicts content preservation | **Resolved.** US62–63, lines 217–220 specify a width-independent logical anchor `(source line, display-column offset)` with acknowledged EOF-clamp loss. |
| H3 — 90% file-list cap vs minimum content width | **Resolved.** US27, line 226 specifies 40% cap with minimum content reservation: `terminal width minus (gutter width + 10 + reserved indicator width)`. |
| H4 — outcome/exit-status contract | **Resolved.** Lines 149–172 provide a complete outcome table covering all rg exit codes, stream integrity, usable results, binary exclusions, record loss, and cancellation. |
| H5 — cumulative `-u` admits binary searching | **Resolved.** Line 129: "a third or later occurrence is a usage error." |
| H6 — asynchronous navigation consistency | **Resolved.** US36–42, lines 176–183 define late-result isolation, latest-target reveal, non-current failure policy, and retry rules. |
| M1 — stale-content rule checks bounds, not correspondence | **Resolved.** Lines 189–192 explicitly label validation as "best-effort correspondence," define stale-entry navigation fallbacks, and specify the persistent "file changed since search" note. |
| M2 — ripgrep transcoding by default | **Resolved.** US47, lines 187–188 specify UTF-16/32 BOM detection with "(unsupported encoding)" placeholder. Line 133 confirms no forced raw-encoding flag; rg's default BOM detection remains. |
| M3 — zero-width markers and removed line endings | **Resolved.** Lines 197–201 specify zero-width marker display, end-of-line extension, CRLF mapping, and grapheme policy. Line 299 includes relevant test cases. |
| M4 — safe rendering for replay and raw paths | **Resolved.** US22, US75, lines 202–206, 244 specify sanitization for every output sink including stderr replay, and raw-byte path identity for access and ordering. |
| M5 — scale target is not a memory bound | **Resolved.** US85, lines 248–251 state independent (non-simultaneous) scale examples, no aggregate memory bound, no eviction, and OOM risk. |
| M6 — error/help interaction and diagnostic access | **Resolved.** US78–84, lines 238–244 specify key precedence, error suspending help with scroll restoration, append-preserving-scroll, wrapping, and replay of all collected diagnostics. |
| M7 — right-edge marker hides another match | **Resolved.** Lines 229–233 reserve a rightmost indicator column that never overwrites text, and line 335 documents the deliberate left/right marker asymmetry. |

## Recommended resolution order

1. Clarify that load completion applies the entry reveal algorithm, including the first-visit starting viewport (M1).
2. Acknowledge mid-session diagnostic access limitations or specify a non-modal indicator (M2).
3. Clarify the ~50 MB scale example vs non-UTF-8 oversized records (M3).
4. Acknowledge `r`-during-load friction in the one-stop case (M4).
5. Specify overlay scroll position preservation through too-small round trips (M5).
6. Add `Esc` to user stories and specify its no-op behavior in normal browsing (L2).
7. Add confirmation notes for CRLF marker consistency (L1) and pop-up resize behavior (L3).

These are requirements clarifications, not a request to expand version 1 into new features. The approved design choices—non-interruption for non-current failures, one-load-per-path, indefinite buffer retention, non-streaming results, and the passive file list—should be preserved.
