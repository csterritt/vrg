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
  values all classify `KindMalformed` and contribute nothing (counting
  lands with Issue #10).
- `TestSubmatchRangeBoundaries` — ranges may touch the ends of the
  decoded line; zero-width matches are in range.

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
  prepared buffer; content rows render with gutter and filename rule.
- `TestCurrentFileListEntryUnderlined`, `TestGutterRightJustified` —
  list order by raw path with the current entry underlined; the
  right-justified gutter plus two spaces.
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
sink-safety table `sinkSafetySinks` — five rows over every sink
existing at this point (file-list entry, filename rule, panel content,
usage-error stderr, CLI-help stdout) — and `TestSinkSafetyTable` runs
`sinktest.Run` over it: each `<sink>/<fixture>` subtest asserts clean
raw output on the no-style path and, for the styled TUI rows, that no
fixture payload follows an unescaped ESC. Each row also proves the
fixture reached the sink (escaped name in the list/rule region, mapped
content forms in the panel, `EscapePath` operand in the stderr block);
the help row injects the hostile operand into argv while rendering
help, which has no substitution points.

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
  interleaved with 18 match records; the final content frame shows all
  18 recorded matches as inverse-video spans (no lost stream), the
  `VRG_TEST_HANDSHAKE` file exists (the child finished writing both
  pipes), and `q` exits 0.
- `TestStderrCapturedWithoutBlocking` — stderr diagnostics with a
  well-formed stream and exit 0 still reach the browse file list.
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
