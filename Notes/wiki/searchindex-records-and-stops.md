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
types can be counted separately from schema violations (Issue #10).

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
wrapping `ErrMalformed`; skipping and counting them is Issue #10's, and
cross-record lifecycle integrity (begin/end pairing, summary
cross-checks) is Issue #9's — the parser deliberately stays per-record.

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

## Tests

See [unit-tests.md](unit-tests.md) § `internal/searchindex`.
