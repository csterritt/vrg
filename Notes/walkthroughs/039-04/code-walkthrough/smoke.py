#!/usr/bin/env python3
"""Issue #39 smoke harness: unified grapheme/cell rendering under a PTY.

Drives the built vrg binary on a real PTY (80x24) through the issue's
manual scenario: a file whose matched line carries a two-cell CJK
glyph, a base-plus-combining cluster, and an emoji ZWJ cluster, each
overlapped by a match, browsed with the file list visible. The
fixture's displayed paths are absolute (~85 cells) and 文beta.txt's
name ends in a wide glyph, so the list sits at the floor(0.40 x 80) =
32 cap and the run-off-edge layout is:

    [list 32][separator 1][gutter 3][text 43][reserved 1] = 80

The emulator replays the byte stream into a cell grid AND a parallel
inverse-style grid (the dark scheme's match pair is SGR '30;47',
current match '30;47;4'), tracking wide-glyph continuation cells and
stacking zero-width codepoints (combining marks, ZWJ, variation
selectors, and anything a pending ZWJ joins) onto the last painted
cell - so a highlight's styled cells can be asserted exactly.

  startup  — wrap mode: the 文 match styles both cells of the cluster
             and no more, the combining-only match styles the whole
             e+acute cell, the 👩-only match styles both cells of the
             👨‍👩‍👧 cluster, and the wide list entry truncates
             to the cell-exact basename end.
  w        — run-off-edge: the far 文 is entirely hidden right, the
             reserved '*' draws at cell 79.
  >        — offset 10: 文 straddles the left edge; its in-window cell
             is an unstyled blank, never a partial glyph.
  >        — offset 20: 👨‍👩‍👧 straddles the left edge the same
             way; the entirely-hidden matches earn the gutter '*'.
  n        — crossing to 文beta.txt pops the file-change box: the
             border is rectangular measured in cells over a
             left-truncated wide path (a rune-counted interior would
             leave the right border short).
  q        — exit 0 with the terminal restored.

Every key is sent only after the preceding expected frame is observed
in the emulated screen; each wait is a bounded poll on an explicit
rendered condition - never a fixed settling delay. assert_no_fixed_
delays() proves the first half mechanically by parsing this file's
AST; the bounded-poll structure makes the second half reviewable.

The harness also plays terminal honestly: it clears the SSH/session
environment variables that suppress bubbletea's DECRQM probes and
answers the mode-2027 (Unicode core) query with DECRPM 'set', so the
renderer adopts grapheme-cluster widths - otherwise the display side
would measure the ZWJ sequence per codepoint and drop the cells it
believes overflow the row.
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
FIXTURE = "fixture"
FIXTURE_ABS = os.path.join(HERE, FIXTURE)
SUB = "g"
PATTERN = "文|\\x{0301}|👩"
BASENAME = "文beta.txt"

COLS, ROWS = 80, 24
# The displayed paths are absolute (~85 cells), so the list sits at
# the floor(0.40 x 80) = 32 cap and the run-off-edge layout is:
# list [0,32), separator 32, gutter [33,36), text [36,79), indicator
# 79; wrap mode lets text claim the reserved cell (44 cells).
LISTW, SEP, TXT0, IND = 32, 32, 36, 79
MARK = TXT0 - 2  # the gutter's first trailing space

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
    if ch in ("‍", "︎", "️"):
        return 0  # ZWJ and variation selectors join the cluster
    return 2 if unicodedata.east_asian_width(ch) in ("W", "F") else 1


def emulate(data, rows, cols):
    """Replay the byte stream into a ROWSxCOLS text grid plus a
    parallel inverse-style grid.

    Handles CUP/CUx cursor motion, EL/ED erases, IL/DL, deferred
    autowrap, and SGR attributes: cells painted while the dark
    scheme's inverse pair '30;47' is active mark True on the second
    grid. Zero-width codepoints - combining marks, ZWJ, variation
    selectors, and any codepoint a pending ZWJ joins - stack onto the
    last painted cell, so a grapheme cluster occupies exactly its
    measured cells. (Extended from the Issue #38 harness.)
    """
    grid = [[" "] * cols for _ in range(rows)]
    inv = [[False] * cols for _ in range(rows)]
    r = c = 0
    lastc = -1  # column of the last nonzero-width cell on this row
    zwj = False  # a ZWJ was just stacked: the next codepoint joins it
    wrap = False  # deferred wrap: cursor sits past the last column
    cur_inv = False  # SGR state: inside the inverse match pair
    top, bot = 0, rows - 1  # DECSTBM scroll region

    def ri():
        """ESC M - reverse index: scroll the region down at its top."""
        nonlocal r
        if r == top:
            grid.insert(top, [" "] * cols)
            inv.insert(top, [False] * cols)
            del grid[bot + 1:]
            del inv[bot + 1:]
        else:
            r = max(0, r - 1)

    def csi(r, c, params, final):
        nonlocal cur_inv, top, bot
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
                    inv[i][:] = [False] * cols
            elif mode == 0:
                grid[r][c:] = [" "] * (cols - c)
                inv[r][c:] = [False] * (cols - c)
                for i in range(r + 1, rows):
                    grid[i][:] = [" "] * cols
                    inv[i][:] = [False] * cols
            elif mode == 1:
                grid[r][:c + 1] = [" "] * (c + 1)
                inv[r][:c + 1] = [False] * (c + 1)
                for i in range(r):
                    grid[i][:] = [" "] * cols
                    inv[i][:] = [False] * cols
        elif final == "K":
            mode = arg(0, 0)
            if mode == 0:
                grid[r][c:] = [" "] * (cols - c)
                inv[r][c:] = [False] * (cols - c)
            elif mode == 1:
                grid[r][:c + 1] = [" "] * (c + 1)
                inv[r][:c + 1] = [False] * (c + 1)
            elif mode == 2:
                grid[r][:] = [" "] * cols
                inv[r][:] = [False] * cols
        elif final == "L":  # insert blank lines
            for _ in range(arg(0, 1)):
                grid.insert(r, [" "] * cols)
                inv.insert(r, [False] * cols)
                del grid[rows:]
                del inv[rows:]
        elif final == "M":  # delete lines
            for _ in range(arg(0, 1)):
                del grid[r]
                del inv[r]
                grid.append([" "] * cols)
                inv.append([False] * cols)
        elif final == "P":  # delete chars
            n = arg(0, 1)
            del grid[r][c:c + n]
            del inv[r][c:c + n]
            grid[r].extend([" "] * n)
            inv[r].extend([False] * n)
        elif final == "@":  # insert blank chars
            n = arg(0, 1)
            grid[r][c:c] = [" "] * n
            inv[r][c:c] = [False] * n
            del grid[r][cols:]
            del inv[r][cols:]
        elif final == "r":  # DECSTBM: set the scroll region
            top, bot = arg(0, 1) - 1, arg(1, rows) - 1
        elif final == "m":  # SGR: the dark scheme's inverse pair
            cur_inv = "30;47" in params
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
        if zwj:
            w, zwj = 0, False  # a ZWJ joins the next codepoint too
        if ch == "‍":
            w, zwj = 0, True
        if wrap:
            r, c, wrap = min(r + 1, rows - 1), 0, False
        if w == 0:
            # A zero-width codepoint stacks on the cluster's cell.
            if 0 <= r < rows and lastc >= 0:
                grid[r][lastc] += ch
        else:
            if 0 <= r < rows and 0 <= c < cols:
                grid[r][c] = ch
                inv[r][c] = cur_inv
                if w == 2 and c + 1 < cols:
                    grid[r][c + 1] = ""
                    inv[r][c + 1] = cur_inv
            lastc = c
            c += w
            if c >= cols:
                c, wrap = cols - 1, True
        i += size
    # Rows stay lists of cell strings - a wide glyph's continuation
    # cell is "" - so indexing is always by terminal cell, never by
    # character ("".join(row) for substring searches only).
    return grid, inv


def row_text(row):
    """Join a cell row for substring searches."""
    return "".join(row)


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
        # Clear the SSH/session vars that keep bubbletea from asking
        # the terminal about Unicode core mode (DECRQM ?2027); the
        # harness answers the query like a terminal that supports it,
        # so the renderer switches from per-codepoint wcwidth to
        # grapheme-cluster widths and a ZWJ sequence measures its true
        # two cells instead of six.
        for k in ("TERM_PROGRAM", "SSH_TTY", "SSH_CLIENT",
                  "SSH_CONNECTION", "WT_SESSION"):
            env.pop(k, None)

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
        self._answered = set()

    def _pump(self, idle):
        """One I/O-multiplexed read step: wait up to idle for the PTY
        to become readable, then drain what arrived. DECRQM mode
        queries are answered like a supporting terminal (DECRPM
        'set') so the renderer adopts grapheme-cluster widths and
        synchronized updates. EOF is observed when the slave side
        closes (PTY masters report EIO)."""
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
        for mode in (b"2026", b"2027"):
            if (mode not in self._answered
                    and b"\x1b[?" + mode + b"$p" in self.raw):
                os.write(self.fd, b"\x1b[?" + mode + b";1$y")
                self._answered.add(mode)

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
        Returns the satisfying (text, inverse) pair."""
        result = []

        def probe():
            scr, inv = self.screen()
            if pred(scr):
                result.append((scr, inv))
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
    """alpha.txt: one 61-cell matched line whose clusters the pattern
    overlaps - 文 at cells 9-10 (a match on the glyph alone), e+acute
    at cell 13 (a match on the acute's bytes alone), 👨‍👩‍👧 at
    cells 19-20 (a match on 👩's bytes inside the ZWJ cluster), and a
    second 文 at cells 55-56 for the hidden-right indicator - plus a
    short 'cc' line. 文beta.txt carries matches on its own line so 'n'
    crosses files and fires the pop-up; its name makes the list entry
    and pop-up interior measure wide text in cells."""
    d = os.path.join(FIXTURE_ABS, SUB)
    if os.path.exists(d):
        shutil.rmtree(d, ignore_errors=True)
    os.makedirs(d, exist_ok=True)
    line1 = ("a" * 9 + "文" + "bb" + "é" + "z" + "c" * 4
             + "👨‍👩‍👧" + "x" + "t" * 33 + "文" + "d" * 4)
    alpha = line1 + "\ncc\n"
    beta = "bb文cc é 👨‍👩‍👧 end\n"
    for name, data in [("alpha.txt", alpha), ("文beta.txt", beta)]:
        with open(os.path.join(d, name), "w") as f:
            f.write(data)


def scenario_unified_cells():
    """The issue's manual scenario on one 80x24 PTY session."""
    write_fixtures()
    run = PtyRun([VRG, PATTERN, FIXTURE], cwd=HERE)
    try:
        # Wrap mode at startup: line 1's cells 0-43 paint on row 1 at
        # columns 36-79, so 文 lands at 45-46, e+acute at 49, the ZWJ
        # cluster at 55-56, and the second 文 wraps to row 2's 47-48.
        scr, inv = run.wait_screen(
            "initial wrap frame",
            lambda s: s[1][45] == "文" and "文beta.txt" in row_text(s[2]))
        check("list width measured in cells (separator at 32)",
              scr[1][SEP] == " " and scr[1][TXT0 - 3] == "1")
        check("wide list entry truncates to the cell-exact tail",
              row_text(scr[2][:LISTW]).startswith("…")
              and row_text(scr[2][:LISTW]).endswith(BASENAME),
              repr(row_text(scr[2][:LISTW])))
        check("CJK match styles exactly the cluster's two cells",
              inv[1][45] and inv[1][46] and not inv[1][47],
              "inv=%r %r %r" % (inv[1][45], inv[1][46], inv[1][47]))
        check("CJK continuation cell is empty, not a second glyph",
              scr[1][46] == "" and scr[1][47] == "b")
        check("combining-only match styles the whole e+acute cell",
              scr[1][49] == "é" and inv[1][49] and not inv[1][50])
        check("ZWJ-cluster match styles both cells and no more",
              scr[1][55] == "👨‍👩‍👧" and scr[1][56] == ""
              and inv[1][55] and inv[1][56] and not inv[1][57])
        check("wrapped far CJK match keeps its two styled cells",
              scr[2][47] == "文" and inv[2][47] and inv[2][48])

        # w -> run-off-edge. Flat line 2 lands on row 2; the far 文 at
        # cells 55-56 is entirely hidden right of the [0,43) window,
        # so the reserved '*' draws at the panel's right edge.
        run.send(b"w")
        scr, inv = run.wait_screen(
            "run-off-edge frame",
            lambda s: s[2][TXT0 - 3] == "2" and s[1][IND] == "*")
        check("reserved '*' for the hidden-right CJK match",
              scr[1][IND] == "*")
        check("highlights unchanged by the mode toggle",
              inv[1][45] and inv[1][46] and inv[1][49]
              and inv[1][55] and inv[1][56])

        # > pans to offset 10: 文's cluster [9,11) straddles the left
        # edge, so its one in-window cell (10) paints an unstyled
        # blank at column 36 - never half a glyph, never a match
        # cell. A blanked straddler counts hidden under the indicator
        # rule, so the whole 文 match is entirely hidden left and the
        # gutter upgrades straight to '*'. The other clusters paint
        # whole at their new columns, and the far 文 stays out of the
        # [10,53) window.
        run.send(b">")
        scr, inv = run.wait_screen(
            "pan to offset 10",
            lambda s: s[1][TXT0] == " " and s[1][MARK] == "*")
        check("straddling CJK cluster clips to an unstyled blank",
              scr[1][TXT0] == " " and not inv[1][TXT0])
        check("no partial glyph left of the blank",
              scr[1][TXT0 + 1] == "b" and not inv[1][TXT0 + 1])
        check("blanked straddler counts hidden: gutter '*'",
              scr[1][MARK] == "*")
        check("combining cluster still whole and styled",
              scr[1][39] == "é" and inv[1][39] and not inv[1][40])
        check("ZWJ cluster still whole and styled",
              scr[1][45] == "👨‍👩‍👧" and scr[1][46] == ""
              and inv[1][45] and inv[1][46] and not inv[1][47])
        check("reserved '*' holds - far 文 hidden right again",
              scr[1][IND] == "*")
        check("list and separator untouched by the pan",
              scr[1][SEP] == " "
              and row_text(scr[2][:LISTW]).endswith(BASENAME))

        # > again pans to offset 20: 👨‍👩‍👧's cluster [19,21)
        # straddles the left edge; the 文 and e+acute matches are now
        # entirely hidden left, so the gutter upgrades to '*', and
        # the far 文 at cells 55-56 lands styled at columns 71-72.
        run.send(b">")
        scr, inv = run.wait_screen(
            "pan to offset 20",
            lambda s: s[1][71] == "文" and s[1][TXT0] == " ")
        check("straddling ZWJ cluster clips to an unstyled blank",
              scr[1][TXT0] == " " and not inv[1][TXT0])
        check("no partial emoji left of the blank",
              scr[1][TXT0 + 1] == "x" and not inv[1][TXT0 + 1])
        check("gutter '*' holds for the hidden matches",
              scr[1][MARK] == "*")
        check("far CJK match still styled whole",
              scr[1][71] == "文" and inv[1][71] and inv[1][72])
        check("reserved cell blank - no entirely-hidden-right match",
              scr[1][IND] == " ")

        # n crosses to 文beta.txt: the file-change pop-up boxes the
        # escaped absolute path, left-truncated to the 76-cell
        # interior with a leading … - so the border row's '┐' and
        # the interior row's right '│' must share a column even
        # though the tail carries a wide glyph. A rune-counted
        # interior would leave the border one column short.
        run.send(b"n")
        scr, inv = run.wait_screen(
            "file-change pop-up",
            lambda s: any("┌" in row for row in s))
        prow = next(r for r, row in enumerate(scr)
                    if "│" in row and "文beta.txt" in row_text(row))
        pcol = scr[prow].index("│")
        rcol = COLS - 1 - scr[prow][::-1].index("│")
        check("pop-up interior left-truncates on whole clusters",
              scr[prow][pcol + 2] == "…"
              and "文beta.txt" in row_text(scr[prow]))
        check("pop-up border aligned in cells (┐ over right │)",
              scr[prow - 1][rcol] == "┐"
              and scr[prow + 1][rcol] == "┘",
              "rcol=%d top=%r bot=%r"
              % (rcol, scr[prow - 1][rcol], scr[prow + 1][rcol]))
        check("filename rule fits the wide path in cells",
              "文beta.txt" in row_text(scr[0])
              and scr[0][COLS - 1] == " ")

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
    log("scenario: grapheme-cluster rendering, clipping, and the "
        "wide-path pop-up")
    try:
        scenario_unified_cells()
    except SmokeError as e:
        check("unified-rendering scenario", False, str(e))
    except Exception as e:
        check("unified-rendering scenario", False,
              "unexpected error: %r" % e)
    if failures:
        log("FAILED: %d check(s)" % len(failures))
        sys.exit(1)
    log("all checks passed")


if __name__ == "__main__":
    main()
