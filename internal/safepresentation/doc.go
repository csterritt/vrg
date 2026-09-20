// Package safepresentation is the single shared utility that turns
// external bytes — result paths, file content, diagnostics, and
// generated text — into terminal-safe display forms, so raw control
// sequences from searched data never reach the terminal. Original bytes
// always remain the identity, ordering, and filesystem key; escaped text
// is presentation only. Every output sink routes through it: file-list
// entries, the filename rule, panel content, usage errors, generated
// command-line help, and stderr diagnostics.
package safepresentation
