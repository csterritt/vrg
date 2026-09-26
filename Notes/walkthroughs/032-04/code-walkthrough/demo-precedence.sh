#!/bin/bash
# Issue #32 manual check: overlay precedence and Esc/q dismissal
# semantics on the real vrg binary in a real tmux PTY with fake rg
# scripts standing in for ripgrep.
#
# The route never touches repository or user files: fixtures live in
# disposable mktemp directories an EXIT trap removes. The fake rg
# scripts and the session captures are generated under ./run in this
# directory.
#
# Route — session 1 (browse, fake rg emits a two-file stream plus a
# stderr warning, exit 0):
#   startup   -> the warning overlay opens over the browse screen
#   Escape    -> dismissed; a.txt renders
#   Escape    -> no overlay: a no-op — the frame is unchanged and the
#                program is still running
#   ?         -> help opens; down x3 scrolls it
#   Escape    -> help closes back to browsing
#   ? then r  -> r is ignored while help is open (no reload, no change)
#   Escape    -> help closed
#   chmod 000 b.txt; n
#             -> crossing opens the error overlay (cannot read b.txt)
#                over the cancelled file-change pop-up
#   q         -> the overlay closes only — browsing still runs
#   q         -> the base-state q exits with the fixed status 0
#
# Route — sessions 2 and 3 (fatal, fake rg exits 3 with no output):
#   startup   -> the fatal overlay stands alone ("exit status 3")
#   Escape / q-> dismissal terminates with status 2 — there is no
#                underlying state
set -u
D="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
W="$D/run"
rm -rf "$W"
mkdir -p "$W"

if [ "$(id -u)" = 0 ]; then
	echo "SKIP: must run unprivileged — chmod 000 does not deny root"
	exit 1
fi

# Disposable fixtures and fake rg installs: nothing outside mktemp.
T="$(mktemp -d /tmp/vrg32-fixture.XXXXXX)"
BIN="$(mktemp -d /tmp/vrg32-bin.XXXXXX)"
printf 'MARK alpha\nplain\n' >"$T/a.txt"
printf 'MARK beta\n' >"$T/b.txt"

# fake rg: the complete two-file stream plus a stderr warning, exit 0.
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
echo 'rg: warn: fake warning' >&2
exit 0
EOF
chmod +x "$BIN/rg"
cp "$BIN/rg" "$W/fake-rg-warn"

# fatal rg: no output, exit 3 — the fatal no-results overlay.
mkdir -p "$BIN/fatal"
cat >"$BIN/fatal/rg" <<'EOF'
#!/bin/sh
exit 3
EOF
chmod +x "$BIN/fatal/rg"
cp "$BIN/fatal/rg" "$W/fake-rg-fatal"

SES="vrg32-$$"
cleanup() {
	tmux kill-session -t "$SES" 2>/dev/null
	chmod -R u+rwX "$T" 2>/dev/null
	rm -rf "$T" "$BIN"
}
trap cleanup EXIT
trap 'exit 130' INT TERM

TAG=browse
# start <bin-dir> <tag>: launch vrg in a fresh tmux session on the
# fixture; each session's stderr and exit status land in tag-named
# artifacts.
start() {
	TAG="$2"
	rm -f "$W/exit-$TAG.txt"
	cat >"$W/inner-$TAG.sh" <<EOF
#!/bin/sh
cd "$T"
unset RIPGREP_CONFIG_PATH
PATH="$1:\$PATH" "$D/vrg" MARK . 2>"$W/stderr-$TAG.txt"
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
running() { # still running: no exit artifact landed
	if [ -f "$W/exit-$TAG.txt" ]; then echo "FAIL: $1 (vrg exited)"; FAIL=1; else echo "ok: $1"; fi
}
key() { tmux send-keys -t "$SES" "$@"; }
await_exit() { # await_exit <want> — wait for the exit artifact and compare
	for _ in $(seq 1 100); do [ -f "$W/exit-$TAG.txt" ] && break; sleep 0.05; done
	check "vrg exit status" "$(cat "$W/exit-$TAG.txt" 2>/dev/null)" "$1"
}

# ==================== session 1: browse ====================
start "$BIN" browse

# The startup warning overlay opens over the browse screen.
wait_for "fake warning" || exit 1
present "startup: warning overlay over browse" "fake warning"
snap screen-00-warning-overlay.txt

# Esc dismisses it; a.txt renders beneath.
key Escape; sleep 0.4
absent "Esc: warning overlay dismissed" "fake warning"
present "Esc: browse revealed" "MARK alpha"
snap screen-01-warning-dismissed.txt

# Esc with no overlay is a no-op: identical frame, still running.
screen | plain >"$W/before-esc.txt"
key Escape; sleep 0.4
screen | plain >"$W/after-esc.txt"
check "Esc no overlay: frame unchanged" "$(cmp -s "$W/before-esc.txt" "$W/after-esc.txt" && echo same)" "same"
running "Esc no overlay: still running"

# ? opens help; down x3 scrolls the binding list; Esc closes it.
key "?"; wait_for "Key bindings" || exit 1
present "?: help opens over browse" "Key bindings"
key Down; key Down; key Down; sleep 0.3
snap screen-02-help-scrolled.txt
key Escape; sleep 0.3
absent "Esc: help closed" "Key bindings"
present "Esc: browse intact" "MARK alpha"

# r while help is open is ignored: reopen help, press r, the panel is
# unchanged (no Loading… flash) and help stays up.
key "?"; wait_for "Key bindings" || exit 1
key r; sleep 0.3
present "r under help: help still open" "Key bindings"
absent "r under help: no reload started" "Loading…"
key Escape; sleep 0.3

# chmod 000 b.txt and cross into it: the file-change pop-up flashes and
# the arriving error overlay cancels it (cannot read b.txt).
chmod 000 "$T/b.txt"
key n
wait_for "cannot read" || exit 1
present "n into b.txt: error overlay opens" "cannot read"
present "n into b.txt: overlay names the path" "b.txt"
present "n into b.txt: (unreadable) placeholder" "(unreadable)"
snap screen-03-error-overlay.txt

# q closes the overlay only — browsing is still running underneath.
key q; sleep 0.4
absent "q: error overlay closed" "cannot read"
present "q: still browsing" "(unreadable)"
running "q: program still running"
snap screen-04-overlay-closed.txt

# A second q is the base-state quit: fixed status 0.
key q
await_exit 0
check "stderr replays the warning" "$(grep -cF 'fake warning' "$W/stderr-browse.txt")" "1"
check "stderr replays the read failure" "$(grep -cF 'cannot read b.txt' "$W/stderr-browse.txt")" "1"
snap screen-05-exit.txt

tmux kill-session -t "$SES" 2>/dev/null

# ==================== sessions 2 and 3: fatal ====================
for K in Escape q; do
	start "$BIN/fatal" "fatal-$K"
	wait_for "exit status 3" || exit 1
	present "fatal rg 3: overlay stands alone" "exit status 3"
	absent "fatal rg 3: no browse beneath" "MARK alpha"
	snap "screen-06-fatal-$K.txt"
	key "$K"
	await_exit 2
	check "fatal rg 3 $K: stderr replays the diagnostic" \
		"$(grep -cF 'exit status 3' "$W/stderr-fatal-$K.txt")" "1"
	tmux kill-session -t "$SES" 2>/dev/null
done

[ "$FAIL" = 0 ] && echo "demo-precedence: all checks passed" || exit 1
