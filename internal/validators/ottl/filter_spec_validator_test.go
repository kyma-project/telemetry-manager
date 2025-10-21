package ottl

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
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
	runFilterValidatorTestCases(t, "spanevent", SignalTypeTrace, filterSpanEventContextTestCases())
	runFilterValidatorTestCases(t, "log", SignalTypeLog, filterLogContextTestCases())
	runFilterValidatorTestCases(t, "metric", SignalTypeMetric, filterMetricContextTestCases())
	runFilterValidatorTestCases(t, "datapoint", SignalTypeMetric, filterDataPointContextTestCases())
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

func filterLogContextTestCases() []filterResourceContextTestCase {
	return []filterResourceContextTestCase{
		{
			name: "valid filter spec - simple condition",
			filters: []telemetryv1alpha1.FilterSpec{
				{
					Conditions: []string{
						`log.severity_text == "ERROR"`,
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
						`log.severity_text == "ERROR"`,
						`log.body == "Database connection failed"`,
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
						`log.attributes["service.name"] == "auth-service"`,
					},
				},
			},
			isErrorExpected: false,
		},
		{
			name: "valid filter spec - severity number",
			filters: []telemetryv1alpha1.FilterSpec{
				{
					Conditions: []string{
						`log.severity_number >= 17`,
					},
				},
			},
			isErrorExpected: false,
		},
		{
			name: "valid filter spec - timestamp access",
			filters: []telemetryv1alpha1.FilterSpec{
				{
					Conditions: []string{
						`log.time_unix_nano != nil`,
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
						`log.severity_text == "ERROR`,
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
						`get(log.body) == "error message"`,
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
						`truncate_all(log.attributes, 100)`,
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
						`log.invalid == "value"`,
					},
				},
			},
			isErrorExpected: true,
		},
	}
}

func filterMetricContextTestCases() []filterResourceContextTestCase {
	return []filterResourceContextTestCase{
		{
			name: "valid filter spec - simple condition",
			filters: []telemetryv1alpha1.FilterSpec{
				{
					Conditions: []string{
						`metric.name == "http_requests_total"`,
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
						`metric.name == "http_requests_total"`,
						`metric.type == 1`,
					},
				},
			},
			isErrorExpected: false,
		},
		{
			name: "valid filter spec - unit access",
			filters: []telemetryv1alpha1.FilterSpec{
				{
					Conditions: []string{
						`metric.unit == "ms"`,
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
						`metric.name == "http_requests_total`,
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
						`get(metric.name) == "http_requests_total"`,
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
						`prettify(metric.name) == "http_requests_total"`,
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
						`metric.invalid == "value"`,
					},
				},
			},
			isErrorExpected: true,
		},
	}
}

func filterDataPointContextTestCases() []filterResourceContextTestCase {
	return []filterResourceContextTestCase{
		{
			name: "valid filter spec - simple condition",
			filters: []telemetryv1alpha1.FilterSpec{
				{
					Conditions: []string{
						`datapoint.value_int > 100`,
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
						`datapoint.value_int > 100`,
						`datapoint.time_unix_nano != nil`,
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
						`datapoint.attributes["service.name"] == "auth-service"`,
					},
				},
			},
			isErrorExpected: false,
		},
		{
			name: "valid filter spec - timestamp access",
			filters: []telemetryv1alpha1.FilterSpec{
				{
					Conditions: []string{
						`datapoint.start_time_unix_nano != nil`,
					},
				},
			},
			isErrorExpected: false,
		},
		{
			name: "valid filter spec - exemplars access",
			filters: []telemetryv1alpha1.FilterSpec{
				{
					Conditions: []string{
						`datapoint.exemplars != nil`,
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
						`datapoint.value > 100`,
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
						`get(datapoint.value_int) > 100`,
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
						`truncate_all(datapoint.attributes, 100)`,
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
						`datapoint.invalid == "value"`,
					},
				},
			},
			isErrorExpected: true,
		},
	}
}
