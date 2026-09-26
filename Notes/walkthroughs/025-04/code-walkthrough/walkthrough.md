# Issue #25: Asynchronous load isolation — keyed completions and live navigation

*2026-09-24T15:37:31Z by Showboat 0.6.1*
<!-- showboat-id: 3be99e97-9ae4-493f-890a-8c01589e80dc -->

Walkthrough for [Issue #25](../../../issues/025-async-load-isolation.md), implementing the asynchronous load-isolation contract per `Notes/PRD-vrg.md` (*File loading, cache, reload, and selection consistency* — the first three bullets; *Resources and responsiveness*): navigation stays fully active while a file loads, load completions are keyed by raw path **and** a minted request identity so a late result updates only its own file, at most one load is in flight per path with re-entry dropped not queued, successful buffers are retained for the session with no eviction, post-cancellation completions are discarded, and the decode/map phase is separately gatable with `ctrl+c`, `n`, `p`, `w`, `c`, and resizes still actionable while it is held. All generated artifacts live in this directory: the built `vrg` binary, the `demo-load.sh` tmux harness, and its `load/` session captures. Test durations are stripped so the document verifies cleanly.

```bash
set -o pipefail; cd /home/chris/vrg && test -z "$(gofmt -l internal/)" && echo GOFMT-CLEAN && go vet ./... && go build ./... && go test -count=1 ./... | sed "s/[[:space:]][0-9.]*s$//" && CGO_ENABLED=1 go test -race -count=1 ./internal/app ./internal/filebuffer | sed "s/[[:space:]][0-9.]*s$//" && go build -o Notes/walkthroughs/025-04/code-walkthrough/vrg ./cmd/vrg && echo GATES-OK
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
ok  	vrg/internal/filebuffer
GATES-OK
```

## Gated model tests — `internal/app/load_test.go`

The isolation contracts are pinned at the model seam with two test gates: `loadGate` parks the whole load worker before its read, and `mapGate` parks only the decode/map phase after the read has finished. `gatedModel` wires the seams and returns the startup file's load command uninvoked; `mintLoad` registers an in-flight request identity so a test can inject a completion whose real worker never ran. `TestNavigationActiveWhileLoadInFlight` navigates past a gate-held load — `n` crosses to B at once, `p` returns to the still-loading A — and the original request installs on release. `TestPlaceholderScrollAndPanAreNoOp` pins the placeholder no-ops and the still-normal keys. `TestReEnterLoadingPathIsDroppedNotQueued` proves one in-flight load per path. `TestLateCompletionIsolatedToItsPath` runs A→B→C with late completions for the departed files leaving C's frame byte-identical, then proves the cached revisits issue no new load. `TestLoadCompletionKeyedByRequestIdentity` drops stale and unsolicited completions. `TestLoadCompletionAfterQuitDiscarded` covers post-cancellation rejection. `TestDecodeMapGateKeepsInputsActionable` holds decode/map while resize, `w`, `c`, `n`, `p` all apply. `TestMapGateHoldsDecodeMapNotRead` proves the gate sits after the read — a missing file fails promptly.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestNavigationActiveWhileLoadInFlight|TestPlaceholderScrollAndPanAreNoOp|TestReEnterLoadingPathIsDroppedNotQueued|TestLateCompletionIsolatedToItsPath|TestLoadCompletionKeyedByRequestIdentity|TestLoadCompletionAfterQuitDiscarded|TestDecodeMapGateKeepsInputsActionable|TestMapGateHoldsDecodeMapNotRead' ./internal/app 2>&1 | grep -vE '^(=== RUN|=== CONT)' | sed -E 's/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//'
```

```output
--- PASS: TestNavigationActiveWhileLoadInFlight
--- PASS: TestPlaceholderScrollAndPanAreNoOp
--- PASS: TestReEnterLoadingPathIsDroppedNotQueued
--- PASS: TestLateCompletionIsolatedToItsPath
--- PASS: TestLoadCompletionKeyedByRequestIdentity
--- PASS: TestLoadCompletionAfterQuitDiscarded
--- PASS: TestDecodeMapGateKeepsInputsActionable
--- PASS: TestMapGateHoldsDecodeMapNotRead
PASS
ok  	vrg/internal/app
```

## Manual check — the real binary on a PTY

`demo-load.sh` (checked into this directory) runs the freshly built `vrg` on a real tmux PTY with real `rg` over a two-file fixture it generates under `load/`: file A (`a-big-slow-loading-file.txt`) is ~1.5M lines — its read and decode/map phases take several seconds — with `MARK alpha` at line 1, and file B (`b-small-fast-file.txt`) is three lines with `MARK beta`. The cursor starts on A's first stop. The harness presses `n` immediately when the browse view opens so B's content renders while A is still loading, then `p` back to A where only the `Loading…` placeholder may ever appear — A's content is asserted absent at every mid-load capture — plus a `c` scheme flip to prove ordinary input stays live, and a second `n`→`p` round trip showing B instant from the session cache while A still loads, before A's completion finally lands its content. The ~95MB fixture file is regenerated per run and removed by the harness's cleanup; the captured screens remain.

```bash
cd /home/chris/vrg/Notes/walkthroughs/025-04/code-walkthrough && unset RIPGREP_CONFIG_PATH && ./demo-load.sh
```

```output
ok: n at startup: B's content renders while A still loads
ok: n at startup: B's filename rule
ok: p to A mid-load: the placeholder shows
ok: p to A mid-load: A's filename rule
ok: p to A mid-load: no A content before the load completes (MARK alpha)
ok: p to A mid-load: no A content before the load completes (padlines)
ok: c during A's load flips to the light scheme
ok: n during A's load: B renders instantly from the session cache
ok: p to A again mid-load: still the placeholder
ok: p to A again mid-load: still no A content
ok: A's load completing: content lands only now
ok: A's load completing: the first stop's line
ok: A's load completing: the placeholder is gone
ok: vrg exit status -> 0
demo-load: all checks passed
```

```bash
cd /home/chris/vrg/Notes/walkthroughs/025-04/code-walkthrough && echo "== n at startup: B renders while A is still loading ==" && sed -n "1,4p" load/screen-01-b-while-a-loads.txt | sed "s/ *$//" && echo "== p to A mid-load: the placeholder — A content asserted absent ==" && sed -n "1,4p" load/screen-02-a-still-loading.txt | sed "s/ *$//" && echo "== n back to B: instant from the session cache ==" && sed -n "1,4p" load/screen-03-b-cached.txt | sed "s/ *$//" && echo "== p to A once more: still loading ==" && sed -n "1,4p" load/screen-04-a-still-loading.txt | sed "s/ *$//" && echo "== the A load completes: content lands only now ==" && sed -n "1,6p" load/screen-05-a-loaded.txt | sed "s/ *$//"
```

```output
== n at startup: B renders while A is still loading ==
./a-big-slow-loading-file.txt  ── ./b-small-fast-file.txt ──────────────────────
./b-small-fast-file.txt        1  MARK beta
                               2  x
                               3  x
== p to A mid-load: the placeholder — A content asserted absent ==
./a-big-slow-loading-file.txt  ── ./a-big-slow-loading-file.txt ────────────────
./b-small-fast-file.txt           Loading…


== n back to B: instant from the session cache ==
./a-big-slow-loading-file.txt  ── ./b-small-fast-file.txt ──────────────────────
./b-small-fast-file.txt        1  MARK beta
                               2  x
                               3  x
== p to A once more: still loading ==
./a-big-slow-loading-file.txt  ── ./a-big-slow-loading-file.txt ────────────────
./b-small-fast-file.txt           Loading…


== the A load completes: content lands only now ==
./a-big-slow-loading-file.txt  ── ./a-big-slow-loading-file.txt ────────────────
./b-small-fast-file.txt              1  MARK alpha
                                     2  padline 0000002 xxxxxxxxxxxxxxxxxxxxxxxx
                                        xxxxxxxxxxxx
                                     3  padline 0000003 xxxxxxxxxxxxxxxxxxxxxxxx
                                        xxxxxxxxxxxx
```

## Verdict

Issue #25 is verified. On the real binary, pressing `n` immediately at startup renders B's content while A's ~95MB load is still in flight; `p` back to A shows only the `Loading…` placeholder — the harness asserts at every mid-load capture that neither `MARK alpha` nor any padding line ever appears early — and a `c` press mid-load flips the scheme, proving ordinary input stays actionable. A second `n` renders B instantly from the session cache with no reload while A still loads, and A's content arrives only when its own request-keyed completion lands. The gated model tests pin the rest of the contract: placeholder scroll/pan no-ops, one in-flight load per path with re-entry dropped not queued, A→B→C late-completion panel isolation, session-long buffer retention, request-identity rejection of stale and unsolicited completions, post-cancellation discard, and the separately gated decode/map phase keeping `ctrl+c`, `n`, `p`, `w`, `c`, and resizes live.

