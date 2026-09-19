# Issue #4: cancellation, child cleanup, terminal restore

*2026-09-16T18:25:35Z by Showboat 0.6.1*
<!-- showboat-id: 117817f4-e7c0-4551-85ed-4f0f2cb067fd -->

Walkthrough for Issue #4 (`Notes/issues/004-cancellation-child-cleanup-terminal-restore.md`), implementing the cancellation and cleanup contracts of `Notes/PRD-vrg.md` (*Outcome and exit-status contract* rows 1–2 plus the cleanup bullet; *Colours, overlays, and key precedence*; *Module Design → App*; *Testing Decisions → Subprocess boundary*). `ctrl+c` in any state and `q` while searching or while index preparation is gate-held begin a controlled exit: the rg child is terminated and reaped (`Child.Terminate` + idempotent `Wait`, reported through the `VRG_TEST_REAP` side channel), the display and PTY input modes are restored, and the process exits 130 with no further screen; late search completions are discarded through the `quitting` flag. An injected controlled failure (`VRG_TEST_FAIL`) runs the same cleanup, then emits a sanitized diagnostic exactly once after restoration through `writeFailureDiag`, exiting 2. All generated artifacts (the built `vrg` binary, `pty_cancel.py`) live in this directory.

```bash
cd /home/chris/vrg && go mod verify && go build ./... && go vet ./... && echo GATES-OK
```

```output
all modules verified
GATES-OK
```

```bash
cd /home/chris/vrg && go test -count=1 ./... 2>&1 | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
ok  	vrg/cmd/vrg
ok  	vrg/internal/app
ok  	vrg/internal/cli
?   	vrg/internal/filebuffer	[no test files]
ok  	vrg/internal/searchindex
?   	vrg/internal/theme	[no test files]
?   	vrg/internal/viewport	[no test files]
```

Model tests (`internal/app/cancel_test.go`): `killChild` blocks `Wait` until `Terminate`, so a cleanup path that forgets to terminate hangs instead of passing. Covered: `q` and `ctrl+c` while searching cancel with status 130 and a reap report; `ctrl+c` on the summary overrides the fixed status with 130; `Esc` while searching is a no-op; a late completion after cancellation — including the one a released gate produces — cannot revive the UI; a summary `q` against a still-running child still terminates and reaps it; the failure hook batches into `Init` and surfaces as `failMsg`; and `Run`'s post-program safety net reaps the child and writes exactly one diagnostic even when the program dies without the model's quit command.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestQWhileSearchingCancels|TestCtrlCWhileSearchingCancels|TestCtrlCOnSummaryCancels|TestEscWhileSearchingNoOp|TestLateCompletionAfterCancelDiscarded|TestQDuringGateHeldPreparationCancels|TestOrdinaryQuitTerminatesRunningChild|TestFailMsgTriggersCleanup|TestInitRunsFailureHook|TestRunCleansUpOnProgramError' ./internal/app/ 2>&1 | grep -E '^(--- (PASS|FAIL)|ok|FAIL|PASS)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestQWhileSearchingCancels
--- PASS: TestCtrlCWhileSearchingCancels
--- PASS: TestCtrlCOnSummaryCancels
--- PASS: TestEscWhileSearchingNoOp
--- PASS: TestLateCompletionAfterCancelDiscarded
--- PASS: TestQDuringGateHeldPreparationCancels
--- PASS: TestOrdinaryQuitTerminatesRunningChild
--- PASS: TestFailMsgTriggersCleanup
--- PASS: TestInitRunsFailureHook
--- PASS: TestRunCleansUpOnProgramError
PASS
ok  	vrg/internal/app
```

PTY harness tests (`cmd/vrg/cancel_test.go`): `startVrgTermPTY` opens the pty pair itself, keeps the slave fd, and captures `term.GetState` before launch so the test requires the PTY input modes after exit to equal those before. The controllable fake rg (`blockedRG`) writes a ready file, records its pid, then `exec sleep`s — only vrg's `Terminate` ends it. `TestCancelWhileSearchingKillsChild` sends `q` and `ctrl+c` against the blocked child: exit 130, the pid gone, the `VRG_TEST_REAP` line proving vrg's wait/reap path ran (`code=-1`), the display-restoration sequences emitted, termios equal, no further screen. `TestQDuringGateHeldPreparationCancels` holds index preparation after the collect-ack and confirms `q` is cancellation (130), not a browse quit. `TestOrdinaryQuitLeavesNoChild` shows the summary `q` exit 0 with reap evidence. `TestControlledFailureCleanupExit2` triggers `VRG_TEST_FAIL` after the child's ready signal: exit 2, the sanitized `vrg: injected test failure ^[[7m` diagnostic exactly once and after the restoration sequence, child reaped, termios restored.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestCancelWhileSearchingKillsChild|TestQDuringGateHeldPreparationCancels|TestOrdinaryQuitLeavesNoChild|TestControlledFailureCleanupExit2' ./cmd/vrg/ 2>&1 | grep -E '^(=== RUN|    --- (PASS|FAIL)|--- (PASS|FAIL)|ok|FAIL|PASS)' | grep -v '^=== RUN' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestCancelWhileSearchingKillsChild
    --- PASS: TestCancelWhileSearchingKillsChild/q
    --- PASS: TestCancelWhileSearchingKillsChild/ctrl+c
--- PASS: TestQDuringGateHeldPreparationCancels
--- PASS: TestOrdinaryQuitLeavesNoChild
--- PASS: TestControlledFailureCleanupExit2
PASS
ok  	vrg/cmd/vrg
```

Manual scenario — the real binary and the real rg on a pty. `pty_cancel.py` (in this directory) runs `vrg . /`, a whole-filesystem search that stays on "Searching…", twice: once cancelled with `q`, once with `ctrl+c`. For each it records the rg child pid while searching, then after vrg exits asserts the prompt is back (a shell spawned on the same pty answers `echo back-at-prompt`), exit status 130, the recorded rg pid is gone (no orphan), the leave-alt-screen and cursor-visible sequences were emitted, and `stty -a` on the slave is byte-identical to its pre-run state (cooked mode, echo, icanon restored).

```bash
cd /home/chris/vrg && go build -o Notes/walkthroughs/004-04/code-walkthrough/vrg ./cmd/vrg && cd Notes/walkthroughs/004-04/code-walkthrough && file vrg | cut -d: -f2 | cut -c1-60
```

```output
 ELF 64-bit LSB executable, ARM aarch64, version 1 (SYSV), s
```

```bash
cd /home/chris/vrg/Notes/walkthroughs/004-04/code-walkthrough && python3 pty_cancel.py
```

```output
q      : exit=130, rg child gone, alt-screen/cursor restored, stty -a identical, prompt alive
ctrl+c : exit=130, rg child gone, alt-screen/cursor restored, stty -a identical, prompt alive
OK
```

## Result

Every Issue #4 requirement is demonstrated. The model suite proves the cancellation rules: `q` while searching — including gate-held post-exit preparation — and `ctrl+c` in any state start a controlled exit at status 130; `Esc` while searching is a no-op; late search completions after cancellation are discarded; ordinary exits terminate and reap a still-running child; the injected failure hook surfaces through `failMsg`; and `Run`'s post-program net reaps the child with an exactly-once diagnostic when the program dies without a model quit. The PTY harness suite proves the boundary contracts end to end against a controllable fake rg: cancellation exits 130 with the child terminated and reaped (`VRG_TEST_REAP` evidence, not an inferred missing pid), the display-restoration sequence emitted, and PTY termios after exit equal to before launch — and the injected controlled failure exits 2 with the sanitized diagnostic written exactly once after restoration. The manual runs cancel a real `vrg . /` search mid-flight with `q` and `ctrl+c`: prompt returned, exit 130, no orphaned rg, `stty -a` identical to its pre-run state. Per `Notes/PRD-vrg.md` and `Notes/issues/004-cancellation-child-cleanup-terminal-restore.md`; stderr session replay is deliberately deferred to Issue #11.
