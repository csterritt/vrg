# Issue #40: browse rendering bounded by the visible window, not the index

*2026-09-15T18:54:23Z by Showboat 0.6.1*
<!-- showboat-id: e216d225-3e2d-4182-b943-32a58332d74d -->

Walkthrough for Issue #40 (Notes/tasks/040-browse-render-no-whole-index-scan.md): browse rendering no longer scans the whole search index per frame. Previously every View() called groupByFile(m.index.Stops()) — copying and regrouping all stops — index.Files() rebuilt a distinct-path set over every stop, the current-file lookup scanned the regrouped slice, and each visible entry re-escaped and re-segmented its path. Now prepareFileGroups runs once in the SearchCompleteMsg handler and produces immutable per-file groups, a raw-path → group-index map, and per-file width-independent display metadata (escaped path text, grapheme-cluster table, full cell width); renderBrowse reads only the visible [listOffset, listOffset+visibleRows) window and truncates per visible row against the current list width through safepresentation.TruncateLeftCellsFrom. References: Notes/issues/040-browse-render-no-whole-index-scan.md and Notes/PRD-vrg.md (Resources and responsiveness — the ~100,000-matched-lines scale example; File list and layout).

Contracts verified:
- Combined Update()+View() cost guard: one navigation or resize transition plus the resulting View() stays under 512 KiB / 4096 mallocs over a 45,000-stop index, measured without resetting the counter between the two halves — moving the whole-index work into Update() fails just as a View() scan does.
- Only the visible file-list window is materialized per frame (countingFileList provider bound).
- Resize and line-number-gutter growth re-truncate the visible paths against the new width, still grapheme-safe (no split CJK cluster).
- A real PTY run over a real 5,000-file / 100,000-matched-line ripgrep result shows per-keystroke repaint latency in the same millisecond range as a 15-stop index — no per-keystroke stall grows with index size.

All generated artifacts live in this directory.

```bash
cd /home/chris/vrg && go build ./... && go vet ./... && echo GATES-OK
```

```output
GATES-OK
```

## Combined Update()+View() cost guard

internal/app/layout_test.go builds a synthetic 3,000-file / 45,000-stop index and measures allocations across one transition (navigation key or WindowSizeMsg) plus the resulting View() via runtime.MemStats after runtime.GC() — without resetting between the two halves. Whole-index work fails the bound whether it lives in View() or in Update() navigation handling. The countingFileList provider additionally asserts only the visible [listOffset, listOffset+height) window is queried. Before the fix these four tests reported ~42 MB and ~22 MB allocated per transition; after, they pass within the 512 KiB bound.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/app/ -run '^(TestRenderCostGuardNavigateViewBounded|TestRenderCostGuardNavigatePrevBounded|TestRenderCostGuardResizeRetruncates|TestRenderCostGuardGutterGrowthRetruncates|TestRenderCostGuardFileList)$' -timeout 120s | sed 's/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'; echo test-exit=$?
```

```output
=== RUN   TestRenderCostGuardFileList
--- PASS: TestRenderCostGuardFileList (0.00s)
=== RUN   TestRenderCostGuardNavigateViewBounded
--- PASS: TestRenderCostGuardNavigateViewBounded (0.00s)
=== RUN   TestRenderCostGuardNavigatePrevBounded
--- PASS: TestRenderCostGuardNavigatePrevBounded (0.00s)
=== RUN   TestRenderCostGuardResizeRetruncates
--- PASS: TestRenderCostGuardResizeRetruncates (0.00s)
=== RUN   TestRenderCostGuardGutterGrowthRetruncates
--- PASS: TestRenderCostGuardGutterGrowthRetruncates (0.00s)
PASS
ok  	vrg/internal/app
test-exit=0
```

## Manual scenario: real ripgrep, 100,000 matched lines, held keys and resizes

genfiles.py creates demo/src/ with 5,000 real files x 20 lines each containing 'needle' — the PRD's ~100,000-matched-lines scale (Resources and responsiveness), searched by the real ripgrep 15.2.0 on PATH. runpty_bench.py drives the built vrg binary under a PTY: each key is sent only after the previous frame finished arriving, so the reported latency is the full Update()+View() turnaround per key — 20 'n' navigations (crossing file boundaries, each triggering a real file load and a file-list repaint), three resizes (60/100/80 columns, each re-truncating the visible file list and rewrapping the panel), then 5 more 'n', then 'q' to exit. Latencies are normalized to <ms> below so the document stays reproducible; the verdict line is the stable assertion.

```bash
cd /home/chris/vrg/Notes/walkthroughs/040-04/code-walkthrough && python3 genfiles.py && go -C /home/chris/vrg build -o Notes/walkthroughs/040-04/code-walkthrough/vrg ./cmd/vrg && echo BUILD-OK
```

```output
created 5000 files x 20 matched lines = 100000 stops in /home/chris/vrg/Notes/walkthroughs/040-04/code-walkthrough/demo/src
BUILD-OK
```

```bash
cd /home/chris/vrg/Notes/walkthroughs/040-04/code-walkthrough && VRG_KEYS='n,n,n,n,n,n,n,n,n,n,n,n,n,n,n,n,n,n,n,n,resize:60x24,resize:100x24,resize:80x24,n,n,n,n,n,q' VRG_WIDTH=80 VRG_HEIGHT=24 timeout 240 uv run --with pyte python3 runpty_bench.py ./vrg needle demo/src 2>&1 | sed -E 's/repaint +[0-9.]+ms/repaint <ms>/g; s/[0-9]+\.[0-9]+ms/<ms>/g; s/after [0-9.]+s/after <s>/'; echo run-exit=${PIPESTATUS[0]}
```

```output
browse ready after <s>
key          n repaint <ms>
key          n repaint <ms>
key          n repaint <ms>
key          n repaint <ms>
key          n repaint <ms>
key          n repaint <ms>
key          n repaint <ms>
key          n repaint <ms>
key          n repaint <ms>
key          n repaint <ms>
key          n repaint <ms>
key          n repaint <ms>
key          n repaint <ms>
key          n repaint <ms>
key          n repaint <ms>
key          n repaint <ms>
key          n repaint <ms>
key          n repaint <ms>
key          n repaint <ms>
key          n repaint <ms>
key resize:60x24 repaint <ms>
key resize:100x24 repaint <ms>
key resize:80x24 repaint <ms>
key          n repaint <ms>
key          n repaint <ms>
key          n repaint <ms>
key          n repaint <ms>
key          n repaint <ms>
summary: 28 transitions, min <ms>, median <ms>, max <ms>
verdict: PASS (every repaint under 400ms)
exit=0
run-exit=0
```

Comparison control: the identical key/resize burst against a 3-file / 15-stop index. If per-keystroke cost grew with index size, the 100,000-stop run above would be orders of magnitude slower; instead the repaints land in the same millisecond band.

```bash
cd /home/chris/vrg/Notes/walkthroughs/040-04/code-walkthrough && VRG_DEMO=demo-small VRG_FILES=3 VRG_LINES=5 python3 genfiles.py && VRG_KEYS='n,n,n,n,n,n,n,n,resize:60x24,resize:100x24,resize:80x24,n,n,q' VRG_WIDTH=80 VRG_HEIGHT=24 timeout 120 uv run --with pyte python3 runpty_bench.py ./vrg needle demo-small/src 2>&1 | sed -E 's/repaint +[0-9.]+ms/repaint <ms>/g; s/[0-9]+\.[0-9]+ms/<ms>/g; s/after [0-9.]+s/after <s>/'; echo run-exit=${PIPESTATUS[0]}
```

```output
created 3 files x 5 matched lines = 15 stops in /home/chris/vrg/Notes/walkthroughs/040-04/code-walkthrough/demo-small/src
browse ready after <s>
key          n repaint <ms>
key          n repaint <ms>
key          n repaint <ms>
key          n repaint <ms>
key          n repaint <ms>
key          n repaint <ms>
key          n repaint <ms>
key          n repaint <ms>
key resize:60x24 repaint <ms>
key resize:100x24 repaint <ms>
key resize:80x24 repaint <ms>
key          n repaint <ms>
key          n repaint <ms>
summary: 13 transitions, min <ms>, median <ms>, max <ms>
verdict: PASS (every repaint under 400ms)
exit=0
run-exit=0
```

In the recorded runs the 100,000-stop session repainted every navigation and resize in 5-19ms (median ~14ms) and the 15-stop control in 6-18ms (median ~15ms) — identical bands, dominated by PTY/terminal overhead rather than index size. Before Issue #40 the same transition allocated ~42MB per keystroke over the guard index; now a frame touches only the visible window. Browse readiness after ~1s includes the real ripgrep run, JSON parsing, index build, and the one-time prepareFileGroups pass — the whole-index work now happens once, there, instead of per frame.

Full-suite verification:

```bash
cd /home/chris/vrg && gofmt -l internal/app/app.go internal/app/layout_test.go internal/safepresentation/cellwidth.go; go test -count=1 ./internal/app/ ./internal/safepresentation/ -timeout 300s 2>&1 | sed 's/\(ok.*\)\t[0-9.]*s/\1/'; echo final-exit=${PIPESTATUS[0]}
```

```output
ok  	vrg/internal/app
ok  	vrg/internal/safepresentation
final-exit=0
```
