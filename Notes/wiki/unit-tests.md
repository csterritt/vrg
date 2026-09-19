# Unit and boundary tests

Catalog of Go tests. Conventions: table-driven, `t.Helper()` in helpers,
externally observable behavior only. Child-argv assertions compare the
public `Result.ChildArgs` vector or the argv a fake `rg` actually
received, never library callback order. PTY tests drive the built binary
through `github.com/creack/pty` and synchronize on explicit conditions
(observed output, handshake files, exit), never settling sleeps.

## internal/cli

`cli_test.go` (external package `cli_test`):

- `TestHelpOnlyResults` — every help-only spelling (bare, first-token,
  later-token, combined `-ih`/`-hi`/`-xh`, `-help`, flag-preceded
  `-i --help`, search flags mixed with help, help after unsupported /
  excess / invalid-root tokens, and help beating the unrestricted limit
  and assignment rejection) yields `KindHelp`, one help copy, no search
  fields or child argv, and never touches the failing root-validation
  sentinel.
- `TestGeneratedHelpContent` — syntax line, `PATTERN`, `ROOT` with
  `(default ".")`, `-h`/`--help`.
- `TestGeneratedHelpListsSearchFlags` — every allow-listed flag appears
  in `cli.HelpText()` in short and long form.
- `TestOptionAssignmentFormsRejected` — every `=` spelling on search
  flags and on help (`--ignore-case=false`, `-i=false`,
  `--unrestricted=false`, `--help=false`, `-h=false`, `=true`/`=maybe`/
  empty-value variants) is an `ErrUnsupportedOption` usage error with no
  help output, no root validation, and no child argv.
- `TestAssignmentSpellingsAfterTerminator` — the same bytes are verbatim
  patterns after `--`.
- `TestSearchFlagsForwarded` — each allow-listed flag in both spellings
  forwards verbatim into `ChildArgs`.
- `TestOrderedChildArgs` — exact argv for `-i -s -i`, `-isi`, `-iwF`,
  mixed aliases, options interleaved with both operands, contradictory
  and repeated flags (no normalization), empty/`-`/dash-leading/literal
  `--` patterns.
- `TestUnrestrictedLimit` — cumulative `-u`/`--unrestricted` counting
  across separate, combined, and mixed spellings; two pass, the third is
  `ErrExcessUnrestricted` with no root validation or child argv.
- `TestFlagsOnlyMissingPattern` — flags without a pattern stay
  `ErrMissingPattern`.
- `TestPositionalsAndRoot` — the full table: default `.`, dir/file/
  symlink roots, empty and `-` patterns, `--` arity, unsupported and
  argument-taking options (`-e`, `--type`, `-t`, `-foo`), invalid help
  assignments, nonexistent/stdin/special/device roots; search rows assert
  the exact `ChildArgs`.
- `TestUsageErrorsPrecedeRootValidation` — non-root errors (including
  excess `-u` and assignment rejection) classified before validation via
  the failing sentinel.
- `TestDashFileRoot` — `./-` names a real file.
- `TestHelpLikeTokensAfterTerminator` — `-h`, `--help`, `--` as
  operands.
- `TestUsageDiagnosticSafety` — hostile bytes escaped in diagnostics.
- `TestDiagnosticsAreSpecific` — no bare `incorrect usage`.

`internal_test.go` (same package): `TestAppUsesContinueOnError` pins
`flag.ContinueOnError` on the adapter's app;
`TestHelpRendersEveryDeclaredOption` iterates `optionDecls` and asserts
every declared spelling reaches generated help;
`TestScanRecordsEveryDeclaredFlag` asserts the shared declarations drive
the scan (each declared flag recorded verbatim, help never forwarded).

## internal/searchindex

`index_test.go` (external package `searchindex_test`; Issue #3):

- `TestEncodingFormsRetainIdenticalBytes` — the same stream in `text`
  and in base64 `bytes` forms retains identical decoded path (resolved),
  line bytes, line number, submatch range, and recorded submatch bytes.
- `TestNonUTF8BytesRetained` — invalid-UTF-8 bytes in path, line, and
  submatch survive the `bytes` round-trip.
- `TestSamePathSameLineMerge` — two match records for one path/line merge
  into a single stop; submatches sorted by `(start, end)`; union
  highlights merge overlapping/touching ranges.
- `TestMixedEncodingSameValueMerge` — a `text` record and a `bytes`
  record carrying identical bytes for the same path/line merge into one
  stop (cross-encoding identity).
- `TestOverlappingSubmatchesUnionCoverage` — overlapping in-record
  submatches retained; `Highlights` is their union.
- `TestIndexOrderingRawPathThenLine` — files by unsigned raw path bytes
  (invalid-UTF-8 high bytes after ASCII), stops by ascending line number.
- `TestRelativePathResolution` — relative paths resolve against the
  working directory; interior `.`/`..` elements are not canonicalized;
  absolute paths pass through.
- `TestRecordKindsAndIgnoredContext` — all five known events classify;
  `context` contributes nothing; an unrecognized string type is
  `KindUnknown`.
- `TestMalformedRecordsSkipped` — invalid JSON/base64, missing or
  non-string `type`, missing/mistyped required fields, and out-of-range
  values all classify `KindMalformed` and contribute nothing (Issue #10
  counts them).
- `TestSubmatchRangeBoundaries` — ranges may touch the ends of the
  decoded line; zero-width matches are in range.

Issue #8 binary-exclusion and usable-results cases:

- `TestBinaryEndDropsFileMatches` — an `end` with non-null
  `binary_offset` (delivered in the other encoding) drops the file and
  its two collected match stops; the retained neighbour survives,
  `BinaryExcluded` is 1, `UsableResults` is 1.
- `TestBinaryExclusionCountsDistinctFiles` — two binary files count
  two; a repeated binary `end` re-drops interim matches without
  double-counting.
- `TestUsableResultsCountsRetainedStops` — usable results is retained
  stops after filtering: a summary-only stream and an all-excluded
  stream report 0; merged surviving matches count once.

`lifecycle_test.go` (external package `searchindex_test`; Issue #9) is
the lifecycle transition matrix — one `lifecycleCase` row per
disposition, fed through `Build` so the unterminated tail is exercised
through the real stream entry point:

- `TestLifecycleMatrix` — every matrix row: opens, duplicate and
  excluded-path begins, valid and orphaned matches (retained with
  `File.Incomplete`), the binary-exclusion precedence over orphan
  retention, valid/orphaned/duplicate/binary ends, lifecycle-neutral
  `context` in both positions, files still open at stream end, the
  summary-only zero-result stream, missing/second/post-summary records,
  the unterminated tail, `text`/`bytes` path-identity agreement, and
  independently tracked interleaved open files. Each row asserts
  `Integrity().Complete`, the retained files with their incomplete
  flags and stop lines, `BinaryExcluded`, and `UsableResults()`.
- `TestTrailingUnterminatedRecordDisposition` — `FeedTail` returns
  `KindMalformed` and marks the stream incomplete without indexing the
  fragment.

`disposition_test.go` (external package `searchindex_test`; Issue #10)
makes malformed versus integrity failure deterministic categories —
one fixture per matrix row, asserting `Index.Malformed`, `Index.Unknown`,
and `Integrity().Complete` together:

- `TestSchemaMatrixDispositions` — every Issue #3 per-record schema row
  (each missing/wrongly-typed required field, every invalid range,
  invalid JSON/base64, missing or non-string `type`) embedded between
  two valid `match` records inside a valid lifecycle: each is counted
  malformed, never dispatched, and the surrounding stops are still
  indexed — the resynchronization assertion. Lifecycle-neutral rows
  keep their own dispositions: a `data`-less `context` is valid, and an
  unrecognized string `type` counts unknown, never malformed.
- `TestLifecycleMatrixDispositions` — every Issue #9 integrity row
  (duplicate/orphaned `begin`/`end`, `match` after `end`, second
  `summary`, records after `summary`, missing `end`, missing summary)
  fails integrity without inflating the malformed count; an unknown
  type after `summary` counts unknown *and* fails integrity.
- `TestUnterminatedTailIsMalformedAndIncomplete` and
  `TestMalformedAfterSummaryIsMalformedAndIncomplete` — the two
  composite rows, each asserting both counters.

`oversized_test.go` (same external package; Issue #10) covers the
64 MiB contract:

- `TestOversizedBoundary` — exactly `MaxRecordBytes` accepted, one byte
  over discarded through its newline with resynchronization proved by
  the following records.
- `TestOversizedRecordPathDiagnostics` — a path-first oversized `match`
  lands its path in `OversizedPaths` while the oversized-only file is
  absent from the file list; a path-after-the-limit record is counted
  anonymously.
- `TestOversizedUnterminatedTailTripleDisposition` — the oversized
  unterminated tail asserts all three dispositions: oversized count,
  malformed count, incomplete stream.
- `TestUnknownTypeCannotSubstituteForSummary` — a stream ending on an
  unknown record without a `summary` fails integrity while counting
  unknown, not malformed.

`cursor_test.go` (same external package; Issue #13) covers the circular
matched-line cursor over real built indexes:

- `TestCursorStartsAtFirstStop` — startup selects the first stop in
  path-then-line order.
- `TestNextWalksStopsAndWraps`, `TestPrevRetreatsAndWraps` — both
  directions walk every stop in order and wrap circularly, with
  `Move.Wrapped` and `Move.FileChanged` reported independently.
- `TestWrapWithinOneFileReportsNoFileChange` — a one-file index wraps
  its stops with `Wrapped` alone.
- `TestSingleStopStrictNoOp` — one stop makes `Next`/`Prev` zero-Move
  no-ops that cannot move the cursor.
- `TestEmptyIndexNavigationNoOp` — no stops means no cursor position
  and explicit no-ops.
- `TestSubmatchesShareOneStop` — several submatches on one matched line
  are one stop.
- `TestCursorFollowsPreparedOrder` — the walk follows raw-path-then-line
  order, not stream order.

## internal/safepresentation

`safepresentation_test.go` (external package `safepresentation_test`;
Issue #5) covers the escaping core and the byte→cell maps:

- `TestEscapePath` — every path rule: `\n`/`\r`/`\t` escapes, backslash
  doubling, C0/DEL caret notation, C1 `\uXXXX` escapes, invalid-UTF-8
  `\xNN`, valid Unicode preserved.
- `TestEscapePathEmitsNoRawControls` — no control byte survives in the
  escaped path output.
- `TestMapContentPlainText` — printable text maps cell-for-cell with
  byte mappings.
- `TestMapContentEscapes` — C0/DEL caret forms, C1 `\uXXXX` escapes,
  invalid bytes as U+FFFD cells with retained byte mappings.
- `TestMapContentStandaloneCR` — a bare CR renders `^M`.
- `TestMapContentTabPlaceholder` — the provisional single-cell `→`
  placeholder for tab.
- `TestMapContentByteCellMaps` — escaped forms map every displayed cell
  back to the producing byte range.
- `TestMapContentWideGlyph` — wide graphemes occupy multiple cells all
  mapping to the cluster's bytes.
- `TestCellsCovering`, `TestCellsCoveringEscapedForms` — byte ranges map
  to covering cell ranges; a match over ESC covers both `^` and `[`.

`diagnostic_test.go` (external package; Issue #6) covers the diagnostic
presentation rules of `EscapeDiagnostic`:

- `TestEscapeDiagnostic` — real line boundaries preserved (LF kept,
  CRLF normalized to LF), standalone CR → `^M`, tabs expand to the next
  multiple of eight columns (including after a wide rune and reset per
  line), C0/DEL caret forms, C1 `\uXXXX`, invalid UTF-8 `\xNN`, literal
  backslash, printable Unicode kept.
- `TestEscapeDiagnosticEmbeddedFilename` — an `EscapePath`-escaped
  filename with a newline stays single-lined inside a multi-line
  diagnostic.
- `TestEscapeDiagnosticEmitsNoRawControls` — no C0 byte other than `\n`,
  no DEL, no C1, and valid UTF-8 in the output.

`internal/safepresentation/sinktest` is the shared test-support package:
`Fixtures` (the hostile fixture set), `Sink` rows, `AssertRawOutput`
(no-style raw-output cleanliness before ANSI stripping),
`AssertPayloadNotEscaped` (styled payload-after-ESC check), and `Run`,
the table driver.

## internal/filebuffer

`filebuffer_test.go` (external package `filebuffer_test`; Issue #5):

- `TestLoadCountsLines`, `TestLoadLineEndings`, `TestLoadEmptyFile` —
  LF/CRLF splitting, unterminated final line, no phantom trailing line,
  empty file.
- `TestGutterWidth` — digit width of the largest line number plus two.
- `TestLoadEscapedContent` — control bytes escaped per the
  safepresentation contract; cell count, not byte count.
- `TestHighlightSpans`, `TestHighlightCoversEscapedCells` — stop byte
  ranges become display-cell ranges; a match over an escaped byte
  covers the whole escape form.
- `TestStopBeyondFileIgnored` — stops outside the loaded file contribute
  nothing.
- `TestLoadReadFailure` — a read error is returned, not panicked.

## internal/app

`app_test.go` (same package; Issues #3–5), driving `Update`/`View` and
the `Init` command directly:

- `TestSearchingScreenShownDuringCollection`,
  `TestCompletionTransitionsToBrowse` — the model opens on
  "Searching…" and a completion message moves it to the browse state.
- `TestQOnBrowseQuitsThroughCollection` — `q` in browse returns the
  quit command with status 0.
- `TestResizeDuringSearching` — `WindowSizeMsg` handled without blocking.
- `TestGateHeldPreparationStaysSearching` — the gate holds index
  preparation after a completed `Wait`; the model stays searching until
  release (deterministic channel handshake, no sleeps).
- `TestRunStartFailureExit2`, `TestRunStartFailureSanitizesError` —
  `app.Run` with a failing `Start` returns 2 and writes a sanitized
  single-line diagnostic; the TUI is never entered.

`browse_test.go` (same package; Issue #5) drives the browse
composition, the async load lifecycle, and sink safety:

- `TestSearchDonePresentsBrowseWithLoading` — search completion
  transitions to `stateBrowse`, issues the load command, and the view
  shows "Loading…" until the buffer arrives.
- `TestLoadCompletionRendersContent` — `fileLoadedMsg` carries the
  prepared buffer; content rows render with gutter and filename rule,
  and the current matched line's match in the true inverse plus
  underline.
- `TestCurrentFileListEntryUnderlined`, `TestGutterRightJustified` —
  list order by raw path with the current entry underlined; the
  right-justified gutter plus two spaces.
- `TestCTogglesColourScheme` (Issue #7) — `c` flips the composed
  `View()` from the dark scheme's white-on-black base to light's
  black-on-white and back; the current matched line's match stays
  inverse and underlined in both.
- `TestGatedLoadStaysResponsive` — `WithLoadGate` holds the load worker
  while keys and resizes are still handled.
- `TestCtrlCDuringHeldLoadExits130`, `TestQOnBrowseExitsZero` —
  `ctrl+c` is 130 through the cleanup path; `q` in browse is 0.
- `TestResizeRecomposesBrowse` — a resize recomposes the frame.
- `TestHostileFixtureRawOutput` — the OSC/CSI/C0/C1/DEL/standalone-CR/
  invalid-UTF-8/embedded-newline fixture through the real composition
  path on the `theme.Plain()` no-style path; assertions run on raw
  output before any ANSI stripping (valid UTF-8, no surviving controls,
  escaped forms in all three sinks, frame still `height` rows).
- `TestLateLoadForOtherFileIgnored` — a completion for a non-current
  path cannot replace the visible panel.

`sinksafety_test.go` (same package; Issue #6) hosts the shared
sink-safety table `sinkSafetySinks` — seven rows over every sink
existing at this point (file-list entry, filename rule, panel content,
the Issue #9 error overlay, usage-error stderr, CLI-help stdout, and
the Issue #11 stderr replay) — and
`TestSinkSafetyTable` runs `sinktest.Run` over it: each
`<sink>/<fixture>` subtest asserts clean raw output on the no-style
path and, for the styled TUI rows, that no fixture payload follows an
unescaped ESC. Each row also proves the fixture reached the sink
(escaped name in the list/rule region, mapped content forms in the
panel, escaped diagnostic pieces in the overlay frame, `EscapePath`
operand in the stderr block); the help row injects the hostile operand
into argv while rendering help, which has no substitution points.

`rg_test.go` (same package) exercises the real `spawn` against fake `rg`
scripts on `PATH`:

- `TestSpawnArgvAndWorkdir` — the fake records its argv and `pwd`; the
  spawned child gets the exact vector and the supplied working directory.
- `TestDualPipeDrainage` — a fake writing ≈1.1 MiB to stderr interleaved
  with 18 stdout match records plus a post-write handshake file: `Wait`
  completes without deadlock, all stdout records arrive, stderr is fully
  buffered.
- `TestSpawnMissingBinary` — rg absent from `PATH` is a start error
  naming `rg`.

`cancel_test.go` (same package; Issue #4) covers the cancellation and
cleanup contracts. `killChild` blocks `Wait` until `Terminate`, so a
cleanup path that forgets to terminate hangs instead of passing;
`runQuittingCmd` requires the exit command to produce `tea.QuitMsg`
only after the child is reaped:

- `TestQWhileSearchingCancels`, `TestCtrlCWhileSearchingCancels`,
  `TestCtrlCOnBrowseCancels` — `q` while searching and `ctrl+c` in any
  state begin the controlled exit, terminate/reap the still-running
  child (reap report observed), and set status 130.
- `TestEscWhileSearchingNoOp` — `Esc` while searching changes nothing
  and returns no command.
- `TestLateCompletionAfterCancelDiscarded`,
  `TestQDuringGateHeldPreparationCancels` — a `searchDoneMsg` arriving
  after cancellation (including the one a released gate produces) cannot
  revive the browse view; gate-held `q` is 130.
- `TestOrdinaryQuitTerminatesRunningChild` — the browse `q` against a
  still-running `killChild` proves ordinary exits terminate and reap.
- `TestFailMsgTriggersCleanup` — `failMsg` records `failErr` and runs
  the same cleanup.
- `TestInitRunsFailureHook` — with `WithFailFunc` set, `Init` returns a
  two-command batch (collection + hook); the hook's error surfaces as
  `failMsg`.
- `TestRunCleansUpOnProgramError` — `app.Run` under a pre-cancelled
  context still terminates/reaps the child, fires the reap report
  exactly once, and writes one sanitized `vrg:` line, exit 2.

`replay_test.go` (same package; Issue #11) covers the session
diagnostic collection and the common post-restoration replay writer
`replayDiags` (driven over `m.diags` in tests, the bytes `Run` emits
once the terminal is restored):

- `TestReplayCollectsEveryDiagInOrder` — one overlay-displayed stderr
  warning plus two never-displayed load failures all collect in
  processing order, each replayed exactly once; the `WithDiagAck`
  acknowledgement fires once per collected line.
- `TestCtrlCReplayBoundary` — a `stderrLineMsg` processed before
  `ctrl+c` is replayed; a gated diagnostic still in flight is not
  waited for and never replayed.
- `TestQWhileSearchingReplayBoundary` — the same boundary on the `q`
  route, while searching and while result preparation is gate-held:
  the acknowledged diagnostic replays, exit 130, no wait on the held
  work.
- `TestControlledFailureEntersCollection` — the `failMsg` diagnostic
  enters the collection before shutdown and replays after the earlier
  diagnostics, exactly once.
- `TestCompletionDoesNotRecollectIncrementalStderr` — a child with the
  incremental `Diags()` channel does not have its captured stderr
  re-collected at completion.
- `TestReplayEscapesEmbeddedFilename` — a filename with newline and ESC
  bytes is `EscapePath`-escaped and single-lined in the collected and
  replayed diagnostic.

`noresults_test.go` (same package; Issue #8) covers the empty-outcome
contracts through the real collection command and `Update`:

- `TestEmptySearchShowsNoResultsScreen` — a complete rg-1 stream
  (summary only) presents the centred "No results found" screen, no
  binary suffix.
- `TestQOnNoResultsExitsOne` — `q` exits 1 through the cleanup path:
  the still-running `killChild` is terminated and reaped before quit.
- `TestAllBinarySearchShowsSkipCount` — an all-excluded stream shows
  "No results found (2 binary files skipped)" under rg code 0 and 1
  alike; `q` exits 1.
- `TestMixedBinaryStreamBrowses` — one excluded + one retained file
  browses with usable results 1; the excluded path never appears.
- `TestEscOnNoResultsNoOp` — `Esc` returns no command and leaves the
  screen up.
- `TestCtrlCOnNoResultsExits130` — `ctrl+c` overrides the fixed exit-1
  outcome with 130.

`outcome_test.go` (same package; Issue #9) is the single table-driven
outcome matrix — the PRD's outcome and exit-status contract as one
`outcomeCase` row per combination, so later issues extend the table
rather than duplicate the decision. Each row feeds a process result and
stream through the real collection command, then asserts the initial
presentation (base state, overlay open or not, frame contents), the
dismissal key's effect (which state it reveals — or that dismissal
itself exits the overlay-only fatal outcome), and the fixed exit
status. Rows cover rg 0 clean, the anomalous rg 1 with retained
results, empty complete streams, fatal codes and signal death with and
without usable results (both `q` and `Esc` dismissal keys), missing
summary and orphaned end integrity failures, warning stderr over browse
and over no-results, the all-binary warning with its skip count,
`Esc` never exiting a base state, and `ctrl+c` → 130 from browse,
no-results, and the open overlay. Issue #10 extends the same table:
unknown-type warnings with zero results (warning overlay → no-results →
1), malformed records skipped with usable results (browse + overlay →
0), malformed loss with zero usable results (`q` and `Esc` → 2), a
skipped record plus binary exclusion leaving zero retained stops (→ 2,
assessed after all filtering), and missing `end` with and without
retained matches (browse + overlay → 2 and overlay-only → 2).

`scroll_test.go` (same package; Issue #12) covers manual vertical
scrolling and the per-file viewport:

- `TestScrollKeysMoveRenderedRows` — `down`/`up` move the saved top by
  one rendered row, `d`/`u` by `floor(h/2)` of the content height, and
  `pgdn`/`pgup` by a full page, each returning no command; the rendered
  first content row shows the line at the new top.
- `TestScrollStopsAtEOF` — scrolling past EOF clamps at
  `rows − content height` with the file's last line on the panel's
  bottom row; further down keys change nothing.
- `TestScrollStopsAtBOF` — `up`/`u`/`pgup` at the top do nothing.
- `TestScrollShortFileLeavesUnusedRows` — a file shorter than the
  viewport cannot scroll; the unused rows below its content stay
  naturally blank.
- `TestScrollKeysNoOpOnPlaceholders` — every scroll key on the
  "Loading…" and "(unreadable)" placeholders changes nothing and
  creates no viewport state.
- `TestViewportStateSavedPerFile` — file A's scrolled top is recorded
  under A's raw path and survives an `n`/`p` leave-and-revisit (Issue
  #13's navigation now drives the file change) while file B keeps its
  own top-of-file state.
- `TestRenderQueriesOnlyVisibleRows` — the `countingRows` fake proves a
  frame render queries the `rowSource` only for `[top, top + content
  height)`, each visible row exactly once — no O(N) buffer scan.
- `TestResizeReclampsViewport` — growing the frame past the saved top's
  last valid position clamps it to the new `MaxTop`.

`nav_test.go` (same package; Issue #13) drives `n`/`p` through `Update`
over multi-stop fixtures (`navIndex`, `navFile`/`navStop`):

- `TestStartupCursorAtFirstStop` — startup selects the first file's
  first stop: its list entry is underlined and that line's match is
  inverse-plus-underline while the later stop's match is plain inverse.
- `TestNAdvancesWithinFile` — a same-file `n` returns no command and
  only moves the current-line underline.
- `TestNCrossingFileBoundarySwitchesPanel` — crossing to an uncached
  file's stop issues its load command, moves the list underline and the
  filename rule, shows "Loading…", and the completed file starts from
  the top with its stop's match current-line styled.
- `TestNavigationWrapsBothEnds` — `n` on the last stop wraps to the
  first and `p` on the first wraps to the last, across file boundaries,
  with cached destinations issuing no command.
- `TestSingleStopIgnoresNavigation` — the one-stop index makes `n`/`p`
  strict no-ops: no command, no cursor movement, no frame change.
- `TestManualScrollLeavesCursor` — scrolling does not move the cursor;
  `n` continues from the previously selected stop and leaves the
  scrolled viewport in place.
- `TestCrossFileRestoresDepartingViewport` — a scrolled file's saved
  top survives a navigate-away-and-back.
- `TestNavigationWhileLoadInFlight` — the cursor keeps moving while a
  destination's load is in flight, and the in-flight load is not
  reissued on return.
- `TestFileListHasNoDirectSelection` — enter/tab/arrows and other keys
  never move the cursor or change the current file: the file list is a
  passive overview.

`overlay_test.go` (same package; Issue #9) covers the modal overlay's
mechanics:

- `TestOverlayKeyRoutingAndScrolling` — `up`/`down` scroll the complete
  wrapped row set clamped to `[0, rows − visible]`; `pgup`, `pgdown`,
  and every other key — including `c` — are ignored while open.
- `TestOverlayDismissKeys` — `q` and `Esc` dismiss back to the base
  browse state with no command and no exit.
- `TestOverlayCtrlCExits130` — the global override outranks the modal.
- `TestOverlayWrapsUnbrokenDiagnostic` — a 200-cell unbroken diagnostic
  wraps within the single-line border; no rendered row exceeds the
  frame width and the whole text survives across rows.
- `TestOverlayKeepsCompleteDiagnostic` — a >1 MiB stderr diagnostic's
  head and tail are both in the scrollable row set (Issue #41's
  contract: one frame needn't show both ends).
- `TestGeneratedProcessDiagnostics` — a failed process without stderr
  gets the generated line naming the exit code or the signal.

## internal/viewport

`viewport_test.go` (external package `viewport_test`; Issue #12):

- `TestScrollUnitsMoveRenderedRows` — each scroll unit (one row, half
  page, full page) moves the top by its rendered-row count on a file
  longer than the viewport, symmetrically down and back up.
- `TestHalfPageUnit` — `HalfPage` is `max(1, floor(height / 2))` over
  odd and degenerate heights.
- `TestScrollClampByFileLength` — `MaxTop` and the EOF clamp over files
  shorter than, equal to, and longer than the viewport (plus empty and
  zero-height); a second scroll at EOF moves nothing.
- `TestScrollClampBOF` — scrolling up never takes the top below 0.
- `TestClampPullsTopUp` — `Clamp` pulls a stranded top up when the
  height grows or the content shrinks — the lossy EOF clamp.
- `TestPrepareRows` — `Prepare` maps each source line to one rendered
  row in file order with the buffer's gutter width; a nil buffer gives
  an empty model.

## internal/theme

`theme_test.go` (external package `theme_test`; Issue #7):

- `TestSchemeColourPairs` — `Dark()` is white on black (`37;40`),
  `Light()` black on white (`30;47`), as `Base` SGR pairs.
- `TestToggleFlipsSchemes` — `Toggled` flips dark↔light, is a pure
  value transform (receiver unchanged, nothing persisted), and is a
  no-op on `Plain()`.
- `TestMatchIsTrueInverse` — `Match` renders the scheme's colours
  swapped (dark match = light base pair and vice versa).
- `TestCurrentMatchUnderlines` — `CurrentMatch` adds `;4` to the
  inverse pair in both schemes.
- `TestIndicatorIsInverse`, `TestCurrentFileUnderlined`,
  `TestBaseColourStyles` — `Indicator` shares the match style;
  `CurrentFile` is base + underline while `FileList`, `Gutter`, and
  `FilenameRule` are plain base.
- `TestOverlayBorderBaseColours` — `Overlay` frames rows in a plain
  single-line border, all base colours, padding short rows.
- `TestPlainNoStyle` — every `Plain()` decorator is the identity;
  `Plain().Overlay` still draws the border without escapes.

## cmd/vrg (subprocess boundary)

`main_test.go` builds the real binary once in `TestMain` and asserts
stdout/stderr/status separately:

- **`TestGeneratedHelpStdout`** (named group, rerun by Issue #6) — all
  help rows exit 0 with exactly one help copy, empty stderr, no TUI, no
  terminal control sequences; includes search-flag help mixes and help
  beating the `-u` limit.
- **`TestCLIOutputSafety`** (named group, rerun by Issue #6) — hostile
  operand bytes escaped on stderr; a hostile-pattern search under an
  rg-free `PATH` keeps the start-failure diagnostic clean; no native
  library bytes.
- `TestHelpWithoutRipgrep`, `TestHelpDoesNotInvokeRipgrep` — help works
  with rg absent and never execs a sentinel fake rg.
- `TestHelpIgnoresExecutableName` — hostile argv0 cannot reach output.
- `TestExecutableBoundary` — the usage-error status table (unsupported
  `-e`/`--type`, third `-u` variants, assignment rejections, flags-only
  missing pattern): exit 2, empty stdout, a sanitized diagnostic first
  line on stderr, and exactly one generated usage block after it (no
  library `Error:`/`incorrect usage` text).
- `TestDashFileRootAtProcessBoundary` — `./-` in a real temp dir now runs
  the search under a pty and reaches the browse view.
- `TestHelpAssignmentSpellingsAreNotHelp` — `--help=`/`-h=` spellings
  (`=false` and `=true`) are exit-2 usage errors.

`search_test.go` (Issue #3) adds the PTY harness — `startVrgPTY`,
`runVrgWithQuit`, `waitForFile`, `fakeRG`, `testEnv` — and the named
boundary tests:

- `TestChildArgvAndWorkdir` — a fake `rg` records its argv and `pwd`
  through `VRG_TEST_ARGV`/`VRG_TEST_CWD`; the exact protected vector and
  the invocation working directory are asserted for no-flag, flag+root,
  and `--`-protected cases.
- `TestStartFailureExit2` — rg-free `PATH` gives the sanitized
  start-failure diagnostic and exit 2 with empty stdout and no TUI.
- `TestDualPipeBackpressure` — fake `rg` writes ≈1.1 MiB to stderr
  interleaved with 18 match records; the captured stderr opens the
  Issue #9 warning overlay, `Esc` dismisses it, the final content frame
  shows all 18 recorded matches as true-inverse spans (the `30;47`
  pair, since Issue #7; the current matched line's span also carries
  `;4`), the `VRG_TEST_HANDSHAKE` file exists (the child finished
  writing both pipes), and `q` exits 0.
- `TestStderrCapturedWithoutBlocking` — stderr diagnostics with a
  well-formed stream and exit 0 surface as the Issue #9 warning overlay;
  `Esc` dismisses it (the bare ESC resolves before the next key) and
  `q` exits 0.
- `TestGateHeldPreparationKeepsSearching` — with `VRG_TEST_GATE` set, the
  `VRG_TEST_COLLECT_ACK` file proves rg exited and the stream was
  collected while the screen still shows only "Searching…"; removing the
  gate file produces the browse view and `q` exits 0.

`cancel_test.go` (Issue #4; `//go:build unix`) extends the harness:
`startVrgTermPTY` opens the pty pair itself, keeps the slave fd, and
captures `term.GetState` before launch so `assertTermiosRestored` can
require the PTY input modes after exit to equal those before. The
`blockedRG` fake rg writes a ready file, records its pid, then `exec
sleep`s — only vrg's `Terminate` ends it. Assertions combine the exit
code, the recorded pid's absence, the `VRG_TEST_REAP` side-channel line
(vrg's own wait/reap proof), the display-restoration sequences
(`\x1b[?1049l`, `\x1b[?25h`), and termios equality:

- `TestCancelWhileSearchingKillsChild` (`q` and `ctrl+c` subtests) —
  cancellation exits 130 with no browse screen, the blocked child gone
  and reaped (`code=-1`), display and termios restored.
- `TestQDuringGateHeldPreparationCancels` — `q` after the collect-ack
  while the gate holds preparation exits 130; the released completion
  never renders; the already-exited child reaps as `code=0`.
- `TestOrdinaryQuitLeavesNoChild` — the browse `q` exits 0 with reap
  evidence (`code=0`), restoration, and termios equality.
- `TestControlledFailureCleanupExit2` — `VRG_TEST_FAIL` triggered after
  the child's ready signal: exit 2, the sanitized
  `vrg: injected test failure ^[[7m` diagnostic appears exactly once and
  after the restoration sequence, child gone and reaped, termios
  restored.

`outcome_test.go` (Issue #9) drives the outcome contract against the
real binary on a pty: `runVrgWithKeys`/`runVrgKillChild` script
interactions as `keyStep`s — send a key, then wait for its marker in
output written since the send, so a dismissal is proven by a fresh
repaint of what the overlay covered rather than by matching old frame
bytes:

- `TestFatalExitWithResultsShowsOverlay` — a fake rg exiting 3 after
  its handshake with two valid matches and stderr "boom": the error
  overlay shows the stderr, `Esc` reveals the covered browse content,
  `q` exits 2.
- `TestFatalExitNoOutputNamesExitCode`,
  `TestFatalExitNoOutputEscExits2` — a bare `exit 2` produces the
  generated code-naming diagnostic in the overlay-only presentation;
  `q` and `Esc` alike exit 2.
- `TestSignalDeathNamesSignal` — the fake rg records its pid and blocks
  mid-stream; the test SIGKILLs it and the overlay names the signal,
  `Esc` reveals the retained match's browse frame, `q` exits 2.
- `TestStderrWarningWithSummaryShowsWarningOverlay` — stderr "warn"
  beside a summary-only rg-1 stream opens the warning overlay over
  no-results; `Esc` reveals "No results found", `q` exits 1.
- `TestStderrContentFixture` — >1 MiB of stderr interleaved with a
  valid stdout stream: the stream completes (all 18 matches as
  inverse-video spans after dismissal), the captured stderr heads the
  scrollable diagnostic, the handshake proves both pipes drained, and
  `q` exits 0.

`replay_test.go` (Issue #11; `//go:build unix`) drives the replay
contract on the real binary: `waitForAcks` polls the
`VRG_TEST_DIAG_ACK` file — the application-side acknowledgement that a
diagnostic was processed into the session collection, the same
file-evidence family as `VRG_TEST_REAP` — and `assertReplayedOnce`/
`assertReplayOrder` require each diagnostic to appear exactly once
after the display-restoration sequence, in collection order:

- `TestCancelReplaysProcessedDiagnostic` (`q` and `ctrl+c` subtests) —
  a stderr diagnostic acknowledged while the child still runs replays
  exactly once after restoration, exit 130, termios restored, child
  gone and reaped.
- `TestQDuringGateHeldPreparationReplaysDiagnostic` — `q` after the
  acknowledgement while the preparation gate holds: cancellation exit
  130, replay exactly once after restoration.
- `TestNormalQuitReplaysDiagnosticsInOrder` — two warnings shown in the
  overlay replay exactly once each, in collection order, on the normal
  browse quit.
- `TestControlledFailureReplaysViaCollection` — `VRG_TEST_FAIL` after
  the acknowledgement: the earlier diagnostic replays first, the
  `vrg:` failure line second, each exactly once across the whole
  capture, exit 2.
- `TestReplayEscapesHostileFilename` — a stream path carrying ESC and
  LF bytes fails its browse load; the replayed `cannot read …`
  diagnostic is `EscapePath`-single-lined after restoration and the
  raw bytes never reach the terminal.
