#!/bin/bash
# Manual long-search cancellation on a real PTY: script(1) allocates a
# pseudo-terminal, an inner session records `stty -a` before and after
# vrg runs, real rg scans /usr (a multi-second search), and the harness
# sends a literal q keystroke once the Searching… frame is up. Reports
# the searching frame, the process exit status, orphaned rg processes,
# display-restoration evidence, and termios equality.
set -u
D="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
W="$D/manual-run"
rm -rf "$W"
mkdir -p "$W"
mkfifo "$W/in"

cat >"$W/inner.sh" <<EOF
#!/bin/sh
stty rows 24 cols 80
stty -a >"$W/stty-before.txt"
"$D/vrg" foo /usr
echo "\$?" >"$W/exit.txt"
stty -a >"$W/stty-after.txt"
EOF
chmod +x "$W/inner.sh"

before_rg=$(pgrep -xc rg || true)

timeout 30 script -qfec "$W/inner.sh" "$W/typescript" <"$W/in" >/dev/null 2>&1 &
spid=$!
exec 9>"$W/in"

for _ in $(seq 1 600); do
	grep -aq 'Searching' "$W/typescript" 2>/dev/null && break
	sleep 0.05
done
printf 'q' >&9
wait "$spid" 2>/dev/null
exec 9>&-

after_rg=$(pgrep -xc rg || true)

if grep -aq 'Searching' "$W/typescript"; then
	echo "searching-frame=seen"
else
	echo "searching-frame=MISSING"
fi
echo "process-exit=$(cat "$W/exit.txt" 2>/dev/null || echo missing)"
echo "rg-procs: before=$before_rg after=$after_rg"
if grep -aq $'\x1b\[?1049l' "$W/typescript" && grep -aq $'\x1b\[?25h' "$W/typescript"; then
	echo "display-restore=leave-alt-screen+cursor-show"
else
	echo "display-restore=MISSING"
fi
if cmp -s "$W/stty-before.txt" "$W/stty-after.txt"; then
	echo "termios=identical"
else
	echo "termios=DIFFERS"
	diff "$W/stty-before.txt" "$W/stty-after.txt" | head -20
fi
