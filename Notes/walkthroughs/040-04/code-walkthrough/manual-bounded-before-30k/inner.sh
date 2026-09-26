#!/bin/sh
cd "/home/chris/vrg/Notes/walkthroughs/040-04/code-walkthrough/manual-bounded-before-30k/fixture"
PATH="/home/chris/vrg/Notes/walkthroughs/040-04/code-walkthrough/manual-bounded-before-30k/fakebin:$PATH" "/home/chris/vrg/Notes/walkthroughs/040-04/code-walkthrough/vrg-before" hit . 2>"/home/chris/vrg/Notes/walkthroughs/040-04/code-walkthrough/manual-bounded-before-30k/replay.txt"
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/040-04/code-walkthrough/manual-bounded-before-30k/exit.txt"
sleep 60
