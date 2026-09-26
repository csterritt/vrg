#!/bin/sh
cd "/home/chris/vrg/Notes/walkthroughs/023-04/code-walkthrough/markers/fixture"
"/home/chris/vrg/Notes/walkthroughs/023-04/code-walkthrough/vrg" "$" a.txt
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/023-04/code-walkthrough/markers/exit-dollar.txt"
sleep 30
