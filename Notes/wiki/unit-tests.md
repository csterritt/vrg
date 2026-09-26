# Unit and boundary tests

Catalog of Go tests. Conventions: table-driven, `t.Helper()` in helpers,
externally observable behavior only.

## internal/cli

`cli_test.go` (external package `cli_test`):

- `TestHelpOnlyResults` — every help-only spelling (bare, first-token,
  later-token, combined `-ih`/`-hi`/`-xh`, `-help`, flag-preceded
  `-i --help`, flags-mixed `-i -s --help`/`-i -h -s`, help beating
  `-uuu`/`-e`/invalid-root/excess-operand conditions) yields `KindHelp`,
  one help copy, no search fields (including no `ChildArgv`), and never
  touches the failing root-validation sentinel.
- `TestGeneratedHelpContent` — syntax line, `PATTERN`, `ROOT` with
  `(default ".")`, `-h`/`--help`.
- `TestAssignmentSpellingsAreUsageErrors` — every `=` spelling for a
  declared option (`--help=`/`-h=` true and false,
  `--ignore-case=false`, `-i=false`, `--unrestricted=false`) is an
  unsupported-option usage error with no root validation and no child
  argv.
- `TestAssignmentSpellingsArePositionalAfterTerminator` — the same bytes
  are literal operands after `--`.
- `TestPositionalsAndRoot` — the full table: default `.`, dir/file/
  symlink roots, empty and `-` patterns, `--` arity, unsupported options,
  flags-only missing-pattern rows, invalid help assignments,
  nonexistent/stdin/special/device roots.
- `TestUsageErrorsPrecedeRootValidation` — non-root errors (including
  `-uuu` and `--help=false`) classified before validation via the
  failing sentinel.
- `TestDashFileRoot` — `./-` names a real file.
- `TestHelpLikeTokensAfterTerminator` — `-h`, `--help`, `--`, `-i`,
  `-u` as operands.
- `TestUsageDiagnosticSafety` — hostile bytes escaped in diagnostics.
- `TestDiagnosticsAreSpecific` — no bare `incorrect usage`.
- `TestAcceptedSearchFlags` — every allow-listed flag, short and long,
  forwards its supplied spelling into the exact child argv
  (`--json --no-config <flags> -- pattern root`).
- `TestFlagEncounterOrder` — repeated (`-i -s -i`), combined (`-isi`,
  `-iwF`), mixed-alias, and operand-interleaved (`foo -i dir -s`)
  options keep encounter order and supplied spellings; contradictory
  flags are not normalized.
- `TestUnrestrictedCumulativeLimit` — 0–2 `-u`/`--unrestricted`
  occurrences across any token mix accepted; the third is
  `ErrExcessUnrestricted` with no root validation or child argv.
- `TestRejectedOptions` — `-e`, argument-taking options, unknown
  spellings, and combined tokens with undeclared letters are
  `ErrUnsupportedOption`.
- `TestTerminatorAndProtectedPatterns` — `--` protects `-foo`, literal
  `-`, a second `--` as the pattern (default and explicit root), and the
  empty pattern as an empty argv element; unprotected `-foo` is
  rejected.
- `TestGeneratedHelpListsSearchFlags` — every allow-listed flag appears
  in generated help in short and long form.

`internal_test.go` (same package): `TestAppUsesContinueOnError` pins
`flag.ContinueOnError` on the adapter's app;
`TestSharedDeclarationsDriveHelpAndScan` iterates `optionDecls` itself to
prove generated help and the ordered scan share the one declaration
table (every forwarded spelling accepted verbatim).

## internal/searchindex

`searchindex_test.go` (external package `searchindex_test`) builds
records with `jText`/`jBytes` helpers so fixtures mix the `{"text"}`
and base64 `{"bytes"}` encodings freely:

- `TestDecodeRecordKinds` — all five known events decode under either
  encoding, including non-UTF-8 `bytes` paths, `binary_offset` null vs
  integer, and fully ignored `context` payloads.
- `TestDecodeMatchRecord` — path/lines/submatches retained in text,
  bytes, and mixed encodings.
- `TestDecodeIgnoresUnrequiredFields` — `absolute_offset`, `stats`, and
  extra members ride along unread.
- `TestMalformedRecords` — the schema-matrix violations: invalid JSON,
  missing/non-string `type`, missing or mistyped required fields, bad
  base64, `line_number < 1`, non-integer/out-of-range numerics, empty
  `submatches`, `start > end`, `end > len(lines)`, missing
  `binary_offset`.
- `TestUnknownRecordType` — unknown string types decode as
  `KindUnknown`, not malformed.
- `TestHappyPathStream` — begin/match/end/summary plus ignored context
  flow through the index; only matches build stops.
- `TestSameLineMerging` — same path+line records merge into one stop
  with submatches sorted by start then end and recorded bytes retained.
- `TestMixedEncodingSameValueMerge` — a `text` record and a `bytes`
  record carrying identical bytes merge as the same value.
- `TestOverlappingSubmatchUnionCoverage` — overlapping ranges are all
  retained; `Highlights` is the union (overlapping → merged span,
  disjoint → both, adjacent → contiguous span).
- `TestIndexOrdering` — unsigned raw path bytes then ascending line;
  non-UTF-8 bytes order deterministically against UTF-8.
- `TestStopRetainsRawData` — raw path bytes, line number, submatch
  ranges, and recorded submatch bytes all retained.
- `TestRelativePathResolution` — `ResolvedPath` joins the invocation
  working directory with no canonicalization (`./`, `a/../b` survive);
  absolute paths pass through.
- `TestBinaryEndDropsEarlierMatches` (Issue #8) — an `end` with a
  non-null `binary_offset` removes every stop collected from that
  file's earlier `match` records; the file is absent from the index and
  `BinaryExcluded()` counts it once.
- `TestBinaryExclusionCountsDistinctFiles` (Issue #8) — the excluded
  tally counts distinct raw paths: a repeated excluding `end` does not
  recount, a match after exclusion stays dropped, an excluded file with
  no matches still counts, and a null `binary_offset` excludes nothing.
- `TestUsableResultsIsRetainedStops` (Issue #8) — usable results
  (`LineCount`) is the retained-stop count after filtering, not the
  received `match`-event count: four match events for a binary-excluded
  file plus one retained stop yield 1.

`lifecycle_test.go` (external package) is the Issue #9 lifecycle
coverage:

- `TestLifecycleMatrix` — the single transition-matrix table driving
  record lists through `Add` or raw streams through `Feed`, asserting
  `IntegrityFailures` substrings in order, retained-stop count,
  `Incomplete` count, and the `BinaryExcluded` tally. Rows cover every
  transition: `begin` open/closed, `match` open/orphaned (never opened,
  after `end`), `end` open/orphaned/duplicate — including an orphaned
  binary `end` still excluding — `context` inert in any position, a
  file open at stream end, summary positioning (alone = complete
  zero-result stream, missing, second, records after it — including
  malformed bytes), the trailing unterminated record's double
  disposition mid-stream and after a complete stream, `text`/`bytes`
  path-identity agreement (including non-UTF-8), interleaved open files
  pairing independently, and binary exclusion's precedence over orphan
  retention.
- `TestFeedDecodesTheWholeStream` — a complete newline-terminated
  stream is intact and indexes one stop.
- `TestEmptyStreamIntegrity` — an empty stream reports the missing
  summary.

`disposition_test.go` (external package) is the Issue #10
deterministic-disposition coverage:

- `TestMalformedDispositions` — every Issue #3 schema-violation row
  (invalid JSON, bad base64, missing/non-string `type`, missing or
  mistyped required fields, `line_number` violations, bad submatch
  ranges, negative `binary_offset`, empty `submatches`) embedded
  mid-lifecycle in an otherwise intact stream: counted `Malformed()`,
  no integrity failure, and the following records resynchronize and
  index.
- `TestIntegrityDispositions` — the lifecycle-violation rows driven
  through `Feed` asserting the failures are reported while
  `Malformed()` stays zero (a skipped record never becomes a lifecycle
  failure on its own).
- `TestCompositeDispositions` — the two both-disposition rows: the
  trailing unterminated record (malformed + `unterminated trailing
  record`) and malformed bytes after `summary` (malformed +
  `record after summary`).

`oversized_test.go` (external package) covers the Issue #10 resource
limit and unknown types:

- `TestOversizedBoundary` — the 64 MiB boundary: a payload at the
  limit indexes normally; one byte over is oversized.
- `TestOversizedResynchronizes` — the record after an oversized one
  indexes normally (discard-through-newline).
- `TestOversizedDiagnosticNamesRecoveredPath` /
  `TestOversizedDiagnosticAnonymousWhenPathLost` — the `oversized
  record skipped for <path>` line when `type`/`data.path` decoded
  before the limit, and the bare tally when the cut fell first.
- `TestOversizedOnlyFileAbsentFromList` — a file whose match records
  were all oversized leaves no stops while its path still names the
  diagnostic.
- `TestOversizedUnterminatedFinalRecord` — the triple disposition:
  oversized + malformed + `unterminated trailing record`.
- `TestUnknownTypeDispositions` — the separate `Unknown()` tally, the
  `N unrecognised record types skipped` diagnostic, no lifecycle
  effect, no substitution for `summary`, and unknown-after-`summary`
  counted plus independently flagged.

`cursor_test.go` (external package) is the Issue #13 matched-line
cursor coverage; `cursorIndex` builds a two-file fixture whose records
arrive out of order so the index order (a.txt:1, a.txt:3, a.txt:5,
b.txt:2, b.txt:4) proves path-then-line ordering rather than arrival:

- `TestCursorStartsAtFirstStop` — `Current` selects the first stop in
  index order before any navigation.
- `TestCursorNextAdvancesAndWraps` / `TestCursorPrevRetreatsAndWraps` —
  the full step tables in both directions: each `Step`'s destination
  stop plus `Moved`, `FileChanged` (only on a real file crossing), and
  `Wrapped` (only at an index end).
- `TestCursorOneStopIsStrictNoOp` / `TestCursorEmptyIndexIsNoOp` — a
  single stop and an empty index both leave `Moved`/`FileChanged`/
  `Wrapped` false and the selection unchanged.
- `TestCursorSubmatchesShareOneStop` — a line carrying three submatches
  is one stop; the wrap back stays within the same file (`FileChanged`
  false).

## internal/present

`present_test.go` (same package) covers the path and diagnostic
contracts of the shared utility:

- `TestPath` — the full path-rule table: `\n`/`\r`/`\t` to their
  two-character forms, `\\` doubling, `\xNN` for invalid UTF-8, caret
  notation for C0 and `^?` for DEL, `\uXXXX` for C1, printable Unicode
  preserved.
- `TestPathNeverEmitsControls` — every C0 byte and DEL produces no raw
  control byte in the output.
- `TestDiagnostic` — diagnostic rules: LF preserved as a real line
  boundary, CRLF as one boundary, standalone CR → `^M`, tabs expanded
  to 8-column stops (column reset per line, escaped forms counted),
  caret notation, `\uXXXX` C1, `\xNN` invalid UTF-8, backslash and
  printable text passed through.
- `TestDiagnosticNeverEmitsControls` — every C0 byte and DEL produces
  no raw control byte except the message's own `\n`.
- `TestDiagnosticEmbedsSingleLinedFilename` — a `Path`-escaped filename
  stays single-lined inside a diagnostic while the message's own
  newlines still break lines.

`line_test.go` (same package) covers content presentation:

- `TestLineText` — content rules: invalid UTF-8 → U+FFFD, C0/DEL
  caret notation, `\u0085`-style C1 forms, LF/CRLF never displayed,
  standalone CR → `^M`, and a standalone combining mark's `◌́`
  fallback cell (Issue #21).
- `TestLineWidth` — cell counts for escape forms and wide clusters.
- `TestLineSpan` — byte→cell maps for escaped forms, including
  an ESC byte's match covering both `^[` cells and marker positions on
  removed terminator bytes.
- `TestLineTabStops` — Issue #16's structural expansion: no raw tab
  survives, expansion cells land on the next multiple of eight
  source-display columns (the positions Issue #5 deferred), and the
  tab byte's span covers the whole expansion.
- `TestLineRetainsRaw` — raw line bytes survive presentation.

## internal/filebuffer

`filebuffer_test.go` (same package) covers
`Load` against real fixture files:

- `TestLoadLineCount` — source-line counts including empty and
  no-trailing-newline files.
- `TestGutterWidth` — digit width of the largest line number plus two
  spaces, minimum one slot.
- `TestLinePresentation` — escaped text per line through `Buffer`.
- `TestHighlightSpans` — validated submatches become display-cell
  spans on the escaped line.
- `TestSubmatchValidation` — out-of-bounds and byte-mismatched
  submatches are dropped, not clamped into wrong highlights.
- `TestLoadFailure` — unreadable files return the error.

`cluster_test.go` (same package, Issue #16) pins the buffer as the
shared grapheme-policy source:

- `TestCellClusterBoundaries` — `Cells` carries `Lead` on every
  cluster's first cell (the only legal wrap boundary) and `Cont` on
  trailing cells, across ASCII, wide, combining, and caret-escape
  units.
- `TestLeadingCombiningCluster` — a standalone combining mark at line
  start takes a provisional cell of its own on a `◌` dotted-circle
  base (Issue #21's visible fallback), still a boundary.
- `TestTabStopCells` — the tab expands to the next eight-column stop
  as one cluster (first expansion cell leads), and the recorded
  submatch on the tab byte highlights the whole expansion.

`expand_test.go` (same package, Issue #21) pins cluster-expanded
highlight spans:

- `TestSpansExpandToWholeClusters` — the `Load`-level table: a
  combining-only match covers the whole base cluster; a mark's
  zero-width cluster borrowing an escape's trailing cell, a tab
  expansion cell, or a wide glyph's trailing cell expands over the
  whole host cluster; a mark joined to a replaced invalid byte's own
  cell stays one cell; interior bytes of a wide pair or an emoji ZWJ
  sequence cover both cells; and a standalone combining mark at line
  start keeps its provisional fallback cell — never zero cells.
- `TestClusterSpanBoundaries` — the cell-space expansion itself over
  `Lead` marks: start inside, end inside, and strictly interior ranges
  all expand to the whole cluster while whole-cluster, neighbouring,
  line-end, and marker spans pass through.

`lines_test.go` (same package, Issue #22) pins the structural line
contracts and the raw-file/rg-line coordinate split:

- `TestTerminatorsUndisplayed` — LF, CRLF, and mixed terminators
  produce no display text, with unterminated-final-line and blank-line
  cases.
- `TestStandaloneCREscapes` — a CR not followed by LF is `^M` content,
  not a terminator.
- `TestEmptyFile` — zero source lines behind the minimum one-digit
  slot: a three-cell gutter.
- `TestTerminatorBytesMapToDisplayEOL` — terminator bytes and
  zero-width positions (byte 4 of `hit\r\n`, the terminator-only
  match) map to the display end-of-line position.
- `TestSpanAcrossTerminatorHighlightsTextOnly` — a span covering text
  plus terminator maps to the visible text alone.
- `TestLeadingUTF8BOM` — the BOM paints nothing; rg offset 0 validates
  against raw byte 3 and highlights cells 0–2; the shifted end-of-line
  position is a marker; line 2 is unshifted.
- `TestBOMShiftMapsRGOffsetsToRawBytes` — a second, content U+FEFF
  after the file BOM proves the three-byte shift: rg offset 0 names
  raw byte 3, highlighting the fallback cell, not the hidden BOM.
- `TestBOMOnlyFile` — a BOM-only file is one zero-display line.
- `TestNonLeadingFEFFIsContent` — U+FEFF mid-line and at a later
  line's start is ordinary content: it joins the previous cell or
  takes the `◌` fallback cell, matches, and highlights normally.

`marker_test.go` (same package, Issue #23) pins the zero-width marker
positions through `Load`:

- `TestZeroWidthMarkerPositions` — empty submatches validate like any
  other and map to marker spans: BOL on text and empty lines,
  mid-line, at EOL, positions inside a wide pair and a ZWJ cluster
  landing on the cluster's start cell, and positions on or covering
  LF/CRLF terminator bytes landing on the display end-of-line column.
- `TestMarkersOnEveryLine` — `^`-style per-line markers accumulate on
  every line of a multi-line file, empty lines included.

`stale_test.go` (same package, Issue #29) pins stale-match validation
and the fallback reveal targets through `Prepare` — the reload path's
revalidation is a fresh `Prepare` over newly read bytes, so the
helpers drive it in memory:

- `TestFullyValidatingBufferIsNotStale` — every recorded submatch
  validating leaves the buffer clean.
- `TestDroppedSubmatchesMarkStale` — each failure kind marks the
  buffer: same-length replacement, out-of-bounds start and end,
  missing line, and one-of-two partial survival.
- `TestStalePartialSurvivalKeepsValidHighlights` — the surviving
  submatch keeps its span and becomes the reveal target.
- `TestStaleFallbackClampsRecordedStart` — the recorded start clamps
  to the line's bytes and maps to a cell; terminator and past-EOL
  starts land on the last rendered cell (no invented marker), an
  empty line on cell 0.
- `TestStaleFallbackUsesEarliestRecordedStart` — the earliest
  recorded start decides, not the submatch's slice position.
- `TestStaleFallbackMissingLineAndEmptyFile` — a gone line lands at
  the last source line's start; an empty file stays a zero-line panel
  with the inert zero target.
- `TestRevalidationRecomputesStale` — stale → clean → stale across
  successive prepares.
- `TestValidationUsesOriginalNotDisplayBytes` — matches on a raw ESC
  byte and an invalid UTF-8 byte validate against the raw bytes, not
  the `^[`/U+FFFD display text.
- `TestCRLFTerminatorMatchValidatesClean` — a recorded submatch
  covering `\r\n` validates against the retained terminator bytes and
  becomes the ordinary end-of-line marker.
- `TestStaleFallbackShiftsPastLeadingBOM` — line one's rg offsets
  shift by three into the raw view for validation and for the
  fallback start alike.

`encoding_test.go` (same package, Issue #30) pins the UTF-16/UTF-32
BOM classification and its exclusion from display and validation:

- `TestUnsupportedBOMsClassify` — all four marks plus the bare UTF-16
  mark name their encoding through `Unsupported()` while the buffer
  carries no lines, no spans, no stale verdict, and the inert `(0,0)`
  reveal target.
- `TestUTF32LEWinsOverlapWithUTF16LE` — `FF FE 00 00` classifies as
  UTF-32 LE, the longest-first ordering beating the UTF-16 LE mark it
  overlaps.
- `TestLoadReportsUnsupported` — the `ReadFile`-backed `Load` path
  detects the mark the same way.
- `TestUTF8BOMIsNotMisclassified` — the leading UTF-8 BOM stays
  supported content: one line, the shifted validation, no encoding
  name.
- `TestNonBOMPrefixesStaySupported` — a lone `FF`, a non-leading
  `FF FE`, a truncated `00 00 FE`, and plain text classify nothing.
- `TestUnsupportedBytesSkipStaleValidation` — recorded submatches
  that could never equal the encoded bytes (mismatched and
  out-of-range) mark no staleness and produce no spans.

## internal/app

`model_test.go` (same package) drives `Update` directly:

- `TestSearchingScreenShownWhileCollecting` — the view is `Searching…`
  while the completion channel is silent.
- `TestResizeDuringSearching` — `WindowSizeMsg` stores dimensions and
  keeps the searching state.
- `TestQOnBrowseExitsZeroCleanup` — `q` on the browse view returns
  `tea.Quit` and `ExitCode` 0.
- `TestGateHoldsSearchingAfterRgExit` — `Config.Drained` fires once the
  child has exited and both pipes are drained, `PrepareGate` still holds
  preparation, the view stays `Searching…`, and releasing the gate
  delivers the browse view.
- `TestQWhileSearchingCancels` — `q` during searching returns `tea.Quit`
  with `ExitCode` 130 and fires the session cancel.
- `TestQDuringGateHeldPreparationCancels` — `q` in the post-exit
  gate-held window cancels: the collector abandons the gate, never
  prepares an index, and the model is already committed to quitting.
- `TestCtrlCCancelsFromAnyState` — `ctrl+c` cancels from both searching
  and browse, always `ExitCode` 130.
- `TestEscDuringSearchingIsNoOp` — `Esc` during searching leaves the
  model unchanged and does not cancel.
- `TestLateCompletionAfterCancellationDoesNotRevive` — a `searchDoneMsg`
  delivered after cancellation cannot revive the UI; the model stays
  quitting at 130.

`browse_test.go` (same package) covers the Issue #5 browse
composition:

- `TestCompletionTransitionsToBrowse` — `searchDoneMsg` renders the
  two-pane view: file list, filename rule, `Loading…` placeholder.
- `TestLoadCompletionRendersContent` — the `loadDoneMsg` carrying a
  prepared buffer swaps the placeholder for guttered content; the
  completion carries the decoded/mapped buffer so `Update` does no
  full-file work.
- `TestMatchRendersInverse` — a matched span emits the dark scheme's
  true-inverse pair, underlined on the current matched line
  (Issue #7's explicit-pair codes replaced the Issue #5 bare
  `\x1b[7m` assertion).
- `TestGatedLoadKeepsResponsive` — with `loadGate` holding the worker's
  read and decode/map phases, keys and resizes are still processed and
  the placeholder stays until the gate releases.
- `TestCtrlCWhileLoadGateHeld` — `ctrl+c` mid-load goes through the
  Issue #4 path to 130.
- `TestQOnBrowseExitsZero`, `TestEscOnBrowseIsNoOp` — browse-phase key
  behavior.
- `TestLoadFailureShowsUnreadable` — a failed load renders
  `(unreadable)` instead of hanging on the placeholder.
- `TestBrowseRendering` — composed `View()`: raw-path file-list order,
  current-entry underline, `── path ───` filename rule, right-justified
  `Gutter`-styled numbers with two spaces, no other panel borders.
- `TestColourToggleFlipsViewStyling` (Issue #7) — `c` returns no
  command and flips the frame's first SGR sequence between the base
  pairs: `37;40` → `30;47` → `37;40`.
- `TestCurrentLineMatchUnderlined` (Issue #7) — the current matched
  line's span emits `CurrentMatch` (inverse pair + underline) while a
  match on another matched line emits plain `Match`.

`scroll_test.go` (same package) covers the Issue #12 scrolling
contracts; `codePress` synthesizes the special-key messages and
`loadedModel` lands a completed a.txt load:

- `TestDownUpMoveOneRenderedRow` — `down`/`up` move the top row by one
  rendered row, the frame shows the shifted rows, and the matched-line
  cursor (`currentStop`) stays put (manual scroll never moves it).
- `TestHalfPageScrollUsesContentHeight` — `d`/`u` move
  `max(1, floor(h/2))` of the content height (panel height minus the
  filename row): 24→11, 22→10, and the one-row-content floor 2→1.
- `TestPageScrollUsesContentHeight` — `pgdown`/`pgup` move the full
  content height.
- `TestScrollClampsAtEOFAndBOF` — `up` at BOF leaves the view
  byte-identical; `pgdown`+`down` clamp at the last full page with the
  file's final row rendered at the bottom.
- `TestScrollOnShortFileIsNoOp` — a file shorter than the viewport
  ignores every scroll key.
- `TestScrollOnPlaceholderIsNoOp` — all six scroll keys on a
  `Loading…` panel return no command, move nothing, and write no saved
  state.
- `TestPerFileSavedViewportState` — scrolling writes
  `saved[raw path]` (a `viewport.Target` anchor since Issue #17); an
  `n` keypress crosses to the other file with its saved anchor seeded
  first, and the load-plus-layout completion restores it while the
  departed file's saved state is untouched — the seeded position keeps
  the destination match on screen, so Issue #14's no-scroll reveal
  leaves it alone.
- `TestRenderQueriesOnlyVisibleRows` — the render-cost guard: a
  `countingRows` fake installed through a `layoutDoneMsg` (Issue #17)
  records exactly the visible `Row` indices per `View()`, before and
  after a scroll — never O(N) over the buffer.
- `TestBufferPreparesAsRowSource` — a real `*filebuffer.Buffer`
  satisfies `viewport.Source`: `Prepare` maps rendered row i to
  source line i in run-off-edge mode with the buffer's cells (the
  Issue #12 `bufferRows` adapter is gone since Issue #16).

`wrap_test.go` (same package, Issue #16) drives the wrap toggle
through `Update`:

- `TestWrapOnByDefaultBlankContinuationGutter` — a freshly loaded
  long line wraps at the text width (panel minus gutter, no reserved
  column) with continuation rows behind a blank gutter aligned to the
  first row's text.
- `TestWTogglesRunOffEdge` — `w` switches to run-off-edge: the
  reserved indicator column widens by one (text width shrinks), the
  long line renders as one clipped row, and a second `w` restores the
  wrapped rows.
- `TestNRevealInsideWrappedLine` — a match near the end of a
  screen-tall wrapped line is revealed on its own rendered row at
  `floor(h/3)` — at startup, after `n` away, and after `p` back.

`anchor_test.go` (same package, Issue #17) covers the model-level
anchor contract:

- `TestResizePreservesCursorSelection` — a resize keeps the cursor's
  selected stop: the underline stays on the same matched line.
- `TestResizeKeepsAnchorTextAtTop` — scrolling partway into a wrapped
  long line then narrowing and widening keeps the same text at the
  panel's top; the ordinal of the top row changes, the logical anchor
  does not.

`layout_test.go` (same package, Issue #17) covers off-UI preparation
and obsolete-layout isolation. Helpers: `longLineModel` and
`twoFileLayoutModel` (settled browse fixtures), `wantKey` (the model's
current `(path, rev, width, wrap)` formula), `mustLayout` (assert an
update issued a layout request and return it), and `pump`/`settle`
from `model_test.go` (drive a command's message chain through
`Update`). Holding the worker is simply retaining the returned command
uninvoked; `pump` on it delivers the `layoutDoneMsg` completion:

- `TestResizeRequestsLayoutOffUpdatePath` — a resize returns a layout
  command immediately without re-laying out on the update path; the
  keyed completion installs the new row model.
- `TestLayoutWorkerHeldKeepsEveryInputActionable` — with the layout
  worker held after a resize: `ctrl+c` exits 130, `q` the fixed
  status, `n`/`p` move the cursor immediately with the newest stop's
  reveal intent preserved, `w` flips wrap and issues the new-mode
  request without releasing the held one, and a second resize is
  accepted — all while the gate stays held; releasing it then reveals
  the newest stop per Issue #14.
- `TestCtrlCWhileLayoutPendingExits130` /
  `TestQWhileLayoutPendingQuitsFixedStatus` — the exit contracts while
  a worker is held.
- `TestOutOfOrderLayoutCompletionsInstallNewestOnly` — W1→W2→W3
  resizes complete out of order; only the W3-keyed layout installs
  and the anchor is unaffected by the discards.
- `TestRapidWrapToggleDiscardsStaleMode` — rapid `w` toggles
  supersede each other; stale-mode completions never install.
- `TestLayoutForDepartedFileLeavesPanelAndSavedState` — a matching
  layout for a file that is no longer current caches for the revisit
  without touching the visible panel, the anchor, or saved state.
- `TestObsoleteLayoutDoesNotConsumePendingReveal` — a stale
  completion neither installs nor consumes the pending reveal; the
  intent still commits when the matching layout lands.
- `TestStaleLayoutNavigationCarriesSavedAndRevealIntent` — navigating
  to a cached file whose installed layout is stale-keyed requests a
  prepared layout for the current parameters and carries the
  saved-viewport-plus-reveal intent to commit on installation.
- `TestMatchingLayoutNavigationCommitsImmediately` — the fast path:
  a cached file whose installed layout already matches commits with
  no request issued.
- `TestRenderQueriesOnlyVisibleListEntries` — the file-list
  render-cost guard: a counting `listEntry` fake sees only the
  scrolled window's entries, never the whole list.

`filelist_test.go` (same package, Issue #24) pins the file-list layout
contracts — the width formula, the visibility toggle, grapheme-safe
truncation, the filename-row status slot, the scrolled window, and
anchor preservation through every relayout cause:

- `TestListWidthFormula` — a `Model`-seam table over
  `listWidth(gutterW, res)`: each of the three terms winning (longest
  sanitized path plus two, the `floor(0.40 × width)` cap including its
  odd-width rounding, and `width − (gutter + 10 + reserved)`), gutter
  growth narrowing the allocation, the ten-cell panel minimum, and
  zero/pathological widths clamped nonnegative.
- `TestTruncateLeftGraphemeSafe` — the leading-`…` cut fits the budget
  exactly and lands only on cluster boundaries: wide characters
  straddling the cut drop whole, combining clusters and ZWJ sequences
  are never split, and zero/negative budgets yield empty strings.
- `TestFilenameRuleStatusSlot` — the note slot wins cells over the
  path (which left-truncates to nothing so the note paints whole), a
  too-wide note is dropped rather than clipped, and degenerate widths
  fall back to dashes.
- `TestListToggleKeys` / `TestListToggleKeysInertWhileSearching` —
  shown initially; `tab`/`left` hide and `shift+tab`/`right` show in
  browse, each press returning the keyed relayout request for the new
  text width and a repeated press issuing nothing; all four keys are
  inert while searching.
- `TestListHideShowPreservesAnchorText` — with the top mid-way through
  a wrapped line, `tab` then `shift+tab` leaves the same anchor and
  the same text at the top of the panel.
- `TestGutterGrowthRelayoutKeepsAnchorText` — a reload with a wider
  gutter narrows `textW` through a keyed relayout and the anchor's
  text stays at the top, in both directions of the round trip.
- `TestListReducedLeavesTenTextCells` — a real five-digit-gutter file
  at 30 columns in run-off-edge mode leaves the list at twelve cells
  and the panel at ten text cells plus the reserved indicator column.
- `TestZeroWidthAllocationKeepsPreference` — the `W=20`, gutter 9,
  reserved 1 example allocates zero list cells, draws no entries, and
  leaves `listShow` untouched so the list returns when width allows.
- `TestListScrollsToKeepActiveVisible` — navigating past the window's
  end shifts `listTop` minimally; retreating leaves later entries
  visible until the entry crosses the top edge.
- `TestListRenderQueriesOnlyVisibleWindow` — deep in a 30-file list,
  the counting `listEntry` fake is queried for exactly
  `files[listTop:listTop+height]`.
- `TestStatusSlotRendersInFilenameRow` — a synthetic `statusNote`
  joins the filename rule at 80 columns, and at 30 columns the path
  truncates so the note still paints whole.

`load_test.go` (same package, Issue #25) pins the asynchronous
load-isolation contracts. Helpers: `gatedModel` (a browse model wired
with `loadGate` holding the worker's start and `mapGate` holding the
decode/map phase after the read, plus a suppressed pop-up timer,
returning the startup file's load command uninvoked) and `mintLoad`
(registers an in-flight request identity so a test can inject a
completion whose real worker never ran — existing injectors in
`browse_test.go`, `popup_test.go`, `replay_test.go`,
`filelist_test.go`, and `sinksafety_test.go` mint or read the
in-flight request identity the same way):

- `TestNavigationActiveWhileLoadInFlight` — with a.txt's worker
  genuinely started and parked on `loadGate`, `n` crosses to b.txt at
  once (placeholder panel, its own load issues) and `p` returns to the
  still-loading a.txt whose in-flight request is unchanged; releasing
  the gate lets the original request render its content.
- `TestPlaceholderScrollAndPanAreNoOp` — all six scroll keys and all
  six pan keys on a `Loading…` panel return no command, leave the
  frame byte-identical, and write no saved state, while `w`, `c`, and
  `tab` keep their ordinary meanings mid-load.
- `TestReEnterLoadingPathIsDroppedNotQueued` — away-and-back during a
  held load returns no command (no second worker, nothing queued), the
  in-flight request identity is unchanged, and its completion still
  installs.
- `TestLateCompletionIsolatedToItsPath` — A→B→C with every worker
  held: a.txt's and b.txt's completions while c.txt is current fill
  only their own cache entries — c.txt's frame, viewport, and saved
  state byte-identical — and revisits show the cached content with no
  new load (the crossing issues only a layout request) and no
  eviction.
- `TestLoadCompletionKeyedByRequestIdentity` — a completion whose
  identity is not the in-flight request's, and one for a path with no
  request at all, are dropped without touching the cache, the status
  maps, the diagnostics, or the panel; the in-flight request's own
  completion installs.
- `TestLoadCompletionAfterQuitDiscarded` — a worker completion after
  `ctrl+c` returns no command, keeps exit 130, and fills no cache.
- `TestDecodeMapGateKeepsInputsActionable` — with the read finished
  and the worker parked on `mapGate`, resize, `w`, `c`, `n`, and `p`
  all apply without waiting; releasing the gate delivers the prepared
  buffer to the then-current file (in the light scheme `c` switched
  to).
- `TestMapGateHoldsDecodeMapNotRead` — a read-phase failure (missing
  file) completes promptly even while `mapGate` is held, proving the
  gate sits after the read.

`failures_test.go` (same package, Issue #26) pins the read-failure and
re-entry contracts through the injected `readFile` seam — no
filesystem permissions involved. Helpers: `failLoader`/`failLoaderFor`
(all-fail and per-path failing loaders), `loaderModel` (a browse model
wired with the injected loader, returning the startup load uninvoked),
`gatedLoaderModel` (the loader plus `loadGate` so a retry can be
parked in flight), `contentRow1` (the stripped first content row the
placeholders occupy), and `overlayOccurrences` (diagnostic-occurrence
count across the open overlay's lines):

- `TestCurrentFileReadFailureNotifies` — the current file's failure
  opens the overlay, the panel reads `(unreadable)`, the filename row
  still names the path, and the retained stops stay navigable — a
  same-file `n` moves the cursor and requests no reload.
- `TestNonCurrentReadFailureIsDiagnosticOnly` — a completion landing
  after the cursor left: no overlay, no indicator, byte-identical
  frame, exactly one collected occurrence — and visiting the file
  later surfaces its overlay.
- `TestCrossFileReEntryRetriesOnce` — entering a failed file from a
  different file opens the retained prior-failure overlay immediately,
  returns the panel to `Loading…`, and issues exactly one retry.
- `TestUnreadableComposedViewStaysWellFormed` — a long escaped path
  through 80×24 down to 20×3: the filename row keeps the truncated
  safe path (full tail at 80 cells), the placeholder clips to its
  slot, no frame row overflows, layout stays nonnegative.
- `TestReEntrySequenceGated` — the gated sequence: immediate prior
  overlay plus `Loading…`, exactly one parked retry, `Esc` dismissing
  the overlay without disturbing the in-flight load, and settlement
  updating the panel without waiting for dismissal.
- `TestReEntrySecondFailureAppendsPreservingScroll` — a second failure
  appends exactly one occurrence to the open overlay with the reader's
  scroll position preserved, and collects exactly one new occurrence
  for the replay.
- `TestReEntryRetrySuccessKeepsPriorOverlay` — a successful retry
  collects nothing new; the content replaces `Loading…` while the
  prior-failure overlay stays up until dismissed.
- `TestReEntryRetrySettlesAfterNavigatingAway` — navigating away
  mid-retry lets it settle as a non-current completion (a second
  failure there is diagnostic-only), and a later re-entry repeats the
  sequence against the new prior state.
- `TestReEntryDuringInflightRetryIsDropped` — re-entry while a retry
  is already in flight mints nothing: dropped, not queued, per the
  Issue #25 one-load-per-path rule.

`reload_test.go` (same package, Issue #27) pins the explicit-`r`
contracts through the same gated seams: `loaderModel` and
`gatedLoaderModel` from `failures_test.go` drive the loads, and the
layout worker is held by retaining the command `pump` would run:

- `TestExplicitReloadRereadsCurrentFile` — `r` issues exactly one
  reread, shows `Loading…` with the filename row intact, leaves the
  cursor and stops untouched, and installs the new bytes.
- `TestReloadDuplicateDroppedNotQueued` — a second `r` while the load
  is held mints nothing, as does a re-entry in flight; only after the
  placeholder settles does the next `r` start a new load.
- `TestReloadPreservesAnchorThroughMatchingLayout` — the anchor and
  top are asserted only after the new revision's prepared layout
  installs, never against the superseded one.
- `TestReloadAnchorClampsToShrunkContent` — an anchor past the shrunken
  EOF is rewritten by the lossy clamp.
- `TestFailedReloadReplacesContent` — a failed reread shows
  `(unreadable)` plus the overlay; no stale text survives.
- `TestReloadSecondFailureAppendsPreservingScroll` — a second
  consecutive failure appends exactly one occurrence to the reopened
  overlay with the reader's position preserved.
- `TestReloadIsTheOneStopRetryRoute` — `n`/`p` are no-ops on a
  one-stop index; `r` retries to success with the prior-failure
  overlay kept up.
- `TestDiskChangeWithoutRIsNotObserved` — disk edits produce no load
  and no frame change until `r`.
- `TestPreReloadLayoutDiscardedAfterReload` — a held pre-reload layout
  released after the reload completes fails the revision-keyed install
  guard without touching the panel or the anchor.
- `TestNavigationDuringReloadOverridesAnchor` — a selection made
  during the in-flight load commits its reveal over the anchor intent.
- `TestReloadAnchorIntentSurvivesTheLayoutGap` (Issue #28) — the
  reload's completion records `intentAnchor`, the gap paints no rows,
  and the matching install restores the anchor with no reveal.
- `TestReloadSameFileAwayAndBackCommitsEntryReveal` (Issue #28) —
  `n` then `p` inside the reload's flight ends on the starting cursor,
  yet the commit runs the entry reveal: navigation intent, not cursor
  equality, decides.
- `TestReloadCrossFileAwayAndBackCommitsEntryReveal` (Issue #28) —
  A→B→A during the reload: the return shows `Loading…` with the saved
  anchor dormant, and the commit reveals the re-selected stop.
- `TestReloadLateOldRevisionLayoutIsInert` and
  `TestReloadLateOldRevisionLayoutAfterNavigation` (Issue #28) —
  old- and new-revision layouts completing out of order: the new
  revision's install commits the anchor or the reveal, and the late
  old-revision layout rewinds nothing.

`completion_test.go` (same package, Issue #28) pins the two-stage
load-completion contract, driving each stage separately — the load
command's message through `Update`, the layout command held and
invoked on the test's schedule:

- `TestLoadCompletionMakesNoRowDecision` — stage 1 installs the
  buffer, bumps the revision, grows the gutter, and recomputes the
  text width without moving the top, anchor, or offset or consuming
  the reveal intent; the layout request is keyed to the grown
  gutter's width; the matching install then commits the reveal.
- `TestNavigationDuringLayoutGapCommitsNewestTarget` — `n`/`p` while
  the layout is held move the cursor immediately with the top fixed;
  the install reveals the newest selection, not the one current when
  the load completed.
- `TestResizeBetweenStagesCommitsAtNewWidth` — a resize landing
  between the stages supersedes the held layout; the intent survives
  the discard and commits against the new width's row model (a
  wrap-changing first line makes the widths distinguishable).
- `TestListToggleBetweenStagesCommitsAtFinalWidth` — hiding the file
  list between the stages is likewise a width change the commit
  honors.
- `TestObsoleteLayoutsNeverConsumeTheIntent` — wrong-width,
  wrong-mode, wrong-revision, and wrong-path completions are all
  discarded with the intent and the frame untouched.
- `TestStartupHiddenTargetCommitsOnInstall` — a startup target hidden
  from top 0 lands at `floor(23/3)` on install, then the first `n`
  advances to the second stop.
- `TestStartupVisibleTargetKeepsTopZero` — an already-visible startup
  target keeps top 0 through both stages and writes no saved state.
- `TestSavedViewportRevisitCommitsOnLoad` — a revisit during the
  file's reload starts from the saved anchor; a target inside the
  restored window leaves the top alone on commit.
- `TestSavedViewportRevisitHiddenTargetMoves` — the same revisit with
  a saved top that hides the target applies the one-third placement
  on commit and rewrites the saved state.
- `TestMarkerTargetCommitPaintsMarkerCell` — a terminator-only `$`
  target far down a run-off-edge file reveals its row and moves the
  offset so the marker cell paints as the last text cell.
- `TestClusterTargetCommitPaintsWholeCluster` — `n` during the gap
  selects a mid-cluster target; the commit's minimal reveal paints
  the whole `^A` escape cluster flush with the right edge.
- `TestNonCurrentCompletionLeavesPanelUntouched` — a departed file's
  load completion and keyed layout completion update only its cache:
  the current panel stays byte-identical and its intent survives.
- `TestPopupUnaffectedByCompletionStages` — the file-change pop-up
  keeps its instance through both the destination's load completion
  and the layout install that commits the reveal under it.

`stale_test.go` (same package, Issue #29) pins the stale-state
integration: the `file changed since search` note, the fallback
reveal through the two-stage commit, and the fixed exit status:

- `TestStaleBufferShowsFileChangedNote` — the note paints on every
  display (load, scroll, `n`, resize) with no timer, and no inverse
  video survives anywhere in the frame.
- `TestStaleNoteClearsOnlyOnCleanReload` — the note persists through
  a gate-held reread and clears only when the completion's
  revalidation is clean, the highlight returning with it.
- `TestStaleNoteComposedAtAllWidths` — the status slot across
  80→20-column frames: the note paints whole under a truncating
  escaped path where it fits and drops where it cannot, no row
  overflowing and no layout dimension negative.
- `TestGatedReloadCommitRevealsSurvivingSubmatch` — `n` during the
  gate-held reload selects the half-stale stop; the matching-layout
  commit reveals the survivor's highlight while the dropped submatch
  paints plain.
- `TestGatedReloadCommitRevealsClampedFallback` — all submatches
  dropped on a still-present line: the commit lands on the row
  holding the clamped recorded start (deep inside a wrapping line)
  with no invented highlight.
- `TestStaleAllDroppedRevealsClampedStart` — the same fallback on a
  direct navigation: the destination row reveals with no highlight
  and no marker.
- `TestStaleMissingLineLandsOnLastSourceLine` — a vanished stop line
  lands at the last source line's start, clamped to the frame's
  bottom.

`encoding_test.go` (same package, Issue #30) pins the
unsupported-encoding integration — the `(unsupported encoding)`
placeholder, the explanatory diagnostic's notification split, and
the reloadability and stale-exclusion contracts, all driven through
`Update` over real UTF-16 LE fixture files (`u16le`):

- `TestUnsupportedCurrentFileNotifies` — the current file's
  detection opens the explanatory overlay (`cannot display …:
  unsupported encoding UTF-16 LE`), collects one diagnostic, and
  paints the placeholder plus the filename-row note with no file
  bytes and no inverse video anywhere.
- `TestUnsupportedFileRemainsACursorStop` — `n`/`p` traverse the
  file's stops like any file's, and re-entering it re-opens the
  retained explanatory overlay instead of the file-change pop-up,
  without re-collecting.
- `TestUnsupportedNonCurrentIsDiagnosticOnly` — a detection settling
  while another file is current collects once, opens no overlay, and
  repaints nothing; visiting the file then surfaces the same overlay
  over the placeholder.
- `TestUnsupportedReloadReissuesAndPreserves` — `r` is the overlay's
  while it is open; dismissed, `r` drops the placeholder to
  `Loading…`, issues exactly one reread, and the still-encoded
  completion restores the placeholder with a fresh overlay and a
  second collection.
- `TestUnsupportedFileIsNeverStale` — encoded bytes never produce
  the stale mark or the `file changed since search` note.
- `TestUnsupportedComposedViewStaysWellFormed` — a long escaped path
  at 80→20 columns: the truncated safe path stays identifiable, the
  placeholder paints, no row overflows, and no dimension goes
  negative.

`pan_test.go` (same package, Issue #18) drives the horizontal-pan
keys through `Update`; `panRecords` builds a one-stop record set for a
file whose first line is the match:

- `TestPanKeysMoveOffset` — `,`/`.` one column, `<`/`>` ten, `[`/`]`
  `max(1, floor(text width / 2))` of the actual `m.textW`, all moving
  `m.vp.Offset()` in run-off-edge mode.
- `TestPanKeysNoOpInWrapMode` — every pan key inert under wrap: the
  offset stays 0 and the wrapped frame is byte-identical.
- `TestPanOffsetSurvivesWrapToggle` — `w w` through the async layout
  swap retains a valid offset.
- `TestPanOffsetResetsOnFileChange` — `n` into another file starts at
  offset 0 and `p` back does not restore the old value.
- `TestHalfClippedGlyphPaintsBlank` — a CJK cluster split by the left
  clip edge paints a blank cell, not a half glyph, in the real frame.
- `TestPanToMaximumPaintsFinalCluster` — panning to the visible-lines
  maximum stops at the final cluster's start so it paints whole, and
  further pans do nothing.

`nav_test.go` (same package) covers the Issue #13 `n`/`p` navigation
wiring; `twoFileModel` lands the model in browse on a.txt:1 with stops
a.txt:1, a.txt:3, b.txt:2 (records emitted b.txt-first) and b.txt
uncached (since Issue #15 `browseModel` also installs a nil-command
`popupTimer`, so crossings keep returning just the load command and no
real timer is started):

- `TestStartupSelectsFirstStopInPathOrder` — the cursor starts on the
  first stop in path order: the filename rule and list underline show
  a.txt, the startup load command is for a.txt, and its line-1 match
  renders `CurrentMatch` (inverse + underline) while line 3's stays
  plain `Match`.
- `TestNextWithinFileMovesCurrentLine` — `n` moves the underline to
  the line-3 match with no command and no viewport movement: the
  target is already on screen, so Issue #14's reveal is a no-scroll.
- `TestNextAcrossFileSwitchesPanelAndLoads` — `n` into b.txt returns
  the uncached file's load command, moves the list underline and
  filename rule immediately, shows `Loading…` until the buffer lands,
  starts the new file at top 0, and saves the departing file's
  viewport — a same-file `n` first pulled the scrolled top back to 0
  via the reveal, so 0 is what `saved` holds.
- `TestNavigationWrapsBothEnds` — `n` on the last stop wraps to the
  first and `p` on the first wraps to the last, both cached-file
  crossings returning no command.
- `TestOneStopIndexIgnoresNP` — `n`/`p` on a one-stop index return no
  command and leave a byte-identical frame.
- `TestManualScrollThenNContinuesFromStop` — scrolling deep into the
  file then `n` still advances from the last selected stop, and the
  Issue #14 reveal scrolls back to the now-hidden target, BOF-clamped
  to top 0.
- `TestDepartingViewportSavedAndRestoredOnRevisit` — `p` back into
  cached a.txt restores its saved top as the reveal's starting point;
  the already-visible target makes the reveal a no-scroll.
- `TestFileListHasNoDirectSelection` — non-navigation keys move neither
  the cursor nor the view: the file list is passive.

`reveal_test.go` (same package) covers the Issue #14 destination-reveal
triggers through `Update`; `fileWithStops` writes a lines-line fixture
whose stopped lines read `hitNNNNN` with the submatch covering the
leading `hit`:

- `TestStartupRevealPlacesHiddenTargetOneThirdDown` — the startup load
  lands the line-200 match on content row `floor(23/3)` = 7 (top 192)
  and the moving reveal replaces `saved`.
- `TestStartupRevealVisibleTargetDoesNotScroll` — an on-screen startup
  target keeps top 0 and writes no saved entry.
- `TestStartupRevealAppliesOnLoadCompletion` — a `loadGate`-held load
  shows `Loading…` at top 0; the reveal lands when the `loadDoneMsg`
  arrives.
- `TestNPRevealHiddenTargets` — `n` to a hidden target places it a
  third down and replaces `saved`; `p` back to a near-BOF target
  clamps to top 0.
- `TestNToVisibleTargetDoesNotScroll` — `n` between two on-screen
  matches moves only the underline: no scroll, no saved write.
- `TestRevisitStartsFromSavedViewport` — a cached revisit resumes the
  saved top when the target is inside it.
- `TestRevisitRevealOverridesHiddenSavedViewport` — a seeded saved top
  that hides the destination is overridden by the reveal, which writes
  the new top.
- `TestFirstVisitStartsAtTopThenReveals` — an uncached destination
  shows `Loading…` at top 0, then the load's top-of-file start plus
  reveal lands the target a third down.

`hreveal_test.go` (same package, Issue #19) drives the horizontal
reveal triggers through `Update` in run-off-edge mode, on fixtures
whose matches sit far right of the text area:

- `TestHRevealSameFileNScrollsRightMinimal` — `n` to a hidden-right
  match sets `m.vp.Offset()` to `start − (text width − 1)` and the
  frame paints the match start on the last text column.
- `TestHRevealPBackScrollsLeftToTarget` — `p` back to a hidden-left
  match lands the offset on the target column.
- `TestHRevealVisibleMatchKeepsOffset` — `n` to an already-painted
  match moves nothing.
- `TestHRevealAppliesAtStartup` — run-off-edge selected before the
  startup completion reveals the first stop's far match horizontally
  when the layout installs.
- `TestHRevealAfterFileChangeReset` — a file crossing resets the
  departed offset to 0, then reveals the destination's match.
- `TestHRevealWideClusterMatchPaintsBothCells` — a match starting on
  a two-cell CJK glyph reveals to `start + 2 − text width` so both
  cells of the first glyph paint at the right edge.
- `TestHRevealMidClusterMatchPaintsWholeCluster` (Issue #21) — `n` to
  a match recorded mid-cluster (a combining mark borrowing an escape
  cluster's trailing cell) keeps the offset because the expanded
  target cell is visible, and the painted span covers the whole
  cluster — both escape cells inverse.

`cluster_test.go` (same package, Issue #21) pins `renderCells` blank
safety directly:

- `TestRenderCellsBlankFillersNeverMatchStyled` — a `Blank`-marked
  filler under a covering span and a clip-edge split blank both paint
  unstyled, while a marker on a blanked cell still paints its inverse
  space.

`indicators_test.go` (same package, Issue #20) drives the
hidden-content indicators through the rendered frame in run-off-edge
mode; `indModel` lands a settled `w`-toggled browse, `frameLines`/
`cellAt` probe cells of the ANSI-stripped `View()`, and `indCol`
locates the gutter's first trailing space:

- `TestGutterUnderscoreStarAndBlank` — per-line gutters at a nonzero
  offset: `*` where the line's match is entirely hidden left, `_`
  where only text hides left, blank on an empty line, and `_` on a
  line whose match hides right instead; no right `*` anywhere.
- `TestIndicatorsPaintInverse` — both marks emit the theme's inverse
  pair `30;47` restored to base.
- `TestRightStarCurrentMatchedLineOnly` — the reserved column's `*`
  on the current matched line's row only: an unmatched line's
  hidden-right text and a non-current matched line's hidden-right
  match both leave it blank.
- `TestRightStarAbsentWhenCurrentLineOffScreen` — scrolling the
  current matched line out of view blanks every row's reserved column
  while other gutters keep `_`.
- `TestBothSidesHiddenStarsTogether` — one row carries the gutter `*`
  and the reserved `*` at once.
- `TestPartialMatchVisibilityDrawsNoStar` — a match one cell into the
  window stays visible on either side: `_` not `*` left, blank not
  `*` right, turning to `*` only when the last cell leaves.
- `TestLastCellMatchAndFarMatchRightStar` — a match ending on the
  last text cell paints its final glyph while the farther match's `*`
  takes the reserved cell — the indicator never overwrites text.
- `TestSplitGlyphBlanksDrawStars` — a match on a two-cell 世 split by
  the right edge earns the current row's right `*`, split by the left
  edge earns another row's gutter `*`, and whole-cluster offsets
  clear both.
- `TestWrapModeDrawsNoIndicatorsOrReservedColumn` — wrap mode draws
  blank indicator cells and lets wrapped text occupy the frame's last
  column.
- `TestUniformLinesEveryGutterUnderscore` — the Issue #18
  uniform-lines geometry signposts `_` on every visible line.
- `TestMidClusterMatchCountsFromClusterStart` (Issue #21) — the
  indicator tests consume the expanded spans: a match recorded on a
  combining mark mid-cluster paints its clipped row as the expanded
  `[cluster start, cluster end)` span, and once that whole cluster
  stands left of the window the gutter upgrades to `*`.
- `matchRecJSON` — the record helper for lines carrying raw control
  bytes, which JSON-escapes the `lines` text (`\x01` → `\u0001`).

`popup_test.go` (same package) covers the Issue #15 file-change
pop-up; `popupModel` lands a two-file browse with `instantPopupTimer`
(an injected `popupTimer` seam resolving to `popupExpiredMsg{id}`
immediately, so `cmdMsgs` flattens a crossing's batch — load + expiry —
with no sleeps) and `popupBox` locates the box in a frame:

- `TestPopupOpensCentredAtSelection` — the same-file `n` shows no box;
  the crossing `n` shows the destination's `Path`-escaped name in a
  bordered box whose border columns sit exactly at the centre of the
  80×24 frame over `Loading…`, and the returned command flattens to
  the destination's `loadDoneMsg` plus `popupExpiredMsg` carrying the
  instance ID — the pop-up starts at selection, not at load.
- `TestLoadCompletionDoesNotRestartPopup` — the load completing
  leaves the same instance active (content renders beneath); its
  follow-up command is the Issue #17 layout request, not a pop-up
  restart, and the instance's own expiry then dismisses.
- `TestPopupInstanceKeyedExpiry` — a second crossing mints a fresh
  instance; the first's stale expiry leaves the newer pop-up up, and
  only its own expiry clears it.
- `TestPopupKeyDismissalStillActs` — `down` dismisses the pop-up and
  scrolls one row in the same update.
- `TestPopupDismissKeyStillNavigates` — the quick second `n` dismisses
  and navigates: the cursor lands on the next file and that
  crossing's own fresh pop-up opens.
- `TestPopupDismissalKeys` — `q` dismisses and quits with `ExitCode` 0;
  `Esc` performs only the dismissal.
- `TestPopupRecentresOnResize` — shrinking to 40×10 recentres the box
  on the new frame with the same instance alive and no command
  returned.
- `TestPopupLeftTruncatesLongPath` — a 104-cell destination name
  renders left-truncated with a leading `…` sized to the 78-cell
  interior at 80 columns, and renders untruncated at 120.
- `TestErrorOverlayCancelsPopup` — a `loadDoneMsg` failure for the
  current path opens the diagnostics overlay (`cannot read …`), the
  pop-up is gone and never returns after `Esc` dismissal, and its
  late expiry is inert.
- `TestNoPopupWithoutCrossing` — the startup stop and its completed
  load open nothing.

`noresults_test.go` (same package) covers the Issue #8 no-results
outcome; `exitErr` builds a real `*exec.ExitError` child wait error and
`noResultsModel` lands the model on the screen after a done message:

- `TestEmptyStreamShowsNoResults` — an rg-1 summary-only stream shows
  the centred `No results found` screen (32-cell pad at 80 columns,
  middle of 24 rows), no suffix, no load command, neither the
  searching nor browse screens.
- `TestAllBinaryStreamShowsSkipCount` — an rg-0 stream whose every
  matched file was binary-excluded appends `(N binary files skipped)`
  with the distinct-file count.
- `TestMixedStreamBrowsesRetained` — one excluded and one retained file
  browse with usable results 1; the excluded file never appears.
- `TestQOnNoResultsExitsOne` — `q` quits with `ExitCode` 1 through the
  ordinary path, not cancellation.
- `TestEscOnNoResultsIsNoOp` — `Esc` changes nothing; the fixed status
  stays 1 and the screen stays up.
- `TestCtrlCOnNoResultsExits130` — `ctrl+c` overrides the fixed status
  with 130.

`outcome_test.go` (same package) holds the Issue #9 outcome-transition
matrix — the single table (`outcomeMatrix`) owning every outcome row so
later issues extend it with rows rather than duplicating the decision:

- `TestOutcomeMatrix` — drives each row's records + `code` (`exitErr`
  for codes, `sigErr` — a real SIGKILLed child — for signal death) +
  `stderr` through `searchDoneMsg`, asserting the initial screen and
  overlay state, the overlay's diagnostic substrings, the dismissal
  key's post-dismissal state (or quit-at-2 for the fatal-only overlay),
  and the final exit status. Rows cover rg 0/1 clean results and empty
  streams, fatal codes and signal deaths with and without usable
  results, missing-summary/orphaned-end/open-at-end integrity
  failures, stderr warnings over browse and no-results, all-binary
  after a warning, and `ctrl+c` → 130 from browse, no-results, and an
  open overlay. Issue #10 added the record-loss rows (malformed/oversized
  skips with usable results → browse + warning overlay → 0; with zero
  usable results — including after binary exclusion emptied the index —
  → record-loss fatal → 2; unknown-type-only warnings → warning overlay
  → no-results → 1) plus missing-`end` variants; rows carrying
  undecodable bytes use the `stream` field through `fixtureStream`,
  which pipes raw bytes into `Index.Feed` rather than decoding records.
  Issue #26 added the `failAll`/`absent`/`viewHas`/`replayHas` row
  fields — `failAll` marks every retained file's `loadDoneMsg` an
  injected error and the new assertions check the overlay's absences,
  the composed frame's `(unreadable)`/path forms, and the exit replay —
  plus three rows: fixed-0 all-fail → still 0, fixed-2 current-file
  failure → still 2, and the composed all-fail-with-fixed-2 row proving
  the already-fixed fatal outcome is never recomputed. Issue #29 added
  the `fileData`/`loadCurrent` row fields — `fileData` replaces the
  fixture's bytes so the load can validate stale and `loadCurrent`
  settles the load before the assertions — and the all-stale row
  proving every retained stop validating stale still exits with the
  fixed status 0. Issue #30 reuses those fields for the
  all-unsupported row: every retained file detecting a UTF-16 BOM
  still exits with the fixed status 0, the overlay and the exit
  replay carrying the `unsupported encoding` diagnostic.

`overlay_test.go` (same package) pins the Issue #9 modal overlay:

- `TestOverlayScrollsWithUpDown` — head visible first, tail only after
  scrolling, clamped at both ends.
- `TestOverlayDismissKeys` — `q` and `Esc` each dismiss a non-fatal
  overlay to the underlying screen.
- `TestOverlayCtrlCExits130` — cancellation beats the fixed status.
- `TestOverlayIgnoresOtherKeys` — `x`, `n`, `c`, digits, arrows, tab,
  enter all inert; `c` never reaches the colour toggle.
- `TestOverlayWrapsUnbrokenDiagnostic` — a 300-cell diagnostic wraps
  inside the border; no frame row exceeds the terminal width.
- `TestFailedProcessWithoutStderrGetsGeneratedDiagnostic` — a failed
  child with empty stderr still yields a diagnostic naming its exit
  status or signal.

`help_test.go` (same package, Issue #31) pins the modal help overlay —
the second instance of the shared wrapped-scrollable `overlay`
component:

- `TestHelpOpensFromBrowse` / `TestHelpOpensFromNoResults` — `h` and `?`
  each open the bordered binding list over both base states; over
  no-results, `Esc` closes back to the screen and a subsequent `q`
  still exits 1.
- `TestHelpCloseKeys` — `q`, `Esc`, `h`, and `?` each close to browsing.
- `TestHelpIgnoresOtherKeys` — `n`/`p`/`w`/`c`/`r` and more are inert:
  cursor, wrap flag, theme, and phase unchanged, no commands.
- `TestHelpCtrlCExits130` — cancellation beats the fixed status.
- `TestHelpKeysInertWhileSearching` — `h`/`?` during collection open
  nothing.
- `TestHelpScrollsWithUpDown` — at a reduced height the binding list
  scrolls, clamps at both ends, and reaches head and tail.
- `TestHelpWrapsUnbrokenSubstitution` — a 300-cell unbroken footer
  string wraps inside the border; no frame row exceeds the width.
- `TestHelpClippedAtTinySize` — at 25×8 the box is clipped to the
  terminal without a borderless mode and renders without panic; growth
  restores the normal layout.
- `TestHelpRendersBindingTable` / `TestHelpRendersFooterSlot` — every
  `helpBindings` row's key spelling and description, plus the footer
  slot's text, appear in the render.
- `TestHelpCancelsPopup` — an active file-change pop-up is cancelled on
  open, does not return on close, and its stale expiry stays inert.

`precedence_test.go` (same package, Issue #32) pins the combined
overlay-precedence and dismissal semantics:

- `TestErrorSuspendsHelpRestoringScroll` — for both `q` and `Esc`: the
  load gate parks the current file's failing worker, help opens at a
  reduced height and scrolls to row 5, the released failure opens the
  error over the suspended help, `down` scrolls the error while the
  help offset stays put and `h`/`n`/`r`/`w`/`c` are swallowed, and
  dismissal restores help at row 5 over browsing.
- `TestAppendedErrorPreservesReaderScroll` — a read-failure overlay
  scrolled to 3 receives an unsupported-encoding append (a different
  error source than Issue #26's reload re-entry): the reader stays at
  3 and the appended text is reachable at the bottom.
- `TestDismissalOutcomeTable` — the dismissal-outcome table run for
  both `q` and `Esc`: browse error overlay → browsing, browse help →
  browsing, browse error-over-help → help restored at its scroll then
  a second dismissal → browsing then a base-state `q` exits, warning
  overlay over an empty result → no-results, help over no-results →
  no-results, fatal overlay → exit 2, record-loss overlay → exit 2 —
  with state-specific follow-ups (a second `q` exits the fixed status
  from each still-running base state; a second `Esc` is a no-op).

`sinksafety_test.go` (same package) holds the Issue #6 shared
sink-safety table:

- `TestSinkSafetyTable` — the shared hostile fixture set (OSC, CSI,
  C0, C1, DEL, standalone CR, invalid UTF-8 path bytes, embedded
  filename newline) driven through every sink's real composition path:
  file-list entry, filename rule, panel content, usage-error stderr
  (`cli.Parse` diagnostic + `cli.HelpText()` composition), CLI-help
  stdout, and — since Issue #9 — the `error overlay` row, where the
  fixture rides in as captured child stderr over a fatal exit and the
  check asserts the `Diagnostic`-escaped `wantDiag` forms inside the
  border, and — since Issue #15 — the `file-change pop-up` row, where
  `renderPopupSink` names the destination file with the fixture bytes
  and asserts the `Path`-escaped `wantPath` form inside the box, and —
  since Issue #31 — the `help overlay` row, where `renderHelpSink`
  injects the fixture bytes through the footer substitution slot and
  asserts the `Diagnostic`-escaped `wantDiag` forms inside the border.
  Under
  `theme.Plain` (the no-style composition path) the raw
  output — asserted before any ANSI stripping — must contain no fixture
  control byte verbatim and none of the universal set
  (`\x1b`, `\x07`, `\x9b`, `\xc2\x85`, bare `\r`); TUI rows pin the
  frame at height-1 newlines so an embedded filename newline cannot
  add a row. A styled pass asserts the fixture's distinctive payload
  never follows an unescaped ESC. The `sinkSafetySinks` table is
  extensible: later issues add rows for their sinks without
  duplicating fixtures.

`replay_test.go` (same package) is the Issue #11 session-collection
and replay coverage — `replayed` runs `Model.ReplayTo` and `driveEvent`
pumps one collector event through `Update` the way the real program
does (bounded, so a silent channel fails rather than hangs):

- `TestReplayCollectsDisplayedAndUndisplayedInOrder` — a streamed
  stderr warning (shown in the overlay) plus two `loadDoneMsg` failures
  no screen displays replay exactly once each in collection order on a
  normal `q` quit.
- `TestShutdownBoundaryCtrlC` — a real `Start`ed session with a
  gate-held completion: the streamed `rg: warn one` is collected, then
  `ctrl+c` exits 130 and replays only it; the completion's gated
  `missing summary` diagnostic is never waited for or replayed, and
  cancellation ends collection promptly with no index prepared.
- `TestShutdownBoundaryQ` — the same boundary on the `q` route in both
  incomplete states: `searching` (fake rg warns then `sleep`s forever —
  collected warning replayed, `Reaped` closes promptly) and
  `gate-held preparation` (child exited, completion held — only the
  collected warning replays).
- `TestControlledFailureJoinsSessionCollection` — the boundary's
  `CollectDiagnostic` route replays the `vrg:` failure line after the
  earlier diagnostics, once each, in collection order.
- `TestReplayEscapesEmbeddedFilename` — a load failure on a path with
  an embedded newline and ESC replays as one line carrying the
  `Path`-escaped form (`f\no^[.txt`) with no raw control byte.

`sinksafety_test.go` additionally gained the `stderr replay` row:
`renderReplaySink` drives each hostile fixture through the collection
plus `ReplayTo` and the check asserts the escaped `wantDiag` and
`wantPath` forms in the replayed output.

`model_test.go`'s completion helper became `awaitDone`: `Init` now
yields incremental `diagMsg`s as well as `searchDoneMsg`, so the helper
pumps the command and every follow-up through `Update` until the
completion arrives (bounded — a missing follow-up or timeout fails).

`subprocess_test.go` re-executes the test binary as fake rg via
`TestMain` (`VRG_FAKE_RG` mode, `VRG_FAKE_DIR` artifacts):

- `TestChildArgvAndWorkingDirectory` — the child receives `Config.Argv`
  verbatim and runs in `Config.Workdir` (argv and `getwd` captured to
  files).
- `TestDualPipeBackpressure` — the `flood` fixture writes 20 × 64 KiB to
  stderr interleaved with 20 valid match records; all matches land in
  the index, all stderr bytes are captured, and a post-write
  `writes-done` handshake proves the child finished both pipes before
  exiting.
- `TestStartFailure` — explicit missing path and PATH-lookup failure
  both error from `Start` with control-byte-free diagnostics.
- `TestCancelTerminatesAndReapsChild` — `Session.Cancel` against a
  `block` fake rg: the child pid disappears (`ESRCH`), `Reaped` closes
  promptly, and `ReapReport` receives the `signal: killed` wait status —
  proving vrg's own `Wait` path ran.

## internal/viewport

`viewport_test.go` (same package) pins the Issue #12 scroll and clamp
contracts against the `countingRows` provider fake:

- `TestScrollUnits` — the per-key table: `Down`/`Up` one rendered row,
  `HalfDown`/`HalfUp` `max(1, floor(h/2))` including odd heights and
  the height-1 floor, `PageDown`/`PageUp` the content height.
- `TestScrollSequence` — a mixed sequence lands on accumulated
  offsets.
- `TestClampAtBOF` / `TestClampAtEOF` — `up`/`u`/`pgup` at the top do
  nothing; downward units clamp at the last full page with the final
  row on the bottom row.
- `TestFileShorterThanViewport` / `TestFileEqualToViewport` — short
  and exact-fit content pin the top at 0; short content shows only its
  own rows.
- `TestEmptyContentIsInert` — nil and zero-row providers give empty
  `Visible()` and inert scrolls.
- `TestVisibleQueriesOnlyVisibleRows` /
  `TestVisibleNearEOFQueriesRemainder` — the provider records exactly
  the shown row indices, including the truncated range near EOF.
- `TestResizeReclampsTop` — growth re-clamps the top (the lossy EOF
  clamp); width-only resizes don't move it.
- `TestSetRowsClampsToNewContent` — swapped rows preserve the top
  clamped to the new count; the clamp loss is permanent.

`anchor_test.go` (same package, Issue #17) pins the logical-anchor
contract against real prepared `viewport.Model`s at alternating widths
and wrap modes:

- `TestAnchorRoundTripThroughRewrap` /
  `TestAnchorRoundTripOnLaterLine` — narrowing then widening (and the
  reverse) lands the effective top back on the row containing the
  anchor's text, on the wrapped lead line and on later lines.
- `TestAnchorRoundTripThroughWrapToggle` — wrap off then on restores
  the retained logical column rather than the line's first row.
- `TestScrollReplacesAnchor` — a scroll that moves rewrites the
  anchor to the resulting top row's logical location.
- `TestMovingRevealReplacesAnchor` /
  `TestMovingRevealInRunOffEdgeDropsColumn` — a reveal that moves the
  viewport replaces the anchor; in run-off-edge mode a retained
  mid-line column is dropped for the row's own start.
- `TestNoScrollRevealKeepsAnchorColumn` — a reveal whose target is
  already visible leaves the anchor — logical column included —
  untouched.
- `TestEOFClampRewritesAnchorLossy` — growth that pulls the top off
  the anchor's row rewrites the anchor to the clamped row; a later
  shrink does not restore the pre-clamp position.

`reveal_test.go` (same package) pins the Issue #14 `Reveal` contract
against `countingRows` plus `mappingRows` — a provider fake whose
`RowOf` mapping is programmable and records every target it sees,
proving the reveal consumes a rendered row, not a source-line ordinal:

- `TestRevealVisibleTargetDoesNotScroll` — a target on the first,
  middle, or last visible row never scrolls and reports no move.
- `TestRevealHiddenTargetLandsOneThirdDown` — a hidden target above or
  below lands on row `floor(height/3)`, including the just-past-edge
  case.
- `TestRevealTargetNearBOFClampsToTop` / `TestRevealTargetNearEOFClampsToLastPage`
  — the 0/`maxTop` clamps beat exact one-third placement.
- `TestRevealUsesRenderedRowContainingTarget` — a wrap-like many-to-one
  `RowOf` lands the containing row; the provider sees the exact
  `(line, cell)` target.
- `TestRevealAfterSavedAndTopStartingPoints` — the entry sequence:
  saved top + visible target survives, saved top + hidden target moves,
  first visit starts at 0 then reveals.
- `TestRevealClampsOutOfRangeTarget` — an out-of-range target resolves
  to the nearest real row, then the EOF clamp still applies.
- `TestRevealWithoutContentIsNoOp` — nil and zero-row providers are
  inert.

`wrap_test.go` (same package, Issue #16) pins the prepared row model
against `lineSource`, a `Source` fake over real `present.Line`
segmentation that records every per-line `Cells`/`Spans` query:

- `TestWrapRowModel` — the wrap table: short/empty/exact-multiple row
  counts, ASCII packing, a wide cluster unfit for the row's remainder
  moving whole and leaving a blank, a combining cluster kept whole, an
  over-wide cluster splitting across rows with its clipped lead
  blanked, and the tab expansion moving whole or splitting like any
  oversized cluster.
- `TestWrapRowLineAndContinuation` — every rendered row reports its
  source line and `Cont` flag; scroll units stay rendered rows.
- `TestRunOffEdgeRowModel` — one row per line carrying full cells for
  the frame to clip; `RowOf` is the source line regardless of cell.
- `TestRowModelKey` — `Key{Path, Rev, Width, Wrap}` identifies what a
  model was prepared for; distinct preparations differ by key.
- `TestWrapTranslatesSpansPerRow` — coverage spans clip to each row's
  cell range and markers paint on their owning row (a boundary
  position belongs to the next row).
- `TestEndOfLineMarkerRows` — a marker after a completely full final
  wrap row occupies another row (an empty continuation row painting
  the marker at column zero); on a non-full last row it stays put.
- `TestRevealFindsRowOfWrappedLine` — `Reveal` resolves a target deep
  inside a screen-tall wrapped line to its containing rendered row and
  lands it at `floor(height/3)`; a tail-line target still EOF-clamps.
- `TestVisibleRowsNeverWrapsOffscreenLines` — the render-cost guard:
  after `Prepare`, a frame queries `Cells`/`Spans` only for the lines
  behind its visible rows — never O(N) per frame.

`pan_test.go` (same package, Issue #18) pins the horizontal-pan and
visible-lines extent contracts against `lineSource`-backed prepared
models:

- `TestPanUnits` — the per-key table: `Left`/`Right` one column,
  `TenLeft`/`TenRight` ten, `HalfLeft`/`HalfRight`
  `max(1, floor(width/2))` including odd widths and the width-1
  floor, clamped at 0 and at the 300-cell fixture's 299 maximum.
- `TestSetOffsetClamps` — the stored-offset entry point clamps to
  `[0, S]`; the file-change reset drives it with 0.
- `TestPanNoOpInWrapMode` — every pan unit inert under a wrap model;
  the dormant offset neither moves nor leaks into the wrapped rows.
- `TestPanWithoutContentIsNoOp` — no prepared rows → pans do nothing.
- `TestOffsetRetainedThroughWrapToggle` /
  `TestWrapReentryClampsToNewVisibleSet` /
  `TestWrapReentryClampsAfterWidthChange` — `w w` retention, re-entry
  clamping against a visible set that changed while wrapped (a scroll
  or a width change re-fitting clusters), and no restoration.
- `TestExtentFollowsWidestVisibleLine` — the mixed-width policy: 299
  while the 300-cell line is visible, 9 once it scrolls out, permanent
  loss on scroll-back.
- `TestExtentEmptyViewsClampToZero` — placeholder, empty buffer, and
  all-empty visible lines all clamp to 0.
- `TestPaintableBoundaryStopsAtFinalCluster` — a two-cell final
  cluster makes the maximum its start, and at that offset both cells
  paint whole.
- `TestPaintableBoundaryUnfittableFinalCluster` — a final cluster
  wider than the text width falls back to the last fitting cluster's
  start; a line with no fitting cluster reports 0, the documented
  exception where the window paints clipping blanks.
- `TestReclampOnVisibleSetChanges` — vertical scroll, moving reveal,
  width resize (also how list hide/show and gutter growth arrive), and
  a row-model swap each re-clamp; a pan recomputes the maximum from
  the now-visible rows.
- `TestSplitClusterClipsToBlankCells` — a clip edge inside a two-cell
  cluster paints blanks at either edge; whole inside the window it
  paints whole.
- `TestClipTranslatesSpans` — coverage spans shift and clip to the
  window; marker spans follow their cell and vanish when clipped away.
- `TestUniformLinesAllHiddenLeft` — every visible line can have
  hidden-left text at a nonzero offset while still painting — the
  geometry Issue #20's `_` indicators require.
- `TestExtentQueriesOnlyVisibleRows` — the render-cost guard: a pan
  queries only the visible rows' lines; a scroll touches only the new
  window — never a whole-buffer scan.

`hreveal_test.go` (same package, Issue #19) pins the horizontal half
of `Reveal` against `lineSource`-backed prepared models in
run-off-edge mode:

- `TestHRevealRightOfViewPaintsStartCell` — a single-cell target right
  of view moves the offset to `T − (text width − 1)` and paints.
- `TestHRevealRightOfViewWideClusterPaintsWhole` — a two-cell target
  moves to `T + cluster width − text width`, both cells painted.
- `TestHRevealLeftOfViewLandsOnTargetColumn` — a hidden-left target
  (including one split by the left clip edge) moves the offset to its
  start column.
- `TestHRevealPaintedTargetKeepsOffset` — painted targets, including a
  cluster flush with the right edge, move nothing.
- `TestHRevealClippedBlankCountsAsHidden` — a geometrically inside
  position blanked by a two-cell glyph at the right edge counts as
  hidden and reveals.
- `TestHRevealOversizedSpanShowsStartCell` — a match wider than the
  text area reveals its start cell alone (clipped span `{9, 10}` at
  the window edge).
- `TestHRevealUnpaintableClusterUsesStartColumn` — a cluster wider
  than the text area takes `off = start` beyond the paintable
  boundary, renders in-window clipping blanks, and a repeat reveal
  does not move (no loop).
- `TestHRevealMarkerCells` — marker targets: an end-of-line marker
  reveals as one cell; a marker on a clipped cluster's lead stays
  painted.
- `TestHRevealMovesBothAxes` — a vertically and horizontally hidden
  target reveals on both axes in one call.
- `TestHRevealNoOpInWrapMode` — wrap-mode reveal leaves the dormant
  offset untouched.

`indicators_test.go` (same package, Issue #20) pins the `Row`
indicator flags against `lineSource`-backed run-off-edge models;
`hid` returns the one visible row for a line, width, and offset:

- `TestHiddenLeftTextFlag` — `HiddenLeft` for any line cells left of
  the window, including a line fully hidden beside a wide sibling,
  while an empty line and offset zero stay clear.
- `TestMatchHiddenLeftFlag` — `MatchHiddenLeft` for a match or marker
  entirely hidden left; half-painted and window-edge-flush matches
  and an in-window marker count as visible.
- `TestMatchHiddenRightFlag` — `MatchHiddenRight` likewise on the
  right, including a marker on the first column past the window.
- `TestHiddenBothSidesFlags` — one line flags both sides at once.
- `TestSplitClusterBlankCountsAsHidden` — a match on a two-cell
  cluster split by either clip edge counts as entirely hidden on that
  side; whole-cluster offsets clear it.
- `TestUnpaintableClusterCountsNotVisible` — a cluster wider than the
  window blanks the whole text area; a match on it reports hidden
  right while `HiddenLeft` covers the plain cells.
- `TestWrapModeNoIndicatorFlags` — wrap-model rows carry no flags.

`blanks_test.go` (same package, Issue #21) pins the `Blank` filler
mark on substituted cells:

- `TestWrapSplitBlankIsMarked` — the lead cell a wrap row blanks for a
  last-resort-split cluster is `Blank`-marked with the covering span
  still spanning it; rows continuing the split keep unmarked `Cont`
  cells.
- `TestClipBlanksAreMarked` — `clipRow` marks the in-window cells of a
  cluster split at either clip edge `Blank` while the clipped span
  still covers them.

`marker_test.go` (same package, Issue #23) pins marker participation
in the shared rules against `lineSource`-backed models:

- `TestMarkerExtentAndPanClamp` — an end-of-line marker extends the
  line's extent so the pan maximum lands on the marker's own cell,
  including past a final cluster too wide to fit.
- `TestMarkerOnlyLineExtent` — a marker on an empty line gives extent
  1 and maximum offset 0.
- `TestMarkerHiddenDrivesIndicators` — an entirely hidden marker
  upgrades the gutter signpost even on a marker-only line with no
  text to hide; the `hit\r\n` marker flags hidden left.
- `TestTerminatorMarkerIsOrdinary` — the `hit\r\n` marker at display
  column 3 follows the shared extent, reveal, clip, right-indicator,
  and wrap-overflow rules unchanged.
- `TestInteriorMarkers` — BOL and mid-line markers follow their cells
  through clipping and set the hidden-side flags when clipped away.

## internal/theme

`theme_test.go` (same package) pins the Issue #7 style contracts at
exact-SGR granularity:

- `TestDarkSchemeIsInitiallyActive` — `Dark()` is not light and its
  `Base` emits white on black (`37;40`).
- `TestSchemeColourPairs` — both schemes' base pairs.
- `TestToggleRoundTrip` — dark → light → dark.
- `TestMatchIsTrueInverse` — `Match` emits the swapped pair and
  restores the base pair, in each scheme.
- `TestCurrentMatchUnderline` — `CurrentMatch` adds underline to the
  inverse pair and restores both.
- `TestCurrentFileUnderline` — `CurrentFile` is underline-only in both
  schemes.
- `TestIndicatorInverse` — `Indicator` uses the inverse pair.
- `TestBaseColourStyles` — `Gutter`/`FileList`/`FilenameRule` emit the
  base pair in each scheme.
- `TestOverlayStyle` — `Overlay` frames content in a plain single-line
  border painted in the base colours.
- `TestPlainIsIdentity` — every style is the identity under `Plain`
  and `Overlay` emits no escape byte.

## cmd/vrg (subprocess boundary)

`main_test.go` builds the real binary once in `TestMain` and asserts
stdout/stderr/status separately:

- **`TestGeneratedHelpStdout`** (named group, rerun by Issue #6) — all
  help rows exit 0 with exactly one help copy, empty stderr, no stub, no
  terminal control sequences.
- **`TestCLIOutputSafety`** (named group, rerun by Issue #6) — hostile
  operand bytes escaped on stderr, stub substitutions escaped, no native
  library bytes.
- `TestHelpWithoutRipgrep`, `TestHelpDoesNotInvokeRipgrep` — help works
  with rg absent and never execs a sentinel fake rg.
- `TestHelpIgnoresExecutableName` — hostile argv0 cannot reach output.
- `TestExecutableBoundary` — search/error status table, stdin-rejection
  wording. Usage errors assert exit 2, empty stdout, a sanitized
  diagnostic first line on stderr, and exactly one generated usage block
  after it (no library `Error:`/`incorrect usage` text).
- `TestDashFileRootAtProcessBoundary` — `./-` in a real temp dir reaches
  the TUI's browse view.
- `TestHelpAssignmentSpellingsAreUsageErrorsAtBoundary` —
  `--help=false`/`-h=false`/`--help=true` are exit-2 usage errors with
  no help on stdout (Issue #2 pinned the status).
- `TestSearchLifecycleAtBoundary` — replaces the Issue #2 stub test. A
  shell-script fake rg captures its argv and cwd, sleeps briefly so the
  harness observes `Searching…`, then emits one match. The
  `runVrgTUI` helper pipes stdin, watches stdout for the browse
  filename marker, and sends `q`. Asserts per row: `Searching…` then
  the browse view's `f.txt` in the list/rule on stdout, exact child
  argv (combined expansion, mixed aliases, empty/`-`/`--`/`-foo`
  patterns), exact working directory, empty stderr, exit 0. The fixture
  workdir gained `f.txt` under Issue #11 — without it the load failure
  is a legitimately collected diagnostic and replays to stderr.
- `TestDualPipeDrainageAtBoundary` — a shell fake rg floods stderr with
  16 × 64 KiB while emitting a valid begin/match/end/summary stream;
  the captured stderr opens the warning overlay (escaped NULs read as
  `^@`, the first step's marker), `q` dismisses it to the browse view
  (the second step's `f.txt` marker), `q` quits at 0, a `writes-done`
  handshake file proves the child finished both pipes, and — since
  Issue #11 — vrg's own stderr carries the collected flood exactly
  once, sanitized (`^@` forms), as the post-restoration replay.
  `runVrgTUISteps` drives the
  marker/key step sequence; `runVrgTUI` is the one-step wrapper.
- `TestStartFailureNoRipgrep` — rg-free PATH: exit 2, empty stdout (no
  TUI), and a sanitized `vrg:` diagnostic naming the failure with no
  raw control bytes.
- `TestFlagAndArgvUsageErrors` — Issue #2 error classes at the boundary:
  `-e`, argument-taking options, `-uuu` and mixed-alias third `-u`,
  `=` spellings, unprotected `-foo`, and a post-terminator `--help`
  root.
- `TestHelpWithSearchFlags` — help precedence with flags present
  (`-i --help`, `-ih foo`, `-i -s --help`, `-uuu --help`, `-e --help`,
  `foo . --help`): exit 0, one help copy, empty stderr.

`pty_test.go` (Linux-only, `//go:build linux`) runs the real binary on a
real PTY: `openPTY` allocates `/dev/ptmx`, grants/unlocks the slave, and
the child runs in a new session with the slave as controlling terminal
for stdin/stdout/stderr while the master captures all bytes. Fake-rg
shell scripts (installed by `writeFakeRg` from `main_test.go`):
`fakeRgBlockScript` writes its pid to a `ready` file then `exec sleep
3600`; `fakeRgStreamScript` writes its pid, emits a complete one-match
record stream, and exits 0. The `VRG_CAPTURE_DIR` env tells the scripts
where to write `ready`; the `VRG_TEST_*` seams come from
`cmd/vrg/hooks.go`:

- `TestPTYQWhileSearchingExits130` — `q` against a blocked fake rg:
  exit 130, child pid gone (`ESRCH`), the `VRG_TEST_REAP_FILE` side
  channel reports `killed`, the captured stream carries the
  display-restoration sequences (`\x1b[?1049l` leave-alt-screen,
  `\x1b[?25h` show-cursor), and `waitExit` asserts the slave's termios
  equals its pre-launch value.
- `TestPTYCtrlCWhileSearchingExits130` — same assertions driven by the
  raw `ctrl+c` byte through the PTY.
- `TestPTYSIGINTWhileSearchingExits130` — a real `SIGINT` signal to the
  process (surfacing as `tea.ErrInterrupted`): exit 130, child reaped,
  termios restored.
- `TestPTYQDuringGateHeldPreparationExits130` — fake rg emits and exits
  while `VRG_TEST_GATE_FIFO` holds index preparation; the reap file
  already shows `exit status 0` when `q` cancels from the gate-held
  window: exit 130, no browse filename rendered, termios restored.
- `TestPTYOrdinaryExitReapsChild` — `SIGTERM` ends the program without
  a cancellation key while the fake rg still runs: exit 0, child reaped
  (`killed`), terminal restored — the ordinary path cleans up too.
- `TestPTYControlledFailureExits2` — `VRG_TEST_FAIL_FIFO` injects a
  program error mid-search: exit 2, child reaped, termios restored, and
  the `vrg:` diagnostic appears in the PTY stream only after the
  restoration sequences, exactly once.
- `TestControlledFailureDiagnosticOnStderr` — pipe-mode run (held-open
  stdin pipe) with the fail fifo: exit 2, stderr carries the sanitized
  `vrg:` diagnostic exactly once, stdout carries none, and the child is
  still reaped.
- `TestPTYNonZeroExitBrowseOverlayExits2` (Issue #9) —
  `fakeRgExit3Script` signals `ready`, emits a complete valid stream,
  and exits 3: the error overlay opens over browse naming `exit status
  3`, the first `q` dismisses it, the second quits at the fixed status
  2, and the child is reaped.
- `TestPTYStderrContentOverlayHeadAndTail` (Issue #9) —
  `fakeRgFloodScript` interleaves over 1 MiB of stderr (head marker,
  ~525 padded lines, tail marker) with a valid stdout stream; on a
  2100×640 PTY (`openPTYSize`/`startVrgPTYSize`) the whole warning
  overlay fits one frame, so both `ERRHEAD-MARKER` and `ERRTAIL-MARKER`
  are visible at once, dismissal reveals the complete browse view, and
  the exit status stays 0 — a warning, not a failure.

`pty_replay_test.go` (same Linux-only harness) is the Issue #11
process-boundary replay coverage. Every test wires
`VRG_TEST_DIAG_ACK_FILE` and waits on the acknowledgement — one line
per diagnostic processed into the session collection — before sending
the exit key; `assertReplayedOnce`/`replayTail` assert on the captured
stream after the `\x1b[?1049l` display-restoration sequence. Fake-rg
scripts: `fakeRgWarnBlockScript` (warns then `sleep`s),
`fakeRgWarnStreamScript` (warns + complete stream), `fakeRgWarnNoSummaryScript`
(warns + stream without `summary`, so the completion carries a gated
`missing summary` diagnostic), `fakeRgBadPathScript` (a complete stream
whose one match names a file with an embedded newline and ESC):

- `TestPTYCtrlCAfterDiagnosticReplaysOnce` — ack then `ctrl+c` against
  a blocked rg: exit 130, `warn one` exactly once after restoration,
  termios restored, `killed` reap status.
- `TestPTYQWhileSearchingReplaysDiagnostic` — the same boundary for `q`
  while searching: exit 130, replay once.
- `TestPTYQDuringGateHeldPreparationReplaysDiagnostic` — rg exited
  (reap shows `exit status 0`) while `VRG_TEST_GATE_FIFO` holds
  preparation; the acknowledged warning replays once at 130 and the
  gate-held `missing summary` never appears — no waiting on
  undelivered work.
- `TestPTYQuitAfterCompletedStreamReplaysWarning` — ack, warning
  overlay, `q` dismisses, `q` quits at 0; `warn one` replays once after
  restoration.
- `TestPTYControlledFailureReplaysAlongsideEarlierDiagnostics` — ack
  then the `VRG_TEST_FAIL_FIFO` handshake: exit 2, `warn one` and the
  `vrg:` diagnostic each exactly once across the whole captured stream
  (both mechanisms counted), in collection order.
- `TestPTYReplayEscapesEmbeddedFilename` — the absent newline/ESC file
  fails its load; the replayed `cannot read` diagnostic carries the
  single-lined `we\nir^[d.txt` form and no raw control byte.
