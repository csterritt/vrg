# Cancellation, child cleanup, and terminal restoration (Issue #4)

The cancellation and cleanup contract delivered by
[Issue #4](../issues/004-cancellation-child-cleanup-terminal-restore.md),
implemented in `internal/app` (`app.go`, `search.go`) and wired by
`cmd/vrg` (`main.go`, `hooks.go`). Relevant PRD sections:
*Implementation Decisions → Outcome and exit-status contract* (rows 1–2
and the cleanup bullet), *Colours, overlays, and key precedence*
(`ctrl+c` precedence), *Module Design → App* (failure modes), and
*Testing Decisions* (subprocess boundary).

## Cancellation rules

`ctrl+c` in any state and `q` while searching cancel outstanding work
and exit 130. "Searching" spans the whole collection **and** the
post-exit preparation window: `q` after rg has exited but before the
prepared index arrives (the gate-held window proven by
`Config.PrepareGate`) is still cancellation, not a browse quit. `q` on
the interim summary remains the ordinary quit, exit 0. `Esc` during
searching is a strict no-op — it is an overlay-dismissal key only and
never exits a base state.

In the model, both cancellation keys funnel through `Model.cancelled`:
`quit` is set, `code` becomes 130, `m.cancel()` fires the context that
`exec.CommandContext` turns into a kill, and `tea.Quit` ends the
program. Once `quit` is set, `Update` discards every further message —
a search completion that lands after cancellation cannot revive the UI.
A real SIGINT (not the raw-mode `ctrl+c` keystroke) reaches the boundary
as `tea.ErrInterrupted` and maps to 130 without involving the model.

## The cleanup boundary

Every controlled exit — ordinary quit, cancellation, or failure — passes
through the same code in `runSearch` after `Program.Run` returns: the
session's `Cancel` terminates a still-running child, then `<-Reaped()`
blocks until the collector's `cmd.Wait()` has returned, so the process
never leaves an orphaned or unreaped rg behind. Killing the child closes
its pipes, so Issue #3's dual-pipe drainage ends promptly rather than
waiting for further output; cancellation likewise abandons a held
preparation gate and skips index preparation entirely.

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
cleanup (terminate, reap, restore) and then writes a sanitized
`vrg:`-prefixed diagnostic to stderr **exactly once, after terminal
restoration**, exiting 2. The single write site is `runSearch`'s
post-`Run` branch; there is no second path (a replay/collection
mechanism is Issue #11's, and the diagnostic must never flow through
both). The injectable hook is `VRG_TEST_FAIL_FIFO`: a fifo whose first
writer's close cancels the program context, surfacing as a `Run` error.

## Test seams and the PTY harness

- `VRG_TEST_REAP_FILE` — receives one line with the child's reaped wait
  status (`signal: killed`, `exit status 0`) immediately after `Wait`.
  This is the evidence side channel proving vrg's reap path ran, rather
  than inferring reaping from a missing pid. Wired to
  `Config.ReapReport`.
- `VRG_TEST_GATE_FIFO` — boundary-level `PrepareGate`: preparation is
  held until a writer opens and closes the fifo.
- `VRG_TEST_FAIL_FIFO` — the controlled-failure hook described above.

The PTY harness (`cmd/vrg/pty_test.go`, Linux-only) opens `/dev/ptmx`,
gives the child a session with the slave as controlling terminal
(stdin/stdout/stderr all on the slave), and captures everything from the
master. The fake rg signals readiness by writing its pid to a `ready`
file and either blocks forever or emits a complete stream and exits.
Assertions combine: exit status, pid gone (`kill -0` → `ESRCH`), reap
side-channel content, the display-restoration sequences in the captured
stream, and slave termios equality. Issues #9 and #11 reuse this
harness.

## Tests

See [unit-tests.md](unit-tests.md) § `internal/app` (cancellation model
tests and `TestCancelTerminatesAndReapsChild`) and § `cmd/vrg` (the
`TestPTY*` boundary group plus `TestControlledFailureDiagnosticOnStderr`).
