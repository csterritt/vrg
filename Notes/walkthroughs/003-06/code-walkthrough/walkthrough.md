# Issue #3: spawn rg, collect results, Searching… screen

*2026-09-23T16:48:21Z by Showboat 0.6.1*
<!-- showboat-id: ddc09910-2845-46a6-b82e-a227225962e5 -->

Walkthrough for [Issue #3](../../../issues/003-spawn-rg-collect-results-searching-screen.md), implementing rg spawn, result collection, and the searching screen per `Notes/PRD-vrg.md` (Implementation Decisions → Result index, records, and stream integrity; Module Design → App and Search index). All generated artifacts live in this directory: the built `vrg` binary, `demo-live.sh` and its fifo/capture files, and the `empty-path` directory for the rg-free start-failure check. Durations are stripped so the document verifies cleanly.

```bash
cd /home/chris/vrg && go vet ./... && go build ./... && go test -count=1 ./internal/searchindex ./internal/app ./cmd/vrg | sed 's/[[:space:]][0-9.]*s$//' && echo GATES-OK
```

```output
ok  	vrg/internal/searchindex
ok  	vrg/internal/app
ok  	vrg/cmd/vrg
GATES-OK
```

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestDecodeRecordKinds|TestDecodeMatchRecord|TestDecodeIgnoresUnrequiredFields|TestMalformedRecords|TestUnknownRecordType|TestHappyPathStream|TestSameLineMerging|TestMixedEncodingSameValueMerge|TestOverlappingSubmatchUnionCoverage|TestIndexOrdering|TestStopRetainsRawData|TestRelativePathResolution' ./internal/searchindex 2>&1 | grep -vE '^(=== RUN|    --- (PASS|FAIL))' | sed -E 's/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//'
```

```output
--- PASS: TestDecodeRecordKinds
--- PASS: TestDecodeMatchRecord
--- PASS: TestDecodeIgnoresUnrequiredFields
--- PASS: TestMalformedRecords
--- PASS: TestUnknownRecordType
--- PASS: TestHappyPathStream
--- PASS: TestSameLineMerging
--- PASS: TestMixedEncodingSameValueMerge
--- PASS: TestOverlappingSubmatchUnionCoverage
--- PASS: TestIndexOrdering
--- PASS: TestStopRetainsRawData
--- PASS: TestRelativePathResolution
PASS
ok  	vrg/internal/searchindex
```

Focused `internal/app` model tests: the searching screen while the completion channel is silent, resize during search, the summary transition, `q` → `tea.Quit` with exit 0, and the preparation gate holding the searching state after rg has exited and both pipes are drained.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestSearchingScreenShownWhileCollecting|TestResizeDuringSearching|TestCompletionTransitionsToSummary|TestQOnSummaryExitsZero|TestGateHoldsSearchingAfterRgExit' ./internal/app 2>&1 | grep -vE '^(=== RUN|    --- (PASS|FAIL))' | sed -E 's/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//'
```

```output
--- PASS: TestSearchingScreenShownWhileCollecting
--- PASS: TestResizeDuringSearching
--- PASS: TestCompletionTransitionsToSummary
--- PASS: TestQOnSummaryExitsZero
--- PASS: TestGateHoldsSearchingAfterRgExit
PASS
ok  	vrg/internal/app
```

Subprocess tests with the test binary re-executed as fake rg: the child receives the protected argv verbatim and runs in the invocation working directory; the flood fixture writes 20 × 64 KiB to stderr interleaved with 20 valid stdout records — no deadlock, no lost stream, and a post-write handshake file proves the child finished both pipes before exiting; a missing rg fails synchronously.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestChildArgvAndWorkingDirectory|TestDualPipeBackpressure|TestStartFailure' ./internal/app 2>&1 | grep -vE '^(=== RUN|    --- (PASS|FAIL))' | sed -E 's/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//'
```

```output
--- PASS: TestChildArgvAndWorkingDirectory
--- PASS: TestDualPipeBackpressure
--- PASS: TestStartFailure
PASS
ok  	vrg/internal/app
```

Process boundary: a shell fake rg on PATH captures its argv and cwd and holds the stream open so the harness observes `Searching…`; the boundary flood fixture writes 16 × 64 KiB to stderr; and an rg-free PATH yields the sanitized diagnostic, exit 2, no TUI.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestSearchLifecycleAtBoundary|TestDualPipeDrainageAtBoundary|TestStartFailureNoRipgrep' ./cmd/vrg 2>&1 | grep -vE '^(=== RUN|    --- (PASS|FAIL))' | sed -E 's/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//'
```

```output
--- PASS: TestSearchLifecycleAtBoundary
--- PASS: TestDualPipeDrainageAtBoundary
--- PASS: TestStartFailureNoRipgrep
PASS
ok  	vrg/cmd/vrg
```

```bash
cd /home/chris/vrg && go build -o Notes/walkthroughs/003-06/code-walkthrough/vrg ./cmd/vrg && cd Notes/walkthroughs/003-06/code-walkthrough && test -x vrg && ./vrg --help | head -1
```

```output
Usage: vrg [OPTIONS] PATTERN [ROOT]
```

Live session with the real binary in the real repository: stdin/stdout are pipes, and real `rg` searches the Go module cache (large enough that collection visibly outlives the first frame). `demo-live.sh` watches the raw stream for the interim-summary marker, sends `q`, and records the exit status.

```bash
./demo-live.sh
```

```output
Searching…
120 files, 365 matched lines
process-exit=0
stderr-bytes=0
```

Start failure: explicit binary-path invocation with an rg-free PATH. The diagnostic is sanitized on stderr, the exit status is 2, and no TUI output reaches stdout.

```bash
cd /home/chris/vrg/Notes/walkthroughs/003-06/code-walkthrough && mkdir -p empty-path && env -i PATH="$PWD/empty-path" ./vrg foo . > start-out.txt 2> start-err.txt; echo "exit=$?"; echo "stdout-bytes=$(wc -c < start-out.txt)"; cat start-err.txt
```

```output
exit=2
stdout-bytes=0
vrg: cannot start rg: exec: "rg": executable file not found in $PATH
```

## Result. Every Issue #3 behavior is demonstrated: `internal/searchindex` decodes both record encodings, enforces the per-record schema matrix, merges same-line stops, retains overlapping submatches with union highlight coverage, orders by unsigned raw path bytes then line, and resolves relative paths against the invocation working directory without canonicalization; `internal/app` spawns rg with the exact protected argv in the invocation working directory, drains stdout and stderr concurrently for the whole child lifetime (1 MiB+ flood without deadlock or loss), keeps `Searching…` up across collection and gated post-exit preparation, shows the `N files, M matched lines` interim summary, and exits 0 on `q`; and an rg start failure is a sanitized stderr diagnostic with exit 2 and no TUI. Deferred by design: stderr classification and outcome effects (Issue #9), cancellation/cleanup (Issue #4), malformed-record accounting (Issue #10), and stale line-number validation (Issue #29).
