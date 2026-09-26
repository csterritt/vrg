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
  standalone CR → `^M`.
- `TestLineWidth` — cell counts for escape forms and wide clusters.
- `TestLineSpan` — byte→cell maps for escaped forms, including
  an ESC byte's match covering both `^[` cells and marker positions on
  removed terminator bytes.
- `TestLineTabForm` — the provisional single `→` cell (no
  position assertions, per the Issue #16 deferral).
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
  border. Under `theme.Plain` (the no-style composition path) the raw
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
