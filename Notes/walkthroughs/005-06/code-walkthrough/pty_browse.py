#!/usr/bin/env python3
"""Issue #5 walkthrough: the real vrg binary browsing on a pty.

Builds a fixture directory inside this walkthrough directory containing
three matched files — one whose filename embeds a newline and an ESC
byte and whose matched line contains a raw OSC title-set sequence —
then drives the built binary on a pty three times:

  browse   — file list, filename rule, gutter, inverse-video matches,
             a mid-session resize, and q exiting 0.
  hostile  — the same fixture asserting on the RAW byte stream (what a
             terminal would execute): no OSC bytes, no BEL, no raw ESC
             inside the filename, escaped forms in all three sinks.
  gated    — VRG_TEST_LOAD_GATE holds the file load; "Loading…" renders
             and the UI stays responsive to resize until release.

The pty records the exact bytes a terminal would consume; the hostile
checks are therefore byte-level, not emulator-level.
"""
import fcntl
import os
import pty
import select
import signal
import struct
import sys
import termios
import time

HERE = os.path.dirname(os.path.abspath(__file__))
BIN = os.path.join(HERE, "vrg")
FIXTURE = os.path.join(HERE, "fixture")

# The hostile file: newline + ESC in the name, and a matched line
# carrying a raw OSC title-set sequence (ESC ] 0 ; PWNED BEL).
EVIL_NAME = "evil\n\x1bd.txt"
OSC_LINE = b"alpha \x1b]0;PWNED\x07rest\n"
# Displayed paths are resolved absolute, so the escaped name is a tail
# inside /home/.../fixture/./evil\n^[d.txt.
ESCAPED_NAME = b"./evil\\n^[d.txt"
ESCAPED_OSC = b"^[]0;PWNED^G"
RULE = "─"


def fail(msg):
    print("FAIL: " + msg)
    sys.exit(1)


def make_fixture():
    os.makedirs(FIXTURE, exist_ok=True)
    with open(os.path.join(FIXTURE, EVIL_NAME), "wb") as f:
        f.write(OSC_LINE)
    with open(os.path.join(FIXTURE, "normal.txt"), "wb") as f:
        f.write(b"alpha one\nbeta two\ngamma alpha\n")
    with open(os.path.join(FIXTURE, "zeta.txt"), "wb") as f:
        f.write(b"tail alpha\n")


def pump(fd, out, t=0.1):
    r, _, _ = select.select([fd], [], [], t)
    if r:
        try:
            out.extend(os.read(fd, 65536))
        except OSError:
            pass


class Session:
    """vrg running on its own pty; out accumulates the raw byte stream."""

    # 120 cols: the fixture's resolved paths are long, and the Issue #5
    # heuristic list width is longest+1 — a real panel needs the room.
    def __init__(self, env_extra=None, cols=120, rows=30):
        master, slave = pty.openpty()
        fcntl.ioctl(slave, termios.TIOCSWINSZ,
                    struct.pack("HHHH", rows, cols, 0, 0))
        env = dict(os.environ)
        env["TERM"] = "xterm"
        env.update(env_extra or {})
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
            os.execvpe(BIN, [BIN, "alpha", "."], env)
            os._exit(127)
        os.close(slave)
        self.pid = pid
        self.master = master
        self.out = bytearray()
        self.code = None

    def poll(self):
        if self.code is None:
            done, st = os.waitpid(self.pid, os.WNOHANG)
            if done:
                self.code = os.waitstatus_to_exitcode(st)
        return self.code

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

    def resize(self, cols, rows):
        fcntl.ioctl(self.master, termios.TIOCSWINSZ,
                    struct.pack("HHHH", rows, cols, 0, 0))
        os.kill(self.pid, signal.SIGWINCH)

    def send(self, data):
        os.write(self.master, data)

    def wait_exit(self, label, t=30):
        deadline = time.time() + t
        while time.time() < deadline:
            pump(self.master, self.out)
            if self.poll() is not None:
                while True:
                    r, _, _ = select.select([self.master], [], [], 0.3)
                    if not r:
                        break
                    try:
                        self.out.extend(os.read(self.master, 65536))
                    except OSError:
                        break
                os.close(self.master)
                return self.code
        self.kill()
        fail("%s: vrg did not exit" % label)

    def kill(self):
        try:
            os.kill(self.pid, 9)
            os.waitpid(self.pid, 0)
        except OSError:
            pass


def run_browse():
    s = Session()
    s.wait_for(b"PWNED", "browse")  # the current file's content loaded
    out = s.out
    # File list: every retained file, raw-path order.
    for name in (b"./normal.txt", b"./zeta.txt"):
        if name not in out:
            s.kill()
            fail("browse: file list missing %r" % name)
    # Escaped hostile name in the list AND embedded in the filename rule.
    if out.count(ESCAPED_NAME) < 2:
        s.kill()
        fail("browse: escaped name must appear in list and rule; output %r" % bytes(out))
    if ESCAPED_NAME + b" " + RULE.encode() not in out:
        s.kill()
        fail("browse: filename rule missing; output %r" % bytes(out))
    # Gutter: right-justified line number plus two spaces before content.
    if b"1  " not in out:
        s.kill()
        fail("browse: gutter missing; output %r" % bytes(out))
    # Inverse-video match and underlined current entry.
    if b"\x1b[7malpha" not in out:
        s.kill()
        fail("browse: no inverse-video match; output %r" % bytes(out))
    if b"\x1b[4m" not in out:
        s.kill()
        fail("browse: no underlined current entry; output %r" % bytes(out))

    # Resize mid-session: the frame recomposes wider — at 120 cols the
    # rule's trailing dash run is ~35, at 140 it is ~55.
    wide = RULE.encode() * 45
    if wide in out:
        s.kill()
        fail("browse: test error — 45-dash run already present at 120 cols")
    s.resize(140, 35)
    s.wait_for(wide, "browse-resize")

    s.send(b"q")
    if s.wait_exit("browse") != 0:
        fail("browse: exit=%d, want 0" % s.code)
    print("browse : file list, filename rule, gutter, inverse match, "
          "underline, resize recompose, q exit=0")


def run_hostile():
    s = Session()
    s.wait_for(b"PWNED", "hostile")
    s.send(b"q")
    if s.wait_exit("hostile") != 0:
        fail("hostile: exit=%d, want 0" % s.code)
    out = s.out
    # The raw stream is what a terminal would execute: no OSC bytes and
    # no BEL means no title-set (or any other control) sequence can run.
    if b"\x1b]" in out:
        fail("hostile: raw OSC survived to the terminal — title attack viable")
    if b"\x07" in out:
        fail("hostile: raw BEL survived to the terminal")
    if b"evil\x1bd" in out:
        fail("hostile: raw ESC survived inside the filename")
    if b"\x1b[?1049l" not in out:
        fail("hostile: alt-screen restoration missing")
    # Escaped forms in all three sinks: list, rule, panel content.
    if out.count(ESCAPED_NAME) < 2:
        fail("hostile: escaped name must appear in list and rule")
    if ESCAPED_OSC not in out:
        fail("hostile: OSC content not escaped; output %r" % bytes(out))
    print("hostile: no raw OSC/BEL/ESC in stream (title cannot change), "
          "escaped name in list+rule, OSC content as ^[]0;PWNED^G, q exit=0")


def run_gated():
    gate = os.path.join(FIXTURE, "load-gate")
    with open(gate, "w") as f:
        f.write("held")
    s = Session(env_extra={"VRG_TEST_LOAD_GATE": gate})
    s.wait_for(b"Loading", "gated")
    # Still responsive while the load worker is held: a resize must
    # produce a recomposed frame that still carries the placeholder.
    mark = len(s.out)
    s.resize(140, 35)
    s.wait_for(RULE.encode() * 45, "gated-resize")
    if b"Loading" not in s.out[mark:]:
        s.kill()
        fail("gated: recomposed frame lost the Loading… placeholder")
    os.remove(gate)
    s.wait_for(b"PWNED", "gated-release")
    s.send(b"q")
    if s.wait_exit("gated") != 0:
        fail("gated: exit=%d, want 0" % s.code)
    print("gated  : Loading… while held, resize responsive, release loads, "
          "q exit=0")


def main():
    if not os.path.exists(BIN):
        fail("build the binary first: go build -o %s ./cmd/vrg" % BIN)
    make_fixture()
    run_browse()
    run_hostile()
    run_gated()
    print("OK")


if __name__ == "__main__":
    main()
