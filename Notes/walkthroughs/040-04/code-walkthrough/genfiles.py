#!/usr/bin/env python3
"""Generate the large-result-set fixture for the Issue #40 walkthrough.

Creates demo/src/fileNNNNN.txt with LINES lines each, every line
containing the pattern 'needle', so a real `rg --json needle` run
produces FILES*LINES matched stops across FILES distinct files.
"""
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
SRC = os.path.join(HERE, os.environ.get("VRG_DEMO", "demo"), "src")

FILES = int(os.environ.get("VRG_FILES", "5000"))
LINES = int(os.environ.get("VRG_LINES", "20"))

os.makedirs(SRC, exist_ok=True)
for f in range(FILES):
    path = os.path.join(SRC, f"file{f:05d}.txt")
    with open(path, "w") as fh:
        for i in range(LINES):
            fh.write(f"row {i:03d} needle tail f{f:05d}\n")

print(f"created {FILES} files x {LINES} matched lines = {FILES*LINES} stops in {SRC}")
