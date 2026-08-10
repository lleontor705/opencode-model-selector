package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// MDAgent — recognized field set + raw-map preservation (T-B1)
//
// MDAgent is the READ-ONLY markdown-backed agent representation. Recognized
// OpenCode frontmatter fields have typed accessors; every other frontmatter
// key is preserved verbatim in Raw so nothing is lost on load.
// ---------------------------------------------------------------------------

// TestRecognizedFieldSet verifies that the recognized frontmatter set contains
// exactly the OpenCode agent fields and excludes the JSON-only agent fields.
func TestRecognizedFieldSet(t *testing.T) {
	expected := []string{"description", "mode", "model", "temperature", "permission", "tools"}
	for _, f := range expected {
		assert.True(t, RecognizedMDFields[f],
			"%q must be a recognized markdown frontmatter field", f)
	}

	// JSON-only fields must NOT be in the recognized markdown set; they are
	// preserved as unknown raw fields instead.
	for _, f := range []string{"steps", "color", "top_p", "disable", "hidden"} {
		assert.False(t, RecognizedMDFields[f],
			"%q must NOT be a recognized markdown field (preserved as raw)", f)
	}
	assert.Len(t, RecognizedMDFields, 6,
		"exactly 6 recognized markdown frontmatter fields")
}

// TestRawMapPreservesUnknownFields verifies that unknown frontmatter keys
// (e.g. steps, color) are retained in Raw while recognized fields are also
// accessible there.
func TestRawMapPreservesUnknownFields(t *testing.T) {
	a := MDAgent{
		Name: "build",
		Body: "You are a build agent.",
		Raw: map[string]interface{}{
			// recognized
			"description": "Build orchestrator",
			"mode":        "primary",
			"model":       "opencode-go/glm-5.2",
			"temperature": float64(0.2),
			"permission":  map[string]interface{}{"bash": "allow"},
			"tools":       map[string]interface{}{"skill": false},
			// unknown — must survive
			"steps":  5,
			"color":  "blue",
			"custom": "anything",
		},
	}

	// Unknown fields preserved verbatim.
	assert.Equal(t, 5, a.Raw["steps"])
	assert.Equal(t, "blue", a.Raw["color"])
	assert.Equal(t, "anything", a.Raw["custom"])

	// Recognized accessors return typed values.
	assert.Equal(t, "Build orchestrator", a.Description())
	assert.Equal(t, "primary", a.Mode())
	m, ok := a.Model()
	assert.True(t, ok)
	assert.Equal(t, "opencode-go/glm-5.2", m)
	temp, ok := a.Temperature()
	assert.True(t, ok)
	assert.Equal(t, 0.2, temp)
	assert.NotNil(t, a.PermissionRaw())
	assert.NotNil(t, a.ToolsRaw())
}

// TestMDAgent_AccessorsDefaultOnAbsent verifies typed accessors return zero /
// "all" defaults when fields are absent or mistyped (no coercion).
func TestMDAgent_AccessorsDefaultOnAbsent(t *testing.T) {
	a := MDAgent{
		Name: "stale",
		Raw: map[string]interface{}{
			"mode": "bogus-is-not-coerced", // mistyped: not a valid mode
		},
	}

	// mode present but not a recognized value -> Mode() returns it verbatim
	// (we only default when ABSENT or NON-STRING, not when the string is odd).
	assert.Equal(t, "bogus-is-not-coerced", a.Mode(),
		"Mode returns the string as-is; it does not validate the value")

	// Absent fields.
	empty := MDAgent{Name: "x", Raw: map[string]interface{}{}}
	assert.Equal(t, "", empty.Description())
	assert.Equal(t, "all", empty.Mode(), "absent mode defaults to all")
	_, ok := empty.Model()
	assert.False(t, ok)
	_, ok = empty.Temperature()
	assert.False(t, ok)
	assert.Nil(t, empty.PermissionRaw())
	assert.Nil(t, empty.ToolsRaw())
}

// TestMDAgent_TemperatureAcceptsInt verifies the temperature accessor handles
// integer frontmatter values (YAML decodes whole numbers as int).
func TestMDAgent_TemperatureAcceptsInt(t *testing.T) {
	a := MDAgent{Raw: map[string]interface{}{"temperature": 1}}
	temp, ok := a.Temperature()
	assert.True(t, ok)
	assert.Equal(t, 1.0, temp)
}
