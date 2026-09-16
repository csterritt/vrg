#!/usr/bin/env python3
"""Drive vrg under a PTY for the Issue #43 fallback-cell walkthrough.

Scenario: combining.txt line 1 begins with a standalone combining
acute (CC 81) followed by 'x'; the fake rg reports a match whose
submatch covers exactly the mark's bytes [0,2). Under Issue #43 the
filebuffer prepends U+25CC (E2 97 8C), so '◌́' is one real width-1
cluster. Line 2's 'mid ́ mark' is the contrast case: a mark attached
to a preceding space is one ordinary cluster ' ́' — no fallback.

Asserts and prints:
  - '◌́' occupies exactly one screen cell and 'x' sits in the next
    cell with no overlap
  - the '◌́' cell carries the match style (inverse fg=black/bg=white);
    'x' is plain base colours — the highlight covers exactly the
    fallback cell and nothing adjacent
  - the attached mark on line 2 gets no '◌' (standalone-only rule)
  - 'w' + one right pan hides the fallback cell: 'x' becomes the first
    text cell and the '*' clipped-match indicator appears
  - exit status 0

Usage: runpty_fallback.py <vrg-binary> <workdir>
Env: VRG_FAKE_DIR (dir containing the fake rg), VRG_HANDSHAKE.
"""
import os, pty, sys, time, select, signal, struct, fcntl, termios
import pyte

binary = os.path.abspath(sys.argv[1])
workdir = os.path.abspath(sys.argv[2])
fake_dir = os.environ["VRG_FAKE_DIR"]
handshake = os.environ["VRG_HANDSHAKE"]
try:
    os.remove(handshake)
except FileNotFoundError:
    pass

W, H = 80, 24
screen = pyte.Screen(W, H)
stream = pyte.ByteStream(screen)

pid, fd = pty.fork()
if pid == 0:
    os.chdir(workdir)
    env = {"PATH": fake_dir + ":" + os.environ["PATH"],
           "VRG_TEST_HANDSHAKE": handshake, "TERM": "xterm-256color"}
    os.execvpe(binary, [binary, "́", "."], env)

fcntl.ioctl(fd, termios.TIOCSWINSZ, struct.pack("HHHH", H, W, 0, 0))


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


def frame():
    stream.feed(drain(0.3))
    return "\n".join(screen.display)


def send(b):
    os.write(fd, b)


def wait_for(marker, timeout, present=True):
    t0 = time.monotonic()
    while time.monotonic() - t0 < timeout:
        rows = frame()
        if (marker in rows) == present:
            return rows
        time.sleep(0.05)
    return frame()


def bail(msg):
    print("FAIL:", msg)
    os.kill(pid, signal.SIGTERM)
    os.waitpid(pid, 0)
    sys.exit(1)


def find_cells(pred):
    return [(y, x) for y in sorted(screen.buffer)
            for x in sorted(screen.buffer[y])
            if pred(screen.buffer[y][x])]


# Wait for the file to be visible: '◌' is emitted by the renderer.
rows = wait_for("◌", 30)
if "◌" not in rows:
    bail("fallback glyph ◌ never appeared")
print("file loaded: '◌' fallback glyph is visible in the view")
for i in range(1, 5):
    print(f"row {i}:", screen.display[i].rstrip())

# Line 1: the fallback cluster '◌́' occupies exactly one cell; 'x' is
# in the next cell — no overlap, no shared cell.
hits = find_cells(lambda ch: ch.data.startswith("◌"))
if len(hits) != 1:
    bail(f"expected exactly one '◌' cell, found {len(hits)}: {hits}")
y, x = hits[0]
xcell = screen.buffer[y].get(x + 1)
adjacent = xcell is not None and xcell.data == "x"
print(f"'◌́' at row {y} col {x} (one cell), 'x' in the next cell = {adjacent}")
if not adjacent:
    bail("'x' is not in the cell right after the fallback cluster")

# Highlight: the match covers the mark -> the '◌́' cell carries the
# current-match style (inverse fg=black bg=white in the dark scheme);
# 'x' is plain base colours. Underline spans the whole current line by
# design, so it is not part of this check.
fb = screen.buffer[y][x]
match_styled = fb.fg == "black" and fb.bg == "white"
x_plain = xcell.fg == "white" and xcell.bg == "black"
print(f"highlight: ◌́ fg={fb.fg} bg={fb.bg} (match) = {match_styled}; "
      f"x fg={xcell.fg} bg={xcell.bg} (plain) = {x_plain}")
if not (match_styled and x_plain):
    bail("highlight does not cover exactly the fallback cell")

# Contrast: file line 2's mark is attached to the preceding space (one
# ordinary cluster ' ́'), so no fallback glyph appears on that row.
# Line 1 wraps, so find the 'mid' row dynamically.
mid_row = next(r for r in range(H) if "mid" in screen.display[r])
mid_cells = screen.buffer[mid_row]
attached = any(ch.data.startswith(" ") and "́" in ch.data
               for ch in mid_cells.values())
no_fallback_row2 = "◌" not in "".join(
    ch.data for ch in mid_cells.values())
print(f"attached mark on file line 2 (row {mid_row}): cluster ' ́' "
      f"present = {attached}, no ◌ on row = {no_fallback_row2}")
if not (attached and no_fallback_row2):
    bail("line-2 attached mark should not get a fallback")

# Pan right one cell in run-off-edge mode: the fallback cell scrolls
# off, 'x' becomes the first text cell, and '*' marks the hidden match.
# 'w' triggers an async relayout — wait for run-off-edge rows (file
# line 2 back on screen row 2) before sending the pan key.
send(b"w")
t0 = time.monotonic()
while time.monotonic() - t0 < 10:
    if "mid" in screen.display[2]:
        break
    frame()
    time.sleep(0.05)
send(b".")
rows = wait_for("*", 10)
frame()
row1 = "".join(screen.buffer[1].get(c).data for c in range(W)
               if c in screen.buffer[1])
pan_ok = "1* x" in row1 and "◌" not in row1
print("row 1 after 'w' + right pan:", screen.display[1].rstrip())
print(f"x is first text cell, '*' clip indicator, ◌ scrolled off = {pan_ok}")
if not pan_ok:
    bail("panning did not count the fallback as a real cell")

send(b"q")
drain(1.0)
finished, status = os.waitpid(pid, os.WNOHANG)
if finished == 0:
    time.sleep(1)
    finished, status = os.waitpid(pid, os.WNOHANG)
if finished == 0:
    os.kill(pid, signal.SIGTERM)
    _, status = os.waitpid(pid, 0)
code = os.waitstatus_to_exitcode(status)

ok = (adjacent and match_styled and x_plain and attached
      and no_fallback_row2 and pan_ok and code == 0)
print(f"verdict: {'PASS' if ok else 'FAIL'}")
print(f"exit={code}")
sys.exit(0 if ok else 1)
