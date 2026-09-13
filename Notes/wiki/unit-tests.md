# Unit and boundary tests

Catalog of Go tests. Conventions: table-driven, `t.Helper()` in helpers,
externally observable behavior only.

## internal/cli

`cli_test.go` (external package `cli_test`):

- `TestHelpOnlyResults` — every help-only spelling (bare, first-token,
  later-token, combined `-ih`/`-hi`/`-xh`, `-help`, flag-preceded
  `-i --help`, after unsupported/excess/invalid-root tokens) yields
  `KindHelp`, one help copy, no search fields, and never touches the
  failing root-validation sentinel.
- `TestGeneratedHelpContent` — syntax line, `PATTERN`, `ROOT` with
  `(default ".")`, `-h`/`--help`.
- `TestParsedLocalHelpValue` — the library seam: `--help=true`/`-h=true`
  spellings produce `KindHelp` without root validation; bare
  `--help=true` with no pattern stays a missing-pattern error.
- `TestHelpAssignmentSpellingsAreNotHelpRequests` — `--help=false` and
  `-h=false` are not help and (since Issue #2) are lexically rejected
  unsupported-option usage errors before root validation.
- `TestPositionalsAndRoot` — the full table: default `.`, dir/file/
  symlink roots, empty and `-` patterns, `--` arity, unsupported options,
  invalid help assignments, nonexistent/stdin/special/device roots.
- Issue #2 flag tables (`wantChildArgs`/`assertSearchArgv`/
  `assertRejectedOption` helpers):
  - `TestSearchFlagSpellingsForwarded` — every allow-listed flag, short
    and long, forwarded verbatim into `--json --no-config <flags> --
    PATTERN ROOT`.
  - `TestChildArgvPreservesEncounterOrderAndSpelling` — `-i -s -i`,
    `-isi`, `--ignore-case -s -i`, `-iwF`, options interleaved with both
    operands, mixed `-u` aliases, contradictory flags not normalized.
  - `TestCumulativeUnrestrictedBoundary` — `-u`/`-uu`/`-iu`/`-iuu`/
    `-u --unrestricted` accepted; `-uuu`/`-u -uu`/`-iuuu`/mixed third
    occurrences rejected before root validation.
  - `TestUnsupportedOptionsRejected` — `-e`, argument-taking and
    undeclared options in separate/`=`/combined spellings.
  - `TestAssignmentSpellingsRejected` — `=`-forms for search flags and
    falsy help assignments; `TestAssignmentSpellingsPositionalAfter-
    Terminator` — the same bytes are operands after `--`.
  - `TestChildArgvOperandForms` — empty pattern element, literal `-`,
    protected dash-leading patterns, literal `--` pattern in both root
    forms.
  - `TestFlagsOnlyMissingPattern` — flags without a pattern stay
    `ErrMissingPattern`.
- `TestUsageErrorsPrecedeRootValidation` — non-root errors classified
  before validation via the failing sentinel.
- `TestDashFileRoot` — `./-` names a real file.
- `TestHelpLikeTokensAfterTerminator` — `-h`, `--help`, `--` as
  operands.
- `TestUsageDiagnosticSafety` — hostile bytes escaped in diagnostics.
- `TestDiagnosticsAreSpecific` — no bare `incorrect usage`.
- Issue #6 shared sink-safety table (`TestSinkSafetyTableUsageErrorStderr`,
  `TestSinkSafetyTableUsageErrorStderrStyled`,
  `TestSinkSafetyTableHelpStdout`,
  `TestSinkSafetyTableHelpStdoutStyled`) — the shared
  `internal/sinkfixtures` fixture set driven through the usage-error
  stderr and CLI-help stdout sinks. The usage-error path asserts no
  raw control bytes survive in the diagnostic; the help path asserts no
  dangerous control bytes (tabs and newlines allowed as formatting).
  The styled path asserts the fixture payload never appears
  immediately after an unescaped ESC.

`internal_test.go` (same package): `TestAppUsesContinueOnError` pins
`flag.ContinueOnError` on the adapter's app;
`TestGeneratedHelpListsDeclaredOptions` iterates `optionDecls` and
requires every option's spellings line in `HelpText()`, so the shared
table, parser config, and generated help cannot drift.

## internal/searchindex

`searchindex_test.go` (external package `searchindex_test`):

- Event-type coverage: `begin`, `match`, `end`, `summary`, `context`
  (ignored).
- Encoding: `text` and `bytes` fields for paths and lines; mixed
  encoding equivalence (same logical value from either form); raw
  non-UTF-8 byte retention for paths and line content.
- Same-line match merging: multiple `match` records for the same file
  and line number merge into one stop with combined submatches.
- Submatch range normalization: sorting by start offset; merging
  overlapping, contained, and adjacent ranges.
- Path identity: merging uses raw path bytes, not resolved strings.
- Index ordering: files ordered by unsigned byte ordering of raw path
  bytes (non-UTF-8 paths sort after ASCII by byte value).
- Relative path resolution against the working directory without
  canonicalization.
- Schema matrix conformance, multiple files and lines, empty indexes,
  line terminator retention, reusable builders, copy semantics from
  `Stops()`.
- Issue #8 binary exclusion tests (`TestBinaryExclusionDropsMatches`,
  `TestBinaryExclusionRetainsOtherFiles`,
  `TestBinaryExclusionDistinctFileCount`,
  `TestBinaryExclusionNullOffsetRetainsMatches`,
  `TestBinaryExclusionMixedRetention`,
  `TestBinaryExclusionNoExcludedFilesForEmptyStream`,
  `TestBinaryExclusionMatchAfterEndDropped`) — a non-null `binary_offset`
  in an `end` event drops the file and all its previously collected
  matches, counts the file as a distinct excluded file, drops later
  matches for the same file, retains matches for other files, and
  reports the retained stop count as the usable-results value.
- Issue #9 lifecycle matrix tests (`lifecycle_test.go`,
  `TestLifecycleMatrix`) — table-driven coverage of every lifecycle
  transition row: duplicate `begin`, orphaned `match`, orphaned `end`,
  match after `end`, records after summary, second summary, interleaved
  open files with valid pairing, `text`/`bytes` path identity
  agreement, missing `end`, missing `summary`, trailing malformed/
  unterminated records, context records in arbitrary positions, and
  binary exclusion precedence. Each row asserts integrity completeness
  and stop count.

## internal/safepresentation

`safepresentation_test.go` (external package `safepresentation_test`):

- Path escaping: `\n`/`\r`/`\t` → backslash escapes, `\\` → `\\\\`,
  invalid UTF-8 → `\xNN`, C0 controls → caret notation, DEL → `^?`,
  C1 controls → `\u00XX`, valid printable Unicode preserved.
- Content escaping: invalid UTF-8 → U+FFFD with raw-byte mapping
  retained, C0/DEL → caret notation (`^[` for ESC), C1 → `\u00XX`,
  LF/CRLF → terminators (never displayed), standalone CR → `^M`,
  tab → `→` (single cell, no specific cell position assertions).
- Byte→cell mappings: escaped forms expose cell ranges so a match
  covering an ESC byte highlights both `^` and `[`.
- CSI input `\x1b[2J` displays as `^[[2J` (ESC → `^[`, literal `[`
  preserved).
- Issue #6 diagnostic escaping (`EscapeDiagnostic`): LF preserved as
  line boundary, CRLF normalized to LF, tabs expanded to eight-column
  stops (column resets at newline), C0/DEL → caret notation, C1 →
  `\u00XX`, invalid UTF-8 → `\xNN`, standalone CR → `^M`, backslash not
  escaped (so embedded filenames escaped through `EscapePath` are not
  double-escaped), printable Unicode preserved, empty input → empty
  output.
- Issue #6 embedded filename single-line rule: a filename with a
  newline escaped through `EscapePath` then embedded in a diagnostic
  escaped through `EscapeDiagnostic` produces no real newline and
  retains the literal `\n`.
- Issue #6 shared sink-safety table (`TestSinkSafetyTableEscapePath`,
  `TestSinkSafetyTableEscapeContent`, `TestSinkSafetyTableEscapeDiagnostic`)
  — every shared fixture from `internal/sinkfixtures` driven through
  each escaper, asserting no raw control bytes survive (except
  preserved LF in diagnostics).

## internal/filebuffer

`filebuffer_test.go` (external package `filebuffer_test`):

- Line counts, final-newline behavior, CRLF handling.
- Gutter width (digit count of largest line number + two spaces,
  minimum one digit).
- Highlight spans from `Stop.Submatches` mapped to display cells.
- Highlighting escaped ESC byte covers both cells of `^[`.
- Invalid UTF-8 and control bytes safely escaped in display.
- Nonexistent files return an error.

## internal/theme

`theme_test.go` (external package `theme_test`, Issue #7):

- Scheme tests: `New()` starts dark; `Toggle()` flips dark→light and
  light→dark; toggle has no persistence (fresh `New()` is always dark);
  `NoStyle()` reports no-style; `NoStyle().Toggle()` is a no-op.
- Base colour pair tests: dark `Base` uses white on black
  (`\x1b[37;40m`); light `Base` uses black on white (`\x1b[30;47m`);
  `Base` ends with reset.
- True-inverse match tests: dark `Match` starts with black on white
  (`\x1b[30;47m`, true inverse of dark base); light `Match` starts
  with white on black (`\x1b[37;40m`, true inverse of light base);
  `Match` restores base colours after the span.
- Current-match underline tests: dark `CurrentMatch` contains
  true-inverse colours (30;47) + underline attribute (;4m); light
  `CurrentMatch` contains true-inverse colours (37;40) + underline;
  `CurrentMatch` restores base.
- Indicator tests: dark `Indicator` uses inverse colours (30;47);
  light `Indicator` uses inverse colours (37;40).
- Underline tests: `Underline` contains SGR 4; restores base in both
  schemes.
- Gutter and file-list tests: `Gutter` and `FileList` use base colours
  in both schemes.
- Filename-rule tests: `FilenameRule` embeds the name in a horizontal
  rule; uses base colours in both schemes.
- Overlay tests: `Overlay` uses base colours in both schemes; wraps
  content with a plain single-line border (┌┐└┘─│); preserves content.
- No-style tests: every style method on `NoStyle()` returns the input
  unchanged (or plain formatting for `FilenameRule`); no ANSI escape
  sequences produced by any method.

## internal/app

`app_test.go` (external package `app_test`):

- `TestInitReturnsCommand` — `Init` returns nil without an injected
  process (test mode).
- Searching state, completion-to-summary transition, zero-result
  summary, `q` behavior (exit 0 from summary, exit 130 from searching),
  Ctrl-C behavior (exit 130), resize handling while searching.
- Start-failure state with sanitized diagnostics.
- Input/key gating (escape ignored).
- `tea.Model` interface compliance.
- Gate behavior: the searching state persists after rg exits but before
  the gate releases the index.
- Issue #4: `TestLateCompletionAfterCancellationIgnored` — late
  `SearchCompleteMsg` after `q` cancellation does not revive the UI.
- Issue #4: `TestLateCompletionAfterCtrlCIgnored` — late
  `SearchCompleteMsg` after `ctrl+c` does not revive the UI.
- Issue #4: `TestQDuringGateHeldExits130` — `q` while gate-held
  preparation is cancellation (130), not a browse quit.
- Issue #6 diagnostic tests (`TestStartFailureDiagnosticPreservesLine-
  Boundaries`, `TestStartFailureDiagnosticExpandsTabs`,
  `TestStartFailureDiagnosticEscapesControls`,
  `TestStartFailureDiagnosticSingleLinedFilename`) — verify the
  diagnostic escaper preserves LF line boundaries, expands tabs to
  eight-column stops, escapes C0/DEL with caret notation, and does not
  double-escape filenames already escaped through `EscapePath`.

`browse_test.go` (Issue #5, external package `app_test`):

- `TestBrowseViewAfterCompletion` — `SearchCompleteMsg` with an `Index`
  transitions to `StateBrowse` and shows a file list.
- `TestBrowseLoadingPlaceholder` — panel shows `Loading…` until the file
  load completes.
- `TestBrowseKeyHandledWhileLoading` — a key message while loading is
  handled without quitting or blocking.
- `TestBrowseResizeHandledWhileLoading` — a resize message while loading
  is handled.
- `TestBrowseCtrlCExits130` — `ctrl+c` while loading exits 130 through
  the Issue #4 cancellation path.
- `TestBrowseQExitsZero` — `q` in browse state exits 0.
- `TestBrowseFileLoadComplete` — `FileLoadCompleteMsg` replaces the
  loading placeholder with content.
- `TestBrowseLateLoadIgnoredAfterCancel` — a late `FileLoadCompleteMsg`
  after cancellation does not revive the UI.
- `TestBrowseFileListOrder` — file list is in raw-path order.
- `TestBrowseCurrentFileUnderlined` — current file is underlined.
- `TestBrowseFilenameRule` — filename is embedded in a horizontal rule.
- `TestBrowseGutterFormat` — gutter is right-justified with two
  trailing spaces.
- `TestBrowseGutterRightJustified` — gutter is right-justified across
  different digit counts.
- `TestBrowseNoBorders` — no box-drawing border characters around the
  panel.
- `TestBrowseInverseVideo` — matched spans rendered with the
  true-inverse match style (Issue #7: replaces Issue #5's SGR 7 reverse
  video with explicit inverse colour pairs).
- `TestBrowseInverseVideoCoversEscapedForm` — highlight over an ESC
  byte covers both cells of `^[`.
- Sink-safety tests (`TestSinkSafetyFileList`,
  `TestSinkSafetyFilenameRule`, `TestSinkSafetyPanelContent`,
  `TestSinkSafetyAllSinksHostile`) — hostile fixtures (OSC, CSI, C0, C1,
  DEL, standalone CR, invalid UTF-8, embedded newline/tab, backslash)
  driven through the real composition path via a no-style theme,
  asserting no fixture control byte survives verbatim in raw output.
- Issue #6 shared sink-safety table (`TestSinkSafetyTableFileListNoStyle`,
  `TestSinkSafetyTableFilenameRuleNoStyle`,
  `TestSinkSafetyTablePanelContentNoStyle`,
  `TestSinkSafetyTableFileListStyled`,
  `TestSinkSafetyTableFilenameRuleStyled`,
  `TestSinkSafetyTablePanelContentStyled`) — the shared
  `internal/sinkfixtures` fixture set driven through the file-list,
  filename-rule, and panel-content sinks. The no-style path asserts no
  raw control bytes survive; the styled path asserts the fixture
  payload never appears immediately after an unescaped ESC.

`browse_test.go` (Issue #7, external package `app_test`):

- `TestBrowseCToggleThemeDarkToLight` — pressing `c` in browse state
  toggles the theme from dark (white on black) to light (black on
  white), changing the composed `View()` styling.
- `TestBrowseCToggleThemeLightToDark` — pressing `c` again toggles
  back to dark.
- `TestBrowseCToggleNoPersistence` — toggling one model does not
  persist; a fresh model always starts dark.
- `TestBrowseCDoesNotQuit` — `c` does not quit or change the app
  state.
- `TestBrowseMatchTrueInverseDark` — dark scheme matches use the
  true inverse of the base colours (black on white, 30;47).
- `TestBrowseMatchTrueInverseLight` — light scheme matches use the
  true inverse of the base colours (white on black, 37;40).
- `TestBrowseCurrentMatchUnderlineDark` — dark current matched line
  highlights add underline to the true inverse (30;47;4m).
- `TestBrowseCurrentMatchUnderlineLight` — light current matched line
  highlights add underline to the true inverse (37;40;4m).

`browse_test.go` (Issue #8, external package `app_test`):

- `TestNoResultsEmptyStream` — a complete successful search with no
  matches and no binary exclusions presents the centred `No results
  found` screen (rg-1 emptiness case).
- `TestNoResultsAllBinary` — a search where every matched file is
  binary-excluded presents `No results found (N binary files skipped)`
  (rg-0 all-filtered case).
- `TestNoResultsSingleBinary` — the binary skip suffix with a count
  of 1.
- `TestNoResultsQExitsOne` — `q` from the no-results screen exits 1
  through the Issue #4 cleanup path.
- `TestNoResultsQExitsOneAllBinary` — `q` from the no-results screen
  with binary exclusions also exits 1.
- `TestNoResultsEscIsNoOp` — `Esc` from the no-results screen is a
  no-op.
- `TestNoResultsCtrlCExits130` — `ctrl+c` from the no-results screen
  exits 130.
- `TestMixedRetentionBrowses` — a mixed stream (one file excluded, one
  retained) browses with usable results of 1, not the no-results
  screen.
- `TestNoResultsDoesNotShowSearching` — the no-results view does not
  contain `Searching`.
- `TestNoResultsCancelledRejectsLateCompletion` — a late
  `SearchCompleteMsg` after cancellation from the no-results screen
  does not revive the UI.

`outcome_test.go` (Issue #9, external package `app_test`):

- `TestDecideOutcomeMatrix` — the single table-driven test covering
  every row of the Issue #9 outcome table through the pure
  `DecideOutcome` function: rg 0 clean browse, rg 1 retained results,
  rg 1 empty, rg 0 empty, fatal code with/without results, signal
  death with/without results, missing summary with matches, orphaned
  end, incomplete stream (rg 0/1), stderr warning with/without
  results, and fatal code with stderr. Each row asserts state, overlay
  kind, overlay fatality, and exit status.
- `TestDecideOutcomeGeneratedDiagnostic` — when a failed process
  supplies no stderr, the outcome generates a diagnostic naming the
  exit code or signal.
- `TestDecideOutcomeStderrDiagnostic` — when a failed process supplies
  stderr, the outcome uses it as the overlay text.
- `TestOutcomeMatrixFlow` — the single table-driven test covering
  every outcome-table row through the full `Update` flow, with
  dismissal and exit assertions: rg 0 clean browse, rg 1 retained
  results, rg 1 empty, fatal code with results (browse + overlay →
  dismiss → browse → q → 2), fatal code without results (overlay → q/Esc
  → 2), signal death with/without results, stderr warning with/without
  results. Each row asserts initial presentation, dismissal behavior,
  and final status.
- `TestFixedStatusCtrlCOverride` — `ctrl+c` after search completion
  exits 130 from browse, no-results, and open-overlay states,
  overriding the fixed status.
- `TestEscNeverExitsBaseState` — Esc never exits from a base state
  (browse or no-results without an overlay).
- `TestFixedStatusNotRecomputed` — the exit status is chosen once
  after search and later dismissal does not recompute it.

`overlay_test.go` (Issue #9, external package `app_test`):

- `TestOverlayKeyDownScrolls` / `TestOverlayKeyUpScrolls` — up/down
  scroll the overlay content.
- `TestOverlayKeyQDismisses` / `TestOverlayKeyEscDismisses` — q/Esc
  dismiss a non-fatal browse overlay (returns to browse, no quit).
- `TestOverlayKeyCtrlCExits130` — ctrl+c from an open overlay exits
  130.
- `TestOverlayKeyOtherIgnored` — any key other than up/down/q/Esc/
  ctrl+c is ignored by the overlay.
- `TestOverlayFatalNoResultsQExits2` /
  `TestOverlayFatalNoResultsEscExits2` — q/Esc on a fatal no-results
  overlay exits 2.
- `TestOverlayFatalNoResultsCtrlCExits130` — ctrl+c on a fatal
  no-results overlay exits 130 (overriding the fixed 2).
- `TestOverlayRendersDiagnostic` — the overlay view contains the
  diagnostic text.
- `TestOverlayRendersBorder` — the overlay view contains a
  single-line border (┌┐└┘─│).
- `TestOverlayBaseColors` — the overlay uses the theme base colours.
- `TestOverlayLongUnbrokenWraps` — a long unbroken diagnostic string
  wraps within the overlay border.
- `TestOverlaySinkSafetyNoStyle` / `TestOverlaySinkSafetyStyled` —
  the shared hostile-fixture set driven through the error overlay in
  no-style and styled compositions, asserting no raw control bytes
  survive and no fixture payload appears after an unescaped ESC.
- `TestOverlayDiagnosticSanitized` — hostile diagnostic content is
  sanitized through the Issue #6 utility (no raw control bytes).
- `TestOverlayPreservesSafeWrapping` — overlay rendering preserves
  safe wrapping and scrolling after a resize.
- `TestEscNeverQuitsWhenNoOverlay` — Esc never quits when no overlay
  is open, across all base states.

## cmd/vrg (subprocess boundary)

`main_test.go` builds the real binary once in `TestMain` and asserts
stdout/stderr/status separately:

- **`TestGeneratedHelpStdout`** (named group, rerun by Issue #6) — all
  help rows exit 0 with exactly one help copy, empty stderr, no stub, no
  terminal control sequences.
- **`TestCLIOutputSafety`** (named group, rerun by Issue #6) — hostile
  operand bytes escaped on stderr, no native library bytes.
- `TestHelpWithoutRipgrep`, `TestHelpDoesNotInvokeRipgrep` — help works
  with rg absent and never execs a sentinel fake rg.
- `TestHelpIgnoresExecutableName` — hostile argv0 cannot reach output.
- `TestExecutableBoundary` — search/error status table, stdin-rejection
  wording. Usage errors assert exit 2, empty stdout, a sanitized
  diagnostic first line on stderr, and exactly one generated usage block
  after it (no library `Error:`/`incorrect usage` text).
- `TestFlagContractUsageErrors` — exit-2 boundary rows: `-e`,
  argument-taking options, third cumulative `-u`, and `=` assignment
  spellings for search and help options.
- `TestDashFileRootAtProcessBoundary` — `./-` in a real temp dir, run
  with a fake rg and a PTY.
- `TestHelpAssignmentSpellingsAreNotHelp` — `--help=false`/`-h=false`
  are usage errors (status pinned by Issue #2).
- Issue #6 shared sink-safety table
  (`TestSinkSafetyTableUsageErrorProcessBoundary`,
  `TestSinkSafetyTableHelpStdoutProcessBoundary`) — the shared
  `internal/sinkfixtures` fixture set driven through the process
  boundary. The usage-error path asserts exit 2 and no dangerous
  control bytes on stderr; the help path asserts no dangerous control
  bytes on stdout.

`search_test.go` (Issue #3) uses a fake `rg` shell script and a PTY
(`github.com/creack/pty`) to drive the Bubble Tea program:

- `TestChildArgvAndWorkdir` — vrg starts rg with the exact protected
  child argv from the invocation working directory.
- `TestChildArgvWithFlags` — user flags forwarded in order to the child
  argv.
- `TestStartFailureExit2` — rg not on PATH produces a sanitized stderr
  diagnostic and exit 2 without entering the TUI.
- `TestStartFailureExplicitPath` — explicit binary path with rg-free PATH
  produces the start-failure diagnostic and exit 2.
- `TestDualPipeBackpressure` — rg writing 1 MiB to stderr interleaved
  with valid stdout records does not deadlock or lose the stdout stream.
- `TestStderrCapturedWithoutBlocking` — rg writing diagnostics to stderr
  while exiting 0 with a valid stdout stream does not block the child.

`outcome_test.go` (Issue #9) extends the fake-rg/PTY harness with fatal
exit, signal death, and large-stderr fixtures:

- `TestFatalExitWithResultsShowsOverlay` — a fake rg that emits two
  valid matches then exits non-zero with stderr shows the browse view
  with an error overlay containing the stderr text. Esc dismisses to
  browse; q exits 2.
- `TestFatalExitNoOutputNamesExitCode` — a fake rg that exits non-zero
  with no output produces an overlay naming the exit code. q exits 2.
- `TestFatalExitNoOutputEscExits2` — Esc on the fatal no-results overlay
  also exits 2.
- `TestSignalDeathNamesSignal` — a fake rg killed by SIGKILL
  mid-stream produces an overlay naming the signal. q exits 2.
- `TestStderrWarningWithSummaryShowsWarningOverlay` — a fake rg that
  writes "warn" to stderr and a summary-only stream, exit 1, shows a
  warning overlay then the no-results screen, and q exits 1.
- `TestStderrContentFixture` — a fake rg writing ≥ 1 MiB to stderr
  interleaved with a valid stdout stream produces a complete stdout
  stream, includes the captured stderr in diagnostics, and the overlay
  contains both the head and tail of the stderr text.

`cancel_test.go` (Issue #4) extends the fake-rg/PTY harness with a
controllable blocked fake rg (readiness handshake + indefinite block),
reap-evidence side channel (`VRG_TEST_REAP`), termios snapshot/restore
assertions, display-restoration sequence checks, gate injection
(`VRG_TEST_GATE`), and controlled-failure injection
(`VRG_TEST_FAIL_TRIGGER` / `VRG_TEST_FAIL_DIAGNOSTIC`):

- `TestQAgainstBlockedFakeRGExits130` — `q` while the fake rg is
  blocked exits 130, terminates and reaps the child, restores the
  display, restores PTY termios.
- `TestCtrlCAgainstBlockedFakeRGExits130` — `ctrl+c` while the fake rg
  is blocked exits 130, terminates and reaps the child, restores the
  display, restores PTY termios.
- `TestNormalExitReapsChild` — normal exit while rg is still running
  leaves no orphaned or unreaped child; reap evidence present.
- `TestQDuringGateHeldPreparationExits130` — `q` after rg has exited
  but while index preparation is gate-held exits 130 (cancellation),
  not a browse quit.
- `TestInjectedControlledFailure` — injected controlled failure after
  child readiness terminates and reaps the child, restores termios,
  writes a sanitized diagnostic exactly once after display restoration,
  exits 2.
