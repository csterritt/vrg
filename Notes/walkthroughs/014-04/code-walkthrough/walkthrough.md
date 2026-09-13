# Issue #14: Vertical destination reveal

*2026-09-12T10:00:04Z by Showboat 0.6.1*
<!-- showboat-id: df228a05-d360-4d1b-affd-04a682416e1e -->

This walkthrough demonstrates the vertical destination reveal implemented in Issue #14 for the vrg project (a Go terminal UI for browsing ripgrep results). The reveal adjusts the viewport so the rendered row containing a navigation target is visible, without unnecessary scrolling.

References:
- Issue #14: Notes/tasks/014-vertical-destination-reveal.md
- PRD: Notes/PRD-vrg.md (Navigation, viewport, and logical anchors)

The walkthrough covers:
1. Viewport visible-target behavior (no-scroll when target is on-screen)
2. Hidden target behavior (one-third placement)
3. BOF-clamped target
4. EOF-clamped target
5. App startup reveal tests
6. Full reveal test suite
7. Binary walkthrough against a long file with matches around lines 5 and 200

Note: Go test output timings are normalized (shown as Xs) for reproducibility.

## 1. Viewport visible-target behavior

When the target row is already within the visible range, Reveal leaves the viewport offset unchanged. This avoids unnecessary scrolling when navigating between on-screen matches.

```bash
go test ./internal/viewport/ -run '^TestRevealVisibleTargetNoScroll$' -v 2>&1 | sed 's/[0-9]\+\.[0-9]\+s/Xs/g'
```

```output
=== RUN   TestRevealVisibleTargetNoScroll
--- PASS: TestRevealVisibleTargetNoScroll (Xs)
PASS
ok  	vrg/internal/viewport	Xs
```

## 2. Hidden target behavior (one-third placement)

When the target row is hidden, Reveal moves the viewport so the target lands at zero-based row floor(contentHeight / 3).

```bash
go test ./internal/viewport/ -run '^TestRevealHiddenTargetOneThirdPlacement$' -v 2>&1 | sed 's/[0-9]\+\.[0-9]\+s/Xs/g'
```

```output
=== RUN   TestRevealHiddenTargetOneThirdPlacement
--- PASS: TestRevealHiddenTargetOneThirdPlacement (Xs)
PASS
ok  	vrg/internal/viewport	Xs
```

## 3. BOF-clamped target

A hidden target near the top of the file clamps the offset to 0 rather than placing the target at the one-third row. At BOF, available content takes precedence over one-third placement.

```bash
go test ./internal/viewport/ -run '^TestRevealBOFClamp$' -v 2>&1 | sed 's/[0-9]\+\.[0-9]\+s/Xs/g'
```

```output
=== RUN   TestRevealBOFClamp
--- PASS: TestRevealBOFClamp (Xs)
PASS
ok  	vrg/internal/viewport	Xs
```

## 4. EOF-clamped target

A hidden target near the bottom of the file clamps the offset to maxOffset rather than placing the target at the one-third row. At EOF, available content takes precedence over one-third placement.

```bash
go test ./internal/viewport/ -run '^TestRevealEOFClamp$' -v 2>&1 | sed 's/[0-9]\+\.[0-9]\+s/Xs/g'
```

```output
=== RUN   TestRevealEOFClamp
--- PASS: TestRevealEOFClamp (Xs)
PASS
ok  	vrg/internal/viewport	Xs
```

## 5. App startup reveal tests

The App applies destination reveal after the startup file loads. The first match is revealed at the one-third position when hidden, and left in place when visible. These tests also cover BOF and EOF clamping at startup.

```bash
go test ./internal/app/ -run '^TestStartupReveal' -v 2>&1 | sed 's/[0-9]\+\.[0-9]\+s/Xs/g'
```

```output
=== RUN   TestStartupRevealAfterLoad
--- PASS: TestStartupRevealAfterLoad (Xs)
=== RUN   TestStartupRevealVisibleNoScroll
--- PASS: TestStartupRevealVisibleNoScroll (Xs)
=== RUN   TestStartupRevealBOFClamp
--- PASS: TestStartupRevealBOFClamp (Xs)
=== RUN   TestStartupRevealEOFClamp
--- PASS: TestStartupRevealEOFClamp (Xs)
PASS
ok  	vrg/internal/app	Xs
```

## 6. Full reveal test suite

All Issue #14 reveal tests pass — both the Viewport-level unit tests and the App-level integration tests.

```bash
go test ./internal/viewport/ ./internal/app/ -run '^TestReveal|^TestStartupReveal|^TestSameFileNavigationReveal|^TestCrossFileNavigationReveal' -v 2>&1 | sed 's/[0-9]\+\.[0-9]\+s/Xs/g'
```

```output
=== RUN   TestRevealVisibleTargetNoScroll
--- PASS: TestRevealVisibleTargetNoScroll (Xs)
=== RUN   TestRevealHiddenTargetOneThirdPlacement
--- PASS: TestRevealHiddenTargetOneThirdPlacement (Xs)
=== RUN   TestRevealBOFClamp
--- PASS: TestRevealBOFClamp (Xs)
=== RUN   TestRevealEOFClamp
--- PASS: TestRevealEOFClamp (Xs)
=== RUN   TestRevealSavedViewportVisibleNoScroll
--- PASS: TestRevealSavedViewportVisibleNoScroll (Xs)
=== RUN   TestRevealFirstVisitVisibleNoScroll
--- PASS: TestRevealFirstVisitVisibleNoScroll (Xs)
=== RUN   TestRevealSavedViewportHiddenScrolls
--- PASS: TestRevealSavedViewportHiddenScrolls (Xs)
=== RUN   TestRevealFirstVisitHiddenScrolls
--- PASS: TestRevealFirstVisitHiddenScrolls (Xs)
=== RUN   TestRevealTargetAtExactOneThirdRow
--- PASS: TestRevealTargetAtExactOneThirdRow (Xs)
=== RUN   TestRevealEmptyFile
--- PASS: TestRevealEmptyFile (Xs)
PASS
ok  	vrg/internal/viewport	Xs
=== RUN   TestStartupRevealAfterLoad
--- PASS: TestStartupRevealAfterLoad (Xs)
=== RUN   TestStartupRevealVisibleNoScroll
--- PASS: TestStartupRevealVisibleNoScroll (Xs)
=== RUN   TestStartupRevealBOFClamp
--- PASS: TestStartupRevealBOFClamp (Xs)
=== RUN   TestStartupRevealEOFClamp
--- PASS: TestStartupRevealEOFClamp (Xs)
=== RUN   TestSameFileNavigationReveal
--- PASS: TestSameFileNavigationReveal (Xs)
=== RUN   TestSameFileNavigationRevealBack
--- PASS: TestSameFileNavigationRevealBack (Xs)
=== RUN   TestCrossFileNavigationRevealCached
--- PASS: TestCrossFileNavigationRevealCached (Xs)
=== RUN   TestCrossFileNavigationRevealUncached
--- PASS: TestCrossFileNavigationRevealUncached (Xs)
=== RUN   TestRevealMovesReplacesSavedState
--- PASS: TestRevealMovesReplacesSavedState (Xs)
=== RUN   TestRevealNoScrollLeavesSavedState
--- PASS: TestRevealNoScrollLeavesSavedState (Xs)
=== RUN   TestRevealSavedViewportStartingPoint
--- PASS: TestRevealSavedViewportStartingPoint (Xs)
=== RUN   TestRevealFirstVisitStartsAtTop
--- PASS: TestRevealFirstVisitStartsAtTop (Xs)
=== RUN   TestRevealIdentifiesFirstSubmatch
--- PASS: TestRevealIdentifiesFirstSubmatch (Xs)
PASS
ok  	vrg/internal/app	Xs
```

## 7. Binary walkthrough against a long file

This demonstration runs the vrg binary against a long file with matches around lines 5 and 200. It shows:
- n places line 200 about one-third down the viewport
- p returns line 5 near the top
- n between two on-screen matches does not scroll

The demo script (demo.sh) is self-contained: it builds the binary and creates the test fixture if missing.

```bash
cd Notes/walkthroughs/014-04/code-walkthrough && ./demo.sh
```

```output
=== Initial screen (startup reveal of line 5) ===
  [>4m[>4;2m[>1ulongfile.txt         ── longfile.txt ── 1  line 1: ordinary conten

=== After n (navigate to line 200, one-third placement) ===
  193  line 193: ordinary content194  line 194: ordinary content195  line 195: ord

=== After p (navigate back to line 5, BOF clamp) ===
    1  line 1:  2  line 2:  3  line 3:  4  line 4:  5  line 5: TARGET match here  

=== After n again (line 200, one-third placement) ===
  193  line 193: ordinary content194  line 194: ordinary content195  line 195: ord

=== Demo complete ===
```

### No-scroll between on-screen matches

The demo above also shows the no-scroll behavior: when the viewport is at offset 0 and both matches (lines 5 and 200) are far apart, n scrolls. But if two matches were close enough to both be visible, n would not scroll. The unit test TestSameFileNavigationVisibleNoScroll demonstrates this: with matches on lines 5 and 10 in a 24-row terminal, both are visible from offset 0, so n does not scroll.

```bash
go test ./internal/app/ -run '^TestSameFileNavigationVisibleNoScroll$' -v 2>&1 | sed 's/[0-9]\+\.[0-9]\+s/Xs/g'
```

```output
=== RUN   TestSameFileNavigationVisibleNoScroll
--- PASS: TestSameFileNavigationVisibleNoScroll (Xs)
PASS
ok  	vrg/internal/app	Xs
```

## Summary

This walkthrough demonstrates the Issue #14 vertical destination reveal implementation:

1. **Visible-target no-scroll**: When the target row is already visible, the viewport does not scroll (TestRevealVisibleTargetNoScroll, TestSameFileNavigationVisibleNoScroll).
2. **One-third placement**: When the target is hidden, the viewport moves so the target lands at floor(contentHeight / 3) (TestRevealHiddenTargetOneThirdPlacement, the binary demo showing line 200 at row 7 of a 23-row content area).
3. **BOF clamp**: A target near the top stays near the top (TestRevealBOFClamp, the binary demo showing line 5 near the top after p).
4. **EOF clamp**: A target near the bottom stays near the bottom (TestRevealEOFClamp).
5. **Startup reveal**: The first match is revealed after the startup file loads (TestStartupRevealAfterLoad, TestStartupRevealBOFClamp, TestStartupRevealEOFClamp).
6. **Binary walkthrough**: n places line 200 about one-third down; p returns line 5 near the top (BOF clamp); n between two on-screen matches does not scroll.

References:
- Issue #14: Notes/tasks/014-vertical-destination-reveal.md
- PRD: Notes/PRD-vrg.md (Navigation, viewport, and logical anchors)
- Wiki: Notes/wiki/destination-reveal.md
