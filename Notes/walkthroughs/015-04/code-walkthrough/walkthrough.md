# Issue #15: File-change pop-up

*2026-09-23T22:41:54Z by Showboat 0.6.1*
<!-- showboat-id: b945490b-9c54-4b16-ae1a-e7d358729cbc -->

Walkthrough for [Issue #15](../../../issues/015-file-change-popup.md), implementing the file-change pop-up per `Notes/PRD-vrg.md` (*Colours, overlays, and key precedence* — the pop-up bullet and the error-over-pop-up precedence, *File loading, cache, reload, and selection consistency* — the pop-up starts at selection, and *Navigation, viewport, and logical anchors* — the one-stop no-op). When `n`/`p` navigation lands the cursor in a different file (`searchindex.Step.FileChanged`), a centred single-line box flashes the destination's `present.Path`-escaped name for one second or until any key — a key that dismisses it still performs its normal action in the same update. Every pop-up mints a fresh instance ID and its `tea.Tick` expiry carries that ID, so a stale timer can never dismiss a newer pop-up. Centring and left-truncation (leading `…`) are computed from the live terminal size at every render, so resize recentres and re-truncates without restarting the timer, and an error overlay cancels the pop-up permanently. All generated artifacts live in this directory: the built `vrg` binary, the `demo-popup.sh` tmux harness, and its `popup/` session captures. Test durations and measured timings are normalized so the document verifies cleanly.

```bash
cd /home/chris/vrg && go vet ./... && go build ./... && go test -count=1 ./... | sed "s/[[:space:]][0-9.]*s$//" && echo GATES-OK
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

## Instance-keyed model tests

`internal/app/popup_test.go` drives the lifecycle through `Update` with injected messages — no sleeps: `popupModel` lands a two-file browse with `instantPopupTimer`, an injected `popupTimer` seam whose command resolves to `popupExpiredMsg{id}` immediately, so `cmdMsgs` flattens a crossing's `tea.Batch` (load + expiry) synchronously. `TestPopupOpensCentredAtSelection` proves the pop-up starts at selection — the centred box is already up over `Loading…` and the returned batch carries the destination's `loadDoneMsg` plus the instance-keyed expiry — while a same-file `n` shows no box. `TestLoadCompletionDoesNotRestartPopup` shows the load completing beneath the same instance. `TestPopupInstanceKeyedExpiry` mints a second instance and proves the first's stale expiry leaves it alone until its own arrives. `TestPopupKeyDismissalStillActs`, `TestPopupDismissKeyStillNavigates`, and `TestPopupDismissalKeys` cover the any-key contract: `down` scrolls, `n` navigates into the next crossing's fresh pop-up, `q` quits, `Esc` dismisses only. `TestPopupRecentresOnResize` recentres the same instance on the new frame with no command returned. `TestPopupLeftTruncatesLongPath` checks the leading-`…` left-truncation at 80 columns and the untruncated form at 120. `TestErrorOverlayCancelsPopup` drives a current-file `loadDoneMsg` failure — the first slice of the Issue #26 read-failure rules — which opens the diagnostics overlay, kills the pop-up, and leaves it dead after dismissal. `TestNoPopupWithoutCrossing` pins the startup side. The shared sink-safety table's new `file-change pop-up` row (`renderPopupSink`) names the destination with each hostile fixture and asserts the `Path`-escaped form inside the box.

```bash
cd /home/chris/vrg && go test -count=1 -v -run "TestPopup|TestLoadCompletionDoesNotRestartPopup|TestFreshInstancesRejectStaleExpiry|TestResizeRecentresWithoutRestart|TestErrorOverlayCancelsPopup" ./internal/app 2>&1 | grep -vE "^(=== RUN|=== CONT|    --- (PASS|FAIL))" | sed -E "s/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//"
```

```output
--- PASS: TestPopupOpensCentredAtSelection
--- PASS: TestLoadCompletionDoesNotRestartPopup
--- PASS: TestPopupInstanceKeyedExpiry
--- PASS: TestPopupKeyDismissalStillActs
--- PASS: TestPopupDismissKeyStillNavigates
--- PASS: TestPopupDismissalKeys
--- PASS: TestPopupRecentresOnResize
--- PASS: TestPopupLeftTruncatesLongPath
--- PASS: TestErrorOverlayCancelsPopup
PASS
ok  	vrg/internal/app
```

```bash
cd /home/chris/vrg && go test -count=1 -v -run "TestNoPopupWithoutCrossing|TestSinkSafetyTable/file-change" ./internal/app 2>&1 | grep -vE "^(=== RUN|=== CONT)" | sed -E "s/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//"
```

```output
--- PASS: TestNoPopupWithoutCrossing
--- PASS: TestSinkSafetyTable
    --- PASS: TestSinkSafetyTable/file-change_pop-up/osc_title_set
    --- PASS: TestSinkSafetyTable/file-change_pop-up/csi_erase
    --- PASS: TestSinkSafetyTable/file-change_pop-up/c0_controls
    --- PASS: TestSinkSafetyTable/file-change_pop-up/c1_nel
    --- PASS: TestSinkSafetyTable/file-change_pop-up/delete
    --- PASS: TestSinkSafetyTable/file-change_pop-up/standalone_carriage_return
    --- PASS: TestSinkSafetyTable/file-change_pop-up/invalid_utf-8_path_bytes
    --- PASS: TestSinkSafetyTable/file-change_pop-up/embedded_filename_newline
PASS
ok  	vrg/internal/app
```

## Manual check — the pop-up on a real PTY

`demo-popup.sh` (checked into this directory) builds the real `vrg` and runs it on a real tmux PTY at 80x24. Its `fakebin/rg` reports five files in path order — `first.txt` (stops at lines 3 and 5), `mid.txt` (3), a hostile name carrying a real OSC payload (`nasty<ESC>]0;pwned<BEL>.txt`, stop at 1), an 88-`x` name (5), and `zzz.txt` — all real on-disk files so loads succeed. A same-file `n` opens no box; the crossing `n` to `mid.txt` flashes a centred 9-cell box (columns 36–44, rows 11–13) at selection time. A second `n` ~0.3s in crosses to the hostile file and opens instance 2 — rendered `nasty^[]0;pwned^G.txt` — while instance 1's timer is still pending; a resize to 100x30 recentres instance 2 (columns 39–61, rows 14–16) without restarting it, it survives instance 1's deadline (~t1+1000ms), and expires on its own ~1s clock. Back at 80x24, `n` to the 92-byte name shows the leading-`…` left-truncated interior filling the frame (box width 80, `x`=0), and `down` dismisses it and scrolls one row in the same update. Plain-text captures are stored under `popup/`; the timing line is normalized (`Nms`) for verification.

```bash
cd /home/chris/vrg/Notes/walkthroughs/015-04/code-walkthrough && ./demo-popup.sh | sed -E "s/[0-9]+ms/Nms/g"
```

```output
ok: startup: file panel -> first.txt
ok: startup: current match -> hit 00003
ok: startup: no pop-up
ok: same-file n: no pop-up
ok: crossing: file panel -> mid.txt
ok: crossing: pop-up text -> mid.txt
ok: crossing: box top -> ┌───────┐
ok: crossing: box middle -> │mid.txt│
ok: crossing: box bottom -> └───────┘
ok: hostile: file panel -> nasty^[]0;pwned^G.txt
ok: hostile: pop-up escaped -> nasty^[]0;pwned^G.txt
ok: hostile: box middle -> │nasty^[]0;pwned^G.txt│
ok: resized: recentred box -> │nasty^[]0;pwned^G.txt│
ok: resized: box top -> ┌─────────────────────┐
ok: stale expiry cannot dismiss instance 2
ok: instance 2 lifetime ~1s -> in-range
ok: outlived instance 1's deadline -> yes
timing: instance1 open->gone Nms, instance2 open->gone Nms
ok: truncated: leading ellipsis -> │…
ok: truncated: left border col 1 -> ┌
ok: truncated: right border col 80 -> ┐
ok: truncated: basename kept -> .txt
ok: down: dismissed -> no
ok: down: scrolled one row -> x000002
ok: exit status -> 0
demo-popup: all checks passed
```

```bash
cd /home/chris/vrg/Notes/walkthroughs/015-04/code-walkthrough && echo "--- pop-up over mid.txt, centred on the 80x24 frame (rows 11-13, cols 36-44) ---" && sed -n "10,13p" popup/screen-02-popup-mid.txt && echo "--- hostile name escaped: ESC -> ^[, BEL -> ^G ---" && sed -n "10,14p" popup/screen-03-popup-hostile.txt && echo "--- same instance recentred after resize to 100x30 (rows 14-16) ---" && sed -n "13,17p" popup/screen-04-popup-resized.txt && echo "--- 92-byte name left-truncated with a leading ellipsis at 80 cols ---" && sed -n "10,13p" popup/screen-06-popup-truncated.txt
```

```output
--- pop-up over mid.txt, centred on the 80x24 frame (rows 11-13, cols 36-44) ---
                                 9  x000009
                                10 ┌───────┐
                                   │mid.txt│
                                   └───────┘
--- hostile name escaped: ESC -> ^[, BEL -> ^G ---
                                 9  x000009
                            ┌─────────────────────┐
                            │nasty^[]0;pwned^G.txt│
                            └─────────────────────┘
                                13  x000013
--- same instance recentred after resize to 100x30 (rows 14-16) ---
                                        12  x000012
                                      ┌─────────────────────┐
                                      │nasty^[]0;pwned^G.txt│
                                      └─────────────────────┘
                                        16  x000016
--- 92-byte name left-truncated with a leading ellipsis at 80 cols ---
                                 9  x000009
┌──────────────────────────────────────────────────────────────────────────────┐
│…xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx.txt│
└──────────────────────────────────────────────────────────────────────────────┘
```

## Summary

The pop-up contract from Issue #15 is implemented and demonstrated end to end: opened at selection on every `FileChanged` navigation step and never at load completion; one second or any key, with the key's normal action still applied; instance-keyed expiry immune to stale timers; render-time centring and `…`-truncation that follow resizes without touching the timer; permanent cancellation by an error overlay; and every displayed byte routed through `present.Path`. Wiki coverage lives at `Notes/wiki/file-change-popup.md`, with the matching rows in `source-code.md`, `unit-tests.md`, `match-navigation.md`, `error-overlay-and-fatal-outcomes.md`, and `safe-presentation.md`. Deferred neighbours: the help overlay's pop-up cancellation (Issue #31), the combined overlay precedence matrix (Issue #32), and the rest of the read-failure overlay rules (Issue #26).
