# Issue #6: safe-presentation utility for every output sink

*2026-09-16T20:45:04Z by Showboat 0.6.1*
<!-- showboat-id: 9cc70cf8-cb61-4e7e-a1ae-89396b24a43b -->

Issue #6 generalizes the Issue #5 safe-presentation core into a single shared utility in `internal/safepresentation` that every output sink routes through: `EscapePath` for paths/filenames, `MapContent` for content lines with byte-to-cell maps, and the new `EscapeDiagnostic` for whole diagnostic lines and generated help (real LF boundaries preserved, CRLF normalized, tabs expanded to eight-column stops, other controls escaped, embedded external strings single-line-escaped first). The Issue #1 `cli.Escape` is deleted; usage errors and generated CLI help now flow through the shared utility, and a reusable sink-safety table (`internal/safepresentation/sinktest` + `internal/app/sinksafety_test.go`) re-runs one hostile fixture set against every sink without duplication. See `Notes/issues/006-safe-presentation-utility-for-all-sinks.md`, `Notes/tasks/006-safe-presentation-utility-for-all-sinks.md`, and the PRD section "Text, graphemes, and safe presentation" in `Notes/PRD-vrg.md`. All artifacts live in this directory.

## Gates — module integrity, build, vet

```bash
cd /home/chris/vrg && go mod verify && go build ./... && go vet ./... && echo GATES-OK
```

```output
all modules verified
GATES-OK
```

## Full suite — `go test ./...`

```bash
cd /home/chris/vrg && go test -count=1 ./... 2>&1 | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
ok  	vrg/cmd/vrg
ok  	vrg/internal/app
ok  	vrg/internal/cli
ok  	vrg/internal/filebuffer
ok  	vrg/internal/safepresentation
?   	vrg/internal/safepresentation/sinktest	[no test files]
ok  	vrg/internal/searchindex
?   	vrg/internal/theme	[no test files]
?   	vrg/internal/viewport	[no test files]
```

## Unit tests — path, content, and diagnostic rules

`internal/safepresentation` runs the Issue #5 path/content escaping tests unchanged, plus the new `EscapeDiagnostic` tests covering real line boundaries (LF kept, CRLF normalized), eight-column tab expansion, control/invalid-byte escaping, and single-line-escaped embedded filenames.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/safepresentation 2>&1 | grep -E '^(--- (PASS|FAIL)|ok|FAIL)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestEscapeDiagnostic
--- PASS: TestEscapeDiagnosticEmbeddedFilename
--- PASS: TestEscapeDiagnosticEmitsNoRawControls
--- PASS: TestEscapePath
--- PASS: TestEscapePathEmitsNoRawControls
--- PASS: TestMapContentPlainText
--- PASS: TestMapContentEscapes
--- PASS: TestMapContentStandaloneCR
--- PASS: TestMapContentTabPlaceholder
--- PASS: TestMapContentByteCellMaps
--- PASS: TestMapContentWideGlyph
--- PASS: TestCellsCovering
--- PASS: TestCellsCoveringEscapedForms
ok  	vrg/internal/safepresentation
```

## Shared sink-safety table — five sinks × one hostile fixture set

`TestSinkSafetyTable` re-runs the single hostile fixture set (OSC, CSI, the C0 run, C1, DEL, standalone CR, invalid UTF-8, embedded filename newline) against every existing sink — file-list entry, filename rule, panel content, usage-error stderr, and generated CLI-help stdout. Each row renders once with no styles (asserts on the raw output, where no escape byte is legitimate) and once styled (asserts the hostile payload never occurs immediately after an unescaped ESC). Fixtures are defined once in `internal/safepresentation/sinktest`; future issues add rows, not fixture copies.

```bash
cd /home/chris/vrg && go test -count=1 -v -run TestSinkSafetyTable ./internal/app 2>&1 | grep -E '^(    --- (PASS|FAIL)|--- (PASS|FAIL)|ok|FAIL)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestSinkSafetyTable
    --- PASS: TestSinkSafetyTable/file-list_entry/osc_title-set
    --- PASS: TestSinkSafetyTable/file-list_entry/csi_erase
    --- PASS: TestSinkSafetyTable/file-list_entry/c0_run
    --- PASS: TestSinkSafetyTable/file-list_entry/c1_nel
    --- PASS: TestSinkSafetyTable/file-list_entry/del
    --- PASS: TestSinkSafetyTable/file-list_entry/standalone_cr
    --- PASS: TestSinkSafetyTable/file-list_entry/invalid_utf-8
    --- PASS: TestSinkSafetyTable/file-list_entry/embedded_filename_newline
    --- PASS: TestSinkSafetyTable/filename_rule/osc_title-set
    --- PASS: TestSinkSafetyTable/filename_rule/csi_erase
    --- PASS: TestSinkSafetyTable/filename_rule/c0_run
    --- PASS: TestSinkSafetyTable/filename_rule/c1_nel
    --- PASS: TestSinkSafetyTable/filename_rule/del
    --- PASS: TestSinkSafetyTable/filename_rule/standalone_cr
    --- PASS: TestSinkSafetyTable/filename_rule/invalid_utf-8
    --- PASS: TestSinkSafetyTable/filename_rule/embedded_filename_newline
    --- PASS: TestSinkSafetyTable/panel_content/osc_title-set
    --- PASS: TestSinkSafetyTable/panel_content/csi_erase
    --- PASS: TestSinkSafetyTable/panel_content/c0_run
    --- PASS: TestSinkSafetyTable/panel_content/c1_nel
    --- PASS: TestSinkSafetyTable/panel_content/del
    --- PASS: TestSinkSafetyTable/panel_content/standalone_cr
    --- PASS: TestSinkSafetyTable/panel_content/invalid_utf-8
    --- PASS: TestSinkSafetyTable/panel_content/embedded_filename_newline
    --- PASS: TestSinkSafetyTable/usage-error_stderr/osc_title-set
    --- PASS: TestSinkSafetyTable/usage-error_stderr/csi_erase
    --- PASS: TestSinkSafetyTable/usage-error_stderr/c0_run
    --- PASS: TestSinkSafetyTable/usage-error_stderr/c1_nel
    --- PASS: TestSinkSafetyTable/usage-error_stderr/del
    --- PASS: TestSinkSafetyTable/usage-error_stderr/standalone_cr
    --- PASS: TestSinkSafetyTable/usage-error_stderr/invalid_utf-8
    --- PASS: TestSinkSafetyTable/usage-error_stderr/embedded_filename_newline
    --- PASS: TestSinkSafetyTable/CLI-help_stdout/osc_title-set
    --- PASS: TestSinkSafetyTable/CLI-help_stdout/csi_erase
    --- PASS: TestSinkSafetyTable/CLI-help_stdout/c0_run
    --- PASS: TestSinkSafetyTable/CLI-help_stdout/c1_nel
    --- PASS: TestSinkSafetyTable/CLI-help_stdout/del
    --- PASS: TestSinkSafetyTable/CLI-help_stdout/standalone_cr
    --- PASS: TestSinkSafetyTable/CLI-help_stdout/invalid_utf-8
    --- PASS: TestSinkSafetyTable/CLI-help_stdout/embedded_filename_newline
ok  	vrg/internal/app
```

## Regressions — Issue #5 browse sinks and Issue #1 CLI output

The Issue #5 browse sink-safety test (`TestHostileFixtureRawOutput`) and the Issue #1 CLI output tests (generated-help stdout, output-safety boundary, usage-diagnostic safety, help content, diagnostic specificity) all pass unchanged — the utility generalization did not alter established behavior.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestHostileFixtureRawOutput|TestLoadCompletionRendersContent' ./internal/app 2>&1 | grep -E '^(--- (PASS|FAIL)|ok|FAIL)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//' && go test -count=1 -v -run 'TestGeneratedHelpStdout|TestCLIOutputSafety|TestUsageDiagnosticSafety|TestGeneratedHelpContent|TestDiagnosticsAreSpecific' ./internal/cli ./cmd/vrg 2>&1 | grep -E '^(--- (PASS|FAIL)|ok|FAIL)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestLoadCompletionRendersContent
--- PASS: TestHostileFixtureRawOutput
ok  	vrg/internal/app
--- PASS: TestGeneratedHelpContent
--- PASS: TestUsageDiagnosticSafety
--- PASS: TestDiagnosticsAreSpecific
ok  	vrg/internal/cli
--- PASS: TestGeneratedHelpStdout
--- PASS: TestCLIOutputSafety
ok  	vrg/cmd/vrg
```

## Manual checks — real binary, real terminal bytes

Build the binary into this directory, then `pty_sinks.py` drives it: a pty run over a fixture whose filename embeds a newline plus an ESC byte and whose matched line carries a raw OSC title-set sequence (asserts on the raw byte stream a terminal would execute — no OSC or BEL survive, so the terminal title cannot change; escaped forms appear in the file list, filename rule, and panel content), a usage error with a control-byte path (escaped on stderr, exit 2), and a clean `--help` stdout.

```bash
cd /home/chris/vrg && go build -o Notes/walkthroughs/006-04/code-walkthrough/vrg ./cmd/vrg && python3 Notes/walkthroughs/006-04/code-walkthrough/pty_sinks.py
```

```output
hostile: no raw OSC/BEL/ESC in stream (terminal title cannot change), escaped name in list+rule, OSC content as ^[]0;PWNED^G, q exit=0
usage  : control-byte path escaped as bad^[dir on stderr, diagnostic + usage block, exit=2
help   : CLI-help stdout clean — one help copy, no raw control byte (tabs expanded), exit=0
OK
```

One shared utility in `internal/safepresentation` now covers every output sink — browse surfaces, usage-error stderr, and generated CLI-help stdout — with the raw path bytes untouched for filesystem identity, a single hostile fixture set driving the five-row sink table, and future sinks (Issues #9, #11, #15, #31, #34) plugging in as new rows rather than new sanitizers.
