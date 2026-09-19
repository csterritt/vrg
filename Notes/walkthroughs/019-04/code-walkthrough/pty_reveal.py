#!/usr/bin/env python3
"""Issue #19 walkthrough: minimal horizontal reveal in run-off-edge mode.

One session on a real pty (100x24) over a single fixture file whose
four "needle" stops land at display cells 5, 300, 270, and 300:

  a.txt — line 1: "xxxxx" + "needle"          (start cell 5)
          line 2: "y" * 300 + "needle"        (start cell 300)
          line 3: "z" * 270 + "needle"        (start cell 270)
          line 4: "x" * 300 + "文needle"      (start cell 300; 文 is
                  one two-cell cluster — the match's first glyph)

The pattern is the regex alternation "needle|文needle" so the last
match begins on the wide glyph itself.

The script drives:

  w       — wrap → run-off-edge; the startup reveal commits against
            the flat layout with the cell-5 target already visible,
            so the offset stays 0
  n       — the cell-300 match is hidden right: off = 301 − textW,
            the minimum that paints the match's first cell at the
            right edge
  n       — the cell-270 match is already painted inside that window:
            the offset does not move
  n       — the 文-headed match at cell 300 is hidden right: off =
            302 − textW, so BOTH cells of the two-cell cluster paint
            at the right edge — never a clipping blank
  n       — wraps to the cell-5 match, hidden left: off = 5, the
            target column itself — the minimum leftward move
  q       — exit 0
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
import unicodedata

HERE = os.path.dirname(os.path.abspath(__file__))
BIN = os.path.join(HERE, "vrg")
FIXTURE = os.path.join(HERE, "fixture")

COLS, ROWS = 100, 24
GW = 3  # four lines: one digit column + two spaces


def fail(msg):
    print("FAIL: " + msg)
    sys.exit(1)


def write_fixture():
    os.makedirs(FIXTURE, exist_ok=True)
    lines = [
        "x" * 5 + "needle",
        "y" * 300 + "needle",
        "z" * 270 + "needle",
        "x" * 300 + "文needle",
    ]
    with open(os.path.join(FIXTURE, "a.txt"), "w") as f:
        f.write("\n".join(lines) + "\n")
    return lines


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
        if 0 <= r < rows and 0 <= c < cols:
            grid[r][c] = ch
            if w == 2 and c + 1 < cols:
                grid[r][c + 1] = ""
        c += max(w, 1)
        if c >= cols:
            c, wrap = cols - 1, True
        i += size
    return ["".join(row) for row in grid]


class Session:
    """vrg running on its own pty; out accumulates the raw byte stream.

    The child runs with cwd=workdir and searches operand — the file
    list then displays the resolved path workdir/operand/name, so a
    short tmp cwd keeps the file panel usable at narrow widths.
    """

    def __init__(self, cols, rows, workdir, operand, name):
        self.cols, self.rows = cols, rows
        self.panel = len(workdir + "/" + operand + "/" + name) + 1
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
            os.chdir(workdir)
            os.execvpe(BIN, [BIN, "needle|文needle", operand], env)
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
        """Settle until the panel shows installed rows, not 'Loading…'.

        A w toggle or file change repaints the placeholder first and
        the real rows once the off-update-path layout command
        completes; loop until the installed repaint lands.
        """
        deadline = time.time() + t
        while time.time() < deadline:
            screen = self.settle(quiet=0.4)
            if "Loading…" not in screen[1][self.panel:]:
                return screen
        self.kill()
        fail("%s: panel still Loading… after %ds" % (label, t))

    def send(self, data):
        os.write(self.master, data)

    def send_settle(self, data, quiet=0.4):
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


def panel(s, screen, r):
    return screen[r][s.panel:]


def cellslice(line, off, tw):
    """The line's display cells [off, off + tw), blank-padded.

    Cells come from display columns, not string indices — a two-cell
    文 consumes two slots of the window.
    """
    cells = []
    for ch in line:
        cells.append(ch)
        cells.extend([""] * (cell_width(ch) - 1))
    win = cells[off:off + tw]
    pad = tw - sum(1 if c == "" else max(cell_width(c), 1) for c in win)
    return "".join(win) + " " * pad


def main():
    lines = write_fixture()
    a1, a2, a3, a4 = lines

    # A short /tmp cwd with a symlink f → fixture keeps the file list
    # narrow: displayed paths are workdir/f/<name>.
    workdir = tempfile.mkdtemp(prefix="vrghrev")
    os.symlink(FIXTURE, os.path.join(workdir, "f"))

    s = Session(COLS, ROWS, workdir, "f", "a.txt")
    tw = COLS - s.panel - GW - 1  # flat text width (reserved col)
    if tw < 30:
        fail("file list too wide for the demo: panel=%d" % s.panel)

    def want(lineno, line, off):
        return "%d  " % lineno + cellslice(line, off, tw) + " "

    s.wait_for(b"a.txt", "initial frame")
    screen = s.wait_loaded("initial load")

    # w: run-off-edge. The startup reveal commits against the flat
    # layout: the cell-5 target is already painted at offset 0, so the
    # window does not move.
    s.send(b"w")
    screen = s.wait_loaded("w off")
    if panel(s, screen, 1) != want(1, a1, 0):
        fail("w: row 1 = %r, want %r" % (panel(s, screen, 1), want(1, a1, 0)))
    print("%-9s: run-off-edge, startup reveal — offset stays 0 — %r"
          % ("w", panel(s, screen, 1).rstrip()))

    # n to the cell-300 match: hidden right, so the offset moves to
    # 300 + 1 − tw — the minimum that paints the match's first cell as
    # the last text cell.
    off = 301 - tw
    screen = s.send_settle(b"n")
    if panel(s, screen, 2) != want(2, a2, off):
        fail("n: row 2 = %r, want the match start at the right edge %r"
             % (panel(s, screen, 2), want(2, a2, off)))
    print("%-9s: far match hidden right — off = %d — %r…%r"
          % ("n", off, panel(s, screen, 2)[:GW + 4], panel(s, screen, 2)[-10:]))

    # n to the cell-270 match: already painted inside the window, so
    # the offset does not move — minimal reveal is no movement.
    screen = s.send_settle(b"n")
    if panel(s, screen, 3) != want(3, a3, off):
        fail("n: row 3 = %r, want %r at the unchanged offset %d"
             % (panel(s, screen, 3), want(3, a3, off), off))
    print("%-9s: cell-270 match already painted — offset stays %d — %r…%r"
          % ("n", off, panel(s, screen, 3)[:GW + 4], panel(s, screen, 3)[-40:-26]))

    # n to the 文-headed match at cell 300: hidden right, and the
    # target cluster is two cells — off = 300 + 2 − tw paints BOTH
    # cells of 文 at the right edge, never a clipping blank.
    off = 302 - tw
    screen = s.send_settle(b"n")
    if panel(s, screen, 4) != want(4, a4, off):
        fail("n: row 4 = %r, want both cells of 文 at the right edge %r"
             % (panel(s, screen, 4), want(4, a4, off)))
    print("%-9s: CJK match — off = %d paints 文 whole — %r…%r"
          % ("n", off, panel(s, screen, 4)[:GW + 4], panel(s, screen, 4)[-10:]))

    # n wraps to the cell-5 match: hidden left, so the offset moves to
    # the target column itself — the minimum leftward move.
    screen = s.send_settle(b"n")
    if panel(s, screen, 1) != want(1, a1, 5):
        fail("n wrap: row 1 = %r, want the left-edge reveal %r"
             % (panel(s, screen, 1), want(1, a1, 5)))
    print("%-9s: wrapped to cell 5 — off = 5, the target column — %r"
          % ("n", panel(s, screen, 1).rstrip()))

    s.send(b"q")
    if s.wait_exit("quit") != 0:
        fail("quit: exit=%d, want 0" % s.code)
    print("%-9s: exit 0" % "q")
    print("OK")


if __name__ == "__main__":
    main()
