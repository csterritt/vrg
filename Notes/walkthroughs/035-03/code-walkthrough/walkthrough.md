# Issue #35: final integration verification — clean build, vet, test, and the five smoke outcomes

*2026-09-18T03:37:26Z by Showboat 0.6.1*
<!-- showboat-id: c59191c9-0119-4532-b23e-75c7043c855a -->

Issue #35 is the closing verification pass over the fully composed implementation — no new product behavior is built; the composed RED contracts of Issues #1–#34 are the contract. From a clean checkout of the completed repository the pass runs the repository-wide gates (go build ./..., go vet ./..., go test ./...), explicitly re-executes the Issue #4/#9/#11 PTY/subprocess boundary tests with caching disabled, then smoke-runs the final binary through the five representative outcomes — browse 0, no-results 1, fatal 2 after overlay dismissal with q and Esc, cancellation 130 with the child terminated and reaped, and help-only 0 — each asserting exit status, terminal restoration, and stderr replay where applicable. Every command, output, and exit status is recorded below as the closing review evidence for the whole task set. See Notes/issues/035-final-integration-verification.md, Notes/tasks/035-final-integration-verification.md, and the Testing Decisions section of Notes/PRD-vrg.md. All artifacts live in this directory.

## Clean checkout

The pass starts from a fresh clone of the repository so no stale artifact — untracked binaries, editor state, or a warm test cache — can mask a failure. The clone lands at the Issue #34 head; a cold GOCACHE accompanies it for the gate runs.

```bash
git clone --quiet /home/chris/vrg /tmp/vrg-035-verify && cd /tmp/vrg-035-verify && git log --oneline -1 && echo "untracked/dirty paths: $(git status --porcelain | wc -l)"; echo "exit=$?"
```

```output
Note: switching to 'a1647a6340fbb38e0f5f833a6cfd3b70ac312322'.

You are in 'detached HEAD' state. You can look around, make experimental
changes and commit them, and you can discard any commits you make in this
state without impacting any branches by switching back to a branch.

If you want to create a new branch to retain commits you create, you may
do so (now or later) by using -c with the switch command. Example:

  git switch -c <new-branch-name>

Or undo this operation with:

  git switch -

Turn off this advice by setting config variable advice.detachedHead to false

a1647a6 Task Notes/tasks/034-documentation-scale-and-memory-limits.md implemented by SWE-2
untracked/dirty paths: 0
exit=0
```

## Gates — go build ./..., go vet ./..., go test ./...

The three repository-wide gates over the fully composed implementation, run in the clean checkout against a cold GOCACHE. go test ./... covers every package: cmd/vrg runs the real-binary PTY suite, so it is the slow line.

```bash
cd /tmp/vrg-035-verify && export GOCACHE=/tmp/vrg-035-verify-gocache && go build ./...; echo "build exit=$?" && go vet ./...; echo "vet exit=$?"
```

```output
build exit=0
vet exit=0
```

```bash
cd /tmp/vrg-035-verify && export GOCACHE=/tmp/vrg-035-verify-gocache && go test ./...; echo "test exit=$?"
```

```output
?   	vrg/Notes/walkthroughs/007-04/code-walkthrough/fixture	[no test files]
ok  	vrg/cmd/vrg	58.482s
ok  	vrg/internal/app	0.351s
ok  	vrg/internal/cli	0.005s
ok  	vrg/internal/docs	0.005s
ok  	vrg/internal/filebuffer	0.010s
ok  	vrg/internal/safepresentation	0.003s
?   	vrg/internal/safepresentation/sinktest	[no test files]
ok  	vrg/internal/searchindex	0.910s
ok  	vrg/internal/theme	0.002s
ok  	vrg/internal/viewport	0.007s
test exit=0
```

## Explicit PTY/subprocess reruns — Issues #4, #9, #11

The critical boundary tests execute explicitly with caching disabled (-count=1): the Issue #4 cancellation/cleanup PTY tests (q and ctrl+c against a blocked fake rg, gate-held preparation, ordinary-quit reaping, controlled-failure cleanup), the Issue #9 fatal/warning outcome tests, and the Issue #11 stderr-replay tests — all in the cmd/vrg subprocess-boundary package, driven on real PTYs with handshake files and the VRG_TEST_REAP/VRG_TEST_DIAG_ACK side channels rather than sleeps. internal/app and internal/searchindex run uncached alongside.

```bash
cd /tmp/vrg-035-verify && go test -count=1 -v ./cmd/vrg 2>&1 | grep -E '^(--- (PASS|FAIL|SKIP)|ok|FAIL|PASS)' | sed -E 's/\([0-9.]+s\)//; s/\t[0-9.]+s$//'; echo "cmd/vrg exit=${PIPESTATUS[0]}"
```

```output
--- PASS: TestCancelWhileSearchingKillsChild 
--- PASS: TestQDuringGateHeldPreparationCancels 
--- PASS: TestOrdinaryQuitLeavesNoChild 
--- PASS: TestControlledFailureCleanupExit2 
--- PASS: TestGeneratedHelpStdout 
--- PASS: TestHelpWithoutRipgrep 
--- PASS: TestHelpDoesNotInvokeRipgrep 
--- PASS: TestHelpIgnoresExecutableName 
--- PASS: TestCLIOutputSafety 
--- PASS: TestExecutableBoundary 
--- PASS: TestDashFileRootAtProcessBoundary 
--- PASS: TestHelpAssignmentSpellingsAreNotHelp 
--- PASS: TestFatalExitWithResultsShowsOverlay 
--- PASS: TestFatalExitNoOutputNamesExitCode 
--- PASS: TestFatalExitNoOutputEscExits2 
--- PASS: TestSignalDeathNamesSignal 
--- PASS: TestStderrWarningWithSummaryShowsWarningOverlay 
--- PASS: TestStderrContentFixture 
--- PASS: TestCancelReplaysProcessedDiagnostic 
--- PASS: TestQDuringGateHeldPreparationReplaysDiagnostic 
--- PASS: TestNormalQuitReplaysDiagnosticsInOrder 
--- PASS: TestControlledFailureReplaysViaCollection 
--- PASS: TestReplayEscapesHostileFilename 
--- PASS: TestChildArgvAndWorkdir 
--- PASS: TestStartFailureExit2 
--- PASS: TestDualPipeBackpressure 
--- PASS: TestStderrCapturedWithoutBlocking 
--- PASS: TestGateHeldPreparationKeepsSearching 
PASS
ok  	vrg/cmd/vrg
cmd/vrg exit=0
```

```bash
cd /tmp/vrg-035-verify && go test -count=1 ./internal/app ./internal/searchindex 2>&1 | sed -E 's/\t[0-9.]+s$//'; echo "internal exit=${PIPESTATUS[0]}"
```

```output
ok  	vrg/internal/app
ok  	vrg/internal/searchindex
internal exit=0
```

## Final binary — the five smoke outcomes

The final vrg binary is built from the clean checkout into this directory, then driven end to end by smoke.py — the Issue #4 fake-rg PTY harness. Each scenario launches vrg on a real PTY with a separated stderr pipe and a controllable fake rg (handshake/ready/pid side files), gates every key on the rendered marker of the state it depends on, drains output to EOF, and asserts the exit status, the terminal restoration (cursor visible, alt-screen left, termios identical to the pre-launch capture), the externally observed child termination with the VRG_TEST_REAP wait-status evidence where a child was killed, and the stderr replay where diagnostics were collected. The help-only invocation runs with PATH holding only a sentinel fake rg — real ripgrep unavailable — and proves the sentinel is never invoked: the no-child/no-TUI startup branch.

```bash
cd /tmp/vrg-035-verify && go build -o /home/chris/vrg/Notes/walkthroughs/035-03/code-walkthrough/vrg ./cmd/vrg; echo "build-vrg exit=$?"
```

```output
build-vrg exit=0
```

```bash
cd /home/chris/vrg && python3 Notes/walkthroughs/035-03/code-walkthrough/smoke.py; echo "smoke exit=$?"
```

```output
Scenario 0: successful browse, q exits 0
  [PASS] browse exit 0
  [PASS] browse view shows test.txt
  [PASS] browse stderr empty
  [PASS] browse PTY reached EOF
  [PASS] browse cursor restored
  [PASS] browse alt screen exited
  [PASS] browse termios restored
Scenario 1: no-results search, warning dismissed, q exits 1
  [PASS] no-results exit 1
  [PASS] no-results shows 'No results found'
  [PASS] no-results stderr replays 'warn'
  [PASS] no-results PTY reached EOF
  [PASS] no-results cursor restored
  [PASS] no-results alt screen exited
  [PASS] no-results termios restored
Scenario 2: fatal fake-rg, composed integrity/record-loss diagnostics, q and Esc exit 2
  [PASS] fatal q exit 2
  [PASS] fatal q overlay names exit code
  [PASS] fatal q overlay states integrity cause
  [PASS] fatal q overlay states record-loss cause
  [PASS] fatal q stderr replays composed diagnostic
  [PASS] fatal q cursor restored
  [PASS] fatal q alt screen exited
  [PASS] fatal q termios restored
  [PASS] fatal Esc exit 2
  [PASS] fatal Esc cursor restored
  [PASS] fatal Esc alt screen exited
  [PASS] fatal Esc termios restored
Scenario 130a: cancellation via q while searching, child gone and reaped, terminal restored
  [PASS] cancel-q exit 130
  [PASS] cancel-q child process gone
  [PASS] cancel-q child reaped (wait status reported)
  [PASS] cancel-q PTY reached EOF
  [PASS] cancel-q cursor restored
  [PASS] cancel-q alt screen exited
  [PASS] cancel-q termios restored
Scenario 130b: cancellation via ctrl+c while searching
  [PASS] cancel-ctrl+c exit 130
  [PASS] cancel-ctrl+c child process gone
  [PASS] cancel-ctrl+c child reaped (wait status reported)
  [PASS] cancel-ctrl+c PTY reached EOF
  [PASS] cancel-ctrl+c cursor restored
  [PASS] cancel-ctrl+c alt screen exited
  [PASS] cancel-ctrl+c termios restored
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
smoke exit=0
```

## Result

Every gate, every explicitly executed PTY/subprocess test, and all five final-binary smoke outcomes pass on the clean-checkout pass — no regression was uncovered, so no production code changed and no focused test was patched, weakened, or deleted (AC4's fix-and-rerun path went unused). This record — the clean-checkout go build/go vet/go test runs, the -count=1 PTY/subprocess reruns, and the five smoke outcomes with their exit statuses, terminal restoration, and stderr replay — is the closing review evidence for Issues #1–#35. Reference: Notes/issues/035-final-integration-verification.md and Notes/PRD-vrg.md.

Artifacts in this directory: vrg (the final binary built from the clean checkout), smoke.py (the Issue #4 fake-rg PTY smoke harness that produced the evidence above), walkthrough.md (this record).
