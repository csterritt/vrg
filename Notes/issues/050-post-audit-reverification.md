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
- **Modernize and re-run the five smoke scenarios** with the fake-rg PTY harness in `Notes/walkthroughs/035-03/code-walkthrough/smoke.py`, against the **untagged production binary** built by `go build -o /tmp/vrg-smoke/vrg ./cmd/vrg`: successful browse → 0, no-results → 1, fatal → 2 (both `q` and `Esc` dismissal), cancellation → 130 (child terminated/reaped, terminal restored), help-only → 0 (bare `vrg`, `-h`, `--help`; exactly one help copy on stdout, empty stderr, sentinel fake rg never invoked). This is distinct from the `vrg_testhooks`-tagged binary used by focused `cmd/vrg` subprocess tests. Rename fixture-owned fake-rg variables off the `VRG_TEST_` prefix throughout the PTY/smoke fixtures: `VRG_TEST_HANDSHAKE`, `VRG_TEST_READY`, `VRG_TEST_PID`, `VRG_TEST_ARGV`, and `VRG_TEST_CWD` become corresponding `FAKE_RG_*` names (for example, `FAKE_RG_PID_FILE`). Names consumed by tagged vrg acknowledgement seams remain in Issue 45's explicit hook manifest. The production smoke must neither set nor depend on any `VRG_TEST_*` name. For cancellation, the fake `rg` fixture may receive the renamed `FAKE_RG_*` variables and expose its PID/process group; the parent harness must externally observe that the process and process group are gone, PTY completion/EOF has occurred, and terminal state is restored.
- **Make the smoke harness condition-driven.** Replace fixed pre-key, inter-key, process-exit, and post-exit sleeps with bounded waits for externally observable PTY output/state markers, process or pipe completion, and EOF. A fake-child ready file may prove that the fixture started, but it is not sufficient evidence that the App entered a state or processed a prior key. Send every key only after the preceding expected UI state/output is observed. Bounded condition polling may be used only to fail a missing explicit condition, never as an assumed settling interval; draining is completion/EOF-driven. Add a static/review assertion that the harness contains no `time.sleep` call and no other fixed delay used as a progress proxy. Special attention: the newly composed fatal diagnostics (Issues 36–37, 44) state the integrity/record-loss causes in the overlay and in stderr replay.
- **Record the evidence** in the walkthrough artifact `Notes/walkthroughs/050-01/code-walkthrough/walkthrough.md`: every command, its output/exit status, and the smoke results, as the closing review evidence for the audit cycle.

A regression discovered by this pass is fixed against the owning issue's contract — never by weakening or deleting a focused test.

### How to verify

- **Manual**: from a clean checkout with network access or the documented warm caches, run `scripts/verify.sh` and record its output; build and run the untagged production smoke exactly as `go build -o /tmp/vrg-smoke/vrg ./cmd/vrg && python3 Notes/walkthroughs/035-03/code-walkthrough/smoke.py`. Confirm both untagged and `vrg_testhooks` build/vet variants, exit statuses, external child/process-group termination, PTY EOF, terminal restoration, and — for fatal outcomes — that the composed diagnostics state the integrity/record-loss causes. Confirm fixture variables use `FAKE_RG_*`, the smoke environment contains no `VRG_TEST_*` controls, and the harness uses observable conditions rather than settling sleeps.
- **Automated**: `scripts/verify.sh` exits 0, which entails: `go test ./...` uncached pass, `cmd/vrg` race and repeated runs, clean `go mod tidy -diff`, and no reachable vulnerabilities from pinned `govulncheck` v1.5.0.

### Acceptance criteria

- [ ] Given a clean checkout after Issues 36–49 and the documented network/cache prerequisites, then `scripts/verify.sh` runs `go build ./...`, `go vet ./...`, `go build -tags vrg_testhooks ./cmd/vrg`, `go vet -tags vrg_testhooks ./cmd/vrg`, `go test ./... -count=1`, `CGO_ENABLED=1 go test -race ./... -count=1`, `go test ./cmd/vrg -count=3`, `go mod verify`, `go run golang.org/x/vuln/cmd/govulncheck@v1.5.0 ./...`, and `go mod tidy -diff` — all green; a missing network/cache prerequisite is identified separately from a code failure.
- [ ] Given the PTY/subprocess tests, then the named uncached and race runs pass with deterministic handshakes, and no fixed settling/inter-key delay remains in any `cmd/vrg` PTY helper; bounded condition-poll sleeps are allowed only for explicit conditions.
- [ ] Given `go mod tidy -diff`, then it reports a clean manifest and is permanently owned by `scripts/verify.sh` (not a one-time walkthrough note).
- [ ] Given `go run golang.org/x/vuln/cmd/govulncheck@v1.5.0 ./...`, then no reachable vulnerabilities are reported.
- [ ] Given the PTY and smoke fixtures, then fake-rg-owned handshake/ready/PID/argv/cwd variables use `FAKE_RG_*`; given the five smoke scenarios against the untagged production binary, then each produces its contracted exit status with terminal restoration and correct stderr replay — including the improved fatal-outcome explanations — and cancellation cleanup is proven externally without setting or depending on any `VRG_TEST_*` name.
- [ ] Given the smoke harness, then every key follows an observed preceding UI state, output draining follows completion/EOF, bounded polling only checks an explicit condition, and no `time.sleep` or other fixed settling delay is used as a progress proxy.
- [ ] Given any regression found, then it is fixed against the owning issue's contract and the full suite rerun green.
- [ ] Given the completed pass, then all commands and results are recorded in `Notes/walkthroughs/050-01/code-walkthrough/walkthrough.md`.

### User stories addressed

- User stories 1–85: the composed post-audit implementation is proven to build, vet, test, and run end to end, with the audit's high-severity gaps closed.

---
