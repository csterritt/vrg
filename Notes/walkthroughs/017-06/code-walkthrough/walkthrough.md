# Issue #17: Logical anchor through rewrap and resize; off-UI layout preparation

*2026-09-24T00:22:10Z by Showboat 0.6.1*
<!-- showboat-id: e6eadd77-cc81-4062-9a23-fca6c3fea7fc -->

Walkthrough for [Issue #17](../../../issues/017-logical-anchor-through-rewrap-and-resize.md), implementing the logical anchor and asynchronous layout preparation per `Notes/PRD-vrg.md` (*Navigation, viewport, and logical anchors* — the width-independent anchor, replacement rules, and deliberately lossy EOF clamp; *Resources and responsiveness* — expensive layout preparation off the update path with obsolete results isolated). The viewport now holds an `anchor Target` — a (source line, display-column offset) — and resolves the effective top through `Rows.RowOf` after every resize or row-model swap; `Row.Start` gives each rendered row its logical location. Scrolling and moving reveals replace the anchor; no-scroll reveals and clamped-to-nothing scrolls keep it; the EOF clamp rewrites it to the clamped row. `viewport.Prepare` runs in a worker command keyed by (path, content revision, text width, wrap mode); `Update` installs a completion only while its key equals the current parameters, so out-of-order and superseded layouts are inert, and a pending reveal intent commits for the newest stop. Rendering stays limited to the visible rows and the visible file-list window via the `listEntry` provider seam. All generated artifacts live in this directory: the built `vrg` binary, the `demo-anchor.sh` tmux harness, and its `anchor/` session captures. Test durations are stripped so the document verifies cleanly.

```bash
set -o pipefail; cd /home/chris/vrg && test -z "$(gofmt -l internal/)" && echo "GOFMT-CLEAN" && go vet ./... && go build ./... && go test -count=1 ./... | sed "s/[[:space:]][0-9.]*s$//" && CGO_ENABLED=1 go test -race -count=1 ./internal/app ./internal/viewport | sed "s/[[:space:]][0-9.]*s$//" && go build -o Notes/walkthroughs/017-06/code-walkthrough/vrg ./cmd/vrg && echo GATES-OK
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
ok  	vrg/internal/viewport
GATES-OK
```

## Anchor round-trip and EOF-clamp tests

`internal/viewport/anchor_test.go` pins the width-independent anchor against real prepared models: a top mid-way through a 200-cell line (cell 150, row 15 at width 10) lands on the row covering cell 150 at width 20 (row 7) and round-trips back exactly; the same holds for later-line anchors; wrap off then on restores the retained logical column; scrolling and moving reveals replace the anchor with the resulting top row's location (run-off-edge drops the column for the row's own start); a no-scroll reveal keeps it; and the EOF clamp rewrites the anchor to the clamped row — the loss is permanent, so narrowing back lands on the clamped location, not the pre-clamp one. `internal/app/anchor_test.go` proves a resize preserves the cursor selection and keeps the anchored text at the panel's top.

```bash
cd /home/chris/vrg && go test -count=1 -v -run "TestAnchor|TestScrollReplacesAnchor|TestMovingReveal|TestNoScrollReveal|TestEOFClamp" ./internal/viewport 2>&1 | grep -vE "^(=== RUN|=== CONT)" | sed -E "s/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//" && go test -count=1 -v -run "TestResizePreservesCursorSelection|TestResizeKeepsAnchorTextAtTop" ./internal/app 2>&1 | grep -vE "^(=== RUN|=== CONT)" | sed -E "s/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//"
```

```output
--- PASS: TestAnchorRoundTripThroughRewrap
--- PASS: TestAnchorRoundTripOnLaterLine
--- PASS: TestAnchorRoundTripThroughWrapToggle
--- PASS: TestScrollReplacesAnchor
--- PASS: TestMovingRevealReplacesAnchor
--- PASS: TestNoScrollRevealKeepsAnchorColumn
--- PASS: TestMovingRevealInRunOffEdgeDropsColumn
--- PASS: TestEOFClampRewritesAnchorLossy
PASS
ok  	vrg/internal/viewport
--- PASS: TestResizePreservesCursorSelection
--- PASS: TestResizeKeepsAnchorTextAtTop
PASS
ok  	vrg/internal/app
```

## Gated preparation, isolation, and cached-file tests

`internal/app/layout_test.go` pins the off-UI contract. `TestResizeRequestsLayoutOffUpdatePath` proves a resize returns a keyed layout command rather than rewrapping inline. `TestLayoutWorkerHeldKeepsEveryInputActionable` holds the worker (the returned command, simply uninvoked) after a resize and drives every required input — `ctrl+c` → 130, `q` → the fixed status, `n`/`p` moving the cursor immediately with the newest stop's reveal intent preserved, `w` flipping wrap and issuing the new-mode request without releasing the held one, and a second resize accepted — then releasing shows the reveal landing per Issue #14. `TestOutOfOrderLayoutCompletionsInstallNewestOnly` and `TestRapidWrapToggleDiscardsStaleMode` prove superseded completions across W1→W2→W3 resizes and rapid toggles never install and never touch the anchor. `TestLayoutForDepartedFileLeavesPanelAndSavedState` and `TestObsoleteLayoutDoesNotConsumePendingReveal` isolate late results from the panel, saved state, and the pending intent. `TestStaleLayoutNavigationCarriesSavedAndRevealIntent` and `TestMatchingLayoutNavigationCommitsImmediately` cover cached-file navigation — a stale-keyed layout is reprepared for the current parameters with the intent carried to install, while a matching layout commits at once with no request. `TestRenderQueriesOnlyVisibleListEntries` joins the row-level guard: a counting `listEntry` fake proves a 200-file frame paints only the scrolled window.

```bash
cd /home/chris/vrg && go test -count=1 -v -run "TestResizeRequestsLayoutOffUpdatePath|TestLayoutWorkerHeld|TestCtrlCWhileLayoutPending|TestQWhileLayoutPending|TestOutOfOrderLayout|TestRapidWrapToggle|TestLayoutForDepartedFile|TestObsoleteLayout|TestStaleLayoutNavigation|TestMatchingLayoutNavigation|TestRenderQueriesOnlyVisibleListEntries" ./internal/app 2>&1 | grep -vE "^(=== RUN|=== CONT)" | sed -E "s/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//"
```

```output
--- PASS: TestResizeRequestsLayoutOffUpdatePath
--- PASS: TestLayoutWorkerHeldKeepsEveryInputActionable
--- PASS: TestCtrlCWhileLayoutPendingExits130
--- PASS: TestQWhileLayoutPendingQuitsFixedStatus
--- PASS: TestOutOfOrderLayoutCompletionsInstallNewestOnly
--- PASS: TestRapidWrapToggleDiscardsStaleMode
--- PASS: TestLayoutForDepartedFileLeavesPanelAndSavedState
--- PASS: TestObsoleteLayoutDoesNotConsumePendingReveal
--- PASS: TestStaleLayoutNavigationCarriesSavedAndRevealIntent
--- PASS: TestMatchingLayoutNavigationCommitsImmediately
--- PASS: TestRenderQueriesOnlyVisibleListEntries
PASS
ok  	vrg/internal/app
```

## Manual check — anchor, wrap toggle, and lossy EOF clamp on a real PTY

`demo-anchor.sh` (checked into this directory) runs the freshly built `vrg` on real tmux PTYs — `window-size manual` plus `resize-window` deliver genuine SIGWINCH resizes to the detached panes. Part A's fixture `big.txt` is 90 lines: `hit00001` (the one stop), 39 short lines, a 600-cell line 41 with `MARKER` at display cell 268 (a wrap-row boundary at the 67-cell text width), 48 more short lines, and a 300-cell last line. After 44 `down` presses the top row is the MARKER row — rendered row 44, logical location (line 41, cell 268). Narrowing to 60 (text 47) and widening to 100 (text 87) keep MARKER on the top row — at offsets 33 and 7, since the anchor tracks the *location*, not the row ordinal — and back at 80 MARKER leads again. `w` collapses line 41 to one clipped run-off-edge row (the column is invisible but retained); `w` again restores the MARKER row. Then `pgdown` twice lands at the last full page (top line 72); widening to 140 pulls the top up to line 70 — the deliberate EOF clamp — and the clamped position survives narrowing back and widening again: the old top is gone for good. Part B loads a ~50 MB, ~530k-line fixture, fires 24 alternating resizes (each minting a keyed layout request and superseding the last), then sends `ctrl+c` mid-rewrap: exit 130, promptly.

```bash
cd /home/chris/vrg/Notes/walkthroughs/017-06/code-walkthrough && ./demo-anchor.sh
```

```output
ok: startup: file panel -> big.txt
ok: startup: top line -> hit00001
ok: scrolled: MARKER leads the top row -> MARKER
ok: scrolled: continuation gutter ->     
ok: narrow 60: MARKER still on the top row (at offset 33)
ok: widen 100: MARKER still on the top row (at offset 7)
ok: back to 80: MARKER leads the top row -> MARKER
ok: w: run-off-edge top is line 41 -> 41  
ok: w: clipped row from cell 0 -> xxx
ok: w back: MARKER leads the top row -> MARKER
ok: w back: continuation gutter ->     
ok: EOF: top line is 72 at 80 cols (last full page)
ok: widen 140: top pulled up to line 70 (EOF clamp)
ok: narrow back: line 70 stays at top — the old top is lost
ok: widen again: line 70 still at top
ok: part A exit status -> 0
fixture size: 49M
ok: ctrl+c mid-rewrap: exit status -> 130
ok: resize burst + ctrl+c exited in under 15s
demo-anchor: all checks passed
```

```bash
cd /home/chris/vrg/Notes/walkthroughs/017-06/code-walkthrough && echo "== 80 cols, scrolled into the wrapped line: MARKER leads the top row ==" && sed -n "2p" anchor/screen-01-marker-top-80.txt | sed "s/ *$//" && echo "== 60 cols, same anchor (line 41, cell 268): MARKER at offset 33 ==" && sed -n "2p" anchor/screen-02-marker-top-60.txt | sed "s/ *$//" && echo "== back to 80: MARKER leads again — the round trip loses nothing ==" && sed -n "2p" anchor/screen-03-marker-top-back-80.txt | sed "s/ *$//" && echo "== w: run-off-edge, line 41 is one clipped row (gutter 41) ==" && sed -n "2,3p" anchor/screen-04-runoff.txt | sed "s/ *$//" && echo "== w again: the retained column restores the MARKER row ==" && sed -n "2p" anchor/screen-05-wrap-again.txt | sed "s/ *$//"
```

```output
== 80 cols, scrolled into the wrapped line: MARKER leads the top row ==
             MARKERxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
== 60 cols, same anchor (line 41, cell 268): MARKER at offset 33 ==
             xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxMARKERxxxxxxxx
== back to 80: MARKER leads again — the round trip loses nothing ==
             MARKERxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
== w: run-off-edge, line 41 is one clipped row (gutter 41) ==
         41  xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
         42  u000042
== w again: the retained column restores the MARKER row ==
             MARKERxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

```bash
cd /home/chris/vrg/Notes/walkthroughs/017-06/code-walkthrough && echo "== EOF at 80 cols: last full page leads with line 72 ==" && sed -n "2,3p" anchor/screen-06-eof-80.txt | sed "s/ *$//" && echo "...tail rows at the bottom of the pane:" && tail -3 anchor/screen-06-eof-80.txt | sed "s/ *$//" && echo "== widened to 140: the clamp pulled the top up to line 70 ==" && sed -n "2,3p" anchor/screen-07-eof-140.txt | sed "s/ *$//" && echo "== narrowed back to 80: line 70 stays — the pre-clamp top is gone ==" && sed -n "2,3p" anchor/screen-08-eof-back-80.txt | sed "s/ *$//"
```

```output
== EOF at 80 cols: last full page leads with line 72 ==
         72  u000072
         73  u000073
...tail rows at the bottom of the pane:
             zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz
             zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz
             zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz
== widened to 140: the clamp pulled the top up to line 70 ==
         70  u000070
         71  u000071
== narrowed back to 80: line 70 stays — the pre-clamp top is gone ==
         70  u000070
         71  u000071
```

## Verdict

Issue #17 is verified. The reading position is a width-independent logical anchor — `(source line, display-column offset)` — resolved to the effective top through `Rows.RowOf` after every resize, rewrap, or row-model swap, with `Row.Start` giving each rendered row its logical location. Scrolling and moving reveals replace the anchor; no-scroll reveals and clamped-to-nothing scrolls retain the column; the EOF clamp deliberately rewrites the anchor to the clamped row. Layout preparation runs in worker commands keyed by (path, content revision, text width, wrap mode), installing only on an exact key match — out-of-order and superseded completions are inert, pending reveal intents commit for the newest stop, and cached-file navigation reprepares stale layouts while a matching layout commits immediately. Frames render only visible rows and visible list entries. The tmux run shows the contract end to end: MARKER stays at the top through narrow/widen round trips and a `w` round trip, the widened EOF clamp pulls the top from line 72 to line 70 permanently, and `ctrl+c` during a 50 MB rewrap burst exits with 130 promptly.
