# Issue #28: Load completion reveals the latest selected target

*2026-09-24T17:49:37Z by Showboat 0.6.1*
<!-- showboat-id: 8bbd9cde-2ef5-46bb-9e2b-a7fb42c86765 -->

Walkthrough for [Issue #28](../../../issues/028-load-completion-reveal-latest-target.md), implementing the two-stage load-completion contract per `Notes/PRD-vrg.md` (*File loading, cache, reload, and selection consistency*; *Navigation, viewport, and logical anchors*): a current-file load completion validates the buffer, establishes the new content revision, computes the final gutter and text width (list visibility participating), and requests a prepared layout keyed to the post-load (path, revision, text width, wrap mode) — performing no row-based decision itself. The reveal intent lives in the model and commits only when a matching layout installs: it targets the newest selected stop's final display target (the cluster-expanded first-submatch start cell, or the marker cell for a zero-width/terminator-only match), never a target captured at request time; obsolete layouts are discarded without consuming it; a reload commits the retained anchor only when no navigation intervened — navigation during the load, including away-and-back to the same cursor, replaces the anchor intent with the entry reveal. All generated artifacts live in this directory: the built `vrg` binary, `fixture-src/` fixtures, the `demo-load-completion.sh` tmux harness, and its `completion/` session captures. Test durations are stripped so the document verifies cleanly.

```bash
set -o pipefail; cd /home/chris/vrg && test -z "$(gofmt -l internal/ cmd/)" && echo GOFMT-CLEAN && go vet ./... && go build ./... && go test -count=1 ./... | sed "s/[[:space:]][0-9.]*s$//" && CGO_ENABLED=1 go test -race -count=1 ./internal/app | sed "s/[[:space:]][0-9.]*s$//" && go build -o Notes/walkthroughs/028-06/code-walkthrough/vrg ./cmd/vrg && echo GATES-OK
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
GATES-OK
```

## Gated two-stage tests — `internal/app/completion_test.go`

Each stage is driven separately: the load command's message goes through `Update`, while the returned layout command is held and invoked on the test's schedule — so the gap between the stages is observable. `TestLoadCompletionMakesNoRowDecision` pins stage 1: buffer installed, revision bumped, gutter grown, text width recomputed, layout requested with the new key — while top, anchor, offset, and the pending intent are all untouched; the commit lands only when the matching layout installs. `TestNavigationDuringLayoutGapCommitsNewestTarget` moves the cursor with `n`/`p` during the gap and proves the commit reveals the newest selection, not the one current at load completion. `TestResizeBetweenStagesCommitsAtNewWidth` and `TestListToggleBetweenStagesCommitsAtFinalWidth` supersede the in-flight request mid-gap — a wrap-changing first line makes the committed width observable — while `TestObsoleteLayoutsNeverConsumeTheIntent` fires wrong-width/mode/revision/path completions and proves the intent and frame survive. `TestStartupHiddenTargetCommitsOnInstall`/`TestStartupVisibleTargetKeepsTopZero` pin the one-third-versus-no-scroll startup rule plus the first `n`; `TestSavedViewportRevisitCommitsOnLoad`/`TestSavedViewportRevisitHiddenTargetMoves` cover revisits starting from the saved anchor; `TestMarkerTargetCommitPaintsMarkerCell` and `TestClusterTargetCommitPaintsWholeCluster` commit the Issue #23 marker cell and Issue #21 cluster-expanded cell in run-off-edge mode; `TestNonCurrentCompletionLeavesPanelUntouched` and `TestPopupUnaffectedByCompletionStages` pin the isolation contracts.

```bash
cd /home/chris/vrg && go test -count=1 -v -run "TestLoadCompletionMakesNoRowDecision|TestNavigationDuringLayoutGapCommitsNewestTarget|TestResizeBetweenStagesCommitsAtNewWidth|TestListToggleBetweenStagesCommitsAtFinalWidth|TestObsoleteLayoutsNeverConsumeTheIntent|TestStartupHiddenTargetCommitsOnInstall|TestStartupVisibleTargetKeepsTopZero|TestSavedViewportRevisitCommitsOnLoad|TestSavedViewportRevisitHiddenTargetMoves|TestMarkerTargetCommitPaintsMarkerCell|TestClusterTargetCommitPaintsWholeCluster|TestNonCurrentCompletionLeavesPanelUntouched|TestPopupUnaffectedByCompletionStages" ./internal/app 2>&1 | grep -vE "^(=== RUN|=== CONT)" | sed -E "s/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//"
```

```output
--- PASS: TestLoadCompletionMakesNoRowDecision
--- PASS: TestNavigationDuringLayoutGapCommitsNewestTarget
--- PASS: TestResizeBetweenStagesCommitsAtNewWidth
--- PASS: TestListToggleBetweenStagesCommitsAtFinalWidth
--- PASS: TestObsoleteLayoutsNeverConsumeTheIntent
--- PASS: TestStartupHiddenTargetCommitsOnInstall
--- PASS: TestStartupVisibleTargetKeepsTopZero
--- PASS: TestSavedViewportRevisitCommitsOnLoad
--- PASS: TestSavedViewportRevisitHiddenTargetMoves
--- PASS: TestMarkerTargetCommitPaintsMarkerCell
--- PASS: TestClusterTargetCommitPaintsWholeCluster
--- PASS: TestNonCurrentCompletionLeavesPanelUntouched
--- PASS: TestPopupUnaffectedByCompletionStages
PASS
ok  	vrg/internal/app
```

## Reload-intent transitions — `internal/app/reload_test.go`

The Issue #27 seam generalizes: a reload completion records `intentAnchor` only when no reveal is already carried, so navigation during the load — any navigation, including away-and-back sequences that end on the starting cursor — leaves the reveal intent standing and wins the commit. `TestReloadAnchorIntentSurvivesTheLayoutGap` pins the undisturbed path (anchor intent held through the layout gap, committed with no reveal); `TestReloadSameFileAwayAndBackCommitsEntryReveal` and `TestReloadCrossFileAwayAndBackCommitsEntryReveal` prove navigation intent beats cursor equality — the cross-file variant also shows `Loading…` on return with the saved anchor dormant; `TestReloadLateOldRevisionLayoutIsInert` and `TestReloadLateOldRevisionLayoutAfterNavigation` deliver the new revision's layout first and the old revision's late, proving the commit happens once, against the new revision, and the straggler consumes nothing.

```bash
cd /home/chris/vrg && go test -count=1 -v -run "TestReloadAnchorIntentSurvivesTheLayoutGap|TestReloadSameFileAwayAndBackCommitsEntryReveal|TestReloadCrossFileAwayAndBackCommitsEntryReveal|TestReloadLateOldRevisionLayoutIsInert|TestReloadLateOldRevisionLayoutAfterNavigation" ./internal/app 2>&1 | grep -vE "^(=== RUN|=== CONT)" | sed -E "s/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//"
```

```output
--- PASS: TestReloadAnchorIntentSurvivesTheLayoutGap
--- PASS: TestReloadSameFileAwayAndBackCommitsEntryReveal
--- PASS: TestReloadCrossFileAwayAndBackCommitsEntryReveal
--- PASS: TestReloadLateOldRevisionLayoutIsInert
--- PASS: TestReloadLateOldRevisionLayoutAfterNavigation
PASS
ok  	vrg/internal/app
```

## Manual route — the real binary on a PTY

`demo-load-completion.sh` (checked into this directory) runs the freshly built `vrg` on real tmux PTYs with real `rg`, in four sessions over disposable `mktemp` directories — repository and user files are never touched, and an EXIT trap removes them.

A slow startup load needs no test seam: `big.txt` is generated with ~1.5M lines (~30MB), so the read+prepare phase takes seconds and `Loading…` is painted and held for the whole window — long enough to capture it and to resize the terminal mid-load. For the reload cases the settled file is swapped for a writerless FIFO while vrg is idle: the reread's `open()` blocks until the harness pulses a writer, a deterministic hold.

- **Session A** — slow startup file (MARK stops at lines 500 and 550): `Loading…` paints while the load runs; on completion the line-500 match lands a third down (content row 7 of 23, top 492), and the first `n` reveals the second stop (top 542).
- **Session B** — the same file at 80x24 with the window resized to 80x40 while `Loading…` still holds: the reveal commits against the *new* size — line 500 lands 13 down the 39-row content area (top 486), not the stale 24-row geometry's 492.
- **Session C** — `near.txt` with its only match at line 3: the visible startup target keeps top 0, and `n` on the one-stop file is a no-op.
- **Session D** — `pair.txt` (stops at lines 5 and 200): the file is swapped for a FIFO, `r` holds on `Loading…`, an immediate `n` moves the cursor while the panel waits, and the pulse's commit reveals the *new* stop (top 192). Then with the match scrolled off-screen, `r` + `n` + `p` — away-and-back ending on the same cursor — still reveals on completion: navigation intent, not cursor equality, decides.

```bash
cd /home/chris/vrg/Notes/walkthroughs/028-06/code-walkthrough && unset RIPGREP_CONFIG_PATH && ./demo-load-completion.sh
```

```output
ok: startup: Loading… while the file loads
ok: startup: filename rule names the path
ok: hidden startup target lands a third down (capture row 9) -> 9
ok: startup: top row -> line-0000493
ok: first n reveals the second stop (capture row 9) -> 9
ok: first n: top row -> line-0000543
ok: session A: vrg exit status -> 0
ok: resize case: Loading… while the load runs
ok: resize during the load: still Loading…
ok: resize during the load: pane is 40 rows -> 40
ok: resize case: target a third down the NEW height (capture row 15) -> 15
ok: resize case: top row -> line-0000487
ok: session B: vrg exit status -> 0
ok: visible startup target: top stays at line 1 -> line-001
ok: visible startup target: match in place (capture row 4) -> 4
ok: visible target: n on the one-stop file is a no-op -> line-001
ok: session C: vrg exit status -> 0
ok: startup: pair.txt at top -> line-001
ok: r: Loading… while the reread blocks
ok: n during the reload: still Loading… (cursor moved, commit pending)
ok: reload + n: the NEW stop is revealed (capture row 9) -> 9
ok: reload + n: top row -> line-193
ok: match scrolled off-screen before r -> line-170
ok: scrolled-off match is hidden
ok: away-and-back during the reload: still Loading…
ok: r + n + p: the entry reveal lands (capture row 9) -> 9
ok: r + n + p: top row is the reveal's, not the anchor's 170 -> line-193
ok: session D: vrg exit status -> 0
demo-load-completion: all checks passed
```

The deterministic model tests remain the authority for the staged-commit and intent-arbitration semantics; the PTY route demonstrates the visible behavior end to end — the placeholder holding during each stage, the reveal landing only on install, and the reload cases where the newest navigation intent decides. Wiki ingest lives in `Notes/wiki/load-completion-reveal.md`.
