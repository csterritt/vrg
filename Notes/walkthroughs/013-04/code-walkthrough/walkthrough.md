# Issue #13: Circular matched-line navigation (n/p)

*2026-09-23T21:51:11Z by Showboat 0.6.1*
<!-- showboat-id: a2f2db36-101b-4f3d-bce5-7244ff2de0a0 -->

Walkthrough for [Issue #13](../../../issues/013-match-navigation-n-p-circular-cursor.md), implementing the circular matched-line cursor per `Notes/PRD-vrg.md` (*Navigation, viewport, and logical anchors* — the matched-line cursor bullet: `n`/`p` step through match-bearing lines in path-then-line order and wrap, the file list follows passively, and scrolling is independent). `searchindex.Index` owns a single global cursor over the prepared stops — one stop per matched line, ordered by raw path bytes then source line — and `Next`/`Prev` return a `Step` describing the destination, whether the cursor moved, whether it crossed files, and whether it wrapped; zero- and one-stop indexes are strict no-ops. The App derives the current file from `Index.Current()` — crossing files saves the departing viewport, restores the destination's saved top or starts at the top, and requests a load when the file is uncached, while same-file steps only restyle the current line (destination reveal is Issue #14's). All generated artifacts live in this directory: the built `vrg` binary, the `demo-nav.sh` tmux harness, and its `match-nav/` session captures. Test durations are stripped so the document verifies cleanly.

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
ok  	vrg/internal/theme
ok  	vrg/internal/viewport
GATES-OK
```

## SearchIndex cursor tests

`internal/searchindex/cursor_test.go` pins the cursor contract on the index itself. `TestCursorStartsAtFirstStop` proves the cursor begins on the first stop in path-then-line order even when records arrive out of order; `TestCursorNextAdvancesAndWraps` and `TestCursorPrevRetreatsAndWraps` walk a three-stop index in both directions, asserting each `Step`'s destination plus its `Moved`/`FileChanged`/`Wrapped` flags — including the wrap at each end; `TestCursorOneStopIsStrictNoOp` and `TestCursorEmptyIndexIsNoOp` cover the degenerate indexes where `n`/`p` must report `Moved=false`; and `TestCursorSubmatchesShareOneStop` proves two submatches on one line merge into a single stop so `n` does not rest on the same line twice.

```bash
cd /home/chris/vrg && go test -count=1 -v -run TestCursor ./internal/searchindex 2>&1 | grep -vE "^(=== RUN|=== CONT|    --- (PASS|FAIL))" | sed -E "s/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//"
```

```output
--- PASS: TestCursorStartsAtFirstStop
--- PASS: TestCursorNextAdvancesAndWraps
--- PASS: TestCursorPrevRetreatsAndWraps
--- PASS: TestCursorOneStopIsStrictNoOp
--- PASS: TestCursorEmptyIndexIsNoOp
--- PASS: TestCursorSubmatchesShareOneStop
PASS
ok  	vrg/internal/searchindex
```

## App wiring tests

`internal/app/nav_test.go` drives `n`/`p` through `Update`. `TestStartupSelectsFirstStopInPathOrder` shows the cursor-derived selection: b.txt's records arrive first but a.txt:1 is selected — its entry is underlined, its load is requested, and its line-1 match renders inverse+underlined while line 3's stays plain inverse. `TestNextWithinFileMovesCurrentLine` proves a same-file `n` restyles the match without moving the viewport or returning a command. `TestNextAcrossFileSwitchesPanelAndLoads` scrolls a.txt, crosses to uncached b.txt — the panel switches to `Loading…` with the list underline on b.txt, the returned command is b.txt's load, b.txt opens at the top, and a.txt's scrolled top lands in `m.saved`. `TestNavigationWrapsBothEnds` wraps forward last→first and backward first→last across already-cached files with no loads. `TestOneStopIndexIgnoresNP` is the strict no-op: no command and a byte-identical frame. `TestManualScrollThenNContinuesFromStop` scrolls deep into a.txt then proves `n` advances from the last selected stop — a.txt:3 — not the scrolled position. `TestDepartingViewportSavedAndRestoredOnRevisit` returns to a.txt at its saved top of 5. `TestFileListHasNoDirectSelection` confirms keys outside the navigation set move neither cursor nor frame — the list is passive.

```bash
cd /home/chris/vrg && go test -count=1 -v -run "TestStartupSelectsFirstStopInPathOrder|TestNextWithinFileMovesCurrentLine|TestNextAcrossFileSwitchesPanelAndLoads|TestNavigationWrapsBothEnds|TestOneStopIndexIgnoresNP|TestManualScrollThenNContinuesFromStop|TestDepartingViewportSavedAndRestoredOnRevisit|TestFileListHasNoDirectSelection" ./internal/app 2>&1 | grep -vE "^(=== RUN|=== CONT|    --- (PASS|FAIL))" | sed -E "s/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//"
```

```output
--- PASS: TestStartupSelectsFirstStopInPathOrder
--- PASS: TestNextWithinFileMovesCurrentLine
--- PASS: TestNextAcrossFileSwitchesPanelAndLoads
--- PASS: TestNavigationWrapsBothEnds
--- PASS: TestOneStopIndexIgnoresNP
--- PASS: TestManualScrollThenNContinuesFromStop
--- PASS: TestDepartingViewportSavedAndRestoredOnRevisit
--- PASS: TestFileListHasNoDirectSelection
PASS
ok  	vrg/internal/app
```

## Manual check — n/p on a real PTY across three files

`demo-nav.sh` (checked into this directory) runs the freshly built `vrg` on a real tmux PTY at 80x24. Its `fakebin/rg` reports five stops across three files — `a.txt` lines 1 and 4, `b.txt` lines 2 and 5, `c.txt` line 3 — and the script sends real keypresses, asserting at each step the filename rule (`── <path> ──`), the underlined file-list entry, and the inverse+underlined current match (detected via `capture-pane -e` SGR sequences). Startup selects `a.txt:1`; `n` restyles the line-4 match with the viewport unmoved, crosses into `b.txt` and `c.txt` with the list underline following, then wraps last→first back to `a.txt:1`; `p` wraps first→last to `c.txt:3` and steps back to `b.txt:5`; a five-row manual scroll of `b.txt` leaves the cursor put so the next `n` continues to `c.txt:3` from `b.txt:5`; and `p` revisits `b.txt` at its saved top of 5 (`b006` on the first content row). Plain-text screen captures are stored under `match-nav/`.

```bash
cd /home/chris/vrg && go build -o Notes/walkthroughs/013-04/code-walkthrough/vrg ./cmd/vrg && cd Notes/walkthroughs/013-04/code-walkthrough && ./demo-nav.sh
```

```output
ok: startup: file panel -> a.txt
ok: startup: list underline -> a.txt
ok: startup: current match -> hit a001
ok: n same-file: file panel -> a.txt
ok: n same-file: current match -> hit a004
ok: n same-file: viewport unmoved -> hit a001
ok: n cross-file: file panel -> b.txt
ok: n cross-file: list underline -> b.txt
ok: n cross-file: current match -> hit b002
ok: n same-file: current match -> hit b005
ok: n cross-file: list underline -> c.txt
ok: n cross-file: current match -> hit c003
ok: n wrap last->first: file panel -> a.txt
ok: n wrap last->first: list underline -> a.txt
ok: n wrap last->first: current match -> hit a001
ok: p wrap first->last: file panel -> c.txt
ok: p wrap first->last: current match -> hit c003
ok: p step back: current match -> hit b005
ok: manual scroll: file panel unchanged -> b.txt
ok: n continues from last stop -> hit c003
ok: revisit restores saved viewport -> b006
ok: exit status -> 0
demo-nav: all checks passed
```

```bash
cd /home/chris/vrg/Notes/walkthroughs/013-04/code-walkthrough && echo "--- screen after n wrapped last->first (a.txt:1) ---" && cat match-nav/screen-05-n-wrap.txt | head -8 && echo "--- screen after p revisit: b.txt at saved top 5 ---" && cat match-nav/screen-08-revisit.txt | head -8
```

```output
--- screen after n wrapped last->first (a.txt:1) ---
a.txt  ── a.txt ────────────────────────────────────────────────────────────────
b.txt   1  hit a001
c.txt   2  a002
        3  a003
        4  hit a004
        5  a005
        6  a006
        7  a007
--- screen after p revisit: b.txt at saved top 5 ---
a.txt  ── b.txt ────────────────────────────────────────────────────────────────
b.txt   6  b006
c.txt   7  b007
        8  b008
        9  b009
       10  b010
       11  b011
       12  b012
```

## Verdict

Issue #13 is verified. `searchindex.Index` owns the single global matched-line cursor: `Current` reports the selected `Stop`, and `Next`/`Prev` return a `Step` carrying the destination plus `Moved`/`FileChanged`/`Wrapped` — circular at both ends, a strict no-op on empty and one-stop indexes, with submatches on one line sharing a stop. The App holds no cursor of its own: `currentStop`/`currentPath` derive from the index, `n`/`p` step it in browse mode, same-file steps only restyle the current match (destination reveal is Issue #14's), and `FileChanged` crossings save the departing viewport, restore the destination's saved top or start at the top, and request a load when uncached — the filename rule, file-list underline, and inverse+underlined match all follow the cursor. The list remains passive with no direct selection route, and manual scrolling changes only viewport state so a later `n` continues from the last selected stop. The tmux run shows every one of these behaviors on a real PTY across three matched files.

