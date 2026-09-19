#!/usr/bin/env python3
"""Issue #26 walkthrough: read failures on a real pty.

One session on a real pty (80x24) against a disposable temporary
fixture directory prepared by manual_route.sh (argv[1]):

  <tmp>/aaa-readable.txt
      line 1 'needle a one' — one stop; the startup file
  <tmp>/bbb-blocked.txt
      'needle b one' (line 1) and 'needle b two' (line 3) — two stops;
      chmod 000 after the index is built so its loads fail
  <tmp>/gate — the VRG_TEST_LOAD_GATE file; while it exists every
      file-load worker holds before the read
  <tmp>/vrg-stderr.log — the child's stderr, so the Issue #11 replay
      is verified on real stderr, not the pty

The session drives the issue's manual check:

  startup   : file 1 shows its content — the search read both files
              while readable, so bbb's two stops are already indexed
  chmod 000 : bbb becomes unreadable — every later read fails; the
              immutable index keeps its stops
  n         : into file 2 — its load fails: the error overlay opens
              with 'cannot read …' and the panel shows '(unreadable)'
  Esc       : overlay dismissed; '(unreadable)' stays
  n         : same-file step to bbb's second stop — no retry, no new
              overlay
  p, p      : back through bbb's first stop onto file 1 — cached
              content, no loads
  gate + p  : re-enter file 2 from file 1 — the prior-failure overlay
              reopens immediately and the panel shows 'Loading…' while
              the one retry is minted and held at the gate
  gate gone : the retry fails — exactly one new 'cannot read' appends
              to the still-open overlay
  Esc       : dismissed — '(unreadable)' again
  q         : exit 0 — load failures never change the fixed status
  stderr    : the two failures replay to stderr after exit
"""
import fcntl
import os
import pty
import select
import stat
import struct
import sys
import termios
import time
import unicodedata

HERE = os.path.dirname(os.path.abspath(__file__))
BIN = os.path.join(HERE, "vrg")
TMP = sys.argv[1]
FILE_B = os.path.join(TMP, "bbb-blocked.txt")
GATE = os.path.join(TMP, "gate")
ERRLOG = os.path.join(TMP, "vrg-stderr.log")

COLS, ROWS = 80, 24


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


def cell_width(ch):
    if unicodedata.combining(ch):
        return 0
    return 2 if unicodedata.east_asian_width(ch) in ("W", "F") else 1


def emulate(data, rows, cols):
    """Replay the byte stream into a ROWSxCOLS text grid.

    Handles CUP/CUx cursor motion, EL/ED erases, IL/DL, and deferred
    autowrap — enough for vrg's frames.
    """
    grid = [[" "] * cols for _ in range(rows)]
    r = c = 0
    wrap = False  # deferred wrap: cursor sits past the last column
    top, bot = 0, rows - 1  # DECSTBM scroll region

    def ri():
        """ESC M — reverse index: scroll the region down at its top."""
        nonlocal r
        if r == top:
            grid.insert(top, [" "] * cols)
            del grid[bot + 1:]
        else:
            r = max(0, r - 1)

    def csi(r, c, params, final):
        priv = params.lstrip("?><=!")
        nums = [int(p) for p in priv.split(";") if p.isdigit()]

        def arg(k, dflt):
            v = nums[k] if k < len(nums) else 0
            return v if v else dflt

        if final in "Hf":
            r, c = arg(0, 1) - 1, arg(1, 1) - 1
        elif final == "A":
            r -= arg(0, 1)
        elif final == "B":
            r += arg(0, 1)
        elif final == "C":
            c += arg(0, 1)
        elif final == "D":
            c -= arg(0, 1)
        elif final == "E":
            r, c = r + arg(0, 1), 0
        elif final == "F":
            r, c = r - arg(0, 1), 0
        elif final == "G":
            c = arg(0, 1) - 1
        elif final == "d":
            r = arg(0, 1) - 1
        elif final == "e":
            r += arg(0, 1)
        elif final == "J":
            mode = arg(0, 0)
            if mode in (2, 3):
                for i in range(rows):
                    grid[i][:] = [" "] * cols
            elif mode == 0:
                grid[r][c:] = [" "] * (cols - c)
                for i in range(r + 1, rows):
                    grid[i][:] = [" "] * cols
            elif mode == 1:
                grid[r][:c + 1] = [" "] * (c + 1)
                for i in range(r):
                    grid[i][:] = [" "] * cols
        elif final == "K":
            mode = arg(0, 0)
            if mode == 0:
                grid[r][c:] = [" "] * (cols - c)
            elif mode == 1:
                grid[r][:c] = [" "] * (c + 1)
            elif mode == 2:
                grid[r][:] = [" "] * cols
        elif final == "L":  # insert blank lines
            for _ in range(arg(0, 1)):
                grid.insert(r, [" "] * cols)
                del grid[rows:]
        elif final == "M":  # delete lines
            for _ in range(arg(0, 1)):
                del grid[r]
                grid.append([" "] * cols)
        elif final == "P":  # delete chars
            n = arg(0, 1)
            del grid[r][c:c + n]
            grid[r].extend([" "] * n)
        elif final == "@":  # insert blank chars
            n = arg(0, 1)
            grid[r][c:c] = [" "] * n
            del grid[r][cols:]
        elif final == "r":  # DECSTBM: set the scroll region
            nonlocal top, bot
            top, bot = arg(0, 1) - 1, arg(1, rows) - 1
        return max(0, min(r, rows - 1)), max(0, min(c, cols - 1))

    i, n = 0, len(data)
    while i < n:
        b = data[i]
        if b == 0x1b:
            if i + 1 < n and data[i + 1] == ord("["):
                j = i + 2
                while j < n and not (0x40 <= data[j] <= 0x7e):
                    j += 1
                if j >= n:
                    break
                params = data[i + 2:j].decode("ascii", "replace")
                r, c = csi(r, c, params, chr(data[j]))
                wrap = False
                i = j + 1
            elif i + 1 < n and data[i + 1] == ord("]"):
                j = i + 2
                while j < n:
                    if data[j] == 7:
                        break
                    if data[j] == 0x1b and j + 1 < n and data[j + 1] == ord("\\"):
                        break
                    j += 1
                i = j + 1
                if i < n and data[i - 1] == 0x1b:
                    i += 1
            elif i + 1 < n and data[i + 1] in b"()#":
                i += 3  # charset/alignment sequences: ESC X Y
            elif i + 1 < n and data[i + 1] == ord("M"):
                ri()
                i += 2
            else:
                i += 2  # ESC + single byte (modes, DECSC, …)
            continue
        if b == 0x0d:
            c, wrap, i = 0, False, i + 1
            continue
        if b == 0x0a:
            r, wrap, i = min(r + 1, rows - 1), False, i + 1
            continue
        if b == 0x08:
            c, wrap, i = max(0, c - 1), False, i + 1
            continue
        if b == 0x09:
            c, wrap, i = min((c // 8 + 1) * 8, cols - 1), False, i + 1
            continue
        if b < 0x20 or b == 0x7f:
            i += 1
            continue
        # Decode one UTF-8 rune.
        if b < 0x80:
            ch, size = chr(b), 1
        else:
            size = 2 if b < 0xe0 else 3 if b < 0xf0 else 4
            ch = data[i:i + size].decode("utf-8", "replace")
        w = cell_width(ch)
        if wrap:
            r, c, wrap = min(r + 1, rows - 1), 0, False
        if w == 0:
            # A zero-width combining mark stacks on the previous
            # cell — the terminal's cell accounting, so differential
            # repaints keep landing on the right columns.
            if 0 <= r < rows and 0 < c <= cols:
                grid[r][c - 1] += ch
        else:
            if 0 <= r < rows and 0 <= c < cols:
                grid[r][c] = ch
                if w == 2 and c + 1 < cols:
                    grid[r][c + 1] = ""
            c += w
            if c >= cols:
                c, wrap = cols - 1, True
        i += size
    return ["".join(row) for row in grid]


class Session:
    """vrg running on its own pty; out accumulates the raw byte stream.

    The child runs with cwd and the fixture path as an absolute
    operand; stdout stays on the pty while stderr is redirected to
    ERRLOG so the post-exit diagnostic replay is checked on real
    stderr. VRG_TEST_LOAD_GATE points at the gate file: while it
    exists every file-load worker holds before the read.
    """

    def __init__(self, cols, rows, workdir, operand, pattern):
        self.cols, self.rows = cols, rows
        master, slave = pty.openpty()
        self.master = master
        fcntl.ioctl(slave, termios.TIOCSWINSZ,
                    struct.pack("HHHH", rows, cols, 0, 0))
        env = dict(os.environ)
        env["TERM"] = "xterm"
        env["VRG_TEST_LOAD_GATE"] = GATE
        errfd = os.open(ERRLOG, os.O_CREAT | os.O_WRONLY | os.O_TRUNC, 0o644)
        pid = os.fork()
        if pid == 0:
            os.setsid()
            fcntl.ioctl(slave, termios.TIOCSCTTY, 0)
            os.dup2(slave, 0)
            os.dup2(slave, 1)
            os.dup2(errfd, 2)
            os.close(master)
            if slave > 2:
                os.close(slave)
            if errfd > 2:
                os.close(errfd)
            os.chdir(workdir)
            os.execvpe(BIN, [BIN, pattern, operand], env)
            os._exit(127)
        os.close(slave)
        os.close(errfd)
        self.pid = pid
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

    def screen(self):
        return emulate(self.out, self.rows, self.cols)

    def settle(self, quiet=0.4, t=15):
        """Pump until the output is quiet, then return the screen."""
        deadline = time.time() + t
        last = -1
        while time.time() < deadline:
            pump(self.master, self.out, quiet)
            if len(self.out) == last:
                return self.screen()
            last = len(self.out)
        self.kill()
        fail("output never settled")

    def send(self, data):
        os.write(self.master, data)

    def send_settle(self, data, quiet=0.4):
        """Send a key and settle into the frame it produced."""
        self.send(data)
        return self.settle(quiet)

    def wait_exit(self, label, t=15):
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


def text(screen):
    return "\n".join(screen)


def disp(s):
    """Normalize the disposable directory so the transcript is stable."""
    return s.replace(TMP, "<tmp>")


def main():
    orig = stat.S_IMODE(os.stat(FILE_B).st_mode)
    chmodded = False
    try:
        s = Session(COLS, ROWS, TMP, TMP, "needle")
        s.wait_for(b"aaa-readable.txt", "initial frame")
        screen = s.settle()
        if "aaa-readable.txt" not in screen[0]:
            fail("filename rule = %r, want aaa-readable.txt" % screen[0])
        if "needle a one" not in text(screen):
            fail("file 1 content missing at startup: %r" % screen)
        print("%-10s: file 1 up — the search read both files while "
              "readable — %r" % ("startup", disp(screen[0].strip()[:60])))

        # The index is built — bbb's two stops are fixed. Now its file
        # becomes unreadable: every later load fails while the stops
        # stay.
        os.chmod(FILE_B, 0)
        chmodded = True

        # n into file 2: the load fails — the Issue #9 error overlay
        # opens with the diagnostic and the panel shows the
        # "(unreadable)" placeholder; the filename rule still names
        # the path.
        screen = s.send_settle(b"n", quiet=1.0)
        body = text(screen)
        if "cannot read" not in body or "bbb-blocked.txt" not in body:
            fail("no failure overlay after n into file 2: %r" % screen)
        if "(unreadable)" not in body:
            fail("no (unreadable) placeholder after n: %r" % screen)
        print("%-10s: file 2's load failed — overlay 'cannot read …' "
              "over '(unreadable)'" % "n")

        # Esc dismisses the overlay; the placeholder stays.
        screen = s.send_settle(b"\x1b", quiet=1.0)
        body = text(screen)
        if "cannot read" in body:
            fail("overlay still up after Esc: %r" % screen)
        if "(unreadable)" not in body:
            fail("(unreadable) gone after Esc: %r" % screen)
        print("%-10s: overlay dismissed — '(unreadable)' remains" % "Esc")

        # Same-file n: the cursor moves to bbb's second stop without
        # minting a retry — no new overlay, no reload.
        screen = s.send_settle(b"n", quiet=1.0)
        body = text(screen)
        if "cannot read" in body:
            fail("same-file n opened an overlay: %r" % screen)
        if "(unreadable)" not in body:
            fail("(unreadable) lost on same-file n: %r" % screen)
        print("%-10s: same-file step — no retry, no new overlay" % "n")

        # p back through bbb's first stop onto file 1 — the cached
        # buffer serves, no loads.
        screen = s.send_settle(b"p", quiet=0.8)
        screen = s.send_settle(b"p", quiet=1.0)
        if "aaa-readable.txt" not in screen[0]:
            fail("filename rule after p p = %r, want aaa-readable.txt"
                 % screen[0])
        if "needle a one" not in text(screen):
            fail("file 1 content missing after p p: %r" % screen)
        print("%-10s: back on file 1 — cached content, no loads" % "p p")

        # Re-entry into failed file 2: the prior-failure overlay opens
        # immediately while the panel switches to "Loading…" and
        # exactly one retry is minted — held at the load gate so the
        # intermediate state is observable.
        open(GATE, "w").close()
        screen = s.send_settle(b"p", quiet=1.0)
        body = text(screen)
        if "cannot read" not in body:
            fail("no prior-failure overlay on re-entry: %r" % screen)
        if "Loading…" not in body:
            fail("no Loading… on re-entry: %r" % screen)
        print("%-10s: re-entry — prior-failure overlay up, 'Loading…' "
              "under it, one retry held" % "p")

        # Release the gate: the retry fails — exactly one new
        # occurrence appends to the still-open overlay.
        os.remove(GATE)
        screen = s.settle(quiet=1.0)
        nreads = text(screen).count("cannot read")
        if nreads != 2:
            fail("overlay shows %d 'cannot read' occurrences, want 2: %r"
                 % (nreads, screen))
        print("%-10s: retry failed — one occurrence appended to the "
              "open overlay (2 shown)" % "release")

        # Dismiss, then quit: exit 0 — the fixed search status never
        # moved.
        screen = s.send_settle(b"\x1b", quiet=1.0)
        if "(unreadable)" not in text(screen):
            fail("(unreadable) missing after dismissal: %r" % screen)
        s.send(b"q")
        if s.wait_exit("quit") != 0:
            fail("quit: exit=%d, want 0" % s.code)
        print("%-10s: exit 0 — failures never touched the fixed status"
              % "q")

        # The two failures replayed to real stderr after exit.
        with open(ERRLOG) as f:
            err = f.read()
        lines = [l for l in err.splitlines() if "cannot read" in l]
        if len(lines) != 2 or not all("bbb-blocked.txt" in l for l in lines):
            fail("stderr = %r, want two bbb 'cannot read' lines" % err)
        for l in lines:
            print("stderr    : %s" % disp(l))
        print("OK")
    finally:
        # Defense in depth under the caller's shell trap: never leave
        # the disposable fixture unreadable.
        if chmodded:
            os.chmod(FILE_B, orig)


if __name__ == "__main__":
    main()
