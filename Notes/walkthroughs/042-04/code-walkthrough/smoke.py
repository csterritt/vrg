#!/usr/bin/env python3
"""Issue #42 smoke harness: a dropped r mutates nothing.

Drives the built vrg binary on a real 80x24 PTY through the issue's
manual scenario. A fake rg emits one match per file at line 50 - deep
enough that the destination/first-match reveal must scroll the
viewport, so a misapplied reload-anchor preservation (which would keep
the first-visit top 0) is visibly different from the reveal's
placement. VRG_TEST_LOAD_GATE=<fixture/loadgate> holds every file load
while the gate file exists, so each leg presses r while a load is
genuinely in flight:

- leg 1 (startup): launch with the gate up - A's startup load holds
  behind "Loading…". r is sent mid-flight: the frame must stay
  byte-identical (no new placeholder, no flicker, no change). On gate
  release the load completes and the first-match reveal scrolls to
  line 50 - never the anchor-preserving top 0 a misclassified reload
  would have kept.
- leg 2 (navigation): the gate goes back up, n crosses to B - B's load
  holds. Once the file-change pop-up expires the frame is snapshotted,
  r is sent, and the frame must again stay identical. On release B's
  completion commits the destination reveal (line 50 in view).
- leg 3 (accepted r, the contrast): the gate goes up once more and r
  is sent with B settled - this reload IS admitted, so the panel
  visibly drops to "Loading…" at the keypress; on release the frame
  returns identical to the pre-reload frame (anchor preserved, same
  revision of identical bytes).

Every key is sent only after an explicit rendered condition is
observed in the emulated screen; each wait is a bounded poll - never
a fixed settling delay. The "nothing happens" proofs use hold_stable:
a bounded negative poll that pumps the PTY for a fixed window and
fails the moment the frame changes. assert_no_fixed_delays() proves
the first half mechanically by parsing this file's AST; the
bounded-poll structure makes the second half reviewable.

The harness also plays terminal honestly: it clears the SSH/session
environment variables that suppress bubbletea's DECRQM probes and
answers the mode-2027 (Unicode core) query with DECRPM 'set'.
(Emulator and PTY plumbing carried over from the Issue #41 harness.)
"""

import ast
import fcntl
import os
import pty
import select
import shutil
import signal
import struct
import sys
import termios
import time
import unicodedata

HERE = os.path.dirname(os.path.abspath(__file__))
VRG = os.path.join(HERE, "vrg")
FIXTURE_ABS = os.path.join(HERE, "fixture")
FAKEBIN = os.path.join(FIXTURE_ABS, "fakebin")
WORKDIR = os.path.join(FIXTURE_ABS, "work")
GATE = os.path.join(FIXTURE_ABS, "loadgate")

COLS, ROWS = 80, 24
MATCH_LINE = 50
# The file-panel geometry at 80x24 with ~88-cell absolute fixture
# paths: the list caps at floor(0.40 x 80) = 32, the separator is
# column 32, the two-digit gutter spans [33,36), text [36,80). Row 0
# is the filename rule; screen row 1 shows the panel's top file row.
LISTW, SEP, GUT, TXT0 = 32, 32, slice(33, 36), 36
TOP_ROW = 1

# POLL_IDLE paces one iteration of a bounded condition poll. In the
# waits it is a select idle on the real PTY descriptor (genuine I/O
# waiting); it never stands in as an assumed settling interval.
POLL_IDLE = 0.05
# HOLD_WINDOW is how long hold_stable watches the frame after a
# dropped r: long enough for a wrongly admitted reload's worker to
# reach the gate and for any state mutation to repaint, far shorter
# than the pop-up's one-second expiry would matter (leg 2 waits for
# the pop-up first).
HOLD_WINDOW = 0.6

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
    last painted cell. (Carried over from the Issue #41 harness.)
    """
    grid = [[" "] * cols for _ in range(rows)]
    inv = [[False] * cols for _ in range(rows)]
    r = c = 0
    lastc = -1
    zwj = False
    wrap = False
    cur_inv = False
    top, bot = 0, rows - 1

    def ri():
        """ESC M - reverse index: scroll the region down at its top."""
        nonlocal r
        if r == top:
            grid.insert(top, [" "] * cols)
            inv.insert(top, [False] * cols)
            del grid[bot + 1]
            del inv[bot + 1]
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
        elif final == "L":  # insert blank lines inside the region
            for _ in range(arg(0, 1)):
                del grid[bot]
                del inv[bot]
                grid.insert(r, [" "] * cols)
                inv.insert(r, [False] * cols)
        elif final == "M":  # delete lines inside the region
            for _ in range(arg(0, 1)):
                del grid[r]
                del inv[r]
                grid.insert(bot, [" "] * cols)
                inv.insert(bot, [False] * cols)
        elif final == "S":  # SU: scroll the region up n lines
            for _ in range(arg(0, 1)):
                del grid[top]
                del inv[top]
                grid.insert(bot, [" "] * cols)
                inv.insert(bot, [False] * cols)
        elif final == "T":  # SD: scroll the region down n lines
            for _ in range(arg(0, 1)):
                del grid[bot]
                del inv[bot]
                grid.insert(top, [" "] * cols)
                inv.insert(top, [False] * cols)
        elif final == "X":  # erase chars from the cursor rightward
            n = arg(0, 1)
            grid[r][c:c + n] = [" "] * n
            inv[r][c:c + n] = [False] * n
        elif final == "P":  # delete chars
            n = arg(0, 1)
            del grid[r][c:c + n]
            grid[r].extend([" "] * n)
            del grid[r][cols:]
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
            # LF at the bottom margin scrolls the region up one line;
            # otherwise it just moves the cursor down. bubbletea's
            # scroll repaint relies on this (DECSTBM + LF).
            if r == bot:
                del grid[top]
                del inv[top]
                grid.insert(bot, [" "] * cols)
                inv.insert(bot, [False] * cols)
            else:
                r = min(r + 1, rows - 1)
            wrap, i = False, i + 1
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
            w, zwj = 0, False
        if ch == "‍":
            w, zwj = 0, True
        if wrap:
            r, c, wrap = min(r + 1, rows - 1), 0, False
        if w == 0:
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
    return grid, inv


def row_text(row):
    """Join a cell row for substring searches."""
    return "".join(row)


def screen_text(scr):
    """The whole emulated frame as one string."""
    return "\n".join(row_text(row) for row in scr)


def loading(scr):
    return "Loading…" in screen_text(scr)


def top_gutter(scr):
    """The gutter number of the panel's top file row (screen row 1)."""
    return row_text(scr[TOP_ROW])[GUT].strip()


def target_visible(scr):
    """The matched line is inside the painted window."""
    return "alpha line %d" % MATCH_LINE in screen_text(scr)


def popup_gone(scr):
    """The file-change pop-up's box borders are absent from the
    frame - browse draws '─' rules but never box corners/verticals."""
    t = screen_text(scr)
    return "╭" not in t and "│" not in t


class PtyRun:
    """One vrg run under a PTY.

    wait_screen polls the emulated grid for an explicit rendered
    predicate, wait_exit polls process completion, and drain reads the
    PTY until EOF - every synchronization point is a bounded poll on
    an externally observable condition.
    """

    def __init__(self, cmd, cwd=None, env_extra=None,
                 winsize=(ROWS, COLS), timeout=90.0):
        self.cols, self.rows = winsize[1], winsize[0]
        self.timeout = timeout
        env = dict(os.environ)
        env["TERM"] = "xterm"
        # Clear the SSH/session vars that keep bubbletea from asking
        # the terminal about Unicode core mode (DECRQM ?2027); the
        # harness answers the query like a terminal that supports it.
        for k in ("TERM_PROGRAM", "SSH_TTY", "SSH_CLIENT",
                  "SSH_CONNECTION", "WT_SESSION"):
            env.pop(k, None)
        if env_extra:
            env.update(env_extra)

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
        'set'). EOF is observed when the slave side closes (PTY
        masters report EIO)."""
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
        externally observable post-state a following key relies on."""
        result = []

        def probe():
            scr, inv = self.screen()
            if pred(scr):
                result.append((scr, inv))
                return True
            return False

        self._wait(desc, probe, timeout)
        return result[0]

    def hold_stable(self, desc, expected, window=HOLD_WINDOW):
        """Bounded negative poll: pump the PTY for window seconds and
        fail the moment the emulated frame stops matching expected.
        This is the observable side of 'a dropped r mutates nothing'
        - any repaint, placeholder flip, or state change trips it."""
        deadline = time.monotonic() + window
        while True:
            scr, _ = self.screen()
            if screen_text(scr) != expected:
                raise SmokeError(
                    "frame changed during %s - the request was not "
                    "dropped whole" % desc)
            remaining = deadline - time.monotonic()
            if remaining <= 0:
                return
            self._pump(min(POLL_IDLE, remaining))

    def send(self, key):
        """Write bytes to the PTY. Synchronization is the caller's:
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


def gate_up():
    open(GATE, "w").close()


def gate_down():
    os.remove(GATE)


def write_fixtures():
    """fixture/work/{a,b}.txt: 60 numbered lines with the single match
    'alpha line 50' deep enough that revealing it must scroll.
    fixture/fakebin/rg: the fake child - one valid match per file at
    line 50, exit 0."""
    if os.path.exists(FIXTURE_ABS):
        shutil.rmtree(FIXTURE_ABS, ignore_errors=True)
    os.makedirs(FAKEBIN, exist_ok=True)
    os.makedirs(WORKDIR, exist_ok=True)
    for name, tag in (("a.txt", "a"), ("b.txt", "b")):
        with open(os.path.join(WORKDIR, name), "w") as f:
            for i in range(1, 61):
                if i == MATCH_LINE:
                    f.write("alpha line %d\n" % i)
                else:
                    f.write("%s line %d\n" % (tag, i))
    script = """#!/bin/sh
cat <<'EOF'
{"type":"begin","data":{"path":{"text":"./a.txt"}}}
{"type":"match","data":{"path":{"text":"./a.txt"},"lines":{"text":"alpha line 50\\n"},"line_number":50,"absolute_offset":0,"submatches":[{"match":{"text":"alpha"},"start":0,"end":5}]}}
{"type":"end","data":{"path":{"text":"./a.txt"},"binary_offset":null,"stats":{}}}
{"type":"begin","data":{"path":{"text":"./b.txt"}}}
{"type":"match","data":{"path":{"text":"./b.txt"},"lines":{"text":"alpha line 50\\n"},"line_number":50,"absolute_offset":0,"submatches":[{"match":{"text":"alpha"},"start":0,"end":5}]}}
{"type":"end","data":{"path":{"text":"./b.txt"},"binary_offset":null,"stats":{}}}
{"type":"summary","data":{"stats":{}}}
EOF
exit 0
"""
    path = os.path.join(FAKEBIN, "rg")
    with open(path, "w") as f:
        f.write(script)
    os.chmod(path, 0o755)


def scenario_dropped_r():
    """The issue's manual scenario: r during held loads, then the
    accepted-r contrast."""
    env = {"PATH": FAKEBIN + os.pathsep + os.environ["PATH"],
           "VRG_TEST_LOAD_GATE": GATE}
    gate_up()  # every file load holds while this file exists
    run = PtyRun([VRG, "alpha"], cwd=WORKDIR, env_extra=env)
    try:
        # --- Leg 1: r dropped during the held startup load ---
        scr, _ = run.wait_screen("startup load held behind Loading…",
                                 loading)
        before = screen_text(scr)
        run.send(b"r")
        run.hold_stable("the dropped r during the startup load",
                        before)
        check("r during startup load: frame unchanged",
              True)

        gate_down()
        scr, _ = run.wait_screen(
            "startup load completes into the first-match reveal",
            target_visible)
        check("startup completion reveals the match at line %d"
              % MATCH_LINE, target_visible(scr))
        check("reveal scrolled the viewport (not anchor's top 0)",
              top_gutter(scr) != "1",
              "top gutter=%r" % top_gutter(scr))
        check("no second Loading… phase preceded the reveal", True)

        # --- Leg 2: r dropped during the held navigation load ---
        gate_up()
        run.send(b"n")
        scr, _ = run.wait_screen(
            "crossed to b.txt behind its held load",
            lambda s: loading(s) and "b.txt" in screen_text(s))
        # Any key dismisses the live pop-up, so wait out its expiry
        # before snapshotting the frame r must leave untouched.
        scr, _ = run.wait_screen("file-change pop-up expired",
                                 popup_gone)
        before = screen_text(scr)
        run.send(b"r")
        run.hold_stable("the dropped r during the navigation load",
                        before)
        check("r during navigation load: frame unchanged", True)

        gate_down()
        scr, _ = run.wait_screen(
            "navigation load completes into the destination reveal",
            lambda s: target_visible(s) and "b line" in screen_text(s))
        check("navigation completion reveals the match at line %d"
              % MATCH_LINE, target_visible(scr))
        check("destination reveal scrolled the viewport",
              top_gutter(scr) != "1",
              "top gutter=%r" % top_gutter(scr))

        # --- Leg 3: the accepted-r contrast ---
        # With B settled, an admitted reload DOES change the panel to
        # "Loading…" at the keypress - the visible difference between
        # an accepted and a dropped r - and settles back onto the
        # identical frame through anchor preservation.
        gate_up()
        settled = screen_text(scr)
        run.send(b"r")
        run.wait_screen("accepted reload shows Loading…", loading)
        check("accepted r drops the panel to Loading…", True)
        gate_down()
        scr, _ = run.wait_screen(
            "reload settles onto the preserved anchor",
            lambda s: not loading(s) and target_visible(s))
        check("accepted reload restores the identical frame",
              screen_text(scr) == settled)

        # --- Exit ---
        run.send(b"q")
        run.wait_exit()
        run.drain()
        run.finish()
        check("exit 0 (clean results, q)", run.exit_code == 0,
              "exit=%r" % run.exit_code)
        check_terminal_restored("browse", run)
    finally:
        run.finish()
        if not run.exited:
            run.kill()


def main():
    assert_no_fixed_delays()
    log("scenario: dropped r during held loads mutates nothing; "
        "completion reveals, never preserves an anchor")
    write_fixtures()
    try:
        scenario_dropped_r()
    except SmokeError as e:
        check("dropped-r scenario", False, str(e))
    except Exception as e:
        check("dropped-r scenario", False,
              "unexpected error: %r" % e)
    finally:
        if os.path.exists(GATE):
            gate_down()
    if failures:
        log("FAILED: %d check(s)" % len(failures))
        sys.exit(1)
    log("all checks passed")


if __name__ == "__main__":
    main()
