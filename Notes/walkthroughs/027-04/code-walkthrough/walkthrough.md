# Issue #27: Explicit reload — r rereads the current file

*2026-09-24T16:53:26Z by Showboat 0.6.1*
<!-- showboat-id: 5d364c6b-31c4-4a6e-b1c7-7eefca319b7e -->

Walkthrough for [Issue #27](../../../issues/027-explicit-reload-r.md), implementing the explicit-reload contract per `Notes/PRD-vrg.md` (*File loading, cache, reload, and selection consistency*): `r` rereads the current file from disk without rerunning rg or touching the cursor stops; a duplicate `r` or a re-entry while that path's load is in flight is dropped, not queued, with the placeholder's settlement the only completion signal; the cached buffer is dropped immediately so stale content can never present as refreshed; the cursor and logical viewport anchor are preserved — clamped to the new content — and committed only when the new revision's matching prepared layout installs through the Issue #17 path (the `pendingIntent` seam this issue owns and Issue #28 generalizes); a failed reload replaces the display with `(unreadable)` plus the Issue #26 failure overlay, a second consecutive failure appending exactly one scroll-preserving occurrence; and `r` is the one-stop index's only retry route. All generated artifacts live in this directory: the built `vrg` binary, the `fixture-src/` files, the `demo-reload.sh` tmux harness, and its `reload/` session captures. Test durations are stripped so the document verifies cleanly.

```bash
set -o pipefail; cd /home/chris/vrg && test -z "$(gofmt -l internal/ cmd/)" && echo GOFMT-CLEAN && go vet ./... && go build ./... && go test -count=1 ./... | sed "s/[[:space:]][0-9.]*s$//" && CGO_ENABLED=1 go test -race -count=1 ./internal/app | sed "s/[[:space:]][0-9.]*s$//" && go build -o Notes/walkthroughs/027-04/code-walkthrough/vrg ./cmd/vrg && echo GATES-OK
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
GATES-OK
```

## Gated model tests — `internal/app/reload_test.go`

The contracts are pinned at the model seam: the tests drive `Update` through `loaderModel`/`gatedLoaderModel` (the Issue #26 injected `readFile` seam) plus `loadGate`, and hold the layout worker by retaining the returned command. `TestExplicitReloadRereadsCurrentFile` covers exactly one reread, `Loading…`, the intact filename row, and the untouched cursor and stops; `TestReloadDuplicateDroppedNotQueued` pins the dropped-not-queued rule — for a duplicate `r` and a mid-load re-entry alike — with placeholder settlement gating the next `r`; `TestReloadPreservesAnchorThroughMatchingLayout` asserts the anchor and top only after the new revision's matching layout installs, never against the superseded one; `TestReloadAnchorClampsToShrunkContent` covers the lossy EOF clamp; `TestFailedReloadReplacesContent` and `TestReloadSecondFailureAppendsPreservingScroll` exercise the Issue #26 failure path (the `(unreadable)` replacement, the reopened overlay, one appended occurrence); `TestReloadIsTheOneStopRetryRoute` proves `r` is the one-stop retry route where `n`/`p` are no-ops; `TestDiskChangeWithoutRIsNotObserved` pins cache stability until `r`; `TestPreReloadLayoutDiscardedAfterReload` is the Issue #17 revision-superseded case — a held pre-reload layout released after the reload completes is discarded without touching the panel or the anchor; and `TestNavigationDuringReloadOverridesAnchor` proves a selection made during the in-flight load wins over the anchor intent, the precedence Issue #28 generalizes.

```bash
cd /home/chris/vrg && go test -count=1 -v -run "TestExplicitReloadRereadsCurrentFile|TestReloadDuplicateDroppedNotQueued|TestReloadPreservesAnchorThroughMatchingLayout|TestReloadAnchorClampsToShrunkContent|TestFailedReloadReplacesContent|TestReloadSecondFailureAppendsPreservingScroll|TestReloadIsTheOneStopRetryRoute|TestDiskChangeWithoutRIsNotObserved|TestPreReloadLayoutDiscardedAfterReload|TestNavigationDuringReloadOverridesAnchor" ./internal/app 2>&1 | grep -vE "^(=== RUN|=== CONT)" | sed -E "s/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//"
```

```output
--- PASS: TestExplicitReloadRereadsCurrentFile
--- PASS: TestReloadDuplicateDroppedNotQueued
--- PASS: TestReloadPreservesAnchorThroughMatchingLayout
--- PASS: TestReloadAnchorClampsToShrunkContent
--- PASS: TestFailedReloadReplacesContent
--- PASS: TestReloadSecondFailureAppendsPreservingScroll
--- PASS: TestReloadIsTheOneStopRetryRoute
--- PASS: TestDiskChangeWithoutRIsNotObserved
--- PASS: TestPreReloadLayoutDiscardedAfterReload
--- PASS: TestNavigationDuringReloadOverridesAnchor
PASS
ok  	vrg/internal/app
```

## Manual route — the real binary on a PTY

`demo-reload.sh` (checked into this directory) runs the freshly built `vrg` on a real tmux PTY with real `rg`, in two sessions. Session A copies `a-long.txt` (80 lines, stops at 2 and 70) and `b-short.txt` from `fixture-src/` into a disposable `mktemp` directory — repository and user files are never touched — and an EXIT trap removes it. After startup renders file a it scrolls a page, appends lines 81–90 to the file externally — the cached frame stays byte-identical, proving nothing watches the disk — then presses `r`: the reread installs the new bytes at the same top row, and scrolling to EOF shows the appended lines. Deleting the file and pressing `r` shows `(unreadable)` under the error overlay; restoring it and pressing `r` reopens the retained prior-failure overlay over the reloaded content, exactly like Issue #26's re-entry. A writerless-FIFO swap then holds the reread's `open()` so `Loading…` and the filename row paint deterministically — the transient state a fast local read never lets a PTY capture — a duplicate `r` while the read blocks is dropped (a queued press would re-block on the FIFO after settlement, so the settled frame staying settled is the observable proof), and a writer pulse settles it to content. `q` exits 0 with the one collected failure replayed on stderr. Session B opens a one-stop fixture where `n` is a strict no-op — so `r` is the only retry route — and `r` still reloads an external append. The deterministic model tests remain the authority for the in-flight and commit semantics.

```bash
cd /home/chris/vrg/Notes/walkthroughs/027-04/code-walkthrough && unset RIPGREP_CONFIG_PATH && ./demo-reload.sh
```

```output
ok: startup: a-long.txt renders
ok: startup: filename rule names the path
ok: pgdown scrolls -> moved
ok: external append leaves display unchanged -> identical
ok: r: same top position -> line-24
ok: r: appended lines are loaded
ok: r on deleted file: failure overlay opens
ok: r on deleted file: (unreadable) placeholder
ok: Esc: overlay dismissed
ok: Esc: (unreadable) stays
ok: restore + r: prior failure overlay reopens
ok: restore + r: content reloaded behind it
ok: restore + r + Esc: overlay gone
ok: restore + r + Esc: (unreadable) cleared
ok: restore + r + Esc: content readable
ok: r on fifo: Loading… while the read blocks
ok: r on fifo: filename row still names the path
ok: duplicate r mid-load: still just Loading…
ok: fifo settled: no queued second reread blocks
ok: fifo settled: no (unreadable)
ok: fifo settled: content painted
ok: session A: vrg exit status -> 0
ok: session A: stderr replays one failure -> 1
ok: one-stop index: solo.txt renders
ok: one-stop index: n is a strict no-op -> identical
ok: external append: still the cached display
ok: one-stop index: r reloads
ok: session B: vrg exit status -> 0
demo-reload: all checks passed
```

```bash
cd /home/chris/vrg/Notes/walkthroughs/027-04/code-walkthrough && strip="s|/tmp/vrg27-fixture-[ab]\.[A-Za-z0-9_]*/||g" && echo "== scrolled: top row at line 24 ==" && sed -n "1,4p" reload/screen-01-scrolled.txt | sed "$strip; s/ *$//" && echo "== external append: display unchanged ==" && sed -n "1,4p" reload/screen-02-append-unchanged.txt | sed "$strip; s/ *$//" && echo "== r: same top position ==" && sed -n "1,4p" reload/screen-03-after-r.txt | sed "$strip; s/ *$//" && echo "== EOF after r: the appended lines ==" && sed -n "21,24p" reload/screen-04-eof-appended.txt | sed "$strip; s/ *$//" && echo "== r on the deleted file: overlay over (unreadable) ==" && sed -n "1,3p;11,14p" reload/screen-05-deleted-overlay.txt | sed "$strip; s/ *$//" && echo "== restore + r: prior overlay reopened over reloaded content ==" && sed -n "1,3p;11,14p" reload/screen-07-restore-overlay.txt | sed "$strip; s/ *$//" && echo "== r on a writerless FIFO: Loading… held, path still named ==" && sed -n "1,3p" reload/screen-09-loading.txt | sed "$strip; s/ *$//" && echo "== FIFO writer pulse: settled content, nothing queued ==" && sed -n "1,3p;21,24p" reload/screen-10-fifo-settled.txt | sed "$strip; s/ *$//" && echo "== one-stop index: r reloads ==" && sed -n "1,12p" reload/screen-12-solo-reloaded.txt | sed "$strip; s/ *$//" && echo "== q: exit 0, the failure replayed on stderr ==" && cat reload/stderr-a.txt | sed "$strip"
```

```output
== scrolled: top row at line 24 ==
./a-long.txt   ── ./a-long.txt ─────────────────────────────────────────────────
./b-short.txt  24  line-24
               25  line-25
               26  line-26
== external append: display unchanged ==
./a-long.txt   ── ./a-long.txt ─────────────────────────────────────────────────
./b-short.txt  24  line-24
               25  line-25
               26  line-26
== r: same top position ==
./a-long.txt   ── ./a-long.txt ─────────────────────────────────────────────────
./b-short.txt  24  line-24
               25  line-25
               26  line-26
== EOF after r: the appended lines ==
               87  line-87 appended
               88  line-88 appended
               89  line-89 appended
               90  line-90 appended
== r on the deleted file: overlay over (unreadable) ==
./a-long.txt   ── ./a-long.txt (unreadable) ────────────────────────────────────
./b-short.txt     (unreadable)

 ┌───────────────────────────────────────────────────────────────────────────┐
 │cannot read ./a-long.txt: open ./a-long.txt: no│
 │such file or directory                                                     │
 └───────────────────────────────────────────────────────────────────────────┘
== restore + r: prior overlay reopened over reloaded content ==
./a-long.txt   ── ./a-long.txt ─────────────────────────────────────────────────
./b-short.txt  58  line-58
               59  line-59
 ┌───────────────────────────────────────────────────────────────────────────┐
 │cannot read ./a-long.txt: open ./a-long.txt: no│
 │such file or directory                                                     │
 └───────────────────────────────────────────────────────────────────────────┘
== r on a writerless FIFO: Loading… held, path still named ==
./a-long.txt   ── ./a-long.txt ─────────────────────────────────────────────────
./b-short.txt     Loading…

== FIFO writer pulse: settled content, nothing queued ==
./a-long.txt   ── ./a-long.txt ─────────────────────────────────────────────────
./b-short.txt  58  line-58
               59  line-59
               77  line-77
               78  line-78
               79  line-79
               80  line-80
== one-stop index: r reloads ==
./solo.txt  ── ./solo.txt ──────────────────────────────────────────────────────
             1  s-01
             2  s-02
             3  s-03
             4  s-04
             5  MARK solo s-005
             6  s-06
             7  s-07
             8  s-08
             9  s-09
            10  s-10
            11  s-appended after reload
== q: exit 0, the failure replayed on stderr ==
cannot read ./a-long.txt: open ./a-long.txt: no such file or directory
```

## Verdict

Issue #27 is verified. On the real binary, scrolling to line 24 then appending ten lines externally left the frame byte-identical — no watcher, no reread — until `r` repainted the same top row with the new bytes, the appended lines visible at EOF. Deleting the file and pressing `r` produced `(unreadable)` under the error overlay naming the path, the filename row carrying the `(unreadable)` status note; restoring it and pressing `r` reopened the retained prior failure and settled to readable content, the anchor landing on the same top row (58) it held before the swap. The FIFO-held reread painted `Loading…` beside the filename row, a duplicate `r` mid-load minted nothing — the settled frame stayed settled, so nothing was queued — and the one-stop session reloaded via `r` where `n` is a strict no-op. `q` exited 0 with the single failure replayed on stderr, the fixed status untouched. The gated model tests are the deterministic authority for the anchor-commit-after-matching-layout, shrink-clamp, revision-supersession, and dropped-duplicate contracts.

