# Issue #47: read-failure diagnostics stay single-line for hostile filenames

*2026-09-18T11:16:27Z by Showboat 0.6.1*
<!-- showboat-id: 0b5b10b5-6e2e-49b5-b3ee-2c84cdf2dca0 -->

Issue #47 keeps a file-read failure to exactly one diagnostic line however hostile the filename: internal/app/browse.go's loadDiag builds 'cannot read <EscapePath(path)>: <reason>' where the path is single-line-escaped and the reason is the unwrapped *fs.PathError cause — a newline, tab, ESC, or invalid-UTF-8 byte in the name can never forge an extra diagnostic line in the overlay, the collected session diagnostics, or the post-exit stderr replay. One construction site is shared by the initial load, the r reload, and the failed-path re-entry retry. See Notes/issues/047-read-failure-single-line-filenames.md, Notes/tasks/047-read-failure-single-line-filenames.md, and the 'Text, graphemes, and safe presentation' and 'File loading, cache, reload, and selection consistency' sections of Notes/PRD-vrg.md. This walkthrough runs the focused real-read-failure tests, then drives the manual scenario on a real PTY with genuine ENOENT failures under the load gate for filenames embedding ESC, an invalid UTF-8 byte, a newline, and a tab. Artifacts (the tagged binary, pty_singleline.py, manual_route.sh) live in this directory; every hostile fixture is created and destroyed inside disposable /tmp directories.

## The real-read-failure tests

internal/app/readdiag_test.go's three tests build hostile-named fixtures in t.TempDir(), hold each load worker at an explicit gate, remove the fixture, and release the real os.ReadFile — the failure is a genuine *fs.PathError, never a permission denial, and cleanup is the temp dir's own. Each asserts the result is exactly one diagnostic line equal to 'cannot read ' + EscapePath(path) + ': ' + the sanitized reason, observed identically through the overlay row set, the collected diagnostics, and the stderr replay — at the initial-load, r-reload, and re-entry-retry sites.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestInitialLoadFailureSingleLineDiag|TestReloadFailureSingleLineDiag|TestReentryRetryFailureSingleLineDiag' ./internal/app 2>&1 | grep -E '^( *--- (PASS|FAIL)|ok|FAIL)' | sed -E 's/\([0-9.]+s\)//g; s/\t[0-9.]+s$//; s/^ +//'; echo "exit=${PIPESTATUS[0]}"
```

```output
--- PASS: TestInitialLoadFailureSingleLineDiag 
--- PASS: TestInitialLoadFailureSingleLineDiag/newline 
--- PASS: TestInitialLoadFailureSingleLineDiag/tab 
--- PASS: TestInitialLoadFailureSingleLineDiag/invalid-utf8 
--- PASS: TestInitialLoadFailureSingleLineDiag/esc 
--- PASS: TestReloadFailureSingleLineDiag 
--- PASS: TestReloadFailureSingleLineDiag/newline 
--- PASS: TestReloadFailureSingleLineDiag/tab 
--- PASS: TestReloadFailureSingleLineDiag/invalid-utf8 
--- PASS: TestReloadFailureSingleLineDiag/esc 
--- PASS: TestReentryRetryFailureSingleLineDiag 
ok  	vrg/internal/app
exit=0
```

```bash
cd /home/chris/vrg && go build -tags vrg_testhooks -o Notes/walkthroughs/047-04/code-walkthrough/vrg-testhooks ./cmd/vrg && go vet ./... && go vet -tags vrg_testhooks ./cmd/vrg && go build ./... && echo 'build+vet OK (both variants)'
```

```output
build+vet OK (both variants)
```

## Manual scenario: hostile filenames fail as one line each

The manual route uses only disposable directories. manual_route.sh mktemps <tmp-a> carrying four hostile-named needle files and <tmp-b> carrying one clean file plus a newline-named one, installs an EXIT trap that removes both directories on success or failure, then runs pty_singleline.py. The PTY driver runs the vrg_testhooks binary on a real 160x40 pty with the child's stderr redirected to a real file, so the Issue #11 replay is verified on real stderr. VRG_TEST_LOAD_GATE holds each file-load worker ahead of its real os.ReadFile while <tmp>/loadgate exists: the route waits for 'Loading…' — the index is built and the load is provably in flight — removes the fixture, then drops the gate so the real read fails with ENOENT. There is no chmod denial anywhere, so the failure is deterministic under any privilege level or ACL and no mode needs restoring. Session A covers all four byte classes at the initial-load and navigation sites; session B drives the re-entry retry and the r reload of the failed file. Esc dismisses each overlay (q under a modal overlay only dismisses it); q after dismissal exits and the collected diagnostics replay to stderr.

```bash
cd /home/chris/vrg/Notes/walkthroughs/047-04/code-walkthrough && bash manual_route.sh
```

```output
fixture dirs (normalized): <tmp-a> /tmp/vrg-047-a.NAovAN  <tmp-b> /tmp/vrg-047-b.OwyXNx
fixtures created (byte-repr listing):
  <tmp-a>:
    b'a-esc\x1bfile.txt'
    b'b-inv\xfffile.txt'
    b'c-nl\nfile.txt'
    b'd-tab\tfile.txt'
  <tmp-b>:
    b'aaa.txt'
    b'zz-nl\nfile.txt'
A initial : cannot read <tmp>/a-esc^[file.txt: no such file or directory
A nav[1]  : cannot read <tmp>/b-inv\xfffile.txt: no such file or directory
A nav[2]  : cannot read <tmp>/c-nl\nfile.txt: no such file or directory
A nav[3]  : cannot read <tmp>/d-tab\tfile.txt: no such file or directory
A stderr  : cannot read <tmp>/a-esc^[file.txt: no such file or directory
A stderr  : cannot read <tmp>/b-inv\xfffile.txt: no such file or directory
A stderr  : cannot read <tmp>/c-nl\nfile.txt: no such file or directory
A stderr  : cannot read <tmp>/d-tab\tfile.txt: no such file or directory
A OK — 4 hostile names, 4 single-line diagnostics, identical in overlay and replay
B startup : aaa.txt loaded — content visible
B nav     : cannot read <tmp>/zz-nl\nfile.txt: no such file or directory
B re-entry: prior failure shown while the retry is held
B retry   : identical single line appended (2 shown)
B reload  : r on the failed file — same single line (3 shown)
B stderr  : cannot read <tmp>/zz-nl\nfile.txt: no such file or directory
B stderr  : cannot read <tmp>/zz-nl\nfile.txt: no such file or directory
B stderr  : cannot read <tmp>/zz-nl\nfile.txt: no such file or directory
B OK — nav, re-entry retry, and r reload all emit the same one-line diagnostic
pty route exit: 0
cleanup ok: <tmp-a> and <tmp-b> removed — no hostile filename survives
```

Every failed read surfaced as exactly one diagnostic line — 'cannot read ' plus the EscapePath-escaped path plus ': no such file or directory' — identical between the overlay row set and the post-exit stderr replay, across the initial load, navigation loads, the re-entry retry, and the r reload. Session A's four hostile byte classes each produced a single escaped line (ESC as ^[, the invalid byte as \xff, newline as \n, tab as \t); session B collected and replayed three identical lines for the same newline-named path. The route's own exit status is 0 and the trap's 'cleanup ok' line is the evidence that no hostile filename, gate file, or changed permission survives outside the disposable directories.
