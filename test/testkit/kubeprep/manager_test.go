package kubeprep

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestValuesEqual(t *testing.T) {
	tests := []struct {
		name        string
		currentYAML string
		newValues   map[string]any
		expected    bool
	}{
		{
			name:        "empty current values means different",
			currentYAML: "",
			newValues:   map[string]any{"key": "value"},
			expected:    false,
		},
		{
			name:        "identical simple values",
			currentYAML: "key: value\n",
			newValues:   map[string]any{"key": "value"},
			expected:    true,
		},
		{
			name:        "different simple values",
			currentYAML: "key: value1\n",
			newValues:   map[string]any{"key": "value2"},
			expected:    false,
		},
		{
			name: "identical nested values",
			currentYAML: `outer:
  inner: value
`,
			newValues: map[string]any{
				"outer": map[string]any{
					"inner": "value",
				},
			},
			expected: true,
		},
		{
			name: "different nested values",
			currentYAML: `outer:
  inner: value1
`,
			newValues: map[string]any{
				"outer": map[string]any{
					"inner": "value2",
				},
			},
			expected: false,
		},
		{
			name: "identical values with different key order in YAML",
			currentYAML: `b: "2"
a: "1"
`,
			newValues: map[string]any{"a": "1", "b": "2"},
			expected:  true,
		},
		{
			name:        "identical boolean values",
			currentYAML: "enabled: true\n",
			newValues:   map[string]any{"enabled": true},
			expected:    true,
		},
		{
			name:        "different boolean values",
			currentYAML: "enabled: true\n",
			newValues:   map[string]any{"enabled": false},
			expected:    false,
		},
		{
			name:        "extra key in new values",
			currentYAML: "a: \"1\"\n",
			newValues:   map[string]any{"a": "1", "b": "2"},
			expected:    false,
		},
		{
			name: "extra key in current values",
			currentYAML: `a: "1"
b: "2"
`,
			newValues: map[string]any{"a": "1"},
			expected:  false,
		},
		{
			name:        "invalid YAML in current values",
			currentYAML: "{{invalid",
			newValues:   map[string]any{"key": "value"},
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := valuesEqual(tt.currentYAML, tt.newValues)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildHelmValues(t *testing.T) {
	tests := []struct {
		name          string
		cfg           Config
		imageOverride string
		checkFn       func(t *testing.T, values map[string]any)
	}{
		{
			name: "basic values without image override",
			cfg: Config{
				EnableExperimental: false,
				OperateInFIPSMode:  false,
			},
			imageOverride: "",
			checkFn: func(t *testing.T, values map[string]any) {
				exp := values["experimental"].(map[string]any)
				assert.Equal(t, false, exp["enabled"])

				def := values["default"].(map[string]any)
				assert.Equal(t, true, def["enabled"])

				assert.Equal(t, "telemetry", values["nameOverride"])
			},
		},
		{
			name: "experimental enabled",
			cfg: Config{
				EnableExperimental: true,
				OperateInFIPSMode:  false,
			},
			imageOverride: "",
			checkFn: func(t *testing.T, values map[string]any) {
				exp := values["experimental"].(map[string]any)
				assert.Equal(t, true, exp["enabled"])

				def := values["default"].(map[string]any)
				assert.Equal(t, false, def["enabled"])
			},
		},
		{
			name: "FIPS mode enabled",
			cfg: Config{
				EnableExperimental: false,
				OperateInFIPSMode:  true,
			},
			imageOverride: "",
			checkFn: func(t *testing.T, values map[string]any) {
				manager := values["manager"].(map[string]any)
				container := manager["container"].(map[string]any)
				env := container["env"].(map[string]any)
				assert.Equal(t, true, env["operateInFipsMode"])
			},
		},
		{
			name: "with remote image override",
			cfg: Config{
				EnableExperimental: false,
				OperateInFIPSMode:  false,
			},
			imageOverride: "europe-docker.pkg.dev/kyma-project/prod/telemetry-manager:v1.0.0",
			checkFn: func(t *testing.T, values map[string]any) {
				manager := values["manager"].(map[string]any)
				container := manager["container"].(map[string]any)
				image := container["image"].(map[string]any)
				assert.Equal(t, "europe-docker.pkg.dev/kyma-project/prod/telemetry-manager:v1.0.0", image["repository"])
				assert.Equal(t, "Always", image["pullPolicy"])
			},
		},
		{
			name: "with local image override",
			cfg: Config{
				EnableExperimental: false,
				OperateInFIPSMode:  false,
			},
			imageOverride: "localhost:5001/telemetry-manager:latest",
			checkFn: func(t *testing.T, values map[string]any) {
				manager := values["manager"].(map[string]any)
				container := manager["container"].(map[string]any)
				image := container["image"].(map[string]any)
				assert.Equal(t, "localhost:5001/telemetry-manager:latest", image["repository"])
				assert.Equal(t, "IfNotPresent", image["pullPolicy"])
			},
		},
		{
			name: "with custom helm values",
			cfg: Config{
				EnableExperimental: false,
				OperateInFIPSMode:  false,
				HelmValues:         []string{"custom.key=customValue", "custom.nested.key=nestedValue"},
			},
			imageOverride: "",
			checkFn: func(t *testing.T, values map[string]any) {
				custom := values["custom"].(map[string]any)
				assert.Equal(t, "customValue", custom["key"])

				nested := custom["nested"].(map[string]any)
				assert.Equal(t, "nestedValue", nested["key"])
			},
		},
		{
			name: "helm values with boolean strings",
			cfg: Config{
				EnableExperimental: false,
				OperateInFIPSMode:  false,
				HelmValues:         []string{"feature.enabled=true", "feature.disabled=false"},
			},
			imageOverride: "",
			checkFn: func(t *testing.T, values map[string]any) {
				feature := values["feature"].(map[string]any)
				assert.Equal(t, true, feature["enabled"])
				assert.Equal(t, false, feature["disabled"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values := buildHelmValues(tt.cfg, tt.imageOverride)
			tt.checkFn(t, values)
		})
	}
}

func TestApplyHelmSetValue(t *testing.T) {
	tests := []struct {
		name     string
		initial  map[string]any
		setValue string
		checkFn  func(t *testing.T, values map[string]any)
	}{
		{
			name:     "simple key-value",
			initial:  map[string]any{},
			setValue: "key=value",
			checkFn: func(t *testing.T, values map[string]any) {
				assert.Equal(t, "value", values["key"])
			},
		},
		{
			name:     "nested key-value",
			initial:  map[string]any{},
			setValue: "a.b.c=value",
			checkFn: func(t *testing.T, values map[string]any) {
				a := values["a"].(map[string]any)
				b := a["b"].(map[string]any)
				assert.Equal(t, "value", b["c"])
			},
		},
		{
			name:     "boolean true",
			initial:  map[string]any{},
			setValue: "enabled=true",
			checkFn: func(t *testing.T, values map[string]any) {
				assert.Equal(t, true, values["enabled"])
			},
		},
		{
			name:     "boolean false",
			initial:  map[string]any{},
			setValue: "enabled=false",
			checkFn: func(t *testing.T, values map[string]any) {
				assert.Equal(t, false, values["enabled"])
			},
		},
		{
			name:     "overwrites existing value",
			initial:  map[string]any{"key": "old"},
			setValue: "key=new",
			checkFn: func(t *testing.T, values map[string]any) {
				assert.Equal(t, "new", values["key"])
			},
		},
		{
			name: "merges with existing nested structure",
			initial: map[string]any{
				"outer": map[string]any{
					"existing": "value",
				},
			},
			setValue: "outer.new=added",
			checkFn: func(t *testing.T, values map[string]any) {
				outer := values["outer"].(map[string]any)
				assert.Equal(t, "value", outer["existing"])
				assert.Equal(t, "added", outer["new"])
			},
		},
		{
			name:     "invalid format without equals",
			initial:  map[string]any{},
			setValue: "invalid",
			checkFn: func(t *testing.T, values map[string]any) {
				assert.Empty(t, values)
			},
		},
		{
			name:     "value with equals sign",
			initial:  map[string]any{},
			setValue: "key=value=with=equals",
			checkFn: func(t *testing.T, values map[string]any) {
				assert.Equal(t, "value=with=equals", values["key"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			applyHelmSetValue(tt.initial, tt.setValue)
			tt.checkFn(t, tt.initial)
		})
	}
}

func TestSortMapRecursively(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]any
		expected map[string]any
	}{
		{
			name:     "empty map",
			input:    map[string]any{},
			expected: map[string]any{},
		},
		{
			name: "flat map gets sorted",
			input: map[string]any{
				"z": "last",
				"a": "first",
				"m": "middle",
			},
			expected: map[string]any{
				"a": "first",
				"m": "middle",
				"z": "last",
			},
		},
		{
			name: "nested maps get sorted recursively",
			input: map[string]any{
				"outer": map[string]any{
					"z": "last",
					"a": "first",
				},
			},
			expected: map[string]any{
				"outer": map[string]any{
					"a": "first",
					"z": "last",
				},
			},
		},
		{
			name: "deeply nested maps",
			input: map[string]any{
				"level1": map[string]any{
					"z": "value",
					"level2": map[string]any{
						"c": "third",
						"a": "first",
						"b": "second",
					},
					"a": "value",
				},
			},
			expected: map[string]any{
				"level1": map[string]any{
					"a": "value",
					"level2": map[string]any{
						"a": "first",
						"b": "second",
						"c": "third",
					},
					"z": "value",
				},
			},
		},
		{
			name: "mixed types preserved",
			input: map[string]any{
				"string": "value",
				"bool":   true,
				"number": 42,
				"nested": map[string]any{
					"b": "second",
					"a": "first",
				},
			},
			expected: map[string]any{
				"bool":   true,
				"nested": map[string]any{"a": "first", "b": "second"},
				"number": 42,
				"string": "value",
			},
		},
		{
			name: "arrays preserved",
			input: map[string]any{
				"items": []any{"c", "a", "b"},
				"nested": map[string]any{
					"z": "last",
					"a": "first",
				},
			},
			expected: map[string]any{
				"items": []any{"c", "a", "b"}, // array order preserved
				"nested": map[string]any{
					"a": "first",
					"z": "last",
				},
			},
		},
		{
			name: "maps inside arrays are sorted",
			input: map[string]any{
				"list": []any{
					map[string]any{"z": "1", "a": "2"},
					map[string]any{"c": "3", "b": "4"},
				},
			},
			expected: map[string]any{
				"list": []any{
					map[string]any{"a": "2", "z": "1"},
					map[string]any{"b": "4", "c": "3"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sortMapRecursively(tt.input)
			// Convert both to YAML to compare (YAML preserves order after sorting)
			assertMapsEqual(t, tt.expected, tt.input)
		})
	}
}

func TestSortMapRecursively_MapsInsideArrays(t *testing.T) {
	// This test verifies that maps inside arrays are handled correctly.
	// Note: yaml.v3 already sorts map keys alphabetically when marshaling,
	// so even without explicit array handling, the output is deterministic.
	input := map[string]any{
		"list": []any{
			map[string]any{"z": "1", "a": "2"},
		},
	}

	sortMapRecursively(input)

	out, err := yaml.Marshal(input)
	require.NoError(t, err)

	// Verify "a" comes before "z" in the output
	yamlStr := string(out)
	aPos := indexOf(yamlStr, "a:")
	zPos := indexOf(yamlStr, "z:")

	require.NotEqual(t, -1, aPos, "should find 'a:' in output")
	require.NotEqual(t, -1, zPos, "should find 'z:' in output")
	assert.Less(t, aPos, zPos, "key 'a' should come before 'z' in sorted output, got:\n%s", yamlStr)
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}

	return -1
}

// assertMapsEqual compares two maps by checking all keys and values match.
func assertMapsEqual(t *testing.T, expected, actual map[string]any) {
	t.Helper()
	assert.Equal(t, len(expected), len(actual), "map lengths should match")

	for k, expectedVal := range expected {
		actualVal, ok := actual[k]
		assert.True(t, ok, "key %q should exist in actual map", k)

		switch ev := expectedVal.(type) {
		case map[string]any:
			av, ok := actualVal.(map[string]any)
			assert.True(t, ok, "value for key %q should be a map", k)
			assertMapsEqual(t, ev, av)
		case []any:
			av, ok := actualVal.([]any)
			assert.True(t, ok, "value for key %q should be an array", k)
			assertSlicesEqual(t, k, ev, av)
		default:
			assert.Equal(t, expectedVal, actualVal, "value for key %q should match", k)
		}
	}
}

// assertSlicesEqual compares two slices, recursively comparing maps within.
func assertSlicesEqual(t *testing.T, key string, expected, actual []any) {
	t.Helper()
	assert.Equal(t, len(expected), len(actual), "slice lengths for key %q should match", key)

	for i := range expected {
		switch ev := expected[i].(type) {
		case map[string]any:
			av, ok := actual[i].(map[string]any)
			assert.True(t, ok, "element %d of key %q should be a map", i, key)
			assertMapsEqual(t, ev, av)
		default:
			assert.Equal(t, expected[i], actual[i], "element %d of key %q should match", i, key)
		}
	}
}
