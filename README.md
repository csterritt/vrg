# vrg

`vrg` runs ripgrep and browses the results in a terminal UI: matched
files in a side list, the current file's content with its matches
highlighted, and keyboard navigation between matched lines.

vrg targets the ripgrep 15.x reference family. It always invokes
ripgrep with `--json` and `--no-config`, so ripgrep
configuration files are never honoured. The child argv is exactly
`rg --json --no-config <flags> -- <pattern> <root>`.

## Usage

    vrg [OPTIONS] PATTERN [ROOT]

`PATTERN` is the ripgrep search pattern. `ROOT` is the search root — a
directory or a regular file — and defaults to `.`.

Options may appear anywhere before `--`. The first `--` ends option
parsing: every later token is a positional operand, so a dash-leading
pattern needs the terminator (`vrg -- -foo`), while a literal pattern
`-` is valid as-is. Combined short options expand in their original
order — `-iw` means `-i -w`. All options are no-argument switches, so
`=`-assignment spellings such as `--ignore-case=false` or `-h=false`
are rejected as usage errors.

### Options

Apart from the local help options, the accepted flags are exactly this
allow-list; each is forwarded to ripgrep in encounter order with the
spelling as supplied:

| Option | Description |
|---|---|
| `-h`, `--help` | Show command-line help and exit. |
| `-i`, `--ignore-case` | Search case-insensitively. |
| `-S`, `--smart-case` | Search case-insensitively unless the pattern contains uppercase. |
| `-s`, `--case-sensitive` | Search case-sensitively. |
| `-w`, `--word-regexp` | Match the pattern against whole words only. |
| `-x`, `--line-regexp` | Match the pattern against whole lines only. |
| `-F`, `--fixed-strings` | Treat the pattern as a literal string. |
| `--hidden` | Search hidden files and directories. |
| `--no-hidden` | Do not search hidden files and directories. |
| `--no-ignore` | Do not respect ignore files. |
| `-u`, `--unrestricted` | Reduce ignore restrictions; may be given at most twice. |
| `-L`, `--follow` | Follow symbolic links while searching. |

`-u`/`--unrestricted` is counted cumulatively across all spellings —
`-uu` and `-u --unrestricted` are both two — and a third occurrence is
a usage error.

### Command-line help

With no arguments, or with `-h`/`--help` anywhere before `--`, vrg
prints command-line help to stdout with exit 0 — no search runs and
the TUI never starts, so help works even when ripgrep is unavailable.
This is separate from the TUI's help dialog: `h` or `?` while browsing
opens a scrollable key-binding overlay.

A nonempty invocation with no pattern — `vrg -i`, flags only — is a
usage error: a diagnostic and the usage text on stderr, exit 2.

### Keys

While browsing, `h` or `?` opens the help overlay listing these
bindings:

| Keys | Action |
|---|---|
| n / p | next / previous matched line |
| up / down | scroll one row |
| u / d | scroll half a page |
| pgup / pgdown | scroll a whole page |
| , / . | pan one column |
| < / > | pan ten columns |
| [ / ] | pan half the text width |
| w | toggle line wrapping |
| c | toggle colour scheme |
| left / tab | hide the file list |
| right / shift+tab | show the file list |
| r | reload the current file |
| h / ? | open or close this help |
| q / esc | quit, or dismiss the open overlay |
| ctrl+c | exit immediately with status 130 |

### Exit status

| Status | Condition |
|---|---|
| 0 | A successful search browsed to a normal quit; also command-line help (bare `vrg`, `-h`/`--help`) |
| 1 | The search completed with no usable results |
| 2 | A pre-TUI failure — a usage error (including the flags-only `vrg -i`), an invalid root, or an rg start failure — or a fatal search outcome (unexpected rg exit, signal, incomplete stream, or record loss with no usable results) |
| 130 | Cancellation: `q` while searching or preparing results, or `ctrl+c` in any state |

## Scale and resource limits

Scale examples: approximately 10,000 matched files, 100,000 matched lines, or individual files around 50 MB. The examples are independent, not simultaneous capacity guarantees.
The 50 MB file example assumes UTF-8 content with ordinary line lengths. Lines ripgrep must emit as base64 bytes expand by roughly a third, so a single match record can exceed the 64 MiB record limit even when the file itself is under 50 MB.
An oversized record is skipped and counted; the diagnostic names the path when it was parsed before the limit was reached.
Loaded buffers are retained for the session: there is no eviction, no aggregate memory bound, no reliable OOM recovery, and no guaranteed terminal cleanup under forced termination.

Aggregate match data, record expansion, decoded buffers, and
visited-file retention determine memory demand; a large search or many
visited files may exhaust memory and terminate the process.
