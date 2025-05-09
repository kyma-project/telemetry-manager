package ottlexpr

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOttlExprFunctions(t *testing.T) {
	tests := []struct {
		name     string
		actual   string
		expected string
	}{
		{
			name:     "NamespaceEquals",
			actual:   NamespaceEquals("default"),
			expected: `resource.attributes["k8s.namespace.name"] == "default"`,
		},
		{
			name:     "ResourceAttributeEquals",
			actual:   ResourceAttributeEquals("key", "value"),
			expected: `resource.attributes["key"] == "value"`,
		},
		{
			name:     "ResourceAttributeNotEquals",
			actual:   ResourceAttributeNotEquals("key", "value"),
			expected: `resource.attributes["key"] != "value"`,
		},
		{
			name:     "ResourceAttributeNotNil",
			actual:   ResourceAttributeIsNotNil("key"),
			expected: `resource.attributes["key"] != nil`,
		},
		{
			name:     "NameAttributeEquals",
			actual:   NameAttributeEquals("my-service"),
			expected: `name == "my-service"`,
		},
		{
			name:     "JoinWithOr",
			actual:   JoinWithOr("a", "b", "c"),
			expected: "(a or b or c)",
		},
		{
			name:     "JoinWithRegExpOr",
			actual:   JoinWithRegExpOr("a", "b", "c"),
			expected: "(a|b|c)",
		},
		{
			name:     "JoinWithAnd",
			actual:   JoinWithAnd("a", "b", "c"),
			expected: "a and b and c",
		},
		{
			name:     "IsMatch",
			actual:   IsMatch("key", "regex"),
			expected: `IsMatch(key, "regex")`,
		},
		{
			name:     "HasAttrOnDatapoint",
			actual:   HasAttrOnDatapoint("key", "value"),
			expected: `HasAttrOnDatapoint("key", "value")`,
		},
		{
			name:     "ScopeNameEquals",
			actual:   ScopeNameEquals("my-scope"),
			expected: `instrumentation_scope.name == "my-scope"`,
		},
		{
			name:     "Not",
			actual:   Not("a"),
			expected: `not(a)`,
		},
		{
			name:     "NotWithParentheses",
			actual:   Not("(a)"),
			expected: `not(a)`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.expected, test.actual)
		})
	}
}
