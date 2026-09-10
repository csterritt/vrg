# Issue #1: Go scaffold, mow.cli help, positionals, and root validation

*2026-09-10T16:23:58Z by Showboat 0.6.1*
<!-- showboat-id: ea7b0905-7b5d-44a1-88b5-482dacbe1133 -->

Walkthrough for Issue #1 (`Notes/issues/001-go-scaffold-cli-positionals-and-root.md`), implementing the CLI foundation specified in `Notes/PRD-vrg.md` (Invocation and child arguments; Module Design → CLI; Testing Decisions → CLI) under the architecture approved in `Notes/decisions/001-cli-scaffold-and-output-architecture.md`: module `vrg`, six internal packages, `mow.cli v1.2.0` behind an adapter, `flag.ContinueOnError`, and the emission-prevention native-output strategy (metadata-rendered help + ordered raw-token preflight making every library emission point unreachable). All generated artifacts live in this directory: the built `vrg` binary and the `fixtures/` tree.

```bash
cd /home/chris/vrg && go mod verify && go build ./... && go vet ./... && echo GATES-OK
```

```output
all modules verified
GATES-OK
```

```bash
cd /home/chris/vrg && go test -count=1 ./internal/cli/ ./cmd/vrg/ | sed 's/[[:space:]][0-9.]*s$//'
```

```output
ok  	vrg/internal/cli
ok  	vrg/cmd/vrg
```

```bash
cd /home/chris/vrg && go build -o Notes/walkthroughs/001-06/code-walkthrough/vrg ./cmd/vrg && file Notes/walkthroughs/001-06/code-walkthrough/vrg | cut -d: -f2 | cut -c1-60
```

```output
 ELF 64-bit LSB executable, ARM aarch64, version 1 (SYSV), s
```

## Help-only paths: bare, first-token, later-token, combined, and precedence over errors. Each run captures stdout and stderr separately; every row must show exit 0, exactly one `Usage:` line, zero stderr bytes, and no stub output or terminal control bytes.

```bash
cd /home/chris/vrg/Notes/walkthroughs/001-06/code-walkthrough
check() { ./vrg "$@" >o.txt 2>e.txt; c=$?; u=$(grep -c "^Usage:" o.txt); s=$(wc -c <e.txt); st=$(grep -c "search stub:" o.txt); esc=$(cat o.txt e.txt | tr -d -c "\033" | wc -c); printf "%-36s exit=%d usage_lines=%d stderr_bytes=%d stub_lines=%d esc_bytes=%d\n" "vrg $*" "$c" "$u" "$s" "$st" "$esc"; }
check
check -h
check --help
check foo --help /nonexistent
check foo /nonexistent --help
check -h foo bar baz
check foo bar baz --help
check -i --help
check -ih
check --unsupported --help
```

```output
vrg                                  exit=0 usage_lines=1 stderr_bytes=0 stub_lines=0 esc_bytes=0
vrg -h                               exit=0 usage_lines=1 stderr_bytes=0 stub_lines=0 esc_bytes=0
vrg --help                           exit=0 usage_lines=1 stderr_bytes=0 stub_lines=0 esc_bytes=0
vrg foo --help /nonexistent          exit=0 usage_lines=1 stderr_bytes=0 stub_lines=0 esc_bytes=0
vrg foo /nonexistent --help          exit=0 usage_lines=1 stderr_bytes=0 stub_lines=0 esc_bytes=0
vrg -h foo bar baz                   exit=0 usage_lines=1 stderr_bytes=0 stub_lines=0 esc_bytes=0
vrg foo bar baz --help               exit=0 usage_lines=1 stderr_bytes=0 stub_lines=0 esc_bytes=0
vrg -i --help                        exit=0 usage_lines=1 stderr_bytes=0 stub_lines=0 esc_bytes=0
vrg -ih                              exit=0 usage_lines=1 stderr_bytes=0 stub_lines=0 esc_bytes=0
vrg --unsupported --help             exit=0 usage_lines=1 stderr_bytes=0 stub_lines=0 esc_bytes=0
```

```bash
cd /home/chris/vrg/Notes/walkthroughs/001-06/code-walkthrough && ./vrg --help; echo "exit=$?"
```

```output
Usage: vrg [OPTIONS] PATTERN [ROOT]

Search with ripgrep and browse the results in a terminal UI.

Arguments:
  PATTERN	The ripgrep search pattern.
  ROOT	Search root: a directory or regular file. (default ".")

Options:
  -h, --help	Show command-line help and exit.
exit=0
```

```bash
cd /home/chris/vrg/Notes/walkthroughs/001-06/code-walkthrough
mkdir -p nopath fakebin && printf "#!/bin/sh\n: > \"$(pwd)/fakebin/rg-ran\"\n" > fakebin/rg && chmod +x fakebin/rg
env -i PATH="$PWD/nopath" ./vrg -h >o.txt 2>e.txt; echo "no-rg exit=$? usage=$(grep -c "^Usage:" o.txt) stderr_bytes=$(wc -c <e.txt)"
env -i PATH="$PWD/fakebin" ./vrg --help >o.txt 2>e.txt; echo "fake-rg exit=$? usage=$(grep -c "^Usage:" o.txt) stderr_bytes=$(wc -c <e.txt)"
test ! -e fakebin/rg-ran && echo "fake rg was not invoked"
```

```output
no-rg exit=0 usage=1 stderr_bytes=0
fake-rg exit=0 usage=1 stderr_bytes=0
fake rg was not invoked
```

Help-only paths work with ripgrep absent and never execute a sentinel `rg` on PATH; the full generated help (syntax, required `PATTERN`, optional `ROOT` with `." ` default, `-h`/`--help`) is rendered from the shared declarations — not by the library — and goes to stdout only. Next: classified usage errors, each a sanitized diagnostic line followed by the generated usage block on stderr, with exit 2 and nothing on stdout.

```bash
cd /home/chris/vrg/Notes/walkthroughs/001-06/code-walkthrough
err() { ./vrg "$@" >o.txt 2>e.txt; c=$?; printf "%-24s exit=%d stdout_bytes=%d\n" "vrg $*" "$c" "$(wc -c <o.txt)"; sed "s/^/  stderr: /" e.txt; }
err --
err a b c
err --unsupported
err -i
err foo -x
```

```output
vrg --                   exit=2 stdout_bytes=0
  stderr: vrg: missing required argument PATTERN
  stderr: 
  stderr: Usage: vrg [OPTIONS] PATTERN [ROOT]
  stderr: 
  stderr: Search with ripgrep and browse the results in a terminal UI.
  stderr: 
  stderr: Arguments:
  stderr:   PATTERN	The ripgrep search pattern.
  stderr:   ROOT	Search root: a directory or regular file. (default ".")
  stderr: 
  stderr: Options:
  stderr:   -h, --help	Show command-line help and exit.
vrg a b c                exit=2 stdout_bytes=0
  stderr: vrg: unexpected extra operand c
  stderr: 
  stderr: Usage: vrg [OPTIONS] PATTERN [ROOT]
  stderr: 
  stderr: Search with ripgrep and browse the results in a terminal UI.
  stderr: 
  stderr: Arguments:
  stderr:   PATTERN	The ripgrep search pattern.
  stderr:   ROOT	Search root: a directory or regular file. (default ".")
  stderr: 
  stderr: Options:
  stderr:   -h, --help	Show command-line help and exit.
vrg --unsupported        exit=2 stdout_bytes=0
  stderr: vrg: unsupported option --unsupported
  stderr: 
  stderr: Usage: vrg [OPTIONS] PATTERN [ROOT]
  stderr: 
  stderr: Search with ripgrep and browse the results in a terminal UI.
  stderr: 
  stderr: Arguments:
  stderr:   PATTERN	The ripgrep search pattern.
  stderr:   ROOT	Search root: a directory or regular file. (default ".")
  stderr: 
  stderr: Options:
  stderr:   -h, --help	Show command-line help and exit.
vrg -i                   exit=2 stdout_bytes=0
  stderr: vrg: unsupported option -i
  stderr: 
  stderr: Usage: vrg [OPTIONS] PATTERN [ROOT]
  stderr: 
  stderr: Search with ripgrep and browse the results in a terminal UI.
  stderr: 
  stderr: Arguments:
  stderr:   PATTERN	The ripgrep search pattern.
  stderr:   ROOT	Search root: a directory or regular file. (default ".")
  stderr: 
  stderr: Options:
  stderr:   -h, --help	Show command-line help and exit.
vrg foo -x               exit=2 stdout_bytes=0
  stderr: vrg: unsupported option -x
  stderr: 
  stderr: Usage: vrg [OPTIONS] PATTERN [ROOT]
  stderr: 
  stderr: Search with ripgrep and browse the results in a terminal UI.
  stderr: 
  stderr: Arguments:
  stderr:   PATTERN	The ripgrep search pattern.
  stderr:   ROOT	Search root: a directory or regular file. (default ".")
  stderr: 
  stderr: Options:
  stderr:   -h, --help	Show command-line help and exit.
```

Root validation: reject stdin (`-`), special files, and nonexistent paths; accept directories, regular files, and symlinks to either.

```bash
cd /home/chris/vrg/Notes/walkthroughs/001-06/code-walkthrough
err() { ./vrg "$@" >o.txt 2>e.txt; c=$?; printf "%-28s exit=%d\n" "vrg $*" "$c"; sed "s/^/  stderr: /" e.txt; }
err foo /nonexistent
err foo -
err foo /dev/null
err foo fixtures/fifo
```

```output
vrg foo /nonexistent         exit=2
  stderr: vrg: invalid root /nonexistent: does not exist
  stderr: 
  stderr: Usage: vrg [OPTIONS] PATTERN [ROOT]
  stderr: 
  stderr: Search with ripgrep and browse the results in a terminal UI.
  stderr: 
  stderr: Arguments:
  stderr:   PATTERN	The ripgrep search pattern.
  stderr:   ROOT	Search root: a directory or regular file. (default ".")
  stderr: 
  stderr: Options:
  stderr:   -h, --help	Show command-line help and exit.
vrg foo -                    exit=2
  stderr: vrg: invalid root "-": standard input is not a searchable root
  stderr: 
  stderr: Usage: vrg [OPTIONS] PATTERN [ROOT]
  stderr: 
  stderr: Search with ripgrep and browse the results in a terminal UI.
  stderr: 
  stderr: Arguments:
  stderr:   PATTERN	The ripgrep search pattern.
  stderr:   ROOT	Search root: a directory or regular file. (default ".")
  stderr: 
  stderr: Options:
  stderr:   -h, --help	Show command-line help and exit.
vrg foo /dev/null            exit=2
  stderr: vrg: invalid root /dev/null: not a directory or regular file
  stderr: 
  stderr: Usage: vrg [OPTIONS] PATTERN [ROOT]
  stderr: 
  stderr: Search with ripgrep and browse the results in a terminal UI.
  stderr: 
  stderr: Arguments:
  stderr:   PATTERN	The ripgrep search pattern.
  stderr:   ROOT	Search root: a directory or regular file. (default ".")
  stderr: 
  stderr: Options:
  stderr:   -h, --help	Show command-line help and exit.
vrg foo fixtures/fifo        exit=2
  stderr: vrg: invalid root fixtures/fifo: not a directory or regular file
  stderr: 
  stderr: Usage: vrg [OPTIONS] PATTERN [ROOT]
  stderr: 
  stderr: Search with ripgrep and browse the results in a terminal UI.
  stderr: 
  stderr: Arguments:
  stderr:   PATTERN	The ripgrep search pattern.
  stderr:   ROOT	Search root: a directory or regular file. (default ".")
  stderr: 
  stderr: Options:
  stderr:   -h, --help	Show command-line help and exit.
```

```bash
cd /home/chris/vrg/Notes/walkthroughs/001-06/code-walkthrough
ok() { ./vrg "$@" >o.txt 2>e.txt; c=$?; printf "%-32s exit=%d  %s\n" "vrg $*" "$c" "$(cat o.txt)"; }
ok foo
ok foo fixtures/sub
ok foo fixtures/file
ok foo fixtures/linkdir
ok foo fixtures/linkfile
```

```output
vrg foo                          exit=0  search stub: pattern=foo root=.
vrg foo fixtures/sub             exit=0  search stub: pattern=foo root=fixtures/sub
vrg foo fixtures/file            exit=0  search stub: pattern=foo root=fixtures/file
vrg foo fixtures/linkdir         exit=0  search stub: pattern=foo root=fixtures/linkdir
vrg foo fixtures/linkfile        exit=0  search stub: pattern=foo root=fixtures/linkfile
```

Operand edge cases: `./-` addresses a real file named `-`; empty and literal `-` patterns are retained; everything after the first `--` is positional.

```bash
cd /home/chris/vrg/Notes/walkthroughs/001-06/code-walkthrough/fixtures
../vrg foo ./-; echo "exit=$?"
cd ..
ok() { ./vrg "$@" >o.txt 2>e.txt; c=$?; printf "%-22s exit=%d  %s\n" "vrg $*" "$c" "$(cat o.txt)"; }
ok "" .
ok - .
ok -- --help
ok -- -h
ok -- --
ok -- - .
```

```output
search stub: pattern=foo root=./-
exit=0
vrg  .                 exit=0  search stub: pattern= root=.
vrg - .                exit=0  search stub: pattern=- root=.
vrg -- --help          exit=0  search stub: pattern=--help root=.
vrg -- -h              exit=0  search stub: pattern=-h root=.
vrg -- --              exit=0  search stub: pattern=-- root=.
vrg -- - .             exit=0  search stub: pattern=- root=.
```

```bash
cd /home/chris/vrg/Notes/walkthroughs/001-06/code-walkthrough
./vrg foo "$(printf "/no\033[31msuch")" >o.txt 2>e.txt; echo "exit=$?"
echo "stderr bytes:"; od -c e.txt | head -3
echo "raw ESC present: $(grep -c $"\x1b" e.txt)"
```

```output
exit=2
stderr bytes:
0000000   v   r   g   :       i   n   v   a   l   i   d       r   o   o
0000020   t       /   n   o   ^   [   [   3   1   m   s   u   c   h   :
0000040       d   o   e   s       n   o   t       e   x   i   s   t  \n
raw ESC present: 0
```

Hostile operand bytes (above) are escaped — `^[` is printed, no raw ESC reaches the terminal. Finally, the help-assignment spellings `--help=false`/`-h=false` are *not* help requests: no `Usage:` on stdout. Their eventual exit status is Issue #2's contract and is not pinned here.

```bash
cd /home/chris/vrg/Notes/walkthroughs/001-06/code-walkthrough
for a in "--help=false" "-h=false"; do
  ./vrg foo "$a" >o.txt 2>e.txt; c=$?
  echo "vrg foo $a -> exit=$c usage_lines=$(grep -c "^Usage:" o.txt) stdout=$(cat o.txt) stderr=$(cat e.txt)"
done
```

```output
vrg foo --help=false -> exit=0 usage_lines=0 stdout=search stub: pattern=foo root=. stderr=
vrg foo -h=false -> exit=0 usage_lines=0 stdout=search stub: pattern=foo root=. stderr=
```

## Result. Every Issue #1 manual-verification class is demonstrated: generated help on stdout/exit-0/empty-stderr for all help spellings with no root validation, child, TUI, or stub; classified sanitized exit-2 diagnostics followed by the generated usage block on stderr for missing pattern, excess operands, unsupported options, and invalid roots (stdin, special, nonexistent); default/explicit/symlink/regular-file roots; `./-`; empty and literal `-` patterns; `--` operand protection; and hostile-byte escaping. The emission-prevention output strategy and explicit result-kind dispatch implemented here match the Task 1 decision record: the library never writes to either stream and never exits the process; `cmd/vrg` owns both. Suites: `TestGeneratedHelpStdout`, `TestCLIOutputSafety`, `TestExecutableBoundary` (`cmd/vrg/main_test.go`); help-only/positional/diagnostic tables (`internal/cli/cli_test.go`).
