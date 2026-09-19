# Issue #34: the README and help footer — scale, record limits, and memory

*2026-09-18T03:15:30Z by Showboat 0.6.1*
<!-- showboat-id: f35e6cfa-dd34-404d-bb4b-6d3aed016f3e -->

Issue #34 lands the user-facing documentation: a committed README.md at the repository root plus the Issue #31 help dialog's footer note, both rendered from a new shared structured source — internal/docs — so neither sink can drift from the other or from the implementation tables they document. The README covers invocation and the help-only exit-0 path (bare vrg and -h/--help print command-line help to stdout, no search, no TUI — distinct from the TUI's h/? dialog), the allow-listed no-argument flags asserted against the CLI's shared option declarations, the key bindings asserted against Issue #31's helpBindings table, the complete exit-status table for 0/1/2/130 agreeing with the Issue #9 outcome function, the ripgrep 15.x reference family with --no-config, and the scale-and-memory-limits section: three independent scale examples (about 10,000 matched files, about 100,000 matched lines, individual files around 50 MB), explicitly not simultaneous capacity guarantees; the ~50 MB UTF-8 assumption with the base64 bytes caveat that can push one match record over the 64 MiB limit; the oversized-record diagnostic naming the path when recoverable; and session-long buffer retention with no eviction, no aggregate memory bound, no reliable OOM recovery, and no forced-termination cleanup guarantee. The footer renders the same limits through safepresentation.EscapeDiagnostic, so the sink-safety table's new 'help footer note' row drives the Issue #6 hostile fixtures through it. See Notes/issues/034-documentation-scale-and-memory-limits.md, Notes/tasks/034-documentation-scale-and-memory-limits.md, and the 'Resources and responsiveness' and 'Out of Scope' sections of Notes/PRD-vrg.md. All artifacts live in this directory.

## Gates — module integrity, build, vet

```bash
cd /home/chris/vrg && go mod verify && go build ./... && go vet ./... && echo GATES-OK
```

```output
all modules verified
GATES-OK
```

## Synchronization tests — the README cannot drift from the implementation

The Issue #34 tests pin the committed README to the same data sources the code consumes. internal/docs/docs_test.go asserts the README embeds docs.Limits() verbatim — the shared limits text the footer also renders — and that docs.Footer() carries every required statement. internal/app/readme_test.go iterates Issue #31's helpBindings table requiring each binding in the README, asserts the exit-status documentation covers 0/1/2/130 with its search-derived values agreeing with app.DecideOutcome, and requires the help footer to share the README's limits statements. internal/cli/readme_test.go iterates the CLI's shared optionDecls — the same declarations parsing and generated help consume — requiring each allow-listed flag and the local -h/--help options in the README's flag table, plus the help-only path, the ripgrep 15.x and --no-config statements.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestReadmeLimitsSection' ./internal/docs && go test -count=1 -v -run 'TestReadme|TestHelpFooterSharesReadmeLimits' ./internal/app && go test -count=1 -v -run 'TestReadme|TestHelpRendersEveryDeclaredOption' ./internal/cli 2>&1 | grep -E '^(--- |=== RUN Test|ok|FAIL)' | grep -v '=== RUN'
```

```output
=== RUN   TestReadmeLimitsSection
--- PASS: TestReadmeLimitsSection (0.00s)
PASS
ok  	vrg/internal/docs	0.001s
=== RUN   TestReadmeListsEveryBinding
--- PASS: TestReadmeListsEveryBinding (0.00s)
=== RUN   TestHelpFooterSharesReadmeLimits
--- PASS: TestHelpFooterSharesReadmeLimits (0.00s)
=== RUN   TestReadmeExitStatuses
--- PASS: TestReadmeExitStatuses (0.00s)
PASS
ok  	vrg/internal/app	0.024s
--- PASS: TestHelpRendersEveryDeclaredOption (0.00s)
--- PASS: TestReadmeFlagTable (0.00s)
--- PASS: TestReadmeHelpOnlyPath (0.00s)
--- PASS: TestReadmeRipgrepFamily (0.00s)
ok  	vrg/internal/cli	0.002s
```

## Sink safety — the 'help footer note' row against the hostile fixtures

Issue #34 adds a row to the Issue #6 sink-safety table in internal/app/sinksafety_test.go: 'help footer note' substitutes every hostile fixture — OSC title-set, CSI erase, C0 run, C1 NEL, DEL, standalone CR, invalid UTF-8, embedded filename newline — through helpText()'s footer slot, the real runtime-substitution point routed through safepresentation.EscapeDiagnostic, and asserts the raw no-style output contains no fixture control bytes. Static documentation text needs no sanitization; the substitution does.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestSinkSafetyTable/help_footer_note' ./internal/app 2>&1 | grep -E '(--- |ok |FAIL)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestSinkSafetyTable
    --- PASS: TestSinkSafetyTable/help_footer_note/osc_title-set
    --- PASS: TestSinkSafetyTable/help_footer_note/csi_erase
    --- PASS: TestSinkSafetyTable/help_footer_note/c0_run
    --- PASS: TestSinkSafetyTable/help_footer_note/c1_nel
    --- PASS: TestSinkSafetyTable/help_footer_note/del
    --- PASS: TestSinkSafetyTable/help_footer_note/standalone_cr
    --- PASS: TestSinkSafetyTable/help_footer_note/invalid_utf-8
    --- PASS: TestSinkSafetyTable/help_footer_note/embedded_filename_newline
ok  	vrg/internal/app
```

## README read-through — invocation and flags against the CLI declarations

The Invocation section states the syntax 'vrg [flags] pattern [root]' with root defaulting to '.', the help-only path — no arguments or -h/--help print command-line help on stdout with exit 0, no search, no TUI, distinct from the TUI's h/? dialog — and the flags-only usage error 'vrg -i' exiting 2. The Flags table lists only the allow-listed no-argument options; internal/cli/readme_test.go iterates optionDecls in internal/cli/cli.go — the declarations parsing and generated help share — so the table cannot drift. Review: every optionDecl spelling appears in the table, the rejection of assignment spellings matches the parser's pre-parse guard, and '--' ends option parsing as documented.

```bash
cd /home/chris/vrg && sed -n '11,51p' README.md && echo '--- optionDecls (internal/cli/cli.go) ---' && sed -n '/^var optionDecls/,/^}/p' internal/cli/cli.go
```

```output
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
--- optionDecls (internal/cli/cli.go) ---
var optionDecls = []optionDecl{
	{short: 'h', long: "help", help: true, desc: "Show command-line help and exit."},
	{short: 'i', long: "ignore-case", desc: "Case-insensitive search."},
	{short: 'S', long: "smart-case", desc: "Smart case search."},
	{short: 's', long: "case-sensitive", desc: "Case-sensitive search."},
	{short: 'w', long: "word-regexp", desc: "Match whole words only."},
	{short: 'x', long: "line-regexp", desc: "Match whole lines only."},
	{short: 'F', long: "fixed-strings", desc: "Treat the pattern as a literal string."},
	{long: "hidden", desc: "Search hidden files and directories."},
	{long: "no-hidden", desc: "Do not search hidden files and directories."},
	{long: "no-ignore", desc: "Do not respect ignore files."},
	{short: 'u', long: "unrestricted", unrestricted: true, desc: "Reduce filtering; may be supplied at most twice."},
	{short: 'L', long: "follow", desc: "Follow symbolic links."},
}
```

## README read-through — key bindings and exit status against the model

The Key bindings table is asserted row-for-row against Issue #31's helpBindings — the table the TUI help dialog renders and the key router consumes — so the documented keys are the routed keys. The Exit status table covers 0, 1, 2, and 130: both reasons for exit 0 (successful browse and command-line help), no usable results at 1, pre-TUI usage/root/start failures and fatal search outcomes at 2, and the two cancellation triggers at 130 — its search-derived values agree with app.DecideOutcome, which TestReadmeExitStatuses exercises. Review: every helpBindings row appears verbatim, and each documented status maps to the outcomes DecideOutcome returns.

```bash
cd /home/chris/vrg && sed -n '53,83p' README.md && echo '--- helpBindings (internal/app/help.go) ---' && sed -n '/^var helpBindings/,/^}/p' internal/app/help.go && echo '--- DecideOutcome cases (internal/app) ---' && grep -rn 'func DecideOutcome' internal/app/*.go
```

```output
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
--- helpBindings (internal/app/help.go) ---
var helpBindings = []helpBinding{
	{"n / p", "next / previous matched line"},
	{"up / down", "scroll one row"},
	{"u / d", "scroll half a page"},
	{"pgup / pgdn", "scroll a full page"},
	{", / .", "pan one column"},
	{"< / >", "pan ten columns"},
	{"[ / ]", "pan half the text width"},
	{"left / tab", "hide the file list"},
	{"right / shift+tab", "show the file list"},
	{"w", "toggle wrapping"},
	{"c", "toggle the colour scheme"},
	{"r", "reload the current file"},
	{"h / ?", "open or close this help"},
	{"q / esc", "dismiss an overlay or quit"},
	{"ctrl+c", "exit immediately"},
}
--- DecideOutcome cases (internal/app) ---
internal/app/outcome.go:81:func DecideOutcome(in OutcomeInput) Outcome {
```

## README read-through — scale, record limit, and memory against the shared source

The Scale and memory limits section embeds docs.Limits() verbatim — TestReadmeLimitsSection requires the committed README to contain the shared source's rendered text, so the review compares the README tail against internal/docs/docs.go's statements: three independent scale examples explicitly not simultaneous capacity guarantees, the ~50 MB UTF-8 assumption and the base64 bytes caveat that can push one match record over the 64 MiB limit, the oversized-record skip with its path diagnostic, and session-long buffer retention — no eviction, no aggregate bound, no reliable OOM recovery, no forced-termination cleanup guarantee. Review: every README sentence is a docs.go statement.

```bash
cd /home/chris/vrg && sed -n '85,$p' README.md && echo '--- docs.go (internal/docs/docs.go) ---' && cat internal/docs/docs.go
```

```output
## Scale and memory limits

The documented scale examples are independent, not simultaneous capacity guarantees.

- about 10,000 matched files
- about 100,000 matched lines
- individual files around 50 MB

The ~50 MB file example assumes UTF-8 (or near-UTF-8) content with ordinary line lengths: a single very long line that ripgrep must emit as base64 bytes expands by roughly a third in the JSON stream and can push one match record over the 64 MiB limit even when the file itself is under 50 MB.

Every JSON record is limited to 64 MiB: an oversized record is skipped and counted, never silently truncated, and the oversized-record diagnostic names the affected file's path when it can be recovered.

Loaded file buffers are retained for the whole session: no eviction, no aggregate memory bound, and no reliable OOM recovery; large searches or many visited files may exhaust memory and terminate the process, and forced termination cannot guarantee terminal cleanup.
--- docs.go (internal/docs/docs.go) ---
// Package docs is the shared structured source for vrg's user-facing
// documentation (Issue #34): the scale, record-limit, and memory
// statements that both the repository README and the TUI help-overlay
// footer render, so neither sink can drift from the other. The
// documentation synchronization tests assert each statement lands
// verbatim in both sinks.
package docs

import "strings"

const (
	// ScaleStatement introduces the scale examples: each stands alone,
	// never a simultaneous capacity guarantee.
	ScaleStatement = "The documented scale examples are independent, not simultaneous capacity guarantees."
	// AssumptionStatement qualifies the ~50 MB example's content:
	// UTF-8 (or near-UTF-8) with ordinary line lengths. A single very
	// long line that ripgrep must emit as base64 bytes expands by
	// roughly a third in the JSON stream and can push one match record
	// over the limit even when the file itself is under 50 MB.
	AssumptionStatement = "The ~50 MB file example assumes UTF-8 (or near-UTF-8) content with ordinary line lengths: a single very long line that ripgrep must emit as base64 bytes expands by roughly a third in the JSON stream and can push one match record over the 64 MiB limit even when the file itself is under 50 MB."
	// RecordLimitStatement states the per-record bound and the
	// oversized-record disposition: skipped and counted, never
	// truncated, the path named when recoverable.
	RecordLimitStatement = "Every JSON record is limited to 64 MiB: an oversized record is skipped and counted, never silently truncated, and the oversized-record diagnostic names the affected file's path when it can be recovered."
	// MemoryStatement states the retention and termination limits:
	// session-long buffers, no eviction, no aggregate bound, no
	// reliable OOM recovery, no guaranteed terminal cleanup under
	// forced termination.
	MemoryStatement = "Loaded file buffers are retained for the whole session: no eviction, no aggregate memory bound, and no reliable OOM recovery; large searches or many visited files may exhaust memory and terminate the process, and forced termination cannot guarantee terminal cleanup."
)

// ScaleItems are the three independent scale examples ScaleStatement
// introduces; each is a standalone example, not a combined capacity.
var ScaleItems = []string{
	"about 10,000 matched files",
	"about 100,000 matched lines",
	"individual files around 50 MB",
}

// Statements is the ordered required-statement list the documentation
// synchronization tests iterate: each statement must survive verbatim
// in every sink, so deleting one fails the suite.
var Statements = []string{
	ScaleStatement,
	AssumptionStatement,
	RecordLimitStatement,
	MemoryStatement,
}

// Footer renders the TUI help-overlay footer note from the shared
// source: the scale statement and items followed by the assumption,
// record-limit, and memory statements as one compact paragraph under
// the binding table. The help renderer treats the note as substituted
// text and routes it through the safe-presentation diagnostic escaper
// like every runtime string.
func Footer() string {
	items := make([]string, len(ScaleItems))
	copy(items, ScaleItems)
	items[len(items)-1] = "and " + items[len(items)-1]
	return "Scale and memory limits — " + ScaleStatement + " They are " +
		strings.Join(items, ", ") + ". " +
		AssumptionStatement + " " + RecordLimitStatement + " " + MemoryStatement
}

// Limits renders the README's scale-and-memory-limits section body
// from the same shared source: the committed README must carry this
// text verbatim — the synchronization test asserts it — so the section
// cannot drift from the source or from the help footer.
func Limits() string {
	var b strings.Builder
	b.WriteString(ScaleStatement + "\n\n")
	for _, item := range ScaleItems {
		b.WriteString("- " + item + "\n")
	}
	b.WriteString("\n" + AssumptionStatement + "\n\n" +
		RecordLimitStatement + "\n\n" + MemoryStatement + "\n")
	return b.String()
}
```

## Manual check — the footer note rendering in the real overlay

manual_route.sh runs the issue's manual route against a disposable mktemp fixture — never a repository file: aa.txt and bb.txt with 'foo' matches, under a trap removing the tree on exit or interruption. pty_footer.py drives one session on a real 60x14 pty ('vrg foo .'): '?' opens the bordered 'Key bindings' box over the browse view; down scrolls until the Issue #34 footer note's head is in frame — 'Scale and memory limits' with its independence statement — then until the three scale examples are in frame; down x30 reaches the bottom of the complete wrapped table where the 64 MiB record limit and the session-retention/no-cleanup statements close the note with the title row scrolled off. The overlay wraps cell-wise, splitting words across rows, so the assertions compare on the whitespace- and border-free compaction of each frame. Esc closes back to the browse view and q exits 0.

```bash
cd /home/chris/vrg && go build -o Notes/walkthroughs/034-04/code-walkthrough/vrg ./cmd/vrg && timeout 120 bash Notes/walkthroughs/034-04/code-walkthrough/manual_route.sh
```

```output
fixture: <tmp> (aa.txt, bb.txt — 'foo' matches)
?           : bordered 'Key bindings' over the browse view
down        : footer note head — scale, independent
down        : scale examples — 10k files, 100k lines, 50 MB
down x30    : footer note tail — 64 MiB limit, memory
Esc         : help closed — the browse view is back
q           : exit 0
OK
```

## Full module regression

The shared docs package, the help-footer wiring, and the new tests touch packages every overlay and the CLI consume — the whole suite runs, plus the race detector over internal/app.

```bash
cd /home/chris/vrg && go test -count=1 ./internal/... ./cmd/vrg 2>&1 | sed -E 's/\t[0-9.]+s$//' && CGO_ENABLED=1 go test -race -count=1 ./internal/app 2>&1 | sed -E 's/\t[0-9.]+s$//'
```

```output
ok  	vrg/internal/app
ok  	vrg/internal/cli
ok  	vrg/internal/docs
ok  	vrg/internal/filebuffer
ok  	vrg/internal/safepresentation
?   	vrg/internal/safepresentation/sinktest	[no test files]
ok  	vrg/internal/searchindex
ok  	vrg/internal/theme
ok  	vrg/internal/viewport
ok  	vrg/cmd/vrg
ok  	vrg/internal/app
```

Artifacts in this directory: vrg (the built binary the pty session ran), manual_route.sh (the disposable-fixture driver), pty_footer.py (the real-pty session and assertions — including DECSTBM-region scrolling and cell-wise-wrap compaction, which is how vrg's differential renderer repaints scrolled overlays).
