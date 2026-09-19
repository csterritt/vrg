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
- `TestMapContentTabStops` — Issue #16's structural tab rule: a tab
  expands with blank cells to the next multiple of eight source-display
  columns (a tab already on a stop takes a full eight), every expansion
  cell blank and mapped to the tab byte, and the whole expansion one
  unbreakable `Cluster`.
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
  empty file (zero `Lines()` entries, one digit slot, three-cell
  gutter).
- `TestTerminatorsRemovedFromDisplayRetainedInRaw` — Issue #22: LF,
  CRLF, and mixed terminators produce no display cells while `Line.Raw`
  keeps the original bytes, terminator included.
- `TestStandaloneCREscapesNotTerminator` — a `\r` with no following
  `\n` is content: `^M` mid-line and on an unterminated final line.
- `TestTerminatorBytesMapToEndOfLine` — Issue #22: zero-width
  positions and spans covering only removed terminator bytes land on
  the display end-of-line position (byte 4 of `hit\r\n` → column 3),
  recorded as the empty cell span Issue #23's marker occupies.
- `TestSpanCrossingTerminatorHighlightsVisibleTextOnly` — a span
  covering visible text plus the terminator (`.*` over `hit\r`)
  highlights only the visible cells.
- `TestLeadingUTF8BOMInvisibleWithAdjustedCoordinates` — a leading
  `\xef\xbb\xbf` is not displayed, `Raw` retains it, `SearchBytes()`
  is the BOM-free rg-line view, cell offsets stay raw-file
  coordinates (rg offset 0 → raw byte 3), and later lines run
  unadjusted.
- `TestBOMLineTerminatorMapsToEndOfLine` — the terminator mapping
  applies in rg coordinates on a BOM line too.
- `TestNonLeadingFEFFIsOrdinaryContent` — `U+FEFF` mid-line-1 or at a
  later line's start is ordinary content: the `\ufeff` escape cells,
  unadjusted offsets.
- `TestGutterWidth` — digit width of the largest line number plus two.
- `TestLoadEscapedContent` — control bytes escaped per the
  safepresentation contract; cell count, not byte count; the trailing
  tab's blank expansion cells included (Issue #16).
- `TestTabExpandsToEightColumnStops` — Issue #16: leading, mid-line,
  and on-a-stop tab positions land text on the next multiple of eight,
  every expansion cell a blank mapped to the tab's byte range, and the
  expansion one `Cluster`.
- `TestLineClustersExposeBoundaries` — `Line.Clusters` exposes the
  shared grapheme segmentation: single-cell clusters for plain text,
  one multi-cell cluster per wide glyph or tab expansion.
- `TestHighlightCoversTabExpansion` — a stop over a tab's byte range
  highlights its whole expansion.
- `TestHighlightSpans`, `TestHighlightCoversEscapedCells` — stop byte
  ranges become display-cell ranges; a match over an escaped byte
  covers the whole escape form.
- `TestHighlightExpandsPartialCluster` — Issue #21: a nonempty span
  snaps outward to whole clusters for a start inside, an end inside,
  both ends inside one cluster (decomposed `é`), and both
  ends inside different wide clusters (`文`/`日`).
- `TestHighlightCombiningOnlyMatchExpandsToBaseCluster` — a match on
  only `U+0301`'s bytes highlights the whole é cluster, never a
  zero-width sliver.
- `TestHighlightStandaloneCombiningGetsFallbackCell` — a standalone
  combining mark's cell is the Issue #43 `◌`-plus-marks form occupying
  exactly one cell, and a match on it highlights that cell — never a
  zero-cell highlight.
- `TestHighlightWideGlyphPairNeverSplit` — a match touching any byte
  range of `文` covers both cells `[2,4)` together.
- `TestHighlightEmojiZWJCluster` — a partial-byte match inside
  `👨‍👩‍👧` expands to the whole two-cell ZWJ cluster.
- `TestZeroWidthMarkerPositions` — Issue #23: the marker cell each
  zero-width or terminator-only span records — beginning of line,
  inside text, inside a wide cluster's bytes and inside a combining
  cluster (both map to the cluster start — no split glyph), end of
  line, the LF terminator byte, an empty line and its terminator,
  the `$` on `hit\r\n` plus the `\r`-only and whole-`\r\n` spans all
  landing on display column 3, and a position past the line — each
  asserting `MarkerAt` and the recorded empty `[c,c)` span.
- `TestEOLMarkerExtendsEffectiveWidth` — `Line.Extent()` adds one
  only for an end-of-line marker: an empty matched line's extent is
  1, BOL and mid-text markers add nothing, `ab文` plus its marker is
  5, and BOL+EOL markers together extend once.
- `TestMarkerFeedsPaintableBoundary` — `MaxStart` counts the EOL
  marker as a one-cell candidate (3 even at width 1, so the marker
  can paint alone), a marker-only line reports 0, and a BOL marker
  adds no candidate of its own.
- `TestStopBeyondFileIgnored` — stops outside the loaded file contribute
  nothing.
- `TestLoadReadFailure` — a read error is returned, not panicked.

`stale_test.go` (same external package; Issue #29) drives the
stale-match guard — recorded submatches carried by the `stopSubs`
helper — and `Buffer.StopTarget`'s three landings:

- `TestCleanContentNotStale` — fully validating content stays clean
  and targets the first submatch's start cell.
- `TestStaleOutOfBoundsRangeDrops`,
  `TestStaleSameLengthReplacementDrops` — a range past the line's
  bytes and a same-length byte swap each drop the submatch, paint no
  highlight, and mark the buffer stale.
- `TestStalePartialSurvivalKeepsValidHighlight` — one of two recorded
  submatches surviving keeps its highlight, marks the buffer stale,
  and supplies the first-survivor reveal cell.
- `TestStaleAllDroppedClampedStart`,
  `TestStaleClampedStartEOLFallbackLastCell` — every submatch dropped
  with the line present resolves the recorded start's cell, and a
  start beyond the shortened line clamps through the end-of-line
  position to the last rendered cell — no marker, no highlight.
- `TestStaleMissingLineLandsOnLastLine` — a gone recorded line lands
  at the last source line's start.
- `TestStaleEmptyFileZeroLines` — the empty file stays a zero-line
  panel, still stale, the stop keeping its bare recorded line.
- `TestStaleRecomputedPerLoad` — the mark is per-load state:
  mismatched, clean, and mismatched-again contents flip it.
- `TestStaleValidationComparesSearchBytesNotDisplay` — a match on the
  raw ESC byte validates against search bytes where stripped display
  text could never equal it.
- `TestStaleCRLFTerminatorMatchValidates` — a match recorded on the
  removed CRLF terminator bytes validates and lands on the
  end-of-line marker.
- `TestStaleBOMAdjustedValidation` — rg coordinates index the
  BOM-stripped view: the adjusted range validates, the unadjusted
  misses and marks stale.
- `TestStaleZeroWidthSurvives` — a recorded zero-width submatch
  validates trivially and keeps its marker.

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

Since Issue #15 the helper `finishLoad` unwraps `tea.BatchMsg` via
`fileLoadOf` — file-crossing navigation batches the pop-up's expiry
command after the load leaf — and `navSendsNoLoad` (in
`popup_test.go`) asserts a crossing issued no `fileLoadedMsg` leaf.

`sinksafety_test.go` (same package; Issue #6) hosts the shared
sink-safety table `sinkSafetySinks` — eight rows over every sink
existing at this point (file-list entry, filename rule, panel content,
the Issue #9 error overlay, the Issue #15 file-change pop-up driven
through `popupFixtureView`, usage-error stderr, CLI-help stdout, and
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
retained matches (browse + overlay → 2 and overlay-only → 2). Issue #26
adds the `failLoads` field — index file positions whose loads fail once
the search settles, the current file's through the transition's own
load command under `failAllLoader` — plus three rows proving the fixed
status survives: all loads failing under status 0 (→ 0), a current-file
failure under status 2 (→ 2), and the composed usable-results-at-2
all-fail row (→ 2, failures confined to presentation and diagnostics).
Issue #29 adds the mirroring `staleLoads` field — positions whose loads
return stale content, the current file's under `staleAllLoader` and the
rest by injected `filebuffer.Decode(staleContent, stops)` buffers —
plus the all-stale row: every retained stop validates stale, the
filename row carries "file changed since search", and the fixed status
stays 0 (→ 0).

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

`wrap_test.go` (same package; Issue #16) covers wrap mode end to end:

- `TestWrapOnByDefaultBlankContinuationGutters` — wrap is on at
  startup: the long line's first rendered row carries the numbered
  gutter and a full text-width band, and its continuation rows carry a
  blank gutter with text aligned to the first row's column.
- `TestWTogglesWrapMode` — `w` swaps the keyed row model for
  run-off-edge (the long line becomes one clipped row with the
  reserved rightmost cell left blank) and back to wrapped.
- `TestRevealMatchDeepInWrappedLine` — a match near the end of a
  many-screen wrapped line is revealed on its own continuation row at
  `floor(content height / 3)`, behind a blank gutter.
- `TestResizeRebuildsRowModel` — a narrower frame rebuilds the row
  model at the smaller text width (more wrapped rows) and re-clamps
  the saved viewport.

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
  with cached destinations issuing no load leaf (the crossing's pop-up
  command still issues — Issue #15).
- `TestSingleStopIgnoresNavigation` — the one-stop index makes `n`/`p`
  strict no-ops: no command, no cursor movement, no frame change.
- `TestManualScrollLeavesCursor` — scrolling does not move the cursor;
  `n` continues from the previously selected stop, then Issue #14's
  reveal moves the viewport to the now-hidden target row.
- `TestCrossFileRestoresDepartingViewport` — a scrolled file's saved
  top survives a navigate-away-and-back as the reveal's starting point.
- `TestNavigationWhileLoadInFlight` — the cursor keeps moving while a
  destination's load is in flight, and the in-flight load is not
  reissued on return; the crossings issue only the pop-up's expiry
  leaf under the `popupStubTicks` seam.
- `TestFileListHasNoDirectSelection` — enter/tab/arrows and other keys
  never move the cursor or change the current file: the file list is a
  passive overview.

`reveal_test.go` (same package; Issue #14) covers the vertical
destination reveal over the `longFile`/`navIndex` fixtures — a 160×24
window (content height 23, so one-third placement is content row 7) on
the plain theme:

- `TestStartupRevealPlacesHiddenTarget` — once the startup file loads,
  a hidden first-stop target lands at `floor(h/3)`: top = 199 − 7 = 192
  for the line-200 stop.
- `TestStartupRevealVisibleTargetStaysTop` — a first stop visible from
  top 0 does not scroll and records no saved viewport state.
- `TestNavigationRevealHiddenThenBOF` — `n` to the hidden line-200 stop
  reveals it a third down; `p` back to line 5 clamps to top 0 — BOF
  content beats one-third placement.
- `TestNavBetweenVisibleTargetsNoScroll` — `n` between two on-screen
  stops moves only the cursor; the top and rendered rows are unchanged.
- `TestNoOpNavigationDoesNotReveal` — a one-stop index's `n`/`p` is not
  a transition: no reveal, so the manually scrolled viewport stays put.
- `TestMovingRevealReplacesSavedViewport` — a reveal that moves the
  viewport overwrites the saved top, and the later leave-and-revisit
  resumes from the revealed position.
- `TestRevisitRevealStartsFromSavedViewport` — a revisit's reveal starts
  from the saved top: the target row still visible from it means no
  scroll (a top-of-file start would have differed).
- `TestFirstVisitStartsFromTopThenReveal` — a cached-but-never-visited
  file's reveal starts from the top: a hidden target lands at row 7.
- `TestNavToUncachedFileRevealsOnLoad` — navigating into an uncached
  file issues the load; the reveal lands when it completes for the
  now-current file.

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

`popup_test.go` (same package; Issue #15) covers the file-change
pop-up, driving timers by injected `popupExpireMsg` instance IDs under
the `popupStubTicks` seam — never by real time — with `leafMsgs`
unwrapping navigation's `tea.BatchMsg` and `deliverLoad` feeding back
only the load leaf:

- `TestPopupStartsAtSelectionNotLoad` — the crossing's command carries
  the destination load plus the first instance's expiry; the box shows
  the escaped path over "Loading…", and load completion neither
  dismisses nor restarts the instance.
- `TestPopupCentredOnFrame` — the bordered box centres on the current
  size at render.
- `TestPopupStaleExpiryCannotDismissNewer` — a second crossing mints a
  second ID even to a cached file; the first instance's expiry is
  discarded, only the live instance's own expiry dismisses it.
- `TestPopupKeyDismissesAndActs` — `down` dismisses and scrolls, `q`
  dismisses and still quits, in the same update.
- `TestPopupEscDismissesOnly` — `Esc` in browsing dismisses the pop-up
  and does nothing else.
- `TestPopupRecentresOnResizeWithoutRestart` — grow and shrink
  recentre/re-truncate the same live instance, which still answers its
  own expiry — the timer never restarted.
- `TestPopupCancelledByErrorOverlay` — `openOverlay` clears the live
  instance and the pop-up never returns after the overlay is dismissed
  (Issue #26 owns which diagnostics open the overlay).
- `TestPopupLeftTruncatesLongPath` — an over-wide path renders as a
  leading `…` plus the basename tail inside the frame width.
- `TestPopupEscapesHostilePath` — control bytes and an embedded
  newline in the raw path render as the single-line escaped form; the
  frame keeps its row count.

`anchor_test.go` (same package; Issue #17) covers the logical anchor
through the app model, plus the `deliverLayout` helper that runs a
returned layout command and feeds its `layoutReadyMsg` back through
`Update`:

- `TestResizePreservesCursorAndAnchor` — after scrolling to a mid-line
  anchor inside a wrapped 500-cell line, narrowing and re-widening the
  frame keep the identical anchor, land the top on the row containing
  it (not the former ordinal, and the original top returns), and never
  move the matched-line cursor.

`layout_test.go` (same package; Issue #17) covers the off-`Update`
prepared-layout pipeline under the `heldLayouts` gate (workers signal
`entered` then block on `release`, run via `runCmd` goroutines) and
the `crossFiles` single-stop fixture:

- `TestGatedLayoutKeepsInputsResponsive` — while a layout worker is
  held, `n`/`p` move the cursor and record the pending reveal intent,
  `w` flips wrap and issues a newly keyed request without releasing
  the worker, a second resize is accepted likewise, `q` exits with the
  fixed status, and releasing all workers installs only the newest key
  and commits the pending reveal to the latest stop.
- `TestGatedLayoutCtrlCExits130` — `ctrl+c` while a worker is held
  cancels to 130 without waiting on it.
- `TestOutOfOrderLayoutCompletions` — three resizes mint three keyed
  requests; delivered out of order, the two stale ones change nothing
  (installed layout, saved viewport, and anchor all untouched) while
  the current key installs and restores the anchor's containing row.
- `TestRapidWrapToggleDiscardsSupersededLayout` — the wrap-off
  completion arriving after wrap toggled back on is obsolete; the
  second `w` is the matching fast path issuing no request, and the
  stale delivery changes neither the installed layout nor the anchor.
- `TestObsoleteLayoutForOtherFileDiscarded` — a superseded completion
  for a non-current file leaves that file's installed layout and saved
  viewport and the visible frame untouched; an obsolete completion for
  the current file does not consume its pending reveal intent, which
  commits when the matching layout installs.
- `TestStaleRevisionCompletionDiscarded` — a reload bumps the content
  revision, so a completion minted under the superseded revision is
  obsolete; the rev-2 request installs.
- `TestCachedFileStaleLayoutRequestsFresh` — navigating to a cached
  file whose installed layout no longer matches the live key pends the
  reveal, requests a fresh preparation, shows "Loading…" rather than
  stale rows, and commits the reveal on install.
- `TestCachedFileFreshLayoutFastPath` — navigating to a cached file
  whose installed layout still matches applies the reveal at once and
  issues no request.
- `TestRenderEscapesOnlyVisibleListEntries` — the `escapePath` seam
  counts provider queries: one frame over a 50-file index escapes only
  the visible list window plus the filename-rule path.

`pan_test.go` (same package; Issue #18) drives the horizontal pan keys
through `Update`/`View` under `theme.Plain()`, with `widenFrame`
resizing the fixture frame so the flat text width is exactly 40 cells
regardless of temporary-path length:

- `TestPanKeysShiftText` — `.`/`>`/`]` move the offset 1, 10, and
  `HalfText(40)` cells and the rendered text shifts left by the same
  amount; `<`/`,`/`[` move it back symmetrically to 0.
- `TestPanKeysNoOpInWrapMode` — all six keys leave the offset 0 and
  the frame byte-identical while wrapping.
- `TestPanKeysClampAtLeftEdge` — `,`/`<`/`[` at offset 0 move nothing
  and change no frame.
- `TestPanOffsetSurvivesWrapToggle` — `w` `w` keeps the offset 20 and
  the row still renders from cell 20.
- `TestPanResetsOnFileChange` — `n` to file B starts at offset 0 from
  the left edge; `p` back to A finds A's offset reset too — the saved
  10 is not restored.
- `TestPanClippedClusterRendersBlank` — at offset 3 inside a
  `ab文…` line's two-cell cluster the window opens with a blank cell
  where 文's second half was clipped, then the following text — no
  partial glyph anywhere in the row.
- `TestPanMaximumPaintsFinalCluster` — panning to the maximum (40 on
  `x`×40 + `文`) paints the whole trailing cluster then blanks; `.`,
  `>`, and `]` there change neither the offset nor the frame.
- `TestScrollReclampsOffsetLeftwards` — `down` past the 300-cell line
  re-clamps the stored 200 to the pad lines' 2; `up` returns to the
  long line without restoring it.
- `TestRevealReclampsOffset` — `n` to a same-file stop whose reveal
  scrolls into short lines re-clamps the offset; the destination
  match starts at cell 2, painted at the clamped offset, so Issue
  #19's horizontal reveal leaves it in place.
- `TestWrapReentryReclampsOffset` — offset retained through wrap,
  `pgdn` deep into short lines while wrapped, then `w` re-entry
  clamps it to 2.
- `TestResizeReclampsOffset` — shrinking the frame's height so the
  long line leaves the window re-clamps the stored offset.
- `TestPanKeysNoOpOnPlaceholder` — pan keys over "Loading…" create no
  viewport state and change nothing.

`hreveal_test.go` (same package; Issue #19) drives the minimal
horizontal reveal through `Update`/`View` under `theme.Plain()` in
run-off-edge mode:

- `TestSameFileNavTriggersHorizontalReveal` — over the three-stop
  `hrevealFile` (matches at cells 5, 300, 270): `n` to the cell-300
  match sets `off = 261` (`300 + 1 − 40`) with the match start at the
  right edge, `n` to the already-painted cell-270 match moves nothing,
  and wrapping `n` back to cell 5 reveals left to the target column.
- `TestStartupRevealAppliesHorizontalReveal` — toggling `w` before the
  first load lands makes the pending startup reveal commit against the
  flat layout: `off = 261` with the match start at the right edge.
- `TestFileChangeResetsOffsetBeforeHorizontalReveal` — a revisited
  file whose saved offset was 50 resets to 0 before the reveal, so its
  cell-10 target is already visible and the offset stays 0 rather than
  revealing to 10.
- `TestHorizontalRevealPaintsWideClusterAtRightEdge` — a match on `文`
  at cell 298 sets `off = 260` (`298 + 2 − 40`) and the row paints
  both cells of the glyph at the right edge.
- `TestHorizontalRevealClippedBlankCountsHidden` — a match on `文`
  whose start cell is inside the window but clipped blank by the right
  edge reveals to `off = 1`, painting the whole cluster.
- `TestHorizontalRevealUnpaintableClusterFallback` — a match on a
  six-cell tab cluster at text width 5 sets `off` to its start column
  2, the in-window cells render all blank, and repeated `n` round
  trips land on 2 again — no panning loop.
- `TestHorizontalRevealInertInWrapMode` — startup on a wrapped layout
  records no horizontal offset even with a far-off match.

`indicators_test.go` (same package; Issue #20) drives the
hidden-content indicators through `Update`/`View` under
`theme.Plain()` (and the dark scheme for the styling check), with
`flatIndicatorModel` sizing a 40-cell flat text width and `panTo`
landing exact offsets:

- `TestGutterMarkPerVisibleLine` — the first trailing gutter space
  per visible line: `*` for a match entirely hidden left, `_` for
  text hidden left (matched or not, even the whole line hidden),
  blank on the empty line.
- `TestRightStarFollowsCurrentLine` — the reserved right `*` belongs
  to the current matched line alone: it moves to line 2 on `n` while
  non-current lines with hidden-right matches stay blank.
- `TestRightStarAbsentWhenCurrentLineOffScreen` — paging the current
  line out of the window blanks every row's reserved column.
- `TestBothStarsAppearTogether` — one match entirely hidden left and
  another entirely hidden right show both marks at once.
- `TestPartiallyVisibleMatchShowsNoStar` — a match straddling an edge
  keeps that side's indicator off: blank reserved column on the
  right, `_` never `*` on the left.
- `TestLastCellMatchWithFartherMatchHidden` — a match filling the
  last text cell is painted (never overwritten); the star reports
  only the farther match, and pans away once that match paints.
- `TestSplitGlyphBlanksCountHidden` — a `文` clipped to blanks at
  either edge counts hidden: the left gutter upgrades to `*` and the
  current line earns the right `*`.
- `TestUniformLinesMarkEveryVisibleRow` — every visible row of a
  uniform-lines window shows `_` at a nonzero offset.
- `TestWrapModeDrawsNoIndicators` — wrap mode keeps both trailing
  gutter spaces and lets text claim the cell the flat layout
  reserves; the marks return on toggling back.
- `TestIndicatorsRenderInverseStyled` — under the dark scheme both
  marks render inside the `30;47` inverse SGR, not as plain text.

`pan_test.go` and `hreveal_test.go` row expectations gain the marks
at nonzero offsets — hidden-left text and matches mark gutters, and
the unpaintable-tab row shows the gutter `_` beside the reserved `*`
for its entirely-hidden-right match.

`grapheme_test.go` (same package; Issue #21) drives the rendering
half of grapheme-cluster highlight expansion under the styled scheme:

- `TestClipBlanksNeverPaintMatchCells` — a `文` clipped at the right
  edge and another plus a tab expansion clipped at the left edge each
  render their in-window cells as unstyled clip blanks — no inverse
  styled blank anywhere — while the hidden-match stars still appear.
- `TestWrapBoundaryBlankIsNotAMatchCell` — a cluster that cannot fit
  a row's last cell wraps whole; the boundary row's filler blank is
  unstyled and the wrapped row highlights the glyph whole.
- `TestCombiningOnlyMatchPaintsWholeGlyph` — a match on `U+0301`'s
  bytes alone styles the whole decomposed é.
- `TestStandaloneCombiningFallbackCellHighlighted` — a standalone
  combining mark renders the `◌` fallback cell highlighted.
- `TestCJKMatchPaintsBothCells` — a `文` match paints both cells
  inside one inverse-styled run.

`filelist_test.go` (same package; Issue #24) drives the file-list
layout contracts, with the `dropCells` cell-aware slicing helper
(`…` occupies three bytes but one cell):

- `TestFileListWidthFormula` — each of the three terms winning in
  turn, `floor(0.40 × w)` rounding at an odd width, the ten-cell
  text reservation outranking the 40% cap, gutter growth narrowing
  the list, and the zero clamp.
- `TestListHideShowToggles` — `left`/`tab` hiding and
  `right`/`shift+tab` showing, each issuing a re-keyed layout whose
  install reclaims the panel width.
- `TestListToggleIdempotent` — a repeated hide or show key changes
  nothing.
- `TestZeroWidthListRetainsPreference` — a computed zero width
  leaves the preference set; the list returns when the terminal
  widens.
- `TestLeftTruncateGraphemeSafe` and
  `TestListEntriesLeftTruncate` — the `…` tail truncation never
  splitting a grapheme, in the helper and in rendered list cells.
- `TestFilenameRuleStatusSlot` and `TestFilenameRuleNoNote` — the
  synthetic note inside the rule with the path truncated around it,
  the note clipping under a tiny width, and the note-free rule
  unchanged.
- `TestListAutoScrollsToActiveEntry` — `n` crossings scrolling the
  list window so the active entry stays visible.
- `TestAnchorSurvivesListToggle` and
  `TestAnchorSurvivesGutterGrowth` — the same logical line at the
  top of the panel after a hide/show round trip and after a
  gutter-widening file load.
- `TestHiddenListEscapesNoEntries` — the render-cost guard: a
  hidden list escapes zero entries; a visible one escapes only the
  visible window's.

`nav_test.go`'s `TestFileListHasNoDirectSelection` drops the
`tab`/`left`/`right` passive-key rows now that they toggle
visibility, and `wantUnderlinedEntry` compares against the
`leftTruncate`-clipped entry. `layout_test.go`'s `frameWidthFor`
delegates to `frameWidthForG` — frame sizing under the new formula
with the gutter given explicitly. The panning, indicator,
grapheme, and wrap suites move their panel-row slicing to
`dropCells`, and `TestRevealMatchDeepInWrappedLine` uses a deeper
fixture target since the wider text panel leaves a shallow one
inside the initial window.

`loadiso_test.go` (same package; Issue #25) drives the keyed
load-isolation contracts through gated workers — `heldLoads` holds
every load and counts entries; `heldFirstLoad` holds only the slow
file's; `reqOf`/`mintRequest` read and mint request identities:

- `TestNavigationRemainsActiveWhileLoadHeld` — `n` moves past the
  held load at once (cursor, filename rule, pop-up, destination's
  own request), placeholder scrolling keeps the viewport at top 0,
  `w`/`c`/resize apply, and A's later completion caches without
  disturbing B's panel.
- `TestLateCompletionCachesOnlyItsOwnPath` — A→B→C with A's load
  held: A's completion landing while C is current caches A (even its
  layout installs invisibly) and leaves C's panel, cursor, and
  viewport untouched.
- `TestReentryDuringLoadStartsNothing` — `p` back onto a still-held
  path mints no second request: the crossing batch is only the
  pop-up leaf, the live identity is unchanged, and settlement
  leaves nothing queued.
- `TestCachedRevisitIssuesNoLoad` — a revisited file serves the
  retained buffer with no `fileLoadedMsg` leaf.
- `TestUnrequestedCompletionDropped` — a completion naming a path
  with no request in flight is discarded: no cache, no failure
  state, no diagnostic, no panel change, no issued work.
- `TestStaleCompletionDroppedWhileLoadInFlight` — a completion not
  carrying the live request's identity leaves the request pending
  and the placeholder up; the real answer still lands on release.
- `TestSettledCompletionDropped` — a duplicate completion after
  settlement replaces nothing: buffer, revision, layout, and panel
  all stay put.
- `TestPostCancellationCompletionDiscarded` — success and failure
  completions released after `ctrl+c` mutate nothing.
- `TestGatedDecodeMapKeepsInputsResponsive` — with `decodeGate`
  holding the phase after `Read`, `n`/`p` navigate, `w`/`c` toggle,
  resize applies, and `ctrl+c` exits 130 without waiting; the
  released completions land on the cancelled UI as non-events.

`readfail_test.go` (same package; Issue #26) drives the read-failure
notification and retry contracts through the `WithLoader` injected
loader — `failAllLoader` fails every read, `failPathsLoader` fails
listed raw paths, `gatedFailLoader` arms a mutex-guarded fail set a
gated worker's read observes — plus `heldNthLoad`, the gate holding
only the nth minted load (the re-entry retry is a deterministic
ordinal):

- `TestCurrentFileFailureShowsOverlay` — a current-file failure opens
  the overlay with the `cannot read …` diagnostic, shows
  "(unreadable)", keeps the filename row naming the path, and the
  retained stops still step without minting a load or a new overlay.
- `TestNonCurrentFailureIsDiagnosticOnly` and
  `TestNonCurrentFailureAppearsInReplay` — a non-current failure
  changes no frame and opens no overlay, yet collects the diagnostic
  that reaches the stderr replay.
- `TestCrossFileEntryRetriesFailedFile` — re-entering a failed file
  shows the prior-failure overlay with "Loading…" and mints exactly
  one retry; its second failure appends once to the still-open
  overlay and the collection.
- `TestUnreadableComposedViewAtConstrainedWidths` — the unreadable
  state at 80/30/20 columns: the …-truncated path in the filename
  rule, the placeholder up, no row overflowing, nonnegative layout
  dimensions.
- `TestReentryShowsPriorFailureAndStartsOneRetry` — the gated
  re-entry: prior-failure overlay and "Loading…" immediately, one
  minted retry, and a re-entry during the in-flight retry dropped by
  the one-load-per-path rule.
- `TestReentryRetryEscLeavesLoadUndisturbed` — `Esc` dismisses the
  overlay while the request stays live and held; the second failure
  then restores "(unreadable)" and re-opens the overlay.
- `TestReentryRetrySuccessKeepsPriorOverlay` — a successful retry
  shows content, collects nothing new, and leaves the prior-failure
  overlay up until dismissed.
- `TestReentryRetrySecondFailureAppends` — the append-preserving
  scroll primitive: the reader's `overlayScroll` survives the second
  failure's single appended occurrence, mirrored once in the
  collection.
- `TestReentryRetryAwayAndBack` — navigating away lets the retry
  settle as a non-current diagnostic-only failure; a later re-entry
  runs the same sequence against the new prior state.

`reload_test.go` (same package; Issue #27) drives the explicit-`r`
reload contracts through the existing seams — `heldNthLoad` holds the
reload's ordinal load, `gatedFailLoader` flips read outcomes, and the
new `layoutHold` arms a layout gate mid-test so the initial install
runs through while later workers are held; `reloadCmd` presses `r`,
`rewriteFile` replaces a fixture's bytes on disk:

- `TestRShowsLoadingAndRereadsOnce` — `r` shows "Loading…" over the
  filename row still naming the path, mints exactly one reread under
  the path's identity, and leaves the index pointer, the cursor, and
  the pre-request revision untouched.
- `TestDuplicateRDroppedNotQueued` — a second `r` while the reload is
  held issues no leaf, re-mints nothing, and runs no extra worker;
  after settlement `r` mints a fresh request.
- `TestReentryDuringReloadDropped` — an `n`/`p` there-and-back onto a
  path whose reload is held mints no second load and keeps the
  placeholder up.
- `TestReloadPreservesAnchor` — the intent waits for the matching
  layout: while the new revision's worker is held the placeholder and
  the saved viewport are untouched, and on install the anchor lands on
  its row in the new rows — position preserved, cursor unmoved, no
  reveal.
- `TestReloadAnchorClampedOnShrink` — a shorter reread hits the lossy
  clamp: the top pulls to `MaxTop` and the anchor rewrites to the
  clamped top's location.
- `TestFailedReloadReplacesContent` — the failed reread drops the
  buffer and layout, shows "(unreadable)" plus the overlay, keeps the
  filename row, and collects the one diagnostic.
- `TestSecondConsecutiveReloadFailureAppends` — `r` fires while the
  overlay is open (the retry route it cannot block, never dismissing
  it); the second failure appends exactly one occurrence with the
  reader's `overlayScroll` preserved and mirrored once in the
  collection.
- `TestReloadSuccessLeavesOverlayOpen` — a successful reread under the
  prior-failure overlay shows the new content behind it and leaves the
  overlay up until `Esc`.
- `TestRWorksWithOneStopIndex` — the only retry route a single-stop
  index has still rereads and bumps the revision.
- `TestDiskChangeWithoutRIsStable` — a simulated disk edit changes
  nothing without `r`: resize/scroll/re-entry mint no load and the
  cached buffer, revision, and rows stay.
- `TestPreReloadLayoutSupersededDiscarded` — a layout prepared for the
  pre-reload revision and released after completion is discarded
  without touching the installed model, viewport, anchor, or panel;
  the new revision's layout then installs and the anchor commits.

`loadreveal_test.go` (same package; Issue #28) drives the two-stage
load-completion contract with the stages separately gated —
`deliverStageOne` feeds the load completion and returns the layout
command stage one issued, while `consultedRows` records every
`rowSource` call as the no-row-decision probe:

- `TestLoadCompletionStageOneDefersToInstall` — stage one caches the
  buffer, bumps the revision, requests the current key, and records
  the reveal intent with no viewport state created and "Loading…"
  still up; the commit lands the hidden target at 192 on install.
- `TestLoadCompletionMakesNoRowDecision` — a `consultedRows` fake
  standing in as installed sees zero calls through the fabricated
  completion: no visibility test, placement, clamping, or horizontal
  reveal at stage one, and the intent commits on the matching install.
- `TestStartupHiddenTargetCommitsAfterBothStages` — "Loading…" holds
  through the held load and again through the held layout; the
  install's reveal lands row 199 at content row 7 and the first `n`
  advances to the second stop.
- `TestStartupVisibleTargetKeepsTopThroughStages` — a first stop
  visible from top 0 commits without scrolling or recording state.
- `TestResizeBetweenLoadAndLayoutCommitsAtNewWidth` — the resize
  mints a replacement request, the old-width completion is discarded
  without consuming the intent or moving the viewport, and the commit
  lands at the new width.
- `TestNavigateWhileLayoutPendingCommitsNewestTarget` — two `n`s
  during the held layout move the cursor at once; the commit reveals
  the newest selection's target.
- `TestListHideBetweenStagesCommitsAtFinalWidth` and
  `TestGutterGrowthBetweenStagesCommitsAtFinalWidth` — a `left` list
  hide or a gutter-widening reload between the stages re-keys the
  request; the installed model carries the final text width.
- `TestStaleRevisionLayoutDiscardsWithoutConsuming` — a revision-1
  layout arriving after the revision-2 reload is discarded leaving
  the installed model and the anchor intent untouched; the revision-2
  install then commits the anchor.
- `TestSavedViewportRevisitVisibleStays` /
  `TestSavedViewportRevisitHiddenMoves` — revisiting a file whose
  layout went stale pends the reveal; the commit keeps the saved top
  when the target is visible from it and moves to the BOF-clamped
  third when hidden.
- `TestMarkerTargetCommitPaintsMarkerCell` — a terminator-only `$`
  stop commits to the marker's row in run-off-edge mode with the
  offset staying 0 and the inverse-underline marker cell painted.
- `TestClusterTargetCommitPaintsWholeCluster` — a submatch starting
  mid-cluster commits to the cluster's start cell: the horizontal
  reveal moves `off` to 260 so the whole two-cell 文 paints at the
  right edge.
- `TestNonCurrentCompletionLeavesPanelUntouched` — a fabricated
  completion for a non-current file records no intent and leaves the
  panel, viewport, and foreign intents untouched.
- `TestPopupSurvivesBothStages` — the same pop-up instance survives
  the load completion and the layout install.

`reloadintent_test.go` (same package; Issue #28) drives the
reload-versus-reveal arbitration on the `intentFiles` fixture —
a.txt's two deep stops scroll the match off screen, b.txt is the
cross-file leg — with `heldNthLoad(2)` holding only the reload:

- `TestReloadWithoutNavigationRecordsAnchorIntent` — an undisturbed
  reload records the anchor intent and no reveal; the held layout
  keeps the placeholder and saved viewport until the install restores
  the anchor's row.
- `TestReloadNavigationDuringLoadReplacesIntent` — `n` during the
  held reload leaves the completion recording no anchor intent; the
  commit reveals the newest stop at 42.
- `TestReloadAwayBackSameFileEntryReveal` — `n`/`p` during the reload
  ends on the initial stop, yet the entry reveal — not anchor
  preservation — commits: navigation intent, never cursor equality,
  decides.
- `TestReloadAwayBackCrossFileEntryReveal` — A→B→A during the held
  reload shows "Loading…" on the return (the re-entry minting nothing
  new) and the commit applies the entry reveal at 2.
- `TestReloadOutOfOrderRevisionsCommitReveal` — a revision-1 layout
  delivered before the revision-2 one is discarded without consuming
  the reveal intent navigation pended during the load; the revision-2
  install commits it.

`stale_test.go` (same package; Issue #29) drives the file-changed
integration — the `staleIndex`/`staleFile`/`staleStop` helpers extend
`navIndex` to several recorded submatches per stop:

- `TestStaleNoteInFilenameRow` — a same-length replacement on disk
  before the load puts "file changed since search" in the filename
  row's slot beside the still-named path, holding through a scroll and
  a constrained resize with nothing overflowing and nonnegative
  composed dimensions.
- `TestStaleNoteClearedByCleanReload` — reverting the file and `r`
  clears the slot; a still-mismatched reload restores it — the note
  recomputes per load.
- `TestStaleGatedReloadCommitRevealsSurvivor` — `n` during the held
  reload selects the stop whose first recorded submatch the new bytes
  drop; the matching-layout commit reveals the first survivor's cell,
  the render highlights the survivor and not the dropped bytes, and
  the note shows.
- `TestStaleGatedReloadCommitRevealsClampedFallback` — all submatches
  dropped with the line present commits the clamped recorded start,
  the stale row paints no invented highlight, and the note shows.
- `TestStaleMissingLineLandsOnLastLine` — a deleted trailing matched
  line lands at the last source line's start, revealed in view, with
  no invented highlight and the note showing.

Since Issue #25, `injectLoad` (in `layout_test.go`) fills a
fabricated completion with the live request's identity before
feeding `Update` — a message for a path with no request in flight
is discarded exactly like a stale one — and the
replay/sink-safety/reveal/scroll/popup fabrications mint requests
with `mintRequest`, the same way Issue #27's reload will mint them.

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
- `TestPrepareRows` — `Prepare(buf, Key)` builds the keyed row model;
  in run-off-edge mode each source line is one rendered row in file
  order with the buffer's gutter width; a nil buffer gives an empty
  model.

The file's `loadBufferStops` helper (Issue #21) loads content through
the real `filebuffer.Load` with recorded navigation stops, so tests
consume the cluster-expanded highlight cell spans production hands
the viewport rather than hand-built `Line` values.

`wrap_test.go` (external package; Issue #16) covers the wrap and
run-off-edge row models:

- `TestWrapRowCountsASCII` — a 13-cell line at text width 5 wraps into
  three rows partitioning its cells in order with `Continuation()`
  true after the first; a short line and an empty line each yield one
  row.
- `TestWrapMovesWideClusterToNextRow` — a two-cell cluster that does
  not fit in the row's remaining cells starts the next row, leaving
  blank cells at the previous row's end; a cluster is never split.
- `TestWrapKeepsClustersTogether` — a regional-indicator flag pair
  stays within one row's span rather than splitting across the
  boundary.
- `TestWrapTabIsOneCluster` — a tab expansion wraps as a single
  unbreakable cluster.
- `TestWrapRowsAlignToClusterBoundaries` — over a mixed wide/combining/
  tab line at many widths, every wrapped row's `[Start, End)` lands on
  the `Line.Clusters` boundaries the buffer exposed.
- `TestRunOffEdgeOneRowPerLine` — run-off-edge mode maps each source
  line to exactly one row spanning all its cells, at any text width
  including zero.
- `TestRowsCarryTheirKey` — the model reports the exact
  (path, revision, text width, wrap mode) key it was prepared for, so
  a layout change makes it stale.
- `TestWrappedTargetRow` — `TargetRow` resolves a match's start cell
  to the wrapped row containing it, and a past-end target lands on the
  line's last row.
- `TestRevealDeepInWrappedLine` — a stop deep inside a line taller
  than several screens reveals its own row at `floor(h / 3)`.

`reveal_test.go` (external package; Issue #14, extended by Issues
#17–#19) covers destination reveal, the row/target selectors, and the
minimal horizontal reveal over real prepared buffers:

- `TestStopTargetIsFirstSubmatchStartCell` — the display target's cell
  is the first submatch's *start cell* through the byte→cell map, not
  its byte offset (an ESC byte widens to two cells), and the earliest
  of several submatches supplies it.
- `TestStopTargetZeroWidthLandsPastLastCell` — a zero-width position at
  end of line targets the marker cell one past the last cell.
- `TestTargetRow` — the rendered row containing the target is the
  destination line's own row (lines shorter than the width stay one
  row each), clamped to the prepared rows (line 0, past EOF, empty
  model).
- `TestRevealVisibleTargetNoScroll` — a target row anywhere in
  `[top, top + height)` — edges included — leaves the top and reports
  no move.
- `TestRevealHiddenTargetOneThird` — a hidden target lands at row
  `floor(h/3)`: top = target − `h/3`, from below and from above.
- `TestRevealBOFEOFClamps` — near BOF the target lands higher than the
  third (top clamps to 0); near EOF it lands lower (top clamps to
  `MaxTop`), including a file barely taller than the viewport.
- `TestRevealStartsFromCurrentTop` — the reveal is relative to the
  viewport's current top: the same target moves a first-visit top of 0
  it is hidden from but stays put when visible from a saved top.
- `TestRevealOffRightOfView` — a target right of the window sets
  `off = start + cluster width − text width` (single-cell and
  two-cell targets alike), landing the whole target cluster at the
  right edge.
- `TestRevealOffLeftOfView` — a target left of the window sets
  `off = start`, landing it at the left edge.
- `TestRevealOffPaintedTargetKeepsOffset` — a target whose start cell
  is already painted leaves the offset alone, mid-window and at the
  edges.
- `TestRevealOffClippedBlankCountsHidden` — a two-cell target
  geometrically inside the window but clipped to a blank start cell
  counts as hidden and reveals by the right-edge rule.
- `TestRevealOffOversizedSpanByStartCell` — a match wider than the
  text width reveals by its start cell only; the off-screen tail does
  not force further movement.
- `TestRevealOffUnpaintableClusterFallback` — a target cluster wider
  than the whole text width sets `off` to its start and is treated as
  geometrically revealed: a second `RevealOff` is a no-op, so
  repeated navigation cannot loop.
- `TestRevealOffMarkerCell` — a zero-width match's marker cell (one
  past the last cell) reveals by the single-cell right-edge rule.
- `TestRevealOffNoOpWrapAndEmpty` — a wrap model and a nil-buffer
  model both leave the offset at 0.
- `TestCellVisible` — the painted-cell predicate itself: start cell
  painted, clipped blank (hidden), left/right of window, and marker
  cell at the boundary.

`anchor_test.go` (external package; Issue #17) covers the logical
anchor over real prepared buffers:

- `TestAnchorRewrapKeepsTextLocation` — a 95-cell line scrolled to row
  5 at text width 10 anchors `{Line: 1, Cell: 50}`; restoring at width
  7 lands the top on the row containing cell 50 (row 7), and returning
  to width 10 restores row 5 — the same text, not the same ordinal.
- `TestAnchorSurvivesWrapToggle` — wrap-off shows the anchor's line as
  one row while retaining cell 50; wrapping back restores the row
  holding that cell.
- `TestScrollReplacesAnchor` — a scroll that moves the effective top
  replaces the anchor with the new top row's location.
- `TestMovingRevealReplacesAnchor` — a reveal that moves the viewport
  replaces the anchor with the resulting top row's location.
- `TestNoScrollRevealRetainsLogicalColumn` — a reveal of an
  already-visible target keeps the retained mid-row column rather than
  snapping it to the top row's start.
- `TestEOFClampUpdatesAnchorLossy` — `Clamp` pulling the top up (from
  90 to 70) rewrites the anchor to the clamped row (line 91 → line
  71), and a later shrink does not restore the pre-clamp top.
- `TestRestoreEOFClampUpdatesAnchor` — `Restore` under a grown
  viewport that forces the clamp updates the anchor the same way.

`pan_test.go` (external package; Issue #18) covers the horizontal
offset and extent contracts over real prepared buffers:

- `TestHalfTextUnit` — `HalfText` is `max(1, floor(width / 2))` over
  odd and degenerate widths.
- `TestPanUnitsMoveColumns` — the 1-, 10-, and half-width units move
  the offset symmetrically right and left.
- `TestPanClampsToPaintableBoundary` — overshooting pans clamp to the
  widest visible line's last paintable start (99 for a 100-cell line),
  and further pans change nothing; panning past the left edge clamps
  to 0.
- `TestPanNoOpInWrapMode` — `Pan` under a wrap model never moves the
  offset and `MaxOff` reports 0.
- `TestPanOffsetRetainedThroughWrapToggle` — `Restore` into wrap keeps
  the offset, a wrap-mode `Scroll` leaves it, and re-entry keeps it
  when still within the visible extent.
- `TestWrapReentryClampsToNewVisibleSet` — scrolling deep into short
  lines while wrapped, then restoring to flat, re-clamps to their
  maximum — the retained offset is not restored.
- `TestMaxOffFollowsVisibleLines` — the 300-cell line among 10-cell
  lines gives 299 while visible and 9 once scrolled out; scrolling
  back does not restore the stored offset, and a fresh pan
  re-evaluates the live visible rows.
- `TestMaxOffEmptyViews` — a nil-buffer model and an all-empty view
  both report and clamp to 0.
- `TestMaxOffTrailingWideCluster` — a line ending in a two-cell
  cluster stops at the cluster's start, and at width 1 falls back to
  the last fitting cluster's start.
- `TestMaxOffUnpaintableFinalCluster` — a final cluster wider than the
  text width falls back to the last fitting start, or 0 when none
  fits; panning such a line stays at 0.
- `TestRevealReclampsOffset`, `TestClampReclampsOffset` — a moving
  reveal and a window-shrinking `Clamp` each re-clamp the stored
  offset against the rows still visible.
- `TestUniformLinesAllHiddenLeftIsLegal` — every visible 50-cell line
  permits offset 49 (its last paintable start), the geometry Issue
  #20's `_` indicator needs on every line.
- `TestResetOff` — `ResetOff` zeroes the offset for the file-change
  reset.
- `TestExtentEvaluationTouchesOnlyVisibleRows` — the `countingExtent`
  fake records exactly `[top, top + n)` queries for a pan, a scroll's
  re-clamp, and a `MaxOff` call — the render-cost guard.

`indicators_test.go` (external package; Issue #20) covers the
hidden-content mark predicates over real prepared lines:

- `TestLeftMark` — the gutter mark table: blank when nothing is
  hidden (including an empty row), `_` for text hidden left, `*` when
  a match or marker span is entirely hidden left, `_` when the match
  is only partially hidden or hidden right instead, a clipped `文`
  cluster's blanked cells counting hidden, and zero-width marker
  positions judged as one-cell targets.
- `TestRightMark` — the reserved-column table: `*` only when a match
  or marker span is entirely hidden right, never for a partially
  visible or hidden-left match, a clipped `文` at the right edge
  counting hidden, the last-cell match painted while a farther match
  stars, and zero-width marker positions.
- `TestLeftMarkOnEveryFlatRow` — the uniform-lines layout: every
  prepared row reports `_` at a nonzero offset.
- `TestMidClusterMatchCountsFromClusterBoundary` — Issue #21: the
  `markRowOf` helper loads recorded byte spans through the real
  FileBuffer path, so a byte match inside `文` arrives as the expanded
  cell span `[2,4)` and the marks judge it from the cluster
  boundaries — entirely hidden left when the cluster start is hidden
  left, entirely hidden right when the window ends inside it, no star
  when the whole cluster paints.

`marker_test.go` (external package; Issue #23) covers the zero-width
marker through the row model — recorded byte spans loaded through the
real FileBuffer path, so empty spans arrive as marker positions:

- `TestEOLMarkerWrapRows` — the end-of-line marker joins a wrapped
  row with a cell to spare (width 5 → `[{0,4}]`, width 2 →
  `[{0,2},{2,4}]`) and occupies its own continuation row `[{3,4}]`
  after an exactly full row (width 3).
- `TestEOLMarkerFlatRows` — run-off-edge rows span the effective
  width: `hit` plus its `$` marker is `[0,4)`; an empty matched
  line's marker is its single cell `[0,1)`.
- `TestMarkerExtentFeedsMaxOff` — the marker's start feeds the
  paintable-boundary maximum (`MaxOff` 3 for `hit` + `$`, the offset
  where the marker paints alone, `CellVisible` true there), and a
  marker-only line has extent 1 with `MaxOff` 0 — `Pan` cannot move
  the offset off it.
- `TestMarkerHiddenDrivesIndicators` — LF and CRLF (`hit\r\n` byte-4
  `$` → column 3) markers alike: entirely hidden left upgrades the
  gutter to `*`, painted marker beside hidden text leaves `_`,
  entirely hidden right earns the reserved `*`, painted earns
  neither.
- `TestMarkerIsRevealTarget` — `StopTarget` resolves the empty first
  submatch to the marker cell, `TargetRow`/`RowOf` find the marker's
  own wrapped row after a full text row (the line's single row in
  flat mode), and a marker-only line's cell 0 is equally a target.
- `TestRevealLandsOnMarkerRow` — a reveal aimed at the marker's own
  row below the window moves the viewport by the one-third placement
  clamped to EOF, landing the marker row visible at the bottom.

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
boundary tests. Since Issue #26, `writeHappyFiles` creates the two
files `happyStreamRG` reports so the quit-driving tests' browse loads
succeed — a failed current-file load now opens the modal error
overlay, which a single `q` would only dismiss:

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
  raw bytes never reach the terminal. Since Issue #26 the same
  failure opens the modal error overlay, so the test dismisses it
  with one `q` before the ordinary browse quit.
