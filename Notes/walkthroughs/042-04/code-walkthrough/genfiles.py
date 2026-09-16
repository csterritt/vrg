#!/usr/bin/env python3
"""Generate the demo repo and fake rg for the Issue #42 walkthrough.

Creates:
  demo/slow.txt   - a FIFO standing in for a slow-loading file: vrg's
                    os.ReadFile blocks on open until a writer supplies
                    the content, letting the walkthrough hold the
                    startup load in flight while 'r' is pressed
  fakerg/rg       - fake ripgrep: emits valid stdout JSON records for
                    slow.txt (one match at line 40), touches the
                    handshake file, exits 0
"""
import os, stat

here = os.path.dirname(os.path.abspath(__file__))
demo = os.path.join(here, "demo")
fake = os.path.join(here, "fakerg")
os.makedirs(demo, exist_ok=True)
os.makedirs(fake, exist_ok=True)

fifo = os.path.join(demo, "slow.txt")
try:
    os.remove(fifo)
except FileNotFoundError:
    pass
os.mkfifo(fifo)

rg = r"""#!/bin/sh
echo '{"type":"begin","data":{"path":{"text":"slow.txt"}}}'
printf '%s\n' '{"type":"match","data":{"path":{"text":"slow.txt"},"lines":{"text":"line-40 MATCH-TARGET\n"},"line_number":40,"submatches":[{"match":{"text":"MATCH-TARGET"},"start":8,"end":20}],"absolute_offset":0}}'
echo '{"type":"end","data":{"path":{"text":"slow.txt"},"binary_offset":null}}'
echo '{"type":"summary","data":{}}'
touch "$VRG_TEST_HANDSHAKE"
exit 0
"""
path = os.path.join(fake, "rg")
with open(path, "w") as f:
    f.write(rg)
os.chmod(path, os.stat(path).st_mode | stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH)

print("created demo/slow.txt (FIFO) and fakerg/rg (match at line 40, exit 0)")
