#!/bin/sh
cd "/tmp/vrg26-fixture.Mp2s3f"
unset RIPGREP_CONFIG_PATH
"/home/chris/vrg/Notes/walkthroughs/026-06/code-walkthrough/vrg" MARK . 2>"/home/chris/vrg/Notes/walkthroughs/026-06/code-walkthrough/fail/stderr.txt"
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/026-06/code-walkthrough/fail/exit.txt"
cat "/home/chris/vrg/Notes/walkthroughs/026-06/code-walkthrough/fail/stderr.txt"
sleep 30
