#!/bin/bash
# Issue #13 manual check: n/p drive a circular matched-line cursor.
# The fake rg reports hits in three files — a.txt (lines 1 and 4),
# b.txt (lines 2 and 5), c.txt (line 3) — five stops ordered by path
# then line. The real vrg binary runs on a real tmux PTY at 80x24.
# The script sends real keypresses and asserts, at each step, the
# filename rule, the underlined file-list entry, and the underlined
# (inverse + underline) current match: n moves through same-file stops
# without touching the viewport, crosses files with the list underline
# following, wraps last->first; p wraps first->last and steps back;
# a manual scroll leaves the cursor on its stop and n continues from
# there; revisiting a file restores its saved viewport.
set -u
D="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
W="$D/match-nav"
SES="vrg13-nav-$$"
rm -rf "$W"
mkdir -p "$W/fixture" "$W/fakebin"

{ for i in $(seq 1 40); do
	if [ "$i" = 1 ] || [ "$i" = 4 ]; then printf 'hit a%03d\n' "$i"; else printf 'a%03d\n' "$i"; fi
done; } >"$W/fixture/a.txt"
{ for i in $(seq 1 30); do
	if [ "$i" = 2 ] || [ "$i" = 5 ]; then printf 'hit b%03d\n' "$i"; else printf 'b%03d\n' "$i"; fi
done; } >"$W/fixture/b.txt"
{ for i in $(seq 1 10); do
	if [ "$i" = 3 ]; then printf 'hit c%03d\n' "$i"; else printf 'c%03d\n' "$i"; fi
done; } >"$W/fixture/c.txt"

cat >"$W/fakebin/rg" <<'RG'
#!/bin/sh
printf '%s\n' \
'{"type":"begin","data":{"path":{"text":"a.txt"}}}' \
'{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"hit a001\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}' \
'{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"hit a004\n"},"line_number":4,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}' \
'{"type":"end","data":{"path":{"text":"a.txt"},"binary_offset":null}}' \
'{"type":"begin","data":{"path":{"text":"b.txt"}}}' \
'{"type":"match","data":{"path":{"text":"b.txt"},"lines":{"text":"hit b002\n"},"line_number":2,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}' \
'{"type":"match","data":{"path":{"text":"b.txt"},"lines":{"text":"hit b005\n"},"line_number":5,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}' \
'{"type":"end","data":{"path":{"text":"b.txt"},"binary_offset":null}}' \
'{"type":"begin","data":{"path":{"text":"c.txt"}}}' \
'{"type":"match","data":{"path":{"text":"c.txt"},"lines":{"text":"hit c003\n"},"line_number":3,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}' \
'{"type":"end","data":{"path":{"text":"c.txt"},"binary_offset":null}}' \
'{"type":"summary","data":{}}'
exit 0
RG
chmod +x "$W/fakebin/rg"

cat >"$W/inner.sh" <<EOF
#!/bin/sh
cd "$W/fixture"
PATH="$W/fakebin:\$PATH" "$D/vrg" hit .
echo "\$?" >"$W/exit.txt"
sleep 60
EOF
chmod +x "$W/inner.sh"

cleanup() { tmux kill-session -t "$SES" 2>/dev/null; }
trap cleanup EXIT
tmux new-session -d -s "$SES" -x 80 -y 24 "$W/inner.sh"

ESC=$(printf '\033')
screen() { tmux capture-pane -p -t "$SES" 2>/dev/null; }
screen_e() { tmux capture-pane -p -e -t "$SES" 2>/dev/null; }
plain() { sed "s/${ESC}\[[0-9;]*m//g"; }
# Pane row 1 is the filename rule ("-- <path> ---"); the current file
# is underlined in the list; the current match is inverse + underline.
cur_file() { screen | sed -n '1p' | plain | sed -n 's/.*── \([^ ]*\) .*/\1/p'; }
list_cur() { screen_e | grep -oE "${ESC}\[4m(${ESC}\[[0-9;]*m)*[a-z]+\.txt" | head -1 | plain; }
cur_match() {
	screen_e | while IFS= read -r l; do
		case "$l" in *"${ESC}[4m"*) printf '%s\n' "$l" | plain ;; esac
	done | grep -o 'hit [a-z][0-9]*' | head -1
}
first_content() { screen | sed -n '2p' | plain | sed -n 's/.* [0-9]*  //p'; }

wait_for() { # wait_for <literal> anywhere on the pane
	for _ in $(seq 1 600); do
		screen | grep -qF "$1" && return 0
		sleep 0.05
	done
	echo "TIMEOUT waiting for: $1"; return 1
}
wait_file() { # wait_file <name> on the filename rule
	for _ in $(seq 1 600); do
		[ "$(cur_file)" = "$1" ] && return 0
		sleep 0.05
	done
	echo "TIMEOUT waiting for file $1 (got $(cur_file))"; return 1
}
wait_match() { # wait_match "hit xNNN" underlined
	for _ in $(seq 1 600); do
		[ "$(cur_match)" = "$1" ] && return 0
		sleep 0.05
	done
	echo "TIMEOUT waiting for match $1 (got $(cur_match))"; return 1
}
wait_first() { # wait_first <text> on the first content row
	for _ in $(seq 1 600); do
		[ "$(first_content)" = "$1" ] && return 0
		sleep 0.05
	done
	echo "TIMEOUT waiting for first row $1 (got $(first_content))"; return 1
}

FAIL=0
check() { # check <desc> <got> <want>
	if [ "$2" = "$3" ]; then echo "ok: $1 -> $2"; else echo "FAIL: $1 -> got $2 want $3"; FAIL=1; fi
}
snap() { screen | plain >"$W/$1"; }

wait_for "hit a001" || exit 1
check "startup: file panel" "$(cur_file)" "a.txt"
check "startup: list underline" "$(list_cur)" "a.txt"
check "startup: current match" "$(cur_match)" "hit a001"
snap screen-01-start.txt

tmux send-keys -t "$SES" n
wait_match "hit a004" || exit 1
check "n same-file: file panel" "$(cur_file)" "a.txt"
check "n same-file: current match" "$(cur_match)" "hit a004"
check "n same-file: viewport unmoved" "$(first_content)" "hit a001"
snap screen-02-n-samefile.txt

tmux send-keys -t "$SES" n
wait_file "b.txt" || exit 1
wait_match "hit b002" || exit 1
check "n cross-file: file panel" "$(cur_file)" "b.txt"
check "n cross-file: list underline" "$(list_cur)" "b.txt"
check "n cross-file: current match" "$(cur_match)" "hit b002"
snap screen-03-n-crossfile.txt

tmux send-keys -t "$SES" n
wait_match "hit b005" || exit 1
check "n same-file: current match" "$(cur_match)" "hit b005"
tmux send-keys -t "$SES" n
wait_file "c.txt" || exit 1
wait_match "hit c003" || exit 1
check "n cross-file: list underline" "$(list_cur)" "c.txt"
check "n cross-file: current match" "$(cur_match)" "hit c003"
snap screen-04-n-thirdfile.txt

tmux send-keys -t "$SES" n
wait_file "a.txt" || exit 1
wait_match "hit a001" || exit 1
check "n wrap last->first: file panel" "$(cur_file)" "a.txt"
check "n wrap last->first: list underline" "$(list_cur)" "a.txt"
check "n wrap last->first: current match" "$(cur_match)" "hit a001"
snap screen-05-n-wrap.txt

tmux send-keys -t "$SES" p
wait_file "c.txt" || exit 1
wait_match "hit c003" || exit 1
check "p wrap first->last: file panel" "$(cur_file)" "c.txt"
check "p wrap first->last: current match" "$(cur_match)" "hit c003"
tmux send-keys -t "$SES" p
wait_file "b.txt" || exit 1
wait_match "hit b005" || exit 1
check "p step back: current match" "$(cur_match)" "hit b005"
snap screen-06-p-back.txt

for _ in 1 2 3 4 5; do tmux send-keys -t "$SES" Down; done
wait_first "b006" || exit 1
check "manual scroll: file panel unchanged" "$(cur_file)" "b.txt"
tmux send-keys -t "$SES" n
wait_file "c.txt" || exit 1
wait_match "hit c003" || exit 1
check "n continues from last stop" "$(cur_match)" "hit c003"
snap screen-07-scroll-n.txt

tmux send-keys -t "$SES" p
wait_file "b.txt" || exit 1
check "revisit restores saved viewport" "$(first_content)" "b006"
snap screen-08-revisit.txt

tmux send-keys -t "$SES" q
for _ in $(seq 1 100); do [ -f "$W/exit.txt" ] && break; sleep 0.05; done
check "exit status" "$(cat "$W/exit.txt")" "0"
[ "$FAIL" = 0 ] && echo "demo-nav: all checks passed" || exit 1
