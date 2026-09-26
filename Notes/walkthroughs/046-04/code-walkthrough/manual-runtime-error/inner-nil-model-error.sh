#!/bin/sh
cd "/home/chris/vrg/Notes/walkthroughs/046-04/code-walkthrough/manual-runtime-error/fixture"
env PATH="/home/chris/vrg/Notes/walkthroughs/046-04/code-walkthrough/manual-runtime-error/fakebin:$PATH" TERM=xterm-256color   VRG_TEST_COLLECT_ACK="/home/chris/vrg/Notes/walkthroughs/046-04/code-walkthrough/manual-runtime-error/ack-nil-model-error" VRG_TEST_REAP="/home/chris/vrg/Notes/walkthroughs/046-04/code-walkthrough/manual-runtime-error/reap-nil-model-error" VRG_TEST_RUN_FINAL_MODEL=nil VRG_TEST_RUN_ERROR=boom   "/home/chris/vrg/Notes/walkthroughs/046-04/code-walkthrough/vrg-hooks" hit . 2>"/home/chris/vrg/Notes/walkthroughs/046-04/code-walkthrough/manual-runtime-error/err-nil-model-error.txt"
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/046-04/code-walkthrough/manual-runtime-error/exit-nil-model-error.txt"
sleep 3
