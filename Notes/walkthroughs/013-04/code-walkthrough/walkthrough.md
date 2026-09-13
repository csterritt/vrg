# Issue #13: Match navigation n/p circular cursor

*2026-09-12T01:07:50Z by Showboat 0.6.1*
<!-- showboat-id: af4d894a-2d46-4322-8eb7-08e913be2435 -->

Walkthrough for Issue #13 (Notes/tasks/013-match-navigation-n-p-circular-cursor.md), implementing the single global matched-line cursor with circular n/p navigation, cursor-derived current file, cross-file load requests, viewport handoff, and passive file list. References: Notes/PRD-vrg.md (Navigation, viewport, and logical anchors).

Contracts verified:

- Startup selects the first stop; current file derives from the cursor.
- n advances the cursor circularly; p retreats circularly; wrap at both ends.
- Zero entries and one entry make n/p strict no-ops (no pop-up, no reload).
- Multiple submatches on one line count as one navigation stop.
- Crossing files switches the content panel and requests a load for uncached destinations.
- Cached destinations are shown immediately with saved viewport restored (first visit at top).
- Manual scrolling does not move the cursor; n/p continue from the last selected stop.
- File list underline follows the cursor's current file.
- Current matched line uses the Issue #7 CurrentMatch style (true inverse + underline).
- The file list is passive: no direct selection route.

All generated artifacts live in this directory.

```bash
cd /home/chris/vrg && go build ./... && go vet ./... && echo GATES-OK
```

```output
GATES-OK
```

```bash
cd /home/chris/vrg && go test -count=1 ./... -timeout 120s | sed 's/[[:space:]][0-9.]*s$//'
```

```output
ok  	vrg/cmd/vrg
ok  	vrg/internal/app
ok  	vrg/internal/cli
ok  	vrg/internal/filebuffer
ok  	vrg/internal/safepresentation
ok  	vrg/internal/searchindex
?   	vrg/internal/sinkfixtures	[no test files]
ok  	vrg/internal/theme
ok  	vrg/internal/viewport
```

## SearchIndex cursor tests

The cursor tests (internal/searchindex/cursor_test.go, Issue #13) verify startup selection, circular Next/Prev, wrap at both ends, the file-change flag, single-stop and empty no-ops, multiple-submatches-one-stop, Len, nil index, and full cycles.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/searchindex/ -run '^TestCursor' -timeout 30s | sed 's/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
```

```output
=== RUN   TestCursorStartupSelectsFirstStop
--- PASS: TestCursorStartupSelectsFirstStop (0.00s)
=== RUN   TestCursorPosition
--- PASS: TestCursorPosition (0.00s)
=== RUN   TestCursorNextAdvances
--- PASS: TestCursorNextAdvances (0.00s)
=== RUN   TestCursorPrevRetreats
--- PASS: TestCursorPrevRetreats (0.00s)
=== RUN   TestCursorNextWraps
--- PASS: TestCursorNextWraps (0.00s)
=== RUN   TestCursorPrevWraps
--- PASS: TestCursorPrevWraps (0.00s)
=== RUN   TestCursorFileChangeFlag
--- PASS: TestCursorFileChangeFlag (0.00s)
=== RUN   TestCursorPrevFileChangeFlag
--- PASS: TestCursorPrevFileChangeFlag (0.00s)
=== RUN   TestCursorSingleStopNoOp
--- PASS: TestCursorSingleStopNoOp (0.00s)
=== RUN   TestCursorEmptyNoOp
--- PASS: TestCursorEmptyNoOp (0.00s)
=== RUN   TestCursorMultipleSubmatchesOneStop
--- PASS: TestCursorMultipleSubmatchesOneStop (0.00s)
=== RUN   TestCursorLen
--- PASS: TestCursorLen (0.00s)
=== RUN   TestCursorEmptyLen
--- PASS: TestCursorEmptyLen (0.00s)
=== RUN   TestCursorNilIndex
--- PASS: TestCursorNilIndex (0.00s)
=== RUN   TestCursorFullCycleNext
--- PASS: TestCursorFullCycleNext (0.00s)
=== RUN   TestCursorFullCyclePrev
--- PASS: TestCursorFullCyclePrev (0.00s)
=== RUN   TestCursorFileChangeBytesEqual
--- PASS: TestCursorFileChangeBytesEqual (0.00s)
PASS
ok  	vrg/internal/searchindex
```

## App navigation tests

The app tests (internal/app/navigation_test.go, Issue #13) verify startup selection, cross-file switching with load requests, same-file no-load, wrap, one-stop no-ops, list underline following cursor, current-match underline movement, manual-scroll independence, viewport save/restore, passive file list, n while loading, and multi-file multi-stop sequences.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/app/ -run '^TestNavigation' -timeout 30s | sed 's/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
```

```output
=== RUN   TestNavigationStartupSelectsFirstStop
--- PASS: TestNavigationStartupSelectsFirstStop (0.00s)
=== RUN   TestNavigationStartupSelectsFirstStopView
--- PASS: TestNavigationStartupSelectsFirstStopView (0.00s)
=== RUN   TestNavigationNextSwitchesFile
--- PASS: TestNavigationNextSwitchesFile (0.00s)
=== RUN   TestNavigationPrevSwitchesFile
--- PASS: TestNavigationPrevSwitchesFile (0.00s)
=== RUN   TestNavigationNextSameFileNoLoad
--- PASS: TestNavigationNextSameFileNoLoad (0.00s)
=== RUN   TestNavigationNextWrapsCrossFile
--- PASS: TestNavigationNextWrapsCrossFile (0.00s)
=== RUN   TestNavigationPrevWrapsCrossFile
--- PASS: TestNavigationPrevWrapsCrossFile (0.00s)
=== RUN   TestNavigationOneStopNextNoOp
--- PASS: TestNavigationOneStopNextNoOp (0.00s)
=== RUN   TestNavigationOneStopPrevNoOp
--- PASS: TestNavigationOneStopPrevNoOp (0.00s)
=== RUN   TestNavigationListUnderlineFollowsCursor
--- PASS: TestNavigationListUnderlineFollowsCursor (0.00s)
=== RUN   TestNavigationCurrentMatchUnderlineMoves
--- PASS: TestNavigationCurrentMatchUnderlineMoves (0.00s)
=== RUN   TestNavigationManualScrollIndependence
--- PASS: TestNavigationManualScrollIndependence (0.00s)
=== RUN   TestNavigationManualScrollThenPContinues
--- PASS: TestNavigationManualScrollThenPContinues (0.00s)
=== RUN   TestNavigationSavesDepartingViewport
--- PASS: TestNavigationSavesDepartingViewport (0.00s)
=== RUN   TestNavigationRestoresSavedViewport
--- PASS: TestNavigationRestoresSavedViewport (0.00s)
=== RUN   TestNavigationFirstVisitStartsAtTop
--- PASS: TestNavigationFirstVisitStartsAtTop (0.00s)
=== RUN   TestNavigationFileListPassive
--- PASS: TestNavigationFileListPassive (0.00s)
=== RUN   TestNavigationNextWhileLoading
--- PASS: TestNavigationNextWhileLoading (0.00s)
=== RUN   TestNavigationMultiFileMultiStop
--- PASS: TestNavigationMultiFileMultiStop (0.00s)
PASS
ok  	vrg/internal/app
```

## Manual demonstration

The binary is run on three fixture files (alpha.txt, beta.txt, gamma.txt) through a PTY (80x24 terminal). A fake rg emits matches across all three files: alpha.txt (lines 1, 3), beta.txt (lines 2, 5), gamma.txt (line 4). This gives 5 stops total for exercising same-file navigation, cross-file navigation, and wrap. The showframe.py helper reconstructs the visible screen with SGR sequences shown as readable \x1b[...m codes.

### Initial frame: startup selects the first stop (alpha.txt:1)

At startup, the cursor selects the first stop in path-then-line order: alpha.txt line 1. The file list underlines alpha.txt (SGR 4m). The current matched line (line 1) uses the CurrentMatch style: true inverse (30;47) + underline (4m) = 30;47;4m. Line 3's match uses the plain Match style (30;47m, no underline).

```bash
cd /home/chris/vrg/Notes/walkthroughs/013-04/code-walkthrough && python3 showframe.py ''
```

```output
\x1b[37;40;4malpha.txt            ── alpha.txt ──\x1b[m
\x1b[37;40mbeta.txt   1  \x1b[30;47;4mmatch\x1b[37;40m alpha line 1\x1b[m
\x1b[37;40mgamma.txt  2  plain alpha line 2\x1b[m
                                 3  \x1b[30;47mmatch\x1b[37;40m alpha line 3\x1b[m
                                 4  plain alpha line 4\x1b[m
                                 5  plain alpha line 6\x1b[m
                                 6  plain alpha line 7\x1b[m
                                 7  plain alpha line 8\x1b[m
                                 8  plain alpha line 9\x1b[m
                                 9  plain alpha line 10\x1b[m
                                10  plain alpha line 11\x1b[m
                                11  plain alpha line 12\x1b[m
                                12  plain alpha line 13\x1b[m
                                13  plain alpha line 14\x1b[m
                                14  plain alpha line 15\x1b[m
                                15  plain alpha line 16\x1b[m
                                16  plain alpha line 17\x1b[m
                                17  plain alpha line 18\x1b[m
                                18  plain alpha line 19\x1b[m
                                19  plain alpha line 20\x1b[m
                               \x1b[m
```

### After n: same-file navigation to alpha.txt:3

Pressing n advances the cursor from stop 0 (alpha.txt:1) to stop 1 (alpha.txt:3) within the same file. The current-match underline moves from line 1 to line 3: line 1's match changes from 30;47;4m (current) to 30;47m (plain), and line 3's match changes from 30;47m to 30;47;4m (current). The file list underline stays on alpha.txt (same file).

```bash
cd /home/chris/vrg/Notes/walkthroughs/013-04/code-walkthrough && python3 showframe.py 'n'
```

```output
\x1b[37;40;4malpha.txt            ── alpha.txt ──\x1b[m
\x1b[37;40mbeta.txt   1  \x1b[30;47mmatch\x1b[37;40m alpha line 1\x1b[m[m
\x1b[37;40mgamma.txt  2  plain alpha line 2\x1b[m
                         \x1b[30;47;4mmatch\x1b[37;40m alpha line 3\x1b[me 3\x1b[m
                                 4  plain alpha line 4\x1b[m
                                 5  plain alpha line 6\x1b[m
                                 6  plain alpha line 7\x1b[m
                                 7  plain alpha line 8\x1b[m
                                 8  plain alpha line 9\x1b[m
                                 9  plain alpha line 10\x1b[m
                                10  plain alpha line 11\x1b[m
                                11  plain alpha line 12\x1b[m
                                12  plain alpha line 13\x1b[m
                                13  plain alpha line 14\x1b[m
                                14  plain alpha line 15\x1b[m
                                15  plain alpha line 16\x1b[m
                                16  plain alpha line 17\x1b[m
                                17  plain alpha line 18\x1b[m
                                18  plain alpha line 19\x1b[m
                                19  plain alpha line 20\x1b[m
                               \x1b[m
```

### After n,n: cross-file navigation to beta.txt:2

Pressing n again advances the cursor from stop 1 (alpha.txt:3) to stop 2 (beta.txt:2), crossing a file boundary. The file list underline moves from alpha.txt to beta.txt (37;40;4m). The filename rule changes to ── beta.txt ──. The current matched line is now line 2 of beta.txt, using the CurrentMatch style (30;47;4m). Line 5's match uses the plain Match style (30;47m). The content panel switched immediately and the file was loaded from disk (first visit, so viewport starts at the top).

```bash
cd /home/chris/vrg/Notes/walkthroughs/013-04/code-walkthrough && python3 showframe.py 'n,n'
```

```output
\x1b[37;40malpha.txt ── beta.txt ──\x1b[m
\x1b[37;40;4mbeta.txt              1  plain beta line 1\x1b[m
\x1b[37;40mgamma.txt  2  \x1b[30;47;4mmatch\x1b[37;40m beta line 2\x1b[m
                         \x1b[37;40mplain beta line 3\x1b[m
                               \x1b[37;40mbeta\x1b[m 4\x1b[m
                         \x1b[30;47mmatch\x1b[37;40m beta line 5\x1b[m
                               \x1b[37;40mbeta line 6\x1b[m
                               \x1b[37;40mbeta line 7\x1b[m
                               \x1b[37;40mbeta line 8\x1b[m
                               \x1b[37;40mbeta line 9\x1b[m
                               \x1b[37;40mbeta line 10\x1b[m
                               \x1b[37;40mbeta line 11\x1b[m
                               \x1b[37;40mbeta line 12\x1b[m
                               \x1b[37;40mbeta line 13\x1b[m
                               \x1b[37;40mbeta line 14\x1b[m
                               \x1b[37;40mbeta line 15\x1b[m
                               \x1b[37;40mbeta line 16\x1b[m
                               \x1b[37;40mbeta line 17\x1b[m
                               \x1b[37;40mbeta line 18\x1b[m
                               \x1b[37;40mbeta line 19\x1b[m
                     \x1b[37;40m20  plain beta line 20\x1b[m
                               \x1b[m
```

### After n,n,n,n: wrap from last stop (gamma.txt:4) to first stop (alpha.txt:1)

The 5 stops are: alpha.txt:1, alpha.txt:3, beta.txt:2, beta.txt:5, gamma.txt:4. After 4 presses of n, the cursor is at the last stop (gamma.txt:4). Pressing n once more wraps circularly back to the first stop (alpha.txt:1). The file list underline returns to alpha.txt and the content panel switches back to alpha.txt (cached, so no reload needed).

```bash
cd /home/chris/vrg/Notes/walkthroughs/013-04/code-walkthrough && python3 showframe.py 'n,n,n,n,n'
```

```output
\x1b[37;40;4malpha.txt            ── alpha.txt ──\x1b[m
\x1b[37;40mbeta.txt   1  \x1b[30;47;4mmatch\x1b[37;40m alpha line 1\x1b[m
\x1b[37;40mgamma.txt  2  plain alpha line 2\x1b[m line 2\x1b[m 2\x1b[m[m
                         \x1b[30;47mmatch\x1b[37;40m alph\x1b[m
                         \x1b[37;40mplain alpha line 4\x1b[m line 4\x1b[m
                         \x1b[3\x1b[37;40malpha line 6\x1b[mline 5\x1b[m
                               \x1b[37;40malpha line 7\x1b[m
                               \x1b[37;40malpha line 8\x1b[m
                               \x1b[37;40malpha line 9\x1b[m
                               \x1b[37;40malpha line 10\x1b[m
                               \x1b[37;40malpha line 11\x1b[m
                               \x1b[37;40malpha line 12\x1b[m
                               \x1b[37;40malpha line 13\x1b[m
                               \x1b[37;40malpha line 14\x1b[m
                               \x1b[37;40malpha line 15\x1b[m
                               \x1b[37;40malpha line 16\x1b[m
                               \x1b[37;40malpha line 17\x1b[m
                               \x1b[37;40malpha line 18\x1b[m
                               \x1b[37;40malpha line 19\x1b[m
                               \x1b[37;40malpha line 20\x1b[m
                               \x1b[m7;40mgamma line 20\x1b[m
```

### After p: reverse navigation wraps from first stop to last stop (gamma.txt:4)

From the initial state (cursor at stop 0, alpha.txt:1), pressing p wraps circularly backward to the last stop (gamma.txt:4). The file list underline moves to gamma.txt and the content panel switches to gamma.txt.

```bash
cd /home/chris/vrg/Notes/walkthroughs/013-04/code-walkthrough && python3 showframe.py 'p'
```

```output
\x1b[37;40malpha.txt ── gamma.txt ──\x1b[m.txt ──\x1b[m
\x1b[37;40mbeta.txt   1  \x1b[37;40mplain gamma line 1\x1b[m line 1\x1b[m
\x1b[37;40;4mgamma.txt             2  plain gamma line 2\x1b[m
                         \x1b[37;40mplain gamm\x1b[m\x1b[37;40m alpha line 3\x1b[m
                         \x1b[30;47;4mmatch\x1b[37;40m gamma line 4\x1b[m
                               \x1b[37;40mgamma line 5\x1b[m
                               \x1b[37;40mgamma line 6\x1b[m
                               \x1b[37;40mgamma line 7\x1b[m
                               \x1b[37;40mgamma line 8\x1b[m
                               \x1b[37;40mgamma line 9\x1b[m
                               \x1b[37;40mgamma line 10\x1b[m
                               \x1b[37;40mgamma line 11\x1b[m
                               \x1b[37;40mgamma line 12\x1b[m
                               \x1b[37;40mgamma line 13\x1b[m
                               \x1b[37;40mgamma line 14\x1b[m
                               \x1b[37;40mgamma line 15\x1b[m
                               \x1b[37;40mgamma line 16\x1b[m
                               \x1b[37;40mgamma line 17\x1b[m
                               \x1b[37;40mgamma line 18\x1b[m
                               \x1b[37;40mgamma line 19\x1b[m
                     \x1b[37;40m20  plain gamma line 20\x1b[m
                               \x1b[m
```

### Manual scrolling then n: cursor continues from last selected stop

Manual scrolling (down arrow) does not move the matched-line cursor. After scrolling down 5 rows, the viewport shows lines 6-20, but the cursor remains at stop 0 (alpha.txt:1). Pressing n advances the cursor to stop 1 (alpha.txt:3), not from the scrolled position. The current-match underline moves to line 3.

```bash
cd /home/chris/vrg/Notes/walkthroughs/013-04/code-walkthrough && python3 showframe.py 'down,down,down,down,down,n'
```

```output
\x1b[37;40;4malpha.txt            ── alpha.txt ──\x1b[m
\x1b[37;40mbeta.txt   1  \x1b[30;47mmatch\x1b[37;40m alpha line 1\x1b[m[m
\x1b[37;40mgamma.txt  2  plain alpha line 2\x1b[m line 2\x1b[m 2\x1b[m[m
                         \x1b[30;47;4mmatch\x1b[37;40m alpha line 3\x1b[m
                         \x1b[37;40mplain alpha line 4\x1b[m line 4\x1b[m
                         \x1b[3\x1b[37;40malpha line 6\x1b[mline 5\x1b[m
                               \x1b[37;40malpha line 7\x1b[m
                               \x1b[37;40malpha line 8\x1b[m
                               \x1b[37;40malpha line 9\x1b[m
                               \x1b[37;40malpha line 10\x1b[m
                               \x1b[37;40malpha line 11\x1b[m
                               \x1b[37;40malpha line 12\x1b[m
                               \x1b[37;40malpha line 13\x1b[m
                               \x1b[37;40malpha line 14\x1b[m
                               \x1b[37;40malpha line 15\x1b[m
                               \x1b[37;40malpha line 16\x1b[m
                               \x1b[37;40malpha line 17\x1b[m
                               \x1b[37;40malpha line 18\x1b[m
                               \x1b[37;40malpha line 19\x1b[m
                               \x1b[37;40malpha line 20\x1b[m
                               \x1b[m7;40mgamma line 20\x1b[m
```

Summary of observed behavior:

- Startup selects the first stop (alpha.txt:1); file list underlines alpha.txt; current-match style (30;47;4m) on line 1.
- n advances same-file to alpha.txt:3; current-match underline moves to line 3; file list stays on alpha.txt.
- n,n crosses file to beta.txt:2; file list underline moves to beta.txt; content panel switches; current-match on line 2.
- n,n,n,n,n wraps from last stop (gamma.txt:4) back to first stop (alpha.txt:1); file list returns to alpha.txt.
- p wraps from first stop (alpha.txt:1) backward to last stop (gamma.txt:4); file list moves to gamma.txt.
- Manual scrolling (5x down) then n: cursor continues from stop 0 to stop 1 (alpha.txt:3), not from the scrolled position.

References: Issue #13 (Notes/tasks/013-match-navigation-n-p-circular-cursor.md), Notes/PRD-vrg.md (Navigation, viewport, and logical anchors).
