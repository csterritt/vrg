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

## internal/app

`model_test.go` (same package) drives `Update` directly:

- `TestSearchingScreenShownWhileCollecting` — the view is `Searching…`
  while the completion channel is silent.
- `TestResizeDuringSearching` — `WindowSizeMsg` stores dimensions and
  keeps the searching state.
- `TestCompletionTransitionsToSummary` — an injected `searchDoneMsg`
  renders `2 files, 3 matched lines`.
- `TestQOnSummaryExitsZero` — `q` on the summary returns `tea.Quit` and
  `ExitCode` 0.
- `TestGateHoldsSearchingAfterRgExit` — `Config.Drained` fires once the
  child has exited and both pipes are drained, `PrepareGate` still holds
  preparation, the view stays `Searching…`, and releasing the gate
  delivers the summary.

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
  the TUI's interim summary.
- `TestHelpAssignmentSpellingsAreUsageErrorsAtBoundary` —
  `--help=false`/`-h=false`/`--help=true` are exit-2 usage errors with
  no help on stdout (Issue #2 pinned the status).
- `TestSearchLifecycleAtBoundary` — replaces the Issue #2 stub test. A
  shell-script fake rg captures its argv and cwd, sleeps briefly so the
  harness observes `Searching…`, then emits one match. The
  `runVrgTUI` helper pipes stdin, watches stdout for the `matched line`
  marker, and sends `q`. Asserts per row: `Searching…` then
  `1 file, 1 matched line` on stdout, exact child argv (combined
  expansion, mixed aliases, empty/`-`/`--`/`-foo` patterns), exact
  working directory, empty stderr, exit 0.
- `TestDualPipeDrainageAtBoundary` — a shell fake rg floods stderr with
  16 × 64 KiB while emitting 16 valid matches; the summary shows
  `1 file, 16 matched lines`, a `writes-done` handshake file proves the
  child finished both pipes, and the child's stderr never reaches vrg's
  own stderr.
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
