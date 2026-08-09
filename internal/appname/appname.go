// Package appname holds the canonical binary/program name to avoid duplicated
// literals across the CLI, TUI, and tests. Changing the program name in one
// place keeps the flag set, header banner, error strings, and integration-test
// screen markers in sync.
package appname

// Name is the user-facing program/binary name.
const Name = "ocs"
