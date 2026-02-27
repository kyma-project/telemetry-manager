package suite

import (
	"fmt"
	"log"
	"strings"

	"github.com/expr-lang/expr"
)

// all we do here is to convert our syntax to expr syntax and evaluate it
// our syntax uses AND, OR, NOT for better readability

// convertLabelExpressionSyntax converts our syntax (AND, OR, NOT) to expr syntax (&&, ||, !)
// expr-lang/expr uses different syntax for logical operators. For our purpose AND OR NOT make more sense.
func convertLabelExpressionSyntax(legacyExpr string) string {
	if strings.TrimSpace(legacyExpr) == "" {
		return ""
	}

	converted := strings.ToLower(legacyExpr)

	converted = replaceWord(converted, "and", "&&")
	converted = replaceWord(converted, "or", "||")
	converted = replaceWord(converted, "not", "!")

	return converted
}

// replaceWord replaces whole words only, not substrings within words
func replaceWord(s, original, replacement string) string {
	var result strings.Builder

	i := 0
	oldLen := len(original)

	for i < len(s) {
		// Check if we found the word at current position
		if i+oldLen <= len(s) && s[i:i+oldLen] == original {
			// Check if it's a complete word (not part of another word)
			beforeOK := i == 0 || !isAlphaNumeric(s[i-1])
			afterOK := i+oldLen == len(s) || !isAlphaNumeric(s[i+oldLen])

			if beforeOK && afterOK {
				result.WriteString(replacement)

				i += oldLen

				continue
			}
		}

		result.WriteByte(s[i])
		i++
	}

	return result.String()
}

// isAlphaNumeric checks if a character is alphanumeric, underscore, or hyphen
func isAlphaNumeric(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-'
}

// evaluateLabelExpression evaluates a label filter expression against test labels using expr-lang/expr
func evaluateLabelExpression(testLabels []string, filterExpr string) (bool, error) {
	if strings.TrimSpace(filterExpr) == "" {
		return true, nil // No filter means run all tests
	}

	// convert our syntax to expr syntax
	exprSyntax := convertLabelExpressionSyntax(filterExpr)

	labelSet := make(map[string]bool)
	for _, label := range testLabels {
		labelSet[strings.ToLower(label)] = true
	}

	log.Printf("evaluating test labels: %v", testLabels)

	// this is some hack. we replace all labels with hasLabel("label") function calls.
	// the haslabel function checks if the label is present in the label set
	// the function is dynamically defined in the env
	env := map[string]any{
		"hasLabel": func(label string) bool {
			return labelSet[strings.ToLower(label)]
		},
	}

	transformedExpr := transformExpressionToFunctionCalls(exprSyntax)

	program, err := expr.Compile(transformedExpr, expr.Env(env), expr.AsBool())
	if err != nil {
		return false, fmt.Errorf("invalid label filter expression '%s': %w", filterExpr, err)
	}

	result, err := expr.Run(program, env)
	if err != nil {
		return false, fmt.Errorf("failed to evaluate label filter '%s': %w", filterExpr, err)
	}

	switch r := result.(type) {
	case bool:
		return r, nil
	default:
		return false, fmt.Errorf("label filter expression '%s' did not evaluate to a boolean", filterExpr)
	}
}

// transformExpressionToFunctionCalls converts label identifiers to hasLabel() function calls
func transformExpressionToFunctionCalls(exprStr string) string {
	var result strings.Builder

	i := 0

	for i < len(exprStr) {
		if exprStr[i] == '&' || exprStr[i] == '|' || exprStr[i] == '!' ||
			exprStr[i] == '(' || exprStr[i] == ')' || exprStr[i] == ' ' {
			result.WriteByte(exprStr[i])
			i++

			continue
		}

		// we try to find a `word` (label)
		// a word is defined as a sequence of alphanumeric characters, underscores, or hyphens
		start := i
		for i < len(exprStr) && isAlphaNumeric(exprStr[i]) {
			i++
		}

		label := exprStr[start:i]
		result.WriteString(fmt.Sprintf("hasLabel(\"%s\")", label))
	}

	return result.String()
}
