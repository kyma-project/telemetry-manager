package ottl

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

type transformTestCase struct {
	name            string
	conditions      []string
	statements      []string
	isErrorExpected bool
}

func TestTransformValidator(t *testing.T) {
	for _, signalType := range []SignalType{SignalTypeLog, SignalTypeMetric, SignalTypeTrace} {
		runTransformValidatorTestCases(t, "resource", signalType, transformResourceContextTestCases())
		runTransformValidatorTestCases(t, "scope", signalType, transformScopeContextTestCases())
	}

	runTransformValidatorTestCases(t, "span", SignalTypeTrace, transformSpanContextTestCases())
	runTransformValidatorTestCases(t, "spanevent", SignalTypeTrace, transformSpanEventContextTestCases())
	runTransformValidatorTestCases(t, "log", SignalTypeLog, transformLogContextTestCases())
	runTransformValidatorTestCases(t, "metric", SignalTypeMetric, transformMetricContextTestCases())
	runTransformValidatorTestCases(t, "datapoint", SignalTypeMetric, transformDataPointContextTestCases())
}

func runTransformValidatorTestCases(t *testing.T, context string, signalType SignalType, tests []transformTestCase) {
	t.Helper()

	t.Run(fmt.Sprintf("%s_%s", signalType, context), func(t *testing.T) {
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				validator, err := NewTransformSpecValidator(signalType)
				require.NoError(t, err)

				transforms := []telemetryv1alpha1.TransformSpec{{
					Conditions: test.conditions,
					Statements: test.statements,
				}}

				err = validator.Validate(transforms)
				if test.isErrorExpected {
					require.Error(t, err)
					require.True(t, IsInvalidOTTLSpecError(err))

					var typedErr *InvalidOTTLSpecError
					require.True(t, errors.As(err, &typedErr))
					require.Contains(t, typedErr.Error(), "invalid TransformSpec")
				} else {
					require.NoError(t, err)
				}
			})
		}
	})
}

func transformResourceContextTestCases() []transformTestCase {
	return []transformTestCase{
		{
			name:       "statement and condition",
			conditions: []string{`IsMatch(resource.attributes["test"], "bar")`},
			statements: []string{`set(resource.attributes["test"], "foo")`},
		},
		{
			name:       "condition only",
			conditions: []string{`IsMatch(resource.attributes["test"], "bar")`},
		},
		{
			name:       "statement only",
			statements: []string{`set(resource.attributes["test"], "foo")`},
		},
		{
			name:            "statement used as condition",
			conditions:      []string{`set(resource.attributes["test"], "foo")`},
			isErrorExpected: true,
		},
		{
			name:            "condition used as statement",
			statements:      []string{`IsMatch(resource.attributes["test"], "bar")`},
			isErrorExpected: true,
		},
		{
			name:            "invalid function in condition",
			conditions:      []string{`IisMatch(resource.attributes["test"], "bar")`},
			statements:      []string{`set(resource.attributes["test"], "foo")`},
			isErrorExpected: true,
		},
		{
			name:            "invalid function in statement",
			conditions:      []string{`IsMatch(resource.attributes["test"], "bar")`},
			statements:      []string{`sset(resource.attributes["test"], "foo")`},
			isErrorExpected: true,
		},
		{
			name:            "invalid syntax in condition",
			conditions:      []string{`IsMatch(resource.attributes["test"], "bar"`},
			statements:      []string{`set(resource.attributes["test"], "foo")`},
			isErrorExpected: true,
		},
		{
			name:            "invalid syntax in statement",
			conditions:      []string{`IsMatch(resource.attributes["test"], "bar")`},
			statements:      []string{`set(resource.attributes["test"], "foo"`},
			isErrorExpected: true,
		},
		{
			name:            "invalid path in condition",
			conditions:      []string{`IsMatch(resource.invalid["test"], "bar")`},
			statements:      []string{`set(resource.attributes["test"], "foo")`},
			isErrorExpected: true,
		},
		{
			name:            "invalid path in statement",
			conditions:      []string{`IsMatch(resource.attributes["test"], "bar")`},
			statements:      []string{`set(resource.invalid["test"], "foo")`},
			isErrorExpected: true,
		},
		{
			name: "multiple conditions and statements",
			conditions: []string{
				`IsMatch(resource.attributes["service"], "auth")`,
				`resource.attributes["environment"] == "prod"`,
			},
			statements: []string{
				`set(resource.attributes["service.name"], "auth-service")`,
				`set(resource.attributes["version"], "1.0.0")`,
			},
		},
	}
}

func transformScopeContextTestCases() []transformTestCase {
	return []transformTestCase{
		{
			name:       "statement and condition",
			conditions: []string{`IsMatch(scope.name, "opentelemetry")`},
			statements: []string{`set(scope.attributes["version"], "1.0.0")`},
		},
		{
			name:       "condition only",
			conditions: []string{`IsMatch(scope.name, "opentelemetry")`},
		},
		{
			name:       "statement only",
			statements: []string{`set(scope.attributes["version"], "1.0.0")`},
		},
		{
			name:            "statement used as condition",
			conditions:      []string{`set(scope.attributes["version"], "1.0.0")`},
			isErrorExpected: true,
		},
		{
			name:            "condition used as statement",
			statements:      []string{`IsMatch(scope.name, "opentelemetry")`},
			isErrorExpected: true,
		},
		{
			name:            "invalid function in condition",
			conditions:      []string{`IisMatch(scope.name, "opentelemetry")`},
			statements:      []string{`set(scope.attributes["version"], "1.0.0")`},
			isErrorExpected: true,
		},
		{
			name:            "invalid function in statement",
			conditions:      []string{`IsMatch(scope.name, "opentelemetry")`},
			statements:      []string{`sset(scope.attributes["version"], "1.0.0")`},
			isErrorExpected: true,
		},
		{
			name:            "invalid syntax in condition",
			conditions:      []string{`IsMatch(scope.name, "opentelemetry"`},
			statements:      []string{`set(scope.attributes["version"], "1.0.0")`},
			isErrorExpected: true,
		},
		{
			name:            "invalid syntax in statement",
			conditions:      []string{`IsMatch(scope.name, "opentelemetry")`},
			statements:      []string{`set(scope.attributes["version"], "1.0.0"`},
			isErrorExpected: true,
		},
		{
			name:            "invalid path in condition",
			conditions:      []string{`IsMatch(scope.invalid, "opentelemetry")`},
			statements:      []string{`set(scope.attributes["version"], "1.0.0")`},
			isErrorExpected: true,
		},
		{
			name:            "invalid path in statement",
			conditions:      []string{`IsMatch(scope.name, "opentelemetry")`},
			statements:      []string{`set(scope.invalid["version"], "1.0.0")`},
			isErrorExpected: true,
		},
		{
			name: "multiple conditions and statements",
			conditions: []string{
				`IsMatch(scope.name, "opentelemetry")`,
				`scope.version != nil`,
			},
			statements: []string{
				`set(scope.attributes["processed"], "true")`,
				`set(scope.attributes["timestamp"], Now())`,
			},
		},
	}
}

func transformLogContextTestCases() []transformTestCase {
	return []transformTestCase{
		{
			name:       "statement and condition",
			conditions: []string{`log.severity_text == "ERROR"`},
			statements: []string{`set(log.attributes["processed"], "true")`},
		},
		{
			name:       "condition only",
			conditions: []string{`log.severity_text == "ERROR"`},
		},
		{
			name:       "statement only",
			statements: []string{`set(log.attributes["processed"], "true")`},
		},
		{
			name:            "statement used as condition",
			conditions:      []string{`set(log.attributes["processed"], "true")`},
			isErrorExpected: true,
		},
		{
			name:            "condition used as statement",
			statements:      []string{`log.severity_text == "ERROR"`},
			isErrorExpected: true,
		},
		{
			name:            "invalid function in condition",
			conditions:      []string{`llog.severity_text == "ERROR"`},
			statements:      []string{`set(log.attributes["processed"], "true")`},
			isErrorExpected: true,
		},
		{
			name:            "invalid function in statement",
			conditions:      []string{`log.severity_text == "ERROR"`},
			statements:      []string{`sset(log.attributes["processed"], "true")`},
			isErrorExpected: true,
		},
		{
			name:            "invalid syntax in condition",
			conditions:      []string{`log.severity_text == "ERROR`},
			statements:      []string{`set(log.attributes["processed"], "true")`},
			isErrorExpected: true,
		},
		{
			name:            "invalid syntax in statement",
			conditions:      []string{`log.severity_text == "ERROR"`},
			statements:      []string{`set(log.attributes["processed"], "true"`},
			isErrorExpected: true,
		},
		{
			name:            "invalid path in condition",
			conditions:      []string{`log.invalid == "ERROR"`},
			statements:      []string{`set(log.attributes["processed"], "true")`},
			isErrorExpected: true,
		},
		{
			name:            "invalid path in statement",
			conditions:      []string{`log.severity_text == "ERROR"`},
			statements:      []string{`set(log.invalid["processed"], "true")`},
			isErrorExpected: true,
		},
		{
			name: "multiple conditions and statements",
			conditions: []string{
				`log.severity_text == "ERROR"`,
				`IsMatch(log.body, "database")`,
			},
			statements: []string{
				`set(log.attributes["error_category"], "database")`,
				`set(log.attributes["processed"], "true")`,
			},
		},
		{
			name:       "body manipulation",
			statements: []string{`replace_pattern(log.body, "password=\\w+", "password=***")`},
		},
		{
			name:       "severity manipulation",
			conditions: []string{`log.severity_number >= 17`},
			statements: []string{`set(log.severity_text, "CRITICAL")`},
		},
	}
}

func transformSpanContextTestCases() []transformTestCase {
	return []transformTestCase{
		{
			name:       "statement and condition",
			conditions: []string{`span.name == "HTTP GET"`},
			statements: []string{`set(span.attributes["processed"], "true")`},
		},
		{
			name:       "condition only",
			conditions: []string{`span.name == "HTTP GET"`},
		},
		{
			name:       "statement only",
			statements: []string{`set(span.attributes["processed"], "true")`},
		},
		{
			name:            "statement used as condition",
			conditions:      []string{`set(span.attributes["processed"], "true")`},
			isErrorExpected: true,
		},
		{
			name:            "condition used as statement",
			statements:      []string{`span.name == "HTTP GET"`},
			isErrorExpected: true,
		},
		{
			name:            "invalid function in condition",
			conditions:      []string{`sspan.name == "HTTP GET"`},
			statements:      []string{`set(span.attributes["processed"], "true")`},
			isErrorExpected: true,
		},
		{
			name:            "invalid function in statement",
			conditions:      []string{`span.name == "HTTP GET"`},
			statements:      []string{`sset(span.attributes["processed"], "true")`},
			isErrorExpected: true,
		},
		{
			name:            "invalid syntax in condition",
			conditions:      []string{`span.name == "HTTP GET`},
			statements:      []string{`set(span.attributes["processed"], "true")`},
			isErrorExpected: true,
		},
		{
			name:            "invalid syntax in statement",
			conditions:      []string{`span.name == "HTTP GET"`},
			statements:      []string{`set(span.attributes["processed"], "true"`},
			isErrorExpected: true,
		},
		{
			name:            "invalid path in condition",
			conditions:      []string{`span.invalid == "HTTP GET"`},
			statements:      []string{`set(span.attributes["processed"], "true")`},
			isErrorExpected: true,
		},
		{
			name:            "invalid path in statement",
			conditions:      []string{`span.name == "HTTP GET"`},
			statements:      []string{`set(span.invalid["processed"], "true")`},
			isErrorExpected: true,
		},
		{
			name: "multiple conditions and statements",
			conditions: []string{
				`span.name == "HTTP GET"`,
				`span.status.code == 1`,
			},
			statements: []string{
				`set(span.attributes["http_method"], "GET")`,
				`set(span.attributes["processed"], "true")`,
			},
		},
		{
			name:       "span name manipulation",
			statements: []string{`replace_pattern(span.name, "^HTTP\\s+", "HTTP_")`},
		},
		{
			name:       "span status manipulation",
			conditions: []string{`span.status.code == 2`},
			statements: []string{`set(span.status.message, "Request failed")`},
		},
	}
}

func transformSpanEventContextTestCases() []transformTestCase {
	return []transformTestCase{
		{
			name:       "statement and condition",
			conditions: []string{`spanevent.name == "exception"`},
			statements: []string{`set(spanevent.attributes["processed"], "true")`},
		},
		{
			name:       "condition only",
			conditions: []string{`spanevent.name == "exception"`},
		},
		{
			name:       "statement only",
			statements: []string{`set(spanevent.attributes["processed"], "true")`},
		},
		{
			name:            "statement used as condition",
			conditions:      []string{`set(spanevent.attributes["processed"], "true")`},
			isErrorExpected: true,
		},
		{
			name:            "condition used as statement",
			statements:      []string{`spanevent.name == "exception"`},
			isErrorExpected: true,
		},
		{
			name:            "invalid function in condition",
			conditions:      []string{`sspanevent.name == "exception"`},
			statements:      []string{`set(spanevent.attributes["processed"], "true")`},
			isErrorExpected: true,
		},
		{
			name:            "invalid function in statement",
			conditions:      []string{`spanevent.name == "exception"`},
			statements:      []string{`sset(spanevent.attributes["processed"], "true")`},
			isErrorExpected: true,
		},
		{
			name:            "invalid syntax in condition",
			conditions:      []string{`spanevent.name == "exception`},
			statements:      []string{`set(spanevent.attributes["processed"], "true")`},
			isErrorExpected: true,
		},
		{
			name:            "invalid syntax in statement",
			conditions:      []string{`spanevent.name == "exception"`},
			statements:      []string{`set(spanevent.attributes["processed"], "true"`},
			isErrorExpected: true,
		},
		{
			name:            "invalid path in condition",
			conditions:      []string{`spanevent.invalid == "exception"`},
			statements:      []string{`set(spanevent.attributes["processed"], "true")`},
			isErrorExpected: true,
		},
		{
			name:            "invalid path in statement",
			conditions:      []string{`spanevent.name == "exception"`},
			statements:      []string{`set(spanevent.invalid["processed"], "true")`},
			isErrorExpected: true,
		},
		{
			name: "multiple conditions and statements",
			conditions: []string{
				`spanevent.name == "exception"`,
				`IsMatch(spanevent.attributes["exception.type"], ".*Error")`,
			},
			statements: []string{
				`set(spanevent.attributes["severity"], "high")`,
				`set(spanevent.attributes["processed"], "true")`,
			},
		},
		{
			name:       "event name manipulation",
			statements: []string{`replace_pattern(spanevent.name, "^exception$", "error_event")`},
		},
		{
			name:       "timestamp manipulation",
			statements: []string{`set(spanevent.time_unix_nano, Now())`},
		},
	}
}

func transformMetricContextTestCases() []transformTestCase {
	return []transformTestCase{
		{
			name:       "statement and condition",
			conditions: []string{`metric.name == "http_requests_total"`},
			statements: []string{`set(metric.description, "Total HTTP requests")`},
		},
		{
			name:       "condition only",
			conditions: []string{`metric.name == "http_requests_total"`},
		},
		{
			name:       "statement only",
			statements: []string{`set(metric.description, "Total HTTP requests")`},
		},
		{
			name:            "statement used as condition",
			conditions:      []string{`set(metric.description, "Total HTTP requests")`},
			isErrorExpected: true,
		},
		{
			name:            "condition used as statement",
			statements:      []string{`metric.name == "http_requests_total"`},
			isErrorExpected: true,
		},
		{
			name:            "invalid function in condition",
			conditions:      []string{`mmetric.name == "http_requests_total"`},
			statements:      []string{`set(metric.description, "Total HTTP requests")`},
			isErrorExpected: true,
		},
		{
			name:            "invalid function in statement",
			conditions:      []string{`metric.name == "http_requests_total"`},
			statements:      []string{`sset(metric.description, "Total HTTP requests")`},
			isErrorExpected: true,
		},
		{
			name:            "invalid syntax in condition",
			conditions:      []string{`metric.name == "http_requests_total`},
			statements:      []string{`set(metric.description, "Total HTTP requests")`},
			isErrorExpected: true,
		},
		{
			name:            "invalid syntax in statement",
			conditions:      []string{`metric.name == "http_requests_total"`},
			statements:      []string{`set(metric.description, "Total HTTP requests"`},
			isErrorExpected: true,
		},
		{
			name:            "invalid path in condition",
			conditions:      []string{`metric.invalid == "http_requests_total"`},
			statements:      []string{`set(metric.description, "Total HTTP requests")`},
			isErrorExpected: true,
		},
		{
			name:            "invalid path in statement",
			conditions:      []string{`metric.name == "http_requests_total"`},
			statements:      []string{`set(metric.invalid, "Total HTTP requests")`},
			isErrorExpected: true,
		},
		{
			name: "multiple conditions and statements",
			conditions: []string{
				`metric.name == "http_requests_total"`,
				`metric.type == 1`,
			},
			statements: []string{
				`set(metric.description, "Total HTTP requests")`,
				`set(metric.unit, "requests")`,
			},
		},
		{
			name:       "name manipulation",
			statements: []string{`replace_pattern(metric.name, "^kube_", "k8s_")`},
		},
		{
			name:       "unit manipulation",
			conditions: []string{`metric.unit == "ms"`},
			statements: []string{`set(metric.unit, "milliseconds")`},
		},
	}
}

func transformDataPointContextTestCases() []transformTestCase {
	return []transformTestCase{
		{
			name:       "statement and condition",
			conditions: []string{`datapoint.value_int > 100`},
			statements: []string{`set(datapoint.attributes["high_value"], "true")`},
		},
		{
			name:       "condition only",
			conditions: []string{`datapoint.value_int > 100`},
		},
		{
			name:       "statement only",
			statements: []string{`set(datapoint.attributes["processed"], "true")`},
		},
		{
			name:            "statement used as condition",
			conditions:      []string{`set(datapoint.attributes["processed"], "true")`},
			isErrorExpected: true,
		},
		{
			name:            "condition used as statement",
			statements:      []string{`datapoint.value_int > 100`},
			isErrorExpected: true,
		},
		{
			name:            "invalid function in condition",
			conditions:      []string{`IisMatch(datapoint.attributes["processed"], "true")`},
			statements:      []string{`set(datapoint.attributes["processed"], "true")`},
			isErrorExpected: true,
		},
		{
			name:            "invalid function in statement",
			conditions:      []string{`IsMatch(datapoint.attributes["test"], "bar")`},
			statements:      []string{`sset(datapoint.attributes["processed"], "true")`},
			isErrorExpected: true,
		},
		{
			name:            "invalid syntax in condition",
			conditions:      []string{`IsMatch(datapoint.attributes["test"],, "bar")`},
			statements:      []string{`set(datapoint.attributes["processed"], "true")`},
			isErrorExpected: true,
		},
		{
			name:            "invalid syntax in statement",
			conditions:      []string{`IsMatch(datapoint.attributes["test"], "bar")`},
			statements:      []string{`set(datapoint.attributes["processed"],, "true"`},
			isErrorExpected: true,
		},
		{
			name:            "invalid path in condition",
			conditions:      []string{`IsMatch(datapoint.aattributes["test"], "bar")`},
			statements:      []string{`set(datapoint.attributes["processed"], "true"`},
			isErrorExpected: true,
		},
		{
			name:            "invalid path in statement",
			conditions:      []string{`IsMatch(datapoint.attributes["test"], "bar")`},
			statements:      []string{`set(datapoint.aattributes["processed"], "true"`},
			isErrorExpected: true,
		},
		{
			name: "multiple conditions and statements",
			conditions: []string{
				`datapoint.value_int > 100`,
				`datapoint.time_unix_nano != nil`,
			},
			statements: []string{
				`set(datapoint.attributes["high_value"], "true")`,
				`set(datapoint.attributes["processed"], "true")`,
			},
		},
		{
			name:       "value manipulation",
			conditions: []string{`datapoint.value_double < 0`},
			statements: []string{`set(datapoint.value_double, 0.0)`},
		},
		{
			name:       "timestamp manipulation",
			statements: []string{`set(datapoint.start_time_unix_nano, datapoint.time_unix_nano)`},
		},
	}
}
