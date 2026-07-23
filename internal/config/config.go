// Package config provides JSON config management for opencode: path resolution,
// loading, atomic saving, and field access via map[string]interface{}.
//
// The map-based JSON strategy preserves ALL unknown/future fields automatically,
// which is critical for MCP API keys, permissions, and any fields the tool does
// not explicitly model.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Config wraps the parsed config data and the filesystem path it was loaded from
// (or will be saved to).
type Config struct {
	path string
	data map[string]interface{}
}

// Data returns the raw parsed config map. Callers MUST NOT mutate the returned
// map without understanding that changes are reflected in the Config.
func (c *Config) Data() map[string]interface{} {
	return c.data
}

// Path returns the filesystem path associated with this Config.
func (c *Config) Path() string {
	return c.path
}

// LoadConfig reads the JSON config file at the given path and unmarshals it into
// a Config wrapping map[string]interface{}.
//
// Returns ErrConfigNotFound (wrapping the path) when the file does not exist.
// Returns a JSON parse error when the file exists but contains invalid JSON.
// Error messages reference the path only, never config values (REQ-CFG-013).
func LoadConfig(path string) (*Config, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: %s", ErrConfigNotFound, path)
		}
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(content, &data); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}

	return &Config{path: path, data: data}, nil
}

// Save writes the config data atomically to the configured path.
//
// The write is atomic: data is marshalled with 2-space indentation, written to a
// temporary file (path + ".tmp") with 0o600 permissions, then os.Rename replaces
// the target. If the rename fails the temp file is cleaned up and the original
// file remains untouched.
//
// Returns ErrWriteFailed (wrapping the path) on any write or rename failure.
func (c *Config) Save() error {
	data, err := json.MarshalIndent(c.data, "", "  ")
	if err != nil {
		return fmt.Errorf("%w: marshal %s", ErrWriteFailed, c.path)
	}

	tmpFile := c.path + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0o600); err != nil {
		return fmt.Errorf("%w: write temp %s", ErrWriteFailed, c.path)
	}

	if err := os.Rename(tmpFile, c.path); err != nil {
		// Best-effort cleanup of the temp file on rename failure.
		_ = os.Remove(tmpFile)
		return fmt.Errorf("%w: rename %s", ErrWriteFailed, c.path)
	}

	return nil
}

// GetConfigPath resolves the config file path.
//
// If override is non-empty it is returned verbatim (flag or env override).
// Otherwise the default path is constructed as
// filepath.Join(os.UserHomeDir(), ".config", "opencode", "opencode.json").
//
// os.UserHomeDir() is used — NOT os.UserConfigDir() — because opencode uses an
// XDG-style ".config/opencode/" path on ALL platforms, while os.UserConfigDir()
// returns %AppData% on Windows (wrong for opencode).
func GetConfigPath(override string) (string, error) {
	if override != "" {
		return override, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}

	return filepath.Join(home, ".config", "opencode", "opencode.json"), nil
}
