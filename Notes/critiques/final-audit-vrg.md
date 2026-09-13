# Audit Report: VRG — Terminal UI for ripgrep

Parent PRD: `Notes/PRD-vrg.md` (revision 6)  
Date: 2026-09-13  
Files in scope: 78

## Summary

The implementation has strong automated coverage and passes build, vet, uncached tests, race tests, repeated subprocess tests, module verification, and `govulncheck`. It is not safe to consider production-complete as-is: seven high-severity findings can suppress required diagnostics, calculate the viewport against the wrong width, render Unicode highlights incorrectly, make diagnostics inaccessible, violate the documented responsiveness target, or alter load intent when a reload request should be dropped.

## Critical findings

None.

## High findings

### 1. Stream-integrity failures are not explained and discard other diagnostics

**Location**: `internal/app/app.go:255-280`  
**Category**: Logic / Consistency  
**Problem**: `DecideOutcome` correctly classifies incomplete stream integrity as fatal, but the fatal branch ignores `RecordLossDiagnostics` and has no integrity diagnostic. With no ripgrep stderr, an incomplete stream caused by a missing summary or missing `end` is presented as `ripgrep exited with code 0`; with stderr, only stderr is shown. This violates the PRD requirement to report incomplete metadata and include stream-integrity and record-loss notes.  
**Suggestion**: Carry structured integrity diagnostics from `searchindex` into `OutcomeInput`, compose process, integrity, stderr, and record-loss diagnostics in every fatal branch, and add acceptance tests that assert the actual explanation rather than only overlay kind and exit status.

---

### 2. Anonymous oversized records can produce an empty overlay or no warning

**Location**: `internal/app/app.go:283-304`, `internal/app/app.go:316-332`  
**Category**: Logic  
**Problem**: `recordLossDiagnostics` emits malformed and unknown counts plus only per-path oversized messages. It never emits the required aggregate oversized count. If path recovery fails, an oversized record with zero results produces a fatal but empty overlay; with usable results, the skipped record can produce no overlay or replay diagnostic at all.  
**Suggestion**: Always emit `N oversized record(s) skipped`, then append recoverable per-path details. Add outcome tests for anonymous oversized records both with and without usable results.

---

### 3. Installed viewports use terminal width instead of content-panel width

**Location**: `internal/app/app.go:1026-1040`, `internal/app/app.go:1385-1420`  
**Category**: Logic / Consistency  
**Problem**: `LayoutKey` correctly computes row-model width from terminal width minus file-list width and separator, but `LayoutReadyMsg` installs the viewport with `viewport.TextWidth(m.width, ...)`. The viewport therefore pans, clips, reveals, pads, and places indicators as if the file list did not exist. The row model and viewport disagree on their width, and composed rows can exceed the terminal. Tests in `internal/app/reveal_horizontal_test.go:68-94` explicitly encode the incorrect full-terminal calculation, masking the defect.  
**Suggestion**: Install the viewport with `msg.Key.TextWidth` (or the same shared panel-width calculation used by `LayoutKey`) and update app-level horizontal tests to include list width and separator.

---

### 4. Final rendering abandons the shared grapheme/cell model

**Location**: `internal/app/app.go:3312-3395`, `internal/theme/theme.go:177-223`  
**Category**: Logic / Consistency  
**Problem**: FileBuffer and Viewport produce cell-based, grapheme-aware ranges, but `renderLineWithHighlights`, `cellToBytePos`, `visibleWidth`, and Theme overlay sizing revert to one rune equals one cell. A two-cell CJK highlight can consume the following character; combining sequences can be styled or truncated separately from their base; padding and overlay widths are wrong for wide text. This breaks the PRD's single consistent grapheme/cell policy at the final output boundary.  
**Suggestion**: Render directly from `Line.Clusters` and cell spans, with one shared ANSI-aware cell-width helper based on the same grapheme policy. Add composed-view assertions for CJK, combining clusters, emoji ZWJ sequences, and wide diagnostic/path text.

---

### 5. Browse rendering scans and allocates the complete match index every frame

**Location**: `internal/app/app.go:2917-2936`, `internal/app/app.go:2938-3013`  
**Category**: Best practices / Logic  
**Problem**: Every `View()` calls `m.index.Stops()` (copying every stop) and `groupByFile` (scanning and allocating groups for the entire index) before rendering a visible list window. This directly violates the requirement that frame rendering not scan the whole file list and is likely to harm responsiveness at the documented scale of about 100,000 matched lines. The file-list provider test does not detect this because the complete grouping still runs first.  
**Suggestion**: Prepare immutable file groups, current-file indexes, and display metadata once after search completion; render only the visible file range. Strengthen the cost guard so accessing/copying all stops fails the test.

---

### 6. Error overlays discard their middle instead of allowing full scrolling

**Location**: `internal/app/app.go:2384-2459`  
**Category**: Logic  
**Problem**: Non-help overlays longer than the visible height are compressed to head + ellipsis + tail before scrolling. The omitted middle can never be reached, contradicting the requirement that diagnostics wrap and remain vertically scrollable and accessible at usable sizes. This is especially damaging for large ripgrep stderr output or appended errors.  
**Suggestion**: Keep all wrapped rows and clamp `overlayScroll` over the complete row set. If head/tail summarization is desired, make it an explicit alternate view rather than destructive preprocessing.

---

### 7. A dropped `r` request still changes revision and reveal intent

**Location**: `internal/app/app.go:1305-1318`, `internal/app/app.go:2140-2177`, `internal/app/app.go:2852-2873`  
**Category**: Logic / Concurrency  
**Problem**: `handleReload` marks the path as reloading, switches to `IntentReloadAnchor`, and changes presentation before `startLoad` checks whether that path already has an in-flight load. If `startLoad` drops the request, the original startup/navigation completion is later misclassified as a reload, its revision is incremented, and its pending destination reveal can be replaced by anchor preservation. Pressing `r` during the initial or navigation load can therefore suppress the required match reveal even though the reload did not start.  
**Suggestion**: Make the one-load-per-path admission check atomic with reload-state mutation. Only set reload flags, presentation, and intent after a new request has actually been accepted. Add a test for `r` during startup/navigation loading, not only a second `r` during an accepted reload.

## Medium findings

### 8. Standalone combining clusters do not receive a real fallback cell

**Location**: `internal/filebuffer/filebuffer.go:203-218`, `internal/filebuffer/filebuffer.go:382-399`  
**Category**: Logic  
**Problem**: The code assigns a synthetic width of one only to `clusterCells`, but leaves the cluster width and display text unchanged and advances `cellPos` by the original zero width. A following cluster can overlap the supposed fallback, and the renderer has no actual independent cell to paint. Tests assert a non-zero highlight range but do not prove a visible fallback cell exists.  
**Suggestion**: Materialize a one-cell display fallback and propagate its width consistently through clusters, byte mappings, wrapping, clipping, highlights, and rendering. Add a composed terminal-output test followed by another cluster.

---

### 9. `context` after `summary` is accepted despite the final-record contract

**Location**: `internal/searchindex/searchindex.go:487-526`, `internal/searchindex/lifecycle_test.go:90-100`  
**Category**: Logic / Consistency  
**Problem**: The PRD and Issue 9 state that the summary must be the final record and any record after it is an integrity failure. The implementation exempts `context`, and the lifecycle test explicitly expects that contradictory behavior.  
**Suggestion**: Mark every post-summary record as an integrity failure while continuing to ignore context payload/lifecycle semantics before summary. Correct the contradictory test row.

---

### 10. Production binaries expose test controls and busy-spin file watchers

**Location**: `cmd/vrg/main.go:76-167`  
**Category**: Security / Best practices  
**Problem**: Environment variables can make the released binary truncate or append arbitrary user-writable files, inject diagnostics/failures, hold result preparation indefinitely, and start tight polling loops with no sleep or cancellation. These are test harness capabilities compiled into normal production behavior.  
**Suggestion**: Move these seams behind test-only build constraints or inject them from test code through a dedicated harness. If any polling remains, make it cancellable and non-spinning.

---

### 11. Bubble Tea runtime errors bypass common diagnostic replay

**Location**: `cmd/vrg/main.go:169-207`  
**Category**: Logic / Consistency  
**Problem**: When `program.Run()` returns an error, the function writes that error directly and returns before reading the final model or replaying diagnostics already collected by the session. This violates the common post-restoration, in-order replay contract for controlled application failures. A failed final-model type assertion also exits silently.  
**Suggestion**: Route runtime failures through one shutdown result that retains the latest model diagnostics, appends the application failure once, restores/cleans up, and then performs the common replay. Emit a diagnostic for an invalid final model.

---

### 12. Read-failure diagnostics do not single-line embedded filenames

**Location**: `internal/filebuffer/filebuffer.go:111-119`, `internal/app/app.go:1291-1304`  
**Category**: Security / Consistency  
**Problem**: `os.ReadFile` returns a `PathError` containing the raw filename. App passes `err.Error()` directly to `EscapeDiagnostic`, which preserves newlines as diagnostic boundaries. A filename containing a newline therefore becomes multiple diagnostic lines instead of the required single-line escaped filename. Controls are escaped, but filename/diagnostic structure is not preserved.  
**Suggestion**: Construct file-load diagnostics from a separately `EscapePath`-escaped path plus a sanitized reason that does not repeat the raw path; test newline, tab, invalid UTF-8, and ESC filenames through actual load failures.

---

### 13. Critical PTY tests rely on fixed sleeps

**Location**: `cmd/vrg/outcome_test.go:18-58`, `cmd/vrg/replay_test.go:176-184`, `cmd/vrg/replay_test.go:221-234`, `cmd/vrg/replay_test.go:315-326`  
**Category**: Best practices / Logic  
**Problem**: Issue 35 explicitly requires the critical PTY/subprocess tests to run without relying on sleeps, but the outcome and replay harnesses use fixed 100–500 ms delays to assume model transitions have completed. This can hide ordering bugs and become flaky under load.  
**Suggestion**: Add application-side acknowledgements for each state transition/key processing boundary and wait on those handshakes instead of elapsed time.

## Low findings

### 14. Dependency manifests are not tidy and retain unused UI dependencies

**Location**: `go.mod:5-27`, `go.sum:1-54`  
**Category**: Best practices / Consistency  
**Problem**: `go mod tidy -diff` reports substantial drift: direct imports are marked indirect, required test dependencies are absent from the declared graph, and unused direct requirements for Bubbles and Lip Gloss would be removed because the implementation does not import them. This weakens dependency clarity and conflicts with the stated stack.  
**Suggestion**: Decide whether Bubbles/Lip Gloss are genuinely required. Either use them as intended or remove them, then commit a clean `go mod tidy` result and require `go mod tidy -diff` in verification.

## No findings

- No known reachable vulnerabilities were reported by `govulncheck` (`golang.org/x/vuln/cmd/govulncheck@v1.5.0`).
- No command/shell injection was found in user-controlled search arguments; ripgrep is launched with an argument vector and a mandatory `--` separator.
- The shared path/content/diagnostic sanitizers consistently escape raw terminal control bytes in the reviewed primary output paths.
- Root validation, ripgrep argument ordering, cumulative unrestricted-flag enforcement, binary exclusion, and the core circular cursor logic are consistent with their acceptance criteria.
- No authentication or authorization concerns apply to this local terminal application.

## Verification performed

- `go build ./...` — passed.
- `go vet ./...` — passed.
- `go test ./... -count=1` — passed, including PTY/subprocess tests.
- `CGO_ENABLED=1 go test -race ./... -count=1` — passed.
- `go test ./cmd/vrg -count=3` — passed.
- `go mod verify` — passed.
- `govulncheck ./...` via pinned `v1.5.0` — no vulnerabilities found.
- `go mod tidy -diff` — reported the dependency-manifest drift described in Low finding 14.

## Overall assessment

VRG should not be treated as production-complete until the high findings are resolved. The most urgent work is to make fatal/record-loss diagnostics complete, unify layout and rendering around the actual content-panel cell width, eliminate whole-index work from `View()`, and fix dropped-reload intent mutation; after those fixes, rerun the complete acceptance and PTY verification suite with deterministic handshakes.
