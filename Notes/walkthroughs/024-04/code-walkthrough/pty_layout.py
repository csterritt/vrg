#!/usr/bin/env python3
"""Issue #24 walkthrough: file-list layout on a real pty.

One session on a real pty (starts 80x24, resizes mid-session), rooted
at a fixture directory under this walkthrough so displayed paths are
~100 cells — far over the 32-cell cap at 80 columns:

  fixture/deep-path-segment-that-is-quite-long-aaa/alpha.txt
      line 1 'needle alpha one' — the match
      line 2 a 500-cell line with 'MIDLINE-MARKER' at cell ~200
      pad lines to 32 lines total — a four-cell gutter
  fixture/deep-path-segment-that-is-quite-long-aaa/beta.txt
      same first two lines, padded to 12,001 lines — a seven-cell
      gutter (five-digit line numbers)

The session drives the issue's manual checks:

  80 cols  : the list sits at the floor(0.40*80)=32 cap, entries show
             a leading '…' with the basename tail visible
  scroll   : down into alpha's wrapped 500-cell line until
             MIDLINE-MARKER tops the panel — a mid-line anchor
  tab      : the list hides, the panel widens, and the same text
             stays at the top (anchor preserved through the rewrap)
  shift+tab: the list returns; the top row is byte-identical
  n        : crosses to beta (five-digit gutter). At 80 columns the
             40%% cap still binds — 'narrows if needed' — the
             recompute ran but the width holds at 32
  30 cols  : resize — the list is present but constrained to 12
  26 cols  : resize — on beta the third term binds (min(…, 10, 9)=9);
             p back to alpha widens to 10, n narrows to 9 again — the
             five-digit gutter driving the list width
  80 cols  : enlarge — the list widens back to the 32 cap
  q        : exit 0
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
import unicodedata

HERE = os.path.dirname(os.path.abspath(__file__))
BIN = os.path.join(HERE, "vrg")
FIXTURE = os.path.join(HERE, "fixture")
DEEP = "deep-path-segment-that-is-quite-long-aaa"

COLS, ROWS = 80, 24


def fail(msg):
    print("FAIL: " + msg)
    sys.exit(1)


def write_fixtures():
    d = os.path.join(FIXTURE, DEEP)
    os.makedirs(d, exist_ok=True)
    long_line = "x" * 200 + "MIDLINE-MARKER" + "y" * 286  # 500 cells
    alpha = ("needle alpha one\n" + long_line + "\n"
             + "".join("pad %d\n" % i for i in range(3, 33)))
    beta = ("needle beta one\n" + long_line + "\n"
            + "".join("pad %d\n" % i for i in range(3, 12002)))
    for name, data in [("alpha.txt", alpha), ("beta.txt", beta)]:
        with open(os.path.join(d, name), "w") as f:
            f.write(data)


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
                grid[r][:c + 1] = [" "] * (c + 1)
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
            n = arg(0, 0)
            del grid[r][c:c + n]
            grid[r].extend([" "] * n)
        elif final == "@":  # insert blank chars
            n = arg(0, 0)
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
            c, wrap, i = max(c - 1, 0), False, i + 1
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

    The child runs with cwd=workdir and the fixture path as an
    absolute operand, so displayed paths are ~100 cells and the list
    always truncates. resize() applies TIOCSWINSZ and raises SIGWINCH
    so the TUI sees a real size change.
    """

    def __init__(self, cols, rows, workdir, operand, pattern):
        self.cols, self.rows = cols, rows
        master, slave = pty.openpty()
        self.master = master
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
            os.chdir(workdir)
            os.execvpe(BIN, [BIN, pattern, operand], env)
            os._exit(127)
        os.close(slave)
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

    def wait_loaded(self, label, t=30):
        """Settle until the panel shows installed rows, not 'Loading…'."""
        deadline = time.time() + t
        while time.time() < deadline:
            screen = self.settle(quiet=0.4)
            if "Loading…" not in "".join(screen):
                return screen
        self.kill()
        fail("%s: panel still Loading… after %ds" % (label, t))

    def send(self, data):
        os.write(self.master, data)

    def send_settle(self, data, quiet=0.4):
        """Send a key and settle into the frame it produced."""
        self.send(data)
        return self.settle(quiet)

    def resize(self, cols):
        """Apply a real terminal resize: TIOCSWINSZ then SIGWINCH."""
        self.cols = cols
        fcntl.ioctl(self.master, termios.TIOCSWINSZ,
                    struct.pack("HHHH", self.rows, cols, 0, 0))
        os.kill(self.pid, signal.SIGWINCH)
        return self.settle()

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


def main():
    workdir = HERE
    write_fixtures()

    s = Session(COLS, ROWS, workdir, FIXTURE, "needle")
    s.wait_for(b"alpha.txt", "initial frame")
    screen = s.wait_loaded("initial load")

    # 80 columns: the list sits at the floor(0.40*80)=32 cap. Every
    # entry is a '…'-led tail ending in the basename; the panel
    # gutter begins at column 32.
    row1 = screen[1]
    if "…" not in row1[:32] or "alpha.txt" not in row1[:32]:
        fail("list row 1 = %r, want a '…'-led tail ending in alpha.txt" % row1[:32])
    if "…" not in screen[2][:32] or "beta.txt" not in screen[2][:32]:
        fail("list row 2 = %r, want a '…'-led tail ending in beta.txt"
             % screen[2][:32])
    if not row1[32:].startswith(" 1  needle alpha one"):
        fail("panel row 1 = %r, want it starting at column 32" % row1[32:])
    print("%-10s: 80 cols — list at the 32-cell cap, '…'-led basenames — "
          "%r | panel %r" % ("80x24", row1[:32].rstrip(), row1[32:50]))

    # Scroll partway into the wrapped 500-cell line: down five rows
    # puts the top on the continuation row holding MIDLINE-MARKER —
    # a mid-line anchor.
    for _ in range(5):
        s.send(b"\x1b[B")
    screen = s.settle()
    top0 = screen[1]
    if "MIDLINE-MARKER" not in top0[32:]:
        fail("scrolled top row = %r, want MIDLINE-MARKER at the top" % top0[32:])
    print("%-10s: scrolled into the wrapped line — top row %r"
          % ("down x5", top0[32:56]))

    # tab hides the list: the panel widens, the layout rewraps, and
    # the anchor keeps the same text at the top.
    screen = s.send_settle(b"\t")
    top1 = screen[1]
    if "MIDLINE-MARKER" not in top1:
        fail("post-tab top row = %r, want MIDLINE-MARKER still at the top" % top1)
    if "…" in top1[:20] or not top1.startswith("    "):
        fail("post-tab row = %r, want no list cells — panel from column 0" % top1)
    print("%-10s: list hidden, same text at top — %r" % ("tab", top1[:56]))

    # shift+tab restores the list: the same layout key returns, so the
    # whole top row is byte-identical to before the toggles.
    screen = s.send_settle(b"\x1b[Z")
    if screen[1] != top0:
        fail("post-shift+tab top row = %r, want the original %r" % (screen[1], top0))
    print("%-10s: list restored, top row byte-identical" % "shift+tab")

    # n crosses to beta.txt — a 12,001-line file, gutter 7. At 80
    # columns the 40%% cap still binds: the recompute ran (the panel
    # gutter widened) but the list holds at 32 — 'narrows if needed'.
    screen = s.send_settle(b"n", quiet=1.3)
    if "beta.txt" not in screen[0]:
        fail("filename rule after n = %r, want beta.txt" % screen[0])
    if not screen[1][32:].startswith("    1  "):
        fail("beta row 1 = %r, want the seven-cell gutter at column 32"
             % screen[1][32:])
    print("%-10s: on five-digit beta — list still 32 at 80 cols "
          "(cap binds); gutter now 7 — %r" % ("n", screen[1][32:48]))

    # Shrink to 30 columns: the list is constrained but present —
    # min(longest+2, 12, 13) = 12 with the seven-cell gutter.
    screen = s.resize(30)
    if "…" not in screen[1][:12]:
        fail("30-col list row 1 = %r, want a '…'-led entry in 12 cells"
             % screen[1][:12])
    if not screen[1][12:].startswith("    1  "):
        fail("30-col panel row 1 = %r, want the gutter at column 12"
             % screen[1][12:])
    print("%-10s: list constrained but present — 12 cells — %r | %r"
          % ("30 cols", screen[1][:12], screen[1][12:26]))

    # Shrink to 26: on beta the third term binds — min(…, 10, 9) = 9.
    screen = s.resize(26)
    if not screen[1][9:].startswith("    1  "):
        fail("26-col beta row 1 = %r, want the gutter at column 9" % screen[1][9:])
    print("%-10s: on beta the list narrows to 9 — the five-digit "
          "gutter's third term binds — %r" % ("26 cols", screen[1][:9]))

    # p returns to alpha (four-cell gutter): the list widens back to
    # the 40%% cap — min(…, 10, 12) = 10 — proving the load-time
    # gutter drives the width. n narrows it again to 9.
    screen = s.send_settle(b"p", quiet=1.3)
    if not screen[1][10:].startswith(" 1  "):
        fail("26-col alpha row 1 = %r, want the gutter at column 10"
             % screen[1][10:])
    print("%-10s: back on alpha the list is 10 — the smaller gutter "
          "widens it" % "p")
    screen = s.send_settle(b"n", quiet=1.3)
    if not screen[1][9:].startswith("    1  "):
        fail("26-col beta row 1 again = %r, want the gutter at column 9"
             % screen[1][9:])
    print("%-10s: five-digit beta narrows the list to 9 again" % "n")

    # Enlarge back to 80: the list widens to the 32 cap once more.
    screen = s.resize(80)
    if "…" not in screen[1][:32] or not screen[1][32:].startswith("    1  "):
        fail("80-col row 1 = %r, want the 32-cell list restored"
             % screen[1])
    print("%-10s: enlarged — list back at the 32-cell cap — %r"
          % ("80 cols", screen[1][:32].rstrip()))

    s.send(b"q")
    if s.wait_exit("quit") != 0:
        fail("quit: exit=%d, want 0" % s.code)
    print("%-10s: exit 0" % "q")
    print("OK")


if __name__ == "__main__":
    main()
