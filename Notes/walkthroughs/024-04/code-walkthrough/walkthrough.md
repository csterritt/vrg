# Issue #24: file-list layout — width formula, … truncation, hide/show

*2026-09-17T22:11:29Z by Showboat 0.6.1*
<!-- showboat-id: 669e4fd8-ce04-4ab3-9050-145872aa710f -->

Issue #24 makes the file list responsive: its width is the nonnegative minimum of the longest sanitized path width plus two, floor(0.40 × terminal width), and terminal width minus (gutter width + 10 + reserved indicator width) — minimum text width outranking the 40% cap, and the clamp keeping pathological sizes at zero rather than negative. The longest-path term is prepared once per index so frames never rescan; every text-width change — hide/show, gutter growth on load, mode or size changes — routes through the Issue #17 prepared-layout path and preserves the logical reading anchor. left/tab hide the list and right/shift+tab show it, with the list initially shown; a computed zero width draws no cells but never clears the visibility preference. Over-wide paths left-truncate with a leading … on whole grapheme clusters, the list auto-scrolls to keep the active entry visible, and the filename rule gains a buffer-status note slot the path truncates to make room for — synthetic until Issues #26, #29, and #30 supply the real notes. See Notes/issues/024-file-list-layout-width-truncation-toggle.md, Notes/tasks/024-file-list-layout-width-truncation-toggle.md, and the PRD sections 'File list and layout' (stories 24–30) and 'Layout and indicators' in Notes/PRD-vrg.md. All artifacts live in this directory.

## Gates — module integrity, build, vet

```bash
cd /home/chris/vrg && go mod verify && go build ./... && go vet ./... && echo GATES-OK
```

```output
all modules verified
GATES-OK
```

## Layout-function tests — the width formula, truncation, and the note slot

internal/app/filelist_test.go pins the layout contracts. TestFileListWidthFormula drives each of the three terms winning in turn — longest+2, the floor(0.40 × w) cap (with floor rounding at an odd width), and terminal minus (gutter + 10 + reserved indicator) — plus the ten-cell text reservation outranking the 40% cap, gutter growth narrowing the list, and the nonnegative clamp. TestLeftTruncateGraphemeSafe and TestListEntriesLeftTruncate prove the leading-… tail truncation never splits a grapheme — wide glyphs, combining marks, and flags — in the helper and in rendered list cells. TestFilenameRuleStatusSlot and TestFilenameRuleNoNote cover the status-note slot: a synthetic note sits inside the rule with the path truncated around it, the note itself clips under a tiny width, and the note-free rule is unchanged. TestZeroWidthListRetainsPreference proves a computed zero width draws no cells while keeping the user's preference — the list returns when the terminal widens.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestFileListWidthFormula|TestLeftTruncateGraphemeSafe|TestListEntriesLeftTruncate|TestFilenameRuleStatusSlot|TestFilenameRuleNoNote|TestZeroWidthListRetainsPreference' ./internal/app 2>&1 | grep -E '^(--- |ok|FAIL|    ---)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestFileListWidthFormula
--- PASS: TestZeroWidthListRetainsPreference
--- PASS: TestLeftTruncateGraphemeSafe
--- PASS: TestListEntriesLeftTruncate
--- PASS: TestFilenameRuleStatusSlot
--- PASS: TestFilenameRuleNoNote
ok  	vrg/internal/app
```

## Toggle, auto-scroll, anchor, and render-cost tests

The same file drives the interactive contracts through Update/View. TestListHideShowToggles sends left/tab to hide and right/shift+tab to show — each issuing a re-keyed layout whose install reclaims the panel width — and TestListToggleIdempotent proves a repeated key changes nothing. TestListAutoScrollsToActiveEntry crosses files with n until the active entry would leave the window and the list scrolls to keep it visible. The anchor-through-relayout pair is the heart of the issue: TestAnchorSurvivesListToggle scrolls partway into a wrapped line, presses tab then shift+tab, and the top row holds the anchor's row — the same text location — each time; TestAnchorSurvivesGutterGrowth reloads a file that grew to five-digit line numbers, watches the gutter widen and the list narrow, and finds the anchor's row still at the top. TestHiddenListEscapesNoEntries is the render-cost guard: a hidden list escapes zero entries and a visible one escapes only its visible window's.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestListHideShowToggles|TestListToggleIdempotent|TestListAutoScrollsToActiveEntry|TestAnchorSurvivesListToggle|TestAnchorSurvivesGutterGrowth|TestHiddenListEscapesNoEntries' ./internal/app 2>&1 | grep -E '^(--- |ok|FAIL|    ---)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestListHideShowToggles
--- PASS: TestListToggleIdempotent
--- PASS: TestListAutoScrollsToActiveEntry
--- PASS: TestAnchorSurvivesListToggle
--- PASS: TestAnchorSurvivesGutterGrowth
--- PASS: TestHiddenListEscapesNoEntries
ok  	vrg/internal/app
```

## Full module regression

The new width feeds every layout consumer — text width, gutter, pan clamping, indicators, reveal — so the whole suite runs.

```bash
cd /home/chris/vrg && go test -count=1 ./internal/filebuffer ./internal/viewport ./internal/app ./internal/safepresentation ./internal/searchindex ./internal/theme ./internal/cli ./cmd/vrg 2>&1 | sed -E 's/\t[0-9.]+s$//'
```

```output
ok  	vrg/internal/filebuffer
ok  	vrg/internal/viewport
ok  	vrg/internal/app
ok  	vrg/internal/safepresentation
ok  	vrg/internal/searchindex
ok  	vrg/internal/theme
ok  	vrg/internal/cli
ok  	vrg/cmd/vrg
```

## Manual check — the real binary on a pty

pty_layout.py runs the built vrg on a real pty (80x24, resized mid-session) over a fixture directory of ~100-cell paths, replaying the byte stream through a small terminal emulator. At 80 columns the list sits at the floor(0.40×80)=32 cap with …-led basenames; five down-scrolls land MIDLINE-MARKER at the top of alpha's wrapped 500-cell line — a mid-line anchor — and tab hides the list while the same text stays at the top, shift+tab restoring the byte-identical row. n crosses to the 12,001-line beta: its seven-cell gutter widens but the 40% cap still binds at 80 columns — "narrows if needed" — so the width holds at 32. Shrinking to 30 columns leaves a constrained but present 12-cell list; at 26 the third term binds — beta's five-digit gutter narrows the list to 9 while alpha's four-cell gutter widens it to 10 — and enlarging returns the 32-cell cap.

```bash
cd /home/chris/vrg && go build -o Notes/walkthroughs/024-04/code-walkthrough/vrg ./cmd/vrg && python3 Notes/walkthroughs/024-04/code-walkthrough/pty_layout.py
```

```output
80x24     : 80 cols — list at the 32-cell cap, '…'-led basenames — '…hat-is-quite-long-aaa/alpha.txt' | panel ' 1  needle alpha o'
down x5   : scrolled into the wrapped line — top row '    xxxxxxxxxxxxxxxxxxxx'
tab       : list hidden, same text at top — '    xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxMIDL'
shift+tab : list restored, top row byte-identical
n         : on five-digit beta — list still 32 at 80 cols (cap binds); gutter now 7 — '    1  needle be'
30 cols   : list constrained but present — 12 cells — '…a/alpha.txt' | '    1  needle '
26 cols   : on beta the list narrows to 9 — the five-digit gutter's third term binds — '…lpha.txt'
p         : back on alpha the list is 10 — the smaller gutter widens it
n         : five-digit beta narrows the list to 9 again
80 cols   : enlarged — list back at the 32-cell cap — '…hat-is-quite-long-aaa/alpha.txt'
q         : exit 0
OK
```
