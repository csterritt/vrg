#!/usr/bin/env python3
"""Issue #31 walkthrough: the help overlay on a real pty.

Two sessions on a real pty against the disposable fixture tree
manual_route.sh prepared (argv[1]):

  <tmp>/aa.txt — 'foo' lines (the browse fixture's first file)
  <tmp>/bb.txt — 'foo' lines (the second)

The issue's manual route:

  Session 1 (60x14, 'vrg foo .'):
    1.  '?' opens the bordered help box — 'Key bindings' inside the
        theme.Overlay single-line border
    2.  'down' x4 scrolls the complete wrapped table to its end —
        'ctrl+c' visible, the title row scrolled off
    3.  'n' is ignored — the frame behind help is untouched
    4.  Esc closes back to the browse view
    5.  'h' reopens, the pty shrinks to 25x8 — help clips to the
        terminal, still bordered, no borderless fallback
    6.  enlarging back to 60x14 restores the normal centred layout
    7.  Esc closes; q exits 0
  Session 2 (60x14, 'vrg <no-match> .'):
    8.  '?' opens help over 'No results found'; Esc returns to the
        no-results screen; q exits 1
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
ERRLOG = os.path.join(TMP, "vrg-stderr.log")

DOWN = b"\x1b[B"
ESC = b"\x1b"


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
            del grid[bot + 1]
        else:
            r = max(0, r - 1)

    def su(n=1):
        """Scroll the region up n lines — vrg repaints scrolled
        overlays by DECSTBM-ing the interior and emitting LF at the
        bottom margin."""
        for _ in range(n):
            del grid[top]
            grid.insert(bot, [" "] * cols)

    def csi(r, c, params, final):
        nonlocal top, bot
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
        elif final == "L":  # insert blank lines (region-bounded)
            if top <= r <= bot:
                for _ in range(arg(0, 1)):
                    grid.insert(r, [" "] * cols)
                    del grid[bot + 1]
        elif final == "M":  # delete lines (region-bounded)
            if top <= r <= bot:
                for _ in range(arg(0, 1)):
                    del grid[r]
                    grid.insert(bot, [" "] * cols)
        elif final == "X":  # ECH — erase n cells, cursor stays
            grid[r][c:c + arg(0, 1)] = [" "] * arg(0, 1)
        elif final == "S":  # SU — scroll the region up n lines
            su(arg(0, 1))
        elif final == "T":  # SD — scroll the region down n lines
            for _ in range(arg(0, 1)):
                grid.insert(top, [" "] * cols)
                del grid[bot + 1]
        elif final == "P":  # delete chars
            n = arg(0, 1)
            del grid[r][c:c + n]
            grid[r].extend([" "] * n)
        elif final == "@":  # insert blank chars
            n = arg(0, 1)
            grid[r][c:c] = [" "] * n
            del grid[r][cols:]
        elif final == "r":  # DECSTBM: set the scroll region
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
            elif i + 1 < n and data[i + 1] == ord("D"):
                # IND — index: LF's scroll-at-bottom-margin behaviour.
                if r == bot:
                    su()
                else:
                    r = min(r + 1, rows - 1)
                wrap = False
                i += 2
            else:
                i += 2  # ESC + single byte (modes, DECSC, …)
            continue
        if b == 0x0d:
            c, wrap, i = 0, False, i + 1
            continue
        if b == 0x0a:
            # LF at the bottom margin scrolls the DECSTBM region —
            # vrg's differential overlay repaint uses exactly that.
            if r == bot:
                su()
            else:
                r = min(r + 1, rows - 1)
            wrap, i = False, i + 1
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
    ERRLOG.
    """

    def __init__(self, cols, rows, workdir, operand, pattern):
        self.cols, self.rows = cols, rows
        master, slave = pty.openpty()
        self.master = master
        fcntl.ioctl(slave, termios.TIOCSWINSZ,
                    struct.pack("HHHH", rows, cols, 0, 0))
        env = dict(os.environ)
        env["TERM"] = "xterm"
        # Keep the child's stderr diagnostic-clean: a set-but-missing
        # ripgrep config file would emit a warning rg's stream carries
        # — a startup warning overlay is not part of this route.
        env.pop("RIPGREP_CONFIG_PATH", None)
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


def bordered(screen):
    """True when the frame carries the theme.Overlay single-line border."""
    body = text(screen)
    return "┌" in body and "│" in body


def case1():
    """Browse session: '?' opens bordered help, down scrolls the
    table, 'n' is ignored, Esc closes, 25x8 clips but keeps the box,
    growth restores the layout, q exits 0."""
    s = Session(60, 14, TMP, ".", "foo")
    try:
        s.wait_for(b"aa.txt", "browse")
        browse = s.settle()
        if "aa.txt" not in text(browse):
            fail("startup frame = %r, want the browse view" % browse)

        # '?' opens the bordered help box over the browse view.
        help0 = s.send_settle(b"?")
        if "Key bindings" not in text(help0) or not bordered(help0):
            fail("'?' frame = %r, want bordered help" % help0)
        if "n / p" not in text(help0):
            fail("'?' frame = %r, want the binding table" % help0)
        print("%-12s: bordered 'Key bindings' over the browse view"
              % "?")

        # down x4 scrolls the complete table to its last row.
        scrolled = help0
        for _ in range(4):
            scrolled = s.send_settle(DOWN)
        body = text(scrolled)
        if "ctrl+c" not in body:
            fail("scrolled frame = %r, want the last binding row"
                 % scrolled)
        if "Key bindings" in body:
            fail("scrolled frame = %r, the title must have scrolled off"
                 % scrolled)
        print("%-12s: table scrolled — 'ctrl+c' in, title out"
              % "down x4")

        # 'n' is ignored: the frame behind help is untouched.
        after_n = s.send_settle(b"n")
        if after_n != scrolled:
            fail("'n' while help is open changed the frame: %r"
                 % after_n)
        print("%-12s: ignored — the file behind help is unchanged"
              % "n")

        # Esc closes back to the browse view.
        closed = s.send_settle(ESC)
        if bordered(closed) or "Key bindings" in text(closed):
            fail("Esc frame = %r, help must be gone" % closed)
        if "aa.txt" not in text(closed):
            fail("Esc frame = %r, want the browse view back" % closed)
        print("%-12s: help closed — the browse view is back" % "Esc")

        # 'h' reopens, then shrink to 25x8: help clips to the terminal
        # — still bordered, no borderless fallback.
        s.send_settle(b"h")
        s.resize(25, 8)
        tiny = s.settle(0.8)
        if not bordered(tiny) or "Key bindings" not in text(tiny):
            fail("25x8 frame = %r, want clipped-but-bordered help"
                 % tiny)
        if s.poll() is not None:
            fail("vrg died at 25x8 (exit %d)" % s.code)
        print("%-12s: clipped to the terminal, still bordered"
              % "25x8")

        # Enlarging restores the normal centred layout.
        s.resize(60, 14)
        grown = s.settle(0.8)
        if "Key bindings" not in text(grown) or not bordered(grown):
            fail("restored frame = %r, want the normal help box"
                 % grown)
        print("%-12s: enlarged — the normal layout is back" % "60x14")

        closed = s.send_settle(ESC)
        if "aa.txt" not in text(closed) or bordered(closed):
            fail("Esc frame = %r, want the browse view back" % closed)
        print("%-12s: help closed — the browse view is back" % "Esc")

        s.send(b"q")
        if s.wait_exit("quit") != 0:
            fail("quit: exit=%d, want 0" % s.code)
        print("%-12s: exit 0" % "q")
    finally:
        s.kill()


def case2():
    """No-results session: '?' opens help over 'No results found',
    Esc returns to it, q exits 1."""
    s = Session(60, 14, TMP, ".", "zz-no-such-match-zz")
    try:
        s.wait_for(b"No results found", "no-results")
        screen = s.settle()
        if "No results found" not in text(screen):
            fail("startup frame = %r, want no-results" % screen)

        opened = s.send_settle(b"?")
        if "Key bindings" not in text(opened) or not bordered(opened):
            fail("'?' on no-results = %r, want bordered help" % opened)
        print("%-12s: bordered help over 'No results found'" % "?")

        closed = s.send_settle(ESC)
        if "No results found" not in text(closed) or bordered(closed):
            fail("Esc frame = %r, want no-results back" % closed)
        print("%-12s: help closed — 'No results found' is back"
              % "Esc")

        s.send(b"q")
        if s.wait_exit("quit") != 1:
            fail("quit: exit=%d, want 1" % s.code)
        print("%-12s: exit 1" % "q")
    finally:
        s.kill()


case1()
case2()
print("OK")
