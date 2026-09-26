# Post-audit re-verification (Issue #50)

The closing gate for the audit cycle: a clean-checkout verification
pass over the composed Issues #36–#49 implementation, per
[Notes/tasks/050-post-audit-reverification.md](../tasks/050-post-audit-reverification.md)
and the *Testing Decisions* section of `Notes/PRD-vrg.md`. It is the
post-audit analogue of
[final-verification.md](final-verification.md) (Issue #35) — this time
verifying that the audit-driven repairs did not regress each other's
contracts. The evidence run is recorded in the
[050-01 walkthrough](../walkthroughs/050-01/code-walkthrough/walkthrough.md).
The pass found no production regressions; it surfaced and repaired one
test-harness race (below).

## The FAKE_RG_* fixture rename (Task 1)

Fixture-owned environment variables leave the `VRG_*` namespace so
only vrg-consumed seams carry `VRG_TEST_*` (the explicit
`hookManifest` in `cmd/vrg/testhooks_test.go` — see
[test-hook-build-topology.md](test-hook-build-topology.md)). The task
plan names the pre-#48 five-variable shape
(`VRG_TEST_HANDSHAKE`/`READY`/`PID`/`ARGV`/`CWD`); the active fixtures
had consolidated those into a single capture directory plus the
help-sentinel marker, so the rename landed on the names that actually
exist:

- `VRG_CAPTURE_DIR` → `FAKE_RG_CAPTURE_DIR` across the fake-rg shell
  fixtures (`fakeRgScript`, `fakeRgBlockScript`, `fakeRgStreamScript`,
  `fakeRgFatalScript`, `fakeRgExit3Script`, `fakeRgFloodScript`,
  `fakeRgWarnBlockScript`, `fakeRgWarnStreamScript`,
  `fakeRgWarnNoSummaryScript`, `fakeRgBadPathScript`) and the
  `cmd/vrg` environment lists `searchEnv`/`ptyEnv`.
- `VRG_RG_MARKER` → `FAKE_RG_MARKER` in `TestHelpDoesNotInvokeRipgrep`.

The uncached `cmd/vrg` suite passed unchanged before and after the
rename — it is the behavioural safety net. The `internal/app`
re-exec fixture's `VRG_FAKE_RG`/`VRG_FAKE_DIR` pair is out of scope:
it is consumed by the test binary's own `TestMain`, not a shell
fixture or a `cmd/vrg` environment list.

## The canonical smoke harness (Task 2)

`scripts/smoke.py` is the canonical condition-driven fake-rg PTY
smoke harness. Every key is sent only after the preceding expected UI
state or output marker is observed in the ANSI-stripped PTY stream;
draining is completion/EOF-driven on both the PTY master and the
separated stderr pipe; bounded polls re-check an explicit observable
condition every iteration (`wait_condition`, `wait_output`,
`wait_exit`, `wait_pid_gone`, `wait_pgid_gone` — paced by select
idles, never assumed settling). The fixture ready/handshake files
prove only that the fake rg started; the rendered `Searching…`/overlay
markers prove the app state a key depends on. Cancellation observes
the child PID *and* its process group externally (`kill -0`/`killpg`
probes — no test seam). Two static guards: `assert_no_fixed_delays`
parses the file's own AST and fails on any `time.sleep` call, and
`assert_no_vrg_test_env` rejects any `VRG_TEST_*` name in a run's
environment (the untagged binary would silently ignore a leaked
control, masking a synchronization gap). The Issue #35 walkthrough
artifact remains a frozen historical record — the canonical harness
evolves in place.

## The permanent verification entry point (Task 3)

`scripts/verify.sh` is the executable permanent gate set, runnable
from a clean checkout and failing on the first non-zero step:

1. `go build ./...`
2. `go vet ./...`
3. `go build -tags vrg_testhooks ./cmd/vrg`
4. `go vet -tags vrg_testhooks ./cmd/vrg`
5. `go test ./... -count=1`
6. `CGO_ENABLED=1 go test -race ./... -count=1`
7. `go test ./cmd/vrg -count=3` — the repeated PTY runs
8. `go mod verify`
9. `go run golang.org/x/vuln/cmd/govulncheck@v1.5.0 ./...`
10. `go mod tidy -diff` — the permanent tidy gate Issue #49 deferred
    to Issue #50; any output fails.

Gate 9's prerequisite is documented in the script header: fetching the
pinned tool and the vulnerability database needs network access or
pre-populated module/tool and vulnerability-database caches. An
offline clean machine without them is reported as an ENVIRONMENT
failure (exit 75) — distinguished from a repository regression, which
propagates the scan's own exit status.

## The closing pass (Task 4)

From a clean detached-HEAD worktree checkout of the post-rename tree:

- `scripts/verify.sh` exited 0 — all ten gates green, including the
  full race suite, three consecutive `cmd/vrg` runs, `govulncheck`
  reporting no vulnerabilities, and zero tidy drift.
- The named outcome/replay/cancellation contracts were re-run
  explicitly and uncached (`go test -count=1 -v -run …`) plus the full
  package under `-race -count=1`. The task names predate the Issue #48
  handshake refactor; the current carriers are
  `TestPTYNonZeroExitBrowseOverlayExits2` (fatal overlay over usable
  results), the `TestHandshakeMatrixRowsAcknowledged` fatal session
  plus `TestOverlayDismissalAcknowledgedBeforeQuit` (fatal no-output
  naming its exit status under both dismissal keys — the signal-death
  naming contract is covered by `internal/app`'s `sigErr` fixture
  tests inside gate 5), `TestPTYQuitAfterCompletedStreamReplaysWarning`
  (stderr warning + summary → warning overlay),
  `TestPTYStderrContentFixture` (the ≥ 1 MiB stderr fixture),
  `TestPTYCtrlCAfterDiagnosticReplaysOnce`,
  `TestPTYQWhileSearchingReplaysDiagnostic`,
  `TestPTYQDuringGateHeldPreparationReplaysDiagnostic`,
  `TestPTYControlledFailureReplaysAlongsideEarlierDiagnostics`,
  `TestPTYReplayEscapesEmbeddedFilename`, `TestPTYQWhileSearchingExits130`,
  `TestPTYCtrlCWhileSearchingExits130`, `TestPTYSIGINTWhileSearchingExits130`,
  `TestPTYOrdinaryExitReapsChild`, `TestPTYQDuringGateHeldPreparationExits130`,
  `TestPTYControlledFailureExits2`,
  `TestControlledFailureDiagnosticOnStderr`,
  `TestSearchLifecycleAtBoundary` (child argv/workdir),
  `TestStartFailureNoRipgrep`, and `TestDualPipeDrainageAtBoundary` —
  all on the Issue #48 `VRG_TEST_EVENT_ACK` handshakes, with
  `TestHarnessHasNoFixedDelays` pinning that no fixed settling or
  inter-key delay survives outside the bounded-poll allowlist
  ([pty-handshake-harness.md](pty-handshake-harness.md)).
- The five smoke outcomes ran via `scripts/smoke.py` against the
  **untagged** production binary (`go build -o /tmp/vrg-smoke/vrg
  ./cmd/vrg`): browse → 0; no-results → 1 with `warn` replayed to
  stderr; fatal → 2 under both `q` and `Esc` with the composed
  `rg failed: exit status 2` + `missing summary` + malformed-record
  diagnostics (Issues #36/#37/#44); cancellation → 130 under both `q`
  and `ctrl+c` with the child and its process group externally
  observed gone, PTY EOF, and cursor/alt-screen/termios restoration;
  help-only → 0 with exactly one `Usage:` copy and the sentinel fake
  rg never invoked. The smoke environment carried only `FAKE_RG_*`
  fixture names.

## Regressions found and repaired

No production regression — every audit-repair contract held under the
composed run. Re-executing the recorded pass did surface one
harness-side race and the pass repaired it:

- **Ready-file create/write race** in `internal/app/subprocess_test.go`
  (`awaitReadyPID`/`awaitPIDFile`): the helpers read the fake child's
  `ready`/`grandchild` handshake file the moment it *existed*, but the
  child's `os.WriteFile` creates the file before the pid bytes land —
  an existent-but-empty read failed `strconv.Atoi` inside
  `TestCancelTerminatesAndReapsChild` (observed once under
  `internal/app`'s uncached suite). Both helpers now require non-empty
  content — the same semantics `cmd/vrg`'s `awaitFileContent` already
  carries — proven by a 30-run cancellation stress plus the full
  post-fix closing pass. A test-helper repair; no behavioral assertion
  weakened (owning contract:
  [cancellation-and-cleanup.md](cancellation-and-cleanup.md)).
