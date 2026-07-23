package opencode

import (
	"errors"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDetect_InPath verifies that when opencode is available on PATH,
// Detect() returns the resolved path without error.
//
// Spec: REQ-OC-001 — Scenario: Happy path — opencode installed
func TestDetect_InPath(t *testing.T) {
	origLookPath := lookPath
	lookPath = func(file string) (string, error) {
		return "/usr/local/bin/opencode", nil
	}
	defer func() { lookPath = origLookPath }()

	path, err := Detect()

	require.NoError(t, err, "Detect should not error when opencode is found")
	assert.Equal(t, "/usr/local/bin/opencode", path, "should return the resolved path")
}

// TestDetect_WindowsExe verifies that on Windows, Go's exec.LookPath
// resolves the .exe extension automatically.
//
// Spec: REQ-OC-001 — Scenario: Edge case — opencode is a .exe on Windows
func TestDetect_WindowsExe(t *testing.T) {
	origLookPath := lookPath
	lookPath = func(file string) (string, error) {
		return `C:\Users\user\scoop\shims\opencode.exe`, nil
	}
	defer func() { lookPath = origLookPath }()

	path, err := Detect()

	require.NoError(t, err)
	assert.Equal(t, `C:\Users\user\scoop\shims\opencode.exe`, path)
}

// TestDetect_NotInPath verifies that when opencode is not on PATH,
// Detect() returns ErrOpencodeNotFound with install instructions.
//
// Spec: REQ-OC-001 — Scenario: Error — opencode not installed
func TestDetect_NotInPath(t *testing.T) {
	origLookPath := lookPath
	lookPath = func(file string) (string, error) {
		return "", &exec.Error{Name: file, Err: exec.ErrNotFound}
	}
	defer func() { lookPath = origLookPath }()

	path, err := Detect()

	require.Error(t, err, "Detect should error when opencode is not found")
	assert.Empty(t, path, "path should be empty on error")
	assert.True(t, errors.Is(err, ErrOpencodeNotFound),
		"error must wrap ErrOpencodeNotFound so callers can check with errors.Is")
	assert.Contains(t, err.Error(), "opencode CLI not found on PATH",
		"error message should describe the problem")
	assert.Contains(t, err.Error(), "Install it from https://opencode.ai",
		"error message should include install instructions")
}
