#!/bin/sh
cd "/home/chris/vrg/Notes/walkthroughs/017-06/code-walkthrough/anchor/fixture"
PATH="/home/chris/vrg/Notes/walkthroughs/017-06/code-walkthrough/anchor/fakebin:$PATH" "/home/chris/vrg/Notes/walkthroughs/017-06/code-walkthrough/vrg" hit .
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/017-06/code-walkthrough/anchor/exit.txt"
sleep 30
