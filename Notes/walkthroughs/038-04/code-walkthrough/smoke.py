#!/usr/bin/env python3
"""Issue #38 smoke harness: panel-derived text width under a PTY.

Drives the built vrg binary on a real PTY (80x24) through the issue's
manual scenario: a file whose matched lines are longer than the text
width, browsed with the file list visible. The fixture's displayed
paths are absolute (~100 cells), so the list sits at the
floor(0.40*80) = 32 cap and the run-off-edge layout is:

    [list 32][separator 1][gutter 3][text 43][reserved 1] = 80

  w        — wrap -> run-off-edge; the far matches are hidden right,
             the reserved '*' draws at cell 79, the separator cell at
             column 32 stays blank.
  > x5     — pan offset 50: the window slides inside the text area;
             the match at cells 5-10 is entirely hidden left so the
             gutter marks '*'; content never bleeds under the list.
  n        — the cursor's next match (cell 250) reveals right-edge:
             off = 250 + 1 - 43 = 208, landing cell 250 on the last
             text column 78 — not the terminal width's column 79.
  left     — the list hides; the layout re-keys at the wider panel
             (text 75) while the separator column survives as a dead
             blank at column 0; the revealed match re-lands at
             column 46 and the indicators re-measure.
  right    — the list returns; the layout re-keys back to text 43.
  q        — exit 0 with the terminal restored.

Every key is sent only after the preceding expected frame is observed
in the emulated screen; each wait is a bounded poll on an explicit
rendered condition - never a fixed settling delay. assert_no_fixed_
delays() proves the first half mechanically by parsing this file's
AST; the bounded-poll structure makes the second half reviewable.
"""

import ast
import fcntl
import os
import pty
import select
import shutil
import signal
import struct
import subprocess
import sys
import termios
import time
import unicodedata

HERE = os.path.dirname(os.path.abspath(__file__))
VRG = os.path.join(HERE, "vrg")
FIXTURE = os.path.join(HERE, "fixture")
DEEP = "deep-path-segment-that-is-quite-long-aaa"

COLS, ROWS = 80, 24
# Run-off-edge geometry at 80 columns with the list at the 32 cap:
# list [0,32), separator 32, gutter [33,36), text [36,79), indicator 79.
SEP, TXT0, TXTW, IND = 32, 36, 43, 79
# Hidden list: separator 0, gutter [1,4), text [4,79), indicator 79.
H_TXT0, H_TXTW = 4, 75

# POLL_IDLE paces one iteration of a bounded condition poll. In the
# waits it is a select idle on the real PTY descriptor (genuine I/O
# waiting); it never stands in as an assumed settling interval.
POLL_IDLE = 0.05

failures = []


class SmokeError(Exception):
    """A bounded wait failed: the named condition never held."""


def log(msg):
    print(msg, flush=True)


def assert_no_fixed_delays():
    """Static self-check: parse this file's AST and fail if it calls
    time.sleep - the fixed-settling primitive the harness contract
    bans. Bounded polls pace themselves with select idles that
    re-check an explicit condition every iteration; nothing else in
    this file waits on elapsed time."""
    with open(__file__) as f:
        tree = ast.parse(f.read())
    for node in ast.walk(tree):
        if (isinstance(node, ast.Call)
                and isinstance(node.func, ast.Attribute)
                and node.func.attr == "sleep"
                and isinstance(node.func.value, ast.Name)
                and node.func.value.id == "time"):
            raise SmokeError(
                "time.sleep call found in %s - use a bounded "
                "poll on an explicit condition" % __file__)


def check(name, cond, detail=""):
    status = "PASS" if cond else "FAIL"
    if cond:
        log("  [%s] %s" % (status, name))
    else:
        log("  [%s] %s%s" % (status, name, ": " + detail if detail else ""))
        failures.append(name)


def cell_width(ch):
    if unicodedata.combining(ch):
        return 0
    return 2 if unicodedata.east_asian_width(ch) in ("W", "F") else 1


def emulate(data, rows, cols):
    """Replay the byte stream into a ROWSxCOLS text grid.

    Handles CUP/CUx cursor motion, EL/ED erases, IL/DL, and deferred
    autowrap - enough for vrg's frames. (Shared with the Issue #24
    walkthrough harness.)
    """
    grid = [[" "] * cols for _ in range(rows)]
    r = c = 0
    wrap = False  # deferred wrap: cursor sits past the last column
    top, bot = 0, rows - 1  # DECSTBM scroll region

    def ri():
        """ESC M - reverse index: scroll the region down at its top."""
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
                i += 2  # ESC + single byte (modes, DECSC, ...)
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
            # A zero-width combining mark stacks on the previous cell.
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


class PtyRun:
    """One vrg run under a PTY.

    wait_screen polls the emulated grid for an explicit rendered
    predicate, wait_exit polls process completion, and drain reads the
    PTY until EOF - every synchronization point is a bounded poll on
    an externally observable condition.
    """

    def __init__(self, cmd, cwd=None, winsize=(ROWS, COLS), timeout=60.0):
        self.cols, self.rows = winsize[1], winsize[0]
        self.timeout = timeout
        env = dict(os.environ)
        env["TERM"] = "xterm"

        pid, fd = pty.fork()
        if pid == 0:
            if cwd:
                os.chdir(cwd)
            os.execvpe(cmd[0], cmd, env)
            os._exit(127)

        try:
            wins = struct.pack("HHHH", winsize[0], winsize[1], 0, 0)
            fcntl.ioctl(fd, termios.TIOCSWINSZ, wins)
        except OSError:
            pass

        self.pid = pid
        self.fd = fd
        # The baseline is a fresh PTY pair's slave settings: the child
        # may already have set raw mode by the time the parent reads
        # the shared line discipline, so the run's own fd cannot time
        # the "before" reliably. vrg restores those defaults on exit.
        m, s = pty.openpty()
        try:
            self.before_termios = termios.tcgetattr(s)
        finally:
            os.close(m)
            os.close(s)
        self.after_termios = None
        self.raw = bytearray()
        self.eof = False
        self.exited = False
        self.exit_code = None

    def _pump(self, idle):
        """One I/O-multiplexed read step: wait up to idle for the PTY
        to become readable, then drain what arrived. EOF is observed
        when the slave side closes (PTY masters report EIO)."""
        if self.eof:
            return
        r, _, _ = select.select([self.fd], [], [], idle)
        for f in r:
            try:
                data = os.read(f, 65536)
            except OSError:
                data = b""
            if data:
                self.raw += data
            else:
                self.eof = True

    def _wait(self, desc, cond, timeout=None):
        """Bounded condition poll over pumped PTY state."""
        timeout = self.timeout if timeout is None else timeout
        deadline = time.monotonic() + timeout
        while True:
            if cond():
                return
            remaining = deadline - time.monotonic()
            if remaining <= 0:
                raise SmokeError("timed out after %.1fs waiting for %s"
                                 % (timeout, desc))
            self._pump(min(POLL_IDLE, remaining))

    def screen(self):
        return emulate(bytes(self.raw), self.rows, self.cols)

    def wait_screen(self, desc, pred, timeout=None):
        """Wait until the emulated screen satisfies pred - the
        externally observable post-state a following key relies on.
        Returns the satisfying screen."""
        result = []

        def probe():
            scr = self.screen()
            if pred(scr):
                result.append(scr)
                return True
            return False

        self._wait(desc, probe, timeout)
        return result[0]

    def send(self, key):
        """Write one key to the PTY. Synchronization is the caller's:
        a key is only ever sent after wait_screen observed the UI
        state that key depends on."""
        os.write(self.fd, key)

    def wait_exit(self, timeout=None):
        """Wait for process completion (bounded)."""
        self._wait("process exit", self._reap, timeout)

    def _reap(self):
        if self.exited:
            return True
        try:
            wpid, status = os.waitpid(self.pid, os.WNOHANG)
        except ChildProcessError:
            self.exited = True
            return True
        if wpid != self.pid:
            return False
        self.exited = True
        if os.WIFEXITED(status):
            self.exit_code = os.WEXITSTATUS(status)
        elif os.WIFSIGNALED(status):
            self.exit_code = 128 + os.WTERMSIG(status)
        return True

    def drain(self, timeout=None):
        """Completion/EOF-driven draining: read the PTY master until
        EOF (bounded)."""
        self._wait("PTY EOF", lambda: self.eof, timeout)

    def finish(self):
        """Snapshot post-exit termios and close the PTY (idempotent)."""
        if self.fd is None:
            return
        fd, self.fd = self.fd, None
        try:
            self.after_termios = termios.tcgetattr(fd)
        except (OSError, termios.error):
            pass
        try:
            os.close(fd)
        except OSError:
            pass

    def kill(self):
        """Failure path for a run that never completes."""
        try:
            os.kill(self.pid, signal.SIGKILL)
        except OSError:
            pass
        try:
            os.waitpid(self.pid, 0)
        except ChildProcessError:
            pass
        self.finish()


def check_terminal_restored(name, run):
    """Terminal restoration evidence: the display sequences (cursor
    visible; alt-screen left when it was entered) and the PTY input
    modes identical to the pre-launch capture."""
    raw = run.raw.decode("utf-8", "replace")
    check("%s cursor restored" % name, "\x1b[?25h" in raw)
    if "\x1b[?1049h" in raw:
        check("%s alt screen exited" % name, "\x1b[?1049l" in raw)
    check("%s termios restored" % name,
          run.before_termios == run.after_termios)


def write_fixtures():
    """alpha-file.txt: line 1 is 305 cells with 'needle' at cells
    5-10 and 295-300; line 2 is 300 cells with 'needle' at cells
    250-255 and 290-295; line 3 is a short 'cc'. A navigation stop is
    a matched line, so the cursor sits on line 1 first - its cell-5
    match visible at offset 0 while the cell-295 match feeds the
    reserved '*' - and 'n' lands on line 2 whose first submatch at
    cell 250 needs a right-edge reveal while its cell-290 match keeps
    the '*' lit. Line 2's longer text also keeps a hidden-left '_'
    once the window slides."""
    d = os.path.join(FIXTURE, DEEP)
    if os.path.exists(d):
        shutil.rmtree(d, ignore_errors=True)
    os.makedirs(d, exist_ok=True)
    line1 = ("x" * 5 + "needle" + "x" * 284 + "needle" + "x" * 4)
    line2 = ("x" * 250 + "needle" + "x" * 34 + "needle" + "x" * 4)
    alpha = line1 + "\n" + line2 + "\ncc\n"
    beta = "needle beta\n" + "x" * 200 + "\n"
    for name, data in [("alpha-file.txt", alpha),
                       ("beta-file.txt", beta)]:
        with open(os.path.join(d, name), "w") as f:
            f.write(data)


def scenario_panel_width():
    """The issue's manual scenario on one 80x24 PTY session."""
    write_fixtures()
    run = PtyRun([VRG, "needle", FIXTURE], cwd=HERE)
    try:
        # Wrap mode at startup: line 1's cells 0-43 paint on row 1, so
        # the cell-5 needle lands at column 41; the truncated list
        # entry ends in the basename.
        scr = run.wait_screen(
            "initial wrap frame",
            lambda s: s[1][41:47] == "needle")
        check("list entry truncated to basename",
              scr[1][:SEP].rstrip().endswith("alpha-file.txt"))
        check("separator cell blank at column %d" % SEP,
              scr[1][SEP] == " ")
        check("wrap mode paints cell 43 at the right edge",
              scr[1][IND] == "x")

        # w -> run-off-edge. Flat line 2 lands on row 2 (the wrapped
        # continuation rows collapse); off = 0 keeps the cell-5 match
        # visible while line 1's cell-295 match hides right -> the
        # reserved '*' draws at the panel's right edge (cell 79).
        run.send(b"w")
        scr = run.wait_screen(
            "run-off-edge frame",
            lambda s: s[2][33] == "2" and s[1][IND] == "*")
        check("separator stays blank in run-off-edge",
              scr[1][SEP] == " ")
        check("cell-5 match visible at offset 0",
              scr[1][41:47] == "needle")
        check("reserved '*' at the panel right edge",
              scr[1][IND] == "*")
        check("no left mark before any panning",
              scr[1][34] != "*")
        check("reserved cell blank on the non-current row",
              scr[2][IND] == " ")

        # > x5 pans the window to offset 50: line 1's cell-5 match is
        # entirely hidden left (gutter '*'), line 2 has text hidden
        # left but no entirely hidden match ('_'), the text area is
        # all content cells, and nothing bleeds under the list.
        run.send(b">>>>>")
        scr = run.wait_screen(
            "pan to offset 50",
            lambda s: s[1][TXT0:TXT0 + TXTW] == "x" * TXTW)
        check("left gutter '*' for the entirely hidden match",
              scr[1][34] == "*")
        check("right '*' keeps the far match signposted",
              scr[1][IND] == "*")
        check("hidden-left text marks '_' on the next line",
              scr[2][34] == "_")
        check("separator and list untouched by the pan",
              scr[1][SEP] == " "
              and scr[1][:SEP].rstrip().endswith("alpha-file.txt"))
        check("short-line row is all padding inside the panel",
              scr[3][SEP] == " "
              and scr[3][TXT0:IND] == " " * (IND - TXT0))

        # n -> the cursor lands on line 2 (a navigation stop is a
        # matched line) whose first submatch at cell 250 is hidden
        # right; the minimal reveal lands its start cell on the last
        # text column: off = 250 + 1 - 43 = 208, so cell 250 paints
        # at column 78 - inside the text area, never under the list
        # or over the reserved column. Line 2's second submatch at
        # cell 290 stays entirely hidden right -> '*'.
        run.send(b"n")
        scr = run.wait_screen(
            "reveal at the panel text width",
            lambda s: s[2][78] == "n")
        check("revealed match lands on the last text column",
              scr[2][78] == "n")
        check("text area left of the match is all content",
              scr[2][TXT0:78] == "x" * (78 - TXT0))
        check("right '*' holds for the still-hidden match",
              scr[2][IND] == "*")
        check("prior line keeps its hidden-left '*'",
              scr[1][34] == "*")

        # left hides the list: the layout re-keys at the wider panel
        # (text 75). The separator survives as a dead blank at column
        # 0; the offset-208 window re-lands cell 250 at column 46 and
        # the indicators re-measure against the new width.
        run.send(b"\x1b[D")
        scr = run.wait_screen(
            "list hidden at the wider panel",
            lambda s: s[1][0] == " " and s[2][46] == "n")
        check("separator survives as a blank column while hidden",
              scr[1][0] == " ")
        check("revealed match re-measured into the wider window",
              scr[2][46] == "n")
        check("wider window paints content through column 78",
              scr[2][78] == "x")
        check("indicators re-measure against the wider panel",
              scr[2][2] == "_" and scr[2][IND] == "*")
        check("no list cells bleed into the panel",
              "alpha-file.txt" not in scr[1][:SEP]
              and "alpha-file.txt" not in scr[2][:SEP])

        # right restores the list: the layout re-keys back to text 43
        # and the reveal's offset re-lands cell 250 at column 78.
        run.send(b"\x1b[C")
        scr = run.wait_screen(
            "list restored at text width 43",
            lambda s: s[1][:SEP].rstrip().endswith("alpha-file.txt")
            and s[2][78] == "n")
        check("separator back between list and panel",
              scr[1][SEP] == " ")
        check("match back on the last text column",
              scr[2][78] == "n")
        check("indicators hold after the show",
              scr[2][34] == "_" and scr[2][IND] == "*")

        run.send(b"q")
        run.wait_exit()
        run.drain()
        run.finish()
        check("exit 0", run.exit_code == 0,
              "exit=%r" % run.exit_code)
        check_terminal_restored("browse", run)
    finally:
        run.finish()
        if not run.exited:
            run.kill()


def main():
    assert_no_fixed_delays()
    log("scenario: pan, reveal, and indicators at the panel text width")
    try:
        scenario_panel_width()
    except SmokeError as e:
        check("panel-width scenario", False, str(e))
    except Exception as e:
        check("panel-width scenario", False, "unexpected error: %r" % e)
    if failures:
        log("FAILED: %d check(s)" % len(failures))
        sys.exit(1)
    log("all checks passed")


if __name__ == "__main__":
    main()
