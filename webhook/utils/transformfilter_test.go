package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/validators/ottl"
)

func TestValidateFilterTransform(t *testing.T) {
	ctx := t.Context()

	tests := []struct {
		name                      string
		signalType                ottl.SignalType
		filterSpec                []telemetryv1beta1.FilterSpec
		transformSpec             []telemetryv1beta1.TransformSpec
		expectErr                 bool
		errFailedToCreatePipeline bool
	}{
		{
			name:          "empty specs",
			signalType:    ottl.SignalTypeMetric,
			filterSpec:    nil,
			transformSpec: nil,
		},
		{
			name:       "valid filters and transforms",
			signalType: ottl.SignalTypeMetric,
			filterSpec: []telemetryv1beta1.FilterSpec{
				{
					Conditions: []string{
						`datapoint.attributes["http.status_code"] == 200`,
					},
				},
			},
			transformSpec: []telemetryv1beta1.TransformSpec{
				{
					Statements: []string{
						`set(datapoint.attributes["new_attr"], "value")`,
					},
					Conditions: []string{
						`datapoint.attributes["foo"] != nil`,
					},
				},
			},
		},
		{
			name:                      "invalid signal type",
			signalType:                ottl.SignalType("invalid"),
			filterSpec:                []telemetryv1beta1.FilterSpec{},
			transformSpec:             []telemetryv1beta1.TransformSpec{},
			expectErr:                 true,
			errFailedToCreatePipeline: true,
		},
		{
			name:       "filter validation fails",
			signalType: ottl.SignalTypeMetric,
			filterSpec: []telemetryv1beta1.FilterSpec{
				{
					Conditions: []string{
						"invalid syntax",
					},
				},
			},
			transformSpec: nil,
			expectErr:     true,
		},
		{
			name:       "transform validation fails",
			signalType: ottl.SignalTypeMetric,
			filterSpec: nil,
			transformSpec: []telemetryv1beta1.TransformSpec{
				{
					Conditions: []string{
						"invalid condition syntax",
					},
				},
			},
			expectErr: true,
		},
		{
			name:       "both filter and transform validation fail",
			signalType: ottl.SignalTypeMetric,
			filterSpec: []telemetryv1beta1.FilterSpec{
				{
					Conditions: []string{"bad filter"},
				},
			},
			transformSpec: []telemetryv1beta1.TransformSpec{
				{
					Statements: []string{"bad transform"},
				},
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFilterTransform(ctx, tt.signalType, tt.filterSpec, tt.transformSpec)

			if tt.expectErr {
				assert.Error(t, err)

				if tt.errFailedToCreatePipeline {
					assert.ErrorIs(t, err, errFailedToCreatePipeline)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConvertFilterTransformToBeta(t *testing.T) {
	tests := []struct {
		name        string
		filters     []telemetryv1alpha1.FilterSpec
		transforms  []telemetryv1alpha1.TransformSpec
		checkResult func(t *testing.T, filters []telemetryv1beta1.FilterSpec, transforms []telemetryv1beta1.TransformSpec)
	}{
		{
			name:       "empty specs",
			filters:    nil,
			transforms: nil,
			checkResult: func(t *testing.T, filters []telemetryv1beta1.FilterSpec, transforms []telemetryv1beta1.TransformSpec) {
				assert.Empty(t, filters)
				assert.Empty(t, transforms)
			},
		},
		{
			name: "valid conversion",
			filters: []telemetryv1alpha1.FilterSpec{
				{
					Conditions: []string{"condition1", "condition2"},
				},
			},
			transforms: []telemetryv1alpha1.TransformSpec{
				{
					Statements: []string{"statement1", "statement2"},
				},
			},
			checkResult: func(t *testing.T, filters []telemetryv1beta1.FilterSpec, transforms []telemetryv1beta1.TransformSpec) {
				assert.Len(t, filters, 1)
				assert.Len(t, transforms, 1)
				assert.Contains(t, filters[0].Conditions, "condition1")
				assert.Contains(t, filters[0].Conditions, "condition2")
				assert.Contains(t, transforms[0].Statements, "statement1")
				assert.Contains(t, transforms[0].Statements, "statement2")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filters, transforms, err := ConvertFilterTransformToBeta(tt.filters, tt.transforms)

			assert.NoError(t, err)

			if tt.checkResult != nil {
				tt.checkResult(t, filters, transforms)
			}
		})
	}
}
