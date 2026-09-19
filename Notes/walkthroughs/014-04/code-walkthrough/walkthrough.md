# Issue #14: vertical destination reveal

*2026-09-17T13:07:42Z by Showboat 0.6.1*
<!-- showboat-id: 14fac9ca-dd8e-4d18-9355-648f413dcacc -->

Issue #14 adds the vertical destination reveal: startup and every actual n/p transition reveal the rendered row holding the destination's display target — the start cell of the first submatch on the matched line, not merely the source-line ordinal. An already-visible target row leaves the viewport unchanged; a hidden one lands at zero-based row floor(content height / 3), clamped to valid tops so BOF and EOF content take precedence over one-third placement. A file change starts the reveal from the saved per-file viewport on a revisit or the top of the file on a first visit — including the startup file; a moving reveal replaces the saved vertical state while a no-scroll reveal leaves it. Horizontal reveal is Issue #19's and the two-stage prepared-layout commit is Issue #28's. See Notes/issues/014-vertical-destination-reveal.md, Notes/tasks/014-vertical-destination-reveal.md, and the 'Navigation, viewport, and logical anchors' (target-row and placement bullets) and 'Testing Decisions' sections of Notes/PRD-vrg.md. All artifacts live in this directory.

## Gates — module integrity, build, vet

```bash
cd /home/chris/vrg && go mod verify && go build ./... && go vet ./... && echo GATES-OK
```

```output
all modules verified
GATES-OK
```

## Viewport — target identification and the reveal rules

internal/viewport/reveal_test.go covers the reveal contract over real prepared buffers: the display target's cell is the first submatch's start cell through the byte->cell map (an escaped byte widens, so the cell is not the byte offset) with the zero-width marker-cell fallback; TargetRow resolves the rendered row holding the target and clamps to the prepared rows; a visible target row leaves the viewport untouched; a hidden one moves the top to row - floor(h/3) from either direction; BOF and EOF clamps beat the one-third placement; and the reveal is relative to the viewport's current top — the same target that moves a first-visit top stays put when visible from a saved one.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'StopTarget|TargetRow|Reveal' ./internal/viewport 2>&1 | grep -E '^(--- |    --- |ok|FAIL)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestStopTargetIsFirstSubmatchStartCell
--- PASS: TestStopTargetZeroWidthLandsPastLastCell
--- PASS: TestTargetRow
--- PASS: TestRevealVisibleTargetNoScroll
--- PASS: TestRevealHiddenTargetOneThird
--- PASS: TestRevealBOFEOFClamps
--- PASS: TestRevealStartsFromCurrentTop
ok  	vrg/internal/viewport
```

## App — reveal triggers, starting viewport, saved state

internal/app/reveal_test.go drives n/p through Update over multi-stop fixtures on a 160x24 window (content height 23, one-third row 7): the startup file reveals after its load completes — a hidden first stop lands a third down while a visible one keeps the top and records no saved state; n reveals the hidden line-200 stop and p clamps the line-5 return at BOF; n between two on-screen stops moves only the cursor; a strict no-op step triggers no reveal; a moving reveal replaces the file's saved top so a later revisit resumes the revealed position, while a revisit's reveal starts from the saved top; a cached first visit reveals from the top; and navigating into an uncached file reveals when its load completes.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'Reveal|VisibleTargets|NoOpNavigation|MovingReveal|RevisitReveal|FirstVisit|UncachedFileReveals' ./internal/app 2>&1 | grep -E '^(--- |    --- |ok|FAIL)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestStartupRevealPlacesHiddenTarget
--- PASS: TestStartupRevealVisibleTargetStaysTop
--- PASS: TestNavigationRevealHiddenThenBOF
--- PASS: TestNavBetweenVisibleTargetsNoScroll
--- PASS: TestNoOpNavigationDoesNotReveal
--- PASS: TestMovingRevealReplacesSavedViewport
--- PASS: TestRevisitRevealStartsFromSavedViewport
--- PASS: TestFirstVisitStartsFromTopThenReveal
--- PASS: TestNavToUncachedFileRevealsOnLoad
ok  	vrg/internal/app
```

## Manual PTY — reveal on the real binary

pty_reveal.py builds a fixture — a.txt, 250 lines with matches on lines 5 and 200 only, and b.txt, 30 lines with matches on lines 3 and 8 — and runs the real binary on a 100x24 pty (content height 23, so the one-third row is content row 7, screen row 8). The raw byte stream is replayed through a small screen emulator that also tracks the SGR underline attribute, then the script asserts where the underline lands and which row tops the panel. Startup's first-visit reveal leaves the visible line-5 match at the top; n to the hidden line-200 match lands it at content row 7 — a third down — with top = line 193; p back to line 5 clamps to top 0 (row 4 - 7 < 0), putting the match near the top — BOF content beats one-third placement; the first visit to b.txt starts at its top where the line-3 target is already visible; and n between b.txt's two on-screen matches moves only the underline — the viewport does not scroll.

```bash
cd /home/chris/vrg && go build -o Notes/walkthroughs/014-04/code-walkthrough/vrg ./cmd/vrg && cd Notes/walkthroughs/014-04/code-walkthrough && python3 pty_reveal.py
```

```output
init      : file=a.txt list=a.txt underline=row 5: '5  match a-five' top='1  a row 01'
n         : file=a.txt list=a.txt underline=row 8: '200  match a-twohundred' top='193  a row 193'
p         : file=a.txt list=a.txt underline=row 5: '5  match a-five' top='1  a row 01'
n         : file=a.txt list=a.txt underline=row 8: '200  match a-twohundred' top='193  a row 193'
n->b      : file=b.txt list=b.txt underline=row 3: '3  match b-three' top='1  b row 01'
n         : file=b.txt list=b.txt underline=row 8: '8  match b-eight' top='1  b row 01'
q         : exit 0
OK
```

## Full suite — go test ./... and the race detector

```bash
cd /home/chris/vrg && go test -count=1 ./... 2>&1 | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//' && CGO_ENABLED=1 go test -count=1 -race ./internal/... ./cmd/vrg 2>&1 | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
?   	vrg/Notes/walkthroughs/007-04/code-walkthrough/fixture	[no test files]
ok  	vrg/cmd/vrg
ok  	vrg/internal/app
ok  	vrg/internal/cli
ok  	vrg/internal/filebuffer
ok  	vrg/internal/safepresentation
?   	vrg/internal/safepresentation/sinktest	[no test files]
ok  	vrg/internal/searchindex
ok  	vrg/internal/theme
ok  	vrg/internal/viewport
ok  	vrg/internal/app
ok  	vrg/internal/cli
ok  	vrg/internal/filebuffer
ok  	vrg/internal/safepresentation
?   	vrg/internal/safepresentation/sinktest	[no test files]
ok  	vrg/internal/searchindex
ok  	vrg/internal/theme
ok  	vrg/internal/viewport
ok  	vrg/cmd/vrg
```
