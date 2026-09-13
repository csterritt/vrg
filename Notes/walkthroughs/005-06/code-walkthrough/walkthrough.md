# Issue #5: Browse tracer — file list, file panel with highlights, async Loading…

*2026-09-11T23:30:10Z by Showboat 0.6.1*
<!-- showboat-id: 94effe2a-ab63-4d04-9e03-54cfe657c74d -->

Walkthrough for Issue #5 (Notes/issues/005-browse-tracer-file-list-and-file-panel.md), implementing the two-pane browse tracer for the vrg terminal UI. References: Notes/PRD-vrg.md (File list and layout, Text, graphemes, and safe presentation, Module Design → FileBuffer / Viewport / Theme / App).

Contracts verified:
- After a completed search with results, the app enters a browse state with a file list on the left and a content panel on the right.
- The file list is in raw-path order; the current file is underlined.
- The content panel shows the filename in a horizontal rule, a right-justified gutter, and content rows with matches in inverse video.
- File loading is asynchronous with a visible Loading… placeholder; the UI stays responsive while loading.
- q in browse exits 0; ctrl+c exits 130 through the Issue #4 cleanup path.
- Hostile fixture bytes (OSC, CSI, C0, C1, DEL, CR, invalid UTF-8, embedded newline) produce no raw control bytes in any sink.

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
?   	vrg/internal/theme	[no test files]
?   	vrg/internal/viewport	[no test files]
```

## Safe-presentation core tests

The safe-presentation core (internal/safepresentation) escapes raw path and content bytes for safe display. Path escaping uses backslash escapes for \\n/\\r/\\t/\\\\, \\xNN for invalid UTF-8, caret notation for C0/DEL, \\u00XX for C1. Content escaping uses U+FFFD for invalid UTF-8, caret notation for C0/DEL, \\u00XX for C1, ^M for standalone CR, → for tab, and omits LF/CRLF terminators. Both expose byte→cell mappings so a match covering an ESC byte highlights both cells of ^[.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/safepresentation/ -timeout 30s
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
=== RUN   TestPathNoRawControls
--- PASS: TestPathNoRawControls (0.00s)
=== RUN   TestContentNoRawControls
--- PASS: TestContentNoRawControls (0.00s)
PASS
ok  	vrg/internal/safepresentation	0.002s
```

## FileBuffer tests

The FileBuffer (internal/filebuffer) loads, decodes, and maps a file's bytes into a display-ready buffer. Tests cover line counts, final-newline behavior, CRLF, gutter width, highlight spans from submatches, highlighting escaped ESC bytes, invalid UTF-8/controls, and nonexistent files.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/filebuffer/ -timeout 30s
```

```output
=== RUN   TestLoadLineCount
--- PASS: TestLoadLineCount (0.00s)
=== RUN   TestLoadLineCountNoFinalNewline
--- PASS: TestLoadLineCountNoFinalNewline (0.00s)
=== RUN   TestLoadLineCountEmptyFile
--- PASS: TestLoadLineCountEmptyFile (0.00s)
=== RUN   TestLoadLineCountCRLF
--- PASS: TestLoadLineCountCRLF (0.00s)
=== RUN   TestLoadLineCountTrailingNewline
--- PASS: TestLoadLineCountTrailingNewline (0.00s)
=== RUN   TestLoadGutterWidth
=== RUN   TestLoadGutterWidth/3_lines_(1_digit_+_2)
=== RUN   TestLoadGutterWidth/10_lines_(2_digits_+_2)
=== RUN   TestLoadGutterWidth/1_line_(1_digit_+_2)
=== RUN   TestLoadGutterWidth/0_lines_(min_1_digit_+_2)
--- PASS: TestLoadGutterWidth (0.00s)
    --- PASS: TestLoadGutterWidth/3_lines_(1_digit_+_2) (0.00s)
    --- PASS: TestLoadGutterWidth/10_lines_(2_digits_+_2) (0.00s)
    --- PASS: TestLoadGutterWidth/1_line_(1_digit_+_2) (0.00s)
    --- PASS: TestLoadGutterWidth/0_lines_(min_1_digit_+_2) (0.00s)
=== RUN   TestLoadHighlightSpans
--- PASS: TestLoadHighlightSpans (0.00s)
=== RUN   TestLoadHighlightSpansMultipleMatches
--- PASS: TestLoadHighlightSpansMultipleMatches (0.00s)
=== RUN   TestLoadHighlightEscapedForm
--- PASS: TestLoadHighlightEscapedForm (0.00s)
=== RUN   TestLoadNonExistentFile
--- PASS: TestLoadNonExistentFile (0.00s)
=== RUN   TestLoadDisplayText
--- PASS: TestLoadDisplayText (0.00s)
=== RUN   TestLoadDisplayTextEscapesControls
--- PASS: TestLoadDisplayTextEscapesControls (0.00s)
=== RUN   TestLoadLineNumbers
--- PASS: TestLoadLineNumbers (0.00s)
=== RUN   TestLoadNoMatches
--- PASS: TestLoadNoMatches (0.00s)
PASS
ok  	vrg/internal/filebuffer	0.003s
```

## App model browse tests

The app model (internal/app) transitions to StateBrowse after a completed search with results. Tests cover the browse view, loading placeholder, gated-load responsiveness, ctrl+c exit 130, q exit 0, file load completion, late-load rejection after cancel, file list order, current-file underline, filename rule, gutter format, no borders, inverse video, and inverse video covering escaped forms.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/app/ -run 'TestBrowse' -timeout 30s
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

## Sink-safety raw-output tests

Hostile fixtures (OSC, CSI, C0, C1, DEL, standalone CR, invalid UTF-8, embedded newline/tab, backslash) are driven through the real composition path via a no-style theme (theme.NoStyle()) that disables all ANSI sequences. The raw output is checked before any ANSI stripping: no fixture control byte may survive verbatim in the file list, filename rule, or panel content.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/app/ -run 'TestSinkSafety' -timeout 30s 2>&1 | tail -40
```

```output
=== RUN   TestSinkSafetyFilenameRule/rule_"file\nname"
=== RUN   TestSinkSafetyFilenameRule/rule_"file\tname"
=== RUN   TestSinkSafetyFilenameRule/rule_"a\\b"
--- PASS: TestSinkSafetyFilenameRule (0.00s)
    --- PASS: TestSinkSafetyFilenameRule/rule_"\x1b]0;x\a" (0.00s)
    --- PASS: TestSinkSafetyFilenameRule/rule_"\x1b[2J" (0.00s)
    --- PASS: TestSinkSafetyFilenameRule/rule_"\a\b\x1b" (0.00s)
    --- PASS: TestSinkSafetyFilenameRule/rule_"\u0085" (0.00s)
    --- PASS: TestSinkSafetyFilenameRule/rule_"\x7f" (0.00s)
    --- PASS: TestSinkSafetyFilenameRule/rule_"\r" (0.00s)
    --- PASS: TestSinkSafetyFilenameRule/rule_"foo\xff\xfebar" (0.00s)
    --- PASS: TestSinkSafetyFilenameRule/rule_"file\nname" (0.00s)
    --- PASS: TestSinkSafetyFilenameRule/rule_"file\tname" (0.00s)
    --- PASS: TestSinkSafetyFilenameRule/rule_"a\\b" (0.00s)
=== RUN   TestSinkSafetyPanelContent
=== RUN   TestSinkSafetyPanelContent/content_"\x1b]0;pwned\a"
=== RUN   TestSinkSafetyPanelContent/content_"\x1b[2J"
=== RUN   TestSinkSafetyPanelContent/content_"\a\b\x1b"
=== RUN   TestSinkSafetyPanelContent/content_"\u0085"
=== RUN   TestSinkSafetyPanelContent/content_"\x7f"
=== RUN   TestSinkSafetyPanelContent/content_"a\rb"
=== RUN   TestSinkSafetyPanelContent/content_"foo\xff\xfebar"
=== RUN   TestSinkSafetyPanelContent/content_"line\nline"
=== RUN   TestSinkSafetyPanelContent/content_"line\r\nline"
=== RUN   TestSinkSafetyPanelContent/content_"a\tb"
--- PASS: TestSinkSafetyPanelContent (0.00s)
    --- PASS: TestSinkSafetyPanelContent/content_"\x1b]0;pwned\a" (0.00s)
    --- PASS: TestSinkSafetyPanelContent/content_"\x1b[2J" (0.00s)
    --- PASS: TestSinkSafetyPanelContent/content_"\a\b\x1b" (0.00s)
    --- PASS: TestSinkSafetyPanelContent/content_"\u0085" (0.00s)
    --- PASS: TestSinkSafetyPanelContent/content_"\x7f" (0.00s)
    --- PASS: TestSinkSafetyPanelContent/content_"a\rb" (0.00s)
    --- PASS: TestSinkSafetyPanelContent/content_"foo\xff\xfebar" (0.00s)
    --- PASS: TestSinkSafetyPanelContent/content_"line\nline" (0.00s)
    --- PASS: TestSinkSafetyPanelContent/content_"line\r\nline" (0.00s)
    --- PASS: TestSinkSafetyPanelContent/content_"a\tb" (0.00s)
=== RUN   TestSinkSafetyAllSinksHostile
--- PASS: TestSinkSafetyAllSinksHostile (0.00s)
PASS
ok  	vrg/internal/app	0.026s
```

```bash
cd /home/chris/vrg && go build -o Notes/walkthroughs/005-06/code-walkthrough/vrg ./cmd/vrg && file Notes/walkthroughs/005-06/code-walkthrough/vrg | cut -d: -f2 | cut -c1-60
```

```output
 ELF 64-bit LSB executable, ARM aarch64, version 1 (SYSV), s
```

## Manual demonstration: browse view with file list, filename rule, gutter, and inverse-video matches

Using the built vrg binary with a fake rg that emits JSON results for two files. The PTY helper (runpty.py) waits for output to settle, then sends q to exit. The stripped output shows the file list on the left (alpha before zebra in raw-path order, alpha underlined) and the content panel on the right (filename rule, gutter, content with matches).

```bash
cd /home/chris/vrg/Notes/walkthroughs/005-06/code-walkthrough && PATH=$(pwd)/fakebin:/usr/bin:/bin VRG_KEY=q VRG_DELAY=1.0 timeout 10 python3 runpty.py ./vrg hello fixtures 2>&1; echo exit=$?
```

```output
fixtures/src/alpha.go     ── fixtures/src/alpha.go ──
fixtures/src/zebra.go     1  hello world2  line 23  foo bar bazexit=0
```

## Manual demonstration: raw output with ANSI sequences

The raw PTY output (before ANSI stripping) shows the underline sequence (\\\\\\\\x1b[4m) on the current file and the inverse-video sequence (\\\\\\\\x1b[7m) on matched spans. The terminal title is unchanged (no OSC sequence in the output from vrg itself).

```bash
cd /home/chris/vrg/Notes/walkthroughs/005-06/code-walkthrough && PATH=$(pwd)/fakebin:/usr/bin:/bin timeout 10 python3 -c "
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
'\x1b[>4m\x1b[?1049h\x1b[?25l\x1b[?5W\x1b[?2004h\x1b[>4;2m\x1b[>1u\x1b[?u\x1b[H\x1b[2J\x1b[4mfixtures/src/alpha.go\x1b[m     ── fixtures/src/alpha.go ──\r\nfixtures/src/zebra.go     1  \x1b[7mhello\x1b[m world\x1b[3;27H2  line 2\x1b[4;27H3  \x1b[7mfoo\x1b[m bar baz\x1b[>4m\x1b[<1u\r\x1b[30d\x1b[?1049l\x1b[?25h\x1b[?2004l'
exit=0
```

## Manual demonstration: q exits 0 from browse

The PTY helper sends q after the browse view appears. The exit code is 0 (browse quit), not 130 (cancellation).

```bash
cd /home/chris/vrg/Notes/walkthroughs/005-06/code-walkthrough && PATH=$(pwd)/fakebin:/usr/bin:/bin VRG_KEY=q VRG_DELAY=1.0 timeout 10 python3 runpty.py ./vrg hello fixtures >/dev/null 2>&1; echo exit=$?
```

```output
exit=0
```

## Manual demonstration: hostile fixture safety

A hostile filename containing a newline and an ESC byte, with a matching line containing an OSC sequence (\\\\\\\\x1b]0;pwned\\\\\\\\x07). The fake rg (fakebin_hostile/rg) emits the fixture using base64 encoding. The browse view shows escaped forms in the file list and filename rule: ESC becomes ^[ and newline becomes \\n. The terminal title is unchanged (no OSC from the fixture survives). The content panel shows Loading… because the hostile path does not exist as a real file.

```bash
cd /home/chris/vrg/Notes/walkthroughs/005-06/code-walkthrough && PATH=$(pwd)/fakebin_hostile:/usr/bin:/bin VRG_KEY=q VRG_DELAY=1.0 timeout 10 python3 runpty.py ./vrg hello fixtures 2>&1; echo exit=$?
```

```output
file^[\nname── file^[\nname ──Loading…exit=0
```

## Manual demonstration: resize handling

The PTY helper sends a resize (SIGWINCH) before the browse view appears. The browse view re-renders at the new dimensions without losing state.

```bash
cd /home/chris/vrg/Notes/walkthroughs/005-06/code-walkthrough && PATH=$(pwd)/fakebin:/usr/bin:/bin VRG_KEY=q VRG_DELAY=1.0 VRG_WIDTH=120 VRG_HEIGHT=40 timeout 10 python3 runpty.py ./vrg hello fixtures 2>&1; echo exit=$?
```

```output
fixtures/src/alpha.go          ── fixtures/src/alpha.go ──
fixtures/src/zebra.go          1  hello world2  line 23  foo bar bazexit=0
```

```bash
cd /home/chris/vrg && CGO_ENABLED=1 go test -race -count=1 ./cmd/... ./internal/... -timeout 120s | sed 's/[[:space:]][0-9.]*s$//'
```

```output
ok  	vrg/cmd/vrg
ok  	vrg/internal/app
ok  	vrg/internal/cli
ok  	vrg/internal/filebuffer
ok  	vrg/internal/safepresentation
ok  	vrg/internal/searchindex
?   	vrg/internal/theme	[no test files]
?   	vrg/internal/viewport	[no test files]
```

## Summary

Issue #5 delivers the two-pane browse tracer:

- After a completed search with results, the app enters StateBrowse with a file list on the left and a content panel on the right.
- The file list is in raw-path order; the current file is underlined.
- The content panel shows the filename in a horizontal rule, a right-justified gutter, and content rows with matches in inverse video.
- File loading is asynchronous with a visible Loading… placeholder; the UI stays responsive while loading.
- q in browse exits 0; ctrl+c exits 130 through the Issue #4 cleanup path.
- The safe-presentation core escapes all external path and content bytes before they reach View(); byte→cell mappings ensure highlights cover all cells of escaped forms.
- Hostile fixture bytes (OSC, CSI, C0, C1, DEL, CR, invalid UTF-8, embedded newline) produce no raw control bytes in any sink via the no-style composition path.
- Late file-load completions after cancellation are ignored.

References:
- Issue #5: Notes/issues/005-browse-tracer-file-list-and-file-panel.md
- Task #5: Notes/tasks/005-browse-tracer-file-list-and-file-panel.md
- PRD: Notes/PRD-vrg.md (File list and layout, Text, graphemes, and safe presentation, Module Design → FileBuffer / Viewport / Theme / App)
