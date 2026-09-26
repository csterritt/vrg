#!/bin/bash
# Usage-error sink demo: a root operand carrying a raw ESC byte must
# render escaped on stderr with exit 2 — the diagnostic's single line
# shows the caret form and no fixture byte reaches the stream.
set -u
D="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
W="$D/manual-usage"
rm -rf "$W"
mkdir -p "$W"

"$D/vrg" foo "$(printf 'bad\x1bdir')" >"$W/stdout" 2>"$W/stderr"
echo "exit=$?"

T="$W/stderr"
check() { # check <label> <present|absent> <grep-args...> <file>
	label=$1; want=$2; shift 2
	if grep -aq "$@" "$T"; then got=present; else got=absent; fi
	if [ "$got" = "$want" ]; then echo "$label=$got"; else echo "$label=VIOLATION($got)"; fi
}
check "escaped-path"  present -F 'bad^[dir'
check "raw-esc"       absent  -F $'bad\x1bdir'
if [ -s "$W/stdout" ]; then echo "stdout-empty=VIOLATION"; else echo "stdout-empty=yes"; fi
echo "--- diagnostic line ---"
head -1 "$T"
