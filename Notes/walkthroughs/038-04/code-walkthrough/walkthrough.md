# Issue #38: Viewport content-panel text width

*2026-09-25T00:26:30Z by Showboat 0.6.1*
<!-- showboat-id: 143024eb-77b2-45cb-937e-1aa012a793ce -->

Walkthrough for [Issue #38](../../../tasks/038-viewport-content-panel-width.md), pinning the viewport's horizontal geometry to the content panel's usable text width per `Notes/PRD-vrg.md` (*File list and layout*, *Layout and indicators*, and *Navigation, viewport, and logical anchors*). The chain is computed once in `syncLayout`: the panel width is the terminal width minus the file list's allocated cells, and the text width — the width `viewport.Key` carries into `Prepare`, `Resize`, and every install site — is the panel width minus the buffer gutter minus the mode's reserved right-indicator column (one cell run-off-edge, zero in wrap). The audit found every install site already routed through that single computation; the work hardened the tests so each expectation recomputes the width chain independently, and added list-hidden, wrap-mode, resize, and composed-view coverage. All generated artifacts live in this directory: the built `vrg` binary, the `demo-panel-width.sh` tmux harness, and its `manual-panel-width/` captures. Test durations are stripped so the document verifies cleanly.

```bash
cd /home/chris/vrg && go vet ./... && go build ./... && go test -count=1 ./... | sed 's/[[:space:]][0-9.]*s$//' && echo GATES-OK
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

## Corrected horizontal-reveal tests

`internal/app/reveal_horizontal_test.go` (renamed from `hreveal_test.go`) no longer expects positions from the cached `m.textW`. The `wantTextW()` helper recomputes the expected width from the layout chain — terminal width minus the `listWidth` allocation, the buffer gutter, and the reserved indicator column — so a regression installing the viewport at a terminal-derived width fails. New coverage: `TestHRevealListHiddenRemeasuresPanelWidth` (list hidden → wider panel), `TestHRevealWrapModeReservesNoIndicatorColumn` (wrap's zero reservation in the installed key and full-width rows), `TestHRevealResizeRemeasuresTextWidth` (re-measure on terminal resize), and `TestComposedViewContentStaysInsidePanel` (no rendered row exceeds the terminal width; panned content stays inside the panel; the reserved `*` sits at the panel's right edge). `pan_test.go`, `layout_test.go`'s `wantKey`, and `completion_test.go` consume the same helper. Both omitted-term regressions — dropping `m.listW` and dropping `res` — were exercised during development and every new test failed as intended.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestHReveal|TestComposedView|TestPanKeysMoveOffset|TestListToggleKeys|TestLayoutKey' ./internal/app 2>&1 | grep -vE '^(=== RUN|    --- (PASS|FAIL))' | sed -E 's/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//'
```

```output
--- PASS: TestListToggleKeys
--- PASS: TestListToggleKeysInertWhileSearching
--- PASS: TestPanKeysMoveOffset
--- PASS: TestHRevealSameFileNScrollsRightMinimal
--- PASS: TestHRevealPBackScrollsLeftToTarget
--- PASS: TestHRevealVisibleMatchKeepsOffset
--- PASS: TestHRevealAppliesAtStartup
--- PASS: TestHRevealAfterFileChangeReset
--- PASS: TestHRevealListHiddenRemeasuresPanelWidth
--- PASS: TestHRevealWrapModeReservesNoIndicatorColumn
--- PASS: TestHRevealResizeRemeasuresTextWidth
--- PASS: TestComposedViewContentStaysInsidePanel
--- PASS: TestHRevealMidClusterMatchPaintsWholeCluster
--- PASS: TestHRevealWideClusterMatchPaintsBothCells
PASS
ok  	vrg/internal/app
```

## One width at every install site

The audit confirmed every viewport sizing and install path shares the single `syncLayout` computation — no site re-derives a width from the raw terminal width: `syncLayout` computes `panelW = width − listW` then `textW = panelW − gutter − reserved` and calls `vp.Resize(m.textW, …)`; `layoutKey` carries `m.textW` as `viewport.Key.Width`; `layoutDoneMsg` and `currentRows` install only on a matching key; `ensureLayout`/`layoutCmd` prepare rows at that key's width.

```bash
cd /home/chris/vrg && grep -n 'panelW :=\|m.textW =\|vp.Resize\|Width: m.textW\|inst.key == m.layoutKey\|key != m.layoutKey\|viewport.Prepare' internal/app/browse.go internal/app/app.go
```

```output
internal/app/browse.go:417:	panelW := max(0, m.width-m.listW)
internal/app/browse.go:418:	m.textW = max(0, panelW-gutterW-res)
internal/app/browse.go:420:	m.vp.Resize(m.textW, max(0, m.height-1))
internal/app/browse.go:428:	return viewport.Key{Path: path, Rev: m.revs[path], Width: m.textW, Wrap: m.wrap}
internal/app/browse.go:440:	if inst, ok := m.rows[string(cur)]; ok && inst.key == m.layoutKey(string(cur)) {
internal/app/browse.go:473:		return layoutDoneMsg{key: key, rows: viewport.Prepare(buf, key)}
internal/app/app.go:395:		if key != m.layoutKey(key.Path) {
```

## Manual check — pan, indicators, and list toggle against the panel boundary

Build the binary into this directory, then run `demo-panel-width.sh` (checked in here). The harness drives the built `vrg` on a real tmux PTY at 80x24 against a fixture whose `fakebin/rg` emits two `needle` matches on one 200-cell line of `long.txt`: cells 140–145 and 176–181. With the list shown, `listWidth` allocates 10 cells (`longest + 2`), so the panel is 70 and the run-off-edge text width is 66.

```bash
cd /home/chris/vrg && go build -o Notes/walkthroughs/038-04/code-walkthrough/vrg ./cmd/vrg && echo built
```

```output
built
```

The scenario per the task: after `w` leaves wrap mode the first needle is hidden right, so the reserved column carries `*`; eight `>` presses pan right ten cells each to offset 80, revealing the match on the last text cell while the second needle stays hidden right. The harness asserts the Issue #38 boundary contract at every stage — no rendered row exceeds 80 cells, no content paints into the file list's 10 columns while it is shown, and the `*` indicator always sits at the panel's right edge outside the text area. `left` hides the list: the panel becomes the full 80 cells, the text width re-measures to 76, and the same pan and indicators track the new geometry. `right` restores the list; `q` exits 0.

```bash
./demo-panel-width.sh
```

```output
exit=0
--- panned right, list shown (text width 66) ---
long.txt  ── long.txt ──────────────────────────────────────────────────────────
          1_ xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxneedle*
          2_
--- list hidden (text width 76) ---
── long.txt ────────────────────────────────────────────────────────────────────
1_ xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxneedleyyyyyyyyyy*
2_
--- list hidden, panned once more ---
── long.txt ────────────────────────────────────────────────────────────────────
1_ xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxneedleyyyyyyyyyyyyyyyyyyyy*
2_
--- list shown again (text width 66) ---
long.txt  ── long.txt ──────────────────────────────────────────────────────────
          1_ xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxneedleyyyyyyyyyy*
          2_
```

## Result

Issue #38 is verified. The captures show the contract directly: with the list shown, content occupies only the 70-cell panel (columns 11–80 — the first ten columns stay the list's), the gutter `_` signposts the hidden-left text, and the reserved `*` sits at cell 80 outside the 66-cell text area; hiding the list re-measures the text width to 76 and pans still land inside the widened panel; re-showing restores the 66-cell geometry with the same indicators. `go build`, `go vet`, and `go test ./...` all pass, and the hardened suite independently recomputes every expected position from the terminal→panel→text width chain, so any viewport install site that drifted back to a terminal-derived width — or dropped the gutter or reserved terms — fails.

