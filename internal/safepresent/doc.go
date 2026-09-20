// Package safepresent turns external bytes — result paths and file
// content — into terminal-safe display forms. Original bytes always
// remain the identity, ordering, and filesystem key; escaped text is
// presentation only. Issue 6 generalizes this core into the shared
// all-sink utility.
package safepresent
