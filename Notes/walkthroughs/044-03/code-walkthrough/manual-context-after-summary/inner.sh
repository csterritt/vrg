#!/bin/sh
cd "/home/chris/vrg/Notes/walkthroughs/044-03/code-walkthrough/manual-context-after-summary/fixture"
PATH="/home/chris/vrg/Notes/walkthroughs/044-03/code-walkthrough/manual-context-after-summary/fakebin:$PATH" "/home/chris/vrg/Notes/walkthroughs/044-03/code-walkthrough/vrg" hit . 2>"/home/chris/vrg/Notes/walkthroughs/044-03/code-walkthrough/manual-context-after-summary/replay.txt"
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/044-03/code-walkthrough/manual-context-after-summary/exit.txt"
sleep 60
