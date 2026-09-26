# Issue #12: Manual vertical scrolling and per-file viewport

*2026-09-23T21:29:18Z by Showboat 0.6.1*
<!-- showboat-id: 8df06f15-231a-494d-aae8-373b70dccacf -->

Walkthrough for [Issue #12](../../../issues/012-manual-vertical-scrolling-and-per-file-viewport.md), implementing manual vertical scrolling and per-file saved viewport state per `Notes/PRD-vrg.md` (*Navigation, viewport, and logical anchors* — the scroll-unit bullet and clamping rules; *Module Design → Viewport*). `up`/`down` scroll one rendered row, `u`/`d` move `max(1, floor(h/2))`, and `pgup`/`pgdown` move a full page where the content height is the panel height minus the filename row; the top row clamps to `[0, max(0, count − height)]` so EOF leaves no avoidable blank rows and short files pin to the top; scroll keys on a `Loading…` placeholder are no-ops; each file's top row is saved per path and restored when its load completes; and frame rendering slices prepared row data, querying the row provider only for the visible range. All generated artifacts live in this directory: the built `vrg` binary, the `demo-scroll.sh` harness, and its `manual-scroll/` tmux session captures. Test durations are stripped so the document verifies cleanly.

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

## Viewport unit and clamp tests

`internal/viewport/viewport_test.go` pins the scroll-unit and clamping contracts against the `countingRows` provider fake. `TestScrollUnits` covers each unit — one rendered row, `max(1, floor(h/2))` for even and odd heights and the height-1 floor, and the full content-height page; `TestScrollSequence` accumulates a mixed sequence. `TestClampAtBOF` and `TestClampAtEOF` prove both clamps — the latter ending with the file's last row on the bottom row — while `TestFileShorterThanViewport`, `TestFileEqualToViewport`, and `TestEmptyContentIsInert` cover content shorter than, equal to, and absent from the viewport. `TestVisibleQueriesOnlyVisibleRows` and `TestVisibleNearEOFQueriesRemainder` are the unit-level render-cost guard, and `TestResizeReclampsTop`/`TestSetRowsClampsToNewContent` pin the lossy EOF clamp on layout and content changes.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/viewport 2>&1 | grep -vE '^(=== RUN|    --- (PASS|FAIL))' | sed -E 's/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//'
```

```output
--- PASS: TestScrollUnits
--- PASS: TestScrollSequence
--- PASS: TestClampAtBOF
--- PASS: TestClampAtEOF
--- PASS: TestFileShorterThanViewport
--- PASS: TestFileEqualToViewport
--- PASS: TestEmptyContentIsInert
--- PASS: TestVisibleQueriesOnlyVisibleRows
--- PASS: TestVisibleNearEOFQueriesRemainder
--- PASS: TestResizeReclampsTop
--- PASS: TestSetRowsClampsToNewContent
PASS
ok  	vrg/internal/viewport
```

## App-level scroll, placeholder, per-file state, and render-cost tests

`internal/app/scroll_test.go` drives the keys through `Update`. `TestDownUpMoveOneRenderedRow` shows the frame shifting one rendered row while the matched-line cursor stays put; `TestHalfPageScrollUsesContentHeight` and `TestPageScrollUsesContentHeight` prove the units are computed from the content height — terminal height minus the filename row — across even, odd, and one-row content heights. `TestScrollClampsAtEOFAndBOF` leaves the view byte-identical on `up` at the top and clamps `pgdown` at the last full page. `TestScrollOnShortFileIsNoOp` and `TestScrollOnPlaceholderIsNoOp` cover the short-file and `Loading…` no-ops, the latter asserting no command, no top movement, and no saved-state write. `TestPerFileSavedViewportState` scrolls one file, then completes another file's load with seeded saved state — the new panel starts at the saved top while the first file's entry is untouched. `TestRenderQueriesOnlyVisibleRows` is the render-cost guard: a `countingRows` fake installed as the prepared rows records exactly the 23 visible row indices per `View()`, before and after a scroll — never O(N) over the buffer. `TestBufferRowsAdaptsBuffer` pins the unwrapped row-to-line adapter.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestDownUpMoveOneRenderedRow|TestHalfPageScrollUsesContentHeight|TestPageScrollUsesContentHeight|TestScrollClampsAtEOFAndBOF|TestScrollOnShortFileIsNoOp|TestScrollOnPlaceholderIsNoOp|TestPerFileSavedViewportState|TestRenderQueriesOnlyVisibleRows|TestBufferRowsAdaptsBuffer' ./internal/app 2>&1 | grep -vE '^(=== RUN|=== CONT|    --- (PASS|FAIL))' | sed -E 's/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//'
```

```output
--- PASS: TestDownUpMoveOneRenderedRow
--- PASS: TestHalfPageScrollUsesContentHeight
--- PASS: TestPageScrollUsesContentHeight
--- PASS: TestScrollClampsAtEOFAndBOF
--- PASS: TestScrollOnShortFileIsNoOp
--- PASS: TestScrollOnPlaceholderIsNoOp
--- PASS: TestPerFileSavedViewportState
--- PASS: TestRenderQueriesOnlyVisibleRows
--- PASS: TestBufferRowsAdaptsBuffer
PASS
ok  	vrg/internal/app
```

## Manual check — scrolling a long file on a real PTY

`demo-scroll.sh` (checked into this directory) runs the freshly built `vrg` on a real tmux PTY at 80x24 — content height 23 — in a fixture directory whose `fakebin/rg` reports one match in a 200-line `long.txt`. The script sends real keypresses and asserts the first and last content rows at each step: `Down`/`Up` move one rendered row, `d`/`u` move `floor(23/2)=11`, `NPage`/`PPage` move the full 23-row page, repeated `NPage` clamps at top row 178 with `line-200` on the bottom row and a further `pgdn` leaves the frame byte-identical, and `Up` back at the top changes nothing. Screen captures are stored under `manual-scroll/`.

```bash
cd /home/chris/vrg && go build -o Notes/walkthroughs/012-04/code-walkthrough/vrg ./cmd/vrg && cd Notes/walkthroughs/012-04/code-walkthrough && ./demo-scroll.sh
```

```output
ok: initial top row -> line-001
ok: Down moves one rendered row -> line-002
ok: Up moves one rendered row back -> line-001
ok: d moves half a page (floor(23/2)=11) -> line-012
ok: u moves half a page back -> line-001
ok: pgdn moves a full page (23 rows) -> line-024
ok: EOF clamp top -> line-178
ok: last file row at the bottom -> line-200
ok: further pgdn at EOF changes nothing
ok: pgup returns to the top -> line-001
ok: up at the top does nothing
exit=0
--- screen at EOF clamp ---
long.txt  ── long.txt ──────────────────────────────────────────────────────────
          178  line-178
          179  line-179
          180  line-180
          181  line-181
          182  line-182
          183  line-183
          184  line-184
          185  line-185
          186  line-186
          187  line-187
          188  line-188
          189  line-189
          190  line-190
          191  line-191
          192  line-192
          193  line-193
          194  line-194
          195  line-195
          196  line-196
          197  line-197
          198  line-198
          199  line-199
          200  line-200
```

## Verdict

Issue #12 is verified. `viewport.Viewport` owns the current file's prepared rows and clamped top rendered row: `Up`/`Down` move one row, `HalfUp`/`HalfDown` move `max(1, floor(h/2))`, and `PageUp`/`PageDown` move the content height — the panel height minus the filename row that `relayout` installs via `Resize(width, m.height-1)`. The clamp `[0, max(0, count − height)]` stops `up` at BOF, stops `down` with the last row on the bottom row at EOF, and pins short files to the top with their unused rows left naturally. Scroll keys are no-ops on the `Loading…` and `(unreadable)` placeholders and outside browse. Each scroll records the top row into `m.saved` keyed by raw path, and a load completing for the current path restores it via `SetTop` — the revisit seam Issue #13 will drive. Rendering slices prepared `viewport.Rows` data through `Visible()`, and the counting-fake tests prove a frame queries only the visible range. The tmux run shows every unit moving by the expected amount on a real PTY, EOF stopping with `line-200` at the bottom, and `up` at the top leaving a byte-identical frame.
