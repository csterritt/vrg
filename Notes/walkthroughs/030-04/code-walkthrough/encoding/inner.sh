#!/bin/sh
cd "/tmp/vrg30-fixture.Jad1j0"
unset RIPGREP_CONFIG_PATH
"/home/chris/vrg/Notes/walkthroughs/030-04/code-walkthrough/vrg" hi . 2>"/home/chris/vrg/Notes/walkthroughs/030-04/code-walkthrough/encoding/stderr.txt"
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/030-04/code-walkthrough/encoding/exit.txt"
cat "/home/chris/vrg/Notes/walkthroughs/030-04/code-walkthrough/encoding/stderr.txt"
sleep 30
