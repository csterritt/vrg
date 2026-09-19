#!/usr/bin/env python3
"""Issue #8 walkthrough: the no-results screen on a real pty.

Runs the real vrg binary on a pty in three sessions and asserts on the
RAW byte stream a terminal would execute:

  empty    — `vrg zzzznotfound .` in a dir of text: rg exits 1 with a
             summary-only stream; the centred "No results found"
             appears; q exits 1 after leaving the alt screen.
  traversed— `vrg foo .` in a dir holding only the binary b.bin: rg
             15.x drops traversed binary files silently (no records at
             all), so the same plain "No results found" shows; q → 1.
  operand  — `vrg foo b.bin` with the binary file as the search root:
             rg emits the match then an end with binary_offset, so the
             screen reads "No results found (1 binary files skipped)";
             q → 1.
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

    def __init__(self, cwd, args, cols=100, rows=30):
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
            os.chdir(cwd)
            os.execvpe(BIN, [BIN] + args, env)
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


def check_noresults(s, label, want):
    """Assert the no-results line rendered, centred on the frame."""
    s.wait_for(b"No results found", label)
    if want not in s.out:
        fail("%s: %r missing; output %r" % (label, want, bytes(s.out)))
    # Centred: the renderer emits the line preceded by its left padding
    # (a leading space follows the cursor-positioning inside the frame).
    if b" " + want.strip() not in s.out:
        fail("%s: no-results line does not look centred; output %r"
             % (label, bytes(s.out)))


def check_quit(s, label):
    """q dismisses the no-results screen to exit 1; alt screen restored."""
    s.send(b"q")
    code = s.wait_exit(label + " quit")
    if code != 1:
        fail("%s: exit=%d, want 1" % (label, code))
    if b"\x1b[?1049l" not in s.out:
        fail("%s: alt-screen restoration missing" % label)


def main():
    if not os.path.exists(BIN):
        fail("build the binary first: go build -o %s ./cmd/vrg" % BIN)

    s = Session(os.path.join(FIXTURE, "empty"), ["zzzznotfound", "."])
    check_noresults(s, "empty", b"No results found")
    if b"binary files skipped" in s.out:
        fail("empty: binary-skip suffix must not appear — nothing was "
             "excluded; output %r" % bytes(s.out))
    check_quit(s, "empty")
    print("empty    : vrg zzzznotfound . shows \"No results found\", "
          "no suffix; q exits 1, alt screen restored")

    s = Session(os.path.join(FIXTURE, "binonly"), ["foo", "."])
    check_noresults(s, "traversed", b"No results found")
    if b"binary files skipped" in s.out:
        fail("traversed: rg emits no records for a traversed binary "
             "file, so no suffix may appear; output %r" % bytes(s.out))
    check_quit(s, "traversed")
    print("traversed: vrg foo . over the binary-only dir shows plain "
          "\"No results found\" (rg drops it silently); q exits 1")

    s = Session(os.path.join(FIXTURE, "binonly"), ["foo", "b.bin"])
    check_noresults(s, "operand",
                    b"No results found (1 binary files skipped)")
    check_quit(s, "operand")
    print("operand  : vrg foo b.bin shows \"No results found "
          "(1 binary files skipped)\"; q exits 1")
    print("OK")


if __name__ == "__main__":
    main()
