# Issue #2: CLI flag allow-list, combined shorts, cumulative -u, --, ordered child argv

*2026-09-10T19:52:45Z by Showboat 0.6.1*
<!-- showboat-id: d3f46267-093b-4853-b941-6750b2b766e1 -->

Walkthrough for Issue #2 (`Notes/issues/002-cli-flag-allow-list-and-child-argv.md`), implementing the flag allow-list and child-argv contract specified in `Notes/PRD-vrg.md` (*Implementation Decisions → Invocation and child arguments*; *Module Design → CLI*). It extends the Issue #1 CLI foundation: the shared `optionDecls` table now carries the allow-listed no-argument search flags, and the ordered raw-token preflight records each accepted spelling in encounter order — the child argv and cumulative `-u` count come solely from those scan records, never from library option values. All generated artifacts live in this directory: the built `vrg` binary and the `fixtures/` tree.

```bash
cd /home/chris/vrg && go build ./... && go vet ./... && echo GATES-OK
```

```output
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
cd /home/chris/vrg && go build -o Notes/walkthroughs/002-04/code-walkthrough/vrg ./cmd/vrg && file Notes/walkthroughs/002-04/code-walkthrough/vrg | cut -d: -f2 | cut -c1-60
```

```output
 ELF 64-bit LSB executable, ARM aarch64, version 1 (SYSV), s
```

## Exact child argv for every accepted flag. The stub prints the full child argument vector: mandatory `--json --no-config`, the supplied spelling verbatim, `--`, pattern, root (default `.`). Each allow-listed flag is shown in short and long form.

```bash
cd /home/chris/vrg/Notes/walkthroughs/002-04/code-walkthrough
for f in -i --ignore-case -S --smart-case -s --case-sensitive -w --word-regexp -x --line-regexp -F --fixed-strings --hidden --no-hidden --no-ignore -u --unrestricted -L --follow; do
  ./vrg "$f" foo >o.txt 2>e.txt; printf '%-16s exit=%d  %s\n' "$f" "$?" "$(cat o.txt)"
done
```

```output
-i               exit=0  search stub: argv=rg --json --no-config -i -- foo .
--ignore-case    exit=0  search stub: argv=rg --json --no-config --ignore-case -- foo .
-S               exit=0  search stub: argv=rg --json --no-config -S -- foo .
--smart-case     exit=0  search stub: argv=rg --json --no-config --smart-case -- foo .
-s               exit=0  search stub: argv=rg --json --no-config -s -- foo .
--case-sensitive exit=0  search stub: argv=rg --json --no-config --case-sensitive -- foo .
-w               exit=0  search stub: argv=rg --json --no-config -w -- foo .
--word-regexp    exit=0  search stub: argv=rg --json --no-config --word-regexp -- foo .
-x               exit=0  search stub: argv=rg --json --no-config -x -- foo .
--line-regexp    exit=0  search stub: argv=rg --json --no-config --line-regexp -- foo .
-F               exit=0  search stub: argv=rg --json --no-config -F -- foo .
--fixed-strings  exit=0  search stub: argv=rg --json --no-config --fixed-strings -- foo .
--hidden         exit=0  search stub: argv=rg --json --no-config --hidden -- foo .
--no-hidden      exit=0  search stub: argv=rg --json --no-config --no-hidden -- foo .
--no-ignore      exit=0  search stub: argv=rg --json --no-config --no-ignore -- foo .
-u               exit=0  search stub: argv=rg --json --no-config -u -- foo .
--unrestricted   exit=0  search stub: argv=rg --json --no-config --unrestricted -- foo .
-L               exit=0  search stub: argv=rg --json --no-config -L -- foo .
--follow         exit=0  search stub: argv=rg --json --no-config --follow -- foo .
```

## Ordered exact-spelling forwarding. Repeated and mixed options preserve encounter order and the supplied spelling (`-i -s -i`, `-isi`, `--ignore-case -s -i`); combined shorts expand left to right (`-iwF`); options interleaved with both operands still forward in order; contradictory flags are forwarded without normalization for ripgrep to resolve.

```bash
cd /home/chris/vrg/Notes/walkthroughs/002-04/code-walkthrough
ok() { ./vrg "$@" >o.txt 2>e.txt; printf '%-34s exit=%d  %s\n' "vrg $*" "$?" "$(cat o.txt)"; }
ok -i -s -i foo
ok -isi foo
ok --ignore-case -s -i foo
ok -iwF foo
ok -iw foo fixtures/sub
ok foo -i fixtures/sub
ok foo -i fixtures/sub -s
ok -i -s -S foo
```

```output
vrg -i -s -i foo                   exit=0  search stub: argv=rg --json --no-config -i -s -i -- foo .
vrg -isi foo                       exit=0  search stub: argv=rg --json --no-config -i -s -i -- foo .
vrg --ignore-case -s -i foo        exit=0  search stub: argv=rg --json --no-config --ignore-case -s -i -- foo .
vrg -iwF foo                       exit=0  search stub: argv=rg --json --no-config -i -w -F -- foo .
vrg -iw foo fixtures/sub           exit=0  search stub: argv=rg --json --no-config -i -w -- foo fixtures/sub
vrg foo -i fixtures/sub            exit=0  search stub: argv=rg --json --no-config -i -- foo fixtures/sub
vrg foo -i fixtures/sub -s         exit=0  search stub: argv=rg --json --no-config -i -s -- foo fixtures/sub
vrg -i -s -S foo                   exit=0  search stub: argv=rg --json --no-config -i -s -S -- foo .
```

## Cumulative `-u` boundary. Occurrences accumulate across separate, combined, short, and long spellings: zero to two are forwarded; a third in any mix is a sanitized exit-2 usage error with no child argv and no TUI.

```bash
cd /home/chris/vrg/Notes/walkthroughs/002-04/code-walkthrough
run() { ./vrg "$@" >o.txt 2>e.txt; c=$?; printf '%-32s exit=%d  %s%s\n' "vrg $*" "$c" "$(cat o.txt)" "$(head -1 e.txt)"; }
run -u foo
run -uu foo
run -iu foo
run -iuu foo
run -u --unrestricted foo
run -uuu foo
run -u -uu foo
run -iuuu foo
run -u --unrestricted -u foo
run --unrestricted -uu foo
```

```output
vrg -u foo                       exit=0  search stub: argv=rg --json --no-config -u -- foo .
vrg -uu foo                      exit=0  search stub: argv=rg --json --no-config -u -u -- foo .
vrg -iu foo                      exit=0  search stub: argv=rg --json --no-config -i -u -- foo .
vrg -iuu foo                     exit=0  search stub: argv=rg --json --no-config -i -u -u -- foo .
vrg -u --unrestricted foo        exit=0  search stub: argv=rg --json --no-config -u --unrestricted -- foo .
vrg -uuu foo                     exit=2  vrg: option -u/--unrestricted may be used at most twice
vrg -u -uu foo                   exit=2  vrg: option -u/--unrestricted may be used at most twice
vrg -iuuu foo                    exit=2  vrg: option -u/--unrestricted may be used at most twice
vrg -u --unrestricted -u foo     exit=2  vrg: option -u/--unrestricted may be used at most twice
vrg --unrestricted -uu foo       exit=2  vrg: option -u/--unrestricted may be used at most twice
```

## Unsupported and argument-taking options. Everything outside the allow-list — including `-e` and argument-taking options in separate, `=`, or combined spellings — is a usage error: exit 2, sanitized diagnostic plus the generated usage block on stderr, nothing on stdout.

```bash
cd /home/chris/vrg/Notes/walkthroughs/002-04/code-walkthrough
err() { ./vrg "$@" >o.txt 2>e.txt; printf '%-30s exit=%d stdout_bytes=%d diag=%s\n' "vrg $*" "$?" "$(wc -c <o.txt)" "$(head -1 e.txt)"; }
err -e foo
err --type go foo
err --type=go foo
err --max-count 3 foo
err -m 3 foo
err -g '*.go' foo
err -A2 foo
err --vimgrep foo
err -foo
```

```output
vrg -e foo                     exit=2 stdout_bytes=0 diag=vrg: unsupported option -e
vrg --type go foo              exit=2 stdout_bytes=0 diag=vrg: unsupported option --type
vrg --type=go foo              exit=2 stdout_bytes=0 diag=vrg: unsupported option --type=go
vrg --max-count 3 foo          exit=2 stdout_bytes=0 diag=vrg: unsupported option --max-count
vrg -m 3 foo                   exit=2 stdout_bytes=0 diag=vrg: unsupported option -m
vrg -g *.go foo                exit=2 stdout_bytes=0 diag=vrg: unsupported option -g
vrg -A2 foo                    exit=2 stdout_bytes=0 diag=vrg: unsupported option -A2
vrg --vimgrep foo              exit=2 stdout_bytes=0 diag=vrg: unsupported option --vimgrep
vrg -foo                       exit=2 stdout_bytes=0 diag=vrg: unsupported option -foo
```

## Boolean assignment rejection. The allow-list is a no-argument contract, so `=` spellings are rejected lexically even though the library would parse them — for search flags, for `--unrestricted`, and for the help option. The same bytes after `--` are ordinary positional operands.

```bash
cd /home/chris/vrg/Notes/walkthroughs/002-04/code-walkthrough
run() { ./vrg "$@" >o.txt 2>e.txt; c=$?; printf '%-36s exit=%d  %s%s\n' "vrg $*" "$c" "$(cat o.txt)" "$(head -1 e.txt)"; }
run --ignore-case=false foo
run -i=false foo
run --unrestricted=false foo
run --help=false foo
run -h=false foo
run -- --ignore-case=false
run -- -i=false .
run -- --help=false .
```

```output
vrg --ignore-case=false foo          exit=2  vrg: unsupported option --ignore-case=false
vrg -i=false foo                     exit=2  vrg: unsupported option -i=false
vrg --unrestricted=false foo         exit=2  vrg: unsupported option --unrestricted=false
vrg --help=false foo                 exit=2  vrg: unsupported option --help=false
vrg -h=false foo                     exit=2  vrg: unsupported option -h=false
vrg -- --ignore-case=false           exit=0  search stub: argv=rg --json --no-config -- --ignore-case=false .
vrg -- -i=false .                    exit=0  search stub: argv=rg --json --no-config -- -i=false .
vrg -- --help=false .                exit=0  search stub: argv=rg --json --no-config -- --help=false .
```

## Operand forms after `--`. The first `--` ends option parsing: dash-leading patterns need it (`-foo` alone is an unsupported option), the lone `-` and the empty pattern pass through verbatim as argv elements, and a second `--` is the pattern itself in both root forms.

```bash
cd /home/chris/vrg/Notes/walkthroughs/002-04/code-walkthrough
run() { ./vrg "$@" >o.txt 2>e.txt; c=$?; printf '%-30s exit=%d  %s%s\n' "vrg $*" "$c" "$(cat o.txt)" "$(head -1 e.txt)"; }
run "" .
run - .
run -- -foo
run -foo
run -- --
run -- -- .
run -i -- -foo
```

```output
vrg  .                         exit=0  search stub: argv=rg --json --no-config --  .
vrg - .                        exit=0  search stub: argv=rg --json --no-config -- - .
vrg -- -foo                    exit=0  search stub: argv=rg --json --no-config -- -foo .
vrg -foo                       exit=2  vrg: unsupported option -foo
vrg -- --                      exit=0  search stub: argv=rg --json --no-config -- -- .
vrg -- -- .                    exit=0  search stub: argv=rg --json --no-config -- -- .
vrg -i -- -foo                 exit=0  search stub: argv=rg --json --no-config -i -- -foo .
```

## Help regressions with flags present. Every help-only spelling — bare, first-token, mixed with search flags, combined, after both operands, even after a `-u` overrun or a rejected assignment — prints exactly one help copy on stdout, exit 0, empty stderr, no child argv or stub, no TUI. Help resolves before all allow-list validation. `vrg -i` without help stays a missing-pattern exit 2, and help-like tokens after `--` stay positional.

```bash
cd /home/chris/vrg/Notes/walkthroughs/002-04/code-walkthrough
check() { ./vrg "$@" >o.txt 2>e.txt; c=$?; u=$(grep -c '^Usage:' o.txt); s=$(wc -c <e.txt); st=$(grep -c 'search stub:' o.txt); printf '%-32s exit=%d usage_lines=%d stderr_bytes=%d stub_lines=%d\n' "vrg $*" "$c" "$u" "$s" "$st"; }
check
check -h
check --help
check -i --help
check --ignore-case --help
check -ih foo
check -iwF -h
check foo fixtures/sub --help
check -uuu --help
check --ignore-case=false --help
check -i
check -- --help
```

```output
vrg                              exit=0 usage_lines=1 stderr_bytes=0 stub_lines=0
vrg -h                           exit=0 usage_lines=1 stderr_bytes=0 stub_lines=0
vrg --help                       exit=0 usage_lines=1 stderr_bytes=0 stub_lines=0
vrg -i --help                    exit=0 usage_lines=1 stderr_bytes=0 stub_lines=0
vrg --ignore-case --help         exit=0 usage_lines=1 stderr_bytes=0 stub_lines=0
vrg -ih foo                      exit=0 usage_lines=1 stderr_bytes=0 stub_lines=0
vrg -iwF -h                      exit=0 usage_lines=1 stderr_bytes=0 stub_lines=0
vrg foo fixtures/sub --help      exit=0 usage_lines=1 stderr_bytes=0 stub_lines=0
vrg -uuu --help                  exit=0 usage_lines=1 stderr_bytes=0 stub_lines=0
vrg --ignore-case=false --help   exit=0 usage_lines=1 stderr_bytes=0 stub_lines=0
vrg -i                           exit=2 usage_lines=0 stderr_bytes=873 stub_lines=0
vrg -- --help                    exit=0 usage_lines=0 stderr_bytes=0 stub_lines=1
```

```bash
cd /home/chris/vrg/Notes/walkthroughs/002-04/code-walkthrough
./vrg -i 2>&1 | head -1; echo '...'
./vrg -- --help
```

```output
vrg: missing required argument PATTERN
...
search stub: argv=rg --json --no-config -- --help .
```

## Generated help lists every supported search flag in short and long form, rendered from the same shared `optionDecls` table that configures mow.cli and validates the preflight scan.

```bash
cd /home/chris/vrg/Notes/walkthroughs/002-04/code-walkthrough && ./vrg --help
```

```output
Usage: vrg [OPTIONS] PATTERN [ROOT]

Search with ripgrep and browse the results in a terminal UI.

Arguments:
  PATTERN	The ripgrep search pattern.
  ROOT	Search root: a directory or regular file. (default ".")

Options:
  -h, --help	Show command-line help and exit.
  -i, --ignore-case	Case insensitive search.
  -S, --smart-case	Case insensitive unless the pattern has uppercase.
  -s, --case-sensitive	Case sensitive search.
  -w, --word-regexp	Match whole words only.
  -x, --line-regexp	Match whole lines only.
  -F, --fixed-strings	Treat the pattern as a literal string.
  --hidden	Search hidden files and directories.
  --no-hidden	Do not search hidden files and directories.
  --no-ignore	Do not respect ignore files.
  -u, --unrestricted	Loosen search restrictions; may be given twice.
  -L, --follow	Follow symbolic links.
```

## Result. Every Issue #2 manual-verification class is demonstrated: each allow-listed flag forwarded verbatim in short and long form; encounter-order and exact-spelling preservation for `-i -s -i`, `-isi`, `--ignore-case -s -i`, and options interleaved with both operands (`foo -i fixtures/sub -s`); combined-short expansion including `-iwF`; cumulative `-u` acceptance (`-u`, `-uu`, `-iu`, `-iuu`, `-u --unrestricted`) and rejection (`-uuu`, `-u -uu`, `-iuuu`, `-u --unrestricted -u`, `--unrestricted -uu`); unsupported and argument-taking options (`-e`, `--type`, `--max-count`, `-m`, `-g`, `-A2`, `--vimgrep`); lexical rejection of `--ignore-case=false`, `-i=false`, `--unrestricted=false`, `--help=false`, `-h=false` with the same bytes positional after `--`; empty, literal `-`, dash-leading, and literal `--` patterns; and the mandatory child order `--json --no-config <flags> -- <pattern> <root>`. Retained Issue #1 help regressions hold with flags present — one stdout copy, empty stderr, exit 0, no child or TUI — and `vrg -i` remains a missing-pattern exit 2. Suites: `TestSearchFlagSpellingsForwarded`, `TestChildArgvPreservesEncounterOrderAndSpelling`, `TestCumulativeUnrestrictedBoundary`, `TestUnsupportedOptionsRejected`, `TestAssignmentSpellingsRejected`, `TestChildArgvOperandForms`, `TestGeneratedHelpListsDeclaredOptions` (`internal/cli`); `TestChildArgv`, `TestFlagContractUsageErrors`, `TestGeneratedHelpStdout`, `TestCLIOutputSafety`, `TestExecutableBoundary` (`cmd/vrg`).
