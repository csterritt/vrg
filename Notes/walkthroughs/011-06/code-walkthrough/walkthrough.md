# Issue #11: Stderr replay of collected diagnostics

*2026-09-23T21:12:15Z by Showboat 0.6.1*
<!-- showboat-id: c78cd858-1ea5-4bde-beb6-eadea58e94f1 -->

Walkthrough for [Issue #11](../../../issues/011-stderr-replay-of-collected-diagnostics.md), implementing the stderr-replay contract per `Notes/PRD-vrg.md` (*Colours, overlays, and key precedence* — the last bullet; *Outcome and exit-status contract* — the cleanup bullet). The model maintains a session diagnostic collection independent of what any screen displayed; child stderr lines stream to it as `diagMsg`s while the search runs, completion diagnostics and load failures join as their messages are processed, and a diagnostic counts as collected only once the model has processed the message carrying it — in-flight work at exit is neither waited for nor replayed. On every controlled exit — normal completion, cancellation, controlled failure — the collection replays to sanitized stderr exactly once, in collection order, after terminal restoration. The controlled-failure diagnostic now enters the collection rather than writing directly, so exactly-once holds across both mechanisms. All generated artifacts live in this directory: the built `vrg` binary, the `demo-warn-replay.sh` harness, and its `manual-warn-replay/` tmux session capture. Test durations are stripped so the document verifies cleanly.

```bash
cd /home/chris/vrg && go vet ./... && go build ./... && go test -count=1 ./... | sed "s/[[:space:]][0-9.]*s$//" && echo GATES-OK
```

```output
ok  	vrg/cmd/vrg
ok  	vrg/internal/app
ok  	vrg/internal/cli
ok  	vrg/internal/filebuffer
ok  	vrg/internal/present
ok  	vrg/internal/searchindex
ok  	vrg/internal/theme
?   	vrg/internal/viewport	[no test files]
GATES-OK
```

## Model collection tests

`internal/app/replay_test.go` pins the session-collection contract at the model level. `TestReplayCollectsDisplayedAndUndisplayedInOrder` collects three diagnostics — one stderr warning the overlay displays plus two non-current file-load failures no screen shows — and asserts the replay writer emits exactly those three, in collection order, once each. `TestControlledFailureJoinsSessionCollection` proves the boundary's `CollectDiagnostic` route: the `vrg:` failure line replays after the earlier diagnostics with no separate write. `TestReplayEscapesEmbeddedFilename` drives a load failure on a path with an embedded newline and ESC through the collection and asserts the replayed line carries the `present.Path` single-line form (`f\\nho^[.txt` shape) with no raw control byte.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestReplayCollectsDisplayedAndUndisplayedInOrder|TestControlledFailureJoinsSessionCollection|TestReplayEscapesEmbeddedFilename' ./internal/app 2>&1 | grep -vE '^(=== RUN|    --- (PASS|FAIL))' | sed -E 's/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//'
```

```output
--- PASS: TestReplayCollectsDisplayedAndUndisplayedInOrder
--- PASS: TestControlledFailureJoinsSessionCollection
--- PASS: TestReplayEscapesEmbeddedFilename
PASS
ok  	vrg/internal/app
```

## Shutdown-boundary tests

The collected-versus-in-flight boundary: a diagnostic is collected once the model has *processed* the message carrying it. `TestShutdownBoundaryCtrlC` runs a real `Start`ed session whose fake rg warns on stderr then emits a summary-less stream — its `missing summary` diagnostic exists only inside the completion message the preparation gate holds back. The streamed warning is collected, `ctrl+c` exits 130, and the replay contains only the warning: the gated completion is never waited for and its diagnostics never replay. `TestShutdownBoundaryQ` pins the same boundary on the `q` route in both incomplete states — searching with a live (blocked) child, and post-exit gate-held preparation — each replaying only the collected warning at 130 while collection ends promptly.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestShutdownBoundary' ./internal/app 2>&1 | grep -vE '^(=== RUN|=== CONT|    --- (PASS|FAIL))' | sed -E 's/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//'
```

```output
--- PASS: TestShutdownBoundaryCtrlC
--- PASS: TestShutdownBoundaryQ
PASS
ok  	vrg/internal/app
```

## PTY acknowledgement and ordering tests

`cmd/vrg/pty_replay_test.go` exercises the real binary on the Issue #4 PTY harness. Every test wires `VRG_TEST_DIAG_ACK_FILE` — the test-only side channel that receives one line per diagnostic the model processes into the session collection — and waits for that **application-side** acknowledgement before sending the exit key: a child-side write handshake would only prove bytes reached the pipe. Assertions then pin the replay strictly after the `\\x1b[?1049l` display-restoration sequence with termios already restored: `ctrl+c` and `q` while searching exit 130 with the collected diagnostic exactly once; `q` during gate-held preparation replays the acknowledged warning without ever surfacing the completion's gated `missing summary`; a normal `q` after a completed warning stream exits 0; the injected controlled failure lands at 2 with its `vrg:` diagnostic counted exactly once across both mechanisms in collection order; and a diagnostic embedding a newline-plus-ESC filename replays single-lined and escaped.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestPTYCtrlCAfterDiagnosticReplaysOnce|TestPTYQWhileSearchingReplaysDiagnostic|TestPTYQDuringGateHeldPreparationReplaysDiagnostic|TestPTYQuitAfterCompletedStreamReplaysWarning|TestPTYControlledFailureReplaysAlongsideEarlierDiagnostics|TestPTYReplayEscapesEmbeddedFilename' ./cmd/vrg 2>&1 | grep -vE '^(=== RUN|    --- (PASS|FAIL))' | sed -E 's/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//'
```

```output
--- PASS: TestPTYCtrlCAfterDiagnosticReplaysOnce
--- PASS: TestPTYQWhileSearchingReplaysDiagnostic
--- PASS: TestPTYQDuringGateHeldPreparationReplaysDiagnostic
--- PASS: TestPTYQuitAfterCompletedStreamReplaysWarning
--- PASS: TestPTYControlledFailureReplaysAlongsideEarlierDiagnostics
--- PASS: TestPTYReplayEscapesEmbeddedFilename
PASS
ok  	vrg/cmd/vrg
```

## Manual check — warn, browse, quit, replay

`demo-warn-replay.sh` (checked into this directory) runs the freshly built `vrg` on a real tmux PTY in a fixture directory whose `fakebin/rg` writes `warn one` to stderr, emits a complete one-match stream, and exits 0. The collected diagnostic opens the warning overlay over the browse view; the first `q` dismisses it, the second quits — and the post-exit pane shows the shell sandwiching exactly one `warn one` line: the replay landed on stderr after the TUI restored the terminal.

```bash
cd /home/chris/vrg && go build -o Notes/walkthroughs/011-06/code-walkthrough/vrg ./cmd/vrg && cd Notes/walkthroughs/011-06/code-walkthrough && ./demo-warn-replay.sh
```

```output
exit=0
--- screen: warning overlay over browse ---
f.txt  ── f.txt ────────────────────────────────────────────────────────────────
       1  hello
                                   ┌────────┐
                                   │warn one│
                                   └────────┘
--- screen: the shell after vrg exited ---
shell: about to run vrg
warn one
shell: vrg exited
post-exit 'warn one' lines: 1
```

## Verdict

Issue #11 is verified. The session collection in `Model.diags` is independent of display: child stderr streams in line-by-line through `diagMsg` over an unbuffered channel (so the completion can never overtake a diagnostic), completion diagnostics join through the `completionDiagnostics` subset that excludes stderr to preserve exactly-once, and load failures collect through `CollectDiagnostic`. The shutdown boundary is model processing — a diagnostic handled before the exit keypress replays; one still in flight, including inside a gate-held completion, is neither waited for nor replayed, and cancellation never deadlocks the drain. `runSearch` runs `ReplayTo` on every controlled exit after `Program.Run` has restored display and termios, and the controlled-failure path collects its `vrg:` line rather than writing directly — one writer, one emission, collection order. The PTY tests prove the acknowledgement-before-keypress ordering and post-restoration placement; the tmux run shows the shell receiving `warn one` exactly once after the TUI closes.
