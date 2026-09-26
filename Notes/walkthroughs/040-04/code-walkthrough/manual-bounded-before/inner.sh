#!/bin/sh
cd "/home/chris/vrg/Notes/walkthroughs/040-04/code-walkthrough/manual-bounded-before/fixture"
PATH="/home/chris/vrg/Notes/walkthroughs/040-04/code-walkthrough/manual-bounded-before/fakebin:$PATH" "/home/chris/vrg/Notes/walkthroughs/040-04/code-walkthrough/vrg-before" hit . 2>"/home/chris/vrg/Notes/walkthroughs/040-04/code-walkthrough/manual-bounded-before/replay.txt"
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/040-04/code-walkthrough/manual-bounded-before/exit.txt"
sleep 60
