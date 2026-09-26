#!/bin/sh
cd "/home/chris/vrg/Notes/walkthroughs/038-04/code-walkthrough/manual-panel-width/fixture"
PATH="/home/chris/vrg/Notes/walkthroughs/038-04/code-walkthrough/manual-panel-width/fakebin:$PATH" "/home/chris/vrg/Notes/walkthroughs/038-04/code-walkthrough/vrg" needle . 2>"/home/chris/vrg/Notes/walkthroughs/038-04/code-walkthrough/manual-panel-width/replay.txt"
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/038-04/code-walkthrough/manual-panel-width/exit.txt"
sleep 60
