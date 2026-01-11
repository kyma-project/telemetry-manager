package ottl

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
)

type filterTestCase struct {
	name            string
	conditions      []string
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
	runFilterValidatorTestCases(t, "mixed", SignalTypeMetric, filterMixedMetricContextTestCases())

	t.Run("invalid signal type", func(t *testing.T) {
		_, err := NewFilterSpecValidator("invalid_signal")
		require.Error(t, err)
	})
}

func runFilterValidatorTestCases(t *testing.T, context string, signalType SignalType, tests []filterTestCase) {
	t.Helper()

	t.Run(fmt.Sprintf("%s/%s context", signalType, context), func(t *testing.T) {
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				validator, err := NewFilterSpecValidator(signalType)
				require.NoError(t, err)

				filters := []telemetryv1beta1.FilterSpec{{Conditions: test.conditions}}

				err = validator.Validate(filters)
				if test.isErrorExpected {
					require.Error(t, err)
					require.True(t, IsInvalidOTTLSpecError(err))

					var typedErr *InvalidOTTLSpecError
					require.True(t, errors.As(err, &typedErr))
					require.Contains(t, typedErr.Error(), "invalid FilterSpec")
				} else {
					require.NoError(t, err)
				}
			})
		}
	})
}

func filterResourceContextTestCases() []filterTestCase {
	return []filterTestCase{
		{
			name:       "simple condition",
			conditions: []string{`resource.attributes["service.name"] == "auth-service"`},
		},
		{
			name: "multiple conditions",
			conditions: []string{
				`resource.attributes["service.name"] == "auth-service"`,
				`resource.attributes["environment"] == "production"`,
			},
		},
		{
			name:            "invalid syntax",
			conditions:      []string{`resource.attributes["service.name" == "auth-service"`},
			isErrorExpected: true,
		},
		{
			name:            "invalid function",
			conditions:      []string{`invalid_func(resource.attributes["service.name"]) == "auth-service"`},
			isErrorExpected: true,
		},
		{
			name:            "editor function",
			conditions:      []string{`truncate_all(resource.attributes, 100)`},
			isErrorExpected: true,
		},
		{
			name:            "invalid path",
			conditions:      []string{`resource.invalid["service.name"] == "auth-service"`},
			isErrorExpected: true,
		},
	}
}

func filterScopeContextTestCases() []filterTestCase {
	return []filterTestCase{
		{
			name:       "simple condition",
			conditions: []string{`scope.name == "io.opentelemetry.contrib.mongodb"`},
		},
		{
			name: "multiple conditions",
			conditions: []string{
				`scope.name == "io.opentelemetry.contrib.mongodb"`,
				`scope.version == "1.0.0"`,
			},
		},
		{
			name:       "attributes access",
			conditions: []string{`scope.attributes["custom.key"] == "value"`},
		},
		{
			name:            "invalid syntax",
			conditions:      []string{`scope.name == "io.opentelemetry.contrib.mongodb`},
			isErrorExpected: true,
		},
		{
			name:            "invalid function",
			conditions:      []string{`invalid_func(scope.name) == "io.opentelemetry.contrib.mongodb"`},
			isErrorExpected: true,
		},
		{
			name:            "editor function",
			conditions:      []string{`truncate_all(scope.attributes, 100)`},
			isErrorExpected: true,
		},
		{
			name:            "invalid path",
			conditions:      []string{`scope.invalid == "value"`},
			isErrorExpected: true,
		},
	}
}

func filterSpanContextTestCases() []filterTestCase {
	return []filterTestCase{
		{
			name:       "simple condition",
			conditions: []string{`span.name == "HTTP GET"`},
		},
		{
			name: "multiple conditions",
			conditions: []string{
				`span.name == "HTTP GET"`,
				`span.status.code == 1`,
			},
		},
		{
			name:       "attributes access",
			conditions: []string{`span.attributes["http.method"] == "GET"`},
		},
		{
			name:       "span context access",
			conditions: []string{`span.span_id != nil`},
		},
		{
			name:            "invalid syntax",
			conditions:      []string{`span.name == "HTTP GET`},
			isErrorExpected: true,
		},
		{
			name:            "invalid function",
			conditions:      []string{`invalid_func(span.name) == "HTTP GET"`},
			isErrorExpected: true,
		},
		{
			name:            "editor function",
			conditions:      []string{`truncate_all(span.attributes, 100)`},
			isErrorExpected: true,
		},
		{
			name:            "invalid path",
			conditions:      []string{`span.invalid == "value"`},
			isErrorExpected: true,
		},
		{
			name:       "uses IsRootSpan() function",
			conditions: []string{`span.name == "HTTP GET" and IsRootSpan() == true`},
		},
	}
}

func filterSpanEventContextTestCases() []filterTestCase {
	return []filterTestCase{
		{
			name:            "spanevent not supported",
			conditions:      []string{`spanevent.name == "exception"`},
			isErrorExpected: true,
		},
		{
			name:            "invalid syntax",
			conditions:      []string{`spanevent.name == "exception`},
			isErrorExpected: true,
		},
	}
}

func filterLogContextTestCases() []filterTestCase {
	return []filterTestCase{
		{
			name:       "simple condition",
			conditions: []string{`log.severity_text == "ERROR"`},
		},
		{
			name: "multiple conditions",
			conditions: []string{
				`log.severity_text == "ERROR"`,
				`log.body == "Database connection failed"`,
			},
		},
		{
			name:       "attributes access",
			conditions: []string{`log.attributes["service.name"] == "auth-service"`},
		},
		{
			name:       "severity number",
			conditions: []string{`log.severity_number >= 17`},
		},
		{
			name:       "timestamp access",
			conditions: []string{`log.time_unix_nano != nil`},
		},
		{
			name:            "invalid syntax",
			conditions:      []string{`log.severity_text == "ERROR`},
			isErrorExpected: true,
		},
		{
			name:            "invalid function",
			conditions:      []string{`invalid_func(log.body) == "error message"`},
			isErrorExpected: true,
		},
		{
			name:            "editor function",
			conditions:      []string{`truncate_all(log.attributes, 100)`},
			isErrorExpected: true,
		},
		{
			name:            "invalid path",
			conditions:      []string{`log.invalid == "value"`},
			isErrorExpected: true,
		},
	}
}

func filterMetricContextTestCases() []filterTestCase {
	return []filterTestCase{
		{
			name:       "simple condition",
			conditions: []string{`metric.name == "http_requests_total"`},
		},
		{
			name: "multiple conditions",
			conditions: []string{
				`metric.name == "http_requests_total"`,
				`metric.type == 1`,
			},
		},
		{
			name:       "unit access",
			conditions: []string{`metric.unit == "ms"`},
		},
		{
			name:            "invalid syntax",
			conditions:      []string{`metric.name == "http_requests_total`},
			isErrorExpected: true,
		},
		{
			name:            "invalid function",
			conditions:      []string{`invalid_func(metric.name) == "http_requests_total"`},
			isErrorExpected: true,
		},
		{
			name:            "editor function",
			conditions:      []string{`replace_pattern(metric.name, "^kube_([0-9A-Za-z]+_)", "k8s.$$1.")`},
			isErrorExpected: true,
		},
		{
			name:            "invalid path",
			conditions:      []string{`metric.invalid == "value"`},
			isErrorExpected: true,
		},
		{
			name: "unsupported metric context-only functions",
			conditions: []string{
				`HasAttrKeyOnDatapoint("metric, "environment")`,
				`HasAttrOnDatapoint(metric, "environment", "production")`,
			},
			isErrorExpected: true,
		},
	}
}

func filterDataPointContextTestCases() []filterTestCase {
	return []filterTestCase{
		{
			name:       "simple condition",
			conditions: []string{`datapoint.value_int > 100`},
		},
		{
			name: "multiple conditions",
			conditions: []string{
				`datapoint.value_int > 100`,
				`datapoint.time_unix_nano != nil`,
			},
		},
		{
			name:       "attributes access",
			conditions: []string{`datapoint.attributes["service.name"] == "auth-service"`},
		},
		{
			name:       "timestamp access",
			conditions: []string{`datapoint.start_time_unix_nano != nil`},
		},
		{
			name:       "exemplars access",
			conditions: []string{`datapoint.exemplars != nil`},
		},
		{
			name:            "invalid syntax",
			conditions:      []string{`datapoint.value > 100`},
			isErrorExpected: true,
		},
		{
			name:            "invalid function",
			conditions:      []string{`invalid_func(datapoint.value_int) > 100`},
			isErrorExpected: true,
		},
		{
			name:            "editor function",
			conditions:      []string{`truncate_all(datapoint.attributes, 100)`},
			isErrorExpected: true,
		},
		{
			name:            "invalid path",
			conditions:      []string{`datapoint.invalid == "value"`},
			isErrorExpected: true,
		},
	}
}

func filterMixedMetricContextTestCases() []filterTestCase {
	return []filterTestCase{
		{
			name:       "metric and datapoint access",
			conditions: []string{`metric.name == "http_requests_total" and datapoint.value_int > 100`},
		},
		{
			name:            "invalid mixed context access",
			conditions:      []string{`metric.invalid_field == "value" and datapoint.value_int > 100`},
			isErrorExpected: true,
		},
		{
			name:            "missing context",
			conditions:      []string{`attributes["service.name"] == "auth-service"`},
			isErrorExpected: true,
		},
	}
}
