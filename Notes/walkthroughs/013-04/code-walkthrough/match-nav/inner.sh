#!/bin/sh
cd "/home/chris/vrg/Notes/walkthroughs/013-04/code-walkthrough/match-nav/fixture"
PATH="/home/chris/vrg/Notes/walkthroughs/013-04/code-walkthrough/match-nav/fakebin:$PATH" "/home/chris/vrg/Notes/walkthroughs/013-04/code-walkthrough/vrg" hit .
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/013-04/code-walkthrough/match-nav/exit.txt"
sleep 60
