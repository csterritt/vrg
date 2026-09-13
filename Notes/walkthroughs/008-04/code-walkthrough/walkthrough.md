# Issue #8: No-results screen and binary exclusion

*2026-09-11T23:59:09Z by Showboat 0.6.1*
<!-- showboat-id: f1263d71-f4c0-42b0-9582-8f4e549856d5 -->

Walkthrough for Issue #8 (Notes/tasks/008-no-results-screen-and-binary-exclusion.md), implementing binary-file exclusion during ripgrep result indexing and a distinct no-results TUI outcome for searches that complete successfully but yield no usable results. References: Notes/PRD-vrg.md (Result index contract, Outcome and exit-status contract).

Contracts verified:
- A non-null binary_offset in an end event drops the file and all its previously collected matches.
- The distinct excluded-file count is exposed via Index.ExcludedFiles().
- The usable-results value is the retained stop count after binary filtering, never the raw received match-event count.
- A complete successful search with no usable results presents a centred No results found screen.
- The optional (N binary files skipped) suffix is appended when every matched file was excluded.
- Both rg exit 1 (empty stream) and rg exit 0 (all-binary-filtered) use the no-results outcome.
- q from the no-results screen exits 1 through the Issue #4 cleanup path.
- Esc from the no-results screen is a no-op.
- ctrl+c from the no-results screen exits 130.
- Mixed streams (one file excluded, one retained) transition to ordinary browsing.

All generated artifacts live in this directory.

```bash
cd /home/chris/vrg && go build ./... && go vet ./... && echo GATES-OK
```

```output
GATES-OK
```

```bash
cd /home/chris/vrg && go test -count=1 ./... -timeout 60s | sed 's/[[:space:]][0-9.]*s$//'
```

```output
ok  	vrg/cmd/vrg
ok  	vrg/internal/app
ok  	vrg/internal/cli
ok  	vrg/internal/filebuffer
ok  	vrg/internal/safepresentation
ok  	vrg/internal/searchindex
?   	vrg/internal/sinkfixtures	[no test files]
ok  	vrg/internal/theme
?   	vrg/internal/viewport	[no test files]
```

## SearchIndex binary exclusion tests

The SearchIndex tests (internal/searchindex/searchindex_test.go, Issue #8) verify that a non-null binary_offset in an end event drops the file and all its previously collected matches, counts the file as a distinct excluded file, drops later matches for the same file, retains matches for other files, and reports the retained stop count as the usable-results value.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/searchindex/ -run '^TestBinaryExclusion' -timeout 30s
```

```output
=== RUN   TestBinaryExclusionDropsMatches
--- PASS: TestBinaryExclusionDropsMatches (0.00s)
=== RUN   TestBinaryExclusionRetainsOtherFiles
--- PASS: TestBinaryExclusionRetainsOtherFiles (0.00s)
=== RUN   TestBinaryExclusionDistinctFileCount
--- PASS: TestBinaryExclusionDistinctFileCount (0.00s)
=== RUN   TestBinaryExclusionNullOffsetRetainsMatches
--- PASS: TestBinaryExclusionNullOffsetRetainsMatches (0.00s)
=== RUN   TestBinaryExclusionMixedRetention
--- PASS: TestBinaryExclusionMixedRetention (0.00s)
=== RUN   TestBinaryExclusionNoExcludedFilesForEmptyStream
--- PASS: TestBinaryExclusionNoExcludedFilesForEmptyStream (0.00s)
=== RUN   TestBinaryExclusionMatchAfterEndDropped
--- PASS: TestBinaryExclusionMatchAfterEndDropped (0.00s)
PASS
ok  	vrg/internal/searchindex	0.002s
```

## App no-results outcome tests

The App tests (internal/app/browse_test.go, Issue #8) verify the no-results screen, the binary skip suffix, the q exit 1, Esc no-op, ctrl+c exit 130, late-completion rejection, and mixed-retention browsing behavior.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/app/ -run '^TestNoResults|^TestMixedRetention' -timeout 30s
```

```output
=== RUN   TestNoResultsEmptyStream
--- PASS: TestNoResultsEmptyStream (0.00s)
=== RUN   TestNoResultsAllBinary
--- PASS: TestNoResultsAllBinary (0.00s)
=== RUN   TestNoResultsSingleBinary
--- PASS: TestNoResultsSingleBinary (0.00s)
=== RUN   TestNoResultsQExitsOne
--- PASS: TestNoResultsQExitsOne (0.00s)
=== RUN   TestNoResultsQExitsOneAllBinary
--- PASS: TestNoResultsQExitsOneAllBinary (0.00s)
=== RUN   TestNoResultsEscIsNoOp
--- PASS: TestNoResultsEscIsNoOp (0.00s)
=== RUN   TestNoResultsCtrlCExits130
--- PASS: TestNoResultsCtrlCExits130 (0.00s)
=== RUN   TestMixedRetentionBrowses
--- PASS: TestMixedRetentionBrowses (0.00s)
=== RUN   TestNoResultsDoesNotShowSearching
--- PASS: TestNoResultsDoesNotShowSearching (0.00s)
=== RUN   TestNoResultsCancelledRejectsLateCompletion
--- PASS: TestNoResultsCancelledRejectsLateCompletion (0.00s)
PASS
ok  	vrg/internal/app	0.024s
```

## Binary demo: empty stream (rg exit 1)

Using the built vrg binary with a fake rg that exits 1 with no output (simulating ripgrep finding no matches). The PTY helper waits for output to settle, then sends q. The stripped output shows the centred No results found screen. The exit code is 1.

```bash
cd /home/chris/vrg/Notes/walkthroughs/008-04/code-walkthrough && PATH=$(pwd)/fakebin:/usr/bin:/bin VRG_KEYS=q VRG_DELAY=1.0 timeout 10 python3 runpty.py ./vrg zzzznotfound . 2>&1; echo exit=$?
```

```output
No results found
exit=1
```

## Binary demo: all-binary stream (rg exit 0)

Using the built vrg binary with a fake rg (fakebin-binary/rg) that emits matches for one file then an end event with a non-null binary_offset, then a summary and exit 0. The PTY helper waits for output to settle, then sends q. The stripped output shows the centred No results found (1 binary files skipped) screen. The exit code is 1.

```bash
cd /home/chris/vrg/Notes/walkthroughs/008-04/code-walkthrough && PATH=$(pwd)/fakebin-binary:/usr/bin:/bin VRG_KEYS=q VRG_DELAY=1.0 timeout 10 python3 runpty.py ./vrg hello . 2>&1; echo exit=$?
```

```output
No results found (1 binary files skipped)
exit=1
```

## Binary demo: mixed retention (one binary, one text)

Using the built vrg binary with a fake rg (fakebin-mixed/rg) that emits matches for two files: one binary-excluded (binary_offset=42) and one retained (binary_offset=null). The PTY helper waits for output to settle, then sends q. The stripped output shows the browse view with only the retained file (text.txt); the binary-excluded file (binary.bin) does not appear. The exit code is 0 (normal browse quit).

```bash
cd /home/chris/vrg/Notes/walkthroughs/008-04/code-walkthrough && PATH=$(pwd)/fakebin-mixed:/usr/bin:/bin VRG_KEYS=q VRG_DELAY=1.0 timeout 10 python3 runpty.py ./vrg hello . 2>&1; echo exit=$?
```

```output
fixtures/src/text.txt     ── fixtures/src/text.txt ── 1  hello world
exit=0
```

## Manual binary demo: no matches (real rg)

Running the built vrg binary with the real ripgrep against a pattern that matches nothing in a temp directory containing only a text file without the pattern. The output shows the centred No results found screen. Pressing q exits 1.

```bash
mkdir -p /tmp/vrg-empty-demo && printf 'some text without the pattern\n' > /tmp/vrg-empty-demo/file.txt && cd /home/chris/vrg/Notes/walkthroughs/008-04/code-walkthrough && VRG_KEYS=q VRG_DELAY=1.0 timeout 10 python3 runpty.py ./vrg zzzznotfound /tmp/vrg-empty-demo 2>&1; echo exit=$?
```

```output
No results found
exit=1
```

## Manual binary demo: binary-only directory

Running the built vrg binary with a fake rg (fakebin-binary/rg) that simulates ripgrep finding a match in a binary file and reporting a non-null binary_offset. The output shows the centred No results found (1 binary files skipped) screen. Pressing q exits 1.

```bash
cd /home/chris/vrg/Notes/walkthroughs/008-04/code-walkthrough && PATH=$(pwd)/fakebin-binary:/usr/bin:/bin VRG_KEYS=q VRG_DELAY=1.0 timeout 10 python3 runpty.py ./vrg hello . 2>&1; echo exit=$?
```

```output
No results found (1 binary files skipped)
exit=1
```

## Summary

The walkthrough demonstrates:
- SearchIndex binary exclusion tests: a non-null binary_offset drops the file and its matches, counts distinct excluded files, retains other files, and reports retained stops as the usable-results value.
- App no-results outcome tests: the centred No results found screen, the (N binary files skipped) suffix, q exit 1, Esc no-op, ctrl+c exit 130, late-completion rejection, and mixed-retention browsing.
- The binary with a fake rg exiting 1 (empty stream): No results found, q exits 1.
- The binary with a fake rg emitting an all-binary stream (rg exit 0): No results found (1 binary files skipped), q exits 1.
- The binary with a fake rg emitting a mixed stream: browse view with only the retained file, q exits 0.
- The binary with the real ripgrep against a non-matching pattern in a temp directory: No results found, q exits 1.
- The binary with a fake rg simulating a binary-only directory: No results found (1 binary files skipped), q exits 1.

References: Issue #8 (Notes/tasks/008-no-results-screen-and-binary-exclusion.md), Notes/PRD-vrg.md (Result index contract, Outcome and exit-status contract).
