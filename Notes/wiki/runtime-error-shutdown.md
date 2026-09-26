# Runtime errors through the common shutdown/diagnostic-replay path (Issue #46)

The unified shutdown contract delivered by Issue #46
(`Notes/tasks/046-runtime-error-common-diagnostic-replay.md`), spanning
`cmd/vrg/main.go` (the `diagSnapshot` and the ordered shutdown sequence)
and `internal/app` (`Config.OnCollect` and `Model.onCollect`). Relevant
PRD section: *Outcome and exit-status contract* (the cleanup bullet —
cleanup applies on normal completion, cancellation, and application
failures under vrg's control).

## The diagnostic snapshot decoupled from the final model

Before Issue #46 the only channel carrying collected diagnostics to
stderr was the **final model** `Run()` returned: `runSearch` asserted
`fm.(app.Model)` and called `ReplayTo`. A `Run()` that returned a nil
or wrong-type final model therefore silently dropped every diagnostic
the session had collected — and, with a nil error, exited 0.

`Config.OnCollect` (`internal/app/search.go`) closes that hole: it is a
per-collection callback the boundary wires in at session start. The
model's `collect` invokes `m.onCollect(d)` with each already-sanitized
line right after appending it to `Model.diags` and writing the
`DiagAck` acknowledgement — so the process boundary accumulates its own
copy, `diagSnapshot` in `cmd/vrg/main.go`, independent of the
final-model type assertion. Because collection happens only inside
`Update`, the snapshot is complete once `Run()` returns and the
boundary reads it without synchronisation. `Model.diags`/`ReplayTo`
remain the in-model collection the unit tests drive; the boundary
replays the snapshot, not the model.

## The ordered shutdown sequence

Every `Run()` return shape funnels through one sequence in `runSearch`:

1. `runProgram` returns — Bubble Tea has already restored the terminal
   (leave-alt-screen, cursor-show, termios).
2. `sess.Cancel()` + `<-sess.Reaped()` — the child is terminated and
   reaped exactly as on normal exits, even when cleanup already ran
   inside the model before `Run()` returned.
3. Only after both restoration and cleanup are complete does the
   snapshot replay to stderr, in order:
   - session diagnostics retained in the snapshot, in collection order;
   - the invalid-final-model diagnostic when `fm` is absent or of the
     wrong type — `vrg: program returned a nil final model` /
     `vrg: program returned an unexpected final model`
     (`finalModelDiagnostic`) — a failing shape is never a silent exit;
   - the `program.Run()` runtime error, appended exactly once through
     `present.Diagnostic` — the single writer means no direct-write
     plus replay duplication.

## Exit statuses per return shape

- Valid final model + nil error → `model.ExitCode()` (the fixed search
  status 0/1/2, or 130 cancellation).
- Valid model + `tea.ErrInterrupted` → 130 (a real SIGINT).
- **Every** failing shape → exit 2: any non-interrupt `Run()` error, an
  absent final model, or a wrong-type final model — including the
  nil-model/nil-error shape that previously exited 0. The exit-2
  controlled-failure convention the startup-failure path established
  now extends to runtime errors and invalid return shapes.

## The tagged-runner injection the tests use

The tests drive `VRG_TEST_RUN_FINAL_MODEL` (`valid`/`nil`/`invalid`)
and `VRG_TEST_RUN_ERROR` through the Issue #45 `runProgram` seam — the
override is applied after the real `Run()` completes, so the injected
tuple reaches the executable's actual post-`Run()` type/error branches.
Because a plain searching-quit would exit 130, observing exit 2 plus
the injected texts proves the substituted tuple — not a controlled
model quit — decided the branch. See
[test-hook-build-topology.md](test-hook-build-topology.md).

`cmd/vrg/pty_returnshape_test.go` (`TestPTYRunReturnShapes`) runs the
full matrix on the real PTY lifecycle — `warn one` collected and
acknowledged before `q` ends the real program — asserting exit 2, the
ordered replay tail, exactly-once emission, the `killed` reap status,
the gone pid, and both halves of terminal restoration.
`TestTaggedRunnerSeamSelectsReturnShape` covers the same contract on
pipes. See [unit-tests.md](unit-tests.md).

## Files

- `cmd/vrg/main.go` — `diagSnapshot` (`collect`/`replay`),
  `finalModelDiagnostic`, and `runSearch`'s single shutdown switch.
- `internal/app/search.go` — `Config.OnCollect`, wired into the model
  in `Start`.
- `internal/app/app.go` — `Model.onCollect`, invoked from `collect`.
- `cmd/vrg/pty_returnshape_test.go` — the return-shape matrix.
- `cmd/vrg/testhooks_test.go` — the updated runner-seam expectations.

See also: [stderr-replay.md](stderr-replay.md) (the session collection
and replay contract the snapshot feeds),
[cancellation-and-cleanup.md](cancellation-and-cleanup.md) (the
terminate/reap half of the sequence),
[test-hook-build-topology.md](test-hook-build-topology.md) (the runner
seam), and [safe-presentation.md](safe-presentation.md) (the
sanitization the boundary diagnostics pass through).
