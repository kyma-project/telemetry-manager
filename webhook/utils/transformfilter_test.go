package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/validators/ottl"
)

func TestValidateFilterTransform(t *testing.T) {
	err := ValidateFilterTransform(
		ottl.SignalTypeLog,
		[]telemetryv1beta1.FilterSpec{},
		[]telemetryv1beta1.TransformSpec{},
	)
	assert.NoError(t, err)
}

func TestInvalidSignalType(t *testing.T) {
	err := ValidateFilterTransform(
		"invalid",
		[]telemetryv1beta1.FilterSpec{},
		[]telemetryv1beta1.TransformSpec{},
	)
	assert.ErrorContains(t, err, "failed to instantiate")
}

func TestWithValidFilterTransform(t *testing.T) {
	err := ValidateFilterTransform(
		ottl.SignalTypeLog,
		[]telemetryv1beta1.FilterSpec{
			{Conditions: []string{`log.severity_number < SEVERITY_NUMBER_WARN`}},
		},
		[]telemetryv1beta1.TransformSpec{
			{Statements: []string{`set(resource.attributes["deployment.environment.name"], "production")`}},
			{Conditions: []string{`IsMatch(resource.attributes["k8s.namespace.name"], ".*-system")`}},
		},
	)
	assert.NoError(t, err)
}

func TestWithInvalidFilter(t *testing.T) {
	err := ValidateFilterTransform(
		ottl.SignalTypeLog,
		[]telemetryv1beta1.FilterSpec{
			{Conditions: []string{`invalid condition`}},
		},
		[]telemetryv1beta1.TransformSpec{
			{Statements: []string{`set(resource.attributes["deployment.environment.name"], "production")`}},
			{Conditions: []string{`IsMatch(resource.attributes["k8s.namespace.name"], ".*-system")`}},
		},
	)
	assert.Error(t, err)
}

func TestWithInvalidTransform(t *testing.T) {
	err := ValidateFilterTransform(
		ottl.SignalTypeLog,
		[]telemetryv1beta1.FilterSpec{
			{Conditions: []string{`log.severity_number < SEVERITY_NUMBER_WARN`}},
		},
		[]telemetryv1beta1.TransformSpec{
			{Statements: []string{`invalid statement`}},
			{Conditions: []string{`IsMatch(resource.attributes["k8s.namespace.name"], ".*-system")`}},
		},
	)
	assert.Error(t, err)
}

func TestConvertFilterTransformToBeta(t *testing.T) {
	var (
		filterSpec    []telemetryv1alpha1.FilterSpec
		transformSpec []telemetryv1alpha1.TransformSpec
	)

	filterSpecBeta, transformSpecBeta := ConvertFilterTransformToBeta(filterSpec, transformSpec)

	assert.IsType(t, filterSpecBeta, []telemetryv1beta1.FilterSpec{})
	assert.IsType(t, transformSpecBeta, []telemetryv1beta1.TransformSpec{})
}
