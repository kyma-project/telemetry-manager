package ottl

import (
	"errors"
	"fmt"
	"testing"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/stretchr/testify/require"
)

type filterResourceContextTestCase struct {
	name            string
	filters         []telemetryv1alpha1.FilterSpec
	isErrorExpected bool
}

func TestFilterValidator(t *testing.T) {
	for _, signalType := range []SignalType{SignalTypeLog, SignalTypeMetric, SignalTypeTrace} {
		runFilterValidatorTestCases(t, "resource", signalType, filterResourceContextTestCases())
		runFilterValidatorTestCases(t, "scope", signalType, filterScopeContextTestCases())
	}

	runFilterValidatorTestCases(t, "span", SignalTypeTrace, filterSpanContextTestCases())
	runFilterValidatorTestCases(t, "spanevent", SignalTypeLog, filterSpanEventContextTestCases())
}

func runFilterValidatorTestCases(t *testing.T, context string, signalType SignalType, tests []filterResourceContextTestCase) {
	t.Helper()

	t.Run(fmt.Sprintf("%s_%s", signalType, context), func(t *testing.T) {
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				validator, err := NewFilterSpecValidator(signalType)
				require.NoError(t, err)

				err = validator.Validate(test.filters)
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
	})
}

func filterResourceContextTestCases() []filterResourceContextTestCase {
	return []filterResourceContextTestCase{
		{
			name: "valid filter spec - simple condition",
			filters: []telemetryv1alpha1.FilterSpec{
				{
					Conditions: []string{
						`resource.attributes["service.name"] == "auth-service"`,
					},
				},
			},
			isErrorExpected: false,
		},
		{
			name: "valid filter spec - multiple conditions",
			filters: []telemetryv1alpha1.FilterSpec{
				{
					Conditions: []string{
						`resource.attributes["service.name"] == "auth-service"`,
						`resource.attributes["environment"] == "production"`,
					},
				},
			},
			isErrorExpected: false,
		},
		{
			name: "invalid filter spec - missing context",
			filters: []telemetryv1alpha1.FilterSpec{
				{
					Conditions: []string{
						`attributes["service.name"] == "auth-service"`,
					},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "invalid filter spec - invalid syntax",
			filters: []telemetryv1alpha1.FilterSpec{
				{
					Conditions: []string{
						`resource.attributes["service.name" == "auth-service"`,
					},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "invalid filter spec - invalid function",
			filters: []telemetryv1alpha1.FilterSpec{
				{
					Conditions: []string{
						`get(resource.attributes["service.name"]) == "auth-service"`,
					},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "invalid filter spec - converter function",
			filters: []telemetryv1alpha1.FilterSpec{
				{
					Conditions: []string{
						`truncate_all(resource.attributes, 100)`,
					},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "invalid filter spec - invaid path",
			filters: []telemetryv1alpha1.FilterSpec{
				{
					Conditions: []string{
						`resource.invalid["service.name"] == "auth-service"`,
					},
				},
			},
			isErrorExpected: true,
		},
	}
}

func filterScopeContextTestCases() []filterResourceContextTestCase {
	return []filterResourceContextTestCase{
		{
			name: "valid filter spec - simple condition",
			filters: []telemetryv1alpha1.FilterSpec{
				{
					Conditions: []string{
						`scope.name == "io.opentelemetry.contrib.mongodb"`,
					},
				},
			},
			isErrorExpected: false,
		},
		{
			name: "valid filter spec - multiple conditions",
			filters: []telemetryv1alpha1.FilterSpec{
				{
					Conditions: []string{
						`scope.name == "io.opentelemetry.contrib.mongodb"`,
						`scope.version == "1.0.0"`,
					},
				},
			},
			isErrorExpected: false,
		},
		{
			name: "valid filter spec - attributes access",
			filters: []telemetryv1alpha1.FilterSpec{
				{
					Conditions: []string{
						`scope.attributes["custom.key"] == "value"`,
					},
				},
			},
			isErrorExpected: false,
		},
		{
			name: "invalid filter spec - invalid syntax",
			filters: []telemetryv1alpha1.FilterSpec{
				{
					Conditions: []string{
						`scope.name == "io.opentelemetry.contrib.mongodb`,
					},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "invalid filter spec - invalid function",
			filters: []telemetryv1alpha1.FilterSpec{
				{
					Conditions: []string{
						`get(scope.name) == "io.opentelemetry.contrib.mongodb"`,
					},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "invalid filter spec - converter function",
			filters: []telemetryv1alpha1.FilterSpec{
				{
					Conditions: []string{
						`truncate_all(scope.attributes, 100)`,
					},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "invalid filter spec - invalid path",
			filters: []telemetryv1alpha1.FilterSpec{
				{
					Conditions: []string{
						`scope.invalid == "value"`,
					},
				},
			},
			isErrorExpected: true,
		},
	}
}

func filterSpanContextTestCases() []filterResourceContextTestCase {
	return []filterResourceContextTestCase{
		{
			name: "valid filter spec - simple condition",
			filters: []telemetryv1alpha1.FilterSpec{
				{
					Conditions: []string{
						`span.name == "HTTP GET"`,
					},
				},
			},
			isErrorExpected: false,
		},
		{
			name: "valid filter spec - multiple conditions",
			filters: []telemetryv1alpha1.FilterSpec{
				{
					Conditions: []string{
						`span.name == "HTTP GET"`,
						`span.status.code == 1`,
					},
				},
			},
			isErrorExpected: false,
		},
		{
			name: "valid filter spec - attributes access",
			filters: []telemetryv1alpha1.FilterSpec{
				{
					Conditions: []string{
						`span.attributes["http.method"] == "GET"`,
					},
				},
			},
			isErrorExpected: false,
		},
		{
			name: "valid filter spec - span context access",
			filters: []telemetryv1alpha1.FilterSpec{
				{
					Conditions: []string{
						`span.span_id != nil`,
					},
				},
			},
			isErrorExpected: false,
		},
		{
			name: "invalid filter spec - invalid syntax",
			filters: []telemetryv1alpha1.FilterSpec{
				{
					Conditions: []string{
						`span.name == "HTTP GET`,
					},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "invalid filter spec - invalid function",
			filters: []telemetryv1alpha1.FilterSpec{
				{
					Conditions: []string{
						`get(span.name) == "HTTP GET"`,
					},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "invalid filter spec - converter function",
			filters: []telemetryv1alpha1.FilterSpec{
				{
					Conditions: []string{
						`truncate_all(span.attributes, 100)`,
					},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "invalid filter spec - invalid path",
			filters: []telemetryv1alpha1.FilterSpec{
				{
					Conditions: []string{
						`span.invalid == "value"`,
					},
				},
			},
			isErrorExpected: true,
		},
	}
}

func filterSpanEventContextTestCases() []filterResourceContextTestCase {
	return []filterResourceContextTestCase{
		{
			name: "invalid filter spec - spanevent not supported",
			filters: []telemetryv1alpha1.FilterSpec{
				{
					Conditions: []string{
						`spanevent.name == "exception"`,
					},
				},
			},
			isErrorExpected: true,
		},
		{
			name: "invalid filter spec - invalid syntax",
			filters: []telemetryv1alpha1.FilterSpec{
				{
					Conditions: []string{
						`spanevent.name == "exception`,
					},
				},
			},
			isErrorExpected: true,
		},
	}
}
