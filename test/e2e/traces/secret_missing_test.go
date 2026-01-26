package traces

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1beta1 "github.com/kyma-project/telemetry-manager/apis/operator/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestSecretMissing(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelTraces)

	const (
		endpointKey   = "traces-endpoint"
		endpointValue = "http://localhost:4317"
	)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
		secretName   = uniquePrefix()
	)

	secret := kitk8sobjects.NewOpaqueSecret(secretName, kitkyma.DefaultNamespaceName, kitk8sobjects.WithStringData(endpointKey, endpointValue))

	pipeline := testutils.NewTracePipelineBuilder().
		WithName(pipelineName).
		WithOTLPOutput(testutils.OTLPEndpointFromSecret(
			secret.Name(),
			secret.Namespace(),
			endpointKey,
		)).
		Build()

	resources := []client.Object{
		&pipeline,
	}

	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	assert.TracePipelineHasCondition(t, pipelineName, metav1.Condition{
		Type:   conditions.TypeConfigurationGenerated,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonReferencedSecretMissing,
	})

	assert.TracePipelineHasCondition(t, pipelineName, metav1.Condition{
		Type:   conditions.TypeFlowHealthy,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonSelfMonConfigNotGenerated,
	})

	assert.TelemetryHasState(t, operatorv1beta1.StateWarning)
	assert.TelemetryHasCondition(t, suite.K8sClient, metav1.Condition{
		Type:   conditions.TypeTraceComponentsHealthy,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonReferencedSecretMissing,
	})

	// Create the secret and make sure the pipeline heals
	Expect(kitk8s.CreateObjects(t, secret.K8sObject())).To(Succeed())

	assert.TracePipelineHealthy(t, pipelineName)
}
