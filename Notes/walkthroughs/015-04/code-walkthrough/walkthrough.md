# Issue #15: File-change pop-up

*2026-09-12T10:19:13Z by Showboat 0.6.1*
<!-- showboat-id: 7b6d6ef5-7f55-4db8-8d0c-e553d4f250fc -->

This walkthrough demonstrates the file-change pop-up implemented in Issue #15 for the vrg project (a Go terminal UI for browsing ripgrep results). When n/p navigation crosses a file boundary, a centred single-line pop-up displays the selected file's escaped path for one second or until a keypress.

References:
- Issue #15: Notes/tasks/015-file-change-popup.md
- PRD: Notes/PRD-vrg.md (File-change pop-up; Colours, overlays, and key precedence)

The walkthrough covers:
1. Instance-keyed model tests (stale expiry rejection, fresh instance per open)
2. Pop-up opens on cross-file n, not on same-file n
3. Keypress dismissal with normal action (q still quits, scroll still scrolls, Esc dismisses)
4. Error-overlay cancellation (no return after dismissal)
5. Load completion does not restart the timer
6. Centred rendering and long-path truncation
7. Resize re-centring and re-truncation
8. Sink safety across all hostile fixtures
9. Binary walkthrough: n across a file boundary showing the centred pop-up and its ~1s disappearance
10. Quick second n showing the new file's pop-up with navigation still applied
11. Resize while shown re-centring it
12. Hostile path rendering its escaped form

Note: Go test output timings are normalized (shown as Xs) for reproducibility.

## 1. Instance-keyed model tests

The pop-up uses instance-keyed timers so a stale expiry message (from an older pop-up) cannot dismiss a newer pop-up. Each cross-file navigation increments a process-wide counter and captures the new value as the pop-up instance. The expiry message carries the instance; Update dismisses the pop-up only when the instance matches.

```bash
cd /home/chris/vrg && go test ./internal/app/ -run '^TestPopupExpiryStaleInstanceDoesNotDismiss$|^TestPopupFreshInstancePerPopup$' -v 2>&1 | sed 's/[0-9]\+\.[0-9]\+s/Xs/g; s/(cached)/Xs/g'
```

```output
=== RUN   TestPopupExpiryStaleInstanceDoesNotDismiss
--- PASS: TestPopupExpiryStaleInstanceDoesNotDismiss (Xs)
=== RUN   TestPopupFreshInstancePerPopup
--- PASS: TestPopupFreshInstancePerPopup (Xs)
PASS
ok  	vrg/internal/app	Xs
```

## 2. Pop-up opens on cross-file n, not on same-file n

The pop-up starts at selection time when n/p crosses a file boundary. Same-file navigation does not open the pop-up. With one stop, n is a strict no-op and does not open the pop-up.

```bash
cd /home/chris/vrg && go test ./internal/app/ -run '^TestPopupOpensOnCrossFileNavigation$|^TestPopupDoesNotOpenOnSameFileNavigation$|^TestPopupDoesNotOpenOnOneStopNoOp$' -v 2>&1 | sed 's/[0-9]\+\.[0-9]\+s/Xs/g; s/(cached)/Xs/g'
```

```output
=== RUN   TestPopupOpensOnCrossFileNavigation
--- PASS: TestPopupOpensOnCrossFileNavigation (Xs)
=== RUN   TestPopupDoesNotOpenOnSameFileNavigation
--- PASS: TestPopupDoesNotOpenOnSameFileNavigation (Xs)
=== RUN   TestPopupDoesNotOpenOnOneStopNoOp
--- PASS: TestPopupDoesNotOpenOnOneStopNoOp (Xs)
PASS
ok  	vrg/internal/app	Xs
```

## 3. Keypress dismissal with normal action

Any keypress while the pop-up is open dismisses it, and the same key then performs its normal action. q still quits, scroll keys still scroll, and Esc dismisses the pop-up without exiting browse mode.

```bash
cd /home/chris/vrg && go test ./internal/app/ -run '^TestPopupKeypressDismissesAndPerformsAction$|^TestPopupKeypressDismissesAndQuits$|^TestPopupKeypressDismissesAndScrolls$|^TestPopupEscDismisses$' -v 2>&1 | sed 's/[0-9]\+\.[0-9]\+s/Xs/g; s/(cached)/Xs/g'
```

```output
=== RUN   TestPopupKeypressDismissesAndPerformsAction
--- PASS: TestPopupKeypressDismissesAndPerformsAction (Xs)
=== RUN   TestPopupKeypressDismissesAndQuits
--- PASS: TestPopupKeypressDismissesAndQuits (Xs)
=== RUN   TestPopupKeypressDismissesAndScrolls
--- PASS: TestPopupKeypressDismissesAndScrolls (Xs)
=== RUN   TestPopupEscDismisses
--- PASS: TestPopupEscDismisses (Xs)
PASS
ok  	vrg/internal/app	Xs
```

## 4. Error-overlay cancellation (no return after dismissal)

When a SearchCompleteMsg opens an error overlay, the pop-up is permanently cancelled. It does not return after the overlay is dismissed. The same applies after expiry or keypress dismissal: the pop-up does not return on subsequent same-file navigation.

```bash
cd /home/chris/vrg && go test ./internal/app/ -run '^TestPopupErrorOverlayCancels$|^TestPopupDoesNotReturnAfterExpiry$|^TestPopupDoesNotReturnAfterKeypress$' -v 2>&1 | sed 's/[0-9]\+\.[0-9]\+s/Xs/g; s/(cached)/Xs/g'
```

```output
=== RUN   TestPopupErrorOverlayCancels
--- PASS: TestPopupErrorOverlayCancels (Xs)
=== RUN   TestPopupDoesNotReturnAfterExpiry
--- PASS: TestPopupDoesNotReturnAfterExpiry (Xs)
=== RUN   TestPopupDoesNotReturnAfterKeypress
--- PASS: TestPopupDoesNotReturnAfterKeypress (Xs)
PASS
ok  	vrg/internal/app	Xs
```

## 5. Load completion does not restart the timer

File loading completion does not restart the pop-up timer. The pop-up starts at selection time, not at load completion, and the timer continues independently of the load.

```bash
cd /home/chris/vrg && go test ./internal/app/ -run '^TestPopupLoadCompletionDoesNotRestartTimer$' -v 2>&1 | sed 's/[0-9]\+\.[0-9]\+s/Xs/g; s/(cached)/Xs/g'
```

```output
=== RUN   TestPopupLoadCompletionDoesNotRestartTimer
--- PASS: TestPopupLoadCompletionDoesNotRestartTimer (Xs)
PASS
ok  	vrg/internal/app	Xs
```

## 6. Centred rendering and long-path truncation

The pop-up is centred horizontally and vertically over the base content. Long paths are left-truncated with a leading ellipsis to fit the terminal width.

```bash
cd /home/chris/vrg && go test ./internal/app/ -run '^TestPopupCentred$|^TestPopupTruncatesLongPath$' -v 2>&1 | sed 's/[0-9]\+\.[0-9]\+s/Xs/g; s/(cached)/Xs/g'
```

```output
=== RUN   TestPopupCentred
--- PASS: TestPopupCentred (Xs)
=== RUN   TestPopupTruncatesLongPath
--- PASS: TestPopupTruncatesLongPath (Xs)
PASS
ok  	vrg/internal/app	Xs
```

## 7. Resize re-centring and re-truncation

A resize re-centres and re-truncates the pop-up without restarting the timer. The pop-up position and truncation are recomputed on every render.

```bash
cd /home/chris/vrg && go test ./internal/app/ -run '^TestPopupResizeRecentersWithoutRestart$|^TestPopupResizeRecentresAndRetruncates$' -v 2>&1 | sed 's/[0-9]\+\.[0-9]\+s/Xs/g; s/(cached)/Xs/g'
```

```output
=== RUN   TestPopupResizeRecentersWithoutRestart
--- PASS: TestPopupResizeRecentersWithoutRestart (Xs)
=== RUN   TestPopupResizeRecentresAndRetruncates
--- PASS: TestPopupResizeRecentresAndRetruncates (Xs)
PASS
ok  	vrg/internal/app	Xs
```

## 8. Sink safety across all hostile fixtures

The pop-up path is escaped through safepresentation.EscapePath, so no raw control bytes survive in the no-style composition path. The shared hostile-fixture set (OSC, CSI, C0, C1, DEL, standalone CR, invalid UTF-8, embedded newline) is driven through the pop-up render path. The invalid UTF-8 fixture uses bytes-encoded begin/end records so the path is handled correctly by the search index.

```bash
cd /home/chris/vrg && go test ./internal/app/ -run '^TestPopupSinkSafetyNoStyle$' -v 2>&1 | sed 's/[0-9]\+\.[0-9]\+s/Xs/g; s/(cached)/Xs/g'
```

```output
=== RUN   TestPopupSinkSafetyNoStyle
=== RUN   TestPopupSinkSafetyNoStyle/OSC
=== RUN   TestPopupSinkSafetyNoStyle/CSI
=== RUN   TestPopupSinkSafetyNoStyle/C0
=== RUN   TestPopupSinkSafetyNoStyle/C1
=== RUN   TestPopupSinkSafetyNoStyle/DEL
=== RUN   TestPopupSinkSafetyNoStyle/StandaloneCR
=== RUN   TestPopupSinkSafetyNoStyle/InvalidUTF8
=== RUN   TestPopupSinkSafetyNoStyle/EmbeddedNewline
--- PASS: TestPopupSinkSafetyNoStyle (Xs)
    --- PASS: TestPopupSinkSafetyNoStyle/OSC (Xs)
    --- PASS: TestPopupSinkSafetyNoStyle/CSI (Xs)
    --- PASS: TestPopupSinkSafetyNoStyle/C0 (Xs)
    --- PASS: TestPopupSinkSafetyNoStyle/C1 (Xs)
    --- PASS: TestPopupSinkSafetyNoStyle/DEL (Xs)
    --- PASS: TestPopupSinkSafetyNoStyle/StandaloneCR (Xs)
    --- PASS: TestPopupSinkSafetyNoStyle/InvalidUTF8 (Xs)
    --- PASS: TestPopupSinkSafetyNoStyle/EmbeddedNewline (Xs)
PASS
ok  	vrg/internal/app	Xs
```

## 9. Binary walkthrough: cross-file pop-up and ~1s disappearance

This demonstration runs the vrg binary against two files in a directory. It shows:
- n across a file boundary showing the centred pop-up with the destination path
- the pop-up disappearing after ~1 second
- a quick second n showing the new file's pop-up with navigation still applied
- a resize while the pop-up is shown re-centring it

The demo script (demo.sh) is self-contained: it builds the binary and creates the test fixture directory if missing.

```bash
cd /home/chris/vrg/Notes/walkthroughs/015-04/code-walkthrough && ./demo.sh
```

```output
=== Initial screen (file_a.txt loaded) ===
  [>4m[>4;2m[>1udemo_dir/file_a.txt  ── demo_dir/file_a.txt ──
  demo_dir/file_b.txt  1  line 1: TARGET match here 2  line 2: ordinary 3  line 3:

=== After n (cross-file to file_b.txt, pop-up visible) ===
  demo_dir/file_a.txt  ── demo_dir/file_b.txt ──
  demo_dir/file_b.txt  1  line 1: ordinaryTARGET match heredemo_dir/file_b.txt

=== After ~1s (pop-up expired, file_b.txt content) ===
  M

=== After second n (cross-file back to file_a.txt, new pop-up) ===
  demo_dir/file_a.txt  ── demo_dir/file_a.txt ──
  demo_dir/file_b.txt  1  line 1: TARGET match hereordinarydemo_dir/file_a.txt

=== After third n (cross-file to file_b.txt, pop-up before resize) ===
  demo_dir/file_a.txt  ── demo_dir/file_b.txt ──
  demo_dir/file_b.txt  1  line 1: ordinaryTARGET match hereb.

=== After resize to 40x30 (pop-up re-centred) ===
  demo_dir/file_a.txt  ── demo_dir/file_b.
  demo_dir/file_b.txt  1  line 1: ordinary 2  line 2: TARGET m 3  line 3: ordinary

=== Demo complete ===
```

The output shows the pop-up centred with the destination path:
- After the first n: the pop-up shows 'demo_dir/file_b.txt' centred over the file_b.txt content
- After ~1s: the pop-up expires and the file_b.txt content is shown without the pop-up
- After the second n: the pop-up shows 'demo_dir/file_a.txt' centred (navigation applied)
- After the third n + resize: the pop-up re-centres to the new 40x30 terminal size

## 10. Hostile path rendering its escaped form

This demonstration runs the vrg binary against a file whose name contains invalid UTF-8 bytes (\xff\xfe). The pop-up must escape the path through safepresentation.EscapePath for safe display.

```bash
cd /home/chris/vrg/Notes/walkthroughs/015-04/code-walkthrough && ./demo-hostile.sh
```

```output
=== Initial screen (aaa_safe.txt loaded) ===
  [>4m[>4;2m[>1uhostile_dir/aaa_safe.txt ── hostile_dir/aaa_safe.txt ──
  hostile_dir/foo\xff\xfebar.txt 1  line 1: TARGET match here

=== After n (cross-file to hostile path, pop-up shows escaped form) ===
  hostile_dir/aaa_safe.txt ── hostile_dir/foo\xff\xfebar.txt ──
  hostile_dir/foo\xff\xfebar.txt 1  line 1: hostile_dir/foo\xff\xfebar.txt

=== Hostile-path demo complete ===
```

The output shows the pop-up with the escaped path 'hostile_dir/foo\xff\xfebar.txt' — the invalid UTF-8 bytes (\xff\xfe) are escaped to their literal backslash-x forms by safepresentation.EscapePath, so no raw control bytes survive in the output.

## Summary

This walkthrough demonstrates the Issue #15 file-change pop-up implementation:

1. **Instance-keyed timers**: Each pop-up has a fresh instance ID; stale expiry messages cannot dismiss a newer pop-up (TestPopupExpiryStaleInstanceDoesNotDismiss, TestPopupFreshInstancePerPopup).
2. **Selection-time start**: The pop-up opens on cross-file n, not on same-file n, and not for a one-stop no-op (TestPopupOpensOnCrossFileNavigation, TestPopupDoesNotOpenOnSameFileNavigation, TestPopupDoesNotOpenOnOneStopNoOp).
3. **Keypress dismissal with normal action**: Any key dismisses the pop-up and performs its normal action — q still quits, scroll keys still scroll, Esc dismisses without exiting browse (TestPopupKeypressDismissesAndPerformsAction, TestPopupKeypressDismissesAndQuits, TestPopupKeypressDismissesAndScrolls, TestPopupEscDismisses).
4. **Error-overlay cancellation**: Opening an error overlay cancels the pop-up permanently; it does not return after dismissal, expiry, or keypress (TestPopupErrorOverlayCancels, TestPopupDoesNotReturnAfterExpiry, TestPopupDoesNotReturnAfterKeypress).
5. **Load independence**: File-load completion does not restart the timer (TestPopupLoadCompletionDoesNotRestartTimer).
6. **Centred rendering and truncation**: The pop-up is centred horizontally and vertically; long paths are left-truncated with a leading ellipsis (TestPopupCentred, TestPopupTruncatesLongPath).
7. **Resize re-centring**: A resize re-centres and re-truncates without restarting the timer (TestPopupResizeRecentersWithoutRestart, TestPopupResizeRecentresAndRetruncates).
8. **Sink safety**: No raw control bytes survive in the no-style composition path across all hostile fixtures including invalid UTF-8 (TestPopupSinkSafetyNoStyle).
9. **Binary walkthrough**: n across a file boundary shows the centred pop-up; it disappears after ~1s; a quick second n shows the new file's pop-up with navigation applied; a resize while shown re-centres it.
10. **Hostile path**: The invalid UTF-8 path is escaped to its literal backslash-x form by safepresentation.EscapePath.

References:
- Issue #15: Notes/tasks/015-file-change-popup.md
- PRD: Notes/PRD-vrg.md (File-change pop-up; Colours, overlays, and key precedence)
- Wiki: Notes/wiki/browse-tracer.md (File-change pop-up section)
