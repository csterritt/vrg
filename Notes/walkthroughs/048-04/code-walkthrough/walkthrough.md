# Issue #48: deterministic PTY handshakes

*2026-09-15T23:14:57Z by Showboat 0.6.1*
<!-- showboat-id: 009019d1-99a1-4284-a15a-207b6c686ecb -->

Walkthrough for Issue #48 (Notes/tasks/048-pty-tests-deterministic-handshakes.md, Notes/issues/048-pty-tests-deterministic-handshakes.md), which replaces every fixed settling and inter-key time.Sleep in the cmd/vrg PTY/subprocess helpers with deterministic, application-side acknowledgements riding the Issue #45 vrg_testhooks seam — never the production binary. The new VRG_TEST_UPDATE_ACK hook wires app.WithUpdateAck: Model.Update emits one acknowledgement record per processed message (per-process monotonic seq, message kind, key label, post-update state/overlay/dismissal flag), and every PTY helper waits on the exact handshake its preceding action caused. References: Notes/PRD-vrg.md (*Testing Decisions* — subprocess boundary: deterministic application-side handshakes, bounded condition-poll sleeps only); Notes/wiki/pty-handshake-tests.md.

Contracts demonstrated below:
- The handshake contract tests: seam contract, per-occurrence correlation (an earlier same-kind event cannot satisfy a later wait), overlay-dismissal-before-quit ordering, and bounded-timeout failure.
- The static no-fixed-sleep check and a source scan proving only bounded condition polls retain short sleeps.
- A live acknowledgement log from a real PTY run of the tagged binary.
- go test ./cmd/vrg -count=10 and CGO_ENABLED=1 go test -race -count=1 ./cmd/vrg — all PTY/subprocess tests passing deterministically.
- The repository-wide suite.

All generated artifacts (the tagged binary, fake rg, PTY driver, acknowledgement log) live in this directory.

```bash
cd /home/chris/vrg && go build ./... && go build -tags vrg_testhooks ./cmd/vrg && go vet ./... && go vet -tags vrg_testhooks ./cmd/vrg && echo GATES-OK
```

```output
GATES-OK
```

## Gates: build and vet in both build variants

The production build (untagged) and the vrg_testhooks variant both compile and vet cleanly. The acknowledgement seam exists only in the tagged variant; the untagged binary contains no hook names (proven by TestUntaggedBinaryIgnoresHookManifest, run below as part of the package suite).

## Handshake contract tests

cmd/vrg/handshake_test.go pins the finite helper/action/postcondition/acknowledgement matrix and its proofs: the seam emits a causally ordered record per Update-processed message; the ackLog cursor correlates per process and per occurrence so an earlier same-kind event can never satisfy a later wait; an Esc dismissal must report dismissed=true before a following q; a missing handshake fails on a bounded timeout naming the awaited condition; and the AST check rejects any time.Sleep outside the named bounded condition polls.

```bash
cd /home/chris/vrg && go test ./cmd/vrg -count=1 -v -run 'TestUpdateAck|TestAckWaitBoundedTimeout|TestNoFixedSleepsInPtyHelpers' 2>&1 | sed 's/([0-9.]*s)//g; s/[[:space:]][0-9.]*s$//'
```

```output
=== RUN   TestUpdateAckSeamContract
--- PASS: TestUpdateAckSeamContract 
=== RUN   TestUpdateAckPerOccurrenceCorrelation
--- PASS: TestUpdateAckPerOccurrenceCorrelation 
=== RUN   TestUpdateAckOverlayDismissalBeforeQuit
--- PASS: TestUpdateAckOverlayDismissalBeforeQuit 
=== RUN   TestAckWaitBoundedTimeout
--- PASS: TestAckWaitBoundedTimeout 
=== RUN   TestNoFixedSleepsInPtyHelpers
--- PASS: TestNoFixedSleepsInPtyHelpers 
PASS
ok  	vrg/cmd/vrg
```

## A live acknowledgement log

A real PTY run of the tagged binary, synchronized entirely on the VRG_TEST_UPDATE_ACK log: ackpty.py (in this directory) polls the log for msg=search-complete before sending q, then for that key's own msg=key key=q record — the same per-occurrence contract the Go helpers use. The log below is the application-side record the test seams produce: one line per Update-processed message with a per-process monotonic sequence number.

```bash
cd /home/chris/vrg/Notes/walkthroughs/048-04/code-walkthrough && rm -f update-ack.log && W=$PWD && (cd repo && VRG_ACK_FILE=$W/update-ack.log VRG_KEYS=q PATH="$W/fakebin:$PATH" VRG_TEST_UPDATE_ACK="$W/update-ack.log" python3 $W/ackpty.py $W/vrg-tagged hello .) && echo '--- update-ack.log ---' && cat update-ack.log
```

```output
ack: seq=4 msg=search-complete -> first key unlocked
ack: seq=7 msg=key key=q -> key processed
exit=0
--- update-ack.log ---
1 msg=other key= state=searching overlay=none dismissed=false
2 msg=window-size key= state=searching overlay=none dismissed=false
3 msg=other key= state=searching overlay=none dismissed=false
4 msg=search-complete key= state=browse overlay=none dismissed=false
5 msg=file-load-complete key= state=browse overlay=none dismissed=false
6 msg=layout-ready key= state=browse overlay=none dismissed=false
7 msg=key key=q state=browse overlay=none dismissed=false
```

The run exited 0 from browse. seq=4 (search-complete) is the application-side proof the model left the searching state — the first key send waits on it, not on elapsed time; seq=7 is the per-occurrence acknowledgement that Update processed the q. The file-load-complete and layout-ready records (seqs 5-6) show every message kind is acknowledged, not just the ones the matrix waits on.

## No fixed settling or inter-key delay remains

TestNoFixedSleepsInPtyHelpers (run above) parses every *_test.go in cmd/vrg and rejects any time.Sleep outside the named bounded condition polls. The source scan below shows the only remaining test-file sleeps are the paced re-checks inside those polls — 10 ms intervals inside waitForFile (fixture file), waitForAckLines (collect-ack line count), waitEvent (update-ack match), and waitForOutput (rendered marker). The seams_testhooks.go watcher is the same pattern in the fixture direction and is not a test file.

```bash
cd /home/chris/vrg && grep -rn 'time\.Sleep' cmd/vrg --include='*.go' && echo '--- functions containing the sleeps ---' && grep -rn 'func ' cmd/vrg/handshake_test.go cmd/vrg/replay_test.go cmd/vrg/cancel_test.go cmd/vrg/seams_testhooks.go | grep -E 'waitForFile|waitForAckLines|waitEvent|waitForOutput|watchForFile'
```

```output
cmd/vrg/handshake_test.go:176:		time.Sleep(10 * time.Millisecond)
cmd/vrg/handshake_test.go:334:		time.Sleep(10 * time.Millisecond)
cmd/vrg/handshake_test.go:597:// rendered). Every other time.Sleep in a cmd/vrg test file is a
cmd/vrg/handshake_test.go:609:// settling or inter-key time.Sleep. It scans every *_test.go file in
cmd/vrg/handshake_test.go:610:// this package for time.Sleep calls and requires each to live inside
cmd/vrg/handshake_test.go:649:						fmt.Sprintf("%s:%d: time.Sleep inside %s is a fixed delay, not a bounded condition poll",
cmd/vrg/cancel_test.go:25:		time.Sleep(10 * time.Millisecond)
cmd/vrg/replay_test.go:29:		time.Sleep(10 * time.Millisecond)
cmd/vrg/seams_testhooks.go:187:			time.Sleep(10 * time.Millisecond)
--- functions containing the sleeps ---
cmd/vrg/handshake_test.go:155:func (l *ackLog) waitEvent(desc string, match func(ackEvent) bool, timeout time.Duration) (ackEvent, error) {
cmd/vrg/handshake_test.go:308:func (d *ptyDriver) waitForOutput(t *testing.T, want string) {
cmd/vrg/replay_test.go:18:func waitForAckLines(t *testing.T, ackFile string, n int, timeout time.Duration) {
cmd/vrg/cancel_test.go:18:func waitForFile(t *testing.T, file string, timeout time.Duration) {
cmd/vrg/seams_testhooks.go:180:func watchForFile(path string, fire func()) {
```

## Repeated and race-mode verification

Every PTY/subprocess test in the package now waits on deterministic acknowledgements, so the package passes under repetition and the race detector — the runs that used to expose the hidden ordering bugs fixed sleeps could mask.

```bash
cd /home/chris/vrg && go test ./cmd/vrg -count=10 2>&1 | sed 's/([0-9.]*s)//g; s/[[:space:]][0-9.]*s$//'
```

```output
ok  	vrg/cmd/vrg
```

```bash
cd /home/chris/vrg && CGO_ENABLED=1 go test -race -count=1 ./cmd/vrg 2>&1 | sed 's/([0-9.]*s)//g; s/[[:space:]][0-9.]*s$//'
```

```output
ok  	vrg/cmd/vrg
```

## Repository-wide suite

The full module test suite passes; the acknowledgement seam is observational, so production behaviour and timing are unchanged — the untagged build/vet gates above and the in-package TestUntaggedBinaryIgnoresHookManifest artifact scan prove the hook is absent from the released binary.

```bash
cd /home/chris/vrg && go test -count=1 ./... -timeout 300s 2>&1 | sed 's/([0-9.]*s)//g; s/[[:space:]][0-9.]*s$//'
```

```output
?   	vrg/Notes/walkthroughs/023-04/code-walkthrough/demo_artifacts	[no test files]
ok  	vrg/cmd/vrg
ok  	vrg/internal/app
ok  	vrg/internal/cli
ok  	vrg/internal/docs
ok  	vrg/internal/filebuffer
ok  	vrg/internal/safepresentation
ok  	vrg/internal/searchindex
?   	vrg/internal/sinkfixtures	[no test files]
ok  	vrg/internal/theme
ok  	vrg/internal/viewport
```

## Summary

Issue #48 is implemented and verified:
- VRG_TEST_UPDATE_ACK joins the explicit vrg-consumed hook manifest (cmd/vrg/testhooks_test.go) and is probed in the untagged-artifact boundary test — the hook exists only in the vrg_testhooks build.
- app.WithUpdateAck emits one acknowledgement per Update-processed message; the seam is observational and never changes production behaviour or timing.
- The shared runVrgPTY/ptyDriver scaffold correlates waits per process and per occurrence; every key send blocks on its own event, overlay dismissal is acknowledged before a following q, and tests asserting on transient painted content wait for the text in rendered output (the Update ack proves the transition, not the frame).
- All fixed settling and inter-key sleeps are gone from the PTY helpers; the only remaining sleeps pace bounded condition polls, enforced by TestNoFixedSleepsInPtyHelpers.
- Missing handshakes fail on bounded timeouts naming the awaited condition.
- Verified: contract tests, go test ./cmd/vrg -count=10, CGO_ENABLED=1 go test -race -count=1 ./cmd/vrg, and go test ./....

Artifacts in this directory: vrg-tagged (vrg_testhooks binary), fakebin/rg (fake ripgrep), repo/test.txt (fixture), ackpty.py (acknowledgement-driven PTY driver), update-ack.log (the captured acknowledgement log).
