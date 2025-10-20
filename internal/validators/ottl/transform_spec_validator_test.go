package ottl

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

type transformTestCase struct {
	name            string
	transforms      []telemetryv1alpha1.TransformSpec
	isErrorExpected bool
}

func TestValidateLogPipelineTransforms(t *testing.T) {
	tests := transformResourceContextTestCases()
	tests = append(tests, transformScopeContextTestCases()...)
	tests = append(tests, transformLogContextTestCases()...)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			validator, err := NewTransformSpecValidator(SignalTypeLog)
			require.NoError(t, err)

			err = validator.Validate(test.transforms)
			if test.isErrorExpected {
				require.Error(t, err)
				require.True(t, IsInvalidOTTLSpecError(err))

				var invalidTransformSpecErr *InvalidOTTLSpecError
				require.True(t, errors.As(err, &invalidTransformSpecErr))
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateTracePipelineTransforms(t *testing.T) {
	tests := transformResourceContextTestCases()
	tests = append(tests, transformScopeContextTestCases()...)
	tests = append(tests, transformSpanContextTestCases()...)
	tests = append(tests, transformSpanEventContextTestCases()...)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			validator, err := NewTransformSpecValidator(SignalTypeTrace)
			require.NoError(t, err)

			err = validator.Validate(test.transforms)
			if test.isErrorExpected {
				require.Error(t, err)
				require.True(t, IsInvalidOTTLSpecError(err))

				var invalidTransformSpecErr *InvalidOTTLSpecError
				require.True(t, errors.As(err, &invalidTransformSpecErr))
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestMetricPipelineTransforms(t *testing.T) {
	tests := transformResourceContextTestCases()
	tests = append(tests, transformScopeContextTestCases()...)
	tests = append(tests, transformMetricContextTestCases()...)
	tests = append(tests, transformDataPointContextTestCases()...)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			validator, err := NewTransformSpecValidator(SignalTypeMetric)
			require.NoError(t, err)

			err = validator.Validate(test.transforms)
			if test.isErrorExpected {
				require.Error(t, err)
				require.True(t, IsInvalidOTTLSpecError(err))

				var invalidTransformSpecErr *InvalidOTTLSpecError
				require.True(t, errors.As(err, &invalidTransformSpecErr))
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// transformResourceContextTestCases generates test cases for the validation of the "resource" context
// The "resource" context is common in log_statements, metric_statements and trace_statements
// For more info, check https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/transformprocessor#config
func transformResourceContextTestCases() []transformTestCase {
	return []transformTestCase{
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
			name: "[resource context] valid condition but incorrectly used as a statement",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Statements: []string{"IsMatch(resource.attributes[\"test\"], \"bar\")"}},
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

// transformScopeContextTestCases generates test cases for the validation of the "scope" context
// The "scope" context is common in log_statements, metric_statements and trace_statements
// For more info, check https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/transformprocessor#config
func transformScopeContextTestCases() []transformTestCase {
	return []transformTestCase{
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
			name: "[scope context] valid condition but incorrectly used as a statement",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Statements: []string{"IsMatch(scope.attributes[\"test\"], \"bar\")"},
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

// transformLogContextTestCases generates test cases for the validation of the "log" context
func transformLogContextTestCases() []transformTestCase {
	return []transformTestCase{
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
			name: "[log context] valid condition but incorrectly used as a statement",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Statements: []string{"IsMatch(log.attributes[\"test\"], \"bar\")"},
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
}

// transformSpanContextTestCases generates test cases for the validation of the "span" context
func transformSpanContextTestCases() []transformTestCase {
	return []transformTestCase{
		{
			name: "[span context] valid transform spec with both statement and condition",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IsMatch(span.attributes[\"test\"], \"bar\")"},
					Statements: []string{"set(span.attributes[\"test\"], \"foo\")"},
				},
			},
			isErrorExpected: false,
		},
		{
			name: "[span context] valid transform spec with condition only",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IsMatch(span.attributes[\"test\"], \"bar\")"},
				},
			},
			isErrorExpected: false,
		},
		{
			name: "[span context] valid transform spec with statement only",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Statements: []string{"set(span.attributes[\"test\"], \"foo\")"},
				},
			},
			isErrorExpected: false,
		},
		{
			name: "[span context] valid transform spec with IsRootSpan() function",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Statements: []string{"set(span.attributes[\"isRoot\"], \"true\") where IsRootSpan()"},
				},
			},
			isErrorExpected: false,
		},
		{
			name: "[span context] valid statement but incorrectly used as a condition",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"set(span.attributes[\"test\"], \"foo\")"},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "[span context] valid condition but incorrectly used as a statement",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Statements: []string{"IsMatch(span.attributes[\"test\"], \"bar\")"},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "[span context] invalid context",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IsMatch(datapoint.attributes[\"test\"], \"bar\")"},
					Statements: []string{"set(datapoint.attributes[\"test\"], \"foo\")"},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "[span context] invalid function name in condition",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IisMatch(span.attributes[\"test\"], \"bar\")"},
					Statements: []string{"set(span.attributes[\"test\"], \"foo\")"},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "[span context] invalid path in condition",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IsMatch(span.aattributes[\"test\"], \"bar\")"},
					Statements: []string{"set(span.attributes[\"test\"], \"foo\")"},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "[span context] invalid syntax in condition",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IsMatch(span.attributes[\"test\"],, \"bar\")"},
					Statements: []string{"set(span.attributes[\"test\"], \"foo\")"},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "[span context] invalid function name in statement",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IsMatch(span.attributes[\"test\"], \"bar\")"},
					Statements: []string{"sset(span.attributes[\"test\"], \"foo\")"},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "[span context] invalid path in statement",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IsMatch(span.attributes[\"test\"], \"bar\")"},
					Statements: []string{"set(span.aattributes[\"test\"], \"foo\")"},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "[span context] invalid syntax in statement",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IsMatch(span.attributes[\"test\"], \"bar\")"},
					Statements: []string{"set(span.attributes[\"test\"],, \"foo\")"},
				},
			},
			isErrorExpected: true,
		},
	}
}

// transformSpanEventContextTestCases generates test cases for the validation of the "spanevent" context
func transformSpanEventContextTestCases() []transformTestCase {
	return []transformTestCase{
		{
			name: "[spanevent context] valid transform spec with both statement and condition",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IsMatch(spanevent.attributes[\"test\"], \"bar\")"},
					Statements: []string{"set(spanevent.attributes[\"test\"], \"foo\")"},
				},
			},
			isErrorExpected: false,
		},
		{
			name: "[spanevent context] valid transform spec with condition only",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IsMatch(spanevent.attributes[\"test\"], \"bar\")"},
				},
			},
			isErrorExpected: false,
		},
		{
			name: "[spanevent context] valid transform spec with statement only",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Statements: []string{"set(spanevent.attributes[\"test\"], \"foo\")"},
				},
			},
			isErrorExpected: false,
		},
		{
			name: "[spanevent context] valid statement but incorrectly used as a condition",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"set(spanevent.attributes[\"test\"], \"foo\")"},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "[spanevent context] valid condition but incorrectly used as a statement",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Statements: []string{"IsMatch(spanevent.attributes[\"test\"], \"bar\")"},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "[spanevent context] invalid context",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IsMatch(datapoint.attributes[\"test\"], \"bar\")"},
					Statements: []string{"set(datapoint.attributes[\"test\"], \"foo\")"},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "[spanevent context] invalid function name in condition",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IisMatch(spanevent.attributes[\"test\"], \"bar\")"},
					Statements: []string{"set(spanevent.attributes[\"test\"], \"foo\")"},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "[spanevent context] invalid path in condition",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IsMatch(spanevent.aattributes[\"test\"], \"bar\")"},
					Statements: []string{"set(spanevent.attributes[\"test\"], \"foo\")"},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "[spanevent context] invalid syntax in condition",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IsMatch(spanevent.attributes[\"test\"],, \"bar\")"},
					Statements: []string{"set(spanevent.attributes[\"test\"], \"foo\")"},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "[spanevent context] invalid function name in statement",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IsMatch(spanevent.attributes[\"test\"], \"bar\")"},
					Statements: []string{"sset(spanevent.attributes[\"test\"], \"foo\")"},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "[spanevent context] invalid path in statement",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IsMatch(spanevent.attributes[\"test\"], \"bar\")"},
					Statements: []string{"set(spanevent.aattributes[\"test\"], \"foo\")"},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "[spanevent context] invalid syntax in statement",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IsMatch(spanevent.attributes[\"test\"], \"bar\")"},
					Statements: []string{"set(spanevent.attributes[\"test\"],, \"foo\")"},
				},
			},
			isErrorExpected: true,
		},
	}
}

// transformMetricContextTestCases generates test cases for the validation of the "metric" context
func transformMetricContextTestCases() []transformTestCase {
	return []transformTestCase{
		{
			name: "[metric context] valid transform spec with both statement and condition",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IsMatch(metric.name, \"bar\")"},
					Statements: []string{"set(metric.name, \"foo\")"},
				},
			},
			isErrorExpected: false,
		},
		{
			name: "[metric context] valid transform spec with condition only",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IsMatch(metric.name, \"bar\")"},
				},
			},
			isErrorExpected: false,
		},
		{
			name: "[metric context] valid transform spec with statement only",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Statements: []string{"set(metric.name, \"foo\")"},
				},
			},
			isErrorExpected: false,
		},
		{
			name: "[metric context] valid statement but incorrectly used as a condition",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"set(metric.name, \"foo\")"},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "[metric context] valid condition but incorrectly used as a statement",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Statements: []string{"IsMatch(metric.name, \"bar\")"},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "[metric context] invalid context",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IsMatch(log.attributes[\"test\"], \"bar\")"},
					Statements: []string{"set(log.attributes[\"test\"], \"foo\")"},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "[metric context] invalid function name in condition",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IisMatch(metric.name, \"bar\")"},
					Statements: []string{"set(metric.name, \"foo\")"},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "[metric context] invalid path in condition",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IsMatch(metric.nname, \"bar\")"},
					Statements: []string{"set(metric.name, \"foo\")"},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "[metric context] invalid syntax in condition",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IsMatch(metric.name,, \"bar\")"},
					Statements: []string{"set(metric.name, \"foo\")"},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "[metric context] invalid function name in statement",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IsMatch(metric.name, \"bar\")"},
					Statements: []string{"sset(metric.name, \"foo\")"},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "[metric context] invalid path in statement",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IsMatch(metric.name, \"bar\")"},
					Statements: []string{"set(metric.nname, \"foo\")"},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "[metric context] invalid syntax in statement",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IsMatch(metric.name, \"bar\")"},
					Statements: []string{"set(metric.name,, \"foo\")"},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "[metric context] context inference is not possible",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Statements: []string{
						"convert_sum_to_gauge() where metric.name == \"system.processes.count\"",
						"limit(datapoint.attributes, 100, [\"host.name\"])",
					},
				},
			},
			isErrorExpected: true,
		},
	}
}

// transformDataPointContextTestCases generates test cases for the validation of the "datapoint" context
func transformDataPointContextTestCases() []transformTestCase {
	return []transformTestCase{
		{
			name: "[datapoint context] valid transform spec with both statement and condition",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IsMatch(datapoint.attributes[\"test\"], \"bar\")"},
					Statements: []string{"set(datapoint.attributes[\"test\"], \"foo\")"},
				},
			},
			isErrorExpected: false,
		},
		{
			name: "[datapoint context] valid transform spec with condition only",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IsMatch(datapoint.attributes[\"test\"], \"bar\")"},
				},
			},
			isErrorExpected: false,
		},
		{
			name: "[datapoint context] valid transform spec with statement only",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Statements: []string{"set(datapoint.attributes[\"test\"], \"foo\")"},
				},
			},
			isErrorExpected: false,
		},
		{
			name: "[datapoint context] valid statement but incorrectly used as a condition",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"set(datapoint.attributes[\"test\"], \"foo\")"},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "[datapoint context] valid condition but incorrectly used as a statement",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Statements: []string{"IsMatch(datapoint.attributes[\"test\"], \"bar\")"},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "[datapoint context] invalid context",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IsMatch(log.attributes[\"test\"], \"bar\")"},
					Statements: []string{"set(log.attributes[\"test\"], \"foo\")"},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "[datapoint context] invalid function name in condition",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IisMatch(datapoint.attributes[\"test\"], \"bar\")"},
					Statements: []string{"set(datapoint.attributes[\"test\"], \"foo\")"},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "[datapoint context] invalid path in condition",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IsMatch(datapoint.aattributes[\"test\"], \"bar\")"},
					Statements: []string{"set(datapoint.attributes[\"test\"], \"foo\")"},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "[datapoint context] invalid syntax in condition",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IsMatch(datapoint.attributes[\"test\"],, \"bar\")"},
					Statements: []string{"set(datapoint.attributes[\"test\"], \"foo\")"},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "[datapoint context] invalid function name in statement",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IsMatch(datapoint.attributes[\"test\"], \"bar\")"},
					Statements: []string{"sset(datapoint.attributes[\"test\"], \"foo\")"},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "[datapoint context] invalid path in statement",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IsMatch(datapoint.attributes[\"test\"], \"bar\")"},
					Statements: []string{"set(datapoint.aattributes[\"test\"], \"foo\")"},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "[datapoint context] invalid syntax in statement",
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Conditions: []string{"IsMatch(datapoint.attributes[\"test\"], \"bar\")"},
					Statements: []string{"set(datapoint.attributes[\"test\"],, \"foo\")"},
				},
			},
			isErrorExpected: true,
		},
	}
}
