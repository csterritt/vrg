# Cancellation, child cleanup, and terminal restoration (Issue #4)

The cancellation and cleanup contract delivered by
[Issue #4](../issues/004-cancellation-child-cleanup-terminal-restore.md),
implemented in `internal/app` (`app.go`, `search.go`) and wired by
`cmd/vrg` (`main.go`, `seams.go`, `seams_testhooks.go`). Relevant PRD sections:
*Implementation Decisions → Outcome and exit-status contract* (rows 1–2
and the cleanup bullet), *Colours, overlays, and key precedence*
(`ctrl+c` precedence), *Module Design → App* (failure modes), and
*Testing Decisions* (subprocess boundary).

## Cancellation rules

`ctrl+c` in any state and `q` while searching cancel outstanding work
and exit 130. "Searching" spans the whole collection **and** the
post-exit preparation window: `q` after rg has exited but before the
prepared index arrives (the gate-held window proven by
`Config.PrepareGate`) is still cancellation, not a browse quit. The
same holds while a layout worker is held mid-rewrap since Issue #17 —
`ctrl+c` exits 130 and `q` the fixed status without waiting for
preparation (see
[logical-anchor-and-layout.md](logical-anchor-and-layout.md)). `q` on
a completed screen remains the ordinary quit with its fixed status. `Esc` during
searching is a strict no-op — it is an overlay-dismissal key only and
never exits a base state.

In the model, both cancellation keys funnel through `Model.cancelled`:
`quit` is set, `code` becomes 130, `m.cancel()` fires the context that
`exec.CommandContext` turns into a kill, and `tea.Quit` ends the
program. Once `quit` is set, `Update` discards every further message —
a search completion that lands after cancellation cannot revive the UI.
A real SIGINT (not the raw-mode `ctrl+c` keystroke) reaches the boundary
as `tea.ErrInterrupted` and maps to 130 without involving the model.

Since Issue #35 the kill reaches the child's whole **process group**:
the child is spawned with `SysProcAttr.Setpgid` and `cmd.Cancel`
SIGKILLs the group, so a scripted rg whose blocking payload runs as a
grandchild cannot survive its parent holding the drained pipes open —
the cancellation hang the final verification pass caught and repaired
(see [final-verification.md](final-verification.md)).

## The cleanup boundary

Every controlled exit — ordinary quit, cancellation, or failure — passes
through the same code in `runSearch` after `Program.Run` returns: the
session's `Cancel` terminates a still-running child, then `<-Reaped()`
blocks until the collector's `cmd.Wait()` has returned, so the process
never leaves an orphaned or unreaped rg behind. Killing the child's
process group closes its pipes even when a grandchild inherited them,
so Issue #3's dual-pipe drainage ends promptly rather than waiting for
further output; cancellation likewise abandons a held preparation gate
and skips index preparation entirely.

Terminal restoration has two halves, both asserted by the PTY harness.
The display half: every `View` sets `AltScreen`, so on exit the renderer
emits the leave-alternate-screen sequence (`\x1b[?1049l`) and shows the
cursor (`\x1b[?25h`). The input half: Bubble Tea's `initInput`/`restoreInput`
pair puts the tty into raw mode on start and restores the saved termios
on shutdown — the harness compares the slave's termios after exit
byte-for-byte with its pre-launch value.

## Controlled failures

A controlled application failure — TUI startup error, program error,
injected test failure — after the child has started runs the same
cleanup (terminate, reap, restore), exits 2, and its sanitized
`vrg:`-prefixed diagnostic reaches stderr **exactly once, after terminal
restoration**. Since Issue #11 the failure diagnostic no longer writes
directly: it joins the session diagnostic collection and the common
post-restoration replay writer emits it alongside every earlier
diagnostic in collection order — exactly-once holds across what were
previously two mechanisms. The injectable hook is
`VRG_TEST_FAIL_TRIGGER`: a fifo whose first writer's close cancels the
program context, surfacing as a `Run` error. See
[stderr-replay.md](stderr-replay.md).

Since Issue #46 **every** `Run()` return shape funnels through that one
sequence — the boundary replays its own `diagSnapshot` (fed by
`Config.OnCollect`), so a nil or wrong-type final model cannot strand
the collected diagnostics; such a shape earns the
invalid-final-model diagnostic and, like any non-interrupt `Run()`
error, exits 2. See
[runtime-error-shutdown.md](runtime-error-shutdown.md).

## Test seams and the PTY harness

Since Issue #45 every seam below lives behind the `vrg_testhooks` build
tag in `cmd/vrg/seams_testhooks.go` — the production binary contains
none of them. See
[test-hook-build-topology.md](test-hook-build-topology.md) for the
build variant, the full manifest, and the artifact-cleanliness proof.

- `VRG_TEST_REAP` — receives one line with the child's reaped wait
  status (`signal: killed`, `exit status 0`) immediately after `Wait`.
  This is the evidence side channel proving vrg's reap path ran, rather
  than inferring reaping from a missing pid. Wired to
  `Config.ReapReport`.
- `VRG_TEST_GATE` — boundary-level `PrepareGate`: preparation is
  held until a writer opens and closes the fifo.
- `VRG_TEST_FAIL_TRIGGER` — the controlled-failure hook described
  above; `VRG_TEST_FAIL_DIAGNOSTIC` supplies the failure's diagnostic
  text.
- `VRG_TEST_COLLECT_ACK` (Issue #11, renamed by Issue #45) — receives
  one acknowledgement line per diagnostic the model processes into the
  session collection; the application-side evidence replay tests wait
  on before sending an exit key. Wired to `Config.DiagAck`.
- `VRG_TEST_DIAGNOSTIC_TRIGGER` / `VRG_TEST_DIAGNOSTIC_TEXT` (Issue
  #45) — inject one line into the session collection on a fifo
  handshake. Wired to `Config.DiagInject`.
- `VRG_TEST_RUN_FINAL_MODEL` / `VRG_TEST_RUN_ERROR` (Issue #45) — the
  program-runner return-shape controls at the real `program.Run()`
  site.

The PTY harness (`cmd/vrg/pty_test.go`, Linux-only) opens `/dev/ptmx`,
gives the child a session with the slave as controlling terminal
(stdin/stdout/stderr all on the slave), and captures everything from the
master. The fake rg signals readiness by writing its pid to a `ready`
file and either blocks forever or emits a complete stream and exits.
Assertions combine: exit status, pid gone (`kill -0` → `ESRCH`), reap
side-channel content, the display-restoration sequences in the captured
stream, and slave termios equality. Issues #9 and #11 reuse this
harness — Issue #11's `pty_replay_test.go` additionally waits on the
diagnostic-acknowledgement side channel before keypresses and asserts
replayed diagnostics land after the restoration sequence.

## Tests

See [unit-tests.md](unit-tests.md) § `internal/app` (cancellation model
tests and `TestCancelTerminatesAndReapsChild`) and § `cmd/vrg` (the
`TestPTY*` boundary group plus `TestControlledFailureDiagnosticOnStderr`).
