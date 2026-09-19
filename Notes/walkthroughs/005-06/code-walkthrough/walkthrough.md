# Issue #5: browse tracer — file list and file panel

*2026-09-16T20:01:40Z by Showboat 0.6.1*
<!-- showboat-id: 18ca7b06-ecbc-4cb2-97f6-8c57c74ebbf1 -->

Walkthrough for Issue #5 (`Notes/issues/005-browse-tracer-file-list-and-file-panel.md`), implementing the browse-tracer requirements of `Notes/PRD-vrg.md` (*File list and layout* stories 24–26/31/34, *File loading, cache, reload, and selection consistency*, *Layout and indicators* gutter bullet, *Text, graphemes, and safe presentation*, *Module Design → FileBuffer / Viewport / Theme / App*). The interim summary is replaced by a two-pane browse view: a file list in raw-path order beside the current file's content, a filename rule above both panes, a right-justified gutter, and inverse-video matches. The current file loads asynchronously — the load command performs read + decode + byte→cell mapping and the completion carries a prepared `filebuffer.Buffer` — with "Loading…" until it arrives. Every sink routes through the new `internal/safepresentation` escaping core, so hostile path/content bytes can never become terminal instructions. All generated artifacts (the built `vrg` binary, `pty_browse.py`, the `fixture/` directory it creates) live in this directory.

```bash
cd /home/chris/vrg && go mod verify && go build ./... && go vet ./... && echo GATES-OK
```

```output
all modules verified
GATES-OK
```

```bash
cd /home/chris/vrg && go test -count=1 ./... 2>&1 | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
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

Escaping unit tests (`internal/safepresentation`): every path rule (`\\n`/`\\r`/`\\t` escapes, backslash doubling, C0/DEL caret notation, C1 `\\uXXXX`, invalid-UTF-8 `\\xNN`), the content rules (standalone CR → `^M`, provisional single-cell `→` tab placeholder, U+FFFD invalid bytes), and the byte→cell maps where a match over an ESC byte covers both cells of `^[`.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/safepresentation/ 2>&1 | grep -E '^(--- (PASS|FAIL)|ok|FAIL)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
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

FileBuffer tests (`internal/filebuffer`): `Load` reads by raw path bytes, splits LF/CRLF (terminators never displayed), keeps an unterminated final line, computes the gutter width, and converts each stop's recorded byte ranges into display-cell highlight spans — including a match covering an escaped byte covering the whole escape form.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/filebuffer/ 2>&1 | grep -E '^(--- (PASS|FAIL)|ok|FAIL)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestLoadCountsLines
--- PASS: TestLoadLineEndings
--- PASS: TestLoadEmptyFile
--- PASS: TestGutterWidth
--- PASS: TestLoadEscapedContent
--- PASS: TestHighlightSpans
--- PASS: TestHighlightCoversEscapedCells
--- PASS: TestStopBeyondFileIgnored
--- PASS: TestLoadReadFailure
ok  	vrg/internal/filebuffer
```

App model tests (`internal/app/browse_test.go`): the browse transition and `Loading…` placeholder, the gated-load seam holding a worker while keys and resizes stay responsive, the underline/gutter/rule rendering, `q` → 0 and `ctrl+c` → 130 through the Issue #4 cleanup path, late-load isolation, and `TestHostileFixtureRawOutput` — the OSC/CSI/C0/C1/DEL/standalone-CR/invalid-UTF-8/embedded-newline fixture through the real composition path on the `theme.Plain()` no-style path, asserted on raw output before any ANSI stripping.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestSearchDonePresentsBrowseWithLoading|TestLoadCompletionRendersContent|TestCurrentFileListEntryUnderlined|TestGutterRightJustified|TestGatedLoadStaysResponsive|TestCtrlCDuringHeldLoadExits130|TestQOnBrowseExitsZero|TestResizeRecomposesBrowse|TestHostileFixtureRawOutput|TestLateLoadForOtherFileIgnored' ./internal/app/ 2>&1 | grep -E '^(--- (PASS|FAIL)|ok|FAIL)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestSearchDonePresentsBrowseWithLoading
--- PASS: TestLoadCompletionRendersContent
--- PASS: TestCurrentFileListEntryUnderlined
--- PASS: TestGutterRightJustified
--- PASS: TestGatedLoadStaysResponsive
--- PASS: TestCtrlCDuringHeldLoadExits130
--- PASS: TestQOnBrowseExitsZero
--- PASS: TestResizeRecomposesBrowse
--- PASS: TestHostileFixtureRawOutput
--- PASS: TestLateLoadForOtherFileIgnored
ok  	vrg/internal/app
```

Subprocess-boundary tests (`cmd/vrg`): the ≥1 MiB dual-pipe backpressure fixture now proves record survival through the rendered content — all 18 recorded matches appear as inverse-video spans in the final frame — and the gate-held searching test shows the browse view appears only after release.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestDualPipeBackpressure|TestStderrCapturedWithoutBlocking|TestGateHeldPreparationKeepsSearching|TestOrdinaryQuitLeavesNoChild' ./cmd/vrg/ 2>&1 | grep -E '^(--- (PASS|FAIL)|ok|FAIL)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestOrdinaryQuitLeavesNoChild
--- PASS: TestDualPipeBackpressure
--- PASS: TestStderrCapturedWithoutBlocking
--- PASS: TestGateHeldPreparationKeepsSearching
ok  	vrg/cmd/vrg
```

Manual scenario — the real binary and real rg on a pty. `pty_browse.py` (in this directory) builds a `fixture/` containing three matched files, one named `evil\\n\\x1bd.txt` — a literal newline and ESC byte in the filename — whose matched line carries a raw OSC title-set sequence (`ESC ] 0 ; PWNED BEL`). It drives the binary three times: **browse** asserts the file list, the escaped name in both the list and the filename rule, the gutter, the inverse-video match, the underlined current entry, a mid-session resize recomposition, and `q` exiting 0; **hostile** asserts on the raw pty byte stream — what a terminal would execute — that no `ESC ]` or BEL survived (so no title-set sequence can run: the terminal title is unchanged), no raw ESC survives inside the filename, the escaped forms appear in all three sinks, and the alt-screen restoration sequence is emitted; **gated** holds the file load via `VRG_TEST_LOAD_GATE`, shows `Loading…`, proves a resize still recomposes while the load worker is held, then releases the gate.

```bash
cd /home/chris/vrg && go build -o Notes/walkthroughs/005-06/code-walkthrough/vrg ./cmd/vrg && cd Notes/walkthroughs/005-06/code-walkthrough && file vrg | cut -d: -f2 | cut -c1-60 && ls -A fixture | cat -v
```

```output
 ELF 64-bit LSB executable, ARM aarch64, version 1 (SYSV), s
evil
^[d.txt
normal.txt
zeta.txt
```

```bash
cd /home/chris/vrg/Notes/walkthroughs/005-06/code-walkthrough && python3 pty_browse.py
```

```output
browse : file list, filename rule, gutter, inverse match, underline, resize recompose, q exit=0
hostile: no raw OSC/BEL/ESC in stream (title cannot change), escaped name in list+rule, OSC content as ^[]0;PWNED^G, q exit=0
gated  : Loading… while held, resize responsive, release loads, q exit=0
OK
```

## Result

Every Issue #5 requirement is demonstrated. The safe-presentation suite proves every path and content escape rule plus the byte→cell maps (an ESC-byte match highlights both cells of `^[`); the FileBuffer suite proves raw-byte loading, LF/CRLF splitting, gutter width, and byte-range → display-cell highlight conversion; the app suite proves the two-pane browse transition, the prepared-buffer completion, the `Loading…` placeholder, gated-load responsiveness, `q` → 0 / `ctrl+c` → 130 through the Issue #4 cleanup path, and the hostile-fixture raw-output checks on the no-style composition path. The manual pty runs prove the real binary end to end: file list in raw-path order, escaped hostile filename (literal newline + ESC) in the list and the filename rule, right-justified gutter, inverse-video matches, mid-session resize recomposition, `q` exit 0 — and, on the raw byte stream a terminal would execute, no `ESC ]` or BEL survived the OSC-in-content attack, so no terminal title change is possible. Per `Notes/PRD-vrg.md` and `Notes/issues/005-browse-tracer-file-list-and-file-panel.md`; the real list-width formula, truncation, and reveal behavior remain Issue #24/#14's, and `c` styling is Issue #7's.
