# Issue #5: browse tracer — file list, file panel, async loading

*2026-09-23T18:16:40Z by Showboat 0.6.1*
<!-- showboat-id: 2c9df97e-68f3-4924-99a2-9acdc62c7b34 -->

Walkthrough for [Issue #5](../../../issues/005-browse-tracer-file-list-and-file-panel.md), implementing the two-pane browse tracer per `Notes/PRD-vrg.md` (File list and layout; Text, graphemes, and safe presentation; Module Design). The interim search summary is replaced by a raw-path-ordered file list beside the current file's content: filename rule, right-justified gutter, inverse-video matches, asynchronous prepared-buffer loads behind a `Loading…` placeholder, and a safe-presentation core keeping hostile bytes out of every sink. All generated artifacts live in this directory: the built `vrg` binary, `demo-browse.sh` and `demo-hostile.sh`, and their `manual-run/` and `manual-hostile/` session files. Test durations are stripped so the document verifies cleanly.

```bash
cd /home/chris/vrg && go vet ./... && go build ./... && go test -count=1 ./... | sed "s/[[:space:]][0-9.]*s$//" && echo GATES-OK
```

```output
ok  	vrg/cmd/vrg
ok  	vrg/internal/app
ok  	vrg/internal/cli
ok  	vrg/internal/filebuffer
ok  	vrg/internal/searchindex
?   	vrg/internal/theme	[no test files]
?   	vrg/internal/viewport	[no test files]
GATES-OK
```

The safe-presentation core (`internal/filebuffer/present.go`): `EscapePath` covers the path rules — `\\n`/`\\r`/`\\t` forms, `\\\\` doubling, `\\xNN` for invalid UTF-8, caret notation for C0 and `^?` for DEL, `\\uXXXX` for C1, printable Unicode preserved — and `presentLine` covers content — U+FFFD for invalid UTF-8 with retained raw-byte mappings, caret notation, invisible LF/CRLF, standalone CR as `^M`, the provisional single-cell `→` tab pending Issue #16 — plus the byte→cell span mapping that lets a match on an ESC byte highlight both `^[` cells.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestEscapePath|TestPresentLine' ./internal/filebuffer 2>&1 | grep -vE '^(=== RUN|    --- (PASS|FAIL))' | sed -E 's/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//'
```

```output
--- PASS: TestEscapePath
--- PASS: TestEscapePathNeverEmitsControls
--- PASS: TestPresentLineText
--- PASS: TestPresentLineWidth
--- PASS: TestPresentLineSpan
--- PASS: TestPresentLineTabForm
--- PASS: TestPresentLineRetainsRaw
PASS
ok  	vrg/internal/filebuffer
```

The FileBuffer first path (`internal/filebuffer/filebuffer.go`): `Load` reads, splits, escapes, and maps a whole file inside the worker command so the completion message carries a prepared buffer and `Update` does no full-file work; gutter width is the largest line number's digit width plus two spaces (minimum one slot); each stop's submatches are validated against the line's raw bytes — out-of-bounds or byte-mismatched ranges are dropped.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestLoadLineCount|TestLoadFailure|TestGutterWidth|TestLinePresentation|TestHighlightSpans|TestSubmatchValidation' ./internal/filebuffer 2>&1 | grep -vE '^(=== RUN|    --- (PASS|FAIL))' | sed -E 's/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//'
```

```output
--- PASS: TestLoadLineCount
--- PASS: TestGutterWidth
--- PASS: TestLinePresentation
--- PASS: TestHighlightSpans
--- PASS: TestSubmatchValidation
--- PASS: TestLoadFailure
PASS
ok  	vrg/internal/filebuffer
```

App model transitions (`internal/app`): a completed search enters the browse phase, shows `Loading…` until the worker delivers the prepared buffer, then renders guttered content; `loadGate` holds the worker's read and decode/map phases while keys and resizes are still processed; `ctrl+c` mid-load follows the Issue #4 path to 130, `q` exits 0, and a failed load renders `(unreadable)`.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestCompletionTransitionsToBrowse|TestLoadCompletionRendersContent|TestMatchRendersInverse|TestGatedLoadKeepsResponsive|TestCtrlCWhileLoadGateHeld|TestQOnBrowse|TestEscOnBrowseIsNoOp|TestLoadFailureShowsUnreadable|TestGateHoldsSearchingAfterRgExit' ./internal/app 2>&1 | grep -vE '^(=== RUN|    --- (PASS|FAIL))' | sed -E 's/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//'
```

```output
--- PASS: TestCompletionTransitionsToBrowse
--- PASS: TestLoadCompletionRendersContent
--- PASS: TestMatchRendersInverse
--- PASS: TestGatedLoadKeepsResponsive
--- PASS: TestCtrlCWhileLoadGateHeld
--- PASS: TestQOnBrowseExitsZero
--- PASS: TestEscOnBrowseIsNoOp
--- PASS: TestLoadFailureShowsUnreadable
--- PASS: TestQOnBrowseExitsZeroCleanup
--- PASS: TestGateHoldsSearchingAfterRgExit
PASS
ok  	vrg/internal/app
```

Rendering and sink safety on the composed `View()` string: raw-path file-list order, the underlined current entry, the `── path ───` filename rule, the right-justified gutter with two spaces, and no other panel borders. `TestSinkSafetyRawOutput` drives the hostile fixture — OSC, CSI, C0, C1, DEL, standalone CR, invalid UTF-8 path bytes, an embedded filename newline — through the real composition path under `theme.Plain` (the no-style seam where any escape byte is illegitimate) and asserts on the raw output before ANSI stripping that no fixture control byte survives in the list, rule, or content sinks.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestBrowseRendering|TestSinkSafetyRawOutput' ./internal/app 2>&1 | grep -vE '^(=== RUN|    --- (PASS|FAIL))' | sed -E 's/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//'
```

```output
--- PASS: TestBrowseRendering
--- PASS: TestSinkSafetyRawOutput
PASS
ok  	vrg/internal/app
```

```bash
cd /home/chris/vrg && go build -o Notes/walkthroughs/005-06/code-walkthrough/vrg ./cmd/vrg && cd Notes/walkthroughs/005-06/code-walkthrough && test -x vrg && ./vrg --help | head -1
```

```output
Usage: vrg [OPTIONS] PATTERN [ROOT]
```

Manual browse session with the real binary on a real PTY. `demo-browse.sh` uses `script(1)` to allocate a pseudo-terminal; the inner session sizes it 80x24, runs `./vrg hit .` in a two-file fixture tree, and a background `stty` resizes the pty to 30x100 mid-run. The harness waits for a 70+-dash filename rule — only possible on the widened frame — then sends a literal `q`. The rendered rows below show both frames: the raw-path-ordered list with `./b.c` underlined, the `── ./b.c ──` filename rule, and the right-justified gutter with inverse-video `hit`.

```bash
./demo-browse.sh
```

```output
exit=0
underline=seen
inverse=seen
resize=widened
--- rendered rows ---
./b.c        ── ./b.c ──────────────────────────────────────────────────────────
./src/a.txt  1  int hit = 0;
2  no match
./b.c        ── ./b.c ──────────────────────────────────────────────────────────────────────────────
./src/a.txt  1  int hit = 0;
2  no match
```

Hostile manual case on a real PTY. `demo-hostile.sh` creates a file whose name contains an embedded newline and an ESC byte, with a matched line carrying an OSC title-set sequence (`ESC ] 0 ; pwned BEL`), plus one normal file. The escaped forms appear in every sink — the list and rule show `evil\\nesc^[name.txt`, the content row shows `^[]0;pwned^G` — while the raw pty stream contains no fixture control bytes: with no `ESC ] 0 ;` reaching the terminal, the title cannot be rewritten.

```bash
./demo-hostile.sh
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

## Result. Every Issue #5 behavior is demonstrated: the completed search presents the two-pane browse view — the file list in raw-path order with the current entry underlined and scrolled into view, the escaped filename embedded in a horizontal rule, the right-justified gutter followed by two spaces, no other panel borders, and matched spans in inverse video over the escaped display text via the byte→cell maps. Full files load asynchronously off the update path with the prepared buffer arriving in the completion message — `Loading…` meanwhile, `(unreadable)` on failure — while keys and resizes stay responsive behind the load gate; `q` exits 0 and `ctrl+c` follows the Issue #4 path to 130. The safe-presentation core holds at every sink: hostile path and content bytes render only as escaped forms, proven both by the `theme.Plain` raw-output test and by the live PTY capture where the OSC title-set sequence and the newline/ESC filename arrive as inert text. Deferred by design: the real file-list width formula (Issue #24), structural tab stops (Issue #16), scrolling and reveal (Issues #12–#19), and the all-sink unification of the escaper (Issue #6).
