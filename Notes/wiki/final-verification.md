# Final integration verification — Issue #35

The closing verification pass for the vrg implementation: a clean-checkout
repository-wide build, vet, and test pass; an explicit no-cache rerun of
the PTY/subprocess boundary suites from Issues #4, #9, and #11; and a
final-binary smoke run through the five representative outcomes with the
Issue #4 fake-rg harness. No regressions were found; every gate and
smoke outcome was green on the first clean pass.

References: Issue #35 (`Notes/tasks/035-final-integration-verification.md`),
[PRD-vrg.md](../PRD-vrg.md) (Testing Decisions, Outcome and exit-status
contract), [outcome-contract](outcome-contract.md),
[search-collection-path](search-collection-path.md).

## Clean-checkout gates

Started from a clean checkout with the Go build and test caches cleared
(`go clean -cache`, `go clean -testcache`) so no stale artifact could
mask a failure.

- `go build ./...` — exit 0, no diagnostics.
- `go vet ./...` — exit 0, no diagnostics.
- `go test ./...` — exit 0; every package green:

  | Package | Result |
  |---|---|
  | `vrg/cmd/vrg` | ok |
  | `vrg/internal/app` | ok |
  | `vrg/internal/cli` | ok |
  | `vrg/internal/docs` | ok |
  | `vrg/internal/filebuffer` | ok |
  | `vrg/internal/safepresentation` | ok |
  | `vrg/internal/searchindex` | ok |
  | `vrg/internal/sinkfixtures` | no test files |
  | `vrg/internal/theme` | ok |
  | `vrg/internal/viewport` | ok |

## Explicit PTY/subprocess suite rerun

The PTY/subprocess tests for Issues #4, #9, and #11 all live in
`cmd/vrg` (package `main`). They were re-run with caching disabled
(`go test -count=1 -v ./cmd/vrg/ -timeout 120s`) so every critical
boundary test executed rather than being satisfied by a cached result.

- **Issue #4** (`search_test.go`): `TestChildArgvAndWorkdir`,
  `TestChildArgvWithFlags`, `TestStartFailureExit2`,
  `TestStartFailureExplicitPath`, `TestDualPipeBackpressure`,
  `TestStderrCapturedWithoutBlocking` — all PASS.
- **Issue #9** (`outcome_test.go`): `TestFatalExitWithResultsShowsOverlay`,
  `TestFatalExitNoOutputNamesExitCode`, `TestFatalExitNoOutputEscExits2`,
  `TestSignalDeathNamesSignal`,
  `TestStderrWarningWithSummaryShowsWarningOverlay`,
  `TestStderrContentFixture` — all PASS.
- **Issue #11** (`replay_test.go`): `TestReplayCtrlCAfterStderrDiagnostic`,
  `TestReplayQWhileSearchingAfterDiagnostic`,
  `TestReplayQWhileGateHeldAfterDiagnostic`,
  `TestReplayNormalQAfterCompletedStreamWithWarning`,
  `TestReplayControlledFailureWithEarlierDiagnostic`,
  `TestReplayFilenameWithNewlineAndESC` — all PASS.
- **Issue #4 cancellation** (`cancel_test.go`):
  `TestQAgainstBlockedFakeRGExits130`,
  `TestCtrlCAgainstBlockedFakeRGExits130`, `TestNormalExitReapsChild`,
  `TestQDuringGateHeldPreparationExits130`,
  `TestInjectedControlledFailure` — all PASS.
- **Help-only process boundary** (`main_test.go`):
  `TestGeneratedHelpStdout` (15 sub-cases), `TestHelpWithoutRipgrep`,
  `TestHelpDoesNotInvokeRipgrep`, `TestHelpIgnoresExecutableName`,
  `TestCLIOutputSafety`, `TestExecutableBoundary` (8 sub-cases),
  `TestDashFileRootAtProcessBoundary`,
  `TestHelpAssignmentSpellingsAreNotHelp`, `TestFlagContractUsageErrors`
  (15 sub-cases), `TestSinkSafetyTableUsageErrorProcessBoundary` (8
  sub-cases), `TestSinkSafetyTableHelpStdoutProcessBoundary` — all PASS.

## Final-binary smoke outcomes

The final `vrg` binary was built (`go build -o /tmp/vrg-smoke/vrg
./cmd/vrg`) and driven through the five representative outcomes under a
real PTY using the Issue #4 fake-rg harness. Each scenario captured the
exit status, terminal restoration, and stderr replay.

### 1. Successful browse — exit 0

A fake rg emitting a valid stream with one match in `test.txt`, the
file present on disk. After the completion handshake, `q` was sent.
Result: the browse view rendered `test.txt`, exit 0, empty stderr.

### 2. No-results search — exit 1

A fake rg emitting a summary-only stream (no matches), writing `warn`
to stderr, exiting 1. `Esc` dismissed the warning overlay, then `q`
quit. Result: "No results found" shown, exit 1, and the collected
`warn` diagnostic replayed to stderr after terminal restoration (Issue
[#11](outcome-contract.md) replay contract).

### 3. Fatal fake-rg, no usable results — exit 2

A fake rg exiting 2 with no output. The fatal no-results overlay
named the exit code. Dismissal was verified with both keys:

- `q` — exit 2.
- `Esc` — exit 2 (fatal no-results overlay has no underlying state to
  return to, so either dismissal key terminates with status 2).

### 4. Cancellation while searching — exit 130

A blocked fake rg (touching a ready file, then `sleep 100000`). While
searching, the cancellation key was sent. Verified:

- `q` while searching — exit 130.
- `ctrl+c` while searching — exit 130.
- The child process was terminated and reaped (reap evidence file
  non-empty; the fake rg PID no longer alive).
- Terminal restoration: cursor-show sequence (`\x1b[?25h`) present;
  alt-screen exit (`\x1b[?1049l`) present after alt-screen enter; PTY
  termios restored to the pre-run state.

### 5. Help-only invocation — exit 0

Run with ripgrep unavailable on `PATH` (a directory containing only a
sentinel fake rg) so the no-child/no-TUI startup branch is exercised.
The sentinel fake rg was never invoked (its marker file never
appeared). Verified for bare `vrg`, `-h`, and `--help`:

- exit 0;
- exactly one `Usage:` line on stdout (one help copy);
- empty stderr;
- no terminal control sequences (no alt-screen enter, no CSI) in
  either stream.

## Regressions

None. The clean-checkout build, vet, and test pass was green on the
first run. The explicit no-cache PTY/subprocess rerun was green. All
five final-binary smoke outcomes matched their contracts. No production
code was changed during this pass; no focused test was patched,
weakened, or deleted.

See [unit-tests](unit-tests.md) for the full test catalog and
[outcome-contract](outcome-contract.md) for the outcome matrix these
smoke outcomes exercise.
