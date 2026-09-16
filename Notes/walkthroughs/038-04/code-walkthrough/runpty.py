#!/usr/bin/env python3
"""Run vrg under a PTY, send keys, and print the composed screen after
each key using pyte terminal emulation (Issue #38 walkthrough).

Environment variables:
  VRG_KEYS     - comma-separated keys to send (default: q). Named keys
               (up/down/left/right/tab/shift+tab/enter/esc/pgup/pgdn)
               map to their escape sequences; anything else is sent
               literally.
  VRG_DELAY    - seconds to wait before each key (default: 0.6)
  VRG_WIDTH    - terminal width (default: 80)
  VRG_HEIGHT   - terminal height (default: 24)

Each screen row is printed between | markers so the panel boundary,
padding, and the reserved right-indicator cell are visible.
"""
import os, pty, sys, time, select, signal, struct, fcntl, termios
import pyte

KEYMAP = {
    "up": "\x1b[A", "down": "\x1b[B", "right": "\x1b[C", "left": "\x1b[D",
    "tab": "\t", "shift+tab": "\x1b[Z", "enter": "\r", "esc": "\x1b",
    "pgup": "\x1b[5~", "pgdn": "\x1b[6~",
}

keys = os.environ.get("VRG_KEYS", "q").split(",")
delay = float(os.environ.get("VRG_DELAY", "0.6"))
width = int(os.environ.get("VRG_WIDTH", "80"))
height = int(os.environ.get("VRG_HEIGHT", "24"))

screen = pyte.Screen(width, height)
stream = pyte.ByteStream(screen)

pid, fd = pty.fork()
if pid == 0:
    os.execvp(sys.argv[1], sys.argv[1:])

winsize = struct.pack("HHHH", height, width, 0, 0)
fcntl.ioctl(fd, termios.TIOCSWINSZ, winsize)


def drain(timeout):
    out = b""
    while True:
        try:
            r, _, _ = select.select([fd], [], [], timeout)
            if r:
                data = os.read(fd, 65536)
                if not data:
                    break
                out += data
            else:
                break
        except OSError:
            break
    return out


def show(label):
    print(f"=== {label} ({width}x{height}) ===")
    for row in screen.display:
        print(f"|{row}|")
    print(flush=True)


out = drain(1.5)
stream.feed(out)
show("initial screen")

for key in keys:
    time.sleep(delay)
    try:
        os.write(fd, KEYMAP.get(key, key).encode())
    except OSError:
        break
    stream.feed(drain(0.5))
    show(f"after '{key}'")

drain(1.0)
finished, status = os.waitpid(pid, os.WNOHANG)
if finished == 0:
    # The key sequence did not exit vrg; terminate it and report.
    os.kill(pid, signal.SIGTERM)
    _, status = os.waitpid(pid, 0)
exitcode = os.waitstatus_to_exitcode(status)
print(f"exit={exitcode}")
sys.exit(exitcode)
