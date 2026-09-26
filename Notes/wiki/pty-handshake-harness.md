# PTY deterministic handshakes (Issue #48)

Issue #48 (`Notes/tasks/048-pty-tests-deterministic-handshakes.md`)
replaces every timing-based synchronization point in the `cmd/vrg`
PTY/subprocess harness with causally correlated, application-side
acknowledgements. It rides the Issue #45 `vrg_testhooks` seam mechanism
— see [test-hook-build-topology.md](test-hook-build-topology.md) — and
implements the *Testing Decisions → Subprocess boundary* rule of
`Notes/PRD-vrg.md`: explicit handshakes, no sleeps as progress proxies.

## The event-acknowledgement seam

The tagged binary reads `VRG_TEST_EVENT_ACK` (joined to the explicit
`hookManifest`, tenth name) and wires `app.Config.EventAck` to the
named file. The model's `emit(kind, detail)` appends one record per
awaited transition:

```
<seq> <kind> [<detail>]
```

`seq` is a per-session monotonic counter (`Model.eventSeq`) minted on
every emit — so a helper waiting for the n-th matching record can never
be satisfied by an earlier same-kind record. Every run writes its own
file, making records per-process as well as per-occurrence. Emission
happens inside `Update`/`Start` on the event loop, so records observe
the real production transitions and ordering; `EventAck` is nil in
production binaries and plain unit-test models, so the seam never
changes timing or behaviour. The emitted kinds:

- `phase <searching|browse|noresults|fatal>` — session start and the
  post-completion phase entry.
- `key <name>` — every consumed key press, emitted before the key's
  transition records.
- `collected` — one per diagnostic processed into the session
  collection (alongside the pre-existing `VRG_TEST_COLLECT_ACK` line).
- `overlay <open|closed>` — the diagnostics overlay's transitions;
  `closed` lands before a following `q` can be sent, closing the
  overlay-dismissal race.
- `help <open|closed>` — the help overlay's transitions.
- `load <ok|fail> <presented-path>` — a file-load completion processed.
- `quitting <exit-code>` — the model committed to quit with its settled
  status (0/1/2/130), including the fatal-overlay quit path.

## The handshake matrix

`cmd/vrg/handshake_test.go` declares the finite
`handshakeMatrix` — every covered helper's trigger, exact
application-side postcondition, acknowledgement channel, and the next
action it unlocks. Besides the event log the channels are the
pre-existing handshakes: the child's `ready` file, the `VRG_TEST_REAP`
side channel, the `VRG_TEST_COLLECT_ACK` collect file, the fifo
open/close pairing, process exit (status + restored termios + full
stream), and the stdout-marker condition poll — which may never be the
sole gate on a key send, except for `runStepsBin`, kept only for the
untagged production-artifact boundary test that cannot carry the seam.

## Harness helpers

`cmd/vrg/acksteps_test.go` (untagged, shared with the portable boundary
tests) holds the acknowledgement readers (`readAcks`, `pollAck`,
`awaitAck`) and `runAckSteps`, the stepper every scripted key run now
uses: each `ackStep` writes its keys only once the n-th matching record
has landed — plus an optional stdout marker for content assertions —
under a context deadline that reports the pending step on timeout.
`pollAck` failures name the wanted record and list every observed one;
`TestMissingAcknowledgementFailsBounded` pins the bounded, informative
timeout. `holdFifo` holds a gate fifo with an `O_RDWR` open released by
the caller's close — no fixed hold duration.

`TestHarnessHasNoFixedDelays` parses every `cmd/vrg` test function and
forbids `time.Sleep`/`time.After` outside a small allowlist of bounded
condition polls (`awaitFileContent`, `pollAck`, `runAckSteps`, the PTY
`waitFor`) that must loop on a deadline while checking an explicit
condition each iteration. `waitForGrowth` — frame growth as a dismissal
proxy — is gone: overlay tests wait on the `overlay closed` record and
content assertions on `waitFor` markers.

## Correlation and regression proofs

`TestHandshakeMatrixRowsAcknowledged` drives five sessions (gated
browse with help and repeated `w`, cancellation, load-failure overlay,
fatal outcome, the stepper itself) and requires every event-log row of
the matrix to be observed plus every non-event channel exercised, with
strictly increasing sequences asserted per session.
`TestAckRecordsCorrelatePerOccurrence` proves a second-w wait cannot be
satisfied by the first w's record, and
`TestOverlayDismissalAcknowledgedBeforeQuit` pins the
open-before-closed causal order.

`TestEventAckHookInManifest` keeps `VRG_TEST_EVENT_ACK` in
`hookManifest`, and `TestProductionBinaryHasNoTestHooks` probes the new
name along with the rest — the untagged binary neither writes the
events file nor carries the name in its artifact bytes.

See also: [stderr-replay.md](stderr-replay.md) (the collection
acknowledgement the `collected` records complement),
[runtime-error-shutdown.md](runtime-error-shutdown.md) (the runner seam
the return-shape matrix rides), [unit-tests.md](unit-tests.md) (the
full boundary test catalog).
