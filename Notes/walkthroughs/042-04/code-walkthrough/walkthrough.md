# Issue #42: A dropped r request must not change revision or reveal intent

*2026-09-25T01:36:53Z by Showboat 0.6.1*
<!-- showboat-id: a92e3f4d-e3d7-4241-bf42-7fc680f15a56 -->

Walkthrough for [Issue #42](../../../tasks/042-dropped-reload-no-intent-mutation.md): a dropped `r` while the current path's load is in flight commits nothing — the request identity, the content revision, the pending reveal intent, and the presentation are all exactly as they were, so the in-flight startup or navigation load completes under its original classification (the destination reveal per `Notes/PRD-vrg.md` *File loading, cache, reload, and selection consistency* and *Navigation, viewport, and logical anchors*), never misclassified as a reload's anchor preservation with an extra revision bump. Admission is atomic inside `reload()`: the one-load-per-path check is the single decision point, evaluated before the reread mark, the buffer drop, the prior-failure overlay, and the identity mint — and navigation re-entry is deliberately ungated. All generated artifacts live in this directory: the built `vrg` binary, the `demo-dropped-r.sh` tmux harness, and its `manual-dropped-r/` captures. Test durations are stripped so the document verifies cleanly.

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

## Load-admission contract tests

`internal/app/admission_test.go` pins the Issue #42 boundary, driving the model with held load commands so the in-flight state is exact. `TestDroppedReloadDuringStartupLoadKeepsRevealIntent` and `TestDroppedReloadDuringNavigationLoadKeepsRevealIntent` assert a dropped `r` returns no command and leaves the in-flight request's identity, `loadSeq`, the absent `reloading` mark, `intentReveal`, the revision, and the frame untouched — the load then completes under its own classification (the line-200 target revealing at one-third placement rather than anchor-preserving at top 0). `TestAcceptedReloadAppliesReloadStateOnce` shows an admitted `r` landing the request, mark, and `Loading…` together — one worker, one revision increment, the anchor intent at completion. `TestRepeatedReloadKeepsSingleInFlightLoad` keeps one reread in flight under rapid presses with the placeholder → content / `(unreadable)` settlement. `TestNavigationReentryDuringInFlightLoadIsUngated` proves re-entry updates selection, placeholder, and `intentReveal` while its duplicate load drops — the atomic restriction applies only to the explicit reread.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestDroppedReload|TestAcceptedReload|TestRepeatedReload|TestNavigationReentry' ./internal/app 2>&1 | grep -vE '^(=== RUN|    --- PASS)' | sed -E 's/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//'
```

```output
--- PASS: TestDroppedReloadDuringStartupLoadKeepsRevealIntent
--- PASS: TestDroppedReloadDuringNavigationLoadKeepsRevealIntent
--- PASS: TestAcceptedReloadAppliesReloadStateOnce
--- PASS: TestRepeatedReloadKeepsSingleInFlightLoad
--- PASS: TestNavigationReentryDuringInFlightLoadIsUngated
PASS
ok  	vrg/internal/app
```

## Manual check — r dropped while a slow load is in flight

`demo-dropped-r.sh` (checked into this directory) runs the freshly built `vrg` on a real tmux PTY at 80×24. The slow file `slow.txt` is a FIFO: the load worker's `os.ReadFile` blocks until a writer closes, so the load is genuinely in flight for as long as the harness wants — a deterministic slow-read seam instead of gigabytes of fixture data. A fake `rg` emits one match record for the fifo's line 200 — hidden below the fold, so a first-visit anchor-keep (top 0) and the destination reveal (one-third placement, top 192) are observably different outcomes.

Run A covers the startup load: `slow.txt` alone, the startup load parks on the fifo, `r` is dropped with the pane byte-identical, and releasing the fifo reveals `hit00200` at the one-third position. Run B covers the navigation load: `fast.txt` settles, `n` crosses to `slow.txt` behind the file-change pop-up, a placeholder no-op `Up` dismisses the pop-up (the any-key contract, so the dropped-`r` comparison measures the panel alone), `r` is again dropped with the pane byte-identical, and the completion reveals the same match. Both runs exit 0.

```bash
cd /home/chris/vrg && go build -o Notes/walkthroughs/042-04/code-walkthrough/vrg ./cmd/vrg && cd Notes/walkthroughs/042-04/code-walkthrough && ./demo-dropped-r.sh
```

```output
ok: startup: dropped r left the pane unchanged
ok: startup: still Loading… after the dropped r
ok: startup: load completed to the revealed match
ok: startup: placeholder replaced by content
ok: startup: viewport moved to the match (reveal ran)
ok: navigation: dropped r left the pane unchanged
ok: navigation: still Loading… after the dropped r
ok: navigation: load completed to the revealed match
ok: navigation: placeholder replaced by content
ok: navigation: viewport moved to the match (reveal ran)
exit-startup=0
exit-navigation=0
--- startup: pane while the load was in flight (before and after r — identical) ---
slow.txt  ── slow.txt ──────────────────────────────────────────────────────────
             Loading…
--- startup: pane after the load completed (destination reveal) ---
slow.txt  ── slow.txt ──────────────────────────────────────────────────────────
          193  x000193
          194  x000194
          195  x000195
          196  x000196
          197  x000197
          198  x000198
          199  x000199
          200  hit00200
          201  x000201
          202  x000202
          203  x000203
--- navigation: pane while the load was in flight (before and after r — identical) ---
fast.txt  ── slow.txt ──────────────────────────────────────────────────────────
slow.txt     Loading…
--- navigation: pane after the load completed (destination reveal) ---
fast.txt  ── slow.txt ──────────────────────────────────────────────────────────
slow.txt  193  x000193
          194  x000194
          195  x000195
          196  x000196
          197  x000197
          198  x000198
          199  x000199
          200  hit00200
          201  x000201
          202  x000202
          203  x000203
```
