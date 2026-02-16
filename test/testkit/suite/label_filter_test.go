package suite

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

/*
Package suite provides enhanced label filtering for test execution.

Usage Examples:

Basic filtering:
  go test -- -labels="fips"                   // Run only FIPS tests
  go test -- -labels="not fips"               // Run all tests except FIPS

Complex filtering:
  go test -- -labels="fips and logs"          // Run tests that have both FIPS and logs labels
  go test -- -labels="logs or metrics"        // Run tests that have either logs or metrics labels
  go test -- -labels="not (slow or flaky)"    // Run tests that are neither slow nor flaky

Real-world examples:
  go test -- -labels="(integration or e2e) and not experimental"
  go test -- -labels="fips and (logs or metrics) and not slow"
  go test -- -labels="not (slow and flaky) or critical"

In test files, register tests with labels:
  func TestMyFeature(t *testing.T) {
      suite.SetupTest(t, "fips", "logs", "integration")
      // ... test implementation
  }

  func TestAnotherFeature(t *testing.T) {
      suite.SetupTest(t, "metrics", "unit")
      // ... test implementation
  }

Operator precedence (highest to lowest):
  1. NOT (!)
  2. AND (&)
  3. OR (|)
  4. Parentheses override precedence

Note: All operators are case-insensitive
*/

// TestLabelExpressionParsing serves as both validation and documentation
// for the enhanced label filtering system that supports complex boolean expressions.
func TestLabelExpressionParsing(t *testing.T) {
	tests := []struct {
		name        string
		expression  string
		description string
		testLabels  []string
		expected    bool
	}{
		// Basic label matching
		{
			name:        "simple_label_match",
			expression:  "fips",
			description: "Test with single label - should match when label is present",
			testLabels:  []string{"fips", "logs"},
			expected:    true,
		},
		{
			name:        "simple_label_no_match",
			expression:  "vector",
			description: "Test with single label - should not match when label is absent",
			testLabels:  []string{"fips", "logs"},
			expected:    false,
		},

		// NOT operations
		{
			name:        "not_operation_exclude",
			expression:  "not fips",
			description: "Exclude tests with 'fips' label - should skip when label is present",
			testLabels:  []string{"fips", "integration"},
			expected:    false,
		},
		{
			name:        "not_operation_include",
			expression:  "not fips",
			description: "Exclude tests with 'fips' label - should run when label is absent",
			testLabels:  []string{"integration", "logs"},
			expected:    true,
		},

		// AND operations - require all labels
		{
			name:        "and_operation_all_present",
			expression:  "fips and logs",
			description: "Require both 'fips' AND 'logs' labels - should match when both present",
			testLabels:  []string{"fips", "logs", "integration"},
			expected:    true,
		},
		{
			name:        "and_operation_partial",
			expression:  "fips and logs",
			description: "Require both 'fips' AND 'logs' labels - should not match when only one present",
			testLabels:  []string{"fips", "integration"},
			expected:    false,
		},
		{
			name:        "and_operation_none",
			expression:  "fips and logs",
			description: "Require both 'fips' AND 'logs' labels - should not match when neither present",
			testLabels:  []string{"metrics", "integration"},
			expected:    false,
		},

		// OR operations - require at least one label
		{
			name:        "or_operation_both_present",
			expression:  "fips or vector",
			description: "Require 'fips' OR 'vector' label - should match when both present",
			testLabels:  []string{"fips", "vector", "logs"},
			expected:    true,
		},
		{
			name:        "or_operation_one_present",
			expression:  "fips or vector",
			description: "Require 'fips' OR 'vector' label - should match when one present",
			testLabels:  []string{"fips", "logs"},
			expected:    true,
		},
		{
			name:        "or_operation_none_present",
			expression:  "fips or vector",
			description: "Require 'fips' OR 'vector' label - should not match when neither present",
			testLabels:  []string{"logs", "metrics"},
			expected:    false,
		},

		// Operator precedence: NOT > AND > OR
		{
			name:        "precedence_not_and_or",
			expression:  "not slow and fips or logs",
			description: "Precedence test: ((not slow) and fips) or logs - should match due to 'logs'",
			testLabels:  []string{"logs"},
			expected:    true,
		},
		{
			name:        "precedence_complex",
			expression:  "fips or logs and not slow",
			description: "Precedence test: fips or (logs and (not slow)) - should match due to 'fips'",
			testLabels:  []string{"fips", "slow"},
			expected:    true,
		},

		// Parentheses for explicit grouping
		{
			name:        "parentheses_grouping_or_and",
			expression:  "(fips or vector) and logs",
			description: "Explicit grouping: (fips or vector) and logs - both conditions must be true",
			testLabels:  []string{"fips", "logs"},
			expected:    true,
		},
		{
			name:        "parentheses_grouping_or_and_fail",
			expression:  "(fips or vector) and logs",
			description: "Explicit grouping: (fips or vector) and logs - should fail when 'logs' missing",
			testLabels:  []string{"fips"},
			expected:    false,
		},
		{
			name:        "parentheses_grouping_not",
			expression:  "not (slow and experimental)",
			description: "Grouping with NOT: not (slow and experimental) - should match when not both present",
			testLabels:  []string{"slow", "integration"},
			expected:    true,
		},
		{
			name:        "parentheses_grouping_not_fail",
			expression:  "not (slow and experimental)",
			description: "Grouping with NOT: not (slow and experimental) - should fail when both present",
			testLabels:  []string{"slow", "experimental"},
			expected:    false,
		},

		// Complex real-world scenarios
		{
			name:        "real_world_fips_scenario",
			expression:  "fips and (logs or metrics) and not experimental",
			description: "Real scenario: FIPS tests for logs or metrics, but not experimental ones",
			testLabels:  []string{"fips", "logs", "integration"},
			expected:    true,
		},
		{
			name:        "real_world_exclude_slow_tests",
			expression:  "(integration or unit) and not slow and not flaky",
			description: "Real scenario: Run integration or unit tests, but exclude slow and flaky ones",
			testLabels:  []string{"integration", "reliable"},
			expected:    true,
		},
		{
			name:        "real_world_exclude_slow_tests_fail",
			expression:  "(integration or unit) and not slow and not flaky",
			description: "Real scenario: Should exclude tests marked as slow",
			testLabels:  []string{"integration", "slow"},
			expected:    false,
		},
		{
			name:        "real_world_agent_tests",
			expression:  "(fluent-bit or otel) and not (fips or experimental)",
			description: "Real scenario: Agent tests for fluent-bit or otel, excluding fips and experimental",
			testLabels:  []string{"fluent-bit", "logs"},
			expected:    true,
		},

		// Edge cases
		{
			name:        "not_with_matching_label",
			expression:  "not fips",
			description: "NOT with matching label: 'not fips' with test having fips label should be false",
			testLabels:  []string{"fips"},
			expected:    false,
		},
		{
			name:        "not_without_matching_label",
			expression:  "not fips",
			description: "NOT without matching label: 'not fips' with test not having fips label should be true",
			testLabels:  []string{"logs"},
			expected:    true,
		},
		{
			name:        "nested_parentheses",
			expression:  "((fips and logs) or (vector and metrics)) and not experimental",
			description: "Deeply nested expression with multiple groupings",
			testLabels:  []string{"vector", "metrics"},
			expected:    true,
		},
		{
			name:        "empty_expression",
			expression:  "",
			description: "Empty expression should match all tests (no filtering)",
			testLabels:  []string{"any", "labels"},
			expected:    true,
		},
		{
			name:        "whitespace_only",
			expression:  "   ",
			description: "Whitespace-only expression should match all tests",
			testLabels:  []string{"any", "labels"},
			expected:    true,
		},

		// Case sensitivity and formatting
		{
			name:        "case_insensitive_operators",
			expression:  "FIPS AND LOGS OR NOT SLOW",
			description: "Operators should be case-insensitive",
			testLabels:  []string{"fips", "logs"},
			expected:    true,
		},
		{
			name:        "mixed_case_operators",
			expression:  "fips And logs Or Not slow",
			description: "Mixed case operators should work",
			testLabels:  []string{"fips", "logs"},
			expected:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the complete parsing and evaluation pipeline
			result, err := evaluateLabelExpression(tt.testLabels, tt.expression)
			require.NoError(t, err, "Failed to evaluate expression: %s", tt.expression)

			assert.Equal(t, tt.expected, result,
				"Expression: '%s'\nDescription: %s\nTest labels: %v\nExpected: %v, Got: %v",
				tt.expression, tt.description, tt.testLabels, tt.expected, result)
		})
	}
}

// TestOperatorPrecedence specifically tests the operator precedence rules.
// This serves as documentation for the precedence: NOT > AND > OR
func TestOperatorPrecedence(t *testing.T) {
	tests := []struct {
		name           string
		expression     string
		testLabels     []string
		expected       bool
		precedenceNote string
	}{
		{
			name:           "not_binds_tighter_than_and",
			expression:     "not a and b",
			testLabels:     []string{"b"},
			expected:       true,
			precedenceNote: "Should parse as: (not a) and b",
		},
		{
			name:           "and_binds_tighter_than_or",
			expression:     "a or b and c",
			testLabels:     []string{"a"},
			expected:       true,
			precedenceNote: "Should parse as: a or (b and c)",
		},
		{
			name:           "complex_precedence",
			expression:     "not a or b and c",
			testLabels:     []string{"b", "c"},
			expected:       true,
			precedenceNote: "Should parse as: (not a) or (b and c)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := evaluateLabelExpression(tt.testLabels, tt.expression)
			require.NoError(t, err, "Failed to evaluate expression: %s", tt.expression)

			assert.Equal(t, tt.expected, result,
				"Expression: '%s'\nPrecedence note: %s\nTest labels: %v",
				tt.expression, tt.precedenceNote, tt.testLabels)
		})
	}
}

// TestLabelFilteringIntegration tests the complete label filtering integration
// by directly testing the filtering logic without mocking the testing.T structure.
func TestLabelFilteringIntegration(t *testing.T) {
	// Save original os.Args to restore later
	originalArgs := make([]string, len(os.Args))
	copy(originalArgs, os.Args)

	defer func() {
		os.Args = originalArgs
	}()

	tests := []struct {
		name        string
		labelFilter string
		testLabels  []string
		shouldSkip  bool
		description string
	}{
		{
			name:        "no_filter",
			labelFilter: "",
			testLabels:  []string{"any", "labels"},
			shouldSkip:  false,
			description: "When no filter is specified, all tests should run",
		},
		{
			name:        "simple_match",
			labelFilter: "fips",
			testLabels:  []string{"fips", "logs"},
			shouldSkip:  false,
			description: "Test with matching label should run",
		},
		{
			name:        "simple_no_match",
			labelFilter: "fips",
			testLabels:  []string{"logs", "metrics"},
			shouldSkip:  true,
			description: "Test without matching label should be skipped",
		},
		{
			name:        "complex_expression_match",
			labelFilter: "(fips or vector) and not slow",
			testLabels:  []string{"fips", "integration"},
			shouldSkip:  false,
			description: "Test matching complex expression should run",
		},
		{
			name:        "complex_expression_no_match",
			labelFilter: "(fips or vector) and not slow",
			testLabels:  []string{"fips", "slow"},
			shouldSkip:  true,
			description: "Test not matching complex expression should be skipped",
		},
		{
			name:        "label_with_dash",
			labelFilter: "not fluent-bit",
			testLabels:  []string{"fluent-bit"},
			shouldSkip:  true,
			description: "Test with label containing dash should be handled correctly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up the command line args to simulate the label filter
			if tt.labelFilter != "" {
				os.Args = []string{"test", "-labels=" + tt.labelFilter}
			} else {
				os.Args = []string{"test"}
			}

			// Test the filtering logic directly
			labelFilterExpr := findLabelFilterExpression()
			if labelFilterExpr == "" && tt.labelFilter == "" {
				require.False(t, tt.shouldSkip, "Test should not be skipped when no filter")
				return
			}

			if labelFilterExpr != "" {
				shouldRun, err := evaluateLabelExpression(tt.testLabels, labelFilterExpr)
				require.NoError(t, err, "Failed to evaluate expression: %s", labelFilterExpr)

				actualShouldSkip := !shouldRun

				assert.Equal(t, tt.shouldSkip, actualShouldSkip,
					"Filter: '%s', Labels: %v, Description: %s",
					tt.labelFilter, tt.testLabels, tt.description)
			}
		})
	}
}

// TestSyntaxConversion tests the conversion from legacy syntax to expr-lang syntax
func TestSyntaxConversion(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple_and",
			input:    "fips and logs",
			expected: "fips && logs",
		},
		{
			name:     "simple_or",
			input:    "fips or logs",
			expected: "fips || logs",
		},
		{
			name:     "simple_not",
			input:    "not fips",
			expected: "! fips",
		},
		{
			name:     "complex_expression",
			input:    "(fips or logs) and not slow",
			expected: "(fips || logs) && ! slow",
		},
		{
			name:     "uppercase_operators",
			input:    "FIPS AND LOGS OR NOT SLOW",
			expected: "fips && logs || ! slow",
		},
		{
			name:     "mixed_case",
			input:    "Fips And Logs Or Not Slow",
			expected: "fips && logs || ! slow",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertLabelExpressionSyntax(tt.input)
			assert.Equal(t, tt.expected, result,
				"Conversion of: '%s'", tt.input)
		})
	}
}
