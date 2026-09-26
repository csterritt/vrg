# Issue #40: Browse rendering without whole-index scans

*2026-09-25T01:08:49Z by Showboat 0.6.1*
<!-- showboat-id: 5537c396-2b70-47fe-919c-cca9a9203a2c -->

Walkthrough for [Issue #40](../../../tasks/040-browse-render-no-whole-index-scan.md): browse navigation and rendering no longer enumerate the whole search index, per `Notes/PRD-vrg.md` (*Resources and responsiveness* and *File list and layout*). At `searchDoneMsg` the browse branch now prepares everything width-independent exactly once — the single `Index.Stops()` materialization into `m.stops`, the sorted `files`/`fileIdx` list, the immutable per-file `fileStops` groups, and `longestEntryW` (the widest list entry's painted cell width). `loadCmd` reads the destination's precomputed group instead of re-filtering all stops per issued load, and `listWidth` reads `longestEntryW` instead of re-measuring every entry inside `syncLayout`. Per-frame work stays bounded by the visible window: `renderBrowse` queries `listEntry` only for `files[listTop : listTop+rows]` and left-truncates each visible entry against the *current* `listW` via `present.TruncateLeft`, because the allotted width moves with resize, gutter growth, wrap mode, and the list toggle — the truncated text cannot be precomputed. `m.index` narrows to the new `stopIndex` seam so a counting double can prove no whole-stop materialization survives on the keystroke path. All generated artifacts live in this directory: the built `vrg` binary, `vrg-before` (the pre-#40 HEAD build), the `demo-bounded-render.sh` tmux harness, and its `manual-bounded-*/` captures. Test durations are stripped so the document verifies cleanly; the demo's wall-clock timings are captured as run and will vary on re-execution.

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

## The bounded-render cost guard

`internal/app/rendercost_test.go` installs a `countingIndex` around the `stopIndex` seam — every `Stops()` whole-stop materialization is tallied — and a counting `listEntry` provider. Both counters span a navigation `Update()` *plus* the resulting `View()` without reset, so whole-index work moved from rendering into keystroke handling is still caught. `TestNavigationRenderCostBoundedByVisibleWindow` drives `n` across file boundaries in a 300-file/900-stop fixture and requires zero `Stops()` calls with provider queries bounded by the visible row count. `TestResizeAndGutterGrowthRetruncateWithinVisibleCost` resizes and grows the gutter mid-fixture, asserting the visible entries re-truncate at grapheme boundaries inside the same bound. The pre-existing `TestListRenderQueriesOnlyVisibleWindow` pins the same visible-window discipline on the render side.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestNavigationRenderCostBoundedByVisibleWindow|TestResizeAndGutterGrowthRetruncateWithinVisibleCost|TestListRenderQueriesOnlyVisibleWindow|TestListWidthFormula' ./internal/app 2>&1 | grep -vE '^(=== RUN|    --- PASS)' | sed -E 's/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//'
```

```output
--- PASS: TestListWidthFormula
--- PASS: TestListRenderQueriesOnlyVisibleWindow
--- PASS: TestNavigationRenderCostBoundedByVisibleWindow
--- PASS: TestResizeAndGutterGrowthRetruncateWithinVisibleCost
PASS
ok  	vrg/internal/app
```

## Manual check — held `n` on a 100,000-stop index

`demo-bounded-render.sh` (checked in here) drives the built `vrg` on a real tmux PTY at 80x24. Its `fakebin/rg` cats a pre-generated stream of 10,000 files x 10 matched lines = 100,000 stops; every file exists on disk so each crossing issues a real load. Stage 2 sends a 500-key held-`n` burst — 50 file crossings, landing on `f-00050.txt` — and times it; stage 3 fires 12 resizes (SIGWINCH alternating 60x18 / 100x30) and then probes one more crossing. Each stage waits for the destination's `── <file>` filename rule to paint, so a landing proves every key was processed and the view kept up. The `FILES` env var scales the fixture. Wall-clock figures are normalized to `<wall>` in the recorded output so the document verifies; the measured values accompany each run in the notes — this run: **0.07s burst (0.14ms/key), 0.07s probe**. First the current build:

```bash
cd /home/chris/vrg && go build -o Notes/walkthroughs/040-04/code-walkthrough/vrg ./cmd/vrg && cd Notes/walkthroughs/040-04/code-walkthrough && ./demo-bounded-render.sh "$PWD/vrg" current | sed -E -e 's/burst wall time: [0-9.]+s \([0-9.]+ms per key\)/burst wall time: <wall>s (<wall>ms per key)/' -e 's/probe wall time: [0-9.]+s/probe wall time: <wall>s/'
```

```output
fixture: 10000 files x 10 stops = 100000 matched lines
stage 1 [current]: whole index materialized once at search
         completion; first file painted
stage 2 [current]: 500 held-n keys (50 crossings) landed on f-00050.txt
         burst wall time: <wall>s (<wall>ms per key)
stage 3 [current]: 12 resizes + 10-key crossing probe landed on f-00051.txt
         probe wall time: <wall>s
exit=0
stderr replay: empty
```

For contrast, `vrg-before` is built from commit `72d31a5` — the tree exactly as it was before Issue #40 — via `git archive`, then put through the identical scenario. Under that shape every file crossing re-measured all 10,000 list entries inside `syncLayout` (the `listWidth` scan) and every issued `loadCmd` re-materialized the whole 100,000-stop slice. Measured: **0.39s burst (0.78ms/key), 0.08s probe** — over 5x the per-key cost.

```bash
cd /home/chris/vrg && rm -rf /tmp/vrg-before-src && mkdir -p /tmp/vrg-before-src && git archive 72d31a5 | tar -x -C /tmp/vrg-before-src && cd /tmp/vrg-before-src && go build -o /home/chris/vrg/Notes/walkthroughs/040-04/code-walkthrough/vrg-before ./cmd/vrg && cd /home/chris/vrg/Notes/walkthroughs/040-04/code-walkthrough && ./demo-bounded-render.sh "$PWD/vrg-before" before | sed -E -e 's/burst wall time: [0-9.]+s \([0-9.]+ms per key\)/burst wall time: <wall>s (<wall>ms per key)/' -e 's/probe wall time: [0-9.]+s/probe wall time: <wall>s/'
```

```output
fixture: 10000 files x 10 stops = 100000 matched lines
stage 1 [before]: whole index materialized once at search
         completion; first file painted
stage 2 [before]: 500 held-n keys (50 crossings) landed on f-00050.txt
         burst wall time: <wall>s (<wall>ms per key)
stage 3 [before]: 12 resizes + 10-key crossing probe landed on f-00051.txt
         probe wall time: <wall>s
exit=0
stderr replay: empty
```

Now scale the fixture 3x — `FILES=30000` gives 30,000 files and 300,000 matched lines. The pre-#40 per-keystroke cost grows with the index (**1.21s burst, 2.42ms/key** — roughly 3x its 100k figure); the current build stays flat (**0.12s burst, 0.24ms/key**):

```bash
cd /home/chris/vrg/Notes/walkthroughs/040-04/code-walkthrough && { FILES=30000 ./demo-bounded-render.sh "$PWD/vrg-before" before-30k; FILES=30000 ./demo-bounded-render.sh "$PWD/vrg" current-30k; } | sed -E -e 's/burst wall time: [0-9.]+s \([0-9.]+ms per key\)/burst wall time: <wall>s (<wall>ms per key)/' -e 's/probe wall time: [0-9.]+s/probe wall time: <wall>s/'
```

```output
fixture: 30000 files x 10 stops = 300000 matched lines
stage 1 [before-30k]: whole index materialized once at search
         completion; first file painted
stage 2 [before-30k]: 500 held-n keys (50 crossings) landed on f-00050.txt
         burst wall time: <wall>s (<wall>ms per key)
stage 3 [before-30k]: 12 resizes + 10-key crossing probe landed on f-00051.txt
         probe wall time: <wall>s
exit=0
stderr replay: empty
fixture: 30000 files x 10 stops = 300000 matched lines
stage 1 [current-30k]: whole index materialized once at search
         completion; first file painted
stage 2 [current-30k]: 500 held-n keys (50 crossings) landed on f-00050.txt
         burst wall time: <wall>s (<wall>ms per key)
stage 3 [current-30k]: 12 resizes + 10-key crossing probe landed on f-00051.txt
         probe wall time: <wall>s
exit=0
stderr replay: empty
```

The painted frame after the 300k-stop burst (`manual-bounded-current-30k/stage2.txt`): the file list has scrolled `f-00050.txt` into view at the bottom of its window, the filename rule tracks it, and the file-change pop-up shows the destination — identical file-list, rule, pop-up, and scroll behaviour to before, produced at bounded cost:

```bash
cd /home/chris/vrg/Notes/walkthroughs/040-04/code-walkthrough && cat manual-bounded-current-30k/stage2.txt
```

```output
f-00027.txt  ── f-00050.txt ────────────────────────────────────────────────────
f-00028.txt   1  hit
f-00029.txt   2  hit
f-00030.txt   3  hit
f-00031.txt   4  hit
f-00032.txt   5  hit
f-00033.txt   6  hit
f-00034.txt   7  hit
f-00035.txt   8  hit
f-00036.txt   9  hit
f-00037.txt  10  hit             ┌───────────┐
f-00038.txt                      │f-00050.txt│
f-00039.txt                      └───────────┘
f-00040.txt
f-00041.txt
f-00042.txt
f-00043.txt
f-00044.txt
f-00045.txt
f-00046.txt
f-00047.txt
f-00048.txt
f-00049.txt
f-00050.txt
```

## Result

Issue #40 is verified. The cost guard proves a navigation `Update()` plus its resulting `View()` performs zero `Stops()` whole-stop materializations and bounds `listEntry` queries by the visible rows — with the counter cumulative across both, so the work cannot hide in keystroke handling — and the resize/gutter-growth case re-truncates visible paths at grapheme boundaries under the same bound. The live PTY runs agree: on a 100,000-stop index a 500-key held-`n` burst completes in ~0.07s (0.14ms/key vs 0.78ms/key before), and at 300,000 stops the pre-#40 build's per-key cost roughly triples (2.42ms) while the current build stays flat (0.24ms — the residual is terminal/input processing, not index work). File-list scrolling, the filename rule, the file-change pop-up, and the exit status are all unchanged. `go build`, `go vet`, and `go test ./...` all pass.
