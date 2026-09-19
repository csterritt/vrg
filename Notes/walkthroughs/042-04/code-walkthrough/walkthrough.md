# Issue #42: a dropped r mutates nothing — atomic load admission

*2026-09-18T09:57:05Z by Showboat 0.6.1*
<!-- showboat-id: 82173710-fb6a-4f64-8813-a889ed20415a -->

Issue #42 makes load admission atomic: mintLoad in internal/app/browse.go is the single admission point for every file-load request — the in-flight check runs inside it, ahead of the failed-record clear and the loadSeq/loading mutation, so a duplicate request is dropped whole and can never reclassify the load already in flight, bump a revision, or plant a reveal/anchor intent. A dropped r during a startup or navigation load leaves the in-flight request's original non-reload classification intact, so its completion performs the normal destination/first-match reveal rather than anchor preservation; an accepted r still mints a fresh identity, shows 'Loading…', marks its completion reload, and bumps the revision exactly once; and navigation re-entry is deliberately ungated — it updates selection, placeholder, and reveal intent even when its duplicate load leaf is dropped. See Notes/issues/042-dropped-reload-no-intent-mutation.md, Notes/tasks/042-dropped-reload-no-intent-mutation.md, and the 'File loading, cache, reload, and selection consistency' and 'Navigation, viewport, and logical anchors' sections of Notes/PRD-vrg.md. This walkthrough runs the Issue #42 admission tests and the neighboring reload/isolation suites, then drives the issue's manual scenario on a real PTY through smoke.py. Artifacts (the built vrg binary, smoke.py, and the generated fixture) live in this directory.

## Automated tests - atomic admission and the neighboring contracts

internal/app/admission_test.go pins the contract: TestDroppedRDuringStartupLoadPreservesIntent holds the startup load, sends r, and asserts the live request identity, loadSeq, revision, intents, and rendered frame are all unchanged - then releases and proves the completion is not reload-marked and commits the first-match reveal; TestDroppedRDuringNavigationLoadPreservesIntent does the same for a held cross-file load, keeping the destination's pendingReveals intact; TestAcceptedReloadAppliesFlagsIntentAndOneRevision proves an accepted r mints fresh, shows Loading…, marks the completion reload, and bumps the revision exactly once at completion; TestRapidRPressesKeepOneLoadInFlight sends repeated r under a held reload and proves no extra worker ever starts; TestReentryDuringInFlightLoadUpdatesSelectionAndIntent proves re-entry drops only the duplicate load while selection, placeholder, and reveal intent still update.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'Test(DroppedRDuringStartupLoadPreservesIntent|DroppedRDuringNavigationLoadPreservesIntent|AcceptedReloadAppliesFlagsIntentAndOneRevision|RapidRPressesKeepOneLoadInFlight|ReentryDuringInFlightLoadUpdatesSelectionAndIntent)$' ./internal/app 2>&1 | grep -E '^( *--- (PASS|FAIL|SKIP)|ok|FAIL)' | sed -E 's/\([0-9.]+s\)//g; s/\t[0-9.]+s$//; s/^ +//'; echo "exit=${PIPESTATUS[0]}"
```

```output
--- PASS: TestDroppedRDuringStartupLoadPreservesIntent 
--- PASS: TestDroppedRDuringNavigationLoadPreservesIntent 
--- PASS: TestAcceptedReloadAppliesFlagsIntentAndOneRevision 
--- PASS: TestRapidRPressesKeepOneLoadInFlight 
--- PASS: TestReentryDuringInFlightLoadUpdatesSelectionAndIntent 
ok  	vrg/internal/app
exit=0
```

The neighboring reload and load-isolation suites - the contracts admission guards - stay green alongside, and the full build/vet/test pass holds.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestR|TestReload|TestDuplicate|TestReentry|TestNavigation|TestLate|TestStale|TestSettled|TestPost|TestUnrequested|TestCached|TestGated' ./internal/app 2>&1 | grep -E '^( *--- (PASS|FAIL|SKIP)|ok|FAIL)' | sed -E 's/\([0-9.]+s\)//g; s/\t[0-9.]+s$//; s/^ +//'; echo "exit=${PIPESTATUS[0]}"
```

```output
--- PASS: TestRapidRPressesKeepOneLoadInFlight 
--- PASS: TestReentryDuringInFlightLoadUpdatesSelectionAndIntent 
--- PASS: TestResizePreservesCursorAndAnchor 
--- PASS: TestResizeDuringSearching 
--- PASS: TestRunStartFailureExit2 
--- PASS: TestRunStartFailureSanitizesError 
--- PASS: TestGatedLoadStaysResponsive 
--- PASS: TestResizeRecomposesBrowse 
--- PASS: TestLateLoadForOtherFileIgnored 
--- PASS: TestLateCompletionAfterCancelDiscarded 
--- PASS: TestRunCleansUpOnProgramError 
--- PASS: TestRightStarFollowsCurrentLine 
--- PASS: TestRightStarAbsentWhenCurrentLineOffScreen 
--- PASS: TestGatedLayoutKeepsInputsResponsive 
--- PASS: TestGatedLayoutCtrlCExits130 
--- PASS: TestRapidWrapToggleDiscardsSupersededLayout 
--- PASS: TestStaleRevisionCompletionDiscarded 
--- PASS: TestCachedFileStaleLayoutRequestsFresh 
--- PASS: TestCachedFileFreshLayoutFastPath 
--- PASS: TestRenderEscapesNoPaths 
--- PASS: TestNavigationKeepsPreparedGroups 
--- PASS: TestResizeRetruncatesFromPreparedPaths 
--- PASS: TestNavigationRemainsActiveWhileLoadHeld 
--- PASS: TestLateCompletionCachesOnlyItsOwnPath 
--- PASS: TestReentryDuringLoadStartsNothing 
--- PASS: TestCachedRevisitIssuesNoLoad 
--- PASS: TestUnrequestedCompletionDropped 
--- PASS: TestStaleCompletionDroppedWhileLoadInFlight 
--- PASS: TestSettledCompletionDropped 
--- PASS: TestPostCancellationCompletionDiscarded 
--- PASS: TestGatedDecodeMapKeepsInputsResponsive 
--- PASS: TestResizeBetweenLoadAndLayoutCommitsAtNewWidth 
--- PASS: TestStaleRevisionLayoutDiscardsWithoutConsuming 
--- PASS: TestNavigationWrapsBothEnds 
--- PASS: TestNavigationWhileLoadInFlight 
--- PASS: TestRecordLossDiagnostics 
--- PASS: TestRevealReclampsOffset 
--- PASS: TestResizeReclampsOffset 
--- PASS: TestReentryShowsPriorFailureAndStartsOneRetry 
--- PASS: TestReentryRetryEscLeavesLoadUndisturbed 
--- PASS: TestReentryRetrySuccessKeepsPriorOverlay 
--- PASS: TestReentryRetrySecondFailureAppends 
--- PASS: TestReentryRetryAwayAndBack 
--- PASS: TestReadmeListsEveryBinding 
--- PASS: TestReadmeExitStatuses 
--- PASS: TestRShowsLoadingAndRereadsOnce 
--- PASS: TestDuplicateRDroppedNotQueued 
--- PASS: TestReentryDuringReloadDropped 
--- PASS: TestReloadPreservesAnchor 
--- PASS: TestReloadAnchorClampedOnShrink 
--- PASS: TestReloadSuccessLeavesOverlayOpen 
--- PASS: TestRWorksWithOneStopIndex 
--- PASS: TestReloadWithoutNavigationRecordsAnchorIntent 
--- PASS: TestReloadNavigationDuringLoadReplacesIntent 
--- PASS: TestReloadAwayBackSameFileEntryReveal 
--- PASS: TestReloadAwayBackCrossFileEntryReveal 
--- PASS: TestReloadOutOfOrderRevisionsCommitReveal 
--- PASS: TestReplayCollectsEveryDiagInOrder 
--- PASS: TestReplayEscapesEmbeddedFilename 
--- PASS: TestResizeRemeasuresTextWidth 
--- PASS: TestNavigationRevealHiddenThenBOF 
--- PASS: TestRevisitRevealStartsFromSavedViewport 
--- PASS: TestRenderQueriesOnlyVisibleRows 
--- PASS: TestResizeReclampsViewport 
--- PASS: TestStaleNoteInFilenameRow 
--- PASS: TestStaleNoteClearedByCleanReload 
--- PASS: TestStaleGatedReloadCommitRevealsSurvivor 
--- PASS: TestStaleGatedReloadCommitRevealsClampedFallback 
--- PASS: TestStaleMissingLineLandsOnLastLine 
--- PASS: TestRevealMatchDeepInWrappedLine 
--- PASS: TestResizeRebuildsRowModel 
ok  	vrg/internal/app
exit=0
```

```bash
cd /home/chris/vrg && go build ./... && go vet ./... && go test -count=1 ./... 2>&1 | sed -E 's/\t([0-9.]+s|\(cached\))//g'; echo "exit=${PIPESTATUS[0]}"
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

## Manual scenario - r during held loads, then the accepted-r contrast

smoke.py generates fixture/work/{a,b}.txt (60 numbered lines each, the single match 'alpha line 50' deep enough that revealing it must scroll the 23-row panel) and fixture/fakebin/rg (a fake child emitting one match per file at line 50, exit 0), then drives the built vrg binary on a real 80x24 PTY with VRG_TEST_LOAD_GATE=fixture/loadgate - while that file exists every file load holds, so r can be pressed with a load genuinely in flight. Leg 1 launches with the gate up: the startup load sits behind 'Loading…', r is sent, and a bounded negative poll proves the frame stays byte-identical; on gate release the completion commits the first-match reveal - the match row is in view and the panel's top gutter is past line 1, never the top-0 an anchor-preserving (misclassified) completion would have kept. Leg 2 re-arms the gate, crosses to b.txt with n, waits out the file-change pop-up, sends r under the held navigation load - again a byte-identical frame - and the release commits the destination reveal. Leg 3 is the contrast: with B settled the gate goes up once more and this r IS admitted - the panel visibly drops to 'Loading…' at the keypress, then settles back onto the byte-identical pre-reload frame through anchor preservation. q exits 0. Every key is sent only after an explicit rendered condition is observed; the 'nothing happens' proofs are bounded negative polls, never fixed delays.

```bash
cd /home/chris/vrg/Notes/walkthroughs/042-04/code-walkthrough && python3 smoke.py; echo "smoke exit=$?"
```

```output
scenario: dropped r during held loads mutates nothing; completion reveals, never preserves an anchor
  [PASS] r during startup load: frame unchanged
  [PASS] startup completion reveals the match at line 50
  [PASS] reveal scrolled the viewport (not anchor's top 0)
  [PASS] no second Loading… phase preceded the reveal
  [PASS] r during navigation load: frame unchanged
  [PASS] navigation completion reveals the match at line 50
  [PASS] destination reveal scrolled the viewport
  [PASS] accepted r drops the panel to Loading…
  [PASS] accepted reload restores the identical frame
  [PASS] exit 0 (clean results, q)
  [PASS] browse cursor restored
  [PASS] browse alt screen exited
  [PASS] browse termios restored
all checks passed
smoke exit=0
```

## Result

Issue #42 is proven at both levels: the model tests pin the atomic-admission invariant (a duplicate request mints nothing and mutates nothing, classification is fixed inside the accepted request, navigation re-entry stays ungated), and the PTY scenario shows the user-visible consequence - r pressed while a load is in flight changes no pixel, and the completing load reveals the destination/first match instead of preserving an anchor it never had. The accepted-r leg provides the control: when a reload is admitted, 'Loading…' appears at the keypress and the completion restores the identical frame through genuine anchor preservation.
