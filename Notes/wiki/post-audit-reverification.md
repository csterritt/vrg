# Post-audit re-verification — Issue #50

The closing verification pass for the audit cycle — the counterpart to
Issue #35's [final-verification](final-verification.md) for the
original implementation set. The audit's overall assessment
(`Notes/critiques/final-audit-vrg.md`, *Overall assessment*) required
rerunning "the complete acceptance and PTY verification suite with
deterministic handshakes" after the high findings were resolved; this
issue owns that proof over the composed Issues #36–#49 implementation:
a permanent `scripts/verify.sh` gate runner, the `FAKE_RG_*`
fixture-variable rename, a canonical condition-driven smoke harness at
`scripts/smoke.py`, the explicit uncached and race reruns of the named
PTY/subprocess tests on the Issue #48 handshakes, and the five smoke
outcomes against the **untagged** production binary. No repository
regression was found; every gate and smoke outcome is green.

References: Issue #50
(`Notes/issues/050-post-audit-reverification.md`,
`Notes/tasks/050-post-audit-reverification.md`),
[PRD-vrg.md](../PRD-vrg.md) (*Testing Decisions* — the subprocess
boundary's deterministic-handshake and no-fixed-sleep rules),
[final-verification](final-verification.md),
[pty-handshake-tests](pty-handshake-tests.md),
[test-hook-topology](test-hook-topology.md),
[outcome-contract](outcome-contract.md). Closing review evidence:
`Notes/walkthroughs/050-01/code-walkthrough/walkthrough.md`.

## `scripts/verify.sh` — the permanent gate runner

`scripts/verify.sh` (executable, runnable from a clean checkout) runs
exactly ten ordered gates and fails on the first non-zero step:

| # | Gate | Notes |
|---|---|---|
| 1 | `go build ./...` | untagged production build |
| 2 | `go vet ./...` | untagged vet |
| 3 | `go build -tags vrg_testhooks ./cmd/vrg` | tagged build variant |
| 4 | `go vet -tags vrg_testhooks ./cmd/vrg` | tagged vet variant |
| 5 | `go test ./... -count=1` | full suite, uncached |
| 6 | `CGO_ENABLED=1 go test -race ./... -count=1` | full suite under race |
| 7 | `go test ./cmd/vrg -count=3` | repeated PTY/subprocess package run |
| 8 | `go mod verify` | module checksum verification |
| 9 | `go run golang.org/x/vuln/cmd/govulncheck@v1.5.0 ./...` | the audit's pinned invocation |
| 10 | `go mod tidy -diff` | the permanent tidy gate Issue #49 refers to; must report no drift |

Gate 9's pinned `govulncheck` invocation requires network access to
fetch the tool and the vulnerability database, or pre-populated
module/tool and vulnerability-database caches. An offline clean machine
without those caches does not satisfy the gate's environmental
prerequisites; the script reports that as an **ENVIRONMENT failure**
(exit 75), not a repository regression.

## The `FAKE_RG_*` fixture rename

Fake-rg-owned fixture variables moved off the `VRG_TEST_` prefix, so
the `VRG_TEST_*` namespace now names only the vrg-consumed hook
manifest (Issue #45, see
[test-hook-topology](test-hook-topology.md)):

- `VRG_TEST_ARGV` → `FAKE_RG_ARGV_FILE`
- `VRG_TEST_CWD` → `FAKE_RG_CWD_FILE`
- `VRG_TEST_HANDSHAKE` → `FAKE_RG_HANDSHAKE_FILE`
- `VRG_TEST_READY` → `FAKE_RG_READY_FILE`
- `VRG_TEST_PID` → `FAKE_RG_PID_FILE`

The rename covers every active fake-rg shell fixture in the `cmd/vrg`
test files (`writeFakeRG`, `writeBlockedFakeRG`, `writeCompleteFakeRG`,
`writeStderrFakeRG`, `writeFatalFakeRG`, `writeRunShapeFakeRG`, and the
inline fixtures) and the `cmd.Env` lists that set them; the uncached
`cmd/vrg` suite is green before and after the rename. The frozen
walkthrough artifacts keep their historical names.

## `scripts/smoke.py` — the canonical condition-driven smoke harness

`scripts/smoke.py` is the canonical fake-rg PTY smoke harness. The
Issue #35 copy at
`Notes/walkthroughs/035-03/code-walkthrough/smoke.py` is a **frozen
historical artifact** of that issue's recorded walkthrough and is not
edited. The canonical harness replaces every fixed pre-key, inter-key,
process-exit, and post-exit sleep of the original with bounded waits on
externally observable conditions:

- every key is sent only after the preceding expected UI state or
  output marker is observed in the ANSI-stripped PTY stream
  (`wait_output`) — a fake-child ready file proves only that the
  fixture started, never that the app entered a state;
- output draining is completion/EOF-driven (`drain` waits for EOF on
  both the PTY master and the separated stderr pipe);
- process exit is a bounded wait on `waitpid` completion;
- fixture files, PID liveness, and process-group liveness are bounded
  condition polls that re-check their explicit condition every
  iteration;
- `assert_no_fixed_delays` parses the file's own AST and fails if any
  `time.sleep` call is present — the static/review assertion that no
  fixed delay is used as a progress proxy;
- `assert_no_vrg_test_env` fails any run whose environment carries a
  `VRG_TEST_*` name — the production smoke neither sets nor depends on
  the test seams, so the `FAKE_RG_*` rename is also the proof that
  cancellation cleanup is observed externally (kill-0 liveness and
  process-group probes), not through `VRG_TEST_REAP`.

## Explicit PTY/subprocess reruns

The named tests were re-run explicitly and uncached — `go test -count=1
./cmd/vrg` and `CGO_ENABLED=1 go test -race -count=1 ./cmd/vrg` — all
operating on the Issue #48 deterministic handshakes
([pty-handshake-tests](pty-handshake-tests.md)) and all green in both
modes:

- outcome tests: `TestFatalExitWithResultsShowsOverlay`,
  `TestFatalExitNoOutputNamesExitCode`,
  `TestFatalExitNoOutputEscExits2`, `TestSignalDeathNamesSignal`,
  `TestStderrWarningWithSummaryShowsWarningOverlay`,
  `TestStderrContentFixture`;
- replay tests: `TestReplayCtrlCAfterStderrDiagnostic`,
  `TestReplayQWhileSearchingAfterDiagnostic`,
  `TestReplayQWhileGateHeldAfterDiagnostic`,
  `TestReplayNormalQAfterCompletedStreamWithWarning`,
  `TestReplayControlledFailureWithEarlierDiagnostic`,
  `TestReplayFilenameWithNewlineAndESC`;
- cancellation/boundary tests: `TestQAgainstBlockedFakeRGExits130`,
  `TestCtrlCAgainstBlockedFakeRGExits130`, `TestNormalExitReapsChild`,
  `TestQDuringGateHeldPreparationExits130`,
  `TestInjectedControlledFailure`, `TestChildArgvAndWorkdir`,
  `TestStartFailureExit2`, `TestDualPipeBackpressure`,
  `TestStderrCapturedWithoutBlocking`.

No fixed settling or inter-key `time.Sleep` synchronization remains in
`runVrgWithKeys`, `runVrgKillChild`, `runVrgWithQuit`, `runVrgCancel`,
or any `runVrgReplay` trigger callback — the static
`TestNoFixedSleepsInPtyHelpers` check confines every `time.Sleep` to
the named bounded condition polls and passes inside these same runs.

## Smoke outcomes against the untagged production binary

The binary was built untagged (`go build -o /tmp/vrg-smoke/vrg
./cmd/vrg`) — distinct from the `vrg_testhooks`-tagged binary the
subprocess tests use — and driven through the five representative
outcomes by `scripts/smoke.py`. Every scenario produced its contracted
exit status with terminal restoration and correct stderr replay:

1. **Successful browse → 0.** The browse view rendered `test.txt`
   before `q` was sent; exit 0, empty stderr, PTY EOF observed.
2. **No-results → 1.** The `warn` warning overlay was observed before
   `Esc`, "No results found" before `q`; exit 1 and `warn` replayed to
   stderr after terminal restoration.
3. **Fatal → 2 under both `q` and `Esc`.** The fixture emits a
   malformed record and exits 2, so the overlay carries the composed
   Issues #36–#37/#44 diagnostic: the generated process line
   (`ripgrep exited with code 2`), the missing-summary integrity
   cause, and the malformed record-loss component — the same text the
   post-restoration stderr replay emits.
4. **Cancellation → 130.** With the fake rg blocked, `q` (and
   separately `ctrl+c`) exited 130; the harness externally observed
   the child PID and its process group gone (kill-0 probes), PTY EOF,
   cursor-show/alt-screen-exit sequences, and termios restoration —
   without any `VRG_TEST_*` name in the environment.
5. **Help-only → 0.** Bare `vrg`, `-h`, and `--help` each print
   exactly one `Usage:` copy on stdout with empty stderr and no
   terminal control sequences; the sentinel fake rg is never invoked.

## Regressions

None in repository code. Every gate passed on the first clean pass and
no focused test was weakened or deleted. One defect was found and fixed
inside the new `scripts/smoke.py` harness itself during bring-up — a
missing index increment in its `strip_ansi` port made the output poll
spin; the harness is test infrastructure, not the audited artifact.
