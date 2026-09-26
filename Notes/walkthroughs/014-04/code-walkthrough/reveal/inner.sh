#!/bin/sh
cd "/home/chris/vrg/Notes/walkthroughs/014-04/code-walkthrough/reveal/fixture"
PATH="/home/chris/vrg/Notes/walkthroughs/014-04/code-walkthrough/reveal/fakebin:$PATH" "/home/chris/vrg/Notes/walkthroughs/014-04/code-walkthrough/vrg" hit .
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/014-04/code-walkthrough/reveal/exit.txt"
sleep 60
