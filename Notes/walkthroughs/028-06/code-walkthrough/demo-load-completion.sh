#!/bin/bash
# Issue #28 manual check: the load-completion reveal commits against the
# installed layout for the latest selected target — on the real vrg
# binary in real tmux PTYs.
#
# The route never touches repository or user files: fixtures are copied
# or generated into disposable mktemp directories and an EXIT trap
# removes them on completion or interruption.
#
# A slow startup load needs no test seam: big.txt holds ~1.5M lines, so
# the read+prepare phase takes seconds and "Loading…" is painted and
# held the whole time — long enough to capture it and to resize the
# terminal mid-load. For the reload cases the settled file is swapped
# for a writerless FIFO while vrg is idle: the reread's open() blocks
# until the harness pulses a writer, a deterministic hold.
#
# Route — session A (slow startup file, stops b:500 < b:550):
#   startup load held ~seconds -> "Loading…" paints; on completion
#   line 500 lands a third down; n advances to the second stop
# Route — session B (same file at 80x24):
#   resize to 80x40 while "Loading…" still holds -> the reveal commits
#   against the NEW size: line 500 a third down the 39-row content
# Route — session C (near.txt, stop n:3):
#   startup -> the already-visible target keeps top 0
# Route — session D (pair.txt, stops p:5 < p:200):
#   swap for a FIFO, r, n    -> the reload's commit reveals the NEW
#                               stop (line 200), not the old position
#   PageUp, r, n, p          -> match scrolled off; away-and-back during
#                               the reload still reveals on completion
#   q                        -> exit 0
set -u
D="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
W="$D/completion"
rm -rf "$W"
mkdir -p "$W"

TR="$(mktemp -d /tmp/vrg28.XXXXXX)"
TA="$TR/a"; TB="$TR/b"; TC="$TR/c"; TD="$TR/d"
mkdir -p "$TA" "$TB" "$TC" "$TD"
cp "$D/fixture-src/near.txt" "$TC/"
cp "$D/fixture-src/pair.txt" "$TD/"

# ~1.5M lines (~30MB): big enough that Prepare holds "Loading…" for a
# couple of seconds; the MARK stops sit at lines 500 and 550.
python3 - "$TR/big.txt" <<'PY'
import sys
with open(sys.argv[1], "w") as f:
    for i in range(1, 1500001):
        if i in (500, 550):
            f.write(f"line-{i:07d} MARK stop\n")
        else:
            f.write(f"line-{i:07d} filler\n")
PY
cp "$TR/big.txt" "$TA/big.txt"
cp "$TR/big.txt" "$TB/big.txt"

SESA="vrg28-a-$$"
SESB="vrg28-b-$$"
SESC="vrg28-c-$$"
SESD="vrg28-d-$$"
cleanup() {
	jobs -p | xargs -r kill 2>/dev/null
	tmux kill-session -t "$SESA" "$SESB" "$SESC" "$SESD" 2>/dev/null
	rm -rf "$TR"
}
trap cleanup EXIT
trap 'exit 130' INT TERM

inner() { # inner <name> <dir>
	cat >"$W/inner-$1.sh" <<EOF
#!/bin/sh
cd "$2"
unset RIPGREP_CONFIG_PATH
"$D/vrg" MARK . 2>"$W/stderr-$1.txt"
echo "\$?" >"$W/exit-$1.txt"
cat "$W/stderr-$1.txt"
sleep 30
EOF
	chmod +x "$W/inner-$1.sh"
}
inner A "$TA"
inner B "$TB"
inner C "$TC"
inner D "$TD"

tmux new-session -d -s "$SESA" -x 80 -y 24 "$W/inner-A.sh"
tmux set-option -t "$SESA" window-size manual

SES="$SESA"
ESC=$(printf '\033')
export LC_ALL=C.UTF-8
screen() { tmux capture-pane -p -t "$SES" 2>/dev/null; }
plain() { sed "s/${ESC}\[[0-9;]*m//g"; }
snap() { screen | plain >"$W/$1"; }

wait_for() { # wait_for <literal> anywhere on the pane
	for _ in $(seq 1 1200); do
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
key() { tmux send-keys -t "$SES" "$@"; }
topline() { # first content row's source-line marker
	screen | plain | sed -n '2p' | grep -oE 'line-[0-9]+' | head -1
}
matchline() { # 1-based capture line containing a marker
	screen | plain | grep -n "$1" | head -1 | cut -d: -f1
}
pulse() { # pulse <fixture> <fifo> — one writer feeds one blocked read
	timeout 15 sh -c 'cat "$1" >"$2"' _ "$1" "$2"
}

# ============ session A: slow startup load — the reveal commits a
# third down once the prepared layout installs
wait_for "Loading" || exit 1
present "startup: Loading… while the file loads" "Loading…"
present "startup: filename rule names the path" "big.txt"
snap screen-00-loading.txt
wait_for "line-0000500" || exit 1
check "hidden startup target lands a third down (capture row 9)" "$(matchline line-0000500)" "9"
check "startup: top row" "$(topline)" "line-0000493"
snap screen-01-revealed.txt
key n; sleep 0.5
check "first n reveals the second stop (capture row 9)" "$(matchline line-0000550)" "9"
check "first n: top row" "$(topline)" "line-0000543"
snap screen-02-second-stop.txt
key q
for _ in $(seq 1 100); do [ -f "$W/exit-A.txt" ] && break; sleep 0.05; done
check "session A: vrg exit status" "$(cat "$W/exit-A.txt" 2>/dev/null)" "0"

# ============ session B: resize lands while the startup load is held
SES="$SESB"
tmux new-session -d -s "$SESB" -x 80 -y 24 "$W/inner-B.sh"
tmux set-option -t "$SESB" window-size manual
wait_for "Loading" || exit 1
present "resize case: Loading… while the load runs" "Loading…"
tmux resize-window -t "$SESB" -x 80 -y 40
sleep 0.4
present "resize during the load: still Loading…" "Loading…"
check "resize during the load: pane is 40 rows" "$(screen | wc -l)" "40"
wait_for "line-0000500" || exit 1
# At 80x40 the content height is 39: row 499 lands 13 down, top 486 —
# the reveal committed against the resized layout, not the stale one.
check "resize case: target a third down the NEW height (capture row 15)" "$(matchline line-0000500)" "15"
check "resize case: top row" "$(topline)" "line-0000487"
snap screen-10-resized-reveal.txt
key q
for _ in $(seq 1 100); do [ -f "$W/exit-B.txt" ] && break; sleep 0.05; done
check "session B: vrg exit status" "$(cat "$W/exit-B.txt" 2>/dev/null)" "0"

# ============ session C: a visible startup target keeps top 0
SES="$SESC"
tmux new-session -d -s "$SESC" -x 80 -y 24 "$W/inner-C.sh"
tmux set-option -t "$SESC" window-size manual
wait_for "line-003" || exit 1
check "visible startup target: top stays at line 1" "$(topline)" "line-001"
check "visible startup target: match in place (capture row 4)" "$(matchline line-003)" "4"
snap screen-20-visible-target.txt
key n; sleep 0.4
check "visible target: n on the one-stop file is a no-op" "$(topline)" "line-001"
key q
for _ in $(seq 1 100); do [ -f "$W/exit-C.txt" ] && break; sleep 0.05; done
check "session C: vrg exit status" "$(cat "$W/exit-C.txt" 2>/dev/null)" "0"

# ============ session D: reload followed immediately by n
SES="$SESD"
tmux new-session -d -s "$SESD" -x 80 -y 24 "$W/inner-D.sh"
tmux set-option -t "$SESD" window-size manual
wait_for "line-005" || exit 1
check "startup: pair.txt at top" "$(topline)" "line-001"
rm "$TD/pair.txt" && mkfifo "$TD/pair.txt"
key r
wait_for "Loading" || exit 1
present "r: Loading… while the reread blocks" "Loading…"
key n; sleep 0.3
present "n during the reload: still Loading… (cursor moved, commit pending)" "Loading…"
pulse "$D/fixture-src/pair.txt" "$TD/pair.txt"
wait_for "line-200" || exit 1
check "reload + n: the NEW stop is revealed (capture row 9)" "$(matchline line-200)" "9"
check "reload + n: top row" "$(topline)" "line-193"
snap screen-30-reload-n.txt

# ============ session D continued: match scrolled off, r, n, p
key PageUp; sleep 0.4
check "match scrolled off-screen before r" "$(topline)" "line-170"
absent "scrolled-off match is hidden" "line-200"
key r
wait_for "Loading" || exit 1
key n; sleep 0.2
key p; sleep 0.2
present "away-and-back during the reload: still Loading…" "Loading…"
pulse "$D/fixture-src/pair.txt" "$TD/pair.txt"
wait_for "line-200" || exit 1
check "r + n + p: the entry reveal lands (capture row 9)" "$(matchline line-200)" "9"
check "r + n + p: top row is the reveal's, not the anchor's 170" "$(topline)" "line-193"
snap screen-31-away-and-back.txt
key q
for _ in $(seq 1 100); do [ -f "$W/exit-D.txt" ] && break; sleep 0.05; done
check "session D: vrg exit status" "$(cat "$W/exit-D.txt" 2>/dev/null)" "0"

[ "$FAIL" = 0 ] && echo "demo-load-completion: all checks passed" || exit 1
