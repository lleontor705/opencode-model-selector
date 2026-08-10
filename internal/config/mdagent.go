// Package config — markdown agent discovery.
//
// DiscoverMarkdownAgents scans the two OpenCode agent-markdown layers and
// returns them separately so the merge step (T-B4) can apply per-field
// precedence:
//
//   - global:  ~/.config/opencode/agents/*.md  (resolved via os.UserHomeDir,
//              consistent with GetConfigPath)
//   - project: the .opencode/agents directory passed by the caller (the default
//              is <cwd>/.opencode/agents via DefaultProjectAgentsDir)
//
// Missing directories are non-fatal (yield an empty map). Per-file parse errors
// are logged and skipped; one bad file never breaks discovery for the rest.
package config

import (
	"log"
	"os"
	"path/filepath"
	"strings"
)

// mdFileParse is the parse function used by discovery. It is a package-level
// indirection (the same pattern used for the opencode lookPath variable) so
// tests can deterministically inject hard-IO faults without relying on
// OS-specific permission behavior.
var mdFileParse = ParseMarkdownAgentFile

// DiscoverMarkdownAgents loads the global and project markdown-agent layers.
//
//   - global:   ~/.config/opencode/agents/*.md (home-based)
//   - project:  projectDir/*.md (the caller passes the agents directory; use
//     DefaultProjectAgentsDir for the conventional <cwd>/.opencode/agents)
//
// Both returned maps are always non-nil (empty when a directory is missing or
// holds no .md files). They are keyed by file stem; per-file errors are logged
// and skipped.
func DiscoverMarkdownAgents(projectDir string) (global, project map[string]MDAgent) {
	global = loadMarkdownAgentsDir(globalAgentsDir())
	project = loadMarkdownAgentsDir(projectDir)
	return global, project
}

// loadMarkdownAgentsDir scans dir for *.md files and parses each into an
// MDAgent keyed by file stem. A missing or empty dir yields an empty (non-nil)
// map. filepath.Glob returns matches in lexical order, so discovery order is
// deterministic. Per-file parse errors are logged and that file is skipped.
func loadMarkdownAgentsDir(dir string) map[string]MDAgent {
	result := map[string]MDAgent{}
	if dir == "" {
		return result
	}
	matches, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		return result
	}
	for _, full := range matches {
		a, err := mdFileParse(full)
		if err != nil {
			log.Printf("markdown agent: skipping %s: %v", full, err)
			continue
		}
		stem := strings.TrimSuffix(filepath.Base(full), ".md")
		a.Name = stem
		result[stem] = a
	}
	return result
}

// globalAgentsDir returns ~/.config/opencode/agents using os.UserHomeDir()
// (consistent with GetConfigPath: opencode uses an XDG-style ".config/opencode"
// path on every platform). Returns "" if the home directory cannot be resolved.
func globalAgentsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "opencode", "agents")
}

// DefaultProjectAgentsDir returns the conventional project-local agents
// directory <cwd>/.opencode/agents, or "" if the current working directory
// cannot be determined. Used by GetAgents (T-B5) for the project markdown
// layer.
func DefaultProjectAgentsDir() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return filepath.Join(wd, ".opencode", "agents")
}
