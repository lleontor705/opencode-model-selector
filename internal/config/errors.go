package config

import "errors"

// Sentinel errors for programmatic error classification via errors.Is.
// Error messages MUST reference paths only, never config values (REQ-CFG-013).
var (
	// ErrConfigNotFound is returned when the config file does not exist at the
	// resolved path.
	ErrConfigNotFound = errors.New("config file not found")

	// ErrBackupFailed is returned when a backup copy of the config cannot be
	// created before a write operation.
	ErrBackupFailed = errors.New("failed to create backup")

	// ErrWriteFailed is returned when the atomic write to the config file fails
	// (temp write or rename).
	ErrWriteFailed = errors.New("failed to write config file")
)
