package unique

import (
	"path"
	"regexp"
	"runtime"
	"strings"
)

// Prefix generates a function that appends a unique prefix to a given string.
// The prefix is constructed using the file path of the caller, an optional test category
// (if the caller function name follows the "TestXxx_Category" pattern), and any additional
// suffixes provided as arguments.
//
// The returned function takes a string `s` and returns a new string with the constructed
// prefix and `s` joined by hyphens.
//
// Example usage:
//
//	prefixFunc := Prefix("suffix1", "suffix2")
//	result := prefixFunc("myString")
//	// result might be "caller-file-path-test-category-suffix1-suffix2-myString"
func Prefix(fixedSuffixes ...string) func(suffixes ...string) string {
	pc, filePath, _, ok := runtime.Caller(1)
	if !ok {
		panic("failed to retrieve the current file path")
	}

	idSegments := []string{sanitizeFilePath(filePath)}

	fn := runtime.FuncForPC(pc)
	if fn != nil {
		if category := extractTestCategory(fn); category != "" {
			idSegments = append(idSegments, category)
		}
	}

	idSegments = append(idSegments, fixedSuffixes...)

	return func(strs ...string) string {
		return strings.Join(append(idSegments, strs...), "-")
	}
}

// extractTestCategory extracts the category from a test function name if it follows the pattern "TestXxx_Category".
func extractTestCategory(callerFunc *runtime.Func) string {
	if callerFunc == nil {
		return ""
	}

	baseFuncName := extractBaseFuncName(callerFunc.Name())
	if strings.HasPrefix(baseFuncName, "Test") {
		funcNameParts := strings.Split(baseFuncName, "_")
		if len(funcNameParts) > 1 {
			return strings.ToLower(funcNameParts[1])
		}
	}

	return ""
}

// extractBaseFuncName extracts the base function name from a full runtime function name.
// For example, given the input "github.com/kyma-project/telemetry-manager/test/e2e/logs/migrated/shared.TestMyFeature_OTel.func3",
// it will extract and return "TestMyFeature_OTel".
func extractBaseFuncName(fn string) string {
	if fn == "" {
		return ""
	}

	// Remove package path
	parts := strings.Split(fn, "/")
	lastPart := parts[len(parts)-1]

	// Drop any ".funcN" suffix
	lastPart = regexp.MustCompile(`\.func\d+$`).ReplaceAllString(lastPart, "")

	// Extract the method name from the last segment
	if idx := strings.LastIndex(lastPart, "."); idx != -1 {
		return lastPart[idx+1:]
	}

	return lastPart
}

func sanitizeFilePath(filePath string) string {
	if filePath == "" {
		return ""
	}

	fileName := path.Base(filePath)
	specID := strings.TrimSuffix(fileName, "_test.go")

	return strings.ReplaceAll(specID, "_", "-")
}
