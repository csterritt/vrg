# Issue #45: Test hooks live only in the vrg_testhooks build

*2026-09-15T21:25:51Z by Showboat 0.6.1*
<!-- showboat-id: 3e95a8b2-9e93-42f7-af76-5ad238fc13cd -->

Walkthrough for Issue #45 (Notes/tasks/045-remove-test-hooks-from-production-binary.md, Notes/issues/045-remove-test-hooks-from-production-binary.md), which moves every test-only seam out of the released vrg binary behind a vrg_testhooks build variant. Two build-constrained boundaries join unconditional call sites in runSearch: the option/process wiring testSeamOptions(proc) and the program-runner wrapper runProgram(model, stdout). References: Notes/PRD-vrg.md (*Outcome and exit-status contract* — cleanup on application failures; *Testing Decisions* — the subprocess-boundary harness).

Contracts verified:
- The untagged production binary ignores every name in the explicit vrg-consumed hook manifest and contains none of the hook strings (the boundary test TestUntaggedBinaryIgnoresHookManifest plus the manual strings/behavior probe below).
- The vrg_testhooks binary built by TestMain keeps every prior seam-controlled PTY/subprocess behavior working unchanged.
- The tagged runner seam delivers every final-model/error tuple Issue #46 needs at the executable's actual program.Run() return site (TestTaggedRunnerSeamReturnShapes plus the manual override run below).
- No polling remains in production code; the tagged watchers poll with a paced sleep.

All generated artifacts (binaries, fake rg, PTY helper, demo file) live in this directory.

## Gates: build, vet, and the full test suite

The default-configuration gates all pass; the cmd/vrg subprocess suite runs against the vrg_testhooks-tagged binary that TestMain builds.

```bash
cd /home/chris/vrg && go build ./... && go vet ./... && echo GATES-OK
```

```output
GATES-OK
```

```bash
cd /home/chris/vrg && go test -count=1 ./... -timeout 300s | sed 's/([0-9.]*s)//g; s/[[:space:]][0-9.]*s$//'
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

## Boundary tests: the clean artifact and the tagged runner seam

TestUntaggedBinaryIgnoresHookManifest builds an untagged production binary, drives an ordinary fake-rg search with every name in the explicit vrg-consumed hook manifest set (option hooks plus the VRG_TEST_RUN_FINAL_MODEL/VRG_TEST_RUN_ERROR runner controls), and asserts a normal exit 0, no hook marker text on stderr, no side-effect files, and — via bytes.Contains over the artifact — none of the manifest names in the binary. TestTaggedRunnerSeamReturnShapes builds the tagged binary and drives every final-model/error tuple Issue #46 needs through a real PTY quit: injected error text reaches the Run() error branch verbatim for valid, nil, and invalid model shapes; nil/invalid models with a forced-nil error reach the invalid-final-model branch (exit 2); unset controls delegate and exit 0.

```bash
cd /home/chris/vrg && go test -count=1 -v ./cmd/vrg -run 'TestUntaggedBinaryIgnoresHookManifest|TestTaggedRunnerSeamReturnShapes' -timeout 120s | sed 's/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
```

```output
=== RUN   TestUntaggedBinaryIgnoresHookManifest
--- PASS: TestUntaggedBinaryIgnoresHookManifest (0.00s)
=== RUN   TestTaggedRunnerSeamReturnShapes
=== RUN   TestTaggedRunnerSeamReturnShapes/error/valid_model
=== RUN   TestTaggedRunnerSeamReturnShapes/error/nil_model
=== RUN   TestTaggedRunnerSeamReturnShapes/error/invalid_model
=== RUN   TestTaggedRunnerSeamReturnShapes/no_error/nil_model
=== RUN   TestTaggedRunnerSeamReturnShapes/no_error/invalid_model
=== RUN   TestTaggedRunnerSeamReturnShapes/no_controls_delegates
--- PASS: TestTaggedRunnerSeamReturnShapes (0.00s)
    --- PASS: TestTaggedRunnerSeamReturnShapes/error/valid_model (0.00s)
    --- PASS: TestTaggedRunnerSeamReturnShapes/error/nil_model (0.00s)
    --- PASS: TestTaggedRunnerSeamReturnShapes/error/invalid_model (0.00s)
    --- PASS: TestTaggedRunnerSeamReturnShapes/no_error/nil_model (0.00s)
    --- PASS: TestTaggedRunnerSeamReturnShapes/no_error/invalid_model (0.00s)
    --- PASS: TestTaggedRunnerSeamReturnShapes/no_controls_delegates (0.00s)
PASS
ok  	vrg/cmd/vrg
```

## Manual scenario: the production binary ignores the whole manifest

The issue's manual check. Build the untagged production binary (vrg-prod) and the tagged binary (vrg-tagged) into this directory.

```bash
cd /home/chris/vrg && go build -o Notes/walkthroughs/045-04/code-walkthrough/vrg-prod ./cmd/vrg && go build -tags vrg_testhooks -o Notes/walkthroughs/045-04/code-walkthrough/vrg-tagged ./cmd/vrg && ls -la Notes/walkthroughs/045-04/code-walkthrough/vrg-*
```

```output
-rwxrwxr-x 1 chris chris 7056381 Sep 15 17:27 Notes/walkthroughs/045-04/code-walkthrough/vrg-prod
-rwxrwxr-x 1 chris chris 7060401 Sep 15 17:27 Notes/walkthroughs/045-04/code-walkthrough/vrg-tagged
```

Run the production binary under a PTY with every name in the explicit vrg-consumed hook manifest set to a provocative value: side-effect paths inside hooks/ that never get created as triggers, marker text that must never surface, and the runner controls selecting an injected error. The binary must behave exactly as if none were set — normal browse, q exits 0 — and leave hooks/ empty. runpty.py drives the PTY and echoes the stripped terminal output; exit= is the vrg exit status.

```bash
cd /home/chris/vrg/Notes/walkthroughs/045-04/code-walkthrough && PATH=$(pwd)/fakebin:/usr/bin:/bin VRG_KEYS=q VRG_DELAY=0.5 VRG_TEST_REAP=$(pwd)/hooks/reap VRG_TEST_GATE=$(pwd)/hooks/gate VRG_TEST_FAIL_TRIGGER=$(pwd)/hooks/failtrigger VRG_TEST_FAIL_DIAGNOSTIC='hook must not fire: failure' VRG_TEST_DIAGNOSTIC_TRIGGER=$(pwd)/hooks/diagtrigger VRG_TEST_DIAGNOSTIC_TEXT='hook must not fire: diagnostic' VRG_TEST_COLLECT_ACK=$(pwd)/hooks/ack VRG_TEST_RUN_FINAL_MODEL=nil VRG_TEST_RUN_ERROR='hook must not fire: run error' timeout 15 python3 runpty.py ./vrg-prod hello demo 2>&1; echo exit=$?
```

```output
demo/test.txt   ── demo/test.txt ── 1  hello world
exit=0
```

```bash
cd /home/chris/vrg/Notes/walkthroughs/045-04/code-walkthrough && mkdir -p hooks && ls -la hooks/ && echo '--- strings probe on vrg-prod ---' && for n in VRG_TEST_REAP VRG_TEST_GATE VRG_TEST_FAIL_TRIGGER VRG_TEST_FAIL_DIAGNOSTIC VRG_TEST_DIAGNOSTIC_TRIGGER VRG_TEST_DIAGNOSTIC_TEXT VRG_TEST_COLLECT_ACK VRG_TEST_RUN_FINAL_MODEL VRG_TEST_RUN_ERROR; do printf '%-32s %s\n' "$n" "$(strings vrg-prod | grep -c "$n")"; done && echo '--- same probe on vrg-tagged ---' && for n in VRG_TEST_REAP VRG_TEST_GATE VRG_TEST_FAIL_TRIGGER VRG_TEST_FAIL_DIAGNOSTIC VRG_TEST_DIAGNOSTIC_TRIGGER VRG_TEST_DIAGNOSTIC_TEXT VRG_TEST_COLLECT_ACK VRG_TEST_RUN_FINAL_MODEL VRG_TEST_RUN_ERROR; do printf '%-32s %s\n' "$n" "$(strings vrg-tagged | grep -c "$n")"; done
```

```output
total 8
drwxrwxr-x 2 chris chris 4096 Sep 15 17:25 .
drwxrwxr-x 5 chris chris 4096 Sep 15 17:27 ..
--- strings probe on vrg-prod ---
VRG_TEST_REAP                    0
VRG_TEST_GATE                    0
VRG_TEST_FAIL_TRIGGER            0
VRG_TEST_FAIL_DIAGNOSTIC         0
VRG_TEST_DIAGNOSTIC_TRIGGER      0
VRG_TEST_DIAGNOSTIC_TEXT         0
VRG_TEST_COLLECT_ACK             0
VRG_TEST_RUN_FINAL_MODEL         0
VRG_TEST_RUN_ERROR               0
--- same probe on vrg-tagged ---
VRG_TEST_REAP                    1
VRG_TEST_GATE                    1
VRG_TEST_FAIL_TRIGGER            1
VRG_TEST_FAIL_DIAGNOSTIC         1
VRG_TEST_DIAGNOSTIC_TRIGGER      1
VRG_TEST_DIAGNOSTIC_TEXT         1
VRG_TEST_COLLECT_ACK             1
VRG_TEST_RUN_FINAL_MODEL         1
VRG_TEST_RUN_ERROR               1
```

## Tagged build: the runner seam reaches a real program.Run() result branch

The other half of the manual check: on the tagged binary, VRG_TEST_RUN_ERROR injects a non-nil error at the actual Run() return site — the post-Run() error branch prints 'vrg: <text>' after the terminal is restored and the process exits 2. VRG_TEST_RUN_FINAL_MODEL=nil with VRG_TEST_RUN_ERROR=nil reaches the invalid-final-model branch (exit 2, no runtime-error line). The q key still quits the real browse first, proving the tuple is substituted at the genuine return site, not a short-circuit.

```bash
cd /home/chris/vrg/Notes/walkthroughs/045-04/code-walkthrough && PATH=$(pwd)/fakebin:/usr/bin:/bin VRG_KEYS=q VRG_DELAY=0.5 VRG_TEST_RUN_ERROR='runner seam injected error 045' timeout 15 python3 runpty.py ./vrg-tagged hello demo 2>&1; echo exit=$?
```

```output
demo/test.txt   ── demo/test.txt ── 1  hello worldvrg: runner seam injected error 045
exit=2
```

```bash
cd /home/chris/vrg/Notes/walkthroughs/045-04/code-walkthrough && PATH=$(pwd)/fakebin:/usr/bin:/bin VRG_KEYS=q VRG_DELAY=0.5 VRG_TEST_RUN_FINAL_MODEL=nil VRG_TEST_RUN_ERROR=nil timeout 15 python3 runpty.py ./vrg-tagged hello demo 2>&1; echo exit=$?
```

```output
demo/test.txt   ── demo/test.txt ── 1  hello world
exit=2
```

## Summary

The walkthrough demonstrates:
- Full gates: go build, go vet, and the complete test suite pass in the default configuration, with the cmd/vrg subprocess suite exercising the vrg_testhooks-tagged binary TestMain builds.
- The boundary tests: TestUntaggedBinaryIgnoresHookManifest proves the released artifact ignores the entire explicit vrg-consumed hook manifest and contains none of its names; TestTaggedRunnerSeamReturnShapes proves the runner seam delivers every final-model/error tuple Issue #46 needs at the executable's actual program.Run() return site.
- The manual scenario end to end: the production binary with all nine manifest names set to provocative values browses normally and exits 0, leaves hooks/ empty, and strings on the artifact finds zero occurrences of every hook name — while the same probe on the tagged binary finds all nine.
- The tagged runner controls exercised by hand: VRG_TEST_RUN_ERROR surfaces verbatim at the real post-Run() error branch (exit 2), and VRG_TEST_RUN_FINAL_MODEL=nil plus VRG_TEST_RUN_ERROR=nil reaches the invalid-final-model branch — confirming Issues #46 and #48 can extend the same tagged mechanism without adding production hooks.

References: Notes/issues/045-remove-test-hooks-from-production-binary.md, Notes/issues/046-runtime-error-common-diagnostic-replay.md, Notes/issues/048-pty-tests-deterministic-handshakes.md, and Notes/PRD-vrg.md (*Outcome and exit-status contract*; *Testing Decisions*).
