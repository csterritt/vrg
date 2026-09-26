#!/bin/sh
cd "/home/chris/vrg/Notes/walkthroughs/037-04/code-walkthrough/manual-dedup/fixture"
PATH="/home/chris/vrg/Notes/walkthroughs/037-04/code-walkthrough/manual-dedup/fakebin:$PATH" "/home/chris/vrg/Notes/walkthroughs/037-04/code-walkthrough/vrg" hit . 2>"/home/chris/vrg/Notes/walkthroughs/037-04/code-walkthrough/manual-dedup/replay.txt"
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/037-04/code-walkthrough/manual-dedup/exit.txt"
sleep 60
