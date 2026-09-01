package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// DiscoverMarkdownAgents / loadMarkdownAgentsDir (T-B3)
//
// Discovery scans two layers: global (~/.config/opencode/agents/*.md) and
// project (.opencode/agents/*.md). Missing dirs are non-fatal; per-file parse
// errors are logged and skipped. Name = file stem. The two layers are returned
// separately so the merge step (T-B4) can apply per-field precedence.
// ---------------------------------------------------------------------------

// setHomeEnv is shared with config_test.go (sets HOME + USERPROFILE).

// writeAgentMD writes a markdown agent file named name into dir.
func writeAgentMD(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	return p
}

// TestDiscover_MissingDirsNonFatal verifies that absent global and project
// directories produce two empty (non-nil) maps and no panic.
func TestDiscover_MissingDirsNonFatal(t *testing.T) {
	setHomeEnv(t, t.TempDir())
	projectDir := filepath.Join(t.TempDir(), "does", "not", "exist")

	global, project := DiscoverMarkdownAgents(projectDir)
	assert.NotNil(t, global, "global map must be non-nil even when empty")
	assert.NotNil(t, project, "project map must be non-nil even when empty")
	assert.Empty(t, global)
	assert.Empty(t, project)
}

// TestDiscover_NameIsStem verifies the agent name is the file stem (minus .md).
func TestDiscover_NameIsStem(t *testing.T) {
	setHomeEnv(t, t.TempDir())
	projectDir := t.TempDir()
	writeAgentMD(t, projectDir, "build.md", "---\nmode: primary\nmodel: m1\n---\nBody.\n")

	_, project := DiscoverMarkdownAgents(projectDir)
	require.Contains(t, project, "build")
	assert.Equal(t, "build", project["build"].Name, "Name must be the file stem")
	assert.Equal(t, "primary", project["build"].Mode())
}

// TestDiscover_OnlyMDFiles verifies non-.md files are ignored.
func TestDiscover_OnlyMDFiles(t *testing.T) {
	setHomeEnv(t, t.TempDir())
	projectDir := t.TempDir()
	writeAgentMD(t, projectDir, "real.md", "---\nmodel: m\n---\nB.\n")
	writeAgentMD(t, projectDir, "notes.txt", "ignore me")
	writeAgentMD(t, projectDir, "README.md.swp", "ignore me too")

	_, project := DiscoverMarkdownAgents(projectDir)
	assert.Contains(t, project, "real")
	assert.Len(t, project, 1, "only *.md files must be discovered")
}

// TestDiscover_ReturnsBothLayers verifies the global and project layers are
// returned separately (no cross-contamination).
func TestDiscover_ReturnsBothLayers(t *testing.T) {
	home := t.TempDir()
	setHomeEnv(t, home)
	globalDir := filepath.Join(home, ".config", "opencode", "agents")
	require.NoError(t, os.MkdirAll(globalDir, 0o755))
	writeAgentMD(t, globalDir, "global-agent.md", "---\nmodel: gm\n---\nGlobal.\n")

	projectDir := t.TempDir()
	writeAgentMD(t, projectDir, "project-agent.md", "---\nmodel: pm\n---\nProject.\n")

	global, project := DiscoverMarkdownAgents(projectDir)
	assert.Contains(t, global, "global-agent")
	assert.Contains(t, project, "project-agent")
	assert.NotContains(t, global, "project-agent", "project agent must not leak into global")
	assert.NotContains(t, project, "global-agent", "global agent must not leak into project")
}

// TestDiscover_SkipsOnHardIOError verifies that when the parse function returns
// a hard I/O error for one file, discovery skips it (logs) and still returns
// the other valid agents. Deterministic via the unexported mdFileParse
// indirection (same pattern as the opencode lookPath variable).
func TestDiscover_SkipsOnHardIOError(t *testing.T) {
	setHomeEnv(t, t.TempDir())
	projectDir := t.TempDir()
	writeAgentMD(t, projectDir, "good.md", "---\nmodel: m\n---\nGood.\n")
	writeAgentMD(t, projectDir, "bad.md", "---\nmodel: m\n---\nBad.\n")

	orig := mdFileParse
	t.Cleanup(func() { mdFileParse = orig })
	mdFileParse = func(path string) (MDAgent, error) {
		if strings.HasSuffix(filepath.ToSlash(path), "bad.md") {
			return MDAgent{}, errors.New("simulated hard IO error")
		}
		return ParseMarkdownAgentFile(path)
	}

	require.NotPanics(t, func() {
		_, project := DiscoverMarkdownAgents(projectDir)
		assert.Contains(t, project, "good", "the valid file must still be discovered")
		_, present := project["bad"]
		assert.False(t, present, "the file whose parse errored must be skipped")
	})
}

// TestDiscover_RealIOErrorOnDirectorySkips is a non-injected sanity check: a
// filesystem entry that os.ReadFile cannot read (a directory named *.md) is
// skipped without panicking. Cross-platform best-effort.
func TestDiscover_RealIOErrorOnDirectorySkips(t *testing.T) {
	if testing.Short() {
		t.Skip("filesystem-dependent IO-error check")
	}
	setHomeEnv(t, t.TempDir())
	projectDir := t.TempDir()
	writeAgentMD(t, projectDir, "good.md", "---\nmodel: m\n---\nGood.\n")
	require.NoError(t, os.MkdirAll(filepath.Join(projectDir, "baddir.md"), 0o755))

	require.NotPanics(t, func() {
		_, project := DiscoverMarkdownAgents(projectDir)
		assert.Contains(t, project, "good", "valid file must still be discovered alongside the bad entry")
	})
}

// TestDiscover_BothLayersCarrySameStem confirms the project-over-global override
// at the LAYER level is observable (the actual per-field merge is T-B4).
func TestDiscover_BothLayersCarrySameStem(t *testing.T) {
	home := t.TempDir()
	setHomeEnv(t, home)
	globalDir := filepath.Join(home, ".config", "opencode", "agents")
	require.NoError(t, os.MkdirAll(globalDir, 0o755))
	writeAgentMD(t, globalDir, "shared.md", "---\nmodel: global-m\n---\nGlobal body.\n")
	projectDir := t.TempDir()
	writeAgentMD(t, projectDir, "shared.md", "---\nmodel: proj-m\n---\nProject body.\n")

	global, project := DiscoverMarkdownAgents(projectDir)
	assert.Contains(t, global, "shared")
	assert.Contains(t, project, "shared")
}
