# Issue #11: Stderr replay of collected diagnostics

*2026-09-12T00:49:19Z by Showboat 0.6.1*
<!-- showboat-id: 7185756b-ce56-405f-8eb0-ef023ee54799 -->

Walkthrough for Issue #11 (Notes/tasks/011-stderr-replay-of-collected-diagnostics.md), implementing the session diagnostic collection independent of display, the processed-versus-in-flight shutdown boundary, and post-restoration stderr replay of every collected diagnostic exactly once in collection order. References: Notes/PRD-vrg.md (Colours, overlays, and key precedence — replay bullet; Outcome and exit-status contract).

Contracts verified:
- The model maintains a session diagnostic collection independent of what was displayed: one displayed in an overlay and two never displayed all reach the replay writer exactly once each, in collection order, on a normal exit.
- The shutdown boundary is defined at message-processing time: a diagnostic is collected once the model has processed the message carrying it; in-flight diagnostics are not awaited or replayed.
- The same boundary holds for both cancellation keys (ctrl+c and q while searching or gate-held preparation is incomplete), with exit 130 and no wait on undelivered work.
- The Issue #4 controlled-failure diagnostic enters the collection before shutdown and is replayed by the common post-restoration writer with no separate direct write, so exactly-once holds across both mechanisms.
- A diagnostic embedding a filename with newline and ESC is escaped and single-lined through the Issue #6 utility.
- The replay writer is added to the Issue #6 sink-safety table.
- PTY tests wait for the application-side collection acknowledgement (not a child-side write handshake) before sending the exit key.
- Replay occurs after the display-restoration sequence with termios already restored.

All generated artifacts live in this directory.

```bash
cd /home/chris/vrg && go build ./... && go vet ./... && echo GATES-OK
```

```output
GATES-OK
```

```bash
cd /home/chris/vrg && go test -count=1 ./... -timeout 120s | sed 's/[[:space:]][0-9.]*s$//'
```

```output
ok  	vrg/cmd/vrg
ok  	vrg/internal/app
ok  	vrg/internal/cli
ok  	vrg/internal/filebuffer
ok  	vrg/internal/safepresentation
ok  	vrg/internal/searchindex
?   	vrg/internal/sinkfixtures	[no test files]
ok  	vrg/internal/theme
?   	vrg/internal/viewport	[no test files]
```

## Model collection and shutdown-boundary tests

The model tests (internal/app/replay_test.go, Issue #11) verify the session diagnostic collection is independent of display, the shutdown boundary for both cancellation keys, exactly-once across the direct-write and replay mechanisms, and escaped single-lined embedded filenames.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/app/ -run '^TestReplay' -timeout 30s | sed 's/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
```

```output
=== RUN   TestReplayCollectsDisplayedAndNeverDisplayedDiagnostics
--- PASS: TestReplayCollectsDisplayedAndNeverDisplayedDiagnostics (0.00s)
=== RUN   TestReplayNeverDisplayedDiagnosticCollected
--- PASS: TestReplayNeverDisplayedDiagnosticCollected (0.00s)
=== RUN   TestReplayCtrlCAfterProcessedDiagnosticReplayed
--- PASS: TestReplayCtrlCAfterProcessedDiagnosticReplayed (0.00s)
=== RUN   TestReplayCtrlCGatedDiagnosticNotCollected
--- PASS: TestReplayCtrlCGatedDiagnosticNotCollected (0.00s)
=== RUN   TestReplayQAfterProcessedDiagnosticWhileSearching
--- PASS: TestReplayQAfterProcessedDiagnosticWhileSearching (0.00s)
=== RUN   TestReplayQGatedDiagnosticNotCollected
--- PASS: TestReplayQGatedDiagnosticNotCollected (0.00s)
=== RUN   TestReplayControlledFailureCollectedExactlyOnce
--- PASS: TestReplayControlledFailureCollectedExactlyOnce (0.00s)
=== RUN   TestReplayControlledFailureWithEarlierDiagnostic
--- PASS: TestReplayControlledFailureWithEarlierDiagnostic (0.00s)
=== RUN   TestReplayEscapesFilenameInDiagnostic
--- PASS: TestReplayEscapesFilenameInDiagnostic (0.00s)
=== RUN   TestReplaySinkSafetyTable
=== RUN   TestReplaySinkSafetyTable/OSC
=== RUN   TestReplaySinkSafetyTable/CSI
=== RUN   TestReplaySinkSafetyTable/C0
=== RUN   TestReplaySinkSafetyTable/C1
=== RUN   TestReplaySinkSafetyTable/DEL
=== RUN   TestReplaySinkSafetyTable/StandaloneCR
=== RUN   TestReplaySinkSafetyTable/InvalidUTF8
=== RUN   TestReplaySinkSafetyTable/EmbeddedNewline
--- PASS: TestReplaySinkSafetyTable (0.00s)
    --- PASS: TestReplaySinkSafetyTable/OSC (0.00s)
    --- PASS: TestReplaySinkSafetyTable/CSI (0.00s)
    --- PASS: TestReplaySinkSafetyTable/C0 (0.00s)
    --- PASS: TestReplaySinkSafetyTable/C1 (0.00s)
    --- PASS: TestReplaySinkSafetyTable/DEL (0.00s)
    --- PASS: TestReplaySinkSafetyTable/StandaloneCR (0.00s)
    --- PASS: TestReplaySinkSafetyTable/InvalidUTF8 (0.00s)
    --- PASS: TestReplaySinkSafetyTable/EmbeddedNewline (0.00s)
=== RUN   TestReplayOnCollectAcknowledgement
--- PASS: TestReplayOnCollectAcknowledgement (0.00s)
PASS
ok  	vrg/internal/app
```

## PTY acknowledgement and ordering tests

The PTY tests (cmd/vrg/replay_test.go, Issue #11) verify the application-side collection acknowledgement before the exit keypress, replay after the display-restoration sequence with termios already restored, exactly-once alongside earlier diagnostics in collection order, both cancellation keys while work is still in flight, the controlled-failure unification, and the escaped filename case. The tests use the VRG_TEST_COLLECT_ACK side channel to wait for the model to process the diagnostic into the session collection before sending the exit key.

```bash
cd /home/chris/vrg && go test -count=1 -v ./cmd/vrg/ -run '^TestReplay' -timeout 60s | sed 's/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
```

```output
=== RUN   TestReplayCtrlCAfterStderrDiagnostic
--- PASS: TestReplayCtrlCAfterStderrDiagnostic (0.00s)
=== RUN   TestReplayQWhileSearchingAfterDiagnostic
--- PASS: TestReplayQWhileSearchingAfterDiagnostic (0.00s)
=== RUN   TestReplayQWhileGateHeldAfterDiagnostic
--- PASS: TestReplayQWhileGateHeldAfterDiagnostic (0.00s)
=== RUN   TestReplayNormalQAfterCompletedStreamWithWarning
--- PASS: TestReplayNormalQAfterCompletedStreamWithWarning (0.00s)
=== RUN   TestReplayControlledFailureWithEarlierDiagnostic
--- PASS: TestReplayControlledFailureWithEarlierDiagnostic (0.00s)
=== RUN   TestReplayFilenameWithNewlineAndESC
--- PASS: TestReplayFilenameWithNewlineAndESC (0.00s)
PASS
ok  	vrg/cmd/vrg
```

## Manual case: stderr replay after TUI closes

Using the built vrg binary with a fake rg (fakebin/rg) that emits stderr "warn one" followed by a valid ripgrep JSON stream (begin, match, end, summary) and exits 0. The PTY helper sends Esc to dismiss the warning overlay, then q to quit. The stripped PTY output shows the warning overlay with "warn one" and the browse view with demo.txt. After the TUI closes, stderr contains "warn one" exactly once. The exit code is 0.

```bash
cd /home/chris/vrg/Notes/walkthroughs/011-06/code-walkthrough && PATH=$(pwd)/fakebin:/usr/bin:/bin VRG_KEYS=$'\x1b,q' VRG_DELAY=1.0 VRG_STDERR=stderr.txt timeout 15 python3 runpty.py ./vrg hello . 2>&1; echo "exit=$?"; echo '--- stderr ---'; cat stderr.txt
```

```output
┌──────────┐
│ warn one │
│          │
└──────────┘
demo.txt                  ── demo.txt ── Loading…
exit=0
--- stderr ---
warn one

```

## Summary

The walkthrough demonstrates:
- Model collection and shutdown-boundary tests: the session diagnostic collection is independent of display (one displayed, two never displayed, all three collected exactly once in collection order), the shutdown boundary for both cancellation keys (processed diagnostics replayed, gated diagnostics not awaited or replayed, exit 130), the controlled-failure unification (routed through the collection, no separate direct write, exactly-once across both mechanisms), filename escaping (newline and ESC escaped and single-lined through the Issue #6 utility), and the replay writer added to the Issue #6 sink-safety table.
- PTY acknowledgement and ordering tests: ctrl+c after a collected diagnostic exits 130 with termios restored and the diagnostic on stderr exactly once after display restoration, q while searching or gate-held after a collected diagnostic exits 130 with the diagnostic replayed exactly once, normal q after a completed stream with a warning replays the warning exactly once, the controlled-failure diagnostic appears exactly once alongside an earlier diagnostic in collection order with no duplicate, and the filename with newline and ESC is escaped and single-lined in the replayed stderr.
- The manual case: a fake rg emitting stderr "warn one" with a valid stream, browsing, pressing q, and the shell showing "warn one" exactly once after the TUI closes.

References: Issue #11 (Notes/tasks/011-stderr-replay-of-collected-diagnostics.md), Notes/PRD-vrg.md (Colours, overlays, and key precedence — replay bullet; Outcome and exit-status contract).
