# Issue #41: Error overlays keep every row scrollable — no head/tail compression

*2026-09-25T01:21:28Z by Showboat 0.6.1*
<!-- showboat-id: dc9b4c11-a6a4-4f63-9b80-6fae2baa09ec -->

Walkthrough for [Issue #41](../../../tasks/041-overlay-full-scroll-no-head-tail-compression.md): non-help error overlays keep every wrapped row of the diagnostic in the scrollable set, per `Notes/PRD-vrg.md` (*Colours, overlays, and key precedence* — `up`/`down` scroll rendered rows, new errors append preserving the reader's position). The scrollable row set is the complete wrapped diagnostic — no head-plus-ellipsis-plus-tail compression, nothing elided from the model — with `scroll` clamped to `[0, max(0, rows − interiorH)]` in both the key handler (`scrollOverlay`) and the render path (`layout` inside `renderOverlay`). Render-time clipping at tiny terminal sizes (story 83) still applies inside `composite`; the help overlay's scrolling is unchanged. This supersedes Issue #9's simultaneous head/tail rendering requirement for the ≥ 1 MiB stderr fixture — drainage, complete-stdout, and captured-stderr assertions are retained while tail reachability moved to model-level tests. All generated artifacts live in this directory: the built `vrg` binary, the `demo-full-scroll.sh` tmux harness, and its `manual-full-scroll/` captures. Test durations are stripped so the document verifies cleanly.

```bash
cd /home/chris/vrg && go vet ./... && go build ./... && go test -count=1 ./... | sed 's/[[:space:]][0-9.]*s$//' && echo GATES-OK
```

```output
ok  	vrg/cmd/vrg
ok  	vrg/internal/app
ok  	vrg/internal/cli
ok  	vrg/internal/filebuffer
ok  	vrg/internal/present
ok  	vrg/internal/searchindex
ok  	vrg/internal/theme
ok  	vrg/internal/viewport
GATES-OK
```

## Model-level contract tests

`internal/app/overlay_test.go` pins the complete-scrollable-row contract. `TestOverlayScrollableSetIsCompleteDiagnostic` drives a ≥ 1 MiB stderr shape (`floodStderr`: a head marker, 525 numbered 1999-cell lines, a tail marker) and requires `layout`'s row set joined to reproduce every diagnostic line in order — first and last markers intact, no elision row injected — with `maxScroll` exactly `rows − interiorH`. `TestOverlayScrollClampsToCompleteSet` proves the clamp in both places it is enforced — `down` at `maxScroll − 1` still moves and one more is a no-op in the key handler, while stored offsets past either end render the clamped tail or head — and a middle row renders at its own scroll position, so tail and middle reachability hold for an arbitrarily long diagnostic without thousands of key presses. `TestOverlayTraversalReachesBothEnds` runs a bounded row-by-row traversal on a fixture only slightly taller than the interior. `TestAppendedErrorExtendsScrollableSet` shows an appended error joins the set's tail, renderable at the clamped bottom, without moving the reader. `TestOverlayIgnoresOtherKeys` gained the browse scroll keys `u`/`d`/`pgup`/`pgdown` with the scroll position asserted unmoved.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestOverlayScrollableSetIsCompleteDiagnostic|TestOverlayScrollClampsToCompleteSet|TestOverlayTraversalReachesBothEnds|TestAppendedErrorExtendsScrollableSet|TestOverlayScrollsWithUpDown|TestOverlayIgnoresOtherKeys' ./internal/app 2>&1 | grep -vE '^(=== RUN|    --- PASS)' | sed -E 's/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//'
```

```output
--- PASS: TestOverlayScrollsWithUpDown
--- PASS: TestOverlayScrollableSetIsCompleteDiagnostic
--- PASS: TestOverlayScrollClampsToCompleteSet
--- PASS: TestOverlayTraversalReachesBothEnds
--- PASS: TestAppendedErrorExtendsScrollableSet
--- PASS: TestOverlayIgnoresOtherKeys
PASS
ok  	vrg/internal/app
```

## The revised stderr-content fixture

`cmd/vrg/pty_test.go`'s `TestPTYStderrContentFixture` (Issue #9, revised here) floods over 1 MiB of stderr interleaved with a valid stdout stream at an ordinary 80×24 PTY. It retains the issue's real assertions — the `writes-done` handshake proves the child finished both pipes (drainage), `ERRHEAD-MARKER` in the overlay proves captured-stderr inclusion, the revealed browse view proves complete stdout, and the exit status stays 0 — while the simultaneous head/tail frame is gone: tail reachability is the model-level tests' job.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestPTYStderrContentFixture' ./cmd/vrg 2>&1 | grep -vE '^(=== RUN|    --- PASS)' | sed -E 's/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//'
```

```output
--- PASS: TestPTYStderrContentFixture
PASS
ok  	vrg/cmd/vrg
```

## Manual check — scroll a fatal diagnostic end to end

`demo-full-scroll.sh` (checked into this directory) runs the freshly built `vrg` on a real tmux PTY at 80×24 in a fixture whose fake `rg` emits a complete one-match stream, writes 40 numbered stderr lines — 40 rows against an interior height of 22, `maxScroll` 18 — and exits 3. The fatal error overlay opens over browse at the head. The harness then: confirms `u`/`d`/`PPage`/`NPage` leave the pane byte-identical (ignored); sends 9 `Down`s to a mid position (every middle row reachable, no ellipsis substituting for content); 9 more to the last row `err-39`; 5 extra `Down`s proving the bottom clamp; 18 `Up`s back to the first row and 3 extra proving the top clamp; then `q` dismisses to browse and a second `q` quits at the fixed fatal status 2.

```bash
cd /home/chris/vrg && go build -o Notes/walkthroughs/041-04/code-walkthrough/vrg ./cmd/vrg && cd Notes/walkthroughs/041-04/code-walkthrough && ./demo-full-scroll.sh
```

```output
ok: tail hidden before scrolling
ok: u ignored (pane unchanged)
ok: d ignored (pane unchanged)
ok: PPage ignored (pane unchanged)
ok: NPage ignored (pane unchanged)
ok: middle row err-15 reachable
ok: head scrolled off at mid position
ok: no ellipsis row in the scrollable set
ok: head left the frame at the bottom
ok: no ellipsis row at the bottom
ok: scroll clamped at the bottom
ok: up traversal returned to the first row
ok: scroll clamped at the top
exit=2
--- overlay at open (top) ---
f1.txt  ── f1.txt ──────────────────┌──────┐────────────────────────────────────
        1  hit one                  │err-00│
                                    │err-01│
                                    │err-02│
                                    │err-03│
                                    │err-04│
                                    │err-05│
                                    │err-06│
                                    │err-07│
                                    │err-08│
                                    │err-09│
                                    │err-10│
                                    │err-11│
                                    │err-12│
                                    │err-13│
                                    │err-14│
                                    │err-15│
                                    │err-16│
                                    │err-17│
                                    │err-18│
                                    │err-19│
                                    │err-20│
                                    │err-21│
                                    └──────┘
--- overlay mid-scroll ---
f1.txt  ── f1.txt ──────────────────┌──────┐────────────────────────────────────
        1  hit one                  │err-09│
                                    │err-10│
                                    │err-11│
                                    │err-12│
                                    │err-13│
                                    │err-14│
                                    │err-15│
                                    │err-16│
                                    │err-17│
                                    │err-18│
                                    │err-19│
                                    │err-20│
                                    │err-21│
                                    │err-22│
                                    │err-23│
                                    │err-24│
                                    │err-25│
                                    │err-26│
                                    │err-27│
                                    │err-28│
                                    │err-29│
                                    │err-30│
                                    └──────┘
--- overlay at the bottom ---
f1.txt  ── f1.txt ──────────────────┌──────┐────────────────────────────────────
        1  hit one                  │err-18│
                                    │err-19│
                                    │err-20│
                                    │err-21│
                                    │err-22│
                                    │err-23│
                                    │err-24│
                                    │err-25│
                                    │err-26│
                                    │err-27│
                                    │err-28│
                                    │err-29│
                                    │err-30│
                                    │err-31│
                                    │err-32│
                                    │err-33│
                                    │err-34│
                                    │err-35│
                                    │err-36│
                                    │err-37│
                                    │err-38│
                                    │err-39│
                                    └──────┘
--- browse after dismissal ---
f1.txt  ── f1.txt ──────────────────────────────────────────────────────────────
        1  hit one
```
