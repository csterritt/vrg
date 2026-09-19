# Search collection: spawn, drainage, searching screen (Issue #3)

The first real Bubble Tea application path, delivered by
[Issue #3](../issues/003-spawn-rg-collect-results-searching-screen.md):
spawn `rg`, collect its JSON event stream into the SearchIndex, hold a
"Searching…" screen for the whole of collection **and** post-exit index
preparation, then show the interim summary. Relevant PRD sections:
*Implementation Decisions → Invocation and child arguments* (bullets
7–9), *Result index, records, and stream integrity* (bullets 1–3), and
*Module Design → SearchIndex / App* in [`Notes/PRD-vrg.md`](../PRD-vrg.md).

## Child argv and working directory

`cmd/vrg` hands `cli.Result.ChildArgs` — the protected vector
`--json --no-config <ordered user flags> -- <pattern> <root>` built by the
[Issue #2 CLI](cli-flags-child-argv.md) — to `app.Run`, which spawns `rg`
from `PATH` with that argv. The child runs with `cmd.Dir` set to the
invocation working directory (`os.Getwd()`), and relative result paths
resolve against that same directory — never by prepending the supplied
root again, and never canonicalized.

## Dual-pipe drainage

`internal/app/rg.go`'s `spawn` wires both `StdoutPipe` and `StderrPipe`,
starts the child, then a collector goroutine drains **both pipes
concurrently for the whole child lifetime**: two `io.Copy` goroutines
read to EOF before `cmd.Wait()` reaps the child (the correct ordering —
`Wait` must not run before the reads complete). Neither pipe can fill and
block rg; stderr bytes are buffered without backpressure. Terminating the
child closes the pipes, so the copies end promptly rather than waiting
for further output — the structure Issue #4's cancellation rides on.
Classification and presentation of the captured stderr remain Issue #9's.

## Searching state and the test gate

The model (`internal/app/app.go`) opens in `stateSearching`, rendering
"Searching…". `Init` returns one command that runs entirely off the UI
update path: `child.Wait()` (blocks until rg exits **and** both pipes are
drained), then the optional collect-acknowledgement and preparation gate,
then `searchindex.Build`. Because the whole sequence is a single command,
the model provably stays searching until the index is ready — even after
rg itself has exited. Resize messages are ordinary updates and never
block on collection. `q` while searching — including that gate-held
post-exit window — is Issue #4's cancellation, exiting 130; see
[cancellation-cleanup.md](cancellation-cleanup.md).

Two seams, delivered as `app.Option`s, support tests:

- `WithGate(fn)` — runs after collection completes and before index
  preparation; a test can hold preparation independently of rg exit.
- `WithCollectAck(fn)` — runs once collection completes, before the gate.

`cmd/vrg` wires them from environment variables: `VRG_TEST_GATE=<file>`
holds preparation while the file exists (a polled watcher), and
`VRG_TEST_COLLECT_ACK=<file>` appends a line when collection completes,
so a test can observe "rg exited, stream collected, preparation still
held". Issue #4 added `VRG_TEST_REAP` and `VRG_TEST_FAIL` (see
[cancellation-cleanup.md](cancellation-cleanup.md)). Issue #45 will move
these env-var seams behind the `vrg_testhooks` build tag.

## Interim summary and start failure

Completion delivers `searchDoneMsg` carrying the collected result and the
prepared index; the model transitions to `stateSummary`, which renders
`N files, M matched lines`. `q` there returns `tea.Quit` with the fixed
exit status 0. Later issues replace this screen with browsing.

A start failure (rg absent, exec error) is detected in `app.Run` **before**
`tea.NewProgram` runs: a sanitized single-line diagnostic
(`vrg: cannot start ripgrep: …`, via `cli.Escape`) goes to stderr and the
process exits 2 — no TUI is entered.

## SearchIndex record rules

`internal/searchindex` consumes the newline-delimited JSON stream.

- **Records**: `ParseRecord` decodes one record and classifies it —
  `begin`, `match`, `end`, `summary`, `context` (known, data ignored), a
  string `type` outside the five (`KindUnknown`), or `KindMalformed`
  (invalid JSON, invalid base64, missing/non-string `type`, or a known
  event failing the required-field/range matrix). Validation reads
  exactly the required fields; matrix-ignored fields such as
  `absolute_offset` and `stats` are never consulted.
- **Encodings**: `{"text": …}` and `{"bytes": base64}` both decode to raw
  bytes for paths, lines, and submatch text; identical bytes are the same
  value, so a `text` record and a `bytes` record merge by content.
- **Merging**: match records sharing a raw path and `line_number` merge
  into one `Stop`; submatches sort by `(start, end)`; overlapping
  submatches are all retained and `Highlights` holds their union
  coverage (touching ranges merge).
- **Retention**: raw path bytes, line number, decoded line bytes, submatch
  byte ranges, and recorded submatch bytes are all kept.
- **Ordering**: files sort by unsigned raw path bytes (`bytes.Compare`),
  so high/invalid-UTF-8 bytes order deterministically after ASCII;
  stops sort by ascending line number.
- **Resolution**: relative result paths are byte-joined onto the
  invocation working directory (`wd + "/" + path`) without
  canonicalization — interior `.`/`..` elements stay literal.

Lifecycle/integrity accounting (begin/end pairing, binary exclusion,
summary completeness) is Issue #9's; skip/oversize counting is Issue
#10's; only `match` records build stops today.

## Tests

See [unit-tests.md](unit-tests.md): `internal/searchindex/index_test.go`
covers the record/index contracts; `internal/app/app_test.go` covers the
model lifecycle and `rg_test.go` the spawn seam; `cmd/vrg/search_test.go`
drives the real binary on a pty (`runVrgWithQuit`) for the exact child
argv/workdir, the ≥1 MiB dual-pipe backpressure fixture, stderr capture,
the gate-held searching state, and `TestStartFailureExit2`.
