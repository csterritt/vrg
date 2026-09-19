#!/usr/bin/env python3
"""Issue #11 walkthrough: the manual case — a fake rg emits stderr
"warn one" beside a valid stream; the warning overlay displays it in
the TUI, and after q quits the shell shows "warn one" exactly once,
replayed to stderr strictly after the TUI's display restoration.

  replay — fake rg writes "warn one" to stderr, emits a valid stream,
           exits 0: the warning overlay shows "warn one" while
           browsing; q dismisses, q quits; the replayed diagnostic
           appears exactly once after the alt-screen exit sequence
           (\\x1b[?1049l), with exit 0.
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

WARN_RG = """\
printf 'warn one\\n' >&2
cat <<'EOF'
{"type":"begin","data":{"path":{"text":"./a.txt"}}}
{"type":"match","data":{"path":{"text":"./a.txt"},"lines":{"text":"alpha line 2\\n"},"line_number":2,"absolute_offset":0,"submatches":[{"match":{"text":"alpha"},"start":0,"end":5}]}}
{"type":"end","data":{"path":{"text":"./a.txt"},"binary_offset":null,"stats":{}}}
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


def main():
    if not os.path.exists(BIN):
        fail("build the binary first: go build -o %s ./cmd/vrg" % BIN)

    # The manual case: fake rg emits stderr "warn one" plus a valid
    # stream and exits 0. The warning overlay shows the diagnostic
    # while browsing; q dismisses, q quits; the collection replays
    # "warn one" to stderr exactly once — strictly after the TUI's
    # display restoration, so the shell shows it after vrg closes.
    workdir = tempfile.mkdtemp(prefix="vrg-replay-")
    with open(os.path.join(workdir, "a.txt"), "w") as fh:
        for i in range(1, 21):
            fh.write("alpha line %d\n" % i)
    s = Session(workdir, ["foo"], WARN_RG)
    s.wait_for(b"warn one", "warning overlay")
    s.send(b"q")  # dismiss the warning overlay
    s.send(b"q")  # quit the browse view
    code = s.wait_exit("replay quit")
    if code != 0:
        fail("replay: exit=%d, want 0" % code)
    out = bytes(s.out)
    i = out.find(b"\x1b[?1049l")
    if i < 0:
        fail("replay: alt-screen restoration missing")
    post = out[i:]
    if post.count(b"warn one") != 1:
        fail("replay: 'warn one' appears %d times after restoration, want exactly once"
             % post.count(b"warn one"))
    print("replay       : fake rg stderr 'warn one' + valid stream -> overlay shows "
          "it; q q exits 0; the shell sees 'warn one' exactly once after the TUI closes")
    print("OK")


if __name__ == "__main__":
    main()
