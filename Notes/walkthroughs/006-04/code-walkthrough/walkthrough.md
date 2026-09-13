# Issue #6: Safe presentation utility for all sinks

*2026-09-11T23:40:07Z by Showboat 0.6.1*
<!-- showboat-id: 7eaa49ab-792c-4337-b98b-2aadbbc15052 -->

Walkthrough for Issue #6 (Notes/tasks/006-safe-presentation-utility-for-all-sinks.md), implementing the shared safe-presentation utility that sanitizes all external data before it reaches any output sink. References: Notes/PRD-vrg.md (Text, graphemes, and safe presentation).

Contracts verified:
- One shared safe-presentation utility (safepresentation.EscapePath, EscapeContent, EscapeDiagnostic) backs every sink.
- Path escaping: backslash escapes for \n/\r/\t/\\, \xNN for invalid UTF-8, caret notation for C0/DEL, \u00XX for C1, printable Unicode preserved.
- Content escaping: U+FFFD for invalid UTF-8, caret notation for C0/DEL, \u00XX for C1, LF/CRLF as terminators, ^M for standalone CR, → for tab.
- Diagnostic escaping: LF preserved as line boundary, CRLF normalized to LF, tabs expanded to eight-column stops, other controls escaped, backslash not escaped (so embedded filenames are not double-escaped).
- Embedded filename single-line rule: filenames escaped through EscapePath before embedding in diagnostics.
- cli.Escape unified onto EscapePath (duplicated escaper removed).
- app.sanitizeDiagnostic replaced with EscapeDiagnostic delegate.
- Shared sink-safety table (internal/sinkfixtures) covers all five current sinks: file-list entry, filename rule, panel content, usage-error stderr, generated CLI-help stdout.
- Hostile fixtures (OSC, CSI, C0, C1, DEL, standalone CR, invalid UTF-8, embedded filename newline) produce no raw control bytes in any sink via the no-style composition path.
- With styles enabled, fixture payloads never appear immediately after an unescaped ESC.
- Issue #5 path/content tests unchanged. Issue #1 CLI output tests unchanged.
- Usage error with a control-byte path escaped on stderr and exit code 2.

All generated artifacts live in this directory.

```bash
cd /home/chris/vrg && go build ./... && go vet ./... && echo GATES-OK
```

```output
GATES-OK
```

```bash
cd /home/chris/vrg && go test -count=1 ./... -timeout 60s | sed 's/[[:space:]][0-9.]*s$//'
```

```output
ok  	vrg/cmd/vrg
ok  	vrg/internal/app
ok  	vrg/internal/cli
ok  	vrg/internal/filebuffer
ok  	vrg/internal/safepresentation
ok  	vrg/internal/searchindex
?   	vrg/internal/sinkfixtures	[no test files]
?   	vrg/internal/theme	[no test files]
?   	vrg/internal/viewport	[no test files]
```

## Safe-presentation path escaping tests

The safe-presentation core (internal/safepresentation) escapes raw path bytes for safe single-line display. Path escaping uses backslash escapes for \n/\r/\t/\\, \xNN for invalid UTF-8, caret notation for C0/DEL, \u00XX for C1, and preserves valid printable Unicode. These are the Issue #5 path tests, unchanged by Issue #6.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/safepresentation/ -run 'TestPath' -timeout 30s
```

```output
=== RUN   TestPathEscapeNewline
--- PASS: TestPathEscapeNewline (0.00s)
=== RUN   TestPathEscapeCarriageReturn
--- PASS: TestPathEscapeCarriageReturn (0.00s)
=== RUN   TestPathEscapeTab
--- PASS: TestPathEscapeTab (0.00s)
=== RUN   TestPathEscapeBackslash
--- PASS: TestPathEscapeBackslash (0.00s)
=== RUN   TestPathEscapeInvalidUTF8
--- PASS: TestPathEscapeInvalidUTF8 (0.00s)
=== RUN   TestPathEscapeMultipleInvalidUTF8
--- PASS: TestPathEscapeMultipleInvalidUTF8 (0.00s)
=== RUN   TestPathEscapeC0Control
--- PASS: TestPathEscapeC0Control (0.00s)
=== RUN   TestPathEscapeESC
--- PASS: TestPathEscapeESC (0.00s)
=== RUN   TestPathEscapeDEL
--- PASS: TestPathEscapeDEL (0.00s)
=== RUN   TestPathEscapeC1Control
--- PASS: TestPathEscapeC1Control (0.00s)
=== RUN   TestPathPreservePrintableUnicode
--- PASS: TestPathPreservePrintableUnicode (0.00s)
=== RUN   TestPathPreserveNonASCIIPrintable
--- PASS: TestPathPreserveNonASCIIPrintable (0.00s)
=== RUN   TestPathMixedEscapes
--- PASS: TestPathMixedEscapes (0.00s)
=== RUN   TestPathEmpty
--- PASS: TestPathEmpty (0.00s)
=== RUN   TestPathByteCellsLenMatchesInput
--- PASS: TestPathByteCellsLenMatchesInput (0.00s)
=== RUN   TestPathNoRawControls
--- PASS: TestPathNoRawControls (0.00s)
PASS
ok  	vrg/internal/safepresentation	0.002s
```

## Safe-presentation content escaping tests

Content escaping uses U+FFFD for invalid UTF-8, caret notation for C0/DEL, \u00XX for C1, LF/CRLF as terminators (never displayed), ^M for standalone CR, and → for tab. These are the Issue #5 content tests, unchanged by Issue #6.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/safepresentation/ -run 'TestContent' -timeout 30s
```

```output
=== RUN   TestContentInvalidUTF8
--- PASS: TestContentInvalidUTF8 (0.00s)
=== RUN   TestContentInvalidUTF8RetainedMapping
--- PASS: TestContentInvalidUTF8RetainedMapping (0.00s)
=== RUN   TestContentC0ControlCaretNotation
--- PASS: TestContentC0ControlCaretNotation (0.00s)
=== RUN   TestContentESCByteToCellMapping
--- PASS: TestContentESCByteToCellMapping (0.00s)
=== RUN   TestContentBELCaretNotation
--- PASS: TestContentBELCaretNotation (0.00s)
=== RUN   TestContentDEL
--- PASS: TestContentDEL (0.00s)
=== RUN   TestContentC1Control
--- PASS: TestContentC1Control (0.00s)
=== RUN   TestContentLFNeverDisplayed
--- PASS: TestContentLFNeverDisplayed (0.00s)
=== RUN   TestContentCRLFNeverDisplayed
--- PASS: TestContentCRLFNeverDisplayed (0.00s)
=== RUN   TestContentStandaloneCR
--- PASS: TestContentStandaloneCR (0.00s)
=== RUN   TestContentTabPlaceholder
--- PASS: TestContentTabPlaceholder (0.00s)
=== RUN   TestContentPreservePrintableUnicode
--- PASS: TestContentPreservePrintableUnicode (0.00s)
=== RUN   TestContentMixed
--- PASS: TestContentMixed (0.00s)
=== RUN   TestContentEmpty
--- PASS: TestContentEmpty (0.00s)
=== RUN   TestContentByteCellsLenMatchesInput
--- PASS: TestContentByteCellsLenMatchesInput (0.00s)
=== RUN   TestContentOSCSequence
--- PASS: TestContentOSCSequence (0.00s)
=== RUN   TestContentCSISequence
--- PASS: TestContentCSISequence (0.00s)
=== RUN   TestContentNoRawControls
--- PASS: TestContentNoRawControls (0.00s)
PASS
ok  	vrg/internal/safepresentation	0.002s
```

## Diagnostic escaping tests (Issue #6)

EscapeDiagnostic escapes raw diagnostic bytes for safe display while preserving real line boundaries. LF is preserved as a line boundary; CRLF is normalized to LF; tabs are expanded to eight-column stops (column resets at newline); other C0 controls and DEL use caret notation; C1 controls use \u00XX; invalid UTF-8 uses \xNN; backslash is not escaped (so embedded filenames escaped through EscapePath are not double-escaped).

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/safepresentation/ -run 'TestEscapeDiagnostic' -timeout 30s
```

```output
=== RUN   TestEscapeDiagnosticPreservesLFLineBoundaries
--- PASS: TestEscapeDiagnosticPreservesLFLineBoundaries (0.00s)
=== RUN   TestEscapeDiagnosticPreservesCRLFLineBoundaries
--- PASS: TestEscapeDiagnosticPreservesCRLFLineBoundaries (0.00s)
=== RUN   TestEscapeDiagnosticExpandsTabs
--- PASS: TestEscapeDiagnosticExpandsTabs (0.00s)
=== RUN   TestEscapeDiagnosticExpandsTabsToColumn8
--- PASS: TestEscapeDiagnosticExpandsTabsToColumn8 (0.00s)
=== RUN   TestEscapeDiagnosticTabResetsAfterNewline
--- PASS: TestEscapeDiagnosticTabResetsAfterNewline (0.00s)
=== RUN   TestEscapeDiagnosticC0Controls
--- PASS: TestEscapeDiagnosticC0Controls (0.00s)
=== RUN   TestEscapeDiagnosticDEL
--- PASS: TestEscapeDiagnosticDEL (0.00s)
=== RUN   TestEscapeDiagnosticC1Control
--- PASS: TestEscapeDiagnosticC1Control (0.00s)
=== RUN   TestEscapeDiagnosticInvalidUTF8
--- PASS: TestEscapeDiagnosticInvalidUTF8 (0.00s)
=== RUN   TestEscapeDiagnosticStandaloneCR
--- PASS: TestEscapeDiagnosticStandaloneCR (0.00s)
=== RUN   TestEscapeDiagnosticNoBackslashEscape
--- PASS: TestEscapeDiagnosticNoBackslashEscape (0.00s)
=== RUN   TestEscapeDiagnosticPreservesPrintableUnicode
--- PASS: TestEscapeDiagnosticPreservesPrintableUnicode (0.00s)
=== RUN   TestEscapeDiagnosticEmpty
--- PASS: TestEscapeDiagnosticEmpty (0.00s)
=== RUN   TestEscapeDiagnosticSingleLinedFilename
--- PASS: TestEscapeDiagnosticSingleLinedFilename (0.00s)
=== RUN   TestEscapeDiagnosticNoRawControls
--- PASS: TestEscapeDiagnosticNoRawControls (0.00s)
PASS
ok  	vrg/internal/safepresentation	0.002s
```

## Shared sink-safety table: escaper level

The shared internal/sinkfixtures package holds the hostile-fixture set (OSC, CSI, C0, C1, DEL, standalone CR, invalid UTF-8, embedded filename newline). Each escaper is a row: EscapePath, EscapeContent, EscapeDiagnostic. The no-style composition path asserts no raw control bytes survive (except preserved LF in diagnostics).

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/safepresentation/ -run 'TestSinkSafetyTable' -timeout 30s
```

```output
=== RUN   TestSinkSafetyTableEscapePath
=== RUN   TestSinkSafetyTableEscapePath/OSC
=== RUN   TestSinkSafetyTableEscapePath/CSI
=== RUN   TestSinkSafetyTableEscapePath/C0
=== RUN   TestSinkSafetyTableEscapePath/C1
=== RUN   TestSinkSafetyTableEscapePath/DEL
=== RUN   TestSinkSafetyTableEscapePath/StandaloneCR
=== RUN   TestSinkSafetyTableEscapePath/InvalidUTF8
=== RUN   TestSinkSafetyTableEscapePath/EmbeddedNewline
--- PASS: TestSinkSafetyTableEscapePath (0.00s)
    --- PASS: TestSinkSafetyTableEscapePath/OSC (0.00s)
    --- PASS: TestSinkSafetyTableEscapePath/CSI (0.00s)
    --- PASS: TestSinkSafetyTableEscapePath/C0 (0.00s)
    --- PASS: TestSinkSafetyTableEscapePath/C1 (0.00s)
    --- PASS: TestSinkSafetyTableEscapePath/DEL (0.00s)
    --- PASS: TestSinkSafetyTableEscapePath/StandaloneCR (0.00s)
    --- PASS: TestSinkSafetyTableEscapePath/InvalidUTF8 (0.00s)
    --- PASS: TestSinkSafetyTableEscapePath/EmbeddedNewline (0.00s)
=== RUN   TestSinkSafetyTableEscapeContent
=== RUN   TestSinkSafetyTableEscapeContent/OSC
=== RUN   TestSinkSafetyTableEscapeContent/CSI
=== RUN   TestSinkSafetyTableEscapeContent/C0
=== RUN   TestSinkSafetyTableEscapeContent/C1
=== RUN   TestSinkSafetyTableEscapeContent/DEL
=== RUN   TestSinkSafetyTableEscapeContent/StandaloneCR
=== RUN   TestSinkSafetyTableEscapeContent/InvalidUTF8
=== RUN   TestSinkSafetyTableEscapeContent/EmbeddedNewline
--- PASS: TestSinkSafetyTableEscapeContent (0.00s)
    --- PASS: TestSinkSafetyTableEscapeContent/OSC (0.00s)
    --- PASS: TestSinkSafetyTableEscapeContent/CSI (0.00s)
    --- PASS: TestSinkSafetyTableEscapeContent/C0 (0.00s)
    --- PASS: TestSinkSafetyTableEscapeContent/C1 (0.00s)
    --- PASS: TestSinkSafetyTableEscapeContent/DEL (0.00s)
    --- PASS: TestSinkSafetyTableEscapeContent/StandaloneCR (0.00s)
    --- PASS: TestSinkSafetyTableEscapeContent/InvalidUTF8 (0.00s)
    --- PASS: TestSinkSafetyTableEscapeContent/EmbeddedNewline (0.00s)
=== RUN   TestSinkSafetyTableEscapeDiagnostic
=== RUN   TestSinkSafetyTableEscapeDiagnostic/OSC
=== RUN   TestSinkSafetyTableEscapeDiagnostic/CSI
=== RUN   TestSinkSafetyTableEscapeDiagnostic/C0
=== RUN   TestSinkSafetyTableEscapeDiagnostic/C1
=== RUN   TestSinkSafetyTableEscapeDiagnostic/DEL
=== RUN   TestSinkSafetyTableEscapeDiagnostic/StandaloneCR
=== RUN   TestSinkSafetyTableEscapeDiagnostic/InvalidUTF8
=== RUN   TestSinkSafetyTableEscapeDiagnostic/EmbeddedNewline
--- PASS: TestSinkSafetyTableEscapeDiagnostic (0.00s)
    --- PASS: TestSinkSafetyTableEscapeDiagnostic/OSC (0.00s)
    --- PASS: TestSinkSafetyTableEscapeDiagnostic/CSI (0.00s)
    --- PASS: TestSinkSafetyTableEscapeDiagnostic/C0 (0.00s)
    --- PASS: TestSinkSafetyTableEscapeDiagnostic/C1 (0.00s)
    --- PASS: TestSinkSafetyTableEscapeDiagnostic/DEL (0.00s)
    --- PASS: TestSinkSafetyTableEscapeDiagnostic/StandaloneCR (0.00s)
    --- PASS: TestSinkSafetyTableEscapeDiagnostic/InvalidUTF8 (0.00s)
    --- PASS: TestSinkSafetyTableEscapeDiagnostic/EmbeddedNewline (0.00s)
PASS
ok  	vrg/internal/safepresentation	0.002s
```

## Shared sink-safety table: browse sinks (file-list, filename-rule, panel-content)

The browse sinks (file-list entry, filename rule, panel content) are driven through the real composition path with the shared fixtures. The no-style path (theme.NoStyle()) asserts no raw control bytes survive; the styled path (theme.New()) asserts the fixture payload never appears immediately after an unescaped ESC.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/app/ -run 'TestSinkSafetyTable' -timeout 30s 2>&1 | tail -60
```

```output
    --- PASS: TestSinkSafetyTablePanelContentNoStyle/DEL (0.00s)
    --- PASS: TestSinkSafetyTablePanelContentNoStyle/StandaloneCR (0.00s)
    --- PASS: TestSinkSafetyTablePanelContentNoStyle/InvalidUTF8 (0.00s)
    --- PASS: TestSinkSafetyTablePanelContentNoStyle/EmbeddedNewline (0.00s)
=== RUN   TestSinkSafetyTableFileListStyled
=== RUN   TestSinkSafetyTableFileListStyled/OSC
=== RUN   TestSinkSafetyTableFileListStyled/CSI
=== RUN   TestSinkSafetyTableFileListStyled/C0
=== RUN   TestSinkSafetyTableFileListStyled/C1
=== RUN   TestSinkSafetyTableFileListStyled/DEL
=== RUN   TestSinkSafetyTableFileListStyled/StandaloneCR
=== RUN   TestSinkSafetyTableFileListStyled/InvalidUTF8
=== RUN   TestSinkSafetyTableFileListStyled/EmbeddedNewline
--- PASS: TestSinkSafetyTableFileListStyled (0.00s)
    --- PASS: TestSinkSafetyTableFileListStyled/OSC (0.00s)
    --- PASS: TestSinkSafetyTableFileListStyled/CSI (0.00s)
    --- PASS: TestSinkSafetyTableFileListStyled/C0 (0.00s)
    --- PASS: TestSinkSafetyTableFileListStyled/C1 (0.00s)
    --- PASS: TestSinkSafetyTableFileListStyled/DEL (0.00s)
    --- PASS: TestSinkSafetyTableFileListStyled/StandaloneCR (0.00s)
    --- PASS: TestSinkSafetyTableFileListStyled/InvalidUTF8 (0.00s)
    --- PASS: TestSinkSafetyTableFileListStyled/EmbeddedNewline (0.00s)
=== RUN   TestSinkSafetyTableFilenameRuleStyled
=== RUN   TestSinkSafetyTableFilenameRuleStyled/OSC
=== RUN   TestSinkSafetyTableFilenameRuleStyled/CSI
=== RUN   TestSinkSafetyTableFilenameRuleStyled/C0
=== RUN   TestSinkSafetyTableFilenameRuleStyled/C1
=== RUN   TestSinkSafetyTableFilenameRuleStyled/DEL
=== RUN   TestSinkSafetyTableFilenameRuleStyled/StandaloneCR
=== RUN   TestSinkSafetyTableFilenameRuleStyled/InvalidUTF8
=== RUN   TestSinkSafetyTableFilenameRuleStyled/EmbeddedNewline
--- PASS: TestSinkSafetyTableFilenameRuleStyled (0.00s)
    --- PASS: TestSinkSafetyTableFilenameRuleStyled/OSC (0.00s)
    --- PASS: TestSinkSafetyTableFilenameRuleStyled/CSI (0.00s)
    --- PASS: TestSinkSafetyTableFilenameRuleStyled/C0 (0.00s)
    --- PASS: TestSinkSafetyTableFilenameRuleStyled/C1 (0.00s)
    --- PASS: TestSinkSafetyTableFilenameRuleStyled/DEL (0.00s)
    --- PASS: TestSinkSafetyTableFilenameRuleStyled/StandaloneCR (0.00s)
    --- PASS: TestSinkSafetyTableFilenameRuleStyled/InvalidUTF8 (0.00s)
    --- PASS: TestSinkSafetyTableFilenameRuleStyled/EmbeddedNewline (0.00s)
=== RUN   TestSinkSafetyTablePanelContentStyled
=== RUN   TestSinkSafetyTablePanelContentStyled/OSC
=== RUN   TestSinkSafetyTablePanelContentStyled/CSI
=== RUN   TestSinkSafetyTablePanelContentStyled/C0
=== RUN   TestSinkSafetyTablePanelContentStyled/C1
=== RUN   TestSinkSafetyTablePanelContentStyled/DEL
=== RUN   TestSinkSafetyTablePanelContentStyled/StandaloneCR
=== RUN   TestSinkSafetyTablePanelContentStyled/InvalidUTF8
=== RUN   TestSinkSafetyTablePanelContentStyled/EmbeddedNewline
--- PASS: TestSinkSafetyTablePanelContentStyled (0.00s)
    --- PASS: TestSinkSafetyTablePanelContentStyled/OSC (0.00s)
    --- PASS: TestSinkSafetyTablePanelContentStyled/CSI (0.00s)
    --- PASS: TestSinkSafetyTablePanelContentStyled/C0 (0.00s)
    --- PASS: TestSinkSafetyTablePanelContentStyled/C1 (0.00s)
    --- PASS: TestSinkSafetyTablePanelContentStyled/DEL (0.00s)
    --- PASS: TestSinkSafetyTablePanelContentStyled/StandaloneCR (0.00s)
    --- PASS: TestSinkSafetyTablePanelContentStyled/InvalidUTF8 (0.00s)
    --- PASS: TestSinkSafetyTablePanelContentStyled/EmbeddedNewline (0.00s)
PASS
ok  	vrg/internal/app	0.025s
```

## Shared sink-safety table: CLI sinks (usage-error stderr, CLI-help stdout)

The CLI sinks are driven through the shared fixtures. The usage-error stderr path asserts no raw control bytes survive in the diagnostic. The CLI-help stdout path asserts no dangerous control bytes (tabs and newlines allowed as formatting). The styled path asserts the fixture payload never appears immediately after an unescaped ESC.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/cli/ -run 'TestSinkSafetyTable' -timeout 30s
```

```output
=== RUN   TestSinkSafetyTableUsageErrorStderr
=== RUN   TestSinkSafetyTableUsageErrorStderr/OSC
=== RUN   TestSinkSafetyTableUsageErrorStderr/CSI
=== RUN   TestSinkSafetyTableUsageErrorStderr/C0
=== RUN   TestSinkSafetyTableUsageErrorStderr/C1
=== RUN   TestSinkSafetyTableUsageErrorStderr/DEL
=== RUN   TestSinkSafetyTableUsageErrorStderr/StandaloneCR
=== RUN   TestSinkSafetyTableUsageErrorStderr/InvalidUTF8
=== RUN   TestSinkSafetyTableUsageErrorStderr/EmbeddedNewline
--- PASS: TestSinkSafetyTableUsageErrorStderr (0.00s)
    --- PASS: TestSinkSafetyTableUsageErrorStderr/OSC (0.00s)
    --- PASS: TestSinkSafetyTableUsageErrorStderr/CSI (0.00s)
    --- PASS: TestSinkSafetyTableUsageErrorStderr/C0 (0.00s)
    --- PASS: TestSinkSafetyTableUsageErrorStderr/C1 (0.00s)
    --- PASS: TestSinkSafetyTableUsageErrorStderr/DEL (0.00s)
    --- PASS: TestSinkSafetyTableUsageErrorStderr/StandaloneCR (0.00s)
    --- PASS: TestSinkSafetyTableUsageErrorStderr/InvalidUTF8 (0.00s)
    --- PASS: TestSinkSafetyTableUsageErrorStderr/EmbeddedNewline (0.00s)
=== RUN   TestSinkSafetyTableUsageErrorStderrStyled
=== RUN   TestSinkSafetyTableUsageErrorStderrStyled/OSC
=== RUN   TestSinkSafetyTableUsageErrorStderrStyled/CSI
=== RUN   TestSinkSafetyTableUsageErrorStderrStyled/C0
=== RUN   TestSinkSafetyTableUsageErrorStderrStyled/C1
=== RUN   TestSinkSafetyTableUsageErrorStderrStyled/DEL
=== RUN   TestSinkSafetyTableUsageErrorStderrStyled/StandaloneCR
=== RUN   TestSinkSafetyTableUsageErrorStderrStyled/InvalidUTF8
=== RUN   TestSinkSafetyTableUsageErrorStderrStyled/EmbeddedNewline
--- PASS: TestSinkSafetyTableUsageErrorStderrStyled (0.00s)
    --- PASS: TestSinkSafetyTableUsageErrorStderrStyled/OSC (0.00s)
    --- PASS: TestSinkSafetyTableUsageErrorStderrStyled/CSI (0.00s)
    --- PASS: TestSinkSafetyTableUsageErrorStderrStyled/C0 (0.00s)
    --- PASS: TestSinkSafetyTableUsageErrorStderrStyled/C1 (0.00s)
    --- PASS: TestSinkSafetyTableUsageErrorStderrStyled/DEL (0.00s)
    --- PASS: TestSinkSafetyTableUsageErrorStderrStyled/StandaloneCR (0.00s)
    --- PASS: TestSinkSafetyTableUsageErrorStderrStyled/InvalidUTF8 (0.00s)
    --- PASS: TestSinkSafetyTableUsageErrorStderrStyled/EmbeddedNewline (0.00s)
=== RUN   TestSinkSafetyTableHelpStdout
--- PASS: TestSinkSafetyTableHelpStdout (0.00s)
=== RUN   TestSinkSafetyTableHelpStdoutStyled
--- PASS: TestSinkSafetyTableHelpStdoutStyled (0.00s)
PASS
ok  	vrg/internal/cli	0.002s
```

## Shared sink-safety table: process boundary (usage-error stderr, CLI-help stdout)

The process-boundary sinks are driven through the real binary. The usage-error path asserts exit 2 and no dangerous control bytes on stderr. The help path asserts no dangerous control bytes on stdout.

```bash
cd /home/chris/vrg && go test -count=1 -v ./cmd/vrg/ -run 'TestSinkSafetyTable' -timeout 60s
```

```output
=== RUN   TestSinkSafetyTableUsageErrorProcessBoundary
=== RUN   TestSinkSafetyTableUsageErrorProcessBoundary/OSC
=== RUN   TestSinkSafetyTableUsageErrorProcessBoundary/CSI
=== RUN   TestSinkSafetyTableUsageErrorProcessBoundary/C0
=== RUN   TestSinkSafetyTableUsageErrorProcessBoundary/C1
=== RUN   TestSinkSafetyTableUsageErrorProcessBoundary/DEL
=== RUN   TestSinkSafetyTableUsageErrorProcessBoundary/StandaloneCR
=== RUN   TestSinkSafetyTableUsageErrorProcessBoundary/InvalidUTF8
=== RUN   TestSinkSafetyTableUsageErrorProcessBoundary/EmbeddedNewline
--- PASS: TestSinkSafetyTableUsageErrorProcessBoundary (0.19s)
    --- PASS: TestSinkSafetyTableUsageErrorProcessBoundary/OSC (0.02s)
    --- PASS: TestSinkSafetyTableUsageErrorProcessBoundary/CSI (0.02s)
    --- PASS: TestSinkSafetyTableUsageErrorProcessBoundary/C0 (0.02s)
    --- PASS: TestSinkSafetyTableUsageErrorProcessBoundary/C1 (0.02s)
    --- PASS: TestSinkSafetyTableUsageErrorProcessBoundary/DEL (0.02s)
    --- PASS: TestSinkSafetyTableUsageErrorProcessBoundary/StandaloneCR (0.02s)
    --- PASS: TestSinkSafetyTableUsageErrorProcessBoundary/InvalidUTF8 (0.02s)
    --- PASS: TestSinkSafetyTableUsageErrorProcessBoundary/EmbeddedNewline (0.02s)
=== RUN   TestSinkSafetyTableHelpStdoutProcessBoundary
--- PASS: TestSinkSafetyTableHelpStdoutProcessBoundary (0.02s)
PASS
ok  	vrg/cmd/vrg	0.412s
```

## Issue #5 browse tests (unchanged regression)

The Issue #5 browse tests are unchanged by Issue #6. They cover the browse view, loading placeholder, gated-load responsiveness, ctrl+c exit 130, q exit 0, file load completion, late-load rejection, file list order, current-file underline, filename rule, gutter format, no borders, inverse video, and inverse video covering escaped forms.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/app/ -run 'TestBrowse' -timeout 30s 2>&1 | tail -40
```

```output
=== RUN   TestBrowseViewAfterCompletion
--- PASS: TestBrowseViewAfterCompletion (0.00s)
=== RUN   TestBrowseLoadingPlaceholder
--- PASS: TestBrowseLoadingPlaceholder (0.00s)
=== RUN   TestBrowseKeyHandledWhileLoading
--- PASS: TestBrowseKeyHandledWhileLoading (0.00s)
=== RUN   TestBrowseResizeHandledWhileLoading
--- PASS: TestBrowseResizeHandledWhileLoading (0.00s)
=== RUN   TestBrowseCtrlCExits130
--- PASS: TestBrowseCtrlCExits130 (0.00s)
=== RUN   TestBrowseQExitsZero
--- PASS: TestBrowseQExitsZero (0.00s)
=== RUN   TestBrowseFileLoadComplete
--- PASS: TestBrowseFileLoadComplete (0.00s)
=== RUN   TestBrowseLateLoadIgnoredAfterCancel
--- PASS: TestBrowseLateLoadIgnoredAfterCancel (0.00s)
=== RUN   TestBrowseFileListOrder
--- PASS: TestBrowseFileListOrder (0.00s)
=== RUN   TestBrowseCurrentFileUnderlined
--- PASS: TestBrowseCurrentFileUnderlined (0.00s)
=== RUN   TestBrowseFilenameRule
--- PASS: TestBrowseFilenameRule (0.00s)
=== RUN   TestBrowseGutterFormat
--- PASS: TestBrowseGutterFormat (0.00s)
=== RUN   TestBrowseGutterRightJustified
--- PASS: TestBrowseGutterRightJustified (0.00s)
=== RUN   TestBrowseNoBorders
--- PASS: TestBrowseNoBorders (0.00s)
=== RUN   TestBrowseInverseVideo
--- PASS: TestBrowseInverseVideo (0.00s)
=== RUN   TestBrowseInverseVideoCoversEscapedForm
--- PASS: TestBrowseInverseVideoCoversEscapedForm (0.00s)
PASS
ok  	vrg/internal/app	0.024s
```

## Issue #1 CLI output tests (unchanged regression)

The Issue #1 CLI tests are unchanged by Issue #6. The named groups TestGeneratedHelpStdout and TestCLIOutputSafety verify help output and hostile-operand escaping at the process boundary. The cli package tests verify help content, flag forwarding, and diagnostic safety.

```bash
cd /home/chris/vrg && go test -count=1 -v ./cmd/vrg/ -run 'TestGeneratedHelpStdout|TestCLIOutputSafety' -timeout 60s 2>&1 | tail -30
```

```output
=== RUN   TestGeneratedHelpStdout/foo_bar_baz_--help
=== RUN   TestGeneratedHelpStdout/-i_--help
=== RUN   TestGeneratedHelpStdout/-ih
=== RUN   TestGeneratedHelpStdout/-ih_foo
=== RUN   TestGeneratedHelpStdout/--ignore-case_--help
=== RUN   TestGeneratedHelpStdout/foo_src_--help
=== RUN   TestGeneratedHelpStdout/-uuu_--help
=== RUN   TestGeneratedHelpStdout/--ignore-case=false_--help
=== RUN   TestGeneratedHelpStdout/--unsupported_--help
=== RUN   TestGeneratedHelpStdout/foo_/nonexistent_--help
--- PASS: TestGeneratedHelpStdout (0.34s)
    --- PASS: TestGeneratedHelpStdout/#00 (0.02s)
    --- PASS: TestGeneratedHelpStdout/-h (0.02s)
    --- PASS: TestGeneratedHelpStdout/--help (0.02s)
    --- PASS: TestGeneratedHelpStdout/foo_--help_/nonexistent (0.02s)
    --- PASS: TestGeneratedHelpStdout/-h_foo_bar_baz (0.02s)
    --- PASS: TestGeneratedHelpStdout/foo_bar_baz_--help (0.02s)
    --- PASS: TestGeneratedHelpStdout/-i_--help (0.02s)
    --- PASS: TestGeneratedHelpStdout/-ih (0.02s)
    --- PASS: TestGeneratedHelpStdout/-ih_foo (0.02s)
    --- PASS: TestGeneratedHelpStdout/--ignore-case_--help (0.02s)
    --- PASS: TestGeneratedHelpStdout/foo_src_--help (0.02s)
    --- PASS: TestGeneratedHelpStdout/-uuu_--help (0.02s)
    --- PASS: TestGeneratedHelpStdout/--ignore-case=false_--help (0.02s)
    --- PASS: TestGeneratedHelpStdout/--unsupported_--help (0.02s)
    --- PASS: TestGeneratedHelpStdout/foo_/nonexistent_--help (0.02s)
=== RUN   TestCLIOutputSafety
--- PASS: TestCLIOutputSafety (0.07s)
PASS
ok  	vrg/cmd/vrg	0.608s
```

```bash
cd /home/chris/vrg && go build -o Notes/walkthroughs/006-04/code-walkthrough/vrg ./cmd/vrg && file Notes/walkthroughs/006-04/code-walkthrough/vrg | cut -d: -f2 | cut -c1-60
```

```output
 ELF 64-bit LSB executable, ARM aarch64, version 1 (SYSV), s
```

## Manual demonstration: hostile filename/content fixture

A hostile filename containing a newline and an ESC byte, with a matching line containing an OSC sequence (\x1b]0;pwned\x07). The fake rg (fakebin_hostile/rg) emits the fixture using base64 encoding. The browse view shows escaped forms in the file list and filename rule: ESC becomes ^[ and newline becomes \n. The terminal title is unchanged (no OSC from the fixture survives). The content panel shows Loading… because the hostile path does not exist as a real file.

```bash
cd /home/chris/vrg/Notes/walkthroughs/006-04/code-walkthrough && PATH=$(pwd)/fakebin_hostile:/usr/bin:/bin VRG_KEY=q VRG_DELAY=1.0 timeout 10 python3 runpty.py ./vrg hello fixtures 2>&1; echo exit=$?
```

```output
file^[\nname── file^[\nname ──Loading…exit=0
```

## Manual demonstration: terminal title unchanged

The raw PTY output (before ANSI stripping) shows the underline sequence (\x1b[4m) on the current file and the inverse-video sequence (\x1b[7m) on matched spans. The terminal title is unchanged: no OSC sequence from the fixture survives in the output from vrg itself. The only ESC bytes are from Bubble Tea's alt-screen and cursor-control sequences and from the theme's underline/inverse-video styling.

```bash
cd /home/chris/vrg/Notes/walkthroughs/006-04/code-walkthrough && PATH=$(pwd)/fakebin_hostile:/usr/bin:/bin timeout 10 python3 -c "
import os, pty, sys, time, select, struct, fcntl, termios
pid, fd = pty.fork()
if pid == 0:
    os.execvp('./vrg', ['./vrg', 'hello', 'fixtures'])
else:
    winsize = struct.pack('HHHH', 30, 100, 0, 0)
    fcntl.ioctl(fd, termios.TIOCSWINSZ, winsize)
    output = b''
    while True:
        r, _, _ = select.select([fd], [], [], 1.0)
        if r:
            data = os.read(fd, 4096)
            if not data: break
            output += data
        else: break
    time.sleep(1.0)
    os.write(fd, b'q')
    while True:
        try:
            r, _, _ = select.select([fd], [], [], 1.0)
            if r:
                data = os.read(fd, 4096)
                if not data: break
                output += data
            else: break
        except OSError: break
    _, status = os.waitpid(pid, 0)
    text = output.decode('utf-8', errors='replace')
    # Show only the browse-view portion of the raw output
    idx = text.find('x1b[4m')
    if idx > 0:
        text = text[idx-1:]
    end = text.find('x1b[?1049l')
    if end > 0:
        text = text[:end+10]
    print(repr(text))
    sys.exit(os.waitstatus_to_exitcode(status))
" 2>&1; echo exit=$?
```

```output
'\x1b[>4m\x1b[?1049h\x1b[?25l\x1b[?5W\x1b[?2004h\x1b[>4;2m\x1b[>1u\x1b[?u\x1b[H\x1b[2J\x1b[4mfile^[\\nname\x1b[m\x1b[14X\x1b[1;27H── file^[\\nname ──\x1b[2;27HLoading…\x1b[>4m\x1b[<1u\r\x1b[30d\x1b[?1049l\x1b[?25h\x1b[?2004l'
exit=0
```

## Manual demonstration: usage error with control-byte path on stderr, exit 2

A hostile root operand containing an ESC byte and a CSI sequence (\x1b[2J). The usage error escapes the hostile path on stderr (ESC becomes ^[, CSI becomes ^[[2J) and exits with code 2. The terminal title is unchanged (no OSC or CSI from the fixture survives).

```bash
cd /home/chris/vrg/Notes/walkthroughs/006-04/code-walkthrough && printf 'foo\x1b[2J' | xargs -0 ./vrg 2>&1; echo exit=$?
```

```output
vrg: bubbletea: error opening TTY: bubbletea: could not open TTY: open /dev/tty: no such device or address
exit=123
```

```bash
cd /home/chris/vrg/Notes/walkthroughs/006-04/code-walkthrough && ./vrg foo "$(printf 'root\x1b[2J')" 2>&1; echo exit=$?
```

```output
vrg: invalid root root^[[2J: does not exist

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
exit=2
```

```bash
cd /home/chris/vrg && CGO_ENABLED=1 go test -race -count=1 ./... -timeout 120s | sed 's/[[:space:]][0-9.]*s$//'
```

```output
ok  	vrg/cmd/vrg
ok  	vrg/internal/app
ok  	vrg/internal/cli
ok  	vrg/internal/filebuffer
ok  	vrg/internal/safepresentation
ok  	vrg/internal/searchindex
?   	vrg/internal/sinkfixtures	[no test files]
?   	vrg/internal/theme	[no test files]
?   	vrg/internal/viewport	[no test files]
```

```bash
cd /home/chris/vrg/Notes/walkthroughs/006-04/code-walkthrough && PATH=$(pwd)/fakebin_hostile:/usr/bin:/bin VRG_KEY=q VRG_DELAY=1.0 timeout 10 python3 runpty.py ./vrg hello fixtures 2>&1; echo exit=$?
```

```output
file^[\nname── file^[\nname ──Loading…exit=0
```

## Summary

Issue #6 delivers the shared safe-presentation utility for every output sink:

- One shared utility (internal/safepresentation) backs every sink: EscapePath for single-line paths, EscapeContent for multi-line content, EscapeDiagnostic for multi-line diagnostics.
- Path escaping: backslash escapes for \n/\r/\t/\\, \xNN for invalid UTF-8, caret notation for C0/DEL, \u00XX for C1, printable Unicode preserved.
- Content escaping: U+FFFD for invalid UTF-8, caret notation for C0/DEL, \u00XX for C1, LF/CRLF as terminators, ^M for standalone CR, → for tab.
- Diagnostic escaping: LF preserved as line boundary, CRLF normalized to LF, tabs expanded to eight-column stops, other controls escaped, backslash not escaped (so embedded filenames are not double-escaped).
- Embedded filename single-line rule: filenames escaped through EscapePath before embedding in diagnostics, so filename newlines cannot become diagnostic paragraph breaks.
- cli.Escape unified onto EscapePath (duplicated escaper removed).
- app.sanitizeDiagnostic replaced with EscapeDiagnostic delegate (full diagnostic contract: line preservation, tab expansion, complete control escaping).
- Shared sink-safety table (internal/sinkfixtures) covers all five current sinks: file-list entry, filename rule, panel content, usage-error stderr, generated CLI-help stdout.
- Hostile fixtures (OSC, CSI, C0, C1, DEL, standalone CR, invalid UTF-8, embedded filename newline) produce no raw control bytes in any sink via the no-style composition path.
- With styles enabled, fixture payloads never appear immediately after an unescaped ESC.
- Issue #5 path/content tests unchanged. Issue #1 CLI output tests unchanged.
- Usage error with a control-byte path escaped on stderr and exit code 2.

References:
- Issue #6: Notes/issues/006-safe-presentation-utility-for-all-sinks.md
- Task #6: Notes/tasks/006-safe-presentation-utility-for-all-sinks.md
- PRD: Notes/PRD-vrg.md (Text, graphemes, and safe presentation)
