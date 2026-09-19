# CLI flag allow-list and child argv (Issue #2)

The search-flag forwarding and protected child-argv contract delivered by
[Issue #2](../issues/002-cli-flag-allow-list-and-child-argv.md), built on
the [CLI foundation](cli-foundation.md) from Issue #1. Relevant PRD
sections: *Implementation Decisions → Invocation and child arguments*
(bullets 2–5, 7) and *Module Design → CLI* in
[`Notes/PRD-vrg.md`](../PRD-vrg.md).

## The allow-list

Exactly these no-argument ripgrep flags are accepted before `--` and
forwarded; every other option — including `-e`, argument-taking options
such as `--type`, and undeclared combined letters such as `-foo` — is an
`ErrUnsupportedOption` usage error (exit 2, sanitized diagnostic, no
child, no TUI):

| Short | Long | Notes |
|---|---|---|
| `-i` | `--ignore-case` | |
| `-S` | `--smart-case` | |
| `-s` | `--case-sensitive` | |
| `-w` | `--word-regexp` | |
| `-x` | `--line-regexp` | |
| `-F` | `--fixed-strings` | |
| | `--hidden` | long-only |
| | `--no-hidden` | long-only |
| | `--no-ignore` | long-only |
| `-u` | `--unrestricted` | cumulative cap: at most two |
| `-L` | `--follow` | |

Local `-h`/`--help` are **not** search flags: their spellings are help
requests anywhere before `--` (including inside combined tokens like
`-ih`), are never forwarded, and are never allow-list-rejected.

## Shared declarations

`optionDecls` in `internal/cli/cli.go` remains the single declaration
source, now listing help plus all eleven flags. It feeds four consumers:
`mow.cli` parser configuration (`BoolOpt`/`BoolOptPtr`), raw-token
recognition in the preflight scan, ordered search-flag records for the
child argv, and generated help — and later README synchronization reads
from the same source. There is no independently maintained allow-list, so
parser, scan, and help cannot drift.

## Ordered scan records, not callbacks

The Issue #1 `scanArgs` preflight was extended, not replaced. Scanning
argv left to right up to the first `--`, it now records each accepted
spelling in `preflight.flags` in encounter order: long tokens keep the
supplied spelling verbatim (`--ignore-case`); combined short tokens expand
left to right into canonical single-letter spellings (`-isi` → `-i -s
-i`, `-iwF` → `-i -w -F`).

Order and counts are never derived from `VarOpt`/value callbacks: mow.cli
fills option values per container from a map after the whole parse, so
cross-option encounter order (`-i -s -i`) is unreconstructible from them
and callback order is nondeterministic. The library still owns the
declared syntax and parsing (`BoolOpt` satisfies its boolean-option
contract); the scan supplies what it provably cannot preserve. Parsed flag
values are deliberately never consulted — only `PATTERN`/`ROOT` are read
back from the library.

## Cumulative unrestricted boundary

`-u`/`--unrestricted` occurrences accumulate per occurrence across all
spellings — separate tokens (`-u -u`), combined tokens (`-iu`, `-iuu`),
and mixed aliases (`-u --unrestricted`) share one count, recorded while
the scan writes its records. Two pass; the third claims the usage-error
slot (`ErrExcessUnrestricted`, exit 2, no root validation, no child argv):
`-uuu`, `-u -uu`, `-iuuu`, `-u --unrestricted -u`, `--unrestricted -uu`.

## Assignment rejection

The allow-list is a no-argument contract, so the scan rejects every `=`
spelling lexically — `--ignore-case=false`, `-i=false`,
`--unrestricted=false`, and help forms `--help=false`, `-h=false`,
`--help=true`, `-h=true` — before the library's permissive boolean
assignment parsing can broaden the CLI. (This supersedes Issue #1's
`--help=true` → help-via-parsed-value seam: no `=` token reaches the
library now, and the post-parse help-value check remains only as a safety
net.) After the first `--` the same bytes are ordinary operands when arity
permits: `vrg -- --ignore-case=false` parses with that verbatim pattern.

## Option placement and `--`

Options may appear before, between, or after both operands (`vrg foo -i
src -s` forwards `-i -s` with root `src`). The first `--` ends option
recognition; later tokens are positional even when dash-leading —
including a second `--` used as the verbatim pattern (`vrg -- --`, `vrg --
-- .`). The literal pattern `-` needs no terminator; other dash-leading
patterns do (`vrg -- -foo`). The empty pattern is a real operand forwarded
as an empty argv element.

## Child argv

For `KindSearch`, `Result.ChildArgs` is the exact protected ripgrep
argument vector (the `rg` program name itself is the caller's):

```
--json --no-config <ordered expanded user flags> -- <pattern> <root>
```

Contradictory flags are forwarded without normalization — ripgrep
resolves interactions. The `cmd/vrg` stub prints it as
`search stub: argv=rg …` until Issue #3 wires the real child spawn.

## Help precedence (unchanged from Issue #1)

Help resolution precedes all allow-list validation, so `-e --help`,
`-uuu --help`, and `--help=false --help` all yield the help-only result:
exit 0, exactly one generated help copy on stdout, empty stderr, no root
validation, child, or TUI. `vrg -i` without help stays the exit-2
missing-pattern error. Generated help now lists every allow-listed flag
in both forms, rendered from `optionDecls` through the same Issue #1
emission path.

## Tests

See [unit-tests.md](unit-tests.md): `TestSearchFlagsForwarded`,
`TestOrderedChildArgs`, `TestUnrestrictedLimit`,
`TestOptionAssignmentFormsRejected`,
`TestAssignmentSpellingsAfterTerminator`, `TestFlagsOnlyMissingPattern`,
`TestGeneratedHelpListsSearchFlags`, `TestHelpRendersEveryDeclaredOption`,
`TestScanRecordsEveryDeclaredFlag`, `TestChildArgvBoundary`, plus the
extended Issue #1 help/error tables.
