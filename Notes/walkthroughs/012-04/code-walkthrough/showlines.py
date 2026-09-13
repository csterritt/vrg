#!/usr/bin/env python3
"""Run vrg with a PTY, send keys, strip ANSI, and extract visible line
numbers from the last rendered frame. Line numbers are 2-digit (01-60)
so we can split concatenated output on the 'line ' prefix."""
import os, sys, subprocess, re

keys = sys.argv[1] if len(sys.argv) > 1 else "q"
env = os.environ.copy()
env["VRG_KEYS"] = keys
env["VRG_DELAY"] = "0.3"
env["VRG_WIDTH"] = "80"
env["VRG_HEIGHT"] = "24"
env["VRG_RAW"] = "1"
env["VRG_STDERR"] = "stderr.txt"
env["PATH"] = os.path.dirname(os.path.abspath(__file__)) + "/fakebin:" + env.get("PATH", "")

result = subprocess.run(
    ["python3", "runpty.py", "./vrg", "match", "."],
    capture_output=True, text=True, env=env, cwd=os.path.dirname(os.path.abspath(__file__))
)
text = result.stdout
# Strip ANSI sequences (including private mode).
text = re.sub(r'\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)', '', text)  # OSC
text = re.sub(r'\x1b\[[?>]?[0-9;]*[A-Za-z]', '', text)  # CSI
text = re.sub(r'\x1b[()][0B]', '', text)  # charset
text = re.sub(r'\x1b[=>]', '', text)  # keypad
text = text.replace('\r', '')
# Extract all "line NN" patterns where NN is 2 digits.
matches = re.findall(r'line (\d{2})', text)
# The last 23 matches belong to the last frame (contentHeight = 23).
last_frame = matches[-23:] if len(matches) > 23 else matches
for ln in sorted(set(last_frame), key=int):
    print(f"line {ln}")
