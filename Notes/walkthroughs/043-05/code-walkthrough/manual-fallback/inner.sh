#!/bin/sh
cd "/home/chris/vrg/Notes/walkthroughs/043-05/code-walkthrough/manual-fallback/work"
PATH="/home/chris/vrg/Notes/walkthroughs/043-05/code-walkthrough/manual-fallback/fakebin:$PATH" "/home/chris/vrg/Notes/walkthroughs/043-05/code-walkthrough/vrg" "◌́|fe◌́" .
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/043-05/code-walkthrough/manual-fallback/exit.txt"
sleep 60
