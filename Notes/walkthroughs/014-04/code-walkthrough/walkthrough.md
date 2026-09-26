# Issue #14: Vertical destination reveal

*2026-09-23T22:12:34Z by Showboat 0.6.1*
<!-- showboat-id: 2693a9e0-7eaa-4231-8864-45fe9341c15e -->

Walkthrough for [Issue #14](../../../issues/014-vertical-destination-reveal.md), implementing the vertical destination reveal per `Notes/PRD-vrg.md` (*Navigation, viewport, and logical anchors* — the target-row and placement bullets, and *Testing Decisions → Viewport*). The reveal target is a display location — `viewport.Target{Line, Cell}` carrying the zero-based source line and the display cell of the destination line's first submatch start (the marker cell for a zero-width match) — resolved to its rendered row through the new `Rows.RowOf` provider method, the seam Issue #16's wrap mode will consume. `Viewport.Reveal` leaves an already-visible target untouched, lands a hidden target on zero-based content row `floor(height/3)` in either direction, and lets the BOF/EOF clamps take precedence over exact placement; its bool report tells the App whether to replace the file's saved top. The triggers are the end of every actual `n`/`p` transition and the `loadDoneMsg` completion for the current path — the latter covering startup, where the first visit begins at the top of the file before the reveal. Horizontal reveal remains Issue #19's. All generated artifacts live in this directory: the built `vrg` binary, the `demo-reveal.sh` tmux harness, and its `reveal/` session captures. Test durations are stripped so the document verifies cleanly.

```bash
cd /home/chris/vrg && go vet ./... && go build ./... && go test -count=1 ./... | sed "s/[[:space:]][0-9.]*s$//" && echo GATES-OK
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

## Viewport reveal tests

`internal/viewport/reveal_test.go` pins the placement contract against the counting-rows fake plus `mappingRows`, a provider whose `RowOf` is programmable and records every target it sees. `TestRevealVisibleTargetDoesNotScroll` proves a target on the first, middle, or last visible row never scrolls; `TestRevealHiddenTargetLandsOneThirdDown` lands a hidden target — above or below — on row `floor(height/3)`; `TestRevealTargetNearBOFClampsToTop` and `TestRevealTargetNearEOFClampsToLastPage` prove content availability beats exact placement at both ends; `TestRevealUsesRenderedRowContainingTarget` drives a wrap-like many-to-one `RowOf` and confirms the provider is asked about the exact `(line, cell)` target — the reveal consumes a rendered row, not a line ordinal; `TestRevealAfterSavedAndTopStartingPoints` covers the entry sequence (saved top plus visible target survives, saved top plus hidden target moves, first visit starts at 0 then reveals); `TestRevealClampsOutOfRangeTarget` clamps a resolving-outside target to the nearest real row; and `TestRevealWithoutContentIsNoOp` keeps nil and zero-row providers inert.

```bash
cd /home/chris/vrg && go test -count=1 -v -run TestReveal ./internal/viewport 2>&1 | grep -vE "^(=== RUN|=== CONT|    --- (PASS|FAIL))" | sed -E "s/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//"
```

```output
--- PASS: TestRevealVisibleTargetDoesNotScroll
--- PASS: TestRevealHiddenTargetLandsOneThirdDown
--- PASS: TestRevealTargetNearBOFClampsToTop
--- PASS: TestRevealTargetNearEOFClampsToLastPage
--- PASS: TestRevealUsesRenderedRowContainingTarget
--- PASS: TestRevealAfterSavedAndTopStartingPoints
--- PASS: TestRevealClampsOutOfRangeTarget
--- PASS: TestRevealWithoutContentIsNoOp
PASS
ok  	vrg/internal/viewport
```

## App trigger tests

`internal/app/reveal_test.go` drives the reveal through `Update` on a `fileWithStops` fixture. The startup-reveal group — `TestStartupRevealPlacesHiddenTargetOneThirdDown`, `TestStartupRevealVisibleTargetDoesNotScroll`, and `TestStartupRevealAppliesOnLoadCompletion` — proves the load-completion trigger: a hidden destination match lands on content row `floor(23/3)` = 7 (top 192) once the buffer arrives, a gate-held load keeps the `Loading…` placeholder at top 0 until then, and an on-screen startup target writes no saved state. The navigation group — `TestNPRevealHiddenTargets`, `TestNToVisibleTargetDoesNotScroll`, `TestRevisitStartsFromSavedViewport`, `TestRevisitRevealOverridesHiddenSavedViewport`, and `TestFirstVisitStartsAtTopThenReveals` — proves the `n`/`p` trigger: hidden targets land a third down in both directions (BOF clamped), on-screen targets never scroll, a revisit resumes the saved top when it shows the target and is overridden when it hides it, and a first visit to an uncached file starts at the top before its load-completion reveal. A moving reveal always replaces `saved`; a no-scroll reveal never touches it.

```bash
cd /home/chris/vrg && go test -count=1 -v -run "TestStartupReveal|TestNPReveal|TestNToVisibleTarget|TestRevisit|TestFirstVisit" ./internal/app 2>&1 | grep -vE "^(=== RUN|=== CONT|    --- (PASS|FAIL))" | sed -E "s/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//"
```

```output
--- PASS: TestStartupRevealPlacesHiddenTargetOneThirdDown
--- PASS: TestStartupRevealVisibleTargetDoesNotScroll
--- PASS: TestStartupRevealAppliesOnLoadCompletion
--- PASS: TestNPRevealHiddenTargets
--- PASS: TestNToVisibleTargetDoesNotScroll
--- PASS: TestRevisitStartsFromSavedViewport
--- PASS: TestRevisitRevealOverridesHiddenSavedViewport
--- PASS: TestFirstVisitStartsAtTopThenReveals
PASS
ok  	vrg/internal/app
```

## Manual check — n/p reveal on a real PTY

`demo-reveal.sh` (checked into this directory) runs the freshly built `vrg` on a real tmux PTY at 80x24 — content height 23, so the one-third row is 7. Its `fakebin/rg` reports three stops in one 300-line `long.txt` — matches at lines 5, 200, and 210 — so a single file exercises every rule: the startup file opens at the top with line 5 already visible (a no-scroll reveal); `n` to the hidden line-200 target lands it on content row 7 (top 192, first content row `x000193`); `n` onward to line 210 finds it already inside the window and moves only the underline; `p` back to line 200 stays put; and `p` to line 5 is BOF-clamped — the reveal wants top `4 − 7 < 0`, so the file's first row returns with the match near the top rather than a third down. The script detects the current match via the underline SGR and asserts each pane row; plain-text captures are stored under `reveal/`.

```bash
cd /home/chris/vrg/Notes/walkthroughs/014-04/code-walkthrough && ./demo-reveal.sh
```

```output
ok: startup: file panel -> long.txt
ok: startup: current match -> hit 00005
ok: startup: top of file -> x000001
ok: startup: match row -> hit 00005
ok: n hidden target: current match -> hit 00200
ok: n hidden target: first content row -> x000193
ok: n hidden target: match one-third down -> hit 00200
ok: n on-screen target: current match -> hit 00210
ok: n on-screen target: viewport unmoved -> x000193
ok: p on-screen target: current match -> hit 00200
ok: p on-screen target: viewport unmoved -> x000193
ok: p to BOF: current match -> hit 00005
ok: p to BOF: first content row -> x000001
ok: p to BOF: match near top -> hit 00005
ok: exit status -> 0
demo-reveal: all checks passed
```

```bash
cd /home/chris/vrg/Notes/walkthroughs/014-04/code-walkthrough && echo '--- screen after n: line 200 revealed a third down ---' && head -10 reveal/screen-02-n-hidden.txt && echo '--- screen after p to line 5: BOF clamp back to the top ---' && head -7 reveal/screen-05-p-bof.txt
```

```output
--- screen after n: line 200 revealed a third down ---
long.txt  ── long.txt ──────────────────────────────────────────────────────────
          193  x000193
          194  x000194
          195  x000195
          196  x000196
          197  x000197
          198  x000198
          199  x000199
          200  hit 00200
          201  x000201
--- screen after p to line 5: BOF clamp back to the top ---
long.txt  ── long.txt ──────────────────────────────────────────────────────────
            1  x000001
            2  x000002
            3  x000003
            4  x000004
            5  hit 00005
            6  x000006
```

## Verdict

Issue #14 is verified. `Viewport.Reveal` resolves the `Target{Line, Cell}` — the destination line's first submatch start cell — through `Rows.RowOf` to its rendered row, leaves the top untouched when that row is visible, lands it on `floor(height/3)` when hidden, and clamps through the existing BOF/EOF bounds so content wins over exact placement. The App triggers the reveal on every actual `n`/`p` transition and on `loadDoneMsg` for the current path — the startup-after-load reveal — always over the saved-or-top starting point, replacing `saved` only when the viewport actually moved. The tmux run shows the whole contract on a real PTY: line 200 lands exactly one-third down, the on-screen `n` to line 210 scrolls nothing, and `p` to line 5 is clamped back to the top of the file.
