# Search index: records and stops (Issue #3)

The result-index contract delivered by
[Issue #3](../issues/003-spawn-rg-collect-results-searching-screen.md),
implemented in `internal/searchindex` (`record.go`, `index.go`).
Relevant PRD sections: *Implementation Decisions → Result index,
records, and stream integrity* and *Module Design → Search index*.

## Record decoding

`DecodeRecord` validates one raw JSON line into a typed `Record`. The
envelope must be a JSON object whose `type` member is a JSON string.
Five event types are known:

| Type | Required | Carried |
|---|---|---|
| `begin` | `data` object, `data.path` | path bytes |
| `match` | `data` object, `path`, `lines`, `line_number`, non-empty `submatches` | path, line bytes, line number, submatches |
| `end` | `data` object, `path`, `binary_offset` present (`null` or integer) | path, offset (`nil` for null) |
| `summary` | `data` object | nothing (contents ignored) |
| `context` | none — the whole data payload is ignored | nothing |

`context` is recognized because displayed content always comes from
disk, never from the stream. A string type outside the five known events
is a valid decode with `KindUnknown` — not malformed — so unknown record
types are counted separately from schema violations (Issue #10; see
[record-robustness.md](record-robustness.md)).

## Text/bytes encodings

Every emitted value — path, lines, submatch match — is the rg union
`{"text": string}` or `{"bytes": base64}`. Both forms decode to the same
bytes for the same value, so encoding is a transport detail, not
identity: a `text` record and a `bytes` record carrying identical bytes
for the same path and line merge into one stop. Non-UTF-8 path bytes
survive intact via the `bytes` form.

## Schema ranges

`line_number` must be an integer `>= 1`; each submatch requires a
`match` value plus integer `start`/`end` satisfying
`0 <= start <= end <= len(decoded lines)`. Integers must fit `int64` —
floats, strings, and exponents are malformed. Violations return an error
wrapping `ErrMalformed`; `Feed` skips and counts them per Issue #10
(see [record-robustness.md](record-robustness.md)).
Cross-record lifecycle integrity — begin/end pairing, summary
positioning — is validated since Issue #9 inside `Index` itself, so the
parser deliberately stays per-record.

## Stream lifecycle validation

Since Issue #9, `Index` validates the stream's lifecycle as records are
`Add`ed: per-path open state keyed on decoded raw path bytes (so `text`
and `bytes` encodings of one path agree), independent state for
interleaved open files, and summary positioning. `Feed(stream)`
consumes the collected stdout, skipping and counting schema-failing
records (Issue #10) and flagging a trailing unterminated
record as both malformed and stream-incomplete. The full
transition matrix — duplicate `begin`, orphaned `match` (retained, with
`Stop.Incomplete` set), orphaned `end`, `missing end` sealing at
`Prepare`, missing/second `summary`, records after summary — and the
diagnostics it produces are documented in
[error-overlay-and-fatal-outcomes.md](error-overlay-and-fatal-outcomes.md),
as is binary exclusion's precedence over orphan retention.
`IntegrityFailures()` returns the stream-integrity diagnostics, kept
separate from the child's exit status so the app assesses them
independently.

## Stops

The `Index` accumulates only match records. Each *stop* is one
navigation target keyed by (raw path bytes, line number): same-path/
same-line records merge into one stop whose `Submatches` are sorted by
`(start, end)` with byte-identical duplicates compacted. Overlapping
ranges are all retained; `Prepare` computes `Highlights` as the union of
the submatch ranges (overlapping and adjacent ranges merge into single
coverage spans).

Ordering is unsigned lexicographic raw path bytes, then ascending line
number — deterministic for non-UTF-8 paths against valid UTF-8 ones.
Each stop retains the raw emitted path bytes, the line number, the
submatch byte ranges, and the recorded submatch bytes (the latter for
Issue #29's stale line-number validation against loaded file contents).

`ResolvedPath` joins relative paths onto the invocation working
directory with a plain separator — no canonicalization — so `./rel.go`
and `a/../b.go` keep their emitted form. Identity and ordering always
use the raw bytes, never the resolved path.

## Navigation cursor

Since Issue #13 the `Index` also owns the single global matched-line
cursor (`cursor.go`): `Current()` returns the selected stop — the first
stop in index order at startup, by zero value — and `Next()`/`Prev()`
step circularly, reporting a `Step` with the destination stop plus
`Moved`/`FileChanged`/`Wrapped` flags. Zero- and one-stop indexes are
strict no-ops (`Moved` false). See
[match-navigation.md](match-navigation.md).

## Binary exclusion

Since Issue #8, an `end` record with a non-null `binary_offset` drops
its file and all stops collected from it; the `excluded` set keeps the
distinct-file tally exposed by `BinaryExcluded()`, and `LineCount()` —
retained stops after filtering — is the usable-results value the app
outcome consumes. See
[no-results-and-binary-exclusion.md](no-results-and-binary-exclusion.md).

## Tests

See [unit-tests.md](unit-tests.md) § `internal/searchindex`.
