# vrg — terminal UI for ripgrep

`vrg` runs [ripgrep](https://github.com/BurntSushi/ripgrep) with your
pattern, collects the results, and lets you browse the matched files
and lines in a terminal UI: a file list on the left, the current
file's contents on the right, with matches highlighted.

VRG targets ripgrep 15.x semantics. It always supplies `--no-config`
to ripgrep, so ripgrep configuration files are never honoured.

## Invocation

    vrg [flags] pattern [root]

- `pattern` — the ripgrep search pattern (required).
- `root` — a directory or regular file to search; defaults to `.`.

With no arguments, or with `-h`/`--help`, `vrg` prints command-line
help on stdout with exit 0: no search runs and the TUI never starts.
This command-line help is distinct from the modal help dialog the TUI
shows on `h` / `?` while browsing.

A nonempty invocation with flags but no pattern — for example
`vrg -i` — is a usage error: a diagnostic on stderr and exit 2.

### Flags

Only these options are accepted: the local `-h`/`--help` help options
and the no-argument ripgrep search flags below. Every other option is
a usage error. Accepted search flags are forwarded to ripgrep in the
order supplied, combined short flags expand left to right, and
`-u`/`--unrestricted` may appear at most twice. `--` ends option
parsing: tokens after it are positional operands, which is how a
dash-leading pattern is searched. The flags take no arguments —
assignment spellings such as `--ignore-case=false` or `-i=false` are
rejected.

| Flag | Meaning |
| --- | --- |
| -h, --help | Show command-line help and exit. |
| -i, --ignore-case | Case-insensitive search. |
| -S, --smart-case | Smart case search. |
| -s, --case-sensitive | Case-sensitive search. |
| -w, --word-regexp | Match whole words only. |
| -x, --line-regexp | Match whole lines only. |
| -F, --fixed-strings | Treat the pattern as a literal string. |
| --hidden | Search hidden files and directories. |
| --no-hidden | Do not search hidden files and directories. |
| --no-ignore | Do not respect ignore files. |
| -u, --unrestricted | Reduce filtering; may be supplied at most twice. |
| -L, --follow | Follow symbolic links. |

## Key bindings

Inside the TUI, `h` or `?` opens a modal help dialog listing these
bindings — rendered from the same binding table as this list:

| Keys | Action |
| --- | --- |
| n / p | next / previous matched line |
| up / down | scroll one row |
| u / d | scroll half a page |
| pgup / pgdn | scroll a full page |
| , / . | pan one column |
| < / > | pan ten columns |
| [ / ] | pan half the text width |
| left / tab | hide the file list |
| right / shift+tab | show the file list |
| w | toggle wrapping |
| c | toggle the colour scheme |
| r | reload the current file |
| h / ? | open or close this help |
| q / esc | dismiss an overlay or quit |
| ctrl+c | exit immediately |

## Exit status

| Status | When |
| --- | --- |
| 0 | The search completed with usable results and you browsed them — or command-line help was printed (no arguments, `-h`, or `--help`). |
| 1 | The search completed but nothing usable remained: "No results found". |
| 2 | A pre-TUI failure — a usage error (a missing pattern such as `vrg -i`, extra operands, an unsupported flag, or an invalid root) or ripgrep failing to start — or a fatal search outcome (a ripgrep exit code other than 0/1, death by signal, an incomplete event stream, or lost records with nothing usable left). |
| 130 | Cancelled: `q` while searching or result preparation is incomplete, or `ctrl+c` in any state. |

## Scale and memory limits

The documented scale examples are independent, not simultaneous capacity guarantees.

- about 10,000 matched files
- about 100,000 matched lines
- individual files around 50 MB

The ~50 MB file example assumes UTF-8 (or near-UTF-8) content with ordinary line lengths: a single very long line that ripgrep must emit as base64 bytes expands by roughly a third in the JSON stream and can push one match record over the 64 MiB limit even when the file itself is under 50 MB.

Every JSON record is limited to 64 MiB: an oversized record is skipped and counted, never silently truncated, and the oversized-record diagnostic names the affected file's path when it can be recovered.

Loaded file buffers are retained for the whole session: no eviction, no aggregate memory bound, and no reliable OOM recovery; large searches or many visited files may exhaust memory and terminate the process, and forced termination cannot guarantee terminal cleanup.
