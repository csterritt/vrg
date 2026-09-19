# Deterministic PTY handshakes

Issue #48 ([issue](../issues/048-pty-tests-deterministic-handshakes.md),
[task](../tasks/048-pty-tests-deterministic-handshakes.md)) replaces
implicit ordering assumptions in the PTY/subprocess tests with explicit,
application-side acknowledgements: every key send or trigger action in a
PTY helper waits for the exact application transition that action causes
— never for elapsed time and never only for output that could match an
older frame. The mechanism is the per-message event acknowledgement the
[PRD](../PRD-vrg.md) *Testing Decisions* section prescribes: "acknowledge
each Update-processed message" over a per-process channel, correlated
per occurrence.

## The seam

The tagged `vrg_testhooks` binary gains one hook,
`VRG_TEST_EVENT_ACK`, a file path the application appends one
acknowledgement record — `"<seq> <event>"`, seq monotonic per process —
per `Update`-processed message and per awaited transition to. Inside
`internal/app` this is `options.event` / `WithEventAck`: `Update` is
split into a thin wrapper that calls the unchanged real dispatch
(`update`) and then emits a record describing the processed message,
and a handful of `m.ack(...)` call sites inside the dispatch record the
transitions tests wait on. The record vocabulary:

- `state:searching` (emitted by `Init`) and `state:browse` /
  `state:noresults` / `state:overlayonly` (emitted when
  `searchDoneMsg` settles the state).
- `key:<keystroke>` — every `tea.KeyPressMsg`, in bubbletea's
  `Keystroke()` spelling (`q`, `esc`, `ctrl+c`, `n`, …).
- `overlay:open` / `overlay:append` / `overlay:dismissed` — from
  `openOverlay` and the dismissal branch.
- `load:ok` / `load:fail` / `load:stale` — a `fileLoadedMsg` settled
  (applied, failed, or discarded as stale/unrequested).
- `layout` / `layout:stale` — a `layoutReadyMsg` installed or was
  discarded.
- `diag` — one per line `collectDiags` collects; `collected` — the
  child's stream finished collecting.
- `stderrline`, `fileloaded`, `layoutready`, `size`, `popup`, `fail` /
  `fail:nil`, `other` — the per-message records for the remaining
  `Update` inputs (a message discarded under `quitting` earns none).

When no sink is installed `Update` is the same pass-through call as
before; the untagged production binary wires no sink at all, so the
seam is absent and inert there. The hook is a member of Issue #45's
explicit hook manifest in `testhooks_test.go` and is covered by
`TestProductionBinaryIgnoresHookManifest`'s untagged-artifact probe —
a live writer path is set in the environment, the production binary
ignores it, and the name does not appear in the artifact bytes. See
[test-hook-topology.md](test-hook-topology.md).

The seam observes production behavior and timing only: records are
emitted *after* the real `update` returns, in the same goroutine, so
each record is proof the real transition already happened — it never
pre-signals, injects, or reschedules anything.

## Correlation

Acknowledgements are correlated per process (each run points
`VRG_TEST_EVENT_ACK` at a fresh `t.TempDir()` file via `ackEnv`, and
the log's `seq` is monotonic within that process) and per occurrence:
`s.ackCount(event)` counts the records of a name already present, and
`s.awaitAck(event, have, bound)` succeeds only when the count grows
past the `have` baseline captured immediately before the triggering
action. An earlier same-kind event can therefore never satisfy a later
wait — sending `q` twice requires two distinct `key:q` records — and a
missing acknowledgement fails within the caller's bound (20 s for
`waitAck`, an explicit duration for `awaitAck`) with a message naming
the event, the awaited occurrence, the log, and the PTY output rather
than hanging; a process exit before the ack is reported the same way.
`keyEvent(key)` maps a sent byte sequence to its record name
(`"\x1b"`→`key:esc`, `"\x03"`→`key:ctrl+c`).

## Helper contract

`cmd/vrg/handshake_test.go` owns the contract, the matrix (reproduced
in its header comment), and its proofs. The finite
helper/action/postcondition/acknowledgement matrix:

| Helper / test group | Action | Postcondition | Acknowledgement | Unlocks |
|---|---|---|---|---|
| `runVrgWithQuit(AckBin)` | process start | `searchDoneMsg` processed, browse entered | `state:browse` | send `q` |
| | send `q` | key processed | `key:q` | `waitExit` |
| `runVrgWithQuitBin` (prod probe) | process start | browse frame rendered | output marker only — the untagged binary has no seam | send `q` |
| `runSteps` (per `keyStep`) | send key | key processed | `key:<keystroke>` from per-step baseline | step `ack`/`expect` waits |
| | `keyStep.ack` field | named transition occurred | e.g. `overlay:dismissed` | `expect` output check, next step |
| | `keyStep.expect` field | rendered postcondition | output written since the send | next step |
| outcome overlay steps | stream completes | fatal/warning overlay open | `overlay:open` + marker | dismissal key |
| dismissal steps | `Esc`/`q` on the overlay | overlay dismissed | `overlay:dismissed` + repaint | the following `q` |
| `runVrgKillChild` | ready file + SIGKILL | child death processed to overlay | `overlay:open` | the steps |
| replay/cancel diag waits | child writes stderr | diagnostic collected | `diag` (one per line) | send key |
| gate-held tests | child exits | stream fully collected | `collected` | release the gate |
| controlled-failure tests | trigger file appears | `failMsg` processed | `fail` | exit |
| load-dependent content | browse entered | current file's load settled | `load:ok` / `layout` | content repaint |

Every row's postcondition is an application-side record emitted by the
real `Update` dispatch — output markers remain only as *content*
assertions after the acknowledgement proves the transition. The static
check `TestNoFixedSleepsInPTYHelpers` parses every `cmd/vrg/*.go` file's
AST and forbids `time.Sleep` outside the allow-listed bounded condition
polls — `waitFor`, `waitForFrom`, `waitForFile`, `waitForAcks`,
`awaitAck`, Issue #50's `waitProbeGone` (the ESRCH
process/process-group disappearance poll backing
`TestCancelTerminatesChildProcessGroup`), and the tagged seam's
`waitFileGone`/`waitFileExists`
watchers (whose unbounded holds are the gate mechanism itself, not
test-side synchronization) — each of which re-checks an explicit
condition every 10 ms against a deadline; a permitted poll in test code
must also name its `deadline`/`bound`, and a bare `time.Sleep` in a
helper or test is a build break. The check is AST-level rather than a
grep precisely because sleeps inside bounded condition polls remain
permitted.

The contract proofs:

- `TestEventAckSeamRecordsLifecycle` — a real PTY run records
  `state:searching` → `collected` → `state:browse` → `key:q` in causal
  order with strictly increasing per-process sequence numbers.
- `TestAckCorrelationSameKindOccurrences` — two `n` sends require two
  distinct `key:n` records; the second wait baselines past the first.
- `TestOverlayDismissalAcknowledgedBeforeQuit` — the dismissal send's
  `overlay:dismissed` is recorded ahead of both `key:q` records, so the
  following quit is acknowledged only after dismissal settled.
- `TestAckWaitFailsOnBoundedTimeout` — `awaitAck` on a never-arriving
  event errors inside its bound naming the event, occurrence, and
  bound — never a hang.
- `TestNoFixedSleepsInPTYHelpers` — the static check above.

## What changed in the helpers

- `ptySession` gained `ackPath` (resolved from the env by
  `ackPathFromEnv`) plus `ackRecords`/`ackCount`/`awaitAck`/`waitAck`;
  `keyStep` gained an `ack` field naming the transition the send must
  cause. `runSteps` baselines each record it will wait for *before*
  writing the key, then waits `key:<send>`, then `step.ack`, then the
  output marker — all against bytes/records produced after the send.
- `runVrgWithKeys`, `runVrgKillChild`, and `runshape_test.go` install
  `ackEnv(t)`; `runVrgWithQuit` waits `state:browse` before its `q` and
  `key:q` before `waitExit`.
- `search_test.go`'s direct-session tests wait on `state:searching` /
  `state:browse` / `overlay:open` / `overlay:dismissed` / `key:*` at
  the points they previously assumed had happened.
- `cancel_test.go` waits on `state:searching` before sending and on
  `key:*` / `fail` after triggering.
- `replay_test.go` waits on `diag`/`collected` records before triggers
  and on `key:*`/`overlay:dismissed`/`fail` after sends; the
  repeated-`q` regression baselines each `key:q` wait.

Related: [test-hook-topology.md](test-hook-topology.md),
[stderr-replay.md](stderr-replay.md),
[cancellation-cleanup.md](cancellation-cleanup.md), and the test
catalog in [unit-tests.md](unit-tests.md).
