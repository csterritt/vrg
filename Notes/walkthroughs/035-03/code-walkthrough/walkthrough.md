# Issue #35: Final integration verification — clean build/vet/test and the five smoke outcomes

*2026-09-24T20:41:49Z by Showboat 0.6.1*
<!-- showboat-id: 0ea78e69-5224-4281-88fb-6ea09f8949c2 -->

Walkthrough for [Issue #35](../../../issues/035-final-integration-verification.md)'s closing task — the clean repository-wide verification pass over the fully composed Issues #1–#34 implementation, per `Notes/PRD-vrg.md` *Testing Decisions* (subprocess-boundary and help-only CLI contracts). Every gate runs from a fresh clean worktree checkout so no stale artifact can mask a failure: `go build ./...`, `go vet ./...`, `go test ./...`, then the PTY/subprocess test packages re-run uncached (`-count=1`), then the final binary driven through the five smoke outcomes by `scripts/smoke.py` — the condition-driven fake-rg PTY harness whose every keypress waits on an observed rendered marker. The pass caught one real regression — cancellation killed only the direct child PID, so a scripted rg's grandchild payload held the drained pipes open and hung the exit; `internal/app/search.go` now spawns the child as a process-group leader and `cmd.Cancel` SIGKILLs the group (commit 2d9a10c), covered by `TestCancelTerminatesChildProcessGroup`. The harness's stale seeded marker strings were aligned to the composed diagnostic wording the focused tests pin. Test durations are stripped so the document verifies cleanly.

A detached worktree of the repository HEAD is the clean checkout: zero dirty files, then the three repository-wide gates.

```bash
git -C /home/chris/vrg worktree remove --force /tmp/vrg-35-verify 2>/dev/null; rm -rf /tmp/vrg-35-verify; git -C /home/chris/vrg worktree add --detach /tmp/vrg-35-verify HEAD >/dev/null 2>&1 && cd /tmp/vrg-35-verify && echo "HEAD: $(git rev-parse --short HEAD)" && echo "dirty files: $(git status --porcelain | wc -l)" && go build ./... && echo "go build ./... -> exit 0" && go vet ./... && echo "go vet ./... -> exit 0"
```

```output
HEAD: 2d9a10c
dirty files: 0
go build ./... -> exit 0
go vet ./... -> exit 0
```

```bash
cd /tmp/vrg-35-verify && go test ./... 2>&1 | sed "s/[[:space:]][0-9.]*s$//;s/[[:space:]]*(cached)$//"; echo "go test ./... -> exit ${PIPESTATUS[0]}"
```

```output
ok  	vrg/cmd/vrg
ok  	vrg/internal/app
ok  	vrg/internal/cli
ok  	vrg/internal/filebuffer
ok  	vrg/internal/present
ok  	vrg/internal/searchindex
ok  	vrg/internal/theme
ok  	vrg/internal/viewport
go test ./... -> exit 0
```

## Uncached PTY/subprocess boundary re-runs

`go test -count=1` defeats the test cache so every critical boundary test executes rather than reporting a cached result. `cmd/vrg` carries the Issue #4 PTY harness (the `TestPTY*` group — `q`/`ctrl+c`/SIGINT cancellation at 130 with reap evidence and byte-identical termios restoration, gate-held preparation, ordinary-exit reaping, injected controlled failure), the Issue #9 fatal-outcome boundary tests, and the Issue #11 `pty_replay_test.go` replay suite; `internal/app`'s `subprocess_test.go` covers the fake-rg spawn boundary — child argv and working directory, dual-pipe backpressure, cancel-terminate-reap, the new process-group regression test, and start failure.

```bash
cd /tmp/vrg-35-verify && go test -count=1 -v ./cmd/vrg 2>&1 | grep -E "^(--- |PASS|FAIL|ok)" | sed -E "s/ \([0-9.]+s\)//; s/[[:space:]][0-9.]+s$//"; echo "go test -count=1 ./cmd/vrg -> exit ${PIPESTATUS[0]}"
```

```output
--- PASS: TestGeneratedHelpStdout
--- PASS: TestHelpWithoutRipgrep
--- PASS: TestHelpDoesNotInvokeRipgrep
--- PASS: TestHelpIgnoresExecutableName
--- PASS: TestCLIOutputSafety
--- PASS: TestExecutableBoundary
--- PASS: TestDashFileRootAtProcessBoundary
--- PASS: TestSearchLifecycleAtBoundary
--- PASS: TestDualPipeDrainageAtBoundary
--- PASS: TestStartFailureNoRipgrep
--- PASS: TestHelpAssignmentSpellingsAreUsageErrorsAtBoundary
--- PASS: TestFlagAndArgvUsageErrors
--- PASS: TestHelpWithSearchFlags
--- PASS: TestPTYCtrlCAfterDiagnosticReplaysOnce
--- PASS: TestPTYQWhileSearchingReplaysDiagnostic
--- PASS: TestPTYQDuringGateHeldPreparationReplaysDiagnostic
--- PASS: TestPTYQuitAfterCompletedStreamReplaysWarning
--- PASS: TestPTYControlledFailureReplaysAlongsideEarlierDiagnostics
--- PASS: TestPTYReplayEscapesEmbeddedFilename
--- PASS: TestPTYQWhileSearchingExits130
--- PASS: TestPTYCtrlCWhileSearchingExits130
--- PASS: TestPTYSIGINTWhileSearchingExits130
--- PASS: TestPTYQDuringGateHeldPreparationExits130
--- PASS: TestPTYOrdinaryExitReapsChild
--- PASS: TestPTYControlledFailureExits2
--- PASS: TestControlledFailureDiagnosticOnStderr
--- PASS: TestPTYNonZeroExitBrowseOverlayExits2
--- PASS: TestPTYStderrContentOverlayHeadAndTail
PASS
ok  	vrg/cmd/vrg
go test -count=1 ./cmd/vrg -> exit 0
```

```bash
cd /tmp/vrg-35-verify && go test -count=1 -v -run "TestChildArgvAndWorkingDirectory|TestDualPipeBackpressure|TestCancelTerminatesAndReapsChild|TestCancelTerminatesChildProcessGroup|TestStartFailure" ./internal/app 2>&1 | grep -E "^(--- |PASS|FAIL|ok)" | sed -E "s/ \([0-9.]+s\)//; s/[[:space:]][0-9.]+s$//"; echo "internal/app subprocess boundary -> exit ${PIPESTATUS[0]}"
```

```output
--- PASS: TestChildArgvAndWorkingDirectory
--- PASS: TestDualPipeBackpressure
--- PASS: TestCancelTerminatesAndReapsChild
--- PASS: TestCancelTerminatesChildProcessGroup
--- PASS: TestStartFailure
PASS
ok  	vrg/internal/app
internal/app subprocess boundary -> exit 0
```

## The five smoke outcomes — final binary under the fake-rg PTY harness

`scripts/smoke.py` drives the freshly built binary on a real PTY with a separated stderr pipe. Every fake-rg fixture synchronizes through a `FAKE_RG_*` handshake file and every keypress waits on an observed rendered marker — no sleeps. Scenario 0: a complete stream browses and `q` exits 0 with empty stderr. Scenario 1: a summary-only stream with a `warn` diagnostic shows the warning overlay, `Esc` dismisses to the no-results screen, `q` exits 1, and `warn` replays to stderr. Scenario 2: a malformed record plus exit 2 paints the composed diagnostic — `rg failed: exit status 2`, `missing summary`, `1 malformed record skipped` — and both `q` and `Esc` dismissals exit 2 with the diagnostic replayed. Scenario 130 (and the `ctrl+c` variant): `q` against a blocking fake rg exits 130, the child *and its process group* are externally observed gone, the cursor/alt-screen restoration sequences appear, and the slave termios equals its pre-launch value — the scenario that exposed the process-group regression before the fix. Scenario help-only: bare `vrg`, `-h`, and `--help` each print exactly one `Usage:` copy to stdout, empty stderr, exit 0, with ripgrep unavailable and a sentinel fake `rg` on `PATH` that is never invoked — proving the no-child/no-TUI startup branch. The built binary is stored in this directory as `vrg`.

```bash
cd /tmp/vrg-35-verify && go build -o /tmp/vrg-smoke/vrg ./cmd/vrg && cp /tmp/vrg-smoke/vrg /home/chris/vrg/Notes/walkthroughs/035-03/code-walkthrough/vrg && python3 scripts/smoke.py; echo "smoke.py -> exit $?"
```

```output
Scenario 0: successful browse, q exits 0
  [PASS] browse exit 0
  [PASS] browse view shows test.txt
  [PASS] browse stderr empty
  [PASS] browse PTY reached EOF
Scenario 1: no-results search, warning dismissed, q exits 1
  [PASS] no-results exit 1
  [PASS] no-results shows 'No results found'
  [PASS] no-results stderr replays 'warn'
  [PASS] no-results PTY reached EOF
Scenario 2: fatal fake-rg, composed integrity/record-loss diagnostics, q and Esc exit 2
  [PASS] fatal q exit 2
  [PASS] fatal q overlay names exit code
  [PASS] fatal q overlay states integrity cause
  [PASS] fatal q overlay states record-loss cause
  [PASS] fatal q stderr replays composed diagnostic
  [PASS] fatal Esc exit 2
Scenario 130: cancellation while searching, child and process group gone, terminal restored
  [PASS] cancel exit 130
  [PASS] child process gone
  [PASS] child process group gone
  [PASS] cancel PTY reached EOF
  [PASS] terminal cursor restored
  [PASS] alt screen exited
  [PASS] termios restored
Scenario 130b: cancellation via ctrl+c while searching
  [PASS] ctrl+c exit 130
  [PASS] ctrl+c child process gone
  [PASS] ctrl+c process group gone
Scenario help-only: bare vrg, -h, --help (sentinel fake rg never invoked)
  [PASS] bare vrg exit 0
  [PASS] bare vrg exactly one 'Usage:' on stdout
  [PASS] bare vrg stderr empty
  [PASS] bare vrg no terminal control sequences
  [PASS] -h exit 0
  [PASS] -h exactly one 'Usage:' on stdout
  [PASS] -h stderr empty
  [PASS] -h no terminal control sequences
  [PASS] --help exit 0
  [PASS] --help exactly one 'Usage:' on stdout
  [PASS] --help stderr empty
  [PASS] --help no terminal control sequences
  [PASS] sentinel fake rg never invoked

SMOKE OK: all five outcomes verified
smoke.py -> exit 0
```

## Closing review

Every gate green on the clean checkout: `go build ./...`, `go vet ./...`, `go test ./...` (all eight packages), the uncached `TestPTY*`/`pty_replay`/subprocess-boundary re-runs, and all five smoke outcomes — browse 0, no-results 1, fatal 2 under both dismissal keys with stderr replay, cancellation 130 with the child and its process group gone and the terminal restored, and the help-only branch proving no child is spawned and no TUI starts. The one regression the pass caught — the forked-payload cancellation hang — is repaired by the process-group kill in `internal/app/search.go` (commit 2d9a10c) with `TestCancelTerminatesChildProcessGroup` as its regression test; the harness's stale seeded markers were aligned to the wording the focused tests pin, with no behavioral assertion weakened. The pass is ingested in [Notes/wiki/final-verification.md](../../../wiki/final-verification.md) and recorded in `Notes/wiki/log.md`. This closes Issue #35: the composed Issues #1–#34 contracts verify end-to-end.

