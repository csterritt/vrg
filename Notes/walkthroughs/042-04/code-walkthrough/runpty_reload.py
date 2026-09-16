#!/usr/bin/env python3
"""Drive vrg under a PTY for the Issue #42 reload-admission walkthrough.

Scenario (slow-loading file via a FIFO): the fake rg reports a match
at line 40 of slow.txt; vrg's os.ReadFile on the FIFO blocks until a
writer supplies the content, so the startup load stays in flight.
Press 'r' twice while it is in flight — nothing visible may change.
Then supply the content: the load must complete under its original
classification, revealing the destination match (line 40 scrolled
into view) rather than preserving the top-of-file anchor as if a
reload had happened. Finally press 'r' again — now accepted — and
supply fresh content to prove the placeholder->content completion
signal.

Asserts and prints:
  - the frame is identical after the in-flight 'r' presses (nothing
    visible changed)
  - after the FIFO write, MATCH-TARGET is visible and line-01 is not
    (destination reveal to line 40, not anchor preservation at the
    top of file)
  - an accepted 'r' shows Loading… again and the new content replaces
    it (placeholder->content is the completion signal)
  - exit status 0

Usage: runpty_reload.py <vrg-binary> <workdir>
Env: VRG_FAKE_DIR (dir containing the fake rg), VRG_HANDSHAKE.
"""
import os, pty, sys, time, select, signal, struct, fcntl, termios
import pyte

binary = os.path.abspath(sys.argv[1])
workdir = os.path.abspath(sys.argv[2])
fake_dir = os.environ["VRG_FAKE_DIR"]
handshake = os.environ["VRG_HANDSHAKE"]
fifo = os.path.join(workdir, "slow.txt")
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
    os.execvpe(binary, [binary, "MATCH-TARGET", "."], env)

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


def write_fifo(content):
    # Opening a FIFO for writing pairs with vrg's blocked os.ReadFile;
    # closing produces the EOF that completes the load.
    with open(fifo, "w") as f:
        f.write(content)


def make_content(tag):
    lines = ["line-%02d filler text\n" % i for i in range(1, 51)]
    lines[39] = "line-40 %s\n" % tag
    return "".join(lines)


def bail(msg):
    print("FAIL:", msg)
    os.kill(pid, signal.SIGTERM)
    os.waitpid(pid, 0)
    sys.exit(1)


# Wait for the startup load to be in flight: the fake rg has finished
# (handshake exists) and the panel shows the Loading… placeholder
# while os.ReadFile blocks on the FIFO.
t0 = time.monotonic()
while time.monotonic() - t0 < 30:
    rows = frame()
    if os.path.exists(handshake) and "Loading" in rows:
        break
    time.sleep(0.05)
else:
    bail("startup load never showed Loading…")
print("startup load in flight: 'Loading…' placeholder shown, FIFO blocks the read")

# Press r twice while the load is in flight. Both presses must be
# dropped without any visible change — no presentation, intent, or
# revision mutation.
before = frame()
send(b"r")
time.sleep(0.3)
mid = frame()
send(b"r")
time.sleep(0.3)
after = frame()
unchanged = before == mid == after and "Loading" in after
print(f"dropped r presses: frame unchanged and still Loading… = {unchanged}")
if not unchanged:
    bail("frame changed after in-flight r presses")

# Supply the content: the in-flight load completes under its original
# classification — the destination reveal scrolls line 40 into view
# (offset ~27), it does NOT preserve the top-of-file anchor.
write_fifo(make_content("MATCH-TARGET"))
rows = wait_for("MATCH-TARGET", 15)
reveal_ok = "MATCH-TARGET" in rows and "line-01" not in rows and "Loading" not in rows
print(f"completion: MATCH-TARGET visible, line-01 off-screen, "
      f"Loading gone = {reveal_ok}")
if "line-01" in rows:
    print("  (anchor-preserved top-of-file would show line-01 — misclassified reload)")
if not reveal_ok:
    bail("completion did not reveal the destination match")

# Press r now that the load settled — accepted: the panel switches to
# Loading… again, and fresh content replaces it on completion.
send(b"r")
rows = wait_for("Loading", 5)
accepted_loading = "Loading" in rows and "RELOADED-TARGET" not in rows
print(f"accepted r: Loading… presentation applied = {accepted_loading}")
if not accepted_loading:
    bail("accepted r did not show Loading…")

write_fifo(make_content("RELOADED-TARGET"))
rows = wait_for("RELOADED-TARGET", 15)
reloaded = "RELOADED-TARGET" in rows and "Loading" not in rows
print(f"reload completion: RELOADED-TARGET visible, Loading gone = {reloaded}")
if not reloaded:
    bail("accepted reload did not complete to new content")

# Quit: browse state exits with the fixed status 0 (rg exited 0).
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

ok = unchanged and reveal_ok and accepted_loading and reloaded and code == 0
print(f"verdict: {'PASS' if ok else 'FAIL'}")
print(f"exit={code}")
sys.exit(0 if ok else 1)
