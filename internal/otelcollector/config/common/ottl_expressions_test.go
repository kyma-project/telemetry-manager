package common

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
			name:     "ResourceAttributeIsNilOrEmpty",
			actual:   ResourceAttributeIsNilOrEmpty("key"),
			expected: `resource.attributes["key"] == nil or resource.attributes["key"] == ""`,
		},
		{
			name:     "ResourceAttributeHasPrefix",
			actual:   ResourceAttributeHasPrefix("key", "prefix"),
			expected: `HasPrefix(resource.attributes["key"], "prefix")`,
		},
		{
			name:     "ResourceAttribute",
			actual:   ResourceAttribute("my.key"),
			expected: `resource.attributes["my.key"]`,
		},
		{
			name:     "Attribute",
			actual:   Attribute("my.key"),
			expected: `attributes["my.key"]`,
		},
		{
			name:     "AttributeIsNotNil",
			actual:   AttributeIsNotNil("my.key"),
			expected: `attributes["my.key"] != nil`,
		},
		{
			name:     "AttributeIsNil",
			actual:   AttributeIsNil("my.key"),
			expected: `attributes["my.key"] == nil`,
		},
		{
			name:     "IsNotNil",
			actual:   IsNotNil("some.field"),
			expected: `some.field != nil`,
		},
		{
			name:     "IsNil",
			actual:   IsNil("some.field"),
			expected: `some.field == nil`,
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
			name:     "JoinWithWhere",
			actual:   JoinWithWhere("delete_key(attributes, \"key\")", "attributes[\"key\"] != nil"),
			expected: `delete_key(attributes, "key") where attributes["key"] != nil`,
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
			name:     "KymaInputNameEquals",
			actual:   KymaInputNameEquals("my-name"),
			expected: `resource.attributes["kyma.input.name"] == "my-name"`,
		},
		{
			name:     "Not",
			actual:   Not("a"),
			expected: `not(a)`,
		},
		{
			name:     "Not with parentheses",
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
