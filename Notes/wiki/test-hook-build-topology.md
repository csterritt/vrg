# Test-hook build topology (Issue #45)

The `vrg_testhooks` build variant delivered by Issue #45, which moved
every test hook and fifo trigger loop out of the production binary.
Implemented in `cmd/vrg` (`main.go`, `seams.go`, `seams_testhooks.go`,
`testhooks_test.go`) with one new `internal/app` seam field. Relevant
PRD sections: *Implementation Decisions → Outcome and exit-status
contract* (the controlled-exit shapes the seams steer) and *Testing
Decisions* (subprocess boundary).

## The two build-constrained boundaries

`cmd/vrg` splits its test seams behind two narrow `//go:build`
boundaries, each with an unconditional common call site in `runSearch`:

- **Option/process wiring** — `wireTestHooks(&cfg) (context.Context,
  func())`. `seams.go` (`//go:build !vrg_testhooks`) is inert: it
  returns `context.Background()` and a no-op cleanup.
  `seams_testhooks.go` (`//go:build vrg_testhooks`) reads the
  option-related manifest variables and wires `app.Config`
  (`ReapReport`, `PrepareGate`, `DiagAck`, `DiagInject`) plus the
  program context's controlled-failure cancellation.
- **Program runner** — `runProgram(model, progCtx) (tea.Model, error)`,
  a wrapper around Bubble Tea program construction and `Run()`. The
  untagged implementation delegates directly to
  `tea.NewProgram(...).Run()`; the tagged implementation delegates by
  default but substitutes the returned (final model, error) tuple when
  `VRG_TEST_RUN_FINAL_MODEL`/`VRG_TEST_RUN_ERROR` are set, and gives an
  injected failure the `VRG_TEST_FAIL_DIAGNOSTIC` text. The override
  lands at the executable's actual post-`Run()` site, so Issue #46's
  return shapes reach the real branches unchanged.

## The explicit hook manifest

`hookManifest` in `cmd/vrg/testhooks_test.go` is the authoritative list
of every `VRG_TEST_*` name vrg consumes:

- `VRG_TEST_REAP` — file receiving the child's reaped wait status
  (`Config.ReapReport`).
- `VRG_TEST_GATE` — fifo holding index preparation
  (`Config.PrepareGate`).
- `VRG_TEST_COLLECT_ACK` — file receiving one line per diagnostic the
  model collects (`Config.DiagAck`).
- `VRG_TEST_FAIL_TRIGGER` / `VRG_TEST_FAIL_DIAGNOSTIC` — fifo whose
  writer-close cancels the program context (a controlled failure); the
  diagnostic variable supplies the failure's text.
- `VRG_TEST_DIAGNOSTIC_TRIGGER` / `VRG_TEST_DIAGNOSTIC_TEXT` — fifo
  whose writer-close injects one line into the session diagnostic
  collection (`Config.DiagInject`, forwarded by `injectDiags` through
  the same unbuffered channel as live child stderr, so it collects and
  acknowledges identically).
- `VRG_TEST_RUN_FINAL_MODEL` / `VRG_TEST_RUN_ERROR` — the runner
  controls: `nil`/`invalid` final-model substitutions and an
  error-text override.

The probed list derives only from this manifest — never from grepping
`VRG_TEST_*`: fixture-owned variables such as `VRG_CAPTURE_DIR` belong
to the fake-rg harness scripts, not to vrg behaviour (Issue #50 renames
them `FAKE_RG_*`). Issue #48 appends its acknowledgement hooks to this
manifest rather than adding separate machinery.

## Build and verification wiring

`TestMain` (`cmd/vrg/main_test.go`) builds the binary under test with
`go build -tags vrg_testhooks -o binPath .`, so `go test ./cmd/vrg`,
`go test -race ./cmd/vrg`, and `go test ./...` all exercise the hooked
binary automatically — the seams are identical, so prior coverage is
preserved unchanged.

`testhooks_test.go` (Linux-only) holds the boundary proof in both
directions: `TestProductionBinaryHasNoTestHooks` builds an **untagged**
binary, runs help and a full search with every manifest name armed
(trigger fifos fired, gate held, runner overrides set), asserts no
behavioural change and no side-channel files, then probes the artifact
bytes for every manifest name — the released artifact is clean, not
merely defaulted off. `TestTaggedRunnerSeamSelectsReturnShape` drives
the `VRG_TEST_RUN_*` tuple matrix against the tagged build at the real
`Run()` boundary, and `TestTaggedInjectionSeams` covers the diagnostic
and failure triggers.

Issue #46 consumed the runner seam for its return-shape matrix:
`pty_returnshape_test.go` injects the three `Run()` shapes through the
tagged binary on a real PTY lifecycle — diagnostics collected and
acknowledged before `q` ends the program — and asserts the unified
shutdown contract (terminal restoration, child reap, ordered snapshot
replay, exit 2 on every failing shape). See
[runtime-error-shutdown.md](runtime-error-shutdown.md).

## Extension rule

Issue #48 consumes this seam next: new hooks ride the same
`vrg_testhooks` mechanism and join the manifest — the production binary
gains no hooks, no env-var reads beyond legitimate production ones, and
no watcher code; the only watchers (`fifoReleased`, `diagInjected`) live
in the tagged file and are event-driven, never spinning.

See also: [cancellation-and-cleanup.md](cancellation-and-cleanup.md)
(the seams' observable contract),
[stderr-replay.md](stderr-replay.md) (the collection/acknowledgement
side channel), [unit-tests.md](unit-tests.md) (the boundary test
catalog).
