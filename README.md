# vrg — browse ripgrep results in the terminal

`vrg` runs ripgrep and presents the matches in a two-pane terminal UI: a
matched-file list on the left and the current file's contents on the
right, with matches highlighted and navigable.

VRG targets **ripgrep 15.x** semantics and requires `rg` on `PATH`.
Every search runs `rg --json --no-config <flags> -- <pattern> <root>`.
VRG supplies `--no-config`, so ripgrep configuration files are never honoured.

## Usage

    vrg [flags] pattern [root]

- `pattern` is required. `root` is optional and defaults to `.`; it must
  be an existing directory or regular file (a symlink to either is
  accepted). A root of `-` is rejected — standard input is not a
  searchable root; a real file named `-` is addressed as `./-`.
- Options may appear anywhere before `--`. They are forwarded to ripgrep
  in their original encounter order with the spellings supplied, and
  combined short flags expand left to right (`-iw` forwards `-i -w`).
- `--` ends option parsing: a dash-leading pattern needs it
  (`vrg -- -foo`), the literal pattern `-` is valid, and every token
  after the first `--` is positional — including a pattern spelled
  `--`.
- `-u`/`--unrestricted` may occur at most twice in total across all
  tokens and spellings (`-uu` is allowed; `-uuu` and `-u -uu` are usage
  errors). Every other option is rejected: unknown or argument-taking
  options such as `-e` or `--type`, and `=` assignment spellings such as
  `--ignore-case=false` — the allow-list is a no-argument contract.
- Bare `vrg`, or `-h`/`--help` anywhere before `--`, prints the
  command-line help below to stdout (exit 0) — without running a search
  or starting the TUI, and without validating the root or starting
  ripgrep. This command-line help is distinct from the TUI's `h`/`?`
  help dialog described under *Key bindings*.
- A nonempty invocation with no pattern — for example `vrg -i` — is a
  usage error (exit 2), as are more than two operands: a sanitized
  diagnostic plus the help text go to stderr.

`vrg --help` prints exactly:

```text
Usage: vrg [OPTIONS] PATTERN [ROOT]

Search with ripgrep and browse the results in a terminal UI.

Arguments:
  PATTERN	The ripgrep search pattern.
  ROOT	Search root: a directory or regular file. (default ".")

Options:
  -h, --help	Show command-line help and exit.
  -i, --ignore-case	Case-insensitive search.
  -S, --smart-case	Case-insensitive unless the pattern contains uppercase.
  -s, --case-sensitive	Case-sensitive search.
  -w, --word-regexp	Match whole words only.
  -x, --line-regexp	Match whole lines only.
  -F, --fixed-strings	Treat the pattern as a literal string.
  --hidden	Search hidden files and directories.
  --no-hidden	Do not search hidden files and directories.
  --no-ignore	Do not respect ignore files.
  -u, --unrestricted	Reduce ignore-file and hidden-file filtering; may be given at most twice.
  -L, --follow	Follow symbolic links.
```

## Search flags

Only the no-argument ripgrep flags listed under `Options:` in the help
above are accepted, and they are forwarded to the child in encounter
order. Everything else — including `-e`, argument-taking flags, and
assignment spellings like `--ignore-case=false` — is a usage error.

## Key bindings

These bindings apply while browsing; `h` or `?` opens the modal help
dialog listing the same table (the in-app dialog, distinct from the
command-line help above):

| Keys | Action |
| --- | --- |
| `n / p` | next / previous matched line |
| `up / down` | scroll one row |
| `u / d` | scroll half a page |
| `pgup / pgdown` | scroll one page |
| `, / .` | pan one column |
| `< / >` | pan ten columns |
| `[ / ]` | pan half the text width |
| `w` | toggle line wrapping |
| `c` | toggle colour scheme |
| `left / tab` | hide the file list |
| `right / shift+tab` | show the file list |
| `r` | reload the current file |
| `h / ?` | open or close this help |
| `q` | close the overlay / quit |
| `esc` | close the overlay |
| `ctrl+c` | exit immediately |

## Exit statuses

| Status | Meaning |
| --- | --- |
| 0 | Successful search and browse — and command-line help: bare `vrg` or `-h`/`--help` print the help to stdout and exit 0 without searching. |
| 1 | The search completed but produced no usable results — the "No results found" screen, with "(N binary files skipped)" appended when exclusion emptied it. |
| 2 | A pre-TUI failure — a usage error (including a flags-only invocation with no pattern such as `vrg -i`), an invalid root, or a failure to start ripgrep — or a fatal search outcome: ripgrep exiting other than 0/1 or dying by signal, a stream-integrity failure, or record loss that left no usable results. |
| 130 | Cancellation — `q` while searching or result preparation is incomplete, or `ctrl+c` in any state. |

## Scale, records, and memory

- Scale examples — independent, not simultaneous capacity guarantees: approximately 10,000 matched files, 100,000 matched lines, or individual files around 50 MB.
- The ~50 MB file example assumes UTF-8 content with ordinary line lengths; a line ripgrep must emit as base64 bytes expands by roughly a third, so a single match record can exceed the 64 MiB record limit — oversized records are skipped and reported, naming the file's path when recoverable.
- Loaded file buffers are retained for the whole session: no eviction, no aggregate memory bound, and no reliable OOM recovery — large searches or many visited files can exhaust memory, and forced termination cannot guarantee terminal cleanup.

The 64 MiB record limit applies per JSON record in ripgrep's output
stream, independent of the source file's size: escaping or the base64
`bytes` representation can push a single match record over it even when
the file itself is under 50 MB. Oversized records are consumed through
the next record and counted, never silently truncated, and a file whose
only match records were oversized is absent from the file list — which
is why the diagnostic names its path whenever the path was recoverable.
