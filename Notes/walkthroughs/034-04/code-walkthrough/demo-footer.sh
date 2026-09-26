#!/bin/bash
# Issue #34 manual check: the help overlay's footer note — the scale,
# record-limit, and memory statements shared with README.md — rendering
# on the real vrg binary in a real tmux PTY with a fake rg script
# standing in for ripgrep.
#
# The route never touches repository or user files: fixtures live in
# disposable mktemp directories an EXIT trap removes. The fake rg
# script and the session captures are generated under ./run in this
# directory.
#
# Route — session (browse at 80x24, fake rg emits a two-file stream,
# exit 0):
#   ?            -> help opens over browse: "Key bindings" title, the
#                   note's tail still below the fold
#   Down x40     -> help scrolls to the tail: title scrolled off, the
#                   footer note's scale, 64 MiB, and terminal-cleanup
#                   statements visible inside the border
#   q            -> closes the help overlay back to browse
#   q            -> quits with the fixed browse status 0
set -u
D="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
W="$D/run"
rm -rf "$W"
mkdir -p "$W"

# Disposable fixtures and the fake rg install: nothing outside mktemp.
T="$(mktemp -d /tmp/vrg34-fixture.XXXXXX)"
BIN="$(mktemp -d /tmp/vrg34-bin.XXXXXX)"
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

SES="vrg34-$$"
cleanup() {
	tmux kill-session -t "$SES" 2>/dev/null
	rm -rf "$T" "$BIN"
}
trap cleanup EXIT
trap 'exit 130' INT TERM

TAG=browse
# start <tag>: launch vrg in a fresh 80x24 tmux session on the
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
	tmux new-session -d -s "$SES" -x 80 -y 24 "$W/inner-$TAG.sh"
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
key() { tmux send-keys -t "$SES" "$@"; }
await_exit() { # await_exit <want> — wait for the exit artifact and compare
	for _ in $(seq 1 100); do [ -f "$W/exit-$TAG.txt" ] && break; sleep 0.05; done
	check "vrg exit status" "$(cat "$W/exit-$TAG.txt" 2>/dev/null)" "$1"
}

# ==================== session: footer note at the help tail ========
start browse

wait_for "MARK alpha" || exit 1

# ? opens help over browse.
key "?"; wait_for "Key bindings" || exit 1
present "?: help opens over browse" "Key bindings"
absent "?: note tail below the fold" "terminal cleanup"
snap help-top.txt

# Scroll to the tail: the title leaves the top and the Issue #34
# footer note — the scale examples, the 64 MiB record limit, and the
# terminal-cleanup statement — renders inside the border.
for _ in $(seq 1 40); do key Down; done; sleep 0.4
snap help-tail.txt
absent "Down x40: title scrolled off" "Key bindings"
present "footer: scale examples note" "Scale examples"
present "footer: 64 MiB record limit" "64 MiB"
present "footer: terminal cleanup limit" "terminal cleanup"

# q closes help back to browse; the second q quits with the fixed
# browse status 0.
key q; sleep 0.4
absent "q: help closed" "terminal cleanup"
present "q: browse restored" "MARK alpha"
key q
await_exit 0

tmux kill-session -t "$SES" 2>/dev/null

[ "$FAIL" = 0 ] && echo "demo-footer: all checks passed" || exit 1
