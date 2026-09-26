#!/bin/sh
cd "/home/chris/vrg/Notes/walkthroughs/047-04/code-walkthrough/manual-single-line/fixture"
env PATH="/home/chris/vrg/Notes/walkthroughs/047-04/code-walkthrough/manual-single-line/fakebin:$PATH" TERM=xterm-256color   VRG_TEST_GATE="/home/chris/vrg/Notes/walkthroughs/047-04/code-walkthrough/manual-single-line/gate" VRG_TEST_COLLECT_ACK="/home/chris/vrg/Notes/walkthroughs/047-04/code-walkthrough/manual-single-line/ack"   "/home/chris/vrg/Notes/walkthroughs/047-04/code-walkthrough/vrg-hooks" hit . 2>"/home/chris/vrg/Notes/walkthroughs/047-04/code-walkthrough/manual-single-line/err.txt"
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/047-04/code-walkthrough/manual-single-line/exit.txt"
sleep 5
