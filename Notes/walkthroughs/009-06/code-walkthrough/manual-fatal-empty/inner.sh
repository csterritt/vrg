#!/bin/sh
cd "/home/chris/vrg/Notes/walkthroughs/009-06/code-walkthrough/manual-fatal-empty/fixture"
PATH="/home/chris/vrg/Notes/walkthroughs/009-06/code-walkthrough/manual-fatal-empty/fakebin:$PATH" "/home/chris/vrg/Notes/walkthroughs/009-06/code-walkthrough/vrg" hit .
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/009-06/code-walkthrough/manual-fatal-empty/exit.txt"
sleep 60
