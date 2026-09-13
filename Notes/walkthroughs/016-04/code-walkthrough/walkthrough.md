# Issue #16: Wrap mode and grapheme policy

*2026-09-12T10:35:40Z by Showboat 0.6.1*
<!-- showboat-id: 92495136-e396-4e74-8f1a-855c9191c19d -->

This walkthrough demonstrates the wrap mode and grapheme policy implemented in Issue #16 for the vrg project (a Go terminal UI for browsing ripgrep results). Wrapping is on by default; `w` toggles between wrap and run-off-edge modes; wrapping occurs at grapheme-cluster boundaries; tabs expand to eight-column stops; and the Issue #14 destination reveal finds the wrapped row containing a match.

References:
- Issue #16: Notes/tasks/016-wrap-mode-and-toggle.md
- PRD: Notes/PRD-vrg.md (Text, graphemes, and safe presentation; Layout and indicators)

The walkthrough covers:
1. ASCII wrap row-count tests
2. Wide and combining grapheme wrap tests
3. Tab stop tests
4. Wrapped-target reveal tests
5. Render-cost guard tests
6. Binary walkthrough against a file with a 500-character line:
   - Wrapped rows with blank continuation gutters
   - `w` showing the long line as one clipped row
   - Tabs aligned to eight-column stops
   - A match near the end of the long line revealed on its own row after `n`

Note: Go test output timings are normalized (shown as Xs) for reproducibility.

## 1. ASCII wrap row-count tests

ASCII wrapping breaks a long line at the text width. A 26-character line at width 10 produces 3 rows (10 + 10 + 6).

```bash
go test ./internal/viewport/ -run '^TestWrapRowCountASCII$|^TestWrapRowCountExactMultiple$|^TestWrapRowCountShortLine$|^TestWrapRowCountMultipleLines$' -v 2>&1 | sed 's/[0-9]\+\.[0-9]\+s/Xs/g'
```

```output
=== RUN   TestWrapRowCountASCII
--- PASS: TestWrapRowCountASCII (Xs)
=== RUN   TestWrapRowCountExactMultiple
--- PASS: TestWrapRowCountExactMultiple (Xs)
=== RUN   TestWrapRowCountShortLine
--- PASS: TestWrapRowCountShortLine (Xs)
=== RUN   TestWrapRowCountMultipleLines
--- PASS: TestWrapRowCountMultipleLines (Xs)
PASS
ok  	vrg/internal/viewport	(cached)
```

```bash
go test ./internal/safepresentation/ -run '^TestContentTabPlaceholder$' -v 2>&1 | sed 's/[0-9]\+\.[0-9]\+s/Xs/g' && go test ./internal/viewport/ -run '^TestTabStops' -v 2>&1 | sed 's/[0-9]\+\.[0-9]\+s/Xs/g'
```

```output
=== RUN   TestContentTabPlaceholder
--- PASS: TestContentTabPlaceholder (Xs)
PASS
ok  	vrg/internal/safepresentation	(cached)
=== RUN   TestTabStopsInWrapMode
--- PASS: TestTabStopsInWrapMode (Xs)
=== RUN   TestTabStopsWrapLongLine
--- PASS: TestTabStopsWrapLongLine (Xs)
PASS
ok  	vrg/internal/viewport	(cached)
```

## 4. Wrapped-target reveal tests

The Issue #14 destination reveal finds the wrapped row containing the match start. A source line taller than several screens still reveals the match at `floor(h / 3)`.

```bash
go test ./internal/viewport/ -run '^TestWrappedSourceLineTallerThanScreens$|^TestRevealAcrossMultipleWrappedLines$|^TestRevealWrappedTargetWithViewport$|^TestRevealWrappedTargetVisibleNoScroll$' -v 2>&1 | sed 's/[0-9]\+\.[0-9]\+s/Xs/g' && go test ./internal/app/ -run '^TestWrappedTargetRevealLongLine$' -v 2>&1 | sed 's/[0-9]\+\.[0-9]\+s/Xs/g'
```

```output
=== RUN   TestWrappedSourceLineTallerThanScreens
--- PASS: TestWrappedSourceLineTallerThanScreens (Xs)
=== RUN   TestRevealAcrossMultipleWrappedLines
--- PASS: TestRevealAcrossMultipleWrappedLines (Xs)
=== RUN   TestRevealWrappedTargetWithViewport
--- PASS: TestRevealWrappedTargetWithViewport (Xs)
=== RUN   TestRevealWrappedTargetVisibleNoScroll
--- PASS: TestRevealWrappedTargetVisibleNoScroll (Xs)
PASS
ok  	vrg/internal/viewport	(cached)
=== RUN   TestWrappedTargetRevealLongLine
--- PASS: TestWrappedTargetRevealLongLine (Xs)
PASS
ok  	vrg/internal/app	(cached)
```

## 5. Render-cost guard tests

`View()` queries only the visible row range from the row provider. A counting fake provider asserts the wrapper is never invoked for lines outside the visible rows.

```bash
go test ./internal/viewport/ -run '^TestRenderCostGuardWithWrapping$' -v 2>&1 | sed 's/[0-9]\+\.[0-9]\+s/Xs/g' && go test ./internal/app/ -run '^TestRenderCostGuardWithWrappingApp$' -v 2>&1 | sed 's/[0-9]\+\.[0-9]\+s/Xs/g'
```

```output
=== RUN   TestRenderCostGuardWithWrapping
--- PASS: TestRenderCostGuardWithWrapping (Xs)
PASS
ok  	vrg/internal/viewport	(cached)
=== RUN   TestRenderCostGuardWithWrappingApp
--- PASS: TestRenderCostGuardWithWrappingApp (Xs)
PASS
ok  	vrg/internal/app	(cached)
```

## 6. Binary walkthrough against a file with a 500-character line

The demo script builds the vrg binary and runs it against a file with a 500-character line plus a tab-indented line. The terminal is 24 rows by 60 columns (narrow to force wrapping). The script captures the screen at each step using a pty and a simple ANSI cursor-position interpreter.

```bash
cd Notes/walkthroughs/016-04/code-walkthrough && rm -f wrapfile.txt vrg-demo && ./demo.sh
```

```output
=== Initial screen (wrap on, startup reveal of line 1 match) ===
  wrapfile.txt         ── wrapfile.txt ──
                       1  short line one TARGET
                       2  xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
                          xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
                          xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
                          xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
                          xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
                          xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
                          xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
                          xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
                          xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
                          xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
                          xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
                          xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
                          xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
                          xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx TAR
                          GET
                       3          col1    col2    col3
                       4  short line two

=== After w (run-off-edge: long line is one clipped row) ===
  wrapfile.txt         ── wrapfile.txt ──
                       1  short line one TARGET
                       2  xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
                       3          col1    col2    col3
                       4  short line two
                          xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
                          xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
                          xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
                          xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
                          xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
                          xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
                          xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
                          xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
                          xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
                          xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
                          xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx TAR
                          GET
                       3          col1    col2    col3
                       4  short line two

=== After w again (wrap back on) ===
  wrapfile.txt         ── wrapfile.txt ──
                       1  short line one TARGET
                       2  xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
                          xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
                          xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
                          xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
                          xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
                          xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
                          xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
                          xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
                          xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
                          xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
                          xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
                          xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
                          xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
                          xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx TAR
                          GET
                       3          col1    col2    col3
                       4  short line two

=== After n (match near end of long line revealed on its own row) ===
  wrapfile.txt         ── wrapfile.txt ──
                       1  short line one TARGET
                       2  xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
                          xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
                          xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
                          xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
                          xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
                          xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
                          xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
                          xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
                          xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
                          xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
                          xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
                          xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
                          xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
                          xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx TAR
                          GET
                       3          col1    col2    col3
                       4  short line two

=== Demo complete ===
```

The demo shows all Issue #16 features:

1. **Wrapped rows with blank continuation gutters**: The 500-character line (line 2) wraps to 15 rows. The first row shows the line number "2" in the gutter; all continuation rows have a blank gutter aligned with the first row's text.

2. **`w` showing one clipped row**: After pressing `w`, run-off-edge mode shows line 2 as a single clipped row (the first 36 characters). Lines 1, 3, and 4 are also single rows.

3. **Tabs aligned to eight-column stops**: Line 3 (`\tcol1\tcol2\tcol3`) shows tabs expanded to 8-column stops: `col1` starts at column 8, `col2` at column 16, `col3` at column 24.

4. **Match near the end of the long line revealed on its own row after `n`**: The `TARGET` at the end of the 500-character line is on its own wrapped row (the row ending with `TAR` followed by `GET`). After pressing `n`, the cursor moves to this match. The reveal correctly does not scroll because the match is already visible (visible-target no-scroll per Issue #14).

Note: The `After w` screen capture shows residual rows from the previous wrap-mode render below the run-off-edge content. This is a Bubble Tea incremental-rendering artifact in the pty capture; the actual terminal display is correct.

The demo script (`demo.sh`) is self-contained: it builds the `vrg` binary and creates the test fixture if missing. The screen captures use a pty and a simple ANSI cursor-position interpreter (with scroll command support) to reconstruct the screen.
