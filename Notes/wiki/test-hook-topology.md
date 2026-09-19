# Test-hook topology

Issue #45 (audit Medium finding 10) moved every test-only seam out of
the released binary. Previously `cmd/vrg/main.go` read `VRG_TEST_*`
environment variables unconditionally: the production binary could
truncate/append arbitrary files, inject fake failures, and hold index
preparation indefinitely on poll loops. Now those seams compile only
into the `vrg_testhooks` build variant.

## The two build-constrained boundaries

`cmd/vrg` keeps two narrow boundaries, each a complementary file pair
sharing one unconditional call site in `main.go`'s `run`:

- **Option/process wiring** — `seams.go` (`//go:build !vrg_testhooks`)
  returns nil; `seams_testhooks.go` (`//go:build vrg_testhooks`) reads
  the option-related manifest variables and returns the corresponding
  `app.Option`s plus the `appendLine`/`waitFileGone`/`waitFileExists`
  file watchers they need. The shared call site is
  `testSeamOptions()` in the `app.Run` invocation.
- **Program-runner** — `runner.go`/`runner_testhooks.go`, distinct from
  the app options, wrap the `program.Run()` call at the executable's
  real boundary. `app.Config.RunProgram` carries the seam into
  `internal/app` (nil → `p.Run()`); the untagged implementation is
  plain delegation and the tagged implementation delegates by default
  but can then override the returned `(model, error)` tuple. See
  [source-code.md](source-code.md).

The inert untagged halves and their unconditional call sites are
deliberately permitted: they perform only direct delegation or return
no options, and the untagged artifact contains no manifest string and
no env-var reads beyond legitimate production ones. No
polling/watching remains in production code — the only `os.Stat` poll
loops live in `seams_testhooks.go`; `internal/app`'s drain loops are
blocking/event-driven (`notify` channel, `ReadString`).

## The explicit vrg-consumed hook manifest

The tagged build consumes exactly these names — the manifest the
boundary test probes (fixture-owned fake-rg variables are excluded
because vrg never reads them; Issue #50 renames them `FAKE_RG_*`):

| Variable | Seam |
|---|---|
| `VRG_TEST_REAP` | file appended with `reaped code=N err=…` once the wait/reap path ran (`WithReapReport`) |
| `VRG_TEST_GATE` | index preparation held while the file exists (`WithGate`) |
| `VRG_TEST_LOAD_GATE` | each file load held while the file exists (`WithLoadGate`) |
| `VRG_TEST_COLLECT_ACK` | one `collected` line once the child's stream is fully collected (`WithCollectAck`) |
| `VRG_TEST_FAIL_TRIGGER` | controlled failure injected once the file exists (`WithFailFunc`) |
| `VRG_TEST_FAIL_DIAGNOSTIC` | overrides the injected failure's text (default `injected test failure \x1b[7m`) |
| `VRG_TEST_DIAGNOSTIC_TRIGGER` | one line appended per diagnostic processed into the session collection (`WithDiagAck`) |
| `VRG_TEST_DIAGNOSTIC_TEXT` | overrides the recorded acknowledgement line (default `diag`) |
| `VRG_TEST_RUN_FINAL_MODEL` | `nil`/`invalid` replace the `program.Run()` final model (`invalid` returns a foreign `tea.Model` so the post-Run type assertion fails); any other value keeps the real model |
| `VRG_TEST_RUN_ERROR` | non-empty replaces the `program.Run()` error with that message |
| `VRG_TEST_EVENT_ACK` | file appended with one acknowledgement record per `Update`-processed message — `state:<name>`, `key:<name>`, `overlay:open`/`dismissed`/`appended`, `load:fail:<path>`, `layoutRequested`/`layoutInstalled`, `diag`, `collected`, `fail`, `size:WxH`, `popup` — emitted after the real dispatch returns (`WithEventAck`; Issue #48, see [pty-handshakes.md](pty-handshakes.md)) |

Renames relative to the pre-#45 seams: `VRG_TEST_FAIL` →
`VRG_TEST_FAIL_TRIGGER` (+`VRG_TEST_FAIL_DIAGNOSTIC` text override) and
`VRG_TEST_DIAG_ACK` → `VRG_TEST_DIAGNOSTIC_TRIGGER`
(+`VRG_TEST_DIAGNOSTIC_TEXT` line override). Capabilities are
identical, so every prior seam-controlled PTY/subprocess behaviour is
preserved against the tagged binary.

## Harness and boundary tests

`TestMain` (`cmd/vrg/main_test.go`) builds the binary under test with
`go build -tags vrg_testhooks -o binPath .`, so `go test ./cmd/vrg`,
`go test -race ./cmd/vrg`, and `go test ./...` all exercise the hooked
binary automatically. `testhooks_test.go` adds the boundary proofs:

- `TestProductionBinaryIgnoresHookManifest` builds an **untagged**
  binary, runs it with every manifest name set to a live trigger or
  writer path (gate files present, fail trigger present), asserts a
  normal exit-0 browse/quit with no side effects, then scans the
  artifact bytes for each manifest name. The probed list derives only
  from the explicit manifest — never from grepping `VRG_TEST_*`
  occurrences — so the test proves the released artifact is clean, not
  merely that the tag defaults off.
- `TestRunnerSeamInjectsRunReturnShapes` builds with `vrg_testhooks`
  and drives every final-model/error tuple Issue #46 needs through a
  real PTY lifecycle, asserting each reaches the executable's actual
  `program.Run()` return branches unchanged (exit status plus the
  replayed injected-error line).

Issue #46 (runtime-error replay, see
[stderr-replay.md](stderr-replay.md) and the PRD *Outcome and
exit-status contract*) has landed on this seam: `cmd/vrg/runshape_test.go`
injects the full final-model/error matrix through
`VRG_TEST_RUN_FINAL_MODEL`/`VRG_TEST_RUN_ERROR` at the real
`program.Run()` boundary and asserts the unified shutdown replay. Issue
#48 (acknowledgement handshakes, PRD *Testing Decisions*) has landed on
the same mechanism: `VRG_TEST_EVENT_ACK` joins the manifest and the
untagged-artifact probe, and every PTY key-send helper waits on the
per-occurrence acknowledgement records — see
[pty-handshakes.md](pty-handshakes.md). `scripts/verify.sh` gates 3–4
keep the tagged variant under `go build`/`go vet`.
