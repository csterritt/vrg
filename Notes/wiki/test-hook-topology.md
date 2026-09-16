# Test-hook build topology

Issue #45 (`Notes/issues/045-remove-test-hooks-from-production-binary.md`,
tasks `Notes/tasks/045-remove-test-hooks-from-production-binary.md`)
moves every test-only seam out of the released `vrg` binary behind a
`vrg_testhooks` build variant. Audit source: `Notes/critiques/final-audit-vrg.md`
Medium finding 10 — the seams previously let the production binary
truncate/append arbitrary files, inject failures and diagnostics, hold
result preparation indefinitely, and run uncancellable tight-poll
watchers. Cross-references: `Notes/PRD-vrg.md` *Outcome and exit-status
contract* (cleanup on application failures) and *Testing Decisions*
(subprocess-boundary harness); see also
[search-collection-path](search-collection-path.md),
[outcome-contract](outcome-contract.md), and
[final-verification](final-verification.md).

## The two build-constrained boundaries

`cmd/vrg` carries two narrow boundaries, each with a `//go:build
!vrg_testhooks` production file and a `//go:build vrg_testhooks`
tagged file, joined by an unconditional common call site in
`runSearch` (`cmd/vrg/main.go`):

- **Option/process wiring** — `testSeamOptions(proc) []app.Option`,
  called as `opts = append(opts, testSeamOptions(proc)...)`.
  `seams.go` (untagged) returns nil; `seams_testhooks.go` (tagged)
  reads the option-related manifest names and returns the
  corresponding `app.Option`s plus `proc.OnReap` wiring. Trigger and
  gate watchers live in the tagged file only and poll with a paced
  sleep instead of the original tight `os.Stat` spin — no file
  watching remains in production code.
- **Program-runner wrapper** — `runProgram(model, stdout)
  (tea.Model, error)`, wrapping `tea.NewProgram(...).Run()` at the
  real call site. `runner.go` (untagged) delegates directly;
  `runner_testhooks.go` (tagged) runs the real program and then
  substitutes the returned final-model/error tuple selected by the
  runner controls, so Issue #46's return-shape matrix lands at the
  executable's actual post-`Run()` branches. Issue #46 consumes it in
  `runshape_test.go`, driving every matrix shape through a real PTY
  lifecycle with diagnostics collected before the injected return.

The inert complementary implementations are permitted by the issue:
the untagged halves may only delegate directly or return no options.

## The explicit vrg-consumed hook manifest

Only this list defines production behaviour to probe — it is
duplicated in `testhooks_test.go`, never derived by grepping
`VRG_TEST_*` occurrences:

| Name | Effect (tagged build only) |
|---|---|
| `VRG_TEST_REAP` | `proc.OnReap` writes the wait status to the named file |
| `VRG_TEST_GATE` | holds index preparation until the named file appears |
| `VRG_TEST_FAIL_TRIGGER` | injects a controlled failure when the named file appears |
| `VRG_TEST_FAIL_DIAGNOSTIC` | controlled-failure text (default `vrg: controlled failure`) |
| `VRG_TEST_DIAGNOSTIC_TRIGGER` | emits a `DiagnosticMsg` when the named file appears |
| `VRG_TEST_DIAGNOSTIC_TEXT` | diagnostic text (default `vrg: test diagnostic`) |
| `VRG_TEST_COLLECT_ACK` | `WithOnCollect` appends each collected diagnostic to the file |
| `VRG_TEST_UPDATE_ACK` | `WithUpdateAck` appends one acknowledgement record per `Update`-processed message (seq, message kind, key label, post-state, overlay, dismissal flag) |
| `VRG_TEST_RUN_FINAL_MODEL` | runner control: `valid`/empty keeps the real model, `nil` returns nil, `invalid` returns a non-`app.Model` |
| `VRG_TEST_RUN_ERROR` | runner control: empty keeps the real error, `nil` forces nil, other values become the injected error text |

Fake-rg fixture variables (`FAKE_RG_ARGV_FILE`, `FAKE_RG_CWD_FILE`,
`FAKE_RG_HANDSHAKE_FILE`, `FAKE_RG_READY_FILE`, `FAKE_RG_PID_FILE`)
are excluded: they are consumed by the test suite's fake-rg shell
scripts, not by the vrg binary. Issue #50 renamed them off the
`VRG_TEST_*` prefix — see
[post-audit-reverification](post-audit-reverification.md). Issues #46 and
#48 extend these same tagged boundaries rather than adding production
hooks. Issue #46 landed first and added no new manifest names — its
return-shape tests reuse the runner controls and the existing
`VRG_TEST_COLLECT_ACK` acknowledgement. Issue #48 added
`VRG_TEST_UPDATE_ACK`, the per-update acknowledgement seam its
deterministic PTY handshakes wait on — see
[pty-handshake-tests](pty-handshake-tests.md).

## TestMain and the boundary test

`TestMain` (`cmd/vrg/main_test.go`) builds the binary under test with
`go build -tags vrg_testhooks -o binPath .`, so `go test ./cmd/vrg`,
`go test -race ./cmd/vrg`, and `go test ./...` all exercise the hooked
binary automatically; every prior seam-controlled PTY/subprocess test
passes unchanged against it.

`testhooks_test.go` holds the two-sided boundary proof:

- `TestUntaggedBinaryIgnoresHookManifest` builds an **untagged**
  binary into a temp dir, drives an ordinary fake-rg search with every
  manifest name set to a provocative value, and asserts a normal exit
  0, no hook marker text on stderr, no side-effect files (including
  the `update-ack` acknowledgement log), and — via `bytes.Contains`
  over the artifact — none of the manifest names in the binary. It
  proves the released artifact is clean, not merely that the tag
  defaults off. The untagged run wires no acknowledgement log, so its
  own synchronization polls the rendered PTY output for the browse
  view (see [pty-handshake-tests](pty-handshake-tests.md)).
- `TestTaggedRunnerSeamReturnShapes` builds the tagged binary and
  drives every `VRG_TEST_RUN_FINAL_MODEL` × `VRG_TEST_RUN_ERROR`
  combination Issue #46 needs through a real PTY quit: injected error
  text surfaces verbatim at the `Run()` error branch for valid, nil,
  and invalid model shapes; nil/invalid models with nil error reach
  the invalid-final-model branch (exit 2, not the `app.Model` path);
  unset controls delegate and exit 0.

Issue #46's `runshape_test.go` then owns the shutdown/replay contract
behind each branch: `TestRunReturnShapeUnifiedShutdown` covers the
full matrix — valid model plus `Run()` error, nil/invalid model plus
`Run()` error, and nil/invalid model plus nil error — asserting exit
2, terminal restoration, child reap, and the ordered replay (session
diagnostics in collection order, then the invalid-final-model
diagnostic when applicable, then the runtime error exactly once). See
[outcome-contract](outcome-contract.md).

See [unit-tests](unit-tests.md) and [source-code](source-code.md).
