// Package config — markdown frontmatter parsing.
//
// ParseMarkdownAgentFile reads an OpenCode agent markdown file and extracts its
// YAML frontmatter (into MDAgent.Raw) and its system-prompt body. It is the
// parse half of markdown-agent discovery (T-B2); discovery and name assignment
// live in mdagent.go (T-B3).
//
// Frontmatter format (decision #6): a YAML block delimited by leading and
// closing "---" lines, optionally preceded by a UTF-8 BOM (EF BB BF) which is
// stripped before parsing. Degraded handling is deterministic:
//
//   - leading delimiter ABSENT              -> body-only (Raw nil)
//   - leading delimiter present, UNCLOSED   -> body-only (Raw nil)
//   - delimiters present, YAML UNDECODABLE  -> body-only (Raw nil)
//   - hard I/O failure                      -> error returned (caller skips)
//
// The parser never panics. The returned MDAgent.Name is left empty; the caller
// (discovery) assigns the file stem.
package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// ParseMarkdownAgentFile reads path and parses its frontmatter + body. On a
// hard I/O error it returns the error (caller skips the file). On any degraded
// frontmatter condition it returns a body-only MDAgent and a nil error.
func ParseMarkdownAgentFile(path string) (MDAgent, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return MDAgent{}, fmt.Errorf("read markdown agent %s: %w", path, err)
	}
	return parseMarkdownContent(content), nil
}

// parseMarkdownContent extracts frontmatter from an in-memory byte slice. It is
// split out from ParseMarkdownAgentFile so the pure parsing logic is testable
// without touching the filesystem. It is defensively coded to never panic.
func parseMarkdownContent(content []byte) MDAgent {
	// Strip a leading UTF-8 BOM (EF BB BF) if present.
	content = stripUTF8BOM(content)

	// Normalize CRLF -> LF so line splitting is uniform.
	text := strings.ReplaceAll(string(content), "\r\n", "\n")
	lines := strings.Split(text, "\n")

	// No leading delimiter -> body-only (whole content, trimmed).
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return MDAgent{Body: strings.TrimSpace(string(content))}
	}

	// Find the closing "---" line (must come after the opening line).
	closeIdx := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			closeIdx = i
			break
		}
	}
	if closeIdx == -1 {
		// Unclosed delimiter -> body-only (whole content, trimmed).
		return MDAgent{Body: strings.TrimSpace(string(content))}
	}

	yamlText := strings.Join(lines[1:closeIdx], "\n")
	body := strings.TrimSpace(strings.Join(lines[closeIdx+1:], "\n"))

	// Decode the YAML block into a generic map so ALL fields — recognized and
	// unknown — are preserved. A decode failure degrades to body-only, using
	// the structural body region after the closing delimiter.
	var raw map[string]interface{}
	if err := yaml.Unmarshal([]byte(yamlText), &raw); err != nil {
		return MDAgent{Body: body}
	}
	if raw == nil {
		// An empty-but-valid frontmatter block (e.g. "---\n---") decodes to a
		// nil map; normalize to non-nil empty so callers can range safely.
		raw = map[string]interface{}{}
	}
	return MDAgent{Raw: raw, Body: body}
}

// stripUTF8BOM removes a leading UTF-8 BOM (EF BB BF) if present.
func stripUTF8BOM(content []byte) []byte {
	if len(content) >= 3 && content[0] == 0xEF && content[1] == 0xBB && content[2] == 0xBF {
		return content[3:]
	}
	return content
}
