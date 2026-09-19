# Issue #2: CLI flag allow-list, combined shorts, cumulative -u, --, ordered child argv

*2026-09-16T16:54:03Z by Showboat 0.6.1*
<!-- showboat-id: 11aa99fe-e42a-4a0a-9d16-9a0d61de3aeb -->

Walkthrough for Issue #2 (`Notes/issues/002-cli-flag-allow-list-and-child-argv.md`), extending the Issue #1 CLI foundation per `Notes/PRD-vrg.md` (*Implementation Decisions → Invocation and child arguments*; *Module Design → CLI*): the shared `optionDecls` table now drives `mow.cli` configuration, raw-token scan validation, ordered search-flag records, and generated help; the scan records accepted spellings in encounter order (combined shorts expand left to right, longs keep the supplied spelling), rejects every `=` assignment spelling lexically, and enforces the cumulative two-occurrence `-u`/`--unrestricted` cap. The child argv is exactly `rg --json --no-config <ordered expanded user flags> -- <pattern> <root>`. All generated artifacts (the built `vrg` binary, `fixtures/`) live in this directory.

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
cd /home/chris/vrg && go test -count=1 -v -run "TestSearchFlagsForwarded|TestOrderedChildArgs|TestUnrestrictedLimit|TestOptionAssignmentFormsRejected|TestGeneratedHelpListsSearchFlags|TestChildArgvBoundary" ./internal/cli/ ./cmd/vrg/ 2>&1 | grep -E "^(=== RUN *|--- (PASS|FAIL)|ok|FAIL)" | grep -v "^=== RUN" | sed "s/[[:space:]][0-9.]*s$//" | tail -50
```

```output
--- PASS: TestOptionAssignmentFormsRejected (0.00s)
--- PASS: TestSearchFlagsForwarded (0.00s)
--- PASS: TestOrderedChildArgs (0.00s)
--- PASS: TestUnrestrictedLimit (0.00s)
--- PASS: TestGeneratedHelpListsSearchFlags (0.00s)
ok  	vrg/internal/cli
--- PASS: TestChildArgvBoundary (0.01s)
ok  	vrg/cmd/vrg
```

```bash
cd /home/chris/vrg && go build -o Notes/walkthroughs/002-04/code-walkthrough/vrg ./cmd/vrg && mkdir -p Notes/walkthroughs/002-04/code-walkthrough/fixtures && file Notes/walkthroughs/002-04/code-walkthrough/vrg | cut -d: -f2 | cut -c1-60
```

```output
 ELF 64-bit LSB executable, ARM aarch64, version 1 (SYSV), s
```

Every allow-listed no-argument flag, in short and long form, is forwarded verbatim into the child argv: `rg --json --no-config <flags> -- <pattern> <root>` (manual check 1, AC1).

```bash
cd /home/chris/vrg/Notes/walkthroughs/002-04/code-walkthrough
ok() { ./vrg "$@" >o.txt 2>e.txt; printf "%-24s exit=%d  %s" "vrg $*" "$?" "$(cat o.txt)"; test -s e.txt && echo "  STDERR: $(head -1 e.txt)" || echo; }
for f in -i --ignore-case -S --smart-case -s --case-sensitive -w --word-regexp -x --line-regexp -F --fixed-strings --hidden --no-hidden --no-ignore -u --unrestricted -L --follow; do ok "$f" foo; done
```

```output
vrg -i foo               exit=0  search stub: argv=rg --json --no-config -i -- foo .
vrg --ignore-case foo    exit=0  search stub: argv=rg --json --no-config --ignore-case -- foo .
vrg -S foo               exit=0  search stub: argv=rg --json --no-config -S -- foo .
vrg --smart-case foo     exit=0  search stub: argv=rg --json --no-config --smart-case -- foo .
vrg -s foo               exit=0  search stub: argv=rg --json --no-config -s -- foo .
vrg --case-sensitive foo exit=0  search stub: argv=rg --json --no-config --case-sensitive -- foo .
vrg -w foo               exit=0  search stub: argv=rg --json --no-config -w -- foo .
vrg --word-regexp foo    exit=0  search stub: argv=rg --json --no-config --word-regexp -- foo .
vrg -x foo               exit=0  search stub: argv=rg --json --no-config -x -- foo .
vrg --line-regexp foo    exit=0  search stub: argv=rg --json --no-config --line-regexp -- foo .
vrg -F foo               exit=0  search stub: argv=rg --json --no-config -F -- foo .
vrg --fixed-strings foo  exit=0  search stub: argv=rg --json --no-config --fixed-strings -- foo .
vrg --hidden foo         exit=0  search stub: argv=rg --json --no-config --hidden -- foo .
vrg --no-hidden foo      exit=0  search stub: argv=rg --json --no-config --no-hidden -- foo .
vrg --no-ignore foo      exit=0  search stub: argv=rg --json --no-config --no-ignore -- foo .
vrg -u foo               exit=0  search stub: argv=rg --json --no-config -u -- foo .
vrg --unrestricted foo   exit=0  search stub: argv=rg --json --no-config --unrestricted -- foo .
vrg -L foo               exit=0  search stub: argv=rg --json --no-config -L -- foo .
vrg --follow foo         exit=0  search stub: argv=rg --json --no-config --follow -- foo .
```

Ordered exact-spelling forwarding (manual checks 1, 2, 7; AC2, AC3): encounter order and the spellings as supplied survive for repeated and mixed-alias options, combined shorts expand left to right, options may sit anywhere before `--`, and contradictory flags are forwarded without normalization.

```bash
cd /home/chris/vrg/Notes/walkthroughs/002-04/code-walkthrough
ok() { ./vrg "$@" >o.txt 2>e.txt; printf "%-34s exit=%d  %s" "vrg $*" "$?" "$(cat o.txt)"; test -s e.txt && echo "  STDERR: $(head -1 e.txt)" || echo; }
ok -iw foo fixtures
ok foo -i fixtures
ok foo -i fixtures -s
ok -i -s -i foo
ok -isi foo
ok --ignore-case -s -i foo
ok -iwF foo
ok --hidden --no-hidden foo
ok -w foo -F fixtures
```

```output
vrg -iw foo fixtures               exit=0  search stub: argv=rg --json --no-config -i -w -- foo fixtures
vrg foo -i fixtures                exit=0  search stub: argv=rg --json --no-config -i -- foo fixtures
vrg foo -i fixtures -s             exit=0  search stub: argv=rg --json --no-config -i -s -- foo fixtures
vrg -i -s -i foo                   exit=0  search stub: argv=rg --json --no-config -i -s -i -- foo .
vrg -isi foo                       exit=0  search stub: argv=rg --json --no-config -i -s -i -- foo .
vrg --ignore-case -s -i foo        exit=0  search stub: argv=rg --json --no-config --ignore-case -s -i -- foo .
vrg -iwF foo                       exit=0  search stub: argv=rg --json --no-config -i -w -F -- foo .
vrg --hidden --no-hidden foo       exit=0  search stub: argv=rg --json --no-config --hidden --no-hidden -- foo .
vrg -w foo -F fixtures             exit=0  search stub: argv=rg --json --no-config -w -F -- foo fixtures
```

Cumulative `-u`/`--unrestricted` counting across separate, combined, short, and long spellings (manual checks 3, 8; AC4, AC9): two occurrences pass, the third is an exit-2 usage error with no child argv.

```bash
cd /home/chris/vrg/Notes/walkthroughs/002-04/code-walkthrough
chk() { ./vrg "$@" >o.txt 2>e.txt; printf "%-36s exit=%d  %s" "vrg $*" "$?" "$(cat o.txt)"; test -s e.txt && echo "  stderr1: $(head -1 e.txt)" || echo; }
chk -u foo
chk -uu foo
chk -iu foo
chk -iuu foo
chk -u --unrestricted foo
chk --unrestricted -u foo
chk -uuu foo
chk -u -uu foo
chk -iuuu foo
chk -u --unrestricted -u foo
chk --unrestricted -uu foo
```

```output
vrg -u foo                           exit=0  search stub: argv=rg --json --no-config -u -- foo .
vrg -uu foo                          exit=0  search stub: argv=rg --json --no-config -u -u -- foo .
vrg -iu foo                          exit=0  search stub: argv=rg --json --no-config -i -u -- foo .
vrg -iuu foo                         exit=0  search stub: argv=rg --json --no-config -i -u -u -- foo .
vrg -u --unrestricted foo            exit=0  search stub: argv=rg --json --no-config -u --unrestricted -- foo .
vrg --unrestricted -u foo            exit=0  search stub: argv=rg --json --no-config --unrestricted -u -- foo .
vrg -uuu foo                         exit=2    stderr1: vrg: too many unrestricted options: -u/--unrestricted may appear at most twice
vrg -u -uu foo                       exit=2    stderr1: vrg: too many unrestricted options: -u/--unrestricted may appear at most twice
vrg -iuuu foo                        exit=2    stderr1: vrg: too many unrestricted options: -u/--unrestricted may appear at most twice
vrg -u --unrestricted -u foo         exit=2    stderr1: vrg: too many unrestricted options: -u/--unrestricted may appear at most twice
vrg --unrestricted -uu foo           exit=2    stderr1: vrg: too many unrestricted options: -u/--unrestricted may appear at most twice
```

Unsupported and argument-taking options are exit-2 usage errors (manual check 4; AC5): `-e`, `--type`, and every other non-allow-listed spelling. A dash-leading pattern without `--` hits the same rejection.

```bash
cd /home/chris/vrg/Notes/walkthroughs/002-04/code-walkthrough
err() { ./vrg "$@" >o.txt 2>e.txt; printf "%-26s exit=%d stdout_bytes=%d stderr1: %s\n" "vrg $*" "$?" "$(wc -c <o.txt)" "$(head -1 e.txt)"; }
err -e foo
err --type go foo
err -t go foo
err -v foo
err -foo
```

```output
vrg -e foo                 exit=2 stdout_bytes=0 stderr1: vrg: unsupported option -e
vrg --type go foo          exit=2 stdout_bytes=0 stderr1: vrg: unsupported option --type
vrg -t go foo              exit=2 stdout_bytes=0 stderr1: vrg: unsupported option -t
vrg -v foo                 exit=2 stdout_bytes=0 stderr1: vrg: unsupported option -v
vrg -foo                   exit=2 stdout_bytes=0 stderr1: vrg: unsupported option -foo
```

Assignment rejection (manual check 10; AC6): every `=` spelling on a declared no-argument or help option is rejected lexically before the library's permissive boolean parsing can see it — the same bytes after `--` are ordinary operands when arity permits.

```bash
cd /home/chris/vrg/Notes/walkthroughs/002-04/code-walkthrough
err() { ./vrg "$@" >o.txt 2>e.txt; printf "%-34s exit=%d stdout_bytes=%d stderr1: %s\n" "vrg $*" "$?" "$(wc -c <o.txt)" "$(head -1 e.txt)"; }
err --ignore-case=false foo
err -i=false foo
err --unrestricted=false foo
err --help=false foo
err -h=false foo
err --help=true foo
ok() { ./vrg "$@" >o.txt 2>e.txt; printf "%-34s exit=%d  %s\n" "vrg $*" "$?" "$(cat o.txt)"; }
ok -- --ignore-case=false
ok -- -i=false
ok -- --help=false
ok -- -h=false
```

```output
vrg --ignore-case=false foo        exit=2 stdout_bytes=0 stderr1: vrg: unsupported option --ignore-case=false
vrg -i=false foo                   exit=2 stdout_bytes=0 stderr1: vrg: unsupported option -i=false
vrg --unrestricted=false foo       exit=2 stdout_bytes=0 stderr1: vrg: unsupported option --unrestricted=false
vrg --help=false foo               exit=2 stdout_bytes=0 stderr1: vrg: unsupported option --help=false
vrg -h=false foo                   exit=2 stdout_bytes=0 stderr1: vrg: unsupported option -h=false
vrg --help=true foo                exit=2 stdout_bytes=0 stderr1: vrg: unsupported option --help=true
vrg -- --ignore-case=false         exit=0  search stub: argv=rg --json --no-config -- --ignore-case=false .
vrg -- -i=false                    exit=0  search stub: argv=rg --json --no-config -- -i=false .
vrg -- --help=false                exit=0  search stub: argv=rg --json --no-config -- --help=false .
vrg -- -h=false                    exit=0  search stub: argv=rg --json --no-config -- -h=false .
```

`--` protection and operand edge cases (manual checks 5, 6; AC7, AC10): a dash-leading pattern requires `--`, the literal `-` pattern needs none, a second `--` is the verbatim pattern, and the empty pattern forwards as an empty argv element (note the empty field between `--` and `.`).

```bash
cd /home/chris/vrg/Notes/walkthroughs/002-04/code-walkthrough
ok() { ./vrg "$@" >o.txt 2>e.txt; printf "%-22s exit=%d  %s\n" "vrg $*" "$?" "$(cat o.txt)"; }
err() { ./vrg "$@" >o.txt 2>e.txt; printf "%-22s exit=%d stderr1: %s\n" "vrg $*" "$?" "$(head -1 e.txt)"; }
ok -- -foo
err -foo
ok - .
ok -- --
ok -- -- .
ok "" .
ok -- -h
ok -- --help
```

```output
vrg -- -foo            exit=0  search stub: argv=rg --json --no-config -- -foo .
vrg -foo               exit=2 stderr1: vrg: unsupported option -foo
vrg - .                exit=0  search stub: argv=rg --json --no-config -- - .
vrg -- --              exit=0  search stub: argv=rg --json --no-config -- -- .
vrg -- -- .            exit=0  search stub: argv=rg --json --no-config -- -- .
vrg  .                 exit=0  search stub: argv=rg --json --no-config --  .
vrg -- -h              exit=0  search stub: argv=rg --json --no-config -- -h .
vrg -- --help          exit=0  search stub: argv=rg --json --no-config -- --help .
```

Help regressions (manual check 9; AC5): bare, first-token, mixed-flag, combined, and later help all produce the Issue #1 help-only result — exit 0, exactly one `Usage:` copy on stdout, empty stderr, no child argv, no TUI — and help resolution precedes all allow-list validation. Flags alone with no pattern remain the exit-2 missing-pattern error.

```bash
cd /home/chris/vrg/Notes/walkthroughs/002-04/code-walkthrough
check() { ./vrg "$@" >o.txt 2>e.txt; c=$?; u=$(grep -c "^Usage:" o.txt); s=$(wc -c <e.txt); st=$(grep -c "search stub:" o.txt); esc=$(cat o.txt e.txt | tr -d -c "\033" | wc -c); printf "%-32s exit=%d usage_lines=%d stderr_bytes=%d stub_lines=%d esc_bytes=%d\n" "vrg $*" "$c" "$u" "$s" "$st" "$esc"; }
check
check -h
check --help
check -i --help
check -ih foo
check foo fixtures --help
check -e --help
check -uuu --help
check --help=false --help
```

```output
vrg                              exit=0 usage_lines=1 stderr_bytes=0 stub_lines=0 esc_bytes=0
vrg -h                           exit=0 usage_lines=1 stderr_bytes=0 stub_lines=0 esc_bytes=0
vrg --help                       exit=0 usage_lines=1 stderr_bytes=0 stub_lines=0 esc_bytes=0
vrg -i --help                    exit=0 usage_lines=1 stderr_bytes=0 stub_lines=0 esc_bytes=0
vrg -ih foo                      exit=0 usage_lines=1 stderr_bytes=0 stub_lines=0 esc_bytes=0
vrg foo fixtures --help          exit=0 usage_lines=1 stderr_bytes=0 stub_lines=0 esc_bytes=0
vrg -e --help                    exit=0 usage_lines=1 stderr_bytes=0 stub_lines=0 esc_bytes=0
vrg -uuu --help                  exit=0 usage_lines=1 stderr_bytes=0 stub_lines=0 esc_bytes=0
vrg --help=false --help          exit=0 usage_lines=1 stderr_bytes=0 stub_lines=0 esc_bytes=0
```

```bash
cd /home/chris/vrg/Notes/walkthroughs/002-04/code-walkthrough
./vrg -i >o.txt 2>e.txt; echo "vrg -i -> exit=$? stdout_bytes=$(wc -c <o.txt)"; head -1 e.txt
./vrg -iwF >o.txt 2>e.txt; echo "vrg -iwF -> exit=$? stdout_bytes=$(wc -c <o.txt)"; head -1 e.txt
```

```output
vrg -i -> exit=2 stdout_bytes=0
vrg: missing required argument PATTERN
vrg -iwF -> exit=2 stdout_bytes=0
vrg: missing required argument PATTERN
```

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
  -i, --ignore-case	Case-insensitive search.
  -S, --smart-case	Smart case search.
  -s, --case-sensitive	Case-sensitive search.
  -w, --word-regexp	Match whole words only.
  -x, --line-regexp	Match whole lines only.
  -F, --fixed-strings	Treat the pattern as a literal string.
  --hidden	Search hidden files and directories.
  --no-hidden	Do not search hidden files and directories.
  --no-ignore	Do not respect ignore files.
  -u, --unrestricted	Reduce filtering; may be supplied at most twice.
  -L, --follow	Follow symbolic links.
```

## Result

Every Issue #2 manual-verification class is demonstrated against the real binary: all eleven allow-listed flags in short and long form forwarded verbatim after the mandatory `--json --no-config`; exact ordered child argv for `-i -s -i`, `-isi`, `--ignore-case -s -i`, `-iwF`, and options interleaved with both operands (`foo -i fixtures -s`); combined-short and cumulative `-u` boundaries (two pass, three rejected, across mixed aliases); unsupported (`-e`, `-v`, `-foo`) and argument-taking (`--type`, `-t`) options rejected; every `=` assignment spelling (search flags and help, `=false` and `=true`) rejected lexically while staying positional after `--`; literal `-`, empty, dash-leading (`-- -foo`), and literal `--` patterns; and every Issue #1 help regression unchanged — exit 0, one stdout help copy, empty stderr, no child or TUI — with `vrg -i` still an exit-2 missing-pattern error and generated help listing every flag from the shared declarations. Suites: `TestSearchFlagsForwarded`, `TestOrderedChildArgs`, `TestUnrestrictedLimit`, `TestOptionAssignmentFormsRejected`, `TestGeneratedHelpListsSearchFlags` (`internal/cli/cli_test.go`); `TestChildArgvBoundary`, `TestGeneratedHelpStdout`, `TestCLIOutputSafety`, `TestExecutableBoundary` (`cmd/vrg/main_test.go`). The forwarding order comes solely from the ordered scan records — never from `VarOpt` callback order — per `Notes/PRD-vrg.md` and `Notes/issues/002-cli-flag-allow-list-and-child-argv.md`.

