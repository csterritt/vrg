# Issue #2: CLI flag allow-list, combined shorts, cumulative -u, --, ordered child argv

*2026-09-23T15:55:52Z by Showboat 0.6.1*
<!-- showboat-id: cbd111fe-4bf9-43a8-8e16-18b3244f1d50 -->

Walkthrough for Issue #2 (`Notes/issues/002-cli-flag-allow-list-and-child-argv.md`), implementing the flag allow-list and protected child argv specified in `Notes/PRD-vrg.md` (Implementation Decisions → Invocation and child arguments, bullets 2–5 and 7; Module Design → CLI). It extends the Issue #1 CLI foundation: one shared option-declaration table drives mow.cli configuration, the ordered raw-token preflight, and generated help. All generated artifacts live in this directory (the built `vrg` binary, the `src/` fixture root, capture files).

```bash
cd /home/chris/vrg && go vet ./... && go build ./... && go test -count=1 ./internal/cli/ ./cmd/vrg/ | sed 's/[[:space:]][0-9.]*s$//' && echo GATES-OK
```

```output
ok  	vrg/internal/cli
ok  	vrg/cmd/vrg
GATES-OK
```

```bash
cd /home/chris/vrg && go build -o Notes/walkthroughs/002-04/code-walkthrough/vrg ./cmd/vrg && mkdir -p Notes/walkthroughs/002-04/code-walkthrough/src && ls Notes/walkthroughs/002-04/code-walkthrough
```

```output
src
vrg
walkthrough.md
```

Generated help now lists every allow-listed search flag in short and long form, rendered from the same `optionDecls` table that configures the parser and validates the scan — no separately maintained list.

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

Every allow-listed flag in both spellings forwards verbatim into the child argv: `rg --json --no-config <flags> -- <pattern> <root>`. Mandatory internal flags come first; the pattern and root sit behind `--`.

```bash
cd /home/chris/vrg/Notes/walkthroughs/002-04/code-walkthrough && for f in -i --ignore-case -S --smart-case -s --case-sensitive -w --word-regexp -x --line-regexp -F --fixed-strings --hidden --no-hidden --no-ignore -u --unrestricted -L --follow; do printf "%-16s -> " "vrg $f foo"; ./vrg "$f" foo; done
```

```output
vrg -i foo       -> search stub: rg --json --no-config -i -- foo .
vrg --ignore-case foo -> search stub: rg --json --no-config --ignore-case -- foo .
vrg -S foo       -> search stub: rg --json --no-config -S -- foo .
vrg --smart-case foo -> search stub: rg --json --no-config --smart-case -- foo .
vrg -s foo       -> search stub: rg --json --no-config -s -- foo .
vrg --case-sensitive foo -> search stub: rg --json --no-config --case-sensitive -- foo .
vrg -w foo       -> search stub: rg --json --no-config -w -- foo .
vrg --word-regexp foo -> search stub: rg --json --no-config --word-regexp -- foo .
vrg -x foo       -> search stub: rg --json --no-config -x -- foo .
vrg --line-regexp foo -> search stub: rg --json --no-config --line-regexp -- foo .
vrg -F foo       -> search stub: rg --json --no-config -F -- foo .
vrg --fixed-strings foo -> search stub: rg --json --no-config --fixed-strings -- foo .
vrg --hidden foo -> search stub: rg --json --no-config --hidden -- foo .
vrg --no-hidden foo -> search stub: rg --json --no-config --no-hidden -- foo .
vrg --no-ignore foo -> search stub: rg --json --no-config --no-ignore -- foo .
vrg -u foo       -> search stub: rg --json --no-config -u -- foo .
vrg --unrestricted foo -> search stub: rg --json --no-config --unrestricted -- foo .
vrg -L foo       -> search stub: rg --json --no-config -L -- foo .
vrg --follow foo -> search stub: rg --json --no-config --follow -- foo .
```

Ordering: repeated, combined, and mixed-alias options forward in exact encounter order with the supplied spellings, including options interleaved with both operands; combined shorts expand left to right; contradictory flags are never normalized.

```bash
cd /home/chris/vrg/Notes/walkthroughs/002-04/code-walkthrough && ok() { printf "%-38s -> " "vrg $*"; ./vrg "$@"; }; ok -iw foo src; ok foo -i src; ok -i -s -i foo; ok -isi foo; ok --ignore-case -s -i foo; ok foo -i src -s; ok -iwF foo; ok -i -s -S foo; ok --hidden --no-hidden foo
```

```output
vrg -iw foo src                        -> search stub: rg --json --no-config -i -w -- foo src
vrg foo -i src                         -> search stub: rg --json --no-config -i -- foo src
vrg -i -s -i foo                       -> search stub: rg --json --no-config -i -s -i -- foo .
vrg -isi foo                           -> search stub: rg --json --no-config -i -s -i -- foo .
vrg --ignore-case -s -i foo            -> search stub: rg --json --no-config --ignore-case -s -i -- foo .
vrg foo -i src -s                      -> search stub: rg --json --no-config -i -s -- foo src
vrg -iwF foo                           -> search stub: rg --json --no-config -i -w -F -- foo .
vrg -i -s -S foo                       -> search stub: rg --json --no-config -i -s -S -- foo .
vrg --hidden --no-hidden foo           -> search stub: rg --json --no-config --hidden --no-hidden -- foo .
```

Cumulative `-u`/`--unrestricted` counting spans separate, combined, and long spellings: at most two occurrences are accepted; the third — wherever it lands — is a sanitized exit-2 usage error with no child.

```bash
cd /home/chris/vrg/Notes/walkthroughs/002-04/code-walkthrough && run() { ./vrg "$@" >o.txt 2>e.txt; c=$?; printf "%-38s exit=%d out=%s err=%s\n" "vrg $*" "$c" "$(head -1 o.txt)" "$(head -1 e.txt)"; }; run -u foo; run -uu foo; run -iu foo; run -iuu foo; run -u --unrestricted foo; run --unrestricted --unrestricted foo; run -uuu foo; run -u -uu foo; run -iuuu foo; run -u --unrestricted -u foo; run --unrestricted -uu foo
```

```output
vrg -u foo                             exit=0 out=search stub: rg --json --no-config -u -- foo . err=
vrg -uu foo                            exit=0 out=search stub: rg --json --no-config -u -u -- foo . err=
vrg -iu foo                            exit=0 out=search stub: rg --json --no-config -i -u -- foo . err=
vrg -iuu foo                           exit=0 out=search stub: rg --json --no-config -i -u -u -- foo . err=
vrg -u --unrestricted foo              exit=0 out=search stub: rg --json --no-config -u --unrestricted -- foo . err=
vrg --unrestricted --unrestricted foo  exit=0 out=search stub: rg --json --no-config --unrestricted --unrestricted -- foo . err=
vrg -uuu foo                           exit=2 out= err=vrg: unrestricted option given more than twice: -uuu
vrg -u -uu foo                         exit=2 out= err=vrg: unrestricted option given more than twice: -uu
vrg -iuuu foo                          exit=2 out= err=vrg: unrestricted option given more than twice: -iuuu
vrg -u --unrestricted -u foo           exit=2 out= err=vrg: unrestricted option given more than twice: -u
vrg --unrestricted -uu foo             exit=2 out= err=vrg: unrestricted option given more than twice: -uu
```

Unsupported and argument-taking options are exit-2 usage errors — nothing reaches the child.

```bash
cd /home/chris/vrg/Notes/walkthroughs/002-04/code-walkthrough && run() { ./vrg "$@" >o.txt 2>e.txt; c=$?; printf "%-38s exit=%d out=%s err=%s\n" "vrg $*" "$c" "$(head -1 o.txt)" "$(head -1 e.txt)"; }; run -e foo; run foo -e; run --type go foo; run --type=go foo; run -t go foo; run -A 2 foo; run --context 2 foo; run --glob "*.go" foo; run -iz foo
```

```output
vrg -e foo                             exit=2 out= err=vrg: unsupported option -e
vrg foo -e                             exit=2 out= err=vrg: unsupported option -e
vrg --type go foo                      exit=2 out= err=vrg: unsupported option --type
vrg --type=go foo                      exit=2 out= err=vrg: unsupported option --type=go
vrg -t go foo                          exit=2 out= err=vrg: unsupported option -t
vrg -A 2 foo                           exit=2 out= err=vrg: unsupported option -A
vrg --context 2 foo                    exit=2 out= err=vrg: unsupported option --context
vrg --glob *.go foo                    exit=2 out= err=vrg: unsupported option --glob
vrg -iz foo                            exit=2 out= err=vrg: unsupported option -iz
```

Boolean assignment spellings are rejected lexically — the no-argument contract accepts no `=` form for any declared option, help included. The same bytes after the first `--` are ordinary positional operands.

```bash
cd /home/chris/vrg/Notes/walkthroughs/002-04/code-walkthrough && run() { ./vrg "$@" >o.txt 2>e.txt; c=$?; printf "%-38s exit=%d out=%s err=%s\n" "vrg $*" "$c" "$(head -1 o.txt)" "$(head -1 e.txt)"; }; run --ignore-case=false foo; run -i=false foo; run --unrestricted=false foo; run --help=false foo; run -h=false foo; run --help=true foo; run -- --ignore-case=false; run -- -i=false .; run -- --help=false; run -- --unrestricted=false src
```

```output
vrg --ignore-case=false foo            exit=2 out= err=vrg: unsupported option --ignore-case=false
vrg -i=false foo                       exit=2 out= err=vrg: unsupported option -i=false
vrg --unrestricted=false foo           exit=2 out= err=vrg: unsupported option --unrestricted=false
vrg --help=false foo                   exit=2 out= err=vrg: unsupported option --help=false
vrg -h=false foo                       exit=2 out= err=vrg: unsupported option -h=false
vrg --help=true foo                    exit=2 out= err=vrg: unsupported option --help=true
vrg -- --ignore-case=false             exit=0 out=search stub: rg --json --no-config -- --ignore-case=false . err=
vrg -- -i=false .                      exit=0 out=search stub: rg --json --no-config -- -i=false . err=
vrg -- --help=false                    exit=0 out=search stub: rg --json --no-config -- --help=false . err=
vrg -- --unrestricted=false src        exit=0 out=search stub: rg --json --no-config -- --unrestricted=false src err=
```

Operand protection: the empty pattern forwards as an empty argv element, `-` is a literal pattern, dash-leading patterns need `--`, and a second `--` is the verbatim pattern — not another terminator.

```bash
cd /home/chris/vrg/Notes/walkthroughs/002-04/code-walkthrough && run() { ./vrg "$@" >o.txt 2>e.txt; c=$?; printf "%-30s exit=%d out=[%s] err=%s\n" "vrg $*" "$c" "$(head -1 o.txt)" "$(head -1 e.txt)"; }; run "" .; run - .; run -- -foo; run -- -foo src; run -foo; run -- --; run -- -- .; run -- -i
```

```output
vrg  .                         exit=0 out=[search stub: rg --json --no-config --  .] err=
vrg - .                        exit=0 out=[search stub: rg --json --no-config -- - .] err=
vrg -- -foo                    exit=0 out=[search stub: rg --json --no-config -- -foo .] err=
vrg -- -foo src                exit=0 out=[search stub: rg --json --no-config -- -foo src] err=
vrg -foo                       exit=2 out=[] err=vrg: unsupported option -foo
vrg -- --                      exit=0 out=[search stub: rg --json --no-config -- -- .] err=
vrg -- -- .                    exit=0 out=[search stub: rg --json --no-config -- -- .] err=
vrg -- -i                      exit=0 out=[search stub: rg --json --no-config -- -i .] err=
```

Help precedence: a help spelling anywhere before `--` wins over search flags, unsupported options, excess `-u`, and bad roots — exit 0, exactly one help copy on stdout, empty stderr, no stub, no child, no terminal control bytes. Flags alone without a pattern remain the missing-pattern exit-2 error.

```bash
cd /home/chris/vrg/Notes/walkthroughs/002-04/code-walkthrough && check() { ./vrg "$@" >o.txt 2>e.txt; c=$?; u=$(grep -c "^Usage:" o.txt); s=$(wc -c <e.txt); st=$(grep -c "search stub:" o.txt); esc=$(cat o.txt e.txt | tr -d -c "\033" | wc -c); printf "%-30s exit=%d usage=%d stderr_bytes=%d stub=%d esc=%d\n" "vrg $*" "$c" "$u" "$s" "$st" "$esc"; }; check; check -h; check --help; check -i --help; check -ih foo; check -i -s --help; check -uuu --help; check -e --help; check foo src --help; check -- --help
```

```output
vrg                            exit=0 usage=1 stderr_bytes=0 stub=0 esc=0
vrg -h                         exit=0 usage=1 stderr_bytes=0 stub=0 esc=0
vrg --help                     exit=0 usage=1 stderr_bytes=0 stub=0 esc=0
vrg -i --help                  exit=0 usage=1 stderr_bytes=0 stub=0 esc=0
vrg -ih foo                    exit=0 usage=1 stderr_bytes=0 stub=0 esc=0
vrg -i -s --help               exit=0 usage=1 stderr_bytes=0 stub=0 esc=0
vrg -uuu --help                exit=0 usage=1 stderr_bytes=0 stub=0 esc=0
vrg -e --help                  exit=0 usage=1 stderr_bytes=0 stub=0 esc=0
vrg foo src --help             exit=0 usage=1 stderr_bytes=0 stub=0 esc=0
vrg -- --help                  exit=0 usage=0 stderr_bytes=0 stub=1 esc=0
```

```bash
cd /home/chris/vrg/Notes/walkthroughs/002-04/code-walkthrough && run() { ./vrg "$@" >o.txt 2>e.txt; c=$?; printf "%-30s exit=%d out=%s\n" "vrg $*" "$c" "$(head -1 o.txt)"; head -1 e.txt; }; run -i; run -i -s; run -u -u
```

```output
vrg -i                         exit=2 out=
vrg: missing required argument PATTERN
vrg -i -s                      exit=2 out=
vrg: missing required argument PATTERN
vrg -u -u                      exit=2 out=
vrg: missing required argument PATTERN
```

## Result. Every Issue #2 manual-verification class is demonstrated: all eleven allow-listed flags forward in both spellings; encounter order and supplied spellings survive repeated (`-i -s -i`), combined (`-isi`, `-iwF`), mixed-alias, and operand-interleaved options; cumulative `-u` accepts two across any token mix and rejects the third; unsupported (`-e`) and argument-taking options exit 2; every `=` assignment spelling is rejected lexically while the same bytes after `--` are positional; `--` protects empty, `-`, dash-leading, and literal `--` patterns; and the child argv is exactly `rg --json --no-config <ordered flags> -- <pattern> <root>`. Issue #1 help regressions remain intact: bare/first-token/mixed/combined/later help emit one stdout copy with empty stderr, exit 0, no child and no TUI; help-like tokens after `--` stay positional; flags-only invocations stay missing-pattern exit 2. Suites: `internal/cli` (TestAcceptedSearchFlags, TestFlagEncounterOrder, TestUnrestrictedCumulativeLimit, TestRejectedOptions, TestAssignmentSpellings*, TestTerminatorAndProtectedPatterns, TestGeneratedHelpListsSearchFlags, TestSharedDeclarationsDriveHelpAndScan) and `cmd/vrg` (TestChildArgvStub, TestFlagAndArgvUsageErrors, TestHelpWithSearchFlags, TestHelpAssignmentSpellingsAreUsageErrorsAtBoundary).
