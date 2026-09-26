#!/bin/sh
cd "/home/chris/vrg/Notes/walkthroughs/045-04/code-walkthrough/manual-test-hooks/fixture"
env PATH="/home/chris/vrg/Notes/walkthroughs/045-04/code-walkthrough/manual-test-hooks/fakebin-block:$PATH" TERM=xterm-256color   VRG_TEST_COLLECT_ACK="/home/chris/vrg/Notes/walkthroughs/045-04/code-walkthrough/manual-test-hooks/ack-nil-model" VRG_TEST_RUN_FINAL_MODEL=nil   "/home/chris/vrg/Notes/walkthroughs/045-04/code-walkthrough/vrg-hooks" hit . 2>"/home/chris/vrg/Notes/walkthroughs/045-04/code-walkthrough/manual-test-hooks/err-nil-model.txt"
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/045-04/code-walkthrough/manual-test-hooks/exit-nil-model.txt"
sleep 3
