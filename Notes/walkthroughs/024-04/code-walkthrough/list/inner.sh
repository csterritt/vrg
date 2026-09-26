#!/bin/sh
cd "/home/chris/vrg/Notes/walkthroughs/024-04/code-walkthrough/list/fixture"
unset RIPGREP_CONFIG_PATH
"/home/chris/vrg/Notes/walkthroughs/024-04/code-walkthrough/vrg" MARK .
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/024-04/code-walkthrough/list/exit.txt"
sleep 30
