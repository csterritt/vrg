#!/bin/sh
cd "/home/chris/vrg/Notes/walkthroughs/036-06/code-walkthrough/manual-damaged/fixture"
PATH="/home/chris/vrg/Notes/walkthroughs/036-06/code-walkthrough/manual-damaged/fakebin:$PATH" "/home/chris/vrg/Notes/walkthroughs/036-06/code-walkthrough/vrg" hit . 2>"/home/chris/vrg/Notes/walkthroughs/036-06/code-walkthrough/manual-damaged/replay.txt"
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/036-06/code-walkthrough/manual-damaged/exit.txt"
sleep 60
