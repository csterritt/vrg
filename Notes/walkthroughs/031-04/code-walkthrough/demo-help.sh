#!/bin/bash
# Issue #31 manual check: h/? open the modal help overlay — bordered,
# scrollable, and modal — over ordinary browsing and the no-results
# screen, on the real vrg binary in a real tmux PTY with real rg.
#
# The route never touches repository or user files: the fixture lives
# in a disposable mktemp directory an EXIT trap removes.
#
# Route — session 1 (browse, started at 80x16 so the 18-row help
# scrolls):
#   startup        -> a.txt renders on its match
#   ?              -> the bordered help box opens
#   down x4        -> the binding list scrolls; the ctrl+c row enters
#                     view, the head leaves it
#   n              -> ignored: help stays open and nothing behind
#                     changes
#   Escape         -> help closes; the browse view is unchanged
#   h, resize 25x8 -> help is clipped to the terminal but present —
#                     no borderless mode
#   resize 80x24   -> the normal layout is restored
#   q, q           -> the first q closes help, the second quits at 0
#
# Route — session 2 (no-results):
#   vrg zzz .      -> "No results found"
#   ?              -> help opens over it
#   Escape, q      -> closes back to no-results, then exits 1
set -u
D="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
W="$D/help"
rm -rf "$W"
mkdir -p "$W"

# Disposable fixtures: copies only; the originals stay pristine.
TA="$(mktemp -d /tmp/vrg31-fixture.XXXXXX)"
printf 'hit alpha\nplain\nhit omega\n' >"$TA/a.txt"
printf 'hit beta\n' >"$TA/b.txt"

SES="vrg31-$$"
cleanup() {
	tmux kill-session -t "$SES" 2>/dev/null
	tmux kill-session -t "$SES-nr" 2>/dev/null
	rm -rf "$TA"
}
trap cleanup EXIT
trap 'exit 130' INT TERM

cat >"$W/inner.sh" <<EOF
#!/bin/sh
cd "$TA"
unset RIPGREP_CONFIG_PATH
"\$@" 2>"$W/stderr.txt"
echo "\$?" >"$W/exit.txt"
cat "$W/stderr.txt"
sleep 30
EOF
chmod +x "$W/inner.sh"

ESC=$(printf '\033')
export LC_ALL=C.UTF-8
screen() { tmux capture-pane -p -t "$1" 2>/dev/null; }
plain() { sed "s/${ESC}\[[0-9;]*m//g"; }
snap() { screen "$1" | plain >"$W/$2"; }

wait_for() { # wait_for <session> <literal> anywhere on the pane
	for _ in $(seq 1 600); do
		screen "$1" | grep -qF "$2" && return 0
		sleep 0.05
	done
	echo "TIMEOUT waiting for: $2"; return 1
}

FAIL=0
check() {
	if [ "$2" = "$3" ]; then echo "ok: $1 -> $2"; else echo "FAIL: $1 -> got [$2] want [$3]"; FAIL=1; fi
}
SES2="$SES-nr"
present() { # present <desc> <literal>
	if screen "$SES" | grep -qF "$2"; then echo "ok: $1"; else echo "FAIL: $1"; FAIL=1; fi
}
present2() { if screen "$SES2" | grep -qF "$2"; then echo "ok: $1"; else echo "FAIL: $1"; FAIL=1; fi; }
absent() { # absent <desc> <literal>
	if screen "$SES" | grep -qF "$2"; then echo "FAIL: $1"; FAIL=1; else echo "ok: $1"; fi
}
absent2() { if screen "$SES2" | grep -qF "$2"; then echo "FAIL: $1"; FAIL=1; else echo "ok: $1"; fi; }
key() { tmux send-keys -t "$SES" "$@"; }
key2() { tmux send-keys -t "$SES2" "$@"; }

# ============ session 1: help over browse at 80x16
tmux new-session -d -s "$SES" -x 80 -y 16 "$W/inner.sh" "$D/vrg" hit .
tmux set-option -t "$SES" window-size manual

wait_for "$SES" "hit alpha" || exit 1
present "startup: a.txt renders" "hit alpha"
snap "$SES" screen-00-startup.txt

# ============ ? opens the bordered help box
key -l '?'
wait_for "$SES" "Key bindings" || exit 1
present "?: bordered help opens" "Key bindings"
present "?: single-line border" "┌"
present "?: binding rows listed" "next / previous matched line"
# At 80x16 the 18-row help scrolls: the ctrl+c row is below the fold.
absent "?: ctrl+c row still below the fold" "ctrl+c"
snap "$SES" screen-01-help.txt

# ============ down scrolls the rendered rows
key Down; key Down; key Down; key Down; sleep 0.3
present "down: ctrl+c row scrolled into view" "ctrl+c"
absent "down: head scrolled off" "Key bindings"
snap "$SES" screen-02-scrolled.txt

# ============ n is ignored: help stays open, nothing behind changes
key n; sleep 0.3
present "n under help: help stays open" "ctrl+c"
key Escape; sleep 0.3
absent "Esc: help closed" "Key bindings"
present "Esc: file behind unchanged (a.txt still current)" "── ./a.txt"
present "Esc: a.txt content intact" "hit alpha"
snap "$SES" screen-03-closed.txt

# ============ h reopens; shrinking to 25x8 clips but keeps the overlay
key h
wait_for "$SES" "Key bindings" || exit 1
tmux resize-window -t "$SES" -x 25 -y 8
sleep 0.4
present "25x8: clipped help still present" "Key bindings"
present "25x8: border still drawn" "┌"
check "25x8: pane rows" "$(screen "$SES" | wc -l)" "8"
if screen "$SES" | plain | awk 'length($0) > 25 {bad=1} END {exit bad}'; then
	echo "ok: 25x8: every row within 25 cells"
else
	echo "FAIL: 25x8: a row overflows 25 cells"; FAIL=1
fi
snap "$SES" screen-04-tiny.txt

# ============ growth restores the normal layout
tmux resize-window -t "$SES" -x 80 -y 24
sleep 0.4
present "80x24: help restored" "Key bindings"
present "80x24: bottom border restored" "└"
snap "$SES" screen-05-restored.txt

# ============ q closes help; a second q quits at the fixed status 0
key q; sleep 0.3
absent "q: help closed" "Key bindings"
key q
for _ in $(seq 1 100); do [ -f "$W/exit.txt" ] && break; sleep 0.05; done
check "vrg exit status" "$(cat "$W/exit.txt" 2>/dev/null)" "0"
snap "$SES" screen-06-exit.txt

# ============ session 2: help over the no-results screen
rm -f "$W/exit.txt"
tmux new-session -d -s "$SES2" -x 80 -y 24 "$W/inner.sh" "$D/vrg" zzz-no-such-match .
tmux set-option -t "$SES2" window-size manual

wait_for "$SES2" "No results found" || exit 1
present2 "no-results: screen shown" "No results found"
snap "$SES2" screen-07-noresults.txt
key2 -l '?'
wait_for "$SES2" "Key bindings" || exit 1
present2 "no-results: ? opens help" "Key bindings"
snap "$SES2" screen-08-noresults-help.txt
key2 Escape; sleep 0.3
present2 "no-results: Esc returns to the screen" "No results found"
absent2 "no-results: help closed" "Key bindings"
key2 q
for _ in $(seq 1 100); do [ -f "$W/exit.txt" ] && break; sleep 0.05; done
check "no-results vrg exit status" "$(cat "$W/exit.txt" 2>/dev/null)" "1"
snap "$SES2" screen-09-exit.txt

[ "$FAIL" = 0 ] && echo "demo-help: all checks passed" || exit 1
