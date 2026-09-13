# VRG — Terminal UI for ripgrep

`vrg` runs ripgrep, collects its JSON results, and presents a file list
on the left with the current file's contents on the right, matches
highlighted. Navigate matched lines, scroll full file contents, toggle
wrapping and colour scheme, and consult modal help — all without
leaving the terminal.

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

Bare `vrg` (no arguments) and `-h`/`--help` print command-line help to
stdout and exit 0 without starting ripgrep or entering the TUI. This
command-line help is distinct from the TUI's `h`/`?` help dialog, which
opens a modal overlay inside the terminal UI.

A flags-only invocation with no pattern (for example `vrg -i`) is a
usage error: the missing pattern diagnostic is printed to stderr and
the exit status is 2.

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

VRG targets ripgrep 15.x as the reference family. It supplies
`--no-config` so ripgrep configuration files are never honoured, and
`--json` for the structured result stream.
