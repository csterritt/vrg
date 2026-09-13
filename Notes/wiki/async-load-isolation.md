# Asynchronous load isolation (Issue #25)

Issue #25 added keyed asynchronous load isolation so the TUI stays
navigable while file loads are pending, late completions update only
their own file, and at most one load is in flight per raw path.
Relevant PRD section: *File loading, cache, reload, and selection
consistency*. See [browse-tracer](browse-tracer.md) for the base
browse view and [logical-anchor-and-layout-preparation](logical-anchor-and-layout-preparation.md)
for the separately-gated decode/map phase.

## Problem

Before Issue #25, the model had a single global `loading` flag and a
single `loadCancel` channel. `FileLoadCompleteMsg` carried a raw path
but no request identity. The completion handler unconditionally
assigned the completed buffer to `m.buffer` and cleared `m.loading`,
regardless of whether that path was still the current file. This worked
for simple sequential loads but did not safely support A→B→C
navigation with out-of-order completions: a late completion for A
arriving while C was current would overwrite C's panel with A's
content. There was also no mechanism to prevent a second load for a
path already in flight.

## Solution

Issue #25 added per-path request identity and in-flight tracking:

- `FileLoadCompleteMsg` carries a `RequestID uint64` identifying the
  load request that produced it.
- `Model.loadingPaths map[string]uint64` tracks in-flight loads by raw
  path, mapping to the active request ID.
- `Model.loadRequestID` is the monotonically increasing request
  identity counter.
- `startLoad(path) (Model, tea.Cmd)` is the single entry point for
  starting a load. It enforces the one-load-per-path rule, assigns a
  fresh request ID, records the in-flight request, and returns the
  updated model and the load command.
- `loadFileFor(path, requestID)` tags the completion message with the
  request identity.

### Navigation during loads

Navigation (`n`/`p`) remains fully active while a file loads. The
cursor advances immediately and the panel switches to the
destination. For an uncached destination, the panel shows the
`Loading…` placeholder while the load proceeds asynchronously. The
user can move past a slow file without waiting for it. Placeholder
scrolling is a strict no-op (the viewport is `nil` while loading).
Other keys (`w`, `c`, resize) retain their normal meanings while a
load is pending.

### Keyed late completions

A `FileLoadCompleteMsg` is accepted only when its `RequestID` matches
the in-flight request for its path in `loadingPaths`. Stale
completions (from a cancelled or superseded request, or a path whose
load already completed) are discarded without touching the cache or
the visible panel. On acceptance:

1. The path is removed from `loadingPaths` (no longer in flight).
2. The buffer is cached in `fileCache` keyed by raw path, regardless
   of whether the path is still current.
3. The visible panel is updated only when the completion's path is
   still the current path (`bytes.Equal(msg.Path, m.currentPath)`).
   A late completion for a non-current file updates only that file's
   cache, leaving the visible panel and loading state untouched.

### A→B→C scenario

Navigate from A to B to C. A's completion arrives while C is current.
C's panel remains unchanged (still loading). A is cached. When the
user navigates back to A, its content shows from the cache without a
new load.

### One load per raw path

`startLoad` checks `loadingPaths` before starting a load. If a load is
already in flight for the path, it starts no second load and queues
nothing. The panel shows the loading placeholder; the existing load's
completion will update the cache and, if the path is still current,
the panel. Re-entering a loading path is a no-op for the loader.

### Session-long buffer retention

Successful buffers are retained in `fileCache` for the entire session.
No eviction occurs. A revisited cached file shows its content
immediately without a new load. The loader is called at most once per
path per session.

### Post-cancellation rejection

Late `FileLoadCompleteMsg` messages arriving after cancellation
(`ctrl+c` or `q`) are ignored. The model stays cancelled and is not
revived. The global `m.cancelled` check runs before the request
identity check, so any completion after cancellation is discarded.

### Separately gated decode/map phase

The decode/map phase (the file gate) is separately gatable from
navigation. While the current file's load is held at the file gate,
all of these remain actionable: `ctrl+c` (exit 130), `n` (next),
`p` (previous), `w` (wrap toggle), `c` (theme toggle), and terminal
resize. The file gate holds the load goroutine; it does not block
`Update` or input handling.

## Model fields

- `loadRequestID uint64` — the next request identity for file loads.
- `loadingPaths map[string]uint64` — in-flight loads by raw path,
  mapping to the active request ID. A path is removed when its load
  completes or is cancelled.

## Message contract

```go
type FileLoadCompleteMsg struct {
    Path      []byte
    RequestID uint64
    Buffer    *filebuffer.Buffer
    Err       error
}
```

The `RequestID` is set by `startLoad` and carried through
`loadFileFor` into the completion message. `Update` validates it
against `loadingPaths` before mutating state.

## Cancellation

The existing `loadCancel` channel and `cancelLoad()` mechanism are
unchanged. Cancellation signals the in-flight load goroutine to stop
waiting at the file gate. Late completions are rejected by the
`m.cancelled` check (global) and the `loadingPaths` request-identity
check (per-path). A completion for a path whose load was cancelled
will not match the in-flight request (the path was removed from
`loadingPaths` or the request ID was superseded), so it is discarded.

## Testing

See [unit-tests](unit-tests.md) for the `load_isolation_test.go`
catalog covering navigation during loads, keyed late completions,
one-load-per-path, cached revisits, post-cancellation rejection, and
input responsiveness during the decode/map phase.
