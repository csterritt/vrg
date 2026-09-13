#!/usr/bin/env bash
# Demonstration script for Issue #15 file-change pop-up with a hostile
# (invalid UTF-8) path. Runs the vrg binary against a file whose name
# contains \xff\xfe so the pop-up must escape the path for safe display.
set -euo pipefail
cd "$(dirname "$0")"
VRG_ROOT="$(cd ../../../.. && pwd)"

# Build the binary if missing.
if [ ! -x ./vrg-demo ]; then
  (cd "$VRG_ROOT" && go build -o Notes/walkthroughs/015-04/code-walkthrough/vrg-demo ./cmd/vrg)
fi

# Create the test fixture files if missing.
if [ ! -d ./hostile_dir ]; then
  mkdir -p ./hostile_dir
  printf 'line 1: TARGET match here\n' > ./hostile_dir/aaa_safe.txt
  printf 'line 1: TARGET match here\n' > $'./hostile_dir/foo\xff\xfebar.txt'
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
    s = re.sub(rb'\x1b\[[0-9;?]*[a-zA-Z]', b'', data)
    s = re.sub(rb'\x1b[=>]', b'', s)
    s = re.sub(rb'\x1b\].*?\x07', b'', s)
    s = re.sub(rb'[\x00-\x08\x0b-\x1f\x7f]', b'', s)
    return s.decode('utf-8', errors='replace')

def show_screen(data, label):
    print(f"=== {label} ===")
    clean = clean_ansi(data)
    lines = [l for l in clean.split('\n') if l.strip()]
    for line in lines:
        print(f"  {line[:80]}")
    print()

pid, fd = pty.fork()
if pid == 0:
    os.execv("./vrg-demo", ["vrg-demo", "--", "TARGET", "hostile_dir"])
    os._exit(1)

winsize = struct.pack("HHHH", 24, 80, 0, 0)
fcntl.ioctl(fd, termios.TIOCSWINSZ, winsize)

initial = drain(fd, 1.5)
show_screen(initial, "Initial screen (aaa_safe.txt loaded)")

# Press n to navigate to the hostile-path file. The pop-up must show
# the escaped form of the path (foo\xff\xfebar -> foo\\xff\\xfebar).
os.write(fd, b'n')
popup = drain(fd, 0.3)
show_screen(popup, "After n (cross-file to hostile path, pop-up shows escaped form)")

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
print("=== Hostile-path demo complete ===")
PYEOF
