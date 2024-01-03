package ottlexpr

import (
	"fmt"
	"strings"
)

func NamespaceEquals(name string) string {
	return ResourceAttributeEquals("k8s.namespace.name", name)
}

func ResourceAttributeEquals(key, value string) string {
	return fmt.Sprintf("resource.attributes[\"%s\"] == \"%s\"", key, value)
}

func NameAttributeEquals(name string) string {
	return fmt.Sprintf("name == \"%s\"", name)
}

func JoinWithOr(parts ...string) string {
	return fmt.Sprintf("(%s)", strings.Join(parts, " or "))
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
