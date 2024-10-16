package builder

import (
	"fmt"
	"strings"
)

func FormatEnvVarName(prefix, namespace, name, key string) string {
	result := fmt.Sprintf("%s_%s_%s_%s", prefix, namespace, name, key)
	result = strings.ToUpper(result)
	result = strings.ReplaceAll(result, ".", "_")
	result = strings.ReplaceAll(result, "-", "_")

	return result
}
