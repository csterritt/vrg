# Cancellation, child cleanup, terminal restoration (Issue #4)

Delivered by
[Issue #4](../issues/004-cancellation-child-cleanup-terminal-restore.md):
the cancellation rules, the single cleanup boundary every controlled
exit routes through, terminal restoration (display **and** PTY input
modes), the injectable controlled-failure hook, and the subprocess
boundary test harness. Relevant PRD sections: *Outcome and exit-status
contract* (rows 1–2 and the cleanup bullet), *Colours, overlays, and key
precedence* (`ctrl+c` precedence), *Module Design → App*, and *Testing
Decisions → Subprocess boundary* in [`Notes/PRD-vrg.md`](../PRD-vrg.md).
Builds on the [Issue #3 search path](search-collection.md).

## Cancellation rules

- `ctrl+c` has global precedence: in any state it begins the controlled
  exit with status 130. At the model level it is a `tea.KeyPressMsg`
  whose `Keystroke()` is `"ctrl+c"`; a real `SIGINT` (only possible when
  input is not a TTY) surfaces from `program.Run` as `ErrInterrupted`,
  which `Run` also maps to 130.
- `q` while searching or while result preparation is incomplete is
  cancellation, not a browse quit. "Incomplete" includes the post-exit
  window: after rg has exited and its stream is fully collected, a
  gate-held index preparation still counts as searching, so `q` there
  exits 130 — never 0.
- `Esc` during searching is a no-op.
- Cancellation shows no further screen: the exit is immediate and the
  browse view (or any later UI) never appears.

## The cleanup boundary

Every controlled exit routes through `model.quitCmd` in
`internal/app/app.go`: terminate the child, wait for it (reap), report
the reaped status through the once-wrapped `reap` option, then return
`tea.QuitMsg` so the program shuts down and restores the terminal.
`Child` (`internal/app/rg.go`) gained `Terminate()` (a best-effort
kill, a no-op once exited) and an idempotent `Wait` — the
collection command and the cleanup path may both call it. Terminating
the child closes its pipes, so Issue #3's dual-pipe drainage ends
promptly and `cmd.Wait()` reaps the process.

Since Issue #50 termination is process-group scoped: `spawn` starts the
child as a process-group leader (`rg_unix.go`/`rg_other.go` hold the
platform split) and `Terminate` signals the whole group. The closing
smoke pass caught the regression this repairs — a descendant that
inherited the child's output pipes (a fake rg whose shell `sleep`s
without `exec`) survived a direct-PID kill and held the pipes open, so
collection's drain-before-`Wait` deadlock-prone order hung the
cancellation. Killing the group guarantees no surviving descendant
holds the pipes; see
[post-audit-verification.md](post-audit-verification.md).

`app.Run` additionally calls `reapChild` unconditionally after
`program.Run` returns. That safety net covers the exits that bypass the
model's quit command — `InterruptMsg`, a program error, a caught panic —
while `Wait`'s idempotence and the `sync.Once`-wrapped report keep a
model-driven quit from reaping or reporting twice.

After cancellation the model is marked `quitting`: `Update` discards
every subsequent message, so a late `searchDoneMsg` — for example the
one that lands when a released gate finishes preparation — cannot revive
the UI.

## Terminal restoration

Each view sets `AltScreen`, so the renderer holds the terminal in the
alternate screen with the cursor hidden; on shutdown Bubble Tea emits
the restoration sequence (leave alt screen `\x1b[?1049l`, cursor visible
`\x1b[?25h`, plus the mode resets it had enabled) and restores the
saved termios. The contract is both halves: the display-restoration
bytes **and** the PTY input modes (raw/no-echo undone). This holds on
ordinary quit, cancellation, and controlled failure alike.

## Controlled application failure and the post-restoration writer

`WithFailFunc` installs the injectable hook: when set, `Init` batches the
hook command with collection; a non-nil return becomes `failMsg`, which
begins the same cleanup exit and records `failErr`. After the program
has returned — terminal restored — `app.Run` emits the sanitized
diagnostic (`vrg: <EscapePath-escaped error text>`) and exits 2. A
`program.Run` error (TTY/startup failure, caught panic) takes the same
post-restoration write and status. As of Issue #11 the writer is
`replayDiags`, the common stderr-replay sink: the failure diagnostic
enters the session collection in `Update` before shutdown and replays
exactly once after restoration — there is no separate direct write.
Issue #46 extended the same sequence to every `program.Run()` return
shape: `Run` retains the session diagnostics in its own `diagSink`
snapshot (mirrored by `collectDiags`, independent of the final-model
assertion), replays them after restoration and reaping, adds
`invalidFinalModelDiag` when the returned model is absent or the wrong
type, and appends the runtime error exactly once — every failing shape
exits 2. See
[stderr-replay.md](stderr-replay.md).

## Test seams and the PTY harness

`VRG_TEST_*` seams wired in `cmd/vrg` — since Issue #45 only in
`seams_testhooks.go` under the `vrg_testhooks` build tag, never in the
released binary (see [test-hook-topology.md](test-hook-topology.md)):

- `VRG_TEST_REAP=<file>` — `WithReapReport` appends
  `reaped code=N err=<wait status>` once the wait/reap path ran; the
  evidence side channel proving reaping rather than inferring it from a
  missing pid.
- `VRG_TEST_FAIL_TRIGGER=<file>` — `WithFailFunc` blocks until the file
  exists, then returns an error containing a raw ESC byte (proving
  diagnostic sanitization); `VRG_TEST_FAIL_DIAGNOSTIC` overrides the
  injected text. (Pre-#45 name: `VRG_TEST_FAIL`.)
- `VRG_TEST_DIAGNOSTIC_TRIGGER=<file>` — Issue #11's `WithDiagAck`
  appends one line per diagnostic processed into the session
  collection (`VRG_TEST_DIAGNOSTIC_TEXT` overrides the recorded line),
  so a test can wait on collection rather than a child-side write; see
  [stderr-replay.md](stderr-replay.md). (Pre-#45 name:
  `VRG_TEST_DIAG_ACK`.)

`cmd/vrg/cancel_test.go` extends the Issue #3 harness with
`startVrgTermPTY`: it opens the pty pair itself (via `pty.Open`), keeps
the slave fd open, captures `term.GetState` before launch, and starts
vrg with `Setsid`/`Setctty`. After exit it re-reads the slave termios
and requires exact equality — the proof that input modes were restored.
The controllable fake rg (`blockedRG`) writes a ready file, records its
pid, and `exec sleep`s so only vrg's `Terminate` can end it; tests then
assert exit status, the pid's absence, reap evidence, the
display-restoration sequence, and termios equality. Issues #9 and #11
reuse this harness.

## Tests

See [unit-tests.md](unit-tests.md): `internal/app/cancel_test.go` covers
the model-level cancellation, discard, and failure contracts plus the
`Run` cleanup net; `internal/app/rg_unix_test.go` covers the
process-group `Terminate` directly; `cmd/vrg/cancel_test.go` covers the
PTY contracts for
`q`/`ctrl+c` cancellation, gate-held cancellation, ordinary-exit
reaping, the injected controlled failure, and Issue #50's end-to-end
process-group termination.
