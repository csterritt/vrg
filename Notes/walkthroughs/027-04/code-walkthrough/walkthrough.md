# Issue #27: Explicit reload (r)

*2026-09-12T13:33:45Z by Showboat 0.6.1*
<!-- showboat-id: 32d47fbc-a8d9-4039-98cd-66c39a2ffd8f -->

Walkthrough for Issue #27 (Notes/tasks/027-explicit-reload-r.md), implementing explicit reload in vrg. Pressing r rereads the current file exactly once without rerunning ripgrep, without changing cursor stops, and without revealing a match. The reload shows the Loading… placeholder while pending, replaces stale content on failure with (unreadable), supports retries and the one-stop index, and discards superseded layouts safely through content revisions and the Issue #17 installation guard. References: Notes/PRD-vrg.md (File loading, cache, reload, and selection consistency), Notes/wiki/explicit-reload.md.

Contracts verified:

- r shows Loading… and issues exactly one reread of the current file.
- r does not rerun ripgrep or change cursor stops.
- A duplicate r while the path's load is already in flight is dropped, not queued.
- Re-entry (n/p) while a reload is in flight is dropped under the one-load-per-path rule.
- The placeholder transition (Loading… to content or (unreadable)) is the only completion signal; after completion, another r starts a new load.
- A reload preserves the cursor and logical viewport anchor, clamped to new content.
- The anchor is asserted after the new revision's matching prepared layout installs, not against the old revision's layout.
- A failed reload replaces the old display with (unreadable) and shows the current-file failure overlay.
- A second consecutive failure appends exactly one new diagnostic occurrence and preserves the reader's overlay scroll position.
- r works with a one-stop index.
- A simulated disk change does not trigger reload; the cache is stable until r is pressed.
- The filename row keeps identifying the path after a reload.
- A reload creates a new content revision, invalidating stale cached layouts.
- A gated pre-reload layout released after reload completion is discarded (key mismatch) without replacing reloaded content or its anchor.

The injected-loader model tests are the authoritative deterministic verification. The manual route at the end demonstrates the behavior with an injected gated loader.

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

## Gated reload tests

The gated reload tests (internal/app/reload_test.go) verify the reload lifecycle: Loading… placeholder, exactly one reread, no cursor-stop changes, dropped duplicate reloads, dropped re-entry while the same path is loading, anchor preservation clamped to new content, failure replacement with (unreadable), the current-file failure overlay, second-failure append with overlay scroll preserved, the one-stop index route, no reload on simulated disk change, and the filename row retaining the path.

```bash
go test -count=1 -v ./internal/app/ -run '^TestReload' -timeout 60s | sed 's/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
```

```output
=== RUN   TestReloadShowsLoadingAndRereads
--- PASS: TestReloadShowsLoadingAndRereads (0.00s)
=== RUN   TestReloadDropsDuplicateWhileInFlight
--- PASS: TestReloadDropsDuplicateWhileInFlight (0.00s)
=== RUN   TestReloadDropsReentryWhileInFlight
--- PASS: TestReloadDropsReentryWhileInFlight (0.00s)
=== RUN   TestReloadPreservesAnchorClamped
--- PASS: TestReloadPreservesAnchorClamped (0.00s)
=== RUN   TestReloadPreservesAnchorClampedOnShrink
--- PASS: TestReloadPreservesAnchorClampedOnShrink (0.00s)
=== RUN   TestReloadFailureReplacesWithUnreadable
--- PASS: TestReloadFailureReplacesWithUnreadable (0.00s)
=== RUN   TestReloadSecondFailureAppends
--- PASS: TestReloadSecondFailureAppends (0.00s)
=== RUN   TestReloadOneStopIndex
--- PASS: TestReloadOneStopIndex (0.00s)
=== RUN   TestReloadFilenameRowIdentifiesPath
--- PASS: TestReloadFilenameRowIdentifiesPath (0.00s)
=== RUN   TestReloadRevisionSupersedesGatedLayout
--- PASS: TestReloadRevisionSupersedesGatedLayout (0.00s)
=== RUN   TestReloadPreservesAnchorAfterLayoutInstall
--- PASS: TestReloadPreservesAnchorAfterLayoutInstall (0.00s)
PASS
ok  	vrg/internal/app
```

```bash
go test -count=1 -v ./internal/app/ -run '^TestNoReloadOnDiskChange$' -timeout 30s | sed 's/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
```

```output
=== RUN   TestNoReloadOnDiskChange
--- PASS: TestNoReloadOnDiskChange (0.00s)
PASS
ok  	vrg/internal/app
```

## Revision-supersession tests

The revision-supersession tests verify that a reload creates a new content revision, and a gated pre-reload layout (from a resize) released after the reload completes is discarded (key mismatch) without replacing the reloaded content or its anchor. The reload's matching layout installs and shows the new content. The anchor is asserted after the new revision's matching prepared layout installs, not against the old revision's layout.

```bash
go test -count=1 -v ./internal/app/ -run '^TestReload(RevisionSupersedesGatedLayout|PreservesAnchorAfterLayoutInstall)$' -timeout 30s | sed 's/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
```

```output
=== RUN   TestReloadRevisionSupersedesGatedLayout
--- PASS: TestReloadRevisionSupersedesGatedLayout (0.00s)
=== RUN   TestReloadPreservesAnchorAfterLayoutInstall
--- PASS: TestReloadPreservesAnchorAfterLayoutInstall (0.00s)
PASS
ok  	vrg/internal/app
```

```bash
cp Notes/walkthroughs/027-04/code-walkthrough/demo_artifacts/manual_demo_test.go internal/app/manual_demo_test.go && go test -count=1 -v -tags manual_demo ./internal/app/ -run '^TestManualDemoExplicitReload$' -timeout 30s 2>&1 | sed 's/[[:space:]]0\.[0-9]*s//g; s/([0-9.]*s)/(0.00s)/g'; rm internal/app/manual_demo_test.go
```

```output
=== RUN   TestManualDemoExplicitReload
Step 1 (open file): panel shows line-00 (correct)
Step 2 (scroll): ViewportOffset = 5 (correct)
Step 3 (append externally): display unchanged (cache stable, correct)
Step 4 (press r): new content at same top position (anchor preserved, correct)
Step 5 (delete + r): panel shows (unreadable) (correct)
Step 6 (restore + r): panel shows content again (correct)
Step 7 (reload one-stop): panel shows reloaded content (correct)

Manual demonstration passed: explicit reload behavior matches the Issue #27 contracts.
--- PASS: TestManualDemoExplicitReload (0.00s)
PASS
ok  	vrg/internal/app
```
