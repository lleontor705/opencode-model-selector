package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// ParseMarkdownAgentFile / parseMarkdownContent (T-B2)
//
// Frontmatter is YAML delimited by leading+closing "---" lines. A leading
// UTF-8 BOM is stripped. Degraded rule (decision #6): delimiter absent OR
// unclosed OR YAML-undecodable -> body-only (Raw nil). Hard I/O error -> error
// returned (caller skips). Never panics.
// ---------------------------------------------------------------------------

// writeMD is a test helper that writes content to a temp .md file and returns
// its path.
func writeMD(t *testing.T, content []byte) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "agent.md")
	require.NoError(t, os.WriteFile(p, content, 0o644))
	return p
}

// TestParseStripsBOM verifies that a leading UTF-8 BOM (EF BB BF) is stripped
// before frontmatter parsing so the opening delimiter is recognized.
func TestParseStripsBOM(t *testing.T) {
	bom := []byte{0xEF, 0xBB, 0xBF}
	content := append(bom, []byte("---\nmodel: opencode-go/glm-5.2\n---\nYou are an agent.\n")...)
	p := writeMD(t, content)

	a, err := ParseMarkdownAgentFile(p)
	require.NoError(t, err)
	assert.Empty(t, a.Name, "Name is assigned by the caller, not the parser")
	m, ok := a.Model()
	require.True(t, ok, "model must be parsed after BOM strip")
	assert.Equal(t, "opencode-go/glm-5.2", m)
	assert.Equal(t, "You are an agent.", a.Body)
}

// TestParse_FullFrontmatter verifies the happy path: valid YAML frontmatter is
// decoded into Raw (recognized + unknown fields) and the body follows.
func TestParse_FullFrontmatter(t *testing.T) {
	content := []byte("---\ndescription: Builder\nmode: primary\nmodel: m1\ntemperature: 0.4\nsteps: 3\ncolor: red\n---\nBuild it.\n")
	p := writeMD(t, content)

	a, err := ParseMarkdownAgentFile(p)
	require.NoError(t, err)
	assert.Equal(t, "Builder", a.Description())
	assert.Equal(t, "primary", a.Mode())
	m, ok := a.Model()
	require.True(t, ok)
	assert.Equal(t, "m1", m)
	temp, ok := a.Temperature()
	require.True(t, ok)
	assert.Equal(t, 0.4, temp)
	// Unknown fields preserved in Raw.
	assert.Equal(t, 3, a.Raw["steps"])
	assert.Equal(t, "red", a.Raw["color"])
	assert.Equal(t, "Build it.", a.Body)
}

// TestParseBodyOnly_NoFrontmatter verifies that content with no leading "---"
// is treated as body-only with a nil Raw map.
func TestParseBodyOnly_NoFrontmatter(t *testing.T) {
	content := []byte("Just a system prompt with no frontmatter at all.\n")
	p := writeMD(t, content)

	a, err := ParseMarkdownAgentFile(p)
	require.NoError(t, err)
	assert.Nil(t, a.Raw, "Raw must be nil when no frontmatter is present")
	assert.Contains(t, a.Body, "Just a system prompt")
}

// TestParseBodyOnly_UnclosedDelimiter verifies that an opening "---" with no
// matching closing delimiter degrades to body-only (Raw nil), never panicking.
func TestParseBodyOnly_UnclosedDelimiter(t *testing.T) {
	content := []byte("---\nmodel: m1\nthis never closes\n")
	p := writeMD(t, content)

	a, err := ParseMarkdownAgentFile(p)
	require.NoError(t, err)
	assert.Nil(t, a.Raw, "unclosed delimiter must yield body-only (nil Raw)")
	assert.NotEmpty(t, a.Body, "body must still carry the content")
}

// TestParseBodyOnly_UnclosedDelimiterDoesNotPanic is an explicit no-panic guard
// for the unclosed path with minimal content.
func TestParseBodyOnly_UnclosedDelimiterDoesNotPanic(t *testing.T) {
	require.NotPanics(t, func() {
		_, _ = ParseMarkdownAgentFile(writeMD(t, []byte("---\n")))
	})
	require.NotPanics(t, func() {
		_, _ = ParseMarkdownAgentFile(writeMD(t, []byte("---")))
	})
}

// TestParseBodyOnly_UndecodableYAML verifies that a present-but-garbage YAML
// block degrades to body-only using the structural body region after the
// closing delimiter.
func TestParseBodyOnly_UndecodableYAML(t *testing.T) {
	content := []byte("---\n: : not valid yaml : :\n---\nReal body here.\n")
	p := writeMD(t, content)

	a, err := ParseMarkdownAgentFile(p)
	require.NoError(t, err)
	assert.Nil(t, a.Raw, "undecodable YAML must yield body-only (nil Raw)")
	assert.Equal(t, "Real body here.", a.Body)
}

// TestParse_IOErrorReturnsErr verifies that a hard I/O failure (missing file)
// returns a non-nil error so the caller can skip the file; it never panics.
func TestParse_IOErrorReturnsErr(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist.md")

	var a MDAgent
	var err error
	require.NotPanics(t, func() {
		a, err = ParseMarkdownAgentFile(missing)
	})
	require.Error(t, err, "missing file must return an error")
	assert.Empty(t, a.Name)
	assert.Empty(t, a.Body)
}

// TestParse_EmptyFrontmatterIsNotDegraded verifies that an EMPTY but
// structurally-valid frontmatter block (delimiters present, no fields) yields a
// non-nil empty Raw map (not the body-only degraded path).
func TestParse_EmptyFrontmatterIsNotDegraded(t *testing.T) {
	content := []byte("---\n---\nBody after empty frontmatter.\n")
	p := writeMD(t, content)

	a, err := ParseMarkdownAgentFile(p)
	require.NoError(t, err)
	assert.NotNil(t, a.Raw, "empty frontmatter is structurally valid -> non-nil map")
	assert.Empty(t, a.Raw)
	assert.Equal(t, "Body after empty frontmatter.", a.Body)
}

// TestParse_CRLFLineEndings verifies CRLF (\r\n) files parse the same as LF.
func TestParse_CRLFLineEndings(t *testing.T) {
	content := []byte("---\r\nmodel: m1\r\n---\r\nCRLF body.\r\n")
	p := writeMD(t, content)

	a, err := ParseMarkdownAgentFile(p)
	require.NoError(t, err)
	m, ok := a.Model()
	require.True(t, ok)
	assert.Equal(t, "m1", m)
	assert.Equal(t, "CRLF body.", a.Body)
}
