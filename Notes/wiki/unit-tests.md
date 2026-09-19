# Unit and boundary tests

Catalog of Go tests. Conventions: table-driven, `t.Helper()` in helpers,
externally observable behavior only. Child-argv assertions compare the
public `Result.ChildArgs` vector (or the printed stub line), never library
callback order.

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

## cmd/vrg (subprocess boundary)

`main_test.go` builds the real binary once in `TestMain` and asserts
stdout/stderr/status separately:

- **`TestGeneratedHelpStdout`** (named group, rerun by Issue #6) — all
  help rows exit 0 with exactly one help copy, empty stderr, no stub, no
  terminal control sequences; now includes search-flag help mixes
  (`-iw --help`, `-ih foo`) and help beating the `-u` limit.
- **`TestCLIOutputSafety`** (named group, rerun by Issue #6) — hostile
  operand bytes escaped on stderr and in the stub's argv elements, no
  native library bytes.
- `TestChildArgvBoundary` — the stub's exact `search stub: argv=rg …`
  line for every flag class: combined/repeated/mixed/interleaved flags,
  empty pattern, literal `-`, protected `-foo`, literal `--` pattern,
  mixed-alias `-u` pair.
- `TestHelpWithoutRipgrep`, `TestHelpDoesNotInvokeRipgrep` — help works
  with rg absent and never execs a sentinel fake rg.
- `TestHelpIgnoresExecutableName` — hostile argv0 cannot reach output.
- `TestExecutableBoundary` — search/error status table including
  unsupported `-e`/`--type`, third `-u` variants, assignment rejections,
  and flags-only missing pattern; asserts the exact default-root stub
  line `search stub: argv=rg --json --no-config -- foo .`. Usage errors
  assert exit 2, empty stdout, a sanitized diagnostic first line on
  stderr, and exactly one generated usage block after it (no library
  `Error:`/`incorrect usage` text).
- `TestDashFileRootAtProcessBoundary` — `./-` in a real temp dir.
- `TestHelpAssignmentSpellingsAreNotHelp` — `--help=`/`-h=` spellings
  (`=false` and `=true`) are exit-2 usage errors.
