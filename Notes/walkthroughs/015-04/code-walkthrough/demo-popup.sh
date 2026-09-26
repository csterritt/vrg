#!/bin/bash
# Issue #15 manual check: the file-change pop-up. The fake rg reports
# five files in path order — first.txt (stops at lines 3 and 5),
# mid.txt (3), a hostile name carrying a real OSC payload
# (nasty<ESC>]0;pwned<BEL>.txt, stop at line 1), an 88-x long name
# (stop at 5), and zzz.txt — so n drives same-file and file-crossing
# steps against real on-disk files. The real vrg binary runs on a real
# tmux PTY at 80x24: the script sends real keypresses, resizes the
# window while a pop-up is up, measures each instance's wall-clock
# lifetime, and asserts the box's centred position, its escaped and
# truncated path text, and keypress dismissal.
set -u
D="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
R="$(CDPATH= cd -- "$D/../../../.." && pwd)"
W="$D/popup"
SES="vrg15-popup-$$"
rm -rf "$W"
mkdir -p "$W/fixture" "$W/fakebin"

(cd "$R" && go build -o "$D/vrg" ./cmd/vrg) || exit 1

NASTY="nasty$(printf '\033]0;pwned\007').txt"
LONG="$(printf 'x%.0s' $(seq 1 88)).txt"
{ for i in $(seq 1 30); do
	case "$i" in 3|5) printf 'hit %05d\n' "$i" ;; *) printf 'x%06d\n' "$i" ;; esac
done; } >"$W/fixture/first.txt"
{ for i in $(seq 1 10); do
	case "$i" in 3) printf 'hit %05d\n' "$i" ;; *) printf 'x%06d\n' "$i" ;; esac
done; } >"$W/fixture/mid.txt"
{ for i in $(seq 1 40); do
	case "$i" in 1) printf 'hit %05d\n' "$i" ;; *) printf 'x%06d\n' "$i" ;; esac
done; } >"$W/fixture/$NASTY"
{ for i in $(seq 1 40); do
	case "$i" in 5) printf 'hit %05d\n' "$i" ;; *) printf 'x%06d\n' "$i" ;; esac
done; } >"$W/fixture/$LONG"
printf 'x000001\nhit 00002\n' >"$W/fixture/zzz.txt"

cat >"$W/fakebin/rg" <<'RG'
#!/bin/sh
printf '%s\n' \
'{"type":"begin","data":{"path":{"text":"first.txt"}}}' \
'{"type":"match","data":{"path":{"text":"first.txt"},"lines":{"text":"hit 00003\n"},"line_number":3,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}' \
'{"type":"match","data":{"path":{"text":"first.txt"},"lines":{"text":"hit 00005\n"},"line_number":5,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}' \
'{"type":"end","data":{"path":{"text":"first.txt"},"binary_offset":null}}' \
'{"type":"begin","data":{"path":{"text":"mid.txt"}}}' \
'{"type":"match","data":{"path":{"text":"mid.txt"},"lines":{"text":"hit 00003\n"},"line_number":3,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}' \
'{"type":"end","data":{"path":{"text":"mid.txt"},"binary_offset":null}}' \
'{"type":"begin","data":{"path":{"text":"nasty\u001b]0;pwned\u0007.txt"}}}' \
'{"type":"match","data":{"path":{"text":"nasty\u001b]0;pwned\u0007.txt"},"lines":{"text":"hit 00001\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}' \
'{"type":"end","data":{"path":{"text":"nasty\u001b]0;pwned\u0007.txt"},"binary_offset":null}}' \
'{"type":"begin","data":{"path":{"text":"LONGNAME"}}}' \
'{"type":"match","data":{"path":{"text":"LONGNAME"},"lines":{"text":"hit 00005\n"},"line_number":5,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}' \
'{"type":"end","data":{"path":{"text":"LONGNAME"},"binary_offset":null}}' \
'{"type":"begin","data":{"path":{"text":"zzz.txt"}}}' \
'{"type":"match","data":{"path":{"text":"zzz.txt"},"lines":{"text":"hit 00002\n"},"line_number":2,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}' \
'{"type":"end","data":{"path":{"text":"zzz.txt"},"binary_offset":null}}' \
'{"type":"summary","data":{}}'
exit 0
RG
sed -i "s/LONGNAME/$LONG/g" "$W/fakebin/rg"
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
ms() { date +%s%3N; }

# Pane line 1 carries the file list plus the filename rule
# ("── <path> ───"); content rows are pane lines 2..24.
cur_file() { screen | sed -n '1p' | plain | sed -n 's/.*── \([^ ]*\) .*/\1/p'; }
cur_match() {
	screen_e | while IFS= read -r l; do
		case "$l" in *"${ESC}[4m"*) printf '%s\n' "$l" | plain ;; esac
	done | grep -o 'hit [0-9]*' | head -1
}
first_content() { screen | sed -n '2p' | plain | sed -n 's/.* [0-9]*  //p'; }

has_box() { screen | grep -q '┌'; }
popup_text() { screen | sed -n '/│/p' | head -1 | sed 's/^[^│]*│\([^│]*\)│.*/\1/'; }
# cells <pane line> <1-based column> <count>: slice display cells
# (sed's . counts characters; every glyph in play is one cell wide).
cells() { screen | sed -n "${1}p" | sed -n "s/^.\{$(($2 - 1))\}\(.\{$3\}\).*/\1/p"; }

wait_match() {
	for _ in $(seq 1 600); do
		[ "$(cur_match)" = "$1" ] && return 0
		sleep 0.05
	done
	echo "TIMEOUT waiting for match $1 (got $(cur_match))"; return 1
}
wait_box() { # wait_box <want>: pop-up middle row shows want
	for _ in $(seq 1 600); do
		[ "$(popup_text)" = "$1" ] && return 0
		sleep 0.02
	done
	echo "TIMEOUT waiting for pop-up $1 (got $(popup_text))"; return 1
}
wait_nobox() { # waits for the box to vanish; echoes elapsed ms since $1
	local start=$1
	for _ in $(seq 1 600); do
		has_box || { echo $(( $(ms) - start )); return 0; }
		sleep 0.05
	done
	echo "TIMEOUT waiting for pop-up to clear"; return 1
}
sleep_until_ms() { # sleep_until_ms <epoch-ms>
	local d=$(( $1 - $(ms) ))
	[ "$d" -gt 0 ] && sleep "$(printf '%d.%03d' $((d / 1000)) $((d % 1000)))"
	return 0
}

FAIL=0
check() { # check <desc> <got> <want>
	if [ "$2" = "$3" ]; then echo "ok: $1 -> $2"; else echo "FAIL: $1 -> got $2 want $3"; FAIL=1; fi
}
snap() { screen | plain >"$W/$1"; }

# Startup on first.txt:3 — no pop-up.
wait_match "hit 00003" || exit 1
check "startup: file panel" "$(cur_file)" "first.txt"
check "startup: current match" "$(cur_match)" "hit 00003"
if has_box; then echo "FAIL: startup: no pop-up"; FAIL=1; else echo "ok: startup: no pop-up"; fi
snap screen-01-start.txt

# Same-file n -> first.txt:5: still no pop-up.
tmux send-keys -t "$SES" n
wait_match "hit 00005" || exit 1
if has_box; then echo "FAIL: same-file n: no pop-up"; FAIL=1; else echo "ok: same-file n: no pop-up"; fi

# Crossing n -> mid.txt:3: the pop-up opens centred at 80x24 — box
# width 7+2=9 at x=(80-9)/2 -> columns 36..44, rows 11..13 — while the
# destination may still be loading.
tmux send-keys -t "$SES" n
wait_box "mid.txt" || exit 1
t1=$(ms)
check "crossing: file panel" "$(cur_file)" "mid.txt"
check "crossing: pop-up text" "$(popup_text)" "mid.txt"
check "crossing: box top" "$(cells 11 36 9)" "┌───────┐"
check "crossing: box middle" "$(cells 12 36 9)" "│mid.txt│"
check "crossing: box bottom" "$(cells 13 36 9)" "└───────┘"
snap screen-02-popup-mid.txt

# Quick second n ~0.3s later (pop-up 1 still up) -> the hostile file:
# instance 1's timer is still pending, so its expiry must not kill
# instance 2. The raw ESC/BEL in the name render as ^[ / ^G.
sleep 0.3
tmux send-keys -t "$SES" n
wait_box 'nasty^[]0;pwned^G.txt' || exit 1
t2=$(ms)
check "hostile: file panel" "$(cur_file)" 'nasty^[]0;pwned^G.txt'
check "hostile: pop-up escaped" "$(popup_text)" 'nasty^[]0;pwned^G.txt'
check "hostile: box middle" "$(cells 12 29 23)" '│nasty^[]0;pwned^G.txt│'
snap screen-03-popup-hostile.txt

# Resize while the pop-up is shown: it recentres on the new 100x30
# frame (x=(100-23)/2 -> column 39, rows 14..16) without restarting.
tmux resize-window -t "$SES" -x 100 -y 30
for _ in $(seq 1 200); do
	[ "$(cells 15 39 23)" = '│nasty^[]0;pwned^G.txt│' ] && break
	sleep 0.02
done
check "resized: recentred box" "$(cells 15 39 23)" '│nasty^[]0;pwned^G.txt│'
check "resized: box top" "$(cells 14 39 23)" '┌─────────────────────┐'
snap screen-04-popup-resized.txt

# Instance 1's deadline (~t1+1000) passes while instance 2 — opened
# ~0.35s after instance 1 — still has ~0.3s to live.
sleep_until_ms $((t1 + 1050))
if has_box; then echo "ok: stale expiry cannot dismiss instance 2"; else echo "FAIL: instance 2 died at instance 1's deadline"; FAIL=1; fi

# Instance 2 then ends on its own clock: ~1s after t2, and clearly
# past instance 1's deadline — the stale-expiry rejection held.
d2=$(wait_nobox "$t2") || exit 1
d1=$(( $(ms) - t1 ))
check "instance 2 lifetime ~1s" "$([ "$d2" -ge 900 ] && [ "$d2" -le 1600 ] && echo "in-range" || echo "out-of-range")" "in-range"
check "outlived instance 1's deadline" "$([ "$d1" -gt 1000 ] && echo "yes" || echo "no")" "yes"
echo "timing: instance1 open->gone ${d1}ms, instance2 open->gone ${d2}ms"
snap screen-05-expired.txt

tmux resize-window -t "$SES" -x 80 -y 24
sleep 0.3

# Crossing n -> the 92-byte name: left-truncated interior
# ("…" + last 77 cells) fills the frame — box width 80 at x=0.
tmux send-keys -t "$SES" n
for _ in $(seq 1 600); do
	[ "$(cells 12 1 2)" = '│…' ] && break
	sleep 0.02
done
check "truncated: leading ellipsis" "$(cells 12 1 2)" '│…'
check "truncated: left border col 1" "$(cells 11 1 1)" '┌'
check "truncated: right border col 80" "$(cells 11 80 1)" '┐'
check "truncated: basename kept" "$(popup_text | tail -c 5)" '.txt'
snap screen-06-popup-truncated.txt

# Any key dismisses and still acts: down clears the pop-up and scrolls
# the loaded file one row in the same update.
tmux send-keys -t "$SES" down
for _ in $(seq 1 200); do
	[ "$(first_content)" = "x000002" ] && break
	sleep 0.02
done
check "down: dismissed" "$(has_box && echo yes || echo no)" "no"
check "down: scrolled one row" "$(first_content)" "x000002"
snap screen-07-dismissed.txt

tmux send-keys -t "$SES" q
for _ in $(seq 1 100); do [ -f "$W/exit.txt" ] && break; sleep 0.05; done
check "exit status" "$(cat "$W/exit.txt")" "0"
[ "$FAIL" = 0 ] && echo "demo-popup: all checks passed" || exit 1
