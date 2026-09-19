#!/usr/bin/env python3
"""Issue #6 walkthrough: the real vrg binary and every sink's safety.

Builds a fixture directory inside this walkthrough directory containing
a file whose name embeds a newline and an ESC byte and whose matched
line carries a raw OSC title-set sequence (ESC ] 0 ; PWNED BEL), then
checks the sinks the shared safe-presentation utility now covers:

  hostile — vrg on a pty; the RAW byte stream (what a terminal would
            execute) is asserted: no OSC bytes and no BEL survive, so
            no title-set sequence can run and the terminal title is
            unchanged; the escaped name appears in the file list and
            the filename rule, and the escaped OSC line appears in the
            panel content.
  usage   — `vrg foo "bad<ESC>dir"` without a pty: the control-byte
            path renders escaped on stderr and the process exits 2.
  help    — `vrg --help`: generated help on stdout is clean — exactly
            one help copy, no raw control bytes at all (the layout
            tabs expand through EscapeDiagnostic).

The pty records the exact bytes a terminal would consume; the checks
are byte-level, not emulator-level.
"""
import fcntl
import os
import pty
import select
import struct
import subprocess
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
# The usage-error operand and its escaped form.
BAD_ROOT = "bad\x1bdir"
ESCAPED_ROOT = b"bad^[dir"


def fail(msg):
    print("FAIL: " + msg)
    sys.exit(1)


def make_fixture():
    os.makedirs(FIXTURE, exist_ok=True)
    with open(os.path.join(FIXTURE, EVIL_NAME), "wb") as f:
        f.write(OSC_LINE)
    with open(os.path.join(FIXTURE, "normal.txt"), "wb") as f:
        f.write(b"alpha one\nbeta two\ngamma alpha\n")


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
    def __init__(self, cols=120, rows=30):
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


def run_hostile():
    s = Session()
    s.wait_for(b"PWNED", "hostile")
    s.send(b"q")
    if s.wait_exit("hostile") != 0:
        fail("hostile: exit=%d, want 0" % s.code)
    out = s.out
    # The raw stream is what a terminal would execute: no OSC bytes and
    # no BEL means no title-set (or any other control) sequence can run,
    # so the terminal title is unchanged.
    if b"\x1b]" in out:
        fail("hostile: raw OSC survived to the terminal — title attack viable")
    if b"\x07" in out:
        fail("hostile: raw BEL survived to the terminal")
    if b"evil\x1bd" in out:
        fail("hostile: raw ESC survived inside the filename")
    if b"\x1b[?1049l" not in out:
        fail("hostile: alt-screen restoration missing")
    # Escaped forms in all three browse sinks: file-list entry, filename
    # rule, and panel content.
    if out.count(ESCAPED_NAME) < 2:
        fail("hostile: escaped name must appear in list and rule")
    if ESCAPED_OSC not in out:
        fail("hostile: OSC content not escaped; output %r" % bytes(out))
    print("hostile: no raw OSC/BEL/ESC in stream (terminal title cannot "
          "change), escaped name in list+rule, OSC content as "
          "^[]0;PWNED^G, q exit=0")


def run_usage_error():
    res = subprocess.run([BIN, "foo", BAD_ROOT], capture_output=True)
    if res.returncode != 2:
        fail("usage: exit=%d, want 2 (stderr %r)" % (res.returncode, res.stderr))
    if res.stdout:
        fail("usage: stdout not empty: %r" % res.stdout)
    err = res.stderr
    if b"\x1b" in err or b"\x9b" in err or b"\x07" in err:
        fail("usage: raw control byte reached stderr: %r" % err)
    if ESCAPED_ROOT not in err:
        fail("usage: escaped operand %r missing from stderr: %r"
             % (ESCAPED_ROOT, err))
    diag = err.split(b"\n", 1)[0]
    if not diag.startswith(b"vrg:") or b"\x1b" in diag:
        fail("usage: first stderr line is not a sanitized diagnostic: %r" % diag)
    if err.count(b"Usage:") != 1:
        fail("usage: stderr lacks the single usage block: %r" % err)
    print("usage  : control-byte path escaped as bad^[dir on stderr, "
          "diagnostic + usage block, exit=2")


def run_help():
    res = subprocess.run([BIN, "--help"], capture_output=True)
    if res.returncode != 0:
        fail("help: exit=%d, want 0" % res.returncode)
    if res.stderr:
        fail("help: stderr not empty: %r" % res.stderr)
    out = res.stdout
    if out.count(b"Usage:") != 1 or not out.startswith(b"Usage:"):
        fail("help: not exactly one generated help copy: %r" % out)
    for b in out:
        if b < 0x20 and b != 0x0A:
            fail("help: raw control byte %#04x on stdout: %r" % (b, out))
    print("help   : CLI-help stdout clean — one help copy, no raw "
          "control byte (tabs expanded), exit=0")


def main():
    if not os.path.exists(BIN):
        fail("build the binary first: go build -o %s ./cmd/vrg" % BIN)
    make_fixture()
    run_hostile()
    run_usage_error()
    run_help()
    print("OK")


if __name__ == "__main__":
    main()
