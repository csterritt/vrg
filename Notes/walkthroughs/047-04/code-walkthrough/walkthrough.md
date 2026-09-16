# Issue #47: single-line read-failure diagnostics for hostile filenames

*2026-09-15T22:23:16Z by Showboat 0.6.1*
<!-- showboat-id: bb4fe7b8-2c65-40c8-9c35-bc7cb4aa2c14 -->

This walkthrough demonstrates [Issue #47](../../../../issues/047-read-failure-single-line-filenames.md): a failed file read must surface as **exactly one diagnostic line** even when the filename contains hostile bytes — embedded newline, tab, invalid UTF-8, or ESC — per the [PRD](../../../../PRD-vrg.md) sections *Text, graphemes, and safe presentation* and *File loading, cache, reload, and selection consistency*.

Before the fix, the model built the diagnostic from `PathError.Error()`, which embeds the raw path; a filename newline then split the diagnostic across lines and a raw tab reached the renderer. The fix constructs every file-load diagnostic as `readFailureDiagnostic(path, err)` — `EscapePath(path)` for presentation plus a sanitized reason obtained by unwrapping the `*fs.PathError` — applied uniformly at the initial-load, `r` reload, and failed-path re-entry retry sites. The same single line appears in the overlay and in the stderr replay at exit.

First, the change must build cleanly and pass vet.

```bash
cd /home/chris/vrg && go build ./... && go vet ./... && echo 'build+vet ok'
```

```output
build+vet ok
```

The regression tests create real fixtures under `t.TempDir`-style disposable directories, index them with bytes-encoded records (so invalid UTF-8 filename bytes survive the JSON round-trip), hold the real `filebuffer.Load` at the file-load gate, remove or rename the fixture, then release the gate so the real `os.ReadFile` fails with a genuine `*fs.PathError`. Each test asserts the overlay text, the rendered overlay row set, and the collected replay diagnostics are the same single escaped line, across the initial-load, `r` reload, and failed-path re-entry retry sites.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/app -run 'TestReadFailureSingleLine' -timeout 60s 2>&1 | sed -e 's/([0-9.]*s)/(0.00s)/g' -e 's/[[:space:]][0-9.]*s$//'
```

```output
=== RUN   TestReadFailureSingleLineDiagnostics
=== RUN   TestReadFailureSingleLineDiagnostics/newline
=== RUN   TestReadFailureSingleLineDiagnostics/tab
=== RUN   TestReadFailureSingleLineDiagnostics/invalid-utf8
=== RUN   TestReadFailureSingleLineDiagnostics/esc
--- PASS: TestReadFailureSingleLineDiagnostics (0.00s)
    --- PASS: TestReadFailureSingleLineDiagnostics/newline (0.00s)
    --- PASS: TestReadFailureSingleLineDiagnostics/tab (0.00s)
    --- PASS: TestReadFailureSingleLineDiagnostics/invalid-utf8 (0.00s)
    --- PASS: TestReadFailureSingleLineDiagnostics/esc (0.00s)
=== RUN   TestReadFailureSingleLineReload
--- PASS: TestReadFailureSingleLineReload (0.00s)
=== RUN   TestReadFailureSingleLineReentry
--- PASS: TestReadFailureSingleLineReentry (0.00s)
PASS
ok  	vrg/internal/app
```

## Manual demonstration: real read failures in disposable temp directories

The manual scenario runs the issue's checks end to end through the real `filebuffer.Load` (no injected loader). `demo_artifacts/manual_demo_test.go` is copied into `internal/app` and run under the `manual_demo` build tag so it can reuse the package's model-test helpers; it is removed afterwards.

For each hostile filename kind — embedded newline, tab, invalid UTF-8, ESC — the demo creates the fixture inside `os.MkdirTemp("", "v47")`, indexes it, holds the load at the gate, removes the fixture, and releases the gate so the real read fails with ENOENT. It then prints the overlay row and quits, replaying the collected diagnostics to show the exit stderr stream carries the same single line. A permission-denied variant leaves a `pm\nz` fixture at mode 000 (cleanup trap restores the mode) so the real read fails with EACCES; environments where elevated privileges or ACLs still permit the read are reported unsupported rather than passed.

```bash
cd /home/chris/vrg && cp Notes/walkthroughs/047-04/code-walkthrough/demo_artifacts/manual_demo_test.go internal/app/manual_demo_test.go && go test -count=1 -v -tags manual_demo ./internal/app -run 'TestManualDemo' -timeout 30s 2>&1 | sed -e 's|/tmp/v47[0-9]*|/tmp/v47DIR|g' -e 's/([0-9.]*s)/(0.00s)/g' -e 's/[[:space:]][0-9.]*s$//'; st=${PIPESTATUS[0]}; rm -f internal/app/manual_demo_test.go; echo "manual-demo exit=$st"; exit $st
```

```output
=== RUN   TestManualDemoReadFailureSingleLine
newline: overlay row: open /tmp/v47DIR/nl\nz: no such file or directory
newline: exit stderr replay: open /tmp/v47DIR/nl\nz: no such file or directory
newline: cleanup evidence: /tmp/v47DIR removed (exists=false)
tab: overlay row: open /tmp/v47DIR/tb\tz: no such file or directory
tab: exit stderr replay: open /tmp/v47DIR/tb\tz: no such file or directory
tab: cleanup evidence: /tmp/v47DIR removed (exists=false)
invalid-utf8: overlay row: open /tmp/v47DIR/u\xffz: no such file or directory
invalid-utf8: exit stderr replay: open /tmp/v47DIR/u\xffz: no such file or directory
invalid-utf8: cleanup evidence: /tmp/v47DIR removed (exists=false)
esc: overlay row: open /tmp/v47DIR/es^[z: no such file or directory
esc: exit stderr replay: open /tmp/v47DIR/es^[z: no such file or directory
esc: cleanup evidence: /tmp/v47DIR removed (exists=false)
--- PASS: TestManualDemoReadFailureSingleLine (0.00s)
=== RUN   TestManualDemoReadFailurePermissionDenied
permission-denied: overlay row: open /tmp/v47DIR/pm\nz: permission denied
permission-denied: exit stderr replay: open /tmp/v47DIR/pm\nz: permission denied
permission-denied: cleanup evidence: mode restored, /tmp/v47DIR removed (exists=false)
--- PASS: TestManualDemoReadFailurePermissionDenied (0.00s)
PASS
ok  	vrg/internal/app
manual-demo exit=0
```

```bash
echo 'stray v47 fixture dirs:'; find /tmp -maxdepth 1 -name 'v47*' | wc -l; test ! -e internal/app/manual_demo_test.go && echo 'manual_demo_test.go removed from internal/app'
```

```output
stray v47 fixture dirs:
0
manual_demo_test.go removed from internal/app
```

Every filename byte kind — newline, tab, invalid UTF-8, ESC — surfaced as exactly one diagnostic line: `open <EscapePath-escaped path>: <sanitized reason>`, identical in the overlay and the exit stderr replay, with the raw path never repeated. The permission-denied variant produced the same single-line shape for EACCES. Cleanup removed every hostile fixture and restored the denied mode; no `v47*` directories remain in `/tmp` and the demo file is gone from `internal/app`.
