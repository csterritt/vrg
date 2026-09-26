# Stale-match validation — "file changed since search"

Issue #29
(`Notes/issues/029-stale-match-validation-and-file-changed-note.md`,
tasks
`Notes/tasks/029-stale-match-validation-and-file-changed-note.md`)
added the stale-content contract: a file that changed between the
search and the load is detected best-effort, its invalid highlights
drop one submatch at a time while valid ones keep painting, the buffer
is marked stale, and the filename row carries a persistent `file
changed since search` note until a reload revalidates cleanly. Stale
stops remain navigation stops with clamped landing positions, the
fallback target feeds Issue #28's two-stage commit, and the fixed exit
status never moves.

PRD cross-references: user stories 44–45; "Encodings and stale-content
validation" (the whole stale bullet list — per-submatch validation,
the note, and the fallback landing rules); "Navigation, viewport, and
logical anchors" (the display target "subject to stale-entry
fallback"); "Exit statuses" ("unreadable, stale, or unsupported files
do not change it") and "Testing Decisions → FileBuffer" (the stale
fixture list) in `Notes/PRD-vrg.md`.

## Validation rules

`filebuffer.Prepare` — the decode/map phase every load and every
reload funnels through — validates each recorded submatch against the
loaded content before it can become a highlight:

1. **Line existence**: the stop's `Line` must index a loaded source
   line. A vanished line drops the whole stop's submatch set.
2. **Range**: `start ≥ 0`, `start ≤ end`, and `end ≤ len(raw line)`
   — bounds run against the raw bytes, terminator included, so a
   recorded submatch covering a CRLF terminator validates (Issue #23's
   end-of-line marker case).
3. **Byte equality**: the raw bytes in `[start:end)` must equal the
   recorded `Submatch.Bytes` — `searchindex` has already normalized
   both the JSON `text` and `bytes` encodings into that field, so one
   comparison covers both.

Validation compares **original/search bytes, never the escaped display
text**: a match on a raw ESC byte or an invalid UTF-8 byte validates
against the byte itself even though it displays as `^[` or U+FFFD.
The leading UTF-8 BOM splits line one's coordinate views before
validation: rg's first-line offsets omit the BOM's three bytes while
the raw view retains them, so first-line `start`/`end` shift by three
before the checks and before display mapping. UTF-16/UTF-32 detection
is Issue #30's; until it lands every loaded file runs this validation,
which is correct for the raw-byte comparison it performs — the PRD's
explicit exclusion applies to *detected* UTF-16/32 files once
detection exists.

Every failure drops **that submatch alone** and marks the buffer
stale; the line's other valid submatches still map to their
cluster-expanded spans, so partial survival keeps partial highlights.
The verdict lives on the `Buffer` (`b.Stale()`): a reload's fresh
`Prepare` recomputes it from the newly read bytes, so the mark clears
only when the new content validates fully — reverting the file clears
it, a still-changed file keeps it, and a clean file can go stale on a
later load.

## Best effort, not a snapshot

The check is correspondence between recorded spans and current bytes,
not change detection: same-text moves and edits outside matched spans
go undetected by design. It catches the dangerous cases — the
highlight no longer being where the match was — without promising
file-change awareness.

## The filename-row note

A stale buffer shows `file changed since search` in the Issue #24
filename-row status slot **every time it is displayed** — no timer, no
dismissal path short of a revalidating reload. `bufferNote` supplies
it behind the `statusNote` test seam, ranked below `(unreadable)`: a
path in the failed state reports the failure. The model's `stale`
map is written only by a *completed* load's verdict, so the note
persists through a reread's `Loading…` placeholder and disappears only
when the completion's own validation is clean — `r` recomputes, it
does not clear.

The slot composes at every width under `filenameRule`'s existing
arithmetic: the note wins cells over the path (which left-truncates to
make room), and a note that cannot fit even with an empty path is
dropped rather than clipped mid-text.

## Fallback landing positions

Stale entries stay navigation stops — no reordering, no dropping.
`Buffer.RevealTarget(stop)` returns the display location `(source
line, display cell)` the reveal must show:

1. **First surviving submatch** — the earliest start cell of the
   line's validated spans, the marker cell for a zero-width survivor
   included. This is also the clean path: a fully valid stop's target
   is the same answer Issue #14 computed from the spans.
2. **No survivors, line present** — the earliest recorded submatch
   start (by start value, not slice position), BOM-shifted on line
   one, clamped to the line's raw bytes, mapped to a display cell. A
   mapping onto the end-of-line position falls back to the last
   rendered cell: no marker paints there, and the fallback never
   invents one. An empty line clamps to cell 0.
3. **Line gone** — the last source line's start. An empty file yields
   the zero value: a zero-line panel has nowhere to land and the
   viewport's empty reveal is a no-op.

Fallbacks invent no highlights and no markers: the dropped submatches
produce no spans, so nothing paints that was not validated.

## Riding the two-stage commit

`reveal()` asks the buffer for the target rather than deriving it, so
the fallback flows through the ordinary path — synchronous reveals on
installed layouts and the `intentReveal` intent alike. That is what
puts it under Issue #28's contract: a navigation landing on a stale
stop while its file's reread is gate-held carries the intent, and when
the reload's matching prepared layout installs, `commitIntent` reveals
the newest selected stop's survivor-or-fallback target computed
against the *new* buffer — the recorded span is never used as a
destination.

## Fixed exit status

Stale state is presentation-only. The outcome decision ran at
`searchDoneMsg` before any load, and validation marks no exit code:
an index whose every retained stop validates stale still exits 0 (or
its fixed search-derived status). The outcome matrix carries the
all-stale row proving it.

## Tests

- `internal/filebuffer/stale_test.go` (Issue #29) drives `Prepare`
  over in-memory bytes — the reload path's revalidation is a fresh
  `Prepare`:
  `TestFullyValidatingBufferIsNotStale`;
  `TestDroppedSubmatchesMarkStale` (same-length replacement,
  out-of-bounds start and end, missing line, one-of-two partial
  survival);
  `TestStalePartialSurvivalKeepsValidHighlights` (the survivor's span
  retained and revealed);
  `TestStaleFallbackClampsRecordedStart` (mid-line, on-the-terminator
  and past-the-line clamps to the last rendered cell, empty line to
  cell 0);
  `TestStaleFallbackUsesEarliestRecordedStart` (start order, not
  slice order);
  `TestStaleFallbackMissingLineAndEmptyFile` (last source line; the
  zero-line panel's inert zero target);
  `TestRevalidationRecomputesStale` (stale → clean → stale across
  prepares);
  `TestValidationUsesOriginalNotDisplayBytes` (ESC and invalid-UTF-8
  matches validate against raw bytes);
  `TestCRLFTerminatorMatchValidatesClean` (the terminator-only match
  validating into the ordinary end-of-line marker);
  `TestStaleFallbackShiftsPastLeadingBOM` (the line-one shift in both
  validation and the fallback).
- `internal/app/stale_test.go` (Issue #29) pins the integration:
  `TestStaleBufferShowsFileChangedNote` (the note on every display —
  scroll, `n`, resize — with no timer, and no inverse video left
  anywhere);
  `TestStaleNoteClearsOnlyOnCleanReload` (the note survives the
  gate-held reread and clears on the validating completion with the
  highlight back);
  `TestStaleNoteComposedAtAllWidths` (the status slot at 80→20
  columns: whole note where it fits under a truncating path, dropped
  where it cannot, no row overflow, nonnegative layout);
  `TestGatedReloadCommitRevealsSurvivingSubmatch` (n during the
  gate-held reload selects the half-stale stop; the matching-layout
  commit reveals the survivor and never the dropped span);
  `TestGatedReloadCommitRevealsClampedFallback` (all submatches
  dropped on a still-present line: the commit lands on the clamped
  recorded start deep inside a wrapping line, painting no highlight);
  `TestStaleAllDroppedRevealsClampedStart`;
  `TestStaleMissingLineLandsOnLastSourceLine` (the last source line
  at the frame's bottom, no invented highlight).
- `internal/app/outcome_test.go` gained the Issue #29 row — `fileData`
  replaces the fixture bytes and `loadCurrent` settles the load before
  the assertions: every retained stop validating stale still exits
  with the fixed status 0.

See [unit-tests.md](unit-tests.md) § `internal/filebuffer` and
§ `internal/app`.

## Files

- `internal/filebuffer/filebuffer.go` — `Prepare`'s validation loop
  (BOM shift, range and byte-equality checks against `Raw()`),
  `Buffer.stale`, `Stale()`, `RevealTarget`, the `bom` field.
- `internal/app/app.go` — the `stale` per-path map and its
  `loadDoneMsg` write of the completed load's verdict.
- `internal/app/browse.go` — `reveal()` consulting `RevealTarget`,
  `bufferNote`'s stale branch under `(unreadable)`.

See also: [destination-reveal.md](destination-reveal.md) (the reveal
contract the fallback feeds),
[load-completion-reveal.md](load-completion-reveal.md) (the two-stage
commit carrying the fallback target),
[explicit-reload.md](explicit-reload.md) (the `r` reread that
recomputes the note),
[file-list-layout.md](file-list-layout.md) (the status slot the note
occupies),
[read-failures.md](read-failures.md) (the `(unreadable)` note that
outranks it),
[line-terminators-and-bom.md](line-terminators-and-bom.md) (the raw
bytes and BOM coordinate split validation depends on),
[zero-width-match-markers.md](zero-width-match-markers.md) (the marker
cell a surviving zero-width submatch supplies — and a fallback never
invents),
[grapheme-highlight-expansion.md](grapheme-highlight-expansion.md)
(the cluster-expanded spans survivors keep), and
[match-navigation.md](match-navigation.md) (the stops that remain
navigable while stale).
