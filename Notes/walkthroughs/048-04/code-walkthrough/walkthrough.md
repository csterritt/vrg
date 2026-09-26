# Issue #48: Deterministic handshakes for the PTY test harness

*2026-09-25T14:42:19Z by Showboat 0.6.1*
<!-- showboat-id: 0e424324-9622-4a2c-8298-ce3c34631a29 -->

Issue #48 (`Notes/tasks/048-pty-tests-deterministic-handshakes.md`) replaces every timing-based synchronization point in the `cmd/vrg` PTY/subprocess harness with causally correlated, application-side acknowledgements, riding the Issue #45 `vrg_testhooks` seam mechanism. The tagged binary writes one `<seq> <kind> [<detail>]` record per awaited model transition to the file `VRG_TEST_EVENT_ACK` names, with a per-session monotonic sequence so an earlier same-kind record can never satisfy a later wait. Every covered helper waits on the handshake for the specific transition it depends on, with bounded timeout failures — no fixed settling or inter-key sleeps anywhere. PRD cross-reference: *Testing Decisions* in `Notes/PRD-vrg.md` ("explicit readiness/completion handshakes … Avoid sleeps"). First the repository gates, then the handshake contract tests, the static no-fixed-delay check, the deterministic repetition and race runs, and a manual scenario showing a real session's acknowledgement log.

```bash
cd /home/chris/vrg && go vet ./... && go build ./... && go vet -tags vrg_testhooks ./cmd/vrg && tmp=$(mktemp -d) && go build -tags vrg_testhooks -o $tmp/vrg-tagged ./cmd/vrg && go build -o $tmp/vrg-prod ./cmd/vrg && manifest="VRG_TEST_REAP VRG_TEST_GATE VRG_TEST_FAIL_TRIGGER VRG_TEST_FAIL_DIAGNOSTIC VRG_TEST_DIAGNOSTIC_TRIGGER VRG_TEST_DIAGNOSTIC_TEXT VRG_TEST_COLLECT_ACK VRG_TEST_EVENT_ACK VRG_TEST_RUN_FINAL_MODEL VRG_TEST_RUN_ERROR" && for b in tagged prod; do n=0; for name in $manifest; do strings $tmp/vrg-$b | grep -qF "$name" && n=$((n+1)); done; echo "$b binary: $n/10 manifest names embedded"; done && rm -rf $tmp && echo GATES-OK
```

```output
tagged binary: 10/10 manifest names embedded
prod binary: 0/10 manifest names embedded
GATES-OK
```

The handshake contract lives in `cmd/vrg/handshake_test.go`: the finite `handshakeMatrix` maps every covered helper's actions to the application-side postcondition and the acknowledgement that proves it; `TestHandshakeMatrixRowsAcknowledged` drives five real sessions (gated browse with help and a repeated `w`, cancellation, a load-failure overlay, a fatal outcome, and a `runAckSteps` pipe run) and requires every event-log row observed plus every non-event channel exercised, with per-session strictly increasing sequences. `TestAckRecordsCorrelatePerOccurrence` proves a second-`w` wait cannot be satisfied by the first `w`'s record; `TestOverlayDismissalAcknowledgedBeforeQuit` pins dismissal-before-quit; `TestMissingAcknowledgementFailsBounded` pins the informative bounded timeout; `TestEventAckHookInManifest` keeps `VRG_TEST_EVENT_ACK` inside Issue #45's explicit `hookManifest`, which `TestProductionBinaryHasNoTestHooks` probes against the untagged artifact.

```bash
cd /home/chris/vrg && go test ./cmd/vrg -count=1 -v -run "TestHandshakeMatrixWellFormed|TestEventAckHookInManifest|TestHandshakeMatrixRowsAcknowledged|TestAckRecordsCorrelatePerOccurrence|TestOverlayDismissalAcknowledgedBeforeQuit|TestMissingAcknowledgementFailsBounded" 2>&1 | grep -E "^(=== RUN|--- (PASS|FAIL)|PASS|FAIL|ok)"
```

```output
=== RUN   TestHandshakeMatrixWellFormed
--- PASS: TestHandshakeMatrixWellFormed (0.00s)
=== RUN   TestEventAckHookInManifest
--- PASS: TestEventAckHookInManifest (0.00s)
=== RUN   TestHandshakeMatrixRowsAcknowledged
=== RUN   TestHandshakeMatrixRowsAcknowledged/browse_session
=== RUN   TestHandshakeMatrixRowsAcknowledged/cancel_session
=== RUN   TestHandshakeMatrixRowsAcknowledged/overlay_session
=== RUN   TestHandshakeMatrixRowsAcknowledged/fatal_session
=== RUN   TestHandshakeMatrixRowsAcknowledged/stepped_session
--- PASS: TestHandshakeMatrixRowsAcknowledged (0.27s)
=== RUN   TestAckRecordsCorrelatePerOccurrence
--- PASS: TestAckRecordsCorrelatePerOccurrence (0.45s)
=== RUN   TestOverlayDismissalAcknowledgedBeforeQuit
--- PASS: TestOverlayDismissalAcknowledgedBeforeQuit (0.05s)
=== RUN   TestMissingAcknowledgementFailsBounded
--- PASS: TestMissingAcknowledgementFailsBounded (0.54s)
PASS
ok  	vrg/cmd/vrg	1.506s
```

No covered helper synchronizes on a fixed settling or inter-key delay: `TestHarnessHasNoFixedDelays` parses every `cmd/vrg` test function and forbids `time.Sleep`/`time.After` outside a small allowlist of bounded condition polls that must loop on a deadline checking an explicit condition each iteration. `waitForGrowth` (frame growth as a dismissal proxy) and `holdFifo`'s fixed hold duration are gone — the fifo hold is now an `O_RDWR` pair the caller releases. The remaining `time.Sleep`/`time.After` call sites are exactly the four allowlisted bounded polls:

```bash
cd /home/chris/vrg && go test ./cmd/vrg -count=1 -v -run TestHarnessHasNoFixedDelays 2>&1 | grep -E "^(--- (PASS|FAIL)|ok|FAIL)" && echo "---- every time.Sleep/time.After call site in cmd/vrg test helpers ----" && grep -rn "time\.Sleep\|time\.After" cmd/vrg/*_test.go | grep -v handshake_test.go
```

```output
--- PASS: TestHarnessHasNoFixedDelays (0.00s)
ok  	vrg/cmd/vrg	0.199s
---- every time.Sleep/time.After call site in cmd/vrg test helpers ----
cmd/vrg/acksteps_test.go:122:		time.Sleep(5 * time.Millisecond)
cmd/vrg/acksteps_test.go:252:				time.Sleep(5 * time.Millisecond)
cmd/vrg/pty_test.go:153:		case <-time.After(5 * time.Millisecond):
cmd/vrg/pty_test.go:223:		time.Sleep(5 * time.Millisecond)
```

Determinism under repetition: the whole `cmd/vrg` suite — every PTY lifecycle, the replay matrix, the return-shape matrix, the seam-boundary probes, and the handshake contract — repeated ten times. Every send and assumed transition waits on its acknowledgement, so scheduling jitter cannot reorder the run.

```bash
cd /home/chris/vrg && go test ./cmd/vrg -count=10
```

```output
ok  	vrg/cmd/vrg	104.433s
```

The race gate: acknowledgement emission rides the model's event loop (`emit` runs inside `Update`/`Start` — single-goroutine, ordered), while the harness side only ever re-reads the event file under its mutexed buffer append — nothing races under `-race`.

```bash
cd /home/chris/vrg && CGO_ENABLED=1 go test -race -count=1 ./cmd/vrg
```

```output
ok  	vrg/cmd/vrg	11.984s
```

```bash
cd /home/chris/vrg && go test ./... -count=1 | sed -E "s/[[:space:]][0-9.]+s$//"
```

```output
ok  	vrg/cmd/vrg
ok  	vrg/internal/app
ok  	vrg/internal/cli
ok  	vrg/internal/filebuffer
ok  	vrg/internal/present
ok  	vrg/internal/searchindex
ok  	vrg/internal/theme
ok  	vrg/internal/viewport
```

Manual scenario (`demo-handshake.sh`, entirely inside a disposable directory): a fake rg emits one complete match stream and exits; the `vrg_testhooks` binary runs on pipes with `VRG_TEST_EVENT_ACK` armed and a fifo for stdin so keys go on demand. The driver sends `h` only after `phase browse` + `load ok` landed, `q` only after `help open`, and the quitting `q` only after `help closed` — the same gated ordering the test harness uses. The captured event file shows the full per-session sequence; artifacts land in `manual-handshake/` (events.log, out.log, err.log).

```bash
cd /home/chris/vrg/Notes/walkthroughs/048-04/code-walkthrough && bash demo-handshake.sh
```

```output
-- ack: 1 x phase searching
-- ack: 1 x phase browse
-- ack: 1 x load ok
-- ack: 1 x key h
-- ack: 1 x help open
-- ack: 1 x help closed
-- ack: 1 x quitting 0
-- exit status: 0
== acknowledgement records (per-session monotonic sequence) ==
1 phase searching
2 phase browse
3 load ok f.txt
4 key h
5 help open
6 key q
7 help closed
8 key q
9 quitting 0
== stderr (empty: no diagnostics collected) ==
```

```bash
cd /home/chris/vrg && go test ./cmd/vrg -count=1 -run "TestHandshake|TestEventAck|TestAck|TestOverlayDismissal|TestMissing|TestHarness" > /dev/null 2>&1; echo "handshake contract tests exit status: $?"; go test ./cmd/vrg -count=1 -run "TestProductionBinaryHasNoTestHooks" > /dev/null 2>&1; echo "production-artifact boundary test exit status: $?"
```

```output
handshake contract tests exit status: 0
production-artifact boundary test exit status: 0
```

All gates green. Issue #48's contract holds end to end: the `VRG_TEST_EVENT_ACK` seam emits a per-session monotonic `<seq> <kind> [<detail>]` record for every awaited transition (`phase`, `key`, `collected`, `overlay`, `help`, `load`, `quitting`); every covered helper — `startVrgPTY`/`startPiped`, `awaitReadyPID`/`awaitFileContent`, `send`, `waitFor`, `waitExit`/`finish`, `fireFifo`/`holdFifo`, `runVrgTUI`/`runAckSteps`, `runStepsBin` — is governed by a matrix row naming its acknowledgement channel; and `TestHarnessHasNoFixedDelays` proves no fixed settling or inter-key `time.Sleep` survives anywhere in the harness. The seam is `vrg_testhooks`-only, joined to the explicit `hookManifest`, and absent from the production binary — the handshakes observe production timing rather than alter it. Sources: `Notes/tasks/048-pty-tests-deterministic-handshakes.md`, `Notes/PRD-vrg.md` (*Testing Decisions*), `internal/app/app.go`/`search.go`, `cmd/vrg/seams_testhooks.go`, `cmd/vrg/acksteps_test.go`, `cmd/vrg/handshake_test.go`, `cmd/vrg/pty*_test.go`, `cmd/vrg/main_test.go`, `cmd/vrg/testhooks_test.go`. Wiki: `Notes/wiki/pty-handshake-harness.md`.
