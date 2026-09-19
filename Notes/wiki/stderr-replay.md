# Stderr replay of collected diagnostics (Issue #11)

Delivered by
[Issue #11](../issues/011-stderr-replay-of-collected-diagnostics.md):
a session diagnostic collection independent of what any overlay
displayed, replayed exactly once each to sanitized stderr after
terminal/display restoration on every controlled exit. Relevant PRD
sections: *Outcome and exit-status contract* (every controlled exit),
*Colours, overlays, and key precedence* (the `ctrl+c`/`q` shutdown
boundary), and *Testing Decisions → Subprocess boundary* in
[`Notes/PRD-vrg.md`](../PRD-vrg.md). Builds on
[cancellation-cleanup.md](cancellation-cleanup.md) (the Issue #4
post-restoration writer it replaces),
[error-overlay-and-outcomes.md](error-overlay-and-outcomes.md) (the
displayed-diagnostic composition it reuses), and
[safe-presentation.md](safe-presentation.md) (the escaping contract it
inherits).

## The session collection

`model.diags` in `internal/app/app.go` is the session diagnostic
collection: sanitized lines appended by `collectDiags` in the order
`Update` processed the messages carrying them — independent of whether
any overlay ever displayed them. Producers:

- `stderrLineMsg` — one drained child-stderr line, terminator retained.
  `proc` (`internal/app/rg.go`) gained incremental delivery: the
  drainage goroutine line-splits stderr into an unbounded pending queue
  (drainage never waits on a consumer, so the pipe can never fill), and
  a feeder started lazily by `Child.Diags()` forwards the queue onto a
  channel closed at stderr EOF. `Run` forwards each line into the model
  via `prog.Send` — a no-op once the program exits, so a late line can
  never stall. Incremental delivery is what lets a diagnostic be
  collected while the child still runs, ahead of any exit decision.
- `searchDoneMsg` — `collectSearchDiags` collects the completion
  components in outcome order: the generated process line when a failed
  child left no stderr, then the integrity/record-loss/warning tail.
  Captured stderr is **not** re-collected here when the child has
  incremental diagnostics — it already arrived as `stderrLineMsg`s — so
  the two routes stay exactly-once; a `Child` with a nil `Diags()`
  channel owes its whole captured stderr at completion.
- `fileLoadedMsg` — a failed browse load collects `loadDiag`'s
  `cannot read <EscapePath(path)>: <reason>` (a `*fs.PathError`
  contributes only its cause, so the raw path never leaks through the
  error string) whether or not the display side shows it.
- `failMsg` — the controlled failure's `vrg: <escaped>` line enters the
  collection **before** the exit decision, replacing Issue #4's
  separate direct write.

## The shutdown boundary

The boundary is *message processing*, not child output: a diagnostic is
collected once `Update` handles the message carrying it. A diagnostic
delivered and processed before `ctrl+c` or `q` in the next update is
replayed; a message still in flight at the exit decision is never
waited for and never replayed — `m.quitting` discards everything that
arrives afterwards, on the cancellation routes and the gate-held
preparation window alike. Exit never blocks on undelivered work.

## Replay after restoration

`replayDiags` is the common post-restoration stderr writer — the
generalization of Issue #4's `writeFailureDiag`: after `program.Run`
returns (terminal restored, termios back) and the `reapChild` safety
net ran, `Run` writes every collected line to `cfg.Err` exactly once,
in collection order, adding only the newline framing — collected lines
are already sanitized (`EscapeDiagnostic` for stream text,
`EscapePath` for embedded filenames). It runs on every controlled exit:
ordinary quit, `q`/`ctrl+c` cancellation (exit 130), and controlled
failure (exit 2, failure line last). A `program.Run` error that never
reached the model is appended after the collection through the same
writer — still exactly once. No persistent log is written.

The exactly-once invariant holds across both mechanism generations:
the incremental `stderrLineMsg` route and the completion-time stderr
route never both collect the same bytes, and the controlled-failure
diagnostic has no writer outside the collection.

## Filename escaping in replayed text

Diagnostics embed filenames only in `EscapePath`-escaped single-line
form — a path carrying LF or ESC bytes can never forge a diagnostic
line boundary or smuggle a control sequence into the replay. The
stderr-replay sink is a row in the
[sink-safety table](safe-presentation.md): the hostile fixture set
drives a load-failure diagnostic through the real
`fileLoadedMsg` → collection → `replayDiags` path and asserts the raw
output stays clean.

## The application-side acknowledgement

`WithDiagAck` is the test-only acknowledgement: it runs once per
diagnostic line *after* `Update` has processed it into the collection —
not when the child merely wrote the bytes. `cmd/vrg/main.go` wires it
to `VRG_TEST_DIAG_ACK=<file>` (one `diag` line appended per collected
diagnostic) — the same file-evidence mechanism family as
`VRG_TEST_REAP`. PTY tests wait on the ack file before sending an exit
key, proving a diagnostic was collected ahead of the exit decision
rather than racing it.

## Tests

See [unit-tests.md](unit-tests.md): `internal/app/replay_test.go`
covers ordered exactly-once collection, never-displayed diagnostics,
the `ctrl+c` and `q`/gate-held shutdown boundaries, the incremental vs
completion stderr routes, controlled-failure collection, and hostile
embedded filenames; `cmd/vrg/replay_test.go` drives the real binary on
a pty — acknowledgement, replay-after-restoration ordering and
exactly-once on the cancellation, gate-held, normal-quit, and
controlled-failure routes, plus the escaped hostile filename.
