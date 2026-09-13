# Issue #34: Documentation — scale examples, content assumptions, memory limits

*2026-09-12T14:58:24Z by Showboat 0.6.1*
<!-- showboat-id: 6a9c7851-1cf4-4013-b200-5f74cb007941 -->

Walkthrough for Issue #34 (Notes/tasks/034-documentation-scale-and-memory-limits.md), the documentation scale, content assumptions, and memory limits task. The README at the repository root is the single user-facing documentation artifact. It documents invocation, flags, key bindings, exit-status table, help-only behaviour, ripgrep 15.x and --no-config, and the scale/record-limit/memory statements. The help overlay footer (app.HelpFooter()) carries the same scale/record-limit/memory text as the README, sourced from the shared scaleLimitsText constant so neither sink can drift. The internal/docs synchronization tests assert the README and footer against the CLI shared declarations (cli.OptionDecls()), the Issue #31 binding table (app.KeyBindings()), and the Issue #9 outcome function (app.DecideOutcome). The internal/app help_footer_test.go adds the Issue #6 sink-safety row for the rendered help footer. References: Notes/PRD-vrg.md (Resources and responsiveness, Out of Scope, Outcome and exit-status contract), Notes/wiki/documentation-sync.md, Issue #34.

Contracts verified:

- README synchronization: every binding from Issue #31's table (app.KeyBindings()), every allow-listed flag and local help option from Issue #2's shared declarations (cli.OptionDecls()), the complete exit-status table (0, 1, 2, 130) agreeing with Issue #9's outcome function (app.DecideOutcome), the help-only exit-0 path distinct from the TUI help dialog, the flags-only usage error, the ripgrep 15.x reference family, and the --no-config statement.
- Scale examples: approximately 10,000 matched files, 100,000 matched lines, and individual files around 50 MB, explicitly independent, not simultaneous capacity guarantees; the ~50 MB example assumes UTF-8 with ordinary line lengths.
- Record limit: 64 MiB maximum JSON record payload; base64 bytes expansion can push a single match record over it; oversized records are skipped with the diagnostic naming the path when recoverable.
- Memory and termination: session-long buffer retention with no eviction, no aggregate memory bound, no reliable OOM recovery, and no guaranteed terminal cleanup under forced termination.
- Help overlay footer: carries the same scale/record-limit/memory statements as the README, sourced from the shared scaleLimitsText constant.
- Sink safety: the rendered help footer passes the Issue #6 sink-safety row (no-style raw output and styled no-payload-after-ESC).

```bash
cd /home/chris/vrg && go build ./cmd/... ./internal/... && go vet ./cmd/... ./internal/... && echo GATES-OK
```

```output
GATES-OK
```

```bash
cd /home/chris/vrg && go test -count=1 ./cmd/... ./internal/... -timeout 120s | sed 's/[[:space:]][0-9.]*s$//'
```

```output
ok  	vrg/cmd/vrg
ok  	vrg/internal/app
ok  	vrg/internal/cli
ok  	vrg/internal/docs
ok  	vrg/internal/filebuffer
ok  	vrg/internal/safepresentation
ok  	vrg/internal/searchindex
?   	vrg/internal/sinkfixtures	[no test files]
ok  	vrg/internal/theme
ok  	vrg/internal/viewport
```

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/docs/ -timeout 60s | sed 's/[[:space:]][0-9.]*s$//; s/[[:space:]]0\.[0-9]*s//g'
```

```output
=== RUN   TestREADMEContainsEveryBinding
--- PASS: TestREADMEContainsEveryBinding (0.00s)
=== RUN   TestREADMEContainsEveryAllowListedFlag
--- PASS: TestREADMEContainsEveryAllowListedFlag (0.00s)
=== RUN   TestREADMEContainsLocalHelpOptions
--- PASS: TestREADMEContainsLocalHelpOptions (0.00s)
=== RUN   TestREADMEFlagDocsFromSharedDeclarations
--- PASS: TestREADMEFlagDocsFromSharedDeclarations (0.00s)
=== RUN   TestREADMEExitStatusAgreesWithOutcomeFunction
--- PASS: TestREADMEExitStatusAgreesWithOutcomeFunction (0.00s)
=== RUN   TestREADMEDocumentsCancellationTriggers
--- PASS: TestREADMEDocumentsCancellationTriggers (0.00s)
=== RUN   TestREADMEDocumentsPreTUIFailures
--- PASS: TestREADMEDocumentsPreTUIFailures (0.00s)
=== RUN   TestREADMEDocumentsHelpOnlyPath
--- PASS: TestREADMEDocumentsHelpOnlyPath (0.00s)
=== RUN   TestREADMEDocumentsFlagsOnlyUsageError
--- PASS: TestREADMEDocumentsFlagsOnlyUsageError (0.00s)
=== RUN   TestREADMEDocumentsRipgrep15x
--- PASS: TestREADMEDocumentsRipgrep15x (0.00s)
=== RUN   TestREADMEDocumentsNoConfig
--- PASS: TestREADMEDocumentsNoConfig (0.00s)
=== RUN   TestREADMEScaleExamples
--- PASS: TestREADMEScaleExamples (0.00s)
=== RUN   TestREADMERecordLimit
--- PASS: TestREADMERecordLimit (0.00s)
=== RUN   TestREADMEMemoryLimits
--- PASS: TestREADMEMemoryLimits (0.00s)
=== RUN   TestHelpFooterCarriesSameStatements
--- PASS: TestHelpFooterCarriesSameStatements (0.00s)
=== RUN   TestREADMEAndFooterBothCarryAllScaleLimitTokens
--- PASS: TestREADMEAndFooterBothCarryAllScaleLimitTokens (0.00s)
PASS
ok  	vrg/internal/docs
```

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestHelpFooter' ./internal/app/ -timeout 60s | sed 's/[[:space:]][0-9.]*s$//; s/[[:space:]]0\.[0-9]*s//g'
```

```output
=== RUN   TestHelpFooterNonEmpty
--- PASS: TestHelpFooterNonEmpty (0.00s)
=== RUN   TestHelpFooterNoDangerousControls
--- PASS: TestHelpFooterNoDangerousControls (0.00s)
=== RUN   TestHelpFooterSinkSafetyNoStyle
--- PASS: TestHelpFooterSinkSafetyNoStyle (0.00s)
=== RUN   TestHelpFooterSinkSafetyStyled
--- PASS: TestHelpFooterSinkSafetyStyled (0.00s)
=== RUN   TestHelpFooterRenderedInOverlay
--- PASS: TestHelpFooterRenderedInOverlay (0.00s)
=== RUN   TestHelpFooterSlot
--- PASS: TestHelpFooterSlot (0.00s)
PASS
ok  	vrg/internal/app
```

## Reading the README against the implementation

The README is the single user-facing documentation artifact. The following sections read each README section against the implementation that backs it.

### Invocation section

```bash
cd /home/chris/vrg && sed -n '/^## Invocation/,/^## /p' README.md | head -20
```

````output
## Invocation

```
vrg [flags] pattern [root]
```

- `pattern` is the ripgrep search pattern (required for search).
- `root` is the search root: a directory or regular file. Defaults to
  `.`. A root of `-` (stdin) is rejected; a real file named `-` is
  addressed as `./-`.
- Options may appear anywhere before `--`. The first positional is the
  pattern and the second is the root. Missing pattern or more than two
  positionals is a usage error (exit 2).
- `--` ends option parsing. A dash-leading pattern other than the lone
  `-` requires `--`; the literal pattern `-` is valid.
- Combined short flags are expanded left to right (`-iw` → `-i -w`).
- A third cumulative `-u`/`--unrestricted` is rejected (exit 2).

## Flags
````

### Flag section

```bash
cd /home/chris/vrg && sed -n '/^## Flags/,/^## /p' README.md | head -25
```

```output
## Flags

VRG accepts only no-argument ripgrep search flags and the local help
options. Every other option (including argument-taking options and
`-e`) is a usage error. The flag list is asserted against the CLI's
shared option declarations so it cannot drift from parsing.

| Flag | Description |
|------|-------------|
| `-h`, `--help` | Show command-line help and exit. |
| `-i`, `--ignore-case` | Case insensitive search. |
| `-S`, `--smart-case` | Case insensitive unless the pattern has uppercase. |
| `-s`, `--case-sensitive` | Case sensitive search. |
| `-w`, `--word-regexp` | Match whole words only. |
| `-x`, `--line-regexp` | Match whole lines only. |
| `-F`, `--fixed-strings` | Treat the pattern as a literal string. |
| `--hidden` | Search hidden files and directories. |
| `--no-hidden` | Do not search hidden files and directories. |
| `--no-ignore` | Do not respect ignore files. |
| `-u`, `--unrestricted` | Loosen search restrictions; may be given twice. |
| `-L`, `--follow` | Follow symbolic links. |

Accepted flags are forwarded to ripgrep in their original order. The
child argv is `--json --no-config <user flags> -- <pattern> <root>`.

```

### Binding section

```bash
cd /home/chris/vrg && sed -n '/^## Key bindings/,/^## /p' README.md | head -25
```

```output
## Key bindings

Key bindings are defined once as data and consumed by both the help
overlay renderer and these documentation tests.

| Key | Action |
|-----|--------|
| n | Next match |
| p | Previous match |
| up/down | Scroll one row |
| u/d | Scroll half a page |
| PgUp/PgDn | Scroll a full page |
| ,/. | Pan one column left/right |
| </> | Pan ten columns left/right |
| [/] | Pan half the text width |
| w | Toggle wrap mode |
| c | Toggle colour scheme |
| left/right | Hide/show file list |
| Tab/Shift+Tab | Hide/show file list |
| r | Reload current file |
| h/? | Open this help |
| q | Quit |
| ctrl+c | Cancel and exit 130 |
| Esc | Dismiss overlay |

```

### Exit-status section

```bash
cd /home/chris/vrg && sed -n '/^## Exit status/,/^## /p' README.md | head -25
```

```output
## Exit status

The exit status is fixed once searching completes and never changes
except that `ctrl+c` still overrides it with 130.

| Status | Meaning |
|--------|---------|
| 0 | Successful search and browse (ripgrep exit 0 or 1, complete stream, usable results). Also exit 0 for command-line help (see below). |
| 1 | No results (ripgrep exit 0 or 1, complete stream, no usable results, no fatal record loss). |
| 2 | Fatal search outcome (ripgrep exits other than 0/1, dies by signal, or stream integrity fails). Also exit 2 for usage errors (missing pattern, unsupported flag, excess operands), root validation failure, and process start failure. Record loss with no usable results also exits 2. |
| 130 | Cancellation: `ctrl+c` in any state, or `q` while searching or result preparation is incomplete. |

The search-derived exit statuses (0, 1, 2) agree with the outcome
function: a complete stream with usable results exits 0; a complete
stream with no usable results and no fatal record loss exits 1; a
fatal process exit, signal death, incomplete stream, or record loss
with no usable results exits 2.

## Help
```

### Help section

```bash
cd /home/chris/vrg && sed -n '/^## Help/,/^## /p' README.md | head -15
```

```output
## Help

Bare `vrg` (no arguments) and `-h`/`--help` print command-line help to
stdout and exit 0 without starting ripgrep or entering the TUI. This
command-line help is distinct from the TUI's `h`/`?` help dialog, which
opens a modal overlay inside the terminal UI.

A flags-only invocation with no pattern (for example `vrg -i`) is a
usage error: the missing pattern diagnostic is printed to stderr and
the exit status is 2.

## Scale, record limits, and memory
```

### Scale, record-limit, and memory section

```bash
cd /home/chris/vrg && sed -n '/^## Scale, record limits, and memory/,/^## /p' README.md | head -25
```

```output
## Scale, record limits, and memory

Scale examples (independent, not simultaneous capacity guarantees):
~10,000 matched files, ~100,000 matched lines, individual files ~50 MB.
The ~50 MB example assumes UTF-8 with ordinary line lengths; base64
bytes expansion can push a single match record over the 64 MiB limit;
oversized records are skipped, and the diagnostic names the path when
recoverable.
Buffers are retained for the session with no eviction and no
aggregate memory bound. No reliable OOM recovery or forced termination
cleanup is guaranteed.

The 64 MiB JSON record limit is independent of source-file size:
escaping or base64 can exceed it even for a source file under 50 MB.
Oversized records use the explicit skip/count and integrity rules, not
silent truncation.

## Ripgrep
```

### Ripgrep section

```bash
cd /home/chris/vrg && sed -n '/^## Ripgrep/,$p' README.md
```

```output
## Ripgrep

VRG targets ripgrep 15.x as the reference family. It supplies
`--no-config` so ripgrep configuration files are never honoured, and
`--json` for the structured result stream.
```

## Help footer note rendering in the overlay

The help overlay footer is rendered at the bottom of the help overlay. The footer text is sourced from the shared scaleLimitsText constant, the same text as the README's scale section. The following test demonstrates the footer rendering in the overlay at 80x50 (tall enough to show the footer).

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestHelpFooterRenderedInOverlay' ./internal/app/ -timeout 60s | sed 's/[[:space:]][0-9.]*s$//; s/[[:space:]]0\.[0-9]*s//g'
```

```output
=== RUN   TestHelpFooterRenderedInOverlay
--- PASS: TestHelpFooterRenderedInOverlay (0.00s)
PASS
ok  	vrg/internal/app
```

## Help-footer sink-safety row against Issue #6 hostile fixtures

The Issue #6 sink-safety row for the rendered help footer substitutes the shared hostile fixtures from internal/sinkfixtures at every runtime-substitution point. The footer is fixed app-authored text (no runtime-string substitution points), so the row verifies the rendered output is safe and guards against future runtime-string additions. The no-style composition path asserts no dangerous control bytes survive in raw output before any ANSI stripping; the styled path asserts no fixture payload appears after an unescaped ESC.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestHelpFooterSinkSafety' ./internal/app/ -timeout 60s | sed 's/[[:space:]][0-9.]*s$//; s/[[:space:]]0\.[0-9]*s//g'
```

```output
=== RUN   TestHelpFooterSinkSafetyNoStyle
--- PASS: TestHelpFooterSinkSafetyNoStyle (0.00s)
=== RUN   TestHelpFooterSinkSafetyStyled
--- PASS: TestHelpFooterSinkSafetyStyled (0.00s)
PASS
ok  	vrg/internal/app
```

## Final verification

The coding standard requires final verification with gofmt, go vet, go test, and go build.

```bash
cd /home/chris/vrg && gofmt -l internal/docs/docs_test.go internal/app/help_footer_test.go internal/app/app.go && echo GOFMT-OK
```

```output
GOFMT-OK
```

```bash
cd /home/chris/vrg && go vet ./cmd/... ./internal/... && echo VET-OK
```

```output
VET-OK
```

```bash
cd /home/chris/vrg && go test -count=1 ./cmd/... ./internal/... -timeout 120s | sed 's/[[:space:]][0-9.]*s$//'
```

```output
ok  	vrg/cmd/vrg
ok  	vrg/internal/app
ok  	vrg/internal/cli
ok  	vrg/internal/docs
ok  	vrg/internal/filebuffer
ok  	vrg/internal/safepresentation
ok  	vrg/internal/searchindex
?   	vrg/internal/sinkfixtures	[no test files]
ok  	vrg/internal/theme
ok  	vrg/internal/viewport
```

```bash
cd /home/chris/vrg && go build ./cmd/... ./internal/... && echo BUILD-OK
```

```output
BUILD-OK
```
