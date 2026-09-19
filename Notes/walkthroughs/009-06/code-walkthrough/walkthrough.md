# Issue #9: error overlay and fatal outcomes

*2026-09-16T22:47:21Z by Showboat 0.6.1*
<!-- showboat-id: 0b9c8ad3-82f6-4c17-b8c0-1e2a7bd2f070 -->

Issue #9 lands the ripgrep-stream lifecycle matrix and the fatal/warning outcome contract: `internal/searchindex` validates every begin/match/end/summary transition per decoded raw path (orphaned matches retained with incomplete metadata, binary exclusion taking precedence over retention, summary final, the unterminated tail classified malformed), `Index.Integrity().Complete` reports stream integrity independently of the child's exit status, `internal/app`'s pure `DecideOutcome` maps process result × integrity × usable results to a presentation and a fixed exit status only `ctrl+c` can override to 130, and the modal error overlay renders escaped diagnostics in a single-line bordered box — scrollable with `up`/`down`, dismissed with `q`/`Esc` (both exiting outright when no underlying state exists). See `Notes/issues/009-error-overlay-and-fatal-outcomes.md`, `Notes/tasks/009-error-overlay-and-fatal-outcomes.md`, and the PRD sections "Result index, records, and stream integrity", "Outcome and exit-status contract", and "Colours, overlays, and key precedence" in `Notes/PRD-vrg.md`. All artifacts live in this directory.

## Gates — module integrity, build, vet

```bash
cd /home/chris/vrg && go mod verify && go build ./... && go vet ./... && echo GATES-OK
```

```output
all modules verified
GATES-OK
```

## SearchIndex — the lifecycle matrix

`internal/searchindex/lifecycle_test.go` is the first single table-driven lifecycle matrix in the codebase: `TestLifecycleMatrix` enumerates the 14×9 lifecycle cases (orphaned match retained with incomplete metadata, binary exclusion superseding retention, summary final, context lifecycle-neutral, …) and each case's expected retained files, incomplete flags, and `Integrity().Complete`; `TestTrailingUnterminatedRecordDisposition` proves the unterminated tail is classified malformed while a newline-terminated record is clean.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'LifecycleMatrix|TrailingUnterminated' ./internal/searchindex 2>&1 | grep -E '^(--- |ok|FAIL)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestLifecycleMatrix
--- PASS: TestTrailingUnterminatedRecordDisposition
ok  	vrg/internal/searchindex
```

## App — the outcome matrix

`internal/app/outcome_test.go` holds the single table-driven `DecideOutcome` matrix: every (process result × stream integrity × usable results) combination maps to a presentation and a fixed status — clean browse, complete no-results, fatal code/signal/stream outcomes exiting 2, warning stderr over browse or no-results, the retained-status rule for ordinary dismissals, and `ctrl+c` → 130. `TestGeneratedDiagnostics` pins the code/signal naming for silent failed children.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'Outcome|Generated' ./internal/app 2>&1 | grep -E '^(--- |ok|FAIL)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestOutcomeMatrix
--- PASS: TestGeneratedProcessDiagnostics
ok  	vrg/internal/app
```

## App — overlay mechanics and sink safety

`internal/app/overlay_test.go` pins the modal overlay: key routing while open (only `up`/`down` scroll, `Esc`/`q` dismiss, everything else ignored), clamped scroll bounds, grapheme-safe wrapping of long unbroken strings, the bordered box centred over the base frame, and complete retention of >1 MiB diagnostics. `internal/app/sinksafety_test.go` gains the overlay row so every diagnostic byte passes `safepresentation.EscapeDiagnostic` before reaching a cell.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'Overlay|Sink' ./internal/app 2>&1 | grep -E '^(--- |ok|FAIL)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestOverlayKeyRoutingAndScrolling
--- PASS: TestOverlayDismissKeys
--- PASS: TestOverlayCtrlCExits130
--- PASS: TestOverlayWrapsUnbrokenDiagnostic
--- PASS: TestOverlayKeepsCompleteDiagnostic
--- PASS: TestSinkSafetyTable
ok  	vrg/internal/app
```

## PTY — real-binary outcome coverage

`cmd/vrg/outcome_test.go` drives the real binary on a pty against scripted fake rg processes: a fatal exit with retained results and stderr, a silent fatal naming its code, signal death naming the signal, warning stderr over summary-only no-results, >1 MiB interleaved stderr drained without blocking, dismissal repainting the covered view, and the fixed-status / `ctrl+c` → 130 rules.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'Fatal|Signal|Stderr|CtrlC' ./cmd/vrg 2>&1 | grep -E '^(--- |ok|FAIL)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestFatalExitWithResultsShowsOverlay
--- PASS: TestFatalExitNoOutputNamesExitCode
--- PASS: TestFatalExitNoOutputEscExits2
--- PASS: TestSignalDeathNamesSignal
--- PASS: TestStderrWarningWithSummaryShowsWarningOverlay
--- PASS: TestStderrContentFixture
--- PASS: TestStderrCapturedWithoutBlocking
ok  	vrg/cmd/vrg
```

## Manual PTY — the real binary end to end

`pty_outcomes.py` builds five sessions against scripted fake rg binaries and asserts on the raw byte stream: fatal exit 3 with usable results overlays "boom" on the browse frame (`Esc` reveals it, `q` → 2); a bare `exit 2` names its code and exits 2 on `q` or `Esc`; a SIGKILLed rg names the signal, reveals the retained match's browse frame on `Esc`, exits 2 on `q`; `warn` stderr beside a complete summary-only stream dismisses to "No results found" with `q` → 1.

```bash
cd /home/chris/vrg && go build -o Notes/walkthroughs/009-06/code-walkthrough/vrg ./cmd/vrg && cd Notes/walkthroughs/009-06/code-walkthrough && python3 pty_outcomes.py
```

```output
fatal-results: exit 3 + stderr 'boom' + 2 matches -> overlay over browse; Esc reveals the content; q exits 2
exit2        : bare 'exit 2' -> generated 'code 2' diagnostic; q exits 2
exit2-esc    : the overlay-only fatal outcome exits 2 on Esc too
signal       : SIGKILL mid-stream -> overlay names the signal; Esc reveals browse; q exits 2
warn-summary : 'warn' + summary-only rg-1 -> warning overlay; Esc shows 'No results found'; q exits 1
OK
```

## Full suite — `go test ./...` and the race detector

```bash
cd /home/chris/vrg && go test -count=1 ./... 2>&1 | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//' && CGO_ENABLED=1 go test -count=1 -race ./... 2>&1 | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
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

## Flake check — `go test ./cmd/vrg -count=3`

The PTY-heavy cmd/vrg package passes three consecutive runs.

```bash
cd /home/chris/vrg && go test -count=3 ./cmd/vrg 2>&1 | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
ok  	vrg/cmd/vrg
```
