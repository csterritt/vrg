# Issue #45: Test hooks live only in the vrg_testhooks build variant

*2026-09-25T12:51:40Z by Showboat 0.6.1*
<!-- showboat-id: 262e7475-5eb1-4fd4-a7e0-b0d045ba0a86 -->

Walkthrough for [Issue #45](../../../tasks/045-remove-test-hooks-from-production-binary.md) per `Notes/PRD-vrg.md` (*Outcome and exit-status contract* — the controlled-exit shapes the seams steer; *Testing Decisions* — the subprocess boundary). Every `VRG_TEST_*` hook moved behind the `vrg_testhooks` build tag: `cmd/vrg/seams.go` (`!vrg_testhooks`) holds the inert halves and `cmd/vrg/seams_testhooks.go` (`vrg_testhooks`) holds every env-var read, fifo trigger, and watcher. The seams split at two boundaries — `wireTestHooks` for option/process wiring and `runProgram` wrapping `tea.NewProgram(...).Run()` — both invoked at unconditional call sites in `runSearch`. `TestMain` now builds the binary under test with `-tags vrg_testhooks`. Generated artifacts live in this directory: the `vrg` (production) and `vrg-hooks` (tagged) binaries, the `demo-test-hooks.sh` harness, and its `manual-test-hooks/` captures. Test durations are stripped so the document verifies cleanly.

```bash
cd /home/chris/vrg && go vet ./... && go build ./... && go test -count=1 ./... | sed 's/[[:space:]][0-9.]*s$//' && echo GATES-OK
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
GATES-OK
```

## The boundary tests

`cmd/vrg/testhooks_test.go` proves the topology in both directions. `TestProductionBinaryHasNoTestHooks` builds an **untagged** binary, runs help and a full search with every manifest name armed — trigger fifos fired, gate held, runner overrides set — asserts no behavioural change and no side-channel files, then probes the artifact bytes for each of the nine manifest names: `VRG_TEST_REAP`, `VRG_TEST_GATE`, `VRG_TEST_COLLECT_ACK`, `VRG_TEST_FAIL_TRIGGER`/`VRG_TEST_FAIL_DIAGNOSTIC`, `VRG_TEST_DIAGNOSTIC_TRIGGER`/`VRG_TEST_DIAGNOSTIC_TEXT`, `VRG_TEST_RUN_FINAL_MODEL`/`VRG_TEST_RUN_ERROR`. The list comes from the explicit manifest only — fixture variables like `VRG_CAPTURE_DIR` are fake-rg behaviour, not vrg seams. `TestTaggedRunnerSeamSelectsReturnShape` drives the `VRG_TEST_RUN_*` tuple matrix at the real `program.Run()` boundary, and `TestTaggedInjectionSeams` covers the diagnostic/failure triggers.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestProductionBinaryHasNoTestHooks|TestTaggedRunnerSeamSelectsReturnShape|TestTaggedInjectionSeams' ./cmd/vrg 2>&1 | grep -E 'PASS|FAIL|ok ' | grep -v '=== RUN' | sed -E 's/[[:space:]][(]?[0-9.]+s\)?$//'
```

```output
--- PASS: TestProductionBinaryHasNoTestHooks
--- PASS: TestTaggedRunnerSeamSelectsReturnShape
    --- PASS: TestTaggedRunnerSeamSelectsReturnShape/baseline_real_tuple
    --- PASS: TestTaggedRunnerSeamSelectsReturnShape/valid_model_with_error
    --- PASS: TestTaggedRunnerSeamSelectsReturnShape/explicit_valid_model_with_error
    --- PASS: TestTaggedRunnerSeamSelectsReturnShape/nil_model_with_error
    --- PASS: TestTaggedRunnerSeamSelectsReturnShape/invalid_model_with_error
    --- PASS: TestTaggedRunnerSeamSelectsReturnShape/nil_model_without_error
--- PASS: TestTaggedInjectionSeams
    --- PASS: TestTaggedInjectionSeams/diagnostic_trigger
    --- PASS: TestTaggedInjectionSeams/failure_trigger
PASS
ok  	vrg/cmd/vrg
```

The race build confirms no watcher spins without cancellation — the fifo watchers live only in the tagged file and are event-driven (`fifoReleased` blocks in `os.Open`/`io.Copy`; `diagInjected` abandons its send under cancellation).

```bash
cd /home/chris/vrg && CGO_ENABLED=1 go test -race -count=1 ./cmd/vrg | sed -E 's/[[:space:]][0-9.]+s$//'
```

```output
ok  	vrg/cmd/vrg
```

## Manual scenario: the released artifact is clean

`demo-test-hooks.sh` builds the production binary (`go build ./cmd/vrg`), probes it with `strings`, then runs a real search on a tmux PTY with every manifest name armed: `VRG_TEST_REAP`/`VRG_TEST_COLLECT_ACK` pointed at side files, `VRG_TEST_GATE` at a fifo that is never written (a hooked binary would hang searching), both trigger fifos fired, `VRG_TEST_FAIL_DIAGNOSTIC`/`VRG_TEST_DIAGNOSTIC_TEXT` supplied, and `VRG_TEST_RUN_FINAL_MODEL=nil`/`VRG_TEST_RUN_ERROR` set. The browse view appears, `q` exits 0, stderr stays empty, and no side files exist — none of the names alter behaviour and none are in the artifact. It then builds the tagged variant and drives `VRG_TEST_RUN_FINAL_MODEL`/`VRG_TEST_RUN_ERROR` through a lifecycle where `warn one` was collected before the injected return (`VRG_TEST_COLLECT_ACK` is the handshake): baseline exits 130 with the diagnostic replayed; the error override exits 2 with `vrg: boom`; the nil-model override drops the collected diagnostic — proving the injected tuple reached the actual post-`Run()` branches, not a substitute path.

```bash
bash demo-test-hooks.sh
```

```output
== production artifact: strings probe ==
absent: VRG_TEST_REAP
absent: VRG_TEST_GATE
absent: VRG_TEST_COLLECT_ACK
absent: VRG_TEST_FAIL_TRIGGER
absent: VRG_TEST_FAIL_DIAGNOSTIC
absent: VRG_TEST_DIAGNOSTIC_TRIGGER
absent: VRG_TEST_DIAGNOSTIC_TEXT
absent: VRG_TEST_RUN_FINAL_MODEL
absent: VRG_TEST_RUN_ERROR
total VRG_TEST_* strings in the production binary: 0

== production run: every manifest name armed ==
browse row: f.txt  ── f.txt ────────────────────────────────────────────────────────────────────────────────────
exit status: 0
stderr bytes: 0
no reap file — hook absent
no ack file — hook absent

== tagged artifact: manifest present ==
present: VRG_TEST_REAP
present: VRG_TEST_GATE
present: VRG_TEST_COLLECT_ACK
present: VRG_TEST_FAIL_TRIGGER
present: VRG_TEST_FAIL_DIAGNOSTIC
present: VRG_TEST_DIAGNOSTIC_TRIGGER
present: VRG_TEST_DIAGNOSTIC_TEXT
present: VRG_TEST_RUN_FINAL_MODEL
present: VRG_TEST_RUN_ERROR

== tagged run: VRG_TEST_RUN_* reach the real Run() result branch ==
-- baseline: exit 130, stderr: warn one|
-- run-error: exit 2, stderr: warn one|vrg: boom|
-- nil-model-error: exit 2, stderr: vrg: boom|
-- nil-model: exit 0, stderr: 
```

## Result

Every Issue #45 behaviour is demonstrated. The production binary — plain `go build` — contains none of the nine manifest names and ignores all of them: the held gate fifo does not hang the search, the fired triggers inject nothing, the runner overrides change nothing, and the run exits 0 with clean stderr and no side-channel files. The `vrg_testhooks` build carries every manifest name and answers each seam: the `VRG_TEST_RUN_FINAL_MODEL`/`VRG_TEST_RUN_ERROR` tuple lands at the real `program.Run()` result site (a nil model drops the collected diagnostic and turns exit 130 into exit 0; the error override exits 2 with `vrg: boom`), and the diagnostic/failure triggers fire through `Config.DiagInject` and the program context. `TestMain` builds the suite's binary with the tag, so existing PTY/subprocess coverage runs against the hooked variant unchanged. Issues #46 and #48 extend this mechanism — new hooks join the manifest and ride the same tag; the production binary gains nothing.
