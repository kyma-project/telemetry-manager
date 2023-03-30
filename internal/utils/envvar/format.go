package envvar

import (
	"fmt"
	"strings"
)

func FormatEnvVarName(prefix, namespace, name, key string) string {
	result := fmt.Sprintf("%s_%s_%s_%s", prefix, namespace, name, key)
	return MakeEnvVarCompliant(result)
}

func MakeEnvVarCompliant(input string) string {
	result := input
	result = strings.ToUpper(result)
	result = strings.Replace(result, ".", "_", -1)
	result = strings.Replace(result, "-", "_", -1)
	return result
}
