# Issue #46: Runtime errors through the unified shutdown/replay path

*2026-09-15T21:54:34Z by Showboat 0.6.1*
<!-- showboat-id: 804ed343-4464-48ad-a7d2-17dae3dac313 -->

Walkthrough for Issue #46 (Notes/tasks/046-runtime-error-common-diagnostic-replay.md, Notes/issues/046-runtime-error-common-diagnostic-replay.md), which unifies the program.Run() error path with the common shutdown contract. runSearch now owns an app.Diagnostics snapshot wired through app.WithDiagnostics — the model appends every collected diagnostic to it, so session diagnostics reach stderr even when the final model is absent or wrong-typed. After Run() returns (terminal restored by Bubble Tea) and cleanup terminates/reaps the child, one ordered replay runs for every return shape: session diagnostics in collection order, then the invalid-final-model diagnostic, then the Run() runtime error exactly once. Every failing shape exits 2, matching the startup-failure convention. References: Notes/PRD-vrg.md (*Outcome and exit-status contract* — diagnostic collection and replay, cleanup on application failures; user stories 22-23).

Contracts verified:
- The full return-shape matrix driven through the Issue #45 tagged program-runner seam in a real PTY lifecycle (TestRunReturnShapeUnifiedShutdown, five subtests covering the three contract cases).
- The manual scenario: VRG_TEST_RUN_ERROR injects an error after diagnostics were collected in a real PTY lifecycle → exit 2, terminal restored, stderr carries the session's collected diagnostics in order followed once by the application error.
- The invalid-final-model shape: VRG_TEST_RUN_FINAL_MODEL=nil with a forced-nil error exits 2 with session diagnostics then the invalid-final-model diagnostic — never a silent exit.

All generated artifacts (the tagged binary, fake rg, PTY helper, demo file, hook dir) live in this directory.

## Gates: build, vet, and the full test suite in both build variants

```bash
cd /home/chris/vrg && go build ./... && go build -tags vrg_testhooks ./... && go vet ./... && go vet -tags vrg_testhooks ./... && echo GATES-OK
```

```output
GATES-OK
```

```bash
cd /home/chris/vrg && go test -count=1 ./... -timeout 300s 2>&1 | sed 's/([0-9.]*s)//g; s/[[:space:]][0-9.]*s$//' && echo '=== vrg_testhooks variant ===' && go test -count=1 -tags vrg_testhooks ./... -timeout 300s 2>&1 | sed 's/([0-9.]*s)//g; s/[[:space:]][0-9.]*s$//'
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
=== vrg_testhooks variant ===
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

## The three return shapes through the tagged runner seam

TestRunReturnShapeUnifiedShutdown (cmd/vrg/runshape_test.go) drives the issue's return-shape matrix through the Issue #45 tagged program-runner boundary (VRG_TEST_RUN_FINAL_MODEL / VRG_TEST_RUN_ERROR) at the real program.Run() call site. Each subtest runs a real PTY lifecycle in which two diagnostics are collected in deterministic order — the fake rg's stderr warning acknowledged first via VRG_TEST_COLLECT_ACK, then a triggered DiagnosticMsg second — before a clean browse quit gives the seam a real (app.Model, nil) tuple to override. The three contract cases:

1. valid final model + Run() error → collected diagnostics replayed to stderr in collection order after terminal restoration, the runtime error appended exactly once, exit 2;
2. invalid/nil final model + Run() error → retained session diagnostics replayed, then the invalid-final-model diagnostic, then the runtime error once, exit 2;
3. invalid/nil final model + nil Run() error → retained session diagnostics then the invalid-final-model diagnostic, exit 2 — never a silent or zero exit.

Each case also proves the injected tuple reached the executable's actual post-Run() type/error branches (exit 2 where the clean browse quit would have exited 0), the child was terminated/reaped and the terminal restored exactly as on normal exits, and no diagnostic is emitted both by a direct write and the replay.

```bash
cd /home/chris/vrg && go test -count=1 -v ./cmd/vrg -run 'TestRunReturnShapeUnifiedShutdown' -timeout 120s | sed 's/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
```

```output
=== RUN   TestRunReturnShapeUnifiedShutdown
=== RUN   TestRunReturnShapeUnifiedShutdown/valid_model_run_error
=== RUN   TestRunReturnShapeUnifiedShutdown/nil_model_run_error
=== RUN   TestRunReturnShapeUnifiedShutdown/invalid_model_run_error
=== RUN   TestRunReturnShapeUnifiedShutdown/nil_model_nil_error
=== RUN   TestRunReturnShapeUnifiedShutdown/invalid_model_nil_error
--- PASS: TestRunReturnShapeUnifiedShutdown (0.00s)
    --- PASS: TestRunReturnShapeUnifiedShutdown/valid_model_run_error (0.00s)
    --- PASS: TestRunReturnShapeUnifiedShutdown/nil_model_run_error (0.00s)
    --- PASS: TestRunReturnShapeUnifiedShutdown/invalid_model_run_error (0.00s)
    --- PASS: TestRunReturnShapeUnifiedShutdown/nil_model_nil_error (0.00s)
    --- PASS: TestRunReturnShapeUnifiedShutdown/invalid_model_nil_error (0.00s)
PASS
ok  	vrg/cmd/vrg
```

## Manual scenario: the tagged runner injects an error after diagnostics were collected

The issue's manual check, driven by hand against the tagged binary in this directory. The fake rg (fakebin/rg) writes one stderr diagnostic — collected as a session diagnostic when SearchCompleteMsg is processed — then emits a valid stream and exits 0, leaving browse with a warning overlay. runpty.py drives the PTY: after the output settles it creates the file named by VRG_TRIGGER_FILE, which the tagged binary's VRG_TEST_DIAGNOSTIC_TRIGGER watcher turns into a second collected diagnostic (manual session diag two); then it sends Esc (dismiss the warning overlay) and q (quit browse), giving the runner seam a real (app.Model, nil) result to override. VRG_TEST_RUN_ERROR then injects the runtime error at the actual program.Run() return site.

Show the fixtures and build the tagged binary.

```bash
cd /home/chris/vrg/Notes/walkthroughs/046-04/code-walkthrough && cat fakebin/rg && echo '--- demo/test.txt ---' && cat demo/test.txt && cd /home/chris/vrg && go build -tags vrg_testhooks -o Notes/walkthroughs/046-04/code-walkthrough/vrg-tagged ./cmd/vrg && ls -la Notes/walkthroughs/046-04/code-walkthrough/vrg-tagged
```

```output
#!/bin/sh
# Fake ripgrep: write a stderr diagnostic (collected as a session
# diagnostic at SearchComplete), emit a valid JSON stream for
# demo/test.txt, then exit 0 — leaving a clean browse state for the
# tagged runner seam to override the Run() return tuple.
printf '%s\n' 'manual session warning 046' >&2
echo '{"type":"begin","data":{"path":{"text":"demo/test.txt"}}}'
printf '%s\n' '{"type":"match","data":{"path":{"text":"demo/test.txt"},"lines":{"text":"hello world\n"},"line_number":1,"submatches":[{"match":{"text":"hello"},"start":0,"end":5}]}}'
echo '{"type":"end","data":{"path":{"text":"demo/test.txt"},"binary_offset":null}}'
echo '{"type":"summary","data":{}}'
exit 0
--- demo/test.txt ---
hello world
-rwxrwxr-x 1 chris chris 7061178 Sep 15 17:56 Notes/walkthroughs/046-04/code-walkthrough/vrg-tagged
```

```bash
cd /home/chris/vrg/Notes/walkthroughs/046-04/code-walkthrough && rm -f hooks/diagtrigger && PATH=$(pwd)/fakebin:/usr/bin:/bin VRG_KEYS=$'\x1b,q' VRG_DELAY=0.5 VRG_TEST_DIAGNOSTIC_TRIGGER=$(pwd)/hooks/diagtrigger VRG_TEST_DIAGNOSTIC_TEXT='manual session diag two' VRG_TEST_RUN_ERROR='manual runtime error 046' VRG_TRIGGER_FILE=$(pwd)/hooks/diagtrigger timeout 20 python3 runpty.py ./vrg-tagged hello demo 2>&1; echo exit=$?
```

```output
┌────────────────────────────┐
│ manual session warning 046 │
└────────────────────────────┘demo/test.txt   ── demo/test.txt ── 1  hello world manual session warning 046
manual session diag two
vrg: manual runtime error 046
exit=2
```

The browse view renders with the warning overlay, then after Esc+q the process exits 2 and the ordered replay reaches stderr — the two session diagnostics in collection order followed once by the application error. Now prove the terminal was restored before the replay: rerun in raw mode and check the PTY bytes for the alt-screen exit and cursor-show sequences, printing what follows the restoration.

```bash
cd /home/chris/vrg/Notes/walkthroughs/046-04/code-walkthrough && rm -f hooks/diagtrigger raw.out && PATH=$(pwd)/fakebin:/usr/bin:/bin VRG_KEYS=$'\x1b,q' VRG_DELAY=0.5 VRG_RAW=1 VRG_TEST_DIAGNOSTIC_TRIGGER=$(pwd)/hooks/diagtrigger VRG_TEST_DIAGNOSTIC_TEXT='manual session diag two' VRG_TEST_RUN_ERROR='manual runtime error 046' VRG_TRIGGER_FILE=$(pwd)/hooks/diagtrigger timeout 20 python3 runpty.py ./vrg-tagged hello demo > raw.out; echo exit=$?; python3 -c "
d = open('raw.out','rb').read()
print('alt-screen enter (1049h):', b'\x1b[?1049h' in d)
print('alt-screen exit  (1049l):', b'\x1b[?1049l' in d)
print('cursor show      (?25h) :', b'\x1b[?25h' in d)
tail = d.rsplit(b'\x1b[?25h', 1)[-1]
print('post-restore stderr tail:', tail.decode('utf-8','replace').replace(chr(27),'<ESC>')[:200])
"; rm -f raw.out
```

```output
exit=2
alt-screen enter (1049h): True
alt-screen exit  (1049l): True
cursor show      (?25h) : True
post-restore stderr tail: <ESC>[?2004lmanual session warning 046
manual session diag two
vrg: manual runtime error 046

```

Restoration verified: the alt screen was entered and exited, the cursor was shown, and the replay lines appear only after the restoration sequences — the runtime error appended exactly once at the end of the ordered replay.

## Invalid final model + nil error: exit 2, never silent

The third contract case by hand: VRG_TEST_RUN_FINAL_MODEL=nil makes the tagged runner return a nil final model while VRG_TEST_RUN_ERROR=nil keeps the real (nil) error — the same PTY lifecycle, the same two collected diagnostics. stderr carries the session diagnostics followed by the invalid-final-model diagnostic (vrg: program returned no usable final model), and the process exits 2 where a silent or zero exit would previously have occurred.

```bash
cd /home/chris/vrg/Notes/walkthroughs/046-04/code-walkthrough && rm -f hooks/diagtrigger && PATH=$(pwd)/fakebin:/usr/bin:/bin VRG_KEYS=$'\x1b,q' VRG_DELAY=0.5 VRG_TEST_DIAGNOSTIC_TRIGGER=$(pwd)/hooks/diagtrigger VRG_TEST_DIAGNOSTIC_TEXT='manual session diag two' VRG_TEST_RUN_FINAL_MODEL=nil VRG_TEST_RUN_ERROR=nil VRG_TRIGGER_FILE=$(pwd)/hooks/diagtrigger timeout 20 python3 runpty.py ./vrg-tagged hello demo 2>&1; echo exit=$?
```

```output
┌────────────────────────────┐
│ manual session warning 046 │
└────────────────────────────┘demo/test.txt   ── demo/test.txt ── 1  hello world manual session warning 046
manual session diag two
vrg: program returned no usable final model
exit=2
```

## Summary

The walkthrough demonstrates:
- Full gates pass in both build variants: go build, go vet, and the complete test suite untagged and under -tags vrg_testhooks.
- TestRunReturnShapeUnifiedShutdown covers the three contract cases through five subtests at the real program.Run() boundary: session diagnostics replayed in collection order after terminal restoration, the invalid-final-model diagnostic when applicable, the runtime error appended exactly once, exit 2 on every failing shape, and the child terminated/reaped exactly as on normal exits.
- The manual scenario end to end: VRG_TEST_RUN_ERROR injects a runtime error after two diagnostics were collected in a real PTY lifecycle → exit 2, terminal restored (alt-screen exit + cursor show precede the replay bytes), and stderr carries the session's collected diagnostics in order followed once by the application error.
- The invalid-final-model shape by hand: VRG_TEST_RUN_FINAL_MODEL=nil with a forced-nil error exits 2 with the session diagnostics followed by vrg: program returned no usable final model — never a silent or zero exit.

References: Notes/issues/046-runtime-error-common-diagnostic-replay.md, Notes/tasks/046-runtime-error-common-diagnostic-replay.md, Notes/issues/045-remove-test-hooks-from-production-binary.md (the tagged runner seam consumed here), and Notes/PRD-vrg.md (*Outcome and exit-status contract*).
