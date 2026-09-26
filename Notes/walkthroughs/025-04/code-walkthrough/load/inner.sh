#!/bin/sh
cd "/home/chris/vrg/Notes/walkthroughs/025-04/code-walkthrough/load/fixture"
unset RIPGREP_CONFIG_PATH
"/home/chris/vrg/Notes/walkthroughs/025-04/code-walkthrough/vrg" MARK .
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/025-04/code-walkthrough/load/exit.txt"
sleep 30
