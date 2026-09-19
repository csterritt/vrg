#!/usr/bin/env python3
"""Issue #21 walkthrough: grapheme-cluster highlight expansion.

Two sessions on a real pty (100x24) over one fixture file:

  a.txt — line 1: "café today"   — é stored decomposed
                  (e + U+0301): a match on the combining-mark bytes
                  alone must highlight the whole é cluster
          line 2: "́lone mark"   — a standalone combining
                  mark at line start: a cluster with no base cell
                  renders the Issue #43 fallback ◌́ (one cell) and
                  the match highlights that visible cell
          line 3: "ab文cd"        — a two-cell CJK glyph
          line 4: "plain tail"    — no match, context

Session A searches the combining-mark bytes themselves (U+0301 as the
pattern, never precomposed é): rg matches the mark bytes inside the
decomposed é and inside the standalone mark — both spans expand to
their whole clusters. Session B searches 文: the match styles both
cells of the wide glyph as one inverse run.

Assertions run on the accumulated raw byte stream: the current
matched line styles runs as 30;47;4 (inverse + underline), other
lines as 30;47 (inverse). The styled run's text must contain the
whole grapheme — "e\xcc\x81" for the expanded é, the ◌́ fallback
bytes for the standalone mark, and the 文 bytes covering both cells.

The script drives:

  session A — vrg $'\u0301' f
      initial : é highlighted whole on the current line; the ◌́
                fallback cell highlighted on line 2
      n       : line 2 becomes current — its ◌́ gains the underline
      q       — exit 0
  session B — vrg 文 f
      initial : 文 highlighted whole — one styled run covering both
                cells of the glyph
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

MATCH = b"\x1b[30;47m"       # inverse — a match on a non-current line
CURMATCH = b"\x1b[30;47;4m"  # inverse + underline — the current line

E_ACUTE = "é".encode()       # decomposed é: e + U+0301
FALLBACK = "◌́".encode()      # Issue #43 fallback: U+25CC + U+0301
WEN = "文".encode()


def fail(msg):
    print("FAIL: " + msg)
    sys.exit(1)


def write_fixture():
    os.makedirs(FIXTURE, exist_ok=True)
    lines = [
        "café today",   # decomposed é — combining-only match
        "́lone mark",   # standalone combining mark at line start
        "ab文cd",               # two-cell CJK glyph
        "plain tail",
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

    The child runs with cwd=workdir and searches operand — the file
    list then displays the resolved path workdir/operand/name, so a
    short tmp cwd keeps the file panel usable at narrow widths.
    """

    def __init__(self, cols, rows, workdir, operand, name, pattern):
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
            os.execvpe(BIN, [BIN, pattern, operand], env)
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
        """Settle until the panel shows installed rows, not 'Loading…'."""
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


def styled(s, needle, label, style):
    """The styled run must contain the needle's whole glyph bytes."""
    if style + needle not in s.out:
        fail("%s: %r + %r never styled; output %r"
             % (label, style, needle, bytes(s.out)))


def panel(s, screen, r):
    return screen[r][s.panel:]


def main():
    # A short /tmp cwd with a symlink f → fixture keeps the file list
    # narrow: displayed paths are workdir/f/<name>.
    workdir = tempfile.mkdtemp(prefix="vrggraph")
    os.symlink(FIXTURE, os.path.join(workdir, "f"))
    write_fixture()

    # Session A — the search pattern is the combining-mark bytes
    # themselves (U+0301, UTF-8 cc 81), never precomposed é. rg matches
    # those bytes inside the decomposed é on line 1 and inside the
    # standalone mark on line 2; both matches expand to their whole
    # clusters before FileBuffer records them.
    s = Session(COLS, ROWS, workdir, "f", "a.txt", "́")
    s.wait_for(b"a.txt", "session A initial frame")
    screen = s.wait_loaded("session A initial load")
    styled(s, E_ACUTE, "é expansion", CURMATCH)
    styled(s, FALLBACK, "standalone fallback", MATCH)
    row1 = panel(s, screen, 1)
    row2 = panel(s, screen, 2)
    if not row1.startswith("1  café"):
        fail("row 1 = %r, want the decomposed é line" % row1)
    if not row2.startswith("2  ◌́"):
        fail("row 2 = %r, want the ◌ fallback cell at line start" % row2)
    print("%-9s: combining-only match — whole é in one styled run — %r"
          % ("setup", row1[:14]))
    print("%-9s: standalone mark — ◌́ fallback cell highlighted — %r"
          % ("setup", row2[:12]))

    # n to line 2: the fallback cell's match becomes the current
    # match — the inverse run gains the underline, still one cell.
    screen = s.send_settle(b"n")
    styled(s, FALLBACK, "current fallback", CURMATCH)
    print("%-9s: ◌́ now current — styled %r — %r"
          % ("n", "30;47;4", panel(s, screen, 2)[:12]))

    s.send(b"q")
    if s.wait_exit("session A quit") != 0:
        fail("session A quit: exit=%d, want 0" % s.code)
    print("%-9s: exit 0" % "q")

    # Session B — a CJK search: the match covers the whole two-cell
    # 文 cluster, painted as one inverse run over both cells.
    s = Session(COLS, ROWS, workdir, "f", "a.txt", "文")
    s.wait_for(b"a.txt", "session B initial frame")
    screen = s.wait_loaded("session B initial load")
    styled(s, WEN, "CJK match", CURMATCH)
    row3 = panel(s, screen, 3)
    if not row3.startswith("3  ab文cd"):
        fail("row 3 = %r, want the 文 line" % row3)
    print("%-9s: 文 match — both cells inside one styled run — %r"
          % ("setup", row3[:12]))

    s.send(b"q")
    if s.wait_exit("session B quit") != 0:
        fail("session B quit: exit=%d, want 0" % s.code)
    print("%-9s: exit 0" % "q")
    print("OK")


if __name__ == "__main__":
    main()
