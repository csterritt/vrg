#!/bin/sh
cd "/home/chris/vrg/Notes/walkthroughs/020-04/code-walkthrough/indicators/fixture"
PATH="/home/chris/vrg/Notes/walkthroughs/020-04/code-walkthrough/indicators/fakebin:$PATH" "/home/chris/vrg/Notes/walkthroughs/020-04/code-walkthrough/vrg" hit .
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/020-04/code-walkthrough/indicators/exit.txt"
sleep 30
