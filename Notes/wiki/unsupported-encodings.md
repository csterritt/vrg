# Unsupported encodings — "(unsupported encoding)", BOM detection, and notification (Issue #30)

Delivered by
[Issue #30](../issues/030-unsupported-encodings-utf16-utf32.md)
([task](../tasks/030-unsupported-encodings-utf16-utf32.md)):
a file opening with a UTF-16 or UTF-32 byte-order mark is detected in
FileBuffer and presented as "(unsupported encoding)" — no file text,
no highlights — with an explanatory diagnostic, while remaining an
indexed cursor stop and reloading through `r` like any other file.
Relevant PRD section: *Encodings and stale-content validation* (the
unsupported bullets — detection ordering, placeholder, retained
indexing and reloadability, the current/non-current distinction, and
the stale-guard exclusion), plus the fixed-status and
unloaded/unreadable/stale/unsupported cursor-stop bullets of *File
loading, cache, reload, and selection consistency*, the FileBuffer
module contract's unsupported-encoding output, and *Invocation and
child arguments* — the child argv carries no forced raw-encoding flag
so rg's default BOM detection stays enabled — in
[`Notes/PRD-vrg.md`](../PRD-vrg.md). Builds on the
[Issue #22 coordinate views](line-structure.md) whose leading UTF-8
BOM is the supported signature the detector must not misclassify, the
[Issue #25 async load isolation](async-load-isolation.md) keyed
completion, the [Issue #26 read-failure](read-failures.md)
current/non-current notification split and "(unreadable)" placeholder
machinery, the [Issue #27 explicit reload](explicit-reload.md) route
the reread rides unchanged, the
[Issue #29 stale-match validation](stale-match-validation.md) the
early return preempts, and the
[Issue #11 stderr replay](stderr-replay.md) collection.

## BOM detection — longer before shorter

`filebuffer.Decode` now opens with `detectEncoding(raw)`, which checks
the four-byte UTF-32 signatures before the overlapping two-byte UTF-16
ones — UTF-32 LE's `FF FE 00 00` itself opens with UTF-16 LE's
`FF FE`, so the longer mark must be tested first — and reports the
encoding name: "UTF-16 LE", "UTF-16 BE", "UTF-32 LE", or "UTF-32 BE".
A detection returns `Buffer{unsupported: enc}` before any line
splitting and before `validateStop`: the buffer carries no lines, no
highlights, and `Stale()` stays false — the raw encoded bytes never
enter Issue #29's per-submatch validation, whose `SearchBytes`
byte-equality checks rg's transcoded line coordinates cannot satisfy.
`Buffer.Unsupported()` surfaces the name; "" means presented as text.
A leading UTF-8 BOM is never reported — `EF BB BF` remains the
supported signature Issue #22 adjusts and hides — nor are a lone
`FF`/`FE`, a truncated UTF-32 mark, or signature bytes away from the
file's start.

## The placeholder and the notification split

`fileLoadedMsg`'s success branch checks `msg.buf.Unsupported()` after
filing the buffer: it collects `encodingDiag` — `cannot display
<EscapePath(path)>: unsupported encoding <name>` — into the session
collection once per detection, and opens the overlay only when the
loaded path is current at completion-arrival — Issue #26's
current/non-current distinction reused unchanged. A non-current
detection is diagnostic-only: no overlay, no frame change; it
surfaces through the stderr replay at exit (entering the file later
shows the cached placeholder but opens nothing — there is no
retry-sequence overlay the way a failed path has).

The panel stand-in moves from a two-way `failed` boolean to the
`placeholder(path)` selector: an in-flight or unsettled load →
"Loading…", `failed[path]` → "(unreadable)", a cached unsupported
buffer → "(unsupported encoding)" — an in-flight reload always reads
"Loading…", never the buffer it replaces. `requestLayout` returns nil
for an unsupported buffer — a placeholder has no rows to prepare —
and `currentRows` rejects a row model whose buffer is unsupported, so
scrolling, panning, and reveal all no-op on it. The filename rule
keeps identifying the path throughout; the note slot stays empty
because the buffer is never stale.

## Indexed, navigable, reloadable

Unsupported files keep their searchindex stops: `n`/`p` cross into
and out of them exactly like readable files, minting the first-visit
load and the file-change pop-up as usual. `r` rides the unchanged
Issue #27 route — it mints a reload (dropped while one is in flight),
the panel reads "Loading…" while it pends, and an unchanged file
settles back to "(unsupported encoding)" with a fresh overlay and a
second collected diagnostic: detection is reported once per load, not
once per file. A file re-encoded to a supported form between `r` and
settlement decodes normally; one re-encoded the other way gains the
placeholder.

## Detection lives in FileBuffer, not the search

rg's default BOM detection stays enabled — the child argv gains no
encoding flag, so these files still yield matches — which is exactly
why the placeholder exists: rg transcoded the file to match, so the
recorded submatch offsets index the transcoded view, not raw-file
bytes, and presenting them as raw-file highlights would misrepresent
the search.

## The fixed status survives

Detection touches only presentation and diagnostics — `m.status`
fixed at `searchDoneMsg` never recomputes. The new `unsupportedLoads`
outcome-matrix row installs unsupported buffers at every retained
file under fixed status 0 — the current file's through the
transition's own load command under `unsupportedAllLoader`, an
injected loader returning the UTF-16 LE payload for every read,
mirroring `failAllLoader`/`staleAllLoader` — shows the current file's
encoding overlay, and still exits 0. Only `ctrl+c` overrides to 130.

## Composed-view robustness

The unsupported state composes safely like the other placeholders:
the filename row keeps the left-truncated safe path ending at the
basename in its Issue #24 slot, no row overflows the frame, and every
layout dimension stays nonnegative at ordinary and constrained
widths. The placeholder itself clips to the panel's cells — at a
20–30-column frame "(unsupported encoding)" exceeds the panel width,
so its unclipped prefix "(unsupported" is what identifies it.

## Tests

See [unit-tests.md](unit-tests.md):
`internal/filebuffer/encoding_test.go` covers each BOM's
classification — including the signature-only file and a NUL third
byte not promoting UTF-16 LE to UTF-32 LE — the
UTF-32-LE-before-UTF-16-LE overlap ordering, the UTF-8 BOM and other
non-signature bytes never misclassifying, and stale validation never
running on the encoded bytes. `internal/app/encoding_test.go` covers
the current-file overlay + placeholder + retained stop, the
non-current diagnostic-only policy, the `r` reload's "Loading…" →
placeholder sequence with a fresh overlay and second diagnostic, the
no-stale-note guarantee, and the composed view at 80/30/20 columns.
`internal/app/outcome_test.go` gains the `unsupportedLoads` field,
`unsupportedAllLoader`, and the all-unsupported fixed-status row.
