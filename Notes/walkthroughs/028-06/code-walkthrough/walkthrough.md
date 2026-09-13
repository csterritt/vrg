# Issue #28: Two-stage load completion (reveal latest target)

*2026-09-12T13:33:45Z by Showboat 0.6.1*
<!-- showboat-id: a1b2c3d4-e5f6-7890-abcd-ef1234567890 -->

Walkthrough for Issue #28 (Notes/tasks/028-load-completion-reveal-latest-target.md), implementing the two-stage load-completion contract in vrg. Stage one (file-load completion) validates the request, stores the loaded data, updates the content revision, and starts asynchronous layout preparation. Stage two (matching layout installation) commits the reveal or reload-anchor intent against the installed rows. The latest selection/navigation intent survives asynchronous loading and layout preparation; stale layouts are discarded without consuming or changing the latest intent. References: Notes/PRD-vrg.md (File loading, cache, reload, and selection consistency), Notes/wiki/load-completion-two-stage.md.

Contracts verified:

- File-load completion does not reveal using an old/current row model before the matching layout is installed.
- A pending latest-selection reveal intent survives while layout preparation is gated.
- A stale layout is discarded without changing the intent.
- Reveal commits only on matching layout installation.
- Reload-anchor and normal reveal intents transition correctly under gated asynchronous completion.
- Cached and uncached navigation paths obey the two-stage behavior.
- Saved viewport revisits preserve visible targets without unnecessary scrolling.
- Hidden targets cause the final installed layout to reveal correctly.
- Non-current file completions do not alter the visible panel or current intent.
- Pop-up behavior remains independent of load/layout staging.
- Reload begins with IntentReloadAnchor.
- A subsequent navigation changes the latest intent to IntentReveal.
- A stale reload layout does not consume the newer reveal intent.
- A matching layout commits the latest intent exactly once.
- A reload with no subsequent navigation preserves the anchor.
- A navigation that occurs while loading still reveals the latest selected target, not the original target.

The injected-loader model tests are the authoritative deterministic verification. The manual route at the end demonstrates the behavior with an injected gated loader.

```bash
go build ./cmd/... ./internal/... && go vet ./cmd/... ./internal/... && echo GATES-OK
```

```output
GATES-OK
```

```bash
go test -count=1 ./cmd/... ./internal/... -timeout 120s | sed 's/[[:space:]][0-9.]*s$//'
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

## Two-stage load-completion tests

The two-stage load-completion tests (internal/app/two_stage_test.go) verify that file-load completion does not reveal using an old/current row model before the matching layout is installed, that a pending reveal intent survives while layout preparation is gated, that a stale layout is discarded without changing the intent, that reveal commits only on matching layout installation, that cached and uncached navigation paths obey the two-stage behavior, that saved viewport revisits preserve visible targets without unnecessary scrolling, that hidden targets cause the final installed layout to reveal correctly, that non-current file completions do not alter the visible panel or current intent, and that pop-up behavior remains independent of load/layout staging.

```bash
go test -count=1 -v ./internal/app/ -run '^TestStageOne|^TestStartupVisible|^TestResizeBetween|^TestNavigationDuring|^TestGutterGrowth|^TestListToggle|^TestSavedViewport|^TestTerminator|^TestNonCurrent|^TestPopupUnaffected|^TestObsolete' -timeout 60s | sed 's/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
```

```output
=== RUN   TestStageOneNoRevealWithGatedLayout
--- PASS: TestStageOneNoRevealWithGatedLayout (0.00s)
=== RUN   TestStartupVisibleTargetKeepsTopZero
--- PASS: TestStartupVisibleTargetKeepsTopZero (0.00s)
=== RUN   TestResizeBetweenLoadAndLayoutPreservesIntent
--- PASS: TestResizeBetweenLoadAndLayoutPreservesIntent (0.00s)
=== RUN   TestNavigationDuringPendingLayoutRevealsNewestTarget
--- PASS: TestNavigationDuringPendingLayoutRevealsNewestTarget (0.00s)
=== RUN   TestGutterGrowthBetweenStagesCommitsAtFinalWidth
--- PASS: TestGutterGrowthBetweenStagesCommitsAtFinalWidth (0.00s)
=== RUN   TestListToggleBetweenStagesCommitsAtFinalWidth
--- PASS: TestListToggleBetweenStagesCommitsAtFinalWidth (0.00s)
=== RUN   TestSavedViewportRevisitVisibleStays
--- PASS: TestSavedViewportRevisitVisibleStays (0.00s)
=== RUN   TestSavedViewportRevisitHiddenMoves
--- PASS: TestSavedViewportRevisitHiddenMoves (0.00s)
=== RUN   TestTerminatorOnlyMarkerRevealsInRunOffEdge
--- PASS: TestTerminatorOnlyMarkerRevealsInRunOffEdge (0.00s)
=== RUN   TestNonCurrentFileCompletionLeavesPanelUntouched
--- PASS: TestNonCurrentFileCompletionLeavesPanelUntouched (0.00s)
=== RUN   TestPopupUnaffectedByStages
--- PASS: TestPopupUnaffectedByStages (0.00s)
=== RUN   TestObsoleteLayoutDiscardedWithoutConsumingIntent
--- PASS: TestObsoleteLayoutDiscardedWithoutConsumingIntent (0.00s)
PASS
ok  	vrg/internal/app
```

## Reload-intent transition tests

The reload-intent transition tests (internal/app/reload_intent_test.go) verify that an explicit reload (r) sets the IntentReloadAnchor intent (preserve anchor, no reveal), that any navigation during the pending load replaces it with IntentReveal (latest selection takes precedence), that the intent is committed only when the matching prepared layout installs, that a stale reload layout does not consume the newer reveal intent, and that a navigation that occurs while loading still reveals the latest selected target, not the original target.

```bash
go test -count=1 -v ./internal/app/ -run '^TestReloadSetsAnchor|^TestNavigationDuringReload|^TestAwayAndBack|^TestReloadDoesNotReveal|^TestNewerNavigation|^TestStaleLoadCompletion|^TestReloadAnchorCommitted' -timeout 60s | sed 's/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
```

```output
=== RUN   TestReloadSetsAnchorIntent
--- PASS: TestReloadSetsAnchorIntent (0.00s)
=== RUN   TestNavigationDuringReloadReplacesIntent
--- PASS: TestNavigationDuringReloadReplacesIntent (0.00s)
=== RUN   TestAwayAndBackReplacesReloadIntent
--- PASS: TestAwayAndBackReplacesReloadIntent (0.00s)
=== RUN   TestReloadDoesNotRevealMatch
--- PASS: TestReloadDoesNotRevealMatch (0.00s)
=== RUN   TestNewerNavigationSupersedesReloadIntent
--- PASS: TestNewerNavigationSupersedesReloadIntent (0.00s)
=== RUN   TestStaleLoadCompletionDoesNotChangeIntent
--- PASS: TestStaleLoadCompletionDoesNotChangeIntent (0.00s)
=== RUN   TestReloadAnchorCommittedOnlyAfterMatchingLayout
--- PASS: TestReloadAnchorCommittedOnlyAfterMatchingLayout (0.00s)
PASS
ok  	vrg/internal/app
```

```bash
cp Notes/walkthroughs/028-06/code-walkthrough/demo_artifacts/manual_demo_test.go internal/app/manual_demo_test.go && go test -count=1 -v -tags manual_demo ./internal/app/ -run '^TestManualDemoTwoStageLoadCompletion$' -timeout 30s 2>&1 | sed 's/[[:space:]]0\.[0-9]*s//g; s/([0-9.]*s)/(0.00s)/g'; rm internal/app/manual_demo_test.go
```

```output
=== RUN   TestManualDemoTwoStageLoadCompletion
Step 1 (startup load): LoadIntent = IntentReveal (carried, not committed)
Step 2 (navigate during gated layout): LoadIntent = IntentReveal (latest selection)
Step 3 (release layout): ViewportOffset = 17 (reveal committed against matching rows)
Step 4 (press r): LoadIntent = IntentReloadAnchor (preserve anchor, no reveal)
Step 4 (reload completes): ViewportOffset = 17 (anchor preserved, no reveal)
Step 5 (navigate during reload): LoadIntent = IntentReveal (replaced reload intent)
Step 5 (release layout): ViewportOffset = 0 (reveal line 1, not anchor 17)

Manual demonstration passed: two-stage load-completion behavior matches the Issue #28 contracts.
--- PASS: TestManualDemoTwoStageLoadCompletion (0.00s)
PASS
ok  	vrg/internal/app
```
