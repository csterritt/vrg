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
