#!/bin/sh
cd "/home/chris/vrg/Notes/walkthroughs/018-04/code-walkthrough/pan/fixture"
PATH="/home/chris/vrg/Notes/walkthroughs/018-04/code-walkthrough/pan/fakebin:$PATH" "/home/chris/vrg/Notes/walkthroughs/018-04/code-walkthrough/vrg" hit .
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/018-04/code-walkthrough/pan/exit.txt"
sleep 30
