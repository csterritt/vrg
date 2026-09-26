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
- `TestDashFileRootAtProcessBoundary` — `./-` in a real temp dir.
- `TestHelpAssignmentSpellingsAreUsageErrorsAtBoundary` —
  `--help=false`/`-h=false`/`--help=true` are exit-2 usage errors with
  no help on stdout (Issue #2 pinned the status).
- `TestChildArgvStub` — the stub prints the exact child argv:
  `search stub: rg --json --no-config <ordered supplied flags> --
  pattern root`, including combined/expansion order, mixed aliases,
  empty and literal `-`/`--`/`-foo` patterns.
- `TestFlagAndArgvUsageErrors` — Issue #2 error classes at the boundary:
  `-e`, argument-taking options, `-uuu` and mixed-alias third `-u`,
  `=` spellings, unprotected `-foo`, and a post-terminator `--help`
  root.
- `TestHelpWithSearchFlags` — help precedence with flags present
  (`-i --help`, `-ih foo`, `-i -s --help`, `-uuu --help`, `-e --help`,
  `foo . --help`): exit 0, one help copy, empty stderr.
