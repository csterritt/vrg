#!/usr/bin/env python3
"""Final-binary smoke run for vrg Issue #35.

Drives the final vrg binary through the five representative outcomes
using the Issue #4 fake-rg harness under a real PTY.
"""

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
import threading

VRG = "/tmp/vrg-smoke/vrg"
WORK = "/tmp/vrg-smoke/run"

failures = []


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


def wait_for_file(path, timeout=15.0):
    deadline = time.time() + timeout
    while time.time() < deadline:
        if os.path.exists(path):
            return True
        time.sleep(0.02)
    return False


def pid_alive(pid):
    try:
        os.kill(pid, 0)
        return True
    except OSError:
        return False


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


def run_pty(cmd, env, keys, handshake=None, ready=None, cwd=None,
            key_delay=0.3, timeout=30.0, winsize=(24, 80)):
    """Run vrg under a PTY, wait for handshake/ready, send keys, return
    (raw_stdout, stderr, exit_code, before_termios, after_termios)."""
    rerr, werr = os.pipe()
    cmd_env = dict(os.environ)
    cmd_env.update(env)
    cmd_env["TERM"] = "xterm-256color"

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
        import fcntl
        wins = struct.pack("HHHH", winsize[0], winsize[1], 0, 0)
        fcntl.ioctl(fd, termios.TIOCSWINSZ, wins)
    except Exception:
        pass

    before_termios = termios.tcgetattr(fd)

    raw_buf = []
    err_buf = []
    stop = threading.Event()

    def reader():
        while not stop.is_set():
            r, _, _ = select.select([fd], [], [], 0.1)
            if fd in r:
                try:
                    data = os.read(fd, 65536)
                    if data:
                        raw_buf.append(data)
                    else:
                        return
                except OSError:
                    return

    def err_reader():
        while not stop.is_set():
            r, _, _ = select.select([rerr], [], [], 0.1)
            if rerr in r:
                try:
                    data = os.read(rerr, 65536)
                    if data:
                        err_buf.append(data)
                    else:
                        return
                except OSError:
                    return

    rt = threading.Thread(target=reader, daemon=True)
    et = threading.Thread(target=err_reader, daemon=True)
    rt.start()
    et.start()

    # wait for handshake/ready then send keys
    gate = handshake or ready
    if gate:
        wait_for_file(gate, timeout=timeout)
    time.sleep(key_delay)
    for k in keys:
        try:
            os.write(fd, k.encode("utf-8"))
        except OSError:
            pass
        time.sleep(0.15)

    # wait for child exit
    exit_code = -1
    deadline = time.time() + timeout
    done = False
    while time.time() < deadline:
        try:
            wpid, status = os.waitpid(pid, os.WNOHANG)
            if wpid == pid:
                if os.WIFEXITED(status):
                    exit_code = os.WEXITSTATUS(status)
                elif os.WIFSIGNALED(status):
                    exit_code = 128 + os.WTERMSIG(status)
                done = True
                break
        except ChildProcessError:
            done = True
            break
        time.sleep(0.05)

    if not done:
        try:
            os.kill(pid, signal.SIGKILL)
        except OSError:
            pass
        os.waitpid(pid, 0)
        failures.append("timeout running %r" % (cmd,))
        stop.set()
        return "", "", -1, before_termios, None

    # drain
    time.sleep(0.3)
    stop.set()
    rt.join(timeout=1)
    et.join(timeout=1)

    after_termios = None
    try:
        after_termios = termios.tcgetattr(fd)
    except Exception:
        pass
    try:
        os.close(fd)
    except OSError:
        pass
    try:
        os.close(rerr)
    except OSError:
        pass

    return (b"".join(raw_buf).decode("utf-8", "replace"),
            b"".join(err_buf).decode("utf-8", "replace"),
            exit_code, before_termios, after_termios)


def main():
    os.makedirs(WORK, exist_ok=True)

    # ---- Scenario 0: successful browse -> q -> exit 0 ----
    log("Scenario 0: successful browse, q exits 0")
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
touch "$VRG_TEST_HANDSHAKE"
exit 0
""")
    raw, stderr, code, bt, at = run_pty(
        [VRG, "hello", "."],
        env={"PATH": fake_dir + ":" + os.environ["PATH"],
             "VRG_TEST_HANDSHAKE": hs},
        keys=["q", "q"], handshake=hs, cwd=repo)
    visible = strip_ansi(raw)
    check("browse exit 0", code == 0, "exit=%d" % code)
    check("browse view shows test.txt", "test.txt" in visible)
    check("browse stderr empty", stderr == "", "stderr=%r" % stderr)

    # ---- Scenario 1: no-results search -> q -> exit 1 ----
    log("Scenario 1: no-results search, q exits 1")
    fake_dir = fresh_dir("s1_fake")
    repo = fresh_dir("s1_repo")
    hs_dir = fresh_dir("s1_hs")
    hs = os.path.join(hs_dir, "handshake")
    with open(os.path.join(repo, "test.txt"), "w") as f:
        f.write("hello\n")
    write_fake_rg(fake_dir, """#!/bin/sh
echo '{"type":"summary","data":{}}'
printf 'warn\\n' >&2
touch "$VRG_TEST_HANDSHAKE"
exit 1
""")
    raw, stderr, code, bt, at = run_pty(
        [VRG, "hello", "."],
        env={"PATH": fake_dir + ":" + os.environ["PATH"],
             "VRG_TEST_HANDSHAKE": hs},
        keys=["\x1b", "q"], handshake=hs, cwd=repo)
    visible = strip_ansi(raw)
    check("no-results exit 1", code == 1, "exit=%d" % code)
    check("no-results shows 'No results found'", "No results found" in visible)
    # Issue #11: collected diagnostics are replayed to stderr after
    # terminal restoration. The fake rg wrote "warn" to stderr, so the
    # replayed stderr must contain it.
    check("no-results stderr replays 'warn'", "warn" in stderr, "stderr=%r" % stderr)

    # ---- Scenario 2: fatal fake-rg, no usable results -> q -> exit 2 ----
    log("Scenario 2: fatal fake-rg no usable results, q exits 2")
    fake_dir = fresh_dir("s2_fake")
    repo = fresh_dir("s2_repo")
    hs_dir = fresh_dir("s2_hs")
    hs = os.path.join(hs_dir, "handshake")
    with open(os.path.join(repo, "test.txt"), "w") as f:
        f.write("hello\n")
    write_fake_rg(fake_dir, """#!/bin/sh
touch "$VRG_TEST_HANDSHAKE"
exit 2
""")
    raw, stderr, code, bt, at = run_pty(
        [VRG, "hello", "."],
        env={"PATH": fake_dir + ":" + os.environ["PATH"],
             "VRG_TEST_HANDSHAKE": hs},
        keys=["q"], handshake=hs, cwd=repo)
    visible = strip_ansi(raw)
    check("fatal q exit 2", code == 2, "exit=%d" % code)
    check("fatal q overlay names exit code", "2" in visible)

    # ---- Scenario 2b: fatal fake-rg, Esc dismissal -> exit 2 ----
    log("Scenario 2b: fatal fake-rg no usable results, Esc exits 2")
    raw, stderr, code, bt, at = run_pty(
        [VRG, "hello", "."],
        env={"PATH": fake_dir + ":" + os.environ["PATH"],
             "VRG_TEST_HANDSHAKE": hs},
        keys=["\x1b"], handshake=hs, cwd=repo)
    check("fatal Esc exit 2", code == 2, "exit=%d" % code)

    # ---- Scenario 130: cancellation while searching -> exit 130 ----
    log("Scenario 130: cancellation while searching, child reaped, terminal restored")
    fake_dir = fresh_dir("s130_fake")
    repo = fresh_dir("s130_repo")
    aux = fresh_dir("s130_aux")
    ready = os.path.join(aux, "ready")
    pid_file = os.path.join(aux, "pid")
    reap_file = os.path.join(aux, "reap")
    with open(os.path.join(repo, "test.txt"), "w") as f:
        f.write("hello\n")
    write_fake_rg(fake_dir, """#!/bin/sh
echo $$ > "$VRG_TEST_PID"
touch "$VRG_TEST_READY"
sleep 100000
""")
    raw, stderr, code, bt, at = run_pty(
        [VRG, "hello", "."],
        env={"PATH": fake_dir + ":" + os.environ["PATH"],
             "VRG_TEST_READY": ready,
             "VRG_TEST_PID": pid_file,
             "VRG_TEST_REAP": reap_file},
        keys=["q"], ready=ready, key_delay=0.3, cwd=repo)
    check("cancel exit 130", code == 130, "exit=%d" % code)
    try:
        with open(pid_file) as f:
            cpid = int(f.read().strip())
        check("child terminated/reaped", not pid_alive(cpid), "pid %d alive" % cpid)
    except Exception as e:
        check("child terminated/reaped", False, "cannot read pid: %s" % e)
    try:
        with open(reap_file) as f:
            reap_data = f.read()
        check("reap evidence present", len(reap_data) > 0)
    except Exception as e:
        check("reap evidence present", False, "cannot read reap: %s" % e)
    check("terminal cursor restored", "\x1b[?25h" in raw)
    if "\x1b[?1049h" in raw:
        check("alt screen exited", "\x1b[?1049l" in raw)
    else:
        check("alt screen exited", True)
    check("termios restored", bt == at)

    # ---- Scenario 130b: cancellation via ctrl+c -> exit 130 ----
    log("Scenario 130b: cancellation via ctrl+c while searching")
    aux2 = fresh_dir("s130b_aux")
    ready2 = os.path.join(aux2, "ready")
    pid_file2 = os.path.join(aux2, "pid")
    reap_file2 = os.path.join(aux2, "reap")
    write_fake_rg(fake_dir, """#!/bin/sh
echo $$ > "$VRG_TEST_PID"
touch "$VRG_TEST_READY"
sleep 100000
""")
    raw, stderr, code, bt, at = run_pty(
        [VRG, "hello", "."],
        env={"PATH": fake_dir + ":" + os.environ["PATH"],
             "VRG_TEST_READY": ready2,
             "VRG_TEST_PID": pid_file2,
             "VRG_TEST_REAP": reap_file2},
        keys=["\x03"], ready=ready2, key_delay=0.3, cwd=repo)
    check("ctrl+c exit 130", code == 130, "exit=%d" % code)

    # ---- Scenario help-only: bare vrg / -h / -- help ----
    log("Scenario help-only: bare vrg, -h, --help (ripgrep unavailable, sentinel fake rg never invoked)")
    help_dir = fresh_dir("help_path")
    sentinel_marker_dir = fresh_dir("help_marker")
    sentinel_marker = os.path.join(sentinel_marker_dir, "rg-ran")
    sentinel = os.path.join(help_dir, "rg")
    with open(sentinel, "w") as f:
        f.write("#!/bin/sh\n: > %s\n" % sentinel_marker)
    os.chmod(sentinel, 0o755)
    for args in ([], ["-h"], ["--help"]):
        proc = subprocess.run([VRG] + args,
                              env={"PATH": help_dir, "TERM": "xterm"},
                              capture_output=True)
        out = proc.stdout.decode("utf-8", "replace")
        err = proc.stderr.decode("utf-8", "replace")
        label = "bare vrg" if not args else " ".join(args)
        check("%s exit 0" % label, proc.returncode == 0, "exit=%d" % proc.returncode)
        check("%s exactly one 'Usage:' on stdout" % label, out.count("Usage:") == 1, "count=%d" % out.count("Usage:"))
        check("%s stderr empty" % label, err == "", "stderr=%r" % err)
        check("%s no terminal control sequences" % label,
              "\x1b[?1049" not in out and "\x9b" not in out)
    check("sentinel fake rg never invoked", not os.path.exists(sentinel_marker),
          "sentinel was invoked")

    # ---- Summary ----
    log("")
    if failures:
        log("SMOKE FAIL: %d failure(s)" % len(failures))
        for f in failures:
            log("  - %s" % f)
        sys.exit(1)
    log("SMOKE OK: all five outcomes verified")


if __name__ == "__main__":
    main()
