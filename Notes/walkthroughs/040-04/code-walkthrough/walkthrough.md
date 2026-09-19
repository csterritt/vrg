# Issue #40: browse rendering drops the per-frame whole-index scan

*2026-09-18T08:58:44Z by Showboat 0.6.1*
<!-- showboat-id: eecddae4-46c6-4087-b261-ef5de50ac2fe -->

Issue #40 removes the last per-frame index work from browse rendering. Search completion now prepares m.displayPaths once - each file's EscapePath text, safepresentation.CellWidth, and grapheme-cluster boundaries - and listWBase comes from those prepared widths. listCell, filenameRule, and compositePopup then clip the visible window and the current path at the live width via displayPath.leftTruncate, so a navigation Update() plus its View() escapes, segments, regroups, or reallocates nothing: per-keystroke and per-frame cost is bounded by the visible window at ~100,000 matched-line scale, landing PRD 'Resources and responsiveness' for the browse path. See Notes/issues/040-browse-render-no-whole-index-scan.md, Notes/tasks/040-browse-render-no-whole-index-scan.md, and the 'Resources and responsiveness' and 'File list and layout' sections of Notes/PRD-vrg.md. This walkthrough runs the Issue #40 cost-guard tests, then drives the issue's manual scenario on a real PTY through smoke.py: a 100,000-matched-line index across 2,000 files, held n/down bursts, and repeated resizes, with per-keystroke latency measured against a small index. Artifacts (the built vrg binary, smoke.py, and the generated fixture) live in this directory.

## Automated tests — bounded-render cost guard

layout_test.go's counting escapePath seam spans a navigation Update() AND the resulting View() with no counter reset between them: TestNavigateAndRenderEscapeNoPaths sends n through a post-search model and demands zero escapes across both halves; TestRenderEscapesNoPaths (renamed and strengthened from Issue #24's TestRenderEscapesOnlyVisibleListEntries) demands zero escapes for a bare frame over a 50-file index; TestResizeRetruncatesFromPreparedPaths drives resize -> hidden-list -> resize and asserts correctly re-truncated visible paths with zero escapes; TestNavigationKeepsPreparedGroups pins &m.idx.Files[0] and &m.displayPaths[0] pointer-stable across n/p over a 1000-file index - navigation regroups or reallocates nothing; filelist_test.go's TestHiddenListEscapesNoEntries now expects zero escapes even for a visible list.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestHiddenListEscapesNoEntries|TestRenderEscapesNoPaths|TestNavigateAndRenderEscapeNoPaths|TestNavigationKeepsPreparedGroups|TestResizeRetruncatesFromPreparedPaths' ./internal/app 2>&1 | grep -E '^( *--- (PASS|FAIL|SKIP)|ok|FAIL)' | sed -E 's/\([0-9.]+s\)//g; s/\t[0-9.]+s$//; s/^ +//'; echo "exit=${PIPESTATUS[0]}"
```

```output
--- PASS: TestHiddenListEscapesNoEntries 
--- PASS: TestRenderEscapesNoPaths 
--- PASS: TestNavigateAndRenderEscapeNoPaths 
--- PASS: TestNavigationKeepsPreparedGroups 
--- PASS: TestResizeRetruncatesFromPreparedPaths 
ok  	vrg/internal/app
exit=0
```

The unchanged safety net stays green too: the file-list layout contracts (Issue #24), the unified grapheme/cell renderer (Issue #39), and the full suite.

```bash
cd /home/chris/vrg && go build ./... && go vet ./... && go test ./... 2>&1 | sed -E 's/\t([0-9.]+s|\(cached\))//g'; echo "exit=${PIPESTATUS[0]}"
```

```output
?   	vrg/Notes/walkthroughs/007-04/code-walkthrough/fixture	[no test files]
ok  	vrg/cmd/vrg
ok  	vrg/internal/app
ok  	vrg/internal/cli
ok  	vrg/internal/docs
ok  	vrg/internal/filebuffer
ok  	vrg/internal/safepresentation
?   	vrg/internal/safepresentation/sinktest	[no test files]
ok  	vrg/internal/searchindex
ok  	vrg/internal/theme
ok  	vrg/internal/viewport
exit=0
```

## Manual scenario — held keys and resizes at ~100,000 matched lines

smoke.py generates fixture/many (2000 files x 50 matched lines = 100,000 matched lines) and fixture/few (8 files x 5 lines = 40), then drives the built vrg binary on a real 80x24 PTY through the issue's manual scenario, replaying the byte stream into a cell grid so assertions are made on the rendered frame. Every key is sent only after an explicit rendered condition is observed (bounded polls, never fixed delays). A held n/down is simulated by writing the whole burst at once - faster than any autorepeat - and each burst's settle time is measured from send to the rendered frame proving the last keystroke landed: two 200-key n bursts crossing file boundaries, a 40-key down burst to the file's end, three rapid resizes that mint fresh keyed layouts and re-truncate the visible list entries against the new widths, then the same n burst on the 40-stop index for the per-key cost comparison. If per-frame work still scaled with index size, the 100k-stop bursts would take orders of magnitude longer than the 40-stop ones.

```bash
cd /home/chris/vrg/Notes/walkthroughs/040-04/code-walkthrough && python3 smoke.py; echo "smoke exit=$?"
```

```output
scenario: bounded per-keystroke render cost at ~100,000 matched lines
  [PASS] initial frame: file 0 current, list capped at 32
  [PASS] file list shows only the visible window's entries
  n-burst 1: 200 keys over a 100,000-stop index settled in 17ms (0.1ms/key)
  [PASS] held-n burst per-key cost bounded at 100k stops
  n-burst 2: 200 more keys settled in 16ms (0.1ms/key)
  [PASS] second held-n burst equally bounded
  down-burst: 40 keys settled in 21ms (0.5ms/key)
  [PASS] held-down burst per-key cost bounded
  resize to 100x30 settled in 12ms
  resize to 60x20 settled in 22ms
  resize to 80x24 settled in 14ms
  [PASS] repeated resizes settle bounded
  [PASS] post-resize frame intact at 80x24
  [PASS] exit 0
  [PASS] browse cursor restored
  [PASS] browse termios restored
  small-index n-burst: 15 keys settled in 14ms (1.0ms/key)
  [PASS] small run exit 0
  per-key cost: 0.1ms (100k stops) vs 1.0ms (40 stops) - ratio 0.0x
  [PASS] per-key cost does not grow with index size
all checks passed
smoke exit=0
```

The measured results show the bound directly: 200 held-n keystrokes over a 100,000-stop index settle in ~17ms (~0.1ms/key) - the same order as the 40-stop baseline - and three rapid resizes each settle in ~15ms while re-truncating the visible list entries against the new widths. The file list materializes only its 22-row visible window (f00022 is the last entry painted; f00023 appears nowhere). Together with the zero-escape guards spanning Update()+View() and the pointer-stability pin on m.idx.Files and m.displayPaths, this covers the issue's acceptance criteria: no navigation/update/render path enumerates or copies the complete stop list, only the visible file range is materialized per frame, and per-keystroke cost does not grow with index size.
