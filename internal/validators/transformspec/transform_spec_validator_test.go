package transformspec

import (
	"errors"
	"testing"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/stretchr/testify/require"
)

type testCase struct {
	name            string
	transforms      []telemetryv1alpha1.TransformSpec
	isErrorExpected bool
}

func TestValidateForLogPipeline(t *testing.T) {
	tests := []testCase{
		{
			name: "[log context] valid transform spec with both statement and condition",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IsMatch(log.attributes[\"test\"], \"bar\")"},
					Statements: []string{"set(log.attributes[\"test\"], \"foo\")"},
				},
			},
			isErrorExpected: false,
		},
		{
			name: "[log context] valid transform spec with condition only",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IsMatch(log.attributes[\"test\"], \"bar\")"},
				},
			},
			isErrorExpected: false,
		},
		{
			name: "[log context] valid transform spec with statement only",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Statements: []string{"set(log.attributes[\"test\"], \"foo\")"},
				},
			},
			isErrorExpected: false,
		},
		{
			name: "[log context] valid statement but incorrectly used as a condition",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"set(log.attributes[\"test\"], \"foo\")"},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "[log context] invalid context",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IsMatch(datapoint.attributes[\"test\"], \"bar\")"},
					Statements: []string{"set(datapoint.attributes[\"test\"], \"foo\")"},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "[log context] invalid function name in condition",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IisMatch(log.attributes[\"test\"], \"bar\")"},
					Statements: []string{"set(log.attributes[\"test\"], \"foo\")"},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "[log context] invalid path in condition",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IsMatch(log.aattributes[\"test\"], \"bar\")"},
					Statements: []string{"set(log.attributes[\"test\"], \"foo\")"},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "[log context] invalid syntax in condition",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IsMatch(log.attributes[\"test\"],, \"bar\")"},
					Statements: []string{"set(log.attributes[\"test\"], \"foo\")"},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "[log context] invalid function name in statement",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IsMatch(log.attributes[\"test\"], \"bar\")"},
					Statements: []string{"sset(log.attributes[\"test\"], \"foo\")"},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "[log context] invalid path in statement",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IsMatch(log.attributes[\"test\"], \"bar\")"},
					Statements: []string{"set(log.aattributes[\"test\"], \"foo\")"},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "[log context] invalid syntax in statement",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IsMatch(log.attributes[\"test\"], \"bar\")"},
					Statements: []string{"set(log.attributes[\"test\"],, \"foo\")"},
				},
			},
			isErrorExpected: true,
		},
	}

	tests = append(tests, resourceContextTestCases()...)
	tests = append(tests, scopeContextTestCases()...)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			validator, err := New(SignalTypeLog)
			require.NoError(t, err)

			err = validator.Validate(test.transforms)
			if test.isErrorExpected {
				require.Error(t, err)
				require.True(t, IsInvalidTransformSpecError(err))

				var invalidTransformSpecErr *InvalidTransformSpecError
				require.True(t, errors.As(err, &invalidTransformSpecErr))
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// resourceContextTestCases generates test cases for the "resource" context transformations.
// The "resource" context is common in log_statements, metric_statements and trace_statements
// For more info, check https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/transformprocessor#config
func resourceContextTestCases() []testCase {
	return []testCase{
		{
			name: "[resource context] valid transform spec with both statement and condition",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IsMatch(resource.attributes[\"test\"], \"bar\")"},
					Statements: []string{"set(resource.attributes[\"test\"], \"foo\")"},
				},
			},
			isErrorExpected: false,
		},
		{
			name: "[resource context] valid transform spec with condition only",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IsMatch(resource.attributes[\"test\"], \"bar\")"},
				},
			},
			isErrorExpected: false,
		},
		{
			name: "[resource context] valid transform spec with statement only",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Statements: []string{"set(resource.attributes[\"test\"], \"foo\")"},
				},
			},
			isErrorExpected: false,
		},
		{
			name: "[resource context] valid statement but incorrectly used as a condition",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"set(resource.attributes[\"test\"], \"foo\")"},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "[resource context] invalid context",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IsMatch(datapoint.attributes[\"test\"], \"bar\")"},
					Statements: []string{"set(datapoint.attributes[\"test\"], \"foo\")"},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "[resource context] invalid function name in condition",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IisMatch(resource.attributes[\"test\"], \"bar\")"},
					Statements: []string{"set(resource.attributes[\"test\"], \"foo\")"},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "[resource context] invalid path in condition",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IsMatch(resource.aattributes[\"test\"], \"bar\")"},
					Statements: []string{"set(resource.attributes[\"test\"], \"foo\")"},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "[resource context] invalid syntax in condition",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IsMatch(resource.attributes[\"test\"],, \"bar\")"},
					Statements: []string{"set(resource.attributes[\"test\"], \"foo\")"},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "[resource context] invalid function name in statement",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IsMatch(resource.attributes[\"test\"], \"bar\")"},
					Statements: []string{"sset(resource.attributes[\"test\"], \"foo\")"},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "[resource context] invalid path in statement",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IsMatch(resource.attributes[\"test\"], \"bar\")"},
					Statements: []string{"set(resource.aattributes[\"test\"], \"foo\")"},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "[resource context] invalid syntax in statement",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IsMatch(resource.attributes[\"test\"], \"bar\")"},
					Statements: []string{"set(resource.attributes[\"test\"],, \"foo\")"},
				},
			},
			isErrorExpected: true,
		},
	}
}

// scopeContextTestCases generates test cases for the "scope" context transformations.
// The "scope" context is common in log_statements, metric_statements and trace_statements
// For more info, check https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/transformprocessor#config
func scopeContextTestCases() []testCase {
	return []testCase{
		{
			name: "[scope context] valid transform spec with both statement and condition",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IsMatch(scope.attributes[\"test\"], \"bar\")"},
					Statements: []string{"set(scope.attributes[\"test\"], \"foo\")"},
				},
			},
			isErrorExpected: false,
		},
		{
			name: "[scope context] valid transform spec with condition only",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IsMatch(scope.attributes[\"test\"], \"bar\")"},
				},
			},
			isErrorExpected: false,
		},
		{
			name: "[scope context] valid transform spec with statement only",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Statements: []string{"set(scope.attributes[\"test\"], \"foo\")"},
				},
			},
			isErrorExpected: false,
		},
		{
			name: "[scope context] valid statement but incorrectly used as a condition",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"set(scope.attributes[\"test\"], \"foo\")"},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "[scope context] invalid context",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IsMatch(datapoint.attributes[\"test\"], \"bar\")"},
					Statements: []string{"set(datapoint.attributes[\"test\"], \"foo\")"},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "[scope context] invalid function name in condition",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IisMatch(scope.attributes[\"test\"], \"bar\")"},
					Statements: []string{"set(scope.attributes[\"test\"], \"foo\")"},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "[scope context] invalid path in condition",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IsMatch(scope.aattributes[\"test\"], \"bar\")"},
					Statements: []string{"set(scope.attributes[\"test\"], \"foo\")"},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "[scope context] invalid syntax in condition",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IsMatch(scope.attributes[\"test\"],, \"bar\")"},
					Statements: []string{"set(scope.attributes[\"test\"], \"foo\")"},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "[scope context] invalid function name in statement",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IsMatch(scope.attributes[\"test\"], \"bar\")"},
					Statements: []string{"sset(scope.attributes[\"test\"], \"foo\")"},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "[scope context] invalid path in statement",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IsMatch(scope.attributes[\"test\"], \"bar\")"},
					Statements: []string{"set(scope.aattributes[\"test\"], \"foo\")"},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "[scope context] invalid syntax in statement",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IsMatch(scope.attributes[\"test\"], \"bar\")"},
					Statements: []string{"set(scope.attributes[\"test\"],, \"foo\")"},
				},
			},
			isErrorExpected: true,
		},
	}
}
