# Issue #11: stderr replay of collected diagnostics

*2026-09-17T00:19:16Z by Showboat 0.6.1*
<!-- showboat-id: 6ca37563-0e62-4e0b-a212-fb0688d71919 -->

Issue #11 makes every diagnostic survive the session: vrg collects each one into a session collection as the model processes the message carrying it — independent of whether an overlay displayed it — and replays the whole collection to sanitized stderr exactly once each, in collection order, after the terminal is restored on every controlled exit (ordinary quit, q/ctrl+c cancellation, controlled failure). The controlled-failure diagnostic from Issue #4 now enters the collection before shutdown and shares the same post-restoration writer; there is no separate direct write, and no persistent log. See `Notes/issues/011-stderr-replay-of-collected-diagnostics.md`, `Notes/tasks/011-stderr-replay-of-collected-diagnostics.md`, and the PRD sections "Outcome and exit-status contract" and "Colours, overlays, and key precedence" in `Notes/PRD-vrg.md`. All artifacts live in this directory.

## Gates — module integrity, build, vet

```bash
cd /home/chris/vrg && go mod verify && go build ./... && go vet ./... && echo GATES-OK
```

```output
all modules verified
GATES-OK
```

## Model — collection, shutdown boundary, exactly-once

`internal/app/replay_test.go` covers the model-level contract: a diagnostic is collected when `Update` processes the message carrying it — a stderr warning shown in the overlay plus two never-displayed load failures all collect in order and replay exactly once; a `stderrLineMsg` processed before `ctrl+c` or `q` is replayed while a diagnostic still in flight is never waited for (including the gate-held preparation window); the `failMsg` controlled failure enters the collection before shutdown and replays last; a child with incremental `Diags()` never re-collects captured stderr at completion; and a filename carrying newline and ESC bytes is `EscapePath`-single-lined in the replayed line.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'Replay|ControlledFailureEntersCollection|CompletionDoesNotRecollect' ./internal/app 2>&1 | grep -E '^(--- |    --- |ok|FAIL)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestReplayCollectsEveryDiagInOrder
--- PASS: TestCtrlCReplayBoundary
--- PASS: TestQWhileSearchingReplayBoundary
    --- PASS: TestQWhileSearchingReplayBoundary/while_searching
    --- PASS: TestQWhileSearchingReplayBoundary/gate-held_preparation
--- PASS: TestControlledFailureEntersCollection
--- PASS: TestCompletionDoesNotRecollectIncrementalStderr
--- PASS: TestReplayEscapesEmbeddedFilename
ok  	vrg/internal/app
```

## PTY — acknowledgement and replay ordering

`cmd/vrg/replay_test.go` drives the real binary on a pty. `VRG_TEST_DIAG_ACK=<file>` is the application-side acknowledgement — one line appended per diagnostic *processed into the session collection* (the same file-evidence family as `VRG_TEST_REAP`), so tests wait on collection rather than a child-side write. The assertions require each diagnostic to appear exactly once in the capture, strictly after the display-restoration sequence (`\x1b[?1049l`), in collection order, with termios restored and the child reaped.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'Replay|Cancel.*Replays|HostileFilename' ./cmd/vrg 2>&1 | grep -E '^(--- |    --- |ok|FAIL)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestCancelReplaysProcessedDiagnostic
    --- PASS: TestCancelReplaysProcessedDiagnostic/q
    --- PASS: TestCancelReplaysProcessedDiagnostic/ctrl+c
--- PASS: TestQDuringGateHeldPreparationReplaysDiagnostic
--- PASS: TestNormalQuitReplaysDiagnosticsInOrder
--- PASS: TestControlledFailureReplaysViaCollection
--- PASS: TestReplayEscapesHostileFilename
ok  	vrg/cmd/vrg
```

## Manual PTY — the issue's manual case end to end

`pty_replay.py` runs the real binary on a pty against a fake rg that emits stderr `warn one` beside a valid stream and exits 0: the warning overlay displays the diagnostic while browsing, `q` dismisses it and `q` quits, and the shell then shows `warn one` exactly once — replayed strictly after the TUI's alt-screen restoration.

```bash
cd /home/chris/vrg && go build -o Notes/walkthroughs/011-06/code-walkthrough/vrg ./cmd/vrg && cd Notes/walkthroughs/011-06/code-walkthrough && python3 pty_replay.py
```

```output
replay       : fake rg stderr 'warn one' + valid stream -> overlay shows it; q q exits 0; the shell sees 'warn one' exactly once after the TUI closes
OK
```

## Full suite — `go test ./...` and the race detector

```bash
cd /home/chris/vrg && go test -count=1 ./... 2>&1 | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//' && CGO_ENABLED=1 go test -count=1 -race ./internal/... ./cmd/vrg 2>&1 | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
?   	vrg/Notes/walkthroughs/007-04/code-walkthrough/fixture	[no test files]
ok  	vrg/cmd/vrg
ok  	vrg/internal/app
ok  	vrg/internal/cli
ok  	vrg/internal/filebuffer
ok  	vrg/internal/safepresentation
?   	vrg/internal/safepresentation/sinktest	[no test files]
ok  	vrg/internal/searchindex
ok  	vrg/internal/theme
?   	vrg/internal/viewport	[no test files]
ok  	vrg/internal/app
ok  	vrg/internal/cli
ok  	vrg/internal/filebuffer
ok  	vrg/internal/safepresentation
?   	vrg/internal/safepresentation/sinktest	[no test files]
ok  	vrg/internal/searchindex
ok  	vrg/internal/theme
?   	vrg/internal/viewport	[no test files]
ok  	vrg/cmd/vrg
```
