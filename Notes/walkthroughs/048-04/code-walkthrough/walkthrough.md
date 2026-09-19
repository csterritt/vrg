# Issue #48: deterministic PTY handshakes via VRG_TEST_EVENT_ACK

*2026-09-18T11:38:52Z by Showboat 0.6.1*
<!-- showboat-id: 1eba658b-f85a-4f8a-8603-5ef6b5ccff79 -->

Issue #48 replaces every implicit ordering assumption in the PTY/subprocess tests with explicit, application-side acknowledgements — the per-Update-message seam Notes/PRD-vrg.md's *Testing Decisions* section prescribes. The vrg_testhooks binary appends one '<seq> <event>' record per processed message and per awaited transition (state entered, key processed, overlay opened/appended/dismissed, load settled, layout installed, diagnostic collected, controlled failure) to the file VRG_TEST_EVENT_ACK names; helpers baseline the per-event count before acting and wait for it to grow, so an earlier same-kind record can never satisfy a later wait and a missing acknowledgement fails on a bounded timeout with the event, occurrence, log, and output — never a hang. The seam observes production timing only (records emit after the real Update dispatch returns) and is absent from the untagged production binary, where TestProductionBinaryIgnoresHookManifest proves the hook name is neither honored nor embedded. See Notes/issues/048-pty-tests-deterministic-handshakes.md and Notes/tasks/048-pty-tests-deterministic-handshakes.md. This walkthrough runs the handshake contract tests and the AST-level no-fixed-sleep static check, drives the seam on a real PTY outside the test harness (ack_probe.py, an artifact of this directory alongside the tagged binary), then runs the issue's manual scenario — go test ./cmd/vrg -count=10 and a -race run — proving the PTY/subprocess suite is deterministic with no fixed settling or inter-key delay remaining in any cmd/vrg helper.

## The handshake contract tests

cmd/vrg/handshake_test.go owns the contract: the finite helper/action/postcondition/acknowledgement matrix (its header comment), the correlated-wait helpers (ackEnv/ackPathFromEnv/ackCount/awaitAck/waitAck/keyEvent), and five proofs — lifecycle causal order with per-process monotonic sequence numbers, same-kind occurrence correlation, dismissal-acknowledged-before-quit, bounded-timeout failure for a never-arriving event, and the static no-fixed-sleep check.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestEventAckSeamRecordsLifecycle|TestAckCorrelationSameKindOccurrences|TestOverlayDismissalAcknowledgedBeforeQuit|TestAckWaitFailsOnBoundedTimeout|TestNoFixedSleepsInPTYHelpers' ./cmd/vrg 2>&1 | grep -E '^(=== RUN| *--- (PASS|FAIL)|ok|FAIL)' | sed -E 's/\([0-9.]+s\)//g; s/^ +//'; echo "exit=${PIPESTATUS[0]}"
```

```output
=== RUN   TestEventAckSeamRecordsLifecycle
--- PASS: TestEventAckSeamRecordsLifecycle 
=== RUN   TestAckCorrelationSameKindOccurrences
--- PASS: TestAckCorrelationSameKindOccurrences 
=== RUN   TestOverlayDismissalAcknowledgedBeforeQuit
--- PASS: TestOverlayDismissalAcknowledgedBeforeQuit 
=== RUN   TestAckWaitFailsOnBoundedTimeout
--- PASS: TestAckWaitFailsOnBoundedTimeout 
=== RUN   TestNoFixedSleepsInPTYHelpers
--- PASS: TestNoFixedSleepsInPTYHelpers 
ok  	vrg/cmd/vrg	0.705s
exit=0
```

## The seam boundary and the static check, demonstrated

The hook is a member of Issue #45's explicit manifest: the untagged production artifact ignores a live VRG_TEST_EVENT_ACK writer path and does not embed the name. And the static check's allowlist is real — introducing a bare settling sleep into a PTY helper is a build break (shown by the negative probe at the end of this section).

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestProductionBinaryIgnoresHookManifest' ./cmd/vrg 2>&1 | tail -4 | sed -E 's/\([0-9.]+s\)//g'; echo "exit=${PIPESTATUS[0]}"
```

```output
=== RUN   TestProductionBinaryIgnoresHookManifest
--- PASS: TestProductionBinaryIgnoresHookManifest 
PASS
ok  	vrg/cmd/vrg	0.413s
exit=0
```

```bash
cd /home/chris/vrg && printf 'package main\n\nimport "time"\n\nfunc zzProbeSettle() { time.Sleep(50 * time.Millisecond) }\n' > cmd/vrg/zz_probe_sleep_test.go && echo '--- with a settling sleep injected into a helper ---' && go test -count=1 -run TestNoFixedSleepsInPTYHelpers ./cmd/vrg 2>&1 | grep -E 'FAIL|time.Sleep' ; rm cmd/vrg/zz_probe_sleep_test.go && echo '--- probe removed; check passes again ---' && go test -count=1 -run TestNoFixedSleepsInPTYHelpers ./cmd/vrg 2>&1 | tail -2
```

```output
--- with a settling sleep injected into a helper ---
--- FAIL: TestNoFixedSleepsInPTYHelpers (0.00s)
    handshake_test.go:383: zz_probe_sleep_test.go:5: fixed time.Sleep in zzProbeSettle — helpers wait on acknowledgements or bounded condition polls
FAIL
FAIL	vrg/cmd/vrg	0.187s
FAIL
--- probe removed; check passes again ---
ok  	vrg/cmd/vrg	0.192s
```

## Manual scenario: the seam on a real PTY, outside go test

ack_probe.py (an artifact of this directory) runs the tagged binary on a real 120x30 PTY against real ripgrep in a fixture directory, and drives the same per-occurrence contract awaitAck implements: baseline the event count, act, wait for the count to grow. The printed log is the deterministic handshake itself — every record the harness waits on, sequence-numbered.

```bash
cd /home/chris/vrg/Notes/walkthroughs/048-04/code-walkthrough && go build -tags vrg_testhooks -o vrg-testhooks /home/chris/vrg/cmd/vrg && rm -rf /tmp/vrg-048-probe && python3 ack_probe.py ./vrg-testhooks /tmp/vrg-048-probe; rc=$?; rm -rf /tmp/vrg-048-probe; echo "exit=$rc"
```

```output
ack: state:searching
ack: state:browse
ack: load:ok
ack: key:q occurrence 1
exit=0
--- VRG_TEST_EVENT_ACK log ---
1 state:searching
2 other
3 size
4 other
5 collected
6 state:browse
7 load:ok
8 fileloaded
9 layout
10 layoutready
11 key:q
exit=0
```

## Manual scenario: the repeated suite and the race run

The issue's manual checks: ten consecutive runs of the full cmd/vrg PTY/subprocess package, then one run under the race detector. -timeout 30m accompanies -count=10 because ten ~90s suite runs exceed go test's default 10m cap — every wait in the harness is already bounded, so the flag only widens the harness-level ceiling. Determinism means identical passes every time: each wait is on an application-side acknowledgement or a bounded explicit condition, so load changes timing, never outcomes.

```bash
cd /home/chris/vrg && out=$(go test -timeout 30m -count=10 ./cmd/vrg 2>&1); rc=$?; echo "$out" | tail -6; echo "exit=$rc"
```

```output
ok  	vrg/cmd/vrg	901.581s
exit=0
```

```bash
cd /home/chris/vrg && out=$(CGO_ENABLED=1 go test -race -count=1 ./cmd/vrg 2>&1); rc=$?; echo "$out" | tail -6; echo "exit=$rc"
```

```output
ok  	vrg/cmd/vrg	91.697s
exit=0
```
