# Issue #8: No-results screen and binary exclusion

*2026-09-23T19:21:51Z by Showboat 0.6.1*
<!-- showboat-id: 4f839629-022f-4192-ae54-70511bce4070 -->

Walkthrough for [Issue #8](../../../issues/008-no-results-screen-and-binary-exclusion.md), implementing binary-file exclusion and the no-results screen per `Notes/PRD-vrg.md` (*Result index, records, and stream integrity* — the `binary_offset` bullet; *Outcome and exit-status contract* — the last row and the empty-screen bullet). A valid `end` record with a non-null `binary_offset` drops its file and all stops collected from it, and the index counts distinct excluded files. Usable results is the count of retained stops after filtering — `Index.LineCount()` — the single value the outcome logic consumes, never the match-event count. When a complete search yields zero usable results the model shows the centred "No results found" screen with fixed exit status 1, appending "(N binary files skipped)" when exclusion emptied the list — for rg-1 empty and rg-0 all-filtered streams alike. On that screen `q` exits 1 through the ordinary cleanup path, `Esc` is a no-op, and `ctrl+c` overrides to 130. All generated artifacts live in this directory: the built `vrg` binary, `demo-empty.sh`, `demo-binary.sh`, and their `manual-empty/` and `manual-binary/` session files. Test durations are stripped so the document verifies cleanly.

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
?   	vrg/internal/viewport	[no test files]
GATES-OK
```

## SearchIndex exclusion tests

`internal/searchindex/searchindex_test.go` pins the exclusion contract: `TestBinaryEndDropsEarlierMatches` feeds two match events then an `end` with `binary_offset` — every stop is removed, the file is absent, and `BinaryExcluded()` counts it once. `TestBinaryExclusionCountsDistinctFiles` proves the tally counts distinct raw paths — a repeated excluding end does not recount, a match arriving after exclusion stays dropped, an excluded file with no collected matches still counts, and a null `binary_offset` excludes nothing. `TestUsableResultsIsRetainedStops` proves usable results is the retained-stop count after filtering: four match events for a binary-excluded file plus one retained stop yield `LineCount()` 1.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestBinaryEndDropsEarlierMatches|TestBinaryExclusionCountsDistinctFiles|TestUsableResultsIsRetainedStops' ./internal/searchindex 2>&1 | grep -vE '^(=== RUN|    --- (PASS|FAIL))' | sed -E 's/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//'
```

```output
--- PASS: TestBinaryEndDropsEarlierMatches
--- PASS: TestBinaryExclusionCountsDistinctFiles
--- PASS: TestUsableResultsIsRetainedStops
PASS
ok  	vrg/internal/searchindex
```

## App outcome tests

`internal/app/noresults_test.go` drives the done message through `Update`: `TestEmptyStreamShowsNoResults` (rg-1 summary-only stream, real `*exec.ExitError` status 1) shows the centred message with no suffix and no load command; `TestAllBinaryStreamShowsSkipCount` (rg-0 stream, every matched file excluded) appends `(2 binary files skipped)`; `TestMixedStreamBrowsesRetained` browses a one-excluded/one-retained stream with usable results 1; `TestQOnNoResultsExitsOne` quits 1 without touching cancellation; `TestEscOnNoResultsIsNoOp` leaves the fixed-1 screen up; `TestCtrlCOnNoResultsExits130` overrides to 130.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestEmptyStreamShowsNoResults|TestAllBinaryStreamShowsSkipCount|TestMixedStreamBrowsesRetained|TestQOnNoResultsExitsOne|TestEscOnNoResultsIsNoOp|TestCtrlCOnNoResultsExits130' ./internal/app 2>&1 | grep -vE '^(=== RUN|    --- (PASS|FAIL))' | sed -E 's/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//'
```

```output
--- PASS: TestEmptyStreamShowsNoResults
--- PASS: TestAllBinaryStreamShowsSkipCount
--- PASS: TestMixedStreamBrowsesRetained
--- PASS: TestQOnNoResultsExitsOne
--- PASS: TestEscOnNoResultsIsNoOp
--- PASS: TestCtrlCOnNoResultsExits130
PASS
ok  	vrg/internal/app
```

## Manual check — rg-1 empty search

`demo-empty.sh` (checked into this directory) drives the freshly built `vrg` on a real PTY via `script(1)`: an 80x24 inner session runs `vrg zzzznotfound .` in a fixture directory whose one text file cannot match. rg exits 1 with a summary-only stream; the harness waits for the no-results frame in the captured typescript, sends `q`, and asserts the message text, the absent suffix, and exit status 1.

```bash
cd /home/chris/vrg && go build -o Notes/walkthroughs/008-04/code-walkthrough/vrg ./cmd/vrg && cd Notes/walkthroughs/008-04/code-walkthrough && ./demo-empty.sh
```

```output
exit=1
screen-message=present
no-suffix=absent
--- final screen ---
 No results found
```

## Manual check — rg-0 all-binary search

`demo-binary.sh` (checked into this directory) runs `vrg foo .` in a directory containing only `b.bin`: 7000 `foo hit N` lines followed by a NUL. rg emits binary output lazily for walked trees — a NUL inside its initial detection window (~64 KiB observed with rg 15.2.0) suppresses the file outright with no events, so the fixture's NUL sits past that window: rg emits thousands of `match` records then an `end` with `binary_offset`, the index drops every collected stop, and the screen reports the distinct-file count. The harness waits for the suffixed frame, sends `q`, and asserts exit 1 with the excluded file never reaching a browse filename rule.

```bash
cd /home/chris/vrg/Notes/walkthroughs/008-04/code-walkthrough && ./demo-binary.sh
```

```output
exit=1
screen-with-suffix=present
excluded-absent=absent
--- final screen ---
 No results found (1 binary files skipped)
```

## Result

Issue #8 is verified. `internal/searchindex` drops a file's collected stops on a non-null `binary_offset` end and counts distinct excluded files; `internal/app` treats retained stops as the usable-results value and shows the centred no-results screen — with the binary-skip suffix when exclusion emptied the list — for rg-1 empty and rg-0 all-filtered streams alike, fixing the ordinary status at 1. `q` exits 1 through the Issue #4 cleanup path, `Esc` is a no-op, and `ctrl+c` overrides to 130. Deferred by design: `waitErr`/`stderr` consumption, stream-integrity lifecycle validation, and error overlays (Issue #9); record-loss counting (Issue #10).
