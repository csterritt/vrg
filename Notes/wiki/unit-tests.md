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

## internal/filebuffer

`present_test.go` (same package) covers the safe-presentation core:

- `TestEscapePath` — the full path-rule table: `\n`/`\r`/`\t` to their
  two-character forms, `\\` doubling, `\xNN` for invalid UTF-8, caret
  notation for C0 and `^?` for DEL, `\uXXXX` for C1, printable Unicode
  preserved.
- `TestEscapePathNeverEmitsControls` — every C0 byte, DEL, and C1
  fixture produces no raw control byte in the output.
- `TestPresentLineText` — content rules: invalid UTF-8 → U+FFFD, C0/DEL
  caret notation, `\u0085`-style C1 forms, LF/CRLF never displayed,
  standalone CR → `^M`.
- `TestPresentLineWidth` — cell counts for escape forms and wide
  clusters.
- `TestPresentLineSpan` — byte→cell maps for escaped forms, including
  an ESC byte's match covering both `^[` cells and marker positions on
  removed terminator bytes.
- `TestPresentLineTabForm` — the provisional single `→` cell (no
  position assertions, per the Issue #16 deferral).
- `TestPresentLineRetainsRaw` — raw line bytes survive presentation.

`filebuffer_test.go` (external package `filebuffer_test`) covers
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
- `TestMatchRendersInverse` — a matched span emits the inverse-video
  SGR run.
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
  gutter with two spaces, no other panel borders.
- `TestSinkSafetyRawOutput` — the hostile fixture (OSC, CSI, C0, C1,
  DEL, standalone CR, invalid UTF-8 path bytes, embedded filename
  newline) through the real composition path under `theme.Plain`,
  asserting on the raw view string before ANSI stripping that no
  fixture control byte survives verbatim in the list, rule, or content
  sinks.

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
  patterns), exact working directory, empty stderr, exit 0.
- `TestDualPipeDrainageAtBoundary` — a shell fake rg floods stderr with
  16 × 64 KiB while emitting 16 valid matches; the browse view shows
  the file, a `writes-done` handshake file proves the child finished
  both pipes, and the child's stderr never reaches vrg's own stderr.
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
