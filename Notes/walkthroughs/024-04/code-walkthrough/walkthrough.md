# Issue #24: File-list layout — width formula, truncation, and visibility toggle

*2026-09-24T15:07:56Z by Showboat 0.6.1*
<!-- showboat-id: 5b58d642-2589-4392-8142-39d320f6757f -->

Walkthrough for [Issue #24](../../../issues/024-file-list-layout-width-truncation-toggle.md), implementing the file-list layout contract per `Notes/PRD-vrg.md` (*File list and layout*; *Layout and indicators* — the width bullets): the visible list's width is the nonnegative minimum of longest sanitized path plus two, `floor(0.40 × terminal width)`, and terminal width minus the panel minimum (gutter + ten text cells + the reserved indicator column); `left`/`tab` hide and `right`/`shift+tab` show the list, and a zero-width allocation draws no cells without moving the visibility preference; long paths left-truncate with a leading `…` on grapheme boundaries; the filename rule reserves a buffer-status note slot (real notes arrive with Issues #26, #29, and #30); the list scrolls minimally to keep the active entry visible; and every width change routes through Issue #17's keyed prepared-layout path so the logical anchor's text stays at the top. All generated artifacts live in this directory: the built `vrg` binary, the `demo-list.sh` tmux harness, and its `list/` session captures. Test durations are stripped so the document verifies cleanly.

```bash
set -o pipefail; cd /home/chris/vrg && test -z "$(gofmt -l internal/)" && echo GOFMT-CLEAN && go vet ./... && go build ./... && go test -count=1 ./... | sed "s/[[:space:]][0-9.]*s$//" && CGO_ENABLED=1 go test -race -count=1 ./internal/app ./internal/viewport ./internal/filebuffer ./internal/present | sed "s/[[:space:]][0-9.]*s$//" && go build -o Notes/walkthroughs/024-04/code-walkthrough/vrg ./cmd/vrg && echo GATES-OK
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
ok  	vrg/internal/filebuffer
ok  	vrg/internal/present
GATES-OK
```

## Layout-function tests

`internal/app/filelist_test.go` pins the width contract at the layout seam: `TestListWidthFormula` table-drives `listWidth(gutterW, res)` through a constructed model — each of the three terms winning (longest sanitized path plus two, the `floor(0.40 × width)` cap including odd-width rounding, and `width − (gutter + 10 + reserved)`), gutter growth, the ten-cell panel minimum, and zero/pathological widths clamped nonnegative; `TestListReducedLeavesTenTextCells` drives a real five-digit-gutter file at 30 columns into run-off-edge mode for the issue's `min(longest+2, 12, 12)` example; `TestZeroWidthAllocationKeepsPreference` covers the `W=20`/gutter 9/reserved 1 zero-allocation case leaving `listShow` untouched; `TestTruncateLeftGraphemeSafe` pins the leading-`…` cut at grapheme boundaries — a wide cluster straddling the cut drops whole so the result never exceeds its budget — and `TestFilenameRuleStatusSlot` covers the status note winning cells over the path (which truncates to nothing so the note paints whole) and the too-wide note being dropped.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestListWidthFormula|TestTruncateLeftGraphemeSafe|TestFilenameRuleStatusSlot|TestListReducedLeavesTenTextCells|TestZeroWidthAllocationKeepsPreference' ./internal/app 2>&1 | grep -vE '^(=== RUN|=== CONT)' | sed -E 's/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//'
```

```output
--- PASS: TestListWidthFormula
    --- PASS: TestListWidthFormula/longest_plus_two_wins
    --- PASS: TestListWidthFormula/longest_plus_two_wins_at_cap_boundary
    --- PASS: TestListWidthFormula/forty_percent_cap_wins
    --- PASS: TestListWidthFormula/forty_percent_floor_rounds_down
    --- PASS: TestListWidthFormula/forty_percent_floor_rounds_down_odd
    --- PASS: TestListWidthFormula/forty_percent_exact
    --- PASS: TestListWidthFormula/panel_minimum_wins_wrap
    --- PASS: TestListWidthFormula/panel_minimum_wins_run-off-edge
    --- PASS: TestListWidthFormula/panel_minimum_leaves_ten_text_cells
    --- PASS: TestListWidthFormula/gutter_growth_narrows
    --- PASS: TestListWidthFormula/gutter_grown_narrows_further
    --- PASS: TestListWidthFormula/reserved_column_counts
    --- PASS: TestListWidthFormula/zero_width_allocation
    --- PASS: TestListWidthFormula/pathological_stays_nonnegative
    --- PASS: TestListWidthFormula/empty_list
--- PASS: TestTruncateLeftGraphemeSafe
    --- PASS: TestTruncateLeftGraphemeSafe/fits
    --- PASS: TestTruncateLeftGraphemeSafe/exact_fit
    --- PASS: TestTruncateLeftGraphemeSafe/one_cell_over
    --- PASS: TestTruncateLeftGraphemeSafe/basename_kept
    --- PASS: TestTruncateLeftGraphemeSafe/one_cell_budget
    --- PASS: TestTruncateLeftGraphemeSafe/zero_budget
    --- PASS: TestTruncateLeftGraphemeSafe/negative_budget
    --- PASS: TestTruncateLeftGraphemeSafe/wide_cluster_straddles_cut
    --- PASS: TestTruncateLeftGraphemeSafe/combining_cluster_survives
    --- PASS: TestTruncateLeftGraphemeSafe/combining_cluster_dropped_whole
    --- PASS: TestTruncateLeftGraphemeSafe/zwj_emoji_never_split
--- PASS: TestFilenameRuleStatusSlot
    --- PASS: TestFilenameRuleStatusSlot/no_note
    --- PASS: TestFilenameRuleStatusSlot/note_joins_the_rule
    --- PASS: TestFilenameRuleStatusSlot/path_truncates_for_the_note
    --- PASS: TestFilenameRuleStatusSlot/note_wins_the_whole_row
    --- PASS: TestFilenameRuleStatusSlot/note_too_wide_is_dropped
    --- PASS: TestFilenameRuleStatusSlot/tiny_width_is_all_dashes
    --- PASS: TestFilenameRuleStatusSlot/zero_width
    --- PASS: TestFilenameRuleStatusSlot/no_path_is_all_dashes
    --- PASS: TestFilenameRuleStatusSlot/wide_grapheme_never_split
--- PASS: TestListReducedLeavesTenTextCells
--- PASS: TestZeroWidthAllocationKeepsPreference
PASS
ok  	vrg/internal/app
```

## Toggle, scroll, and anchor-relayout tests

The App-level half: `TestListToggleKeys` drives all four keys — the list is shown initially, `tab`/`left` hide and `shift+tab`/`right` show, each press returning the current file's keyed relayout request for the new text width and a repeated press issuing no command — and `TestListToggleKeysInertWhileSearching` keeps them inert outside browse; `TestListHideShowPreservesAnchorText` puts the top mid-way through a wrapped line and proves the same text stays at the top through a `tab`/`shift+tab` round trip; `TestGutterGrowthRelayoutKeepsAnchorText` reloads the file with a wider gutter so the rewrap narrows `textW` through the prepared-layout path with the anchor's text at the top in both directions; `TestListScrollsToKeepActiveVisible` pins minimal-movement window scrolling on a 30-file list; `TestListRenderQueriesOnlyVisibleWindow` asserts the `listEntry` provider is queried for exactly the shown slice; and `TestStatusSlotRendersInFilenameRow` renders a synthetic status note at 80 and 30 columns.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestListToggleKeys|TestListToggleKeysInertWhileSearching|TestListHideShowPreservesAnchorText|TestGutterGrowthRelayoutKeepsAnchorText|TestListScrollsToKeepActiveVisible|TestListRenderQueriesOnlyVisibleWindow|TestStatusSlotRendersInFilenameRow' ./internal/app 2>&1 | grep -vE '^(=== RUN|=== CONT)' | sed -E 's/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//'
```

```output
--- PASS: TestListToggleKeys
--- PASS: TestListToggleKeysInertWhileSearching
--- PASS: TestListHideShowPreservesAnchorText
--- PASS: TestGutterGrowthRelayoutKeepsAnchorText
--- PASS: TestListScrollsToKeepActiveVisible
--- PASS: TestListRenderQueriesOnlyVisibleWindow
--- PASS: TestStatusSlotRendersInFilenameRow
PASS
ok  	vrg/internal/app
```

## Manual check — the real binary on a PTY

`demo-list.sh` (checked into this directory) runs the freshly built `vrg` on a real tmux PTY with real `rg`: `vrg MARK .` in a fixture of four files whose paths exceed 30 cells each — one carrying a 250-cell wrapped line with `NEEDLE` at cells 44–49, and `d-wide-gutter-five-digit-line-numbers.txt` carrying 12,001 lines. The probes measure the list's edge by locating the filename rule's first dash: at 80 columns the 40% cap binds at 32 cells and every entry truncates to a `…`-prefixed basename; scrolling two rows lands `NEEDLE` at the top mid-wrap, `tab` hides the list (the rule runs from column 1, the widened row still shows `NEEDLE`) and `shift+tab` restores it with byte-identical top text; navigating to the five-digit file widens the gutter field while the cap keeps the list at 32 — the panel-minimum term only bites at narrower widths, so shrinking to 25 columns narrows the list to 8 on the five-digit file and back to 10 on a one-digit file (and 8 again on return); finally `w` into run-off-edge at 30 columns gives `min(longest+2, 12, 12)` = 12 — a constrained but present list — and enlarging to 80 restores 32.

```bash
cd /home/chris/vrg/Notes/walkthroughs/024-04/code-walkthrough && unset RIPGREP_CONFIG_PATH && ./demo-list.sh
```

```output
ok: 80 cols: the 40%% cap binds — the list is exactly 32 cells -> 32
ok: 80 cols: a truncated entry leads with the … marker
ok: 80 cols: every listed path is left-truncated to its basename tail
ok: two downs: the top row leads with NEEDLE — a mid-line cell 44 -> NEEDLE
ok: tab: the list is hidden — the rule runs from column 1 -> 0
ok: tab: the widened top row still shows NEEDLE
ok: shift+tab: the list is restored at 32 cells -> 32
ok: shift+tab: identical text at the top — the anchor held -> NEEDLExxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
ok: 80 cols on the 5-digit file: the cap still binds at 32 -> 32
ok: 80 cols: the gutter field widened to five digits ->     2  
ok: 25 cols on the 5-digit file: the list narrows to 8 -> 8
ok: 25 cols on a 1-digit file: the list widens back to 10 -> 10
ok: 25 cols back on the 5-digit file: narrowed to 8 again -> 8
ok: 30 cols run-off-edge: min(longest+2, 12, 12) — a constrained but present list -> 12
ok: 30 cols: entries still show …-prefixed basenames
ok: enlarged to 80: the list widens back to 32 -> 32
ok: vrg exit status -> 0
demo-list: all checks passed
```

```bash
cd /home/chris/vrg/Notes/walkthroughs/024-04/code-walkthrough && echo '== 80 cols, wrap: list capped at 32, …-prefixed basenames ==' && sed -n '1,5p' list/screen-01-80-wrap.txt | sed 's/ *$//' && echo '== tab: list hidden — rule from column 1, widened row keeps NEEDLE ==' && sed -n '1,5p' list/screen-02-hidden.txt | sed 's/ *$//' && echo '== shift+tab: restored — same top text ==' && sed -n '1,5p' list/screen-03-restored.txt | sed 's/ *$//' && echo '== 80 cols on the five-digit file: gutter field widened, cap still binds ==' && sed -n '1,5p' list/screen-04-wide-gutter-80.txt | sed 's/ *$//' && echo '== 25 cols on the five-digit file: panel minimum binds — list 8 ==' && sed -n '1,5p' list/screen-05-narrow-25.txt | sed 's/ *$//' && echo '== 30 cols run-off-edge: min(longest+2,12,12) — constrained but present ==' && sed -n '1,5p' list/screen-07-constrained-30.txt | sed 's/ *$//'
```

```output
== 80 cols, wrap: list capped at 32, …-prefixed basenames ==
…ame-for-the-list-width-demo.txt── …-long-file-name-for-the-list-width-demo.txt
…ong-name-for-the-width-demo.txt 1  MARK one
…d-file-name-also-quite-long.txt 2  xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
…ter-five-digit-line-numbers.txt    NEEDLExxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
                                    xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
== tab: list hidden — rule from column 1, widened row keeps NEEDLE ==
── ./a-long-file-name-for-the-list-width-demo.txt ──────────────────────────────
 2  xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxNEEDLExxxxxxxxxxxxxxxxxxxxxxxxxx
    xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
    xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
    xxxxxxxxxxxxxxxxxxxxxx
== shift+tab: restored — same top text ==
…ame-for-the-list-width-demo.txt── …-long-file-name-for-the-list-width-demo.txt
…ong-name-for-the-width-demo.txt    NEEDLExxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
…d-file-name-also-quite-long.txt    xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
…ter-five-digit-line-numbers.txt    xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
                                    xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
== 80 cols on the five-digit file: gutter field widened, cap still binds ==
…ame-for-the-list-width-demo.txt── ./d-wide-gutter-five-digit-line-numbers.txt ─
…ong-name-for-the-width-demo.txt    1  MARK four
…d-file-name-also-quite-long.txt    2  x00002
…ter-five-digit-line-numbers.txt    3  x00003
                                    4  x00004
== 25 cols on the five-digit file: panel minimum binds — list 8 ==
…emo.txt── …-numbers.txt
…emo.txt    1  MARK four
…ong.txt    2  x00002
…ers.txt    3  x00003
            4  x00004
== 30 cols run-off-edge: min(longest+2,12,12) — constrained but present ==
…th-demo.txt── …e-numbers.txt
…th-demo.txt    1  MARK four
…te-long.txt    2  x00002
…numbers.txt    3  x00003
                4  x00004
```

## Verdict

Issue #24 is verified. On the real binary the file list caps at exactly `floor(0.40 × 80) = 32` columns with every long path left-truncated to a `…`-prefixed basename tail; `tab` hides it (the filename rule runs from column 1 and the widened content keeps the mid-line `NEEDLE` at the top) and `shift+tab` restores it with the anchor's text identical at the top — the hide/show round trip is a text-width relayout through the prepared-layout path, not a viewport reset. The five-digit gutter is visible on the 12,001-line file; at 80 columns the 40% cap still binds so no narrowing is needed, while at 25 columns the panel-minimum term narrows the list 10 → 8 on the wide file and widens it back on a narrow one. At 30 columns in run-off-edge mode the formula's `min(longest+2, 12, 12)` gives a constrained but present 12-cell list that widens back to 32 on enlargement. The automated suite pins every term of the formula, the floor rounding, zero-width allocation preserving the visibility preference, grapheme-safe truncation that never exceeds its budget, the filename-row status slot's truncation priority, minimal-movement list scrolling, the visible-window render-cost guarantee, and anchor preservation through hide/show and gutter-growth relayouts.
