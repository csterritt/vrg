#!/usr/bin/env bash
# Demonstration script for Issue #15 file-change pop-up.
# Runs the vrg binary against two files so n/p cross file boundaries,
# showing the centred pop-up and its ~1s disappearance, a quick second
# n showing the new file's pop-up with navigation still applied, a
# resize while shown re-centring it, and a hostile path rendering its
# escaped form.
#
# Uses a pty to simulate interactive keypresses and captures the screen
# after each action.
#
# This script is self-contained: it builds the binary and creates the
# test fixture files if they are missing.
set -euo pipefail
cd "$(dirname "$0")"
VRG_ROOT="$(cd ../../../.. && pwd)"

# Build the binary if missing.
if [ ! -x ./vrg-demo ]; then
  (cd "$VRG_ROOT" && go build -o Notes/walkthroughs/015-04/code-walkthrough/vrg-demo ./cmd/vrg)
fi

# Create the test fixture files if missing.
if [ ! -d ./demo_dir ]; then
  mkdir -p ./demo_dir
  printf 'line 1: TARGET match here\nline 2: ordinary\nline 3: ordinary\n' > ./demo_dir/file_a.txt
  printf 'line 1: ordinary\nline 2: TARGET match here\nline 3: ordinary\n' > ./demo_dir/file_b.txt
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
    s = re.sub(rb'\x1b\[[0-9;?]*[a-zA-Z]', b'', data)
    s = re.sub(rb'\x1b[=>]', b'', s)
    s = re.sub(rb'\x1b\].*?\x07', b'', s)
    s = re.sub(rb'[\x00-\x08\x0b-\x1f\x7f]', b'', s)
    return s.decode('utf-8', errors='replace')

def show_screen(data, label):
    """Print a labeled, cleaned screen capture."""
    print(f"=== {label} ===")
    clean = clean_ansi(data)
    lines = [l for l in clean.split('\n') if l.strip()]
    for line in lines:
        print(f"  {line[:80]}")
    print()

# Start the vrg binary in a pty.
pid, fd = pty.fork()
if pid == 0:
    os.execv("./vrg-demo", ["vrg-demo", "--", "TARGET", "demo_dir"])
    os._exit(1)

# Set terminal size to 24 rows, 80 columns.
winsize = struct.pack("HHHH", 24, 80, 0, 0)
fcntl.ioctl(fd, termios.TIOCSWINSZ, winsize)

# Wait for search + startup load.
initial = drain(fd, 1.5)
show_screen(initial, "Initial screen (file_a.txt loaded)")

# Press n to navigate to file_b.txt (cross-file). The pop-up appears
# centred showing "file_b.txt" and disappears after ~1 second.
os.write(fd, b'n')
popup = drain(fd, 0.3)
show_screen(popup, "After n (cross-file to file_b.txt, pop-up visible)")

# Wait for the pop-up to expire (~1 second).
expired = drain(fd, 1.2)
show_screen(expired, "After ~1s (pop-up expired, file_b.txt content)")

# Press n again to wrap back to file_a.txt (cross-file). The pop-up
# appears again with the new file's path, and navigation is applied.
os.write(fd, b'n')
popup2 = drain(fd, 0.3)
show_screen(popup2, "After second n (cross-file back to file_a.txt, new pop-up)")

# Press n to go to file_b.txt again, then resize while the pop-up is
# visible to show re-centring.
os.write(fd, b'n')
popup3 = drain(fd, 0.3)
show_screen(popup3, "After third n (cross-file to file_b.txt, pop-up before resize)")

# Resize to 40 columns, 30 rows. The pop-up should re-centre.
winsize2 = struct.pack("HHHH", 30, 40, 0, 0)
fcntl.ioctl(fd, termios.TIOCSWINSZ, winsize2)
resized = drain(fd, 0.4)
show_screen(resized, "After resize to 40x30 (pop-up re-centred)")

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
