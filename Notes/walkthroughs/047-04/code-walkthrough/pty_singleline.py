#!/usr/bin/env python3
"""Issue #47 walkthrough: read-failure diagnostics stay single-line.

Two sessions on a real pty against a disposable temporary fixture
directory prepared by manual_route.sh (argv[1]):

  <tmp>/loadgate — the VRG_TEST_LOAD_GATE file; while it exists every
      file-load worker holds before its real os.ReadFile
  <tmp>/vrg-stderr.log — the child's stderr, so the Issue #11 replay
      is verified on real stderr, not the pty

Session A (four hostile filenames, the initial-load and navigation
load sites): files a-esc<ESC>file.txt, b-inv<0xff>file.txt,
c-nl<LF>file.txt, d-tab<TAB>file.txt each carry one 'needle' match.
Every load is held at the gate, the fixture is removed, and the gate
released so the real read fails with ENOENT — never a permission
denial. Each failure must surface as exactly one diagnostic line in
the overlay; the exit replay must carry the same escaped lines
verbatim.

Session B (the r reload and re-entry retry sites): aaa.txt loads
cleanly; zz-nl<LF>file.txt fails its first gated load after removal;
re-entering it from aaa.txt mints the retry (fails identically), and
r on the failed file mints another load (fails identically) — three
identical single-line diagnostics collected, all replayed on exit.

Filenames are removed, never chmod-denied: a removal fails the read
deterministically even where root or ACLs would permit a chmod 000
read.
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
BIN = os.path.join(HERE, "vrg-testhooks")

COLS, ROWS = 160, 40


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
            elif i + 1 < n and data[i + 1] == ord("D"):
                # IND — forward index: like LF, scrolls at the margin.
                if r == bot:
                    del grid[top]
                    grid.insert(bot, [" "] * cols)
                else:
                    r = min(r + 1, rows - 1)
                i += 2
            else:
                i += 2  # ESC + single byte (modes, DECSC, …)
            continue
        if b == 0x0d:
            c, wrap, i = 0, False, i + 1
            continue
        if b == 0x0a:
            # LF at the bottom margin scrolls the region up one line;
            # inside it the cursor just moves down.
            if r == bot:
                del grid[top]
                grid.insert(bot, [" "] * cols)
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
    operand; stdout stays on the pty while stderr is redirected to
    ERRLOG so the post-exit diagnostic replay is checked on real
    stderr. VRG_TEST_LOAD_GATE points at the gate file: while it
    exists every file-load worker holds before the read.
    """

    def __init__(self, cols, rows, workdir, operand, pattern, gate, errlog):
        self.cols, self.rows = cols, rows
        master, slave = pty.openpty()
        self.master = master
        fcntl.ioctl(slave, termios.TIOCSWINSZ,
                    struct.pack("HHHH", rows, cols, 0, 0))
        env = dict(os.environ)
        env["TERM"] = "xterm"
        env["VRG_TEST_LOAD_GATE"] = gate
        errfd = os.open(errlog, os.O_CREAT | os.O_WRONLY | os.O_TRUNC, 0o644)
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

    def screen(self):
        return emulate(self.out, self.rows, self.cols)

    def wait_screen(self, needle, label, t=30):
        """Pump until the emulated screen shows needle."""
        deadline = time.time() + t
        while time.time() < deadline:
            pump(self.master, self.out, 0.1)
            if needle in text(self.screen()):
                return self.screen()
            if self.poll() is not None:
                break
        self.kill()
        fail("%s: %r never appeared on screen" % (label, needle))

    def settle(self, quiet=0.6, t=20):
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

    def send_settle(self, data, quiet=0.8):
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


def disp(s, tmp):
    """Normalize the disposable directory so the transcript is stable."""
    return s.replace(tmp, "<tmp>")


def escape_path(p):
    """The safepresentation.EscapePath contract, for expected text."""
    out = []
    i = 0
    while i < len(p):
        c = p[i]
        if c == 0x5c:
            out.append("\\\\")
        elif c == 0x0a:
            out.append("\\n")
        elif c == 0x0d:
            out.append("\\r")
        elif c == 0x09:
            out.append("\\t")
        elif c < 0x20:
            out.append("^" + chr(c + 0x40))
        elif c == 0x7f:
            out.append("^?")
        elif c < 0x80:
            out.append(chr(c))
        else:
            out.append("\\x%02x" % c)
        i += 1
    return "".join(out)


def gate_up(gate):
    open(gate, "w").close()


def gate_down(gate):
    os.remove(gate)


def expect_diag_row(screen, want, label):
    """want — the whole 'cannot read …' line — sits on ONE screen row."""
    for row in screen:
        if want in row:
            return
    fail("%s: diagnostic %r is not one screen row: %r" % (label, want, screen))


def read_diag_lines(errlog, label):
    """The collected diagnostics replayed to real stderr, one per line."""
    with open(errlog, "rb") as f:
        err = f.read()
    lines = err.decode("utf-8").splitlines()
    if not all(l.startswith("cannot read ") for l in lines):
        fail("%s: stderr replay = %r, want only 'cannot read' lines"
             % (label, lines))
    return lines


def session_a(tmp):
    """Four hostile names, the initial-load and navigation sites."""
    gate = os.path.join(tmp, "loadgate")
    errlog = os.path.join(tmp, "vrg-stderr.log")
    tdir = os.fsencode(tmp)
    names = [
        b"a-esc\x1bfile.txt",   # ESC byte
        b"b-inv\xfffile.txt",   # invalid UTF-8
        b"c-nl\nfile.txt",      # embedded newline
        b"d-tab\tfile.txt",     # embedded tab
    ]
    gate_up(gate)  # every load holds ahead of its real read
    s = Session(COLS, ROWS, tmp, tmp, "needle", gate, errlog)

    # File a's startup load is held at the gate: the index is built and
    # the panel reads Loading… while the read cannot yet have run.
    s.wait_screen("Loading…", "a startup load held")
    os.remove(os.path.join(tdir, names[0]))
    gate_down(gate)
    screen = s.settle()
    want = "cannot read %s: no such file or directory" % escape_path(
        os.path.join(tdir, names[0]))
    expect_diag_row(screen, want, "a initial load")
    print("A initial : %s" % disp(want, tmp))

    # Each later file fails the same way on navigation into it — the
    # overlay is dismissed first so each fresh failure is alone.
    for i in range(1, len(names)):
        s.send_settle(b"\x1b")  # dismiss the previous overlay
        gate_up(gate)
        s.send(b"n")
        s.wait_screen("Loading…", "file %d load held" % i)
        os.remove(os.path.join(tdir, names[i]))
        gate_down(gate)
        screen = s.settle()
        want_i = "cannot read %s: no such file or directory" % escape_path(
            os.path.join(tdir, names[i]))
        expect_diag_row(screen, want_i, "file %d navigation load" % i)
        nrows = text(screen).count("cannot read")
        if nrows != 1:
            fail("file %d: %d 'cannot read' rows, want exactly 1: %r"
                 % (i, nrows, screen))
        print("A nav[%d]  : %s" % (i, disp(want_i, tmp)))

    # The last failure's overlay is still open — q dismisses a modal
    # overlay, so Esc first, then q quits and replays to stderr.
    s.send_settle(b"\x1b")
    s.send(b"q")
    if s.wait_exit("A quit") != 0:
        fail("A quit: exit=%d, want 0" % s.code)
    lines = read_diag_lines(errlog, "A")
    want_all = ["cannot read %s: no such file or directory" % escape_path(
        os.path.join(tdir, n)) for n in names]
    if lines != want_all:
        fail("A replay = %r, want %r" % (lines, want_all))
    for l in lines:
        print("A stderr  : %s" % disp(l, tmp))
    print("A OK — 4 hostile names, 4 single-line diagnostics, "
          "identical in overlay and replay")


def session_b(tmp):
    """The r-reload and re-entry-retry sites on one newline filename."""
    gate = os.path.join(tmp, "loadgate")
    errlog = os.path.join(tmp, "vrg-stderr.log")
    hostile = os.path.join(os.fsencode(tmp), b"zz-nl\nfile.txt")
    want = "cannot read %s: no such file or directory" % escape_path(
        hostile)

    s = Session(COLS, ROWS, tmp, tmp, "needle", gate, errlog)
    s.wait_screen("needle aaa", "aaa loaded at startup")
    print("B startup : aaa.txt loaded — content visible")

    # First failure at the navigation site: gate the crossing load,
    # remove the fixture, release.
    gate_up(gate)
    s.send(b"n")
    s.wait_screen("Loading…", "zz first load held")
    os.remove(hostile)
    gate_down(gate)
    screen = s.settle()
    expect_diag_row(screen, want, "zz first load")
    print("B nav     : %s" % disp(want, tmp))

    s.send_settle(b"\x1b")  # dismiss; '(unreadable)' remains
    if "(unreadable)" not in text(s.screen()):
        fail("(unreadable) missing after Esc: %r" % s.screen())
    s.send_settle(b"p")  # back to cached aaa

    # Re-entry retry site: with the gate up, crossing back opens the
    # prior failure at once while the minted retry holds before its
    # real read — one diagnostic row, provably, until release.
    gate_up(gate)
    s.send(b"n")
    s.wait_screen("cannot read", "prior-failure overlay on re-entry")
    if text(s.screen()).count("cannot read") != 1:
        fail("re-entry: %d 'cannot read' rows, want the one prior "
             "failure: %r" % (text(s.screen()).count("cannot read"),
                              s.screen()))
    print("B re-entry: prior failure shown while the retry is held")
    gate_down(gate)  # fixture stays removed: the retry's read fails
    screen = s.settle()
    if text(screen).count("cannot read") != 2:
        fail("retry: %d 'cannot read' rows, want the identical line "
             "appended once: %r" % (text(screen).count("cannot read"), screen))
    expect_diag_row(screen, want, "zz retry")
    print("B retry   : identical single line appended (2 shown)")

    # The r reload site: on the failed current file r mints another
    # load through the same funnel — the overlay is still open, so the
    # identical line appends a third time. The fixture is still gone:
    # the real read fails without the gate.
    s.send(b"r")
    screen = s.settle()
    if text(screen).count("cannot read") != 3:
        fail("r reload: %d 'cannot read' rows, want 3 identical: %r"
             % (text(screen).count("cannot read"), screen))
    expect_diag_row(screen, want, "zz r reload")
    print("B reload  : r on the failed file — same single line (3 shown)")

    s.send_settle(b"\x1b")  # dismiss the overlay before q quits
    s.send(b"q")
    if s.wait_exit("B quit") != 0:
        fail("B quit: exit=%d, want 0" % s.code)
    lines = read_diag_lines(errlog, "B")
    if lines != [want, want, want]:
        fail("B replay = %r, want three identical single lines" % lines)
    for l in lines:
        print("B stderr  : %s" % disp(l, tmp))
    print("B OK — nav, re-entry retry, and r reload all emit the same "
          "one-line diagnostic")


def main():
    # Fixture files were created by manual_route.sh; this script only
    # removes them under the gate. Each session gets its own disposable
    # directory (argv[1], argv[2]) so neither session's index contains
    # the other's files.
    session_a(os.path.abspath(sys.argv[1]))
    session_b(os.path.abspath(sys.argv[2]))


if __name__ == "__main__":
    main()
