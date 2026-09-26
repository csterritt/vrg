#!/bin/sh
cd "/home/chris/vrg/Notes/walkthroughs/042-04/code-walkthrough/manual-dropped-r/fixture-a"
VRG_FIXTURE_FILTER=a PATH="/home/chris/vrg/Notes/walkthroughs/042-04/code-walkthrough/manual-dropped-r/fakebin:$PATH" "/home/chris/vrg/Notes/walkthroughs/042-04/code-walkthrough/vrg" hit .
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/042-04/code-walkthrough/manual-dropped-r/exit-startup.txt"
sleep 60
