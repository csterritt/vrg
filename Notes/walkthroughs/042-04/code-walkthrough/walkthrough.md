# Issue #42: a dropped r request must not change revision or reveal intent

*2026-09-15T20:21:45Z by Showboat 0.6.1*
<!-- showboat-id: c5b0cb00-8e7b-4cf7-b0e4-51c11ab44eb1 -->

Walkthrough for Issue #42 (Notes/tasks/042-dropped-reload-no-intent-mutation.md): the one-load-per-path admission check and the reload-state mutation are now a single decision point in handleReload (internal/app/app.go). Previously r recorded reloadingPaths, switched to Loading… presentation, and set IntentReloadAnchor *before* startLoad dropped a duplicate request, so the in-flight startup or navigation load's completion was misclassified as a reload — an extra revision bump plus anchor preservation instead of the required destination reveal. Now a dropped r leaves revision, intent, and presentation exactly as they were; an accepted r applies the full reload contract precisely when the new request starts; and navigation re-entry is deliberately ungated. References: Notes/issues/042-dropped-reload-no-intent-mutation.md and Notes/PRD-vrg.md (File loading, cache, reload, and selection consistency; Navigation, viewport, and logical anchors).

Contracts verified:
- Admission tests: a dropped r during the startup load leaves IntentReveal/loading/call-count untouched and the completion performs the story-50 first-match reveal with no revision bump; a dropped r during a navigation load preserves the pending destination reveal per the latest-target rules; an accepted r applies flags, Loading…, IntentReloadAnchor, and exactly one revision increment; rapid r presses keep at most one reload in flight with the placeholder->content signal; navigation re-entry still updates selection, placeholder, and IntentReveal.
- Manual PTY scenario with a genuinely slow file (a FIFO whose read blocks until a writer supplies content): press r twice while the startup load is in flight -> nothing visible changes; when the load completes, line 40's match is revealed (offset ~27, top of file off-screen) — not anchor-preserved as if a reload had happened; a subsequent accepted r shows Loading… and completes to fresh content.

All generated artifacts live in this directory.

```bash
cd /home/chris/vrg && go build ./... && go vet ./... && echo GATES-OK
```

```output
GATES-OK
```

## Admission tests

internal/app/reload_admission_test.go proves the contract through Update/View only (no sleeps): TestDroppedReloadDuringStartupLoadPreservesReveal presses r while the startup load is gated in flight — LoadIntent stays IntentReveal, the loader call count does not grow, and the completion keeps LayoutKey().Revision at 1 and reveals the line-40 match at offset 27 rather than preserving the top-of-file anchor. TestDroppedReloadDuringNavigationLoadPreservesReveal presses r during a gated navigation load, then navigates same-file to prove the completion reveals the latest selected target (line 40) with no revision bump. TestAcceptedReloadBumpsRevisionExactlyOnce locks the accepted path: flags, Loading…, IntentReloadAnchor, exactly one reread, exactly one revision increment. TestRapidReloadPressesKeepOneReloadInFlight holds three rapid presses to one in-flight reload with the placeholder->content signal intact. TestNavigationReentryDuringInFlightLoadStillReveals proves the restriction applies only to handleReload: re-entry still updates selection, placeholder, and IntentReveal.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/app/ -run '^(TestDroppedReloadDuringStartupLoadPreservesReveal|TestDroppedReloadDuringNavigationLoadPreservesReveal|TestAcceptedReloadBumpsRevisionExactlyOnce|TestRapidReloadPressesKeepOneReloadInFlight|TestNavigationReentryDuringInFlightLoadStillReveals)$' -timeout 120s | sed 's/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'; echo test-exit=$?
```

```output
=== RUN   TestDroppedReloadDuringStartupLoadPreservesReveal
--- PASS: TestDroppedReloadDuringStartupLoadPreservesReveal (0.00s)
=== RUN   TestDroppedReloadDuringNavigationLoadPreservesReveal
--- PASS: TestDroppedReloadDuringNavigationLoadPreservesReveal (0.00s)
=== RUN   TestAcceptedReloadBumpsRevisionExactlyOnce
--- PASS: TestAcceptedReloadBumpsRevisionExactlyOnce (0.00s)
=== RUN   TestRapidReloadPressesKeepOneReloadInFlight
--- PASS: TestRapidReloadPressesKeepOneReloadInFlight (0.00s)
=== RUN   TestNavigationReentryDuringInFlightLoadStillReveals
--- PASS: TestNavigationReentryDuringInFlightLoadStillReveals (0.00s)
PASS
ok  	vrg/internal/app
test-exit=0
```

```bash
cd /home/chris/vrg && go test -count=1 ./internal/app/ -timeout 300s | sed 's/[[:space:]][0-9.]*s$//'; echo suite-exit=$?
```

```output
ok  	vrg/internal/app
suite-exit=0
```

## Manual scenario: r during a genuinely slow startup load

genfiles.py creates demo/slow.txt as a FIFO (named pipe) and fakerg/rg — a fake ripgrep that reports one match at line 40 of slow.txt, touches a handshake, and exits 0. The FIFO is the slow-loading-file seam: vrg's os.ReadFile blocks on open until a writer supplies content, so the startup load stays in flight as long as we need. runpty_reload.py drives the freshly built vrg under a PTY at 80x24: it waits for the Loading… placeholder, presses r twice while the read is still blocked (a dropped r must not change anything visible), then writes the 50-line content so the load completes — under the original classification the line-40 match must be revealed (viewport scrolled, top of file off-screen), not anchor-preserved as if a reload had happened. It then presses r again — now accepted — watches Loading… reappear, and supplies fresh content to prove the placeholder->content completion signal.

```bash
cd /home/chris/vrg/Notes/walkthroughs/042-04/code-walkthrough && uv run genfiles.py && go -C /home/chris/vrg build -o $PWD/demo/vrg ./cmd/vrg && test -x demo/vrg && echo binary-ready && echo build-exit=$?
```

```output
created demo/slow.txt (FIFO) and fakerg/rg (match at line 40, exit 0)
binary-ready
build-exit=0
```

```bash
cd /home/chris/vrg/Notes/walkthroughs/042-04/code-walkthrough && VRG_FAKE_DIR=$PWD/fakerg VRG_HANDSHAKE=$PWD/demo/handshake uv run --with pyte runpty_reload.py demo/vrg demo; echo run-exit=$?
```

```output
startup load in flight: 'Loading…' placeholder shown, FIFO blocks the read
dropped r presses: frame unchanged and still Loading… = True
completion: MATCH-TARGET visible, line-01 off-screen, Loading gone = True
accepted r: Loading… presentation applied = True
reload completion: RELOADED-TARGET visible, Loading gone = True
verdict: PASS
exit=0
run-exit=0
```
