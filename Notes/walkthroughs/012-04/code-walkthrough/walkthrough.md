# Issue #12: manual vertical scrolling and per-file viewport

*2026-09-17T01:04:46Z by Showboat 0.6.1*
<!-- showboat-id: e07d6ddc-e340-436d-bbcf-5da238b93b7e -->

Issue #12 adds manual vertical scrolling to the browse frame: up/down move one rendered row, u/d a half page of max(1, floor(height/2)), and page up/page down a full page of the content height — the frame height minus the filename-rule row. The viewport clamps at both ends: top row never goes below 0 and never past max(0, rowCount - contentHeight), so EOF leaves no avoidable blank rows while a file shorter than the panel keeps its natural unused space. Scroll keys are strict no-ops while the panel shows a 'Loading…' or '(unreadable)' placeholder. Each file's vertical position is saved in the model keyed by raw path, so revisiting a file restores where you left off; the frame render reads the prepared row model only for the visible range. See Notes/issues/012-manual-vertical-scrolling-and-per-file-viewport.md, Notes/tasks/012-manual-vertical-scrolling-and-per-file-viewport.md, and the 'Navigation, viewport, and logical anchors' and 'Module Design' sections of Notes/PRD-vrg.md. All artifacts live in this directory.

## Gates — module integrity, build, vet

```bash
cd /home/chris/vrg && go mod verify && go build ./... && go vet ./... && echo GATES-OK
```

```output
all modules verified
GATES-OK
```

## Viewport — scroll units and clamps over prepared rows

internal/viewport/viewport_test.go covers the pure arithmetic: up/down move one rendered row, HalfPage is max(1, floor(h/2)) — 11 for a 23-row panel, and a full page is h itself; Scroll clamps top into [0, max(0, rowCount-h)] so a file shorter than the panel never scrolls, an exact fit stays pinned, and EOF stops with the last row at the bottom; Clamp pulls a stale top back after a shrink; Prepare turns a buffer into the Rows model the render path consults.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/viewport 2>&1 | grep -E '^(--- |    --- |ok|FAIL)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestScrollUnitsMoveRenderedRows
--- PASS: TestHalfPageUnit
--- PASS: TestScrollClampByFileLength
--- PASS: TestScrollClampBOF
--- PASS: TestClampPullsTopUp
--- PASS: TestPrepareRows
ok  	vrg/internal/viewport
```

## Model — keys, placeholders, saved state, render cost

internal/app/scroll_test.go covers the model contract: the six scroll keys move the current file's saved viewport by the expected units over a prepared row model; scrolling is a strict no-op on the 'Loading…' and '(unreadable)' placeholders; EOF and BOF clamp with no avoidable blanks while a short file leaves unused rows naturally; each file's viewport is saved under its raw path so scrolling one file never touches another and a revisit resumes the saved top; a counting rowSource fake proves a frame render queries only the visible row range; and a resize re-clamps every saved viewport.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'Scroll|ViewportState|RenderQueries|ResizeReclamp' ./internal/app 2>&1 | grep -E '^(--- |    --- |ok|FAIL)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestOverlayKeyRoutingAndScrolling
--- PASS: TestScrollKeysMoveRenderedRows
--- PASS: TestScrollStopsAtEOF
--- PASS: TestScrollStopsAtBOF
--- PASS: TestScrollShortFileLeavesUnusedRows
--- PASS: TestScrollKeysNoOpOnPlaceholders
--- PASS: TestViewportStateSavedPerFile
--- PASS: TestRenderQueriesOnlyVisibleRows
--- PASS: TestResizeReclampsViewport
ok  	vrg/internal/app
```

## Manual PTY — the issue's keys end to end

pty_scroll.py builds a fixture — a.txt with 'match top line' plus 'row 02'..'row 60' — and runs the real binary on a 100x24 pty (content height 23). Because the renderer emits a cell-level diff, the script replays the raw byte stream through a small screen emulator and asserts what the panel's first and last content rows actually display: down/up move one rendered row, d/u move 11, pgdn/pgup move 23, the second page-down clamps at top = 60 - 23 = 37 with line 60 on the bottom row, and further down/pgdn and up at BOF emit no repaint at all — the strict no-op — before q exits 0.

```bash
cd /home/chris/vrg && go build -o Notes/walkthroughs/012-04/code-walkthrough/vrg ./cmd/vrg && cd Notes/walkthroughs/012-04/code-walkthrough && python3 pty_scroll.py
```

```output
init    : top='1  match top line'    bottom='23  row 23'
down    : top='2  row 02'            bottom='24  row 24'
up      : top='1  match top line'    bottom='23  row 23'
up@BOF  : no repaint; top still '1  match top line' bottom still '23  row 23'
d       : top='12  row 12'           bottom='34  row 34'
u       : top='1  match top line'    bottom='23  row 23'
pgdn    : top='24  row 24'           bottom='46  row 46'
pgdn    : top='38  row 38'           bottom='60  row 60'
pgdn@EOF: no repaint; top still '38  row 38'        bottom still '60  row 60'
down@EOF: no repaint; top still '38  row 38'        bottom still '60  row 60'
pgup    : top='15  row 15'           bottom='37  row 37'
q       : exit 0
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
