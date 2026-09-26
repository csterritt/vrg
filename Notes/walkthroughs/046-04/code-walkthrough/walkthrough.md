# Issue #46: Bubble Tea runtime errors go through the common shutdown/diagnostic-replay path

*2026-09-25T13:08:57Z by Showboat 0.6.1*
<!-- showboat-id: 99189839-ee2b-4cd8-aaac-a22e02d1bfc3 -->

Walkthrough for [Issue #46](../../../tasks/046-runtime-error-common-diagnostic-replay.md) per `Notes/PRD-vrg.md` (*Outcome and exit-status contract* — the cleanup bullet: every controlled exit restores the terminal, terminates/reaps the child, and replays collected diagnostics). Every `program.Run()` return shape now funnels through one shutdown sequence in `runSearch`: `Config.OnCollect` feeds the process boundary's `diagSnapshot` as each diagnostic is collected — decoupling replay from the final-model type assertion — and after the child is reaped the snapshot replays session diagnostics in collection order, then the invalid-final-model diagnostic when applicable, then the runtime error exactly once. Every failing shape exits 2. Generated artifacts live in this directory: the `vrg-hooks` tagged binary, the `demo-runtime-error.sh` harness, and its `manual-runtime-error/` captures. Test durations are stripped so the document verifies cleanly.

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

## The three return-shape subprocess tests

`cmd/vrg/pty_returnshape_test.go` (`TestPTYRunReturnShapes`) drives the contract through a real PTY lifecycle: `fakeRgWarnBlockScript` emits `warn one` then blocks, `VRG_TEST_COLLECT_ACK` acknowledges its collection, then `q` ends the real program and the Issue #45 `VRG_TEST_RUN_*` runner seam substitutes the `(final model, error)` tuple at the actual post-`Run()` branches. The matrix covers all three shapes — valid model + `Run()` error, nil/invalid model + `Run()` error, nil/invalid model + nil error — asserting exit 2, the ordered post-restoration replay (session diagnostics → invalid-final-model diagnostic → runtime error, each exactly once), the `killed` reap status, and both halves of terminal restoration. `TestTaggedRunnerSeamSelectsReturnShape` covers the same contract on pipes.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestPTYRunReturnShapes|TestTaggedRunnerSeamSelectsReturnShape' ./cmd/vrg 2>&1 | grep -E 'PASS|FAIL|ok ' | grep -v '=== RUN' | sed -E 's/[[:space:]][(]?[0-9.]+s\)?$//'
```

```output
--- PASS: TestPTYRunReturnShapes
    --- PASS: TestPTYRunReturnShapes/valid_model_with_run_error
    --- PASS: TestPTYRunReturnShapes/nil_model_with_run_error
    --- PASS: TestPTYRunReturnShapes/invalid_model_with_run_error
    --- PASS: TestPTYRunReturnShapes/nil_model_without_error
    --- PASS: TestPTYRunReturnShapes/invalid_model_without_error
--- PASS: TestTaggedRunnerSeamSelectsReturnShape
    --- PASS: TestTaggedRunnerSeamSelectsReturnShape/baseline_real_tuple
    --- PASS: TestTaggedRunnerSeamSelectsReturnShape/valid_model_with_error
    --- PASS: TestTaggedRunnerSeamSelectsReturnShape/explicit_valid_model_with_error
    --- PASS: TestTaggedRunnerSeamSelectsReturnShape/nil_model_with_error
    --- PASS: TestTaggedRunnerSeamSelectsReturnShape/invalid_model_with_error
    --- PASS: TestTaggedRunnerSeamSelectsReturnShape/nil_model_without_error
PASS
ok  	vrg/cmd/vrg
```

## Manual scenario: an injected `Run()` error after collected diagnostics

`demo-runtime-error.sh` builds the `vrg_testhooks` binary and runs it on a tmux PTY against a warn-then-block fake rg. `VRG_TEST_COLLECT_ACK` is the handshake proving `warn one` entered the session collection before `q` ends the program; `VRG_TEST_REAP` records the child's reaped wait status. The runner seam then substitutes the `Run()` result. Baseline is the real tuple: a searching-quit exits 130 with the collected diagnostic replayed. `run-error` is the issue's scenario — a `Run()` error after collected diagnostics: exit 2, terminal restored, stderr carrying the session's diagnostics in order followed once by `vrg: boom`. The nil/invalid-model shapes replay the retained session diagnostics — previously stranded with the missing model — then the invalid-final-model diagnostic, then the runtime error; even the nil-model/nil-error shape exits 2 rather than the old silent 0. Every run shows `signal: killed` — the blocked child was terminated and reaped exactly as on normal exits.

```bash
cd /home/chris/vrg/Notes/walkthroughs/046-04/code-walkthrough && bash demo-runtime-error.sh
```

```output
== tagged runner seam: VRG_TEST_RUN_* through a real PTY lifecycle ==
-- baseline: exit 130, reap: signal: killed
   stderr: warn one
-- run-error: exit 2, reap: signal: killed
   stderr: warn one
   stderr: vrg: boom
-- nil-model-error: exit 2, reap: signal: killed
   stderr: warn one
   stderr: vrg: program returned a nil final model
   stderr: vrg: boom
-- invalid-model-error: exit 2, reap: signal: killed
   stderr: warn one
   stderr: vrg: program returned an unexpected final model
   stderr: vrg: boom
-- nil-model: exit 2, reap: signal: killed
   stderr: warn one
   stderr: vrg: program returned a nil final model
-- invalid-model: exit 2, reap: signal: killed
   stderr: warn one
   stderr: vrg: program returned an unexpected final model
```

## Result

Every Issue #46 behaviour is demonstrated. The `diagSnapshot` fed by `Config.OnCollect` decouples session diagnostics from the final-model type assertion, so a nil or wrong-type `Run()` model can no longer strand collected diagnostics or produce a silent exit. The single shutdown sequence — `Run()` returns with the terminal restored, the child is terminated and reaped, then the ordered replay runs (session diagnostics → invalid-final-model diagnostic → the runtime error exactly once) — serves all three return shapes, and every failing shape exits 2, extending the startup-failure convention to runtime errors. The `TestPTYRunReturnShapes` matrix proves it on the real PTY boundary (reap evidence, gone pid, termios and display restoration, ordered exactly-once replay); the manual scenario shows it end-to-end on tmux.

