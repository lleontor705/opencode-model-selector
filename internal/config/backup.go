package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// backupGlobFor returns the backup glob pattern for a given config path,
// e.g. "opencode.jsonc" -> "opencode.jsonc.backup.*". This keeps cleanup
// scoped to the same config basename (extension included) so .json and .jsonc
// backup sets never collide. For "opencode.json" it yields the legacy
// "opencode.json.backup.*" pattern, preserving backward compatibility.
func backupGlobFor(configPath string) string {
	return filepath.Base(configPath) + ".backup.*"
}

// backupTimestampFormat is the timestamp layout embedded in backup filenames.
// It is chosen so that lexicographic sort order matches chronological order.
const backupTimestampFormat = "20060102-150405"

// CreateBackup creates a timestamped backup of the config file in the same
// directory as configPath. The backup is named
// "<configBase>.backup.{YYYYMMDD-HHMMSS}" — where <configBase> is the full
// basename of the config path INCLUDING extension (e.g. "opencode.json" or
// "opencode.jsonc") — and is a byte-for-byte copy of the source written with
// 0o600 permissions on Unix (REQ-CFG-009).
//
// Deriving the backup name from the config basename keeps legacy
// "opencode.json.backup.*" behavior identical for .json configs while
// extending naturally to .jsonc.
//
// Returns the path of the created backup file, or an error wrapping
// ErrBackupFailed on read or write failure.
func CreateBackup(configPath string) (string, error) {
	content, err := os.ReadFile(configPath)
	if err != nil {
		return "", fmt.Errorf("%w: read source %s", ErrBackupFailed, configPath)
	}

	dir := filepath.Dir(configPath)
	base := filepath.Base(configPath)
	timestamp := time.Now().Format(backupTimestampFormat)
	backupPath := filepath.Join(dir, fmt.Sprintf("%s.backup.%s", base, timestamp))

	if err := os.WriteFile(backupPath, content, 0o600); err != nil {
		return "", fmt.Errorf("%w: write %s", ErrBackupFailed, backupPath)
	}

	return backupPath, nil
}

// CleanOldBackups removes backup files beyond the retention count. Backups are
// matched via "<configBase>.backup.*" (derived from configPath's basename) in
// the same directory as configPath, sorted lexicographically (timestamps sort
// correctly), and the N most recent are kept while the rest are deleted
// (REQ-CFG-010).
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
	pattern := filepath.Join(dir, backupGlobFor(configPath))
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
