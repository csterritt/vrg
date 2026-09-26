#!/bin/sh
cd "/home/chris/vrg/Notes/walkthroughs/016-04/code-walkthrough/wrap/fixture"
PATH="/home/chris/vrg/Notes/walkthroughs/016-04/code-walkthrough/wrap/fakebin:$PATH" "/home/chris/vrg/Notes/walkthroughs/016-04/code-walkthrough/vrg" hit .
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/016-04/code-walkthrough/wrap/exit.txt"
sleep 60
