## Issue 50: Post-audit re-verification — full suite, race, repeated PTY runs, and tidy/vuln gates

**Type**: AFK
**Blocked by**: Issues 36–49 — this is the closing pass over the composed post-audit implementation, mirroring Issue 35's role for the original set

### Parent PRD

`Notes/PRD-vrg.md` — audit source: `Notes/critiques/final-audit-vrg.md`, Overall assessment ("rerun the complete acceptance and PTY verification suite with deterministic handshakes")

### What to build

The single closing verification pass over the repository after all audit-fix issues have landed — the counterpart to Issue 35 for the audit cycle. No new product behavior is built here; this issue owns the proof that the fixes compose, no regression was introduced, and the gates the audit ran are green on the final state.

- **Create the permanent verification entry point** `scripts/verify.sh` (executable, runnable from a clean checkout), which runs the full gate set in order and fails on the first non-zero step:
  1. `go build ./...`
  2. `go vet ./...`
  3. `go test ./... -count=1`
  4. `CGO_ENABLED=1 go test -race ./... -count=1`
  5. `go test ./cmd/vrg -count=3` — the repeated subprocess/PTY package run
  6. `go mod verify`
  7. `go run golang.org/x/vuln/cmd/govulncheck@v1.5.0 ./...` — the exact pinned invocation the audit used
  8. `go mod tidy -diff` — must report no drift (the permanent tidy gate Issue 49 refers to)
- **Re-run the critical PTY/subprocess tests explicitly and uncached**: `go test -count=1 ./cmd/vrg` and `CGO_ENABLED=1 go test -race -count=1 ./cmd/vrg`, covering at minimum the outcome tests (`TestFatalExitWithResultsShowsOverlay`, `TestFatalExitNoOutputNamesExitCode`, `TestFatalExitNoOutputEscExits2`, `TestSignalDeathNamesSignal`, `TestStderrWarningWithSummaryShowsWarningOverlay`, `TestStderrContentFixture`), the replay tests (`TestReplayCtrlCAfterStderrDiagnostic`, `TestReplayQWhileSearchingAfterDiagnostic`, `TestReplayQWhileGateHeldAfterDiagnostic`, `TestReplayNormalQAfterCompletedStreamWithWarning`, `TestReplayControlledFailureWithEarlierDiagnostic`, `TestReplayFilenameWithNewlineAndESC`), and the cancellation/boundary tests (`TestQAgainstBlockedFakeRGExits130`, `TestCtrlCAgainstBlockedFakeRGExits130`, `TestNormalExitReapsChild`, `TestQDuringGateHeldPreparationExits130`, `TestInjectedControlledFailure`, `TestChildArgvAndWorkdir`, `TestStartFailureExit2`, `TestDualPipeBackpressure`, `TestStderrCapturedWithoutBlocking`). These must operate on the Issue 48 handshakes — no fixed `time.Sleep` synchronization remains in `cmd/vrg/outcome_test.go` or `cmd/vrg/replay_test.go`.
- **Re-run the five smoke scenarios** with the fake-rg PTY harness, reusing/extending `Notes/walkthroughs/035-03/code-walkthrough/smoke.py` (build the binary, run `python3 smoke.py`): successful browse → 0, no-results → 1, fatal → 2 (both `q` and `Esc` dismissal), cancellation → 130 (child terminated/reaped, terminal restored), help-only → 0 (bare `vrg`, `-h`, `--help`; exactly one help copy on stdout, empty stderr, sentinel fake rg never invoked). Special attention: the newly composed fatal diagnostics (Issues 36–37, 44) state the integrity/record-loss causes in the overlay and in stderr replay.
- **Record the evidence** in the walkthrough artifact `Notes/walkthroughs/050-01/code-walkthrough/walkthrough.md`: every command, its output/exit status, and the smoke results, as the closing review evidence for the audit cycle.

A regression discovered by this pass is fixed against the owning issue's contract — never by weakening or deleting a focused test.

### How to verify

- **Manual**: from a clean checkout, run `scripts/verify.sh` and record its output; run the smoke harness (`go build -o /tmp/vrg-smoke/vrg ./cmd/vrg && python3 Notes/walkthroughs/035-03/code-walkthrough/smoke.py`) and confirm exit statuses, terminal restoration, and — for fatal outcomes — that the composed diagnostics state the integrity/record-loss causes.
- **Automated**: `scripts/verify.sh` exits 0, which entails: `go test ./...` uncached pass, `cmd/vrg` race and repeated runs, clean `go mod tidy -diff`, and no reachable vulnerabilities from pinned `govulncheck` v1.5.0.

### Acceptance criteria

- [ ] Given a clean checkout after Issues 36–49, then `scripts/verify.sh` runs `go build ./...`, `go vet ./...`, `go test ./... -count=1`, `CGO_ENABLED=1 go test -race ./... -count=1`, `go test ./cmd/vrg -count=3`, `go mod verify`, `go run golang.org/x/vuln/cmd/govulncheck@v1.5.0 ./...`, and `go mod tidy -diff` — all green.
- [ ] Given the PTY/subprocess tests, then the named uncached and race runs pass with deterministic handshakes — no fixed sleeps.
- [ ] Given `go mod tidy -diff`, then it reports a clean manifest and is permanently owned by `scripts/verify.sh` (not a one-time walkthrough note).
- [ ] Given `go run golang.org/x/vuln/cmd/govulncheck@v1.5.0 ./...`, then no reachable vulnerabilities are reported.
- [ ] Given the five smoke scenarios, then each produces its contracted exit status with terminal restoration and correct stderr replay — including the improved fatal-outcome explanations.
- [ ] Given any regression found, then it is fixed against the owning issue's contract and the full suite rerun green.
- [ ] Given the completed pass, then all commands and results are recorded in `Notes/walkthroughs/050-01/code-walkthrough/walkthrough.md`.

### User stories addressed

- User stories 1–85: the composed post-audit implementation is proven to build, vet, test, and run end to end, with the audit's high-severity gaps closed.

---
