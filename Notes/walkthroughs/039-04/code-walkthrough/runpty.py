#!/usr/bin/env python3
"""Run vrg under a PTY, send keys, and print the composed screen after
each key using pyte terminal emulation (Issue #39 walkthrough).

Environment variables:
  VRG_KEYS     - comma-separated keys to send (default: q). Named keys
               (up/down/left/right/tab/shift+tab/enter/esc/pgup/pgdn)
               map to their escape sequences; anything else is sent
               literally.
  VRG_DELAY    - seconds to wait before each key (default: 0.6)
  VRG_WIDTH    - terminal width (default: 80)
  VRG_HEIGHT   - terminal height (default: 24)

Each screen row is printed between | markers. Beneath any row that
contains match-styled cells a marker row prints one ^ per styled cell
(the dark scheme's match style paints a white background), so highlight
coverage can be checked cell-by-cell against the grapheme clusters in
the text row. Wide characters render across two cells in the text row,
keeping the two rows visually aligned.
"""
import os, pty, sys, time, select, signal, struct, fcntl, termios
import pyte

# Put the bundled fake rg first on PATH inside the child process so the
# demo is independent of how the outer environment (e.g. uv run)
# rewrites PATH.
_here = os.path.dirname(os.path.abspath(__file__))
os.environ["PATH"] = os.path.join(_here, "fakebin") + os.pathsep + os.environ["PATH"]

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
    for y, row in enumerate(screen.display):
        print(f"|{row}|")
        marks = "".join(
            "^" if screen.buffer[y][x].bg == "white" else " "
            for x in range(width)
        )
        if "^" in marks:
            print(f"|{marks}|")
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
