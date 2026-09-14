All seven findings in critique-4 are already repaired in the current issue files — no edits are needed. Here's the history and the current state:

Already applied. Commit e967155 ("Added updated issues from fourth issue critique") implemented the critique's recommended corrections:

F1 (tab ownership): Issue 5 now says "the structural eight-column-stop rule arrives in Issue 16" in both places, and Issue 22 gained the explicit scope note that tab expansion is owned by Issue 16. @/home/chris/vrg/Notes/issues/005-browse-tracer-file-list-and-file-panel.md:17 @/home/chris/vrg/Notes/issues/022-line-terminators-final-line-empty-file-utf8-bom.md:14
F2 (cached-file stale layout): Issue 17 owns the new trigger — request a prepared layout when navigating to a cached file with no layout matching current (path, revision, width, mode) — plus the gated model test and an acceptance criterion; Issue 13 references it. @/home/chris/vrg/Notes/issues/017-logical-anchor-through-rewrap-and-resize.md:26,40,54 @/home/chris/vrg/Notes/issues/013-match-navigation-n-p-circular-cursor.md:16
F3 (Issue 24 dependency): Blocked by is now Issue 17 only. @/home/chris/vrg/Notes/issues/024-file-list-layout-width-truncation-toggle.md:4
F4 (orphaned match after binary end): Issue 9 gained the "Binary exclusion precedence" bullet — late match is an integrity failure and is not retained. @/home/chris/vrg/Notes/issues/009-error-overlay-and-fatal-outcomes.md:35
F7 (tab placeholder): Issue 5 specifies a single → glyph occupying one cell, with provisional byte→cell mappings and tab-cell assertions deferred to Issue 16. @/home/chris/vrg/Notes/issues/005-browse-tracer-file-list-and-file-panel.md:17
F5 and F6 were resolved differently than critique-4 recommended — deliberately. Critique-4 suggested the oversized-unterminated record be counted as oversized (not malformed), and the post-summary unknown type not be counted as unknown. Commit e967155 applied exactly that, but PRD-issues-critique-5.md then found those dispositions contradict the PRD (its trailing-unterminated and unknown-type rules carry no exceptions) and reopened them. Commit 2219e1c settled them per the PRD:

Oversized record without trailing newline → counted as oversized and malformed and stream incomplete.
Unknown type after summary → counted in the unknown-type count and flagged as an integrity failure.
Both have matching tests and acceptance criteria in Issue 10, and critique-6 confirmed the classifications now agree with the PRD. @/home/chris/vrg/Notes/issues/010-record-robustness-malformed-oversized-unknown.md:15-16,33-34

So the critique-4 "Ready when" conditions are all met. Reverting F5/F6 to critique-4's wording would reintroduce the PRD inconsistency critique-5 caught — I'd recommend leaving them as-is, but let me know if you'd like anything changed.
