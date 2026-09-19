# Issue #46: runtime errors through the common shutdown/diagnostic-replay path

*2026-09-18T10:54:36Z by Showboat 0.6.1*
<!-- showboat-id: c2fcc74a-4d16-4808-b2b9-5defe92a521d -->

Issue #46 routes every program.Run() return shape through the common shutdown/diagnostic-replay path. Previously a Run() error bypassed the post-restoration replay (the collected session diagnostics were reachable only through the final-model type assertion) and a nil or wrong-type final model exited 2 silently. Now Run installs a diagSink snapshot that collectDiags mirrors every line into — independent of the final-model assertion — and one ordered shutdown sequence replays: session diagnostics in collection order, then 'vrg: program ended without a valid final model' when the model is absent or wrong type, then the runtime error exactly once; every failing shape exits 2. See Notes/issues/046-runtime-error-common-diagnostic-replay.md, Notes/tasks/046-runtime-error-common-diagnostic-replay.md, and the 'Outcome and exit-status contract' section of Notes/PRD-vrg.md. This walkthrough runs the return-shape subprocess tests, then drives the issue's manual scenario on a real PTY: the Issue #45 tagged program-runner control returns an error after diagnostics were collected, so the process exits 2 with the terminal restored and stderr carrying the collected diagnostics in order followed once by the application error. Artifacts (the tagged binary, pty_probe.py, and the fixture) live in this directory.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestRunReturnShapesShutdownReplay|TestRunnerSeamInjectsRunReturnShapes' ./cmd/vrg 2>&1 | grep -E '^( *--- (PASS|FAIL)|ok|FAIL)' | sed -E 's/\([0-9.]+s\)//g; s/\t[0-9.]+s$//; s/^ +//'; echo "exit=${PIPESTATUS[0]}"
```

```output
--- PASS: TestRunReturnShapesShutdownReplay 
--- PASS: TestRunReturnShapesShutdownReplay/valid_final_model,_nil_error 
--- PASS: TestRunReturnShapesShutdownReplay/valid_final_model,_run_error 
--- PASS: TestRunReturnShapesShutdownReplay/nil_final_model,_run_error 
--- PASS: TestRunReturnShapesShutdownReplay/invalid_final_model,_run_error 
--- PASS: TestRunReturnShapesShutdownReplay/nil_final_model,_nil_error 
--- PASS: TestRunReturnShapesShutdownReplay/invalid_final_model,_nil_error 
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

## The return-shape subprocess tests

TestRunReturnShapesShutdownReplay (cmd/vrg/runshape_test.go) drives the full matrix — valid / nil / invalid final model x nil / injected error — through the tagged runner at the real program.Run() boundary. Each cell runs a real PTY lifecycle in which two diagnostics ('warn one', 'warn two') are acknowledged as collected before a q quit lets Run() return and the VRG_TEST_RUN_FINAL_MODEL/VRG_TEST_RUN_ERROR controls substitute the tuple. The valid+nil control cell keeps the real quit's exit 130, proving the failing cells' exit 2 comes from the post-Run() branches rather than a controlled model quit. Every cell asserts terminal restoration, child termination/reaping, and one deterministic exactly-once replay.

```bash
cd /home/chris/vrg && go build -tags vrg_testhooks -o Notes/walkthroughs/046-04/code-walkthrough/vrg-testhooks ./cmd/vrg && go vet ./... && go vet -tags vrg_testhooks ./cmd/vrg && go build ./... && echo 'build+vet OK (both variants)'
```

```output
build+vet OK (both variants)
```

## Manual scenario: an injected Run() error after collected diagnostics

The fixture rg (fixture/rg) writes 'warn one' and 'warn two' to stderr, records its pid, then blocks on sleep - so both diagnostics are collected while a live child still owes termination and reaping. pty_probe.py runs the tagged binary on a real 80x24 PTY, waits on the VRG_TEST_DIAGNOSTIC_TRIGGER acknowledgement file (two lines = both diagnostics processed into the session collection), sends q, then reports the exit status, display + termios restoration, the child's fate, the reap evidence, and everything emitted after the leave-alt-screen sequence. The baseline first: no runner override, so q while searching is an ordinary cancellation.

```bash
cd /home/chris/vrg/Notes/walkthroughs/046-04/code-walkthrough && rm -rf probe && mkdir probe && echo "== baseline: tagged binary, no overrides (q while searching cancels) ==" && python3 pty_probe.py ./vrg-testhooks fixture fixture $PWD/probe/ack0 $PWD/probe/pid0 $PWD/probe/reap0 VRG_TEST_DIAGNOSTIC_TRIGGER=$PWD/probe/ack0 VRG_TEST_REAP=$PWD/probe/reap0 VRG_TEST_RG_PID=$PWD/probe/pid0
```

```output
== baseline: tagged binary, no overrides (q while searching cancels) ==
exit=130
restoration-sequence seen: True
termios restored: True
child pid 1454354: gone; reap evidence: reaped code=-1 err=signal: killed
post-restoration output: '\x1b[?1049l\x1b[?25h\x1b[?2004lwarn one\r\nwarn two\r\n'
```

```bash
cd /home/chris/vrg/Notes/walkthroughs/046-04/code-walkthrough && echo "== VRG_TEST_RUN_ERROR=vrg-injected-run-error (valid final model) ==" && python3 pty_probe.py ./vrg-testhooks fixture fixture $PWD/probe/ack1 $PWD/probe/pid1 $PWD/probe/reap1 VRG_TEST_DIAGNOSTIC_TRIGGER=$PWD/probe/ack1 VRG_TEST_REAP=$PWD/probe/reap1 VRG_TEST_RG_PID=$PWD/probe/pid1 VRG_TEST_RUN_ERROR=vrg-injected-run-error
```

```output
== VRG_TEST_RUN_ERROR=vrg-injected-run-error (valid final model) ==
exit=2
restoration-sequence seen: True
termios restored: True
child pid 1454393: gone; reap evidence: reaped code=-1 err=signal: killed
post-restoration output: '\x1b[?1049l\x1b[?25h\x1b[?2004lwarn one\r\nwarn two\r\nvrg: vrg-injected-run-error\r\n'
```

The injected error replaced the real Run() result at the executable's boundary: exit 2 instead of the baseline's 130, the collected diagnostics replayed in collection order after the display-restoration sequence, then the application error appended exactly once - the terminal restored and the blocked child terminated and reaped exactly as on normal exits. The remaining two shapes exercise the absent/wrong-type final model: nil + error, then nil + nil error (the shape that used to exit 2 silently).

```bash
cd /home/chris/vrg/Notes/walkthroughs/046-04/code-walkthrough && echo "== VRG_TEST_RUN_FINAL_MODEL=nil + VRG_TEST_RUN_ERROR ==" && python3 pty_probe.py ./vrg-testhooks fixture fixture $PWD/probe/ack2 $PWD/probe/pid2 $PWD/probe/reap2 VRG_TEST_DIAGNOSTIC_TRIGGER=$PWD/probe/ack2 VRG_TEST_REAP=$PWD/probe/reap2 VRG_TEST_RG_PID=$PWD/probe/pid2 VRG_TEST_RUN_FINAL_MODEL=nil VRG_TEST_RUN_ERROR=vrg-injected-run-error && echo "== VRG_TEST_RUN_FINAL_MODEL=nil, no error ==" && python3 pty_probe.py ./vrg-testhooks fixture fixture $PWD/probe/ack3 $PWD/probe/pid3 $PWD/probe/reap3 VRG_TEST_DIAGNOSTIC_TRIGGER=$PWD/probe/ack3 VRG_TEST_REAP=$PWD/probe/reap3 VRG_TEST_RG_PID=$PWD/probe/pid3 VRG_TEST_RUN_FINAL_MODEL=nil
```

```output
== VRG_TEST_RUN_FINAL_MODEL=nil + VRG_TEST_RUN_ERROR ==
exit=2
restoration-sequence seen: True
termios restored: True
child pid 1454456: gone; reap evidence: reaped code=-1 err=signal: killed
post-restoration output: '\x1b[?1049l\x1b[?25h\x1b[?2004lwarn one\r\nwarn two\r\nvrg: program ended without a valid final model\r\nvrg: vrg-injected-run-error\r\n'
== VRG_TEST_RUN_FINAL_MODEL=nil, no error ==
exit=2
restoration-sequence seen: True
termios restored: True
child pid 1454471: gone; reap evidence: reaped code=-1 err=signal: killed
post-restoration output: '\x1b[?1049l\x1b[?25h\x1b[?2004lwarn one\r\nwarn two\r\nvrg: program ended without a valid final model\r\n'
```

## Result

Every program.Run() return shape now converges on the single ordered shutdown sequence: terminal restoration and child termination/reaping first (identical to normal exits), then one post-restoration replay — retained session diagnostics in collection order, the invalid-final-model diagnostic when the returned model is absent or the wrong type, and the runtime error appended exactly once. The baseline cancellation still exits 130; each failing shape is a controlled application failure exiting 2, never silent. The diagSink snapshot means the final-model type assertion is no longer the session collection's only channel to stderr. Full verification: go build ./..., go vet ./..., go test ./..., and CGO_ENABLED=1 go test -race ./... all pass in the default configuration, plus the tagged-variant build/vet/test.
