# Issue #27: explicit reload — the r reread route

*2026-09-17T23:27:50Z by Showboat 0.6.1*
<!-- showboat-id: 6e70ae7a-e05a-4c67-a2b5-e6fd48813758 -->

Issue #27 gives vrg an explicit reload: pressing r in browse state rereads the current file directly from disk — never rerunning rg, never touching the matched-line cursor or the search-derived stops — while preserving the cursor and the logical viewport anchor clamped to the new content. A duplicate r or a re-entry during the in-flight reload is dropped, not queued; the placeholder's change from 'Loading…' to content or '(unreadable)' is the only completion signal; a failed reload replaces the old display through the Issue #26 overlay machinery; and cached content stays intentionally stable against disk edits until r. Each successful load bumps the path's content revision, which re-keys the Issue #17 layout demand so a pre-reload layout arriving late is discarded, and the reload records a pendingAnchor intent — preserve the anchor, no reveal — committed only when the new revision's matching prepared layout installs. See Notes/issues/027-explicit-reload-r.md, Notes/tasks/027-explicit-reload-r.md, and the PRD section 'File loading, cache, reload, and selection consistency' in Notes/PRD-vrg.md. All artifacts live in this directory.

## Gates — module integrity, build, vet

```bash
cd /home/chris/vrg && go mod verify && go build ./... && go vet ./... && echo GATES-OK
```

```output
all modules verified
GATES-OK
```

## The r route — one reread, dropped duplicates, failure replacement

internal/app/reload_test.go pins the Issue #27 contracts through the existing seams — heldNthLoad holding the reload's ordinal load, gatedFailLoader flipping read outcomes, and the new layoutHold arming a layout gate mid-test. TestRShowsLoadingAndRereadsOnce: r shows 'Loading…' over the filename row still naming the path, mints exactly one reread under the path's identity, and leaves the index pointer, cursor, and pre-request revision untouched. TestDuplicateRDroppedNotQueued: a second r while the reload is held issues no leaf and no worker — dropped, not queued — and after settlement r mints a fresh request. TestReentryDuringReloadDropped: an n/p there-and-back onto a path whose reload is held mints no second load. TestFailedReloadReplacesContent: the failed reread drops the buffer and layout, shows '(unreadable)' plus the overlay, and collects one diagnostic. TestSecondConsecutiveReloadFailureAppends: r fires while the overlay is open — the retry route it cannot block — and the second failure appends exactly one occurrence with the reader's scroll preserved. TestReloadSuccessLeavesOverlayOpen: a successful reread under the prior-failure overlay leaves it up until Esc. TestRWorksWithOneStopIndex: the only retry route a single-stop index has. TestDiskChangeWithoutRIsStable: a simulated disk edit changes nothing without r.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestRShowsLoadingAndRereadsOnce|TestDuplicateRDroppedNotQueued|TestReentryDuringReloadDropped|TestFailedReloadReplacesContent|TestSecondConsecutiveReloadFailureAppends|TestReloadSuccessLeavesOverlayOpen|TestRWorksWithOneStopIndex|TestDiskChangeWithoutRIsStable' ./internal/app 2>&1 | grep -E '^(--- |ok|FAIL|    ---)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestRShowsLoadingAndRereadsOnce
--- PASS: TestDuplicateRDroppedNotQueued
--- PASS: TestReentryDuringReloadDropped
--- PASS: TestFailedReloadReplacesContent
--- PASS: TestSecondConsecutiveReloadFailureAppends
--- PASS: TestReloadSuccessLeavesOverlayOpen
--- PASS: TestRWorksWithOneStopIndex
--- PASS: TestDiskChangeWithoutRIsStable
ok  	vrg/internal/app
```

## Anchor preservation and revision supersession

The anchor intent is recorded when the reload's load completes and commits only when the new revision's matching prepared layout installs — never against the old revision's. TestReloadPreservesAnchor: with the replacement layout held, the placeholder stays up and the saved viewport is untouched; on install the anchor lands on its row in the new rows — position preserved, cursor unmoved, no reveal. TestReloadAnchorClampedOnShrink: a shorter reread hits the lossy clamp — the top pulls to MaxTop and the anchor rewrites to the clamped top's location. TestPreReloadLayoutSupersededDiscarded: a layout prepared for the pre-reload revision and released after the reload completes is discarded by the Issue #17 key check without touching the installed model, viewport, anchor, or panel — then the new revision's layout installs and the anchor commits.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestReloadPreservesAnchor|TestReloadAnchorClampedOnShrink|TestPreReloadLayoutSupersededDiscarded' ./internal/app 2>&1 | grep -E '^(--- |ok|FAIL|    ---)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestReloadPreservesAnchor
--- PASS: TestReloadAnchorClampedOnShrink
--- PASS: TestPreReloadLayoutSupersededDiscarded
ok  	vrg/internal/app
```

## Full module regression

The reload route touches the key switch, the load pipeline, the failure path, the layout-install commit, and currentRows — the whole suite runs, plus the race detector over the gated worker tests.

```bash
cd /home/chris/vrg && go test -count=1 ./internal/filebuffer ./internal/viewport ./internal/app ./internal/safepresentation ./internal/searchindex ./internal/theme ./internal/cli ./cmd/vrg 2>&1 | sed -E 's/\t[0-9.]+s$//' && CGO_ENABLED=1 go test -race -count=1 ./internal/app 2>&1 | sed -E 's/\t[0-9.]+s$//'
```

```output
ok  	vrg/internal/filebuffer
ok  	vrg/internal/viewport
ok  	vrg/internal/app
ok  	vrg/internal/safepresentation
ok  	vrg/internal/searchindex
ok  	vrg/internal/theme
ok  	vrg/internal/cli
ok  	vrg/cmd/vrg
ok  	vrg/internal/app
```

## Manual check — the real binary on a pty

manual_route.sh runs the whole route as the unprivileged user against a disposable mktemp fixture — never a repository file: it copies the single-match fixture (fixture/solo.txt — 'needle solo 01' on line 1, so the index has exactly one stop and n/p are strict no-ops: r is the only retry route this session has) under a trap removing the directory on exit or interruption. pty_reload.py then drives vrg on a real 80x24 pty with stderr redirected to a file: the search opens the one-stop file (the whole session doubles as the single-match-search check), d d scrolls the anchor mid-file, lines are appended externally — no repaint even arrives, the cache is stable until r — then r reloads: the top row is the same text while the appended tail is now reachable. Deleting the file and pressing r shows '(unreadable)' under the 'cannot read …' overlay; restoring it and pressing r while the overlay is still open — the retry route the overlay cannot block — brings the content back behind the still-open prior-failure overlay; Esc dismisses; q exits 0 and the one failure replays to real stderr. The gated model tests above remain the authoritative deterministic verification.

```bash
cd /home/chris/vrg && go build -o Notes/walkthroughs/027-04/code-walkthrough/vrg ./cmd/vrg && bash Notes/walkthroughs/027-04/code-walkthrough/manual_route.sh
```

```output
fixture: <tmp> (solo.txt — one match, one stop)
startup    : one-stop file up — the session is the single-match search — '─ <tmp>/solo.txt ─────────────────────'
d d        : top row '…<tmp>/solo.txt23  pad solo 23' — the anchor is mid-file
append     : disk append — no repaint, display unchanged (cache stable until r)
r          : reload — top row still '…<tmp>/solo.txt23  pad solo 23' (anchor preserved)
pgdn…      : appended tail visible — the new revision is live
rm r       : deleted + r — 'cannot read …' over '(unreadable)'
r          : restored + r under the open overlay — content returns, overlay stays
Esc        : overlay dismissed — reloaded content remains
q          : exit 0 — nothing moved the fixed status
stderr     : cannot read <tmp>/solo.txt: no such file or directory
OK
```
