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

## internal/app

`app_test.go` (same package; Issue #3), driving `Update`/`View` and the
`Init` command directly:

- `TestSearchingScreenShownDuringCollection`,
  `TestCompletionTransitionsToSummary` — the model opens on
  "Searching…" and a completion message moves it to
  `N files, M matched lines`.
- `TestQOnSummaryExitsZero` — `q` on the summary returns `tea.Quit` with
  status 0; `TestQWhileSearchingStaysSearching` — `q` is inert while
  searching (Issue #4 owns cancellation).
- `TestResizeDuringSearching` — `WindowSizeMsg` handled without blocking.
- `TestGateHeldPreparationStaysSearching` — the gate holds index
  preparation after a completed `Wait`; the model stays searching until
  release (deterministic channel handshake, no sleeps).
- `TestRunStartFailureExit2`, `TestRunStartFailureSanitizesError` —
  `app.Run` with a failing `Start` returns 2 and writes a sanitized
  single-line diagnostic; the TUI is never entered.

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
  the search under a pty and reaches the summary.
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
  interleaved with 18 match records; vrg shows `1 files, 18 matched
  lines` (no lost stream), the `VRG_TEST_HANDSHAKE` file exists (the
  child finished writing both pipes), and `q` exits 0.
- `TestStderrCapturedWithoutBlocking` — stderr diagnostics with a
  well-formed stream and exit 0 still reach the summary.
- `TestGateHeldPreparationKeepsSearching` — with `VRG_TEST_GATE` set, the
  `VRG_TEST_COLLECT_ACK` file proves rg exited and the stream was
  collected while the screen still shows only "Searching…"; removing the
  gate file produces the summary and `q` exits 0.
