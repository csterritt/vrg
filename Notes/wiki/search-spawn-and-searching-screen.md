# Search spawn and the searching screen (Issue #3)

The subprocess-and-lifecycle contract delivered by
[Issue #3](../issues/003-spawn-rg-collect-results-searching-screen.md),
implemented in `internal/app` (`search.go`, `app.go`) and wired by
`cmd/vrg/main.go`. Relevant PRD sections: *Implementation Decisions →
Result index, records, and stream integrity* (stream collection) and
*Module Design → App*.

## Spawning

`app.Start` builds `exec.CommandContext` over the rg executable (a PATH
name or explicit path) with `cli.Result.ChildArgv` verbatim — the
protected `--json --no-config <flags> -- pattern root` vector from
[cli-flags-and-child-argv.md](cli-flags-and-child-argv.md) — and sets
`cmd.Dir` to the invocation working directory, which `cmd/vrg` obtains
from `os.Getwd`. That directory both anchors the child and resolves
relative result paths in the
[index](searchindex-records-and-stops.md). A start failure returns an
error synchronously, before any TUI exists; `cmd/vrg` prints it as a
`sanitized vrg: …` diagnostic on stderr and exits 2.

## Dual-pipe drainage

Both child pipes are drained concurrently for the entire child lifetime:
one goroutine `io.Copy`s stdout into a buffer, another does the same for
stderr, the collector waits for both drains to finish, then calls
`cmd.Wait` (correct order — `Wait` closes the pipes, so reads must
complete first). Neither pipe can fill and block rg: a child flooding
stderr at MiB scale while streaming valid records neither deadlocks nor
loses stdout bytes. Stderr is buffered whole and carried on the
completion message as cargo — classification and outcome effects are
Issue #9's. Because killing the child closes both pipes, cancellation
ends drainage promptly — see
[cancellation-and-cleanup.md](cancellation-and-cleanup.md) for the
Issue #4 contract (the collector now reports the reaped wait status on
the `ReapReport` side channel before abandoning a cancelled gate and
index build).

## Searching state

`Init` returns a command that blocks on the completion channel, so
collection and index preparation happen off the `Update`/`View` path.
The model stays in `phaseSearching` — rendering `Searching…` — from
spawn until the *prepared* index arrives, which spans post-exit work:
parsing, stop merging, sorting, and highlight preparation all run after
`Wait` returns while the UI still says searching. `Config.PrepareGate`
(and the `Drained` signal) is a test seam that holds preparation after
the child has exited and both pipes are drained, proving the searching
state outlasts rg itself. Resize messages are handled in any state.

## Interim summary

The done message moves the model to `phaseSummary`, rendering
`N files, M matched lines` (singular forms for 1) from the index's
`FileCount`/`LineCount`. `q` on that screen quits with exit 0. This is
the interim result screen; the real browser replaces it in later issues.

## Process boundary

`cmd/vrg` passes `tea.WithInput(os.Stdin)` so the program reads real
stdin rather than opening `/dev/tty` on a pipe, and
`tea.WithWindowSize(80, 24)` as a fallback frame size — real terminals
override it via the initial resize, but a piped output reports no size
and would otherwise render nothing. After `Run` returns, every
controlled exit terminates and reaps the child before the status is
decided — see
[cancellation-and-cleanup.md](cancellation-and-cleanup.md). The final
model's `ExitCode` is the process status (130 on cancellation,
`ErrInterrupted` also maps to 130).

## Tests

See [unit-tests.md](unit-tests.md) § `internal/app` and the `cmd/vrg`
boundary group (`TestSearchLifecycleAtBoundary`,
`TestDualPipeDrainageAtBoundary`, `TestStartFailureNoRipgrep`).
