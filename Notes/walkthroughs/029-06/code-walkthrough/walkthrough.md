# Issue #29: Stale-match validation and file-changed note

*2026-09-12 by Showboat 0.6.1*

Walkthrough for Issue #29 (Notes/tasks/029-stale-match-validation-and-file-changed-note.md), implementing best-effort stale-match validation in the FileBuffer and a persistent "file changed since search" filename-row note. When loaded file content no longer corresponds to ripgrep's recorded match data, invalid submatches are dropped individually, the buffer is marked stale, surviving submatches keep their highlights, and the filename row shows the note on every display with no timer. Stale entries remain navigation stops; the reveal falls back through three landing rules when the first recorded submatch is dropped. Reload recomputes the validation result so the note clears only after fully validating content. The fixed search-derived exit status is never altered by stale content. References: Notes/PRD-vrg.md (Encodings and stale-content validation), Notes/wiki/stale-match-validation.md, Issue #29.

Contracts verified:

- Each submatch is checked for line existence, range validity against original line bytes including terminators, Issue #22 UTF-8 BOM coordinate adjustment, and byte equality with recorded submatch bytes.
- Both JSON encodings (text and base64) are accepted (already decoded by searchindex).
- Any failed submatch is dropped; any failure marks the buffer stale.
- Valid submatches remain highlighted (partial survival).
- A stale stop with survivors exposes the first surviving submatch as the reveal target.
- A stop with no surviving submatches but an existing line exposes the first recorded start, clamped to available line bytes and mapped to a valid display cell.
- End-of-line fallback clamps to the last rendered cell when there is no marker cell.
- A missing line lands at the last source line's start.
- An empty file remains a zero-line panel.
- Reload recomputes stale state.
- The stale note clears only after fully validating content.
- Validation compares against original/search bytes rather than stripped display text.
- CRLF terminator matches still validate.
- The exact text "file changed since search" appears in the filename row through the Issue #24 status slot.
- The note is shown on every display (no timer).
- The note appears at ordinary and constrained widths with path truncation and no overflow.
- The note never produces negative dimensions.
- The latest selected stop's first recorded submatch is dropped after content changes; the reveal targets the first surviving submatch through the Issue #28 two-stage path.
- Otherwise the reveal targets the clamped fallback.
- An all-dropped stop with line still present reveals the clamped start with no highlight invented.
- A missing line lands at the last source line.
- An all-stale index keeps the fixed exit status unchanged (all-stale outcome-matrix row).
- Fallbacks never invent highlights or markers.

The FileBuffer and App model tests are the authoritative deterministic verification. The manual route at the end demonstrates the behavior with a real file on disk.

```bash
go build ./cmd/... ./internal/... && go vet ./cmd/... ./internal/... && echo GATES-OK
```

```output
GATES-OK
```

```bash
go test -count=1 ./cmd/... ./internal/... -timeout 120s | sed 's/[[:space:]][0-9.]*s$//'
```

```output
ok  	vrg/cmd/vrg
ok  	vrg/internal/app
ok  	vrg/internal/cli
ok  	vrg/internal/filebuffer
ok  	vrg/internal/safepresentation
ok  	vrg/internal/searchindex
?   	vrg/internal/sinkfixtures	[no test files]
ok  	vrg/internal/theme
ok  	vrg/internal/viewport
```

## FileBuffer validation tests

The FileBuffer validation tests (internal/filebuffer/stale_validation_test.go) verify that each submatch is checked for line existence, range validity, and byte equality against the original line bytes (including terminators, BOM-adjusted); that out-of-bounds, negative-start, and same-length-replacement submatches are dropped; that partial survival retains valid highlights; that the first surviving submatch is the reveal target; that the clamped-start fallback maps to a valid display cell with end-of-line clamping when there is no marker; that a missing line lands at the last source line; that an empty file remains zero-line; that reload recomputes staleness (clears and reintroduces); that validation compares against original bytes including CRLF terminators; that CRLF and LF terminator-only matches validate; that BOM first-line matches validate and mismatches are dropped; that raw bytes are retained; and that zero-width submatches validate or are dropped by bounds.

```bash
go test -count=1 -v ./internal/filebuffer/ -run '^TestStale' -timeout 60s | sed 's/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
```

```output
=== RUN   TestStaleValidSubmatchNotStale
--- PASS: TestStaleValidSubmatchNotStale (0.00s)
=== RUN   TestStaleNoStopsNotStale
--- PASS: TestStaleNoStopsNotStale (0.00s)
=== RUN   TestStaleOutOfBoundsSubmatchDropped
--- PASS: TestStaleOutOfBoundsSubmatchDropped (0.00s)
=== RUN   TestStaleNegativeStartDropped
--- PASS: TestStaleNegativeStartDropped (0.00s)
=== RUN   TestStaleSameLengthReplacementDropped
--- PASS: TestStaleSameLengthReplacementDropped (0.00s)
=== RUN   TestStalePartialSurvival
--- PASS: TestStalePartialSurvival (0.00s)
=== RUN   TestStalePartialSurvivalRevealTarget
--- PASS: TestStalePartialSurvivalRevealTarget (0.00s)
=== RUN   TestStaleClampedStartFallback
--- PASS: TestStaleClampedStartFallback (0.00s)
=== RUN   TestStaleClampedStartEOLFallback
--- PASS: TestStaleClampedStartEOLFallback (0.00s)
=== RUN   TestStaleClampedStartEOLWithMarker
--- PASS: TestStaleClampedStartEOLWithMarker (0.00s)
=== RUN   TestStaleClampedStartPastEnd
--- PASS: TestStaleClampedStartPastEnd (0.00s)
=== RUN   TestStaleMissingLineFallback
--- PASS: TestStaleMissingLineFallback (0.00s)
=== RUN   TestStaleEmptyFileRemainsZeroLines
--- PASS: TestStaleEmptyFileRemainsZeroLines (0.00s)
=== RUN   TestStaleReloadRecomputes
--- PASS: TestStaleReloadRecomputes (0.00s)
=== RUN   TestStaleReloadReintroducesStaleness
--- PASS: TestStaleReloadReintroducesStaleness (0.00s)
=== RUN   TestStaleValidationAgainstOriginalBytes
--- PASS: TestStaleValidationAgainstOriginalBytes (0.00s)
=== RUN   TestStaleCRLFTerminatorOnlyMatchValidates
--- PASS: TestStaleCRLFTerminatorOnlyMatchValidates (0.00s)
=== RUN   TestStaleLFTerminatorOnlyMatchValidates
--- PASS: TestStaleLFTerminatorOnlyMatchValidates (0.00s)
=== RUN   TestStaleBOMFirstLineValidates
--- PASS: TestStaleBOMFirstLineValidates (0.00s)
=== RUN   TestStaleBOMFirstLineMismatchDropped
--- PASS: TestStaleBOMFirstLineMismatchDropped (0.00s)
=== RUN   TestStaleRetainsRawBytes
--- PASS: TestStaleRetainsRawBytes (0.00s)
=== RUN   TestStaleZeroWidthValidates
--- PASS: TestStaleZeroWidthValidates (0.00s)
=== RUN   TestStaleZeroWidthOutOfBoundDropped
--- PASS: TestStaleZeroWidthOutOfBoundDropped (0.00s)
PASS
ok  	vrg/internal/filebuffer
```

## App note and two-stage reveal tests

The App note and two-stage reveal tests (internal/app/stale_note_test.go) verify that the exact text "file changed since search" appears in the filename row through the Issue #24 status slot; that the note is persistent (no timer); that it appears at ordinary and constrained widths with path truncation and no overflow; that it never produces negative dimensions; that it clears after a reload that validates; that the first surviving submatch is the reveal target through the Issue #28 two-stage path when the first recorded submatch is dropped; that the clamped fallback reveals when all submatches are dropped with no highlight invented; that a missing line lands at the last source line; that an all-stale index keeps the fixed exit status unchanged; and that no highlights are invented for dropped submatches.

```bash
go test -count=1 -v ./internal/app/ -run '^TestStaleNote|^TestStaleReveal|^TestStaleAllDropped|^TestStaleNoInvented' -timeout 60s | sed 's/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
```

```output
=== RUN   TestStaleNoteShown
--- PASS: TestStaleNoteShown (0.00s)
=== RUN   TestStaleNoteNotShownWhenValid
--- PASS: TestStaleNoteNotShownWhenValid (0.00s)
=== RUN   TestStaleNoteShownEveryDisplay
--- PASS: TestStaleNoteShownEveryDisplay (0.00s)
=== RUN   TestStaleNoteOrdinaryWidth
--- PASS: TestStaleNoteOrdinaryWidth (0.00s)
=== RUN   TestStaleNoteConstrainedWidth
--- PASS: TestStaleNoteConstrainedWidth (0.00s)
=== RUN   TestStaleNoteNoNegativeDimensions
--- PASS: TestStaleNoteNoNegativeDimensions (0.00s)
=== RUN   TestStaleNoteClearsOnReload
--- PASS: TestStaleNoteClearsOnReload (0.00s)
=== RUN   TestStaleRevealFirstSurvivingSubmatch
--- PASS: TestStaleRevealFirstSurvivingSubmatch (0.00s)
=== RUN   TestStaleRevealClampedFallbackAllDropped
--- PASS: TestStaleRevealClampedFallbackAllDropped (0.00s)
=== RUN   TestStaleRevealMissingLineLandsAtLastLine
--- PASS: TestStaleRevealMissingLineLandsAtLastLine (0.00s)
=== RUN   TestStaleAllDroppedOutcomeMatrixRow
--- PASS: TestStaleAllDroppedOutcomeMatrixRow (0.00s)
=== RUN   TestStaleNoInventedHighlight
--- PASS: TestStaleNoInventedHighlight (0.00s)
PASS
ok  	vrg/internal/app
```

## Manual verification

The manual demo test (demo_artifacts/manual_demo_test.go) exercises the Issue #29 manual verification scenarios with a real file on disk: edit the matched word to a same-length different word, enter the file, verify no highlight appears, verify the stale note appears, delete trailing matched lines, verify landing on the last line without a highlight, revert using r, and verify the stale note clears.

```bash
cp Notes/walkthroughs/029-06/code-walkthrough/demo_artifacts/manual_demo_test.go internal/app/manual_demo_test.go && go test -count=1 -v -tags manual_demo ./internal/app/ -run '^TestManualDemoStaleValidation$' -timeout 30s 2>&1 | sed 's/[[:space:]]0\.[0-9]*s//g; s/([0-9.]*s)/(0.00s)/g'; rm internal/app/manual_demo_test.go
```

```output
=== RUN   TestManualDemoStaleValidation
    manual_demo_test.go:78: Step 3 OK: no highlight on 'hat' (submatch dropped)
    manual_demo_test.go:84: Step 4 OK: 'file changed since search' note present
    manual_demo_test.go:111: Step 6 OK: landed on last line 'gone' without highlight
    manual_demo_test.go:129: Step 8 OK: stale note cleared after revert
    manual_demo_test.go:131: Manual demonstration passed: stale-match validation behavior matches the Issue #29 contracts.
--- PASS: TestManualDemoStaleValidation (0.00s)
PASS
ok  	vrg/internal/app
```
