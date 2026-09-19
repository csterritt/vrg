# Issue #3: spawn rg, collect results, searching screen

*2026-09-16T17:47:12Z by Showboat 0.6.1*
<!-- showboat-id: ddee9f35-e9ac-41bf-b750-85d6155cf33c -->

Walkthrough for Issue #3 (`Notes/issues/003-spawn-rg-collect-results-searching-screen.md`), replacing the CLI search stub with the real ripgrep invocation per `Notes/PRD-vrg.md` (*Implementation Decisions → Invocation and child arguments*; *Result index, records, and stream integrity*; *Module Design → SearchIndex / App*): `cmd/vrg` hands the protected `ChildArgs` vector and the invocation working directory to `app.Run`; `internal/app` spawns `rg`, drains both stdout and stderr concurrently for the whole child lifetime, and keeps the "Searching…" screen up through collection and post-exit index preparation; `internal/searchindex` parses the `text`/base64-`bytes` JSON record stream into merged, raw-byte-ordered navigation stops. The interim summary (`N files, M matched lines`) then shows until `q` exits 0; a start failure is a sanitized stderr diagnostic with exit 2 and no TUI. All generated artifacts (the built `vrg` binary, `pty_demo.py`, `fixtures/`, `empty-path/`) live in this directory.

```bash
cd /home/chris/vrg && go mod verify && go build ./... && go vet ./... && echo GATES-OK
```

```output
all modules verified
GATES-OK
```

```bash
cd /home/chris/vrg && go test -count=1 ./... 2>&1 | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
ok  	vrg/cmd/vrg
ok  	vrg/internal/app
ok  	vrg/internal/cli
?   	vrg/internal/filebuffer	[no test files]
ok  	vrg/internal/searchindex
?   	vrg/internal/theme	[no test files]
?   	vrg/internal/viewport	[no test files]
```

Focused SearchIndex tests (`internal/searchindex/index_test.go`): both `text` and base64 `bytes` record encodings, raw-byte retention for non-UTF-8 data, same-path/same-line stop merging including across encodings, `(start, end)` submatch ordering, union highlight coverage over overlapping ranges, unsigned raw-byte file ordering, working-directory path resolution without canonicalization, the five known event kinds plus unknown/malformed classification, and submatch range boundaries.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/searchindex/ 2>&1 | grep -E '^(=== RUN|--- (PASS|FAIL)|ok|FAIL|PASS)' | grep -v '^=== RUN' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestEncodingFormsRetainIdenticalBytes
--- PASS: TestNonUTF8BytesRetained
--- PASS: TestSamePathSameLineMerge
--- PASS: TestMixedEncodingSameValueMerge
--- PASS: TestOverlappingSubmatchesUnionCoverage
--- PASS: TestIndexOrderingRawPathThenLine
--- PASS: TestRelativePathResolution
--- PASS: TestRecordKindsAndIgnoredContext
--- PASS: TestMalformedRecordsSkipped
--- PASS: TestSubmatchRangeBoundaries
PASS
ok  	vrg/internal/searchindex
```

Focused App tests (`internal/app/app_test.go`, `rg_test.go`): the model opens on the searching screen and stays there while a gate holds index preparation after the child has fully exited; a completion message yields the interim summary; `q` exits 0 on the summary and is inert while searching; resizes are handled mid-search. The `spawn` seam is proven against fake `rg` scripts: exact argv and working directory, concurrent dual-pipe drainage under \u22651 MiB of interleaved stderr (handshake proves the child finished writing both pipes), and a clean start error when rg is absent.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/app/ 2>&1 | grep -E '^(=== RUN|--- (PASS|FAIL)|ok|FAIL|PASS)' | grep -v '^=== RUN' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestSearchingScreenShownDuringCollection
--- PASS: TestCompletionTransitionsToSummary
--- PASS: TestQOnSummaryExitsZero
--- PASS: TestQWhileSearchingStaysSearching
--- PASS: TestResizeDuringSearching
--- PASS: TestGateHeldPreparationStaysSearching
--- PASS: TestRunStartFailureExit2
--- PASS: TestRunStartFailureSanitizesError
--- PASS: TestSpawnArgvAndWorkdir
--- PASS: TestDualPipeDrainage
--- PASS: TestSpawnMissingBinary
PASS
ok  	vrg/internal/app
```

Subprocess-boundary tests (`cmd/vrg/search_test.go`) drive the real binary on a pty: `TestChildArgvAndWorkdir` has a fake `rg` record its argv and `pwd` (exact protected vector from the invocation working directory); `TestDualPipeBackpressure` writes \u22651 MiB of stderr interleaved with a valid stdout stream and a post-write handshake; `TestGateHeldPreparationKeepsSearching` holds the gate after the collect-ack and confirms only the searching screen renders; `TestStartFailureExit2` uses an rg-free PATH.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestChildArgvAndWorkdir|TestStartFailureExit2|TestDualPipeBackpressure|TestStderrCapturedWithoutBlocking|TestGateHeldPreparationKeepsSearching' ./cmd/vrg/ 2>&1 | grep -E '^(=== RUN|--- (PASS|FAIL)|ok|FAIL|PASS)' | grep -v '^=== RUN' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestChildArgvAndWorkdir
--- PASS: TestStartFailureExit2
--- PASS: TestDualPipeBackpressure
--- PASS: TestStderrCapturedWithoutBlocking
--- PASS: TestGateHeldPreparationKeepsSearching
PASS
ok  	vrg/cmd/vrg
```

Manual scenario 1 — the real binary searches a repository on a pty. `pty_demo.py` (in this directory) spawns `./vrg needle fixtures` under a 24x80 pty with `VRG_TEST_GATE` holding index preparation and `VRG_TEST_COLLECT_ACK` recording the moment the child has exited and its stream is fully collected: the screen shows "Searching…" with no summary while the gate is held; releasing the gate produces the interim summary, and `q` exits 0. The fixture tree contains two files with three `needle` lines total.

```bash
cd /home/chris/vrg && go build -o Notes/walkthroughs/003-06/code-walkthrough/vrg ./cmd/vrg && cd Notes/walkthroughs/003-06/code-walkthrough && file vrg | cut -d: -f2 | cut -c1-60 && echo '--- fixtures ---' && cat fixtures/alpha.txt fixtures/beta.txt
```

```output
 ELF 64-bit LSB executable, ARM aarch64, version 1 (SYSV), s
--- fixtures ---
alpha
needle one
beta
needle two
needle three
```

```bash
cd /home/chris/vrg/Notes/walkthroughs/003-06/code-walkthrough && python3 pty_demo.py
```

```output
gate-held : rg exited and stream collected; screen shows Searching, no summary
released  : interim summary '2 files, 3 matched lines'
q         : exit=0
OK
```

Manual scenario 2 — rg absent from `PATH`: the explicit binary invocation writes the sanitized single-line start-failure diagnostic to stderr and exits 2, with empty stdout and no TUI (`empty-path/` is an rg-free directory in this walkthrough dir).

```bash
cd /home/chris/vrg/Notes/walkthroughs/003-06/code-walkthrough && PATH="/home/chris/vrg/Notes/walkthroughs/003-06/code-walkthrough/empty-path" ./vrg needle fixtures >o.txt 2>e.txt; echo "exit=$?"; echo "stdout_bytes=$(wc -c <o.txt)"; cat e.txt; rm -f o.txt e.txt
```

```output
exit=2
stdout_bytes=0
vrg: cannot start ripgrep: exec: "rg": executable file not found in $PATH
```

## Result

Every Issue #3 requirement is demonstrated: the focused SearchIndex suite covers both record encodings, stop merging, ordering, retention, and working-directory resolution; the focused App suite covers the searching-to-summary lifecycle, the post-exit gate hold, resize responsiveness, inert `q` while searching, and start failure; the subprocess-boundary suite proves the exact protected argv and invocation working directory through a fake `rg`, deadlock-free dual-pipe drainage under \u22651 MiB of interleaved stderr with a completion handshake, captured stderr without blocking, the gate-held searching screen, and exit-2 start failure. Manually, the real binary on a pty shows "Searching…" while the gate holds collection-complete state, then "2 files, 3 matched lines", and `q` exits 0; an rg-free PATH yields the sanitized `vrg: cannot start ripgrep: exec: "rg": executable file not found in $PATH` diagnostic and exit 2 with empty stdout. Per `Notes/PRD-vrg.md` and `Notes/issues/003-spawn-rg-collect-results-searching-screen.md`; cancellation and stderr classification are deliberately deferred to Issues #4 and #9.
