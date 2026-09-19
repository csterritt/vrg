#!/usr/bin/env python3
"""Issue #7 walkthrough: `c` swaps the colour scheme on a real pty.

Runs the real vrg binary on a pty against a fixture Go tree
(`vrg func .`), then presses c twice and q, asserting on the RAW byte
stream a terminal would execute:

  dark   — the initial frame is white on black (SGR 37;40), the match
           on the current matched line is the true inverse (black on
           white, 30;47) plus underline (;4), and the current file-list
           entry is underlined (37;40;4).
  light  — after the first c the frame is black on white (30;47); the
           current-line match stays the true inverse — now white on
           black (37;40;4) — and the entry stays underlined (30;47;4).
  dark2  — the second c restores the dark frame.
  quit   — q exits 0 after leaving the alt screen.
"""
import fcntl
import os
import pty
import select
import struct
import sys
import termios
import time

HERE = os.path.dirname(os.path.abspath(__file__))
BIN = os.path.join(HERE, "vrg")
FIXTURE = os.path.join(HERE, "fixture")


def fail(msg):
    print("FAIL: " + msg)
    sys.exit(1)


def pump(fd, out, t=0.1):
    r, _, _ = select.select([fd], [], [], t)
    if r:
        try:
            out.extend(os.read(fd, 65536))
        except OSError:
            pass


class Session:
    """vrg running on its own pty; out accumulates the raw byte stream."""

    def __init__(self, cols=100, rows=30):
        master, slave = pty.openpty()
        fcntl.ioctl(slave, termios.TIOCSWINSZ,
                    struct.pack("HHHH", rows, cols, 0, 0))
        env = dict(os.environ)
        env["TERM"] = "xterm"
        pid = os.fork()
        if pid == 0:
            os.setsid()
            fcntl.ioctl(slave, termios.TIOCSCTTY, 0)
            os.dup2(slave, 0)
            os.dup2(slave, 1)
            os.dup2(slave, 2)
            os.close(master)
            if slave > 2:
                os.close(slave)
            os.chdir(FIXTURE)
            os.execvpe(BIN, [BIN, "func", "."], env)
            os._exit(127)
        os.close(slave)
        self.pid = pid
        self.master = master
        self.out = bytearray()

    def poll(self):
        done, st = os.waitpid(self.pid, os.WNOHANG)
        return os.waitstatus_to_exitcode(st) if done else None

    def wait_for(self, needle, label, t=30):
        deadline = time.time() + t
        while time.time() < deadline:
            pump(self.master, self.out)
            if needle in self.out:
                return
            if self.poll() is not None:
                break
        self.kill()
        fail("%s: %r never appeared; output %r" % (label, needle, bytes(self.out)))

    def send(self, data):
        os.write(self.master, data)

    def frame_since(self, mark, t=5):
        """Return the bytes appended since mark once output settles."""
        deadline = time.time() + t
        last = -1
        while time.time() < deadline:
            pump(self.master, self.out, 0.1)
            if len(self.out) == last:
                return bytes(self.out[mark:])
            last = len(self.out)
        return bytes(self.out[mark:])

    def wait_exit(self, label, t=30):
        deadline = time.time() + t
        while time.time() < deadline:
            pump(self.master, self.out)
            code = self.poll()
            if code is not None:
                while True:
                    r, _, _ = select.select([self.master], [], [], 0.3)
                    if not r:
                        break
                    try:
                        self.out.extend(os.read(self.master, 65536))
                    except OSError:
                        break
                os.close(self.master)
                return code
        self.kill()
        fail("%s: vrg did not exit" % label)

    def kill(self):
        try:
            os.kill(self.pid, 9)
            os.waitpid(self.pid, 0)
        except OSError:
            pass


def check_scheme(frame, label, base, inverse, curmatch, curfile):
    """Assert one rendered frame carries the expected scheme's styles."""
    for want, what in [
        (base, "base colours"),
        (inverse + b"func", "non-current match in true inverse"),
        (curmatch + b"func", "current-line match in true inverse + underline"),
        (curfile, "current file-list entry underlined"),
    ]:
        if want not in frame:
            fail("%s: %s missing — want %r in %r" % (label, what, want, frame))


def main():
    if not os.path.exists(BIN):
        fail("build the binary first: go build -o %s ./cmd/vrg" % BIN)

    s = Session()
    # The browse view's first frame: the filename rule's "─ " marker is
    # present while loading and loaded alike; the styled match follows.
    s.wait_for(b"\x1b[30;47;4mfunc", "dark")

    dark = s.frame_since(0)
    check_scheme(dark, "dark", b"\x1b[37;40m", b"\x1b[30;47m",
                 b"\x1b[30;47;4m", b"\x1b[37;40;4m")
    print("dark   : white on black (37;40); current-line match "
          "30;47;4mfunc, other match 30;47m, list entry underlined")

    mark = len(s.out)
    s.send(b"c")
    light = s.frame_since(mark)
    check_scheme(light, "light", b"\x1b[30;47m", b"\x1b[37;40m",
                 b"\x1b[37;40;4m", b"\x1b[30;47;4m")
    print("light  : c swaps to black on white (30;47); current-line "
          "match 37;40;4mfunc, other match 37;40m, entry underlined")

    mark = len(s.out)
    s.send(b"c")
    dark2 = s.frame_since(mark)
    check_scheme(dark2, "dark2", b"\x1b[37;40m", b"\x1b[30;47m",
                 b"\x1b[30;47;4m", b"\x1b[37;40;4m")
    print("dark2  : second c returns to white on black")

    s.send(b"q")
    code = s.wait_exit("quit")
    if code != 0:
        fail("quit: exit=%d, want 0" % code)
    if b"\x1b[?1049l" not in s.out:
        fail("quit: alt-screen restoration missing")
    print("quit   : q exits 0, alt screen restored")
    print("OK")


if __name__ == "__main__":
    main()
