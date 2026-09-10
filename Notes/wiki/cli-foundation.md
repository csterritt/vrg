# CLI foundation (Issue #1)

The command-line foundation delivered by
[Issue #1](../issues/001-go-scaffold-cli-positionals-and-root.md) and
recorded in
[decision 001](../decisions/001-cli-scaffold-and-output-architecture.md).
Relevant PRD sections: *Invocation and child arguments*, *Module Design →
CLI*, *Outcome and exit-status contract*, *Testing Decisions → CLI*.

## Adapter boundary

`internal/cli` owns all `github.com/jawher/mow.cli` v1.2.0 contact.
`cli.Parse(args, out, env)` returns a `cli.Result` with an explicit
`Kind`: `KindHelp`, `KindSearch`, or `KindUsageError`. Callers never see
library types. `cmd/vrg` is a thin boundary that maps kinds to streams and
statuses: help → stdout, exit 0; search → escaped child-argv stub
(`search stub: argv=rg …`), exit 0; usage error → sanitized single-line
diagnostic **followed by the generated usage block** (`cli.HelpText()`)
on stderr, exit 2 — matching standard mow.cli behavior of printing usage
after an error, with only module text on the stream. Issue #2 added the
search-flag allow-list and the child-argv contract; see
[cli-flag-forwarding.md](cli-flag-forwarding.md).

The library application is built with the fixed name `"vrg"` (never
`os.Args[0]`) and `ErrorHandling = flag.ContinueOnError`, so `Run` returns
results instead of calling `os.Exit`; the entry point owns every exit
status.

## Native-output strategy: emission prevention

The library writes help and parse diagnostics to an unexported
package-level writer bound to `os.Stderr` at package init — there is no
injection point. Rather than capturing the process file descriptor around
every call, the module prevents every emission:

- The ordered raw-token **preflight** scans argv left to right up to the
  first `--` and resolves all help requests (`-h`, `--help`, combined
  ASCII-letter shorts containing `h`) before `Run` is invoked, so the
  library's first-token `PrintLongHelp` interception is unreachable.
- The same scan validates every token the library parser could reject —
  unsupported options, invalid `-h=`/`--help=` boolean assignments,
  missing `PATTERN`, excess positionals — so the FSM-failure `Error:` +
  `PrintHelp` path is unreachable too.
- Generated help is rendered from the shared declarations, so the module
  never calls `PrintHelp`/`PrintLongHelp` itself.
- `app.Action` is always set (a nil `Action` would make the library print
  help), and a parsed help value (`--help=true` assignment spellings)
  yields the help-only result before root validation or search work.

## Shared declarations and preflight

`optionDecls` and `argDecls` are one table feeding three consumers:
mow.cli configuration (`Spec`, `BoolOptPtr`, `StringArgPtr`), raw-token
recognition, and generated help — so the parser, the scan, and the help
text cannot drift. Issue #2 extended the same table and the same scan to
record allow-listed search-flag spellings in encounter order for child
argv, instead of maintaining a separate allow-list; see
[cli-flag-forwarding.md](cli-flag-forwarding.md).

## Help contract

- Bare `vrg`, `-h`/`--help` in any pre-`--` position, and combined shorts
  containing `h` (`-ih`, `-hi`) produce the help-only result: one help
  copy on stdout, exit 0, empty stderr, no root validation, no child, no
  TUI, no stub — even when rg is absent.
- Help wins over missing-pattern, excess-operand, unsupported-option, and
  invalid-root conditions.
- Help content: `Usage: vrg [OPTIONS] PATTERN [ROOT]`, the required
  pattern, the optional root with `(default ".")`, and every declared
  option — `-h, --help` plus the Issue #2 allow-listed search flags in
  short and long form. This is command-line help, distinct from the TUI
  key-binding overlay (Issue #31).
- Assignment spellings are resolved lexically: `--help=true`/`-h=true`
  pass the scan, parse into the help value, and still produce help-only;
  falsy or invalid help assignments (`--help=false`, `-h=false`,
  `--help=maybe`) are `ErrUnsupportedOption` usage errors under Issue
  #2's no-argument contract.

## Positionals, `--`, and root validation

- First positional is the pattern, second is the root (default `.`).
  Options may sit before, between, or after positionals.
- The first `--` ends option recognition; `-h`, `--help`, and another `--`
  after it are plain operands. Lone `-` is a positional, not an option.
- Outside help-only paths: missing pattern, more than two positionals, or
  an unsupported option is a classified usage error (`ErrMissingPattern`,
  `ErrExcessOperand`, `ErrUnsupportedOption`) — never the library's bare
  `incorrect usage`.
- Root must stat to a directory or regular file; symlinks resolve through
  `os.Stat`. `-` (stdin), special files, and nonexistent paths are
  `ErrInvalidRoot`; `./-` names a real file. A present-but-empty pattern
  and a literal `-` pattern are valid.

## Sanitization

`cli.Escape` is the interim single-line escaper (Issue #6 generalizes):
`\\` doubles, `\n`/`\r`/`\t` become escape spellings, other C0 controls
and DEL use caret notation (`ESC` → `^[`), C1 controls use `\u` escapes,
invalid UTF-8 bytes use `\xNN`. It covers usage diagnostics and the
success stub; generated help contains only fixed text.

## Tests consumed by later tasks

The named groups `TestGeneratedHelpStdout` and `TestCLIOutputSafety` in
`cmd/vrg/main_test.go`, plus the `internal/cli` suites, are rerun
unchanged by Issue #6's tasks; Issue #2 and the final-verification tasks
(#34, #35) build on the subprocess sentinels. See
[unit-tests.md](unit-tests.md).
