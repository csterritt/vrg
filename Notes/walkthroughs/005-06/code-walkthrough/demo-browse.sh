#!/bin/bash
# Browse-view demo on a real PTY: script(1) allocates a pseudo-terminal,
# an inner session sizes it 80x24 and runs ./vrg hit . in a fixture
# tree, a background stty resizes the pty to 30x100 mid-run, and the
# harness sends a literal q once the widened frame lands. Reports the
# file list, filename rule, gutter, underline and inverse-video
# evidence, the resize, and the exit status.
set -u
D="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
W="$D/manual-run"
rm -rf "$W"
mkdir -p "$W/fix/src"

printf 'alpha\nhit beta\ngamma\n' >"$W/fix/src/a.txt"
printf 'int hit = 0;\nno match\n' >"$W/fix/b.c"

mkfifo "$W/in"

cat >"$W/inner.sh" <<EOF
#!/bin/sh
cd "$W/fix"
stty rows 24 cols 80
( sleep 1.0; stty rows 30 cols 100 </dev/tty ) &
"$D/vrg" hit .
echo "\$?" >"$W/exit.txt"
EOF
chmod +x "$W/inner.sh"

timeout 30 script -qfec "$W/inner.sh" "$W/typescript" <"$W/in" >/dev/null 2>&1 &
spid=$!
exec 9>"$W/in"

# Wait for the widened post-resize frame: a 70+-dash filename rule only
# fits once the pty is 100 columns wide.
D70=$(printf '─%.0s' $(seq 70))
for _ in $(seq 1 600); do
	grep -aqF "$D70" "$W/typescript" 2>/dev/null && break
	sleep 0.05
done
printf 'q' >&9
wait "$spid" 2>/dev/null
exec 9>&-

T="$W/typescript"
# SGR styles become nothing; every other CSI (cursor moves included)
# becomes a newline so each rendered row is its own line.
rows() {
	sed $'s/\x1b\\[[0-9;]*m//g' "$T" | sed $'s/\x1b\\[[0-9;?><]*[a-zA-Z]/\\n/g' | tr -d '\r'
}

echo "exit=$(cat "$W/exit.txt" 2>/dev/null || echo missing)"
if grep -aq $'\x1b\[4m' "$T"; then echo "underline=seen"; else echo "underline=MISSING"; fi
if grep -aq $'\x1b\[7m' "$T"; then echo "inverse=seen"; else echo "inverse=MISSING"; fi
if grep -aqF "$D70" "$T"; then echo "resize=widened"; else echo "resize=MISSING"; fi
echo "--- rendered rows ---"
rows | grep -avE '^\s*$|^Script ' | head -14
