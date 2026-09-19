# Issue #19: minimal horizontal reveal in run-off-edge mode

*2026-09-17T20:05:01Z by Showboat 0.6.1*
<!-- showboat-id: a1f58779-2dd8-4214-8c81-88efaa037a49 -->

Issue #19 lands the minimal horizontal reveal: in run-off-edge mode, at startup and on every actual n/p transition — same-file steps included — a horizontally hidden display target (the first submatch's start cell) moves the pan offset by the minimum number of columns that paints it. Visibility is painted-cell visibility: viewport.CellVisible checks that the whole grapheme cluster holding the target cell fits inside the window, so a position geometrically inside but clipped to a blank still counts as hidden. RevealOff resolves the target cell to its cluster via the shared Line.Clusters segmentation: hidden left → off = start (the target column itself); hidden right → off = start + cluster width − text width, so a two-cell cluster paints both cells at the right edge; an oversized match is revealed by its start cell alone; and a cluster wider than the whole text area falls back to off = start, counted as geometrically revealed so repeated navigation cannot loop — while CellVisible still reports it unpainted for Issue #20's indicators. model.reveal runs RevealOff after the vertical Reveal and writes the viewport back when either moved, so the rule rides every existing trigger: the pending startup reveal, each n/p step, and the file-change path after ResetOff. Wrap mode is a strict no-op. See Notes/issues/019-minimal-horizontal-reveal.md, Notes/tasks/019-minimal-horizontal-reveal.md, and the PRD sections "Navigation, viewport, and logical anchors" and "Layout and indicators" in Notes/PRD-vrg.md. All artifacts live in this directory.

## Gates — module integrity, build, vet

```bash
cd /home/chris/vrg && go mod verify && go build ./... && go vet ./... && echo GATES-OK
```

```output
all modules verified
GATES-OK
```

## Reveal arithmetic — painted-cell visibility over real prepared buffers

internal/viewport/reveal_test.go pins the contract: the right-edge rule uses the target cluster's width (a two-cell target lands at start + 2 − textW, fully painted); the left-edge rule lands on the target column; a target whose start cell is already painted never moves; a geometrically inside position clipped to a blank counts as hidden; an oversized match reveals by its start cell only; a cluster wider than the text area falls back to its start column and is treated as geometrically revealed — a repeated RevealOff is a no-op, so navigation cannot loop; a zero-width match's marker cell (one past the last cell) is a one-cell target; and wrap or empty models are strict no-ops.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'RevealOff|CellVisible' ./internal/viewport 2>&1 | grep -E '^(--- |ok|FAIL)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestRevealOffRightOfView
--- PASS: TestRevealOffLeftOfView
--- PASS: TestRevealOffPaintedTargetKeepsOffset
--- PASS: TestRevealOffClippedBlankCountsHidden
--- PASS: TestRevealOffOversizedSpanByStartCell
--- PASS: TestRevealOffUnpaintableClusterFallback
--- PASS: TestRevealOffMarkerCell
--- PASS: TestRevealOffNoOpWrapAndEmpty
--- PASS: TestCellVisible
ok  	vrg/internal/viewport
```

## App wiring — triggers, clipped blanks, unpaintable clusters

internal/app/hreveal_test.go drives the same contracts through Update/View: same-file n reveals right to 300 + 1 − 40 with the match start at the right edge, an already-painted match moves nothing, and the wrap-around n reveals left to the target column; the pending startup reveal applies the horizontal rule when run-off-edge was toggled before the first load; the file-change reset zeroes the offset before the reveal; a CJK match paints both cells of its first glyph at the right edge; a geometrically inside but clipped-blank target still triggers the reveal; an unpaintable six-cell tab cluster renders all-blank at its start column with no loop across repeated navigation; and the rule is inert in wrap mode.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'SameFileNavTriggers|StartupRevealApplies|FileChangeResetsOffset|HorizontalReveal' ./internal/app 2>&1 | grep -E '^(--- |ok|FAIL)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestSameFileNavTriggersHorizontalReveal
--- PASS: TestStartupRevealAppliesHorizontalReveal
--- PASS: TestFileChangeResetsOffsetBeforeHorizontalReveal
--- PASS: TestHorizontalRevealPaintsWideClusterAtRightEdge
--- PASS: TestHorizontalRevealClippedBlankCountsHidden
--- PASS: TestHorizontalRevealUnpaintableClusterFallback
--- PASS: TestHorizontalRevealInertInWrapMode
ok  	vrg/internal/app
```

## Manual check — the real binary on a pty

pty_reveal.py runs the built vrg on a real 100x24 pty and replays the byte stream through a small terminal emulator, over one generated fixture file whose four stops land at cells 5, 300, 270, and 300 (the last a 文-headed match — 文 is one two-cell cluster, found by the regex alternation needle|文needle). The session toggles w into run-off-edge, then: the startup reveal leaves the visible cell-5 target at offset 0; n to the cell-300 match moves the offset to 301 − textW so the match's first cell paints at the right edge; n to the cell-270 match — already painted inside the window — moves nothing; n to the 文 match moves to 302 − textW so both cells of the wide cluster paint at the right edge; and the wrapping n back to the cell-5 match moves the offset left to the target column itself.

```bash
cd /home/chris/vrg && go build -o Notes/walkthroughs/019-04/code-walkthrough/vrg ./cmd/vrg && python3 Notes/walkthroughs/019-04/code-walkthrough/pty_reveal.py
```

```output
w        : run-off-edge, startup reveal — offset stays 0 — '1  xxxxxneedle'
n        : far match hidden right — off = 234 — '2  yyyy'…'yyyyyyyyn '
n        : cell-270 match already painted — offset stays 234 — '3  zzzz'…'zzzzzzzzneedle'
n        : CJK match — off = 235 paints 文 whole — '4  xxxx'…'xxxxxxxx文 '
n        : wrapped to cell 5 — off = 5, the target column — '1  needle'
q        : exit 0
OK
```

## Full suite

```bash
cd /home/chris/vrg && go test -count=1 ./... 2>&1 | grep -vE '^(=== RUN|--- PASS|=== PAUSE|=== CONT|    )' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
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
```

All Issue #19 contracts verified: in run-off-edge mode the display target's start cell is horizontally revealed with the minimum offset movement — off = start hidden left, off = start + cluster width − textW hidden right (a two-cell cluster paints both cells at the right edge), and no movement at all when the start cell is already painted. Visibility is painted-cell visibility, so a geometrically inside position clipped to a blank still reveals; an oversized match needs only its start cell; and a cluster wider than the text area takes the geometric fallback at its start column — geometrically revealed, idempotent across repeated navigation, with no panning loop, while CellVisible still reports it unpainted for Issue #20's indicators. The rule rides model.reveal's single entry point: the pending startup reveal, every same-file n/p step, and the file-change path after ResetOff — and is a strict no-op in wrap mode. Issue #20 owns the reserved indicator column's content; Issue #21 owns cluster-expanded match spans; Issue #23 owns end-of-line markers.
