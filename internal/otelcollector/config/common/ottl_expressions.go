package common

import (
	"fmt"
	"strings"
)

const (
	K8sNamespaceName = "k8s.namespace.name"
)

func NamespaceEquals(name string) string {
	return ResourceAttributeEquals(K8sNamespaceName, name)
}

func ResourceAttributeEquals(key, value string) string {
	return fmt.Sprintf("%s == \"%s\"", ResourceAttribute(key), value)
}

func ResourceAttributeNotEquals(key, value string) string {
	return fmt.Sprintf("%s != \"%s\"", ResourceAttribute(key), value)
}

// ResourceAttributeIsNotNil returns an OTel expression that checks if the resource attribute exists
func ResourceAttributeIsNotNil(key string) string {
	return fmt.Sprintf("%s != nil", ResourceAttribute(key))
}

func ResourceAttribute(key string) string {
	return fmt.Sprintf("resource.attributes[\"%s\"]", key)
}

func AttributeIsNotNil(key string) string {
	return IsNotNil(Attribute(key))
}

func AttributeIsNil(key string) string {
	return IsNil(Attribute(key))
}

func IsNotNil(key string) string {
	return fmt.Sprintf("%s != nil", key)
}

func IsNil(key string) string {
	return fmt.Sprintf("%s == nil", key)
}

func Attribute(key string) string {
	return fmt.Sprintf("attributes[\"%s\"]", key)
}

func NameAttributeEquals(name string) string {
	return fmt.Sprintf("name == \"%s\"", name)
}

func JoinWithOr(parts ...string) string {
	return fmt.Sprintf("(%s)", strings.Join(parts, " or "))
}

func JoinWithRegExpOr(parts ...string) string {
	return fmt.Sprintf("(%s)", strings.Join(parts, "|"))
}

func JoinWithAnd(parts ...string) string {
	return strings.Join(parts, " and ")
}

func IsMatch(key, regexPattern string) string {
	return fmt.Sprintf("IsMatch(%s, \"%s\")", key, regexPattern)
}

func HasAttrOnDatapoint(key, value string) string {
	return fmt.Sprintf("HasAttrOnDatapoint(\"%s\", \"%s\")", key, value)
}

func ScopeNameEquals(name string) string {
	return fmt.Sprintf("instrumentation_scope.name == \"%s\"", name)
}

func Not(expression string) string {
	if isWrappedInParentheses(expression) {
		return fmt.Sprintf("not%s", expression)
	}

	return fmt.Sprintf("not(%s)", expression)
}

func isWrappedInParentheses(expression string) bool {
	return strings.HasPrefix(expression, "(") && strings.HasSuffix(expression, ")")
}
