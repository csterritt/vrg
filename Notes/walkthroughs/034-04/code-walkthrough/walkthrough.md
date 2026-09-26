# Issue #34: Documentation — README, help footer note, and synchronization tests

*2026-09-24T20:13:23Z by Showboat 0.6.1*
<!-- showboat-id: c65b87d8-71da-4ad7-a8d1-0c212df0ae11 -->

Walkthrough for [Issue #34](../../../issues/034-documentation-scale-and-memory-limits.md), delivering the user-facing documentation per `Notes/PRD-vrg.md` (*Resources and responsiveness*, *Out of Scope*, and *Further Notes*; the documented behaviour itself is *Invocation and child arguments*, *Outcome and exit-status contract*, and *Colours, overlays, and key precedence*). The repository-root `README.md` is the single user-facing artifact: invocation syntax and option-placement rules, the verbatim generated help (the shared `optionDecls` declarations' own output), the full `helpBindings` table, the four-row exit-status table agreeing with Issue #9's `decideOutcome` (both exit-0 reasons and the `vrg -i` flags-only usage error), the three independent scale examples with their not-simultaneous qualification, the 64 MiB record limit with the base64 `bytes` caveat and path-naming oversized diagnostic, the session-long memory-retention and no-cleanup statements, and the ripgrep 15.x /`--no-config` contract. The help overlay's footer slot is filled by `limitNotes` in `internal/app/help.go` — the structured source rendered verbatim into both sinks — and the Issue #6 shared sink-safety table gained a `help footer note` row driving the hostile fixtures through every runtime-substitution point of the rendered footer. All generated artifacts live in this directory: the built `vrg` binary, the `demo-footer.sh` tmux harness, and its generated fake-rg script and `run/` session captures. Test durations are stripped so the document verifies cleanly.

```bash
set -o pipefail; cd /home/chris/vrg && test -z "$(gofmt -l internal/ cmd/)" && echo GOFMT-CLEAN && go vet ./... && go build ./... && go test -count=1 ./... | sed "s/[[:space:]][0-9.]*s$//" && CGO_ENABLED=1 go test -race -count=1 ./internal/app | sed "s/[[:space:]][0-9.]*s$//" && go build -o Notes/walkthroughs/034-04/code-walkthrough/vrg ./cmd/vrg && echo GATES-OK
```

```output
GOFMT-CLEAN
ok  	vrg/cmd/vrg
ok  	vrg/internal/app
ok  	vrg/internal/cli
ok  	vrg/internal/filebuffer
ok  	vrg/internal/present
ok  	vrg/internal/searchindex
ok  	vrg/internal/theme
ok  	vrg/internal/viewport
ok  	vrg/internal/app
GATES-OK
```

## Synchronization tests — README against the shared sources

Task 1's contract tests keep the committed `README.md` honest against the implementation's own tables. In `internal/app/readme_test.go`: `TestREADMEDocumentsEveryKeyBinding` iterates Issue #31's `helpBindings` and requires every key spelling and description in the README; `TestREADMEExitStatusDocumentation` requires the `0`/`1`/`2`/`130` rows and drives representative completed searches through the real `decideOutcome`, asserting every status it produces has a documented row, plus the trigger tokens — both exit-0 reasons, the `vrg -i` flags-only usage error, `q`-while-searching and `ctrl+c` cancellation; `TestREADMEDocumentsHelpOnlyPath` pins the bare-`vrg` and `-h`/`--help` help-only contract and its distinction from the TUI's `h`/`?` dialog; `TestREADMERipgrepReference`, `TestREADMEScaleExamples`, `TestREADMERecordLimit`, and `TestREADMEMemoryLimits` pin the AC1–AC3 statements; and `TestHelpFooterMatchesREADME` asserts every installed `helpFooter` entry appears verbatim in both the composed help text and the README — deleting a statement from either sink fails. In `internal/cli/readme_test.go` (same package): `TestREADMEDocumentsEveryDeclaredOption` iterates `optionDecls` and requires every declared spelling — the allow-listed search flags and the local `-h`/`--help` — with its description and generated option line; `TestREADMECarriesGeneratedHelp` requires the complete `HelpText()` block verbatim; `TestREADMEInvocationContract` pins the synopsis and the assignment-rejection statement.

```bash
cd /home/chris/vrg && go test -count=1 -v -run "README|TestHelpFooterMatchesREADME" ./internal/app ./internal/cli 2>&1 | grep -vE "^(=== RUN|=== CONT)" | sed -E "s/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//"
```

```output
--- PASS: TestREADMEDocumentsEveryKeyBinding
--- PASS: TestREADMEExitStatusDocumentation
--- PASS: TestREADMEDocumentsHelpOnlyPath
--- PASS: TestREADMERipgrepReference
--- PASS: TestREADMEScaleExamples
--- PASS: TestREADMERecordLimit
--- PASS: TestREADMEMemoryLimits
--- PASS: TestHelpFooterMatchesREADME
PASS
ok  	vrg/internal/app
--- PASS: TestREADMEDocumentsEveryDeclaredOption
--- PASS: TestREADMECarriesGeneratedHelp
--- PASS: TestREADMEInvocationContract
PASS
ok  	vrg/internal/cli
```

## Sink safety — the Issue #34 row

The Issue #6 shared sink-safety table gained the `help footer note` row: `renderHelpFooterSink` substitutes each hostile fixture (OSC title-set, CSI erase, C0/C1 controls, DEL, bare CR, invalid-UTF-8 path bytes, embedded-filename newline) at every runtime-substitution point of the rendered footer — appended to each installed `limitNotes` entry plus a dedicated injection line — opens the real overlay with `?`, scrolls to the footer's rows, and the shared check asserts no fixture control byte survives in the no-style raw output, that the `present.Diagnostic`-escaped forms appear inside the border, and that the real note's `64 MiB` text is still present. A styled pass then proves the fixture payload never follows an unescaped ESC.

```bash
cd /home/chris/vrg && go test -count=1 -v -run "TestSinkSafetyTable/help_footer_note" ./internal/app 2>&1 | grep -vE "^(=== RUN|=== CONT)" | sed -E "s/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//"
```

```output
--- PASS: TestSinkSafetyTable
    --- PASS: TestSinkSafetyTable/help_footer_note/osc_title_set
    --- PASS: TestSinkSafetyTable/help_footer_note/csi_erase
    --- PASS: TestSinkSafetyTable/help_footer_note/c0_controls
    --- PASS: TestSinkSafetyTable/help_footer_note/c1_nel
    --- PASS: TestSinkSafetyTable/help_footer_note/delete
    --- PASS: TestSinkSafetyTable/help_footer_note/standalone_carriage_return
    --- PASS: TestSinkSafetyTable/help_footer_note/invalid_utf-8_path_bytes
    --- PASS: TestSinkSafetyTable/help_footer_note/embedded_filename_newline
PASS
ok  	vrg/internal/app
```

## Invocation and exit statuses — the real binary

`check-cli.sh` (checked into this directory) verifies the README's command-line claims against the freshly built binary: the README's fenced `text` block is byte-identical to `vrg --help` output, bare `vrg` prints the same help to stdout and exits 0 with nothing on stderr — the help-only path, no search, no TUI — and the documented usage-error exits hold: `vrg -i` (flags-only, missing pattern), `--ignore-case=false` (an `=` assignment spelling rejected by the no-argument contract), and a nonexistent root all exit 2 with the sanitized diagnostic plus generated help on stderr.

```bash
cd /home/chris/vrg/Notes/walkthroughs/034-04/code-walkthrough && ./check-cli.sh
```

```output
ok: vrg --help exit status -> 0
ok: README help block == generated help -> identical
ok: --help stderr empty -> empty
ok: bare vrg exit status -> 0
ok: bare vrg stdout == --help -> identical
ok: bare vrg stderr empty -> empty
ok: vrg -i exit status -> 2
ok: vrg -i stdout empty -> empty
ok: vrg -i diagnostic -> vrg: missing required argument PATTERN
ok: vrg -i stderr carries help -> 1
ok: vrg --ignore-case=false exit status -> 2
ok: assignment rejected -> vrg: unsupported option --ignore-case=false
ok: vrg bad-root exit status -> 2
ok: bad-root diagnostic -> vrg: invalid root /definitely/not/a/root: does not exist
check-cli: all checks passed
```

## The footer note in the overlay — the real binary on a PTY

`demo-footer.sh` (checked into this directory) runs the built `vrg` on a real tmux PTY at 80×24 with a generated fake-`rg` script on `PATH` — repository and user files are never touched; fixtures live in disposable `mktemp` directories an EXIT trap removes. The fake rg emits a complete two-file stream and exits 0, so browse opens on a.txt with no overlay in the way. `?` opens the help dialog — the *Key bindings* title and table render, with the footer note's tail below the fold — then forty `Down` presses scroll to the tail, where all three `limitNotes` entries are visible inside the border: the independent scale examples, the ~50 MB / base64 / 64 MiB record-limit statement, and the session-retention / no-cleanup statement. `q` closes the dialog back to browse, and a second `q` quits with the fixed status 0.

```bash
cd /home/chris/vrg/Notes/walkthroughs/034-04/code-walkthrough && unset RIPGREP_CONFIG_PATH && ./demo-footer.sh
```

```output
ok: ?: help opens over browse
ok: ?: note tail below the fold
ok: Down x40: title scrolled off
ok: footer: scale examples note
ok: footer: 64 MiB record limit
ok: footer: terminal cleanup limit
ok: q: help closed
ok: q: browse restored
ok: vrg exit status -> 0
demo-footer: all checks passed
```

The captured tail frame — `run/help-tail.txt`, regenerated by the demo on every run — shows the footer note inside the border exactly as `helpLines` composed it:

```bash
cat /home/chris/vrg/Notes/walkthroughs/034-04/code-walkthrough/run/help-tail.txt
```

```output
┌──────────────────────────────────────────────────────────────────────────────┐
│[ / ]              pan half the text width                                    │
│w                  toggle line wrapping                                       │
│c                  toggle colour scheme                                       │
│left / tab         hide the file list                                         │
│right / shift+tab  show the file list                                         │
│r                  reload the current file                                    │
│h / ?              open or close this help                                    │
│q                  close the overlay / quit                                   │
│esc                close the overlay                                          │
│ctrl+c             exit immediately                                           │
│                                                                              │
│Scale examples — independent, not simultaneous capacity guarantees:           │
│approximately 10,000 matched files, 100,000 matched lines, or individual files│
│around 50 MB.                                                                 │
│The ~50 MB file example assumes UTF-8 content with ordinary line lengths; a   │
│line ripgrep must emit as base64 bytes expands by roughly a third, so a single│
│match record can exceed the 64 MiB record limit — oversized records are       │
│skipped and reported, naming the file's path when recoverable.                │
│Loaded file buffers are retained for the whole session: no eviction, no       │
│aggregate memory bound, and no reliable OOM recovery — large searches or many │
│visited files can exhaust memory, and forced termination cannot guarantee     │
│terminal cleanup.                                                             │
└──────────────────────────────────────────────────────────────────────────────┘
```

## README review against the implementation

Reading the committed `README.md` section by section against the code: **invocation** — the `vrg [flags] pattern [root]` synopsis, the `.` root default, the `-` rejection, option placement before `--`, encounter-order forwarding with supplied spellings, left-to-right combined-short expansion, and the cumulative `-u` cap all match `cli.Parse`'s scan in `internal/cli/cli.go`. **Flags** — the embedded help block is the `optionDecls` declarations' own generated output (proven byte-identical above), so the allow-list's exact no-argument spellings and the local `-h`/`--help` options cannot drift from parsing; the `=`-rejection statement matches the `unsupported option` diagnostic. **Bindings** — the key table matches `helpBindings` in `internal/app/help.go` row for row (the test iterates it). **Exit statuses** — all four documented triggers agree with `decideOutcome` in `internal/app/overlay.go` and the CLI layer: 0 for successful browse *and* command-line help (verified on the binary), 1 for no usable results, 2 for pre-TUI usage/root/start failures and fatal search outcomes, 130 for `q`-during-search and `ctrl+c` cancellation. **Scale** — the three examples are stated as independent, not simultaneous guarantees. **Record limit** — 64 MiB per JSON record with the base64 `bytes` expansion caveat and the path-naming oversized diagnostic, matching Issue #10's stream-integrity implementation. **Memory** — session-long retention, no eviction, no aggregate bound, no reliable OOM recovery, no cleanup guarantee under forced termination — matching `filebuffer`'s retention design. **ripgrep** — the 15.x reference family and the always-supplied `--no-config` match the child argv construction. The footer note renders from the shared `limitNotes` source into both sinks, and the sink-safety row proves hostile runtime strings cannot escape through it. Wiki ingest lives in `Notes/wiki/documentation.md`.
