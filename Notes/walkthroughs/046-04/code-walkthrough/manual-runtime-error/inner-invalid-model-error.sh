#!/bin/sh
cd "/home/chris/vrg/Notes/walkthroughs/046-04/code-walkthrough/manual-runtime-error/fixture"
env PATH="/home/chris/vrg/Notes/walkthroughs/046-04/code-walkthrough/manual-runtime-error/fakebin:$PATH" TERM=xterm-256color   VRG_TEST_COLLECT_ACK="/home/chris/vrg/Notes/walkthroughs/046-04/code-walkthrough/manual-runtime-error/ack-invalid-model-error" VRG_TEST_REAP="/home/chris/vrg/Notes/walkthroughs/046-04/code-walkthrough/manual-runtime-error/reap-invalid-model-error" VRG_TEST_RUN_FINAL_MODEL=invalid VRG_TEST_RUN_ERROR=boom   "/home/chris/vrg/Notes/walkthroughs/046-04/code-walkthrough/vrg-hooks" hit . 2>"/home/chris/vrg/Notes/walkthroughs/046-04/code-walkthrough/manual-runtime-error/err-invalid-model-error.txt"
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/046-04/code-walkthrough/manual-runtime-error/exit-invalid-model-error.txt"
sleep 3
