# Issue #17: logical anchor through rewrap and resize

*2026-09-17T18:14:56Z by Showboat 0.6.1*
<!-- showboat-id: 66afa00d-4719-4804-abe5-0dbf2f4cda99 -->

Issue #17 lands the logical viewport anchor: the file panel's reading position is now a width-independent `Anchor{Line, Cell}` — the source line plus display-column offset the effective top row must contain — retained through rewraps, wrap toggles, and resizes instead of a rendered-row ordinal. Scrolling and moving reveals replace the anchor with the resulting top row's location; a no-scroll reveal keeps a retained mid-row column; and the EOF clamp intentionally rewrites it (lossy — a later shrink cannot resurrect the pre-clamp top). Every `viewport.Prepare` now runs off the update path as a Bubble Tea command: `requestLayout` deduplicates by key (path, content revision, text width, wrap mode), `layoutReadyMsg` completions install only while the key still matches the live layout, `currentRows` hides stale installed models so rendering and scrolling see a placeholder, and reveal intents that cannot run pend on `pendingReveals` and commit when a matching layout installs — so input stays responsive no matter how long preparation takes. See `Notes/issues/017-logical-anchor-through-rewrap-and-resize.md`, `Notes/tasks/017-logical-anchor-through-rewrap-and-resize.md`, and the PRD section "Navigation, viewport, and logical anchors" in `Notes/PRD-vrg.md`. All artifacts live in this directory.

## Gates — module integrity, build, vet

```bash
cd /home/chris/vrg && go mod verify && go build ./... && go vet ./... && echo GATES-OK
```

```output
all modules verified
GATES-OK
```

## Logical anchor — rewrap round trip, wrap toggle, replacement rules, lossy EOF clamp

`internal/viewport/anchor_test.go` pins the anchor contract over real prepared buffers: a 95-cell line scrolled to row 5 at text width 10 anchors `{Line: 1, Cell: 50}`, restores at width 7 to the row *containing* cell 50 (row 7, not ordinal 5), and returns to row 5 at width 10; the wrap-off/wrap-on round trip keeps the column while the line shows as one row; a scroll that moves the effective top replaces the anchor while a no-scroll reveal keeps a retained mid-row column; and both lossy-clamp cases — `Clamp` and `Restore` pulling the top up — rewrite the anchor so a later shrink cannot resurrect the pre-clamp position. `internal/app/anchor_test.go` drives the same guarantee through the model: a resize preserves both the cursor selection and the retained anchor.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'Anchor|EOFClamp|WrapToggle|Rewrap|ReplacesAnchor|RetainsLogicalColumn|PreservesCursor' ./internal/viewport ./internal/app 2>&1 | grep -E '^(--- |ok|FAIL)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestAnchorRewrapKeepsTextLocation
--- PASS: TestAnchorSurvivesWrapToggle
--- PASS: TestScrollReplacesAnchor
--- PASS: TestMovingRevealReplacesAnchor
--- PASS: TestNoScrollRevealRetainsLogicalColumn
--- PASS: TestEOFClampUpdatesAnchorLossy
--- PASS: TestRestoreEOFClampUpdatesAnchor
--- PASS: TestRevealBOFEOFClamps
ok  	vrg/internal/viewport
--- PASS: TestResizePreservesCursorAndAnchor
--- PASS: TestRapidWrapToggleDiscardsSupersededLayout
ok  	vrg/internal/app
```

## Prepared layouts off `Update` — gated responsiveness, isolation, cached files, render cost

`internal/app/layout_test.go` pins the async pipeline under the `heldLayouts` gate (`WithLayoutGate`): while a preparation worker is held, `n`/`p` move the cursor and record the pending reveal intent, `w` flips wrap and issues a newly keyed request without releasing the worker, a second resize is accepted likewise, `q` exits with the fixed status and `ctrl+c` with 130 — none of it waits; releasing the workers installs only the newest key and commits the pending reveal to the latest stop. Out-of-order completions (three resizes delivered oldest-first) and a superseded wrap-mode completion are discarded without touching the installed layout, the saved viewports, or the anchors — as is a completion minted under a superseded content revision, and an obsolete completion for a non-current file changes nothing visible. Navigating to a cached file whose layout went stale requests a fresh preparation (the reveal pends and the panel shows "Loading…" rather than stale rows); a cached file whose layout still matches is the fast path — the reveal applies at once with no request. Finally, `TestRenderEscapesOnlyVisibleListEntries` proves the file-list render escapes only the visible window's paths.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'Gated|OutOfOrder|RapidWrap|Obsolete|StaleRevision|CachedFile|EscapesOnlyVisible' ./internal/app 2>&1 | grep -E '^(--- |ok|FAIL)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestGatedLoadStaysResponsive
--- PASS: TestGatedLayoutKeepsInputsResponsive
--- PASS: TestGatedLayoutCtrlCExits130
--- PASS: TestOutOfOrderLayoutCompletions
--- PASS: TestRapidWrapToggleDiscardsSupersededLayout
--- PASS: TestObsoleteLayoutForOtherFileDiscarded
--- PASS: TestStaleRevisionCompletionDiscarded
--- PASS: TestCachedFileStaleLayoutRequestsFresh
--- PASS: TestCachedFileFreshLayoutFastPath
--- PASS: TestRenderEscapesOnlyVisibleListEntries
ok  	vrg/internal/app
```

## Full suite — `go test ./...`

```bash
cd /home/chris/vrg && go test -count=1 ./... 2>&1 | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
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

## Manual check — the real binary on a pty

`pty_anchor.py` runs the built vrg on real ptys and replays the byte stream through a small terminal emulator. Session A uses `fixture/long.txt` — a 2000-cell line carrying `MARKER` at the anchor cell, 40 pad lines, and a 3000-cell tail line straddling EOF — driven from a short /tmp cwd (via symlinks `f`/`h`) so the file list stays narrow at 60 columns. The session scrolls a half-page into the wrapped line (`d`), narrows to 60 and widens to 140 columns and back — the same text stays at the top each time because the retained anchor is (line, cell), not a row ordinal — toggles `w` twice (run-off-edge shows the anchor's line as one numbered row; wrapping back restores the original top row), then scrolls to EOF at 60 columns and widens: the EOF clamp pulls the top to an earlier cell and *rewrites* the anchor, so narrowing back lands on the clamped location (`WIDE`) rather than the pre-clamp one (`TAIL`) — the documented lossy case. Session B loads `fixture-huge/huge.txt` (~50 MB, generated), fires a burst of resizes so layout preparations queue up off the update path, and sends `ctrl+c` mid-rewrap — exit 130 in well under a second.

```bash
cd /home/chris/vrg && go build -o Notes/walkthroughs/017-06/code-walkthrough/vrg ./cmd/vrg && python3 Notes/walkthroughs/017-06/code-walkthrough/pty_anchor.py
```

```output
init     : loaded — ' 1  needle first'
d@100    : anchor is {line 2, cell 650} — '    MARKERxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx'
60       : rewrapped — same text at top — '    MARKERxxxxxxxxxxxxxxxxxxx'
140      : rewrapped — same text at top — '    xxxxxxxxxxxxxxxxxx'…
100      : original top row restored — '    MARKERxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx'
w        : line 2 as one clipped row — ' 2  xxxxxxxxxxxx'…
w        : wrapped again — same top row — '    MARKERxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx'
d…@60    : EOF — top '    xxTAILxxxxxxxxxxxxxxxxxxx', bottom is the tail end '    xxxxxxxxxxxxxxxxxxxxxxxxx'
140      : clamp moved the top to cell 630 — '    ExCLMPxxxxxx'…
60       : stays at the clamped location — '    xxWIDExCLMPxxxxxxxxxxxxxx'
q        : exit 0
huge     : ~50 MB loaded and laid out
ctrl+c   : exit 130 with rewraps in flight — well under the 5s promptness bound
OK
```

All Issue #17 contracts verified: the logical anchor survives rewraps and wrap toggles as a (source line, display column) location — the effective top is always the row containing it, never a stale ordinal; scrolling and moving reveals replace it, a no-scroll reveal keeps the retained column, and the EOF clamp rewrites it so a later shrink cannot resurrect the pre-clamp top. Layout preparation runs off the update path behind keyed completions — `Key{Path, Revision, TextWidth, Wrap}` — deduplicated by `layoutReqs`, guarded by `layoutKey` on install, hidden by `currentRows` while stale, with `pendingReveals` committing the latest intent on install; input stays responsive under a gated worker and `ctrl+c` exits 130 even with a ~50 MB rewrap in flight. Rendering stays bounded: the content render queries `rowSource` only for visible rows and the file list escapes only visible entries. One test-harness fix along the way: the cmd/vrg PTY drain could lag `Wait` by a scheduling quantum, dropping the display-restoration tail — `waitExit` now waits for the drain. Issue #20 owns the reserved indicator column's content; Issue #19 owns horizontal reveal.
