#!/bin/sh
cd "/home/chris/vrg/Notes/walkthroughs/042-04/code-walkthrough/manual-dropped-r/fixture-b"
VRG_FIXTURE_FILTER=b PATH="/home/chris/vrg/Notes/walkthroughs/042-04/code-walkthrough/manual-dropped-r/fakebin:$PATH" "/home/chris/vrg/Notes/walkthroughs/042-04/code-walkthrough/vrg" hit .
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/042-04/code-walkthrough/manual-dropped-r/exit-navigation.txt"
sleep 60
