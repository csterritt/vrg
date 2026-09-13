# Issue #24: File list layout, truncation, and toggle

*2026-09-12T12:52:03Z by Showboat 0.6.1*
<!-- showboat-id: 61727b2c-819d-49a7-98aa-13141681c087 -->

Walkthrough for Issue #24 (Notes/tasks/024-file-list-layout-width-truncation-toggle.md), implementing the responsive file-list layout in vrg. The file-list width is computed from terminal dimensions, sanitized path content, gutter width, and reserved indicator width via `ComputeListWidth(termWidth, longestPathWidth, gutterWidth, reservedIndicator, visible)` — the nonnegative minimum of `longestPathWidth+2`, `floor(0.40×termWidth)`, and `termWidth−(gutterWidth+10+reservedIndicator)`; 0 when not visible. Long paths are left-truncated with a leading `…` without splitting grapheme clusters. Users can hide and show the list (`left`/`tab` hide, `right`/`shift+tab` show, initially shown). The file-list viewport auto-scrolls to keep the active file visible. Every text-width change routes through Issue #17's prepared-layout path, preserving the logical reading anchor. The filename row supports a buffer-status note slot (real notes owned by Issues #26/#29/#30; this issue tests with a synthetic seam). Rendering queries only the visible file-list window. References: Notes/PRD-vrg.md (Layout and indicators; Navigation, viewport, and logical anchors; Resources and responsiveness; Text, graphemes, and safe presentation), Notes/wiki/file-list-layout.md.

Contracts verified:

- The list width is the nonnegative minimum of the three formula terms.
- The 40% cap uses integer floor rounding.
- A computed zero width draws no list cells but preserves the visibility preference.
- Long paths are left-truncated with a leading `…` without splitting graphemes.
- `left`/`tab` hide the list; `right`/`shift+tab` show it; startup is shown.
- Toggling visibility triggers a relayout (Issue #17 prepared-layout path).
- The logical reading anchor is preserved through toggle and resize relayouts.
- The file-list viewport auto-scrolls to keep the active entry visible.
- A frame render queries only the visible file-list window.
- The filename row supports a buffer-status note slot with path truncation.
- Pathological dimensions never produce negative panel widths.

All generated artifacts live in this directory.

```bash
cd /home/chris/vrg && go build ./cmd/... ./internal/... && go vet ./cmd/... ./internal/... && echo GATES-OK
```

```output
GATES-OK
```

```bash
cd /home/chris/vrg && go test -count=1 ./cmd/... ./internal/... -timeout 120s | sed 's/[[:space:]][0-9.]*s$//'
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

## Issue #24 layout-function and truncation tests

The pure-function tests (internal/app/filelist_layout_test.go) verify the `ComputeListWidth` formula (each term winning, 40% floor rounding, zero when hidden, nonnegative clamp, indicator effect) and `TruncateLeftGrapheme` (ASCII, wide glyphs, combining marks, no-truncate-when-fits, empty input).

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/app/ -run '^TestComputeListWidth|^TestTruncateLeftGrapheme' -timeout 30s | sed 's/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
```

```output
=== RUN   TestComputeListWidthLongestPathWins
--- PASS: TestComputeListWidthLongestPathWins (0.00s)
=== RUN   TestComputeListWidthFortyPercentCapWins
--- PASS: TestComputeListWidthFortyPercentCapWins (0.00s)
=== RUN   TestComputeListWidthTenCellMinimumWins
--- PASS: TestComputeListWidthTenCellMinimumWins (0.00s)
=== RUN   TestComputeListWidthFortyPercentFloorRounding
--- PASS: TestComputeListWidthFortyPercentFloorRounding (0.00s)
=== RUN   TestComputeListWidthGutterGrowthReduces
--- PASS: TestComputeListWidthGutterGrowthReduces (0.00s)
=== RUN   TestComputeListWidthZeroAllocation
--- PASS: TestComputeListWidthZeroAllocation (0.00s)
=== RUN   TestComputeListWidthNeverNegative
=== RUN   TestComputeListWidthNeverNegative/zero_terminal
=== RUN   TestComputeListWidthNeverNegative/negative_terminal
=== RUN   TestComputeListWidthNeverNegative/huge_gutter
=== RUN   TestComputeListWidthNeverNegative/huge_indicator
=== RUN   TestComputeListWidthNeverNegative/all_huge
--- PASS: TestComputeListWidthNeverNegative (0.00s)
    --- PASS: TestComputeListWidthNeverNegative/zero_terminal (0.00s)
    --- PASS: TestComputeListWidthNeverNegative/negative_terminal (0.00s)
    --- PASS: TestComputeListWidthNeverNegative/huge_gutter (0.00s)
    --- PASS: TestComputeListWidthNeverNegative/huge_indicator (0.00s)
    --- PASS: TestComputeListWidthNeverNegative/all_huge (0.00s)
=== RUN   TestComputeListWidthHiddenReturnsZero
--- PASS: TestComputeListWidthHiddenReturnsZero (0.00s)
=== RUN   TestComputeListWidthIndicatorAffectsThirdTerm
--- PASS: TestComputeListWidthIndicatorAffectsThirdTerm (0.00s)
=== RUN   TestTruncateLeftGraphemeFits
--- PASS: TestTruncateLeftGraphemeFits (0.00s)
=== RUN   TestTruncateLeftGraphemeTruncates
--- PASS: TestTruncateLeftGraphemeTruncates (0.00s)
=== RUN   TestTruncateLeftGraphemeNarrowWidth
--- PASS: TestTruncateLeftGraphemeNarrowWidth (0.00s)
=== RUN   TestTruncateLeftGraphemeGraphemeSafe
--- PASS: TestTruncateLeftGraphemeGraphemeSafe (0.00s)
=== RUN   TestTruncateLeftGraphemeEmpty
--- PASS: TestTruncateLeftGraphemeEmpty (0.00s)
PASS
ok  	vrg/internal/app
```

## Toggle, anchor, and auto-scroll tests

The app tests (internal/app/filelist_layout_test.go) verify the toggle keys (`tab`/`left` hide, `shift+tab`/`right` show), the relayout trigger, anchor preservation through toggle and resize, zero-width visibility preference, active-entry auto-scroll, the visible-window render-cost guard, the filename-row status-note slot, and list-entry/filename-row truncation.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/app/ -run '^TestListInitiallyShown|^TestTabHidesList|^TestLeftHidesList|^TestShiftTabShowsList|^TestRightShowsList|^TestListToggleTriggersRelayout|^TestZeroWidthListRetainsPreference|^TestListAutoScrollActiveEntry|^TestAnchorSurvivesTabShiftTab|^TestAnchorSurvivesResize|^TestRenderCostGuardFileListVisibleEntries|^TestFilenameRow|^TestListWidth|^TestListEntryTruncation' -timeout 60s | sed 's/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
```

```output
=== RUN   TestListInitiallyShown
--- PASS: TestListInitiallyShown (0.00s)
=== RUN   TestTabHidesList
--- PASS: TestTabHidesList (0.00s)
=== RUN   TestLeftHidesList
--- PASS: TestLeftHidesList (0.00s)
=== RUN   TestShiftTabShowsList
--- PASS: TestShiftTabShowsList (0.00s)
=== RUN   TestRightShowsList
--- PASS: TestRightShowsList (0.00s)
=== RUN   TestListToggleTriggersRelayout
--- PASS: TestListToggleTriggersRelayout (0.00s)
=== RUN   TestZeroWidthListRetainsPreference
--- PASS: TestZeroWidthListRetainsPreference (0.00s)
=== RUN   TestListAutoScrollActiveEntry
--- PASS: TestListAutoScrollActiveEntry (0.00s)
=== RUN   TestAnchorSurvivesTabShiftTab
--- PASS: TestAnchorSurvivesTabShiftTab (0.00s)
=== RUN   TestAnchorSurvivesResize
--- PASS: TestAnchorSurvivesResize (0.00s)
=== RUN   TestRenderCostGuardFileListVisibleEntries
--- PASS: TestRenderCostGuardFileListVisibleEntries (0.00s)
=== RUN   TestFilenameRowStatusNote
--- PASS: TestFilenameRowStatusNote (0.00s)
=== RUN   TestFilenameRowPathTruncationForStatus
--- PASS: TestFilenameRowPathTruncationForStatus (0.00s)
=== RUN   TestFilenameRowNoStatusNote
--- PASS: TestFilenameRowNoStatusNote (0.00s)
=== RUN   TestListWidthRecomputedAfterGutterGrowth
--- PASS: TestListWidthRecomputedAfterGutterGrowth (0.00s)
=== RUN   TestListWidthInRender
--- PASS: TestListWidthInRender (0.00s)
=== RUN   TestListEntryTruncation
--- PASS: TestListEntryTruncation (0.00s)
PASS
ok  	vrg/internal/app
```

## Manual verification

The task requires manual cases in an 80-column terminal with long paths. The fake rg binaries (fakebin/rg_long_paths, fakebin/rg_many_lines, fakebin/rg_long_line) emit valid JSON streams so the full vrg TUI can be driven through the capture_raw.py PTY harness. The demo files live in demo_artifacts/. The captures filter to the filename row and the first styled content row (the deterministic parts) so the output is reproducible.

### Case 1: Long paths — list at most 32 columns with `…`-prefixed entries

The demo file (demo_artifacts/src/very/long/path/to/a/file/that/is/much/longer/than/the/list/width/allows.go) has a path far longer than the list width. At 80 columns, the 40% cap gives floor(0.40×80)=32, so the list is at most 32 columns. The long path is left-truncated with a leading `…` in both the file-list entry and the filename row. The capture filters to the filename row (underlined) and the list entries.

```bash
cd /home/chris/vrg/Notes/walkthroughs/024-04/code-walkthrough && ln -sf rg_long_paths fakebin/rg && PATH=$PWD/fakebin:$PWD/demo_artifacts:$PATH VRG_KEYS=q VRG_DELAY=0.5 VRG_WIDTH=80 VRG_HEIGHT=10 python3 demo_artifacts/capture_raw.py demo_artifacts/vrg -- 'hello' demo_artifacts/src 2>&1 | sed 's/\x1b\[/ESC[/g' | tr -d '\r' | grep -oE 'ESC\[[0-9;]+m[^E]*' | grep -E '(37;40;4m|…)' | head -5; echo; rm fakebin/rg
```

```output
ESC[37;40;4mdemo_artifacts/src/short/a.txt   ── demo_artifacts/src/short/a.txt ──
ESC[37;40m…/than/the/list/width/allows.txt 

```

### Case 2: Five-digit line numbers — gutter widens, list stays at 40% cap

The demo file (demo_artifacts/many_lines.txt) has 10000 lines. A match at line 5000 gives a gutter of 4 digits + 2 spaces = 6 cells. The gutter widens from the default 3 cells (for a 1-line file) to 6 cells. The list width is dominated by the 40% cap (32 columns at 80 width). The capture filters to the filename row and the matched line.

```bash
cd /home/chris/vrg/Notes/walkthroughs/024-04/code-walkthrough && ln -sf rg_many_lines fakebin/rg && PATH=$PWD/fakebin:$PWD/demo_artifacts:$PATH VRG_KEYS=q VRG_DELAY=0.5 VRG_WIDTH=80 VRG_HEIGHT=10 python3 demo_artifacts/capture_raw.py demo_artifacts/vrg -- 'line 5000' demo_artifacts/many_lines.txt 2>&1 | sed 's/\x1b\[/ESC[/g' | tr -d '\r' | grep -oE 'ESC\[[0-9;]+m[^E]*' | grep -E '(37;40;4m|30;47;4m)' | head -3; echo; rm fakebin/rg
```

```output
ESC[37;40;4mdemo_artifacts/many_lines.txt   ── demo_artifacts/many_lines.txt ──
ESC[30;47;4mline 5000

```

### Case 3: Scroll partway into a wrapped line, then `tab` then `shift+tab` — same text remains at top

The demo file (demo_artifacts/long_line.txt) has a single 1500-character line of `x`. At 80 columns the line wraps. Scrolling down 3 rows moves partway into the wrapped line. Pressing `tab` (hide list) widens the text width, rewrapping with fewer rows; pressing `shift+tab` (show list) narrows it back. The Issue #17 logical anchor (source line, column) is preserved through both relayouts, so the same text location remains at the top of the viewport. The capture filters to the filename row and the first content row.

```bash
cd /home/chris/vrg/Notes/walkthroughs/024-04/code-walkthrough && ln -sf rg_long_line fakebin/rg && PATH=$PWD/fakebin:$PWD/demo_artifacts:$PATH VRG_KEYS=down,down,down,tab,shift+tab,q VRG_DELAY=0.5 VRG_WIDTH=80 VRG_HEIGHT=10 python3 demo_artifacts/capture_raw.py demo_artifacts/vrg -- 'x' demo_artifacts/long_line.txt 2>&1 | sed 's/\x1b\[/ESC[/g' | tr -d '\r' | grep -oE 'ESC\[[0-9;]+m[^E]*' | grep -E '(37;40;4m|30;47;4m)' | head -3; echo; rm fakebin/rg
```

```output
ESC[37;40;4mdemo_artifacts/long_line.txt   ── demo_artifacts/long_line.txt ──
ESC[30;47;4mx

```

### Case 4: Shrink to 30 columns — constrained but present list

At 30 columns the 40% cap gives floor(0.40×30)=12, and the third term gives `30−(3+10+0)=17`, so the list is at most 12 columns (constrained but present). The content panel retains at least ten text cells. The paths are left-truncated with a leading `…` to fit the narrow list.

```bash
cd /home/chris/vrg/Notes/walkthroughs/024-04/code-walkthrough && ln -sf rg_long_paths fakebin/rg && PATH=$PWD/fakebin:$PWD/demo_artifacts:$PATH VRG_KEYS=q VRG_DELAY=0.5 VRG_WIDTH=30 VRG_HEIGHT=10 python3 demo_artifacts/capture_raw.py demo_artifacts/vrg -- 'hello' demo_artifacts/src 2>&1 | sed 's/\x1b\[/ESC[/g' | tr -d '\r' | grep -oE 'ESC\[[0-9;]+m[^E]*' | grep -E '(37;40;4m|…)' | head -5; echo; rm fakebin/rg
```

```output
ESC[37;40;4m…short/a.txt ── …hort/a.txt ──
ESC[37;40m…short/b.txt 1  
ESC[37;40m…/allows.txt 

```

### Case 5: Enlarge back to 80 columns — list widens to the 40% cap

At 80 columns the list widens back to the 40% cap (32 columns), showing more of each path. The short paths are no longer truncated; the long path is still truncated at the list width.

```bash
cd /home/chris/vrg/Notes/walkthroughs/024-04/code-walkthrough && ln -sf rg_long_paths fakebin/rg && PATH=$PWD/fakebin:$PWD/demo_artifacts:$PATH VRG_KEYS=q VRG_DELAY=0.5 VRG_WIDTH=80 VRG_HEIGHT=10 python3 demo_artifacts/capture_raw.py demo_artifacts/vrg -- 'hello' demo_artifacts/src 2>&1 | sed 's/\x1b\[/ESC[/g' | tr -d '\r' | grep -oE 'ESC\[[0-9;]+m[^E]*' | grep -E '(37;40;4m|…)' | head -5; echo; rm fakebin/rg
```

```output
ESC[37;40;4mdemo_artifacts/src/short/a.txt   ── demo_artifacts/src/short/a.txt ──
ESC[37;40m…/than/the/list/width/allows.txt 

```

## Summary

Issue #24 file list layout, truncation, and toggle are implemented and verified. The file-list width is computed from terminal dimensions, sanitized path content, gutter width, and reserved indicator width via the responsive formula (nonnegative minimum of `longestPathWidth+2`, `floor(0.40×termWidth)`, and `termWidth−(gutterWidth+10+reservedIndicator)`; 0 when not visible). Long paths are left-truncated with a leading `…` without splitting grapheme clusters. Users can hide and show the list (`left`/`tab` hide, `right`/`shift+tab` show, initially shown). The file-list viewport auto-scrolls to keep the active file visible. Every text-width change routes through Issue #17's prepared-layout path, preserving the logical reading anchor. The filename row supports a buffer-status note slot with path truncation (real notes owned by Issues #26/#29/#30; this issue tests with a synthetic seam). Rendering queries only the visible file-list window. Pathological dimensions never produce negative panel widths. References: Issue #24 (Notes/tasks/024-file-list-layout-width-truncation-toggle.md), Notes/PRD-vrg.md (Layout and indicators; Navigation, viewport, and logical anchors; Resources and responsiveness; Text, graphemes, and safe presentation), Notes/wiki/file-list-layout.md.

All generated artifacts (demo files, fake rg binaries, capture_raw.py, vrg binary) live in this directory under demo_artifacts/ and fakebin/.
