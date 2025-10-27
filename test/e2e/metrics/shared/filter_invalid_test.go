package shared

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestFilterInvalid(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelExperimental)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
	)

	pipeline := testutils.NewMetricPipelineBuilder().
		WithName(pipelineName).
		WithFilter(telemetryv1alpha1.FilterSpec{
			Conditions: []string{
				`Len(resource.attributes["k8s.namespace.name"]) > 0`, // perfectly valid condition with context prefix
				`attributes["foo"] == "bar"`,                         // invalid condition (missing context prefix)
			},
		}).
		WithOTLPOutput(testutils.OTLPEndpoint("https://backend.example.com:4317")).
		Build()

	t.Cleanup(func() {
		Expect(kitk8s.DeleteObjects(&pipeline)).To(Succeed())
	})
	Expect(kitk8s.CreateObjects(t, &pipeline)).To(Succeed())

	assert.MetricPipelineHasCondition(t, pipelineName, metav1.Condition{
		Type:   conditions.TypeConfigurationGenerated,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonOTTLSpecInvalid,
	})

	assert.MetricPipelineHasCondition(t, pipelineName, metav1.Condition{
		Type:   conditions.TypeFlowHealthy,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonSelfMonConfigNotGenerated,
	})

	assert.TelemetryHasState(t, operatorv1alpha1.StateWarning)
	assert.TelemetryHasCondition(t, suite.K8sClient, metav1.Condition{
		Type:   conditions.TypeMetricComponentsHealthy,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonOTTLSpecInvalid,
	})
}
