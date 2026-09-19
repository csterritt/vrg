# Final integration verification (Issue #35)

Delivered by
[Issue #35](../issues/035-final-integration-verification.md)
([task](../tasks/035-final-integration-verification.md)): the single
closing verification pass over the fully composed implementation —
no new product behavior. Issues #30, #33, and #34 are the dependency
graph's terminal issues, so their completion transitively required
every feature issue (#1–#34) to land first; the composed RED contracts
of Issues #1–#34 are this pass's contract. Relevant PRD section:
*Testing Decisions* (subprocess boundary, responsiveness boundaries)
in [`Notes/PRD-vrg.md`](../PRD-vrg.md); user stories 1–85. The full
evidence record lives in the
[walkthrough](../walkthroughs/035-03/code-walkthrough/walkthrough.md).

## Clean-checkout gate results

The pass ran from a fresh `git clone` of the completed repository at
the Issue #34 head (`a1647a6`), with a cold `GOCACHE` so no stale
artifact or cached test result could mask a failure:

- `go build ./...` — clean.
- `go vet ./...` — clean.
- `go test ./...` — all packages pass: `cmd/vrg`, `internal/app`,
  `internal/cli`, `internal/docs`, `internal/filebuffer`,
  `internal/safepresentation`, `internal/searchindex`,
  `internal/theme`, `internal/viewport` (the `sinktest` helper package
  has no test files). `cmd/vrg` is the slow package (~58 s) because it
  runs the real-binary PTY suite.

## Explicit PTY/subprocess reruns

The critical boundary suites from Issues #4, #9, and #11 — all in
`cmd/vrg`, the subprocess-boundary package — were re-run explicitly
with test caching disabled (`go test -count=1`), so every test
executed rather than standing on a cached result, with no sleeps:

- Issue #4 ([cancellation-cleanup.md](cancellation-cleanup.md)):
  `TestCancelWhileSearchingKillsChild` (`q` and `ctrl+c`),
  `TestQDuringGateHeldPreparationCancels`,
  `TestOrdinaryQuitLeavesNoChild`, `TestControlledFailureCleanupExit2`.
- Issue #9 ([error-overlay-and-outcomes.md](error-overlay-and-outcomes.md)):
  `TestFatalExitWithResultsShowsOverlay`,
  `TestFatalExitNoOutputNamesExitCode`,
  `TestFatalExitNoOutputEscExits2`, `TestSignalDeathNamesSignal`,
  `TestStderrWarningWithSummaryShowsWarningOverlay`,
  `TestStderrContentFixture`.
- Issue #11 ([stderr-replay.md](stderr-replay.md)):
  `TestCancelReplaysProcessedDiagnostic`,
  `TestQDuringGateHeldPreparationReplaysDiagnostic`,
  `TestNormalQuitReplaysDiagnosticsInOrder`,
  `TestControlledFailureReplaysViaCollection`,
  `TestReplayEscapesHostileFilename`.

All 28 `cmd/vrg` tests pass uncached; `internal/app` and
`internal/searchindex` were re-run uncached as well. See
[unit-tests.md](unit-tests.md) for the boundary-test catalog.

## The five final-binary smoke outcomes

The binary built from the clean checkout was driven through the five
representative outcomes by the walkthrough's `smoke.py` — the Issue #4
fake-rg PTY harness (explicit readiness/completion handshake files,
the `VRG_TEST_REAP` wait-status side channel, separated stderr pipe,
pre/post termios capture; bounded condition polls, no sleeps):

- **Browse → 0.** A fake rg emitting a complete valid stream renders
  the browse view; `q` exits 0 with empty stderr, PTY EOF, and the
  terminal restored (cursor visible, alt-screen left, termios
  identical to pre-launch).
- **No results → 1.** A summary-only rg-1 stream with a stderr
  `warn` opens the warning overlay; `Esc` reveals "No results found"
  and `q` exits 1 with `warn` replayed to stderr.
- **Fatal → 2.** A fake rg emitting one malformed record and exiting
  2 with no usable results opens the fatal overlay carrying all three
  diagnostic components — the generated `ripgrep exited with code 2`
  process line, the `ripgrep event stream incomplete` integrity note,
  and `1 malformed record skipped` — and dismissing it with `q` exits
  2 with the composed diagnostic replayed to stderr; a second run
  proves `Esc` exits 2 identically.
- **Cancellation → 130.** A blocked fake rg (ready/pid files, then
  `exec sleep`) renders "Searching…"; `q` exits 130, the recorded
  child pid is externally observed gone, the `VRG_TEST_REAP` side
  channel carries `code=-1` — vrg's own wait/reap path ran — and the
  terminal is restored. `ctrl+c` produces the same result.
- **Help-only → 0.** Bare `vrg`, `-h`, and `--help` each print
  exactly one `Usage:` copy on stdout with empty stderr and exit 0,
  run with `PATH` holding only a sentinel fake rg — real ripgrep
  unavailable — which is never invoked, proving the no-child/no-TUI
  startup branch.

## Regressions

None. Every gate and smoke outcome passed on the first clean pass, so
no production repair was needed and no focused test was touched —
AC4's fix-and-rerun path went unused.

Issue #50 ran the post-audit counterpart of this pass over the composed
post-audit implementation: a permanent ten-gate `scripts/verify.sh`, a
canonical condition-driven `scripts/smoke.py` (this page's walkthrough
harness stays frozen as Issue #35's record), and a real regression
caught and repaired — see
[post-audit-verification.md](post-audit-verification.md).
