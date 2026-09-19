## Issue 50: Post-audit re-verification — full suite, race, repeated PTY runs, and tidy/vuln gates

**Type**: AFK
**Blocked by**: Issues 36–49 — this is the closing pass over the composed post-audit implementation, mirroring Issue 35's role for the original set

### Parent PRD

`Notes/PRD-vrg.md` — audit source: `Notes/critiques/final-audit-vrg.md`, Overall assessment ("rerun the complete acceptance and PTY verification suite with deterministic handshakes")

### What to build

The single closing verification pass over the repository after all audit-fix issues have landed — the counterpart to Issue 35 for the audit cycle. No new product behavior is built here; this issue owns the proof that the fixes compose, no regression was introduced, and the gates the audit ran are green on the final state.

- **Create the permanent verification entry point** `scripts/verify.sh` (executable, runnable from a clean checkout with the declared tool/network prerequisites), which runs the full gate set in order and fails on the first non-zero step:
  1. `go build ./...`
  2. `go vet ./...`
  3. `go build -tags vrg_testhooks ./cmd/vrg`
  4. `go vet -tags vrg_testhooks ./cmd/vrg`
  5. `go test ./... -count=1`
  6. `CGO_ENABLED=1 go test -race ./... -count=1`
  7. `go test ./cmd/vrg -count=3` — the repeated subprocess/PTY package run
  8. `go mod verify`
  9. `go run golang.org/x/vuln/cmd/govulncheck@v1.5.0 ./...` — the exact pinned invocation the audit used
  10. `go mod tidy -diff` — must report no drift (the permanent tidy gate Issue 49 refers to)
  Document that the pinned `govulncheck` step requires network access to fetch the tool and vulnerability database, or pre-populated module/tool and vulnerability-database caches. An offline clean machine without those caches does not satisfy the gate's environmental prerequisites; this must be reported as an environment failure, not a repository regression.
- **Re-run the critical PTY/subprocess tests explicitly and uncached**: `go test -count=1 ./cmd/vrg` and `CGO_ENABLED=1 go test -race -count=1 ./cmd/vrg`, covering at minimum the outcome tests (`TestFatalExitWithResultsShowsOverlay`, `TestFatalExitNoOutputNamesExitCode`, `TestFatalExitNoOutputEscExits2`, `TestSignalDeathNamesSignal`, `TestStderrWarningWithSummaryShowsWarningOverlay`, `TestStderrContentFixture`), the replay tests (`TestReplayCtrlCAfterStderrDiagnostic`, `TestReplayQWhileSearchingAfterDiagnostic`, `TestReplayQWhileGateHeldAfterDiagnostic`, `TestReplayNormalQAfterCompletedStreamWithWarning`, `TestReplayControlledFailureWithEarlierDiagnostic`, `TestReplayFilenameWithNewlineAndESC`), and the cancellation/boundary tests (`TestQAgainstBlockedFakeRGExits130`, `TestCtrlCAgainstBlockedFakeRGExits130`, `TestNormalExitReapsChild`, `TestQDuringGateHeldPreparationExits130`, `TestInjectedControlledFailure`, `TestChildArgvAndWorkdir`, `TestStartFailureExit2`, `TestDualPipeBackpressure`, `TestStderrCapturedWithoutBlocking`). These must operate on the Issue 48 handshakes. Across all `cmd/vrg` PTY helpers — including `runVrgWithKeys`, `runVrgKillChild`, `runVrgWithQuit`, `runVrgCancel`, and every `runVrgReplay` trigger callback — no fixed settling or inter-key `time.Sleep` synchronization may remain. Bounded condition-poll sleeps remain permissible only when each iteration checks an explicit condition.

A regression discovered by this pass is fixed against the owning issue's contract — never by weakening or deleting a focused test.

### Acceptance criteria

- [ ] Given a clean checkout after Issues 36–49 and the documented network/cache prerequisites, then `scripts/verify.sh` runs `go build ./...`, `go vet ./...`, `go build -tags vrg_testhooks ./cmd/vrg`, `go vet -tags vrg_testhooks ./cmd/vrg`, `go test ./... -count=1`, `CGO_ENABLED=1 go test -race ./... -count=1`, `go test ./cmd/vrg -count=3`, `go mod verify`, `go run golang.org/x/vuln/cmd/govulncheck@v1.5.0 ./...`, and `go mod tidy -diff` — all green; a missing network/cache prerequisite is identified separately from a code failure.
- [ ] Given the PTY/subprocess tests, then the named uncached and race runs pass with deterministic handshakes, and no fixed settling/inter-key delay remains in any `cmd/vrg` PTY helper; bounded condition-poll sleeps are allowed only for explicit conditions.
- [ ] Given `go mod tidy -diff`, then it reports a clean manifest and is permanently owned by `scripts/verify.sh`.
- [ ] Given `go run golang.org/x/vuln/cmd/govulncheck@v1.5.0 ./...`, then no reachable vulnerabilities are reported.
- [ ] Given the PTY and smoke fixtures, then fake-rg-owned handshake/ready/PID/argv/cwd variables use `FAKE_RG_*`; given the five smoke scenarios against the untagged production binary, then each produces its contracted exit status with terminal restoration and correct stderr replay — including the improved fatal-outcome explanations — and cancellation cleanup is proven externally without setting or depending on any `VRG_TEST_*` name.
- [ ] Given the smoke harness, then every key follows an observed preceding UI state, output draining follows completion/EOF, bounded polling only checks an explicit condition, and no `time.sleep` or other fixed settling delay is used as a progress proxy.
- [ ] Given any regression found, then it is fixed against the owning issue's contract and the full suite rerun green.

### User stories addressed

- User stories 1–85: the composed post-audit implementation is proven to build, vet, test, and run end to end, with the audit's high-severity gaps closed.

---
