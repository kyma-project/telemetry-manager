package kyma

import "strings"

func MakeEnvVarCompliant(input string) string {
	result := input
	result = strings.ToUpper(result)
	result = strings.ReplaceAll(result, ".", "_")
	result = strings.ReplaceAll(result, "-", "_")

	return result
}
