#!/bin/bash
# Issue #25 manual check: navigation stays live while a file loads and
# a file's content never paints before its load completes — on the real
# vrg binary in a real tmux PTY.
#
# Fixture at 80x24: file A is ~1.5M lines (~95MB) — its read+decode/map
# takes several seconds — with MARK alpha at line 1; file B is three
# lines with MARK beta at line 1. The cursor starts on A's first stop.
#
# Manual case (issue #25):
#   press n immediately at startup -> B appears while A still loads
#   press p -> A shows only the "Loading…" placeholder, never content
#   navigation stays live (n back to B is instant — B is cached)
#   p -> A still the placeholder; when the load finishes A's content
#   arrives only then
set -u
D="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
W="$D/load"
rm -rf "$W"
mkdir -p "$W/fixture"

FA="a-big-slow-loading-file.txt"
FB="b-small-fast-file.txt"

# A: line 1 is the first stop; ~1.5M padding lines make the worker's
# read+decode/map take several seconds so the mid-load keys land while
# it is genuinely in flight.
python3 - "$W/fixture/$FA" <<'PY'
import sys
with open(sys.argv[1], "w") as f:
    f.write("MARK alpha\n")
    f.writelines("padline %07d xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx\n" % i
                 for i in range(2, 1_500_002))
PY
printf 'MARK beta\nx\nx\n' >"$W/fixture/$FB"

SES="vrg25-load-$$"
cleanup() {
	tmux kill-session -t "$SES" 2>/dev/null
	# The ~95MB fixture is regenerated on every run; keep only the
	# captured screens as artifacts.
	rm -f "$W/fixture/$FA"
}
trap cleanup EXIT

cat >"$W/inner.sh" <<EOF
#!/bin/sh
cd "$W/fixture"
unset RIPGREP_CONFIG_PATH
"$D/vrg" MARK .
echo "\$?" >"$W/exit.txt"
sleep 30
EOF
chmod +x "$W/inner.sh"
tmux new-session -d -s "$SES" -x 80 -y 24 "$W/inner.sh"
tmux set-option -t "$SES" window-size manual

ESC=$(printf '\033')
export LC_ALL=C.UTF-8
screen() { tmux capture-pane -p -t "$SES" 2>/dev/null; }
plain() { sed "s/${ESC}\[[0-9;]*m//g"; }
styled() { tmux capture-pane -p -e -t "$SES" 2>/dev/null; }

wait_for() { # wait_for <literal> anywhere on the pane
	for _ in $(seq 1 600); do
		screen | grep -qF "$1" && return 0
		sleep 0.05
	done
	echo "TIMEOUT waiting for: $1"; return 1
}

FAIL=0
check() {
	if [ "$2" = "$3" ]; then echo "ok: $1 -> $2"; else echo "FAIL: $1 -> got [$2] want [$3]"; FAIL=1; fi
}
present() { # present <desc> <literal>
	if screen | grep -qF "$2"; then echo "ok: $1"; else echo "FAIL: $1"; FAIL=1; fi
}
absent() { # absent <desc> <literal>
	if screen | grep -qF "$2"; then echo "FAIL: $1"; FAIL=1; else echo "ok: $1"; fi
}
snap() { screen | plain >"$W/$1"; }
key() { tmux send-keys -t "$SES" "$@"; }

# ============ browse opens on A still loading; n crosses to B at once
wait_for "Loading…" || exit 1
key n
key Escape; sleep 0.3 # dismiss the file-change pop-up
wait_for "MARK beta" || exit 1
present "n at startup: B's content renders while A still loads" "MARK beta"
present "n at startup: B's filename rule" "b-small-fast-file.txt"
snap screen-01-b-while-a-loads.txt

# ============ p back to A: only the placeholder — never content early
key p
key Escape; sleep 0.3
sleep 0.2 # let the frame settle
present "p to A mid-load: the placeholder shows" "Loading…"
present "p to A mid-load: A's filename rule" "a-big-slow-loading-file.txt"
absent "p to A mid-load: no A content before the load completes (MARK alpha)" "MARK alpha"
absent "p to A mid-load: no A content before the load completes (padlines)" "padline"
snap screen-02-a-still-loading.txt

# ============ input stays live while A's load runs: c flips the scheme
key c
sleep 0.2
# tmux decomposes the combined SGR — the light scheme's white
# background arrives as a separate [47m sequence.
styled | grep -q "\[47m" && echo "ok: c during A's load flips to the light scheme" || { echo "FAIL: c during A's load did not flip the scheme"; FAIL=1; }
key c
sleep 0.2

# ============ n to B again: instant — B's buffer is session-cached
key n
key Escape; sleep 0.3
wait_for "MARK beta" || exit 1
present "n during A's load: B renders instantly from the session cache" "MARK beta"
snap screen-03-b-cached.txt

# ============ p to A once more: still loading, then content arrives
key p
key Escape; sleep 0.3
sleep 0.2
present "p to A again mid-load: still the placeholder" "Loading…"
absent "p to A again mid-load: still no A content" "padline"
snap screen-04-a-still-loading.txt

wait_for "padline" || exit 1
present "A's load completing: content lands only now" "padline"
present "A's load completing: the first stop's line" "MARK alpha"
absent "A's load completing: the placeholder is gone" "Loading…"
snap screen-05-a-loaded.txt

key q
for _ in $(seq 1 100); do [ -f "$W/exit.txt" ] && break; sleep 0.05; done
check "vrg exit status" "$(cat "$W/exit.txt" 2>/dev/null)" "0"

[ "$FAIL" = 0 ] && echo "demo-load: all checks passed" || exit 1
