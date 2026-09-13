#!/bin/sh
# Run vrg with the given keys, strip ANSI, and extract visible line
# numbers from the LAST rendered frame only. The PTY captures all
# frames; the last contentHeight (23) line-number matches belong to
# the final frame. Usage: showlines.sh "keys"
cd "$(dirname "$0")"
VRG_KEYS="$1" VRG_DELAY="0.3" VRG_WIDTH="80" VRG_HEIGHT="24" \
  VRG_RAW="1" VRG_STDERR="stderr.txt" \
  PATH="$PWD/fakebin:$PATH" \
  python3 runpty.py ./vrg match . 2>&1 |
  sed 's/\x1b\[[0-9;]*[A-Za-z]//g; s/\x1b\[?[0-9;]*[A-Za-z]//g; s/\r//g' |
  grep -oP 'line \d+' | tail -23 | sort -t' ' -k2 -n | uniq
