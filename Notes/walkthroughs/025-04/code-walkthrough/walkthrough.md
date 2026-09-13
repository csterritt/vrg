# Issue #25: Asynchronous load isolation

*2026-09-12T13:06:37Z by Showboat 0.6.1*
<!-- showboat-id: 6a2d9fe5-3064-4801-a883-1bc76687a515 -->

Walkthrough for Issue #25 (Notes/tasks/025-async-load-isolation.md), implementing keyed asynchronous load isolation in vrg. The TUI stays navigable while file loads are pending. Late completions update only their own file's cache and status, never overwriting the currently visible panel. At most one load is in flight per raw path; re-entering a loading path starts no second load and queues nothing. Successful buffers are retained for the session with no eviction. Late completions after cancellation are rejected. The decode/map phase is separately gatable, and ctrl+c, n, p, w, c, and resize all remain actionable while it is held. References: Notes/PRD-vrg.md (File loading, cache, reload, and selection consistency), Notes/wiki/async-load-isolation.md.

Contracts verified:

- Navigation (n/p) remains fully active while a file loads; the user can move past a slow file.
- Placeholder scrolling is a strict no-op while loading.
- w, c, and resize retain their normal meanings while a load is pending.
- Completion messages are keyed by raw path and request identity.
- A completion updates only that path's cache and status.
- A completion changes the visible panel only when that path is still current.
- A->B->C navigation: A's completion arriving while C is current leaves C's panel unchanged; A is cached.
- Successful buffers remain retained for the session; no eviction.
- At most one load may be in flight for a raw path.
- Re-entering a loading path starts no second load and queues nothing.
- Late messages after cancellation are ignored.
- Decode/map work for the current file is separately gatable.
- While decode/map is held, ctrl+c, n, p, w, c, and resize remain actionable.

All generated artifacts live in this directory.

```bash
go build ./cmd/... ./internal/... && go vet ./cmd/... ./internal/... && echo GATES-OK
```

```output
GATES-OK
```

```bash
go test -count=1 ./cmd/... ./internal/... -timeout 120s | sed 's/[[:space:]][0-9.]*s$//'
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
ok  	vrg/internal/viewport
```

## Issue #25 gated model tests

The gated model tests (internal/app/load_isolation_test.go) verify navigation during loads, keyed late completions, one-load-per-path, cached revisits, post-cancellation rejection, and input responsiveness during the decode/map phase.

```bash
go test -count=1 -v ./internal/app/ -run '^TestLoadIsolation' -timeout 60s | sed 's/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
```

```output
=== RUN   TestLoadIsolationNavigationActiveWhileLoading
--- PASS: TestLoadIsolationNavigationActiveWhileLoading (0.00s)
=== RUN   TestLoadIsolationPlaceholderScrollNoOp
--- PASS: TestLoadIsolationPlaceholderScrollNoOp (0.00s)
=== RUN   TestLoadIsolationKeysNormalWhileLoading
--- PASS: TestLoadIsolationKeysNormalWhileLoading (0.00s)
=== RUN   TestLoadIsolationKeyedCompletionNonCurrentPath
--- PASS: TestLoadIsolationKeyedCompletionNonCurrentPath (0.00s)
=== RUN   TestLoadIsolationKeyedCompletionCurrentPath
--- PASS: TestLoadIsolationKeyedCompletionCurrentPath (0.00s)
=== RUN   TestLoadIsolationOneLoadPerPath
--- PASS: TestLoadIsolationOneLoadPerPath (0.00s)
=== RUN   TestLoadIsolationCachedRevisitNoReload
--- PASS: TestLoadIsolationCachedRevisitNoReload (0.00s)
=== RUN   TestLoadIsolationCacheRetainedAcrossMultipleVisits
--- PASS: TestLoadIsolationCacheRetainedAcrossMultipleVisits (0.00s)
=== RUN   TestLoadIsolationPostCancellationRejection
--- PASS: TestLoadIsolationPostCancellationRejection (0.00s)
=== RUN   TestLoadIsolationLateCompletionAfterQuit
--- PASS: TestLoadIsolationLateCompletionAfterQuit (0.00s)
=== RUN   TestLoadIsolationResponsiveNWhileFileGateHeld
--- PASS: TestLoadIsolationResponsiveNWhileFileGateHeld (0.00s)
=== RUN   TestLoadIsolationResponsivePWhileFileGateHeld
--- PASS: TestLoadIsolationResponsivePWhileFileGateHeld (0.00s)
=== RUN   TestLoadIsolationResponsiveWWhileFileGateHeld
--- PASS: TestLoadIsolationResponsiveWWhileFileGateHeld (0.00s)
=== RUN   TestLoadIsolationResponsiveCWhileFileGateHeld
--- PASS: TestLoadIsolationResponsiveCWhileFileGateHeld (0.00s)
=== RUN   TestLoadIsolationResponsiveResizeWhileFileGateHeld
--- PASS: TestLoadIsolationResponsiveResizeWhileFileGateHeld (0.00s)
=== RUN   TestLoadIsolationResponsiveCtrlCWhileFileGateHeld
--- PASS: TestLoadIsolationResponsiveCtrlCWhileFileGateHeld (0.00s)
PASS
ok  	vrg/internal/app
```

## Manual demonstration: large first file, small second file

The manual case demonstrates the load-isolation behavior with real files:

- A is a very large first file (slow to load).
- B is a small second file (fast to load).
- Press n immediately at startup.
- B appears while A is still loading.
- Press p.
- A shows content only once it has loaded.
- A must never show content before completion.

The demonstration uses a test program that injects a gated loader: A's load is held at a gate, B's load completes immediately. The program drives the model through the same Update flow the TUI uses, capturing the view at each step.

```bash
cp Notes/walkthroughs/025-04/code-walkthrough/demo_artifacts/manual_demo_test.go internal/app/manual_demo_test.go && go test -count=1 -v -tags manual_demo ./internal/app/ -run '^TestManualDemoLoadIsolation$' -timeout 30s; rm internal/app/manual_demo_test.go
```

```output
=== RUN   TestManualDemoLoadIsolation
Step 1 (A loading): panel shows Loading (correct)
Step 2 (n to B): B appears while A still loading (correct)
Step 3 (p back to A): A still loading, panel shows Loading (correct)
Step 4 (A completes): A shows content only after completion (correct)

Manual demonstration passed: A never showed content before completion.
--- PASS: TestManualDemoLoadIsolation (0.00s)
PASS
ok  	vrg/internal/app	0.024s
```
