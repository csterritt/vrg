# Issue #29: Stale-match validation and the 'file changed since search' note

*2026-09-24T18:17:42Z by Showboat 0.6.1*
<!-- showboat-id: 7496e0b4-e599-4936-be05-5187b1fd7ddc -->

Walkthrough for [Issue #29](../../../issues/029-stale-match-validation-and-file-changed-note.md), implementing the stale-content contract per `Notes/PRD-vrg.md` (*Encodings and stale-content validation*; *Navigation, viewport, and logical anchors* — the display target "subject to stale-entry fallback"; *Exit statuses* — stale files never change it): on first load and every reload FileBuffer validates each recorded submatch against the line's raw bytes — line existence, range, and byte equality, with the leading UTF-8 BOM's three-byte shift on line one and never the escaped display text — dropping each failing submatch individually while survivors keep their highlights, marking the buffer stale, and carrying the persistent `file changed since search` filename-row note that only a fully validating reload clears. Stale stops remain navigation stops with clamped fallback landings — first survivor, else earliest recorded start clamped to the line's bytes (end-of-line mappings to the last rendered cell), else the last source line's start — inventing no highlights or markers, and the fallback target rides Issue #28's two-stage commit. All generated artifacts live in this directory: the built `vrg` binary, the `demo-stale-validation.sh` tmux harness, and its `stale/` session captures. Test durations are stripped so the document verifies cleanly.

```bash
set -o pipefail; cd /home/chris/vrg && test -z "$(gofmt -l internal/ cmd/)" && echo GOFMT-CLEAN && go vet ./... && go build ./... && go test -count=1 ./... | sed "s/[[:space:]][0-9.]*s$//" && CGO_ENABLED=1 go test -race -count=1 ./internal/app ./internal/filebuffer | sed "s/[[:space:]][0-9.]*s$//" && go build -o Notes/walkthroughs/029-06/code-walkthrough/vrg ./cmd/vrg && echo GATES-OK
```

```output
GOFMT-CLEAN
ok  	vrg/cmd/vrg
ok  	vrg/internal/app
ok  	vrg/internal/cli
ok  	vrg/internal/filebuffer
ok  	vrg/internal/present
ok  	vrg/internal/searchindex
ok  	vrg/internal/theme
ok  	vrg/internal/viewport
ok  	vrg/internal/app
ok  	vrg/internal/filebuffer
GATES-OK
```

## Validation rules — `internal/filebuffer/stale_test.go`

`Prepare` is both the first-load and the reload path, so the tests drive validation in memory over fresh bytes. `TestFullyValidatingBufferIsNotStale` pins the clean case; `TestDroppedSubmatchesMarkStale` tables the failure kinds — same-length replacement, out-of-bounds start and end, a vanished line, and one-of-two partial survival — each dropping only its own submatch and marking the buffer; `TestValidationUsesOriginalNotDisplayBytes` proves the comparison runs against raw bytes (a match on a raw ESC byte or an invalid UTF-8 byte validates even though it displays as `^[` or U+FFFD); `TestCRLFTerminatorMatchValidatesClean` shows a recorded `\r\n` submatch validating against the retained terminator bytes into the ordinary end-of-line marker; `TestRevalidationRecomputesStale` proves the verdict is recomputed per `Prepare` (stale → clean → stale); `TestStaleFallbackShiftsPastLeadingBOM` covers the line-one coordinate shift in validation and the fallback alike.

```bash
cd /home/chris/vrg && go test -count=1 -v -run "TestFullyValidatingBufferIsNotStale|TestDroppedSubmatchesMarkStale|TestValidationUsesOriginalNotDisplayBytes|TestCRLFTerminatorMatchValidatesClean|TestRevalidationRecomputesStale|TestStaleFallbackShiftsPastLeadingBOM" ./internal/filebuffer 2>&1 | grep -vE "^(=== RUN|=== CONT)" | sed -E "s/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//"
```

```output
--- PASS: TestFullyValidatingBufferIsNotStale
--- PASS: TestDroppedSubmatchesMarkStale
    --- PASS: TestDroppedSubmatchesMarkStale/same-length_replacement
    --- PASS: TestDroppedSubmatchesMarkStale/end_out_of_bounds
    --- PASS: TestDroppedSubmatchesMarkStale/start_out_of_bounds
    --- PASS: TestDroppedSubmatchesMarkStale/line_missing
    --- PASS: TestDroppedSubmatchesMarkStale/one_of_two_dropped
--- PASS: TestRevalidationRecomputesStale
--- PASS: TestValidationUsesOriginalNotDisplayBytes
--- PASS: TestCRLFTerminatorMatchValidatesClean
--- PASS: TestStaleFallbackShiftsPastLeadingBOM
PASS
ok  	vrg/internal/filebuffer
```

## Fallback landing targets — `internal/filebuffer/stale_test.go`

`Buffer.RevealTarget` supplies the navigation reveal its `(source line, display cell)`: `TestStalePartialSurvivalKeepsValidHighlights` shows a dropped submatch leaving the survivor's span in place and revealed; `TestStaleFallbackClampsRecordedStart` tables the no-survivor cases — a mid-line start, starts on or past the terminator clamping to the last rendered cell with no invented marker, and an empty line landing on cell 0; `TestStaleFallbackUsesEarliestRecordedStart` pins that the earliest *start value* decides, not slice order; `TestStaleFallbackMissingLineAndEmptyFile` lands a gone-line stop at the last source line's start and keeps an empty file a zero-line panel with the inert zero target.

```bash
cd /home/chris/vrg && go test -count=1 -v -run "TestStalePartialSurvivalKeepsValidHighlights|TestStaleFallbackClampsRecordedStart|TestStaleFallbackUsesEarliestRecordedStart|TestStaleFallbackMissingLineAndEmptyFile" ./internal/filebuffer 2>&1 | grep -vE "^(=== RUN|=== CONT)" | sed -E "s/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//"
```

```output
--- PASS: TestStalePartialSurvivalKeepsValidHighlights
--- PASS: TestStaleFallbackClampsRecordedStart
    --- PASS: TestStaleFallbackClampsRecordedStart/recorded_start_mid-line
    --- PASS: TestStaleFallbackClampsRecordedStart/recorded_start_on_the_terminator
    --- PASS: TestStaleFallbackClampsRecordedStart/recorded_start_past_the_line
    --- PASS: TestStaleFallbackClampsRecordedStart/empty_line
--- PASS: TestStaleFallbackUsesEarliestRecordedStart
--- PASS: TestStaleFallbackMissingLineAndEmptyFile
PASS
ok  	vrg/internal/filebuffer
```

## The filename-row note — `internal/app/stale_test.go` + `outcome_test.go`

`TestStaleBufferShowsFileChangedNote` pins the persistence contract: the note paints on every display — load, scroll, `n`, resize — with no timer, while no inverse video survives anywhere in the frame. `TestStaleNoteClearsOnlyOnCleanReload` holds the reread behind `loadGate`, proves the note survives the whole `Loading…` window, then releases the gate on reverted bytes: the completion's own verdict clears the note and the highlight returns. `TestStaleNoteComposedAtAllWidths` composes the Issue #24 status slot at 80→20 columns — the note whole where it fits under a truncating escaped path, dropped where it cannot, no row overflowing, no dimension negative. The `outcome_test.go` matrix gained the all-stale row (`fileData`/`loadCurrent` row fields): every retained stop validating stale still exits with the fixed status 0.

```bash
cd /home/chris/vrg && go test -count=1 -v -run "TestStaleBufferShowsFileChangedNote|TestStaleNoteClearsOnlyOnCleanReload|TestStaleNoteComposedAtAllWidths|TestOutcomeMatrix/every_retained_stop_validating_stale" ./internal/app 2>&1 | grep -vE "^(=== RUN|=== CONT)" | sed -E "s/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//"
```

```output
--- PASS: TestOutcomeMatrix
    --- PASS: TestOutcomeMatrix/every_retained_stop_validating_stale_keeps_the_fixed_status_0
--- PASS: TestStaleBufferShowsFileChangedNote
--- PASS: TestStaleNoteClearsOnlyOnCleanReload
--- PASS: TestStaleNoteComposedAtAllWidths
    --- PASS: TestStaleNoteComposedAtAllWidths/80x24
    --- PASS: TestStaleNoteComposedAtAllWidths/60x15
    --- PASS: TestStaleNoteComposedAtAllWidths/40x10
    --- PASS: TestStaleNoteComposedAtAllWidths/26x6
    --- PASS: TestStaleNoteComposedAtAllWidths/20x3
PASS
ok  	vrg/internal/app
```

## Fallback reveals through the two-stage commit — `internal/app/stale_test.go`

The stale fallback rides Issue #28's two-stage contract: `TestGatedReloadCommitRevealsSurvivingSubmatch` gate-holds the reread, navigates `n` to the half-stale stop mid-load, and proves the matching-layout commit reveals the *surviving* submatch — the dropped span paints plain beside it — with the note already on the filename row. `TestGatedReloadCommitRevealsClampedFallback` takes the all-dropped variant deep inside a wrapping line: the commit lands on the row holding the clamped recorded start (a wrapping line makes the clamped cell distinguishable from the line's first row) and paints no highlight. `TestStaleAllDroppedRevealsClampedStart` and `TestStaleMissingLineLandsOnLastSourceLine` pin the direct-navigation forms: clamped start on a still-present line, and the last source line's start — clamped to the frame's bottom — for a vanished one.

```bash
cd /home/chris/vrg && go test -count=1 -v -run "TestGatedReloadCommitRevealsSurvivingSubmatch|TestGatedReloadCommitRevealsClampedFallback|TestStaleAllDroppedRevealsClampedStart|TestStaleMissingLineLandsOnLastSourceLine" ./internal/app 2>&1 | grep -vE "^(=== RUN|=== CONT)" | sed -E "s/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//"
```

```output
--- PASS: TestGatedReloadCommitRevealsSurvivingSubmatch
--- PASS: TestGatedReloadCommitRevealsClampedFallback
--- PASS: TestStaleAllDroppedRevealsClampedStart
--- PASS: TestStaleMissingLineLandsOnLastSourceLine
PASS
ok  	vrg/internal/app
```

## Manual route — the real binary on a PTY

`demo-stale-validation.sh` (checked into this directory) runs the freshly built `vrg` on real tmux PTYs with real `rg`, in two sessions over disposable `mktemp` directories — repository and user files are never touched, and an EXIT trap removes them. Fixture edits land while vrg sits idle, before each target file's lazy first load, so the stale verdict is deterministic; inverse-video assertions read the `capture-pane -e` output for the SGR `47` parameter on the match's own row.

- **Session A** — the same-length edit: `b.txt`'s `MARK` becomes `XARK` (four bytes for four) before its first load. `n` enters the file: the recorded submatch fails byte equality, drops, marks the buffer stale — the line paints plain and the filename row carries `b.txt file changed since search`. Reverting `XARK` back to `MARK` and pressing `r` revalidates cleanly: the note disappears and the highlight returns.
- **Session B** — the deleted trailing match: `tail.txt` is truncated from 200 lines to 150 before its first load, so stop `t:200`'s line no longer exists. The first `n` enters a stale buffer — the note shows while `t:1`'s own still-valid match keeps its highlight; the second `n` selects the vanished stop and lands at the last source line's start, clamped to the frame's bottom (top row 128 of 150), with no invented highlight.

```bash
cd /home/chris/vrg/Notes/walkthroughs/029-06/code-walkthrough && unset RIPGREP_CONFIG_PATH && ./demo-stale-validation.sh
```

```output
ok: startup: a.txt loaded and matched
ok: b.txt: filename row carries the stale note
ok: b.txt: the edited line is on screen
ok: b.txt: the dropped submatch paints no highlight
ok: after r: the note cleared on a clean revalidation
ok: after r: the match highlights again
ok: session A: vrg exit status -> 0
ok: tail.txt: one vanished stop marks the whole buffer stale
ok: tail.txt: the still-valid t:1 match keeps its highlight
ok: vanished t:200 lands at the last source line (top row) -> line-000128
ok: last source line paints at the frame's bottom -> line-000150
ok: the landed line invents no highlight
ok: session B: vrg exit status -> 0
demo-stale-validation: all checks passed
```

The deterministic model tests remain the authority for the validation matrix, the clamped-fallback geometry, and the two-stage commit semantics; the PTY route demonstrates the visible behavior end to end — the dropped highlight, the persistent note, the last-line landing, and the `r` revalidation clearing it. Wiki ingest lives in `Notes/wiki/stale-match-validation.md`.
