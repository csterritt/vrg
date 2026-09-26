# CLI flag allow-list and child argv (Issue #2)

The flag-parsing contract delivered by
[Issue #2](../issues/002-cli-flag-allow-list-and-child-argv.md),
extending the [CLI foundation](cli-foundation.md). Relevant PRD section:
*Implementation Decisions → Invocation and child arguments* (bullets
2–5, 7) and *Module Design → CLI*.

## Shared declarations

`optionDecls` in `internal/cli/cli.go` remains the single declaration
source for three consumers — `mow.cli` configuration (`BoolOpt`),
raw-token recognition in the preflight scan, and generated help — so
parser, scanner, and help text cannot drift. Issue #2 added two fields:
`forward` (allow-listed search flag recorded for the child argv) and
`unrestricted` (occurrences count toward the cumulative two-flag cap).
The local help option is neither; it is resolved before all flag
validation and is never forwarded. There is no independently maintained
allow-list. A later README must list these flags; it synchronizes from
the same declarations.

The allow-listed no-argument flags, in declaration order:

| Short | Long | Forwarded spelling(s) |
|---|---|---|
| `-i` | `--ignore-case` | supplied form |
| `-S` | `--smart-case` | supplied form |
| `-s` | `--case-sensitive` | supplied form |
| `-w` | `--word-regexp` | supplied form |
| `-x` | `--line-regexp` | supplied form |
| `-F` | `--fixed-strings` | supplied form |
| — | `--hidden` | `--hidden` |
| — | `--no-hidden` | `--no-hidden` |
| — | `--no-ignore` | `--no-ignore` |
| `-u` | `--unrestricted` | supplied form; capped (below) |
| `-L` | `--follow` | supplied form |

Every other option — including `-e`, argument-taking options
(`--type`, `-t`, `-A`, `--context`, `--glob`, …), and unknown spellings —
is an `ErrUnsupportedOption` usage error, exit 2.

## Ordered scan and forwarding

`scanArgs` still performs the single left-to-right raw-token pass,
stopping option recognition at the first `--`. For each pre-terminator
dash token, `scanOption` calls `expandOption`:

- A token exactly equal to a declared long form (`--ignore-case`)
  forwards that literal spelling.
- A single-dash token whose bytes are all declared short letters expands
  left to right: `-iwF` records `-i`, `-w`, `-F`; `-isi` records
  `-i`, `-s`, `-i`.
- Anything else — unknown names, undeclared letters, `=` spellings —
  records the first failing token as `ErrUnsupportedOption`.

The recorded `flags` slice preserves encounter order and the exact
supplied spelling per occurrence (`-i` stays `-i`, `--ignore-case` stays
`--ignore-case`). This is why the contract must come from the scan, not
from `VarOpt`/`BoolOpt` callbacks: `mow.cli` fills option values per
container after the whole parse, so cross-option order
(`-i -s -i` vs `-isi`) is unreconstructible and callback order is
nondeterministic. The library still owns declared syntax and positional
parsing via `Run`; the scan supplies what it cannot preserve.

## Cumulative unrestricted cap

Each `-u` or `--unrestricted` occurrence — alone, in a combined token,
or across mixed aliases — increments one counter during the scan:
`-iuu` counts two. Zero to two are accepted (`-u`, `-uu`, `-iu`,
`-iuu`, `-u --unrestricted`); the third occurrence records
`ErrExcessUnrestricted` on the token containing it
(`-uuu`, `-u -uu`, `-iuuu`, `-u --unrestricted -u`,
`--unrestricted -uu`). The scan continues after recording an error so a
later help request still wins (`-uuu --help` prints help). The library
itself does not limit repetitions; the cap exists only in the scan.

## Lexical `=` rejection

The library accepts `name=value` for boolean options; the no-argument
contract does not. Assignment spellings fail `expandOption` because the
value bytes are neither a declared long name nor declared short letters —
`--ignore-case=false`, `-i=false`, `--unrestricted=false`, and the help
forms `--help=false`, `-h=false`, `--help=true`, `-h=true` are all
`ErrUnsupportedOption` (exit 2), and help assignments are never treated
as help requests. After the first `--` the same bytes are ordinary
positional operands: `vrg -- --help=false` searches for the literal
pattern. This is a deliberate Issue #1 → Issue #2 contract change:
`--help=true` previously reached the parser and produced help via the
parsed-value check; that check remains in `Parse` as a now-unreachable
defensive seam.

## Child argv

A successful `KindSearch` result carries `ChildArgv`, the exact argument
vector for the rg child (argv[0] excluded):

```
--json --no-config <ordered expanded user flags> -- <pattern> <root>
```

The mandatory internal flags come first; user flags keep encounter order
and supplied spellings without normalizing contradictions (`-i -s -S`
forwards all three); `--` protects the pattern even when it is empty
(`""` forwards as an empty argv element), `-`, dash-leading (`-foo` via
`--`), or a literal `--` (`vrg -- --` and `vrg -- -- .` make the second
`--` the pattern). Since Issue #3 the vector is no longer printed by a
stub — `internal/app` executes it verbatim as the rg child's argv; see
[search-spawn-and-searching-screen](search-spawn-and-searching-screen.md).

## Generated help

`renderHelp` iterates the same `optionDecls`, so every allow-listed flag
appears in help in both declared forms (`-i, --ignore-case`, …) without
changing the emission path or help-only precedence from Issue #1.
`TestSharedDeclarationsDriveHelpAndScan` (internal test) iterates the
table itself to prove help rendering and scan acceptance share it.

## Tests

See [unit-tests.md](unit-tests.md): `TestAcceptedSearchFlags`,
`TestFlagEncounterOrder`, `TestUnrestrictedCumulativeLimit`,
`TestRejectedOptions`, `TestAssignmentSpellingsAreUsageErrors`,
`TestAssignmentSpellingsArePositionalAfterTerminator`,
`TestTerminatorAndProtectedPatterns`, `TestGeneratedHelpListsSearchFlags`
(`internal/cli`); `TestChildArgvStub`, `TestFlagAndArgvUsageErrors`,
`TestHelpWithSearchFlags`,
`TestHelpAssignmentSpellingsAreUsageErrorsAtBoundary` (`cmd/vrg`
subprocess boundary). The Issue #1 named suites
(`TestGeneratedHelpStdout`, `TestCLIOutputSafety`,
`TestExecutableBoundary`) remain green; rows that used `-x` as the
unsupported-option example moved to `-z` because `-x` is now the
allow-listed `--line-regexp`.
