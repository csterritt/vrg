# Issue #18: horizontal panning in run-off-edge mode

*2026-09-17T19:22:05Z by Showboat 0.6.1*
<!-- showboat-id: 42a5594c-df27-42ed-a650-77a98e51481e -->

Issue #18 lands horizontal panning for run-off-edge mode: the saved per-file viewport gains a display-cell offset `off`, panned by `,`/`.` (one cell), `<`/`>` (ten cells), and `[`/`]` (`HalfText` = max(1, floor(textW/2)) cells), clamped on every keypress to the paintable boundary of the widest currently *visible* line — `viewport.MaxOff` evaluates only rows in `[top, top+height)` via the new `Extent` interface, and `filebuffer.Line.MaxStart` returns the largest grapheme-cluster start that fits the text width, so a trailing wide cluster counts only when it paints whole. The offset re-clamps whenever the visible set changes — pan, scroll, reveal, clamp, restore, resize, and wrap re-entry all call `clampOff` — while wrap mode retains the stored value untouched (re-entry re-clamps against the new visible rows); a file change resets it to zero before the destination reveal. Rendering starts at `off` and never splits a cluster: a cell that lands inside a wide grapheme paints blank, never half a glyph. See `Notes/issues/018-horizontal-panning.md`, `Notes/tasks/018-horizontal-panning.md`, and the PRD sections "Navigation, viewport, and logical anchors" and "Wrapped and run-off-edge display" in `Notes/PRD-vrg.md`. All artifacts live in this directory.

## Gates — module integrity, build, vet

```bash
cd /home/chris/vrg && go mod verify && go build ./... && go vet ./... && echo GATES-OK
```

```output
all modules verified
GATES-OK
```

## Pan units, paintable boundary, and the visible-extent contract

`internal/viewport/pan_test.go` pins the panning contract over real prepared buffers: `,`/`.` and `<`/`>`/`[`/`]` move by one cell, ten cells, and `HalfText`; `Pan` clamps to `[0, MaxOff]` where `MaxOff` is the largest paintable cluster start among visible rows only — a trailing wide cluster counts at its start cell, a final cluster too wide to fit falls back to the last fitting one, and empty or placeholder views clamp to zero. Wrap mode makes `Pan` a strict no-op while retaining the stored offset; re-entry re-clamps against the new visible set; `Scroll`, `Reveal`, `Clamp`, and `Restore` re-clamp on every visible-set change; and the extent-evaluation spy proves only `[top, top+height)` rows are touched — the file is never rescanned.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'Pan|MaxOff|HalfText|Extent|ResetOff|UniformLines|WrapReentry|Reclamp' ./internal/viewport 2>&1 | grep -E '^(--- |ok|FAIL)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestHalfTextUnit
--- PASS: TestPanUnitsMoveColumns
--- PASS: TestPanClampsToPaintableBoundary
--- PASS: TestPanNoOpInWrapMode
--- PASS: TestPanOffsetRetainedThroughWrapToggle
--- PASS: TestWrapReentryClampsToNewVisibleSet
--- PASS: TestMaxOffFollowsVisibleLines
--- PASS: TestMaxOffEmptyViews
--- PASS: TestMaxOffTrailingWideCluster
--- PASS: TestMaxOffUnpaintableFinalCluster
--- PASS: TestRevealReclampsOffset
--- PASS: TestClampReclampsOffset
--- PASS: TestUniformLinesAllHiddenLeftIsLegal
--- PASS: TestResetOff
--- PASS: TestExtentEvaluationTouchesOnlyVisibleRows
ok  	vrg/internal/viewport
```

## App wiring — keys, wrap retention, file-change reset, grapheme-safe clipping

`internal/app/pan_test.go` drives the same contracts through the model: the six pan keys shift rendered text by the right units and clamp at both edges; they are no-ops in wrap mode and on the `Loading…`/`(unreadable)` placeholders (no viewport state is created); the offset survives a `w`,`w` round trip but resets to zero when `n`/`p` cross into another file; a left edge inside a wide cluster renders blank cells rather than a clipped glyph; at the maximum offset the final cluster paints whole; and scroll, reveal, wrap re-entry, and resize each re-clamp the offset against the newly visible lines without restoring a stale larger value.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestPan|Reclamp|WrapReentry' ./internal/app 2>&1 | grep -E '^(--- |ok|FAIL)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestPanKeysShiftText
--- PASS: TestPanKeysNoOpInWrapMode
--- PASS: TestPanKeysClampAtLeftEdge
--- PASS: TestPanOffsetSurvivesWrapToggle
--- PASS: TestPanResetsOnFileChange
--- PASS: TestPanClippedClusterRendersBlank
--- PASS: TestPanMaximumPaintsFinalCluster
--- PASS: TestScrollReclampsOffsetLeftwards
--- PASS: TestRevealReclampsOffset
--- PASS: TestWrapReentryReclampsOffset
--- PASS: TestResizeReclampsOffset
--- PASS: TestPanKeysNoOpOnPlaceholder
--- PASS: TestResizeReclampsViewport
ok  	vrg/internal/app
```

## Manual check — the real binary on a pty

`pty_pan.py` runs the built vrg on a real 100x24 pty and replays the byte stream through a small terminal emulator, over four generated fixture files: `a.txt` ("needle" + 294 cells of `x`, then 40 `pad` lines), `b.txt` ("needle second" + y's), `c.txt` ("needle ab文" + 200 c's — 文 is one two-cell cluster), and `d.txt` ("needle" + 293 x's + a trailing 文). The session toggles `w` into run-off-edge, then: `,` at offset 0 leaves the frame identical; `.` shifts the window one cell; `>` ten; `]` half the text width; `w`,`w` retains the offset through the wrap toggle; `n` into b.txt starts at offset 0; on c.txt `>` lands the offset inside 文's cluster and the clipped cell renders blank; on d.txt `>` pans clamp at 299 — the trailing 文's start — painting it whole, after which `.`, `>`, `]` move nothing; back on a.txt, `↓` scrolls the long line out and the offset re-clamps to the pad lines' maximum (2), and `↑` brings the long line back without restoring the lost offset.

```bash
cd /home/chris/vrg && go build -o Notes/walkthroughs/018-04/code-walkthrough/vrg ./cmd/vrg && python3 Notes/walkthroughs/018-04/code-walkthrough/pty_pan.py
```

```output
w          : run-off-edge — ' 1  needlexxxxxxxx'…
,          : offset 0 — frame unchanged
.          : shifted one cell — ' 1  eedlexxxxxxxxx'…
>          : shifted to offset 11 — ' 1  xxxxxxxxxxxxxx'…
]          : half-width pan to offset 44 — ' 1  xxxxxxxxxxxxxx'…
w, w       : offset 44 retained through the toggle — ' 1  xxxxxxxxxxxxxx'…
n          : b.txt starts at offset 0 — '1  needle second '…
n, >       : offset 10 inside 文 — clipped cell is blank — '1   ccccccccccccc'…
n, >…      : offset clamped at 299 — 文 painted whole — '1  文             '…
.,>,]      : ., >, ] at the maximum — frame unchanged
ppp, ]     : back on a.txt — panned to offset 33 — ' 1  xxxxxxxxxxxxxx'…
down       : only pads visible — offset re-clamped to 2 — ' 2  d'
up         : long line back — offset stays 2 — ' 1  edlexxxxxxxxxx'…
q          : exit 0
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

All Issue #18 contracts verified: run-off-edge panning moves the horizontal window by one cell (`,`,`.`), ten cells (`<`,`>`), or half the text width (`[`,`]`), always clamped to the paintable boundary — the largest grapheme-cluster start among the *currently visible* rows — so the last cluster paints whole and nothing ever splits a glyph. The offset is a no-op and untouched in wrap mode, retained through wrap toggles, re-clamped whenever the visible set changes (pan, scroll, reveal, resize, wrap re-entry), and reset to zero on every file change; scrolling into short lines re-clamps leftward and the lost offset is never restored. Extent evaluation touches only visible rows — `Extent` carries `Key`/`At`, and `Line.MaxStart` computes the per-line boundary from the shared `safepresentation` cluster map. Issue #19 owns minimal horizontal reveal; Issue #20 owns the reserved indicator column's content; Issue #23 owns end-of-line marker extents.
