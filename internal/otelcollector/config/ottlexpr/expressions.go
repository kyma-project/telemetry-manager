package ottlexpr

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
	return fmt.Sprintf("resource.attributes[\"%s\"] == \"%s\"", key, value)
}

func ResourceAttributeNotEquals(key, value string) string {
	return fmt.Sprintf("resource.attributes[\"%s\"] != \"%s\"", key, value)
}

// ResourceAttributeNotNil returns an OTel expression that checks if the resource attribute exists
func ResourceAttributeNotNil(key string) string {
	return fmt.Sprintf("resource.attributes[\"%s\"] != nil", key)
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
