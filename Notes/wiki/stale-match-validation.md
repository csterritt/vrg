# Stale-match validation and the file-changed note (Issue #29)

Delivered by
[Issue #29](../issues/029-stale-match-validation-and-file-changed-note.md)
([task](../tasks/029-stale-match-validation-and-file-changed-note.md)):
when a file changes on disk after the search ran, the recorded match
data may no longer describe the loaded bytes. FileBuffer validates
every recorded submatch against the loaded content on **first load and
every reload**, drops what fails, keeps what survives, and marks the
buffer stale; the stale stops stay navigable with defined landings,
and the filename row carries a persistent "file changed since search"
note. Relevant PRD section: *Encodings and stale-content validation*
(stale validation bullets), plus the stale-fallback clause of the
display-target bullet in *Navigation, viewport, and logical anchors*
and the `stale state` output of the FileBuffer module contract in
[`Notes/PRD-vrg.md`](../PRD-vrg.md). Builds on the
[Issue #22 coordinate views](line-structure.md) (validation runs in
rg-line space, `Line.SearchBytes`), the
[Issue #23 marker spans](zero-width-markers.md) (surviving zero-width
submatches still map to marker cells), the
[Issue #24 filename-row note slot](file-list-layout.md), the
[Issue #27 reload route](explicit-reload.md) (staleness recomputes per
load), and the
[Issue #28 two-stage completion](load-completion-reveal.md) (the
fallback target resolves at install through the same reveal intent).

## Validation — per-submatch, in search bytes

`filebuffer.Decode` splits lines first, then runs `validateStop` for
each index stop. A stop recording no submatches maps its prepared
union coverage unchecked (the `mapRecorded` path), and a file whose
raw bytes carry a UTF-16 or UTF-32 signature — `utf16Or32`, longer
BOMs checked before overlapping shorter ones — skips validation
entirely: those bytes are not the rg-line coordinate model, and
Issue #30 owns the unsupported-encoding contract. Every other stop
checks each recorded submatch for:

- **line existence** — the recorded line number must be inside the
  loaded file;
- **range validity** — `0 ≤ Start ≤ End ≤ len(SearchBytes)`, against
  the terminator-including, BOM-adjusted rg-line view — never the
  stripped display text, so a match on removed CRLF bytes or a raw
  control byte still validates;
- **byte equality** — `SearchBytes[Start:End]` must equal the recorded
  `Submatch.Bytes`, which the index parser already populates from
  either the JSON `text` or `bytes` encoding.

Any failure drops **that submatch only** and marks the buffer stale
(`Buffer.Stale()`); surviving submatches' union keeps painting the
line's `Highlights`, so a partially changed line retains its valid
highlight and a zero-width survivor keeps its marker. The check is
best-effort: edits that preserve the recorded bytes at the recorded
offsets — a change elsewhere on the line, or a same-bytes splice — are
undetectable and leave the buffer clean.

## Fallback landings — three rules, no invented marks

Validation also resolves each stop's reveal target while it runs,
stored on the line (`target`/`hasTarget`) and surfaced as
`Buffer.StopTarget(st) → (line, cell)` — `viewport.Rows.StopTarget`
now delegates to it:

1. **Survivors exist** → the first surviving submatch's start cell —
   the marker cell for a zero-width survivor.
2. **Line exists, all dropped** → the first recorded start clamped to
   `len(SearchBytes)` and mapped to a valid display cell; an
   end-of-line landing clamps to the last rendered cell when no
   marker cell sits there.
3. **Line gone** → the last source line's start (`Number`, cell 0); an
   empty file has no rows and the stop keeps its bare recorded line —
   the zero-line panel.

Fallbacks invent nothing: a dropped submatch contributes no highlight
span and no marker cell, so a stale line paints plain text only. A
stop outside the buffer's validated set — a foreign stop queried
against rows, as the viewport tests pass — falls back to the
pre-validation first-submatch mapping, so `hasTarget` also gates the
legacy path.

## The filename-row note

The [Issue #24 status-note slot](file-list-layout.md) gets its second
supplier: `fileLoadedMsg`'s success branch sets
`notes[path] = "file changed since search"` when the installed buffer
is stale, and **deletes the note** when it validates fully — and the
failure branch deletes it too, since a dropped buffer claims nothing
about the file. The slot composes it on **every display with no
timer**: the path left-truncates with a leading `…` to make room at
constrained widths, and the note itself clips to whatever the slot
leaves. Because the mark is recomputed per load, `r` clears the note
only when the newly loaded content passes validation — a still-
mismatched reload keeps it.

## Reveal and status integration

No new plumbing: the stale targets ride the existing commit path.
`reveal`'s `TargetRow`/`StopTarget` calls resolve through the buffer's
recorded landing, so a `pendingReveals` commit — navigation during a
held reload included — reveals the first survivor or the fallback at
install, through the matching prepared layout exactly as Issue #28
arbitrates. The fixed search-derived exit status is untouched: stale
validation touches presentation only, and an all-stale index exits
with the same status — pinned by a new `staleLoads` outcome-matrix row
mirroring Issue #26's `failLoads` mechanism (the current file's load
runs an injected stale-returning loader, other positions take injected
stale-decoded buffers).

## Tests

See [unit-tests.md](unit-tests.md):
`internal/filebuffer/stale_test.go` covers the validation checks —
out-of-bounds ranges, same-length replacement, partial survival, the
BOM-adjusted search view, the CRLF terminator match, the raw-bytes
comparison, zero-width survival — the three landings, the empty file,
per-load recomputation, and `StopTarget` resolution.
`internal/app/stale_test.go` covers the note in the slot at ordinary
and constrained widths, the clean-reload clear and mismatched-reload
retention, the gated-reload commits revealing the first survivor and
the clamped fallback, the missing-line last-line landing, and the
no-invented-highlight renders; `internal/app/outcome_test.go` gains
the `staleLoads` field and the all-stale fixed-status row.
