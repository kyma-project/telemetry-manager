package ottl

import (
	"errors"
	"testing"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/stretchr/testify/require"
)

type filterResourceContextTestCase struct {
	name            string
	filters         []telemetryv1alpha1.FilterSpec
	isErrorExpected bool
}

func TestValidateLogPipelineFilters(t *testing.T) {
	tests := filterResourceContextTestCases()
	// tests = append(tests, filterScopeContextTestCases()...)
	// tests = append(tests, filterLogContextTestCases()...)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			validator, err := NewFilterSpecValidator(SignalTypeLog)
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
}

func filterResourceContextTestCases() []filterResourceContextTestCase {
	return []filterResourceContextTestCase{
		{
			name: "[resource context] valid filter spec - simple condition",
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
			name: "[resource context] valid filter spec - multiple conditions",
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
			name: "[resource context] invalid filter spec - missing context",
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
			name: "[resource context] invalid filter spec - invalid syntax",
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
			name: "[resource context] invalid filter spec - invalid function",
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
			name: "[resource context] invalid filter spec - converter function",
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
			name: "[resource context] invalid filter spec - invaid path",
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
