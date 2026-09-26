# Issue #6: safe-presentation utility for every output sink

*2026-09-23T18:42:31Z by Showboat 0.6.1*
<!-- showboat-id: bf694bb0-8d45-4c76-b4e3-eb3ea24ff89d -->

Walkthrough for [Issue #6](../../../issues/006-safe-presentation-utility-for-all-sinks.md), generalizing the Issue #5 escaping core and the minimal Issue #1 `cli.Escape` escaper into the single shared utility `internal/present` per `Notes/PRD-vrg.md` (*Text, graphemes, and safe presentation*; *Module Design*). Every sink existing at this point routes through it: file-list entries, the filename rule, panel content, usage-error stderr, and the generated command-line help on stdout. All generated artifacts live in this directory: the built `vrg` binary, `demo-hostile.sh` and `demo-usage.sh`, and their `manual-hostile/` and `manual-usage/` session files. Test durations are stripped so the document verifies cleanly.

```bash
cd /home/chris/vrg && go vet ./... && go build ./... && go test -count=1 ./... | sed "s/[[:space:]][0-9.]*s$//" && echo GATES-OK
```

```output
ok  	vrg/cmd/vrg
ok  	vrg/internal/app
ok  	vrg/internal/cli
ok  	vrg/internal/filebuffer
ok  	vrg/internal/present
ok  	vrg/internal/searchindex
?   	vrg/internal/theme	[no test files]
?   	vrg/internal/viewport	[no test files]
GATES-OK
```

## Path and content rules

`internal/present` carries the Issue #5 core tests unchanged — path escaping (`Path`: newline/carriage-return/tab to literal escapes, backslash doubled, invalid UTF-8 to `\xNN`, C0/DEL caret notation, C1 to `\uXXXX`, printable Unicode preserved, never an escape byte in output) and content lines (`LineOf`: LF/CRLF structural, standalone CR `^M`, C0/DEL caret, C1 `\u`, invalid bytes U+FFFD, tab one-cell arrow, zero-width spans, raw-byte-to-cell maps where a match over the ESC byte highlights both cells of `^[`).

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestPath|TestLine' ./internal/present 2>&1 | grep -vE '^(=== RUN|    --- (PASS|FAIL))' | sed -E 's/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//'
```

```output
--- PASS: TestLineText
--- PASS: TestLineWidth
--- PASS: TestLineSpan
--- PASS: TestLineTabForm
--- PASS: TestLineRetainsRaw
--- PASS: TestPath
--- PASS: TestPathNeverEmitsControls
PASS
ok  	vrg/internal/present
```

## Diagnostic rules

New in Issue #6, `present.Diagnostic` renders multi-line error and diagnostic text: real LF boundaries preserved, CRLF treated as one boundary, tabs expanded to the next eight-column stop with columns reset per line, standalone CR escaped as `^M`, and every other control or invalid byte escaped. Filenames embedded in diagnostics are first single-lined by `Path`, so a hostile filename carrying a newline cannot inject fake diagnostic lines.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestDiagnostic' ./internal/present 2>&1 | grep -vE '^(=== RUN|    --- (PASS|FAIL))' | sed -E 's/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//'
```

```output
--- PASS: TestDiagnostic
--- PASS: TestDiagnosticNeverEmitsControls
--- PASS: TestDiagnosticEmbedsSingleLinedFilename
PASS
ok  	vrg/internal/present
```

## Shared sink-safety table

`internal/app/sinksafety_test.go` restructures the Issue #5 hostile fixture set into `TestSinkSafetyTable`: shared fixtures — OSC title-set, CSI clear-screen, BEL, backspace, standalone CR, DEL, C1, invalid UTF-8, an embedded filename newline — crossed with one row per existing sink: file-list entry, filename rule, panel content, usage-error stderr, and generated CLI-help stdout. Each row renders through `theme.Plain` (identity styles, so no ESC byte is legitimate) and asserts on raw output before any ANSI stripping; a styled pass additionally confirms no fixture payload immediately follows an unescaped ESC. Later sink-owning issues (#9, #11, #15, #31, #34) extend the table with new rows instead of duplicating fixtures. The subtests below enumerate every fixture × sink cell.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestSinkSafetyTable' ./internal/app 2>&1 | grep -E '^(\s*--- (PASS|FAIL)|FAIL|ok|PASS$)' | sed -E 's/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//'
```

```output
--- PASS: TestSinkSafetyTable
    --- PASS: TestSinkSafetyTable/file-list_entry/osc_title_set
    --- PASS: TestSinkSafetyTable/file-list_entry/csi_erase
    --- PASS: TestSinkSafetyTable/file-list_entry/c0_controls
    --- PASS: TestSinkSafetyTable/file-list_entry/c1_nel
    --- PASS: TestSinkSafetyTable/file-list_entry/delete
    --- PASS: TestSinkSafetyTable/file-list_entry/standalone_carriage_return
    --- PASS: TestSinkSafetyTable/file-list_entry/invalid_utf-8_path_bytes
    --- PASS: TestSinkSafetyTable/file-list_entry/embedded_filename_newline
    --- PASS: TestSinkSafetyTable/filename_rule/osc_title_set
    --- PASS: TestSinkSafetyTable/filename_rule/csi_erase
    --- PASS: TestSinkSafetyTable/filename_rule/c0_controls
    --- PASS: TestSinkSafetyTable/filename_rule/c1_nel
    --- PASS: TestSinkSafetyTable/filename_rule/delete
    --- PASS: TestSinkSafetyTable/filename_rule/standalone_carriage_return
    --- PASS: TestSinkSafetyTable/filename_rule/invalid_utf-8_path_bytes
    --- PASS: TestSinkSafetyTable/filename_rule/embedded_filename_newline
    --- PASS: TestSinkSafetyTable/panel_content/osc_title_set
    --- PASS: TestSinkSafetyTable/panel_content/csi_erase
    --- PASS: TestSinkSafetyTable/panel_content/c0_controls
    --- PASS: TestSinkSafetyTable/panel_content/c1_nel
    --- PASS: TestSinkSafetyTable/panel_content/delete
    --- PASS: TestSinkSafetyTable/panel_content/standalone_carriage_return
    --- PASS: TestSinkSafetyTable/panel_content/invalid_utf-8_path_bytes
    --- PASS: TestSinkSafetyTable/panel_content/embedded_filename_newline
    --- PASS: TestSinkSafetyTable/usage-error_stderr/osc_title_set
    --- PASS: TestSinkSafetyTable/usage-error_stderr/csi_erase
    --- PASS: TestSinkSafetyTable/usage-error_stderr/c0_controls
    --- PASS: TestSinkSafetyTable/usage-error_stderr/c1_nel
    --- PASS: TestSinkSafetyTable/usage-error_stderr/delete
    --- PASS: TestSinkSafetyTable/usage-error_stderr/standalone_carriage_return
    --- PASS: TestSinkSafetyTable/usage-error_stderr/invalid_utf-8_path_bytes
    --- PASS: TestSinkSafetyTable/usage-error_stderr/embedded_filename_newline
    --- PASS: TestSinkSafetyTable/cli-help_stdout/osc_title_set
    --- PASS: TestSinkSafetyTable/cli-help_stdout/csi_erase
    --- PASS: TestSinkSafetyTable/cli-help_stdout/c0_controls
    --- PASS: TestSinkSafetyTable/cli-help_stdout/c1_nel
    --- PASS: TestSinkSafetyTable/cli-help_stdout/delete
    --- PASS: TestSinkSafetyTable/cli-help_stdout/standalone_carriage_return
    --- PASS: TestSinkSafetyTable/cli-help_stdout/invalid_utf-8_path_bytes
    --- PASS: TestSinkSafetyTable/cli-help_stdout/embedded_filename_newline
PASS
ok  	vrg/internal/app
```

## Regression: Issue #1 CLI output and Issue #5 browse tests

The consolidation preserves the earlier contracts. The Issue #1 process-boundary suites — `TestGeneratedHelpStdout` and `TestCLIOutputSafety` — and the `internal/cli` table pass unchanged, and the Issue #5 browse/filebuffer tests run identically (the moved cases were mechanical renames into `internal/present`).

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestGeneratedHelpStdout|TestCLIOutputSafety' ./cmd/vrg 2>&1 | grep -vE '^(=== RUN|    --- (PASS|FAIL))' | sed -E 's/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//' && go test -count=1 ./internal/cli | sed "s/[[:space:]][0-9.]*s$//"
```

```output
--- PASS: TestGeneratedHelpStdout
--- PASS: TestCLIOutputSafety
PASS
ok  	vrg/cmd/vrg
ok  	vrg/internal/cli
```

```bash
cd /home/chris/vrg && go test -count=1 ./internal/filebuffer ./internal/app | sed "s/[[:space:]][0-9.]*s$//"
```

```output
ok  	vrg/internal/filebuffer
ok  	vrg/internal/app
```

## Manual check — hostile filename and content on a PTY

`demo-hostile.sh` (checked into this directory) drives the `vrg` binary above on a real PTY via `script(1)` against a fixture directory: a filename carrying an embedded newline and a raw ESC byte, and a matched line containing a real OSC title-set sequence. It greps the captured typescript — raw bytes, no stripping — asserting the escaped forms appear (`evil\nesc^[name.txt`, `^[]0;pwned^G`) and that no fixture byte survives verbatim. With no OSC reaching the stream the terminal title cannot be rewritten; the program exits cleanly on `q`.

```bash
cd /home/chris/vrg/Notes/walkthroughs/006-04/code-walkthrough && ./demo-hostile.sh
```

```output
exit=0
escaped-filename=present
escaped-osc=present
raw-osc-title=absent
raw-osc-bel=absent
raw-esc-filename=absent
--- escaped sinks ---
./evil\nesc^[name.txt  ── ./evil\nesc^[name.txt ────────────────────────────────
./plain.txt            1  pre ^[]0;pwned^G mid hit2  plain line
```

## Manual check — usage error with a control-byte operand

`demo-usage.sh` passes a root operand carrying a raw ESC byte. The usage diagnostic renders it escaped on stderr — `bad^[dir`, never the raw byte — through `present.Path`, writes nothing to stdout, and exits 2.

```bash
cd /home/chris/vrg/Notes/walkthroughs/006-04/code-walkthrough && ./demo-usage.sh
```

```output
exit=2
escaped-path=present
raw-esc=absent
stdout-empty=yes
--- diagnostic line ---
vrg: invalid root bad^[dir: does not exist
```

## Result

Issue #6 is verified: one shared utility, `internal/present`, owns path, content, and diagnostic presentation; every existing sink — file-list entry, filename rule, panel content, usage-error stderr, and generated CLI-help stdout — routes through it and is pinned by the fixture × sink safety table on raw output. The Issue #5 and Issue #1 test suites pass unchanged, and both PTY manual checks confirm hostile fixtures render escaped while the terminal title stays untouched. The table is the extension point: each later issue that adds an output sink (#9, #11, #15, #31, #34) appends a sink row.
