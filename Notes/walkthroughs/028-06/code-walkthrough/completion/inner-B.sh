#!/bin/sh
cd "/tmp/vrg28.fBTBrk/b"
unset RIPGREP_CONFIG_PATH
"/home/chris/vrg/Notes/walkthroughs/028-06/code-walkthrough/vrg" MARK . 2>"/home/chris/vrg/Notes/walkthroughs/028-06/code-walkthrough/completion/stderr-B.txt"
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/028-06/code-walkthrough/completion/exit-B.txt"
cat "/home/chris/vrg/Notes/walkthroughs/028-06/code-walkthrough/completion/stderr-B.txt"
sleep 30
