# CLI search-flag allow-list and child argv (Issue #2)

The flag-forwarding contract delivered by
[Issue #2](../issues/002-cli-flag-allow-list-and-child-argv.md), extending
the [CLI foundation](cli-foundation.md) from Issue #1. Relevant PRD
section: *Implementation Decisions → Invocation and child arguments* and
*Module Design → CLI*.

## Shared declarations

`optionDecls` in `internal/cli/cli.go` remains the single declaration
source feeding three consumers: mow.cli configuration (`BoolOptPtr` per
declaration), the ordered raw-token preflight (`longDecl`/`shortDecl`
lookup and help recognition), and generated help (`spellings()` +
`desc`). There is no independently maintained allow-list — parser, scan,
and help text cannot drift, and a later README sync draws from the same
table.

Each `optionDecl` carries `short`, `long`, `desc`, and three role bits:

- `help` — the local help option: its spellings are help requests and are
  never forwarded or rejected by the allow-list.
- `search` — an allow-listed no-argument flag forwarded to the child argv.
- `unrestricted` — counts toward the cumulative two-occurrence `-u` limit.

Allow-list (both spellings shown): `-i`/`--ignore-case`,
`-S`/`--smart-case`, `-s`/`--case-sensitive`, `-w`/`--word-regexp`,
`-x`/`--line-regexp`, `-F`/`--fixed-strings`, `--hidden`, `--no-hidden`,
`--no-ignore`, `-u`/`--unrestricted`, `-L`/`--follow`. Every other option
— including `-e` and every argument-taking option — is a classified
`ErrUnsupportedOption` usage error.

## Ordered preflight scan

`scanArgs` walks argv left to right and stops option recognition at the
first `--`. Per token, in order:

1. Post-terminator tokens and the lone `-` are positionals.
2. The first `--` switches to positional mode.
3. `isHelpToken` resolves help before all search-flag validation: literal
   `-h`/`--help`, or a combined all-ASCII-letters short token containing
   `h` (`-ih`, `-xh`). Help ends the scan and wins over every error
   class, including unsupported options and a `-u` overrun.
4. Other dash tokens go through `scanOption`:
   - `--name` records the exact `--name` spelling when `name` is a
     declared search long.
   - `--name=value` is rejected lexically — except a *truthy* assignment
     to the help option (`--help=true`, `--help=1`), which passes through
     so the library parse sets the help value (the Issue #1
     `TestParsedLocalHelpValue` seam). `--help=false`, `--help=maybe`,
     and every search-flag assignment (`--ignore-case=false`,
     `-i=false`, `--unrestricted=false`, any `=true` form) are
     `ErrUnsupportedOption`.
   - A lone `-x=value` follows the same rule for the help short; any
     other short token expands letter by letter and every letter must be
     a declared search short. Expansion records one `-x` spelling per
     letter, left to right (`-isi` → `-i -s -i`); an undeclared letter or
     a stray `=` rejects the whole token. The help letter `h` is only
     reachable inside an all-letters token, which `isHelpToken` already
     claimed, so it is never forwarded.

Forwarding order, exact spellings, and the cumulative `-u` count come
**solely from these scan records**, never from mow.cli's option values:
the library fills option containers from a map after the whole parse, so
cross-option encounter order (`-i -s -i`) and repetition counts are
unreconstructible from it. The library still performs the declared-syntax
parse (`BoolOpt` values satisfy its boolean-option contract); its outputs
are deliberately unread except for the parsed help value.

## Cumulative unrestricted limit

`-u`, `--unrestricted`, and `u` letters inside combined tokens all
increment one counter from the scan records. Zero to two occurrences are
forwarded (`-u`, `-uu`, `-iu`, `-iuu`, `-u --unrestricted`); a third is
`ErrUnsupportedOption` ("may be used at most twice"), classified after
unsupported options and before arity checks (`-uuu`, `-u -uu`, `-iuuu`,
`-u --unrestricted -u`, `--unrestricted -uu`). The library itself imposes
no repetition limit, so the scan owns the boundary.

## Child argv

For `KindSearch`, `Result.ChildArgs` is exactly:

```
--json --no-config <ordered expanded user flags> -- <pattern> <root>
```

- Mandatory internal flags first; user spellings verbatim and in
  encounter order; `--` protects the operands; pattern and root (default
  `.`) verbatim — including the empty pattern as an empty element, the
  literal `-`, protected dash-leading patterns (`vrg -- -foo`), and a
  second `--` as the pattern (`vrg -- --`, `vrg -- -- .`).
- Contradictory flags are not normalized; ripgrep resolves them.
- `cmd/vrg`'s interim stub prints `search stub: argv=rg <escaped args>`
  on stdout; real rg startup and the TUI belong to later issues.

## Generated help

`renderHelp` lists every declaration's `spellings()` line, so generated
help shows all allow-listed flags in short and long form. The Issue #1
emission path, help-only precedence, and output-safety contracts are
unchanged: exactly one help copy on stdout, exit 0, empty stderr, no root
validation, child argv, stub, or TUI. `vrg -i` (flags only) stays a
missing-pattern exit-2 error.

## Tests

External (`cli_test.go`): per-spelling forwarding table, exact-argv
ordering rows (`-i -s -i`, `-isi`, `--ignore-case -s -i`, `foo -i src
-s`), combined expansion (`-iwF`), cumulative `-u` boundaries, unsupported
and argument-taking options, assignment rejection and post-`--`
positionality, operand forms (empty, `-`, dash-leading, `--`), help
regressions with flags, and flags-only missing-pattern. Internal
(`internal_test.go`): generated help iterates `optionDecls` and requires
every spelling line. Process boundary (`cmd/vrg/main_test.go`):
`TestChildArgv` pins the exact stub argv line; `TestFlagContractUsageErrors`
pins exit 2; `TestGeneratedHelpStdout` gained mixed-flag help rows. See
[unit-tests.md](unit-tests.md).
