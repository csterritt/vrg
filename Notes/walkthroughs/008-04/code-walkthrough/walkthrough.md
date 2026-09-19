# Issue #8: no-results screen and binary exclusion

*2026-09-16T21:39:29Z by Showboat 0.6.1*
<!-- showboat-id: c55aba2c-302c-4377-a683-010c63bfc056 -->

Issue #8 lands the empty-search outcome and binary-file exclusion: a valid `end` event with non-null `binary_offset` drops that file and all its previously collected matches (counting distinct excluded files), `UsableResults` reports retained stops after filtering — never match events received — and a complete successful search with no usable results shows the centred "No results found" screen, appending "(N binary files skipped)" when every matched file was excluded, with `q` exiting 1 through the Issue #4 cleanup path, `Esc` a no-op, and `ctrl+c` → 130. See `Notes/issues/008-no-results-screen-and-binary-exclusion.md`, `Notes/tasks/008-no-results-screen-and-binary-exclusion.md`, and the PRD sections "Result index, records, and stream integrity" (binary bullet) and "Outcome and exit-status contract" (last row) in `Notes/PRD-vrg.md`. All artifacts live in this directory.

## Gates — module integrity, build, vet

```bash
cd /home/chris/vrg && go mod verify && go build ./... && go vet ./... && echo GATES-OK
```

```output
all modules verified
GATES-OK
```

## SearchIndex — binary exclusion and usable results

`internal/searchindex/index_test.go` pins the Issue #8 contracts: `TestBinaryEndDropsFileMatches` feeds two match events then an `end` with a non-null `binary_offset` (in the other encoding — exclusion matches on decoded bytes) and the file is absent from the prepared index with `BinaryExcluded` 1 and `UsableResults` 1; `TestBinaryExclusionCountsDistinctFiles` shows two binary files count two while a repeated binary `end` re-drops interim matches without double-counting; `TestUsableResultsCountsRetainedStops` proves usable results is retained stops after filtering — a summary-only stream and an all-excluded stream both report 0, and merged survivors count once.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'Binary|UsableResults' ./internal/searchindex 2>&1 | grep -E '^(--- |    --- |ok|FAIL)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestBinaryEndDropsFileMatches
--- PASS: TestBinaryExclusionCountsDistinctFiles
--- PASS: TestUsableResultsCountsRetainedStops
ok  	vrg/internal/searchindex
```

## App — the no-results outcome

`internal/app/noresults_test.go` drives the real collection command: `TestEmptySearchShowsNoResultsScreen` sends a complete rg-1 stream (summary only) and asserts the exact centred frame "No results found" with no suffix; `TestAllBinarySearchShowsSkipCount` sends an all-excluded stream under rg codes 0 and 1 and gets "No results found (2 binary files skipped)" with `q` → 1 both ways; `TestMixedBinaryStreamBrowses` retains one file while excluding another and browses with usable results 1; `TestEscOnNoResultsNoOp` returns no command; `TestQOnNoResultsExitsOne` and `TestCtrlCOnNoResultsExits130` prove the cleanup path (terminate/reap before quit) and the 130 override.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'NoResults|AllBinary|MixedBinary' ./internal/app 2>&1 | grep -E '^(--- |ok|FAIL)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestEmptySearchShowsNoResultsScreen
--- PASS: TestQOnNoResultsExitsOne
--- PASS: TestAllBinarySearchShowsSkipCount
--- PASS: TestMixedBinaryStreamBrowses
--- PASS: TestEscOnNoResultsNoOp
--- PASS: TestCtrlCOnNoResultsExits130
ok  	vrg/internal/app
```

## Full suite — `go test ./...`

```bash
cd /home/chris/vrg && go test -count=1 ./... 2>&1 | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
?   	vrg/Notes/walkthroughs/007-04/code-walkthrough/fixture	[no test files]
ok  	vrg/cmd/vrg
ok  	vrg/internal/app
ok  	vrg/internal/cli
ok  	vrg/internal/filebuffer
ok  	vrg/internal/safepresentation
?   	vrg/internal/safepresentation/sinktest	[no test files]
ok  	vrg/internal/searchindex
ok  	vrg/internal/theme
?   	vrg/internal/viewport	[no test files]
```

## What rg actually emits for a binary file

The fixture's `binonly/` directory holds only `b.bin` (`printf 'foo\\0bar\\n'`). With rg 15.x, an explicit file operand gets searched up to the first NUL: the match record is emitted, then the `end` carries `binary_offset` — the record sequence that drops the file and counts the exclusion.

```bash
cd /home/chris/vrg/Notes/walkthroughs/008-04/code-walkthrough/fixture/binonly && rg --json --no-config -- foo b.bin | jq -c 'del(.data.stats, .data.elapsed_total)'; echo "rg exit: ${PIPESTATUS[0]}"
```

```output
{"type":"begin","data":{"path":{"text":"b.bin"}}}
{"type":"match","data":{"path":{"text":"b.bin"},"lines":{"text":"foo\u0000bar\n"},"line_number":1,"absolute_offset":0,"submatches":[{"match":{"text":"foo"},"start":0,"end":3}]}}
{"type":"end","data":{"path":{"text":"b.bin"},"binary_offset":3}}
{"data":{},"type":"summary"}
rg exit: 0
```

During directory traversal the same file produces no records at all — rg 15.x detects the NUL and drops the file silently, so a traversed binary-only search is an ordinary empty stream (summary only, exit 1).

```bash
cd /home/chris/vrg/Notes/walkthroughs/008-04/code-walkthrough/fixture/binonly && rg --json --no-config -- foo . | jq -c 'del(.data.stats, .data.elapsed_total)'; echo "rg exit: ${PIPESTATUS[0]}"
```

```output
{"data":{},"type":"summary"}
rg exit: 1
```

## Manual check — the real binary on a pty

`pty_noresults.py` runs the built vrg on a pty in three sessions and asserts on the raw byte stream a terminal would execute: `vrg zzzznotfound .` in `fixture/empty/` shows the centred "No results found" with no suffix and `q` exits 1 after the alt screen is left; `vrg foo .` in `fixture/binonly/` (rg traversal — silent drop) shows the same plain screen; and `vrg foo b.bin` — the binary file as the search root — shows "No results found (1 binary files skipped)" with `q` exiting 1.

```bash
cd /home/chris/vrg && go build -o Notes/walkthroughs/008-04/code-walkthrough/vrg ./cmd/vrg && python3 Notes/walkthroughs/008-04/code-walkthrough/pty_noresults.py
```

```output
empty    : vrg zzzznotfound . shows "No results found", no suffix; q exits 1, alt screen restored
traversed: vrg foo . over the binary-only dir shows plain "No results found" (rg drops it silently); q exits 1
operand  : vrg foo b.bin shows "No results found (1 binary files skipped)"; q exits 1
OK
```

All Issue #8 contracts verified: a complete search with no usable results presents the centred "No results found" screen; a valid `end` with non-null `binary_offset` drops the file and its earlier matches and counts distinct excluded files; the all-filtered outcome appends "(N binary files skipped)" for rg-0 and rg-1 alike; `q` dismisses to exit 1 through the cleanup path, `Esc` is a no-op, and `ctrl+c` exits 130. Usable results is the retained-stop count the Issue #9 outcome rows will consume.
