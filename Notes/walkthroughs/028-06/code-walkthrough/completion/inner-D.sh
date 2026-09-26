#!/bin/sh
cd "/tmp/vrg28.fBTBrk/d"
unset RIPGREP_CONFIG_PATH
"/home/chris/vrg/Notes/walkthroughs/028-06/code-walkthrough/vrg" MARK . 2>"/home/chris/vrg/Notes/walkthroughs/028-06/code-walkthrough/completion/stderr-D.txt"
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/028-06/code-walkthrough/completion/exit-D.txt"
cat "/home/chris/vrg/Notes/walkthroughs/028-06/code-walkthrough/completion/stderr-D.txt"
sleep 30
