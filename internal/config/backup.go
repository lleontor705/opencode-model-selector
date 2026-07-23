package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// backupGlobPattern matches existing backup files in a config directory.
const backupGlobPattern = "opencode.json.backup.*"

// backupTimestampFormat is the timestamp layout embedded in backup filenames.
// It is chosen so that lexicographic sort order matches chronological order.
const backupTimestampFormat = "20060102-150405"

// CreateBackup creates a timestamped backup of the config file in the same
// directory as configPath. The backup is named
// "opencode.json.backup.{YYYYMMDD-HHMMSS}" and is a byte-for-byte copy of the
// source written with 0o600 permissions on Unix (REQ-CFG-009).
//
// Returns the path of the created backup file, or an error wrapping
// ErrBackupFailed on read or write failure.
func CreateBackup(configPath string) (string, error) {
	content, err := os.ReadFile(configPath)
	if err != nil {
		return "", fmt.Errorf("%w: read source %s", ErrBackupFailed, configPath)
	}

	dir := filepath.Dir(configPath)
	timestamp := time.Now().Format(backupTimestampFormat)
	backupPath := filepath.Join(dir, fmt.Sprintf("opencode.json.backup.%s", timestamp))

	if err := os.WriteFile(backupPath, content, 0o600); err != nil {
		return "", fmt.Errorf("%w: write %s", ErrBackupFailed, backupPath)
	}

	return backupPath, nil
}

// CleanOldBackups removes backup files beyond the retention count. Backups are
// matched via "opencode.json.backup.*" in the same directory as configPath,
// sorted lexicographically (timestamps sort correctly), and the N most recent
// are kept while the rest are deleted (REQ-CFG-010).
//
// A retention value of 0 means skip: no backups are deleted. If fewer backups
// exist than the retention count, or no backups exist at all, CleanOldBackups
// returns nil without modifying anything.
func CleanOldBackups(configPath string, keep int) error {
	// keep == 0 means skip entirely — do NOT delete anything.
	if keep == 0 {
		return nil
	}

	dir := filepath.Dir(configPath)
	pattern := filepath.Join(dir, backupGlobPattern)
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("%w: glob %s", ErrBackupFailed, pattern)
	}

	// Nothing to do if no backups or already within retention.
	if len(matches) == 0 || len(matches) <= keep {
		return nil
	}

	// Sort lexicographically: the timestamp format guarantees that
	// lexicographic order matches chronological order, so the most recent
	// backups are at the end of the slice.
	sort.Strings(matches)

	// Delete the oldest entries; keep the last `keep`.
	for _, p := range matches[:len(matches)-keep] {
		if err := os.Remove(p); err != nil {
			return fmt.Errorf("%w: remove %s", ErrBackupFailed, p)
		}
	}

	return nil
}
