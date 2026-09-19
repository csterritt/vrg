# Issue #25: asynchronous load isolation — navigate during load, late results update only their own file

*2026-09-17T22:36:23Z by Showboat 0.6.1*
<!-- showboat-id: 18caf4e4-2124-4db8-980f-8ec71411af31 -->

Issue #25 makes file loads fully asynchronous and isolated: navigation stays active while a file loads so the user can move past a slow file, load completions are keyed by raw path plus request identity so a late answer updates only that path's cache/status and never another file's panel, at most one load is in flight per raw path with re-entry dropped rather than queued, successful buffers are retained for the session with no eviction, post-cancellation completions are rejected, and the decode/map phase is separately gatable with ctrl+c, n/p, w, c, and resize all actionable while it is held. See Notes/issues/025-async-load-isolation.md, Notes/tasks/025-async-load-isolation.md, and the PRD section 'File loading, cache, reload, and selection consistency' (first three bullets) in Notes/PRD-vrg.md. All artifacts live in this directory.

## Gates — module integrity, build, vet

```bash
cd /home/chris/vrg && go mod verify && go build ./... && go vet ./... && echo GATES-OK
```

```output
all modules verified
GATES-OK
```

## Isolation tests — keyed completions, one load per path, caching

internal/app/loadiso_test.go pins the Issue #25 contracts through gated workers. TestNavigationRemainsActiveWhileLoadHeld holds A's load and drives n past it — the cursor, filename rule, pop-up, and B's own request all land — with placeholder scrolling a no-op and w/c/resize unaffected. TestLateCompletionCachesOnlyItsOwnPath runs A→B→C with A's load held: A's completion arriving while C is current caches A (its layout installs invisibly) and leaves C's panel, cursor, and viewport untouched. TestReentryDuringLoadStartsNothing proves a dropped — never queued — re-entry: the crossing batch is only the pop-up leaf and the live request identity is unchanged. TestCachedRevisitIssuesNoLoad proves session-long retention: the revisited file serves the cached buffer with no load leaf. The rejection trio — TestUnrequestedCompletionDropped, TestStaleCompletionDroppedWhileLoadInFlight, TestSettledCompletionDropped — covers completions with no live request, a wrong request identity, and an already-settled request: all discarded without touching cache, failure state, diagnostics, or panel.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestNavigationRemainsActiveWhileLoadHeld|TestLateCompletionCachesOnlyItsOwnPath|TestReentryDuringLoadStartsNothing|TestCachedRevisitIssuesNoLoad|TestUnrequestedCompletionDropped|TestStaleCompletionDroppedWhileLoadInFlight|TestSettledCompletionDropped' ./internal/app 2>&1 | grep -E '^(--- |ok|FAIL|    ---)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestNavigationRemainsActiveWhileLoadHeld
--- PASS: TestLateCompletionCachesOnlyItsOwnPath
--- PASS: TestReentryDuringLoadStartsNothing
--- PASS: TestCachedRevisitIssuesNoLoad
--- PASS: TestUnrequestedCompletionDropped
--- PASS: TestStaleCompletionDroppedWhileLoadInFlight
--- PASS: TestSettledCompletionDropped
ok  	vrg/internal/app
```

## Cancellation and decode/map-phase tests

TestPostCancellationCompletionDiscarded holds a load, cancels with ctrl+c, then releases the worker: the completion lands on a cancelled UI as a non-event — no cache, no failure record, no diagnostics. TestGatedDecodeMapKeepsInputsResponsive exercises the new WithDecodeGate seam: the load command runs filebuffer.Read under the whole-load gate, then decodeGate, then filebuffer.Decode — so with the read done and decode/map held, n/p still navigate, w and c still toggle, a resize still applies, and ctrl+c exits 130 without waiting on the worker; the released completions are discarded on the cancelled UI.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestPostCancellationCompletionDiscarded|TestGatedDecodeMapKeepsInputsResponsive' ./internal/app 2>&1 | grep -E '^(--- |ok|FAIL|    ---)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestPostCancellationCompletionDiscarded
--- PASS: TestGatedDecodeMapKeepsInputsResponsive
ok  	vrg/internal/app
```

## Full module regression

Request identities thread through every fileLoadedMsg producer and consumer, so the whole suite runs.

```bash
cd /home/chris/vrg && go test -count=1 ./internal/filebuffer ./internal/viewport ./internal/app ./internal/safepresentation ./internal/searchindex ./internal/theme ./internal/cli ./cmd/vrg 2>&1 | sed -E 's/\t[0-9.]+s$//'
```

```output
ok  	vrg/internal/filebuffer
ok  	vrg/internal/viewport
ok  	vrg/internal/app
ok  	vrg/internal/safepresentation
ok  	vrg/internal/searchindex
ok  	vrg/internal/theme
ok  	vrg/internal/cli
ok  	vrg/cmd/vrg
```

## Manual check — the real binary on a pty

pty_loadiso.py runs the built vrg on a real pty (80x24) over a fixture directory under this walkthrough: aaa-huge.txt (~90 MB, two million pad lines after its 'needle a one' match) first in the index and bbb-small.txt (one match, a handful of lines) second. VRG_TEST_LOAD_GATE points at a gate file that holds every load worker before its read. With the gate in place, vrg starts on A's 'Loading…'; pressing n immediately switches the panel to B's placeholder — navigation never waits on A's worker. Removing the gate releases both workers: tiny B decodes first and shows content while A's ~90 MB read plus decode/map is still in flight; pressing p returns to A, which shows 'Loading…' — never content before its load completes — until A's completion arrives while it is current and the content appears.

```bash
cd /home/chris/vrg && go build -o Notes/walkthroughs/025-04/code-walkthrough/vrg ./cmd/vrg && python3 Notes/walkthroughs/025-04/code-walkthrough/pty_loadiso.py
```

```output
startup   : gate held — A on 'Loading…' — '…alkthrough/fixture/aaa-huge.txtLoading…'
n         : navigation live under the held load — now on bbb-small.txt 'Loading…'
release   : gate released — B shows content while A still loads — '…alkthrough/fixture/aaa-huge.txt 1  need'
p         : back on A — still 'Loading…', never early content — '…alkthrough/fixture/aaa-huge.txtLoading…'
A lands   : A's completion arrived while current — content — '…alkthrough/fixture/aaa-huge.txt      1 '
q         : exit 0
OK
```
