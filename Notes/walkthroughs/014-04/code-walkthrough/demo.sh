#!/usr/bin/env bash
# Demonstration script for Issue #14 vertical destination reveal.
# Runs the vrg binary against a long file with matches on lines 5 and 200.
# Uses a pty to simulate interactive keypresses and captures the screen
# after each navigation action.
#
# This script is self-contained: it builds the binary and creates the
# test fixture file if they are missing.
set -euo pipefail
cd "$(dirname "$0")"
VRG_ROOT="$(cd ../../../.. && pwd)"

# Build the binary if missing.
if [ ! -x ./vrg-demo ]; then
  (cd "$VRG_ROOT" && go build -o Notes/walkthroughs/014-04/code-walkthrough/vrg-demo ./cmd/vrg)
fi

# Create the test fixture file if missing.
if [ ! -f ./longfile.txt ]; then
  python3 -c "
lines = []
for i in range(1, 251):
    if i == 5:
        lines.append('line %d: TARGET match here' % i)
    elif i == 200:
        lines.append('line %d: TARGET match here' % i)
    else:
        lines.append('line %d: ordinary content' % i)
with open('longfile.txt', 'w') as f:
    f.write('\n'.join(lines) + '\n')
"
fi

python3 << 'PYEOF'
import os
import pty
import select
import time
import re
import struct
import fcntl
import termios

def drain(fd, settle=0.4):
    """Drain all available output from the pty."""
    time.sleep(settle)
    out = b""
    while True:
        r, _, _ = select.select([fd], [], [], 0.15)
        if not r:
            break
        try:
            chunk = os.read(fd, 65536)
        except OSError:
            break
        if not chunk:
            break
        out += chunk
    return out

def clean_ansi(data):
    """Strip ANSI escape sequences and control codes for readability."""
    # Remove CSI sequences
    s = re.sub(rb'\x1b\[[0-9;?]*[a-zA-Z]', b'', data)
    # Remove other escape sequences
    s = re.sub(rb'\x1b[=>]', b'', s)
    s = re.sub(rb'\x1b\].*?\x07', b'', s)
    # Remove remaining control chars except newline
    s = re.sub(rb'[\x00-\x08\x0b-\x1f\x7f]', b'', s)
    return s.decode('utf-8', errors='replace')

def show_screen(data, label):
    """Print a labeled, cleaned screen capture."""
    print(f"=== {label} ===")
    clean = clean_ansi(data)
    lines = [l for l in clean.split('\n') if l.strip()]
    for i, line in enumerate(lines):
        # Truncate to 80 cols
        print(f"  {line[:80]}")
    print()

# Start the vrg binary in a pty.
pid, fd = pty.fork()
if pid == 0:
    os.execv("./vrg-demo", ["vrg-demo", "--", "TARGET", "longfile.txt"])
    os._exit(1)

# Set terminal size to 24 rows, 80 columns.
winsize = struct.pack("HHHH", 24, 80, 0, 0)
fcntl.ioctl(fd, termios.TIOCSWINSZ, winsize)

# Wait for search + startup load + reveal.
initial = drain(fd, 1.5)
show_screen(initial, "Initial screen (startup reveal of line 5)")

# Press n to navigate to line 200 (row 199).
# With contentHeight 23, floor(23/3) = 7. Reveal: offset = 199 - 7 = 192.
os.write(fd, b'n')
n1 = drain(fd, 0.6)
show_screen(n1, "After n (navigate to line 200, one-third placement)")

# Press p to navigate back to line 5 (row 4).
# Reveal: offset = 4 - 7 = -3, clamped to 0 (BOF).
os.write(fd, b'p')
p1 = drain(fd, 0.6)
show_screen(p1, "After p (navigate back to line 5, BOF clamp)")

# Press n again to go to line 200.
os.write(fd, b'n')
n2 = drain(fd, 0.6)
show_screen(n2, "After n again (line 200, one-third placement)")

# Quit
os.write(fd, b'q')
time.sleep(0.2)
try:
    os.close(fd)
except OSError:
    pass
try:
    os.waitpid(pid, 0)
except OSError:
    pass
print("=== Demo complete ===")
PYEOF
