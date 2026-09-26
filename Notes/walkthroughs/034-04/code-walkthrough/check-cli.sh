#!/bin/bash
# Issue #34 manual check: the README's command-line claims against the
# real vrg binary — the embedded generated-help block is byte-identical
# to `vrg --help`, bare `vrg` is the help-only exit-0 path, and the
# usage-error exits are 2. Captures land under ./run in this directory.
set -u
D="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
R="$(CDPATH= cd -- "$D/../../../.." && pwd)"
W="$D/run"
mkdir -p "$W"
FAIL=0
check() {
	if [ "$2" = "$3" ]; then echo "ok: $1 -> $2"; else echo "FAIL: $1 -> got [$2] want [$3]"; FAIL=1; fi
}

# The README's fenced help block must equal `vrg --help` byte for byte.
"$D/vrg" --help >"$W/help-actual.txt" 2>"$W/help-stderr.txt"
check "vrg --help exit status" "$?" "0"
awk 'BEGIN { f = 0 }
     /^```text$/ { f = 1; next }
     /^```$/ { if (f) exit }
     f { print }' "$R/README.md" >"$W/help-readme.txt"
check "README help block == generated help" \
	"$(cmp -s "$W/help-readme.txt" "$W/help-actual.txt" && echo identical || echo different)" "identical"
check "--help stderr empty" "$(test ! -s "$W/help-stderr.txt" && echo empty)" "empty"

# Bare vrg: the help-only path — same help on stdout, exit 0, no
# search, no TUI, nothing on stderr.
"$D/vrg" </dev/null >"$W/bare-out.txt" 2>"$W/bare-err.txt"
check "bare vrg exit status" "$?" "0"
check "bare vrg stdout == --help" \
	"$(cmp -s "$W/bare-out.txt" "$W/help-actual.txt" && echo identical || echo different)" "identical"
check "bare vrg stderr empty" "$(test ! -s "$W/bare-err.txt" && echo empty)" "empty"

# Usage-error exits: the flags-only invocation, an `=` assignment
# spelling, and a nonexistent root all exit 2 with the sanitized
# diagnostic plus help on stderr and nothing on stdout.
"$D/vrg" -i </dev/null >"$W/flagsonly-out.txt" 2>"$W/flagsonly-err.txt"
check "vrg -i exit status" "$?" "2"
check "vrg -i stdout empty" "$(test ! -s "$W/flagsonly-out.txt" && echo empty)" "empty"
check "vrg -i diagnostic" "$(head -1 "$W/flagsonly-err.txt")" "vrg: missing required argument PATTERN"
check "vrg -i stderr carries help" \
	"$(grep -cF 'Usage: vrg [OPTIONS] PATTERN [ROOT]' "$W/flagsonly-err.txt")" "1"

"$D/vrg" --ignore-case=false pat </dev/null >/dev/null 2>"$W/assign-err.txt"
check "vrg --ignore-case=false exit status" "$?" "2"
check "assignment rejected" "$(head -1 "$W/assign-err.txt")" "vrg: unsupported option --ignore-case=false"

"$D/vrg" pat /definitely/not/a/root </dev/null >/dev/null 2>"$W/root-err.txt"
check "vrg bad-root exit status" "$?" "2"
check "bad-root diagnostic" "$(head -1 "$W/root-err.txt")" \
	"vrg: invalid root /definitely/not/a/root: does not exist"

[ "$FAIL" = 0 ] && echo "check-cli: all checks passed" || exit 1
