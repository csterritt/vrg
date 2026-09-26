#!/bin/bash
# Issue #39 manual scenario — one shared grapheme/cell model end to
# end, driven on a real tmux PTY at 80x24 against a fixture whose
# fakebin/rg emits three match records plus a wide-named second file:
#
#   mixed.txt line 1:  x e<U+0301> y z      — a combining-only match on
#                      the mark's bytes alone must paint the whole é
#                      cell, never a fragment.
#   mixed.txt line 2:  a 👨‍👩‍👧 c             — an interior-bytes match
#                      inside the emoji ZWJ sequence must cover the
#                      whole two-cell cluster.
#   mixed.txt line 3:  9 chars + 世界 + pad — the 世界 match covers
#                      both cells of each CJK cluster; after one >
#                      pan (offset 10) the window edge splits 世's
#                      two cells and the in-window cell must paint
#                      blank, never half the glyph.
#   世界名.txt          — navigating across files opens the file-change
#                      pop-up; the box interior is the 10-cell escaped
#                      path and all three border rows must align at
#                      12 cells.
#
# Assertions run over tmux capture-pane -e: a small python check
# reconstructs which text carries the match style and requires it to
# equal each whole cluster exactly. q exits 0.
set -u
D="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
W="$D/manual-cell-model"
SES="vrg39-cells-$$"
rm -rf "$W"
mkdir -p "$W/fixture" "$W/fakebin"

# Fixture files — combining marks are written decomposed on purpose.
printf 'xe\xcc\x81yz\n' >"$W/fixture/mixed.txt"
printf 'a\xf0\x9f\x91\xa8\xe2\x80\x8d\xf0\x9f\x91\xa9\xe2\x80\x8d\xf0\x9f\x91\xa7c\n' >>"$W/fixture/mixed.txt"
PAD="$(printf 'x%.0s' $(seq 200))"
printf 'p12345678\xe4\xb8\x96\xe7\x95\x8c%s\n' "$PAD" >>"$W/fixture/mixed.txt"
printf 'content of wide file\n' >"$W/fixture/世界名.txt"

L1='xéyz\n'
L2='a👨‍👩‍👧c\n'
L3="p12345678世界$PAD\\n"
cat >"$W/fakebin/rg" <<RG
#!/bin/sh
printf '%s\n' \\
'{"type":"begin","data":{"path":{"text":"mixed.txt"}}}' \\
'{"type":"match","data":{"path":{"text":"mixed.txt"},"lines":{"text":"$L1"},"line_number":1,"submatches":[{"match":{"text":"́"},"start":2,"end":4}]}}' \\
'{"type":"match","data":{"path":{"text":"mixed.txt"},"lines":{"text":"$L2"},"line_number":2,"submatches":[{"match":{"text":"👩"},"start":8,"end":12}]}}' \\
'{"type":"match","data":{"path":{"text":"mixed.txt"},"lines":{"text":"$L3"},"line_number":3,"submatches":[{"match":{"text":"世界"},"start":9,"end":15}]}}' \\
'{"type":"end","data":{"path":{"text":"mixed.txt"},"binary_offset":null}}' \\
'{"type":"begin","data":{"path":{"text":"世界名.txt"}}}' \\
'{"type":"match","data":{"path":{"text":"世界名.txt"},"lines":{"text":"content of wide file\\n"},"line_number":1,"submatches":[{"match":{"text":"content"},"start":0,"end":7}]}}' \\
'{"type":"end","data":{"path":{"text":"世界名.txt"},"binary_offset":null}}' \\
'{"type":"summary","data":{}}'
exit 0
RG
chmod +x "$W/fakebin/rg"

cat >"$W/inner.sh" <<EOF
#!/bin/sh
cd "$W/fixture"
PATH="$W/fakebin:\$PATH" "$D/vrg" content . 2>"$W/replay.txt"
echo "\$?" >"$W/exit.txt"
sleep 60
EOF
chmod +x "$W/inner.sh"

cleanup() { tmux kill-session -t "$SES" 2>/dev/null; }
trap cleanup EXIT
tmux new-session -d -s "$SES" -x 80 -y 24 "$W/inner.sh"

pane() { tmux capture-pane -p -t "$SES" 2>/dev/null; }
epane() { tmux capture-pane -e -p -t "$SES" 2>/dev/null; }

wait_for() {
	for _ in $(seq 1 600); do
		pane | grep -qF "$1" && return 0
		sleep 0.05
	done
	echo "timeout waiting for: $1" >&2
	pane >&2
	exit 1
}

wait_for "mixed.txt"
wait_for "世界"
sleep 0.3

# --- Stage 1: cluster-exact highlights at offset 0 -------------------
epane >"$W/stage1.ansi"
python3 - "$W/stage1.ansi" <<'PY' || exit 1
import re, sys

def styled(row):
    """Text currently carrying the match style (bg 47 in its SGR block)."""
    out, inv = "", False
    for p in re.split(r'(\x1b\[[0-9;]*m)', row):
        if p.startswith('\x1b['):
            params = p[2:-1].split(';')
            if '0' in params or '40' in params or '49' in params:
                inv = False
            if '47' in params:
                inv = True
        elif inv:
            out += p
    return out

rows = open(sys.argv[1]).read().splitlines()
want = {"é": "combining-only match covers the whole é cluster",
        "👨‍👩‍👧": "interior match covers the whole ZWJ cluster",
        "世界": "the CJK match covers both cells of both clusters"}
seen = set()
for r in rows:
    s = styled(r)
    for k in want:
        if k in s:
            seen.add(k)
            print(f"styled run {k!r}: {want[k]}")
missing = want.keys() - seen
if missing:
    print("MISSING styled clusters:", missing)
    sys.exit(1)
PY
pane | sed -n '1,5p'

# --- Stage 2: w leaves wrap, one > pan splits 世 at the window edge ---
tmux send-keys -t "$SES" "w"
sleep 0.2
tmux send-keys -t "$SES" ">"
sleep 0.3
epane >"$W/stage2.ansi"
python3 - "$W/stage2.ansi" <<'PY' || exit 1
import re, sys
rows = open(sys.argv[1]).read().splitlines()
# Line 3's row carries the long x-run; the 世界名.txt list entry must
# not be mistaken for it.
line = next(r for r in rows if '界' in r and 'xxx' in r)
text = re.sub(r'\x1b\[[0-9;]*m', '', line)
# The split cluster's in-window cell is blank; 界 follows at once.
m = re.search(r'\d_? +([ 世]*界)', text)
if not m or '世' in m.group(0):
    print("split cluster painted part of 世:", repr(text))
    sys.exit(1)
print("clip edge blanked the split 世 cell; 界 paints whole:", repr(m.group(0)))
PY
pane | sed -n '1,5p'

# --- Stage 3: n n n opens the file-change pop-up for 世界名.txt ------
tmux send-keys -t "$SES" "n"
sleep 0.2
tmux send-keys -t "$SES" "n"
sleep 0.2
tmux send-keys -t "$SES" "n"
sleep 0.3
epane >"$W/stage3.ansi"
python3 - "$W/stage3.ansi" <<'PY' || exit 1
import re, sys, unicodedata

def w(s):
    n = 0
    for ch in s:
        if unicodedata.combining(ch):
            continue
        n += 2 if unicodedata.east_asian_width(ch) in 'WF' else 1
    return n

rows = open(sys.argv[1]).read().splitlines()
plain = [re.sub(r'\x1b\[[0-9;]*m', '', r) for r in rows]
box = [r for r in plain if '┌' in r or '│' in r or '└' in r]
if len(box) != 3:
    print("pop-up box rows not found:", box)
    sys.exit(1)
widths = [w(r.strip()) for r in box]
print("pop-up rows:", [r.strip() for r in box])
print("measured widths:", widths)
if len(set(widths)) != 1 or '世界名.txt' not in box[1]:
    print("pop-up borders misaligned or path missing")
    sys.exit(1)
print("pop-up interior carries the wide path; all border rows align")
PY
pane | sed -n '1,12p'

# --- Exit ------------------------------------------------------------
tmux send-keys -t "$SES" "q"
for _ in $(seq 1 200); do
	[ -f "$W/exit.txt" ] && break
	sleep 0.05
done
echo "exit=$(cat "$W/exit.txt" 2>/dev/null || echo missing)"
