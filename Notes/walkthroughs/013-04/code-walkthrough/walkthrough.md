# Issue #13: n/p circular matched-line navigation

*2026-09-17T12:35:08Z by Showboat 0.6.1*
<!-- showboat-id: e2728e5b-f4c8-4a05-9e3a-64502df403fc -->

Issue #13 adds the single global matched-line cursor in SearchIndex and its App wiring: startup selects the first stop in path-then-line order, n advances and p retreats circularly with wrap at both ends, zero or one stops make both keys strict no-ops, and multiple submatches on one matched line share their stop. The current file derives from the cursor — crossing to another file's stop switches the panel immediately, requests that file's load when uncached, resumes its saved viewport (or top on a first visit), and moves the list underline — while manual scrolling never moves the cursor and the file list stays a passive overview with no direct selection route. Destination reveal remains Issue #14's, so a same-file move only re-styles the current matched line's matches with the Issue #7 inverse-plus-underline style. See Notes/issues/013-match-navigation-n-p-circular-cursor.md, Notes/tasks/013-match-navigation-n-p-circular-cursor.md, and the 'Navigation, viewport, and logical anchors' (first three bullets) and 'Module Design' sections of Notes/PRD-vrg.md. All artifacts live in this directory.

## Gates — module integrity, build, vet

```bash
cd /home/chris/vrg && go mod verify && go build ./... && go vet ./... && echo GATES-OK
```

```output
all modules verified
GATES-OK
```

## SearchIndex — the circular matched-line cursor

internal/searchindex/cursor_test.go covers the cursor contract over real built indexes: startup selects the first stop in path-then-line order; Next/Prev walk every stop and wrap circularly, reporting Wrapped and FileChanged independently — a wrap inside a one-file index reports Wrapped alone; a single stop makes both directions strict no-ops returning the zero Move, and an empty index has no cursor position at all; several submatches on one matched line are one stop; and the walk follows prepared order, not stream order.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'Cursor|Next|Prev|Wrap|SingleStop|EmptyIndex|SubmatchesShare' ./internal/searchindex 2>&1 | grep -E '^(--- |    --- |ok|FAIL)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestCursorStartsAtFirstStop
--- PASS: TestNextWalksStopsAndWraps
--- PASS: TestPrevRetreatsAndWraps
--- PASS: TestWrapWithinOneFileReportsNoFileChange
--- PASS: TestSingleStopStrictNoOp
--- PASS: TestEmptyIndexNavigationNoOp
--- PASS: TestSubmatchesShareOneStop
--- PASS: TestCursorFollowsPreparedOrder
ok  	vrg/internal/searchindex
```

## App — cursor-derived current file and n/p wiring

internal/app/nav_test.go drives the keys through Update over multi-stop fixtures: startup lands on the first file's first stop with its match underlined; a same-file n returns no command and only moves the current-line underline; crossing into an uncached file issues its load command, moves the list underline and filename rule, and starts the completed file from its top; wrap works at both ends with cached destinations issuing no command; the one-stop index makes n/p strict no-ops; manual scrolling never moves the cursor so n continues from the last selected stop leaving the viewport in place; a departing file's saved top survives leave-and-revisit; navigation stays live under an in-flight load with dedup; and no key beyond n/p can select a file — the list is passive.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'Cursor|Nav|WithinFile|FileBoundary|SingleStop|ManualScroll|CrossFile|FileList' ./internal/app 2>&1 | grep -E '^(--- |    --- |ok|FAIL)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestCurrentFileListEntryUnderlined
--- PASS: TestStartupCursorAtFirstStop
--- PASS: TestNAdvancesWithinFile
--- PASS: TestNCrossingFileBoundarySwitchesPanel
--- PASS: TestNavigationWrapsBothEnds
--- PASS: TestSingleStopIgnoresNavigation
--- PASS: TestManualScrollLeavesCursor
--- PASS: TestCrossFileRestoresDepartingViewport
--- PASS: TestNavigationWhileLoadInFlight
--- PASS: TestFileListHasNoDirectSelection
ok  	vrg/internal/app
```

## Manual PTY — n/p on the real binary

pty_nav.py builds a fixture — a.txt with stops on lines 1 and 15 of 60 lines, b.txt with stops on lines 2 and 4 of 30, c.txt with a stop on line 1 — and runs the real binary on a 100x24 pty. Because the renderer emits a cell-level diff, the script replays the raw byte stream through a small screen emulator that also tracks the SGR underline attribute, then asserts where the cursor actually sits: the filename rule's file, the underlined list entry, and the panel row whose match is underlined. n moves the underline through a.txt's stops, across into b.txt and c.txt with the list underline following, and wraps from the last stop back to the first; p reverses and wraps the other way; scrolling ten rows leaves the cursor on a.txt's first stop (its underline simply goes off-screen), and the following n continues to the next stop — a.txt line 15 — without moving the manually scrolled viewport.

```bash
cd /home/chris/vrg && go build -o Notes/walkthroughs/013-04/code-walkthrough/vrg ./cmd/vrg && cd Notes/walkthroughs/013-04/code-walkthrough && python3 pty_nav.py
```

```output
init      : file=a.txt list=a.txt underline=row 1: '1  match a-one' top='1  match a-one'
n         : file=a.txt list=a.txt underline=row 15: '15  match a-two' top='1  match a-one'
n→b       : file=b.txt list=b.txt underline=row 2: '2  match b-one' top='1  b row 01'
n         : file=b.txt list=b.txt underline=row 4: '4  match b-two' top='1  b row 01'
n→c       : file=c.txt list=c.txt underline=row 1: '1  match c-one' top='1  match c-one'
n(wrap)   : file=a.txt list=a.txt underline=row 1: '1  match a-one' top='1  match a-one'
p(wrap)   : file=c.txt list=c.txt underline=row 1: '1  match c-one' top='1  match c-one'
p         : file=b.txt list=b.txt underline=row 4: '4  match b-two' top='1  b row 01'
p         : file=b.txt list=b.txt underline=row 2: '2  match b-one' top='1  b row 01'
p         : file=a.txt list=a.txt underline=row 15: '15  match a-two' top='1  match a-one'
p         : file=a.txt list=a.txt underline=row 1: '1  match a-one' top='1  match a-one'
scroll↓10 : file=a.txt list=a.txt underline=off-screen top='11  a row 11'
n         : file=a.txt list=a.txt underline=row 5: '15  match a-two' top='11  a row 11'
q         : exit 0
OK
```

## Full suite — go test ./... and the race detector

```bash
cd /home/chris/vrg && go test -count=1 ./... 2>&1 | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//' && CGO_ENABLED=1 go test -count=1 -race ./internal/... ./cmd/vrg 2>&1 | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
?   	vrg/Notes/walkthroughs/007-04/code-walkthrough/fixture	[no test files]
ok  	vrg/cmd/vrg
ok  	vrg/internal/app
ok  	vrg/internal/cli
ok  	vrg/internal/filebuffer
ok  	vrg/internal/safepresentation
?   	vrg/internal/safepresentation/sinktest	[no test files]
ok  	vrg/internal/searchindex
ok  	vrg/internal/theme
ok  	vrg/internal/viewport
ok  	vrg/internal/app
ok  	vrg/internal/cli
ok  	vrg/internal/filebuffer
ok  	vrg/internal/safepresentation
?   	vrg/internal/safepresentation/sinktest	[no test files]
ok  	vrg/internal/searchindex
ok  	vrg/internal/theme
ok  	vrg/internal/viewport
ok  	vrg/cmd/vrg
```
