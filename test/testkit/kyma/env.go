package kyma

import "strings"

func MakeEnvVarCompliant(input string) string {
	result := input
	result = strings.ToUpper(result)
	result = strings.Replace(result, ".", "_", -1)
	result = strings.Replace(result, "-", "_", -1)
	return result
}
