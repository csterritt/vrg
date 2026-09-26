#!/bin/sh
cd "/home/chris/vrg/Notes/walkthroughs/005-06/code-walkthrough/manual-run/fix"
stty rows 24 cols 80
( sleep 1.0; stty rows 30 cols 100 </dev/tty ) &
"/home/chris/vrg/Notes/walkthroughs/005-06/code-walkthrough/vrg" hit .
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/005-06/code-walkthrough/manual-run/exit.txt"
