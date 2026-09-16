# Issue #41: non-help overlays scroll every wrapped row — no head/tail compression

*2026-09-15T20:06:37Z by Showboat 0.6.1*
<!-- showboat-id: 6a0da88c-306f-4527-998c-59fd5a7241bb -->

Walkthrough for Issue #41 (Notes/tasks/041-overlay-full-scroll-no-head-tail-compression.md): the non-help error/warning overlay no longer compresses a long diagnostic to head + ellipsis + tail before scrolling. Previously the omitted middle rows were unreachable; now the scrollable row set is the complete wrapped diagnostic and overlayScroll clamps to [0, max(0, rows−maxVisible)] in both the key handler and the render path. Appended diagnostics extend the set without moving the reader, the modal key contract is unchanged (u, d, PageUp, PageDown are still ignored while the overlay is open), and render-time clipping at tiny sizes remains the only permitted row loss — model-level elision is forbidden. References: Notes/issues/041-overlay-full-scroll-no-head-tail-compression.md and Notes/PRD-vrg.md (Colours, overlays, and key precedence; Outcome and exit-status contract).

Contracts verified:
- Model-level complete-row tests: the rendered window slides row by row over every wrapped row; no ellipsis row is ever injected; overlayScroll clamps at both ends.
- Bounded traversal: a 30-row diagnostic at 80x24 (maxVisible 20) scrolls to the last row in 10 downs and back to the first in 10 ups; a ≥1 MiB fixture-shaped diagnostic proves tail reachability for arbitrarily large input without PTY key floods.
- Appended diagnostics extend the scrollable set (30→31 rows, max scroll 10→11) while preserving the reader's position.
- The revised ≥1 MiB TestStderrContentFixture keeps drainage, complete-stdout, captured-stderr, and completion checks but drops the simultaneous head/tail rendering requirement (superseded by Issue #41).
- A manual PTY scenario: fatal outcome from a fake rg whose stderr is 60 rows — scroll from first row to last and back, u/d/PgUp/PgDn ignored, every middle row reachable, no ellipsis, q dismisses, clamps hold at both ends, exit 2.

All generated artifacts live in this directory.

```bash
cd /home/chris/vrg && go build ./... && go vet ./... && echo GATES-OK
```

```output
GATES-OK
```

## Model-level complete-row, clamp, and traversal tests

internal/app/overlay_full_scroll_test.go proves the contract through Update/View only (no sleeps): TestOverlayCompleteRowsScrollable slides a 20-row window over all 30 rows one down at a time and asserts the exact rendered rows plus both-end clamps; TestOverlayHugeDiagnosticCompleteRows drives a >=1 MiB fixture-shaped diagnostic (17,615 wrapped rows) through the full down/up traversal — the wrap cache keyed on (text, interior) keeps the 35,190 key presses fast; TestOverlayRenderClampsStaleScroll proves a stale position renders the last page and re-clamps on the next scroll key; TestOverlayAppendExtendsScrollableSet proves an appended diagnostic extends the set while preserving the reader position. TestOverlayKeyOtherIgnored now also covers u, d, page up, and page down.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/app/ -run '^(TestOverlayCompleteRowsScrollable|TestOverlayHugeDiagnosticCompleteRows|TestOverlayRenderClampsStaleScroll|TestOverlayAppendExtendsScrollableSet|TestOverlayKeyOtherIgnored)$' -timeout 120s | sed 's/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'; echo test-exit=$?
```

```output
=== RUN   TestOverlayCompleteRowsScrollable
--- PASS: TestOverlayCompleteRowsScrollable (0.00s)
=== RUN   TestOverlayHugeDiagnosticCompleteRows
--- PASS: TestOverlayHugeDiagnosticCompleteRows (0.00s)
=== RUN   TestOverlayRenderClampsStaleScroll
--- PASS: TestOverlayRenderClampsStaleScroll (0.00s)
=== RUN   TestOverlayAppendExtendsScrollableSet
--- PASS: TestOverlayAppendExtendsScrollableSet (0.00s)
=== RUN   TestOverlayKeyOtherIgnored
--- PASS: TestOverlayKeyOtherIgnored (0.00s)
PASS
ok  	vrg/internal/app
test-exit=0
```

## Revised large-stderr PTY fixture

TestStderrContentFixture (cmd/vrg/outcome_test.go) still drives a fake rg that writes >=1 MiB to stderr interleaved with valid stdout records: it asserts the stream drains fully, the stdout stream is complete (the browse view appears), the captured stderr reaches the overlay (the head is visible at scroll 0), and the process completes with exit 2. Issue #41 supersedes the old simultaneous head/tail assertion: the fixture now scrolls a few rows and dismisses with q twice — a bare Esc+q can coalesce into Alt+q while an expensive update is in flight. Tail reachability is covered by the model-level tests above rather than thousands of PTY keys.

```bash
cd /home/chris/vrg && go test -count=1 -v ./cmd/vrg/ -run '^TestStderrContentFixture$' -timeout 120s | sed 's/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'; echo test-exit=$?
```

```output
=== RUN   TestStderrContentFixture
--- PASS: TestStderrContentFixture (0.00s)
PASS
ok  	vrg/cmd/vrg
test-exit=0
```

## Manual scenario: fatal outcome, 60-row stderr, first row to last and back

genfiles.py creates demo/test.txt and fakerg/rg — a fake ripgrep that emits FIRST-MARKER, 58 numbered diag-row-NNN stderr lines, and LAST-MARKER (60 wrapped rows vs the 20-row visible overlay at 80x24), valid stdout JSON for one match, then exit 3 (fatal outcome over the browse view). runpty_scroll.py drives the freshly built vrg under a PTY: it waits for the error overlay, presses down one row at a time until LAST-MARKER is visible (exactly rows−maxVisible = 40 presses), confirms the frame stops changing under further downs, sends u, d, PageUp, and PageDown (all ignored — the frame never moves), traverses back up 40 rows to FIRST-MARKER, confirms the top clamp, dismisses with q, and exits with a second q. Every one of the 58 middle rows is observed on screen during traversal and no ellipsis row ever appears.

```bash
cd /home/chris/vrg/Notes/walkthroughs/041-04/code-walkthrough && python3 genfiles.py && go -C /home/chris/vrg build -o Notes/walkthroughs/041-04/code-walkthrough/vrg ./cmd/vrg && echo BUILD-OK
```

```output
created demo/test.txt and fakerg/rg (60 stderr rows, exit 3)
BUILD-OK
```

```bash
cd /home/chris/vrg/Notes/walkthroughs/041-04/code-walkthrough && VRG_FAKE_DIR=/home/chris/vrg/Notes/walkthroughs/041-04/code-walkthrough/fakerg VRG_HANDSHAKE=/tmp/hs_041_walk timeout 120 uv run --with pyte python3 runpty_scroll.py ./vrg demo; echo run-exit=$?
```

```output
overlay open: head row 'FIRST-MARKER' visible at scroll 0
down traversal: 40 presses, LAST-MARKER visible=True
bottom clamp: extra downs leave frame unchanged = True
ignored keys (u d PgUp PgDn): frame unchanged = True
up traversal: 40 presses, FIRST-MARKER visible=True
top clamp: extra ups leave frame unchanged = True
q dismisses overlay: browse restored = True
rows seen while traversing: 58/58 middle rows, ellipsis row seen: False
verdict: PASS
exit=2
run-exit=0
```

Conclusion: every contract is satisfied — the scrollable row set is the complete wrapped diagnostic, middle rows are reachable by row-wise traversal, overlayScroll clamps to [0, max(0, rows−maxVisible)] in both the key handler and the render path, appended diagnostics extend the set without moving the reader, the modal key contract is unchanged (u/d/PageUp/PageDown ignored while the overlay is open), no ellipsis row is injected for non-help overlays, and the large-stderr PTY fixture keeps drainage/stdout/stderr/completion checks without the superseded simultaneous head/tail requirement. Wiki documentation at Notes/wiki/overlay-full-scroll.md plus updated pages (outcome-contract, help-overlay, source-code, unit-tests, overlay-precedence, index).

