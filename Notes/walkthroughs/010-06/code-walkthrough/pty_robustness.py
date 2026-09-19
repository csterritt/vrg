#!/usr/bin/env python3
"""Issue #10 walkthrough: the manual fake-rg stream from the issue,
driven against the real vrg binary on a pty with raw-byte assertions.

  robust — a valid begin for ./a.txt, a valid match, a garbage line, a
           {"type":"weird"} line, a valid end for ./a.txt (null
           binary_offset), a valid summary, exit 0: the browse view
           opens under an overlay listing "1 malformed record skipped"
           and "1 unrecognised record types skipped"; Esc dismisses to
           the browse content; q exits 0.

  no-pair — the same stream minus the begin/end pairing: the orphaned
           match is retained but stream integrity fails, so the overlay
           also reports the incomplete stream and q exits 2.
"""
import fcntl
import os
import pty
import select
import struct
import sys
import tempfile
import termios
import time

HERE = os.path.dirname(os.path.abspath(__file__))
BIN = os.path.join(HERE, "vrg")

ROBUST_RG = """\
cat <<'EOF'
{"type":"begin","data":{"path":{"text":"./a.txt"}}}
{"type":"match","data":{"path":{"text":"./a.txt"},"lines":{"text":"alpha line 2\\n"},"line_number":2,"absolute_offset":0,"submatches":[{"match":{"text":"alpha"},"start":0,"end":5}]}}
this line is not json
{"type":"weird"}
{"type":"end","data":{"path":{"text":"./a.txt"},"binary_offset":null,"stats":{}}}
{"type":"summary","data":{"stats":{}}}
EOF
exit 0
"""

NOPAIR_RG = """\
cat <<'EOF'
{"type":"match","data":{"path":{"text":"./a.txt"},"lines":{"text":"alpha line 2\\n"},"line_number":2,"absolute_offset":0,"submatches":[{"match":{"text":"alpha"},"start":0,"end":5}]}}
this line is not json
{"type":"weird"}
{"type":"summary","data":{"stats":{}}}
EOF
exit 0
"""


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

    def __init__(self, cwd, args, rg_script):
        fakebin = tempfile.mkdtemp(prefix="fakebin-")
        with open(os.path.join(fakebin, "rg"), "w") as fh:
            fh.write("#!/bin/sh\n" + rg_script)
        os.chmod(os.path.join(fakebin, "rg"), 0o755)
        master, slave = pty.openpty()
        fcntl.ioctl(slave, termios.TIOCSWINSZ,
                    struct.pack("HHHH", 24, 80, 0, 0))
        env = dict(os.environ)
        env["TERM"] = "xterm"
        env["PATH"] = fakebin + ":" + env["PATH"]
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

    def wait_for(self, needle, label, off=0, t=30):
        deadline = time.time() + t
        while time.time() < deadline:
            pump(self.master, self.out)
            if needle in bytes(self.out[off:]):
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


def write_fixture(dirpath):
    os.makedirs(dirpath, exist_ok=True)
    with open(os.path.join(dirpath, "a.txt"), "w") as fh:
        for i in range(1, 21):
            fh.write("alpha line %d\n" % i)


def main():
    if not os.path.exists(BIN):
        fail("build the binary first: go build -o %s ./cmd/vrg" % BIN)

    # 1. The issue's manual stream: valid begin/match/end around a
    #    garbage line and an unknown-type line, then a summary, exit 0.
    #    Browse opens under an overlay listing both skip counts; Esc
    #    dismisses; q exits 0.
    dir1 = tempfile.mkdtemp(prefix="vrg-robust-")
    write_fixture(dir1)
    s = Session(dir1, ["foo"], ROBUST_RG)
    s.wait_for(b"1 malformed record skipped", "malformed diagnostic")
    s.wait_for(b"1 unrecognised record types skipped", "unknown-type diagnostic")
    off = len(s.out)
    s.send(b"\x1b")
    # The covered content row repaints only after the overlay closes.
    s.wait_for(b"alpha line 11", "dismissal repaint", off=off)
    s.send(b"q")
    code = s.wait_exit("robust quit")
    if code != 0:
        fail("robust: exit=%d, want 0" % code)
    if b"\x1b[?1049l" not in s.out:
        fail("robust: alt-screen restoration missing")
    print("robust       : garbage + weird-type lines between valid records "
          "-> browse + overlay listing both skip counts; Esc dismisses; "
          "q exits 0")

    # 2. The same stream without the paired begin/end: the match is
    #    orphaned — retained, but stream integrity fails, so the overlay
    #    adds the incomplete-stream note and q exits 2.
    dir2 = tempfile.mkdtemp(prefix="vrg-nopair-")
    write_fixture(dir2)
    s = Session(dir2, ["foo"], NOPAIR_RG)
    s.wait_for(b"malformed record skipped", "no-pair malformed diagnostic")
    s.wait_for(b"unrecognised record types skipped", "no-pair unknown diagnostic")
    s.wait_for(b"incomplete", "no-pair integrity diagnostic")
    off = len(s.out)
    s.send(b"\x1b")
    s.wait_for(b"alpha line 11", "no-pair dismissal repaint", off=off)
    s.send(b"q")
    code = s.wait_exit("no-pair quit")
    if code != 2:
        fail("no-pair: exit=%d, want 2" % code)
    print("no-pair      : same stream without begin/end -> orphaned match "
          "retained, integrity fails; q exits 2")
    print("OK")


if __name__ == "__main__":
    main()
