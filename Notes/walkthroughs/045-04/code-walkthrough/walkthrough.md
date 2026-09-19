# Issue #45: test hooks out of the production binary — the vrg_testhooks topology

*2026-09-18T10:40:36Z by Showboat 0.6.1*
<!-- showboat-id: 52e6f1e6-38ba-4a97-a7ea-670ebf193b59 -->

Issue #45 removes the test-only seams that used to compile into the released vrg binary: environment variables in cmd/vrg/main.go let any invocation append to arbitrary files, inject failures, and hold preparation on poll loops. The seams now live behind the vrg_testhooks build tag in two narrow boundaries - cmd/vrg/seams.go + seams_testhooks.go (option wiring) and cmd/vrg/runner.go + runner_testhooks.go (the program-runner wrapper over program.Run(), reached through app.Config.RunProgram). TestMain builds the binary under test with -tags vrg_testhooks, so the existing PTY/subprocess suite exercises the hooked variant unchanged. See Notes/issues/045-remove-test-hooks-from-production-binary.md, Notes/tasks/045-remove-test-hooks-from-production-binary.md, and the 'Outcome and exit-status contract' and 'Testing Decisions' sections of Notes/PRD-vrg.md. This walkthrough runs the new boundary tests, demonstrates both build variants, then drives the issue's manual scenario: the untagged production binary probed with every name in the explicit vrg-consumed hook manifest, and the tagged binary's VRG_TEST_RUN_FINAL_MODEL/VRG_TEST_RUN_ERROR overrides reaching a real program.Run() result branch. Artifacts (both built binaries, pty_probe.py, and the fixture) live in this directory.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestProductionBinaryIgnoresHookManifest|TestRunnerSeamInjectsRunReturnShapes' ./cmd/vrg 2>&1 | grep -E '^( *--- (PASS|FAIL)|ok|FAIL)' | sed -E 's/\([0-9.]+s\)//g; s/\t[0-9.]+s$//; s/^ +//'; echo "exit=${PIPESTATUS[0]}"
```

```output
--- PASS: TestProductionBinaryIgnoresHookManifest 
--- PASS: TestRunnerSeamInjectsRunReturnShapes 
--- PASS: TestRunnerSeamInjectsRunReturnShapes/delegated_valid_model_nil_error 
--- PASS: TestRunnerSeamInjectsRunReturnShapes/explicit_valid_model_nil_error 
--- PASS: TestRunnerSeamInjectsRunReturnShapes/valid_model_error 
--- PASS: TestRunnerSeamInjectsRunReturnShapes/nil_model_nil_error 
--- PASS: TestRunnerSeamInjectsRunReturnShapes/nil_model_error 
--- PASS: TestRunnerSeamInjectsRunReturnShapes/invalid_model_nil_error 
--- PASS: TestRunnerSeamInjectsRunReturnShapes/invalid_model_error 
ok  	vrg/cmd/vrg
exit=0
```

## Both build variants

The default build produces the clean released artifact; -tags vrg_testhooks produces the test variant. Both compile and vet cleanly - scripts/verify.sh gates 3-4 keep it that way.

```bash
cd /home/chris/vrg && WT=Notes/walkthroughs/045-04/code-walkthrough && go build -o $WT/vrg ./cmd/vrg && go build -tags vrg_testhooks -o $WT/vrg-testhooks ./cmd/vrg && go vet ./... && go vet -tags vrg_testhooks ./cmd/vrg && echo 'build+vet OK (both variants)'
```

```output
build+vet OK (both variants)
```

## Manual scenario: the untagged artifact carries no hooks

Set each name in the explicit vrg-consumed hook manifest against the untagged production binary: none may alter behaviour, and a byte scan of the artifact finds none of the names. The tagged binary, by contrast, carries every manifest string.

```bash
cd /home/chris/vrg/Notes/walkthroughs/045-04/code-walkthrough && for n in VRG_TEST_REAP VRG_TEST_GATE VRG_TEST_LOAD_GATE VRG_TEST_COLLECT_ACK VRG_TEST_FAIL_TRIGGER VRG_TEST_FAIL_DIAGNOSTIC VRG_TEST_DIAGNOSTIC_TRIGGER VRG_TEST_DIAGNOSTIC_TEXT VRG_TEST_RUN_FINAL_MODEL VRG_TEST_RUN_ERROR; do if grep -q "$n" vrg; then echo "PROD contains $n"; else echo "prod clean: $n"; fi; done; echo '--- tagged variant:'; for n in VRG_TEST_REAP VRG_TEST_GATE VRG_TEST_LOAD_GATE VRG_TEST_COLLECT_ACK VRG_TEST_FAIL_TRIGGER VRG_TEST_FAIL_DIAGNOSTIC VRG_TEST_DIAGNOSTIC_TRIGGER VRG_TEST_DIAGNOSTIC_TEXT VRG_TEST_RUN_FINAL_MODEL VRG_TEST_RUN_ERROR; do grep -q "$n" vrg-testhooks && echo "tagged carries $n" || echo "TAGGED MISSING $n"; done
```

```output
prod clean: VRG_TEST_REAP
prod clean: VRG_TEST_GATE
prod clean: VRG_TEST_LOAD_GATE
prod clean: VRG_TEST_COLLECT_ACK
prod clean: VRG_TEST_FAIL_TRIGGER
prod clean: VRG_TEST_FAIL_DIAGNOSTIC
prod clean: VRG_TEST_DIAGNOSTIC_TRIGGER
prod clean: VRG_TEST_DIAGNOSTIC_TEXT
prod clean: VRG_TEST_RUN_FINAL_MODEL
prod clean: VRG_TEST_RUN_ERROR
--- tagged variant:
tagged carries VRG_TEST_REAP
tagged carries VRG_TEST_GATE
tagged carries VRG_TEST_LOAD_GATE
tagged carries VRG_TEST_COLLECT_ACK
tagged carries VRG_TEST_FAIL_TRIGGER
tagged carries VRG_TEST_FAIL_DIAGNOSTIC
tagged carries VRG_TEST_DIAGNOSTIC_TRIGGER
tagged carries VRG_TEST_DIAGNOSTIC_TEXT
tagged carries VRG_TEST_RUN_FINAL_MODEL
tagged carries VRG_TEST_RUN_ERROR
```

Now run that untagged binary on a real PTY with every manifest name armed: probe/gate, probe/loadgate, and probe/fail pre-exist (a live hook would hang the run or fail it), the ack/reap/diagnostic paths would gain lines, and the runner controls would flip the exit to 2. The binary browses, quits on q, exits 0, restores the display, and writes nothing.

```bash
cd /home/chris/vrg/Notes/walkthroughs/045-04/code-walkthrough && rm -rf probe && mkdir probe && : > probe/gate && : > probe/loadgate && : > probe/fail && python3 pty_probe.py ./vrg fixture fixture VRG_TEST_REAP=probe/reap VRG_TEST_GATE=probe/gate VRG_TEST_LOAD_GATE=probe/loadgate VRG_TEST_COLLECT_ACK=probe/collect VRG_TEST_FAIL_TRIGGER=probe/fail VRG_TEST_FAIL_DIAGNOSTIC=probe-failure VRG_TEST_DIAGNOSTIC_TRIGGER=probe/diag VRG_TEST_DIAGNOSTIC_TEXT=probe-diag VRG_TEST_RUN_FINAL_MODEL=nil VRG_TEST_RUN_ERROR=probe-run-error; echo 'probe dir afterwards (only the 3 pre-created triggers):'; ls probe
```

```output
exit=0
restoration-sequence seen: True
post-restoration output: '\x1b[?1049l\x1b[?25h\x1b[?2004l'
probe dir afterwards (only the 3 pre-created triggers):
fail
gate
loadgate
```

## The tagged variant: runner controls reach program.Run()'s result branch

The vrg_testhooks binary still delegates to the real program — the browse view renders and q drives a normal exit — then applies the overrides at the return site. VRG_TEST_RUN_FINAL_MODEL=nil drops the final model so the post-Run assertion takes the absent-model exit-2 branch; VRG_TEST_RUN_ERROR=<msg> replaces the returned error, which the common post-restoration writer replays as 'vrg: <msg>' after the display-restoration sequence, exit 2.

```bash
cd /home/chris/vrg/Notes/walkthroughs/045-04/code-walkthrough && echo '== baseline: tagged binary, no overrides ==' && python3 pty_probe.py ./vrg-testhooks fixture fixture && echo '== VRG_TEST_RUN_FINAL_MODEL=nil ==' && python3 pty_probe.py ./vrg-testhooks fixture fixture VRG_TEST_RUN_FINAL_MODEL=nil && echo '== VRG_TEST_RUN_FINAL_MODEL=invalid ==' && python3 pty_probe.py ./vrg-testhooks fixture fixture VRG_TEST_RUN_FINAL_MODEL=invalid && echo '== VRG_TEST_RUN_ERROR=vrg-injected-run-error ==' && python3 pty_probe.py ./vrg-testhooks fixture fixture VRG_TEST_RUN_ERROR=vrg-injected-run-error
```

```output
== baseline: tagged binary, no overrides ==
exit=0
restoration-sequence seen: True
post-restoration output: '\x1b[?1049l\x1b[?25h\x1b[?2004l'
== VRG_TEST_RUN_FINAL_MODEL=nil ==
exit=2
restoration-sequence seen: True
post-restoration output: '\x1b[?1049l\x1b[?25h\x1b[?2004l'
== VRG_TEST_RUN_FINAL_MODEL=invalid ==
exit=2
restoration-sequence seen: True
post-restoration output: '\x1b[?1049l\x1b[?25h\x1b[?2004l'
== VRG_TEST_RUN_ERROR=vrg-injected-run-error ==
exit=2
restoration-sequence seen: True
post-restoration output: '\x1b[?1049l\x1b[?25h\x1b[?2004lvrg: vrg-injected-run-error\r\n'
```

## Result

The production artifact is clean: every name in the explicit vrg-consumed hook manifest is absent from the untagged binary and provably inert against it - a fully-armed probe run browses, quits, and exits 0 with nothing written. The tagged variant preserves every prior seam behaviour (the suite's PTY/subprocess tests pass against TestMain's tagged build) and adds the dedicated program-runner seam: VRG_TEST_RUN_FINAL_MODEL=nil|invalid and VRG_TEST_RUN_ERROR=<msg> reach the executable's actual program.Run() result branches, which is the surface Issue #46 needs for its return-shape matrix and Issue #48 extends for acknowledgement hooks. Full verification: go build ./..., go vet ./..., go test ./..., and CGO_ENABLED=1 go test -race ./... all pass in the default configuration, plus the tagged-variant build/vet.
