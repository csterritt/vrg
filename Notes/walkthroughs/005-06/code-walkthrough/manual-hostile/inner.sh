#!/bin/sh
cd "/home/chris/vrg/Notes/walkthroughs/005-06/code-walkthrough/manual-hostile/fix"
stty rows 24 cols 80
"/home/chris/vrg/Notes/walkthroughs/005-06/code-walkthrough/vrg" hit .
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/005-06/code-walkthrough/manual-hostile/exit.txt"
