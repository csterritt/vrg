#!/usr/bin/env python3
"""Issue #30 walkthrough: the unsupported-encoding placeholder on a real pty.

One session on a real pty against the disposable fixture tree
manual_route.sh prepared (argv[1]):

  <tmp>/u16.txt        — UTF-16 LE "hi\n" (FF FE h 00 i 00 \n 00)
  <tmp>/vrg-stderr.log — the child's stderr

The issue's manual route, one session — the file never changes; the
placeholder and the notification are what 'r' exercises:

  1.  vrg hi . opens on the single retained file: rg matched inside
      the transcoded stream, but the raw file's UTF-16 LE BOM makes
      the panel show "(unsupported encoding)" — no file text, no
      highlights — and the current-file detection opens the overlay
      with 'cannot display ./u16.txt: unsupported encoding UTF-16 LE'
  2.  Esc dismisses the overlay; the placeholder stays
  3.  r mints the reload: the load gate holds it so the panel reads
      "Loading…" mid-flight
  4.  releasing the gate lets the unchanged file settle back to
      "(unsupported encoding)" with a fresh overlay — detection is
      reported once per load
  5.  Esc dismisses again; q exits 0 — the fixed search-derived
      status — and stderr replays one encoding diagnostic per load
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


def disp(s):
    """Normalize the disposable directory so the transcript is stable —
    including the left-truncated path forms the file list shows, where
    the random mktemp suffix survives the truncation."""
    s = s.replace(TMP, "<tmp>")
    return re.sub(r"030-fixture\.[A-Za-z0-9]+", "030-fixture.XXXXXX", s)


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
    """The single UTF-16 LE file: placeholder + overlay on entry, Esc,
    r under the held gate shows "Loading…", release settles back to
    the placeholder with a fresh overlay, Esc, q exits 0 — and stderr
    replays one encoding diagnostic per load."""
    s = Session(80, 24, TMP, ".", "hi")
    try:
        s.wait_for(b"unsupported encoding", "startup")
        screen = s.settle()
        body = text(screen)
        if "(unsupported encoding)" not in body:
            fail("startup frame = %r, want the placeholder" % screen)
        # The overlay wraps its single diagnostic line at the interior
        # width — mid-word if needed — so the encoding name is checked
        # with borders and whitespace stripped out.
        flat = re.sub(r"[^0-9A-Za-z-]", "", body)
        if "cannotdisplay" not in flat or "UTF-16LE" not in flat:
            fail("startup overlay = %r, want the encoding diagnostic"
                 % screen)
        # No file text and no highlights: the panel shows only the
        # placeholder — the UTF-16 bytes never paint as ^@-littered
        # text or an inverse-video 'hi'.
        if "^@" in body or "h\x00i" in body:
            fail("startup frame = %r, want no file text" % screen)
        print("%-12s: '(unsupported encoding)' + the UTF-16 LE overlay"
              % "startup")

        screen = s.send_settle(b"\x1b", quiet=0.8)  # Esc dismisses
        flat = re.sub(r"[^0-9A-Za-z-]", "", text(screen))
        if "cannotdisplay" in flat:
            fail("Esc frame = %r, the overlay must be gone" % screen)
        if "(unsupported encoding)" not in text(screen):
            fail("Esc frame = %r, the placeholder must stay" % screen)
        print("%-12s: overlay dismissed — the placeholder stays"
              % "Esc")

        # r under the held gate: the reload pends and the panel reads
        # "Loading…" — the placeholder never presents mid-flight.
        hold()
        s.send(b"r")
        deadline = time.time() + 10
        loading = False
        while time.time() < deadline:
            pump(s.master, s.out, 0.2)
            if "Loading…" in text(s.screen()):
                loading = True
                break
        if not loading:
            s.kill()
            fail("r under the held gate never showed 'Loading…'")
        print("%-12s: held reload reads 'Loading…'" % "r (gated)")

        # Release: the unchanged file settles back to the placeholder
        # and a fresh overlay — detection is reported once per load.
        release()
        deadline = time.time() + 15
        while time.time() < deadline:
            pump(s.master, s.out, 0.2)
            flat = re.sub(r"[^0-9A-Za-z-]", "", text(s.screen()))
            if "UTF-16LE" in flat:
                break
        else:
            s.kill()
            fail("the reload never settled to the encoding overlay")
        screen = s.settle()
        body = text(screen)
        if "(unsupported encoding)" not in body:
            fail("reload frame = %r, want the placeholder again"
                 % screen)
        if "cannotdisplay" not in re.sub(r"[^0-9A-Za-z-]", "", body):
            fail("reload frame = %r, want the fresh overlay" % screen)
        print("%-12s: unchanged — placeholder + a new overlay"
              % "release")
        s.send_settle(b"\x1b", quiet=0.8)  # Esc dismisses
        print("%-12s: second overlay dismissed" % "Esc")
        quit_ok(s, "quit")
        print("%-12s: exit 0" % "q")

        # stderr replay: one encoding diagnostic per detection — the
        # startup load and the reload each collected one.
        log = open(ERRLOG).read()
        n = log.count("unsupported encoding UTF-16 LE")
        if n != 2:
            fail("stderr carries %d encoding diagnostics, want 2: %r"
                 % (n, log))
        if "cannot display" not in log or \
                "u16.txt: unsupported encoding UTF-16 LE" not in log:
            fail("stderr lacks the escaped-path diagnostic: %r" % log)
        print("%-12s: stderr replays both encoding diagnostics"
              % "stderr")
    finally:
        s.kill()


case1()
print("OK")
