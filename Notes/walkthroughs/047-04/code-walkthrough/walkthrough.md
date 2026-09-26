# Issue #47: Single-line read-failure diagnostics for hostile filenames

*2026-09-25T13:42:45Z by Showboat 0.6.1*
<!-- showboat-id: 4039e40f-f4ab-4f78-990f-cb9aa14af878 -->

A read failure reports `cannot read <path>: <reason>` — but `*os.PathError.Error()` embeds the raw resolved path, so a filename containing a newline produced a diagnostic that survived `present.Diagnostic`'s line-boundary preservation and split into two lines, echoing the raw path inside the escaped one. Issue #47 (`Notes/tasks/047-read-failure-single-line-filenames.md`) makes one failed read exactly one diagnostic line: `readReason` in `internal/app/browse.go` unwraps `*os.PathError` through `errors.As` to its bare `Err`, so the line carries the `present.Path`-escaped path once plus the errno text. The construction is shared by initial load, `r` reload, and the failed-path re-entry retry — all three sink through the same `loadDoneMsg` error branch. PRD cross-references: *Text, graphemes, and safe presentation* and *File loading, cache, reload, and selection consistency* in `Notes/PRD-vrg.md`. Walkthrough steps: repo gates, the new regression tests, then a manual PTY scenario driving four genuinely hostile filenames (embedded LF, TAB, invalid UTF-8, ESC) through real `os.ReadFile` failures behind the `vrg_testhooks` preparation gate.

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

`internal/app/readdiag_test.go` proves the contract at all three load sites with real filesystem failures — no injected errors: each case creates a hostile-named fixture (embedded LF, TAB, invalid UTF-8, ESC) inside `t.TempDir()`, indexes it through the base64 `bytes` path form, parks the load worker on `loadGate`, removes the fixture, and releases the gate so `os.ReadFile` fails for real. Every test requires exactly one diagnostic line carrying the `present.Path` form with a reason that never repeats the raw path — checked through the overlay row set, `failLines`, the session collection, and the stderr replay. `TestReadFailureReasonUnwrapsPathError` additionally pins the wrapped-PathError and plain-error branches of `readReason`.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestInitialReadFailureIsOneLine|TestReloadReadFailureIsOneLine|TestReEntryRetryReadFailureIsOneLine|TestReadFailureReasonUnwrapsPathError' ./internal/app 2>&1 | grep -E '^(=== RUN|--- PASS|--- FAIL|PASS|FAIL|ok)' | sed -E 's/\([0-9.]+s\)//; s/[[:space:]][0-9.]+s$//'
```

```output
=== RUN   TestInitialReadFailureIsOneLine
=== RUN   TestInitialReadFailureIsOneLine/newline
=== RUN   TestInitialReadFailureIsOneLine/tab
=== RUN   TestInitialReadFailureIsOneLine/invalid-utf8
=== RUN   TestInitialReadFailureIsOneLine/escape
--- PASS: TestInitialReadFailureIsOneLine 
=== RUN   TestReloadReadFailureIsOneLine
=== RUN   TestReloadReadFailureIsOneLine/newline
=== RUN   TestReloadReadFailureIsOneLine/tab
=== RUN   TestReloadReadFailureIsOneLine/invalid-utf8
=== RUN   TestReloadReadFailureIsOneLine/escape
--- PASS: TestReloadReadFailureIsOneLine 
=== RUN   TestReEntryRetryReadFailureIsOneLine
=== RUN   TestReEntryRetryReadFailureIsOneLine/newline
=== RUN   TestReEntryRetryReadFailureIsOneLine/tab
=== RUN   TestReEntryRetryReadFailureIsOneLine/invalid-utf8
=== RUN   TestReEntryRetryReadFailureIsOneLine/escape
--- PASS: TestReEntryRetryReadFailureIsOneLine 
=== RUN   TestReadFailureReasonUnwrapsPathError
--- PASS: TestReadFailureReasonUnwrapsPathError 
PASS
ok  	vrg/internal/app
```

Manual scenario, entirely inside a disposable directory (`demo-single-line.sh`): four real files named `bad` + LF/TAB/invalid-UTF-8/ESC + `name.txt` are created under `manual-single-line/fixture/`; a fake `rg` indexes them via base64 `bytes` path records; the `vrg_testhooks` binary runs on a tmux PTY with `VRG_TEST_GATE` holding index preparation. All four fixtures are removed while the gate holds, so the released loads hit genuine ENOENT — not a permission denial that elevated privileges could defeat (no cleanup trap needed for modes; the disposable dir itself is removed and recreated each run). Each failure is captured from the overlay, keys are sent spaced apart (an ESC immediately followed by n coalesces into Alt+n on the PTY input parser), and `VRG_TEST_COLLECT_ACK` acknowledges each collected diagnostic. At exit, the stderr replay must be exactly four lines — one per failure — with every path escaped inline and no raw control or high bytes anywhere.

```bash
bash demo-single-line.sh 2>&1
```

```output
== fixtures indexed (names shown escaped) ==
   $'bad\nname.txt'
   $'bad\tname.txt'
   $'bad\377name.txt'
   $'bad\Ename.txt'
== fixtures removed while preparation gate held ==
   (empty)
-- failure 1, overlay row:
   cannot read bad\tname.txt: no such file or directory
-- failure 2, overlay row:
   cannot read bad\nname.txt: no such file or directory
-- failure 3, overlay row:
   cannot read bad^[name.txt: no such file or directory
-- failure 4, overlay row:
   cannot read bad\xffname.txt: no such file or directory
== stderr replay: 4 lines, exit 0 ==
   cannot read bad\tname.txt: no such file or directory
   cannot read bad\nname.txt: no such file or directory
   cannot read bad^[name.txt: no such file or directory
   cannot read bad\xffname.txt: no such file or directory
   no raw control or high bytes in the replay
```

Evidence retained beside this document: `demo-single-line.sh` (the whole scenario), `vrg-hooks` (the tagged binary), and `manual-single-line/` — the disposable fixture dir (empty after the run: every hostile name was removed behind the gate), the fake-rg stream, the gate fifo, the per-failure pane captures `screen-01.txt`–`screen-04.txt`, the collection acks, the four-line `err.txt` replay, and `exit.txt` = 0. Cleanup is intrinsic: `manual-single-line/` is deleted and recreated at the top of every run, the tmux session is killed by a trap, and no fixture survives the scenario — that removal is what makes the failures real. Each diagnostic is exactly one line: the `present.Path` escape converts LF/TAB/ESC/invalid bytes to ASCII render text before composition, and `readReason` keeps `PathError.Error()` — and its raw path copy — out of the reason.
