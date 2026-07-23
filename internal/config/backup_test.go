package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fmtTimestamp returns a YYYYMMDD-HHMMSS timestamp offset by i seconds from a
// fixed base, so test backups have distinct, lexicographically-sortable names.
func fmtTimestamp(i int) string {
	base := time.Date(2025, 1, 1, 0, 0, i, 0, time.UTC)
	return base.Format("20060102-150405")
}

// ---------------------------------------------------------------------------
// CreateBackup (REQ-CFG-009)
// ---------------------------------------------------------------------------

func TestCreateBackup_CreatesTimestampedCopy(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "opencode.json")
	original := []byte(`{"$schema":"test","agent":{}}`)
	require.NoError(t, os.WriteFile(srcPath, original, 0o600))

	backupPath, err := CreateBackup(srcPath)
	require.NoError(t, err)

	// Backup is in the same directory as the source.
	assert.Equal(t, dir, filepath.Dir(backupPath),
		"backup must live in the same directory as the source")

	// Name follows pattern opencode.json.backup.{YYYYMMDD-HHMMSS}.
	base := filepath.Base(backupPath)
	assert.True(t, strings.HasPrefix(base, "opencode.json.backup."),
		"backup name must start with opencode.json.backup., got %q", base)

	suffix := strings.TrimPrefix(base, "opencode.json.backup.")
	_, parseErr := time.Parse("20060102-150405", suffix)
	assert.NoError(t, parseErr,
		"backup suffix must be YYYYMMDD-HHMMSS, got %q", suffix)

	// Content is byte-for-byte identical.
	content, err := os.ReadFile(backupPath)
	require.NoError(t, err)
	assert.Equal(t, original, content, "backup content must equal source content")
}

func TestCreateBackup_FilePermissionsUnix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("0o600 permission bits are not enforced on Windows")
	}

	dir := t.TempDir()
	srcPath := filepath.Join(dir, "opencode.json")
	require.NoError(t, os.WriteFile(srcPath, []byte(`{}`), 0o600))

	backupPath, err := CreateBackup(srcPath)
	require.NoError(t, err)

	info, err := os.Stat(backupPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(),
		"backup file must have 0o600 permissions on Unix")
}

func TestCreateBackup_SourceMissingReturnsError(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist.json")
	_, err := CreateBackup(missing)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrBackupFailed)
}

// ---------------------------------------------------------------------------
// CleanOldBackups (REQ-CFG-010)
// ---------------------------------------------------------------------------

func TestCleanOldBackups_KeepsNMostRecent(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "opencode.json")
	require.NoError(t, os.WriteFile(srcPath, []byte(`{}`), 0o600))

	// Create 7 backups with distinct, sortable timestamps.
	for i := 0; i < 7; i++ {
		name := filepath.Join(dir, "opencode.json.backup."+fmtTimestamp(i))
		require.NoError(t, os.WriteFile(name, []byte(`{}`), 0o600))
	}

	require.NoError(t, CleanOldBackups(srcPath, 5))

	remaining := countBackups(t, dir)
	assert.Equal(t, 5, remaining, "must keep exactly 5 backups when keep=5 and 7 exist")
}

func TestCleanOldBackups_KeepsMostRecentOnes(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "opencode.json")
	require.NoError(t, os.WriteFile(srcPath, []byte(`{}`), 0o600))

	// Create 3 backups: timestamps 0, 1, 2.
	for i := 0; i < 3; i++ {
		name := filepath.Join(dir, "opencode.json.backup."+fmtTimestamp(i))
		require.NoError(t, os.WriteFile(name, []byte(`{}`), 0o600))
	}

	// Keep only the most recent (timestamp 2).
	require.NoError(t, CleanOldBackups(srcPath, 1))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "opencode.json.backup.") {
			continue
		}
		assert.True(t, strings.HasSuffix(e.Name(), fmtTimestamp(2)),
			"only the most recent backup must remain, got %q", e.Name())
	}
}

func TestCleanOldBackups_ZeroKeepsAll(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "opencode.json")
	require.NoError(t, os.WriteFile(srcPath, []byte(`{}`), 0o600))

	for i := 0; i < 3; i++ {
		name := filepath.Join(dir, "opencode.json.backup."+fmtTimestamp(i))
		require.NoError(t, os.WriteFile(name, []byte(`{}`), 0o600))
	}

	require.NoError(t, CleanOldBackups(srcPath, 0),
		"keep=0 means skip — must NOT delete any backups")

	remaining := countBackups(t, dir)
	assert.Equal(t, 3, remaining, "keep=0 must preserve all backups")
}

func TestCleanOldBackups_NoBackupsExists(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "opencode.json")
	require.NoError(t, os.WriteFile(srcPath, []byte(`{}`), 0o600))

	// No backups exist — must return nil, nothing happens.
	require.NoError(t, CleanOldBackups(srcPath, 5))
}

func TestCleanOldBackups_FewerThanKeep(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "opencode.json")
	require.NoError(t, os.WriteFile(srcPath, []byte(`{}`), 0o600))

	// Create 2 backups, ask to keep 5 — nothing should be deleted.
	for i := 0; i < 2; i++ {
		name := filepath.Join(dir, "opencode.json.backup."+fmtTimestamp(i))
		require.NoError(t, os.WriteFile(name, []byte(`{}`), 0o600))
	}

	require.NoError(t, CleanOldBackups(srcPath, 5))
	remaining := countBackups(t, dir)
	assert.Equal(t, 2, remaining, "must not delete when count < keep")
}

// countBackups counts files in dir matching the backup glob pattern.
func countBackups(t *testing.T, dir string) int {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	count := 0
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "opencode.json.backup.") {
			count++
		}
	}
	return count
}
