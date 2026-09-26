# Issue #4: cancellation, child cleanup, and terminal restoration

*2026-09-23T17:35:40Z by Showboat 0.6.1*
<!-- showboat-id: 2de47a6a-74f9-4038-9e95-6ca138d03fe2 -->

Walkthrough for [Issue #4](../../../issues/004-cancellation-child-cleanup-terminal-restore.md), implementing cancellation, child cleanup, and terminal restoration per `Notes/PRD-vrg.md` (Implementation Decisions → Outcome and exit-status contract rows 1-2 plus the cleanup bullet; Module Design → App; Testing Decisions → subprocess boundary). Cancellation keys: `ctrl+c` in any state and `q` while searching — where "searching" spans the post-exit preparation window — terminate and reap the rg child, restore the display (leave alt screen, show cursor) and the PTY input modes, and exit 130. Controlled failures run the same cleanup, then write one sanitized `vrg:` diagnostic to stderr after restoration and exit 2. All generated artifacts live in this directory: the built `vrg` binary, `demo-cancel.sh`, and its `manual-run/` session files. Durations are stripped so the document verifies cleanly.

```bash
cd /home/chris/vrg && go vet ./... && go build ./... && go test -count=1 ./internal/app ./cmd/vrg | sed "s/[[:space:]][0-9.]*s$//" && echo GATES-OK
```

```output
ok  	vrg/internal/app
ok  	vrg/cmd/vrg
GATES-OK
```

Model-level cancellation driven through `Update`: `q` while searching quits with exit 130 and fires the session cancel; `q` in the gate-held post-exit preparation window cancels without ever preparing an index; `ctrl+c` cancels from both searching and summary; `Esc` during searching is a strict no-op; and a search completion delivered after cancellation cannot revive the UI.

```bash
cd /home/chris/vrg && go test -count=1 -v -run "TestQWhileSearchingCancels|TestQDuringGateHeldPreparationCancels|TestCtrlCCancelsFromAnyState|TestEscDuringSearchingIsNoOp|TestLateCompletionAfterCancellationDoesNotRevive|TestQOnSummaryExitsZero" ./internal/app 2>&1 | grep -vE "^(=== RUN|    --- (PASS|FAIL))" | sed -E "s/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//"
```

```output
--- PASS: TestQOnSummaryExitsZero
--- PASS: TestQWhileSearchingCancels
--- PASS: TestQDuringGateHeldPreparationCancels
--- PASS: TestCtrlCCancelsFromAnyState
--- PASS: TestEscDuringSearchingIsNoOp
--- PASS: TestLateCompletionAfterCancellationDoesNotRevive
PASS
ok  	vrg/internal/app
```

Subprocess boundary in `internal/app`: `Session.Cancel` against a fake rg that signals readiness then blocks forever — the child pid vanishes (`ESRCH`), the `Reaped` channel closes promptly, and the `ReapReport` side channel records the `signal: killed` wait status, proving vrg's own `Wait` path ran.

```bash
cd /home/chris/vrg && go test -count=1 -v -run "TestCancelTerminatesAndReapsChild" ./internal/app 2>&1 | grep -vE "^(=== RUN|    --- (PASS|FAIL))" | sed -E "s/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//"
```

```output
--- PASS: TestCancelTerminatesAndReapsChild
PASS
ok  	vrg/internal/app
```

The PTY harness (`cmd/vrg/pty_test.go`) runs the real binary with a `/dev/ptmx` slave as controlling terminal for stdin/stdout/stderr and captures every byte from the master. Fake-rg scripts write their pid to a `ready` handshake file; the `VRG_TEST_REAP_FILE` side channel records the reaped wait status. Each test asserts the exit status, the vanished pid, the reap evidence, the display-restoration sequences (`\x1b[?1049l`, `\x1b[?25h`), and byte-identical slave termios before and after. `q`, the raw `ctrl+c` byte, and a real `SIGINT` signal all cancel a blocked search with exit 130.

```bash
cd /home/chris/vrg && go test -count=1 -v -run "TestPTYQWhileSearchingExits130|TestPTYCtrlCWhileSearchingExits130|TestPTYSIGINTWhileSearchingExits130" ./cmd/vrg 2>&1 | grep -vE "^(=== RUN|    --- (PASS|FAIL))" | sed -E "s/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//"
```

```output
--- PASS: TestPTYQWhileSearchingExits130
--- PASS: TestPTYCtrlCWhileSearchingExits130
--- PASS: TestPTYSIGINTWhileSearchingExits130
PASS
ok  	vrg/cmd/vrg
```

The gate-held window and the ordinary exit path: with `VRG_TEST_GATE_FIFO` holding index preparation after rg has already exited and been reaped (`exit status 0` on the side channel), `q` still cancels — exit 130, no interim summary rendered. A `SIGTERM` on the searching screen (an exit with no cancellation key) also terminates and reaps the blocked child and restores the terminal — every controlled exit runs the same cleanup.

```bash
cd /home/chris/vrg && go test -count=1 -v -run "TestPTYQDuringGateHeldPreparationExits130|TestPTYOrdinaryExitReapsChild" ./cmd/vrg 2>&1 | grep -vE "^(=== RUN|    --- (PASS|FAIL))" | sed -E "s/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//"
```

```output
--- PASS: TestPTYQDuringGateHeldPreparationExits130
--- PASS: TestPTYOrdinaryExitReapsChild
PASS
ok  	vrg/cmd/vrg
```

Injected controlled failure: `VRG_TEST_FAIL_FIFO` cancels the program context mid-search. The PTY test asserts exit 2, the child reaped, termios restored, and the `vrg:` diagnostic present exactly once and only *after* the leave-alt-screen sequence. The pipe-mode test pins the diagnostic to stderr alone — exactly once, never on stdout — with the child still reaped.

```bash
cd /home/chris/vrg && go test -count=1 -v -run "TestPTYControlledFailureExits2|TestControlledFailureDiagnosticOnStderr" ./cmd/vrg 2>&1 | grep -vE "^(=== RUN|    --- (PASS|FAIL))" | sed -E "s/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//"
```

```output
--- PASS: TestPTYControlledFailureExits2
--- PASS: TestControlledFailureDiagnosticOnStderr
PASS
ok  	vrg/cmd/vrg
```

```bash
cd /home/chris/vrg && go build -o Notes/walkthroughs/004-04/code-walkthrough/vrg ./cmd/vrg && cd Notes/walkthroughs/004-04/code-walkthrough && test -x vrg && ./vrg --help | head -1
```

```output
Usage: vrg [OPTIONS] PATTERN [ROOT]
```

Manual long-search cancellation with the real binary on a real terminal session. `demo-cancel.sh` uses `script(1)` to allocate a pseudo-terminal (`-f` flushes the typescript so the harness can observe frames live); inside the session an inner script sizes the pty, records `stty -a`, runs `./vrg foo /usr` — real rg, a multi-second scan — then records the exit status and `stty -a` again. Outside, the harness counts `rg` processes, waits for the `Searching…` frame in the typescript, writes a literal `q` to the pty master, and waits for the session to return to the shell prompt.

```bash
./demo-cancel.sh
```

```output
searching-frame=seen
process-exit=130
rg-procs: before=0 after=0
display-restore=leave-alt-screen+cursor-show
termios=identical
```

## Result. Every Issue #4 behavior is demonstrated: `q` while searching and `ctrl+c` in any state cancel with exit 130 — including the post-exit gate-held preparation window, which stays part of searching — while `Esc` remains a no-op and `q` on the interim summary still exits 0. The single cleanup boundary terminates and reaps the rg child on every controlled exit (ordinary quit included), with the `VRG_TEST_REAP_FILE` side channel proving `Wait` ran and `ESRCH` proving no orphan or zombie. Both halves of terminal restoration hold: leave-alt-screen plus cursor-show in the captured stream, and byte-identical slave termios. Controlled failures write the sanitized `vrg:` diagnostic exactly once, after restoration, on stderr, and exit 2; a late search completion cannot revive a cancelled UI. The manual session shows the real observable contract: the prompt returns, `$?` is 130, no rg process survives, and `stty -a` is unchanged. Deferred by design: session-replay collection (Issue #11) and stderr outcome classification (Issue #9).
