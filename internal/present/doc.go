// Package present is the shared safe-presentation utility: every output
// sink — file content, filenames in the list, rule, pop-ups, and
// diagnostics, help substitutions, usage errors, and stderr replay —
// renders external data through it so raw control sequences from
// searched data never reach the terminal.
//
// Three contracts cover the sink classes. Path renders raw path bytes
// as a single safe display line while the original bytes remain the key
// for identity, ordering, and file access. LineOf presents one source
// line as display cells with a byte→cell map so highlights cover every
// cell an escaped form produces. Diagnostic renders diagnostic text,
// preserving the message's own line boundaries and expanding tabs, with
// embedded filenames passed through Path first so they stay
// single-lined.
package present
