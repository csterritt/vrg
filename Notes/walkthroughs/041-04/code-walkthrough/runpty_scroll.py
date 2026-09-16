#!/usr/bin/env python3
"""Drive vrg under a PTY for the Issue #41 full-scroll walkthrough.

Scenario (fatal outcome from fake rg, stderr taller than the visible
overlay): wait for the error overlay, press down row by row to the
last row, confirm clamping at the bottom, confirm u/d/PgUp/PgDn are
ignored, press up row by row back to the first row, confirm clamping
at the top, dismiss with q, quit with q.

Asserts and prints:
  - every one of the 60 diagnostic rows is rendered while traversing
    (no middle row is elided, no '...' ellipsis row appears)
  - overlayScroll clamps at both ends (frame stops changing)
  - the modal key contract: u, d, PageUp, PageDown have no effect
  - exit status 2 (fatal outcome)

Usage: runpty_scroll.py <vrg-binary> <workdir>
Env: VRG_FAKE_DIR (dir containing the fake rg), VRG_HANDSHAKE.
"""
import os, pty, re, sys, time, select, signal, struct, fcntl, termios
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
           "VRG_TEST_HANDSHAKE": handshake}
    os.execvpe(binary, [binary, "hello", "."], env)

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
    """Current screen rows containing overlay content markers."""
    stream.feed(drain(0.3))
    return list(screen.display)


def diag_rows(rows):
    seen = set()
    for r in rows:
        for m in re.finditer(r"diag-row-(\d+)", r):
            seen.add(int(m.group(1)))
    return seen


def text(rows):
    return "\n".join(rows)


def overlay_open(rows):
    return "FIRST-MARKER" in text(rows) or diag_rows(rows)


def send(b):
    os.write(fd, b)


DOWN, UP = b"\x1b[B", b"\x1b[A"
PGUP, PGDN = b"\x1b[5~", b"\x1b[6~"

# Wait for the fatal error overlay (handshake fired, FIRST-MARKER on
# screen).
t0 = time.monotonic()
while time.monotonic() - t0 < 30:
    rows = frame()
    if os.path.exists(handshake) and "FIRST-MARKER" in text(rows):
        break
    time.sleep(0.05)
else:
    print("FAIL: overlay never showed FIRST-MARKER")
    os.kill(pid, signal.SIGTERM)
    sys.exit(1)

print("overlay open: head row 'FIRST-MARKER' visible at scroll 0")
all_seen = set(diag_rows(rows))
ellipsis_seen = any(re.search(r"│\s*…\s*│", r) or r.strip() == "…"
                    for r in rows)

# Down traversal: press until LAST-MARKER is visible or the frame
# stops changing (bottom clamp), with a hard bound.
prev = text(rows)
stalls = 0
presses = 0
while "LAST-MARKER" not in text(rows) and stalls < 4 and presses < 80:
    send(DOWN)
    presses += 1
    rows = frame()
    all_seen |= diag_rows(rows)
    ellipsis_seen = ellipsis_seen or any(
        re.search(r"│\s*…\s*│", r) or r.strip() == "…" for r in rows)
    if text(rows) == prev:
        stalls += 1
    else:
        stalls = 0
        prev = text(rows)
print(f"down traversal: {presses} presses, "
      f"LAST-MARKER visible={'LAST-MARKER' in text(rows)}")

# Bottom clamp: more downs must not change the frame.
bottom = text(rows)
for _ in range(3):
    send(DOWN)
    rows = frame()
clamp_bottom = text(rows) == bottom
print(f"bottom clamp: extra downs leave frame unchanged = {clamp_bottom}")

# Ignored keys: u, d, PageUp, PageDown must not move the overlay.
for k in (b"u", b"d", PGUP, PGDN):
    send(k)
    rows = frame()
ignored_ok = text(rows) == bottom
print(f"ignored keys (u d PgUp PgDn): frame unchanged = {ignored_ok}")

# Up traversal back to the first row.
prev = text(rows)
stalls = 0
presses = 0
while "FIRST-MARKER" not in text(rows) and stalls < 4 and presses < 80:
    send(UP)
    presses += 1
    rows = frame()
    if text(rows) == prev:
        stalls += 1
    else:
        stalls = 0
        prev = text(rows)
print(f"up traversal: {presses} presses, "
      f"FIRST-MARKER visible={'FIRST-MARKER' in text(rows)}")

# Top clamp.
top = text(rows)
for _ in range(3):
    send(UP)
    rows = frame()
clamp_top = text(rows) == top
print(f"top clamp: extra ups leave frame unchanged = {clamp_top}")

# Dismiss with q, then q exits 2.
send(b"q")
rows = frame()
dismissed = not overlay_open(rows)
print(f"q dismisses overlay: browse restored = {dismissed}")
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

print(f"rows seen while traversing: {len(all_seen)}/58 middle rows, "
      f"ellipsis row seen: {ellipsis_seen}")
ok = (len(all_seen) == 58 and not ellipsis_seen and clamp_bottom
      and clamp_top and ignored_ok and dismissed and code == 2)
print(f"verdict: {'PASS' if ok else 'FAIL'}")
print(f"exit={code}")
sys.exit(0 if ok else 1)
