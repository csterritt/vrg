# PTY test deterministic handshakes

Issue #48 (`Notes/issues/048-pty-tests-deterministic-handshakes.md`,
task `Notes/tasks/048-pty-tests-deterministic-handshakes.md`) replaces
fixed-delay synchronization in the `cmd/vrg` PTY/subprocess helpers
with deterministic, application-side acknowledgements. Every awaited
model transition and every key-processing boundary is now proven by an
event the application emits, correlated per process and per
occurrence, so a stale same-kind record can never satisfy a later
wait. Cross-references: [test-hook-topology](test-hook-topology.md)
(the `vrg_testhooks` seam mechanism these handshakes ride),
[outcome-contract](outcome-contract.md),
[search-collection-path](search-collection-path.md), and
[unit-tests](unit-tests.md); the PRD subprocess-boundary testing rule
lives in `Notes/PRD-vrg.md` *Testing Decisions*.

## The acknowledgement seam

A single new option, `app.WithUpdateAck(func(UpdateAck))` in
`internal/app/app.go`, wires a callback invoked once per
Update-processed message. `Model.Update` is a thin wrapper over the
real `update` implementation: it runs the transition, then — only when
a callback is configured — builds an `UpdateAck` from the pre- and
post-update models and fires it. The record carries the message kind
(`key`, `search-complete`, `file-load-complete`, `layout-ready`,
`search-failed`, `controlled-failure`, `diagnostic`, `popup-expiry`,
`window-size`, `other`), the keystroke label for key events, the
post-update state and open-overlay kind, and an `OverlayDismissed`
flag set when the processed message closed an open overlay. The
callback is purely observational: it observes production behaviour and
timing and never changes either.

The callback is wired only by the `vrg_testhooks` build variant.
`seams_testhooks.go` reads `VRG_TEST_UPDATE_ACK`, opens the named
file, and appends one line per record through the mutex-serialized
`updateAckLog` sink, which assigns a per-process monotonic sequence
number:

```
<seq> msg=<kind> key=<label> state=<name> overlay=<name> dismissed=<bool>
```

An unwritable path yields an inert callback rather than fabricated
records — the test's bounded wait then reports the missing
acknowledgement. The untagged `seams.go` returns no options, so the
hook is absent and inert in the production binary (proven by the
artifact probe below).

## The handshake matrix

`cmd/vrg/handshake_test.go` pins the finite contract every PTY helper
follows. Acknowledgement kinds: `UPDATE_ACK` (the
`VRG_TEST_UPDATE_ACK` event log), `COLLECT_ACK` (the Issues #11/#45
`WithOnCollect` file), `FIXTURE` (fake-rg side-effect files polled by
bounded `waitForFile`), and `OUTPUT` (a bounded poll of rendered PTY
output for an explicit marker).

| helper / test group | action | postcondition | acknowledgement | unlocks |
|---|---|---|---|---|
| `runVrgWithKeys` | start under PTY | `SearchCompleteMsg` processed | `UPDATE_ACK msg=search-complete` | first key send |
| `runVrgWithKeys` | send key K | `KeyPressMsg(K)` processed by `Update` | `UPDATE_ACK msg=key key=K` | next key send |
| `runVrgWithKeys` (esc before q) | send Esc | open overlay dismissed | `UPDATE_ACK key=esc dismissed=true` | q send |
| `runVrgKillChild` | start under PTY | fake rg running and blocked | `FIXTURE` readyFile | SIGKILL child group |
| `runVrgKillChild` | SIGKILL child group | child death processed into outcome | `UPDATE_ACK msg=search-complete` | key sends |
| `runVrgKillChild` | send key K | `KeyPressMsg(K)` processed | `UPDATE_ACK msg=key key=K` | next key send |
| painted-post-state assertions | transient state entered | view/overlay frame actually rendered | `OUTPUT` substring | dismissal/quit keys |
| `runVrgWithQuit` | start under PTY | `SearchCompleteMsg` processed | `UPDATE_ACK msg=search-complete` | first q |
| `runVrgWithQuit` | send q | key processed; `dismissed` iff overlay was open | `UPDATE_ACK msg=key key=q` | second q iff dismissed |
| `runVrgReplay` triggers | emit diagnostic | diagnostic collected into session collection | `COLLECT_ACK` line count | trigger/exit key |
| `runVrgReplay` triggers | send Esc/q/ctrl+c | `KeyPressMsg` processed; esc dismissal acked | `UPDATE_ACK msg=key key=K` | next key |
| `runVrgCancel` (blocked rg) | start under PTY | fake rg running and blocked | `FIXTURE` readyFile | cancel key (still searching) |
| `runVrgCancel` (gate held) | rg exits, gate held | model still searching (gate never releases) | `FIXTURE` handshakeFile | q (cancels) |
| `runVrgCancel` (reaps child) | rg exits | `SearchCompleteMsg` processed → browse | `UPDATE_ACK msg=search-complete` | q (quits) |
| `runshape` replay triggers | emit second diagnostic | both diagnostics collected in order | `COLLECT_ACK` line counts 1 then 2 | esc dismissal, then q |
| `TestInjectedControlledFailure` | start under PTY | fake rg running and blocked | `FIXTURE` readyFile | fail-trigger file write |
| untagged hook-manifest probe | production binary run | browse view rendered | `OUTPUT` substring | q |

Two rows deserve note. The **painted-post-state** row exists because
an `Update` acknowledgement proves the model transition, not the
render: Bubble Tea's renderer coalesces frames, so a transient view or
overlay can be acknowledged yet never painted if the dismissal key
arrives inside the same frame window. Tests asserting on rendered
transient content therefore poll the PTY output for the asserted text
before sending the next key — this is what the removed fixed sleeps
used to buy probabilistically. The **untagged probe** row covers the
one run that cannot use the seam by design: the production binary
ignores `VRG_TEST_UPDATE_ACK`, so synchronization rides on externally
observable output (the rendered browse view), and the asserted exit 0
is itself the proof that q was processed in browse.

`runVrgCancel`'s send-after-ready pattern is unchanged where the test
adds no unacknowledged transition: the cancel key needs no intra-run
acknowledgement because nothing follows it and the asserted exit code
proves processing in the awaited state. `TestNormalExitReapsChild`
previously waited only on the fixture handshake before sending q —
which raced the model transition (q during searching exits 130, not
0); it now uses `runVrgWithQuit`'s search-complete acknowledgement.

## Correlation and bounded failure

Waits correlate per process (each run writes its own acknowledgement
file) and per occurrence: the `ackLog` cursor consumes events in
sequence order, so an earlier same-kind record can never satisfy a
later wait, and repeated same-kind events yield distinct increasing
sequence numbers. Every wait is bounded — `waitEvent`/`waitForOutput`
poll on a paced 10 ms interval inside `ackTimeout` (20 s) and abort
early when the vrg process exits, failing with a message naming the
awaited condition, the log path, and the last consumed sequence.

## The static no-fixed-sleep check

`TestNoFixedSleepsInPtyHelpers` parses every `*_test.go` file in
`cmd/vrg` and requires each `time.Sleep` call to live inside one of
the named bounded condition polls (`waitForFile`, `waitForAckLines`,
`waitEvent`, `waitForOutput`). Any other sleep — a settling delay or
an inter-key delay — is a violation. The paced polls themselves are
the issue's permitted allowance: short sleeps that pace re-checks of
an explicit condition, never an assumption that a transition
completed. `watchForFile`'s paced watcher in `seams_testhooks.go` is
the same pattern in the fixture direction.

## Manifest membership and the artifact probe

`VRG_TEST_UPDATE_ACK` is a member of the explicit vrg-consumed hook
manifest in `testhooks_test.go` (see
[test-hook-topology](test-hook-topology.md)).
`TestUntaggedBinaryIgnoresHookManifest` sets it to a provocative path
alongside every other manifest name, asserts no behaviour change and
no `update-ack` side-effect file, and scans the untagged artifact for
the name — proving the acknowledgement hook exists only in the
`vrg_testhooks` build.

## Helper contract after the rewrite

`runVrgPTY` (`handshake_test.go`) is the shared scaffold: PTY start,
termios capture, concurrent output drain into a mutex-protected
buffer, a `ptyDriver` handed to each test's drive callback, bounded
30 s exit wait, and termios-restoration assertion. The driver exposes
`waitMsg`, `waitAck`, `sendKey` (writes the key, then blocks on that
key's own acknowledgement — which also prevents the input reader from
coalescing consecutive sends into a single chord), and
`waitForOutput`. `runVrgWithKeys`, `runVrgKillChild`, `runVrgWithQuit`,
`runVrgReplay`, and `runVrgCancel` are thin wrappers selecting their
matrix rows; overlay-dismissal-before-quit is enforced by checking the
esc event's `dismissed` flag before the following `q` is sent.
`runVrgWithQuit` sends the second `q` only when the first `q`'s event
reports a dismissal, making the warning-overlay path deterministic.

## Contract tests

- `TestUpdateAckSeamContract` — a search-complete event followed by a
  causally later `key=q` event with strictly increasing sequence.
- `TestUpdateAckPerOccurrenceCorrelation` — a fixture-log cursor test
  plus a real-seam run sending two `down` keys through an open error
  overlay, requiring two distinct increasing events; proves an earlier
  same-kind event cannot satisfy a later wait.
- `TestUpdateAckOverlayDismissalBeforeQuit` — Esc on a warning
  overlay produces `key=esc dismissed=true overlay=none`, strictly
  before the `q` event.
- `TestAckWaitBoundedTimeout` — a never-arriving handshake fails on
  the bounded timeout with a message naming the awaited condition.
- `TestNoFixedSleepsInPtyHelpers` — the static check above.

## Issue #50 re-verification

Issue #50 re-ran this entire suite explicitly and uncached — `go test
-count=1 ./cmd/vrg` and `CGO_ENABLED=1 go test -race -count=1 ./cmd/vrg`
over the named outcome, replay, and cancellation/boundary tests, plus a
repeated `-count=3` package run inside `scripts/verify.sh` — all green
on these handshakes. The same condition-driven contract now also owns
the canonical production smoke harness `scripts/smoke.py` (bounded
output-marker waits, EOF-driven draining, a static no-`time.sleep`
assertion). See
[post-audit-reverification](post-audit-reverification.md).
