# PRD: VRG — Terminal UI for ripgrep

Revision 6. Supersedes revision 5 and adds `github.com/jawher/mow.cli` for command-line parsing and built-in help, with help shown by default when invoked without arguments. Retains revision 5's reconciliation of the `Esc` termination rule with the outcome table per `Notes/critiques/PRD-tasks-critique-2.md` (finding F2). This document specifies version 1; acknowledged limitations are not promises of future features.

## Problem Statement

Developers use ripgrep (`rg`) to find text across a directory tree, then need to read the matches in context. Plain text output, pagers, and ripgrep's JSON event stream do not provide an integrated way to step through matched files and lines, read surrounding content, pan long lines, or adapt the display to ambient lighting without leaving the terminal.

## Solution

A terminal application, `vrg`, invoked as `vrg [flags] pattern [root]`. The search root defaults to `.` and may be a directory or a regular file, but not stdin. Invoking `vrg` without arguments, or requesting `-h`/`--help`, displays command-line usage and help without starting ripgrep or entering the TUI. For a valid search invocation, VRG runs ripgrep, collects its JSON results, and then presents a file list on the left and the current file's contents on the right, with matches highlighted.

Users navigate matched lines with `n`/`p`, scroll through full file contents, toggle wrapping and colour scheme, and consult modal help. Navigation reveals the first match on the destination line, even when that source line wraps across many screens. Partial results remain browsable. Unsupported encodings and unreadable files have explicit placeholders. Collected diagnostics remain recoverable through safe stderr replay after terminal restoration.

## User Stories

### Invocation

1. As a user, I want `vrg [flags] pattern [root]` to search a directory or regular file, so that I can browse either a tree or one file.
2. As a user, I want the root to default to `.`, so that searching the current directory needs no extra typing.
3. As a user, I want bare `vrg` and `-h`/`--help` to show command-line help on stdout and exit 0 without starting ripgrep or entering the TUI, while other missing-pattern invocations, excess operands, unsupported flags, and invalid roots produce sanitized usage diagnostics on stderr and exit 2, so that I can discover usage and distinguish help from invocation mistakes.
4. As a user, I want the documented no-argument ripgrep flags and combined short flags accepted, so that familiar search controls work predictably.
5. As a user, I want options accepted anywhere before `--` and forwarded in their original order, so that ripgrep resolves conflicting options consistently.
6. As a user, I want `--` to end option parsing and protect dash-leading patterns in the child invocation, so that those patterns are not mistaken for flags.
7. As a user, I want a third cumulative `-u` rejected, so that flag combinations do not silently enable binary searching.
8. As a user, I want stdin roots rejected and ripgrep configuration ignored, so that neither can undermine the disk-backed browsing model or output format.
9. As a user, I want a clear stderr diagnostic and exit 2 without a TUI if ripgrep cannot start, so that I know what to fix.

### Search lifecycle and diagnostics

10. As a user, I want a “Searching…” screen until result collection and processing complete, so that I know work is underway.
11. As a user, I want `q` during searching or `ctrl+c` anywhere to cancel outstanding work and exit 130, so that I can leave without waiting for the search.
12. As a user, I want a successful empty search to show “No results found” and exit 1 when dismissed, so that absence of matches is distinct from failure.
13. As a user, I want files identified as binary by completed ripgrep events excluded entirely, so that their earlier matches do not lead to binary previews.
14. As a user, I want binary-only results to show “No results found (N binary files skipped)” and exit 1, so that exclusions explain an empty list.
15. As a user, I want usable partial results browsable alongside an error overlay, so that a failed search does not discard useful work; a process or stream-integrity failure still yields exit 2.
16. As a user, I want a fatal search with no usable results to show an error overlay and exit 2 on dismissal, so that failure is unmistakable.
17. As a user, I want malformed and oversized records skipped and counted, with an oversized record's file path named in the diagnostic whenever it can be recovered, so that a damaged record does not necessarily discard the rest of the search and I can tell which file lost matches.
18. As a user, I want unknown JSON event types counted and reported without independently changing the exit status, so that new ripgrep event types are noticed.
19. As a user, I want missing completion metadata reported even when every received line is valid JSON, so that incomplete results are not passed off as complete.
20. As a user, I want matches from files missing an `end` event retained with an incomplete-search diagnostic, so that uncertain binary classification does not erase partial results.
21. As a user, I want nonfatal diagnostics with no results shown before the no-results screen, so that warnings are available without falsely turning an empty successful search into a fatal one.
22. As a user, I want all collected diagnostics safely replayed to stderr after terminal restoration, including diagnostics never shown in an overlay, so that nothing collected is silently lost.
23. As a user, I want every normal exit to restore the terminal and terminate/reap a running ripgrep child, so that quitting does not leave my terminal or subprocess behind.

### File list and layout

24. As a user, I want every retained matched file listed in deterministic path order, so that match navigation order is predictable.
25. As a user, I want original filename bytes retained for opening files and invalid bytes visibly escaped, so that display decoding neither changes file identity nor prevents file access.
26. As a user, I want the current file underlined and kept within the scrolled file list, so that I can always locate the active file.
27. As a user, I want the list just wide enough for its longest displayed path plus padding, capped at 40% of terminal width and further constrained by minimum file-content width, so that long paths do not crowd out readable content.
28. As a user, I want long paths left-truncated with `…`, so that their basenames remain visible.
29. As a user, I want `left`/`tab` to hide the list and `right`/`shift+tab` to show it, with the list initially shown, so that I control the space available for content.
30. As a user, I want the list to be a passive overview, with current-file selection controlled only by matched-line navigation, so that version-1 navigation has one consistent model.
31. As a user, I want a filename embedded in a horizontal rule above the file content, a right-justified line-number gutter followed by two spaces, and no other file-panel borders, so that context is identified without unnecessary decoration.
32. As a user, I want terminals narrower than 20 columns or shorter than 3 rows to show “Terminal too small” and automatically recover on resize with my selection, any open overlay, and that overlay's scroll position intact, so that small sizes degrade predictably and briefly shrinking the window loses nothing.
33. As a user, I want resizing to preserve selection and logical reading position except for documented end-of-file clamping, so that routine terminal changes are non-destructive.

### File loading, reloads, and changed content

34. As a user, I want full-file loading and expensive processing to leave input responsive, with “Loading…” shown for unavailable content, so that large files do not freeze the interface.
35. As a user, I want navigation to continue during loading, so that I can move past a slow file without waiting for it.
36. As a user, I want a late load result for another file to update only that file's cache, so that it cannot replace the file I am now viewing.
37. As a user, I want a late failure for a non-current file recorded without interrupting me with an overlay, accepting that I will discover it mid-session only by visiting that file or at exit through stderr replay, so that moving on is respected.
38. As a user, I want a current-file read failure to show an error overlay and “(unreadable)” while retaining its cursor stops, so that failure is clear without changing search order.
39. As a user, I want a failed file retried on re-entry from a different file or with `r`, but not on every matched-line step within it, so that retry remains possible without repeated error interruptions.
40. As a user, I want `r` to reread the current file and validate the original matches, without rerunning the search, so that I can refresh changed content without changing the result index.
41. As a user, I want reload to preserve reading position unless I navigate during the load, in which case the latest selected match is revealed with the normal reveal rules, so that asynchronous completion follows my latest intent.
42. As a user, I want repeated load requests for a file already loading to be dropped rather than queued, with the placeholder changing from “Loading…” to content or “(unreadable)” as the completion signal, so that reload and rapid navigation do not create overlapping reads for that file and I know when I may retry.
43. As a user, I want a failed reload to replace the old display with “(unreadable)”, so that old content is not silently presented as refreshed content.
44. As a user, I want out-of-bounds or text-mismatched submatches dropped and a persistent “file changed since search” filename-row note, so that detected stale highlights are not drawn.
45. As a user, I want stale navigation entries retained with a sensible clamped landing position, so that changed files do not unpredictably reorder the search results.
46. As a user, I want cached content intentionally stable until `r`, so that disk edits do not unexpectedly change what I am reading.
47. As a user, I want BOM-marked UTF-16/UTF-32 files to show “(unsupported encoding)” with a diagnostic and no highlights, so that transformed search offsets are not misrepresented as raw-file matches.
48. As a user, I want an empty file to show an empty panel with zero source lines, so that it is not confused with a one-line file.

### Match navigation and scrolling

49. As a user, I want `n`/`p` to navigate matched source lines, ordered by path then line number, so that multiple submatches on one line are one navigation stop.
50. As a user, I want startup to select the first matched line and, once its content loads, reveal its first match with the normal reveal rules, with the first `n` advancing to the second stop, so that browsing starts at the beginning even when the first file loads slowly.
51. As a user, I want navigation to wrap circularly and to be a no-op when there is exactly one matched line, so that the ends of the index need no special action.
52. As a user, I want navigation to reveal the rendered row containing the first submatch's start, not merely any row of its source line, so that a match far down a wrapped line is actually shown.
53. As a user, I want an already-visible target row to stay put and a hidden target row to land approximately one third down the content area, subject to clamping, so that navigation avoids unnecessary scrolling while providing context.
54. As a user, I want minimal horizontal scrolling to reveal an off-screen first-submatch start in run-off-edge mode, at startup and on every navigation action, so that same-file steps reveal their matches too.
55. As a user, I want a match wider than the viewport considered revealed when its start cell is visible, so that revelation is possible even for oversized matches.
56. As a user, I want manual scrolling to leave the matched-line cursor unchanged, so that `n`/`p` continue from the last selected stop.
57. As a user, I want a revisited file's saved vertical viewport used as the starting point, a first visit to start from the top of the file, and destination-match revelation to take precedence over both, so that revisits preserve context when possible without hiding the destination.
58. As a user, I want a brief centered filename pop-up when navigation selects a different file, so that I notice the transition even if loading is slow.
59. As a user, I want each pop-up to last one second or until a key press, with that key also performing its normal action, and to stay centered if the terminal is resized meanwhile, so that pop-ups never delay navigation or quitting.
60. As a user, I want `up`/`down`, `u`/`d`, and `page up`/`page down` to scroll one rendered row, half a page, and a full page respectively, so that I can read wrapped lines and large files at useful speeds.
61. As a user, I want scrolling clamped to valid content, so that I cannot deliberately scroll beyond the file into empty space.
62. As a user, I want a width-independent logical text anchor retained through rewrapping and wrap toggles, so that changing width does not reinterpret a sub-row ordinal as a different text location.
63. As a user, I accept that end-of-file clamping updates that anchor and can make a resize round trip lossy, so that the viewport does not retain avoidable blank space below EOF.

### Wrapping, indicators, and text display

64. As a user, I want wrapping on initially and `w` to toggle it, with blank continuation gutters aligned to the first row's text, so that long lines are readable by default.
65. As a user, I want `,`/`.` to pan one column, `<`/`>` ten columns, and `[`/`]` half the text-area width in run-off-edge mode, so that panning has useful speeds.
66. As a user, I want horizontal offset preserved through wrap toggles but reset on file changes before target revelation, so that mode changes retain my place while new files begin from their left edge.
67. As a user, I want each visible line's first gutter space to show inverse `_` for text hidden left or inverse `*` for a match entirely hidden left, so that hidden content is signposted per line.
68. As a user, I want a reserved rightmost indicator column in run-off-edge mode, showing inverse `*` only for the current matched line when a match is entirely hidden right, so that indicators never overwrite visible matches.
69. As a user, I want partially visible matches to count as visible for indicators, so that stars identify genuinely unseen matches.
70. As a user, I want zero-width matches rendered as one inverse-space cell, including at end of line or on an empty line, so that they are both visible and navigable.
71. As a user, I want tabs expanded to eight-column stops and one consistent grapheme/cell policy for wrapping, clipping, and highlights, so that Unicode and tabbed content do not corrupt layout.
72. As a user, I want partial-grapheme matches to highlight the whole grapheme, so that combining characters and wide characters have meaningful visible highlights.
73. As a user, I want LF and CRLF removed from display but retained in byte-coordinate mapping, with terminator-only matches displayed as an ordinary end-of-line zero-width marker, so that newline handling does not invalidate valid matches.
74. As a user, I want an unterminated final line counted, and a leading UTF-8 BOM invisible with correctly adjusted match coordinates, so that common text-file variants display correctly.
75. As a user, I want invalid content UTF-8 replaced and dangerous controls visibly escaped in every output sink, so that searched data cannot send terminal commands.
76. As a user, I want `c` to toggle white-on-black and black-on-white, initially dark, so that I can adapt to ambient lighting.
77. As a user, I want inverse-video matches, additionally underlined on the current matched line, so that matches and the current stop are distinguishable in both schemes.

### Overlays and operational limits

78. As a user, I want `h`/`?` to open modal help with all key bindings, so that controls are discoverable.
79. As a user, I want help to accept only scrolling, `q`/`Esc`/`h`/`?` to close, and `ctrl+c`, so that keys do not act on content behind it.
80. As a user, I want errors to suspend help and restore its previous scroll position when dismissed, so that interruptions do not lose my place.
81. As a user, I want new errors appended without moving the diagnostic reader, so that new messages do not replace or disrupt earlier ones.
82. As a user, I want help and diagnostics wrapped, including long unbroken strings, and vertically scrollable, so that text is accessible at usable terminal sizes.
83. As a user, I accept clipped overlays at tiny sizes, with normal layout restored on growth, so that version 1 need not introduce a separate compact overlay design.
84. As a user, I want `q` or `Esc` to dismiss an overlay, `q` alone to quit ordinary browsing, `Esc` to do nothing when no overlay is open, and `ctrl+c` always to exit 130, so that modal and emergency exit behavior are predictable and `Esc` never quits from a base state.
85. As a user, I want documented independent scale examples, their content assumptions, and memory limitations rather than an aggregate-capacity guarantee, so that I can judge whether a search is suitable for my machine.

## Implementation Decisions

### Invocation and child arguments

- Use `github.com/jawher/mow.cli` for command-line parsing and generated usage/help. Configure or adapt its integration to preserve the option placement, ordered forwarding, combined-short-flag, and sanitization contracts below rather than relying on library defaults to match them.
- Syntax: `vrg [flags] pattern [root]`; root defaults to `.`. Options may occur anywhere before `--`. The first positional is the pattern and the second is the root. Except for the help-only paths below, missing pattern or more than two positionals is a usage error.
- With no command-line arguments, display the library-generated help on stdout and exit 0. Also provide built-in `-h`/`--help` as local help options before `--`; they are never forwarded to ripgrep. Help describes syntax, the required pattern, optional root and its default, and supported flags. Help-only paths do not validate a root, start ripgrep, or initialize the TUI, and work even when ripgrep is unavailable. This command-line help is separate from the TUI's `h`/`?` key-binding dialog. A nonempty invocation missing its pattern (for example, `vrg -i`) remains a usage error unless help is requested.
- The search-flag forwarding allow-list contains only no-argument user flags: `-i/--ignore-case`, `-S/--smart-case`, `-s/--case-sensitive`, `-w/--word-regexp`, `-x/--line-regexp`, `-F/--fixed-strings`, `--hidden`, `--no-hidden`, `--no-ignore`, `-u/--unrestricted`, and `-L/--follow`.
- Expand combined short flags in their original order. Count unrestricted occurrences across all tokens, including long forms. Zero through two are allowed; a third or later occurrence is a usage error. Thus `-uu` is allowed, while `-uuu`, `-u -uu`, and equivalent combinations are rejected.
- Apart from local `-h`/`--help`, reject every other user option, including argument-taking options and `-e`. Preserve accepted option order and let ripgrep resolve interactions; vrg does not normalize contradictory settings.
- `--` ends vrg option parsing. A dash-leading pattern other than the lone positional `-` requires `--`; otherwise it is parsed as an option and rejected if unsupported. A literal pattern `-` is valid. A root `-` is rejected as stdin; an actual file named `-` can be addressed as `./-`.
- Validate the root as an existing directory or regular file before starting rg. Symlinks resolving to either are accepted; stdin, special-file roots, nonexistent roots, and failed root validation produce sanitized usage diagnostics and exit 2 without a TUI. Disk changes after validation remain possible and are handled as search/load errors.
- Child argument order: mandatory internal flags, ordered user flags, `--`, pattern, root. Mandatory flags are `--json` and `--no-config`. There is **no forced raw-encoding flag**: rg's default BOM detection remains enabled.
- Run rg from the invocation working directory and resolve relative result paths against that same directory, not by prepending the supplied root again. Preserve raw result paths rather than canonicalizing or merging aliases.
- Start failure produces a sanitized stderr diagnostic and exit 2, without entering the TUI.

### Result index, records, and stream integrity

- Collect results completely before browsing. Searching includes result parsing, filtering, sorting, and index preparation. Work remains cancellable and the UI remains responsive throughout.
- Recognize `begin`, `match`, `end`, `summary`, and `context`. Context is known but ignored because content comes from disk. Accept JSON `text` and base64 `bytes` representations for paths, lines, and submatch text.
- Retain original path identity, line number, submatch ranges, and recorded submatch bytes. Merge navigation stops by raw path and source line; order submatches by byte start then end. The navigation index is ordered lexicographically by unsigned raw path bytes, then ascending line number. Valid UTF-8 paths consequently have ordinary lexicographic UTF-8 text order. Display escaping does not affect ordering or identity.
- A non-null `binary_offset` in a valid file `end` event drops that file and all previously collected matches. Count distinct excluded files. Do not infer confirmed nonbinary status for a file lacking its `end`; retain its matches and report incomplete metadata.
- Malformed records include invalid JSON, invalid base64, missing/invalid type fields, and known events with missing or incorrectly typed required fields or invalid ranges. Skip the record and count it. Event fields needed for indexing or lifecycle validation must be validated; ignored context payloads do not need to become display data.
- An unknown string event type is skipped, counted separately, and reported as “N unrecognised record types skipped”; it does not independently alter the exit status. It cannot substitute for required known completion events.
- Maximum JSON record payload: **64 MiB**, excluding its newline delimiter. Consume and discard an oversized record through the next newline, count it separately, and continue parsing. Do not depend on a small default line-reader limit. A trailing unterminated record is counted as malformed and makes the stream incomplete.
- Oversized-record diagnostics name the affected file when recoverable. If the record's `type` and `data.path` fields were parsed before the limit was reached (the usual case, since ripgrep emits them before the line payload), report “oversized record skipped for <sanitized path>” in addition to the count; otherwise report the count only. Identification is best-effort: an oversized record whose path cannot be recovered is counted anonymously. A file whose only match records were oversized is absent from the file list, so this diagnostic is the user's only indication of that loss.
- A complete stream requires a valid summary and valid paired begin/end metadata for encountered files. Orphaned or inconsistent lifecycle records, missing end events, missing summary, or lost/malformed completion metadata make integrity fail. A summary alone can be a complete zero-result stream. A safely skipped match record does not by itself make otherwise intact lifecycle metadata incomplete.
- Process termination by signal or an unexpected exit code is a failure even if it occurs between complete records. Stream integrity and process success are assessed separately.

### Outcome and exit-status contract

“Usable results” means retained matched-line entries after record skipping and confirmed binary exclusion. It does not require that their disk contents subsequently load successfully. Nonfatal record-loss diagnostics comprise malformed/oversized record counts when completion metadata remains intact. Unknown-event counts and nonempty stderr are warning diagnostics, not independent failures.

Apply this table top to bottom; cancellation overrides a completed outcome when applicable.

| Condition | Initial presentation | After error dismissal | Eventual ordinary exit |
|---|---|---|---|
| `ctrl+c` in any state | Cancel work, terminate child, restore terminal | No further screen | 130 |
| `q` while searching/result preparation is incomplete | Same cancellation/cleanup | No further screen | 130 |
| No arguments or command-line help requested | Usage/help on stdout, no child or TUI | Not applicable | 0 |
| Usage/root validation or process start failure | Sanitized stderr, no TUI | Not applicable | 2 |
| rg exits other than 0/1, dies by signal, or stream integrity fails; usable results exist | Browse with error overlay | Browse | 2 |
| Same fatal conditions; no usable results | Error overlay | Exit | 2 |
| rg exits 0/1, stream complete, malformed/oversized records skipped, no usable results | Error overlay explaining record loss | Exit | 2 |
| rg exits 0/1, stream complete, usable results exist | Browse; error overlay if any diagnostics | Browse | 0 |
| rg exits 0/1, stream complete, no usable results, no fatal record-loss condition | No-results screen; warning overlay first if diagnostics exist | No-results screen | 1 |

- The empty screen is “No results found”, with “(N binary files skipped)” appended when applicable. It applies to all-filtered rg-0 results as well as rg-1 results.
- A complete stream with retained results is browsable even for an anomalous rg exit 1; retained-result count decides between successful browsing and successful emptiness once failure rules have been applied.
- Include stderr in diagnostics regardless of rg exit code. If a failed process supplies no explanatory stderr, generate a diagnostic naming its code or signal. Include record-loss and stream-integrity notes rather than presenting an empty error overlay.
- Safe record skipping with usable results yields exit 0 if rg and completion metadata otherwise succeeded. Unknown-type warnings alone never force exit 2, including with zero results.
- Once searching completes, the ordinary exit status is fixed. Unreadable, stale, or unsupported files do not change it. `ctrl+c` still overrides it with 130.
- `q`/`Esc` dismiss an error overlay; a fatal no-results overlay exits 2 on dismissal with either key, because there is no underlying state to return to. On the ordinary no-results screen, `q` exits 1. During normal browsing, `q` without a modal overlay exits with the fixed search-derived status.
- Cleanup applies on normal completion, cancellation, and application failures under vrg's control: restore terminal state, cancel outstanding work, terminate/reap a running child, and safely replay all collected diagnostics. OS-level forced termination or OOM cannot guarantee cleanup.

### File loading, cache, reload, and selection consistency

- Read full files asynchronously on first view. Retain successful buffers for the session; no eviction. At most one load is in flight per raw path. Requesting `r` or re-entering a path already loading does not start another load: the request is **dropped, not queued**, and there is no load cancellation. The user learns that a load has settled only by the placeholder changing from “Loading…” to content or “(unreadable)”; only then does `r` start a new load. In the one-stop index case there is no navigation-based escape from a slow load, so each retry attempt must complete before the next `r` takes effect.
- Load completion is associated with the requested raw path and request identity. It updates only that path's cache/status. It may affect the visible panel only when that path is current. When it does, apply the **same destination-reveal rules as file-change navigation** (see Navigation: target-row identification, visible-target no-scroll, one-third placement with clamping, horizontal reset and minimal horizontal reveal) to the **latest** selected cursor target, never a target captured from an earlier selection. “Reveal” in this section always means those rules, not merely bringing the target on-screen. For a first visit with no saved per-file vertical state, including the initial file at startup, the starting viewport before destination reveal is the top of the file with horizontal offset zero. Late messages after cancellation cannot revive the UI.
- Navigation remains active while loading. Scrolling a loading placeholder is a no-op. Other actions retain their normal state/modal meanings. Unloaded, unreadable, stale, and unsupported entries remain cursor stops.
- On current-file read failure, show an error overlay and “(unreadable)”. Non-current failures are diagnostic-only, not modal interruptions. Retrying on entry means crossing from a different file, not stepping to another matched line in the same failed file. `r` permits retry even with a one-stop index.
- **Mid-session diagnostic access is deliberately limited.** Diagnostics collected after the initial post-search overlay is dismissed (chiefly late non-current load failures) have no in-UI indicator and no on-demand review key in version 1. They become visible only when the user navigates to the affected file (which then shows its overlay and placeholder) or through stderr replay at exit. A long session can accumulate unseen diagnostics; this is an accepted consequence of the non-interruption policy, not a defect.
- The file-change pop-up starts at selection, not at load completion. The current path remains identifiable in the filename row during loading/placeholders; load completion does not restart the pop-up.
- `r` rereads current content and revalidates against the original search index. It does not rerun rg, add/remove cursor stops, or discover new matches. Show “Loading…” during reload; a failed reload replaces the old display with “(unreadable)”.
- Reload preserves the current cursor and logical viewport anchor, clamped to new content. Reload by itself does not reveal a match. If match navigation changes selection during the load, the latest selection's reveal takes precedence. Navigation away and back likewise uses the normal entry reveal rule.
- Cached content intentionally ignores disk edits until reload. A load observes the bytes it successfully reads, not a filesystem snapshot guarantee.

### Encodings and stale-content validation

- Keep rg's default encoding detection. Detect UTF-16 and UTF-32 BOMs in FileBuffer (check longer BOMs before overlapping shorter ones). Such files show “(unsupported encoding)”, no file text/highlights, and an explanatory diagnostic. They remain indexed and reloadable. Apply the same current/non-current notification distinction as loading failures. Do not run the stale-match guard against their raw encoded bytes.
- UTF-8 BOMs are supported and invisible at the beginning of a file. Ripgrep 15.x removes a leading UTF-8 BOM from searched line data under default detection. Maintain separate raw-file and rg-line coordinates: the first line's comparison/search view omits that BOM, and mappings back to disk account for its three bytes. Do not compare a BOM-adjusted rg offset directly with the unadjusted raw first line. Non-leading U+FEFF is not a file BOM.
- For supported text, retain original line bytes including terminators separately from display text. Check each submatch's line existence, range validity, and byte equality against its recorded match bytes (accepting both JSON encodings). On any failure, drop that submatch and mark the buffer stale; retain other valid highlights.
- This is **best-effort correspondence**, not a snapshot or regex re-evaluation. Same-text moves, changes outside matched spans, and zero-width assertion-context changes can remain undetected. Invalid UTF-8 display highlighting is best-effort as well.
- A stale buffer shows “file changed since search” in its filename row every time it is displayed. The note has no timer. `r` recomputes it: it disappears only if the newly loaded content passes validation.
- Stale entries remain navigation stops. If valid submatches survive on the line, reveal the first surviving submatch. If none survive but the line exists, use the first recorded start clamped to the available line bytes and mapped to a valid display location; clamp an end-of-line fallback to the last rendered cell when there is no marker cell. If the line is gone, land at the last source line's start. An empty file remains a zero-line panel. Fallbacks do not invent highlights or zero-width markers.

### Text, graphemes, and safe presentation

- Count LF and CRLF as line endings, not displayed content. Keep them in the original-byte comparison/mapping view. A missing final newline yields a final line; a trailing newline does not invent an extra empty line; an empty file has zero lines.
- Map removed terminator bytes and zero-width positions through the end of the original line to the display end-of-line position. For example, `$` at byte 4 of `hit\r\n` maps to display column 3. A match solely on removed terminator bytes becomes an end-of-line marker; a span also covering visible text highlights that visible text. The end-of-line marker produced by a terminator-only match on an LF or CRLF line is a single cell and is **not a special case**: it follows exactly the same reveal, wrap, clipping, horizontal-extent, and left/right indicator rules as any other zero-width marker, including counting as a “match/marker entirely hidden” for the gutter `*`.
- Use one consistent grapheme segmentation and terminal-cell-width policy for display mapping, wrapping, clipping, highlight expansion, navigation, indicators, and clamping. Tabs expand to the next multiple of 8 source-display columns, independent of gutter and horizontal pan. Invalid UTF-8 content renders as U+FFFD while retaining raw-byte mappings.
- Expand nonempty partial-grapheme spans outward to the entire cluster. A combining-only match within a base cluster highlights that cluster. A cluster without a base/independent visible cell must receive a visible fallback cell rather than an inaccessible zero-cell highlight.
- Wrap only at cluster boundaries. A two-cell cluster that cannot fit in a row's remaining cell moves to the next row, leaving a blank. Horizontal clipping that splits a cluster renders blank cells for the clipped portion. FileBuffer exposes enough cluster-boundary information for Viewport to avoid deriving wrapping from line widths alone.
- A zero-width submatch is shown as one inverse-video space at its mapped location, underlined on the current matched line. It marks an existing display cell without shifting following text; if at end of line, it extends effective line width by one cell. An empty matched line therefore has width one, and a marker after a completely full wrap row occupies another row. A position inside a cluster maps to that cluster's start; composition must not leave a split wide glyph. Marker cells participate in reveal, wrap, clipping, indicators, and horizontal extent.
- **Every output sink is sanitized**, including TUI content, filenames, pop-ups, help substitutions, usage errors, and stderr replay. Never emit searched-data control sequences as terminal instructions.
- For file content, tabs and line endings follow the structural rules above; other C0 controls and DEL use visible caret notation, including ESC as `^[`. C1 controls use explicit escapes such as `\u0085`. Escaped representations have byte-to-cell mappings for highlighting.
- For diagnostics, preserve actual diagnostic line boundaries and expand tabs; escape other controls with the same safe policy. For any filename embedded by vrg in a diagnostic, escape it first as a single-line filename, so filename newlines cannot become diagnostic paragraph breaks.
- For paths, escape newline/carriage-return/tab as `\n`, `\r`, `\t`; invalid bytes as `\xNN`; and literal backslashes as `\\`. Escape remaining controls safely. Preserve valid printable Unicode. Keep original bytes for access, identity, and ordering; displayed strings never become filesystem keys.
- Sanitization does not promise elimination of every Unicode visual confusable. Its terminal-safety contract is that raw control sequences from external data never execute.

### Navigation, viewport, and logical anchors

- One global cursor indexes matched source lines. Startup selects the first stop. `n`/`p` move circularly; zero entries are a no-op and one entry makes both keys strict no-ops, without a pop-up or retry. `r` is the explicit one-entry retry route.
- Current file derives from the cursor. Manual scrolling does not move it. There is no direct file-selection route in version 1.
- The display target is the start cell of the first submatch on the destination line (the marker cell for a zero-width match), subject to stale-entry fallback. At startup and every actual match-navigation transition, reveal the **rendered row containing this target**, not merely the source line.
- If that target row is visible, do not scroll vertically. Otherwise place it at zero-based row `floor(content height / 3)` by moving the viewport, clamped to valid top positions. At BOF/EOF, available content takes precedence over one-third placement.
- In run-off-edge mode, if the target cell is hidden horizontally, move by the minimum number of columns needed to make it visible. If already visible, retain the offset. Oversized matches are revealed by showing their start cell; full-span visibility is not required. If the target's own grapheme cluster is wider than the entire text area, no offset can paint its start cell: set the offset to the target's start column, render the in-window portion as clipping blanks, and treat the target as **geometrically revealed** in this one case so repeated navigation does not loop; indicator visibility still counts the cluster as not visible. Indicators otherwise treat any genuinely visible portion of a span as visible.
- On file change, use saved per-file vertical state as the starting viewport and then apply destination reveal. A file with no saved state (first visit, including the startup file) starts from the top of the file. Reset horizontal offset to zero and then apply horizontal reveal. The same sequence applies when a load completes for the current file (see File loading). There are no stories or tests for hypothetical future selection routes.
- Vertical scroll units are rendered rows. Content height is file-panel height minus the filename row. One-page movement is that height; half-page movement is `max(1, floor(height / 2))`. Horizontal half-screen movement is `max(1, floor(text width / 2))`. Pan/scroll commands clamp to valid extents; panning is a no-op in wrap mode.
- Horizontal pan clamping follows the **visible-lines extent policy**, confirmed by the product owner: the clamp is defined by the widest currently rendered source line, not the widest line in the file. The maximum valid offset is `max(0, S)`, where `S` is the largest cell index at which a grapheme cluster or end-of-line marker of that line starts and its cell width fits within the text width; at that maximum at least one whole cluster or marker cell of the line is fully painted, so the text area is never entirely blank because of clamping and never shows half a glyph. If no cluster of the line fits the text width at all, the maximum is 0; an empty buffer, a placeholder, or a view of only empty lines also clamps to 0. The clamp is re-evaluated whenever the visible line set changes (vertical scrolling, reveal, resize, list hide/show, gutter growth, wrap-toggle re-entry) and on every pan. Scrolling away from a long line into shorter lines re-clamps the stored offset leftwards, and the earlier offset is not restored when the long line becomes visible again; saved per-file horizontal state and horizontal reveal operate on the clamped value.
- Logical anchor is `(source line, display-column offset)` within that line, independent of wrap width. The effective top row after rewrap is the row containing that location, not the row with the same former ordinal. Preserve the logical column through rounding to a containing row.
- Turning wrap off displays the anchor's source line as one row but retains its logical column; turning wrap on restores the row containing that column. Horizontal offset is separate state retained through wrap toggles, subject to actual display clamping and file-change reset.
- User vertical scrolling replaces the logical anchor with the resulting top row's location. A match reveal that moves the vertical viewport likewise replaces it; a no-scroll reveal does not discard a retained logical column.
- End-of-file clamping may pull the effective top upward to avoid avoidable blank rows and **updates** the logical anchor to that resulting top. This is intentionally lossy: a subsequent shrink need not restore the old top. Wrap round trips preserve the anchor only when no scroll/reveal or EOF clamp replaces it. Files shorter than the viewport naturally leave unused rows; this is not overscrolling.

### Layout and indicators

- Measurements use terminal cells, never bytes or rune counts. Gutter width is decimal digit width of the loaded file's largest line number plus two spaces; zero-line/placeholder views use at least one digit slot.
- The fixed terminal minimum is 20 columns and 3 rows. Below either, show centered “Terminal too small” as space permits. Only `q` and `ctrl+c` are active: `q` exits with the applicable outcome (130 if searching), and `ctrl+c` exits 130. Preserve for recovery on resize: cursor selection, per-file viewport state and logical anchors, file-list visibility preference, wrap and colour settings, horizontal offset, and full modal state. **Modal state includes** which overlay is open, the help scroll position, the error-overlay scroll position, and any help-suspended-by-error relationship; all survive a too-small round trip unchanged, consistent with error-suspension scroll restoration. An active pop-up simply continues its timer; it is not displayed on the too-small screen. Do not let this screen trap the user behind an invisible modal overlay.
- Let the reserved right-indicator width be zero in wrap mode and one in run-off-edge mode. File-list width, when requested visible, is the nonnegative minimum of: longest sanitized path width plus two; `floor(0.40 × terminal width)`; and terminal width minus `(gutter width + 10 + reserved indicator width)`. Recompute after loading changes the gutter and after mode/size changes.
- A computed zero-width list draws no cells but does not change the user's requested visibility. No automatic visibility toggle occurs. Minimum text width takes precedence over the 40% cap; pathological dimensions beyond the resource envelope must not produce negative sizes.
- Paths too wide for their allotted cells are left-truncated with a leading `…`, without splitting graphemes. File-list scrolling keeps the active entry visible. The filename row embeds the safe path in a horizontal rule and includes buffer-status notes; truncate the path to make room for status where possible.
- File-panel content width is panel width minus gutter and reserved right-indicator column. All text visibility, panning, wrapping, and horizontal reveal use this text width. Continuation rows have a blank gutter and align with first-row text. No decorative left/right/bottom file-panel border is drawn.
- In run-off-edge mode, every visible source line uses the first trailing gutter space for inverse `_` if text is hidden left, upgraded to inverse `*` if a match/marker is entirely hidden left. Otherwise it stays blank.
- The reserved rightmost column is blank except on the current matched line's visible row, where inverse `*` means at least one match/marker is entirely hidden right. It never overwrites text. Both left and right stars may appear together.
- A partially visible match has no hidden-match indicator for that side. Visibility is calculated over actually rendered text/marker cells after grapheme clipping, excluding the reserved column. A split wide glyph rendered only as blanks is not a visible match.
- If the current matched line is vertically off-screen, its right indicator is absent; other lines' gutter indicators remain. Wrap mode has neither hidden-content markers nor a reserved right column.

### Colours, overlays, and key precedence

- Dark scheme is initially white on black; light is black on white. `c` toggles with no persistence. Matches use true inverse colours; current-line matches also underline. Current file-list item is underlined. Indicators use inverse style. Overlays use base colours and plain single-line borders.
- `ctrl+c` has global precedence and exits 130. The too-small screen and searching cancellation rules follow their dedicated contracts. In ordinary browse/no-results states, modal error takes precedence over help, then pop-up, then base-state keys.
- `Esc` is an overlay-dismissal key only. With no help or error overlay open—during searching, on the no-results screen, on the too-small screen, and in normal browsing—`Esc` is a no-op (it still dismisses a pop-up like any other key, and that is its only effect). `Esc` never exits from a base state; dismissing a fatal no-results overlay with `Esc` terminates with status 2 because there is no underlying state.
- Help opens with `h`/`?`. While open: `up`/`down` scroll rendered rows; `q`/`Esc`/`h`/`?` close; `ctrl+c` exits; other keys are ignored.
- Error overlay: `up`/`down` scroll; `q`/`Esc` dismiss; `ctrl+c` exits; other keys are ignored. New errors append, preserving the reader's scroll position rather than jumping to the bottom.
- Errors suspend help, retaining its position; closing the error restores help. Opening help or error cancels any pop-up. No suspended pop-up returns afterward.
- Help and error text wrap to available interior width, including unbroken strings; vertical scrolling reaches all rows at usable sizes. At tiny sizes overlays are clipped to the terminal, without a special borderless mode. Resize restores access.
- File-change pop-ups start at selection, are centered, and show a single-line safe path left-truncated to fit. Centering and truncation are computed from the current terminal size at every render, so a pop-up recenters and re-truncates on resize; resize neither dismisses it nor restarts its timer. Each has its own fresh one-second lifetime; a stale timer cannot dismiss a newer instance. Any key dismisses a pop-up and is routed normally in the same action.
- Maintain a session diagnostic collection independently of what was displayed. Replay each collected occurrence once, in collection order, to sanitized stderr after terminal restoration. This includes unknown-type warnings, late non-current failures, and diagnostics collected just before exit. Do not wait for unrelated file work to finish merely to collect hypothetical future errors. No persistent log is written.

### Resources and responsiveness

- Independent scale examples: approximately 10,000 matched files, 100,000 matched lines, or individual files around 50 MB. These are **not simultaneous capacity guarantees**. Aggregate match data, record expansion, decoded buffers, and visited-file retention determine memory demand.
- The ~50 MB file example assumes UTF-8 (or near-UTF-8) content with ordinary line lengths. Files with very long lines that ripgrep must emit as base64 `bytes` (invalid UTF-8) expand by roughly a third in the JSON stream; a single such line of tens of megabytes can produce a `match` record over 64 MiB even when the file itself is under 50 MB. Such matches are skipped and reported with the oversized-record diagnostic (naming the path when recoverable); they are not browsable.
- Retain loaded buffers indefinitely within the session. No aggregate memory bound, cache eviction, or reliable OOM recovery is promised. Large searches or many visited files may exhaust memory and terminate the process; graceful terminal cleanup cannot be guaranteed under forced termination.
- A 64 MiB JSON record limit is independent of source-file size: escaping or base64 can exceed it even for a source file under 50 MB. Oversized records use the explicit skip/count and integrity rules, not silent truncation.
- Disk I/O, parsing, sorting, decoding, mapping, and rewrapping must not perform unbounded work on the UI update path. Frame rendering should use prepared viewport data rather than scanning full buffers. Cancellation and normal state-specific input remain responsive during all expensive work. Exact concurrency/message designs are provisional; observable late-result isolation and cancellation are required.

## Module Design

Exact Go signatures and internal data structures are provisional. Stable contracts are raw path identity, matched-line navigation, retained original search data, safe display mapping, target revelation, outcome/status rules, and late-result isolation. All six modules will be tested. A shared safe-presentation utility serves every output sink without requiring another top-level module.

### CLI

- **Responsibility**: Own command-line parsing and generated help through `github.com/jawher/mow.cli`, validate search invocations, and produce the protected ripgrep argument vector.
- **Interface**: Inputs: arguments and root-validation environment. Outputs: a help-only result (safe generated help for stdout, exit 0), pattern/raw root/ordered expanded user flags/child arguments for a search, or a sanitized usage error (stderr, exit 2). No arguments and local `-h`/`--help` take the help-only path without root validation, child startup, or TUI initialization. Enforce cumulative unrestricted count, accepted root types, option placement, and mandatory internal flags. Failures: arity outside help-only paths, unsupported flag, excessive unrestricted flags, invalid root.
- **Tested**: yes.

### SearchIndex

- **Responsibility**: Own parsed result data, stream-integrity accounting, exclusions, and the circular matched-line cursor.
- **Interface**: Consume bounded JSON records; expose raw-path-sorted files, matched lines and original submatch bytes/ranges, binary and record-loss counts, unknown-type counts, and integrity diagnostics. Expose current stop, next/previous with file-change/wrap information, emptiness, and per-file entries. Empty navigation is an explicit no-op. Record errors may be recoverable; incomplete metadata marks partial failure. Does not decide application exit status alone.
- **Tested**: yes.

### FileBuffer

- **Responsibility**: Load/classify one file and turn content plus original match data into safe, display-ready source lines and validated highlights.
- **Interface**: Inputs: raw path and immutable per-file search data. Outputs: ready content, unsupported-encoding status, or read failure; source-line count, gutter width, raw/search/display coordinate mappings, grapheme boundaries/cell widths, highlight and marker spans, effective line extents, and stale state. Handle UTF-8 BOM adjustment before validation and exclude detected UTF-16/32 before stale checks. Caller owns scheduling, cache, retries, and notification.
- **Tested**: yes.

### Viewport

- **Responsibility**: Own logical reading position and derive rendered-row visibility in wrap and run-off-edge modes.
- **Interface**: Set prepared file content, dimensions, wrap state, and reserved indicator width; scroll/pan with clamping; retain/restore logical anchors; reveal explicit display targets; expose visible rows/cell ranges and hidden-left/right flags after clipping. Preserve logical versus effective top distinctions and the documented lossy EOF rule. Empty/unavailable content gives safe empty queries, not fictitious lines. Grapheme mapping is supplied by FileBuffer rather than recreated inconsistently.
- **Tested**: yes.

### Theme

- **Responsibility**: Own the active colour scheme and styles.
- **Interface**: Toggle; supply base, gutter, inverse match, underlined current match, indicator, overlay, filename-rule, and file-list styles. No external failure modes.
- **Tested**: yes.

### App

- **Responsibility**: Coordinate lifecycle, state-specific input, asynchronous work, cached files, overlays, safe diagnostic replay, cleanup, and frame composition.
- **Interface**: Bubble Tea `Init`/`Update`/`View`; messages for keys, resize, search completion with process/integrity data, path/request-keyed file completion/failure, prepared layout, and instance-keyed pop-up expiry. Derive outcomes from the table; preserve current selection against late work; implement reload versus navigation intent. Own process termination/reaping and terminal-restoration boundaries. Failure modes include start/terminal failures, partial searches, loading failures, and cancellation; preserve diagnostic access under normal controlled exits.
- **Tested**: yes, with model-level and subprocess-boundary coverage.

## Testing Decisions

Tests assert external behavior and stable contracts, not private representation. Use deterministic fixture bytes/JSON and injected messages; drive timers with explicit instance IDs. No screenshot suite or sleep-based synchronization. There is no existing Go test prior art in this greenfield repository; follow Bubble Tea's message-driven model testing style.

- **CLI**: no arguments and explicit `-h`/`--help` produce usage/help on stdout and exit 0 without root validation, child startup, or TUI initialization, including when rg is unavailable; help documents syntax, root default, and supported flags; flags-only missing-pattern invocations remain stderr usage errors with exit 2; local help flags are not forwarded and are treated as operands after `--`; generated help and parser diagnostics obey output sanitization; optional root; directory/regular file/symlink roots; stdin and special-file rejection; literal `-` pattern; dash-leading operands and protected child argv; options after pattern; `--`; every accepted/rejected flag; combined flags; cumulative `-u` across mixed short/long tokens; preserved precedence order; invalid roots/arity and sanitized errors.
- **SearchIndex**: `text`/`bytes` paths, lines, and submatches; duplicate same-line matches; raw-byte sorting and display collisions; binary exclusion after earlier matches; malformed JSON/schema/base64/ranges; unknown types versus known context; missing/orphaned/malformed end and summary; valid empty summary; death between complete records; trailing unterminated record; records at and beyond 64 MiB with resynchronization; single-stop no-op and circular navigation.
- **SearchIndex (oversized records)**: oversized `match` record with recoverable `data.path` reports the sanitized path; oversized record with the limit hit before the path is parsed reports count only; a file whose only records were oversized is absent from the file list while its path appears in the diagnostic.
- **FileBuffer**: LF/CRLF, no final newline, empty file, tabs, wide/combining/standalone combining clusters, partial-grapheme matches, control escapes, invalid UTF-8, leading UTF-8 BOM offset correction and invisible display, UTF-16/32 placeholders, stale out-of-bounds and same-length text replacement, surviving versus entirely dropped spans, reload clearing/preserving stale note, zero-width positions at beginning/end/empty lines and removed terminators; a `$`-only match on `hit\r\n` yields one marker cell at display column 3 with the same extent, wrap, clip, and indicator behavior as any other end-of-line marker. Verify bounds against original/search bytes rather than stripped display text.
- **Viewport**: match near the end of a source line taller than several screens; startup and same-file target revelation; visible-target no-scroll; one-third vertical placement at BOF/EOF; minimal horizontal reveal and oversized spans; marker extent after an exact-width line; grapheme-safe wrap/clip; logical anchor across width and wrap round trips; deliberate EOF-clamp loss; manual-scroll anchor replacement; horizontal-offset preservation through wrap; visible-lines horizontal extent clamping (a stored offset re-clamped when the widest line scrolls out, not restored when it returns; a paintable-boundary maximum at a trailing wide cluster); empty/stale fallback; saved revisit state overridden only as needed by reveal; first-visit reveal starting from top-of-file with horizontal offset zero.
- **Layout/indicators**: long paths in an 80-column terminal; fixed 20-by-3 minimum and recovery; too-small round trip restoring a scrolled help overlay, a scrolled error overlay, and an error-over-help stack at their prior scroll positions; 40% floor rounding; gutter growth after loading; list-width reduction to leave ten text cells plus run-off-edge indicator; zero-width list allocation without preference loss; tiny clipped overlays; per-line gutter versus current-line right marker scope; both sides hidden; partial visibility; a match in the last text cell with another farther right; split-wide-glyph blanks not counted as visible.
- **Theme**: both schemes, true inverse matches/indicators, current-match underline, current-file underline, consistent overlay styles.
- **App**: every outcome-table row including all-binary, unknown-only warnings with no results, record loss with/without results, missing completion metadata, stderr on rg 0/1, unexpected codes/signals, cancellation 130, and file-load errors not changing ordinary status. Test warning-to-no-results versus fatal-overlay-to-exit transitions.
- **Asynchronous App behavior**: controlled A→B→C completions; late success never replacing C; late failure diagnostic-only with no in-UI indicator until that file is visited or exit replay; entry into an in-flight path; latest same-file target applied at completion; slow-loading startup file whose first match lands at row `floor(height / 3)` after load completion, not merely on-screen; explicit reload preserving anchor; navigation during reload overriding it; duplicate `r` during an in-flight load dropped (no second load, no queued load) and `r` after the placeholder settles starting a new load; failed reload hiding old data; retry on entry/`r` but not same-file steps; unsupported-file reload; one-stop retry via `r`; pop-up timing at selection independent of load timing.
- **Overlays/output**: modal key routing and global interrupt; `Esc` closing help and error overlays; `Esc` as a no-op in browsing, no-results, searching, and too-small states; error suspending/restoring scrolled help; append preserving scroll; long unbroken diagnostic wrapping; pop-up truncation, fresh timers, dismissal-plus-action, recentering and re-truncation on resize without timer restart; too-small exit behavior; sanitized filenames/control sequences in all sinks; stderr replay including unseen diagnostics exactly once per occurrence, with readable diagnostic line breaks and escaped filename newlines.
- **Subprocess boundary**: use a controllable fake rg with explicit readiness/completion handshakes to verify child argv, stderr draining, actual cancellation and child termination/reaping, unexpected process death, and terminal restoration through a controlled terminal/PTY harness. A model assertion that a cancel command was emitted is insufficient. Avoid sleeps and do not depend on screenshots.
- **Responsiveness boundaries**: hold search processing, loading, decoding, or layout preparation pending through controlled worker gates and verify the UI still accepts relevant input and cancellation. Assert late completion cannot restore a cancelled or obsolete display. Supplement model checks with the subprocess boundary test rather than claiming they alone prove process cleanup.

## Out of Scope

- Interactive pattern entry, search-as-you-type, or rerunning the search inside the TUI. `r` reloads disk content only.
- Direct file-list selection, mouse support, editing, launching an editor, saving/exporting results.
- Search flags outside the forwarding allow-list (local `-h`/`--help` are supported separately), including argument-taking flags, multiline search, context controls, encodings, type/glob controls, count/files/quiet/invert modes, preprocessing, compressed-file searching, and a third unrestricted flag.
- Reading search input from stdin or accepting a special-file search root.
- Honouring ripgrep configuration; persistent preferences or diagnostic logs.
- Streaming results into browsing before search completion.
- Binary preview; viewing detected UTF-16/UTF-32 files or implementing their transcoding. Their retained search entries have an unsupported placeholder.
- Guaranteed exact highlighting for arbitrary non-UTF-8 input, arbitrary transformations, or content changes beyond match-byte validation.
- Search-time file snapshots, filesystem watching, automatic refresh, or regex re-evaluation on reload.
- File-buffer eviction, aggregate memory guarantees, and reliable recovery/cleanup under OOM or OS-forced termination. The per-record limit does not bound total memory.
- A special borderless compact-overlay mode or a guarantee that every overlay is readable without enlarging a tiny terminal.
- Mid-session review of collected diagnostics: no unseen-diagnostic indicator, status count, or on-demand diagnostics key. Diagnostics are seen in the initial post-search overlay, on visiting an affected file, or at exit via stderr replay.
- Load cancellation and load-request queuing. A duplicate `r` or re-entry during an in-flight load is dropped.
- Guaranteed identification of the file behind every oversized JSON record; the path is reported only when it was parsed before the record limit was reached.

## Open Questions

No product-choice questions remain from this revision interview. The contracts above record the selected behavior; this is not a claim that every implementation consequence has already been proven.

Exact Go signatures, message payloads, and the grapheme-mapping representation remain provisional implementation details owned by the implementer. Resolve them through module and cross-boundary tests without changing the observable contracts; any discovered conflict requiring a product change should return to the product owner rather than be silently reinterpreted.

## Further Notes

- Greenfield Go project using Bubble Tea, Bubbles, and Lip Gloss, with `github.com/jawher/mow.cli` for command-line parsing and generated help. ripgrep 15.x is the reference family; local 15.2.0 checks confirmed default UTF-16 transcoding, UTF-8 BOM removal, CRLF end-position offsets, and cumulative unrestricted semantics.
- Startup defaults: dark scheme, wrapping on, file list requested visible, first matched line selected and its first match revealed, horizontal offset initially zero.
- Retained refinements from earlier interviews: optional root; restricted flag forwarding; matched-line rather than submatch navigation; circular ordering; passive list; per-file vertical state; inverse colours and current-match underline; modal help/errors; non-streaming results.
- Revision-3 changes include a regular-file root, explicit reload, 40% list cap with minimum-content reservation, target-row reveal, width-independent anchors with acknowledged EOF loss, complete outcomes/cancellation, async selection isolation, match-text validation, encoding placeholders, consistent marker/grapheme mapping, raw path identity, safe all-diagnostic replay, and resource qualifications.
- Revision-4 changes are clarifications only, with no new features: load completion explicitly applies the file-change reveal rules and first visits start from top-of-file; mid-session diagnostic access is acknowledged as limited to visiting the file or exit replay; the ~50 MB scale example is qualified for content encoding and oversized-record diagnostics name the path when recoverable; duplicate `r` during a load is documented as dropped with the placeholder change as the completion signal; too-small recovery explicitly preserves overlay scroll positions; the CRLF terminator-only marker is confirmed as an ordinary zero-width marker; `Esc` is added to the user stories as dismissal-only and a no-op elsewhere; pop-ups recenter on resize; a target grapheme cluster wider than the text area adopts the geometric-reveal fallback (offset at its start column, clipping blanks, no navigation loop, indicators still not visible), recorded in *Navigation, viewport, and logical anchors* as the authoritative contract.
- Marker scope is deliberately asymmetric: left gutter markers apply to every visible source line; the reserved right indicator applies only to the current matched line. The earlier summary incorrectly scoped both to the current line.
- Horizontal reveal is **minimal scrolling**, not one-third-across placement. One-third placement applies only vertically. The later explicit interview answer supersedes the earlier ambiguous interpretation.
- Horizontal pan clamping follows the **visible-lines extent policy** with a paintable-boundary maximum, confirmed by the product owner after the issues review; the stored offset is destructively re-clamped whenever the visible line set changes. The rule is recorded in *Navigation, viewport, and logical anchors*; it supersedes the earlier unspecified reading of clamping to valid extents.
- Future direct selection and more rg flags may be considered separately; version 1 does not implement or test hypothetical interaction routes.
