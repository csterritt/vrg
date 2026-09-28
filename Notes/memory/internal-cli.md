---
uuid: 1f1bb46f-6618-64de-8074-8f584decf711
created: '2026-09-28T14:14:48Z'
updated: '2026-09-28T14:14:48Z'
title: internal/cli package
summary: 'The implemented CLI module: Parse contract, result kinds, preflight token
  scan, generated help, and the Escape sanitizer.'
---
# internal/cli package

Owns command-line parsing and generated help for vrg (Issue 1). Parsing is
delegated to `github.com/jawher/mow.cli` behind the package's own interface —
callers see only explicit result kinds, never library types.

## Public contract

- `Parse(args []string, out io.Writer, env Env) Result` — classifies argv.
  Writes generated help to `out` only for `KindHelp`; writes nothing for other
  kinds. All stream/exit-status decisions belong to the caller.
- `Result{Kind, Pattern, Root, ErrorKind, Diagnostic}` — `Pattern`/`Root` set
  for `KindSearch` (Root defaults to `"."`); `ErrorKind`/`Diagnostic` set for
  `KindUsageError` (single sanitized line prefixed `vrg: `).
- Kinds: `KindHelp`, `KindSearch`, `KindUsageError`.
- Error kinds: `ErrMissingPattern`, `ErrExcessOperand`,
  `ErrUnsupportedOption`, `ErrInvalidRoot`.
- `Env{Stat}` injects the root validator; nil means `os.Stat`. Tests inject a
  failing `Stat` to prove validation never runs on help/non-root paths.
- `HelpText()` — the generated help; the entry point appends it to usage
  diagnostics on stderr.
- `Escape(s)` — makes external bytes safe for one-line output (see below).

## Single declaration source

`optionDecls` and `argDecls` are the one table feeding three consumers: the
mow.cli spec (`spec()`/`name()`), raw-token recognition in the preflight scan,
and generated help (`renderHelp`). Issue 2 adds the search flag allow-list to
this table.

## Preflight scan

`scanArgs` walks argv left to right before the library runs, so no library
emission path (first-token help, parse-failure diagnostics) can ever fire:

- `--` ends option recognition; later tokens are all positional.
- A help request (`isHelpToken`) wins over every other classification and ends
  the scan: literal `-h`/`--help`, or a combined short token of ASCII letters
  containing `h` (e.g. `-ih`). Assignment spellings like `--help=false` are
  not help requests.
- First unsupported option token is recorded → `ErrUnsupportedOption`.
- Only boolean assignments to declared boolean options (`-h=true`) count as
  supported options (`isSupportedOption`/`validBoolAssignment`).

After preflight, `app.Run` can neither emit nor error; a `helpOpt` set via
assignment still yields `KindHelp` before root validation.

## Root validation

`checkRoot` accepts existing directories and regular files (symlinks
resolving to either), rejects `-` (stdin — diagnostic names "standard
input") and non-dir/non-regular files such as `/dev/null`.

## Library containment

`newApp` uses `flag.ContinueOnError` so mow.cli returns errors instead of
calling `os.Exit` (pinned by `internal_test.go`). `appName` is the constant
`"vrg"` so `os.Args[0]` can never reach generated output. `app.Action` must be
non-nil or the library prints its own help.

## Escape

`Escape` is the minimal Issue-1 sanitizer (Issue 6 generalizes it): backslash
doubles; `\n` `\r` `\t` mnemonics; other C0 and DEL as caret notation (`^[`,
`^?`); C1 controls as `\u00NN`; invalid UTF-8 bytes as `\xNN`; printable text
passes through.

Sources: `/home/chris/vrg/internal/cli/cli.go`,
`/home/chris/vrg/internal/cli/cli_test.go`,
`/home/chris/vrg/internal/cli/internal_test.go`
