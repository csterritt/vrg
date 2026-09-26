# Stderr replay of collected diagnostics (Issue #11)

The diagnostic-replay contract delivered by
[Issue #11](../issues/011-stderr-replay-of-collected-diagnostics.md),
implemented in `internal/app` (`app.go`, `search.go`, `overlay.go`) and
wired by `cmd/vrg` (`main.go`, `seams_testhooks.go`). Relevant PRD sections:
*Colours, overlays, and key precedence* (the last bullet: diagnostics
replayed to the terminal after exit) and *Outcome and exit-status
contract* (the cleanup bullet).

## The session collection

`Model.diags` is the **session diagnostic collection**: every diagnostic
the model has processed, in collection order, **independent of what any
screen displayed**. Display and collection are separate concerns — the
Issue #9 overlay shows the completion's assembled diagnostic list, while
the collection additionally holds diagnostics no overlay ever shows:
stderr lines that arrived while the search ran, non-current file-load
failures, and anything processed just before exit.

Three sources feed the collection:

- **Child stderr, incrementally** — `collect` in `search.go` reads the
  child's stderr line-by-line (a `bufio.Reader` ahead of the same
  buffer that still serves overlay display) and forwards each line to
  the model over an **unbuffered** channel as a `diagMsg`. The send
  pairs with the model's event wait, so lines reach `Update` in read
  order and the completion can never overtake them — the completion
  send happens only after the drain goroutines finish, which requires
  every diagnostic send to have been received.
- **Completion diagnostics** — `searchDoneMsg`'s own diagnostics join
  the collection when `Update` processes it: the generated
  process-failure line (`rg failed: exit status 3` / `signal:
  killed`), the stream-integrity failures, and the record-skip
  diagnostics. `completionDiagnostics` in `overlay.go` deliberately
  **excludes child stderr** — those lines were already collected
  incrementally and collecting them again would break exactly-once —
  while `collectDiagnostics` still includes them for the overlay.
- **Load failures** — a `loadDoneMsg` error collects `cannot read
  <path>: <err>` through `Model.CollectDiagnostic`. Since Issue #26 the
  display side splits on whether the failed path is current: the
  current file's diagnostic opens (or appends to) the overlay as well,
  while a non-current file's is collected with nothing displayed at
  all — it surfaces only by visiting the file or here, at exit. See
  [read-failures.md](read-failures.md).

## The shutdown boundary

A diagnostic counts as **collected once the model has processed the
message carrying it** — not when the child wrote it, not when it
reached a pipe. A `diagMsg` processed before the exit keypress lands in
the collection and is replayed; one still in flight at exit — behind
the unbuffered send, or inside a gate-held completion — is neither
waited for nor replayed. Cancellation flips the collector's delivery
off (`ctx.Done()` in the send's select) while pipe drainage continues
to its end, so a quitting model never leaves the collector blocked and
reaping never stalls.

`Model.Init` returns `awaitEvent`, the command selecting between the
diagnostic channel and the completion channel; each processed `diagMsg`
re-issues it, so collector events arrive in emission order. Once `quit`
is set, `Update` discards everything as before.

## Replay after restoration

`runSearch` keeps the Issue #4 ordering: `Program.Run` returns (display
and termios already restored), `Cancel` + `<-Reaped()` settle the
child, then the session collection replays to stderr — one `Fprintln`
per collected occurrence, sanitized at collection time and emitted
verbatim. No persistent log is written.

Since Issue #46 the replayed lines come from the boundary's own
**diagnostic snapshot**, not the returned model: `Config.OnCollect`
feeds `runSearch`'s `diagSnapshot` each line as `Model.collect` takes
it, so session diagnostics reach stderr even when `Run()` returns a nil
or wrong-type final model that could never carry them. `Model.diags`/
`ReplayTo` still serve the in-model collection the unit tests exercise.
The single ordered replay — session diagnostics in collection order,
then the invalid-final-model diagnostic when applicable, then the
runtime error — and the per-shape exit statuses are documented in
[runtime-error-shutdown.md](runtime-error-shutdown.md).

The same writer serves **every** controlled exit — ordinary quit (the
final model's `ExitCode`), `tea.ErrInterrupted` (130), and controlled
failure (2), which since Issue #46 includes every `Run()` runtime
error and invalid final-model shape. A failure's own diagnostic joins
the snapshot **before** replay runs, so it appears exactly once, after
the earlier diagnostics, in collection order — the separate direct
`Fprintf` the Issue #4 failure path had is gone, and exactly-once
holds across what were previously two mechanisms.

## Sanitization

Collection sanitizes at the boundary: `CollectDiagnostic` runs
`present.Diagnostic` (the Issue #6 utility) over raw text before it
enters the collection, and the completion diagnostics were sanitized
when assembled. Replay therefore emits verbatim — the sink never sees a
raw control byte. Filenames embedded via `present.Path` keep their
single-line escaped form (`\n`, `^[` for ESC), so a hostile filename in
a load failure cannot turn into a diagnostic line break or a terminal
instruction. The replay writer carries its own `stderr replay` row in
the `sinkSafetySinks` table — the fixture set drives hostile bytes in
as child stderr and embedded filenames, and the check asserts the
escaped `wantDiag`/`wantPath` forms in the replayed output.

## The acknowledgement side channel

`Config.DiagAck` is the new test-only seam: one acknowledgement line
written for every diagnostic the model processes into the collection —
wired at the process boundary as `VRG_TEST_COLLECT_ACK` (opened
append-only alongside the reap file in `wireTestHooks`; Issue #45
renamed it and moved it behind the `vrg_testhooks` build tag). It is the
**application-side** evidence the PTY replay tests wait on before
sending an exit key — proving the diagnostic was processed into the
collection, where a child-side write handshake would prove only that
bytes reached the pipe. The acknowledgement fires from `Model.collect`,
so completion diagnostics and load failures acknowledge identically to
streamed stderr lines.

## Tests

`internal/app/replay_test.go` holds the model contract: ordered
exactly-once replay across displayed and never-displayed diagnostics,
the shutdown boundary for `ctrl+c` and `q` in both incomplete states
(searching with a live child, gate-held preparation after exit), the
controlled-failure diagnostic joining the collection, and embedded-
filename escaping with no surviving control bytes.
`cmd/vrg/pty_replay_test.go` holds the process-boundary contract on the
Issue #4 PTY harness: acknowledgement before the keypress, replay
strictly after the display-restoration sequence (and restored termios),
`ctrl+c` → 130 and `q` → 130 with the collected diagnostic replayed
once, `q` on a completed warning stream → 0, the injected failure's
diagnostic counted once across both mechanisms in collection order, and
the escaped single-lined filename. `sinksafety_test.go` gained the
`stderr replay` row; `awaitDone` in `model_test.go` pumps the event
stream through `Update` until `searchDoneMsg` for tests that previously
assumed the first command result was the completion. See
[unit-tests.md](unit-tests.md).

## Files

- `internal/app/app.go` — `Model.diags`/`diagCh`/`diagAck`/`onCollect`,
  `awaitEvent`, the `diagMsg` branch,
  `collect`/`CollectDiagnostic`/`ReplayTo`.
- `internal/app/search.go` — `Config.DiagAck`/`OnCollect`, `diagMsg`,
  the unbuffered diagnostic channel, the stderr line-forwarding drain.
- `internal/app/overlay.go` — the `collectDiagnostics`/`completionDiagnostics`
  split (`processDiagnostic`, `streamDiagnostics`).
- `cmd/vrg/main.go` — `runSearch`'s post-restoration snapshot replay on
  every `Run()` return-shape branch; `diagSnapshot` replacing the
  model-carried write (Issue #46).
- `cmd/vrg/seams_testhooks.go` — `VRG_TEST_COLLECT_ACK` → `Config.DiagAck`
  (Issue #45; `vrg_testhooks`-only).

See also: [cancellation-and-cleanup.md](cancellation-and-cleanup.md)
(the cleanup boundary replay joins),
[runtime-error-shutdown.md](runtime-error-shutdown.md) (the Issue #46
snapshot and unified return-shape sequence),
[error-overlay-and-fatal-outcomes.md](error-overlay-and-fatal-outcomes.md)
(the display-side diagnostic list), and
[safe-presentation.md](safe-presentation.md) (the sanitization utility
and sink-safety table).
