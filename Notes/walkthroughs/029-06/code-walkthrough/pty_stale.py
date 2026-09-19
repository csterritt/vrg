#!/usr/bin/env python3
"""Issue #29 walkthrough: stale-match validation on a real pty.

Three sessions on real ptys against the disposable fixture tree
manual_route.sh prepared (argv[1]):

  <tmp>/c1/one.txt     — 'needle one 10' line 10 of 40
  <tmp>/c2/two.txt     — 'needle two 05' line 5, 'needle two 60' line
                         60 of 60
  <tmp>/c3/revert.txt  — same one-stop shape as c1
  <tmp>/vrg-stderr.log — each child's stderr

The issue's manual checks, one session each — the file edits happen on
disk while vrg holds its stale-until-r cached display:

  c1  same-length replacement 'needle' → 'sizzle' on disk, then r:
      the filename row gains 'file changed since search' and the
      edited line paints no highlight
  c2  the trailing matched line deleted on disk, then r and n: the
      stale stop lands at the last source line's start — no invented
      highlight — and the note shows
  c3  same-length replacement + r (note shows), then the file
      reverts + r: the note clears only on fully validating content
"""
import fcntl
import os
import pty
import re
import select
import struct
import sys
import termios
import time
import unicodedata

HERE = os.path.dirname(os.path.abspath(__file__))
BIN = os.path.join(HERE, "vrg")
TMP = sys.argv[1]
GATE = os.path.join(TMP, "gate.hold")
ERRLOG = os.path.join(TMP, "vrg-stderr.log")


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
                grid[r][c:] = [" "] * cols
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
    operand; stdout stays on the pty while stderr is appended to
    ERRLOG. VRG_TEST_LOAD_GATE=GATE is always in the environment —
    while GATE exists every file load holds.
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
        errfd = os.open(ERRLOG, os.O_CREAT | os.O_WRONLY | os.O_APPEND, 0o644)
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

    def resize(self, cols, rows):
        """Resize the pty — the child gets SIGWINCH and repaints."""
        self.cols, self.rows = cols, rows
        fcntl.ioctl(self.master, termios.TIOCSWINSZ,
                    struct.pack("HHHH", rows, cols, 0, 0))

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
    """Normalize the disposable directory so the transcript is stable —
    including the left-truncated path forms the file list shows, where
    the random mktemp suffix survives the truncation."""
    s = s.replace(TMP, "<tmp>")
    return re.sub(r"029-fixture\.[A-Za-z0-9]+", "029-fixture.XXXXXX", s)


def hold():
    """Create the gate file — every load holds until release()."""
    with open(GATE, "w"):
        pass


def release():
    os.remove(GATE)


def quit_ok(s, label):
    s.send(b"q")
    if s.wait_exit(label) != 0:
        fail("%s: exit=%d, want 0" % (label, s.code))




def case1():
    """Same-length replacement on disk, then r: the filename row gains
    'file changed since search' and the edited line paints no
    highlight."""
    f = os.path.join(TMP, "c1", "one.txt")
    s = Session(80, 24, os.path.join(TMP, "c1"), f, "needle")
    try:
        s.wait_for(b"one.txt", "c1 startup")
        screen = s.settle()
        # The clean load: the line-10 match is visible from the top —
        # current-line styled (inverse + underline).
        if "pad one 01" not in screen[1] or "needle one 10" not in screen[10]:
            fail("c1 startup frame = %r, want the file at the top "
                 "with 'needle one 10' in place" % screen[:11])
        if b"\x1b[30;47;4mneedle" not in s.out:
            fail("c1 clean frame paints no match highlight")
        print("%-12s: 'needle one 10' highlighted in place"
              % "c1 startup")

        # The disk edit: same-length 'needle' → 'sizzle', then r.
        snap = len(s.out)
        with open(f, "w") as fh:
            fh.write(open(f + ".orig").read().replace("needle", "sizzle"))
        screen = s.send_settle(b"r", quiet=1.0)
        if "file changed since search" not in screen[0]:
            fail("c1 filename row = %r, want the stale note"
                 % screen[0])
        if "sizzle one 10" not in screen[10]:
            fail("c1 edited row = %r, want 'sizzle one 10' in place"
                 % screen[10])
        for form in (b"\x1b[30;47;4msizzle", b"\x1b[30;47msizzle"):
            if form in s.out[snap:]:
                fail("c1 edited line paints an invented highlight")
        print("%-12s: same-length 'sizzle' — 'file changed since "
              "search', no highlight" % "c1 edit r")

        # The note has no timer: a scroll later it still rides the row.
        screen = s.send_settle(b"d", quiet=0.8)
        if "file changed since search" not in screen[0]:
            fail("c1 scrolled filename row = %r, the note must persist"
                 % screen[0])
        print("%-12s: scrolled — the note persists, no timer" % "c1 d")
        quit_ok(s, "c1 quit")
        print("%-12s: exit 0" % "c1 q")
    finally:
        s.kill()


def case2():
    """The trailing matched line deleted on disk, then r and n: the
    stale stop lands at the last source line's start with no invented
    highlight."""
    f = os.path.join(TMP, "c2", "two.txt")
    s = Session(80, 24, os.path.join(TMP, "c2"), f, "needle")
    try:
        s.wait_for(b"two.txt", "c2 startup")
        screen = s.settle()
        if "needle two 05" not in screen[5]:
            fail("c2 startup frame = %r, want 'needle two 05' in place"
                 % screen[5])

        # Delete the trailing matched lines (41–60) on disk, then r.
        snap = len(s.out)
        with open(f, "w") as fh:
            fh.write("".join(open(f + ".orig").readlines()[:40]))
        screen = s.send_settle(b"r", quiet=1.0)
        if "file changed since search" not in screen[0]:
            fail("c2 filename row = %r, want the stale note"
                 % screen[0])
        print("%-12s: trailing matched lines deleted + r — the note "
              "shows" % "c2 edit r")

        # n selects the stale stop: it lands at the last source line's
        # start — line 40, the reveal's one-third placement EOF-clamped
        # to the bottom row — with no invented highlight.
        screen = s.send_settle(b"n", quiet=1.0)
        if "pad two 18" not in screen[1]:
            fail("c2 top = %r, want 'pad two 18' — the EOF-clamped "
                 "reveal" % screen[1])
        if "pad two 40" not in screen[23]:
            fail("c2 landing = %r, want 'pad two 40' — the last source "
                 "line's start" % screen[23])
        for form in (b"\x1b[30;47;4mpad two 40", b"\x1b[30;47mpad two 40"):
            if form in s.out[snap:]:
                fail("c2 landing line paints an invented highlight")
        if "file changed since search" not in screen[0]:
            fail("c2 filename row = %r, the note must persist"
                 % screen[0])
        print("%-12s: n — lands at 'pad two 40', no invented highlight"
              % "c2 n")
        quit_ok(s, "c2 quit")
        print("%-12s: exit 0" % "c2 q")
    finally:
        s.kill()


def case3():
    """Same-length replacement + r (note shows), then the file reverts
    + r: the note clears only on fully validating content."""
    f = os.path.join(TMP, "c3", "revert.txt")
    s = Session(80, 24, os.path.join(TMP, "c3"), f, "needle")
    try:
        s.wait_for(b"revert.txt", "c3 startup")
        screen = s.settle()
        if "needle one 10" not in screen[10]:
            fail("c3 startup frame = %r, want 'needle one 10' in place"
                 % screen[10])

        snap = len(s.out)
        with open(f, "w") as fh:
            fh.write(open(f + ".orig").read().replace("needle", "sizzle"))
        screen = s.send_settle(b"r", quiet=1.0)
        if "file changed since search" not in screen[0]:
            fail("c3 stale filename row = %r, want the note" % screen[0])
        print("%-12s: 'sizzle' on disk + r — the note shows"
              % "c3 edit r")

        # Revert: the original content validates fully — the slot
        # empties and the match highlight returns.
        snap = len(s.out)
        with open(f, "w") as fh:
            fh.write(open(f + ".orig").read())
        screen = s.send_settle(b"r", quiet=1.0)
        if "file changed since search" in screen[0]:
            fail("c3 reverted filename row = %r, the note must clear"
                 % screen[0])
        if "needle one 10" not in screen[10]:
            fail("c3 reverted row = %r, want 'needle one 10' back"
                 % screen[10])
        if b"\x1b[30;47;4mneedle" not in s.out[snap:]:
            fail("c3 reverted frame lost the match highlight")
        print("%-12s: reverted + r — the note clears, the highlight "
              "returns" % "c3 revert r")
        quit_ok(s, "c3 quit")
        print("%-12s: exit 0" % "c3 q")
    finally:
        s.kill()


case1()
case2()
case3()
print("OK")
