# Stale-match validation and file-changed note (Issue #29)

Issue #29 added best-effort stale-match validation to the FileBuffer
and a persistent "file changed since search" filename-row note. When
loaded file content no longer corresponds to ripgrep's recorded match
data, invalid submatches are dropped individually, the buffer is
marked stale, surviving submatches keep their highlights, and the
filename row shows the note on every display with no timer. Stale
entries remain navigation stops; the reveal falls back through three
landing rules when the first recorded submatch is dropped. Reload
recomputes the validation result so the note clears only after fully
validating content. The fixed search-derived exit status is never
altered by stale content. Relevant PRD section: *Encodings and
stale-content validation*. See
[structural-line-handling](structural-line-handling.md) for the
original-line-byte retention and UTF-8 BOM coordinate separation that
validation builds on,
[file-list-layout](file-list-layout.md) for the filename-row status
slot that carries the note,
[explicit-reload](explicit-reload.md) for the `r` reload that
recomputes staleness, and
[load-completion-two-stage](load-completion-two-stage.md) for the
two-stage path that commits the stale-aware reveal at layout
installation.

## Problem

Before Issue #29, the FileBuffer trusted the search index's recorded
submatch data unconditionally. Highlights and the reveal target were
computed from `stop.Submatches[0]` even when the file on disk no
longer matched the bytes ripgrep recorded. A same-length replacement
of the matched word (e.g. `hit` → `hat`) left a highlight over
unrelated text and positioned the reveal at a byte that no longer
corresponded to a match. There was no signal to the user that the
displayed content had diverged from the search results.

## Solution

Issue #29 added per-submatch validation in `filebuffer.Load`, a
`Buffer.Stale` flag, per-line survivor and fallback metadata, a
`Buffer.RevealTarget` method that exposes the validated reveal
target, integration of the stale note into the Issue #24 filename-row
status slot, and integration of the validated reveal target into the
Issue #28 two-stage reveal path.

### Validation checks

`filebuffer.Load` validates every submatch of every stop against the
original line bytes (including terminators, in rg-line coordinates
with the leading UTF-8 BOM stripped for line 1, per Issue #22). Each
submatch is checked for:

1. **Line existence** — the stop's line number must be in
   `[1, lineCount]`. A missing line fails every submatch.
2. **Range validity** — `Start <= End` and `[Start, End)` within
   `[0, len(rawLine)]`. Out-of-bounds, negative-start, and
   reversed-range submatches fail.
3. **Byte equality** — `rawLine[Start:End]` must equal the recorded
   `Submatch.Match` bytes. Both JSON encodings (text and base64) are
   already decoded by `searchindex`, so the comparison is plain byte
   equality. A same-length replacement (`hit` → `hat`) fails here.

A zero-width submatch (`Start == End`) validates when `Start` is in
range and the match bytes are empty.

Validation runs on first load and every reload (both call `Load`),
so staleness recomputes each time. This is **best-effort
correspondence**, not a snapshot or regex re-evaluation: same-text
moves, changes outside matched spans, and zero-width
assertion-context changes can remain undetected.

### Per-submatch drops and stale marking

Any failed submatch is dropped from the validated stop list; it
produces no highlight and no marker. Any failure marks the buffer
stale (`Buffer.Stale = true`). Valid submatches on the same line
keep their highlights. A stop with a mix of valid and invalid
submatches retains the valid ones and is still stale.

### Three fallback landing rules

`Buffer.RevealTarget(stop)` returns the validated reveal target
`(lineIdx, byteStart, cell)`:

1. **First surviving submatch** — when the line exists and has
   surviving submatches, the target is the first survivor's start.
2. **Clamped recorded start** — when the line exists but no
   submatches survived, the target is the first recorded start
   clamped to `[0, len(rawLine)]`. The display cell maps through the
   line's `ByteCells`; when the byte maps to the end-of-line position
   (the content width, past the last content cell) and there is no
   marker cell, the cell clamps to the last rendered content cell
   (`ContentWidth - 1`). When there is a marker cell, the
   end-of-line position is the marker cell.
3. **Last source line** — when the line is gone (line number outside
   the file's line count), the target is the last source line's
   start (byte 0, cell 0). An empty file (zero-line panel) returns
   `lineIdx = -1` indicating no target.

Fallbacks never invent highlights or markers: they only position the
reveal. The byte offset feeds the vertical row lookup; the display
cell feeds the horizontal reveal.

### Persistent filename-row note

`renderFilenameRow` shows `file changed since search` in the Issue
#24 status slot when `m.buffer != nil && m.buffer.Stale`. The note is
shown on every display (no timer) and is subject to the same path
truncation and constrained-width rules as the `(unreadable)` note
from Issue #26. The status-note test seam (`WithStatusNote`) takes
precedence; `readFailed` (Issue #26) takes precedence over the
stale note.

### Reload recomputation

Because `filebuffer.Load` recomputes validation on every call, an
explicit reload (`r`, Issue #27) that loads content which fully
validates clears `Buffer.Stale` and the note disappears. A reload
that loads still-stale content keeps the note. A reload can also
reintroduce staleness: a previously valid buffer reloaded after the
file is edited to mismatch becomes stale.

### Two-stage fallback reveal

The Issue #28 two-stage path commits the reveal intent at layout
installation. `revealTarget` now calls `Buffer.RevealTarget(stop)`
to obtain the validated `(lineIdx, byteStart, cell)` instead of
reading `stop.Submatches[0]` directly. `targetRow` accepts the
validated line/byte and falls back to `stop.LineNumber - 1` only
when the buffer does not provide a target (e.g. an empty file). The
horizontal reveal uses the validated cell. This keeps the reveal
consistent with the dropped/surviving submatch state.

### Fixed-status guarantee

Stale content never alters the fixed search-derived exit status.
`DecideOutcome` computes the exit status solely from the process
exit, stream integrity, usable results, diagnostics, and record
loss. An all-stale index (every submatch dropped) with rg 0, a clean
complete stream, and results still exits 0. The all-stale
outcome-matrix row (`TestStaleAllDroppedOutcomeMatrixRow`) proves
this: stale content affects display only, not the exit status.

### UTF-16/UTF-32 exclusion

Stale validation does not run against UTF-16/UTF-32 raw encoded
bytes; those encodings are handled by Issue #30, which shows
`(unsupported encoding)` and no file text/highlights. The validation
in `filebuffer.Load` operates on the decoded UTF-8 line bytes only.

## Test coverage

`internal/filebuffer/stale_validation_test.go` covers the
FileBuffer-level contracts: out-of-bounds and negative-start drops,
same-length replacement detection, partial survival with retained
highlights, first-survivor reveal target, clamped-start fallback,
end-of-line fallback with and without a marker, past-end clamping,
missing-line last-source-line landing, empty-file zero-line panel,
reload recomputation (clears and reintroduces), validation against
original bytes including CRLF terminators, CRLF and LF
terminator-only matches, UTF-8 BOM first-line validation and
mismatch, raw-bytes retention, and zero-width validation.

`internal/app/stale_note_test.go` covers the App-level contracts:
the exact note text through the Issue #24 slot, persistence across
re-renders, ordinary and constrained widths with no overflow,
negative-dimension safety, reload clearing, first-surviving-submatch
reveal through the two-stage path, clamped-fallback reveal,
missing-line last-line landing, the all-stale outcome-matrix row
keeping exit 0, and the no-invented-highlight rule.
