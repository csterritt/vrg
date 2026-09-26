#!/bin/sh
cd "/tmp/vrg29.fn7NxJ/a"
unset RIPGREP_CONFIG_PATH
"/home/chris/vrg/Notes/walkthroughs/029-06/code-walkthrough/vrg" MARK . 2>"/home/chris/vrg/Notes/walkthroughs/029-06/code-walkthrough/stale/stderr-A.txt"
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/029-06/code-walkthrough/stale/exit-A.txt"
cat "/home/chris/vrg/Notes/walkthroughs/029-06/code-walkthrough/stale/stderr-A.txt"
sleep 30
