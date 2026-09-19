# Issue #28: two-stage load-completion reveal — latest target, install-gated commit

*2026-09-18T00:07:47Z by Showboat 0.6.1*
<!-- showboat-id: 7c899834-471b-490d-aafa-39c797861444 -->

Issue #28 completes the two-stage completion contract for a current-file load: stage one — fileLoadedMsg — validates and caches the buffer, bumps the content revision, recomputes the text width under the new gutter and the Issue #24 file-list state, and mints the keyed layout request, while performing no row-based decision itself — no visibility test, placement, clamping, or horizontal reveal. The reveal or anchor-preservation intent is carried by the model, always resolving the latest selected cursor's final display target — the Issue #21 cluster-expanded first-submatch start cell or the Issue #23 marker cell — never a target captured when the load was requested. Obsolete layouts — wrong width, mode, revision, or path — are discarded without consuming or mutating the intent, which commits when and only when a matching prepared layout installs: visible-target no-scroll, else floor(h/3) placement with BOF/EOF precedence, plus horizontal reset then minimal horizontal reveal. A reload with no intervening navigation preserves its anchor; any navigation during the load — including away-and-back ending on the same stop — replaces the intent with the entry reveal, so navigation intent, never cursor equality, decides. See Notes/issues/028-load-completion-reveal-latest-target.md, Notes/tasks/028-load-completion-reveal-latest-target.md, and the PRD sections 'File loading, cache, reload, and selection consistency' and 'Navigation, viewport, and logical anchors' in Notes/PRD-vrg.md. All artifacts live in this directory.

## Gates — module integrity, build, vet

```bash
cd /home/chris/vrg && go mod verify && go build ./... && go vet ./... && echo GATES-OK
```

```output
all modules verified
GATES-OK
```

## The two-stage contract — separately gated loads and layouts

internal/app/loadreveal_test.go pins the contract with the stages held apart — deliverStageOne feeds the load completion and returns the layout command it issued, while consultedRows records every rowSource call. TestLoadCompletionStageOneDefersToInstall: stage one caches the buffer, bumps the revision, requests the current key, and records the reveal intent with no viewport state created and 'Loading…' still up. TestLoadCompletionMakesNoRowDecision: a fake row model standing in as installed sees zero calls through the completion — no visibility test, placement, clamping, or horizontal reveal at stage one. TestStartupHiddenTargetCommitsAfterBothStages: 'Loading…' holds through the held load and again through the held layout, then the install reveals row 199 at content row 7 and the first n advances to the second stop. TestStartupVisibleTargetKeepsTopThroughStages: a first stop visible from top 0 commits without scrolling. TestResizeBetweenLoadAndLayoutCommitsAtNewWidth / TestListHideBetweenStagesCommitsAtFinalWidth / TestGutterGrowthBetweenStagesCommitsAtFinalWidth: a resize, a file-list hide, or gutter growth between the stages re-keys the request — the superseded completion is discarded without consuming the intent, which commits against the final text width. TestNavigateWhileLayoutPendingCommitsNewestTarget: two n presses during the held layout move the cursor at once; the commit reveals the newest selection. TestStaleRevisionLayoutDiscardsWithoutConsuming: a revision-1 layout arriving after the revision-2 reload dies on the key check, the installed model and anchor intent untouched. TestSavedViewportRevisitVisibleStays / TestSavedViewportRevisitHiddenMoves: a stale-layout revisit pends the reveal and the commit keeps or moves the saved top correctly. TestMarkerTargetCommitPaintsMarkerCell / TestClusterTargetCommitPaintsWholeCluster: a terminator-only marker cell and a mid-cluster start cell commit to the right geometry. TestNonCurrentCompletionLeavesPanelUntouched / TestPopupSurvivesBothStages: a non-current completion records no intent and the pop-up instance survives both stages.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestLoadCompletionStageOneDefersToInstall|TestLoadCompletionMakesNoRowDecision|TestStartupHiddenTargetCommitsAfterBothStages|TestStartupVisibleTargetKeepsTopThroughStages|TestResizeBetweenLoadAndLayoutCommitsAtNewWidth|TestNavigateWhileLayoutPendingCommitsNewestTarget|TestListHideBetweenStagesCommitsAtFinalWidth|TestGutterGrowthBetweenStagesCommitsAtFinalWidth|TestStaleRevisionLayoutDiscardsWithoutConsuming|TestSavedViewportRevisitVisibleStays|TestSavedViewportRevisitHiddenMoves|TestMarkerTargetCommitPaintsMarkerCell|TestClusterTargetCommitPaintsWholeCluster|TestNonCurrentCompletionLeavesPanelUntouched|TestPopupSurvivesBothStages' ./internal/app 2>&1 | grep -E '^(--- |ok|FAIL|    ---)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestLoadCompletionStageOneDefersToInstall
--- PASS: TestLoadCompletionMakesNoRowDecision
--- PASS: TestStartupHiddenTargetCommitsAfterBothStages
--- PASS: TestStartupVisibleTargetKeepsTopThroughStages
--- PASS: TestResizeBetweenLoadAndLayoutCommitsAtNewWidth
--- PASS: TestNavigateWhileLayoutPendingCommitsNewestTarget
--- PASS: TestListHideBetweenStagesCommitsAtFinalWidth
--- PASS: TestGutterGrowthBetweenStagesCommitsAtFinalWidth
--- PASS: TestStaleRevisionLayoutDiscardsWithoutConsuming
--- PASS: TestSavedViewportRevisitVisibleStays
--- PASS: TestSavedViewportRevisitHiddenMoves
--- PASS: TestMarkerTargetCommitPaintsMarkerCell
--- PASS: TestClusterTargetCommitPaintsWholeCluster
--- PASS: TestNonCurrentCompletionLeavesPanelUntouched
--- PASS: TestPopupSurvivesBothStages
ok  	vrg/internal/app
```

## Reload-intent transitions — navigation intent, never cursor equality

internal/app/reloadintent_test.go pins the arbitration on the intentFiles fixture — a.txt's two deep stops give same-file away-and-back room, b.txt the cross-file leg — with heldNthLoad(2) holding only the reload. TestReloadWithoutNavigationRecordsAnchorIntent: an undisturbed reload records the anchor intent and no reveal; the held layout keeps the placeholder and saved viewport until the install restores the anchor's row. TestReloadNavigationDuringLoadReplacesIntent: n during the held reload leaves the completion recording no anchor intent at all — the commit reveals the newest stop. TestReloadAwayBackSameFileEntryReveal: n then p during the reload ends on the initial stop, yet the entry reveal commits — the scrolled-off match is placed a third down rather than the scrolled position preserved. TestReloadAwayBackCrossFileEntryReveal: A→B→A during the held reload shows 'Loading…' on the return — the re-entry mints nothing new — and the commit applies the entry reveal. TestReloadOutOfOrderRevisionsCommitReveal: a revision-1 layout delivered before the revision-2 one is discarded without consuming the reveal intent navigation pended during the load; only the new revision's install commits it.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestReloadWithoutNavigationRecordsAnchorIntent|TestReloadNavigationDuringLoadReplacesIntent|TestReloadAwayBackSameFileEntryReveal|TestReloadAwayBackCrossFileEntryReveal|TestReloadOutOfOrderRevisionsCommitReveal' ./internal/app 2>&1 | grep -E '^(--- |ok|FAIL|    ---)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestReloadWithoutNavigationRecordsAnchorIntent
--- PASS: TestReloadNavigationDuringLoadReplacesIntent
--- PASS: TestReloadAwayBackSameFileEntryReveal
--- PASS: TestReloadAwayBackCrossFileEntryReveal
--- PASS: TestReloadOutOfOrderRevisionsCommitReveal
ok  	vrg/internal/app
```

## Full module regression

The change touches fileLoadedMsg's current-file branch, the layout-install commit switch, and the intent maps — the whole suite runs, plus the race detector over the gated worker tests.

```bash
cd /home/chris/vrg && go test -count=1 ./internal/filebuffer ./internal/viewport ./internal/app ./internal/safepresentation ./internal/searchindex ./internal/theme ./internal/cli ./cmd/vrg 2>&1 | sed -E 's/\t[0-9.]+s$//' && CGO_ENABLED=1 go test -race -count=1 ./internal/app 2>&1 | sed -E 's/\t[0-9.]+s$//'
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
ok  	vrg/internal/app
```

## Manual check — the real binary on real ptys

manual_route.sh runs the issue's manual checks as the unprivileged user against disposable mktemp fixtures — never a repository file: it generates five single-file case directories under a trap removing the tree on exit or interruption, and pty_loadreveal.py drives the sessions on real 80x24 (one later 100x30) ptys with stderr redirected to a file. The 'slow load' is VRG_TEST_LOAD_GATE — a file whose existence holds every load command — so the driver holds a load deterministically, acts while it is held, and releases it by deleting the file. c1: the gate holds the first file — 'Loading…' — then the release commits the reveal, the line-500 match landing a third down (top 'pad deep 493'), and the first n advances to the second stop at line 560. c2: a first match on line 3 opens the file from the top — visible-target no-scroll. c3: r with the gate held then n during the load — on completion the newest stop 'needle two 50' is revealed at the EOF-clamped position, not the pre-reload top 0. c4: the line-10 match scrolled off-screen, r held, then n p away-and-back — on completion the entry reveal lands the match at the third rather than preserving the scrolled position: navigation intent, not the equal cursor, decides. c5: an 80x24 → 100x30 resize while the first file is held — the reveal commits against the new size's 29-row content height (top 'pad deep 491'). Every session exits 0. The gated model tests above remain the authoritative deterministic verification.

```bash
cd /home/chris/vrg && go build -o Notes/walkthroughs/028-06/code-walkthrough/vrg ./cmd/vrg && bash Notes/walkthroughs/028-06/code-walkthrough/manual_route.sh
```

```output
fixture: <tmp> (c1/c2/c3/c4/c5 — one file each)
c1 held     : 'Loading…' while the gate holds the load — '─ <tmp>/c1/deep.txt ──────────────'
c1 released : reveal committed — top '…-028-fixture.XXXXXX/c1/deep.txt493  pad deep 493', match a third down
c1 n        : first n — second stop 'needle deep 560' revealed
c1 q        : exit 0
c2 released : first match on line 3 — file opens from the top, no scroll
c2 q        : exit 0
c3 r n      : r held, n during the load — placeholder stays
c3 released : completion — the new stop 'needle two 50' revealed, not the old position
c3 q        : exit 0
c4 d d d    : match scrolled off — top '…-028-fixture.XXXXXX/c4/away.txt34  pad two 34'
c4 r n p    : r held, n p away-and-back — placeholder stays
c4 released : completion — match revealed at the third, the scrolled position not preserved
c4 q        : exit 0
c5 resize   : resized 80x24 → 100x30 mid-load — placeholder stays
c5 released : reveal committed at 100x30 — top '…mp/vrg-028-fixture.XXXXXX/c5/resize.txt491  pad deep 49'
c5 q        : exit 0
OK
```
