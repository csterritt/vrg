#!/bin/bash
# Issue #33 manual check: the "Terminal too small" gate and its state
# recovery on the real vrg binary in a real tmux PTY with a fake rg
# script standing in for ripgrep.
#
# The route never touches repository or user files: fixtures live in
# disposable mktemp directories an EXIT trap removes. The fake rg
# script and the session captures are generated under ./run in this
# directory.
#
# Route — session 1 (browse at 40x10, fake rg emits a two-file
# stream, exit 0):
#   ?            -> help opens over browse
#   Down x4      -> help scrolls off its first binding rows
#   resize 15x2  -> "Terminal too sm" (the clipped gate message);
#                   help hidden, Esc a byte-identical no-op
#   resize 20x3  -> boundary size is viable: the gate lifts and the
#                   help box renders clipped to the tiny frame
#   resize 40x10 -> the recovered help frame is byte-identical to the
#                   pre-shrink capture
#   resize 15x2  -> the gate again
#   q            -> exits with the fixed browse status 0 (not a
#                   dismissal of the logically open help)
#
# Route — session 2 (browse at 40x10):
#   resize 15x2  -> the gate
#   ctrl+c       -> exit 130
set -u
D="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
W="$D/run"
rm -rf "$W"
mkdir -p "$W"

# Disposable fixtures and the fake rg install: nothing outside mktemp.
T="$(mktemp -d /tmp/vrg33-fixture.XXXXXX)"
BIN="$(mktemp -d /tmp/vrg33-bin.XXXXXX)"
printf 'MARK alpha\nplain\n' >"$T/a.txt"
printf 'MARK beta\n' >"$T/b.txt"

# fake rg: the complete two-file stream, exit 0 — no stderr warning,
# so no overlay stands between browse and help.
cat >"$BIN/rg" <<'EOF'
#!/bin/sh
printf '%s\n' \
'{"type":"begin","data":{"path":{"text":"a.txt"}}}' \
'{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"MARK alpha\n"},"line_number":1,"submatches":[{"match":{"text":"MARK"},"start":0,"end":4}]}}' \
'{"type":"end","data":{"path":{"text":"a.txt"},"binary_offset":null}}' \
'{"type":"begin","data":{"path":{"text":"b.txt"}}}' \
'{"type":"match","data":{"path":{"text":"b.txt"},"lines":{"text":"MARK beta\n"},"line_number":1,"submatches":[{"match":{"text":"MARK"},"start":0,"end":4}]}}' \
'{"type":"end","data":{"path":{"text":"b.txt"},"binary_offset":null}}' \
'{"type":"summary","data":{}}'
exit 0
EOF
chmod +x "$BIN/rg"
cp "$BIN/rg" "$W/fake-rg"

SES="vrg33-$$"
cleanup() {
	tmux kill-session -t "$SES" 2>/dev/null
	rm -rf "$T" "$BIN"
}
trap cleanup EXIT
trap 'exit 130' INT TERM

TAG=browse
# start <tag>: launch vrg in a fresh 40x10 tmux session on the
# fixture; the session's stderr and exit status land in tag-named
# artifacts.
start() {
	TAG="$1"
	rm -f "$W/exit-$TAG.txt"
	cat >"$W/inner-$TAG.sh" <<EOF
#!/bin/sh
cd "$T"
unset RIPGREP_CONFIG_PATH
PATH="$BIN:\$PATH" "$D/vrg" MARK . 2>"$W/stderr-$TAG.txt"
echo "\$?" >"$W/exit-$TAG.txt"
cat "$W/stderr-$TAG.txt"
sleep 30
EOF
	chmod +x "$W/inner-$TAG.sh"
	tmux new-session -d -s "$SES" -x 40 -y 10 "$W/inner-$TAG.sh"
	tmux set-option -t "$SES" window-size manual
}

ESC=$(printf '\033')
export LC_ALL=C.UTF-8
screen() { tmux capture-pane -p -t "$SES" 2>/dev/null; }
plain() { sed "s/${ESC}\[[0-9;]*m//g"; }
snap() { screen | plain >"$W/$1"; }

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
present() {
	if screen | grep -qF "$2"; then echo "ok: $1"; else echo "FAIL: $1"; FAIL=1; fi
}
absent() {
	if screen | grep -qF "$2"; then echo "FAIL: $1"; FAIL=1; else echo "ok: $1"; fi
}
running() { # still running: no exit artifact landed
	if [ -f "$W/exit-$TAG.txt" ]; then echo "FAIL: $1 (vrg exited)"; FAIL=1; else echo "ok: $1"; fi
}
key() { tmux send-keys -t "$SES" "$@"; }
resize() { tmux resize-window -t "$SES" -x "$1" -y "$2"; }
await_exit() { # await_exit <want> — wait for the exit artifact and compare
	for _ in $(seq 1 100); do [ -f "$W/exit-$TAG.txt" ] && break; sleep 0.05; done
	check "vrg exit status" "$(cat "$W/exit-$TAG.txt" 2>/dev/null)" "$1"
}

# ==================== session 1: help scroll survives the gate ====
start browse

wait_for "MARK alpha" || exit 1

# ? opens help over browse; Down x4 scrolls four binding rows off.
key "?"; wait_for "Key bindings" || exit 1
present "?: help opens over browse" "Key bindings"
snap help-top.txt
key Down; key Down; key Down; key Down; sleep 0.3
snap help-scrolled.txt
check "Down x4: help scrolled" \
	"$(cmp -s "$W/help-top.txt" "$W/help-scrolled.txt" && echo same || echo different)" "different"
absent "Down x4: top bindings scrolled off" "next / previous"

# Shrink to 15x2: the gate replaces the frame — the message clipped to
# the width, help hidden but logically open.
resize 15 2
wait_for "Terminal too" || exit 1
present "15x2: gate message (clipped to width)" "Terminal too"
absent "15x2: help hidden behind the gate" "Key bindings"
snap screen-too-small.txt

# Esc is a byte-identical no-op while gated — the hidden modal is
# untouched and the program keeps running.
key Escape; sleep 0.4
screen | plain >"$W/after-esc.txt"
check "Esc at 15x2: frame unchanged" \
	"$(cmp -s "$W/screen-too-small.txt" "$W/after-esc.txt" && echo same)" "same"
running "Esc at 15x2: still running"

# 20x3 is the exact boundary — viable again: the gate lifts and the
# help box renders clipped to the tiny frame (one border row and a
# centre-clipped slice of the scrolled content are all that fit).
resize 20 3
sleep 0.6
absent "20x3: boundary viable — gate lifted" "Terminal too"
present "20x3: help box clipped to the frame" "│"
running "20x3: still running"
snap screen-20x3.txt

# Back to 40x10: the recovered frame is byte-identical to the
# pre-shrink scrolled capture — scroll position survived intact.
resize 40 10
wait_for "scroll half a page" || exit 1
sleep 0.3
snap help-restored.txt
check "40x10 recovery: help frame identical" \
	"$(cmp -s "$W/help-scrolled.txt" "$W/help-restored.txt" && echo same)" "same"

# Shrink again, then q: the gate's q exits the program — it does not
# merely dismiss the logically open help.
resize 15 2
wait_for "Terminal too" || exit 1
key q
await_exit 0
snap screen-exit.txt

tmux kill-session -t "$SES" 2>/dev/null

# ==================== session 2: ctrl+c under the gate ============
start ctrlc
wait_for "MARK alpha" || exit 1
resize 15 2
wait_for "Terminal too" || exit 1
key C-c
await_exit 130
tmux kill-session -t "$SES" 2>/dev/null

[ "$FAIL" = 0 ] && echo "demo-toosmall: all checks passed" || exit 1
