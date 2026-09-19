#!/usr/bin/env python3
"""Issue #27 walkthrough: the explicit r reload on a real pty.

One session on a real pty (80x24) against a disposable temporary
fixture directory prepared by manual_route.sh (argv[1]):

  <tmp>/solo.txt
      line 1 'needle solo 01' — the single match, so the index has
      exactly one stop and n/p are strict no-ops: r is the only
      retry route this session has
  <tmp>/vrg-stderr.log — the child's stderr

The session drives the issue's manual check:

  startup     : the one-stop file shows its content — the whole
                session is also the single-match search check
  d d         : scroll — the saved viewport anchor moves mid-file
  append      : lines appended externally — the display does not
                change at all: cached content is stable until r
  r           : the explicit reload — the new content is there, and
                the top row is the same text: the anchor preserved
  pgdn …      : the appended tail is now reachable — new content
                under the preserved position
  rm + r      : the file deleted, r fails — '(unreadable)' plus the
                'cannot read …' overlay
  restore + r : pressed while the overlay is still open — the retry
                route the overlay cannot block — the content returns
  Esc + q     : dismiss, exit 0 — nothing touched the fixed status
"""
import fcntl
import os
import pty
import select
import struct
import sys
import termios
import time
import unicodedata

HERE = os.path.dirname(os.path.abspath(__file__))
BIN = os.path.join(HERE, "vrg")
TMP = sys.argv[1]
FILE = os.path.join(TMP, "solo.txt")
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
    """Normalize the disposable directory so the transcript is stable —
    including the left-truncated '…basename' form the file list shows."""
    return s.replace(TMP, "<tmp>").replace(os.path.basename(TMP), "<tmp>")


def top_line(screen):
    """The first content row's text — the visible top position."""
    return screen[1].strip()


def main():
    with open(FILE) as f:
        original = f.read()

    s = Session(COLS, ROWS, TMP, TMP, "needle")
    try:
        s.wait_for(b"solo.txt", "initial frame")
        screen = s.settle()
        if "solo.txt" not in screen[0]:
            fail("filename rule = %r, want solo.txt" % screen[0])
        if "needle solo 01" not in text(screen):
            fail("file content missing at startup: %r" % screen)
        print("%-11s: one-stop file up — the session is the "
              "single-match search — %r"
              % ("startup", disp(screen[0].strip()[:60])))

        # Scroll mid-file: the saved anchor leaves line 1.
        screen = s.send_settle(b"d", quiet=0.8)
        screen = s.send_settle(b"d", quiet=0.8)
        before_top = top_line(screen)
        if "pad solo" not in before_top:
            fail("scrolling left the top at %r, want a pad line"
                 % before_top)
        print("%-11s: top row %r — the anchor is mid-file"
              % ("d d", disp(before_top)))

        # Append lines externally: the display must not change at
        # all — cached content is stable until r. No repaint even
        # arrives.
        quiet_bytes = len(s.out)
        with open(FILE, "a") as f:
            f.write("".join("added solo %02d\n" % i for i in range(61, 91)))
        time.sleep(0.8)
        pump(s.master, s.out, 0.4)
        if len(s.out) != quiet_bytes:
            fail("the disk append repainted the frame without r")
        screen = s.screen()
        if top_line(screen) != before_top or "added solo" in text(screen):
            fail("the display changed without r: top %r" % top_line(screen))
        print("%-11s: disk append — no repaint, display unchanged "
              "(cache stable until r)" % "append")

        # r: the explicit reload — the same top row text proves the
        # anchor was preserved through the new revision.
        screen = s.send_settle(b"r", quiet=1.0)
        if top_line(screen) != before_top:
            fail("top after r = %r, want the preserved %r"
                 % (top_line(screen), before_top))
        if "Loading…" in text(screen):
            fail("the placeholder never settled: %r" % screen)
        print("%-11s: reload — top row still %r (anchor preserved)"
              % ("r", disp(top_line(screen))))

        # The appended tail is now part of the content — scroll to
        # the end to prove the new revision is real, then back.
        for _ in range(4):
            screen = s.send_settle(b"\x1b[6~", quiet=0.6)  # pgdn
        body = text(screen)
        if "added solo" not in body:
            fail("appended lines not in the reloaded content: %r" % screen)
        print("%-11s: appended tail visible — the new revision is live"
              % "pgdn…")

        # Delete the file, then r: the load fails — '(unreadable)'
        # replaces the content and the failure overlay opens.
        os.remove(FILE)
        screen = s.send_settle(b"r", quiet=1.0)
        body = text(screen)
        if "(unreadable)" not in body:
            fail("no (unreadable) after r on the deleted file: %r" % screen)
        if "cannot read" not in body or "solo.txt" not in body:
            fail("no failure overlay after r on the deleted file: %r" % screen)
        print("%-11s: deleted + r — 'cannot read …' over '(unreadable)'"
              % "rm r")

        # Restore the file, then r while the overlay is still open —
        # the retry route the overlay cannot block. The content
        # returns behind the still-open prior-failure overlay.
        with open(FILE, "w") as f:
            f.write(original)
        screen = s.send_settle(b"r", quiet=1.0)
        body = text(screen)
        if "cannot read" not in body:
            fail("the prior-failure overlay closed on its own: %r" % screen)
        if "pad solo" not in body:
            fail("content missing behind the overlay after restore + r: %r"
                 % screen)
        print("%-11s: restored + r under the open overlay — content "
              "returns, overlay stays" % "r")

        # Esc dismisses the overlay; the reloaded content stands.
        screen = s.send_settle(b"\x1b", quiet=1.0)
        if "pad solo" not in text(screen) or "cannot read" in text(screen):
            fail("frame after Esc = %r, want clean reloaded content" % screen)
        print("%-11s: overlay dismissed — reloaded content remains" % "Esc")

        # q exits 0 — reloads and failures never touched the fixed
        # search status.
        s.send(b"q")
        if s.wait_exit("quit") != 0:
            fail("quit: exit=%d, want 0" % s.code)
        print("%-11s: exit 0 — nothing moved the fixed status" % "q")

        # The delete-file failure replayed to real stderr once.
        with open(ERRLOG) as f:
            err = f.read()
        lines = [l for l in err.splitlines() if "cannot read" in l]
        if len(lines) != 1 or "solo.txt" not in lines[0]:
            fail("stderr = %r, want one solo 'cannot read' line" % err)
        print("stderr     : %s" % disp(lines[0]))
        print("OK")
    finally:
        if not os.path.exists(FILE):
            with open(FILE, "w") as f:
                f.write(original)


if __name__ == "__main__":
    main()
