# Issue #9: Error overlay and fatal outcomes

*2026-09-23T20:12:25Z by Showboat 0.6.1*
<!-- showboat-id: 4091bfb9-0a47-41ca-aac9-a59db7af6726 -->

Walkthrough for [Issue #9](../../../issues/009-error-overlay-and-fatal-outcomes.md), implementing the error overlay and fatal-outcome contract per `Notes/PRD-vrg.md` (*Result index, records, and stream integrity* — the lifecycle matrix; *Outcome and exit-status contract*). Stream integrity is validated independently of process exit status through a begin/match/end lifecycle per file plus summary framing; integrity failures and fatal process outcomes fix exit status 2 while retained matches stay usable. Diagnostics — a generated process-failure line, sanitized stderr, and the integrity failures — appear in a modal overlay over browse or no-results, or alone when nothing is usable; stderr under a benign exit becomes a warning overlay that changes neither the screen nor the status. All generated artifacts live in this directory: the built `vrg` binary, the four `demo-*.sh` harnesses, and their `manual-*/` tmux session captures. Test durations are stripped so the document verifies cleanly.

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

## SearchIndex lifecycle tests

`internal/searchindex/lifecycle_test.go` pins the stream-integrity contract. `TestLifecycleMatrix` is the table of every transition — begin/match/end on open and closed files, orphaned and duplicate records, binary exclusion taking precedence over orphan retention, `text`/`bytes` path identity sharing one lifecycle, interleaved files staying independent, context records ignored, summary positioning (exactly one, final), missing and second summaries, records after summary, and the trailing unterminated record counted as malformed *and* incomplete. `TestFeedDecodesTheWholeStream` proves `Index.Feed` consumes every byte of a valid stream; `TestEmptyStreamIntegrity` proves an empty stream fails integrity.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestLifecycleMatrix|TestFeedDecodesTheWholeStream|TestEmptyStreamIntegrity' ./internal/searchindex 2>&1 | grep -vE '^(=== RUN|    --- (PASS|FAIL))' | sed -E 's/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//'
```

```output
--- PASS: TestLifecycleMatrix
--- PASS: TestFeedDecodesTheWholeStream
--- PASS: TestEmptyStreamIntegrity
PASS
ok  	vrg/internal/searchindex
```

## App outcome matrix

`internal/app/outcome_test.go` drives one table through `Update` covering every required outcome: rg-0/rg-1 complete streams browse with status 0, an intact empty stream shows no-results at 1, fatal exits and signal deaths keep usable results browseable under an error overlay with fixed status 2, fatal-without-results shows the overlay alone, integrity failures fix status 2 whether or not results survive, benign-exit stderr becomes a warning overlay that leaves status untouched, `ctrl+c` overrides any fixed status to 130, and `Esc` is a no-op in base states. `overlay_test.go` adds the modal behavior: `q`/`Esc` dismiss non-fatal overlays (fatal-only exits instead), up/down scroll with clamping, unrelated keys are swallowed while the overlay owns input (`c` never reaches the colour toggle), and long unbroken diagnostics hard-wrap inside the border.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestOutcomeMatrix|TestOverlay|TestFailedProcessWithoutStderrGetsGeneratedDiagnostic' ./internal/app 2>&1 | grep -vE '^(=== RUN|    --- (PASS|FAIL))' | sed -E 's/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//'
```

```output
--- PASS: TestOutcomeMatrix
--- PASS: TestOverlayScrollsWithUpDown
--- PASS: TestOverlayDismissKeys
--- PASS: TestOverlayCtrlCExits130
--- PASS: TestOverlayIgnoresOtherKeys
--- PASS: TestOverlayWrapsUnbrokenDiagnostic
--- PASS: TestFailedProcessWithoutStderrGetsGeneratedDiagnostic
PASS
ok  	vrg/internal/app
```

## PTY integration tests

`cmd/vrg/pty_test.go` runs the real binary on a pseudo-terminal. `TestPTYNonZeroExitBrowseOverlayExits2` serves a complete valid stream then exits 3: the overlay opens over browse, q dismisses, q quits 2 — results survive the fatal exit. `TestPTYStderrContentOverlayHeadAndTail` floods 1 MiB of stderr interleaved with a valid stream — head and tail lines both reach the overlay, proving concurrent drainage never blocked stdout — then dismissal reveals browse and q exits 0. `TestDualPipeDrainageAtBoundary` covers the pipe-capacity boundary, and the sink-safety table (`internal/app/sinksafety_test.go`) feeds OSC/CSI/C0/C1/DEL/CR/invalid-UTF-8 diagnostics through the overlay.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestPTYNonZeroExitBrowseOverlayExits2|TestPTYStderrContentOverlayHeadAndTail|TestDualPipeDrainageAtBoundary' ./cmd/vrg 2>&1 | grep -vE '^(=== RUN|    --- (PASS|FAIL))' | sed -E 's/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//'
```

```output
--- PASS: TestDualPipeDrainageAtBoundary
--- PASS: TestPTYNonZeroExitBrowseOverlayExits2
--- PASS: TestPTYStderrContentOverlayHeadAndTail
PASS
ok  	vrg/cmd/vrg
```

## Manual check — fatal exit with usable results

`demo-fatal-results.sh` (checked into this directory) runs the freshly built `vrg` on a real tmux PTY in a fixture directory whose `fakebin/rg` emits two valid matches — a complete stream — then writes `boom` to stderr and exits 3. The error overlay opens over the browse view naming the exit status and carrying the stderr line; the first `q` dismisses to browse, the second quits with the fixed fatal status 2. The harness captures the composed screen at each step.

```bash
cd /home/chris/vrg && go build -o Notes/walkthroughs/009-06/code-walkthrough/vrg ./cmd/vrg && cd Notes/walkthroughs/009-06/code-walkthrough && ./demo-fatal-results.sh
```

```output
exit=2
--- screen with overlay ---
f1.txt  ── f1.txt ──────────────────────────────────────────────────────────────
f2.txt  1  hit one
                           ┌────────────────────────┐
                           │rg failed: exit status 3│
                           │boom                    │
                           └────────────────────────┘
--- screen after dismissal ---
f1.txt  ── f1.txt ──────────────────────────────────────────────────────────────
f2.txt  1  hit one
```

## Manual check — fatal exit with no results

`demo-fatal-empty.sh` runs `vrg` with an `rg` that emits nothing and exits 2. With empty stderr the app generates the `rg failed: exit status 2` diagnostic; the missing summary is the second diagnostic. Fatal with no usable results means the overlay stands alone on a blank screen, and `Esc` exits directly with status 2.

```bash
cd /home/chris/vrg/Notes/walkthroughs/009-06/code-walkthrough && ./demo-fatal-empty.sh
```

```output
exit=2
--- fatal-only overlay ---
                           ┌────────────────────────┐
                           │rg failed: exit status 2│
                           │missing summary         │
                           └────────────────────────┘
```

## Manual check — SIGKILL mid-stream

`demo-sigkill.sh` uses an `rg` that emits `begin` and `match` then `kill -9`s itself before any end or summary. The signal death is fatal and generated as `rg failed: signal: killed`; the orphaned match is retained as usable-but-incomplete, so the overlay also lists `missing end for f1.txt` and `missing summary`. `Esc` dismisses to browse — the killed process still yielded a browsable hit — and `q` exits 2.

```bash
cd /home/chris/vrg/Notes/walkthroughs/009-06/code-walkthrough && ./demo-sigkill.sh
```

```output
exit=2
--- screen with overlay ---
f1.txt  ── f1.txt ──────────────────────────────────────────────────────────────
        1  hit
                          ┌─────────────────────────┐
                          │rg failed: signal: killed│
                          │missing end for f1.txt   │
                          │missing summary          │
                          └─────────────────────────┘
--- screen after dismissal ---
f1.txt  ── f1.txt ──────────────────────────────────────────────────────────────
        1  hit
```

## Manual check — warning under a benign exit

`demo-warning.sh` uses an `rg` that writes `warn` to stderr, emits a complete summary-only stream, and exits 0. A benign exit demotes stderr to a warning diagnostic: the warning overlay opens over the no-results screen, `q` dismisses to "No results found", and the second `q` exits with the ordinary no-results status 1 — the warning changes neither the screen nor the status.

```bash
cd /home/chris/vrg/Notes/walkthroughs/009-06/code-walkthrough && ./demo-warning.sh
```

```output
exit=1
--- screen with warning overlay ---
                                     ┌────┐
                                     │warn│
                                No re└────┘found
--- screen after dismissal ---
                                No results found
```

## Result

Issue #9 is verified. `internal/searchindex` validates the full begin/match/end-plus-summary lifecycle independently of process exit, retaining orphaned matches as incomplete and letting binary exclusion take precedence. `internal/app` resolves every completed search through the single `decideOutcome` table: usable results browse under an error overlay at fixed status 2 on fatal exits, signal deaths, or integrity failures; fatal-without-results shows the overlay alone; benign-exit stderr becomes a warning overlay over the ordinary screen at its ordinary status; and `ctrl+c` overrides to 130 everywhere. The modal overlay scrolls, wraps unbroken diagnostics inside its border, owns the keyboard while open, and every diagnostic passes through `present.Diagnostic` before rendering. Deferred by design: the record-loss count is passed into `decideOutcome` but unused until Issue #10.
