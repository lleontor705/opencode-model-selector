package tui

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func TestProductionTUIContainsNoGenericEditorRemnants(t *testing.T) {
	_, sourceFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	dir := filepath.Dir(sourceFile)
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	forbidden := []string{
		"editable" + "FieldSchema",
		"commit" + "FieldInput",
		"update" + "FieldInput",
		"view" + "FieldInput",
		"update" + "AgentDetail",
		"view" + "AgentDetail",
		"Screen" + "FieldInput",
		"Screen" + "AgentDetail",
		"SetAgent" + "Field",
		`"temperature"`,
		`"top_p"`,
		`"color"`,
		`"steps"`,
		`"disable"`,
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".go" || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		content, readErr := os.ReadFile(filepath.Join(dir, entry.Name()))
		require.NoError(t, readErr)
		for _, remnant := range forbidden {
			assert.NotContains(t, string(content), remnant, "%s contains dead generic editor symbol %q", entry.Name(), remnant)
		}
	}
}
