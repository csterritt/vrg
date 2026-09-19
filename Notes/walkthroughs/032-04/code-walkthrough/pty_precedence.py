#!/usr/bin/env python3
"""Issue #32 walkthrough: overlay precedence and q/Esc semantics on a
real pty.

Four sessions on a real pty (60x14) against the disposable fixture
tree manual_route.sh prepared (argv[1]):

  <tmp>/aaa-readable.txt — 'needle a one' (line 1); the startup file
  <tmp>/bbb-blocked.txt  — 'needle b one' (line 1); chmod 000 after
      the index is built so its loads fail
  <tmp>/bin/rg           — fake ripgrep: exits 3 with no output
  <tmp>/gate             — the VRG_TEST_LOAD_GATE file; while it
      exists every file-load worker holds before the read

The issue's manual route:

  Session 1 ('vrg needle .' — real rg):
    Esc with no overlay open is a strict no-op; chmod 000 bbb, n into
    it opens the browse error overlay; the first q closes only the
    overlay (still running); the second q exits the fixed status 0.
  Session 2 ('vrg needle .' — fake rg exits 3, no output):
    the fatal overlay names 'ripgrep exited with code 3'; Esc exits 2
    because there is no underlying state.
  Session 3 (same fake rg): q exits 2.
  Session 4 ('vrg needle .' — real rg, VRG_TEST_LOAD_GATE set):
    n into bbb with the gate up mints a held load; '?' opens help,
    down x4 scrolls the table; releasing the gate lands the failure
    while help is open — the error suspends help; Esc restores help
    at its prior scroll position; Esc closes help; q exits 0.
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

    Handles CUP/CUx cursor motion, EL/ED erases, IL/DL, DECSTBM-region
    scrolling, ECH, and deferred autowrap — enough for vrg's frames.
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

    The child runs with the fixture path as its operand; stdout stays
    on the pty while stderr is appended to ERRLOG. extra carries the
    session's VRG_TEST_* seams and PATH overrides.
    """

    def __init__(self, cols, rows, workdir, operand, pattern, extra=None):
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
        if extra:
            env.update(extra)
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
    """Esc with no overlay is a no-op; n into the chmodded file opens
    the browse error overlay; q closes only it (still running); the
    second q exits the fixed status 0."""
    s = Session(60, 14, TMP, ".", "needle")
    try:
        s.wait_for(b"aaa-readable.txt", "browse")
        browse = s.settle()
        if "needle a one" not in text(browse):
            fail("startup frame = %r, want the browse view" % browse)

        # Esc with nothing open: the frame is untouched, still running.
        after_esc = s.send_settle(ESC)
        if after_esc != browse:
            fail("Esc with no overlay changed the frame: %r" % after_esc)
        if s.poll() is not None:
            fail("Esc with no overlay exited (exit %d)" % s.code)
        print("%-12s: no overlay — frame untouched, still running" % "Esc")

        # chmod 000 bbb, then n into it: the load fails — the error
        # overlay opens over browse.
        os.chmod(FILE_B, 0)
        screen = s.send_settle(b"n", quiet=1.0)
        body = text(screen)
        if "cannot read" not in body or "bbb-blocked.txt" not in body:
            fail("n into the unreadable file = %r, want the error overlay"
                 % screen)
        print("%-12s: browse error overlay 'cannot read …' is up" % "n")

        # The first q closes only the overlay — still running.
        screen = s.send_settle(b"q", quiet=1.0)
        body = text(screen)
        if "cannot read" in body or bordered(screen):
            fail("first q left the overlay up: %r" % screen)
        if "(unreadable)" not in body:
            fail("first q = %r, want the browse view behind" % screen)
        if s.poll() is not None:
            fail("first q exited (exit %d) — it must only dismiss" % s.code)
        print("%-12s: overlay closed — browse running behind" % "q")

        # The second q exits with the fixed status: the search
        # succeeded, so 0.
        s.send(b"q")
        if s.wait_exit("quit") != 0:
            fail("quit: exit=%d, want the fixed 0" % s.code)
        print("%-12s: exit 0 — the fixed search status" % "q")
    finally:
        s.kill()


def fatal_session(key, label):
    """A fake rg exiting 3 with no output: the fatal overlay names
    'ripgrep exited with code 3'; the dismissal key exits 2 — there is
    no underlying state to return to."""
    env = {"PATH": os.path.join(TMP, "bin") + ":" + os.environ["PATH"]}
    s = Session(60, 14, TMP, ".", "needle", extra=env)
    try:
        s.wait_for(b"ripgrep exited with code 3", "fatal overlay")
        screen = s.settle()
        body = text(screen)
        if "ripgrep exited with code 3" not in body or not bordered(screen):
            fail("startup = %r, want the fatal overlay" % screen)
        print("%-12s: fatal overlay — 'ripgrep exited with code 3'" % "rg -3")

        s.send(key)
        if s.wait_exit("exit") != 2:
            fail("%s: exit=%d, want 2" % (label, s.code))
        print("%-12s: exit 2 — no underlying state to return to" % label)
    finally:
        s.kill()


def case4():
    """The gated error while help is open: n into bbb with the load
    gate up mints a held load; '?' opens help, down x4 scrolls it;
    releasing the gate lands the failure over the suspended help; Esc
    restores help at its scroll position; Esc closes help; q exits 0.
    """
    # case1 left bbb unreadable; the search must read it to index its
    # stop — restore it for this session's rg run.
    os.chmod(FILE_B, 0o644)

    env = {"VRG_TEST_LOAD_GATE": GATE}
    s = Session(60, 14, TMP, ".", "needle", extra=env)
    try:
        s.wait_for(b"aaa-readable.txt", "browse")
        s.settle()

        # Gate up: n into bbb mints the load and holds it — the panel
        # shows 'Loading…' while the cursor already sits on bbb's stop.
        open(GATE, "w").close()
        s.send(b"n")
        s.wait_for("Loading…".encode(), "held load")
        loading = s.settle(quiet=1.0)
        if "bbb-blocked.txt" not in text(loading):
            fail("held-load frame = %r, want bbb current" % loading)
        print("%-12s: into bbb — 'Loading…', one load held at the gate" % "n")

        # The held read will fail: make bbb unreadable before release.
        os.chmod(FILE_B, 0)

        # '?' opens help over the held load; down x4 scrolls the table
        # — 'ctrl+c' in, the title scrolled off: the position the
        # error must preserve.
        opened = s.send_settle(b"?", quiet=1.0)
        if "Key bindings" not in text(opened) or not bordered(opened):
            fail("'?' frame = %r, want bordered help" % opened)
        scrolled = opened
        for _ in range(4):
            scrolled = s.send_settle(DOWN)
        body = text(scrolled)
        if "ctrl+c" not in body or "Key bindings" in body:
            fail("scrolled help = %r, want the table end" % scrolled)
        print("%-12s: help open and scrolled — 'ctrl+c' in, title out"
              % "? down x4")

        # Release the gate: the held read fails and the error overlay
        # lands over the suspended help.
        os.remove(GATE)
        s.wait_for(b"cannot read", "gated failure")
        stacked = s.settle(quiet=1.0)
        if "cannot read" not in text(stacked) or not bordered(stacked):
            fail("released frame = %r, want the error overlay" % stacked)
        print("%-12s: error landed over help — 'cannot read …' on top"
              % "release")

        # Esc dismisses only the error: help returns at its scrolled
        # position — 'ctrl+c' still in, the title still off.
        restored = s.send_settle(ESC, quiet=1.0)
        body = text(restored)
        if "cannot read" in body:
            fail("Esc left the error overlay up: %r" % restored)
        if "ctrl+c" not in body or "Key bindings" in body:
            fail("restored help = %r, want the scrolled position" % restored)
        print("%-12s: error closed — help restored at scroll" % "Esc")

        # Esc closes the restored help to browsing — still running.
        closed = s.send_settle(ESC, quiet=1.0)
        if bordered(closed) or "Key bindings" in text(closed):
            fail("second Esc = %r, want help closed" % closed)
        if "(unreadable)" not in text(closed):
            fail("second Esc = %r, want the browse view" % closed)
        if s.poll() is not None:
            fail("second Esc exited (exit %d)" % s.code)
        print("%-12s: help closed — '(unreadable)' browse behind" % "Esc")

        s.send(b"q")
        if s.wait_exit("quit") != 0:
            fail("quit: exit=%d, want the fixed 0" % s.code)
        print("%-12s: exit 0 — the fixed search status" % "q")
    finally:
        s.kill()


case1()
fatal_session(ESC, "Esc")
fatal_session(b"q", "q")
case4()
print("OK")
