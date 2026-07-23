// Package opencode handles detection and execution of the opencode CLI binary,
// and parsing of its model output.
package opencode

import (
	"errors"
	"fmt"
	"os/exec"
)

// ErrOpencodeNotFound is returned when the opencode CLI binary cannot be
// located on the system PATH. Callers MUST check with errors.Is.
var ErrOpencodeNotFound = errors.New("opencode CLI not found on PATH")

// lookPath is a package-level indirection over exec.LookPath to enable
// deterministic testing without depending on the host environment.
var lookPath = exec.LookPath

// Detect locates the opencode binary on the system PATH using exec.LookPath.
// Go resolves platform-specific extensions (e.g. .exe on Windows) automatically.
// If the binary is not found, it returns an error wrapping ErrOpencodeNotFound
// with install instructions.
func Detect() (string, error) {
	path, err := lookPath("opencode")
	if err != nil {
		return "", fmt.Errorf("%w. Install it from https://opencode.ai", ErrOpencodeNotFound)
	}
	return path, nil
}
