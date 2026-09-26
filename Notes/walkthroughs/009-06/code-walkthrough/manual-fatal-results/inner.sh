#!/bin/sh
cd "/home/chris/vrg/Notes/walkthroughs/009-06/code-walkthrough/manual-fatal-results/fixture"
PATH="/home/chris/vrg/Notes/walkthroughs/009-06/code-walkthrough/manual-fatal-results/fakebin:$PATH" "/home/chris/vrg/Notes/walkthroughs/009-06/code-walkthrough/vrg" hit .
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/009-06/code-walkthrough/manual-fatal-results/exit.txt"
sleep 60
