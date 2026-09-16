#!/usr/bin/env python3
"""Canonical condition-driven fake-rg PTY smoke harness for vrg.

Issue #50 made this script the canonical smoke harness. The Issue #35
copy at Notes/walkthroughs/035-03/code-walkthrough/smoke.py is a frozen
historical artifact of that issue's walkthrough and is not edited.

Contract (Issue #50): every key is sent only after the preceding
expected UI state or output marker is observed in the PTY stream;
output draining is completion/EOF-driven; bounded polls re-check an
explicit, externally observable condition on every iteration and exist
only to fail a missing condition — never as an assumed settling
interval. A fake-child ready file proves the fixture started; it is
never sufficient evidence that the app entered a state or processed a
key, so every scenario also waits on the rendered output marker for
the state its key depends on.

The harness contains no time.sleep call and no other fixed delay used
as a progress proxy; assert_no_fixed_delays() proves the first half
mechanically by parsing this file's AST, and the bounded-poll
structure makes the second half reviewable.

Fixture variables use the FAKE_RG_* namespace (the Issue #50 rename of
VRG_TEST_HANDSHAKE/READY/PID/ARGV/CWD). The production smoke runs the
untagged binary, so it must neither set nor depend on any VRG_TEST_*
name — assert_no_vrg_test_env proves each run's environment clean and
the cancellation scenario proves child/process-group termination
externally (kill -0 probes) rather than through a test seam.
"""

import ast
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

VRG = "/tmp/vrg-smoke/vrg"
WORK = "/tmp/vrg-smoke/run"

# POLL_IDLE paces one iteration of a bounded condition poll. In PtyRun
# waits it is a select idle on real descriptors (genuine I/O waiting);
# in the file/PID polls it bounds the re-check interval of the explicit
# condition. It is never a settling delay: no code path treats elapsed
# idles as evidence of progress.
POLL_IDLE = 0.05

failures = []


class SmokeError(Exception):
    """A bounded wait failed: the named condition never held."""


def log(msg):
    print(msg, flush=True)


def strip_ansi(s):
    out = []
    i = 0
    while i < len(s):
        if s[i] == "\x1b":
            if i + 1 < len(s) and s[i + 1] == "[":
                i += 2
                while i < len(s) and not (0x40 <= ord(s[i]) <= 0x7e):
                    i += 1
                if i < len(s):
                    i += 1
                continue
            i += 1
            if i < len(s):
                i += 1
            continue
        out.append(s[i])
        i += 1
    return "".join(out)


def assert_no_fixed_delays():
    """Static self-check: parse this file's AST and fail if it calls
    time.sleep — the fixed-settling primitive the Issue #50 contract
    bans. Bounded polls pace themselves with select idles that re-check
    an explicit condition every iteration; nothing else in this file
    waits on elapsed time."""
    with open(__file__) as f:
        tree = ast.parse(f.read(), __file__)
    for node in ast.walk(tree):
        if (isinstance(node, ast.Call)
                and isinstance(node.func, ast.Attribute)
                and node.func.attr == "sleep"
                and isinstance(node.func.value, ast.Name)
                and node.func.value.id == "time"):
            raise SmokeError(
                "time.sleep call found in %s — use a bounded condition "
                "poll on an explicit observable condition" % __file__)


def assert_no_vrg_test_env(env):
    """The production smoke must neither set nor depend on a VRG_TEST_*
    name: the untagged binary has no test seams, so a leaked control
    would silently do nothing and mask a synchronization gap."""
    leaked = sorted(k for k in env if k.startswith("VRG_TEST_"))
    if leaked:
        raise SmokeError(
            "smoke environment carries vrg test-hook names %s — the "
            "production binary ignores them" % leaked)


def wait_condition(desc, cond, timeout):
    """Bounded condition poll: re-check cond every POLL_IDLE until it
    holds or the timeout fails the wait with the condition's name."""
    deadline = time.monotonic() + timeout
    while True:
        if cond():
            return
        if time.monotonic() >= deadline:
            raise SmokeError("timed out after %.1fs waiting for %s"
                             % (timeout, desc))
        select.select([], [], [], POLL_IDLE)


def wait_file(path, timeout=15.0):
    """Bounded poll for a fixture file. The file proves only that the
    fixture reached the point that writes it — never that the app
    entered a state."""
    wait_condition("fixture file %s" % path,
                   lambda: os.path.exists(path), timeout)


def pid_alive(pid):
    try:
        os.kill(pid, 0)
        return True
    except OSError:
        return False


def pgid_alive(pgid):
    try:
        os.killpg(pgid, 0)
        return True
    except OSError:
        return False


def wait_pid_gone(pid, timeout=10.0):
    """Bounded poll until the fixture PID is dead — external evidence
    of child termination with no test seam involved."""
    wait_condition("pid %d gone" % pid,
                   lambda: not pid_alive(pid), timeout)


def wait_pgid_gone(pgid, timeout=10.0):
    """Bounded poll until the fixture process group is gone — external
    evidence that vrg's process-group kill reached every member."""
    wait_condition("process group %d gone" % pgid,
                   lambda: not pgid_alive(pgid), timeout)


def check(name, cond, detail=""):
    status = "PASS" if cond else "FAIL"
    if cond:
        log("  [%s] %s" % (status, name))
    else:
        log("  [%s] %s%s" % (status, name, ": " + detail if detail else ""))
        failures.append(name)


def fresh_dir(label):
    d = os.path.join(WORK, label)
    if os.path.exists(d):
        shutil.rmtree(d, ignore_errors=True)
    os.makedirs(d, exist_ok=True)
    return d


def write_fake_rg(dirpath, script):
    rg = os.path.join(dirpath, "rg")
    with open(rg, "w") as f:
        f.write(script)
    os.chmod(rg, 0o755)
    return rg


class PtyRun:
    """One vrg run under a PTY with a separated stderr pipe.

    wait_output polls the ANSI-stripped PTY stream for an explicit
    rendered marker, wait_exit polls process completion, and drain
    reads both pipes until EOF — every synchronization point is a
    bounded poll on an externally observable condition.
    """

    def __init__(self, cmd, env, cwd=None, winsize=(24, 80), timeout=30.0):
        self.timeout = timeout
        rerr, werr = os.pipe()
        cmd_env = dict(os.environ)
        cmd_env.update(env)
        cmd_env["TERM"] = "xterm-256color"
        assert_no_vrg_test_env(cmd_env)

        pid, fd = pty.fork()
        if pid == 0:
            os.close(rerr)
            os.dup2(werr, 2)
            os.close(werr)
            if cwd:
                os.chdir(cwd)
            os.execvpe(cmd[0], cmd, cmd_env)
            os._exit(127)

        os.close(werr)
        try:
            wins = struct.pack("HHHH", winsize[0], winsize[1], 0, 0)
            import fcntl
            fcntl.ioctl(fd, termios.TIOCSWINSZ, wins)
        except OSError:
            pass

        self.pid = pid
        self.fd = fd
        self.rerr = rerr
        self.before_termios = termios.tcgetattr(fd)
        self.after_termios = None
        self.raw = bytearray()
        self.err = bytearray()
        self.eof = False
        self.err_eof = False
        self.exited = False
        self.exit_code = None

    def _pump(self, idle):
        """One I/O-multiplexed read step: wait up to idle for either
        pipe to become readable, then drain what arrived. EOF is
        observed per pipe (PTY masters report EIO when the slave side
        closes)."""
        fds = []
        if not self.eof:
            fds.append(self.fd)
        if not self.err_eof:
            fds.append(self.rerr)
        if not fds:
            return
        r, _, _ = select.select(fds, [], [], idle)
        for f in r:
            try:
                data = os.read(f, 65536)
            except OSError:
                data = b""
            if f == self.fd:
                if data:
                    self.raw += data
                else:
                    self.eof = True
            else:
                if data:
                    self.err += data
                else:
                    self.err_eof = True

    def _wait(self, desc, cond, timeout=None):
        """Bounded condition poll over pumped PTY/stderr state."""
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

    def text(self):
        return self.raw.decode("utf-8", "replace")

    def stderr(self):
        return self.err.decode("utf-8", "replace")

    def wait_output(self, marker, timeout=None):
        """Wait until the ANSI-stripped rendered stream contains marker
        — the externally observable post-state a following key relies
        on."""
        self._wait("output marker %r" % marker,
                   lambda: marker in strip_ansi(self.text()), timeout)

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
        """Completion/EOF-driven draining: read the PTY master and the
        stderr pipe until both report EOF (bounded)."""
        self._wait("PTY/stderr EOF",
                   lambda: self.eof and self.err_eof, timeout)

    def send(self, key):
        """Write one key to the PTY. Synchronization is the caller's:
        a key is only ever sent after wait_output observed the UI
        state that key depends on."""
        os.write(self.fd, key.encode("utf-8"))

    def finish(self):
        """Snapshot post-exit termios and close both pipes."""
        try:
            self.after_termios = termios.tcgetattr(self.fd)
        except OSError:
            pass
        for f in (self.fd, self.rerr):
            try:
                os.close(f)
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


def run_scenario(name, fn):
    log(name)
    try:
        fn()
    except SmokeError as e:
        check(name, False, str(e))
    except Exception as e:
        check(name, False, "unexpected error: %r" % e)


def scenario_browse():
    """Successful browse -> q -> exit 0."""
    fake_dir = fresh_dir("s0_fake")
    repo = fresh_dir("s0_repo")
    hs_dir = fresh_dir("s0_hs")
    hs = os.path.join(hs_dir, "handshake")
    with open(os.path.join(repo, "test.txt"), "w") as f:
        f.write("hello world\n")
    write_fake_rg(fake_dir, """#!/bin/sh
echo '{"type":"begin","data":{"path":{"text":"test.txt"}}}'
printf '%s\\n' '{"type":"match","data":{"path":{"text":"test.txt"},"lines":{"text":"hello world\\n"},"line_number":1,"submatches":[{"match":{"text":"hello"},"start":0,"end":5}]}}'
echo '{"type":"end","data":{"path":{"text":"test.txt"},"binary_offset":null}}'
echo '{"type":"summary","data":{}}'
if [ -n "$FAKE_RG_HANDSHAKE_FILE" ]; then touch "$FAKE_RG_HANDSHAKE_FILE"; fi
exit 0
""")
    run = PtyRun([VRG, "hello", "."],
                 env={"PATH": fake_dir + ":" + os.environ["PATH"],
                      "FAKE_RG_HANDSHAKE_FILE": hs},
                 cwd=repo)
    try:
        # The handshake proves the fixture wrote its stream; the q is
        # gated on the rendered browse view — the observable post-state
        # of the completed search, not the fixture file.
        wait_file(hs)
        run.wait_output("test.txt")
        run.send("q")
        run.wait_exit()
        run.drain()
    except Exception:
        run.kill()
        raise
    run.finish()

    visible = strip_ansi(run.text())
    check("browse exit 0", run.exit_code == 0, "exit=%s" % run.exit_code)
    check("browse view shows test.txt", "test.txt" in visible)
    check("browse stderr empty", run.stderr() == "",
          "stderr=%r" % run.stderr())
    check("browse PTY reached EOF", run.eof)


def scenario_no_results():
    """No-results search with a warning -> Esc, q -> exit 1."""
    fake_dir = fresh_dir("s1_fake")
    repo = fresh_dir("s1_repo")
    hs_dir = fresh_dir("s1_hs")
    hs = os.path.join(hs_dir, "handshake")
    with open(os.path.join(repo, "test.txt"), "w") as f:
        f.write("hello\n")
    write_fake_rg(fake_dir, """#!/bin/sh
echo '{"type":"summary","data":{}}'
printf 'warn\\n' >&2
if [ -n "$FAKE_RG_HANDSHAKE_FILE" ]; then touch "$FAKE_RG_HANDSHAKE_FILE"; fi
exit 1
""")
    run = PtyRun([VRG, "hello", "."],
                 env={"PATH": fake_dir + ":" + os.environ["PATH"],
                      "FAKE_RG_HANDSHAKE_FILE": hs},
                 cwd=repo)
    try:
        wait_file(hs)
        # The warning overlay must be observed before Esc dismisses it,
        # and the no-results view before q quits it.
        run.wait_output("warn")
        run.send("\x1b")
        run.wait_output("No results found")
        run.send("q")
        run.wait_exit()
        run.drain()
    except Exception:
        run.kill()
        raise
    run.finish()

    visible = strip_ansi(run.text())
    check("no-results exit 1", run.exit_code == 1,
          "exit=%s" % run.exit_code)
    check("no-results shows 'No results found'",
          "No results found" in visible)
    check("no-results stderr replays 'warn'", "warn" in run.stderr(),
          "stderr=%r" % run.stderr())
    check("no-results PTY reached EOF", run.eof)


def scenario_fatal():
    """Fatal fake-rg with a malformed record -> q and Esc -> exit 2.

    The fixture emits one malformed line and exits 2, so the composed
    fatal diagnostic (Issues #36-37, #44) carries all three component
    kinds: the generated process line naming the exit code, the
    missing-summary integrity cause, and the malformed record-loss
    component.
    """
    fake_dir = fresh_dir("s2_fake")
    repo = fresh_dir("s2_repo")
    hs_dir = fresh_dir("s2_hs")
    hs = os.path.join(hs_dir, "handshake")
    with open(os.path.join(repo, "test.txt"), "w") as f:
        f.write("hello\n")
    write_fake_rg(fake_dir, """#!/bin/sh
printf 'not-a-json-record\\n'
if [ -n "$FAKE_RG_HANDSHAKE_FILE" ]; then touch "$FAKE_RG_HANDSHAKE_FILE"; fi
exit 2
""")

    def one_run(key):
        run = PtyRun([VRG, "hello", "."],
                     env={"PATH": fake_dir + ":" + os.environ["PATH"],
                          "FAKE_RG_HANDSHAKE_FILE": hs},
                     cwd=repo)
        try:
            wait_file(hs)
            # The fatal overlay's process component must be observed
            # rendered before the dismissal key — the acknowledgement
            # that the composed diagnostic painted.
            run.wait_output("code 2")
            run.send(key)
            run.wait_exit()
            run.drain()
        except Exception:
            run.kill()
            raise
        run.finish()
        return run

    run = one_run("q")
    visible = strip_ansi(run.text())
    check("fatal q exit 2", run.exit_code == 2,
          "exit=%s" % run.exit_code)
    check("fatal q overlay names exit code", "code 2" in visible,
          "visible=%r" % visible)
    check("fatal q overlay states integrity cause",
          "missing summary record" in visible, "visible=%r" % visible)
    check("fatal q overlay states record-loss cause",
          "malformed record" in visible, "visible=%r" % visible)
    check("fatal q stderr replays composed diagnostic",
          "ripgrep exited with code 2" in run.stderr(),
          "stderr=%r" % run.stderr())

    run = one_run("\x1b")
    check("fatal Esc exit 2", run.exit_code == 2,
          "exit=%s" % run.exit_code)


def scenario_cancel():
    """Cancellation while searching -> exit 130, child and its process
    group externally observed gone, PTY EOF, terminal restored."""
    fake_dir = fresh_dir("s130_fake")
    repo = fresh_dir("s130_repo")
    aux = fresh_dir("s130_aux")
    ready = os.path.join(aux, "ready")
    pid_file = os.path.join(aux, "pid")
    with open(os.path.join(repo, "test.txt"), "w") as f:
        f.write("hello\n")
    write_fake_rg(fake_dir, """#!/bin/sh
if [ -n "$FAKE_RG_PID_FILE" ]; then echo $$ > "$FAKE_RG_PID_FILE"; fi
if [ -n "$FAKE_RG_READY_FILE" ]; then touch "$FAKE_RG_READY_FILE"; fi
sleep 100000
""")
    run = PtyRun([VRG, "hello", "."],
                 env={"PATH": fake_dir + ":" + os.environ["PATH"],
                      "FAKE_RG_READY_FILE": ready,
                      "FAKE_RG_PID_FILE": pid_file},
                 cwd=repo)
    try:
        # The ready file proves only that the fixture started; the
        # rendered "Searching…" view proves the app entered the state
        # whose cancellation contract exit 130 belongs to.
        wait_file(ready)
        run.wait_output("Searching")
        run.send("q")
        run.wait_exit()
        run.drain()
    except Exception:
        run.kill()
        raise
    run.finish()

    check("cancel exit 130", run.exit_code == 130,
          "exit=%s" % run.exit_code)
    try:
        with open(pid_file) as f:
            cpid = int(f.read().strip())
        wait_pid_gone(cpid)
        check("child process gone", not pid_alive(cpid))
        wait_pgid_gone(cpid)
        check("child process group gone", not pgid_alive(cpid))
    except SmokeError:
        raise
    except Exception as e:
        check("child/process-group termination", False,
              "cannot read pid file: %s" % e)
    check("cancel PTY reached EOF", run.eof)
    raw = run.text()
    check("terminal cursor restored", "\x1b[?25h" in raw)
    if "\x1b[?1049h" in raw:
        check("alt screen exited", "\x1b[?1049l" in raw)
    else:
        check("alt screen exited", True)
    check("termios restored", run.before_termios == run.after_termios)


def scenario_ctrl_c():
    """Cancellation via ctrl+c while searching -> exit 130."""
    fake_dir = fresh_dir("s130b_fake")
    repo = fresh_dir("s130b_repo")
    aux = fresh_dir("s130b_aux")
    ready = os.path.join(aux, "ready")
    pid_file = os.path.join(aux, "pid")
    with open(os.path.join(repo, "test.txt"), "w") as f:
        f.write("hello\n")
    write_fake_rg(fake_dir, """#!/bin/sh
if [ -n "$FAKE_RG_PID_FILE" ]; then echo $$ > "$FAKE_RG_PID_FILE"; fi
if [ -n "$FAKE_RG_READY_FILE" ]; then touch "$FAKE_RG_READY_FILE"; fi
sleep 100000
""")
    run = PtyRun([VRG, "hello", "."],
                 env={"PATH": fake_dir + ":" + os.environ["PATH"],
                      "FAKE_RG_READY_FILE": ready,
                      "FAKE_RG_PID_FILE": pid_file},
                 cwd=repo)
    try:
        wait_file(ready)
        run.wait_output("Searching")
        run.send("\x03")
        run.wait_exit()
        run.drain()
    except Exception:
        run.kill()
        raise
    run.finish()

    check("ctrl+c exit 130", run.exit_code == 130,
          "exit=%s" % run.exit_code)
    try:
        with open(pid_file) as f:
            cpid = int(f.read().strip())
        wait_pid_gone(cpid)
        check("ctrl+c child process gone", not pid_alive(cpid))
        wait_pgid_gone(cpid)
        check("ctrl+c process group gone", not pgid_alive(cpid))
    except SmokeError:
        raise
    except Exception as e:
        check("ctrl+c child/process-group termination", False,
              "cannot read pid file: %s" % e)


def scenario_help():
    """Help-only: bare vrg, -h, --help -> exit 0, one help copy on
    stdout, empty stderr, sentinel fake rg never invoked."""
    help_dir = fresh_dir("help_path")
    sentinel_marker_dir = fresh_dir("help_marker")
    sentinel_marker = os.path.join(sentinel_marker_dir, "rg-ran")
    sentinel = os.path.join(help_dir, "rg")
    with open(sentinel, "w") as f:
        f.write("#!/bin/sh\n: > %s\n" % sentinel_marker)
    os.chmod(sentinel, 0o755)
    env = {"PATH": help_dir, "TERM": "xterm"}
    assert_no_vrg_test_env(env)
    for args in ([], ["-h"], ["--help"]):
        proc = subprocess.run([VRG] + args, env=env, capture_output=True)
        out = proc.stdout.decode("utf-8", "replace")
        err = proc.stderr.decode("utf-8", "replace")
        label = "bare vrg" if not args else " ".join(args)
        check("%s exit 0" % label, proc.returncode == 0,
              "exit=%d" % proc.returncode)
        check("%s exactly one 'Usage:' on stdout" % label,
              out.count("Usage:") == 1, "count=%d" % out.count("Usage:"))
        check("%s stderr empty" % label, err == "", "stderr=%r" % err)
        check("%s no terminal control sequences" % label,
              "\x1b[?1049" not in out and "\x9b" not in out)
    check("sentinel fake rg never invoked",
          not os.path.exists(sentinel_marker), "sentinel was invoked")


def main():
    assert_no_fixed_delays()
    if not os.path.exists(VRG):
        log("smoke binary missing: build it with "
            "`go build -o /tmp/vrg-smoke/vrg ./cmd/vrg`")
        sys.exit(2)
    os.makedirs(WORK, exist_ok=True)

    run_scenario("Scenario 0: successful browse, q exits 0",
                 scenario_browse)
    run_scenario("Scenario 1: no-results search, warning dismissed, "
                 "q exits 1", scenario_no_results)
    run_scenario("Scenario 2: fatal fake-rg, composed "
                 "integrity/record-loss diagnostics, q and Esc exit 2",
                 scenario_fatal)
    run_scenario("Scenario 130: cancellation while searching, child "
                 "and process group gone, terminal restored",
                 scenario_cancel)
    run_scenario("Scenario 130b: cancellation via ctrl+c while "
                 "searching", scenario_ctrl_c)
    run_scenario("Scenario help-only: bare vrg, -h, --help (sentinel "
                 "fake rg never invoked)", scenario_help)

    log("")
    if failures:
        log("SMOKE FAIL: %d failure(s)" % len(failures))
        for f in failures:
            log("  - %s" % f)
        sys.exit(1)
    log("SMOKE OK: all five outcomes verified")


if __name__ == "__main__":
    main()
